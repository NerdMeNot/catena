package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeFixture puts src in the first source file and empty package stubs
// in the rest, so fixtures track the sourceFiles list.
func writeFixture(t *testing.T, dir, src string) {
	t.Helper()
	for i, f := range sourceFiles {
		content := "package catena\n"
		if i == 0 {
			content = src
		}
		if err := os.WriteFile(filepath.Join(dir, f), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGolden: generate() over the real package source must reproduce the
// committed list_gen.go byte-for-byte. This is the generate-idempotency
// gate: a hand-edited generated file, or a Seq change without `go
// generate`, fails here (and in CI's diff check).
func TestGolden(t *testing.T) {
	got, err := generate("../../..")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../list_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("generate() disagrees with the committed list_gen.go — run `go generate ./...`")
	}
}

func TestGenerateParseError(t *testing.T) {
	dir := t.TempDir()
	for _, f := range sourceFiles {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("not go source"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := generate(dir); err == nil {
		t.Fatal("expected a parse error")
	}
}

// TestGenerateSkipsAndOverrides pins the selection rules: seq-only
// annotations and hand-written names are excluded, overrides are emitted
// verbatim, and the manifest matches the emitted set.
func TestGenerateSkipsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	src := `package catena

// Keep mirrors normally.
func (s Seq[T]) Keep(n int) Seq[T] { return s }

// Gone is seq-only.
//
//catena:seq-only
func (s Seq[T]) Gone() Seq[T] { return s }

// Append is hand-written on List.
func (s Seq[T]) Append(vals ...T) Seq[T] { return s }

// NotASeqMethod has a different receiver.
func (l List[T]) NotASeqMethod() {}

// Map is an override.
func (s Seq[T]) Map[U any](f func(T) U) Seq[U] { panic("x") }
`
	writeFixture(t, dir, src)
	out, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"func (l List[T]) Keep(n int) List[T]",
		`"Keep",`,
		`"Map",`,
		"out := make([]U, 0, len(l))", // the Map override body, not delegation
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"Gone", "Append", "NotASeqMethod"} {
		if bytes.Contains(out, []byte("func (l List[T]) "+banned)) {
			t.Fatalf("output wrongly contains %s", banned)
		}
	}
}

// TestReceiverShapes pins recvBase across receiver spellings and the
// no-doc-comment path of seqOnly.
func TestReceiverShapes(t *testing.T) {
	dir := t.TempDir()
	src := `package catena

func (s Seq[T]) NoDoc(n int) Seq[T] { return s }

func (s *Seq[T]) PointerRecv() {}

func (s Seq2[K, V]) TwoParams() {}

func (s (Seq[T])) Parenthesized() {}

func unexported() {}

var x = 1
`
	writeFixture(t, dir, src)
	out, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("func (l List[T]) NoDoc(n int) List[T]")) {
		t.Fatalf("NoDoc not mirrored:\n%s", out)
	}
	if !bytes.Contains(out, []byte("func (l List[T]) PointerRecv()")) {
		t.Fatal("pointer receiver on Seq must still mirror")
	}
	for _, banned := range []string{"TwoParams", "Parenthesized", "unexported"} {
		if bytes.Contains(out, []byte(banned)) {
			t.Fatalf("output wrongly contains %s", banned)
		}
	}
}

func TestFormatFailureBranch(t *testing.T) {
	orig := formatSrc
	formatSrc = func([]byte) ([]byte, error) { return nil, os.ErrInvalid }
	defer func() { formatSrc = orig }()
	if _, err := generate("../../.."); err == nil {
		t.Fatal("expected the format-failure error")
	}
}

func TestMain_HappyAndErrorPaths(t *testing.T) {
	var fatals []any
	origFatal := fatal
	fatal = func(v ...any) { fatals = append(fatals, v...) }
	defer func() { fatal = origFatal }()

	// happy path: run in a temp copy of the source dir
	dir := t.TempDir()
	for _, f := range sourceFiles {
		b, err := os.ReadFile(filepath.Join("../../..", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	main()
	if len(fatals) != 0 {
		t.Fatalf("happy path called fatal: %v", fatals)
	}
	if _, err := os.Stat("list_gen.go"); err != nil {
		t.Fatal("main did not write list_gen.go")
	}

	// generate-error path: break a source file
	if err := os.WriteFile("seq.go", []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	main()
	if len(fatals) == 0 {
		t.Fatal("generate failure did not reach fatal")
	}

	// write-error path: valid sources, but list_gen.go is a directory
	fatals = nil
	b, _ := os.ReadFile(filepath.Join(orig, "../../..", "seq.go"))
	if err := os.WriteFile("seq.go", b, 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove("list_gen.go")
	if err := os.Mkdir("list_gen.go", 0o755); err != nil {
		t.Fatal(err)
	}
	main()
	if len(fatals) == 0 {
		t.Fatal("write failure did not reach fatal")
	}
}
