package test

import (
	"bytes"
	"encoding/json"
	"math"
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
	byDisplay    map[string]hostInvocation
}

type nativeResult struct {
	Status        string    `json:"status"`
	StableCaseID  string    `json:"stable_case_id"`
	Invocation    [3]uint32 `json:"invocation"`
	AssertionID   uint32    `json:"assertion_id"`
	Source        [2]uint32 `json:"source"`
	ABIVersion    uint32    `json:"abi_version"`
	Failed        uint32    `json:"failed"`
	ValueKind     uint32    `json:"value_kind"`
	Component     uint32    `json:"component_count"`
	ExpectedBits  [4]uint32 `json:"expected_bits"`
	ActualBits    [4]uint32 `json:"actual_bits"`
	ToleranceBits [4]uint32 `json:"tolerance_bits"`
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
	byDisplay := make(map[string]hostInvocation, len(manifest.Cases))
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
		byDisplay[c.DisplayName] = inv
	}
	return compiledHostSuite{manifest: manifest, groups: groups, manifestPath: manifestPath, byFunction: byFunction, byDisplay: byDisplay}
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

func runNativeHostJSON(t *testing.T, host string, inv hostInvocation, manifestPath string) (string, nativeResult, error) {
	t.Helper()
	out, err := runNativeHost(t, host, inv, manifestPath)
	var result nativeResult
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
		t.Fatalf("invalid native result JSON: %v: %s", decodeErr, out)
	}
	return out, result, err
}

func (inv hostInvocation) casestringIndex() string { return uintString(uint32(inv.caseIndex)) }
func (inv hostInvocation) rowstringIndex() string  { return uintString(uint32(inv.rowIndex)) }

func assertSource(inv hostInvocation, assertionID uint32) [2]uint32 {
	if int(assertionID) >= len(inv.c.Assertions) {
		return [2]uint32{}
	}
	span := inv.c.Assertions[assertionID].CallSpan
	return [2]uint32{uint32(span.Start.Line), uint32(span.Start.Column)}
}

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

func TestSdslvNativeHostXYZAndDeterministicFailingInvocation(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	passingFixture := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M29", "XYZInvocationIndexing.sdslvtest"))
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "XYZAssertionFailures.sdslvvalid"))

	passing := passingFixture.byFunction["CombinedXYZGeometry"]
	out, err := runNativeHost(t, host, passing, passingFixture.manifestPath)
	if err != nil || !strings.Contains(out, `"status":"PASS"`) {
		t.Fatalf("combined XYZ geometry failed: %v: %s", err, out)
	}

	checks := []struct {
		function string
		want     [3]uint32
	}{
		{"OneFailingInvocation", [3]uint32{3, 4, 2}},
		{"TwoFailingInvocationsUseLinearScanOrder", [3]uint32{1, 1, 0}},
	}
	for _, check := range checks {
		t.Run(check.function, func(t *testing.T) {
			inv := fixture.byFunction[check.function]
			first, firstErr := runNativeHost(t, host, inv, fixture.manifestPath)
			second, secondErr := runNativeHost(t, host, inv, fixture.manifestPath)
			if firstErr != nil || secondErr != nil {
				t.Fatalf("host process did not complete normally: first=%v second=%v", firstErr, secondErr)
			}
			if first != second {
				t.Fatalf("failure replay changed:\n%s\n!=\n%s", first, second)
			}
			var result nativeResult
			if err := json.Unmarshal([]byte(first), &result); err != nil {
				t.Fatalf("invalid result JSON: %v: %s", err, first)
			}
			if result.Status != "ASSERTION_FAILED" || result.StableCaseID != inv.c.StableID || result.Invocation != check.want || result.AssertionID != 0 || result.Source == [2]uint32{} {
				t.Fatalf("unexpected failure result: %#v", result)
			}
			if result.ExpectedBits != [4]uint32{} || result.ActualBits != [4]uint32{} || result.ToleranceBits != [4]uint32{} {
				t.Fatalf("Assert.False unused ABI lanes are not zero: %#v", result)
			}
		})
	}
}

func TestSdslvExpectedAssertionFailureIsAHostSuccessContract(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "AssertionFailures.sdslvvalid"))
	inv := fixture.byFunction["FirstFailureWins"]
	out, err := runNativeHost(t, host, inv, fixture.manifestPath)
	if err != nil {
		t.Fatalf("host process did not complete normally: %v: %s", err, out)
	}
	var result nativeResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid assertion result JSON: %v: %s", err, out)
	}
	if result.Status != "ASSERTION_FAILED" || result.StableCaseID != inv.c.StableID || result.Invocation != [3]uint32{} || result.AssertionID != 0 || result.ExpectedBits != [4]uint32{1, 0, 0, 0} || result.ActualBits != [4]uint32{2, 0, 0, 0} || result.ToleranceBits != [4]uint32{} {
		t.Fatalf("first failure contract mismatch: %#v", result)
	}
}

func TestSdslvExpectedAssertionFailureKeepsUserFacingRunnerFailure(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	t.Chdir(root)
	if err := os.Setenv("SDSLV_TEST_HOST", host); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "AssertionFailures.sdslvvalid")
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ProjectManifest(suite, nil)
	var selected Case
	for _, c := range manifest.Cases {
		if c.Function == "FirstFailureWins" {
			selected = c
			break
		}
	}
	if selected.StableID == "" {
		t.Fatal("FirstFailureWins case missing")
	}
	var out bytes.Buffer
	err = executeGPU(suite, manifest, []Case{selected}, &out)
	if err == nil || !strings.Contains(err.Error(), "GPU test failure: FirstFailureWins") {
		t.Fatalf("expected user-facing GPU test failure, got err=%v out=%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"status":"ASSERTION_FAILED"`) || !strings.Contains(out.String(), `"stable_case_id":"`+selected.StableID+`"`) {
		t.Fatalf("expected structured assertion failure JSON, got %s", out.String())
	}
}

func TestSdslvNativeHostAssertAndABIMatrix(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	passing := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M29", "RealAssertions.sdslvtest"))
	passInv := passing.byFunction["ScalarAssertions"]
	first, passResult, err := runNativeHostJSON(t, host, passInv, passing.manifestPath)
	if err != nil {
		t.Fatalf("passing matrix case failed: %v: %s", err, first)
	}
	second, _, err := runNativeHostJSON(t, host, passInv, passing.manifestPath)
	if err != nil {
		t.Fatalf("passing matrix rerun failed: %v: %s", err, second)
	}
	if first != second {
		t.Fatalf("passing ABI JSON changed:\n%s\n!=\n%s", first, second)
	}
	if passResult.Status != "PASS" || passResult.StableCaseID != passInv.c.StableID || passResult.ABIVersion != 1 || passResult.Failed != 0 || passResult.AssertionID != 0 || passResult.Source != [2]uint32{} || passResult.Invocation != [3]uint32{} || passResult.ValueKind != 0 || passResult.Component != 1 || passResult.ExpectedBits != [4]uint32{} || passResult.ActualBits != [4]uint32{} || passResult.ToleranceBits != [4]uint32{} {
		t.Fatalf("passing ABI record mismatch: %#v", passResult)
	}

	failures := compileHostSuiteFromFile(t, filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "AssertionFailures.sdslvvalid"))
	cases := []struct {
		function                    string
		valueKind                   uint32
		expected, actual, tolerance [4]uint32
	}{
		{"TrueFailure", 1, [4]uint32{1}, [4]uint32{}, [4]uint32{}},
		{"FalseFailure", 1, [4]uint32{}, [4]uint32{}, [4]uint32{}},
		{"EqualBoolFailure", 1, [4]uint32{1}, [4]uint32{}, [4]uint32{}},
		{"EqualIntFailure", 2, [4]uint32{0xfffffff9}, [4]uint32{7}, [4]uint32{}},
		{"EqualUIntFailure", 3, [4]uint32{7}, [4]uint32{8}, [4]uint32{}},
		{"EqualFloatFailure", 4, [4]uint32{math.Float32bits(1.0)}, [4]uint32{math.Float32bits(1.5)}, [4]uint32{}},
		{"NotEqualFailure", 3, [4]uint32{7}, [4]uint32{7}, [4]uint32{}},
		{"NearFailure", 4, [4]uint32{math.Float32bits(1.0)}, [4]uint32{math.Float32bits(1.5)}, [4]uint32{math.Float32bits(0.1)}},
	}
	for _, tc := range cases {
		t.Run(tc.function, func(t *testing.T) {
			inv := failures.byFunction[tc.function]
			out, result, err := runNativeHostJSON(t, host, inv, failures.manifestPath)
			if err != nil {
				t.Fatalf("host process did not complete normally: %v: %s", err, out)
			}
			if result.Status != "ASSERTION_FAILED" || result.StableCaseID != inv.c.StableID || result.Invocation != [3]uint32{} || result.AssertionID != 0 || result.Source != assertSource(inv, 0) || result.ABIVersion != 1 || result.Failed != 1 || result.ValueKind != tc.valueKind || result.Component != 1 || result.ExpectedBits != tc.expected || result.ActualBits != tc.actual || result.ToleranceBits != tc.tolerance {
				t.Fatalf("assert ABI matrix mismatch: %#v", result)
			}
		})
	}
}

func TestSdslvNativeHostExactlyOnceOrderAndFirstFailureContinuation(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "ExactlyOnceOperands.sdslvvalid"))
	for _, function := range []string{"AssertOperandsEvaluateExactlyOnce", "AssertOperandsEvaluateLeftToRight", "TheoryAndResourceOperandsEvaluateOnce"} {
		t.Run(function, func(t *testing.T) {
			inv := fixture.byFunction[function]
			first, result, err := runNativeHostJSON(t, host, inv, fixture.manifestPath)
			if err != nil {
				t.Fatalf("%s failed: %v: %s", function, err, first)
			}
			second, _, err := runNativeHostJSON(t, host, inv, fixture.manifestPath)
			if err != nil {
				t.Fatalf("%s rerun failed: %v: %s", function, err, second)
			}
			if first != second {
				t.Fatalf("%s JSON changed:\n%s\n!=\n%s", function, first, second)
			}
			if result.Status != "PASS" || result.ABIVersion != 1 || result.Failed != 0 || result.ValueKind != 0 || result.Component != 1 || result.ExpectedBits != [4]uint32{} || result.ActualBits != [4]uint32{} || result.ToleranceBits != [4]uint32{} {
				t.Fatalf("%s pass ABI mismatch: %#v", function, result)
			}
		})
	}

	inv := fixture.byFunction["FirstFailureDoesNotSuppressLaterOperandEvaluation"]
	out, result, err := runNativeHostJSON(t, host, inv, fixture.manifestPath)
	if err != nil {
		t.Fatalf("first-failure continuation fixture did not complete normally: %v: %s", err, out)
	}
	if result.Status != "ASSERTION_FAILED" || result.StableCaseID != inv.c.StableID || result.AssertionID != 0 || result.Source != assertSource(inv, 0) || result.ABIVersion != 1 || result.Failed != 1 || result.ValueKind != 3 || result.Component != 1 || result.ExpectedBits != [4]uint32{1} || result.ActualBits != [4]uint32{2} || result.ToleranceBits != [4]uint32{} {
		t.Fatalf("first-failure continuation ABI mismatch: %#v", result)
	}
}

func TestSdslvNativeHostNearSpecialValueMatrix(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "internal", "sdslv", "testdata", "language", "valid", "NearSpecialValues.sdslvvalid"))
	for _, function := range []string{"FiniteWithinTolerance", "EqualInfinities", "ZeroToleranceEqual"} {
		out, result, err := runNativeHostJSON(t, host, fixture.byFunction[function], fixture.manifestPath)
		if err != nil || result.Status != "PASS" || result.ABIVersion != 1 || result.Failed != 0 || result.ValueKind != 0 || result.Component != 1 {
			t.Fatalf("%s failed: %v: %s", function, err, out)
		}
	}
	checks := []struct {
		function                    string
		expected, actual, tolerance uint32
	}{
		{"FiniteOutsideTolerance", math.Float32bits(1.0), math.Float32bits(1.5), math.Float32bits(0.1)},
		{"OppositeInfinities", 0x7f800000, 0xff800000, math.Float32bits(1.0)},
		{"NaNExpected", 0x7fc00001, math.Float32bits(1.0), math.Float32bits(0.1)},
		{"NaNActual", math.Float32bits(1.0), 0x7fc00001, math.Float32bits(0.1)},
		{"NegativeRuntimeTolerance", math.Float32bits(1.0), math.Float32bits(1.0), math.Float32bits(-0.1)},
	}
	for _, check := range checks {
		t.Run(check.function, func(t *testing.T) {
			inv := fixture.byFunction[check.function]
			out, err := runNativeHost(t, host, inv, fixture.manifestPath)
			if err != nil {
				t.Fatalf("host process failed: %v: %s", err, out)
			}
			var result nativeResult
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "ASSERTION_FAILED" || result.AssertionID != 0 || result.Source != assertSource(inv, 0) || result.ABIVersion != 1 || result.Failed != 1 || result.ValueKind != 4 || result.Component != 1 || result.ExpectedBits != [4]uint32{check.expected} || result.ActualBits != [4]uint32{check.actual} || result.ToleranceBits != [4]uint32{check.tolerance} {
				t.Fatalf("Near ABI mismatch: %#v", result)
			}
		})
	}
}

func TestSdslvNativeHostExecutesTensorExecutionSuite(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M32b2", "TensorExecution.sdslvtest"))
	if got := len(fixture.groups); got != 2 {
		t.Fatalf("groups=%d, want 2 workgroup groups", got)
	}
	for _, function := range []string{
		"Rank1ElementwiseMap",
		"Rank3BatchedMatmul",
		"TensorGuardedReadsUseTestInput",
		"SgemmStyleTensorParity",
		"TensorInlineHlslExpression",
	} {
		out, result, err := runNativeHostJSON(t, host, fixture.byFunction[function], fixture.manifestPath)
		if err != nil || result.Status != "PASS" {
			t.Fatalf("%s failed: %v: %s", function, err, out)
		}
	}
	multi := fixture.byFunction["MultipleInvocationsKeepTensorStatePrivate"]
	first, result, err := runNativeHostJSON(t, host, multi, fixture.manifestPath)
	if err != nil || result.Status != "PASS" {
		t.Fatalf("multiple invocation case failed: %v: %s", err, first)
	}
	second, result2, err := runNativeHostJSON(t, host, multi, fixture.manifestPath)
	if err != nil || result2.Status != "PASS" {
		t.Fatalf("multiple invocation replay failed: %v: %s", err, second)
	}
	if first != second {
		t.Fatalf("multiple invocation replay changed:\n%s\n!=\n%s", first, second)
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

func TestSdslvM31bFlowStackSuitePassesOnNativeHost(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	t.Chdir(root)
	if err := os.Setenv("SDSLV_TEST_HOST", host); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "SDSL-V", "M31b", "FlowStacks.sdslvtest")
	var out bytes.Buffer
	if err := Execute(path, &out); err != nil {
		t.Fatalf("M31b flow stack suite failed: %v\n%s", err, out.String())
	}
	if strings.Count(out.String(), `"status":"PASS"`) != 13 {
		t.Fatalf("expected 13 passing M31b cases, got output:\n%s", out.String())
	}
}

func TestSdslvM31bFlowStackNativeHostCasesAreDeterministic(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	fixture := compileHostSuiteFromFile(t, filepath.Join(root, "examples", "SDSL-V", "M31b", "FlowStacks.sdslvtest"))
	for _, name := range []string{
		"LinearFallthroughLegacy",
		"GotoOnlyTransfer",
		"FinishOnlyFlowPreservesEpilogue",
		"OnePushPop",
		"NestedPushPopLifo",
		"SharedSubflowCallerSpecificReturn",
		"FinalStatePushReturnsToCompletion",
		"FinishWithNonemptyStack",
		"FlowFollowedByAssert",
		"MultipleInvocationsRemainDeterministic",
		"TheoryRowFlow[0]",
		"TheoryRowFlow[1]",
		"BarrierSubflowRemainsUniform",
	} {
		t.Run(name, func(t *testing.T) {
			inv, ok := fixture.byFunction[name]
			if !ok {
				inv, ok = fixture.byDisplay[name]
			}
			if !ok {
				t.Fatalf("missing M31b case %s", name)
			}
			first, firstResult, err := runNativeHostJSON(t, host, inv, fixture.manifestPath)
			if err != nil {
				t.Fatalf("first run failed: %v: %s", err, first)
			}
			second, secondResult, err := runNativeHostJSON(t, host, inv, fixture.manifestPath)
			if err != nil {
				t.Fatalf("second run failed: %v: %s", err, second)
			}
			if first != second {
				t.Fatalf("deterministic replay changed:\n%s\n!=\n%s", first, second)
			}
			if firstResult.Status != "PASS" || secondResult.Status != "PASS" || firstResult.StableCaseID != inv.c.StableID || firstResult.ABIVersion != 1 || firstResult.Failed != 0 || firstResult.Source != [2]uint32{} {
				t.Fatalf("unexpected pass contract: %#v", firstResult)
			}
		})
	}
}

func TestSdslvM31bStableCaseReplay(t *testing.T) {
	host := nativeHostExecutable(t)
	root := repoRoot(t)
	t.Chdir(root)
	if err := os.Setenv("SDSLV_TEST_HOST", host); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "SDSL-V", "M31b", "FlowStacks.sdslvtest")
	manifest, err := Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	caseID := ""
	for _, c := range manifest.Cases {
		if c.DisplayName == "NestedPushPopLifo" {
			caseID = c.StableID
			break
		}
	}
	if caseID == "" {
		t.Fatal("NestedPushPopLifo case missing")
	}
	var out bytes.Buffer
	if err := ExecuteWithOptions(path, &out, Options{CaseID: caseID}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Count(text, caseID) != 1 || strings.Count(text, `"status":"PASS"`) != 1 {
		t.Fatalf("stable-case replay output = %q", text)
	}
}
