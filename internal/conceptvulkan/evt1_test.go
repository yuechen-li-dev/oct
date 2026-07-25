package conceptvulkan

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readEVT1Fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "Concept-Vulkan", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseEVT1SpecimensAndGenerateDeterministically(t *testing.T) {
	for _, name := range []string{
		"evt1_m1a_language.concept",
		"evt1_m1a_vulkan.concept",
		"evt1_m1b_a_language.concept",
		"evt1_m1b_a_vulkan.concept",
		"evt1_m1b_b_language.concept",
		"evt1_m1b_b_vulkan.concept",
		"evt1_m1b_c_language.concept",
		"evt1_m1b_c_vulkan.concept",
		"evt1_m1b_d_language.concept",
		"evt1_m1b_d_vulkan.concept",
		"evt1_dragongod_m0_language.concept",
		"evt1_dragongod_m0_vulkan.concept",
		"evt1_dragongod_m1_language.concept",
		"evt1_dragongod_m1_vulkan.concept",
		"evt1_dragongod_m2_language.concept",
		"evt1_dragongod_m2_vulkan.concept",
		"evt1_dragongod_m3_language.concept",
		"evt1_dragongod_m3_vulkan.concept",
	} {
		t.Run(name, func(t *testing.T) {
			src := readEVT1Fixture(t, name)
			module, err := ParseEVT1(filepath.ToSlash(filepath.Join("examples", "Concept-Vulkan", name)), src)
			if err != nil {
				t.Fatal(err)
			}
			if len(module.Functions) == 0 {
				t.Fatalf("unexpected module: %#v", module)
			}
			outA, err := GenerateEVT1(module, []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			outB, err := GenerateEVT1(module, []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			for key, want := range outA {
				if string(want) != string(outB[key]) {
					t.Fatalf("nondeterministic output %s", key)
				}
			}
			env, err := analyzeEVT1Module(module)
			if err != nil {
				t.Fatal(err)
			}
			mirText := MIRTextEVT1(buildEVT1MIR(module, env))
			if strings.Contains(name, "m1a") && !strings.Contains(mirText, "match_expr") {
				t.Fatal("MIR text omitted match_expr")
			}
			if strings.Contains(name, "m1b_a") && !strings.Contains(mirText, "struct_construct") {
				t.Fatal("MIR text omitted struct_construct")
			}
			if strings.Contains(name, "m1b_b") && !strings.Contains(mirText, "requirement_call") {
				t.Fatal("MIR text omitted requirement_call")
			}
			if strings.Contains(name, "m1b_d") && !strings.Contains(mirText, "array_index") {
				t.Fatal("MIR text omitted array_index")
			}
			if strings.Contains(name, "dragongod") && !strings.Contains(mirText, "automata") {
				t.Fatal("MIR text omitted automata")
			}
			for _, suffix := range []string{".mir.json", ".map.json", ".manifest.json"} {
				found := false
				for key, body := range outA {
					if strings.HasSuffix(key, suffix) {
						found = true
						var decoded any
						if err := json.Unmarshal(body, &decoded); err != nil {
							t.Fatalf("%s: %v", key, err)
						}
					}
				}
				if !found {
					t.Fatalf("missing %s output", suffix)
				}
			}
		})
	}
}

func TestEVT1CheckedOutputsMatch(t *testing.T) {
	for _, name := range []string{
		"evt1_m1a_language.concept",
		"evt1_m1a_vulkan.concept",
		"evt1_m1b_a_language.concept",
		"evt1_m1b_a_vulkan.concept",
		"evt1_m1b_b_language.concept",
		"evt1_m1b_b_vulkan.concept",
		"evt1_m1b_c_language.concept",
		"evt1_m1b_c_vulkan.concept",
		"evt1_m1b_d_language.concept",
		"evt1_m1b_d_vulkan.concept",
		"evt1_dragongod_m0_language.concept",
		"evt1_dragongod_m0_vulkan.concept",
		"evt1_dragongod_m1_language.concept",
		"evt1_dragongod_m1_vulkan.concept",
		"evt1_dragongod_m2_language.concept",
		"evt1_dragongod_m2_vulkan.concept",
		"evt1_dragongod_m3_language.concept",
		"evt1_dragongod_m3_vulkan.concept",
	} {
		t.Run(name, func(t *testing.T) {
			src := readEVT1Fixture(t, name)
			module, err := ParseEVT1(filepath.ToSlash(filepath.Join("examples", "Concept-Vulkan", name)), src)
			if err != nil {
				t.Fatal(err)
			}
			outputs, err := GenerateEVT1(module, []byte(src))
			if err != nil {
				t.Fatal(err)
			}
			if err := Check(filepath.Join("generated"), outputs); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEVT1DoubleGenerationMatchesAcrossDirectories(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_b_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_b_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputsA, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	outputsB, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := Write(dirA, outputsA); err != nil {
		t.Fatal(err)
	}
	if err := Write(dirB, outputsB); err != nil {
		t.Fatal(err)
	}
	for name := range outputsA {
		a, err := os.ReadFile(filepath.Join(dirA, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Fatalf("directory generation mismatch for %s", name)
		}
	}
}

func TestEVT1CGenerationUsesTransparentStructsAndNoConceptRuntime(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_b_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_b_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_m1b_b_language.generated.h"])
	body := string(outputs["evt1_m1b_b_language.generated.c"])
	for _, needle := range []string{
		"typedef struct concept_vulkan_buffer_range {\n  int bufferId;\n  int offset;\n  int size;\n}",
		"typedef struct concept_vulkan_pipeline_state {\n  int handle;\n  bool alive;\n}",
	} {
		if !strings.Contains(header, needle) {
			t.Fatalf("header missing %q\n%s", needle, header)
		}
	}
	for _, needle := range []string{
		"static int concept_vulkan_template_score_resource__buffer_range(",
		"static void concept_vulkan_template_destroy_resource__pipeline_state(",
		"concept_vulkan_evt1_m1b_b_language_measure__buffer_range",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("body missing %q\n%s", needle, body)
		}
	}
	for _, forbidden := range []string{"template <", "VulkanResource", "Measurable", "Destroyable"} {
		if strings.Contains(header, forbidden) || strings.Contains(body, forbidden) {
			t.Fatalf("concept runtime name %q leaked into generated C", forbidden)
		}
	}
}

func TestEVT1DiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "duplicate fields",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; int offset; };\n",
			code: "CV4124",
		},
		{
			name: "wrong initializer count",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; int size; };\nBufferRange Make() { BufferRange range = BufferRange{1}; return range; }\n",
			code: "CV4126",
		},
		{
			name: "wrong initializer type",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; bool ok; };\nBufferRange Make() { BufferRange range = BufferRange{1, 2}; return range; }\n",
			code: "CV4107",
		},
		{
			name: "unknown field",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nint Read(BufferRange range) { return range.missing; }\n",
			code: "CV4026",
		},
		{
			name: "const borrow mutation",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nvoid Mutate(borrow const BufferRange range) { range.offset = 1; }\n",
			code: "CV4128",
		},
		{
			name: "ownership illegal copy",
			src:  "profile Vulkan;\nstruct HandleBox { owned Pipeline pipeline; };\nPipeline Acquire();\nvoid Use() { HandleBox first = HandleBox{Acquire()}; HandleBox second = first; }\n",
			code: "CV4133",
		},
		{
			name: "immovable copy",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nvoid Use() { PoolState first = PoolState{1, false}; PoolState second = first; }\n",
			code: "CV4134",
		},
		{
			name: "immovable whole value assignment",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nvoid Use() { PoolState first = PoolState{1, false}; PoolState second = PoolState{2, false}; second = first; }\n",
			code: "CV4135",
		},
		{
			name: "immovable by value parameter",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nvoid Use(PoolState state);\n",
			code: "CV4136",
		},
		{
			name: "immovable by value return",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nPoolState Use();\n",
			code: "CV4137",
		},
		{
			name: "immovable embedding",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nstruct Wrapper { PoolState state; };\n",
			code: "CV4138",
		},
		{
			name: "immovable enum payload",
			src:  "profile Vulkan;\nimmovable struct PoolState { int id; bool ok; };\nenum Event { Ready(PoolState state) }\n",
			code: "CV4139",
		},
		{
			name: "unknown concept",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nrequires Missing<BufferRange>;\n",
			code: "CV4151",
		},
		{
			name: "unknown prerequisite",
			src:  "profile Vulkan;\nconcept UsesMissing<T> { requires Missing<T>; }\n",
			code: "CV4152",
		},
		{
			name: "missing required operation",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nrequires Validatable<BufferRange>;\n",
			code: "CV4153",
		},
		{
			name: "wrong operation parameter type",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nbool IsValid(int value);\nrequires Validatable<BufferRange>;\n",
			code: "CV4154",
		},
		{
			name: "wrong operation const qualifier",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nbool IsValid(borrow BufferRange value);\nrequires Validatable<BufferRange>;\n",
			code: "CV4155",
		},
		{
			name: "wrong operation return type",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nint IsValid(borrow const BufferRange value);\nrequires Validatable<BufferRange>;\n",
			code: "CV4156",
		},
		{
			name: "failed nested prerequisite",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nconcept ResourceState<T> { requires Validatable<T>; }\nrequires ResourceState<BufferRange>;\n",
			code: "CV4153",
		},
		{
			name: "direct concept cycle",
			src:  "profile Vulkan;\nconcept A<T> { requires A<T>; }\n",
			code: "CV4162",
		},
		{
			name: "indirect concept cycle",
			src:  "profile Vulkan;\nconcept A<T> { requires B<T>; }\nconcept B<T> { requires A<T>; }\n",
			code: "CV4162",
		},
		{
			name: "concept used as runtime type",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Validatable<T> { requires bool IsValid(borrow const T value); }\nValidatable<BufferRange> Make();\n",
			code: "CV4164",
		},
		{
			name: "constrained template rejected",
			src:  "profile Vulkan;\ntemplate <typename T>\nint Identity(T value);\n",
			code: "CV4166",
		},
		{
			name: "m1a non exhaustive regression",
			src:  "profile Vulkan;\nenum Status { Empty, Ready(int value) }\nint Match(Status value) { return match (value) { Status::Empty => 0, }; }\n",
			code: "CV4115",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestEVT1TemplateDiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "constraint uses concrete type",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\ntemplate <typename T>\nrequires Resource<BufferRange>\nint Score(borrow const T value) { return Measure(value); }\n",
			code: "CV4170",
		},
		{
			name: "dependent field access",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const BufferRange value);\ntemplate <typename T>\nrequires Resource<T>\nint Score(borrow const T value) { return value.offset; }\n",
			code: "CV4172",
		},
		{
			name: "explicit template call required",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const BufferRange value);\nrequires Resource<BufferRange>;\ntemplate <typename T>\nrequires Resource<T>\nint Score(borrow const T value) { return Measure(value); }\nint Use() { BufferRange range = BufferRange{1}; return Score(range); }\n",
			code: "CV4173",
		},
		{
			name: "nested template call rejected",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const BufferRange value);\nrequires Resource<BufferRange>;\ntemplate <typename T>\nrequires Resource<T>\nint Score(borrow const T value) { return Measure(value); }\ntemplate <typename T>\nrequires Resource<T>\nint Forward(borrow const T value) { return Score<T>(value); }\n",
			code: "CV4174",
		},
		{
			name: "dependent operator rejected",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const BufferRange value);\ntemplate <typename T>\nrequires Resource<T>\nbool Larger(borrow const T left, borrow const T right) { return left > right; }\n",
			code: "CV4175",
		},
		{
			name: "call not guaranteed by constraint",
			src:  "profile Vulkan;\nstruct BufferRange { int offset; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const BufferRange value);\ntemplate <typename T>\nrequires Resource<T>\nint Score(borrow const T value) { return Destroy(value); }\n",
			code: "CV4176",
		},
		{
			name: "by value immovable instantiation rejected",
			src:  "profile Vulkan;\nimmovable struct PipelineState { int handle; bool alive; };\nconcept Resource<T> { requires int Measure(borrow const T value); }\nint Measure(borrow const PipelineState value);\nrequires Resource<PipelineState>;\ntemplate <typename T>\nrequires Resource<T>\nint CopyResource(T value) { return Measure(value); }\nint Use() { PipelineState state = PipelineState{1, true}; return CopyResource<PipelineState>(state); }\n",
			code: "CV4136",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestEVT1TemplateInvocationPreservesComparisonParsing(t *testing.T) {
	src := `profile Vulkan;
struct BufferRange { int offset; };
concept Resource<T> { requires int Measure(borrow const T value); }
int Measure(borrow const BufferRange value);
requires Resource<BufferRange>;
template <typename T>
requires Resource<T>
int Score(borrow const T value) { return Measure(value); }
bool Less() { return 1 < 2; }
bool GreaterFromTemplate() { BufferRange range = BufferRange{1}; return Score<BufferRange>(range) > 0; }
bool Chained() { return 3 > 2 > 1; }
`
	_, err := ParseEVT1("test.concept", src)
	if err == nil || !strings.Contains(err.Error(), "CV4028") {
		t.Fatalf("err=%v want CV4028 type-check failure from chained comparison", err)
	}
}

func TestEVT1M1BCDiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "if requires else",
			src:  "profile Vulkan;\nint Use(bool flag) { return if (flag) 1; }\n",
			code: "CV4184",
		},
		{
			name: "else if rejected",
			src:  "profile Vulkan;\nint Use(bool a, bool b) { return if (a) 1 else if (b) 2 else 3; }\n",
			code: "CV4185",
		},
		{
			name: "runtime cannot call comptime function",
			src:  "profile Vulkan;\ncomptime int Bound(int value) { return value; }\nint Use() { return Bound(1); }\n",
			code: "CV4210",
		},
		{
			name: "runtime value rejected in comptime local",
			src:  "profile Vulkan;\nint Use(int value) { comptime int Local = value; return Local; }\n",
			code: "CV4200",
		},
		{
			name: "comptime while must be bounded",
			src:  "profile Vulkan;\ncomptime int Sum(int limit) { int cursor = 0; while (cursor < limit) { cursor = cursor + 1; } return cursor; }\nstatic_assert(Sum(1) == 1);\n",
			code: "CV4205",
		},
		{
			name: "bounded while requires comptime int",
			src:  "profile Vulkan;\nint Use(int limit) { int cursor = 0; while (cursor < limit) bounded(limit) { cursor = cursor + 1; } return cursor; }\n",
			code: "CV4200",
		},
		{
			name: "comptime recursion rejected",
			src:  "profile Vulkan;\ncomptime int First(int value) { return Second(value); }\ncomptime int Second(int value) { return First(value); }\nstatic_assert(First(1) == 1);\n",
			code: "CV4217",
		},
		{
			name: "static assert message must be string",
			src:  "profile Vulkan;\nstatic_assert(true, 1);\n",
			code: "CV4208",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestEVT1M1BDDiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "empty literal needs context",
			src:  "profile Vulkan;\ncomptime int Missing() { return Len([]); }\n",
			code: "CV4225",
		},
		{
			name: "heterogeneous literal rejected",
			src:  "profile Vulkan;\ncomptime int[2] Values = [1, true];\n",
			code: "CV4227",
		},
		{
			name: "runtime local array rejected",
			src:  "profile Vulkan;\nint Use() { int[2] values = [1, 2]; return 0; }\n",
			code: "CV4230",
		},
		{
			name: "non array index target",
			src:  "profile Vulkan;\ncomptime int Value = 3;\nstatic_assert(Value[0] == 0);\n",
			code: "CV4231",
		},
		{
			name: "index requires int",
			src:  "profile Vulkan;\ncomptime int[2] Values = [1, 2];\nstatic_assert(Values[true] == 1);\n",
			code: "CV4232",
		},
		{
			name: "index out of range",
			src:  "profile Vulkan;\ncomptime int[2] Values = [1, 2];\nstatic_assert(Values[2] == 0);\n",
			code: "CV4233",
		},
		{
			name: "len arity",
			src:  "profile Vulkan;\ncomptime int[2] Values = [1, 2];\nstatic_assert(Len(Values, Values) == 2);\n",
			code: "CV4234",
		},
		{
			name: "len requires array",
			src:  "profile Vulkan;\ncomptime int Value = 1;\nstatic_assert(Len(Value) == 1);\n",
			code: "CV4235",
		},
		{
			name: "array ordering rejected",
			src:  "profile Vulkan;\ncomptime int[2] Left = [1, 2];\ncomptime int[2] Right = [1, 3];\nstatic_assert(Left < Right);\n",
			code: "CV4236",
		},
		{
			name: "for loop rejected",
			src:  "profile Vulkan;\nint Use() { for (1) { } return 0; }\n",
			code: "CV4237",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestEVT1M1BCGenerationErasesComptimeRuntime(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_c_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_c_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_m1b_c_language.generated.h"])
	body := string(outputs["evt1_m1b_c_language.generated.c"])
	if strings.Contains(header, "ClampCount") || strings.Contains(body, "ClampCount(") || strings.Contains(body, "SumTo(") {
		t.Fatalf("comptime functions leaked into generated runtime output:\n%s", body)
	}
	for _, needle := range []string{"while (", "if (", "return 3;"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("generated C missing %q\n%s", needle, body)
		}
	}
}

func TestEVT1M1BDGenerationErasesComptimeRuntime(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_d_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_d_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_m1b_d_language.generated.h"])
	body := string(outputs["evt1_m1b_d_language.generated.c"])
	for _, forbidden := range []string{"CanonicalRetryBudgets(", "SumBudgets(", "HasDuplicateTransitionKeys(", "Len("} {
		if strings.Contains(header, forbidden) || strings.Contains(body, forbidden) {
			t.Fatalf("comptime array helper leaked into generated runtime output: %s", forbidden)
		}
	}
	for _, needle := range []string{"return 2;", "return 6;", "return 7;"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("generated C missing %q\n%s", needle, body)
		}
	}
}

func TestDragonGodM0GenerationErasesAutomataRuntime(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_dragongod_m0_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m0_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_dragongod_m0_language.generated.h"])
	body := string(outputs["evt1_dragongod_m0_language.generated.c"])
	for _, forbidden := range []string{"ResourceLifecycle", "SweepMachine", "AwaitingResume", "Cleanup", "LifecycleSignal"} {
		if strings.Contains(header, forbidden) || strings.Contains(body, forbidden) {
			t.Fatalf("automata-only symbol leaked into generated runtime output: %s", forbidden)
		}
	}
}

func TestDragonGodM1GenerationIncludesRuntimeDispatch(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_dragongod_m1_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m1_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_dragongod_m1_language.generated.h"])
	body := string(outputs["evt1_dragongod_m1_language.generated.c"])
	for _, needle := range []string{
		"concept_vulkan_automata_dispatch_outcome",
		"concept_vulkan_resource_lifecycle_instance",
		"concept_vulkan_resource_lifecycle_dispatch(",
		"concept_vulkan_resource_lifecycle_normalize(",
		"concept_vulkan_single_step_lifecycle_instance",
	} {
		if !strings.Contains(header, needle) && !strings.Contains(body, needle) {
			t.Fatalf("generated output missing %q\nHEADER:\n%s\nBODY:\n%s", needle, header, body)
		}
	}
}

func TestDragonGodM2GenerationIncludesGuardedDispatch(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_dragongod_m2_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m2_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_dragongod_m2_language.generated.h"])
	body := string(outputs["evt1_dragongod_m2_language.generated.c"])
	for _, needle := range []string{
		"concept_vulkan_guarded_lifecycle_instance",
		"eligible_count",
		"selected_candidate",
		"instance->context",
		"AMBIGUOUS",
	} {
		if !strings.Contains(header, needle) && !strings.Contains(body, needle) {
			t.Fatalf("generated output missing %q\nHEADER:\n%s\nBODY:\n%s", needle, header, body)
		}
	}
}

func TestDragonGodM3GenerationIncludesEffectBatchStaging(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_dragongod_m3_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m3_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	body := string(outputs["evt1_dragongod_m3_language.generated.c"])
	for _, needle := range []string{
		"concept_vulkan_resource_lifecycle_effects",
		"staged_batch = {0};",
		"staged_instance = *instance;",
		"*instance = staged_instance;",
		"staged_batch.count = staged_count;",
		"*batch = staged_batch;",
		"batch->count = 0;",
		"CONCEPT_VULKAN_RESOURCE_LIFECYCLE_EFFECT_RECORD_SUBMISSION",
		"CONCEPT_VULKAN_RESOURCE_LIFECYCLE_EFFECT_BEGIN_SUBMISSION",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("generated C missing %q\n%s", needle, body)
		}
	}
}

func TestDragonGodM0IdentityIgnoresWhitespaceAndLocation(t *testing.T) {
	left := `profile Vulkan;
enum Signal
{
    Go,
    Stop
}
automata Demo(Signal)
{
    initial machine Main
    {
        initial state Start
        {
            on Signal::Go goto Done;
        }
        terminal state Done
        {
            finish;
        }
    }
}
int Value() { return 1; }
`
	right := `profile Vulkan;

enum Signal { Go, Stop }

automata Demo(Signal)
{
    initial machine Main
    {

        initial state Start
        {
            on Signal::Go goto Done;
        }

        terminal state Done
        {
            finish;
        }
    }
}

int Value()
{
    return 1;
}
`
	leftModule, err := ParseEVT1("left.concept", left)
	if err != nil {
		t.Fatal(err)
	}
	rightModule, err := ParseEVT1("right.concept", right)
	if err != nil {
		t.Fatal(err)
	}
	leftEnv, err := analyzeEVT1Module(leftModule)
	if err != nil {
		t.Fatal(err)
	}
	rightEnv, err := analyzeEVT1Module(rightModule)
	if err != nil {
		t.Fatal(err)
	}
	leftID := buildEVT1MIR(leftModule, leftEnv).Automata[0].GraphIdentity
	rightID := buildEVT1MIR(rightModule, rightEnv).Automata[0].GraphIdentity
	if leftID != rightID {
		t.Fatalf("graph identity changed across whitespace/location-only edits: %s != %s", leftID, rightID)
	}
}

func TestDragonGodM3EffectIdentityTracksEmitOrderWithoutChangingTopology(t *testing.T) {
	left := `profile Vulkan;
enum Signal { Go }
effect First(int value);
effect Second(int value);
automata Demo(Signal) {
  initial machine Main {
    initial state Start {
      on Signal::Go => {
        emit First(1);
        emit Second(2);
        goto Done;
      }
    }
    terminal state Done { finish; }
  }
}
int Value() { return 1; }
`
	right := `profile Vulkan;
enum Signal { Go }
effect First(int value);
effect Second(int value);
automata Demo(Signal) {
  initial machine Main {
    initial state Start {
      on Signal::Go => {
        emit Second(2);
        emit First(1);
        goto Done;
      }
    }
    terminal state Done { finish; }
  }
}
int Value() { return 1; }
`
	leftModule, err := ParseEVT1("left.concept", left)
	if err != nil {
		t.Fatal(err)
	}
	rightModule, err := ParseEVT1("right.concept", right)
	if err != nil {
		t.Fatal(err)
	}
	leftEnv, err := analyzeEVT1Module(leftModule)
	if err != nil {
		t.Fatal(err)
	}
	rightEnv, err := analyzeEVT1Module(rightModule)
	if err != nil {
		t.Fatal(err)
	}
	leftAutomata := buildEVT1MIR(leftModule, leftEnv).Automata[0]
	rightAutomata := buildEVT1MIR(rightModule, rightEnv).Automata[0]
	if leftAutomata.TopologyIdentity != rightAutomata.TopologyIdentity {
		t.Fatalf("topology identity changed across emit reorder: %s != %s", leftAutomata.TopologyIdentity, rightAutomata.TopologyIdentity)
	}
	if leftAutomata.EffectIdentity == rightAutomata.EffectIdentity {
		t.Fatalf("effect identity ignored emit reorder: %s", leftAutomata.EffectIdentity)
	}
}

func TestDragonGodM0DiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "non enum signal type",
			src:  "profile Vulkan;\nstruct SignalSet { int value; };\nautomata Demo(SignalSet) { initial machine Main { initial state Start { on SignalSet::Value goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4241",
		},
		{
			name: "missing initial machine",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4243",
		},
		{
			name: "terminal modifier order",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { terminal initial state Start { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4248",
		},
		{
			name: "duplicate transition key",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; on Signal::Go goto Start; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4253",
		},
		{
			name: "cross machine goto",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Other::Done; } terminal state Done { finish; } } machine Other { initial terminal state Done { pop; } } }\nint Value() { return 1; }\n",
			code: "CV4254",
		},
		{
			name: "push continuation resolved in pushed machine",
			src:  "profile Vulkan;\nenum Signal { Go, Return }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go push Other goto Other::Done; } terminal state Done { finish; } } machine Other { initial state Begin { on Signal::Return goto Done; } terminal state Done { pop; } } }\nint Value() { return 1; }\n",
			code: "CV4256",
		},
		{
			name: "push cycle",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go push Other goto Done; } terminal state Done { finish; } } machine Other { initial state Begin { on Signal::Go push Main goto Begin; } terminal state Done { pop; } } }\nint Value() { return 1; }\n",
			code: "CV4257",
		},
		{
			name: "unreachable machine",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } machine Other { initial terminal state Begin { pop; } } }\nint Value() { return 1; }\n",
			code: "CV4258",
		},
		{
			name: "unreachable state",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } state Dead { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4259",
		},
		{
			name: "missing reachable pop",
			src:  "profile Vulkan;\nenum Signal { Go, Stop }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go push Other goto Done; } terminal state Done { finish; } } machine Other { initial terminal state Begin { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4260",
		},
		{
			name: "missing reachable finish",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Loop; } state Loop { on Signal::Go goto Loop; } } }\nint Value() { return 1; }\n",
			code: "CV4261",
		},
		{
			name: "root pop invalid",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial terminal state Start { pop; } } }\nint Value() { return 1; }\n",
			code: "CV4262",
		},
		{
			name: "automata first class expression",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return Demo; }\n",
			code: "CV4263",
		},
		{
			name: "ordinary statement inside state body",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { int value = 1; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4264",
		},
		{
			name: "machine count limit",
			src: "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) {\ninitial machine M0 { initial state S { on Signal::Go goto D; } terminal state D { finish; } }\nmachine M1 { initial terminal state S { pop; } }\nmachine M2 { initial terminal state S { pop; } }\nmachine M3 { initial terminal state S { pop; } }\nmachine M4 { initial terminal state S { pop; } }\nmachine M5 { initial terminal state S { pop; } }\nmachine M6 { initial terminal state S { pop; } }\nmachine M7 { initial terminal state S { pop; } }\nmachine M8 { initial terminal state S { pop; } }\nmachine M9 { initial terminal state S { pop; } }\nmachine M10 { initial terminal state S { pop; } }\nmachine M11 { initial terminal state S { pop; } }\nmachine M12 { initial terminal state S { pop; } }\nmachine M13 { initial terminal state S { pop; } }\nmachine M14 { initial terminal state S { pop; } }\nmachine M15 { initial terminal state S { pop; } }\nmachine M16 { initial terminal state S { pop; } }\n}\nint Value() { return 1; }\n",
			code: "CV4265",
		},
		{
			name: "compiler owned outcome redeclaration",
			src:  "profile Vulkan;\nenum AutomataDispatchOutcome { Value }\nint Value() { return 1; }\n",
			code: "CV4267",
		},
		{
			name: "signal payload variant rejected",
			src:  "profile Vulkan;\nenum Signal { Go(int payload) }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4269",
		},
		{
			name: "instance requires automata",
			src:  "profile Vulkan;\nenum Signal { Go }\nint Value() { instance Signal value; return 1; }\n",
			code: "CV4270",
		},
		{
			name: "ordinary use of instance rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { instance Demo value; return value; }\n",
			code: "CV4272",
		},
		{
			name: "dispatch first operand must be instance",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { int value = 1; dispatch(value, Signal::Go); return 1; }\n",
			code: "CV4273",
		},
		{
			name: "dispatch wrong signal enum",
			src:  "profile Vulkan;\nenum Left { Go }\nenum Right { Go }\nautomata Demo(Left) { initial machine Main { initial state Start { on Left::Go goto Done; } terminal state Done { finish; } } }\nint Value() { instance Demo value; dispatch(value, Right::Go); return 1; }\n",
			code: "CV4274",
		},
		{
			name: "dispatch name redeclaration rejected",
			src:  "profile Vulkan;\nint dispatch() { return 1; }\nint Value() { return 2; }\n",
			code: "CV4268",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestDragonGodM2DiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "context missing borrow",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, Context context) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4276",
		},
		{
			name: "duplicate context params",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow left: Context, borrow right: Context) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4278",
		},
		{
			name: "pointer context rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal, borrow context: int*) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4281",
		},
		{
			name: "instance missing required context",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { instance Demo value; return 1; }\n",
			code: "CV4283",
		},
		{
			name: "context argument supplied to contextless automata",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { Context context = Context{true}; instance Demo value(context); return 1; }\n",
			code: "CV4284",
		},
		{
			name: "guard must be bool",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go when 1 => goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4290",
		},
		{
			name: "ordinary plus guarded mix rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go goto Done; on Signal::Go when context.ready => goto Start; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4287",
		},
		{
			name: "otherwise must be last",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go otherwise => goto Done; on Signal::Go when context.ready => goto Start; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4288",
		},
		{
			name: "fallback only group rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go otherwise => goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4289",
		},
		{
			name: "retained context blocks mutation",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go when context.ready => goto Done; on Signal::Go otherwise => goto Start; } terminal state Done { finish; } } }\nint Value() { Context context = Context{true}; instance Demo value(context); context.ready = false; return 1; }\n",
			code: "CV4291",
		},
		{
			name: "recursive guard call rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nstruct Context { bool ready; };\nbool Loop(borrow const Context context) { return Loop(context); }\nautomata Demo(Signal, borrow context: Context) { initial machine Main { initial state Start { on Signal::Go when Loop(context) => goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4296",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestDragonGodM3DiagnosticsAreStable(t *testing.T) {
	cases := []struct {
		name string
		src  string
		code string
	}{
		{
			name: "string payload rejected",
			src:  "profile Vulkan;\neffect Bad(string text);\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4303",
		},
		{
			name: "unknown emitted effect rejected",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go => { emit Missing(); goto Done; } } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4301",
		},
		{
			name: "effectful dispatch requires batch",
			src:  "profile Vulkan;\neffect Mark(int value);\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go => { emit Mark(1); goto Done; } } terminal state Done { finish; } } }\nint Value() { instance Demo lifecycle; return 1 + dispatch(lifecycle, Signal::Go).tag; }\n",
			code: "CV4306",
		},
		{
			name: "batch ordinary use rejected",
			src:  "profile Vulkan;\neffect Mark(int value);\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go => { emit Mark(1); goto Done; } } terminal state Done { finish; } } }\nint Value() { instance Demo lifecycle; effects Demo emitted; dispatch(lifecycle, Signal::Go, emitted); return emitted; }\n",
			code: "CV4305",
		},
		{
			name: "effect free dispatch rejects third operand",
			src:  "profile Vulkan;\nenum Signal { Go }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go goto Done; } terminal state Done { finish; } } }\nint Value() { instance Demo lifecycle; effects Demo emitted; return 1 + dispatch(lifecycle, Signal::Go, emitted).tag; }\n",
			code: "CV4309",
		},
		{
			name: "payload call rejected",
			src:  "profile Vulkan;\neffect Mark(int value);\nenum Signal { Go }\nint ValueOf() { return 1; }\nautomata Demo(Signal) { initial machine Main { initial state Start { on Signal::Go => { emit Mark(ValueOf()); goto Done; } } terminal state Done { finish; } } }\nint Value() { return 1; }\n",
			code: "CV4312",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEVT1("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestEVT1CheckRejectsHandEdit(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_a_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_a_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := Write(dir, outputs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evt1_m1b_a_language.generated.c"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, outputs); err == nil || !strings.Contains(err.Error(), "CV3001") {
		t.Fatalf("check error=%v", err)
	}
}

func TestEVT1TemplateInstancesAreDeterministicAndDeduplicated(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1b_b_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_b_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	body := string(outputs["evt1_m1b_b_language.generated.c"])
	for _, needle := range []string{
		"static int concept_vulkan_template_score_resource__buffer_range(",
		"static int concept_vulkan_template_score_resource__pipeline_state(",
		"static void concept_vulkan_template_destroy_resource__buffer_range(",
		"static void concept_vulkan_template_destroy_resource__pipeline_state(",
	} {
		if strings.Count(body, needle) != 1 {
			t.Fatalf("expected one instance for %q\n%s", needle, body)
		}
	}
	env, err := analyzeEVT1Module(module)
	if err != nil {
		t.Fatal(err)
	}
	mir := buildEVT1MIR(module, env)
	if len(mir.Templates) != 2 {
		t.Fatalf("template count=%d", len(mir.Templates))
	}
	if len(mir.Instances) != 4 {
		t.Fatalf("instance count=%d", len(mir.Instances))
	}
}

func TestEVT1LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_a_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_a_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_a_language.generated.h"
#include <stdint.h>

static int g_next_value = 0;

bool concept_vulkan_evt1_m1b_a_language_is_valid(const concept_vulkan_command_pool_state* value) {
  return value->initialized;
}
void concept_vulkan_evt1_m1b_a_language_destroy(concept_vulkan_command_pool_state* value) {
  (void)value;
}
int concept_vulkan_evt1_m1b_a_language_next_value(void) {
  g_next_value += 1;
  return g_next_value;
}
int concept_vulkan_evt1_m1b_a_language_add(int left, int right) {
  return left + right;
}

int main(void) {
  concept_vulkan_observed_values observed = concept_vulkan_evt1_m1b_a_language_observe_construction();
  concept_vulkan_copy_result copy = concept_vulkan_evt1_m1b_a_language_copy_and_mutate();
  if (observed.first != 1 || observed.second != 2 || observed.third != 3) return 1;
  if (g_next_value != 3) return 2;
  if (copy.firstOffset != 1 || copy.secondOffset != 9) return 3;
  if (concept_vulkan_evt1_m1b_a_language_match_allocation() != 18) return 4;
  if (!concept_vulkan_evt1_m1b_a_language_use_immovable()) return 5;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_a_language_harness.c", harness, nil)
}

func TestEVT1VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_a_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_a_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_a_vulkan.generated.h"
#include <stdint.h>

static int g_destroy_calls = 0;

bool concept_vulkan_evt1_m1b_a_vulkan_is_valid(const concept_vulkan_command_pool_state* value) {
  return value->initialized;
}
void concept_vulkan_evt1_m1b_a_vulkan_destroy(concept_vulkan_command_pool_state* value) {
  (void)value;
  g_destroy_calls += 1;
}

int main(void) {
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_m1b_a_vulkan_classify_range(buffer) != 5) return 1;
  if (!concept_vulkan_evt1_m1b_a_vulkan_build_and_validate(pool)) return 2;
  concept_vulkan_evt1_m1b_a_vulkan_cleanup_state(pool);
  if (g_destroy_calls != 1) return 3;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_a_vulkan_harness.c", harness, nil)
}

func TestEVT1TemplateLanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_b_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_b_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_b_language.generated.h"
#include <stdint.h>

static int g_destroy_count = 0;
static int g_destroy_codes[4] = {0, 0, 0, 0};

int concept_vulkan_evt1_m1b_b_language_measure__buffer_range(const concept_vulkan_buffer_range* value) {
  return value->offset + value->size;
}
int concept_vulkan_evt1_m1b_b_language_measure__pipeline_state(const concept_vulkan_pipeline_state* value) {
  return value->handle + (value->alive ? 1 : 0);
}
void concept_vulkan_evt1_m1b_b_language_destroy__buffer_range(concept_vulkan_buffer_range* value) {
  g_destroy_codes[g_destroy_count++] = value->offset;
}
void concept_vulkan_evt1_m1b_b_language_destroy__pipeline_state(concept_vulkan_pipeline_state* value) {
  g_destroy_codes[g_destroy_count++] = value->handle;
}
void concept_vulkan_evt1_m1b_b_language_set_alive(concept_vulkan_pipeline_state* value) {
  value->alive = true;
}

int main(void) {
  concept_vulkan_destroy_audit audit;
  if (concept_vulkan_evt1_m1b_b_language_repeated_score() != 14) return 1;
  if (concept_vulkan_evt1_m1b_b_language_score_pipeline() != 12) return 2;
  audit = concept_vulkan_evt1_m1b_b_language_use_destroyers();
  if (audit.first != 1 || audit.second != 13 || !audit.third) return 3;
  if (!concept_vulkan_evt1_m1b_b_language_use_immovable()) return 4;
  if (!concept_vulkan_evt1_m1b_b_language_compare_template_score()) return 5;
  if (g_destroy_count != 4) return 6;
  if (g_destroy_codes[0] != 1 || g_destroy_codes[1] != 13 || g_destroy_codes[2] != 13 || g_destroy_codes[3] != 5) return 7;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_b_language_harness.c", harness, nil)
}

func TestEVT1TemplateVulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_b_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_b_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_b_vulkan.generated.h"
#include <stdint.h>

static int g_destroy_calls = 0;

int concept_vulkan_evt1_m1b_b_vulkan_measure__buffer_range(const concept_vulkan_buffer_range* value) {
  return value->offset + value->size;
}
int concept_vulkan_evt1_m1b_b_vulkan_measure__pipeline_state(const concept_vulkan_pipeline_state* value) {
  return value->alive ? 100 : 0;
}
void concept_vulkan_evt1_m1b_b_vulkan_destroy__buffer_range(concept_vulkan_buffer_range* value) {
  (void)value;
}
void concept_vulkan_evt1_m1b_b_vulkan_destroy__pipeline_state(concept_vulkan_pipeline_state* value) {
  (void)value;
  g_destroy_calls += 1;
}
void concept_vulkan_evt1_m1b_b_vulkan_set_alive(concept_vulkan_pipeline_state* value) {
  value->alive = true;
}

int main(void) {
  VkBuffer first = (VkBuffer)(uintptr_t)0x10u;
  VkBuffer second = (VkBuffer)(uintptr_t)0x20u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x30u;
  if (concept_vulkan_evt1_m1b_b_vulkan_classify_range(first) != 5) return 1;
  if (concept_vulkan_evt1_m1b_b_vulkan_double_range_score(first, second) != 22) return 2;
  if (!concept_vulkan_evt1_m1b_b_vulkan_build_and_destroy(pool)) return 3;
  if (g_destroy_calls != 1) return 4;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_b_vulkan_harness.c", harness, nil)
}

func TestEVT1M1BCLanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_c_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_c_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_c_language.generated.h"

int main(void) {
  if (concept_vulkan_evt1_m1b_c_language_selected_arm(true, 7, 3) != 7) return 1;
  if (concept_vulkan_evt1_m1b_c_language_selected_arm(false, 7, 3) != 3) return 2;
  if (concept_vulkan_evt1_m1b_c_language_count_up(2) != 4) return 3;
  if (concept_vulkan_evt1_m1b_c_language_count_up(10) != 6) return 4;
  if (concept_vulkan_evt1_m1b_c_language_zero_bound(5) != 99) return 5;
  if (concept_vulkan_evt1_m1b_c_language_default_bound() != 3) return 6;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_c_language_harness.c", harness, nil)
}

func TestEVT1M1BCVulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_c_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_c_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_c_vulkan.generated.h"
#include <stdint.h>

int main(void) {
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_m1b_c_vulkan_classify_range(buffer) != 3) return 1;
  if (!concept_vulkan_evt1_m1b_c_vulkan_pool_ready(pool, true)) return 2;
  if (concept_vulkan_evt1_m1b_c_vulkan_pool_ready(pool, false)) return 3;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_c_vulkan_harness.c", harness, nil)
}

func TestEVT1M1BDLanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_d_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_d_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_d_language.generated.h"

int main(void) {
  if (concept_vulkan_evt1_m1b_d_language_default_retry_budget() != 2) return 1;
  if (concept_vulkan_evt1_m1b_d_language_matrix_corner() != 6) return 2;
  if (concept_vulkan_evt1_m1b_d_language_summary_value() != 7) return 3;
  if (!concept_vulkan_evt1_m1b_d_language_transition_table_stable()) return 4;
  if (concept_vulkan_evt1_m1b_d_language_total_retry_budget() != 7) return 5;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_d_language_harness.c", harness, nil)
}

func TestEVT1M1BDVulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1b_d_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1b_d_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1b_d_vulkan.generated.h"
#include <stdint.h>

int main(void) {
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_m1b_d_vulkan_classify_range(buffer) != 5) return 1;
  if (concept_vulkan_evt1_m1b_d_vulkan_pipeline_stride(buffer) != 4) return 2;
  if (!concept_vulkan_evt1_m1b_d_vulkan_pool_ready(pool, true)) return 3;
  if (concept_vulkan_evt1_m1b_d_vulkan_pool_ready(pool, false)) return 4;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1b_d_vulkan_harness.c", harness, nil)
}

func TestDragonGodM0LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m0_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m0_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m0_language.generated.h"

int main(void) {
  if (concept_vulkan_evt1_dragongod_m0_language_root_retry_budget() != 2) return 1;
  if (concept_vulkan_evt1_dragongod_m0_language_derived_stack_depth() != 3) return 2;
  if (!concept_vulkan_evt1_dragongod_m0_language_finish_is_explicit()) return 3;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m0_language_harness.c", harness, nil)
}

func TestDragonGodM0VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m0_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m0_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m0_vulkan.generated.h"
#include <stdint.h>

int main(void) {
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_dragongod_m0_vulkan_classify_range(buffer) != 5) return 1;
  if (concept_vulkan_evt1_dragongod_m0_vulkan_derived_machine_depth(pool) != 3) return 2;
  if (!concept_vulkan_evt1_dragongod_m0_vulkan_pool_ready(pool, true)) return 3;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m0_vulkan_harness.c", harness, nil)
}

func TestDragonGodM1LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m1_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m1_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m1_language.generated.h"

int main(void) {
  if (concept_vulkan_evt1_dragongod_m1_language_initial_terminal_outcome_code() != 4) return 1;
  if (concept_vulkan_evt1_dragongod_m1_language_zero_capacity_outcome_code() != 3) return 2;
  if (concept_vulkan_evt1_dragongod_m1_language_unhandled_preserves_state_code() != 21) return 3;
  if (concept_vulkan_evt1_dragongod_m1_language_nested_push_resumes_caller_code() != 111113) return 4;
  if (concept_vulkan_evt1_dragongod_m1_language_root_terminal_continuation_finishes_immediately_code() != 1111134) return 5;
  if (concept_vulkan_evt1_dragongod_m1_language_non_root_finish_terminates_whole_instance_code() != 111134) return 6;
  if (concept_vulkan_evt1_dragongod_m1_language_independent_instances_stay_independent_code() != 1121) return 7;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m1_language_harness.c", harness, nil)
}

func TestDragonGodM1VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m1_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m1_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m1_vulkan.generated.h"
#include <stdint.h>

int main(void) {
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_dragongod_m1_vulkan_buffer_cleanup_finishes(buffer).tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_FINISHED) return 1;
  if (concept_vulkan_evt1_dragongod_m1_vulkan_buffer_lifecycle_already_finished_outcome(pool).tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_ALREADY_FINISHED) return 2;
  if (concept_vulkan_evt1_dragongod_m1_vulkan_buffer_cleanup_trace(buffer) != 111113) return 3;
  if (concept_vulkan_evt1_dragongod_m1_vulkan_vulkan_dispatch_audit(buffer, pool) != 1111134) return 4;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m1_vulkan_harness.c", harness, nil)
}

func TestDragonGodM2LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m2_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m2_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m2_language.generated.h"

int main(void) {
  concept_vulkan_lifecycle_context unique = {true, false};
  concept_vulkan_lifecycle_context fallback = {false, false};
  concept_vulkan_lifecycle_context ambiguous = {true, true};
  if (concept_vulkan_evt1_dragongod_m2_language_unique_guard_selection_code(unique) != 13) return 1;
  if (concept_vulkan_evt1_dragongod_m2_language_fallback_selection_code(fallback) != 113) return 2;
  if (concept_vulkan_evt1_dragongod_m2_language_guarded_unhandled_preserves_state_code(fallback) != 1213) return 3;
  if (concept_vulkan_evt1_dragongod_m2_language_ambiguous_preserves_state_code(ambiguous) != 51513) return 4;
  if (concept_vulkan_evt1_dragongod_m2_language_already_finished_skips_guard_selection_code(unique) != 4) return 5;
  if (concept_vulkan_evt1_dragongod_m2_language_contextless_compatibility_code() != 3) return 6;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m2_language_harness.c", harness, nil)
}

func TestDragonGodM2VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m2_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m2_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m2_vulkan.generated.h"
#include <stdint.h>

int main(void) {
  concept_vulkan_buffer_context unique = {true, false};
  concept_vulkan_buffer_context fallback = {false, false};
  concept_vulkan_buffer_context ambiguous = {true, true};
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  if (concept_vulkan_evt1_dragongod_m2_vulkan_vulkan_fallback_trace(fallback, buffer) != 113) return 1;
  if (concept_vulkan_evt1_dragongod_m2_vulkan_vulkan_ambiguous_trace(ambiguous, pool) != 51513) return 2;
  if (concept_vulkan_evt1_dragongod_m2_vulkan_buffer_already_finished_outcome(unique, buffer).tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_ALREADY_FINISHED) return 3;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_dragongod_m2_vulkan_harness.c", harness, nil)
}

func TestDragonGodM3LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m3_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m3_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m3_language.generated.c"

int main(void) {
  concept_vulkan_lifecycle_context unique = {true, false, 41, concept_vulkan_queue_class_make_graphics(), 7};
  concept_vulkan_lifecycle_context fallback = {false, false, 12, concept_vulkan_queue_class_make_compute(), 99};
  concept_vulkan_lifecycle_context ambiguous = {true, true, 8, concept_vulkan_queue_class_make_graphics(), 55};
  concept_vulkan_resource_lifecycle_instance lifecycle;
  concept_vulkan_resource_lifecycle_instance fallbackLifecycle;
  concept_vulkan_resource_lifecycle_instance ambiguousLifecycle;
  concept_vulkan_resource_lifecycle_effects emitted = {0};
  concept_vulkan_resource_lifecycle_effects fallbackBatch = {0};
  concept_vulkan_resource_lifecycle_effects ambiguousBatch = {0};
  concept_vulkan_automata_dispatch_outcome a;
  concept_vulkan_automata_dispatch_outcome b;
  concept_vulkan_automata_dispatch_outcome c;
  concept_vulkan_automata_dispatch_outcome d;
  concept_vulkan_automata_dispatch_outcome e;
  concept_vulkan_automata_dispatch_outcome f;

  concept_vulkan_resource_lifecycle_init(&lifecycle, &unique);
  a = concept_vulkan_resource_lifecycle_dispatch(&lifecycle, concept_vulkan_lifecycle_signal_make_submit(), &emitted);
  if (a.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_TRANSITIONED) return 1;
  if (emitted.count != 2) return 2;
  if (emitted.entries[0].tag != CONCEPT_VULKAN_RESOURCE_LIFECYCLE_EFFECT_RECORD_SUBMISSION) return 3;
  if (emitted.entries[0].payload.record_submission.submission != 41) return 4;
  if (emitted.entries[1].tag != CONCEPT_VULKAN_RESOURCE_LIFECYCLE_EFFECT_BEGIN_SUBMISSION) return 5;
  if (emitted.entries[1].payload.begin_submission.submission != 41) return 6;
  if (emitted.entries[1].payload.begin_submission.queue.tag != CONCEPT_VULKAN_QUEUE_CLASS_GRAPHICS) return 7;

  b = concept_vulkan_resource_lifecycle_dispatch(&lifecycle, concept_vulkan_lifecycle_signal_make_submitted(), &emitted);
  if (b.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_TRANSITIONED) return 8;
  if (emitted.count != 0) return 9;

  concept_vulkan_resource_lifecycle_init(&fallbackLifecycle, &fallback);
  c = concept_vulkan_resource_lifecycle_dispatch(&fallbackLifecycle, concept_vulkan_lifecycle_signal_make_submit(), &fallbackBatch);
  if (c.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_TRANSITIONED) return 10;
  if (fallbackBatch.count != 1) return 11;
  if (fallbackBatch.entries[0].tag != CONCEPT_VULKAN_RESOURCE_LIFECYCLE_EFFECT_FINALIZE_TICKET) return 12;
  if (fallbackBatch.entries[0].payload.finalize_ticket.ticket != 99) return 13;

  concept_vulkan_resource_lifecycle_init(&ambiguousLifecycle, &ambiguous);
  d = concept_vulkan_resource_lifecycle_dispatch(&ambiguousLifecycle, concept_vulkan_lifecycle_signal_make_submit(), &ambiguousBatch);
  if (d.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_AMBIGUOUS) return 14;
  if (ambiguousBatch.count != 0) return 15;

  e = concept_vulkan_resource_lifecycle_dispatch(&ambiguousLifecycle, concept_vulkan_lifecycle_signal_make_finish_now(), &ambiguousBatch);
  if (e.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_FINISHED) return 16;
  if (ambiguousBatch.count != 1) return 17;
  if (ambiguousBatch.entries[0].payload.finalize_ticket.ticket != 55) return 18;

  f = concept_vulkan_resource_lifecycle_dispatch(&ambiguousLifecycle, concept_vulkan_lifecycle_signal_make_submit(), &ambiguousBatch);
  if (f.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_ALREADY_FINISHED) return 19;
  if (ambiguousBatch.count != 0) return 20;

  if (concept_vulkan_evt1_dragongod_m3_language_contextless_compatibility_code() != 3) return 21;
  return 0;
}
`
	runNativeHarnessIncludingGeneratedC(t, outputs, "evt1_dragongod_m3_language_harness.c", harness, nil)
}

func TestDragonGodM3VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_dragongod_m3_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_dragongod_m3_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_dragongod_m3_vulkan.generated.c"
#include <stdint.h>

int main(void) {
  concept_vulkan_buffer_context unique = {true, false, 21, concept_vulkan_queue_class_make_graphics(), 5};
  concept_vulkan_buffer_context fallback = {false, false, 22, concept_vulkan_queue_class_make_transfer(), 8};
  concept_vulkan_buffer_context ambiguous = {true, true, 23, concept_vulkan_queue_class_make_graphics(), 13};
  concept_vulkan_buffer_lifecycle_instance lifecycle;
  concept_vulkan_buffer_lifecycle_instance fallbackLifecycle;
  concept_vulkan_buffer_lifecycle_instance ambiguousLifecycle;
  concept_vulkan_buffer_lifecycle_effects emitted = {0};
  concept_vulkan_buffer_lifecycle_effects fallbackBatch = {0};
  concept_vulkan_buffer_lifecycle_effects ambiguousBatch = {0};
  concept_vulkan_automata_dispatch_outcome a;
  concept_vulkan_automata_dispatch_outcome b;
  concept_vulkan_automata_dispatch_outcome c;
  VkBuffer buffer = (VkBuffer)(uintptr_t)0x10u;
  VkCommandPool pool = (VkCommandPool)(uintptr_t)0x20u;
  (void)buffer;
  (void)pool;

  concept_vulkan_buffer_lifecycle_init(&lifecycle, &unique);
  a = concept_vulkan_buffer_lifecycle_dispatch(&lifecycle, concept_vulkan_resource_signal_make_submit(), &emitted);
  if (a.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_TRANSITIONED) return 1;
  if (emitted.count != 2) return 2;
  if (emitted.entries[0].payload.record_buffer_submission.bufferId != 21) return 3;
  if (emitted.entries[1].payload.begin_buffer_submission.queue.tag != CONCEPT_VULKAN_QUEUE_CLASS_GRAPHICS) return 4;

  concept_vulkan_buffer_lifecycle_init(&fallbackLifecycle, &fallback);
  b = concept_vulkan_buffer_lifecycle_dispatch(&fallbackLifecycle, concept_vulkan_resource_signal_make_submit(), &fallbackBatch);
  if (b.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_TRANSITIONED) return 5;
  if (fallbackBatch.count != 1) return 6;
  if (fallbackBatch.entries[0].payload.mark_buffer_failure.failureCode != 8) return 7;

  concept_vulkan_buffer_lifecycle_init(&ambiguousLifecycle, &ambiguous);
  c = concept_vulkan_buffer_lifecycle_dispatch(&ambiguousLifecycle, concept_vulkan_resource_signal_make_submit(), &ambiguousBatch);
  if (c.tag != CONCEPT_VULKAN_AUTOMATA_DISPATCH_OUTCOME_AMBIGUOUS) return 8;
  if (ambiguousBatch.count != 0) return 9;
  return 0;
}
`
	runNativeHarnessIncludingGeneratedC(t, outputs, "evt1_dragongod_m3_vulkan_harness.c", harness, nil)
}

func runNativeHarness(t *testing.T, outputs Outputs, harnessName, harnessSource string, extraArgs []string) {
	t.Helper()
	if _, err := exec.LookPath("cl"); err != nil {
		t.Skip("cl not found on PATH")
	}
	dir := t.TempDir()
	if err := Write(dir, outputs); err != nil {
		t.Fatal(err)
	}
	harnessPath := filepath.Join(dir, harnessName)
	if err := os.WriteFile(harnessPath, []byte(harnessSource), 0644); err != nil {
		t.Fatal(err)
	}
	var generatedC string
	for key := range outputs {
		if strings.HasSuffix(key, ".generated.c") {
			generatedC = filepath.Join(dir, key)
			break
		}
	}
	if generatedC == "" {
		t.Fatal("generated C output missing")
	}
	args := []string{"/nologo", "/std:c11", "/W4", "/I" + dir}
	if sdk := os.Getenv("VULKAN_SDK"); sdk != "" {
		args = append(args, "/I"+filepath.Join(sdk, "Include"))
	}
	args = append(args, generatedC, harnessPath, "/Fe:"+filepath.Join(dir, "specimen.exe"))
	args = append(args, extraArgs...)
	build := exec.Command("cl", args...)
	build.Dir = dir
	out, err := build.CombinedOutput()
	if err != nil {
		text := strings.ToLower(string(out))
		if strings.Contains(text, "cannot open include file: 'vulkan/vulkan.h'") {
			t.Skip("Vulkan SDK headers are unavailable in this environment")
		}
		if strings.Contains(text, "cannot open include file: 'stdbool.h'") ||
			strings.Contains(text, "cannot open include file: 'stdint.h'") ||
			strings.Contains(text, "cannot open include file: 'stddef.h'") {
			t.Skip("MSVC developer include environment is unavailable in this shell")
		}
		t.Fatalf("cl failed:\n%s", out)
	}
	run := exec.Command(filepath.Join(dir, "specimen.exe"))
	run.Dir = dir
	runOut, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("specimen failed: %v\n%s", err, runOut)
	}
}

func runNativeHarnessIncludingGeneratedC(t *testing.T, outputs Outputs, harnessName, harnessSource string, extraArgs []string) {
	t.Helper()
	if _, err := exec.LookPath("cl"); err != nil {
		t.Skip("cl not found on PATH")
	}
	dir := t.TempDir()
	if err := Write(dir, outputs); err != nil {
		t.Fatal(err)
	}
	harnessPath := filepath.Join(dir, harnessName)
	if err := os.WriteFile(harnessPath, []byte(harnessSource), 0644); err != nil {
		t.Fatal(err)
	}
	args := []string{"/nologo", "/std:c11", "/W4", "/I" + dir}
	if sdk := os.Getenv("VULKAN_SDK"); sdk != "" {
		args = append(args, "/I"+filepath.Join(sdk, "Include"))
	}
	args = append(args, harnessPath, "/Fe:"+filepath.Join(dir, "specimen.exe"))
	args = append(args, extraArgs...)
	build := exec.Command("cl", args...)
	build.Dir = dir
	out, err := build.CombinedOutput()
	if err != nil {
		text := strings.ToLower(string(out))
		if strings.Contains(text, "cannot open include file: 'vulkan/vulkan.h'") {
			t.Skip("Vulkan SDK headers are unavailable in this environment")
		}
		if strings.Contains(text, "cannot open include file: 'stdbool.h'") ||
			strings.Contains(text, "cannot open include file: 'stdint.h'") ||
			strings.Contains(text, "cannot open include file: 'stddef.h'") {
			t.Skip("MSVC developer include environment is unavailable in this shell")
		}
		t.Fatalf("cl failed:\n%s", out)
	}
	run := exec.Command(filepath.Join(dir, "specimen.exe"))
	run.Dir = dir
	runOut, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("specimen failed: %v\n%s", err, runOut)
	}
}
