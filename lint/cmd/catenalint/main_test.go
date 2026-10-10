package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The command is one line of wiring, and wiring is what breaks unnoticed.
// This builds it and runs it the two ways people do — on its own and as a
// vet tool — over the same packages the analyzers' unit tests use.
func TestCommand(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "catenalint")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	testdata, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		cmd.Dir = testdata
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	// One report each check is known to make on its package.
	reports := []string{
		"TopN keeps only that many",
		"Min finds it in one pass",
		"Dedupe yields the same elements",
		"TallyBy keeps one count per key",
		"is built and discarded",
	}

	t.Run("standalone", func(t *testing.T) {
		out, err := run(bin, "./...")
		if err == nil {
			t.Fatalf("exit status 0 despite findings:\n%s", out)
		}
		for _, want := range reports {
			if !strings.Contains(out, want) {
				t.Errorf("no report containing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("vet tool", func(t *testing.T) {
		out, err := run("go", "vet", "-vettool="+bin, "./...")
		if err == nil {
			t.Fatalf("exit status 0 despite findings:\n%s", out)
		}
		for _, want := range reports {
			if !strings.Contains(out, want) {
				t.Errorf("no report containing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("a check can be switched off by name", func(t *testing.T) {
		out, _ := run("go", "vet", "-vettool="+bin, "-catenatopn=false", "./topn")
		if strings.Contains(out, "TopN keeps") {
			t.Fatalf("catenatopn reported with -catenatopn=false:\n%s", out)
		}
	})

	t.Run("clean code exits zero", func(t *testing.T) {
		// The fixed sources are what the fixes produce; they must be
		// reported on no further.
		dir := t.TempDir()
		golden, err := os.ReadFile(filepath.Join(testdata, "dedupe", "dedupe.go.golden"))
		if err != nil {
			t.Fatal(err)
		}
		mod := "module clean\n\ngo 1.27\n\nrequire github.com/NerdMeNot/catena v1.5.0\n\nreplace github.com/NerdMeNot/catena => " +
			filepath.Dir(filepath.Dir(testdata)) + "\n"
		for name, content := range map[string]string{"go.mod": mod, "dedupe.go": string(golden)} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(bin, "./...")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("reported on fixed code: %v\n%s", err, out)
		}
	})
}
