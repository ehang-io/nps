// Package selfupdate lets a node replace its own binary with a release
// artifact from GitHub, and lets the new binary prove itself before the old
// one is thrown away.
//
// The proof has to happen locally. Whoever triggers an update -- an operator
// on the box, or the server pushing one down the control channel -- loses
// contact with the node the moment it restarts, so nobody upstream can
// supervise the outcome. Instead Apply leaves a marker next to the binary;
// the new binary clears it once it has connected to the server, and puts the
// backup back if it never does. A binary too broken to reach that check at
// all is caught by the attempt counter, which survives a crash loop.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego/logs"
)

// Repo is the GitHub repository releases are pulled from. Override it at
// build time with:
//
//	-X ehang.io/nps/lib/selfupdate.Repo=owner/name
var Repo = version.Fork

// Role is which artifact this binary is, "npc" or "nps". It decides both the
// release asset to fetch and the member to take out of it, and it is set here
// rather than read from the binary's filename: a renamed npc, or an nps.exe,
// would otherwise silently fetch the wrong side of the release.
var Role = "npc"

// tagPattern is what a release tag is allowed to look like. The tag arrives
// from the server over the control connection and is interpolated into a
// download URL, so without this a tag containing path segments would send the
// node to a different repository entirely -- and since the checksum file is
// fetched from that same base, it would verify the attacker's archive against
// the attacker's checksums and then exec the result.
var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)

// ValidTag reports whether tag is safe to put in a release URL. An empty tag
// means "the latest release" and is always allowed.
func ValidTag(tag string) bool {
	return tag == "" || tagPattern.MatchString(tag)
}

const (
	// VerifyWindow is how long a freshly installed binary has to connect to
	// the server before it is considered a failure and rolled back.
	VerifyWindow = 60 * time.Second

	// MaxAttempts bounds how many times a binary may start without ever
	// confirming. It exists for the binary that crashes before it can even
	// read the marker: systemd keeps restarting it, the counter keeps
	// climbing, and the rollback eventually happens without anyone present.
	MaxAttempts = 3

	// keepBackups bounds how many previous binaries are kept beside the
	// current one.
	keepBackups = 3

	sumsAsset      = "sha256sums.txt"
	downloadWindow = 10 * time.Minute
	smokeWindow    = 10 * time.Second
)

// applying and confirmed are plain atomics rather than a sync.Mutex.TryLock,
// which the Go 1.15 toolchain this project builds with does not have.
var (
	applying  int32
	confirmed int32
)

type marker struct {
	Backup      string `json:"backup"`
	Attempts    int    `json:"attempts"`
	FromVersion string `json:"from_version"`
	Tag         string `json:"tag"`
	StartedAt   int64  `json:"started_at"`
}

// selfPath returns the absolute, symlink-free path of the running binary.
func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}

func markerPath(bin string) string {
	return filepath.Join(filepath.Dir(bin), "."+filepath.Base(bin)+".update-pending")
}

// assetName maps this build onto the release asset that carries it. The
// GOOS_GOARCH_kind form has to stay in step with build.release.sh, which
// labels its output ${goos}_${goarch}: inventing a finer-grained label here
// (arm_v7 for GOARCH arm, say) just produces a 404 on the node.
func assetName() string {
	kind := "client"
	if Role == "nps" {
		kind = "server"
	}
	return fmt.Sprintf("%s_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH, kind)
}

func releaseURL(tag, asset string) string {
	if tag == "" {
		// This redirect always points at the newest release and, unlike the
		// API, is not rate limited per source IP.
		return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", Repo, asset)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", Repo, tag, asset)
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "npc-selfupdate/"+version.VERSION)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

// verifySum checks the archive against the checksum file published with the
// release. An unlisted asset is a failure, not a pass: the whole point of the
// file is that it is the one thing in a curl-driven update that can be
// checked, so a missing entry means the release is not one we can trust.
func verifySum(sums []byte, asset string, archive []byte) error {
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && filepath.Base(f[len(f)-1]) == asset {
			want = strings.ToLower(f[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("selfupdate: %s is not listed in %s", asset, sumsAsset)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("selfupdate: checksum mismatch for %s: got %s, want %s", asset, got, want)
	}
	return nil
}

// extractBinary pulls exactly one member out of the archive: the executable.
// The release tarballs also carry conf/npc.conf, and unpacking those over a
// live node would replace its real configuration -- its vkey and every tunnel
// it serves -- with the template. Nothing but the binary is ever written.
func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != name {
			continue
		}
		return ioutil.ReadAll(tr)
	}
	return nil, fmt.Errorf("selfupdate: no %s inside the release archive", name)
}

// smokeTest runs the staged binary with -version, which both npc and nps
// answer by printing and exiting. It catches the failures worth catching
// before committing: a truncated download, a binary for the wrong
// architecture, one that cannot link. It cannot prove the new binary tunnels
// traffic -- that is what the verify window is for.
//
// -version specifically, never a bare word: npc treats an argument it does
// not recognise as "start the service", so a careless smoke test would launch
// a second client alongside the running one.
func smokeTest(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), smokeWindow)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("selfupdate: staged binary failed its smoke test: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return errors.New("selfupdate: staged binary printed no version")
	}
	logs.Info("selfupdate: staged binary reports %s", strings.TrimSpace(string(out)))
	return nil
}

// Apply downloads the release for tag (empty means the latest one), installs
// it over the running binary and restarts into it. It does not return on
// success: the process is replaced.
func Apply(tag string) error {
	if !ValidTag(tag) {
		// Refused before it reaches a URL. The server is meant to have
		// checked too, but a node must not depend on that: this string
		// decides which repository the binary it is about to exec comes from.
		return fmt.Errorf("selfupdate: refusing unsafe release tag %q", tag)
	}
	if runtime.GOOS == "windows" {
		// The swap below is a rename over the running image, which Windows
		// refuses, and restart there can only exit -- which the service
		// manager reads as a clean stop and does not undo. Failing here
		// keeps a Windows node up instead of taking it down for good.
		return errors.New("selfupdate: in-place update is not supported on windows")
	}
	if !atomic.CompareAndSwapInt32(&applying, 0, 1) {
		return errors.New("selfupdate: an update is already in progress")
	}
	defer atomic.StoreInt32(&applying, 0)

	bin, err := selfPath()
	if err != nil {
		return err
	}
	asset := assetName()

	ctx, cancel := context.WithTimeout(context.Background(), downloadWindow)
	defer cancel()

	logs.Info("selfupdate: fetching %s", releaseURL(tag, asset))
	archive, err := fetch(ctx, releaseURL(tag, asset))
	if err != nil {
		return err
	}
	sums, err := fetch(ctx, releaseURL(tag, sumsAsset))
	if err != nil {
		return fmt.Errorf("selfupdate: cannot fetch %s: %v", sumsAsset, err)
	}
	if err := verifySum(sums, asset, archive); err != nil {
		return err
	}
	logs.Info("selfupdate: checksum ok, %d bytes", len(archive))

	payload, err := extractBinary(archive, Role)
	if err != nil {
		return err
	}

	// Stage beside the target so the final move is a rename on the same
	// filesystem. A running executable cannot be written to (ETXTBSY), but
	// its directory entry can be replaced.
	staged := bin + ".new"
	if err := ioutil.WriteFile(staged, payload, 0755); err != nil {
		return err
	}
	defer os.Remove(staged)
	if err := smokeTest(staged); err != nil {
		return err
	}

	// Old backups go before the new one is written, not after. Each is a
	// full copy of a twelve megabyte binary, and on a node with a small
	// rootfs the first thing an unbounded pile breaks is the next update.
	pruneBackups(bin, keepBackups-1)

	backup := fmt.Sprintf("%s.bak.%s", bin, time.Now().Format("20060102-150405"))
	if err := copyFile(bin, backup); err != nil {
		return fmt.Errorf("selfupdate: cannot back up the current binary: %v", err)
	}

	// The marker goes down before the swap. If anything from here on fails,
	// including a machine that loses power mid-update, the next start finds
	// it and can put the old binary back.
	m := &marker{
		Backup:      backup,
		FromVersion: version.VERSION,
		Tag:         tag,
		StartedAt:   time.Now().Unix(),
	}
	if err := saveMarker(markerPath(bin), m); err != nil {
		os.Remove(backup)
		return err
	}

	if err := os.Rename(staged, bin); err != nil {
		os.Remove(markerPath(bin))
		os.Remove(backup)
		return err
	}

	logs.Info("selfupdate: installed, restarting into the new binary")
	return restart(bin)
}

// OnStart is called once, early, by a starting binary. If an update is being
// verified it either counts this attempt and arms the watchdog, or gives up
// and rolls back.
func OnStart() {
	bin, err := selfPath()
	if err != nil {
		return
	}
	path := markerPath(bin)
	m, err := loadMarker(path)
	if err != nil || m == nil {
		return
	}

	m.Attempts++
	if m.Attempts > MaxAttempts {
		logs.Error("selfupdate: this binary started %d times without ever connecting, rolling back to %s",
			m.Attempts-1, m.Backup)
		rollback(bin, path, m)
		return
	}
	if err := saveMarker(path, m); err != nil {
		logs.Warn("selfupdate: cannot update the marker: %v", err)
	}
	logs.Info("selfupdate: verifying this build, attempt %d of %d, %s to connect",
		m.Attempts, MaxAttempts, VerifyWindow)

	go func() {
		time.Sleep(VerifyWindow)
		if atomic.LoadInt32(&confirmed) == 1 {
			return
		}
		logs.Error("selfupdate: no connection to the server within %s, rolling back to %s",
			VerifyWindow, m.Backup)
		rollback(bin, path, m)
	}()
}

// Confirm is called by the client the first time it connects to the server.
// That is the event that makes a new binary trustworthy: it starts, it talks
// the protocol, and the server accepted it.
func Confirm() {
	if !atomic.CompareAndSwapInt32(&confirmed, 0, 1) {
		return
	}
	bin, err := selfPath()
	if err != nil {
		return
	}
	path := markerPath(bin)
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := os.Remove(path); err != nil {
		logs.Warn("selfupdate: cannot clear the marker: %v", err)
		return
	}
	logs.Info("selfupdate: this build connected successfully, update confirmed")
}

func rollback(bin, path string, m *marker) {
	if _, err := os.Stat(m.Backup); err != nil {
		logs.Error("selfupdate: backup %s is gone, cannot roll back: %v", m.Backup, err)
		os.Remove(path)
		return
	}
	if err := os.Rename(m.Backup, bin); err != nil {
		logs.Error("selfupdate: cannot restore %s: %v", m.Backup, err)
		return
	}
	os.Remove(path)
	logs.Info("selfupdate: restored the previous binary, restarting")
	if err := restart(bin); err != nil {
		logs.Error("selfupdate: cannot restart after rollback: %v", err)
	}
}

func loadMarker(path string) (*marker, error) {
	b, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := new(marker)
	if err := json.Unmarshal(b, m); err != nil {
		// An unreadable marker must not wedge the node forever.
		logs.Warn("selfupdate: discarding an unreadable marker: %v", err)
		os.Remove(path)
		return nil, nil
	}
	return m, nil
}

func saveMarker(path string, m *marker) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0600)
}

// pruneBackups keeps the newest keep backups next to bin and removes the rest.
// Names carry a sortable timestamp, so lexical order is chronological.
func pruneBackups(bin string, keep int) {
	if keep < 0 {
		keep = 0
	}
	found, err := filepath.Glob(bin + ".bak.*")
	if err != nil || len(found) <= keep {
		return
	}
	sort.Strings(found)
	for _, old := range found[:len(found)-keep] {
		if err := os.Remove(old); err == nil {
			logs.Info("selfupdate: pruned old backup %s", old)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, in, 0755)
}
