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
// Contracts every caller should know:
//
//   - Treat every Seq as single-pass. Re-iterability depends entirely on the
//     producer (see the constructor table). Once() is a development guard.
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
type Seq[T any] iter.Seq[T]

// Seq2 is a lazy pair sequence: iter.Seq2 with methods. It is a bridge back
// to Seq (via Keys, Values, MapTo), deliberately not a full peer surface.
type Seq2[K, V any] iter.Seq2[K, V]

// Try is a lazy sequence of fallible elements: iter.Seq2[T, error] with
// methods. When err != nil the value must not be read.
type Try[T any] iter.Seq2[T, error]

// List is an eager []T with the mirrored operation set. []T(l) unwraps it
// with no copy.
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

// src adapts a possibly-nil Seq to a ranged iter.Seq (§4.6: nil is empty).
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
// count arguments (§4.10).
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
