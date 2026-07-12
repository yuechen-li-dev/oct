package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
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

func TestM36BCanonicalArtifactsRegenerateByteForByte(t *testing.T) {
	if _, err := exec.LookPath("dxc"); err != nil {
		t.Skip("dxc not available:", err)
	}
	if _, err := exec.LookPath("spirv-val"); err != nil {
		t.Skip("spirv-val not available:", err)
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()
	root := filepath.Join("examples", "SDSL-V", "M36a")
	committed, err := os.ReadFile(filepath.Join(root, "artifacts", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want CanonicalManifest
	if err := json.Unmarshal(committed, &want); err != nil {
		t.Fatal(err)
	}
	got, err := GenerateCanonicalArtifacts(filepath.Join(root, "BasicBenchmarks.sdslvbench"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artifacts) != len(want.Artifacts) {
		t.Fatalf("artifact count got %d want %d", len(got.Artifacts), len(want.Artifacts))
	}
	for i, artifact := range got.Artifacts {
		baseline := want.Artifacts[i]
		if artifact.Name != baseline.Name || artifact.BenchmarkID != baseline.BenchmarkID || artifact.SPIRVSHA256 != baseline.SPIRVSHA256 || artifact.SPIRVBytes != baseline.SPIRVBytes {
			t.Fatalf("canonical artifact drift\ngot:  %#v\nwant: %#v", artifact, baseline)
		}
		b, err := os.ReadFile(filepath.Join(root, "artifacts", filepath.Base(baseline.SPIRVPath)))
		if err != nil {
			t.Fatal(err)
		}
		s := sha256.Sum256(b)
		if hex.EncodeToString(s[:]) != baseline.SPIRVSHA256 {
			t.Fatalf("committed %s hash drift", baseline.SPIRVPath)
		}
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
