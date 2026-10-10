package lint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// TopN reports a full sort followed by Take(n). Sorting buffers the whole
// input to keep n elements of it; TopN, BottomN and their By forms keep
// the same n, in the same order, in a heap of n.
var TopN = &analysis.Analyzer{
	Name:     "catenatopn",
	Doc:      "report Sorted(…).Take(n), which buffers and sorts everything to keep n elements; TopN and BottomN hold only n",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runTopN,
}

func runTopN(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	insp.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		take := n.(*ast.CallExpr)
		name, sorted := method(info, take)
		if !push || name != "Seq.Take" {
			return true
		}
		count := text(pass.Fset, take.Args[0])

		var was, better, fixed string
		if u, ok := unary(info, sorted); ok && (u.name == "Sorted" || u.name == "SortedDesc") {
			better = map[string]string{"Sorted": "BottomN", "SortedDesc": "TopN"}[u.name]
			was = u.name
			fixed = fmt.Sprintf("%s%s(%s, %s)", qualifier(pass, take.Pos()), better, text(pass.Fset, u.arg), count)
		} else if call, ok := ast.Unparen(sorted).(*ast.CallExpr); ok {
			by, recv := method(info, call)
			switch by {
			case "Seq.SortedBy":
				was, better = "SortedBy", "BottomNBy"
			case "Seq.SortedByDesc":
				was, better = "SortedByDesc", "TopNBy"
			default:
				return true
			}
			fixed = fmt.Sprintf("%s.%s(%s, %s)", text(pass.Fset, recv), better, count, text(pass.Fset, call.Args[0]))
		} else {
			return true
		}

		d := analysis.Diagnostic{
			Pos: take.Pos(),
			End: take.End(),
			Message: fmt.Sprintf("%s followed by Take buffers and sorts the whole input to keep %s elements: %s keeps only that many",
				was, count, better),
		}
		// The replacement returns a slice where this returns a sequence,
		// so it is only a drop-in when the sequence is collected at once.
		if collect := collectOf(pass, stack); collect != nil {
			d.End = collect.End()
			d.SuggestedFixes = replace(collect, "Use "+better, fixed)
		}
		pass.Report(d)
		return true
	})
	return nil, nil
}

// collectOf returns the x.Collect() call whose x is the call on top of
// the stack, if that is how the call is used. A call always has a
// declaration and a file above it, so the two parents exist.
func collectOf(pass *analysis.Pass, stack []ast.Node) *ast.CallExpr {
	sel, ok := stack[len(stack)-2].(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	call, ok := stack[len(stack)-3].(*ast.CallExpr)
	if !ok || call.Fun != sel {
		return nil // x.Collect taken as a value, not called
	}
	if name, _ := method(pass.TypesInfo, call); name != "Seq.Collect" {
		return nil
	}
	return call
}
