// Package lint holds go/analysis checks for code that uses catena: the
// places where a pipeline is correct but a different operator does the
// same thing in a smaller memory class, and the pipelines that are built
// and never run.
//
// The checks are conservative: each reports only a shape it can prove is
// the wasteful one, and offers a fix only where the replacement returns
// the same values of the same type.
//
// Run them all with the catenalint command:
//
//	go install github.com/NerdMeNot/catena/lint/cmd/catenalint@latest
//	go vet -vettool=$(which catenalint) ./...
package lint

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzers returns every check in this package.
func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{TopN, MinMax, Dedupe, Tally, UnusedSeq}
}

const catenaPath = "github.com/NerdMeNot/catena"

// fromCatena reports whether obj is declared in the catena package.
func fromCatena(obj types.Object) bool {
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == catenaPath
}

// funcName names a catena function the way its documentation does: "Sorted"
// for a package function, "Seq.Take" for a method. It is "" for anything
// else.
func funcName(obj types.Object) string {
	fn, ok := obj.(*types.Func)
	if !ok || !fromCatena(fn) {
		return ""
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return fn.Name()
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem() // catena's own unexported helpers, when it is the package analysed
	}
	return types.Unalias(t).(*types.Named).Obj().Name() + "." + fn.Name()
}

// method matches a method call x.M(...) on a catena type, returning the
// method's name ("Seq.Take") and x. It does not match a method expression
// such as catena.Seq[int].Take(x, 3), whose receiver is its first argument.
func method(info *types.Info, call *ast.CallExpr) (name string, recv ast.Expr) {
	sel, ok := stripTypeArgs(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", nil
	}
	selection := info.Selections[sel]
	if selection == nil || selection.Kind() != types.MethodVal {
		return "", nil
	}
	return funcName(selection.Obj()), sel.X
}

// stripTypeArgs returns f for f, (f), f[T] and f[T, U].
func stripTypeArgs(e ast.Expr) ast.Expr {
	e = ast.Unparen(e)
	if ix, ok := e.(*ast.IndexExpr); ok {
		e = ix.X
	}
	if ix, ok := e.(*ast.IndexListExpr); ok {
		e = ix.X
	}
	return e
}

// unaryCall is a one-argument catena package function applied to a
// sequence, in either of its two spellings.
type unaryCall struct {
	name string     // the function: "Sorted"
	arg  ast.Expr   // the sequence it is applied to
	id   *ast.Ident // the function's identifier, for renaming it in place
}

// unary matches catena.F(x) and x.Then(catena.F), which are the same
// call.
func unary(info *types.Info, e ast.Expr) (unaryCall, bool) {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return unaryCall{}, false
	}
	fun, arg := call.Fun, ast.Expr(nil)
	if name, recv := method(info, call); name == "Seq.Then" {
		fun, arg = call.Args[0], recv
	} else if len(call.Args) == 1 {
		arg = call.Args[0]
	} else {
		return unaryCall{}, false
	}
	id := funcIdent(fun)
	if id == nil {
		return unaryCall{}, false
	}
	name := funcName(info.Uses[id])
	if name == "" {
		return unaryCall{}, false
	}
	return unaryCall{name, arg, id}, true
}

// funcIdent returns the identifier naming the function in f, pkg.f, f[T]
// or pkg.f[T], nil for any other expression.
func funcIdent(e ast.Expr) *ast.Ident {
	switch e := stripTypeArgs(e).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

// qualifier returns what precedes a catena package function's name in the
// file containing pos: "catena.", another name for a renamed import, or
// "" for a dot import.
func qualifier(pass *analysis.Pass, pos token.Pos) string {
	q := "catena."
	for _, f := range pass.Files {
		if f.FileStart > pos || pos >= f.FileEnd {
			continue
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == `"`+catenaPath+`"` && imp.Name != nil {
				q = imp.Name.Name + "."
				if imp.Name.Name == "." {
					q = ""
				}
			}
		}
	}
	return q
}

// text renders an expression as source.
func text(fset *token.FileSet, e ast.Expr) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, e)
	return buf.String()
}

// replace builds the suggested fix that rewrites node as newText.
func replace(node ast.Node, message, newText string) []analysis.SuggestedFix {
	return []analysis.SuggestedFix{{
		Message:   message,
		TextEdits: []analysis.TextEdit{{Pos: node.Pos(), End: node.End(), NewText: []byte(newText)}},
	}}
}
