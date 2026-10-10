package parallel_test

// Examples for package parallel. Each is a real test — `go test` runs it
// and checks its output — and each talks to a real HTTP server: a small
// shop API started for the tests (see shopAPI below), so the calls being
// run concurrently are genuine requests with genuine latency, not
// arithmetic dressed up as I/O.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

func ExampleMap() {
	// A product page needs four products, and each is a request. Eight at
	// a time, and they come back in the order they were asked for —
	// whichever response happened to arrive first.
	ctx := context.Background()
	ids := catena.Of(101, 102, 103, 104)

	products, err := ids.Then(parallel.Map(ctx, 8, getProduct)).Collect()
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

func ExampleMap_errors() {
	// One product is gone. A failed call is an element, not an abort, so
	// the caller decides: here the page renders what it has and reports
	// what it could not load. Collect instead would stop at the first
	// failure and cancel the requests still in flight.
	ctx := context.Background()
	ids := catena.Of(101, 999, 103)

	products, errs := ids.Then(parallel.Map(ctx, 8, getProduct)).CollectAll()
	fmt.Println(len(products), "loaded;", errs)
	// Output: 2 loaded; [product 999: 404 Not Found]
}

func ExampleMapUnordered() {
	// Ask three mirrors for the same file and take whichever answers
	// first. Results arrive as their calls finish, so First returns as
	// soon as one does — and stopping cancels the other two requests and
	// waits for them to end before First returns.
	ctx := context.Background()
	mirrors := catena.Of("eu-slow", "us-fast", "ap-slow")

	winner, ok := mirrors.
		Then(parallel.MapUnordered(ctx, 3, download)).
		Ignore().
		First()
	fmt.Println(winner, ok)
	// Output: 2048 bytes from us-fast true
}

func ExampleForEach() {
	// A price changed: purge it from every edge cache, four nodes at a
	// time. ForEach returns the first error, having cancelled and waited
	// for the purges still running — or nil once every node has confirmed.
	ctx := context.Background()
	nodes := catena.Of("fra1", "iad2", "sin1", "syd1", "gru1", "nrt1")

	var confirmed atomic.Int64
	err := nodes.Then(parallel.ForEach(ctx, 4, func(ctx context.Context, node string) error {
		if err := purgeCache(ctx, node); err != nil {
			return fmt.Errorf("%s: %w", node, err)
		}
		confirmed.Add(1)
		return nil
	}))
	fmt.Println("purged on", confirmed.Load(), "nodes, err:", err)
	// Output: purged on 6 nodes, err: <nil>
}

func ExampleBuffer() {
	// Exporting orders: reading them is slow and so is uploading them.
	// Behind a buffer the reader runs ahead on its own goroutine while
	// the upload is in flight, so the two waits overlap instead of
	// adding up. The order of the rows is unchanged.
	uploaded := 0
	for row := range exportedOrders().Then(parallel.Buffer[string](64)) {
		upload(row)
		uploaded++
	}
	fmt.Println(uploaded, "rows uploaded")
	// Output: 5 rows uploaded
}

func ExampleChunkedWithin() {
	// Click events arrive on a channel and go to the database in bulk:
	// up to three rows per INSERT, but never held back for more than
	// 50ms waiting for a batch to fill. Chunked alone would sit on the
	// last partial batch until more events came.
	ctx := context.Background()
	events := catena.FromChan(ctx, clicks())

	for batch := range events.Then(parallel.ChunkedWithin[string](3, 50*time.Millisecond)) {
		fmt.Println("INSERT", len(batch), "rows:", batch)
	}
	// Output:
	// INSERT 3 rows: [home cart pay]
	// INSERT 3 rows: [home search item]
	// INSERT 1 rows: [cart]
}

func ExampleMerge() {
	// One stream out of three services' logs, each read on its own
	// goroutine. Lines from one service stay in order; how the services
	// interleave is down to timing, so this counts rather than prints.
	logs := parallel.Merge(serviceLog("api"), serviceLog("worker"), serviceLog("db"))

	errors := logs.
		Filter(func(line string) bool { return strings.Contains(line, "ERROR") }).
		TallyBy(func(line string) string { return strings.Fields(line)[0] })
	fmt.Println("api:", errors["api"], "worker:", errors["worker"], "db:", errors["db"])
	// Output: api: 2 worker: 0 db: 1
}

// ---- the shop the examples talk to ----

type product struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

// shopAPI is a real HTTP server with a little latency on every route, so
// that running requests concurrently is visibly different from not.
var shopAPI = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	catalog := map[int]product{
		101: {101, "keyboard", 89}, 102: {102, "mouse", 35},
		103: {103, "monitor", 329}, 104: {104, "webcam", 59},
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/products/"):
		id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/products/"))
		// Later products answer sooner, so responses arrive out of order.
		time.Sleep(time.Duration(110-id%100*20) * time.Millisecond / 10)
		p, ok := catalog[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(p)
	case strings.HasPrefix(r.URL.Path, "/mirror/"):
		if strings.HasSuffix(r.URL.Path, "-slow") {
			<-r.Context().Done() // answers only when the client gives up
			return
		}
		w.Write(make([]byte, 2048))
	case strings.HasPrefix(r.URL.Path, "/purge/"):
		time.Sleep(2 * time.Millisecond)
	}
}))

func get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shopAPI.URL+path, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func getProduct(ctx context.Context, id int) (product, error) {
	var p product
	resp, err := get(ctx, "/products/"+strconv.Itoa(id))
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("product %d: %s", id, resp.Status)
	}
	return p, json.NewDecoder(resp.Body).Decode(&p)
}

func download(ctx context.Context, mirror string) (string, error) {
	resp, err := get(ctx, "/mirror/"+mirror)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	if n == 0 {
		return "", errors.New(mirror + ": no data")
	}
	return fmt.Sprintf("%d bytes from %s", n, mirror), nil
}

func purgeCache(ctx context.Context, node string) error {
	resp, err := get(ctx, "/purge/"+node)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// exportedOrders reads orders the slow way, a few milliseconds apiece.
func exportedOrders() catena.Seq[string] {
	return func(yield func(string) bool) {
		for _, row := range []string{"#1001,ada,89", "#1002,bob,35", "#1003,cy,329", "#1004,ada,59", "#1005,dee,35"} {
			time.Sleep(2 * time.Millisecond)
			if !yield(row) {
				return
			}
		}
	}
}

func upload(string) { time.Sleep(2 * time.Millisecond) }

// clicks is a burst of six events, a pause, then one more.
func clicks() <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for i, page := range []string{"home", "cart", "pay", "home", "search", "item", "cart"} {
			if i == 6 {
				time.Sleep(200 * time.Millisecond)
			}
			ch <- page
		}
	}()
	return ch
}

func serviceLog(service string) catena.Seq[string] {
	lines := map[string][]string{
		"api":    {"api INFO started", "api ERROR upstream timeout", "api INFO 200 /cart", "api ERROR upstream timeout"},
		"worker": {"worker INFO job 1 done", "worker INFO job 2 done"},
		"db":     {"db INFO checkpoint", "db ERROR deadlock detected"},
	}
	return catena.FromSlice(lines[service])
}
