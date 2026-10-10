# Eager

List is a []T carrying the whole Seq *method* set, evaluated at once with exact preallocation. Only the List-only methods are listed here: the mirrored operators behave identically to the Seq ones over AsSeq(), which is not a claim but a conformance check, so their entries live on the pages above. Reach for List when the data is small and in memory, when you touch the result more than once, or when you want O(1) access.

The constraint-bound package functions (`Sorted`, `Sum`, `Max`, `Distinct`, `Contains`, `Union`, `Chunked`, …) are not part of that mirror: they take a `Seq`, so reach them through `AsSeq` and come back with `ToList` — `catena.Sorted(l.AsSeq()).ToList()`. `Concat` likewise takes a `Seq`, so it is `l1.Concat(l2.AsSeq())`. And a few mirrored operators cross back to lazy because their `Seq` counterparts do: `WithIndex`, `ZipWithNext` and `FoldedBy` return `Seq2`, `MapErr` and `FilterErr` return `Try`.

## Len

```go
func (l List[T]) Len() int
```

Len returns the number of elements. O(1).

```go
// A List is a slice, so its length is known without consuming
// anything — unlike Count on a lazy sequence.
replicas := catena.List[string]{"db-1", "db-2", "db-3"}
fmt.Println("quorum:", replicas.Len()/2+1)
```

```
quorum: 2
```

## At

```go
func (l List[T]) At(i int) T
```

At returns the element at index i, panicking exactly like l[i] on an out-of-range index. O(1).

```go
// Panics exactly like l[i] — it IS the index expression. Use it when
// the index is known to be valid.
stages := catena.List[string]{"build", "test", "deploy"}
fmt.Println(stages.At(stages.Len() - 1))
```

```
deploy
```

## Get

```go
func (l List[T]) Get(i int) (T, bool)
```

Get returns the element at index i; (zero, false) for a negative or out-of-range index. O(1).

```go
// The comma-ok form, for when the index may be out of range: an
// optional command-line argument with a default.
args := catena.List[string]{"deploy"}
command, _ := args.Get(0)
target, ok := args.Get(1)
if !ok {
	target = "staging"
}
fmt.Println(command, target)
```

```
deploy staging
```

## Slice

```go
func (l List[T]) Slice(i, j int) List[T]
```

Slice returns l[i:j] as a List, panicking exactly like the slice expression — and, exactly like it, sharing the backing array.

```go
// One page of results. It is the slice expression, aliasing included:
// the page shares memory with the list it came from.
results := catena.List[string]{"ada", "grace", "linus", "ken", "dennis"}
page, size := 1, 2
fmt.Println(results.Slice(page*size, (page+1)*size))
```

```
[linus ken]
```

## Clone

```go
func (l List[T]) Clone() List[T]
```

Clone returns a shallow copy with a fresh backing array.

```go
// Start from shared defaults without changing them for everyone else.
defaults := catena.List[string]{"--color", "--jobs=4"}
mine := defaults.Clone()
mine[1] = "--jobs=16"
fmt.Println(defaults, mine)
```

```
[--color --jobs=4] [--color --jobs=16]
```

## AsSeq

```go
func (l List[T]) AsSeq() Seq[T]
```

AsSeq returns a lazy, re-iterable view of the list. The list is not copied; mutations are visible to later iterations.

```go
// Crossing to lazy is explicit, and free — it is a view, not a copy.
// The search stops at the first match instead of testing every port.
candidates := catena.List[int]{80, 443, 8080, 8443, 9090}
port, ok := candidates.AsSeq().Find(func(p int) bool { return p >= 1024 })
fmt.Println(port, ok)
```

```
8080 true
```

## FoldRight

```go
func (l List[T]) FoldRight[A any](init A, f func(T, A) A) A
```

FoldRight reduces right to left. It exists only on List: a right fold needs the whole sequence in memory, which a List already is.

```go
// Wrapping a handler in middleware: the last one listed has to wrap
// first so the first one listed ends up outermost. Exists only on
// List: a right fold needs the whole sequence, which a List already is.
middleware := catena.List[string]{"logging", "auth", "gzip"}
chain := middleware.FoldRight("handler", func(name, inner string) string {
	return name + "(" + inner + ")"
})
fmt.Println(chain)
```

```
logging(auth(gzip(handler)))
```

## Append

```go
func (l List[T]) Append(vals ...T) List[T]
```

Append returns a new List with vals appended. Unlike the built-in append, the result ALWAYS has a fresh backing array and never aliases l — consistent with every other List transform. Callers who want the built-in's amortized behavior can use it directly: List is a []T.

```go
// Two commands built from one shared prefix. With the builtin append
// and spare capacity, the second would overwrite the first; here the
// result never aliases the receiver.
common := append(make(catena.List[string], 0, 8), "git", "log")
oneline := common.Append("--oneline")
graph := common.Append("--graph")
fmt.Println(oneline, graph)
```

```
[git log --oneline] [git log --graph]
```
