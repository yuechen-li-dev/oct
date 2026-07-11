package test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type hostInvocation struct {
	c         Case
	group     Group
	caseIndex int
	rowIndex  int
}

type compiledHostSuite struct {
	manifest     Manifest
	groups       []Group
	manifestPath string
	byFunction   map[string]hostInvocation
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func nativeHostExecutable(t *testing.T) string {
	t.Helper()
	host := os.Getenv("SDSLV_TEST_HOST")
	if host == "" {
		host = filepath.Join(repoRoot(t), "out", "prometheus", "native", "sdslv_test_host.exe")
	}
	if _, err := os.Stat(host); err != nil {
		t.Skipf("native SDSL-V host unavailable at %s", host)
	}
	return host
}

func compileHostSuiteFromFile(t *testing.T, suitePath string) compiledHostSuite {
	t.Helper()
	artifactRoot := filepath.Join(t.TempDir(), "artifacts")
	suite, err := Prepare(suitePath)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Compile(suite, artifactRoot)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ProjectManifest(suite, groups)
	manifestPath := filepath.Join(artifactRoot, "manifest.json")
	if err := WriteManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	byFunction := make(map[string]hostInvocation, len(manifest.Cases))
	for _, c := range manifest.Cases {
		inv := hostInvocation{c: c, rowIndex: rowNumber(c)}
		found := false
		for _, g := range groups {
			for i, id := range g.Cases {
				if id == c.StableID {
					inv.group = g
					inv.caseIndex = i
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Fatalf("compiled group missing %s", c.StableID)
		}
		byFunction[c.Function] = inv
	}
	return compiledHostSuite{manifest: manifest, groups: groups, manifestPath: manifestPath, byFunction: byFunction}
}

func runNativeHost(t *testing.T, host string, inv hostInvocation, manifestPath string) (string, error) {
	t.Helper()
	args := []string{
		"--manifest", manifestPath,
		"--spv", inv.group.SPIRVPath,
		"--case", inv.c.StableID,
		"--case-index", inv.casestringIndex(),
		"--row-index", inv.rowstringIndex(),
		"--groups", uintString(inv.c.Launch.DispatchGroups[0]), uintString(inv.c.Launch.DispatchGroups[1]), uintString(inv.c.Launch.DispatchGroups[2]),
		"--workgroup", uintString(inv.c.Launch.WorkgroupSize[0]), uintString(inv.c.Launch.WorkgroupSize[1]), uintString(inv.c.Launch.WorkgroupSize[2]),
	}
	data, err := exec.Command(host, args...).CombinedOutput()
	return string(data), err
}

func (inv hostInvocation) casestringIndex() string { return uintString(uint32(inv.caseIndex)) }
func (inv hostInvocation) rowstringIndex() string  { return uintString(uint32(inv.rowIndex)) }

func uintString(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func rewriteManifestJSON(t *testing.T, originalPath string, mutate func(map[string]any)) string {
	t.Helper()
	data, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	mutate(payload)
	updated, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	rewritten := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(rewritten, append(updated, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return rewritten
}

func rewriteManifestTruncated(t *testing.T, originalPath string) string {
	t.Helper()
	data, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	marker := `"payload_words": [`
	index := strings.Index(text, marker)
	if index == -1 {
		t.Fatal("payload marker missing")
	}
	rewritten := filepath.Join(t.TempDir(), "truncated_manifest.json")
	if err := os.WriteFile(rewritten, []byte(text[:index+len(marker)]), 0o644); err != nil {
		t.Fatal(err)
	}
	return rewritten
}

func mutateCaseInput(t *testing.T, payload map[string]any, functionName string, mutate func(map[string]any)) {
	t.Helper()
	cases, ok := payload["cases"].([]any)
	if !ok {
		t.Fatal("manifest cases missing")
	}
	for _, entry := range cases {
		caseMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if caseMap["function"] == functionName {
			input, ok := caseMap["test_input"].(map[string]any)
			if !ok {
				t.Fatal("test_input missing")
			}
			mutate(input)
			return
		}
	}
	t.Fatalf("function %s missing from manifest", functionName)
}

func TestSdslvNativeHostRejectsMalformedTestInputManifest(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M30", "FixedTestInputResources.sdslvtest"))
	inv := fixture.byFunction["GuardedReadUsesSource"]
	cases := []struct {
		name   string
		mutate func(string) string
	}{
		{
			name: "invalid abi version",
			mutate: func(path string) string {
				return rewriteManifestJSON(t, path, func(payload map[string]any) {
					mutateCaseInput(t, payload, "GuardedReadUsesSource", func(input map[string]any) {
						input["abi_version"] = float64(99)
					})
				})
			},
		},
		{
			name: "invalid value kind",
			mutate: func(path string) string {
				return rewriteManifestJSON(t, path, func(payload map[string]any) {
					mutateCaseInput(t, payload, "GuardedReadUsesSource", func(input map[string]any) {
						input["kind"] = "shader"
					})
				})
			},
		},
		{
			name: "element count mismatch",
			mutate: func(path string) string {
				return rewriteManifestJSON(t, path, func(payload map[string]any) {
					mutateCaseInput(t, payload, "GuardedReadUsesSource", func(input map[string]any) {
						input["element_count"] = float64(3)
					})
				})
			},
		},
		{
			name: "oversized payload",
			mutate: func(path string) string {
				return rewriteManifestJSON(t, path, func(payload map[string]any) {
					mutateCaseInput(t, payload, "GuardedReadUsesSource", func(input map[string]any) {
						input["element_count"] = float64(1)
						input["payload_words"] = []any{float64(7), float64(11)}
					})
				})
			},
		},
		{
			name: "truncated payload",
			mutate: func(path string) string {
				return rewriteManifestTruncated(t, path)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutatedPath := tc.mutate(fixture.manifestPath)
			out, err := runNativeHost(t, host, inv, mutatedPath)
			if err == nil {
				t.Fatalf("expected host failure, got %q", out)
			}
			if !strings.Contains(out, `"status":"HOST_FAILURE"`) || !strings.Contains(out, "malformed test input manifest metadata") {
				t.Fatalf("unexpected host output %q", out)
			}
		})
	}
}

func TestSdslvNativeHostNoInputReplayIsDeterministic(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M30", "FixedTestInputResources.sdslvtest"))
	inv := fixture.byFunction["NoInputCompatibility"]
	first, err := runNativeHost(t, host, inv, fixture.manifestPath)
	if err != nil {
		t.Fatalf("first replay failed: %v: %s", err, first)
	}
	second, err := runNativeHost(t, host, inv, fixture.manifestPath)
	if err != nil {
		t.Fatalf("second replay failed: %v: %s", err, second)
	}
	if first != second {
		t.Fatalf("no-input replay output changed:\n%s\n!=\n%s", first, second)
	}
	if !strings.Contains(first, `"status":"PASS"`) {
		t.Fatalf("no-input replay did not pass: %s", first)
	}
}

func TestSdslvStableCaseReplayWithTestInput(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	t.Chdir(root)
	if err := os.Setenv("SDSLV_TEST_HOST", host); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "SDSL-V", "M30", "FixedTestInputResources.sdslvtest")
	manifest, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	caseID := ""
	for _, c := range manifest.Cases {
		if c.Function == "GuardedReadUsesSource" {
			caseID = c.StableID
			break
		}
	}
	if caseID == "" {
		t.Fatal("GuardedReadUsesSource case missing")
	}
	var out bytes.Buffer
	if err := ExecuteWithOptions(path, &out, Options{CaseID: caseID}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Count(text, caseID) != 1 || strings.Contains(text, `"stable_case_id":"sdslv-b71c5378d98c97d63a061800"`) {
		t.Fatalf("stable-case replay output = %q", text)
	}
}

func TestSdslvNativeHostFixedInputContractSourceGuards(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal", "prometheus", "native", "sdslv_test_host.c"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"layout_info.bindingCount = 2u;",
		"bindings[1].binding = 1u;",
		"vkUpdateDescriptorSets(device, 2u, writes, 0u, 0);",
		"input_metadata.element_count == 0u ? 1u : input_metadata.element_count",
		"checked_mul_size((size_t)invocations, sizeof(result_record),",
		"checked_mul_size(input_word_count, sizeof(uint32_t), &input_bytes)",
		"destroy_buffer_allocation(device, &input_buffer);",
		"destroy_buffer_allocation(device, &result_buffer);",
		"json(\"INVALID_RESULT_BUFFER\", \"ABI version\", stable_id, NULL);",
		"json(vr == VK_ERROR_DEVICE_LOST ? \"DEVICE_LOST\" : \"SUBMIT_FAILED\",",
		"json(\"TIMEOUT\", \"deadline exceeded\", stable_id, NULL);",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("native host lost fixed-input contract evidence %q", want)
		}
	}
}
