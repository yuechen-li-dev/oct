package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestSdslvStableIdsRemainUnchangedForM29Fixtures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	m, err := Discover(filepath.Join("examples", "SDSL-V", "M29", "InlineHlslFacts.sdslvtest"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range m.Cases {
		got = append(got, c.StableID)
	}
	want := []string{"sdslv-11e3deb3d1ad94f0071f3d8d", "sdslv-5664efcb0ab3deb7eb8c871b", "sdslv-a20bf18c1aa6672e75d2b267", "sdslv-ea0387cf37037ceec9e4083d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stable IDs changed: got %v, want %v", got, want)
	}
}

func TestSdslvExecutionPathHasNoSourceScannerOrRegexDiscovery(t *testing.T) {
	data, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"regexp.", "factNames(", "stripFactAttributes(", "usesM29Syntax("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("execution path retains source discovery helper %q", forbidden)
		}
	}
}

func TestSdslvGroupingAndManifestAreCanonicalProjections(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	suite, err := Prepare(filepath.Join("examples", "SDSL-V", "M29", "InlineHlslFacts.sdslvtest"))
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.Groups) != 2 || len(suite.Groups[0].Cases) == 0 {
		t.Fatalf("groups=%#v", suite.Groups)
	}
	// Theory rows remain separate replay cases but share their workgroup group.
	if suite.Groups[0].Cases[0].Test.Decl.Launch.WorkgroupSize != suite.Groups[0].Cases[1].Test.Decl.Launch.WorkgroupSize {
		t.Fatal("group did not consume canonical launch metadata")
	}
	m := ProjectManifest(suite, nil)
	var theory *Case
	for i := range m.Cases {
		if m.Cases[i].Function == "FloatBitPattern" {
			theory = &m.Cases[i]
			break
		}
	}
	if theory == nil || theory.RowSpan == nil || len(theory.TypedValues) != 2 || len(theory.ValueSpans) != 2 || len(theory.Assertions) != 1 || theory.GroupID == "" || !theory.FunctionSpan.Known() {
		t.Fatalf("manifest lost canonical metadata: %#v", theory)
	}
	a, _ := json.Marshal(m)
	b, _ := json.Marshal(ProjectManifest(suite, nil))
	if string(a) != string(b) {
		t.Fatal("manifest projection is not deterministic")
	}
}

func TestSdslvBootstrapCompilerDoesNotConsumeManifest(t *testing.T) {
	data, err := os.ReadFile("compile.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"Compile(manifest", "Manifest"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("bootstrap compiler retains manifest DTO coupling %q", forbidden)
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
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Compile(suite, filepath.Join(t.TempDir(), "artifacts"))
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

func TestSdslvTestBodiesUseNormalVDMIRAndNoFixtureEmitter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "real.sdslvtest")
	src := "[Theory]\n[InlineData(1u)]\n[InlineData(2u)]\nfn Rows(v: u32) -> void { let plus: u32 = v + 1u; Assert.Equal(plus, v + 1u); Assert.NotEqual(v, 0u); }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(suite.MIR.Functions) != 1 {
		t.Fatalf("normal lowering did not retain test function: %#v", suite.MIR.Functions)
	}
	groups, err := Compile(suite, filepath.Join(t.TempDir(), "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	hlsl, err := os.ReadFile(groups[0].HLSLPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(hlsl)
	for _, want := range []string{"uint plus = (v + 1u);", "Rows(1u, failure);", "Rows(2u, failure);", "__sdslv_sdslv_once_0", "if(!failure.failed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("real VD-MIR HLSL missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"InlineHlslAsUint", "FloatBitPattern", "ExplicitLaunchMetadata", "strings.Contains(c.Decl.Function.Name"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generated HLSL retains fixture behavior %q", forbidden)
		}
	}
}

func TestSdslvHlslAssertionEmissionConsumesDedicatedVDMIR(t *testing.T) {
	data, err := os.ReadFile("compile.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"assertCall(", "ValidatedAssertCall", "Assert.*"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("test emitter retains assertion rediscovery %q", forbidden)
		}
	}
	for _, forbidden := range []string{"emitStmt", "emitAssert", "func (e *testEmitter) value"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("test orchestration retains ordinary emitter %q", forbidden)
		}
	}
}

func TestSdslvBuildTestProgramKeepsBackendNeutralExecutionData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rows.sdslvtest")
	if err := os.WriteFile(path, []byte("[Theory]\n[InlineData(true, 7u, 1.5)]\nfn Rows(a: bool, b: u32, c: f32) -> void { Assert.True(a); }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	p := BuildTestProgram(suite)
	if p.ABI.ABIVersion != 1 || !p.ABI.LinearIndex.UsesXYZ || len(p.Groups) != 1 || len(p.Groups[0].Entries) != 1 {
		t.Fatalf("program=%#v", p)
	}
	row := p.Groups[0].Entries[0].TheoryRow
	if row == nil || len(row.Values) != 3 || row.Values[0].Type().Kind != "bool" || row.Values[1].Type().Kind != "u32" || row.Values[2].Type().Kind != "f32" {
		t.Fatalf("row=%#v", row)
	}
}
