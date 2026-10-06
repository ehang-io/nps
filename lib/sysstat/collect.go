package sysstat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// SampleInterval is how often the collector reads the host.
const SampleInterval = 2 * time.Second

var (
	startOnce sync.Once
	latest    atomic.Value // []byte
)

// Latest starts the collector on first use and returns its newest record, nil
// until the first sample two seconds later: CPU use is measured between two
// reads, so the first read has nothing to measure against. The record is
// shared; callers must not modify it.
func Latest() []byte {
	startOnce.Do(func() {
		c := newCollector("/sys/class/drm")
		if path, err := exec.LookPath("nvidia-smi"); err == nil {
			c.nvidia = startNvidia(context.Background(), path, nvidiaArgs...)
		}
		c.read() // the baseline for the CPU reads that follow
		go func() {
			for range time.Tick(SampleInterval) {
				latest.Store(Encode(c.read()))
			}
		}()
	})
	b, _ := latest.Load().([]byte)
	return b
}

type gpuNode struct {
	dir        string // hwmon directory
	lastEnergy float64
	lastAt     time.Time
}

type collector struct {
	gpus   []*gpuNode
	nvidia *nvidiaStream // nil without nvidia-smi
}

var cardRe = regexp.MustCompile(`^card\d+$`)

// newCollector finds the GPUs under drmDir (/sys/class/drm) that expose power
// through hwmon, which is how i915, xe and amdgpu do it.
func newCollector(drmDir string) *collector {
	c := &collector{}
	cards, _ := os.ReadDir(drmDir)
	var names []string
	for _, e := range cards {
		if cardRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		// NVIDIA's own driver has no hwmon power and nvidia-smi covers it; with
		// nouveau there is some, and the card would count twice
		if v, err := os.ReadFile(filepath.Join(drmDir, name, "device", "vendor")); err == nil && strings.TrimSpace(string(v)) == "0x10de" {
			continue
		}
		dirs, _ := filepath.Glob(filepath.Join(drmDir, name, "device", "hwmon", "hwmon*"))
		for _, d := range dirs {
			if hasAny(d, "power1_input", "power1_average", "energy1_input") {
				c.gpus = append(c.gpus, &gpuNode{dir: d})
				break
			}
		}
	}
	return c
}

func hasAny(dir string, files ...string) bool {
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

func readFloat(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	return v, err == nil
}

// read takes a sample. Anything that cannot be read is left zero.
func (c *collector) read() Sample {
	var s Sample
	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		s.CPU = p[0]
	}
	if m, err := mem.VirtualMemory(); err == nil {
		s.Mem = m.UsedPercent
	}
	now := time.Now()
	for _, g := range c.gpus {
		s.GPUs = append(s.GPUs, g.read(now))
	}
	if c.nvidia != nil {
		s.GPUs = append(s.GPUs, c.nvidia.read(now)...)
	}
	return s
}

// read returns the card's power in watts. hwmon reports microwatts and
// microjoules. Cards with no instantaneous power node only count energy, so
// their power is the energy used since the last read over the time since.
func (g *gpuNode) read(now time.Time) (r GPU) {
	for _, f := range []string{"power1_input", "power1_average"} {
		if v, ok := readFloat(filepath.Join(g.dir, f)); ok {
			r.Power = v / 1e6
			break
		}
	}
	// The baseline moves on every read, not only when it is used: one that
	// went stale while power1_input was reading would turn the first zero into
	// an average over all that time.
	if e, ok := readFloat(filepath.Join(g.dir, "energy1_input")); ok {
		if r.Power == 0 && !g.lastAt.IsZero() && e >= g.lastEnergy {
			if dt := now.Sub(g.lastAt).Seconds(); dt > 0 {
				r.Power = (e - g.lastEnergy) / 1e6 / dt
			}
		}
		g.lastEnergy, g.lastAt = e, now
	}
	for _, f := range []string{"power1_cap", "power1_max"} {
		if v, ok := readFloat(filepath.Join(g.dir, f)); ok && v > 0 {
			r.Limit = v / 1e6
			break
		}
	}
	return
}
