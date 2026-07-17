package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/newpkg"
	"github.com/yuechen-li-dev/oct/internal/tester"
)

func TestStructuredResultRetainsFallbackDisclosure(t *testing.T) {
	result := baseStructuredResult(
		"oct test",
		"fixture.octest",
		"test",
		"auto",
		time.Now(),
		"PASS Main.Valid (fixture.octest)\nExecution summary: compiled: 0 interpreted fallback: 1\nResult: 1 passed, 0 failed, 0 skipped\n",
		nil,
	)
	if result.Execution.CompiledCases != 0 || result.Execution.InterpretedFallbacks != 1 {
		t.Fatalf("fallback disclosure = %#v", result.Execution)
	}
	if result.Summary.Discovered != 1 || result.Summary.Passed != 1 || !result.OK {
		t.Fatalf("summary = %#v", result)
	}
}

func TestStructuredResultCountsMilestonePrefixedOutcomes(t *testing.T) {
	result := baseStructuredResult(
		"oct test",
		"experiment",
		"test",
		"auto",
		time.Now(),
		"MILESTONE M0 (test)\n[M0] PASS Main.Valid (fixture.octest)\n[M0] Execution summary: compiled: 1 interpreted fallback: 0\n[M0] Result: 1 passed, 0 failed, 0 skipped\nMILESTONE PASS M0 (test)\n",
		nil,
	)
	if result.Summary.Discovered != 1 || result.Summary.Passed != 1 {
		t.Fatalf("milestone summary = %#v", result.Summary)
	}
	if result.Execution.CompiledCases != 1 || result.Execution.InterpretedFallbacks != 0 {
		t.Fatalf("milestone execution = %#v", result.Execution)
	}
}

func TestExecuteTestJSONUsesExistingLanguageContract(t *testing.T) {
	target := filepath.Join("..", "..", "Language", "Types", "UnitsM1", "valid")
	var output bytes.Buffer
	if err := executeTestJSON(target, &output, tester.TestOptions{Execution: "auto"}); err != nil {
		t.Fatalf("structured test command failed: %v\n%s", err, output.String())
	}
	var result structuredCommandResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode structured result: %v\n%s", err, output.String())
	}
	if result.SchemaVersion != structuredSchemaVersion || result.ExitStatus != 0 || result.Summary.Passed != 1 {
		t.Fatalf("unexpected structured test result: %#v", result)
	}
	if len(result.DiscoveredTestFiles) != 1 || result.DiscoveredTestFiles[0] != "signed_exponents_and_hz_alias_m1.octest" {
		t.Fatalf("discovery = %#v", result.DiscoveredTestFiles)
	}
}

func TestExecuteTestJSONUsesCanonicalExperimentMilestoneDiscovery(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "ExampleExperiment")
	if err := newpkg.Write(newpkg.Options{Kind: newpkg.KindExperiment, Name: "ExampleExperiment", Dir: target}); err != nil {
		t.Fatalf("scaffold experiment: %v", err)
	}
	var output bytes.Buffer
	if err := executeTestJSON(target, &output, tester.TestOptions{Execution: "auto"}); err != nil {
		t.Fatalf("structured experiment command failed: %v\n%s", err, output.String())
	}
	var result structuredCommandResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode structured result: %v\n%s", err, output.String())
	}
	if result.Summary.Passed != 1 || result.Summary.Discovered != 1 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.DiscoveredTestFiles) != 1 || result.DiscoveredTestFiles[0] != "M0/example_experiment_m0.octest" {
		t.Fatalf("discovery = %#v", result.DiscoveredTestFiles)
	}
}
