package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
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
	for _, want := range []string{"uint plus = (v + 1u);", "Rows(1u, dispatch_id, failure);", "Rows(2u, dispatch_id, failure);", "__sdslv_sdslv_once_0", "if(!failure.failed"} {
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

func TestSdslvAssertOperandEmissionMaterializesOnceInOrder(t *testing.T) {
	root := repoRoot(t)
	suite, err := Prepare(filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "ExactlyOnceOperands.sdslvvalid"))
	if err != nil {
		t.Fatal(err)
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
	for _, forbidden := range []string{
		"inline HLSL expressions require a statement-valued context",
		"unsupported guarded read expr position",
		"return;",
		"discard",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("assert operand emission retained forbidden path %q:\n%s", forbidden, text)
		}
	}
	expectedBeforeActual := strings.Index(text, "ExactlyOnceOperands.sdslvvalid:13")
	actualAfterExpected := strings.Index(text, "ExactlyOnceOperands.sdslvvalid:16")
	nearExpected := strings.Index(text, "ExactlyOnceOperands.sdslvvalid:27")
	nearActual := strings.Index(text, "ExactlyOnceOperands.sdslvvalid:30")
	nearTolerance := strings.Index(text, "ExactlyOnceOperands.sdslvvalid:33")
	if expectedBeforeActual < 0 || actualAfterExpected < 0 || nearExpected < 0 || nearActual < 0 || nearTolerance < 0 {
		t.Fatalf("missing assert operand temps:\n%s", text)
	}
	if !(expectedBeforeActual < actualAfterExpected && nearExpected < nearActual && nearActual < nearTolerance) {
		t.Fatalf("assert operand temp order changed")
	}
	for _, want := range []string{
		"uint __sdslv_guarded_read_",
		"__sdslv_guarded_read_",
		"= __sdslv_guarded_read_",
		"__sdslv_test_input[index]",
		"AssertOperandsEvaluateExactlyOnce(dispatch_id, failure);",
		"TheoryAndResourceOperandsEvaluateOnce(0u, 10u, dispatch_id, failure);",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated HLSL missing %q:\n%s", want, text)
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

func TestSdslvTestInputManifestAndStableIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.sdslvtest")
	src := "[Fact]\n[TestInputUInt(7u, 11u)]\nfn Guarded() -> void { let index: u32 = 1u; let valid: bool = index < TestInput.Length; let value: u32 = read TestInput.UInt[index] when valid else 99u; Assert.Equal(11u, value); }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Cases) != 1 || manifest.Cases[0].TestInput.Kind != validate.TestInputKindUInt || manifest.Cases[0].TestInputBinding != 1 || manifest.Cases[0].TestInput.PayloadWords[1] != 11 {
		t.Fatalf("manifest case = %#v", manifest.Cases)
	}
	original := manifest.Cases[0].StableID
	changed := strings.Replace(src, "11u", "13u", 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest2, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest2.Cases[0].StableID != original {
		t.Fatalf("stable id changed with payload edit: %s vs %s", original, manifest2.Cases[0].StableID)
	}
}

func TestSdslvTestInputManifestSerializationUsesCanonicalFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.sdslvtest")
	src := "[Fact]\n[TestInputBool(true, false)]\nfn Reads() -> void { let value: bool = read TestInput.Bool[0u] when 0u < TestInput.Length else false; Assert.True(value); }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"test_input_binding":1`, `"abi_version":1`, `"kind":"bool"`, `"element_count":2`, `"payload_words":[1,0]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest json missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{`"AttributeSpan"`, `"ValueSpans"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("manifest json leaked validator span metadata %q:\n%s", forbidden, text)
		}
	}
}

func TestSdslvTestInputCompilesThroughSharedHLSLResource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.sdslvtest")
	src := "[Fact]\n[TestInputFloat(-0.0, 1.5)]\nfn ReadInput() -> void { let x: f32 = read TestInput.Float[1u] when 1u < TestInput.Length else 0.0; Assert.Near(1.5, x, 0.0); }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	p := BuildTestProgram(suite)
	if len(p.Groups) != 1 || len(p.Groups[0].Entries) != 1 || p.Groups[0].Entries[0].Input.ValueKind != vdmir.TestInputValueFloat || p.Groups[0].Entries[0].Input.ElementCount != 2 {
		t.Fatalf("program = %#v", p)
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
	for _, want := range []string{
		"[[vk::binding(1, 0)]] StructuredBuffer<uint> __sdslv_test_input;",
		"asfloat(__sdslv_test_input[1u])",
		"(1u < 2u)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated HLSL missing %q:\n%s", want, text)
		}
	}
}
