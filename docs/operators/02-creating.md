# Creating a sequence

Where a pipeline starts. Some of these are re-iterable and some are single-use — the difference is a property of the producer, not of Seq, and each one says which below. Repeat, Generate and Cycle are infinite, so they need something downstream that stops.

## Of

```go
func Of[T any](vals ...T) Seq[T]
```

Of returns a re-iterable Seq over the given values.

```go
fmt.Println(catena.Of("go", "rust", "zig").Collect())
```

```
[go rust zig]
```

## FromSlice

```go
func FromSlice[T any](s []T) Seq[T]
```

FromSlice returns a re-iterable Seq over s. The slice is not copied; mutations to it are visible to later iterations.

```go
// The slice is not copied: later mutations are visible to later
// iterations, which is what makes this free.
xs := []int{1, 2, 3}
s := catena.FromSlice(xs)
xs[0] = 99
fmt.Println(s.Collect())
```

```
[99 2 3]
```

## From

```go
func From[T any](seq func(func(T) bool)) Seq[T]
```

From adapts any push-function sequence — iter.Seq, catena.Seq, or a third-party alias — with no conversion at the call site. Re-iterability depends on the source.

```go
// Takes the literal function type, so any iterator adapts without a
// conversion at the call site — including the standard library's.
fmt.Println(catena.From(slices.Values([]string{"a", "b"})).Collect())
```

```
[a b]
```

## FromMap

```go
func FromMap[K comparable, V any](m map[K]V) Seq2[K, V]
```

FromMap returns a re-iterable Seq2 over m, in undefined (map) order.

```go
ages := map[string]int{"ada": 36}
fmt.Println(catena.CollectMap(catena.FromMap(ages)))
```

```
map[ada:36]
```

## From2

```go
func From2[K, V any](seq func(func(K, V) bool)) Seq2[K, V]
```

From2 adapts any push-function pair sequence.

```go
pairs := catena.From2(func(yield func(string, int) bool) {
	yield("a", 1)
	yield("b", 2)
})
fmt.Println(pairs.MapTo(func(k string, v int) string {
	return fmt.Sprintf("%s=%d", k, v)
}).Collect())
```

```
[a=1 b=2]
```

## FromErrs

```go
func FromErrs[T any](seq func(func(T, error) bool)) Try[T]
```

FromErrs adapts any push-function fallible sequence.

```go
rows := catena.FromErrs(func(yield func(int, error) bool) {
	yield(1, nil)
	yield(0, fmt.Errorf("row 2: corrupt"))
})
vals, err := rows.Collect()
fmt.Println(vals, err)
```

```
[1] row 2: corrupt
```

## FromChan

```go
func FromChan[T any](ctx context.Context, ch <-chan T) Seq[T]
```

FromChan yields values received from ch until ch is closed or ctx is done. Single-use. No goroutine is started; a sequence that is never consumed never receives.

```go
ch := make(chan int, 3)
for i := 1; i <= 3; i++ {
	ch <- i
}
close(ch)

// Single-use, and it starts no goroutine: a sequence that is never
// consumed never receives.
fmt.Println(catena.FromChan(context.Background(), ch).Collect())
```

```
[1 2 3]
```

## Empty

```go
func Empty[T any]() Seq[T]
```

Empty returns the empty Seq.

```go
fmt.Println(catena.Empty[int]().Collect(), catena.Empty[int]().Count())
```

```
[] 0
```

## Empty2

```go
func Empty2[K, V any]() Seq2[K, V]
```

Empty2 returns the empty Seq2.

```go
fmt.Println(catena.Empty2[string, int]().Count())
```

```
0
```

## EmptyTry

```go
func EmptyTry[T any]() Try[T]
```

EmptyTry returns the empty Try.

```go
vals, err := catena.EmptyTry[int]().Collect()
fmt.Println(vals, err)
```

```
[] <nil>
```

## Once1

```go
func Once1[T any](v T) Seq[T]
```

Once1 returns a re-iterable Seq of exactly one value. (Once, without the suffix, is the single-use guard method on Seq.).

```go
// Once1, not Once: Once is the single-use guard method on Seq.
fmt.Println(catena.Once1("only").Collect())
```

```
[only]
```

## Repeat

```go
func Repeat[T any](v T) Seq[T]
```

Repeat yields v forever. Infinite: pair with Take or a conditional terminal.

```go
// Infinite, so it must be bounded by something downstream.
fmt.Println(catena.Repeat("ha").Take(3).Collect())
```

```
[ha ha ha]
```

## RepeatN

```go
func RepeatN[T any](v T, n int) Seq[T]
```

RepeatN yields v exactly n times. Panics if n is negative.

```go
fmt.Println(catena.RepeatN(0, 4).Collect())
```

```
[0 0 0 0]
```

## Generate

```go
func Generate[T any](seed T, next func(T) T) Seq[T]
```

Generate yields seed, then next(seed), then next(next(seed)), forever. Infinite. Re-iterable iff next is pure.

```go
// The seed is yielded first, then next applied repeatedly. Infinite.
fmt.Println(catena.Generate(1, func(n int) int { return n * 3 }).
	Take(4).
	Collect())
```

```
[1 3 9 27]
```

## GenerateWhile

```go
func GenerateWhile[T any](seed T, next func(T) (T, bool)) Seq[T]
```

GenerateWhile yields seed unconditionally, then successive next values until next reports false.

```go
// The seed is yielded unconditionally; a value produced alongside
// ok=false is not.
fmt.Println(catena.GenerateWhile(1, func(n int) (int, bool) {
	return n * 3, n < 9
}).Collect())
```

```
[1 3 9]
```

## Range

```go
func Range[I Integer](start, stop, step I) Seq[I]
```

Range yields start, start+step, ... while the value is before stop (exclusive). step == 0 panics at construction; a sign mismatch between step and the start→stop direction yields an empty sequence. Termination is overflow-guarded: a step past the type's edge stops rather than wrapping. Unsigned types cannot step downward.

```go
// Half-open, like a slice expression. A sign mismatch between step
// and direction yields nothing rather than panicking, so a computed
// step is safe.
fmt.Println(catena.Range(0, 10, 3).Collect())
fmt.Println(catena.Range(3, 0, -1).Collect())
fmt.Println(catena.Range(0, 10, -1).Collect())
```

```
[0 3 6 9]
[3 2 1]
[]
```

## Cycle

```go
func Cycle[T any](s Seq[T]) Seq[T]
```

Cycle yields s over and over, forever. An empty s yields an empty Cycle — it terminates rather than spinning.

:::caution
The first pass is buffered and replayed (⚠ unbounded memory in len(s))
:::

```go
// Infinite — except over an empty source, which terminates rather
// than spinning.
fmt.Println(catena.Cycle(catena.Of("a", "b")).Take(5).Collect())
fmt.Println(catena.Cycle(catena.Empty[string]()).Collect())
```

```
[a b a b a]
[]
```

## Self

```go
func Self[T any](v T) T
```

Self is the identity selector: catena.Flatten(s) is s.FlatMap(Self).

```go
// The identity selector, for the -By operators when the element is
// already the key.
fmt.Println(catena.Of(3, 1, 2).TallyBy(catena.Self[int])[3])
```

```
1
```
