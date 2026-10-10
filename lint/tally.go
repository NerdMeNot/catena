package lint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Tally reports a GroupBy whose buckets are only ever measured. GroupBy
// retains every element; TallyBy keeps one count per key.
//
// It recognises one shape — ranging directly over the GroupBy and using
// the bucket only as len(bucket) — and stays silent about the rest.
var Tally = &analysis.Analyzer{
	Name:     "catenatally",
	Doc:      "report ranging over GroupBy(…) only to take len of each bucket, which retains every element to count them; TallyBy keeps a count per key",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runTally,
}

func runTally(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	insp.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, func(n ast.Node) {
		loop := n.(*ast.RangeStmt)
		groupBy, ok := ast.Unparen(loop.X).(*ast.CallExpr)
		if !ok {
			return
		}
		if which, _ := method(info, groupBy); which != "Seq.GroupBy" && which != "List.GroupBy" {
			return
		}
		value, ok := loop.Value.(*ast.Ident)
		if !ok {
			return
		}
		bucket := info.Defs[value]
		if bucket == nil {
			return // assigned to an existing variable, which may be read later
		}

		uses, measured := 0, 0
		ast.Inspect(loop.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if info.Uses[n] == bucket {
					uses++
				}
			case *ast.CallExpr:
				fun, ok := ast.Unparen(n.Fun).(*ast.Ident)
				if !ok || len(n.Args) != 1 {
					break
				}
				if _, builtin := info.Uses[fun].(*types.Builtin); !builtin || fun.Name != "len" {
					break
				}
				if arg, ok := ast.Unparen(n.Args[0]).(*ast.Ident); ok && info.Uses[arg] == bucket {
					measured++
				}
			}
			return true
		})
		if uses == 0 || uses != measured {
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:     groupBy.Pos(),
			End:     groupBy.End(),
			Message: "GroupBy retains every element, and this loop only counts them: TallyBy keeps one count per key",
		})
	})
	return nil, nil
}
