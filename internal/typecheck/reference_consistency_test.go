package typecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOctomataReferenceDocumentsBoardArrays(t *testing.T) {
	path := filepath.Join("..", "..", "Language", "reference", "runtime", "21-octomata.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Octomata reference: %v", err)
	}
	reference := string(contents)
	want := "arrays (including nested arrays) whose element type is one of those scalars"
	if !strings.Contains(reference, want) {
		t.Fatalf("Octomata reference must derive board-array wording from current typechecker truth; missing %q", want)
	}
	if strings.Contains(reference, "arrays, vectors, matrices, records, enums, and other non-scalar runtime types are unsupported") {
		t.Fatal("Octomata reference regressed to the stale scalar-only BoardSnapshot contract")
	}
}
