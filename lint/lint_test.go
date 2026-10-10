package lint_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/NerdMeNot/catena/lint"
)

// Each analyzer runs over its own package under testdata, a module that
// imports the real catena from this repository. A `// want` comment names
// each expected report; a .golden file is the source after every
// suggested fix.
func TestAnalyzers(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		analyzer *analysis.Analyzer
		fixes    bool
	}{
		{lint.TopN, true},
		{lint.MinMax, true},
		{lint.Dedupe, true},
		{lint.Tally, false},
		{lint.UnusedSeq, false},
	} {
		pkg := "lintdata/" + tc.analyzer.Name[len("catena"):]
		t.Run(tc.analyzer.Name, func(t *testing.T) {
			if tc.fixes {
				analysistest.RunWithSuggestedFixes(t, testdata, tc.analyzer, pkg)
				return
			}
			analysistest.Run(t, testdata, tc.analyzer, pkg)
		})
	}
}

// The library is the largest body of catena code there is, and the one
// place the analyzers meet catena's own unexported methods. Every analyzer
// must run over it cleanly and report nothing. Its tests are left out:
// they build invalid pipelines on purpose, to prove that building one
// panics.
func TestAnalyzersOnCatenaItself(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	const catena = "github.com/NerdMeNot/catena"
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadSyntax,
		Dir:  testdata,
		Env:  append(os.Environ(), "GOWORK=off"),
	}, catena, catena+"/par", catena+"/source/...")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 4 || packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("loaded %d packages", len(pkgs))
	}
	graph, err := checker.Analyze(lint.Analyzers(), pkgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	for act := range graph.All() {
		if act.Err != nil {
			t.Errorf("%s on %s: %v", act.Analyzer.Name, act.Package.PkgPath, act.Err)
		}
		for _, d := range act.Diagnostics {
			t.Errorf("%s: %s: %s", act.Package.Fset.Position(d.Pos), act.Analyzer.Name, d.Message)
		}
	}
}

func TestAnalyzersAreListed(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range lint.Analyzers() {
		if err := analysis.Validate([]*analysis.Analyzer{a}); err != nil {
			t.Error(err)
		}
		if seen[a.Name] {
			t.Errorf("%s is listed twice", a.Name)
		}
		seen[a.Name] = true
	}
	if len(seen) != 5 {
		t.Errorf("%d analyzers listed", len(seen))
	}
}
