package catena_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The "coming from" guide is a lookup table from other libraries' names to
// catena's. A name in its catena column that does not exist sends a reader
// to nothing, so every one of them is checked against the source.
func TestComingFromPageNamesExist(t *testing.T) {
	page, err := os.ReadFile("docs/08-coming-from.md")
	if err != nil {
		t.Fatal(err)
	}

	known := map[string]bool{}
	for sym := range exportedSymbols(t) {
		known[sym[strings.LastIndex(sym, ".")+1:]] = true // Seq.Filter -> Filter
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, "parallel", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range pkgs["parallel"].Files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.IsExported() {
				known["parallel."+fn.Name.Name] = true
			}
		}
	}

	name := regexp.MustCompile("`([A-Za-z.]+)`")
	checked := 0
	for _, line := range strings.Split(string(page), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.HasPrefix(strings.TrimSpace(cells[1]), "-") {
			continue // not a table row
		}
		for _, m := range name.FindAllStringSubmatch(cells[2], -1) {
			checked++
			if !known[m[1]] {
				t.Errorf("the guide maps %s to `%s`, which catena does not have", strings.TrimSpace(cells[1]), m[1])
			}
		}
	}
	if checked < 150 {
		t.Fatalf("only %d names checked — did the table format change?", checked)
	}
}
