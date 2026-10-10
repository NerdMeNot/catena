package lint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// MinMax reports a full sort used to find one element. The aggregate
// returns the same element — including which of several equal ones — in
// one pass and no buffer.
var MinMax = &analysis.Analyzer{
	Name:     "catenaminmax",
	Doc:      "report Sorted(…).First() and its kin, which sort everything to find one element; Min, Max and their By and With forms do it in one pass",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runMinMax,
}

func runMinMax(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		end := n.(*ast.CallExpr)
		which, sorted := method(info, end)
		if which != "Seq.First" && which != "Seq.Last" {
			return
		}
		first := which == "Seq.First"

		var was, better, fixed string
		if u, ok := unary(info, sorted); ok && (u.name == "Sorted" || u.name == "SortedDesc") {
			// Values of an ordered type that compare equal are
			// interchangeable, so either end of either sort has an
			// aggregate.
			better = "Max"
			if (u.name == "Sorted") == first {
				better = "Min"
			}
			was = u.name
			fixed = fmt.Sprintf("%s%s(%s)", qualifier(pass, end.Pos()), better, text(pass.Fset, u.arg))
		} else if call, ok := ast.Unparen(sorted).(*ast.CallExpr); ok && first {
			// Only First: a stable sort puts the earliest of several
			// equal elements first, which is the one the aggregates
			// return. Last would be the latest, which they do not.
			by, recv := method(info, call)
			switch by {
			case "Seq.SortedBy":
				was, better = "SortedBy", "MinBy"
			case "Seq.SortedByDesc":
				was, better = "SortedByDesc", "MaxBy"
			case "Seq.SortedWith":
				was, better = "SortedWith", "MinWith"
			default:
				return
			}
			fixed = fmt.Sprintf("%s.%s(%s)", text(pass.Fset, recv), better, text(pass.Fset, call.Args[0]))
		} else {
			return
		}

		pass.Report(analysis.Diagnostic{
			Pos: end.Pos(),
			End: end.End(),
			Message: fmt.Sprintf("%s followed by %s buffers and sorts the whole input to find one element: %s finds it in one pass",
				was, which[len("Seq."):], better),
			SuggestedFixes: replace(end, "Use "+better, fixed),
		})
	})
	return nil, nil
}
