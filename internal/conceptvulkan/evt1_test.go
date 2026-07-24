package conceptvulkan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"os/exec"
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
	for _, name := range []string{"evt1_m1a_language.concept", "evt1_m1a_vulkan.concept"} {
		t.Run(name, func(t *testing.T) {
			src := readEVT1Fixture(t, name)
			module, err := ParseEVT1(filepath.ToSlash(filepath.Join("examples", "Concept-Vulkan", name)), src)
			if err != nil {
				t.Fatal(err)
			}
			if len(module.Enums) == 0 || len(module.Functions) == 0 {
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
			if !strings.Contains(MIRTextEVT1(buildEVT1MIR(module)), "match_expr") {
				t.Fatal("MIR text omitted match_expr")
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
	for _, name := range []string{"evt1_m1a_language.concept", "evt1_m1a_vulkan.concept"} {
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
	src := readEVT1Fixture(t, "evt1_m1a_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1a_vulkan.concept", src)
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

func TestEVT1CGenerationUsesTransparentTagUnionAndSwitch(t *testing.T) {
	src := readEVT1Fixture(t, "evt1_m1a_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1a_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	header := string(outputs["evt1_m1a_vulkan.generated.h"])
	body := string(outputs["evt1_m1a_vulkan.generated.c"])
	for _, needle := range []string{
		"typedef enum concept_vulkan_pipeline_state_tag",
		"union {",
		"struct {\n      VkPipelineLayout layout;\n      VkPipeline pipeline;",
	} {
		if !strings.Contains(header, needle) {
			t.Fatalf("header missing %q\n%s", needle, header)
		}
	}
	for _, needle := range []string{
		"switch (cv_match_subject_",
		"concept_vulkan_pipeline_state_make_ready",
		"concept_vulkan_abort_invalid_tag",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("body missing %q\n%s", needle, body)
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
			name: "duplicate variant",
			src: "profile Vulkan;\nenum Status { Empty, Empty }\n",
			code: "CV4100",
		},
		{
			name: "unknown enum in construction",
			src: "profile Vulkan;\nenum Status { Empty }\nStatus Make() { return Missing::Empty; }\n",
			code: "CV4102",
		},
		{
			name: "unknown variant",
			src: "profile Vulkan;\nenum Status { Empty }\nStatus Make() { return Status::Ready; }\n",
			code: "CV4103",
		},
		{
			name: "payload variant without args",
			src: "profile Vulkan;\nenum Status { Ready(int value) }\nStatus Make() { return Status::Ready; }\n",
			code: "CV4104",
		},
		{
			name: "unit variant with args",
			src: "profile Vulkan;\nenum Status { Empty }\nStatus Make() { return Status::Empty(1); }\n",
			code: "CV4104",
		},
		{
			name: "wrong payload count",
			src: "profile Vulkan;\nenum Status { Ready(int value, int other) }\nStatus Make() { return Status::Ready(1); }\n",
			code: "CV4106",
		},
		{
			name: "wrong payload type",
			src: "profile Vulkan;\nenum Status { Ready(int value) }\nVulkanError Err();\nStatus Make() { return Status::Ready(Err()); }\n",
			code: "CV4107",
		},
		{
			name: "non enum subject",
			src: "profile Vulkan;\nint Match(int value) { return match (value) { Missing::Empty => 0, }; }\n",
			code: "CV4108",
		},
		{
			name: "wrong enum pattern",
			src: "profile Vulkan;\nenum Left { Empty }\nenum Right { Empty }\nint Match(Left value) { return match (value) { Right::Empty => 0, }; }\n",
			code: "CV4109",
		},
		{
			name: "unknown arm variant",
			src: "profile Vulkan;\nenum Status { Empty }\nint Match(Status value) { return match (value) { Status::Ready => 0, }; }\n",
			code: "CV4110",
		},
		{
			name: "binding count mismatch",
			src: "profile Vulkan;\nenum Status { Ready(int value) }\nint Match(Status value) { return match (value) { Status::Ready(first, second) => 0, }; }\n",
			code: "CV4111",
		},
		{
			name: "unit bindings",
			src: "profile Vulkan;\nenum Status { Empty }\nint Match(Status value) { return match (value) { Status::Empty(binding) => 0, }; }\n",
			code: "CV4112",
		},
		{
			name: "duplicate binding",
			src: "profile Vulkan;\nenum Status { Ready(int value, int other) }\nint Match(Status value) { return match (value) { Status::Ready(binding, binding) => 0, }; }\n",
			code: "CV4112",
		},
		{
			name: "duplicate arm",
			src: "profile Vulkan;\nenum Status { Empty }\nint Match(Status value) { return match (value) { Status::Empty => 0, Status::Empty => 1, }; }\n",
			code: "CV4113",
		},
		{
			name: "non exhaustive",
			src: "profile Vulkan;\nenum Status { Empty, Ready(int value) }\nint Match(Status value) { return match (value) { Status::Empty => 0, }; }\n",
			code: "CV4115",
		},
		{
			name: "result mismatch",
			src: "profile Vulkan;\nenum Status { Empty, Failed(VulkanError error) }\nint Match(Status value) { return match (value) { Status::Empty => 0, Status::Failed(error) => error, }; }\n",
			code: "CV4116",
		},
		{
			name: "expression block arm",
			src: "profile Vulkan;\nenum Status { Empty }\nint Match(Status value) { return match (value) { Status::Empty => { }, }; }\n",
			code: "CV4117",
		},
		{
			name: "statement arm missing block",
			src: "profile Vulkan;\nenum Status { Empty }\nvoid Match(Status value) { match (value) { Status::Empty => Record(); } }\nvoid Record();\n",
			code: "CV4118",
		},
		{
			name: "binding escape",
			src: "profile Vulkan;\nenum Status { Ready(int value) }\nint Match(Status value) { match (value) { Status::Ready(inner) => { } } return inner; }\n",
			code: "CV4114",
		},
		{
			name: "owned payload rejected",
			src: "profile Vulkan;\nenum Status { Ready(owned Pipeline value) }\n",
			code: "CV4119",
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
	src := readEVT1Fixture(t, "evt1_m1a_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1a_language.concept", src)
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
	if err := os.WriteFile(filepath.Join(dir, "evt1_m1a_language.generated.c"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir, outputs); err == nil || !strings.Contains(err.Error(), "CV3001") {
		t.Fatalf("check error=%v", err)
	}
}

func TestEVT1LanguageSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1a_language.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1a_language.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1a_language.generated.h"
#include <stdint.h>

static int g_observe_calls = 0;
static int g_records[8];
static int g_record_count = 0;

int concept_vulkan_evt1_m1a_language_add(int left, int right) { return left + right; }
concept_vulkan_demo_state concept_vulkan_evt1_m1a_language_observe_state(void) {
  g_observe_calls += 1;
  return concept_vulkan_evt1_m1a_language_make_pair(7, 9);
}
int concept_vulkan_evt1_m1a_language_observe_calls(void) { return g_observe_calls; }
void concept_vulkan_evt1_m1a_language_record_int(int value) { g_records[g_record_count++] = value; }
void concept_vulkan_evt1_m1a_language_record_pair(int first, int second) {
  g_records[g_record_count++] = first;
  g_records[g_record_count++] = second;
}

int main(void) {
  if (concept_vulkan_evt1_m1a_language_classify(concept_vulkan_evt1_m1a_language_make_empty()) != 0) return 1;
  if (concept_vulkan_evt1_m1a_language_classify(concept_vulkan_evt1_m1a_language_make_counted(5)) != 5) return 2;
  if (concept_vulkan_evt1_m1a_language_classify(concept_vulkan_evt1_m1a_language_make_pair(3, 4)) != 7) return 3;
  if (concept_vulkan_evt1_m1a_language_classify(
          concept_vulkan_evt1_m1a_language_make_wrapped(concept_vulkan_evt1_m1a_language_make_inner_counted(9))) != 109) return 4;
  if (concept_vulkan_evt1_m1a_language_observe_and_classify() != 16) return 5;
  if (concept_vulkan_evt1_m1a_language_observe_calls() != 1) return 6;
  concept_vulkan_evt1_m1a_language_visit(concept_vulkan_evt1_m1a_language_make_pair(11, 12));
  concept_vulkan_evt1_m1a_language_visit(
      concept_vulkan_evt1_m1a_language_make_wrapped(concept_vulkan_evt1_m1a_language_make_inner_counted(7)));
  if (g_record_count != 3) return 7;
  if (g_records[0] != 11 || g_records[1] != 12 || g_records[2] != 1007) return 8;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1a_language_harness.c", harness, nil)
}

func TestEVT1VulkanSpecimenNativeC11(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native C11 harness is only configured for Windows in this repository")
	}
	src := readEVT1Fixture(t, "evt1_m1a_vulkan.concept")
	module, err := ParseEVT1("examples/Concept-Vulkan/evt1_m1a_vulkan.concept", src)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := GenerateEVT1(module, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	harness := `#include "evt1_m1a_vulkan.generated.h"
#include <stdint.h>

static int g_events[4];
static int g_event_count = 0;
static int g_failure_code = 0;

void concept_vulkan_evt1_m1a_vulkan_destroy_pipeline(VkPipeline pipeline) {
  (void)pipeline;
  g_events[g_event_count++] = 1;
}
void concept_vulkan_evt1_m1a_vulkan_destroy_pipeline_layout(VkPipelineLayout layout) {
  (void)layout;
  g_events[g_event_count++] = 2;
}
void concept_vulkan_evt1_m1a_vulkan_record_failure(concept_vulkan_vulkan_error error) {
  g_failure_code = error.Code;
}

int main(void) {
  VkPipelineLayout layout = (VkPipelineLayout)(uintptr_t)0x10u;
  VkPipeline pipeline = (VkPipeline)(uintptr_t)0x20u;
  concept_vulkan_vulkan_error error = {77};
  if (concept_vulkan_evt1_m1a_vulkan_get_status_code(concept_vulkan_evt1_m1a_vulkan_make_empty_state()) != 0) return 1;
  if (concept_vulkan_evt1_m1a_vulkan_get_status_code(concept_vulkan_evt1_m1a_vulkan_make_layout_created_state(layout)) != 1) return 2;
  if (concept_vulkan_evt1_m1a_vulkan_get_status_code(concept_vulkan_evt1_m1a_vulkan_make_ready_state(layout, pipeline)) != 2) return 3;
  if (concept_vulkan_evt1_m1a_vulkan_get_status_code(concept_vulkan_evt1_m1a_vulkan_make_failed_state(error)) != 77) return 4;
  concept_vulkan_evt1_m1a_vulkan_destroy_pipeline_state(concept_vulkan_evt1_m1a_vulkan_make_ready_state(layout, pipeline));
  if (g_event_count != 2 || g_events[0] != 1 || g_events[1] != 2) return 5;
  concept_vulkan_evt1_m1a_vulkan_destroy_pipeline_state(concept_vulkan_evt1_m1a_vulkan_make_failed_state(error));
  if (g_failure_code != 77) return 6;
  return 0;
}
`
	runNativeHarness(t, outputs, "evt1_m1a_vulkan_harness.c", harness, nil)
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
		if strings.Contains(text, "cannot open include file: 'stdint.h'") ||
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
