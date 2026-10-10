package parallel_test

// Examples for package parallel. Each is a real test — `go test` runs it
// and checks its output — and each is complete: the function being run
// concurrently is written out in the example, because seeing what an
// ordinary function looks like is most of understanding what this package
// is for.
//
// The ones that make HTTP requests talk to shopURL, a small server
// started for the tests (at the bottom of this file), so the requests and
// their latency are genuine.

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing/fstest"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

func ExampleMap() {
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
	// Output:
	// 101 keyboard  $89
	// 102 mouse     $35
	// 103 monitor   $329
	// 104 webcam    $59
}

func ExampleMap_keepGoingPastFailures() {
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
	// Output:
	// alive: [/products/101 /products/103]
	// dead:  [/products/999: 404 Not Found /products/7: 404 Not Found]
}

func ExampleMap_cpuWork() {
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
	// Output:
	// 6df917c52a19  app-darwin
	// bc58fc1d5d26  app-linux
	// 5d6d1545027b  app-windows.exe <nil>
}

func ExampleMapUnordered() {
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
	// Output: 2048 bytes from us-fast true
}

func ExampleForEach() {
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
	// Output: purged on 6 nodes, err: <nil>
}

func ExampleForEach_differentCalls() {
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
	// Output: ada [1001 1004] monitor <nil>
}

func ExampleBuffer() {
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
	// Output: 1000 lines archived, compressed: true
}

func ExampleChunkedWithin() {
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
	// Output:
	// INSERT 3 rows: [home cart pay]
	// INSERT 3 rows: [home search item]
	// INSERT 1 rows: [cart]
}

func ExampleMerge() {
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
	// Output: api: 2 worker: 0 db: 1
}

// ---- the server the examples talk to ----

// shopURL is the base URL of a small HTTP API started for these tests. It
// has a little latency on every route, so that running requests
// concurrently is visibly different from not.
var shopURL = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	type product struct {
		ID    int
		Name  string
		Price int
	}
	catalog := map[int]product{
		101: {101, "keyboard", 89}, 102: {102, "mouse", 35},
		103: {103, "monitor", 329}, 104: {104, "webcam", 59},
	}
	switch path := r.URL.Path; {
	case strings.HasPrefix(path, "/products/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(path, "/products/"))
		// Later products answer sooner, so responses arrive out of order.
		time.Sleep(time.Duration(11-id%100*2) * time.Millisecond)
		p, ok := catalog[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(p)
	case strings.HasPrefix(path, "/mirror/"):
		if strings.HasSuffix(path, "-slow") {
			<-r.Context().Done() // answers only when the client gives up
			return
		}
		w.Write(make([]byte, 2048))
	case strings.HasPrefix(path, "/purge/"):
		time.Sleep(2 * time.Millisecond)
	case path == "/users/7/name":
		json.NewEncoder(w).Encode("ada")
	case path == "/users/7/orders":
		json.NewEncoder(w).Encode([]int{1001, 1004})
	}
})).URL
