package catena_test

// Examples for Try, Seq2 and List. See example_filtering_test.go for why
// these are Example functions.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/NerdMeNot/catena"
)

// importedOrder is one row of the order export the Try examples share.
type importedOrder struct {
	ID    string
	Cents int
}

// importedOrders parses a four-row CSV export whose second row has a bad
// amount: three orders that parse and one that fails, so every operator's
// treatment of an error is visible.
func importedOrders() catena.Try[importedOrder] {
	rows := catena.Of("A-100,2500", "A-101,pending", "A-102,1200", "A-103,800")
	return rows.MapErr(func(row string) (importedOrder, error) {
		id, amount, _ := strings.Cut(row, ",")
		cents, err := strconv.Atoi(amount)
		if err != nil {
			return importedOrder{}, fmt.Errorf("order %s: bad amount %q", id, amount)
		}
		return importedOrder{ID: id, Cents: cents}, nil
	})
}

// ---- Try: intermediates ----

func ExampleTry_Map() {
	// The callback only runs on rows that parsed. The bad row passes
	// through as its error, so a failure is never formatted into something
	// that looks like a valid receipt line.
	receipts := importedOrders().Map(func(o importedOrder) string {
		return fmt.Sprintf("%s $%.2f", o.ID, float64(o.Cents)/100)
	})
	lines, errs := receipts.CollectAll()
	fmt.Println(lines)
	fmt.Println(errs)
	// Output:
	// [A-100 $25.00 A-102 $12.00 A-103 $8.00]
	// [order A-101: bad amount "pending"]
}

func ExampleTry_MapErr() {
	// Parse, then validate: two stages that can each fail. Errors from
	// either one flow on side by side, in input order.
	ports := catena.Of("8080", "https", "70000", "9090").MapErr(strconv.Atoi).
		MapErr(func(port int) (int, error) {
			if port > 65535 {
				return 0, fmt.Errorf("port %d out of range", port)
			}
			return port, nil
		})
	valid, errs := ports.CollectAll()
	fmt.Println(valid)
	fmt.Println(errs[0])
	fmt.Println(errs[1])
	// Output:
	// [8080 9090]
	// strconv.Atoi: parsing "https": invalid syntax
	// port 70000 out of range
}

func ExampleTry_FlatMap() {
	// One fallible step fans out into more fallible steps: every upload
	// that opened is parsed value by value. The upload that failed to open
	// passes through as its one error, in order with the inner ones.
	uploads := map[string]string{"mon.csv": "12,7", "wed.csv": "3,n/a"}
	open := func(name string) (string, error) {
		if body, ok := uploads[name]; ok {
			return body, nil
		}
		return "", fmt.Errorf("open %s: no such file", name)
	}
	quantities := catena.Of("mon.csv", "tue.csv", "wed.csv").MapErr(open).
		FlatMap(func(body string) catena.Try[int] {
			return catena.FromSlice(strings.Split(body, ",")).MapErr(strconv.Atoi)
		})
	vals, errs := quantities.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [12 7 3] 2
}

func ExampleTry_Filter() {
	// Keep the orders worth reviewing by hand. The predicate is not called
	// on the row that failed, and that row is not filtered out — dropping
	// it would silently discard the failure.
	large := importedOrders().Filter(func(o importedOrder) bool { return o.Cents >= 1000 })
	orders, errs := large.CollectAll()
	fmt.Println(orders)
	fmt.Println(errs)
	// Output:
	// [{A-100 2500} {A-102 1200}]
	// [order A-101: bad amount "pending"]
}

func ExampleTry_FilterErr() {
	// A predicate that has to look something up can fail too. A failed
	// lookup takes the element's place as an error instead of being read
	// as "no" and quietly dropping the order.
	paid := map[string]bool{"A-100": true, "A-102": false}
	settled := importedOrders().FilterErr(func(o importedOrder) (bool, error) {
		isPaid, found := paid[o.ID]
		if !found {
			return false, fmt.Errorf("order %s: no payment record", o.ID)
		}
		return isPaid, nil
	})
	orders, errs := settled.CollectAll()
	fmt.Println(orders)
	fmt.Println(errs)
	// Output:
	// [{A-100 2500}]
	// [order A-101: bad amount "pending" order A-103: no payment record]
}

func ExampleTry_Take() {
	// A preview of the first two rows is two rows, bad ones included —
	// Take counts elements, so it never reads past n looking for
	// successes. For "the first two that parsed", Ignore first.
	preview, errs := importedOrders().Take(2).CollectAll()
	firstTwoGood := importedOrders().Ignore().Take(2).Collect()
	fmt.Println(preview, errs)
	fmt.Println(firstTwoGood)
	// Output:
	// [{A-100 2500}] [order A-101: bad amount "pending"]
	// [{A-100 2500} {A-102 1200}]
}

func ExampleTry_TakeWhile() {
	// Latency samples up to the first slow one. The dropped sample "--"
	// passes through without ending the run; only a successful element
	// failing the predicate stops it.
	samples := catena.Of("12", "15", "--", "18", "950", "14").MapErr(strconv.Atoi)
	fast, errs := samples.TakeWhile(func(ms int) bool { return ms < 100 }).CollectAll()
	fmt.Println(fast, len(errs))
	// Output: [12 15 18] 1
}

func ExampleTry_Drop() {
	// Skipping a header row. The header fails to parse like any other bad
	// row, and Drop counts it all the same: it skips elements, errored or
	// not.
	column := catena.Of("qty", "12", "n/a", "7").MapErr(strconv.Atoi)
	quantities, errs := column.Drop(1).CollectAll()
	fmt.Println(quantities, errs)
	// Output: [12 7] [strconv.Atoi: parsing "n/a": invalid syntax]
}

func ExampleTry_OnEach() {
	// A tap for the rows that worked — here a running total for the import
	// summary — without ending the pipeline. Failed rows pass by untouched.
	importedCents := 0
	_, errs := importedOrders().
		OnEach(func(o importedOrder) { importedCents += o.Cents }).
		CollectAll()
	fmt.Println(importedCents, len(errs))
	// Output: 4500 1
}

func ExampleTry_OnError() {
	// The logging tap, the mirror of OnEach: record each failure as it
	// goes by and still choose afterwards what failures mean. Here they
	// are logged, then skipped.
	orders := importedOrders().
		OnError(func(err error) { fmt.Println("skipped:", err) }).
		Ignore().
		Collect()
	fmt.Println(len(orders), "orders imported")
	// Output:
	// skipped: order A-101: bad amount "pending"
	// 3 orders imported
}

func ExampleTry_Recover() {
	// Fall back where a sensible default exists and let the rest through:
	// an unset timeout becomes 30s, a malformed one is still an error.
	// Reporting true replaces the element, false lets the error continue.
	errUnset := errors.New("not set")
	parse := func(s string) (time.Duration, error) {
		if s == "" {
			return 0, errUnset
		}
		return time.ParseDuration(s)
	}
	timeouts := catena.Of("5s", "", "fast").MapErr(parse).
		Recover(func(err error) (time.Duration, bool) {
			return 30 * time.Second, errors.Is(err, errUnset)
		})
	vals, errs := timeouts.CollectAll()
	fmt.Println(vals, errs)
	// Output: [5s 30s] [time: invalid duration "fast"]
}

func ExampleTry_WrapErr() {
	// The parser knows what went wrong; only this stage knows where. Add
	// that here. Returning nil keeps the original error rather than
	// turning a failure into a zero value.
	line := 0
	quantities := catena.Of("12", "n/a", "7").
		MapErr(func(field string) (int, error) {
			line++
			return strconv.Atoi(field)
		}).
		WrapErr(func(err error) error {
			return fmt.Errorf("stock.csv line %d: %w", line, err)
		})
	_, err := quantities.Collect()
	fmt.Println(err)
	// Output: stock.csv line 2: strconv.Atoi: parsing "n/a": invalid syntax
}

func ExampleTry_UntilDone() {
	// Cancellation enters at the edge. Once ctx is done the sequence
	// yields ctx's error and stops, so a shutdown surfaces like any other
	// failure. Here the request is cancelled while the first order saves.
	ctx, cancel := context.WithCancel(context.Background())
	saved, err := importedOrders().UntilDone(ctx).
		OnEach(func(importedOrder) { cancel() }).
		Collect()
	fmt.Println(len(saved), err)
	// Output: 1 context canceled
}

// ---- Try: terminals, one per error policy ----

func ExampleTry_Collect() {
	// The all-or-nothing policy: stop at the first bad value. You get the
	// error plus what was gathered before it — enough to say how far it
	// got, and "9090" is never parsed.
	ports, err := catena.Of("8080", "https", "9090").MapErr(strconv.Atoi).Collect()
	fmt.Println(ports)
	fmt.Println(err)
	// Output:
	// [8080]
	// strconv.Atoi: parsing "https": invalid syntax
}

func ExampleTry_CollectAll() {
	// The import-report policy: everything that worked and everything that
	// did not, in one pass. The two slices do not correspond positionally.
	orders, errs := importedOrders().CollectAll()
	fmt.Printf("imported %d, rejected %d\n", len(orders), len(errs))
	for _, err := range errs {
		fmt.Println("rejected:", err)
	}
	// Output:
	// imported 3, rejected 1
	// rejected: order A-101: bad amount "pending"
}

func ExampleTry_Ignore() {
	// The log-scan policy: a malformed line is not worth stopping for.
	// Drop the failures and carry on as a plain Seq, with its full API.
	statuses := catena.Of("200", "503", "-", "404", "500").MapErr(strconv.Atoi)
	serverErrors := statuses.Ignore().
		Filter(func(code int) bool { return code >= 500 }).
		Collect()
	fmt.Println(serverErrors)
	// Output: [503 500]
}

func ExampleTry_Errs() {
	// Only what went wrong: a validation pass that lists every bad value
	// and has no use for the good ones. It is the dual of Ignore; wanting
	// both from one single-pass Try is a double consume, so use CollectAll.
	flags := catena.Of("true", "yes", "false", "enabled").MapErr(strconv.ParseBool)
	flags.Errs().ForEach(func(err error) { fmt.Println(err) })
	// Output:
	// strconv.ParseBool: parsing "yes": invalid syntax
	// strconv.ParseBool: parsing "enabled": invalid syntax
}

func ExampleTry_Fold() {
	// A revenue total over rows that may not parse. It stops at the first
	// error and returns the accumulator so far alongside it, so a partial
	// total can never be mistaken for the whole.
	cents, err := importedOrders().Fold(0, func(sum int, o importedOrder) int {
		return sum + o.Cents
	})
	fmt.Println(cents, err)
	// Output: 2500 order A-101: bad amount "pending"
}

func ExampleTry_ForEach() {
	// Do something fallible with each element — here, save it. Stops at
	// the first failure from either side: a bad element, or the callback
	// returning an error.
	err := importedOrders().ForEach(func(o importedOrder) error {
		fmt.Println("saved", o.ID)
		return nil
	})
	fmt.Println("stopped:", err)
	// Output:
	// saved A-100
	// stopped: order A-101: bad amount "pending"
}

func ExampleTry_Err() {
	// A pre-flight check: is every entry in the allowlist a valid address?
	// Only the first error matters, and the parsed values are not needed.
	allowlist := catena.Of("10.0.0.1", "10.0.0.256", "10.0.0.3").MapErr(netip.ParseAddr)
	if err := allowlist.Err(); err != nil {
		fmt.Println("allowlist rejected:", err)
	}
	// Output: allowlist rejected: ParseAddr("10.0.0.256"): IPv4 field has value >255
}

func ExampleTry_Count() {
	// Count is a terminal, so it stops at the first error: this is how
	// many rows were good before the import broke, not how many are good
	// in total. That would be Ignore().Count().
	n, err := importedOrders().Count()
	fmt.Println(n, err)
	fmt.Println(importedOrders().Ignore().Count())
	// Output:
	// 1 order A-101: bad amount "pending"
	// 3
}

func ExampleTry_UntilErr() {
	// Stop at the first failure, but as a sequence: the chain goes on
	// with the full Seq API, and the error is checked once it has run —
	// the way bufio.Scanner's Err is read after the loop.
	var err error
	revenue := importedOrders().UntilErr(&err).
		Map(func(o importedOrder) int { return o.Cents }).
		Then(catena.Sum)
	if err != nil {
		fmt.Println("import stopped:", err)
	}
	fmt.Println("revenue so far:", revenue)
	// Output:
	// import stopped: order A-101: bad amount "pending"
	// revenue so far: 2500
}

func ExampleTry_Must() {
	// For input you wrote yourself — a retry schedule compiled into the
	// binary — where a failure is a programming bug. The panic value is
	// the error itself, so recover() can inspect it.
	defer func() { fmt.Println("recovered:", recover()) }()
	backoff := catena.Of("1s", "5s", "30sec").MapErr(time.ParseDuration)
	fmt.Println(backoff.Must().Collect())
	// Output: recovered: time: unknown unit "sec" in duration "30sec"
}

func ExampleTry_Pull() {
	// When the consumer has to drive — one record at a time, between
	// other work. A failed element still arrives with ok true; ok is false
	// only at the end. Ownership transfers to you: always call stop.
	next, stop := importedOrders().Pull()
	defer stop()
	order, err, ok := next()
	fmt.Println(order, err, ok)
	_, err, ok = next()
	fmt.Println(err, ok)
	// Output:
	// {A-100 2500} <nil> true
	// order A-101: bad amount "pending" true
}

func ExampleTry_Seq2() {
	// A free conversion to the standard pair iterator, for a plain range
	// loop that handles each failure its own way.
	ports := catena.Of("8080", "https", "9090").MapErr(strconv.Atoi)
	for port, err := range ports.Seq2() {
		if err != nil {
			fmt.Println("skipping:", err)
			continue
		}
		fmt.Println("listening on", port)
	}
	// Output:
	// listening on 8080
	// skipping: strconv.Atoi: parsing "https": invalid syntax
	// listening on 9090
}

// ---- Seq2: the bridge back to Seq ----

func ExampleSeq2_Filter() {
	// The predicate sees both sides of the pair: pick hosts by their
	// measured latency.
	latencies := catena.Of("api-1", "api-2", "db-1").Zip(catena.Of(42, 310, 275))
	slow := latencies.Filter(func(host string, ms int) bool { return ms > 200 })
	fmt.Println(slow.Keys().Collect())
	// Output: [api-2 db-1]
}

func ExampleSeq2_FilterNot() {
	// Drop the comment lines of a config file while every surviving line
	// keeps its original number, ready for an error message.
	lines := catena.Of("# cache", "ttl=60", "# auth", "realm=prod").WithIndex().
		FilterNot(func(_ int, line string) bool { return strings.HasPrefix(line, "#") })
	lines.ForEach(func(i int, line string) { fmt.Printf("line %d: %s\n", i+1, line) })
	// Output:
	// line 2: ttl=60
	// line 4: realm=prod
}

func ExampleSeq2_Map() {
	// Both sides can change, types included: a ranked list becomes a
	// name-to-place lookup.
	finishers := catena.Of("ada", "grace", "linus").WithIndex()
	places := finishers.Map(func(i int, name string) (string, int) { return name, i + 1 })
	fmt.Println(catena.CollectMap(places))
	// Output: map[ada:1 grace:2 linus:3]
}

func ExampleSeq2_MapValues() {
	// Keys stay put and values are rewritten. The callback receives the
	// key as well, so the new value can depend on it — here, which items
	// are on sale.
	onSale := map[string]bool{"mug": true}
	prices := catena.Of("mug", "tee").Zip(catena.Of(1200, 2500)).
		MapValues(func(sku string, cents int) int {
			if onSale[sku] {
				return cents / 2
			}
			return cents
		})
	fmt.Println(catena.CollectMap(prices))
	// Output: map[mug:600 tee:2500]
}

func ExampleSeq2_MapTo() {
	// The intended exit: collapse each pair into one value and continue
	// in Seq, where the full API lives.
	hosts := catena.Of("api.internal", "db.internal")
	addrs := hosts.Zip(catena.Of("443", "5432")).MapTo(net.JoinHostPort)
	fmt.Println(addrs.Collect())
	// Output: [api.internal:443 db.internal:5432]
}

func ExampleSeq2_Take() {
	// The podium: the first three finishers, each still paired with its
	// position.
	finishers := catena.Of("ada", "grace", "linus", "ken", "dennis").WithIndex()
	finishers.Take(3).ForEach(func(i int, name string) {
		fmt.Printf("%d. %s\n", i+1, name)
	})
	// Output:
	// 1. ada
	// 2. grace
	// 3. linus
}

func ExampleSeq2_Drop() {
	// Skip a CSV header after numbering the lines, so the data rows keep
	// the line numbers a user would see in their editor.
	rows := catena.Of("sku,qty", "mug,4", "tee,9").WithIndex().Drop(1)
	rows.ForEach(func(i int, row string) { fmt.Printf("line %d: %s\n", i+1, row) })
	// Output:
	// line 2: mug,4
	// line 3: tee,9
}

func ExampleSeq2_Keys() {
	// The settings a config map defines, in a stable order. Keys and
	// Values on the SAME single-pass Seq2 is a double consume; Unzip does
	// both in one pass.
	config := map[string]string{"port": "8080", "host": "localhost", "debug": "true"}
	fmt.Println(catena.FromMap(config).Keys().Then(catena.Sorted).Collect())
	// Output: [debug host port]
}

func ExampleSeq2_Values() {
	// Leave the keys behind once only the values matter: total units in
	// stock, whatever the SKU.
	stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
	fmt.Println(catena.FromMap(stock).Values().Then(catena.Sum))
	// Output: 13
}

func ExampleSeq2_Swap() {
	// Invert a lookup table: from code-to-name to name-to-code.
	names := map[int]string{200: "OK", 404: "Not Found"}
	codes := catena.CollectMap(catena.FromMap(names).Swap())
	fmt.Println(codes["Not Found"], codes)
	// Output: 404 map[Not Found:404 OK:200]
}

func ExampleSeq2_Fold() {
	// A cart total: each pair is a quantity and a unit price in cents.
	cart := catena.Of(2, 1, 3).Zip(catena.Of(1200, 2500, 400))
	total := cart.Fold(0, func(sum, qty, cents int) int { return sum + qty*cents })
	fmt.Println(total)
	// Output: 6100
}

func ExampleSeq2_ForEach() {
	// Both sides arrive as separate arguments — no pair type to unpack.
	env := catena.Of("HOST", "PORT").Zip(catena.Of("localhost", "8080"))
	env.ForEach(func(name, value string) {
		fmt.Printf("%s=%s\n", name, value)
	})
	// Output:
	// HOST=localhost
	// PORT=8080
}

func ExampleSeq2_Any() {
	// Is anything out of stock? Stops at the first pair that says yes.
	stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
	fmt.Println(catena.FromMap(stock).Any(func(_ string, units int) bool { return units == 0 }))
	// Output: true
}

func ExampleSeq2_All() {
	// Is every required setting filled in? Stops at the first one that is
	// not.
	settings := map[string]string{"host": "localhost", "port": "8080", "token": ""}
	complete := catena.FromMap(settings).All(func(_, value string) bool { return value != "" })
	fmt.Println(complete)
	// Output: false
}

func ExampleSeq2_Count() {
	// How many SKUs are running low.
	stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
	low := catena.FromMap(stock).Filter(func(_ string, units int) bool { return units < 5 })
	fmt.Println(low.Count())
	// Output: 2
}

func ExampleSeq2_First() {
	// The first item that cannot be fulfilled, with the comma-ok form for
	// when there is none. Nothing after it is examined.
	stock := catena.Of("mug", "cap", "tee").Zip(catena.Of(4, 0, 0))
	sku, units, ok := stock.Filter(func(_ string, units int) bool { return units == 0 }).First()
	fmt.Println(sku, units, ok)
	// Output: cap 0 true
}

func ExampleSeq2_Pull() {
	// When the consumer has to drive: take one pair now and the rest
	// later. Ownership transfers to you: always call stop.
	fields := catena.Of("id", "email").Zip(catena.Of("42", "ada@example.com"))
	next, stop := fields.Pull()
	defer stop()
	column, value, ok := next()
	fmt.Println(column, value, ok)
	// Output: id 42 true
}

func ExampleSeq2_Seq2() {
	// The standard iterator type, for the stdlib functions that take one —
	// here maps.Insert, layering overrides onto defaults.
	config := map[string]string{"host": "localhost", "port": "8080"}
	overrides := catena.Of("port", "debug").Zip(catena.Of("9090", "true"))
	maps.Insert(config, overrides.Seq2())
	fmt.Println(config)
	// Output: map[debug:true host:localhost port:9090]
}

func ExampleCollectMap() {
	// To a map. On a duplicate key the last value wins, as with plain map
	// assignment — so a later setting overrides an earlier one.
	names := catena.Of("port", "host", "port")
	settings := catena.CollectMap(names.Zip(catena.Of("8080", "localhost", "9090")))
	fmt.Println(settings)
	// Output: map[host:localhost port:9090]
}

func ExampleUnzip() {
	// Both sides in one pass — the safe way to get keys and values from
	// a single-use Seq2. Here: which log lines failed, and what they said.
	log := catena.Of("ok boot", "ERROR disk full", "ok sync", "ERROR timeout")
	failures := log.WithIndex().
		Filter(func(_ int, line string) bool { return strings.HasPrefix(line, "ERROR") })
	lineNumbers, messages := catena.Unzip(failures)
	fmt.Println(lineNumbers, messages)
	// Output: [1 3] [ERROR disk full ERROR timeout]
}

// ---- List: the eager mirror ----

func ExampleList_Len() {
	// A List is a slice, so its length is known without consuming
	// anything — unlike Count on a lazy sequence.
	replicas := catena.List[string]{"db-1", "db-2", "db-3"}
	fmt.Println("quorum:", replicas.Len()/2+1)
	// Output: quorum: 2
}

func ExampleList_At() {
	// Panics exactly like l[i] — it IS the index expression. Use it when
	// the index is known to be valid.
	stages := catena.List[string]{"build", "test", "deploy"}
	fmt.Println(stages.At(stages.Len() - 1))
	// Output: deploy
}

func ExampleList_Get() {
	// The comma-ok form, for when the index may be out of range: an
	// optional command-line argument with a default.
	args := catena.List[string]{"deploy"}
	command, _ := args.Get(0)
	target, ok := args.Get(1)
	if !ok {
		target = "staging"
	}
	fmt.Println(command, target)
	// Output: deploy staging
}

func ExampleList_Slice() {
	// One page of results. It is the slice expression, aliasing included:
	// the page shares memory with the list it came from.
	results := catena.List[string]{"ada", "grace", "linus", "ken", "dennis"}
	page, size := 1, 2
	fmt.Println(results.Slice(page*size, (page+1)*size))
	// Output: [linus ken]
}

func ExampleList_Clone() {
	// Start from shared defaults without changing them for everyone else.
	defaults := catena.List[string]{"--color", "--jobs=4"}
	mine := defaults.Clone()
	mine[1] = "--jobs=16"
	fmt.Println(defaults, mine)
	// Output: [--color --jobs=4] [--color --jobs=16]
}

func ExampleList_AsSeq() {
	// Crossing to lazy is explicit, and free — it is a view, not a copy.
	// The search stops at the first match instead of testing every port.
	candidates := catena.List[int]{80, 443, 8080, 8443, 9090}
	port, ok := candidates.AsSeq().Find(func(p int) bool { return p >= 1024 })
	fmt.Println(port, ok)
	// Output: 8080 true
}

func ExampleList_FoldRight() {
	// Wrapping a handler in middleware: the last one listed has to wrap
	// first so the first one listed ends up outermost. Exists only on
	// List: a right fold needs the whole sequence, which a List already is.
	middleware := catena.List[string]{"logging", "auth", "gzip"}
	chain := middleware.FoldRight("handler", func(name, inner string) string {
		return name + "(" + inner + ")"
	})
	fmt.Println(chain)
	// Output: logging(auth(gzip(handler)))
}

func ExampleList_Append() {
	// Two commands built from one shared prefix. With the builtin append
	// and spare capacity, the second would overwrite the first; here the
	// result never aliases the receiver.
	common := append(make(catena.List[string], 0, 8), "git", "log")
	oneline := common.Append("--oneline")
	graph := common.Append("--graph")
	fmt.Println(oneline, graph)
	// Output: [git log --oneline] [git log --graph]
}
