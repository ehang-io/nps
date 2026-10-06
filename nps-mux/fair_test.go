package nps_mux

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"testing"
	"time"
)

// slowConn paces writes to a fixed rate, standing in for a lossy uplink that
// drains far slower than the streams feeding it.
type slowConn struct {
	net.Conn
	bytesPerSec int
}

func (c *slowConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	time.Sleep(time.Duration(n) * time.Second / time.Duration(c.bytesPerSec))
	return n, err
}

// newMuxPair joins two muxes over an in-memory pipe whose client-to-server
// direction is paced at bytesPerSec.
func newMuxPair(t *testing.T, bytesPerSec int) (client *Mux, accepted <-chan net.Conn) {
	// Closing a mux trips the race detector on flags upstream never
	// synchronised (Mux.IsClose, window.closeOp, conn.isClose), none of them
	// part of the write queue under test. CI runs these without -race.
	if raceEnabled {
		t.Skip("closing a mux races on upstream's unsynchronised close flags")
	}
	a, b := net.Pipe()
	client = NewMux(&slowConn{Conn: a, bytesPerSec: bytesPerSec}, "tcp", 60)
	server := NewMux(b, "tcp", 60)
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	ch := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := server.Accept()
			if err != nil {
				return
			}
			ch <- c
		}
	}()
	return client, ch
}

func dial(t *testing.T, client *Mux, accepted <-chan net.Conn) (local, remote net.Conn) {
	local, err := client.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case remote = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("the server never accepted the stream")
	}
	return local, remote
}

// A few bytes on one stream must not wait for everything another stream has
// already handed the mux. This is an interactive session sharing a tunnel with
// a bulk transfer: before per-stream scheduling the keystroke queued behind
// the bulk stream's whole send window.
func TestSmallStreamIsNotQueuedBehindBulk(t *testing.T) {
	const rate = 128 * 1024
	client, accepted := newMuxPair(t, rate)

	small, smallPeer := dial(t, client, accepted)
	bulk, bulkPeer := dial(t, client, accepted)
	go func() { _, _ = io.Copy(ioutil.Discard, bulkPeer) }()
	go func() {
		chunk := make([]byte, 32*1024)
		for {
			if _, err := bulk.Write(chunk); err != nil {
				return
			}
		}
	}()
	// Let the bulk stream queue as much as the mux will take from it.
	time.Sleep(2 * time.Second)

	start := time.Now()
	if _, err := small.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(smallPeer, buf)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the small write never arrived")
	}
	// One scheduling quantum plus one write batch is 32 KB, a quarter of a
	// second at this rate. A FIFO queue holds the bulk stream's window, which
	// is several times that.
	elapsed := time.Since(start)
	if elapsed > 750*time.Millisecond {
		t.Fatalf("5 bytes took %v to cross a %d KB/s link behind a bulk stream", elapsed, rate/1024)
	}
	t.Logf("5 bytes crossed behind a bulk stream in %v", elapsed)
	if string(buf) != "hello" {
		t.Fatalf("got %q", buf)
	}
}

// Closing a stream right after writing must still deliver every byte: the
// close frame travels in the stream's own queue, behind its data, and may not
// be scheduled ahead of it.
func TestCloseAfterWriteDeliversEverything(t *testing.T) {
	client, accepted := newMuxPair(t, 4*1024*1024)
	local, remote := dial(t, client, accepted)

	want := bytes.Repeat([]byte("0123456789abcdef"), 64*1024) // 1 MB
	go func() {
		_, _ = local.Write(want)
		_ = local.Close()
	}()

	got := make(chan []byte, 1)
	go func() {
		b, _ := ioutil.ReadAll(remote)
		got <- b
	}()
	select {
	case b := <-got:
		if !bytes.Equal(b, want) {
			t.Fatalf("received %d bytes, want %d intact", len(b), len(want))
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the stream never reached EOF")
	}
}

func queuedFrame(t *testing.T, flag uint8, id int32, size int) *muxPackager {
	p := muxPack.Get()
	var data interface{}
	switch flag {
	case muxNewMsg, muxNewMsgPart, muxPingFlag, muxPingReturn:
		data = make([]byte, size)
	case muxMsgSendOk:
		data = uint64(0)
	}
	if err := p.Set(flag, id, data); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSendQueueOrder(t *testing.T) {
	var q sendQueue
	q.New()
	// A bulk stream with far more than a quantum queued, then its close.
	for i := 0; i < 40; i++ {
		q.Push(queuedFrame(t, muxNewMsgPart, 1, maximumSegmentSize))
	}
	q.Push(queuedFrame(t, muxConnClose, 1, 0))
	// Queued after all of it: a keystroke, an acknowledgement and a ping.
	q.Push(queuedFrame(t, muxNewMsg, 2, 5))
	q.Push(queuedFrame(t, muxMsgSendOk, 3, 0))
	q.Push(queuedFrame(t, muxPingFlag, muxPing, 8))

	var order []*muxPackager
	for p := q.TryPop(); p != nil; p = q.TryPop() {
		order = append(order, p)
	}
	if len(order) != 44 {
		t.Fatalf("popped %d frames, want 44", len(order))
	}
	if order[0].flag != muxPingFlag || order[1].flag != muxMsgSendOk {
		t.Fatalf("ping and control frames must go first, got flags %d, %d", order[0].flag, order[1].flag)
	}
	keystroke := -1
	for i, p := range order {
		if p.id == 2 {
			keystroke = i
		}
	}
	// Two control frames, then at most one quantum of the bulk stream.
	if limit := 2 + streamQuantum/maximumSegmentSize + 1; keystroke > limit {
		t.Fatalf("stream 2 went out at position %d, after more than a quantum of stream 1 (limit %d)", keystroke, limit)
	}
	if last := order[len(order)-1]; last.id != 1 || last.flag != muxConnClose {
		t.Fatalf("stream 1's close must follow all its data, last frame is flag %d of stream %d", last.flag, last.id)
	}
	if len(q.streams) != 0 || len(q.active) != 0 {
		t.Fatalf("drained queue still tracks %d streams, %d active", len(q.streams), len(q.active))
	}
}

func TestSendQueueBackpressure(t *testing.T) {
	var q sendQueue
	q.New()
	var drained <-chan struct{}
	pushed := 0
	for drained == nil {
		drained = q.Push(queuedFrame(t, muxNewMsgPart, 1, maximumSegmentSize))
		pushed += 7 + maximumSegmentSize
	}
	if pushed <= streamQueueLimit || pushed > streamQueueLimit+7+maximumSegmentSize {
		t.Fatalf("writer told to wait after %d bytes, limit is %d", pushed, streamQueueLimit)
	}
	if other := q.Push(queuedFrame(t, muxNewMsg, 2, 5)); other != nil {
		t.Fatal("a full stream must not hold back another stream's writer")
	}
	isClosed := func() bool {
		select {
		case <-drained:
			return true
		default:
			return false
		}
	}
	for q.streams[1] != nil && q.streams[1].bytes > streamQueueLimit/2 {
		if isClosed() {
			t.Fatal("writer released before half the queue was sent")
		}
		q.TryPop()
	}
	if !isClosed() {
		t.Fatal("writer still blocked after its stream drained")
	}

	// Stop releases a writer that is still waiting.
	for drained = nil; drained == nil; {
		drained = q.Push(queuedFrame(t, muxNewMsgPart, 4, maximumSegmentSize))
	}
	q.Stop()
	select {
	case <-drained:
	default:
		t.Fatal("Stop left a writer blocked")
	}
	if q.Pop() == nil {
		t.Fatal("Pop must still hand out frames queued before Stop")
	}
}
