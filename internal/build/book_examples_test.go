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
	for _, name := range []string{"Add", "Max", "Choose", "SumTo"} {
		t.Run(name, func(t *testing.T) {
			got := functionFromMIRDump(t, dump, "WasmCompute."+name)
			path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshotName(name)+".mir")
			checkBookSnapshot(t, path, got, "MIR")
		})
	}
}

func TestCompilerOptimizationBookCFGSnapshots(t *testing.T) {
	module := loadWasmComputeMIR(t)
	for _, name := range []string{"Max", "Choose", "SumTo"} {
		t.Run(name, func(t *testing.T) {
			got, err := DumpCFG(findMIRFunction(t, module, name))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshotName(name)+".cfg")
			checkBookSnapshot(t, path, got, "CFG")
		})
	}
}

func TestCompilerOptimizationBookReachingDefinitionsSnapshots(t *testing.T) {
	module := loadWasmComputeMIR(t)
	for _, name := range []string{"Choose", "SumTo"} {
		t.Run(name, func(t *testing.T) {
			got, err := DumpReachingDefinitions(findMIRFunction(t, module, name))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshotName(name)+".reaching")
			checkBookSnapshot(t, path, got, "reaching definitions")
		})
	}
}

func TestCompilerOptimizationBookConstantDemoSnapshots(t *testing.T) {
	module := loadWasmComputeMIR(t)
	before := findMIRFunction(t, module, "ConstantDemo")
	after, _, err := OptimizeFunction(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range []struct {
		name string
		fn   MIRFunction
	}{
		{"constant-demo.before.mir", before},
		{"constant-demo.after.mir", after},
	} {
		dump := dumpMIR(MIRModule{EntryPackage: module.EntryPackage, Functions: []MIRFunction{snapshot.fn}})
		got := functionFromMIRDump(t, dump, "WasmCompute.ConstantDemo")
		path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", snapshot.name)
		checkBookSnapshot(t, path, got, "optimized MIR")
	}
}

func checkBookSnapshot(t *testing.T, path, got, kind string) {
	t.Helper()
	got = strings.TrimSpace(strings.ReplaceAll(got, "\r\n", "\n")) + "\n"
	if os.Getenv("OCT_UPDATE_BOOK_SNAPSHOTS") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(strings.ReplaceAll(string(wantBytes), "\r\n", "\n")) + "\n"
	if got != want {
		t.Fatalf("%s is stale\n--- snapshot ---\n%s--- current %s ---\n%s", path, want, kind, got)
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
