package par_test

// The completeness check: every exported symbol of package par must be
// registered in the conformance harness or named in dedicatedCovered, and
// every operator's doc comment must declare what it starts and holds.
// An operator added without either fails here rather than shipping on
// trust.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestCompleteness(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := pkgs["par"]
	if !ok {
		t.Fatal("package par not found")
	}

	docs := map[string]string{} // exported name -> doc comment
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					docs[d.Name.Name] = d.Doc.Text()
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							t.Errorf("exported type %s: this check only knows operators", s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								t.Errorf("exported value %s: this check only knows operators", n.Name)
							}
						}
					}
				}
			}
		}
	}
	if len(docs) < 6 {
		t.Fatalf("only %d exported operators found — parser broken?", len(docs))
	}

	covered := map[string]bool{}
	for _, c := range mapOps {
		for _, name := range c.covers {
			covered[name] = true
		}
	}
	for _, c := range stageOps {
		for _, name := range c.covers {
			covered[name] = true
		}
	}
	for name := range dedicatedCovered {
		covered[name] = true
	}

	var missing []string
	for name, doc := range docs {
		if !covered[name] {
			missing = append(missing, name)
		}
		// The ⚠ sentence is the operator's declaration of the goroutines
		// it starts and the elements it holds, and it is what the
		// operator reference renders as a caution.
		if !strings.Contains(doc, "⚠") {
			t.Errorf("%s: doc comment has no ⚠ sentence declaring its goroutines and buffering", name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		t.Fatalf("exported operators with no conformance registration or dedicated test:\n%v", missing)
	}
	// A registration naming something that no longer exists is a rename
	// the harness did not follow.
	for name := range covered {
		if _, ok := docs[name]; !ok {
			t.Errorf("the harness claims to cover %q, which is not an exported operator", name)
		}
	}
}
