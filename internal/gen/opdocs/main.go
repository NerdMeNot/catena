// Command opdocs generates the operator reference in docs/operators/.
//
// Each operator's entry is assembled from three things that already exist,
// so the reference cannot drift from the library:
//
//   - its signature and doc comment, read from the package source;
//   - its worked example, read from the Example function in the tests —
//     which `go test` runs and whose output it verifies, so a stale
//     example fails the build rather than misleading a reader;
//   - its memory and termination markers, read from the ⚠ notes the doc
//     comments already carry.
//
// Examples are matched to operators by the godoc naming convention
// (`ExampleSeq_Filter` documents `Seq.Filter`), which means the same
// functions also appear on pkg.go.dev at no extra cost.
//
// The sub-packages are read the same way; their operators are named with
// the package (`parallel.Map`), which is also how a caller writes them.
//
// Run `go generate ./...`; CI fails if the output is not current.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// family is one page of the reference: a group of operators that answer
// the same kind of question, in the order a reader meets them.
type family struct {
	Order   int
	Slug    string
	Title   string
	Summary string
	// Members are godoc names: "Seq.Filter" for a method, "Distinct" for
	// a package function, "parallel.Map" for a sub-package's.
	Members []string
}

// source is one documented package: where it lives, and what its
// operators' names are prefixed with to keep them apart from the core's.
type source struct {
	Dir     string // relative to the module root
	Package string
	Prefix  string
}

var sources = []source{
	{Dir: ".", Package: "catena"},
	{Dir: "parallel", Package: "parallel", Prefix: "parallel."},
	{Dir: "source", Package: "source", Prefix: "source."},
	{Dir: "source/sqlsource", Package: "sqlsource", Prefix: "sqlsource."},
}

var families = []family{
	{
		Order: 1,
		Slug:  "filtering",
		Title: "Filtering",
		Summary: "Keeping some elements and dropping others — by predicate, by key, " +
			"or by what has already been seen. Everything here streams: no operator " +
			"on this page buffers the input, though the distinct family retains one " +
			"entry per distinct key.",
		Members: []string{
			"Seq.Filter", "Seq.FilterNot", "Seq.FilterIndexed", "Seq.FilterErr",
			"Distinct", "Seq.DistinctBy", "Seq.DistinctWith",
			"Duplicates", "Seq.DuplicatesBy",
			"Dedupe", "Seq.DedupeBy",
			"NonZero",
		},
	},
	{
		Order: 2,
		Slug:  "creating",
		Title: "Creating a sequence",
		Summary: "Where a pipeline starts. Some of these are re-iterable and some are " +
			"single-use — the difference is a property of the producer, not of Seq, and " +
			"each one says which below. Repeat, Generate and Cycle are infinite, so they " +
			"need something downstream that stops.",
		Members: []string{
			"Of", "FromSlice", "From", "FromMap", "From2", "FromErrs", "FromChan",
			"Empty", "Empty2", "EmptyTry", "Once1",
			"Repeat", "RepeatN", "Generate", "GenerateWhile", "Range", "Cycle",
			"Self",
		},
	},
	{
		Order: 3,
		Slug:  "slicing",
		Title: "Slicing",
		Summary: "Taking a window of the sequence by position or by a predicate. Take " +
			"and TakeWhile consume only what they emit, so either one bounds an infinite " +
			"source; TakeLast and DropLast cannot, since they have to reach the end first.",
		Members: []string{
			"Seq.Take", "Seq.TakeWhile", "Seq.TakeLast",
			"Seq.Drop", "Seq.DropWhile", "Seq.DropLast",
			"Seq.Step",
		},
	},
	{
		Order: 4,
		Slug:  "transforming",
		Title: "Transforming",
		Summary: "Changing what the elements are. These are the operators generic " +
			"methods made possible: the element type changes mid-chain, and the chain " +
			"keeps its methods. All of them stream — nothing here buffers.",
		Members: []string{
			"Seq.Map", "Seq.MapIndexed", "Seq.FilterMap",
			"Seq.FlatMap", "Seq.FlatMapSlice", "Flatten", "FlattenSlices",
			"Seq.Scan", "Seq.MapErr", "Seq.OnEach",
		},
	},
	{
		Order: 5,
		Slug:  "combining",
		Title: "Combining",
		Summary: "Putting two or more sequences together, or pairing a sequence with " +
			"itself. Concat and Chain are lazy in both operands; Zip pulls its second " +
			"operand, and the set operations buffer theirs.",
		Members: []string{
			"Seq.Concat", "Seq.Append", "Seq.Prepend", "Chain",
			"Seq.Zip", "Seq.ZipWithNext", "Seq.WithIndex", "Seq.JoinBy",
			"Seq.LeftJoinBy", "Seq.GroupJoinBy", "Seq.Then",
			"Union", "Intersect", "Except",
			"Seq.Intersperse", "Seq.IfEmpty",
		},
	},
	{
		Order: 6,
		Slug:  "ordering",
		Title: "Ordering",
		Summary: "Sorting and reversing. Every sort here is stable, and every one " +
			"buffers the whole sequence — they cannot emit a first element until they " +
			"have seen the last, so none of them terminates on an infinite source.",
		Members: []string{
			"Sorted", "SortedDesc",
			"Seq.SortedBy", "Seq.SortedByDesc", "Seq.SortedWith",
			"Seq.Reversed",
			"IsSorted", "Seq.IsSortedBy", "Seq.IsSortedWith",
		},
	},
	{
		Order: 7,
		Slug:  "batching",
		Title: "Batching",
		Summary: "Grouping consecutive elements into slices — for bulk writes, paged " +
			"requests, or moving windows. All three are package functions of necessity: " +
			"a method on Seq[T] returning Seq[[]T] is an instantiation cycle. Every " +
			"emitted slice is fresh, so retaining one is always safe.",
		Members: []string{"Chunked", "ChunkedBy", "Windowed"},
	},
	{
		Order: 8,
		Slug:  "consuming",
		Title: "Consuming",
		Summary: "The operators that make a pipeline run. Everything before one of " +
			"these is a description of work; one of these performs it, once. Collect and " +
			"ForEach drain the sequence, so neither returns on an infinite one — Pull and " +
			"ToChan hand control back to you instead, and both make you responsible for " +
			"stopping.",
		Members: []string{
			"Seq.Collect", "Seq.ToList", "Seq.Seq", "Seq.Pull", "Seq.ToChan",
			"Seq.ForEach", "Seq.ForEachIndexed", "Seq.ForEachErr", "Seq.Drain",
			"Seq.Once", "Seq.UntilDone",
		},
	},
	{
		Order: 9,
		Slug:  "searching",
		Title: "Searching and testing",
		Summary: "Asking a question of a sequence rather than transforming it. The " +
			"short-circuiting ones — First, Find, Any, All, None — stop as soon as the " +
			"answer is settled, so they terminate on an infinite source; Last, Count and " +
			"FindLast cannot, because the answer depends on the final element.",
		Members: []string{
			"Seq.First", "Seq.FirstOr", "Seq.Last", "Seq.LastOr", "Seq.Single", "Seq.ElementAt",
			"Seq.Find", "Seq.FindLast", "Seq.FindIndex", "Seq.FindMap",
			"Seq.Any", "Seq.All", "Seq.None",
			"Seq.Count", "Seq.CountWhere", "Seq.IsEmpty",
			"Contains", "IndexOf", "Equal",
		},
	},
	{
		Order: 10,
		Slug:  "folding",
		Title: "Folding and grouping",
		Summary: "Reducing a sequence to one value, or to one value per key. FoldBy is " +
			"the operator that justifies the library: it aggregates per key as elements " +
			"stream past, so its memory is bounded by the number of distinct keys rather " +
			"than by the number of elements — where GroupBy retains everything.",
		Members: []string{
			"Seq.Fold", "Seq.FoldIndexed", "Seq.FoldWhile", "Seq.FoldErr",
			"Seq.FoldBy", "Seq.Reduce",
			"Seq.FoldedBy", "GroupedBy",
			"Seq.GroupBy", "Seq.IndexBy", "Seq.TallyBy", "Seq.Associate", "Seq.Partition",
			"AssociateWith", "Tally", "ToKeySet",
		},
	},
	{
		Order: 11,
		Slug:  "aggregating",
		Title: "Aggregating",
		Summary: "Extremes, totals and joins. Two conventions run through this page: " +
			"-By returns the element with the extreme key while -Of returns the key " +
			"itself, and ties always go to the first element seen. NaN orders below " +
			"everything, so these agree with the sorts. TopNBy is the one to reach for on " +
			"a large scan — a bounded heap rather than a full sort.",
		Members: []string{
			"Seq.MaxBy", "Seq.MinBy", "Seq.MaxOf", "Seq.MinOf", "Seq.MinMaxOf",
			"Seq.MaxWith", "Seq.MinWith", "Seq.TopNBy", "Seq.BottomNBy", "Seq.Sample",
			"Max", "Min", "MinMax", "TopN", "BottomN",
			"Seq.SumOf", "Seq.ProductOf", "Seq.AverageOf", "Sum", "Product", "Average",
			"Seq.JoinToString", "Join",
		},
	},
	{
		Order: 12,
		Slug:  "errors",
		Title: "Errors",
		Summary: "Try is a sequence whose elements each either succeeded or carry an " +
			"error, and its whole design is that the pipeline does not decide what a " +
			"failure means — the consumer does, by choosing a terminal. Five rules govern " +
			"the intermediates: predicates and map functions are never called on an " +
			"errored element; counts are positional; an error never ends TakeWhile; " +
			"operators that generate an error yield the zero value with it; and the " +
			"single-error terminals stop at the first one.",
		Members: []string{
			"Try.Map", "Try.MapErr", "Try.FlatMap", "Try.Filter", "Try.FilterErr",
			"Try.Take", "Try.TakeWhile", "Try.Drop",
			"Try.OnEach", "Try.OnError", "Try.Recover", "Try.WrapErr", "Try.UntilDone",
			"Try.Collect", "Try.CollectAll", "Try.Ignore", "Try.Errs", "Try.UntilErr",
			"Try.Fold", "Try.ForEach", "Try.Err", "Try.Count", "Try.Must",
			"Try.Pull", "Try.Seq2",
		},
	},
	{
		Order: 13,
		Slug:  "pairs",
		Title: "Pairs",
		Summary: "Seq2 is the standard library's pairing type with methods — what " +
			"maps.All and slices.All speak, so every boundary with the standard library " +
			"is a free conversion. It is deliberately a bridge rather than a peer: it " +
			"carries what you need to get back to Seq and little else, and MapTo is the " +
			"intended exit.",
		Members: []string{
			"Seq2.Filter", "Seq2.FilterNot", "Seq2.Map", "Seq2.MapValues", "Seq2.MapTo",
			"Seq2.Take", "Seq2.Drop", "Seq2.Keys", "Seq2.Values", "Seq2.Swap",
			"Seq2.Fold", "Seq2.ForEach", "Seq2.Any", "Seq2.All", "Seq2.Count",
			"Seq2.First", "Seq2.Pull", "Seq2.Seq2",
			"CollectMap", "Unzip",
		},
	},
	{
		Order: 14,
		Slug:  "eager",
		Title: "Eager",
		Summary: "List is a []T carrying the whole Seq *method* set, evaluated at " +
			"once with exact preallocation. Only the List-only methods are listed here: " +
			"the mirrored operators behave identically to the Seq ones over AsSeq(), " +
			"which is not a claim but a conformance check, so their entries live on the " +
			"pages above. Reach for List when the data is small and in memory, when you " +
			"touch the result more than once, or when you want O(1) access.\n\n" +
			"The constraint-bound package functions (`Sorted`, `Sum`, `Max`, `Distinct`, " +
			"`Contains`, `Union`, `Chunked`, …) are not part of that mirror: they take a " +
			"`Seq`, so reach them through `AsSeq` and come back with `ToList` — " +
			"`catena.Sorted(l.AsSeq()).ToList()`. `Concat` likewise takes a `Seq`, so it " +
			"is `l1.Concat(l2.AsSeq())`. And a few mirrored operators cross back to lazy " +
			"because their `Seq` counterparts do: `WithIndex`, `ZipWithNext` and `FoldedBy` " +
			"return `Seq2`, `MapErr` and `FilterErr` return `Try`.",
		Members: []string{
			"List.Len", "List.At", "List.Get", "List.Slice", "List.Clone",
			"List.AsSeq", "List.FoldRight", "List.Append",
		},
	},
	{
		Order: 15,
		Slug:  "concurrency",
		Title: "Concurrency",
		Summary: "The operators that start goroutines, in the `catena/parallel` package — the " +
			"core package never does. Each is a stage that sits in a chain through " +
			"`Seq.Then`, and each joins every goroutine it started before its iteration " +
			"returns: on a normal end, an early break, a cancelled context and a panic " +
			"alike. `Map`, `MapUnordered` and `ForEach` run a callback concurrently while " +
			"the source stays on the consumer's goroutine; `Buffer`, `ChunkedWithin` and " +
			"`Merge` run the source itself on a goroutine of its own. The " +
			"[concurrency guide](../06-concurrency.md) explains the two models.\n\n" +
			"Every example below is complete: the function being run concurrently is " +
			"written out, because seeing that it is an ordinary function is most of " +
			"understanding what this package is for. The ones that make HTTP requests " +
			"talk to `shopURL`, a small server the tests start, so they are verified " +
			"against real requests.",
		Members: []string{
			"parallel.Map", "parallel.MapUnordered", "parallel.ForEach",
			"parallel.Buffer", "parallel.ChunkedWithin", "parallel.Merge",
		},
	},
	{
		Order: 16,
		Slug:  "sources",
		Title: "Sources",
		Summary: "Ready-made producers for what pipelines most often start from, in the " +
			"`catena/source` and `catena/source/sqlsource` packages. Each takes a function " +
			"that opens its resource rather than an open resource, and calls it inside the " +
			"iteration: a pipeline that is built but never consumed opens nothing, and any " +
			"exit — the end, an early stop, a panic — releases it. All of them return a " +
			"`Try`, so a failure is an element the consumer decides about.",
		Members: []string{"source.Lines", "source.Paginate", "sqlsource.Rows"},
	},
}

// firstSentence trims a family summary to its opening sentence, for the
// one-line-per-family index. Summaries are prose and some run several
// sentences; the index wants only the first.
func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i]
	}
	return strings.TrimSuffix(s, ".")
}

// Usage: opdocs [srcDir] [outDir]
//
// outDir defaults to srcDir/docs/operators. Separating them lets a test
// generate into a scratch directory purely to prove every documented
// operator still has a verified example, without touching the tree.
func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	outDir := filepath.Join(dir, "docs", "operators")
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}

	fset := token.NewFileSet()
	ops := map[string]operator{}
	examples := map[string]example{}
	for _, src := range sources {
		if err := readOperators(fset, filepath.Join(dir, src.Dir), src, ops); err != nil {
			log.Fatal(err)
		}
		if err := readExamples(fset, filepath.Join(dir, src.Dir), src, examples); err != nil {
			log.Fatal(err)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	// The directory is linked to directly from the README and docs index,
	// where GitHub renders a bare file listing unless an index exists.
	var index strings.Builder
	index.WriteString("# Operator reference\n\n" +
		"Every operator, with its signature, its memory and termination\n" +
		"behaviour, and a worked example that `go test` runs and verifies.\n" +
		"For the one-line-per-operator index with costs at a glance, see\n" +
		"[the catalog](../04-operators.md).\n\n")

	documented := map[string]bool{}
	written := 0
	for _, f := range families {
		page, n, err := render(f, ops, examples)
		if err != nil {
			log.Fatalf("%s: %v", f.Slug, err)
		}
		name := fmt.Sprintf("%02d-%s.md", f.Order, f.Slug)
		fmt.Fprintf(&index, "%d. [%s](%s) — %s\n", f.Order, f.Title, name, firstSentence(f.Summary))
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(page), 0o644); err != nil {
			log.Fatal(err)
		}
		for _, m := range f.Members {
			documented[m] = true
		}
		written += n
	}

	// Every exported operator must appear on some page. Without this the
	// reference could quietly fall behind the library — a new operator
	// would ship undocumented and nothing would say so.
	var missing []string
	for name := range ops {
		if documented[name] {
			continue
		}
		// The generated List mirror is exempt: each of those methods is
		// conformance-checked to behave exactly as the Seq operator of the
		// same name does over AsSeq(), so it is documented by that entry.
		// The Eager page explains the rule; dozens of duplicate entries would
		// bury the reference rather than complete it.
		if after, ok := strings.CutPrefix(name, "List."); ok && documented["Seq."+after] {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		log.Fatalf("these exported operators are in no family, so they would ship undocumented: %s",
			strings.Join(missing, ", "))
	}

	if err := os.WriteFile(filepath.Join(outDir, "README.md"), []byte(index.String()), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("opdocs: %d families, %d operators\n", len(families), written)
}

// operator is one entry: what it looks like and what its doc comment says.
type operator struct {
	Name      string // godoc name, e.g. "Seq.Filter"
	Signature string
	Doc       string
}

// readOperators adds every exported function and method of the package in
// dir to ops.
func readOperators(fset *token.FileSet, dir string, src source, ops map[string]operator) error {
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}
	pkg, ok := pkgs[src.Package]
	if !ok {
		return fmt.Errorf("package %s not found in %s", src.Package, dir)
	}

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil {
				recv := receiverName(fn.Recv.List[0].Type)
				if recv == "" {
					continue
				}
				name = recv + "." + name
			}
			name = src.Prefix + name
			ops[name] = operator{
				Name:      name,
				Signature: signature(fset, fn),
				Doc:       docText(fn.Doc),
			}
		}
	}
	return nil
}

func receiverName(t ast.Expr) string {
	for {
		switch r := t.(type) {
		case *ast.StarExpr:
			t = r.X
		case *ast.IndexExpr:
			t = r.X
		case *ast.IndexListExpr:
			t = r.X
		case *ast.Ident:
			return r.Name
		default:
			return ""
		}
	}
}

// signature renders the declaration without its body — what a reader needs
// to call the operator, and nothing else.
func signature(fset *token.FileSet, fn *ast.FuncDecl) string {
	stripped := *fn
	stripped.Body = nil
	stripped.Doc = nil
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, &stripped); err != nil {
		return fn.Name.Name
	}
	return strings.TrimSuffix(strings.TrimSpace(b.String()), "{")
}

// docText flattens a doc comment into prose, dropping the directive lines
// (`//go:generate`, `//catena:seq-only`) that are instructions to tools
// rather than to readers.
func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	var lines []string
	for _, c := range g.List {
		line := strings.TrimPrefix(c.Text, "//")
		if strings.HasPrefix(strings.TrimSpace(line), "catena:") || strings.HasPrefix(c.Text, "//go:") {
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// example is one worked example, taken verbatim from the tests.
type example struct {
	Body   string
	Output string
	// Helpers is the source of the package-level functions and types the
	// body calls that are declared alongside the examples. Shown under
	// the example, so nothing on the page is a name without a meaning.
	Helpers string
	// More are further examples of the same operator: ExampleMap_errors
	// is another example of Map, titled "Errors".
	More []variant
}

// variant is an additional example of an operator, after its first.
type variant struct {
	Title string
	example
}

// readExamples collects Example functions from the external test package.
//
// Bodies are taken from the raw source rather than reprinted from the AST:
// the printer resolves comments from the file's comment list, not from the
// statements, so reprinting a body silently drops the explanatory comments
// — and the `// Output:` line the whole thing hangs on.
func readExamples(fset *token.FileSet, dir string, from source, out map[string]example) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type found struct {
		subject, suffix string
		ex              example
	}
	var examples []found
	helpers := map[string]string{} // package-level name -> its declaration's source

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return err
		}
		base := fset.File(file.Pos()).Base()
		text := func(from, to token.Pos) string { return string(src[int(from)-base : int(to)-base]) }
		// Only the example files' own declarations count as helpers: the
		// other test files are full of fixtures with everyday names.
		isExampleFile := strings.HasPrefix(e.Name(), "example")

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if !isExampleFile || d.Tok != token.TYPE {
					continue
				}
				start := d.Pos()
				if d.Doc != nil {
					start = d.Doc.Pos()
				}
				for _, spec := range d.Specs {
					helpers[spec.(*ast.TypeSpec).Name.Name] = text(start, d.End())
				}
			case *ast.FuncDecl:
				if d.Recv != nil {
					continue
				}
				if !strings.HasPrefix(d.Name.Name, "Example") {
					if isExampleFile {
						start := d.Pos()
						if d.Doc != nil {
							start = d.Doc.Pos()
						}
						helpers[d.Name.Name] = text(start, d.End())
					}
					continue
				}
				// ExampleSeq_Filter -> Seq.Filter; ExampleDistinct ->
				// Distinct; ExampleMap_errors -> Map, variant "errors".
				// A suffix starts in lower case, by godoc's convention.
				rest := strings.TrimPrefix(d.Name.Name, "Example")
				if rest == "" || strings.HasPrefix(rest, "_") {
					continue // a package-level example
				}
				suffix := ""
				if i := strings.LastIndex(rest, "_"); i >= 0 && i+1 < len(rest) && rest[i+1] >= 'a' && rest[i+1] <= 'z' {
					rest, suffix = rest[:i], rest[i+1:]
				}
				body, output := splitExample(text(d.Body.Lbrace+1, d.Body.Rbrace))
				examples = append(examples, found{
					subject: from.Prefix + strings.Replace(rest, "_", ".", 1),
					suffix:  suffix,
					ex:      example{Body: body, Output: output},
				})
			}
		}
	}

	// Variants attach to their operator's first example, in source order.
	for _, f := range examples {
		f.ex.Helpers = helpersUsed(f.ex.Body, helpers)
		if f.suffix == "" {
			more := out[f.subject].More
			out[f.subject] = f.ex
			if len(more) > 0 {
				ex := out[f.subject]
				ex.More = more
				out[f.subject] = ex
			}
			continue
		}
		ex := out[f.subject]
		ex.More = append(ex.More, variant{Title: variantTitle(f.suffix), example: f.ex})
		out[f.subject] = ex
	}
	return nil
}

var identifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// helpersUsed returns the declarations of every helper the code mentions,
// and of every helper those mention, each once, in the order first met.
func helpersUsed(code string, helpers map[string]string) string {
	var order []string
	seen := map[string]bool{}
	var visit func(string)
	visit = func(text string) {
		for _, name := range identifier.FindAllString(text, -1) {
			decl, ok := helpers[name]
			if !ok || seen[name] {
				continue
			}
			seen[name] = true
			order = append(order, decl)
			visit(decl)
		}
	}
	visit(code)
	// A type declaration that names several types is one declaration.
	return strings.Join(slices.Compact(order), "\n\n")
}

// variantTitle turns an example's suffix into words: differentCalls ->
// "Different calls".
func variantTitle(suffix string) string {
	var b strings.Builder
	for i, r := range suffix {
		switch {
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			b.WriteByte(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitExample separates an Example's statements from the `// Output:`
// block that `go test` verifies against.
func splitExample(text string) (body, output string) {
	var kept, out []string
	inOutput := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "// Output:"):
			// Either `// Output: x` or a bare `// Output:` introducing
			// several lines; both forms are what go test accepts.
			inOutput = true
			if rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "// Output:")); rest != "" {
				out = append(out, rest)
			}
		case inOutput && strings.HasPrefix(trimmed, "//"):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
		default:
			inOutput = false
			kept = append(kept, line)
		}
	}
	output = strings.Join(out, "\n")
	return dedent(strings.Trim(strings.Join(kept, "\n"), "\n")), output
}

// dedent removes the one level of indentation every line carries from
// having been inside a function body.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "\t")
	}
	return strings.Join(lines, "\n")
}

// notes lifts the ⚠ sentences out of a doc comment. They are the two facts
// a reader most needs before putting an operator in a pipeline: whether it
// buffers, and whether it terminates.
func notes(doc string) []string {
	var out []string
	for _, sentence := range strings.Split(doc, ". ") {
		if strings.Contains(sentence, "⚠") {
			s := strings.TrimSpace(strings.TrimSuffix(sentence, "."))
			out = append(out, strings.TrimSpace(strings.TrimPrefix(s, "⚠")))
		}
	}
	return out
}

// summary is the doc comment with its ⚠ notes removed, since those are
// surfaced separately.
func summary(doc string) string {
	var kept []string
	for _, sentence := range strings.Split(doc, ". ") {
		if !strings.Contains(sentence, "⚠") {
			kept = append(kept, strings.TrimSpace(sentence))
		}
	}
	s := strings.Join(kept, ". ")
	if s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

func render(f family, ops map[string]operator, examples map[string]example) (string, int, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n", f.Title, f.Summary)

	for _, name := range f.Members {
		op, ok := ops[name]
		if !ok {
			return "", 0, fmt.Errorf("%s is listed in the %s family but is not an exported operator", name, f.Slug)
		}
		ex, ok := examples[name]
		if !ok {
			return "", 0, fmt.Errorf("%s has no example: add its Example function to that package's tests", name)
		}
		if ex.Output == "" {
			return "", 0, fmt.Errorf("the example for %s has no `// Output:` comment, so nothing verifies it", name)
		}

		// The heading is the bare name: the page already says which
		// type or package it belongs to.
		short := name[strings.LastIndex(name, ".")+1:]
		fmt.Fprintf(&b, "\n## %s\n\n", short)
		fmt.Fprintf(&b, "```go\n%s\n```\n\n", op.Signature)
		if s := summary(op.Doc); s != "" {
			fmt.Fprintf(&b, "%s\n\n", s)
		}
		// One block, however many notes: a stack of separate warnings
		// reads as alarm, where these are a short list of facts.
		switch ns := notes(op.Doc); len(ns) {
		case 0:
		case 1:
			fmt.Fprintf(&b, ":::caution\n%s\n:::\n\n", ns[0])
		default:
			b.WriteString(":::caution\n")
			for _, n := range ns {
				fmt.Fprintf(&b, "- %s\n", n)
			}
			b.WriteString(":::\n\n")
		}
		renderExample(&b, ex)
		for _, v := range ex.More {
			if v.Output == "" {
				return "", 0, fmt.Errorf("the %q example for %s has no `// Output:` comment, so nothing verifies it", v.Title, name)
			}
			fmt.Fprintf(&b, "\n### %s\n\n", v.Title)
			renderExample(&b, v.example)
		}
	}
	return b.String(), len(f.Members), nil
}

// renderExample writes an example's code, what it prints, and — folded
// away, since most readers will not need them — the helpers it calls.
func renderExample(b *strings.Builder, ex example) {
	fmt.Fprintf(b, "```go\n%s\n```\n\n", ex.Body)
	fmt.Fprintf(b, "```\n%s\n```\n", ex.Output)
	if ex.Helpers != "" {
		fmt.Fprintf(b, "\n<details>\n<summary>The helpers this example calls</summary>\n\n```go\n%s\n```\n\n</details>\n", ex.Helpers)
	}
}
