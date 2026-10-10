package lint

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// UnusedSeq reports a lazy sequence that is built and discarded. A
// pipeline is a description of work: with no terminal to consume it,
// none of its callbacks ever run — which is almost never what a
// statement like s.OnEach(log) meant.
var UnusedSeq = &analysis.Analyzer{
	Name:     "catenaunusedseq",
	Doc:      "report a lazy Seq, Seq2 or Try that is built as a statement and discarded, so that nothing in it ever runs",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runUnusedSeq,
}

func runUnusedSeq(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.ExprStmt)(nil)}, func(n ast.Node) {
		call, ok := ast.Unparen(n.(*ast.ExprStmt).X).(*ast.CallExpr)
		if !ok {
			return
		}
		named, ok := types.Unalias(pass.TypesInfo.TypeOf(call)).(*types.Named)
		if !ok || !fromCatena(named.Obj()) {
			return
		}
		switch name := named.Obj().Name(); name {
		case "Seq", "Seq2", "Try":
			pass.Report(analysis.Diagnostic{
				Pos: call.Pos(),
				End: call.End(),
				Message: fmt.Sprintf("this %s is built and discarded: a lazy sequence runs nothing until a terminal (Collect, ForEach, Drain, …) consumes it",
					name),
			})
		}
	})
	return nil, nil
}
