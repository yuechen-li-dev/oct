package typecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOctomataReferenceDocumentsPersistentBoardValues(t *testing.T) {
	path := filepath.Join("..", "..", "Language", "reference", "runtime", "21-octomata.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Octomata reference: %v", err)
	}
	reference := string(contents)
	for _, want := range []string{"deterministic persistent values", "arrays, vectors, matrices, records, and enums", "Function values remain excluded"} {
		if !strings.Contains(reference, want) {
			t.Fatalf("Octomata reference must derive board persistence wording from current typechecker truth; missing %q", want)
		}
	}
	if strings.Contains(reference, "Vectors, matrices, records, enums, and other runtime types are unsupported") {
		t.Fatal("Octomata reference regressed to the stale scalar-only BoardSnapshot contract")
	}
}
