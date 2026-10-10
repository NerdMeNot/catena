---
title: "Concurrency"
description: "The operators that start goroutines, in the catena/parallel package — the core package never does. Each is a stage that sits in a chain through Seq.Then, and each joins every goroutine it started before its iteration returns: on a normal end, an early break, a cancelled context and a panic alike. Map, MapUnordered and ForEach run a callback concurrently while the source stays on the consumer's goroutine; Buffer, ChunkedWithin and Merge run the source itself on a goroutine of its own. The concurrency guide explains the two models."
sidebar:
  order: 15
---

The operators that start goroutines, in the `catena/parallel` package — the core package never does. Each is a stage that sits in a chain through `Seq.Then`, and each joins every goroutine it started before its iteration returns: on a normal end, an early break, a cancelled context and a panic alike. `Map`, `MapUnordered` and `ForEach` run a callback concurrently while the source stays on the consumer's goroutine; `Buffer`, `ChunkedWithin` and `Merge` run the source itself on a goroutine of its own. The [concurrency guide](/guides/concurrency/) explains the two models.

Every example below is complete: the function being run concurrently is written out, because seeing that it is an ordinary function is most of understanding what this package is for. The ones that make HTTP requests talk to `shopURL`, a small server the tests start, so they are verified against real requests.

## Map

```go
func Map[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U]
```

Map returns a stage that yields f applied to each element, running up to n calls at once and yielding results in source order. A failed call yields (zero, err) and the stage continues. When ctx is done the stage stops consuming, yields (zero, ctx.Err()) once in place of any results it still held, and ends; over an empty source it yields nothing. f receives a context that is also cancelled the moment the consumer stops. The source runs on the consumer's goroutine. Panics if n <= 0.

:::caution
- Starts up to n goroutines per iteration, all joined before it returns
- Consumes at most n elements beyond those yielded, and holds their results: one slow call at the head delays everything behind it (MapUnordered does not)
- A finished result is delivered, and a done ctx is noticed, only when the source next yields or ends: behind a slow live source both wait for the next element
:::

```go
// A product page needs four products, and each one is a request.
type Product struct {
	ID    int
	Name  string
	Price int
}

// getProduct is an ordinary function: one request, one product. It
// knows nothing about concurrency. It passes on the context it is
// given, which is what lets a call that is no longer needed be
// cancelled.
getProduct := func(ctx context.Context, id int) (Product, error) {
	var p Product
	url := fmt.Sprintf("%s/products/%d", shopURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return p, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("product %d: %s", id, resp.Status)
	}
	return p, json.NewDecoder(resp.Body).Decode(&p)
}

// Eight requests in flight at once. The products come back in the
// order of the ids, whichever response arrived first — and if one
// request fails, Collect returns that error and the rest are
// cancelled.
ctx := context.Background()
products, err := catena.Of(101, 102, 103, 104).
	Then(parallel.Map(ctx, 8, getProduct)).
	Collect()
if err != nil {
	fmt.Println("page failed:", err)
	return
}
for _, p := range products {
	fmt.Printf("%d %-9s $%d\n", p.ID, p.Name, p.Price)
}
```

```
101 keyboard  $89
102 mouse     $35
103 monitor   $329
104 webcam    $59
```

### Keep going past failures

```go
// A link checker. One dead link should not stop the check, so the
// policy here is CollectAll: run every request, and get the failures
// back as a list. The same stage with Collect would stop at the first.
check := func(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, shopURL+path, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s: %s", path, resp.Status)
	}
	return path, nil
}

links := catena.Of("/products/101", "/products/999", "/products/103", "/products/7")
alive, dead := links.Then(parallel.Map(context.Background(), 8, check)).CollectAll()
fmt.Println("alive:", alive)
fmt.Println("dead: ", dead)
```

```
alive: [/products/101 /products/103]
dead:  [/products/999: 404 Not Found /products/7: 404 Not Found]
```

### Cpu work

```go
// Not only network calls. Checksumming a release directory is disk
// and CPU, and every core can take a file. Any function from one
// thing to another fits.
release := fstest.MapFS{ // any fs.FS — os.DirFS("dist") in real life
	"app-darwin":      {Data: []byte("mach-o …")},
	"app-linux":       {Data: []byte("elf …")},
	"app-windows.exe": {Data: []byte("pe …")},
}
checksum := func(_ context.Context, name string) (string, error) {
	data, err := fs.ReadFile(release, name)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x  %s", sum[:6], name), nil
}

files := catena.Of("app-darwin", "app-linux", "app-windows.exe")
lines, err := files.Then(parallel.Map(context.Background(), 4, checksum)).Collect()
fmt.Println(strings.Join(lines, "\n"), err)
```

```
6df917c52a19  app-darwin
bc58fc1d5d26  app-linux
5d6d1545027b  app-windows.exe <nil>
```

## MapUnordered

```go
func MapUnordered[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U]
```

MapUnordered is Map yielding each result as soon as its call finishes rather than in source order: the same elements, bounds and error rules, with no call able to delay another. Panics if n <= 0.

:::caution
- Starts up to n goroutines per iteration, all joined before it returns
- Consumes at most n elements beyond those yielded
- A finished result is delivered only when the source next yields or ends
:::

```go
// Ask three mirrors for the same file and use whichever answers
// first.
download := func(ctx context.Context, mirror string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shopURL+"/mirror/"+mirror, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err // including "context canceled", for the losers
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return fmt.Sprintf("%d bytes from %s", len(body), mirror), err
}

// Results arrive as their calls finish, so First returns as soon as
// one mirror has answered. Stopping there cancels the other two
// requests — their contexts are done — and waits for them to end.
mirrors := catena.Of("eu-slow", "us-fast", "ap-slow")
winner, ok := mirrors.
	Then(parallel.MapUnordered(context.Background(), 3, download)).
	Ignore().
	First()
fmt.Println(winner, ok)
```

```
2048 bytes from us-fast true
```

## ForEach

```go
func ForEach[T any](ctx context.Context, n int, f func(context.Context, T) error) func(catena.Seq[T]) error
```

ForEach returns a terminal stage that calls f for each element, up to n calls at once, and returns the first error to occur — a call's own, or ctx.Err() — or nil once every call has succeeded. "First" is first in time, not in source order. A done ctx is an error even if every call that ran succeeded. On an error the remaining calls are cancelled through their context and joined before it returns. Panics if n <= 0.

:::caution
- Starts up to n goroutines, all joined before it returns
- Full drain on success
:::

```go
// A price changed: purge it from every edge cache, four nodes at a
// time. Nothing comes back but success or failure.
var purged atomic.Int64
purge := func(ctx context.Context, node string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, shopURL+"/purge/"+node, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", node, err)
	}
	resp.Body.Close()
	purged.Add(1)
	return nil
}

// The first error is returned, having cancelled and waited for the
// purges still running; nil means every node confirmed.
nodes := catena.Of("fra1", "iad2", "sin1", "syd1", "gru1", "nrt1")
err := nodes.Then(parallel.ForEach(context.Background(), 4, purge))
fmt.Println("purged on", purged.Load(), "nodes, err:", err)
```

```
purged on 6 nodes, err: <nil>
```

### Different calls

```go
// Not the same call for many inputs, but three different calls a
// dashboard needs, with nothing to do with each other. Put the
// functions themselves in the sequence.
getJSON := func(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shopURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(into)
}

var (
	name     string
	orders   []int
	featured struct{ Name string }
)
loads := catena.Of(
	func(ctx context.Context) error { return getJSON(ctx, "/users/7/name", &name) },
	func(ctx context.Context) error { return getJSON(ctx, "/users/7/orders", &orders) },
	func(ctx context.Context) error { return getJSON(ctx, "/products/103", &featured) },
)

// All three run together. The first to fail cancels the others, and
// all three have returned by the time err is set — so reading what
// they wrote is safe.
err := loads.Then(parallel.ForEach(context.Background(), 3,
	func(ctx context.Context, load func(context.Context) error) error { return load(ctx) }))
fmt.Println(name, orders, featured.Name, err)
```

```
ada [1001 1004] monitor <nil>
```

## Buffer

```go
func Buffer[T any](n int) func(catena.Seq[T]) catena.Seq[T]
```

Buffer returns a stage that runs its source ahead of the consumer, on a goroutine of its own, holding up to n elements ready: a slow source and a slow consumer overlap instead of taking turns. Order is preserved. T cannot be inferred — write parallel.Buffer[int](64). Panics if n <= 0.

:::caution
- Starts one goroutine per iteration, joined before it returns, so stopping waits for the source's current step
- Buffers n elements and consumes up to n+1 beyond those yielded
- Everything upstream runs concurrently with the consumer's loop body
:::

```go
// Archiving a log: reading and splitting it into lines is work, and
// so is compressing them. In one loop they take turns. Behind a
// buffer the reading runs ahead on its own goroutine, so the two
// overlap — and the lines still arrive in order.
log := strings.Repeat("GET /cart 200 12ms\nGET /pay 502 3004ms\n", 500)
lines := catena.FromSlice(strings.Split(strings.TrimSpace(log), "\n"))

var archive bytes.Buffer
zw := gzip.NewWriter(&archive)
written := 0
for line := range lines.Then(parallel.Buffer[string](256)) {
	fmt.Fprintln(zw, line)
	written++
}
zw.Close()
fmt.Println(written, "lines archived, compressed:", archive.Len() < len(log))
```

```
1000 lines archived, compressed: true
```

## ChunkedWithin

```go
func ChunkedWithin[T any](n int, d time.Duration) func(catena.Seq[T]) catena.Seq[[]T]
```

ChunkedWithin returns a stage that batches by size or by time, whichever comes first: it yields a chunk when it holds n elements, or when d has passed since the chunk's first element arrived, and yields the last partial chunk when the source ends. It never yields an empty chunk, so a silent source yields nothing. Every chunk is a fresh slice. T cannot be inferred — write parallel.ChunkedWithin[int](100, time.Second). Panics if n <= 0 or d <= 0.

:::caution
- Starts one goroutine per iteration, joined before it returns, so stopping waits for the source's current step
- Holds up to n elements in the chunk and runs the source up to another n ahead (64 at most), plus one
- Everything upstream runs concurrently with the consumer's loop body
:::

```go
// Click events arrive on a channel whenever they happen: six in a
// burst, a pause, one more.
events := make(chan string)
go func() {
	defer close(events)
	for i, page := range []string{"home", "cart", "pay", "home", "search", "item", "cart"} {
		if i == 6 {
			time.Sleep(200 * time.Millisecond)
		}
		events <- page
	}
}()

// They go to the database in bulk: up to three rows per INSERT, but
// never held back more than 50ms waiting for a batch to fill.
// Chunked alone would sit on a partial batch until more events came.
stream := catena.FromChan(context.Background(), events)
for batch := range stream.Then(parallel.ChunkedWithin[string](3, 50*time.Millisecond)) {
	fmt.Println("INSERT", len(batch), "rows:", batch)
}
```

```
INSERT 3 rows: [home cart pay]
INSERT 3 rows: [home search item]
INSERT 1 rows: [cart]
```

## Merge

```go
func Merge[T any](seqs ...catena.Seq[T]) catena.Seq[T]
```

Merge yields the elements of every sequence as they arrive, each sequence running on a goroutine of its own. Elements of one sequence keep their order; the interleaving between sequences is whatever the timing makes it. It ends when all have ended.

:::caution
- Starts one goroutine per sequence, plus one, per iteration, all joined before it returns, so stopping waits for every source's current step
- Buffers up to 64 elements between the sources and the consumer, so together they run that far ahead, plus one each
- The sources run concurrently with each other and with the consumer's loop body
:::

```go
// Three services' logs, each its own stream.
api := catena.Of("api ERROR upstream timeout", "api INFO 200 /cart", "api ERROR upstream timeout")
worker := catena.Of("worker INFO job 1 done", "worker INFO job 2 done")
db := catena.Of("db INFO checkpoint", "db ERROR deadlock detected")

// Merge reads each on its own goroutine and yields lines as they
// come, so one slow service does not hold up the others. Lines from
// one service stay in order; how the services interleave is down to
// timing, which is why this counts instead of printing.
errors := parallel.Merge(api, worker, db).
	Filter(func(line string) bool { return strings.Contains(line, "ERROR") }).
	TallyBy(func(line string) string { return strings.Fields(line)[0] })
fmt.Println("api:", errors["api"], "worker:", errors["worker"], "db:", errors["db"])
```

```
api: 2 worker: 0 db: 1
```
