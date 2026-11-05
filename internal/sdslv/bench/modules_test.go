package bench

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
)

func TestImportedBenchmarkRetainsItsBoundHelperClosure(t *testing.T) {
	path := filepath.Join("..", "..", "..", "Language", "SDSL-V", "Modules", "ImportedBenchmark.sdslvbench")
	manifest, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	module, err := loadBenchmarkModule(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Benchmarks) != 1 || len(manifest.Dependencies) != 2 {
		t.Fatalf("manifest: %#v", manifest)
	}
	mir, err := lower.Module(isolatedModule(module, manifest.Benchmarks[0]))
	if err != nil {
		t.Fatal(err)
	}
	output, err := hlsl.EmitBenchmark(mir, manifest.Benchmarks[0].EntryPoint, manifest.Benchmarks[0].WorkgroupSize)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "_Double(") || !strings.Contains(output, "_TwicePlusOne(") {
		t.Fatal("benchmark isolation discarded an imported helper")
	}
	if mir.DependencyIdentity != manifest.DependencyIdentity {
		t.Fatal("benchmark isolation lost source identity")
	}
}
