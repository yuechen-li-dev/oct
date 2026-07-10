package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSdslvTestParsesFactTheoryAndLaunchMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.sdslvtest")
	src := "[Fact]\nfn One() -> void {}\n[Theory]\n[InlineData(1.0, 1u)]\n[InlineData(2.0, 2u)]\n[WorkgroupSize(32, 1, 1)]\n[DispatchGroups(2, 1, 1)]\nfn Rows(value: f32, expected: u32) -> void {}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Cases) != 3 {
		t.Fatalf("cases=%d", len(m.Cases))
	}
	if m.Cases[0].StableID == "" {
		t.Fatal("missing stable ID")
	}
	for _, c := range m.Cases {
		if c.Function == "Rows" && c.Launch.WorkgroupSize[0] != 32 {
			t.Fatalf("launch=%v", c.Launch)
		}
	}
}

func TestSdslvTestRejectsInvalidTheoryShapes(t *testing.T) {
	for _, src := range []string{"[Theory]\nfn Missing(x: u32) -> void {}\n", "[Fact]\n[InlineData(1u)]\nfn Bad() -> void {}\n", "[Theory]\n[InlineData(1u)]\nfn Bad(x: f32) -> void {}\n"} {
		path := filepath.Join(t.TempDir(), "bad.sdslvtest")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Discover(path); err == nil {
			t.Fatalf("expected discovery rejection for %q", src)
		}
	}
}

func TestSdslvTestManifestIsDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.sdslvtest")
	if err := os.WriteFile(path, []byte("[Fact]\nfn One() -> void {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cases[0].StableID != b.Cases[0].StableID {
		t.Fatal("stable ID changed")
	}
}

func TestSdslvTestCompilesOneModulePerWorkgroupSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.sdslvtest")
	src := "[Fact]\nfn InlineHlslAsUint() -> void {}\n[Theory]\n[InlineData(1.0, 1065353216u)]\nfn FloatBitPattern(value: f32, expected: u32) -> void {}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Compile(m, filepath.Join(t.TempDir(), "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups=%d", len(groups))
	}
	if _, err := os.Stat(groups[0].SPIRVPath); err != nil {
		t.Fatalf("SPIR-V missing: %v", err)
	}
}
