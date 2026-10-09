package run

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestFallibleVoidMainHasNoPrintedResult(t *testing.T) {
	var output bytes.Buffer
	path := filepath.Join("..", "..", "testdata", "feedback_run_void", "main.oct")
	if err := Execute(path, &output); err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected result output: %q", output.String())
	}
}
