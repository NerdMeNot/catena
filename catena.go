// Package catena is a lazy, fully-typed sequence library for Go 1.27+,
// built on iter.Seq and generic methods.
//
// The four types:
//
//   - Seq[T]:  lazy, single-pass by contract, possibly infinite.
//   - Seq2[K, V]: the stdlib pairing currency; a bridge back to Seq, not a peer.
//   - Try[T]:  Seq2[T, error]; error policy is chosen by the consumer
//     (Collect stops at the first error, CollectAll drains, Ignore skips).
//   - List[T]: eager []T with the mirrored operation set.
//
// Finding an operator: most are methods, but operations that constrain the
// element type (Distinct, Sorted, Sum, Max, Contains, Union, …) are package
// functions, because a method on Seq[T any] may require nothing of T. So are
// Chunked, ChunkedBy and Windowed, which return Seq[[]T] — a method doing
// that is an instantiation cycle — and the Flatten family, which constrains
// the receiver's shape. Spelling is catena.Distinct(s), not s.Distinct(); the
// chain continues normally afterwards, as catena.Distinct(s).Filter(f). If the
// compiler reports "has no field or method Distinct", this is why.
//
// Contracts every caller should know:
//
//   - Treat every Seq as single-pass. Re-iterability depends entirely on the
//     producer (see the table on Seq); operators preserve it either way.
//     Once() is a development guard.
//   - Nil sequences are empty: every method and package function accepts a
//     nil receiver or argument and treats it as an empty sequence.
//   - Invalid construction arguments (negative counts, zero step) panic at
//     construction time with a "catena: " prefixed message. Nil callbacks
//     are not defended and panic at first use.
//   - Operators marked as buffering state their bound; terminals marked as
//     full-drain hang on infinite input.
//   - Map-returning terminals return Go maps: iteration order is undefined.
//   - When a Try element carries a non-nil error, do not read the value.
//   - Not safe for concurrent use; ToChan is the fan-out mechanism.
//   - comparable-constrained functions panic at runtime if T is an
//     interface type holding a non-comparable value (Go 1.20 semantics).
package catena

//go:generate go run ./internal/gen/assets .
//go:generate go run ./internal/gen/opdocs .

import "iter"

// Seq is a lazy sequence: iter.Seq with methods. Range over it directly.
//
// A Seq is a function, so consuming it twice runs its producer twice.
// Whether that works is a property of the producer alone — operators never
// change it. Every operator builds its state inside the returned closure, so
// a chain is re-iterable exactly when its root producer is, and the
// conformance suite re-iterates every operator to prove it. Once() is the
// single deliberate exception.
//
//	Re-iterable   Of, FromSlice, FromMap, Empty, Empty2, EmptyTry, Once1,
//	              Repeat, RepeatN, Range, Generate and GenerateWhile (iff
//	              next is pure)
//	Single-use    FromChan, and From, From2, FromErrs when the underlying
//	              source is (anything over I/O)
//	Cycle         re-iterable iff its source's first pass is
//
// Consuming a single-use sequence a second time yields nothing rather than
// failing: wrap it in Once() during development to turn that into a panic.
// Printing a Seq shows a function pointer — print s.Collect() instead.
type Seq[T any] iter.Seq[T]

// Seq2 is a lazy pair sequence: iter.Seq2 with methods. It is a bridge back
// to Seq (via Keys, Values, MapTo), deliberately not a full peer surface.
//
// Deliberately absent, so nobody goes looking:
//
//   - ToMap: needs K comparable on the receiver. Use catena.CollectMap.
//   - Collect: catena.Unzip is the one way to two slices.
//   - MapKeys: Swap().MapValues(f).Swap() for the rare need; Map otherwise.
type Seq2[K, V any] iter.Seq2[K, V]

// Try is a lazy sequence of fallible elements: iter.Seq2[T, error] with
// methods. When err != nil the value must not be read.
//
// Try carries a small operator set on purpose — it is not a second Seq. The
// intended shape is to stay in Try only while errors are still in play, then
// commit to a policy (Ignore, Must, Collect, CollectAll) and continue on Seq
// with the full API. Wanting Sorted or GroupBy on a Try means the errors have
// been carried one stage too far.
//
// Its operators follow five uniform rules, referred to below as R1–R5:
//
//	R1  Intermediates never inspect errored elements: predicates and map
//	    functions are not called on them; the element flows through.
//	R2  The positional intermediates (Take, Drop) count elements, errored
//	    or not, so Take(n) consumes at most n of the source.
//	    Ignore().Take(n) is the "n successes" spelling. Count is a
//	    terminal and follows R5 instead.
//	R3  An errored element passes through TakeWhile without terminating
//	    the sequence; only a successful element failing pred ends it.
//	R4  Operators that generate an error yield (zero, err).
//	R5  Single-error terminals (Collect, Fold, ForEach, Err, Count) stop
//	    consuming at the first error.
type Try[T any] iter.Seq2[T, error]

// List is an eager []T with the mirrored method set. []T(l) unwraps it with
// no copy, and AsSeq() views it lazily for free.
//
// The mirror covers Seq's methods, not the constraint-bound package
// functions: those take a Seq, so reach them through AsSeq and come back with
// ToList, as catena.Sorted(l.AsSeq()).ToList(). Concat likewise takes a Seq,
// so it is l1.Concat(l2.AsSeq()).
//
// Four mirrored operators cross back to lazy, because their Seq counterparts
// return a lazy type: WithIndex and ZipWithNext return Seq2, MapErr and
// FilterErr return Try.
type List[T any] []T

// Numeric is the constraint for arithmetic aggregations (Sum, Product,
// Average). Complex types are deliberately excluded: they are unordered and
// half the aggregation surface would be meaningless for them.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// Integer is the constraint for Range.
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Self is the identity selector: catena.Flatten(s) is s.FlatMap(Self).
func Self[T any](v T) T { return v }

// emptyFn is the shared empty iteration body; src returns it for nil
// sequences so every operator is nil-safe without per-operator guards.
func emptyFn[T any](yield func(T) bool) { _ = yield }

func emptyFn2[K, V any](yield func(K, V) bool) { _ = yield }

// src adapts a possibly-nil Seq to a ranged iter.Seq: nil is empty.
func src[T any](s Seq[T]) iter.Seq[T] {
	if s == nil {
		return emptyFn[T]
	}
	return iter.Seq[T](s)
}

// src2 adapts a possibly-nil Seq2 to a ranged iter.Seq2.
func src2[K, V any](s Seq2[K, V]) iter.Seq2[K, V] {
	if s == nil {
		return emptyFn2[K, V]
	}
	return iter.Seq2[K, V](s)
}

// srcTry adapts a possibly-nil Try to a ranged iter.Seq2.
func srcTry[T any](t Try[T]) iter.Seq2[T, error] {
	if t == nil {
		return emptyFn2[T, error]
	}
	return iter.Seq2[T, error](t)
}

// negCheck panics with the construction-time message form for negative
// count arguments.
func negCheck(op string, n int) {
	if n < 0 {
		panic("catena: " + op + ": negative count")
	}
}

// posCheck panics for structural size arguments that must be positive.
func posCheck(op, arg string, n int) {
	if n <= 0 {
		panic("catena: " + op + ": " + arg + " must be > 0")
	}
}
