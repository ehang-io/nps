package sysstat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	in := Sample{CPU: 37.3, Mem: 99.9, GPUs: []GPU{{Power: 123, Limit: 600}, {Power: 0, Limit: 0}, {Power: 2000, Limit: 2000}}}
	rec := Encode(in)
	out, ok := Decode(rec)
	if !ok {
		t.Fatal("decode failed")
	}
	if out.CPU != 37.5 || out.Mem != 100 || out.GPUs != 3 {
		t.Fatalf("got %+v", out)
	}
	if out.Power[0] != 124 || out.Limit[0] != 600 || out.Power[1] != 0 || out.Power[2] != 1020 {
		t.Fatalf("gpu got %+v", out)
	}
}

func TestMoreGPUsThanSlots(t *testing.T) {
	in := Sample{GPUs: make([]GPU, 10)}
	out, _ := Decode(Encode(in))
	if out.GPUs != 10 || len(out.Power) != MaxGPU {
		t.Fatalf("got %+v", out)
	}
}

func TestDecodeRejects(t *testing.T) {
	if _, ok := Decode(nil); ok {
		t.Fatal("nil")
	}
	rec := Encode(Sample{})
	rec[0] = '2'
	if _, ok := Decode(rec); ok {
		t.Fatal("bad marker")
	}
}

func TestHistoryRetention(t *testing.T) {
	var h History
	now := time.Now()
	rec := Encode(Sample{CPU: 10})
	// three hours of 5s records: only the last two hours are kept
	for i := 0; i < 3*720; i++ {
		rec[2] = byte(i % 200)
		h.Add(now.Add(-3*time.Hour+time.Duration(i)*5*time.Second), rec)
	}
	pts := h.Points(now)
	if len(pts) < 1400 || len(pts) > 1500 {
		t.Fatalf("kept %d points", len(pts))
	}
	for i := 1; i < len(pts); i++ {
		if !pts[i].At.After(pts[i-1].At) {
			t.Fatal("not oldest first")
		}
	}
	if now.Sub(pts[0].At) > Retention {
		t.Fatal("kept a record older than the retention")
	}
	last, ok := h.Last()
	if !ok || !last.At.Equal(pts[len(pts)-1].At) {
		t.Fatal("last differs")
	}
	h.Add(now, []byte{1, 2, 3}) // wrong length dropped
	if l, _ := h.Last(); !l.At.Equal(last.At) {
		t.Fatal("bad record stored")
	}
}

func write(t *testing.T, path, v string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(v), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectHwmon(t *testing.T) {
	d := t.TempDir()
	// card0: power node and a cap; card1: energy counter only and a max;
	// card0-DP-1 is a connector, card2 has no power node
	write(t, filepath.Join(d, "card0/device/hwmon/hwmon3/power1_average"), "85500000\n")
	write(t, filepath.Join(d, "card0/device/hwmon/hwmon3/power1_cap"), "150000000\n")
	e := filepath.Join(d, "card1/device/hwmon/hwmon4/energy1_input")
	write(t, e, "1000000\n")
	write(t, filepath.Join(d, "card1/device/hwmon/hwmon4/power1_max"), "0\n")
	write(t, filepath.Join(d, "card0-DP-1/device/hwmon/hwmon9/power1_input"), "1\n")
	write(t, filepath.Join(d, "card2/device/hwmon/hwmon5/temp1_input"), "1\n")
	c := newCollector(d)
	if len(c.gpus) != 2 {
		t.Fatalf("found %d gpus", len(c.gpus))
	}
	t0 := time.Now()
	g0 := c.gpus[0].read(t0)
	if g0.Power != 85.5 || g0.Limit != 150 {
		t.Fatalf("card0 %+v", g0)
	}
	c.gpus[1].read(t0) // primes the energy counter
	write(t, e, "21000000\n")
	g1 := c.gpus[1].read(t0.Add(2 * time.Second))
	if g1.Power != 10 || g1.Limit != 0 {
		t.Fatalf("card1 %+v", g1)
	}
}

// With both a power node and an energy counter, the energy baseline has to
// follow the counter while the power node reads, or a later zero reading is
// an average over the whole time since the baseline.
func TestEnergyBaselineFollowsWhilePowerReads(t *testing.T) {
	d := t.TempDir()
	h := filepath.Join(d, "card0/device/hwmon/hwmon1")
	write(t, filepath.Join(h, "power1_input"), "50000000\n")
	write(t, filepath.Join(h, "energy1_input"), "0\n")
	g := newCollector(d).gpus[0]
	t0 := time.Now()
	g.read(t0)
	write(t, filepath.Join(h, "energy1_input"), "100000000\n") // 50W for 2s
	g.read(t0.Add(2 * time.Second))
	write(t, filepath.Join(h, "power1_input"), "0\n")
	write(t, filepath.Join(h, "energy1_input"), "120000000\n") // 10W for the last 2s
	r := g.read(t0.Add(4 * time.Second))
	if r.Power != 10 {
		t.Fatalf("power %v, want 10 over the last 2s", r.Power)
	}
}

func TestParseNvidiaLine(t *testing.T) {
	idx, g, ok := parseNvidiaLine("1, 4.95, 180.00")
	if !ok || idx != 1 || g.Power != 4.95 || g.Limit != 180 {
		t.Fatalf("got %d %+v %v", idx, g, ok)
	}
	_, g, ok = parseNvidiaLine("0, [N/A], [N/A]")
	if !ok || g.Power != 0 || g.Limit != 0 {
		t.Fatalf("n/a gave %+v %v", g, ok)
	}
	for _, bad := range []string{"", "NVIDIA-SMI has failed", "a, 1, 2", "1, 2"} {
		if _, _, ok := parseNvidiaLine(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
}

// The stream keeps the latest line of every card, in index order, and a
// command that never prints is given up on rather than restarted for ever.
func TestNvidiaStream(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := startNvidia(ctx, sh, "-c", "printf '1, 30.5, 200.00\\n0, 4.95, 180.00\\n0, 6.00, 180.00\\n'; sleep 30")
	var got []GPU
	for i := 0; i < 100; i++ {
		if got = n.read(time.Now()); len(got) == 2 && got[0].Power == 6 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(got) != 2 || got[0].Power != 6 || got[0].Limit != 180 || got[1].Power != 30.5 {
		t.Fatalf("got %+v", got)
	}
	if got = n.read(time.Now().Add(time.Minute)); got[0].Power != 0 || got[0].Limit != 180 {
		t.Fatalf("stale reading kept its power: %+v", got)
	}
}

func TestNvidiaCardsLeftToNvidiaSmi(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "card0/device/vendor"), "0x10de\n")
	write(t, filepath.Join(d, "card0/device/hwmon/hwmon1/power1_input"), "1\n")
	write(t, filepath.Join(d, "card1/device/vendor"), "0x8086\n")
	write(t, filepath.Join(d, "card1/device/hwmon/hwmon2/power1_input"), "1\n")
	if c := newCollector(d); len(c.gpus) != 1 {
		t.Fatalf("found %d, want only the Intel card", len(c.gpus))
	}
}
