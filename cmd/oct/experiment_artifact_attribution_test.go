package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestExperimentArtifactAttributionExperimentRootArtifactPrefixesMilestoneOutputsAndExcludesAuxiliary(t *testing.T) {
	root := createExperimentRoot(t)
	shared := filepath.Join(t.TempDir(), "artifact-results.octagon")

	writeMilestoneFile(t, root, "M0", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"m0\")", shared),
		"}",
	}, "\n")+"\n")
	writeMilestoneFile(t, root, "M1", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M1", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"m1\")", shared),
		"}",
	}, "\n")+"\n")
	writeMilestoneFile(t, root, "Mx2", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx2", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"mx\")", shared),
		"}",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	m0Output := filepath.Join(filepath.Dir(shared), "M0."+filepath.Base(shared))
	m1Output := filepath.Join(filepath.Dir(shared), "M1."+filepath.Base(shared))
	mxOutput := filepath.Join(filepath.Dir(shared), "Mx2."+filepath.Base(shared))
	if _, err := os.Stat(m0Output); err != nil {
		t.Fatalf("expected M0 prefixed output, err=%v", err)
	}
	if _, err := os.Stat(m1Output); err != nil {
		t.Fatalf("expected M1 prefixed output, err=%v", err)
	}
	if _, err := os.Stat(mxOutput); !os.IsNotExist(err) {
		t.Fatalf("expected no auxiliary output, stat err=%v", err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("expected unprefixed output path to remain absent, stat err=%v", err)
	}
	if !strings.Contains(stdout.String(), "[M0] RUN") || !strings.Contains(stdout.String(), "[M1] RUN") {
		t.Fatalf("expected milestone-labeled artifact logs, got %q", stdout.String())
	}
}

func TestExperimentArtifactAttributionExperimentRootBenchPrefixesMilestoneOutputsAndOctagonOut(t *testing.T) {
	root := createExperimentRoot(t)
	artifactPath := filepath.Join(t.TempDir(), "bench-artifact.octagon")
	benchOut := filepath.Join(t.TempDir(), "bench-run.octagon")

	writeMilestoneFile(t, root, "M0", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn BenchA() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"bench-a\")", artifactPath),
		"}",
	}, "\n")+"\n")
	writeMilestoneFile(t, root, "M1a", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M1a", "bench.octest", strings.Join([]string{
		"package Main",
		"[Benchmark]",
		"fn BenchB() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"bench-b\")", artifactPath),
		"}",
	}, "\n")+"\n")
	writeMilestoneFile(t, root, "Mx3", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx3", "bench.octest", "package Main\n[Benchmark]\nfn BenchAux() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"bench", root, "--octagon-out", benchOut}, &stdout, &stderr); err != nil {
		t.Fatalf("expected bench success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	for _, prefixed := range []string{
		filepath.Join(filepath.Dir(artifactPath), "M0."+filepath.Base(artifactPath)),
		filepath.Join(filepath.Dir(artifactPath), "M1a."+filepath.Base(artifactPath)),
		filepath.Join(filepath.Dir(benchOut), "M0."+filepath.Base(benchOut)),
		filepath.Join(filepath.Dir(benchOut), "M1a."+filepath.Base(benchOut)),
	} {
		if _, err := os.Stat(prefixed); err != nil {
			t.Fatalf("expected prefixed bench output %s: %v", prefixed, err)
		}
	}
	if _, err := os.Stat(benchOut); !os.IsNotExist(err) {
		t.Fatalf("expected unprefixed --octagon-out path to remain absent, stat err=%v", err)
	}
	if strings.Contains(stdout.String(), "BenchAux") {
		t.Fatalf("expected auxiliary benchmark exclusion, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[M0] RUN") || !strings.Contains(stdout.String(), "[M1a] RUN") {
		t.Fatalf("expected milestone-labeled benchmark logs, got %q", stdout.String())
	}
}

func TestExperimentArtifactAttributionDirectMilestoneArtifactDoesNotPrefixOutputs(t *testing.T) {
	root := createExperimentRoot(t)
	output := filepath.Join(t.TempDir(), "direct.octagon")

	writeMilestoneFile(t, root, "Mx1", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx1", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"direct\")", output),
		"}",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", filepath.Join(root, "Mx1")}, &stdout, &stderr); err != nil {
		t.Fatalf("expected direct milestone artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected direct unprefixed output, err=%v", err)
	}
	prefixed := filepath.Join(filepath.Dir(output), "Mx1."+filepath.Base(output))
	if _, err := os.Stat(prefixed); !os.IsNotExist(err) {
		t.Fatalf("expected no prefixed file during direct milestone run, stat err=%v", err)
	}
}

func TestExperimentArtifactAttributionArtifactCollisionSameFilenameAcrossMilestonesDoesNotOverwrite(t *testing.T) {
	root := createExperimentRoot(t)
	shared := filepath.Join(t.TempDir(), "plot.octagon")

	writeMilestoneFile(t, root, "M1", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M1", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"first\")", shared),
		"}",
	}, "\n")+"\n")
	writeMilestoneFile(t, root, "M2", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M2", "artifact.octest", strings.Join([]string{
		"package Main",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, \"second\")", shared),
		"}",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	onePath := filepath.Join(filepath.Dir(shared), "M1."+filepath.Base(shared))
	twoPath := filepath.Join(filepath.Dir(shared), "M2."+filepath.Base(shared))
	one, err := os.ReadFile(onePath)
	if err != nil {
		t.Fatalf("read first milestone output: %v", err)
	}
	two, err := os.ReadFile(twoPath)
	if err != nil {
		t.Fatalf("read second milestone output: %v", err)
	}
	if string(one) == string(two) {
		t.Fatalf("expected collision outputs to remain distinct, got %q", one)
	}
}
