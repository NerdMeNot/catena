---
title: "Searching and testing"
description: "Asking a question of a sequence rather than transforming it. The short-circuiting ones — First, Find, Any, All, None — stop as soon as the answer is settled, so they terminate on an infinite source; Last, Count and FindLast cannot, because the answer depends on the final element."
sidebar:
  order: 9
---

Asking a question of a sequence rather than transforming it. The short-circuiting ones — First, Find, Any, All, None — stop as soon as the answer is settled, so they terminate on an infinite source; Last, Count and FindLast cannot, because the answer depends on the final element.

## First

```go
func (s Seq[T]) First() (T, bool)
```

First returns the first element.

```go
// The bool tells an empty sequence apart from a first element that
// happens to be the zero value.
queue := catena.Of("deploy api", "rotate keys")
job, ok := queue.First()
_, idle := catena.Empty[string]().First()
fmt.Println(job, ok, idle)
```

```
deploy api true false
```

## FirstOr

```go
func (s Seq[T]) FirstOr(fallback T) T
```

FirstOr returns the first element, or fallback if the sequence is empty — First for use inside an expression.

```go
// First, for use inside an expression: no second result to check.
ports := catena.Of(8080, 8081)
var none catena.Seq[int]
fmt.Println(ports.FirstOr(80), none.FirstOr(80))
```

```
8080 80
```

## Last

```go
func (s Seq[T]) Last() (T, bool)
```

Last returns the final element.

:::caution
Full drain
:::

```go
// The most recent of a series of health checks.
checks := catena.Of(200, 200, 503)
latest, ok := checks.Last()
fmt.Println(latest, ok)
```

```
503 true
```

## LastOr

```go
func (s Seq[T]) LastOr(fallback T) T
```

LastOr returns the final element, or fallback if the sequence is empty.

:::caution
Full drain
:::

```go
versions := catena.Of("v1", "v2", "v3")
fmt.Println(versions.LastOr("v0"), catena.Empty[string]().LastOr("v0"))
```

```
v3 v0
```

## Single

```go
func (s Seq[T]) Single() (T, bool)
```

Single returns the element iff the sequence has exactly one; it stops consuming upon seeing a second.

```go
// True only for exactly one element — the check for "there must be
// one primary". It stops as soon as a second arrives rather than
// counting the rest.
type node struct{ Name, Role string }
isPrimary := func(n node) bool { return n.Role == "primary" }

healthy := catena.Of(node{"db-1", "primary"}, node{"db-2", "replica"})
splitBrain := catena.Of(node{"db-1", "primary"}, node{"db-2", "primary"})

primary, ok := healthy.Filter(isPrimary).Single()
_, unambiguous := splitBrain.Filter(isPrimary).Single()
fmt.Println(primary.Name, ok, unambiguous)
```

```
db-1 true false
```

## ElementAt

```go
func (s Seq[T]) ElementAt(i int) (T, bool)
```

ElementAt returns the element at index i; (zero, false) for a negative or out-of-range index.

```go
// Zero-based. An index out of range, or negative, reports false
// rather than panicking — there is no counting from the end.
leaderboard := catena.Of("mira", "tom", "ada")
runnerUp, ok := leaderboard.ElementAt(1)
_, fromEnd := leaderboard.ElementAt(-1)
fmt.Println(runnerUp, ok, fromEnd)
```

```
tom true false
```

## Find

```go
func (s Seq[T]) Find(pred func(T) bool) (T, bool)
```

Find returns the first element pred admits.

```go
// The first request that broke the 500ms budget; nothing after it is
// examined.
type request struct {
	Path string
	Ms   int
}
requests := catena.Of(request{"/home", 80}, request{"/search", 740}, request{"/export", 2100})
slow, ok := requests.Find(func(r request) bool { return r.Ms > 500 })
fmt.Println(slow.Path, ok)
```

```
/search true
```

## FindLast

```go
func (s Seq[T]) FindLast(pred func(T) bool) (T, bool)
```

FindLast returns the final element pred admits.

:::caution
Full drain
:::

```go
// The most recent release that passed — the one to roll back to.
type deploy struct {
	Version string
	Healthy bool
}
history := catena.Of(deploy{"v41", true}, deploy{"v42", true}, deploy{"v43", false})
good, ok := history.FindLast(func(d deploy) bool { return d.Healthy })
fmt.Println(good.Version, ok)
```

```
v42 true
```

## FindIndex

```go
func (s Seq[T]) FindIndex(pred func(T) bool) int
```

FindIndex returns the index of the first element pred admits; -1 if none.

```go
// Where the first match is, counted from zero; -1 when there is none.
log := catena.Of("INFO boot", "INFO listening on :8080", "ERROR disk full")
hasLevel := func(level string) func(string) bool {
	return func(line string) bool { return strings.HasPrefix(line, level) }
}
fmt.Println(log.FindIndex(hasLevel("ERROR")))
fmt.Println(log.FindIndex(hasLevel("FATAL")))
```

```
2
-1
```

## FindMap

```go
func (s Seq[T]) FindMap[U any](f func(T) (U, bool)) (U, bool)
```

FindMap returns the first mapped value f reports true for — a fused Find + Map.

```go
// Fused find and map: the parsed value is returned, not the header
// line it came from, and the lines after it are never parsed.
headers := catena.Of("Host: example.com", "Content-Length: 348", "Accept: */*")
length, ok := headers.FindMap(func(h string) (int, bool) {
	value, found := strings.CutPrefix(h, "Content-Length: ")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	return n, err == nil
})
fmt.Println(length, ok)
```

```
348 true
```

## Any

```go
func (s Seq[T]) Any(pred func(T) bool) bool
```

Any reports whether pred admits any element; stops at the first match.

```go
// Stops at the first match, so it terminates even on an endless
// source. Here the two lines after the error are never read.
read := 0
log := catena.Of("INFO boot", "ERROR disk full", "INFO retrying", "INFO ok").
	OnEach(func(string) { read++ })

failed := log.Any(func(line string) bool { return strings.HasPrefix(line, "ERROR") })
fmt.Println(failed, read)
```

```
true 2
```

## All

```go
func (s Seq[T]) All(pred func(T) bool) bool
```

All reports whether pred admits every element; stops at the first counterexample. Vacuously true on empty input.

```go
// Vacuously true on an empty sequence: a pipeline with no checks
// "passes", so guard for that separately if it matters.
type check struct {
	Name   string
	Passed bool
}
passed := func(c check) bool { return c.Passed }

ci := catena.Of(check{"lint", true}, check{"unit", true}, check{"e2e", true})
fmt.Println(ci.All(passed))
fmt.Println(catena.Empty[check]().All(passed))
```

```
true
true
```

## None

```go
func (s Seq[T]) None(pred func(T) bool) bool
```

None reports whether pred admits no element; stops at the first match.

```go
// "Did anything go wrong?" asked the other way round; like Any, it
// stops at the first match.
statuses := catena.Of(200, 204, 301, 404)
fmt.Println(statuses.None(func(code int) bool { return code >= 500 }))
```

```
true
```

## Count

```go
func (s Seq[T]) Count() int
```

Count returns the number of elements.

:::caution
Full drain
:::

```go
// Counts by consuming: nothing is collected along the way.
body := "the quick brown fox jumps over the lazy dog"
fmt.Println(catena.FromSlice(strings.Fields(body)).Count())
```

```
9
```

## CountWhere

```go
func (s Seq[T]) CountWhere(pred func(T) bool) int
```

CountWhere returns the number of elements pred admits.

:::caution
Full drain
:::

```go
// Fused filter and count: one stage rather than two.
statuses := catena.Of(200, 503, 200, 404, 500, 200)
fmt.Println(statuses.CountWhere(func(code int) bool { return code >= 500 }))
```

```
2
```

## IsEmpty

```go
func (s Seq[T]) IsEmpty() bool
```

IsEmpty reports whether the sequence yields nothing.

:::caution
It does so by consuming one element — on a single-pass source that element is lost
:::

```go
// Answers by consuming one element — on a single-pass source that
// element is gone.
statuses := catena.Of(200, 204, 301)
serverErrors := statuses.Filter(func(code int) bool { return code >= 500 })
fmt.Println(serverErrors.IsEmpty(), statuses.IsEmpty())
```

```
true false
```

## Contains

```go
func Contains[T comparable](s Seq[T], v T) bool
```

Contains reports whether v occurs in s; stops at the first match.

```go
// Membership by ==, stopping at the first hit.
roles := catena.Of("viewer", "editor", "admin")
fmt.Println(catena.Contains(roles, "admin"), catena.Contains(roles, "owner"))
```

```
true false
```

## IndexOf

```go
func IndexOf[T comparable](s Seq[T], v T) int
```

IndexOf returns the index of the first occurrence of v; -1 if none.

```go
// The position of the first equal element, or -1.
stages := catena.Of("build", "test", "deploy")
fmt.Println(catena.IndexOf(stages, "deploy"))
fmt.Println(catena.IndexOf(stages, "rollback"))
```

```
2
-1
```

## Equal

```go
func Equal[T comparable](a, b Seq[T]) bool
```

Equal reports whether a and b yield the same elements in the same order. Consumes both sequences up to and including the first difference — fully when they are equal. b is consumed through iter.Pull (its cleanup always runs).

```go
// Same elements in the same order. A sequence that merely starts
// the same way is not equal.
expected := catena.Of("001_users", "002_orders", "003_indexes")
applied := catena.Of("001_users", "002_orders", "003_indexes")
behind := catena.Of("001_users", "002_orders")
fmt.Println(catena.Equal(expected, applied))
fmt.Println(catena.Equal(expected, behind))
```

```
true
false
```
