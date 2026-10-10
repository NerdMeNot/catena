// Package parallel holds catena's concurrency and time operators: the ones that
// start goroutines, which the core package never does.
//
// Every operator here is a stage — a function from a sequence to its
// result — built to sit in a chain through Seq.Then:
//
//	bodies, err := catena.FromSlice(urls).
//		Filter(isHTTPS).
//		Then(parallel.Map(ctx, 8, fetch)).
//		Collect()
//
// Contracts every caller should know:
//
//   - Structured concurrency: every goroutine a stage starts has exited
//     before its iteration returns — on a normal end, an early break, a
//     cancelled context and a panic alike. Nothing is left running, so
//     nothing leaks. The price is that stopping waits: a callback must
//     honour its context, and a blocked source must return, before a
//     break completes.
//   - The consumer's loop body always runs on the consumer's goroutine.
//   - A failed callback is an element, not an abort: it arrives as
//     (zero, err) on a catena.Try, and the consumer picks the policy —
//     Collect to stop at the first, CollectAll to gather, Ignore to skip.
//   - A panic in a callback or a source is re-raised, with its original
//     value, on the consumer's goroutine once everything has been joined.
//   - Stages are re-iterable when their source is: all state belongs to
//     one iteration.
//   - Invalid construction arguments panic at construction time with a
//     "parallel: " prefixed message. Nil sequences are empty.
//
// There are two execution models, and each operator says which it is. The
// caller-driven stages (Map, MapUnordered, ForEach) run the source on the
// consumer's goroutine exactly as the core package does, and only the
// callback concurrently. The producer stages (Buffer, ChunkedWithin,
// Merge) run each source on a goroutine of its own, so everything upstream
// of them runs concurrently with the consumer's loop body.
package parallel

import (
	"errors"
	"time"
)

// posCheck panics for size and duration arguments that must be positive.
func posCheck[N int | time.Duration](op, arg string, n N) {
	if n <= 0 {
		panic("parallel: " + op + ": " + arg + " must be > 0")
	}
}

// errGoexit is what a stage re-raises when a callback or source ended its
// goroutine with runtime.Goexit instead of returning.
var errGoexit = errors.New("parallel: callback called runtime.Goexit")

// protect runs fn, and if fn panics or calls runtime.Goexit stores what to
// re-raise in *crash. It reports through a pointer because a function
// that exits its goroutine never returns a value.
func protect(crash *any, fn func()) {
	returned := false
	defer func() {
		if returned {
			return
		}
		// recover is nil while a Goexit unwinds, and never nil for a
		// panic — panic(nil) arrives as a *runtime.PanicNilError.
		if *crash = recover(); *crash == nil {
			*crash = errGoexit
		}
	}()
	fn()
	returned = true
}
