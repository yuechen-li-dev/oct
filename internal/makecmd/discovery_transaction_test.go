package makecmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/project"
)

const fakeDiscoveryKind = "test.fake-discovery"

type fakeCollector struct {
	paths []string
	err   error
	calls int
	seen  []string
}

func (f *fakeCollector) Supports(spec DiscoverySpec) bool {
	return spec.Kind == fakeDiscoveryKind && spec.SchemaVersion == "v1"
}

func (f *fakeCollector) Collect(_ DiscoverySpec, outputPath string) (DiscoveredInputs, error) {
	f.calls++
	f.seen = append(f.seen, outputPath)
	if f.err != nil {
		return DiscoveredInputs{}, f.err
	}
	return DiscoveredInputs{Paths: append([]string(nil), f.paths...), Provenance: DiscoveryProvenance{Collector: fakeDiscoveryKind, Detail: "deterministic fake collector"}}, nil
}

type transactionHarness struct {
	root      string
	plan      Plan
	command   CommandTarget
	targets   map[string]*target
	collector *fakeCollector
	processes int
	attempts  []string
	process   commandProcessRunner
	writer    makeStateWriter
}

func newTransactionHarness(t *testing.T) *transactionHarness {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{"src/main.cpp", "include/main.hpp"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(path), 0644); err != nil {
			t.Fatal(err)
		}
	}
	command := CommandTarget{
		Name:    "Compile",
		Inputs:  []string{"src/main.cpp"},
		Outputs: []string{"out/main.obj"},
		Program: "fake-cl",
		Args:    []string{"/sourceDependencies"},
		Discovery: &DiscoverySpec{
			Kind:          fakeDiscoveryKind,
			SchemaVersion: "v1",
			OutputArgument: RuntimeOutputArgument{
				Position: 1,
			},
			ExpectedSourceIdentity:         "src/main.cpp",
			ExpectedPhysicalOutputIdentity: "out/main.obj",
		},
	}
	plan := Plan{Default: "Compile", Config: Config{Profile: "test", StateDir: ".octmake", Staleness: "Timestamp"}, CommandTargets: []CommandTarget{command}}
	targets, err := validate(plan)
	if err != nil {
		t.Fatal(err)
	}
	h := &transactionHarness{root: root, plan: plan, command: command, targets: targets, collector: &fakeCollector{paths: []string{"include/main.hpp"}}}
	h.process = func(command CommandTarget, root string, _, _ io.Writer) processOutcome {
		h.processes++
		if command.Discovery != nil {
			h.attempts = append(h.attempts, command.Args[command.Discovery.OutputArgument.Position])
		}
		for _, output := range command.Outputs {
			full := filepath.Join(root, filepath.FromSlash(output))
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return processOutcome{Err: err}
			}
			if err := os.WriteFile(full, []byte("object"), 0644); err != nil {
				return processOutcome{Err: err}
			}
		}
		return processOutcome{}
	}
	h.writer = writeMakeStateAtomic
	return h
}

func (h *transactionHarness) executor() executor {
	return executor{collectors: discoveryCollectors{fakeDiscoveryKind: h.collector}, process: h.process, writeState: h.writer}
}

func (h *transactionHarness) run() ([]decision, error) {
	return runWithExecutor([]string{"Compile"}, h.targets, h.root, filepath.Join(h.root, "Make.oct"), project.Program{}, h.plan, "Compile", Options{}, io.Discard, io.Discard, h.executor())
}

func (h *transactionHarness) resetTargets(t *testing.T) {
	t.Helper()
	targets, err := validate(h.plan)
	if err != nil {
		t.Fatal(err)
	}
	h.targets = targets
}

func TestDiscoveryTransactionSuccessfulProcessCollectionAndAtomicStateCommit(t *testing.T) {
	h := newTransactionHarness(t)
	decs, err := h.run()
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(decs) != 1 || decs[0].Status != "Ran" || len(decs[0].DiscoveredInputs) != 1 {
		t.Fatalf("decisions = %#v", decs)
	}
	state := loadState(h.root, h.plan)
	entry, ok := state.Targets["Compile"]
	if !ok || entry.CommandHash != commandHash(h.command) || entry.Discovery == nil {
		t.Fatalf("committed state = %#v", state)
	}
	if entry.Discovery.Kind != fakeDiscoveryKind || entry.Discovery.ActionHash != entry.CommandHash || len(entry.Discovery.DiscoveredInputs) != 1 {
		t.Fatalf("discovery state = %#v", entry.Discovery)
	}
	if state.Version != "1" {
		t.Fatalf("state version = %q, want 1", state.Version)
	}
}

func TestDiscoveryTransactionProcessFailureSkipsCollectionAndState(t *testing.T) {
	h := newTransactionHarness(t)
	h.process = func(CommandTarget, string, io.Writer, io.Writer) processOutcome {
		h.processes++
		return processOutcome{ExitCode: 7, Err: errors.New("process failed")}
	}
	decs, err := h.run()
	if err == nil || len(decs) != 1 || decs[0].FailurePhase != "Process" {
		t.Fatalf("err=%v decisions=%#v", err, decs)
	}
	if h.collector.calls != 0 {
		t.Fatalf("collector calls = %d, want 0", h.collector.calls)
	}
	if _, statErr := os.Stat(filepath.Join(h.root, ".octmake", "state.octagon")); !os.IsNotExist(statErr) {
		t.Fatalf("process failure wrote state: %v", statErr)
	}
}

func TestDiscoveryTransactionFailuresPreservePriorState(t *testing.T) {
	h := newTransactionHarness(t)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(h.root, ".octmake", "state.octagon")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	h.plan.Config.Staleness = "Always"
	h.resetTargets(t)
	successfulProcess := h.process
	h.process = func(CommandTarget, string, io.Writer, io.Writer) processOutcome {
		return processOutcome{ExitCode: 9, Err: errors.New("process failed after prior success")}
	}
	decs, err := h.run()
	if err == nil || len(decs) != 1 || decs[0].FailurePhase != "Process" {
		t.Fatalf("err=%v decisions=%#v", err, decs)
	}
	after, readErr := os.ReadFile(statePath)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("prior state changed after process failure: %v\n%s", readErr, after)
	}
	assertFailureArtifactAndRetry(t, h, decs, "Process")

	h.process = successfulProcess
	h.collector.err = errors.New("malformed discovery")
	decs, err = h.run()
	if err == nil || len(decs) != 1 || decs[0].FailurePhase != "Discovery" {
		t.Fatalf("err=%v decisions=%#v", err, decs)
	}
	after, readErr = os.ReadFile(statePath)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("prior state changed after collection failure: %v\n%s", readErr, after)
	}
	assertFailureArtifactAndRetry(t, h, decs, "Discovery")
}

func assertFailureArtifactAndRetry(t *testing.T, h *transactionHarness, decs []decision, phase string) {
	t.Helper()
	path, err := maybeFailureArtifact(Options{}, h.plan, h.root, filepath.Join(h.root, "Make.oct"), decs, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(body), `Phase: "`+phase+`"`) {
		t.Fatalf("failure artifact phase=%q err=%v\n%s", phase, err, body)
	}
	h.plan.Config.Staleness = "Timestamp"
	h.resetTargets(t)
	staleNow, reason, err := stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", loadState(h.root, h.plan), h.executor().collectors)
	if err != nil || !staleNow || reason != "PreviousFailure" {
		t.Fatalf("%s failure must force retry: stale=%t reason=%q err=%v", phase, staleNow, reason, err)
	}
	h.plan.Config.Staleness = "Always"
	h.resetTargets(t)
}

func TestDiscoveryTransactionStatePersistenceFailureIsNotCacheable(t *testing.T) {
	h := newTransactionHarness(t)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(h.root, ".octmake", "state.octagon")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	h.plan.Config.Staleness = "Always"
	h.resetTargets(t)
	h.writer = func(string, Plan, makeState) error { return errors.New("disk full") }
	decs, err := h.run()
	if err == nil || len(decs) != 1 || decs[0].FailurePhase != "StatePersistence" || decs[0].Status != "Failed" {
		t.Fatalf("err=%v decisions=%#v", err, decs)
	}
	after, readErr := os.ReadFile(statePath)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("persistence failure replaced prior state: %v\n%s", readErr, after)
	}
	path, artifactErr := maybeFailureArtifact(Options{}, h.plan, h.root, filepath.Join(h.root, "Make.oct"), decs, time.Now())
	if artifactErr != nil {
		t.Fatal(artifactErr)
	}
	body, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(body), `Phase: "StatePersistence"`) || !strings.Contains(string(body), `Reason: "StatePersistenceFailed"`) {
		t.Fatalf("phase artifact error=%v\n%s", readErr, body)
	}
	h.plan.Config.Staleness = "Timestamp"
	h.resetTargets(t)
	staleNow, reason, staleErr := stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", loadState(h.root, h.plan), h.executor().collectors)
	if staleErr != nil || !staleNow || reason != "PreviousFailure" {
		t.Fatalf("persistence failure must force retry: stale=%t reason=%q err=%v", staleNow, reason, staleErr)
	}
	h.writer = writeMakeStateAtomic
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	if pending, markerErr := actionHasFailureMarker(h.root, h.plan, h.targets["Compile"]); markerErr != nil || pending {
		t.Fatalf("successful commit did not clear failure marker: pending=%t err=%v", pending, markerErr)
	}
}

func TestDiscoveryStateAbsenceAndInvalidDependenciesForceRebuild(t *testing.T) {
	h := newTransactionHarness(t)
	output := filepath.Join(h.root, "out", "main.obj")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("object"), 0644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(h.root, ".octmake", "state.octagon")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}
	oldState := "MakeState {\n    Version: 0\n    Backend: \"direct\"\n    LastRunTarget: \"Compile\"\n    Targets: [\n        MakeTargetState {\n            Name: \"Compile\"\n            Kind: \"command\"\n            CommandHash: \"" + commandHash(h.command) + "\"\n        }\n    ]\n}\n"
	if err := os.WriteFile(statePath, []byte(oldState), 0644); err != nil {
		t.Fatal(err)
	}
	staleNow, reason, err := stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", loadState(h.root, h.plan), h.executor().collectors)
	if err != nil || !staleNow || reason != "DiscoveryStateAbsent" {
		t.Fatalf("old state stale=%t reason=%q err=%v", staleNow, reason, err)
	}

	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(h.root, "include", "main.hpp")
	if err := os.Remove(dependency); err != nil {
		t.Fatal(err)
	}
	staleNow, reason, err = stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", loadState(h.root, h.plan), h.executor().collectors)
	if err != nil || !staleNow || reason != "DiscoveredDependencyMissing" {
		t.Fatalf("missing dependency stale=%t reason=%q err=%v", staleNow, reason, err)
	}
	if err := os.WriteFile(dependency, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	older := time.Now().Add(-time.Minute)
	newer := time.Now().Add(time.Minute)
	if err := os.Chtimes(output, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dependency, newer, newer); err != nil {
		t.Fatal(err)
	}
	staleNow, reason, err = stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", loadState(h.root, h.plan), h.executor().collectors)
	if err != nil || !staleNow || reason != "DiscoveredDependencyNewerThanOutput" {
		t.Fatalf("newer dependency stale=%t reason=%q err=%v", staleNow, reason, err)
	}
}

func TestDiscoveryStateSourceOutputAndActionMismatchForceRebuild(t *testing.T) {
	h := newTransactionHarness(t)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	check := func(mutate func(*makeDiscoveryState), want string) {
		t.Helper()
		state := loadState(h.root, h.plan)
		mutate(state.discovery["Compile"])
		staleNow, reason, err := stale(h.targets["Compile"], h.root, map[string]bool{}, h.plan, "Timestamp", state, h.executor().collectors)
		if err != nil || !staleNow || reason != want {
			t.Fatalf("want %s; stale=%t reason=%q err=%v", want, staleNow, reason, err)
		}
	}
	check(func(discovery *makeDiscoveryState) {
		discovery.ExpectedSourceIdentity = filepath.Join(h.root, "src", "other.cpp")
	}, "DiscoverySourceOutputIdentityMismatch")
	check(func(discovery *makeDiscoveryState) {
		discovery.ExpectedPhysicalOutputIdentity = filepath.Join(h.root, "out", "other.obj")
	}, "DiscoverySourceOutputIdentityMismatch")
	check(func(discovery *makeDiscoveryState) { discovery.ActionName = "OtherCompile" }, "DiscoveryActionMismatch")
}

func TestDiscoveryTransactionHashAttemptIsolationAndNoOp(t *testing.T) {
	h := newTransactionHarness(t)
	stable := commandHash(h.command)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	if commandHash(h.command) != stable {
		t.Fatalf("attempt-local output changed stable hash")
	}
	pending := filepath.Join(h.root, ".octmake", "discovery", "unrelated-pending.json")
	if err := os.MkdirAll(filepath.Dir(pending), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pending, []byte("not committed"), 0644); err != nil {
		t.Fatal(err)
	}
	decs, err := h.run()
	if err != nil || len(decs) != 1 || decs[0].Status != "Skipped" || h.processes != 1 || h.collector.calls != 1 {
		t.Fatalf("unchanged inputs did not no-op: err=%v decisions=%#v processes=%d collections=%d", err, decs, h.processes, h.collector.calls)
	}

	h.plan.Config.Staleness = "Always"
	h.resetTargets(t)
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}
	if len(h.attempts) != 2 || h.attempts[0] == h.attempts[1] || strings.Contains(h.attempts[0], stable) {
		t.Fatalf("attempt paths=%q, want unique paths outside the stable action hash", h.attempts)
	}
}

func TestDiscoveryTransactionEqualBasenameVariantsRemainIsolated(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"a/main.cpp", "b/main.cpp", "a/main.hpp", "b/main.hpp"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(path), 0644); err != nil {
			t.Fatal(err)
		}
	}
	command := func(name, source, output string) CommandTarget {
		return CommandTarget{Name: name, Inputs: []string{source}, Outputs: []string{output}, Program: "fake-cl", Args: []string{"/sourceDependencies"}, Discovery: &DiscoverySpec{Kind: fakeDiscoveryKind, SchemaVersion: "v1", OutputArgument: RuntimeOutputArgument{Position: 1}, ExpectedSourceIdentity: source, ExpectedPhysicalOutputIdentity: output}}
	}
	plan := Plan{Default: "one", Config: Config{StateDir: ".octmake", Staleness: "Timestamp"}, CommandTargets: []CommandTarget{command("one", "a/main.cpp", "out/a/main.obj"), command("two", "b/main.cpp", "out/b/main.obj")}}
	targets, err := validate(plan)
	if err != nil {
		t.Fatal(err)
	}
	collector := &fakeCollector{}
	attempts := []string{}
	process := func(command CommandTarget, root string, _, _ io.Writer) processOutcome {
		attempts = append(attempts, command.Args[1])
		output := filepath.Join(root, filepath.FromSlash(command.Outputs[0]))
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return processOutcome{Err: err}
		}
		if err := os.WriteFile(output, []byte("object"), 0644); err != nil {
			return processOutcome{Err: err}
		}
		if strings.HasPrefix(command.Name, "one") {
			collector.paths = []string{"a/main.hpp"}
		} else {
			collector.paths = []string{"b/main.hpp"}
		}
		return processOutcome{}
	}
	exec := executor{collectors: discoveryCollectors{fakeDiscoveryKind: collector}, process: process, writeState: writeMakeStateAtomic}
	if _, err := runWithExecutor([]string{"one", "two"}, targets, root, filepath.Join(root, "Make.oct"), project.Program{}, plan, "two", Options{}, io.Discard, io.Discard, exec); err != nil {
		t.Fatal(err)
	}
	state := loadState(root, plan)
	one, two := state.discovery["one"], state.discovery["two"]
	if one == nil || two == nil || one.ExpectedSourceIdentity == two.ExpectedSourceIdentity || one.ExpectedPhysicalOutputIdentity == two.ExpectedPhysicalOutputIdentity || len(attempts) != 2 || attempts[0] == attempts[1] {
		t.Fatalf("variants collided: one=%#v two=%#v attempts=%q", one, two, attempts)
	}
}

func TestMSVCSourceDependenciesCollectorKeepsJSONBehindCollector(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "src", "main.cpp")
	header := filepath.Join(root, "include", "main.hpp")
	transitive := filepath.Join(root, "include", "transitive.hpp")
	for _, path := range []string{source, header, transitive} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("input"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	report := filepath.Join(root, "source-dependencies.json")
	body, err := os.ReadFile(filepath.Join("testdata", "msvc_source_dependencies_1.2.json"))
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.NewReplacer(
		"C:/fixture/space path/main.c", filepath.ToSlash(source),
		"C:/fixture/space path/direct.h", filepath.ToSlash(header),
		"C:/fixture/space path/transitive.h", filepath.ToSlash(transitive),
	).Replace(string(body)))
	if err := os.WriteFile(report, body, 0644); err != nil {
		t.Fatal(err)
	}
	spec := DiscoverySpec{Kind: MSVCSourceDependenciesKind, SchemaVersion: MSVCSourceDependenciesSchemaV1, ExpectedSourceIdentity: filepath.ToSlash(source)}
	result, err := (msvcSourceDependenciesCollector{}).Collect(spec, report)
	if err != nil || len(result.Paths) != 2 || result.Paths[0] != filepath.ToSlash(header) || result.Paths[1] != filepath.ToSlash(transitive) || result.Provenance.Collector != MSVCSourceDependenciesKind {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	wrong := spec
	wrong.ExpectedSourceIdentity = filepath.ToSlash(filepath.Join(root, "src", "other.cpp"))
	if _, err := (msvcSourceDependenciesCollector{}).Collect(wrong, report); err == nil {
		t.Fatal("source identity mismatch was accepted")
	}
}

func TestNativeCompileDiscoveryUsesEphemeralArgumentSlot(t *testing.T) {
	args := []string{"/nologo", "/c", "src/main.cpp", "/sourceDependencies"}
	spec := nativeCompileDiscovery("src/main.cpp", "out/main.obj", args)
	if runtime.GOOS != "windows" {
		if spec != nil {
			t.Fatalf("non-Windows native compile discovery = %#v, want nil", spec)
		}
		return
	}
	if spec == nil || spec.Kind != MSVCSourceDependenciesKind || spec.SchemaVersion != MSVCSourceDependenciesSchemaV1 || spec.OutputArgument.Position != len(args) || spec.ExpectedSourceIdentity != "src/main.cpp" || spec.ExpectedPhysicalOutputIdentity != "out/main.obj" {
		t.Fatalf("MSVC discovery spec = %#v", spec)
	}
}
