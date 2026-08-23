package tester

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func conceptCapabilitiesM2Fixture(parts ...string) string {
	all := append([]string{"..", "..", "Language", "Tooling", "ConceptCapabilitiesM2", "valid"}, parts...)
	return filepath.Join(all...)
}

func TestConceptModeledNativeCapabilityIsDeniedThenExactlyGranted(t *testing.T) {
	target := conceptCapabilitiesM2Fixture("concept_capability_artifact.octest")
	t.Setenv("OCT_WRAPPER_PATH", "")
	var denied bytes.Buffer
	err := ExecuteArtifactsWithOptions(target, &denied, ArtifactOptions{OutputRoot: t.TempDir()})
	if err == nil || !strings.Contains(denied.String(), "requested but not approved") {
		t.Fatalf("expected requested-but-not-approved denial, err=%v output=%s", err, denied.String())
	}
	broader := ArtifactOptions{OutputRoot: t.TempDir(), NativeApprovals: []string{"Main:test-wrapper:NotOkRaw"}}
	if err := ExecuteArtifactsWithOptions(target, &bytes.Buffer{}, broader); err == nil || !strings.Contains(err.Error(), "broader than the discovered requests") {
		t.Fatalf("expected broader approval rejection, got %v", err)
	}
	unavailableOutput := &bytes.Buffer{}
	unavailable := ArtifactOptions{OutputRoot: t.TempDir(), NativeApprovals: []string{"Main:test-wrapper:EchoStringRaw"}}
	if err := ExecuteArtifactsWithOptions(target, unavailableOutput, unavailable); err == nil || !strings.Contains(unavailableOutput.String(), "approved native operation") || !strings.Contains(unavailableOutput.String(), "is unavailable") {
		t.Fatalf("expected approved-but-unavailable diagnostic, err=%v output=%s", err, unavailableOutput.String())
	}

	binDir := t.TempDir()
	name := "octxiliary-test-wrapper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, name), "./cmd/octxiliary-test-wrapper")
	cmd.Dir = filepath.Join("..", "..")
	if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
		t.Fatalf("build trusted test sidecar: %v\n%s", buildErr, output)
	}
	t.Setenv("OCT_WRAPPER_PATH", binDir)
	outputRoot := t.TempDir()
	report := &ArtifactReport{}
	options := ArtifactOptions{OutputRoot: outputRoot, NativeApprovals: []string{"Main:test-wrapper:EchoStringRaw"}, Report: report}
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(target, &stdout, options); err != nil {
		t.Fatalf("granted artifact failed: %v\n%s", err, stdout.String())
	}
	contents, err := os.ReadFile(filepath.Join(outputRoot, "nested", "native-result.txt"))
	if err != nil || string(contents) != "concept grant" {
		t.Fatalf("published native result = %q, err=%v", contents, err)
	}
	if len(report.Capabilities) != 1 || !report.Capabilities[0].Granted || !report.Capabilities[0].Dispatched || report.Capabilities[0].SidecarSHA256 == "" {
		t.Fatalf("incomplete capability provenance: %+v", report.Capabilities)
	}
	if report.Measurements.RequestAtomsDiscovered != 2 || report.Measurements.NormalizedRequestAtoms != 1 || report.Measurements.GrantsEvaluated != 1 || report.Measurements.ApprovalsAccepted != 1 || report.Measurements.NativeDispatches != 1 || report.Measurements.BackendGenerations != 0 || report.Measurements.ArtifactHostExecutablesCreated != 0 {
		t.Fatalf("unexpected capability measurements: %+v", report.Measurements)
	}
	report = &ArtifactReport{}
	options.Report = report
	stdout.Reset()
	if err := ExecuteArtifactsWithOptions(target, &stdout, options); err != nil {
		t.Fatalf("repeat artifact failed: %v", err)
	}
	if len(report.Artifacts) != 1 || report.Artifacts[0].Status != "unchanged" {
		t.Fatalf("repeat report = %+v", report.Artifacts)
	}
	entries, err := os.ReadDir(outputRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".go" || filepath.Ext(entry.Name()) == ".exe" {
			t.Fatalf("artifact evaluation created backend host file %s", entry.Name())
		}
	}
}

func TestConceptCapabilityDeclarationAndBrokerDiagnostics(t *testing.T) {
	cases := []struct{ path, want string }{
		{filepath.Join("..", "..", "Language", "Tooling", "ConceptCapabilitiesM2", "invalid", "missing_provider.octest"), "is not a package-local function"},
		{filepath.Join("..", "..", "Language", "Tooling", "ConceptCapabilitiesM2", "invalid", "request_must_be_concept.octest"), "requires a package-local record Concept"},
		{filepath.Join("..", "..", "Language", "Tooling", "ConceptCapabilitiesM2", "invalid", "effectful_discovery.octest"), "not statically discoverable"},
		{conceptCapabilitiesM2Fixture("no_request_artifact.octest"), "was not requested"},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			var stdout bytes.Buffer
			err := ExecuteArtifactsWithOptions(tc.path, &stdout, ArtifactOptions{OutputRoot: t.TempDir()})
			if err == nil || !strings.Contains(stdout.String()+err.Error(), tc.want) {
				t.Fatalf("expected %q, err=%v output=%s", tc.want, err, stdout.String())
			}
		})
	}

	t.Setenv("OCT_WRAPPER_PATH", "")
	var stdout bytes.Buffer
	err := ExecuteArtifactsWithOptions(conceptCapabilitiesM2Fixture("wrong_operation_artifact.octest"), &stdout, ArtifactOptions{OutputRoot: t.TempDir(), NativeApprovals: []string{"Main:test-wrapper:EchoStringRaw"}})
	if err == nil || !strings.Contains(stdout.String(), "Main:test-wrapper:NotOkRaw was not requested") {
		t.Fatalf("approval for one operation authorized another: err=%v output=%s", err, stdout.String())
	}
}

func artifactLanguageFixture(parts ...string) string {
	all := append([]string{"..", "..", "Language", "Tooling", "Artifacts"}, parts...)
	return filepath.Join(all...)
}

func compiledDataLanguageFixture(parts ...string) string {
	all := append([]string{"..", "..", "Language", "Tooling", "CompiledData", "valid"}, parts...)
	return filepath.Join(all...)
}

func TestCompiledDataStaticFactsAreSubjectScopedBeforeBackendPlanning(t *testing.T) {
	outputRoot := t.TempDir()
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(compiledDataLanguageFixture("proof_scope_publication.octest"), &stdout, ArtifactOptions{OutputRoot: outputRoot}); err != nil {
		t.Fatalf("proof-scope publication failed: %v\n%s", err, stdout.String())
	}
	checks := []struct {
		path       string
		wantLookup bool
	}{
		{path: "proved_scope.generated.go", wantLookup: true},
		{path: "cross_subject.generated.go", wantLookup: false},
		{path: "transformed_after_proof.generated.go", wantLookup: false},
	}
	for _, check := range checks {
		source, err := os.ReadFile(filepath.Join(outputRoot, "compiled", check.path))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Contains(source, []byte("LookupByID")); got != check.wantLookup {
			t.Fatalf("%s lookup presence = %v, want %v\n%s", check.path, got, check.wantLookup, source)
		}
	}
}

func TestBuildTimeArtifactEvaluationPublishesTypedOutputsWithoutBackend(t *testing.T) {
	outputRoot := t.TempDir()
	target := artifactLanguageFixture("valid", "build_time_artifact_evaluation.octest")
	preexisting := filepath.Join(outputRoot, "nested", "model.txt")
	if err := os.MkdirAll(filepath.Dir(preexisting), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preexisting, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := &ArtifactReport{}
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(target, &stdout, ArtifactOptions{OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("artifact evaluation failed: %v\n%s", err, stdout.String())
	}
	if report.Execution != "build-time-interpreted" || report.RequestedExecution != "interpreted" {
		t.Fatalf("unexpected execution report: %+v", report)
	}
	if len(report.Artifacts) != 6 {
		t.Fatalf("expected six typed outputs, got %+v", report.Artifacts)
	}
	for _, artifact := range report.Artifacts {
		if artifact.Status != "produced" {
			t.Fatalf("first publication status for %s = %s", artifact.Path, artifact.Status)
		}
		if _, err := os.Stat(filepath.Join(outputRoot, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("missing published artifact %s: %v", artifact.Path, err)
		}
	}

	modelPath := filepath.Join(outputRoot, "nested", "model.txt")
	firstInfo, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	firstModTime := firstInfo.ModTime()
	time.Sleep(20 * time.Millisecond)
	report = &ArtifactReport{}
	stdout.Reset()
	if err := ExecuteArtifactsWithOptions(target, &stdout, ArtifactOptions{Execution: "compiled", OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("compiled compatibility delegation failed: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "no backend is generated or compiled") {
		t.Fatalf("missing explicit compatibility delegation: %s", stdout.String())
	}
	for _, artifact := range report.Artifacts {
		if artifact.Status != "unchanged" {
			t.Fatalf("second publication status for %s = %s", artifact.Path, artifact.Status)
		}
	}
	secondInfo, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !secondInfo.ModTime().Equal(firstModTime) {
		t.Fatalf("unchanged artifact modification time changed: %v -> %v", firstModTime, secondInfo.ModTime())
	}
}

func TestArtifactCapabilityRejectsUnsafePathsDuplicatesEffectsAndFailures(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"path_traversal.octest", "escapes the artifact output root"},
		{"absolute_path.octest", "must be non-empty and relative"},
		{"duplicate_output.octest", "duplicate artifact output path"},
		{"ambient_write.octest", "outside Artifact.Write*"},
		{"fallible_failure.octest", "artifact failure is visible"},
		{"static_assert_duplicate.octest", "static assertion failed in RejectDuplicateIDs: duplicate ID 7"},
		{"static_assert_unsorted.octest", "static assertion failed in RejectUnsortedIndex: row-ID index is not sorted"},
		{"table_with_extent_mismatch.octest", "replacement column 'Name' has extent 1; expected 2"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			var stdout bytes.Buffer
			outputRoot := t.TempDir()
			err := ExecuteArtifactsWithOptions(artifactLanguageFixture("invalid", tc.file), &stdout, ArtifactOptions{OutputRoot: outputRoot})
			if err == nil || !strings.Contains(stdout.String()+err.Error(), tc.want) {
				t.Fatalf("expected %q, got err=%v output=%s", tc.want, err, stdout.String())
			}
			if tc.file == "fallible_failure.octest" {
				if _, statErr := os.Stat(filepath.Join(outputRoot, "must-not-publish.txt")); !os.IsNotExist(statErr) {
					t.Fatalf("failed artifact evaluation published staged output: %v", statErr)
				}
			}
			if tc.file == "static_assert_duplicate.octest" || tc.file == "static_assert_unsorted.octest" {
				if _, statErr := os.Stat(filepath.Join(outputRoot, "must-not-publish.go")); !os.IsNotExist(statErr) {
					t.Fatalf("failed proof assertion emitted specialized artifact: %v", statErr)
				}
			}
		})
	}
}

func TestArtifactEntryPointAndCapabilityCannotBeUsedAsOrdinaryRuntime(t *testing.T) {
	program, err := project.LoadForTest(artifactLanguageFixture("invalid", "direct_entry_call.octest"))
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err == nil || !strings.Contains(err.Error(), "directly calls artifact entry point") {
		t.Fatalf("expected direct artifact entry diagnostic, got %v", err)
	}

	program, err = project.Load(artifactLanguageFixture("invalid", "artifact_write_outside_phase.oct"))
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	if _, err := interpret.ExecuteMain(program, nil); err == nil || !strings.Contains(err.Error(), "only during `oct artifact` evaluation") {
		t.Fatalf("expected phase capability diagnostic, got %v", err)
	}
}

func TestArtifactDiscoveryOrderIsDeterministic(t *testing.T) {
	target := artifactLanguageFixture("invalid", "duplicate_output.octest")
	program, err := project.LoadForTest(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	artifacts, err := discoverArtifactCases(program, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 || artifacts[0].name != "FirstWriter" || artifacts[1].name != "SecondWriter" {
		t.Fatalf("unexpected artifact discovery order: %+v", artifacts)
	}
}

func TestArtifactPublisherRejectsPortableAbsoluteAndTraversalSpellings(t *testing.T) {
	publisher, err := newArtifactPublisher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.close()
	for _, path := range []string{"/absolute.txt", `C:\absolute.txt`, `..\escape.txt`} {
		if _, err := publisher.StageArtifactOutput(interpret.ArtifactOutputRequest{Path: path, Function: "Probe", SourcePath: "probe.octest"}); err == nil {
			t.Fatalf("expected portable path rejection for %q", path)
		}
	}
}
