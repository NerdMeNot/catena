package lint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Dedupe reports Distinct applied to sorted input. Equal elements of
// sorted input are adjacent, so Dedupe yields exactly what Distinct
// would without the set of everything seen.
var Dedupe = &analysis.Analyzer{
	Name:     "catenadedupe",
	Doc:      "report Distinct(Sorted(…)), which keeps a set of every value seen; on sorted input Dedupe yields the same elements in constant memory",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runDedupe,
}

func runDedupe(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		distinct, ok := unary(info, n.(*ast.CallExpr))
		if !ok || distinct.name != "Distinct" {
			return
		}
		sorted, ok := unary(info, distinct.arg)
		if !ok || (sorted.name != "Sorted" && sorted.name != "SortedDesc") {
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:            distinct.id.Pos(),
			End:            distinct.id.End(),
			Message:        "Distinct on sorted input keeps a set of every value seen: Dedupe yields the same elements in constant memory",
			SuggestedFixes: replace(distinct.id, "Use Dedupe", "Dedupe"),
		})
	})
	return nil, nil
}
