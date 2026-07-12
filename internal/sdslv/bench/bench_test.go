package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverListAndStableIdentity(t *testing.T) {
	p := filepath.Join(t.TempDir(), "basic.sdslvbench")
	if err := os.WriteFile(p, []byte("[Benchmark]\n[DispatchGroups(2, 1, 1)]\nfn One() -> void { return; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Discover(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Discover(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Benchmarks) != 1 || a.Benchmarks[0].ID == "" || a.Benchmarks[0].ID != b.Benchmarks[0].ID {
		t.Fatalf("unstable benchmarks: %#v %#v", a, b)
	}
	if a.Benchmarks[0].Warmup != 10 || a.Benchmarks[0].Iterations != 100 {
		t.Fatalf("defaults: %#v", a.Benchmarks[0])
	}
}
func TestStatisticsFor(t *testing.T) {
	got := StatisticsFor([]uint64{9, 1, 5, 3})
	if got.Count != 4 || got.Min != 1 || got.Median != 3 || got.Max != 9 || got.Mean != 4 {
		t.Fatalf("statistics=%#v", got)
	}
}
func TestRejectsTestExtension(t *testing.T) {
	_, err := Discover("wrong.sdslvtest")
	if err == nil || !strings.Contains(err.Error(), ".sdslvbench") {
		t.Fatalf("err=%v", err)
	}
}
