package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden: the committed marks must be exactly what the generator
// produces. A hand-edited SVG, or a proportion changed without re-running
// `go generate`, fails here as well as in CI's diff check.
func TestGolden(t *testing.T) {
	for _, p := range palettes {
		got, err := mark(p)
		if err != nil {
			t.Fatalf("%s: %v", p.name, err)
		}
		want, err := os.ReadFile(filepath.Join("../../..", "Assets", p.name+".svg"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s.svg differs from the generator — run `go generate ./...`", p.name)
		}
	}
}

// The mark is two links hooked through each other, and every part of that
// sentence is load-bearing: two paths per link colour, a weave so they
// read as linked rather than stacked, and an opening in each link.
func TestMarkStructure(t *testing.T) {
	svg, err := mark(palettes[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(svg)
	for _, want := range []string{
		palettes[0].iron, palettes[0].ember, // both links are drawn
		`mask="url(#weave)"`,    // the ember link is woven under the iron
		`stroke-linecap="butt"`, // the openings are cut square, not rounded
		`role="img"`,            // it is announced to a screen reader
	} {
		if !strings.Contains(s, want) {
			t.Errorf("mark is missing %q", want)
		}
	}
	if got := strings.Count(s, "<path"); got != 3 {
		// two links, plus the iron link redrawn inside the weave mask
		t.Errorf("mark draws %d paths, want 3", got)
	}
}

// The two failure modes that produced a visibly broken mark during design
// must fail loudly rather than emit something subtly wrong.
func TestGeometryIsGuarded(t *testing.T) {
	t.Run("opening wider than its run", func(t *testing.T) {
		// An opening that overruns the straight run strands a fragment of
		// stroke, which reads as a rendering fault rather than a link.
		if _, err := linkPath(linkLen, linkHigh, 0.9, gapPos); err == nil {
			t.Fatal("an opening wider than its run should be refused")
		}
		if _, err := linkPath(linkLen, linkHigh, gapFrac, gapPos); err != nil {
			t.Fatalf("the shipped proportions should be accepted: %v", err)
		}
	})

	t.Run("links too far apart to interlock", func(t *testing.T) {
		if _, err := crossingReach(linkLen, linkHigh, linkLen*2); err == nil {
			t.Fatal("links whose end-caps cannot meet should be refused")
		}
		reach, err := crossingReach(linkLen, linkHigh, gapDist)
		if err != nil {
			t.Fatalf("the shipped proportions should interlock: %v", err)
		}
		if reach <= 0 || reach >= linkHigh/2 {
			t.Fatalf("crossing reach %.2f is outside the link, so the weave would miss", reach)
		}
	})
}

// main writes each mark to every place that consumes it. Exercised here
// against a scratch tree so the write paths and the failure branch are
// covered rather than only run by `go generate`.
func TestMainWritesEveryTarget(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"Assets", filepath.Join("website", "public")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var fatals []any
	restoreFatal := fatal
	fatal = func(v ...any) { fatals = append(fatals, v...) }
	defer func() { fatal = restoreFatal }()

	os.Args = []string{"assets", dir}
	main()
	if len(fatals) != 0 {
		t.Fatalf("writing to a prepared tree called fatal: %v", fatals)
	}
	for _, p := range palettes {
		for _, target := range targets(dir, p.name) {
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s was not written: %v", target, err)
			}
		}
	}

	// An unwritable destination must be reported, not ignored.
	fatals = nil
	os.Args = []string{"assets", filepath.Join(dir, "does-not-exist")}
	main()
	if len(fatals) == 0 {
		t.Fatal("writing into a missing directory should have been reported")
	}
}
