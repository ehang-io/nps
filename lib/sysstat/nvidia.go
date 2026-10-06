package sysstat

import (
	"bufio"
	"context"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// nvidiaStale is how long a reading is believed. nvidia-smi prints every two
// seconds; when it stalls, a driver hang is the likelier cause, and a power
// that was true a minute ago is not worth reporting.
const nvidiaStale = 15 * time.Second

// nvidiaStream follows one long-running nvidia-smi instead of starting one per
// sample: each start costs a fork and an exec, tens of milliseconds of CPU on
// every npc. NVIDIA's proprietary driver puts no power in hwmon, so this is
// the way to read it without linking NVML. When npc goes away the pipe closes
// and nvidia-smi ends at its next print.
type nvidiaStream struct {
	mu   sync.Mutex
	gpus map[int]GPU
	at   time.Time
}

// startNvidia starts following the command, which has to print
// "index, power, limit" lines with no header (see nvidiaArgs). It restarts the
// command when it ends, and gives up on one that never printed anything.
func startNvidia(ctx context.Context, name string, args ...string) *nvidiaStream {
	n := &nvidiaStream{gpus: map[int]GPU{}}
	go n.run(ctx, name, args)
	return n
}

var nvidiaArgs = []string{
	"--query-gpu=index,power.draw,power.limit",
	"--format=csv,noheader,nounits",
	"-l", "2",
}

func (n *nvidiaStream) run(ctx context.Context, name string, args []string) {
	backoff := 10 * time.Second
	failures := 0
	for ctx.Err() == nil {
		lines := n.follow(ctx, name, args)
		if lines == 0 {
			// no driver, or no card: nothing to wait for
			if failures++; failures >= 3 {
				return
			}
		} else {
			failures, backoff = 0, 10*time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Minute {
			backoff *= 2
		}
	}
}

// follow runs the command once and returns how many lines it printed.
func (n *nvidiaStream) follow(ctx context.Context, name string, args []string) (lines int) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return 0
	}
	defer cmd.Wait()
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if idx, g, ok := parseNvidiaLine(sc.Text()); ok {
			n.mu.Lock()
			n.gpus[idx] = g
			n.at = time.Now()
			n.mu.Unlock()
			lines++
		}
	}
	return
}

// parseNvidiaLine reads "0, 4.95, 180.00". A card that cannot report a value
// prints [N/A] for it, which is left zero.
func parseNvidiaLine(line string) (idx int, g GPU, ok bool) {
	f := strings.Split(line, ",")
	if len(f) != 3 {
		return
	}
	idx, err := strconv.Atoi(strings.TrimSpace(f[0]))
	if err != nil {
		return
	}
	g.Power, _ = strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
	g.Limit, _ = strconv.ParseFloat(strings.TrimSpace(f[2]), 64)
	return idx, g, true
}

// read returns the latest of every card seen, in index order. Readings gone
// stale come back as zero power.
func (n *nvidiaStream) read(now time.Time) []GPU {
	n.mu.Lock()
	defer n.mu.Unlock()
	idx := make([]int, 0, len(n.gpus))
	for i := range n.gpus {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	stale := now.Sub(n.at) > nvidiaStale
	out := make([]GPU, 0, len(idx))
	for _, i := range idx {
		g := n.gpus[i]
		if stale {
			g.Power = 0
		}
		out = append(out, g)
	}
	return out
}
