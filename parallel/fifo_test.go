package parallel

import "testing"

// fifo is the one piece of parallel small enough, and enough like a classic
// place for an off-by-one, to test from the inside.
func TestFifo(t *testing.T) {
	var q fifo[*int]
	next, want := 0, 0
	push := func() { v := next; next++; q.push(&v) }
	pop := func() {
		t.Helper()
		if got := *q.peek(); got != want {
			t.Fatalf("peek = %d, want %d", got, want)
		}
		if got := *q.pop(); got != want {
			t.Fatalf("pop = %d, want %d", got, want)
		}
		want++
	}
	// A steady state of a few outstanding, for far longer than any
	// initial capacity: order holds, and the storage does not grow with
	// the number ever pushed.
	for range 3 {
		push()
	}
	for i := range 100_000 {
		push()
		pop()
		if i%7 == 0 { // vary the depth
			push()
			pop()
			pop()
			push()
		}
		if q.len() != next-want {
			t.Fatalf("len = %d, want %d", q.len(), next-want)
		}
	}
	if cap(q.items) > 64 {
		t.Fatalf("storage grew to %d for a queue never more than a few deep", cap(q.items))
	}
	// Nothing popped is still referenced from the backing array.
	for i, p := range q.items[:cap(q.items)][len(q.items):] {
		if p != nil {
			t.Fatalf("dead slot %d still holds an element", i)
		}
	}
	for q.len() > 0 {
		pop()
	}
	if q.head != 0 || len(q.items) != 0 {
		t.Fatalf("empty queue did not reset: head=%d len=%d", q.head, len(q.items))
	}
}
