package catena_test

// The operator reference pairs every documented operator with an Example
// function, and the generator refuses to emit a page for one that is
// missing. Running the generator here means that check fails under
// `go test` rather than only at `go generate` time — and the examples
// themselves are already verified, since Go runs them and diffs their
// output.

import (
	"os/exec"
	"testing"
)

func TestOperatorReferenceIsGeneratable(t *testing.T) {
	out, err := exec.Command("go", "run", "./internal/gen/opdocs", ".", t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatalf("operator reference cannot be generated: %v\n%s", err, out)
	}
}
