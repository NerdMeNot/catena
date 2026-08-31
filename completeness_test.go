package catena_test

// §9.5: every exported symbol must be registered in the conformance
// registries or explicitly listed as covered by a dedicated test. An
// operator that exists but is untested fails CI — this is the mechanism
// that keeps the conformance suite true over time, not a review
// convention.
//
// Enumeration uses go/parser over the package source (not reflect: Go
// cannot enumerate uninstantiated generic methods at runtime — §7.3).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// dedicatedCovered lists exported symbols whose conformance lives in
// dedicated test files rather than a registry entry. Every name here must
// point at a real test; adding a name to silence the completeness check
// without writing the test defeats the suite.
var dedicatedCovered = map[string]string{
	// types (checked for existence only; their methods are checked individually)
	"Seq":     "whole suite",
	"Seq2":    "seq2_test.go",
	"Try":     "try_test.go",
	"List":    "list_test.go",
	"Numeric": "aggregation registrations",
	"Integer": "TestRange",
	"Version": "TestVersion + the release workflow's tag check",

	// constructors
	"Of":            "conformance inputs throughout",
	"From":          "TestInteropSeqConversion",
	"From2":         "TestSeq2EarlyTermination",
	"FromErrs":      "try_test.go",
	"FromSlice":     "conformance harness source",
	"FromMap":       "TestSeq2Operators/FromMap_CollectMap_roundtrip",
	"FromChan":      "TestFromChan",
	"Empty":         "TestRepeatAndOnce1AndEmpty",
	"Empty2":        "TestRepeatAndOnce1AndEmpty",
	"EmptyTry":      "TestRepeatAndOnce1AndEmpty",
	"Once1":         "TestRepeatAndOnce1AndEmpty",
	"Repeat":        "TestRepeatAndOnce1AndEmpty",
	"RepeatN":       "TestRepeatAndOnce1AndEmpty + panics table",
	"Generate":      "TestGenerate",
	"GenerateWhile": "TestGenerateWhile",
	"Range":         "TestRange",
	"Cycle":         "TestCycle",
	"Self":          "TestSelf",

	// Seq methods with dedicated homes
	"Seq.Once":         "registry (single) + TestOncePanicMessage",
	"Seq.Seq":          "TestInteropSeqConversion",
	"Seq.Pull":         "TestPull",
	"Seq.ToChan":       "TestToChan",
	"Seq.UntilDone":    "registry + TestSeqUntilDoneCancellation",
	"Seq.IsEmpty":      "registry + TestIsEmptyConsumesOneElement",
	"Seq.Single":       "registry + TestSingleStopsAtSecondElement",
	"Seq.ElementAt":    "registry + TestElementAtNegative",
	"Seq.Zip":          "registry (Zip_MapTo) + TestZipConsumption",
	"Seq.MaxBy":        "registry + TestTiePolicies",
	"Seq.MinBy":        "registry + TestTiePolicies",
	"Seq.MaxWith":      "registry + TestTiePolicies",
	"Seq.MinWith":      "registry + TestTiePolicies",
	"Seq.IndexBy":      "registry + TestTiePolicies",
	"Seq.TopNBy":       "registry + TestTopNStability",
	"Seq.BottomNBy":    "registry + TestTopNStability",
	"Seq.JoinToString": "registry + TestJoinToStringEmpty",
	"Seq.MapErr":       "registry + TestTryR4ZeroOnError",
	"Seq.FilterErr":    "registry + TestTryR4ZeroOnError",

	// package functions with dedicated homes
	"Equal":     "TestEqual",
	"TopN":      "registry + TestTopNStability",
	"Windowed":  "registry + TestWindowedShapes",
	"Chunked":   "registry + TestWindowedShapes (L3)",
	"ChunkedBy": "registry + TestChunkedByRuns",
	"Join":      "registry + TestJoinToStringEmpty",
	"Max":       "registry + TestFloatNaN",
	"Min":       "registry + TestFloatNaN",
	"Sorted":    "registry + TestFloatNaN",

	// Seq2 methods: seq2_test.go covers the whole surface
	"Seq2.Filter": "TestSeq2Operators", "Seq2.FilterNot": "TestSeq2Operators",
	"Seq2.Map": "TestSeq2Operators", "Seq2.MapValues": "TestSeq2Operators",
	"Seq2.MapTo": "TestSeq2Operators + registry adapters",
	"Seq2.Take":  "TestSeq2Operators", "Seq2.Drop": "TestSeq2Operators",
	"Seq2.Keys": "TestSeq2Operators", "Seq2.Values": "TestSeq2Operators",
	"Seq2.Swap": "TestSeq2Operators", "Seq2.Fold": "TestSeq2Operators",
	"Seq2.ForEach": "TestSeq2Operators", "Seq2.Any": "TestSeq2Operators",
	"Seq2.All": "TestSeq2Operators", "Seq2.Count": "TestSeq2Operators",
	"Seq2.First": "TestSeq2Operators", "Seq2.Seq2": "collect2 throughout",
	"Seq2.Pull": "TestSeq2Operators/Pull",

	// Try methods: try_test.go covers the whole surface (C14)
	"Try.Map": "TestTryR1PassThrough", "Try.MapErr": "TestTryR1PassThrough",
	"Try.FlatMap": "TestTryR1PassThrough", "Try.Filter": "TestTryR1PassThrough",
	"Try.FilterErr": "TestTryR1PassThrough", "Try.Take": "TestTryR2PositionalCounting",
	"Try.TakeWhile": "TestTryR3TakeWhile", "Try.Drop": "TestTryR2PositionalCounting",
	"Try.OnEach": "TestTryR1PassThrough", "Try.OnError": "TestTryOnError",
	"Try.Recover": "TestTryRecover", "Try.WrapErr": "TestTryWrapErr",
	"Try.UntilDone": "TestTryUntilDone", "Try.Collect": "TestTryR5FirstErrorTerminals",
	"Try.CollectAll": "TestTryCollectAll", "Try.Fold": "TestTryR5FirstErrorTerminals",
	"Try.ForEach": "TestTryR5FirstErrorTerminals", "Try.Ignore": "TestTryIgnoreAndErrs",
	"Try.Errs": "TestTryIgnoreAndErrs", "Try.Must": "TestTryMust",
	"Try.Err": "TestTryR5FirstErrorTerminals", "Try.Count": "TestTryR5FirstErrorTerminals",
	"Try.Pull": "TestTryPull", "Try.Seq2": "collectTry throughout",
}

// listCovered marks the generated List mirror: list_test.go's C15 check
// runs every mirrored method against its Seq equivalent, and the
// list-only methods have dedicated tests there.
func listCoveredName(name string) bool {
	return len(name) > 5 && name[:5] == "List."
}

func exportedSymbols(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := pkgs["catena"]
	if !ok {
		t.Fatal("package catena not found")
	}
	syms := map[string]bool{}
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if d.Recv == nil {
					syms[d.Name.Name] = true
					continue
				}
				recv := d.Recv.List[0].Type
				// unwrap *T and generic instantiations to the base name
				for {
					switch r := recv.(type) {
					case *ast.StarExpr:
						recv = r.X
					case *ast.IndexExpr:
						recv = r.X
					case *ast.IndexListExpr:
						recv = r.X
					default:
						goto unwrapped
					}
				}
			unwrapped:
				if id, ok := recv.(*ast.Ident); ok {
					syms[id.Name+"."+d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						if sp.Name.IsExported() {
							syms[sp.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, name := range sp.Names {
							if name.IsExported() {
								syms[name.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return syms
}

func TestCompleteness(t *testing.T) {
	syms := exportedSymbols(t)
	if len(syms) < 100 {
		t.Fatalf("only %d exported symbols found — parser broken?", len(syms))
	}

	covered := map[string]bool{}
	for name := range dedicatedCovered {
		covered[name] = true
	}
	for _, c := range seqOpRegistry {
		for _, name := range c.covers {
			covered[name] = true
			covered["Seq."+name] = true
		}
	}
	for _, c := range termOpRegistry {
		for _, name := range c.covers {
			covered[name] = true
			covered["Seq."+name] = true
		}
	}

	var missing []string
	for sym := range syms {
		if !covered[sym] && !listCoveredName(sym) {
			missing = append(missing, sym)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("exported symbols with no conformance registration or dedicated test:\n%v", missing)
	}

	// The registry must not claim coverage of symbols that don't exist —
	// that would be a stale registration hiding a rename.
	for name := range covered {
		bare := name
		if !syms[bare] && !syms["Seq."+bare] {
			// registry names may be either package-level or Seq methods
			if _, isDedicated := dedicatedCovered[name]; isDedicated && !syms[name] {
				t.Errorf("dedicatedCovered lists %q, which is not an exported symbol", name)
			}
		}
	}
}
