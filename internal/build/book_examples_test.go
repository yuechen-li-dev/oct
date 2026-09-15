package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerOptimizationBookMIRSnapshots(t *testing.T) {
	module, _, err := LoadMIR(filepath.Join("..", "..", "Examples", "WasmCompute"))
	if err != nil {
		t.Fatal(err)
	}
	dump := strings.ReplaceAll(dumpMIR(module), "\r\n", "\n")
	for _, name := range []string{"Add", "Max", "SumTo"} {
		t.Run(name, func(t *testing.T) {
			got := functionFromMIRDump(t, dump, "WasmCompute."+name)
			path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshotName(name)+".mir")
			wantBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimSpace(strings.ReplaceAll(string(wantBytes), "\r\n", "\n")) + "\n"
			if got != want {
				t.Fatalf("%s is stale\n--- snapshot ---\n%s--- current MIR ---\n%s", path, want, got)
			}
		})
	}
}

func TestCompilerOptimizationBookCFGSnapshots(t *testing.T) {
	module := loadWasmComputeMIR(t)
	for _, name := range []string{"Max", "SumTo"} {
		t.Run(name, func(t *testing.T) {
			got, err := DumpCFG(findMIRFunction(t, module, name))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshotName(name)+".cfg")
			wantBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimSpace(strings.ReplaceAll(string(wantBytes), "\r\n", "\n")) + "\n"
			if got != want {
				t.Fatalf("%s is stale\n--- snapshot ---\n%s--- current CFG ---\n%s", path, want, got)
			}
		})
	}
}

func functionFromMIRDump(t *testing.T, dump, qualifiedName string) string {
	t.Helper()
	marker := "fn " + qualifiedName + "("
	start := strings.Index(dump, marker)
	if start < 0 {
		t.Fatalf("MIR dump does not contain %s", qualifiedName)
	}
	rest := dump[start:]
	if end := strings.Index(rest, "\nfn "); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest) + "\n"
}

func snapshotName(function string) string {
	if function == "SumTo" {
		return "sum-to"
	}
	return strings.ToLower(function)
}
