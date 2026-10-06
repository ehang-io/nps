package nps_mux

import (
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// A consumer that reaches a slot it cannot read must say so, and must say it
// in a way its caller can tell apart from "this dequeue is finished". The
// bound exists so it stops burning a core; reporting the wrong thing is how
// that fix turns into data loss (see the chain test below).
func TestPopTailReportsStalledNotEmpty(t *testing.T) {
	d := new(bufDequeue)
	d.vals = make([]unsafe.Pointer, 8)
	atomic.StoreUint64(&d.headTail, d.pack(4, 0)) // head != tail, every slot nil

	type outcome struct {
		val unsafe.Pointer
		res popResult
	}
	done := make(chan outcome, 1)
	go func() {
		v, r := d.popTail()
		done <- outcome{v, r}
	}()

	select {
	case got := <-done:
		if got.res == popOK {
			t.Fatal("popTail returned a value from a dequeue holding none")
		}
		if got.res != popStalled {
			t.Fatalf("popTail reported %v, want popStalled: reporting popEmpty lets the chain drop a dequeue that still holds data", got.res)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("popTail spun for 3s on a stalled dequeue: the busy loop is back")
	}
}

// The chain may only splice out a dequeue that is permanently empty. A slot
// still being written is not that, and dropping the dequeue there strands
// every element behind it. This reproduces the interleaving directly: a full
// dequeue, a successor linked after it, and one slot whose producer has
// claimed an index but not yet stored to it.
func TestChainKeepsDequeueWithAnUnwrittenSlot(t *testing.T) {
	v1, v2, v3 := new(int), new(int), new(int)

	c := new(bufChain)
	c.new(2)
	c.pushHead(unsafe.Pointer(v1))
	c.pushHead(unsafe.Pointer(v2)) // the first dequeue is now full
	c.pushHead(unsafe.Pointer(v3)) // overflows into a second, linked dequeue

	first := loadPoolChainElt(&c.tail)
	if loadPoolChainElt(&first.next) == nil {
		t.Fatal("test setup: expected a second dequeue to be linked")
	}

	// Put the tail slot back into the state a producer leaves it in between
	// claiming the index and storing the value.
	atomic.StorePointer(&first.vals[0], nil)

	if _, ok := c.popTail(); ok {
		t.Fatal("popTail returned a value out of an unwritten slot")
	}
	if loadPoolChainElt(&c.tail) != first {
		t.Fatal("the chain dropped a dequeue whose slot was merely mid-write: everything still queued in it is now unreachable")
	}

	// The producer completes its store; nothing may have been lost.
	atomic.StorePointer(&first.vals[0], unsafe.Pointer(v1))
	for i, want := range []*int{v1, v2, v3} {
		got, ok := c.popTail()
		if !ok {
			t.Fatalf("pop %d: the queue reported empty, but the element was pushed", i)
		}
		if got != unsafe.Pointer(want) {
			t.Fatalf("pop %d: got a different element than was pushed", i)
		}
	}
}

// When the length counter claims data the chain will not produce, Pop must
// give up and let the caller close the stream. What it must never do is
// adjust the counter: discounting an element that is still in the chain
// underflows the counter when that element is finally popped, which clamps
// the receive window to zero for good and blocks the peer forever.
func TestReceiveWindowQueuePopGivesUpWithoutTouchingTheCounter(t *testing.T) {
	defer func(p, o time.Duration) { stallPoll, stallTimeout = p, o }(stallPoll, stallTimeout)
	stallPoll, stallTimeout = time.Millisecond, 100*time.Millisecond

	q := newReceiveWindowQueue()
	const claimed = 1024
	atomic.StoreUint64(&q.lengthWait, q.chain.head.pack(claimed, 0))

	done := make(chan error, 1)
	go func() {
		_, err := q.Pop()
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected Pop to give up on a queue that never produces")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Pop never returned: it is spinning or blocked forever")
	}

	if n := q.Len(); n != claimed {
		t.Fatalf("counter is %d, want %d left untouched: rewriting it here underflows on the next pop and wedges the window", n, claimed)
	}
}

// The ordinary path must keep working, and the counter must come back to zero
// rather than wrapping.
func TestReceiveWindowQueueRoundTripStillWorks(t *testing.T) {
	q := newReceiveWindowQueue()
	for i := 0; i < 100; i++ {
		ele, err := newListElement(make([]byte, 4), 4, false)
		if err != nil {
			t.Fatal(err)
		}
		q.Push(ele)
	}
	for i := 0; i < 100; i++ {
		ele, err := q.Pop()
		if err != nil {
			t.Fatalf("pop %d: %v", i, err)
		}
		if ele.L != 4 {
			t.Fatalf("pop %d: length %d, want 4", i, ele.L)
		}
	}
	if n := q.Len(); n != 0 {
		t.Fatalf("queue length %d after draining, want 0", n)
	}
}
