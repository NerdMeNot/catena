---
title: "Concurrency"
description: "the `parallel` package: `Then`, concurrent `Map` with per-element errors, `MapTry` over input that can fail, `Retry` and `Limit` for a flaky, rate-limited service, `Do`, batching by time"
sidebar:
  order: 9
---

Concurrency: a nightly job that turns a batch of orders into a shipping report, where every order needs two calls to other services. Sequentially that is slow in the dullest way — waiting. The parallel package runs the calls several at a time without the pipeline changing shape: a stage goes into the chain with Then, a failed call is an element so the consumer still picks the error policy, and every goroutine a stage starts has exited before the line that consumed it returns. Input that can already fail goes through MapTry; a service that fails now and then, or allows only so many requests, gets Retry and Limit around the call.  The services here are a real HTTP server on localhost with 40ms of latency per request, so the timings printed are real.

Run it: `go run ./examples/09-concurrency`

```go
// Concurrency: a nightly job that turns a batch of orders into a shipping
// report, where every order needs two calls to other services. Sequentially
// that is slow in the dullest way — waiting. The parallel package runs the
// calls several at a time without the pipeline changing shape: a stage goes
// into the chain with Then, a failed call is an element so the consumer
// still picks the error policy, and every goroutine a stage starts has
// exited before the line that consumed it returns. Input that can already
// fail goes through MapTry; a service that fails now and then, or allows
// only so many requests, gets Retry and Limit around the call.
//
// The services here are a real HTTP server on localhost with 40ms of
// latency per request, so the timings printed are real.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

type Order struct {
	ID       int
	Customer string
	SKU      string
}

type Shipment struct {
	Order   Order
	Address string
	InStock bool
}

func main() {
	api := startServices()
	defer api.Close()
	ctx := context.Background()

	orders := catena.Of(
		Order{1001, "ada", "kbd-01"}, Order{1002, "bob", "mse-07"},
		Order{1003, "ghost", "kbd-01"}, // no such customer
		Order{1004, "cy", "mon-27"}, Order{1005, "ada", "cam-02"},
		Order{1006, "dee", "kbd-01"}, Order{1007, "eve", "mse-07"},
		Order{1008, "flo", "mon-27"},
	)

	// prepare makes the two calls an order needs and combines them. It is
	// an ordinary function: it takes a context, returns a value or an
	// error, and knows nothing about being run concurrently.
	prepare := func(ctx context.Context, o Order) (Shipment, error) {
		address, err := api.address(ctx, o.Customer)
		if err != nil {
			return Shipment{}, fmt.Errorf("order %d: %w", o.ID, err)
		}
		inStock, err := api.inStock(ctx, o.SKU)
		if err != nil {
			return Shipment{}, fmt.Errorf("order %d: %w", o.ID, err)
		}
		return Shipment{o, address, inStock}, nil
	}

	// Eight orders, two 40ms calls each: 640ms one at a time. Four at a
	// time it is two rounds of 80ms. Map keeps the orders in order, and
	// CollectAll keeps going past the order that cannot be prepared.
	start := time.Now()
	shipments, failures := orders.
		Then(parallel.Map(ctx, 4, prepare)).
		CollectAll()
	fmt.Printf("prepared %d shipments in about %v (sequentially: 640ms)\n",
		len(shipments), time.Since(start).Round(40*time.Millisecond))
	for _, s := range shipments[:3] {
		fmt.Printf("  #%d -> %-12s in stock: %v\n", s.Order.ID, s.Address, s.InStock)
	}
	fmt.Println("could not prepare:", failures)

	// The same stage under a stricter policy. Collect stops at the first
	// failure; the calls still in flight are cancelled through their
	// context and have finished before Collect returns.
	api.requests.Store(0)
	_, err := orders.Then(parallel.Map(ctx, 4, prepare)).Collect()
	fmt.Printf("strict run stopped: %v\n", err)
	fmt.Printf("  it made %d of the 16 requests a full run makes\n", api.requests.Load())

	// When order does not matter, MapUnordered hands each result over as
	// its call finishes. Here: which SKUs are out of stock — the answer
	// is a set, so nothing is gained by waiting for the slow ones in turn.
	missing := catena.Of("kbd-01", "mse-07", "mon-27", "cam-02").
		Then(parallel.MapUnordered(ctx, 4, func(ctx context.Context, sku string) (string, error) {
			ok, err := api.inStock(ctx, sku)
			if err != nil || ok {
				return "", err
			}
			return sku, nil
		})).
		Ignore().
		Filter(func(sku string) bool { return sku != "" }).
		Collect()
	fmt.Println("out of stock:", missing)

	// Orders often arrive as text, and text can fail to parse. MapTry
	// takes the parsed lines as a Try: a line that did not parse is never
	// prepared — no request is made for it — and arrives in its place.
	lines := catena.Of("1001,ada,kbd-01", "1002,bob", "1004,cy,mon-27")
	ready, problems := lines.MapErr(parseOrder).
		Then(parallel.MapTry(ctx, 4, prepare)).
		CollectAll()
	fmt.Printf("from text: %d prepared, problems: %v\n", len(ready), problems)

	// The warehouse's stock service is flakier: it allows ten requests per
	// 100ms, and turns the first request for any SKU away with a 503.
	// Retry tries again for a 503 (and only a 503); Limit, inside it,
	// keeps every try within the quota, however many workers are calling.
	api.requests.Store(0)
	unavailable := func(err error) bool { return errors.Is(err, errUnavailable) }
	backoff := func(attempt int) time.Duration { return time.Duration(attempt) * 5 * time.Millisecond }
	check := parallel.Retry(3, backoff, unavailable,
		parallel.Limit(10, 100*time.Millisecond, api.warehouseStock))
	skus := catena.Of("kbd-01", "mse-07", "mon-27", "cam-02", "hub-04", "cbl-09", "dock-3", "pen-11")
	checked, err := skus.Then(parallel.Map(ctx, 8, check)).Collect()
	fmt.Printf("warehouse: %d SKUs checked in %d requests, err: %v\n",
		len(checked), api.requests.Load(), err)

	// Not every concurrent job is "the same call for many inputs".
	// Sometimes it is three different calls that a page needs at once. Do
	// runs them together and returns the first error, having cancelled
	// the others; once it returns, all three have.
	var address string
	var stocked bool
	var recent int
	start = time.Now()
	err = parallel.Do(ctx,
		func(ctx context.Context) (err error) { address, err = api.address(ctx, "ada"); return },
		func(ctx context.Context) (err error) { stocked, err = api.inStock(ctx, "kbd-01"); return },
		func(ctx context.Context) (err error) { recent, err = api.recentOrders(ctx, "ada"); return },
	)
	fmt.Printf("dashboard for ada in about %v: %s, kbd-01 in stock: %v, %d recent orders (err: %v)\n",
		time.Since(start).Round(40*time.Millisecond), address, stocked, recent, err)

	// And the write side. Audit events arrive whenever they arrive, and
	// the audit log wants them in bulk: up to four per write, but never
	// held back more than 100ms. Chunked alone would sit on the last
	// partial batch until more events came — or forever.
	events := make(chan string)
	go func() {
		defer close(events)
		for i := 1; i <= 6; i++ {
			if i == 6 {
				time.Sleep(300 * time.Millisecond) // a quiet spell
			}
			events <- fmt.Sprintf("shipped #%d", 1000+i)
		}
	}()
	fmt.Println("audit log writes:")
	for batch := range catena.FromChan(ctx, events).Then(parallel.ChunkedWithin[string](4, 100*time.Millisecond)) {
		fmt.Printf("  %d events: %v\n", len(batch), batch)
	}
}

// parseOrder reads "id,customer,sku".
func parseOrder(line string) (Order, error) {
	f := strings.Split(line, ",")
	if len(f) != 3 {
		return Order{}, fmt.Errorf("order line %q: want id,customer,sku", line)
	}
	id, err := strconv.Atoi(f[0])
	if err != nil {
		return Order{}, fmt.Errorf("order line %q: %w", line, err)
	}
	return Order{id, f[1], f[2]}, nil
}

// ---- the services ----

var errUnavailable = errors.New("503 Service Unavailable")

type services struct {
	*httptest.Server
	requests atomic.Int64
	seen     sync.Map // warehouse SKUs asked about at least once
}

// startServices runs the customer, inventory and order-history services
// as one local HTTP server, each request taking 40ms.
func startServices() *services {
	s := &services{}
	addresses := map[string]string{
		"ada": "12 Canal St", "bob": "4 Hill Rd", "cy": "9 Dock Lane",
		"dee": "77 Mill Way", "eve": "3 Elm Court", "flo": "58 Quay Side",
	}
	outOfStock := map[string]bool{"mon-27": true}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		select {
		case <-time.After(40 * time.Millisecond):
		case <-r.Context().Done(): // the client gave up: so do we
			return
		}
		kind, name, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		switch kind {
		case "customers":
			address, ok := addresses[name]
			if !ok {
				http.Error(w, "no customer "+name, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(address)
		case "stock":
			json.NewEncoder(w).Encode(!outOfStock[name])
		case "history":
			json.NewEncoder(w).Encode(len(name) - 1)
		case "warehouse":
			if _, again := s.seen.LoadOrStore(name, true); !again {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			json.NewEncoder(w).Encode(!outOfStock[name])
		}
	}))
	return s
}

// call makes one request and decodes its JSON answer. It passes the
// context on, which is what lets a stage cancel a request it no longer
// needs.
func call[T any](ctx context.Context, s *services, path string) (T, error) {
	var out T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+path, nil)
	if err != nil {
		return out, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return out, fmt.Errorf("%s: %w", path, errUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("%s: %s", path, resp.Status)
	}
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

func (s *services) address(ctx context.Context, customer string) (string, error) {
	return call[string](ctx, s, "/customers/"+customer)
}

func (s *services) inStock(ctx context.Context, sku string) (bool, error) {
	return call[bool](ctx, s, "/stock/"+sku)
}

func (s *services) recentOrders(ctx context.Context, customer string) (int, error) {
	return call[int](ctx, s, "/history/"+customer)
}

func (s *services) warehouseStock(ctx context.Context, sku string) (bool, error) {
	return call[bool](ctx, s, "/warehouse/"+sku)
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/09-concurrency/main.go)
