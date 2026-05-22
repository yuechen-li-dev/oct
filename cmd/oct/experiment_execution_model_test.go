package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExperimentExecutionModelExperimentRootTestCommandUsesCanonicalMilestones(t *testing.T) {
	root := createExperimentRoot(t)
	writeMilestoneFile(t, root, "M0", "suite.oct", "package Main\nfn Stable() -> Int { return 1 }\n")
	writeMilestoneFile(t, root, "M0", "suite.octest", "package Main\n[Fact]\nfn CanonicalZeroPasses() -> Void { Assert.Equal(1, 1, \"canonical milestone executes\") }\n")
	writeMilestoneFile(t, root, "M1", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M1", "suite.octest", "package Main\n[Fact]\nfn CanonicalOneFails() -> Void { let x = 1 / 0 Print(x) }\n")
	writeMilestoneFile(t, root, "M2", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M2", "suite.octest", "package Main\n[Fact]\nfn CanonicalTwoPasses() -> Void { Assert.Equal(1, 1, \"canonical milestone executes\") }\n")
	writeMilestoneFile(t, root, "M2a", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M2a", "suite.octest", "package Main\n[Fact]\nfn CanonicalTwoAPasses() -> Void { Assert.Equal(1, 1, \"canonical milestone executes\") }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.octest", "package Main\n[Fact]\nfn AuxiliaryPassesButExcluded() -> Void { Assert.Equal(1, 1, \"auxiliary milestone executes when selected\") }\n")
	writeMilestoneFile(t, root, "Manual", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Manual", "suite.octest", "package Main\n[Fact]\nfn NonMilestoneExcluded() -> Void { Assert.Equal(1, 1, \"non-milestone executes when selected\") }\n")

	stdout, stderr, err := executeCLI("test", root)
	if err == nil {
		t.Fatalf("expected experiment root command to fail when canonical milestone fails")
	}
	if !strings.Contains(stderr, "test failed: 1 milestone(s) failed: M1") {
		t.Fatalf("expected milestone failure summary, got stderr=%q", stderr)
	}
	for _, milestone := range []string{"M0", "M1", "M2", "M2a"} {
		if !strings.Contains(stdout, "MILESTONE "+milestone+" (test)") {
			t.Fatalf("expected milestone heading for %s, got %q", milestone, stdout)
		}
	}
	m0 := strings.Index(stdout, "MILESTONE M0 (test)")
	m1 := strings.Index(stdout, "MILESTONE M1 (test)")
	m2 := strings.Index(stdout, "MILESTONE M2 (test)")
	m2a := strings.Index(stdout, "MILESTONE M2a (test)")
	if !(m0 >= 0 && m1 > m0 && m2 > m1 && m2a > m2) {
		t.Fatalf("expected deterministic milestone order M0,M1,M2,M2a, got %q", stdout)
	}
	if strings.Contains(stdout, "AuxiliaryPassesButExcluded") {
		t.Fatalf("expected auxiliary milestone to be excluded by default, got %q", stdout)
	}
	if strings.Contains(stdout, "NonMilestoneExcluded") {
		t.Fatalf("expected non-milestone directory to be excluded, got %q", stdout)
	}
	if !strings.Contains(stdout, "MILESTONE FAIL M1 (test)") {
		t.Fatalf("expected milestone-specific failure output, got %q", stdout)
	}
}

func TestExperimentExecutionModelDirectMilestonePathRunsOnlyTargetedMilestone(t *testing.T) {
	root := createExperimentRoot(t)
	writeMilestoneFile(t, root, "M0", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "suite.octest", "package Main\n[Fact]\nfn CanonicalPasses() -> Void { Assert.Equal(1, 1, \"canonical milestone executes\") }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.octest", "package Main\n[Fact]\nfn AuxiliaryRunsWhenDirectlyTargeted() -> Void { Assert.Equal(1, 1, \"targeted auxiliary milestone executes\") }\n")

	stdout, stderr, err := executeCLI("test", filepath.Join(root, "Mx1"))
	if err != nil {
		t.Fatalf("expected direct auxiliary milestone path to pass: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.AuxiliaryRunsWhenDirectlyTargeted") {
		t.Fatalf("expected targeted auxiliary milestone to execute, got %q", stdout)
	}
	if strings.Contains(stdout, "CanonicalPasses") {
		t.Fatalf("expected only targeted milestone to run, got %q", stdout)
	}
}

func TestExperimentExecutionModelExperimentRootArtifactCommandUsesCanonicalMilestones(t *testing.T) {
	root := createExperimentRoot(t)
	writeMilestoneFile(t, root, "M0", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "suite.octest", "package Main\n[Artifact]\nfn CanonicalArtifact() -> Void { Print(\"canonical\") }\n")
	writeMilestoneFile(t, root, "M2", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M2", "suite.octest", "package Main\n[Artifact]\nfn CanonicalArtifactTwo() -> Void { Print(\"canonical-two\") }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx1", "suite.octest", "package Main\n[Artifact]\nfn AuxiliaryArtifactExcluded() -> Void { Print(\"aux\") }\n")

	stdout, stderr, err := executeCLI("artifact", root)
	if err != nil {
		t.Fatalf("expected experiment artifact command to pass: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "MILESTONE M0 (artifact)") || !strings.Contains(stdout, "MILESTONE M2 (artifact)") {
		t.Fatalf("expected milestone headings for canonical artifact execution, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Main.CanonicalArtifact") || !strings.Contains(stdout, "PASS Main.CanonicalArtifactTwo") {
		t.Fatalf("expected canonical artifact output, got %q", stdout)
	}
	if strings.Contains(stdout, "AuxiliaryArtifactExcluded") {
		t.Fatalf("expected auxiliary milestone exclusion for artifact, got %q", stdout)
	}
}

func TestExperimentExecutionModelExperimentRootBenchCommandUsesCanonicalMilestones(t *testing.T) {
	root := createExperimentRoot(t)
	writeMilestoneFile(t, root, "M0", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "suite.octest", "package Main\n[Benchmark]\nfn CanonicalBench() -> Void { return }\n")
	writeMilestoneFile(t, root, "M1a", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M1a", "suite.octest", "package Main\n[Benchmark]\nfn CanonicalBenchA() -> Void { return }\n")
	writeMilestoneFile(t, root, "Mx2", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "Mx2", "suite.octest", "package Main\n[Benchmark]\nfn AuxiliaryBenchExcluded() -> Void { return }\n")

	stdout, stderr, err := executeCLI("bench", root)
	if err != nil {
		t.Fatalf("expected experiment benchmark command to pass: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "MILESTONE M0 (bench)") || !strings.Contains(stdout, "MILESTONE M1a (bench)") {
		t.Fatalf("expected milestone headings for canonical benchmark execution, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Main.CanonicalBench") || !strings.Contains(stdout, "PASS Main.CanonicalBenchA") {
		t.Fatalf("expected canonical benchmark output, got %q", stdout)
	}
	if strings.Contains(stdout, "AuxiliaryBenchExcluded") {
		t.Fatalf("expected auxiliary benchmark exclusion, got %q", stdout)
	}
}

func TestExperimentExecutionModelExperimentRootSkipsSharedSupportFolder(t *testing.T) {
	root := createExperimentRoot(t)
	writeMilestoneFile(t, root, "M0", "suite.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeMilestoneFile(t, root, "M0", "suite.octest", "package Main\n[Fact]\nfn CanonicalPasses() -> Void { Assert.Equal(1, 1, \"canonical milestone executes\") }\n")
	writeMilestoneFile(t, root, "Shared", "suite.octest", "package Main\n[Fact]\nfn SharedShouldNotRun() -> Void { Assert.True(false, \"shared should not run as milestone\") }\n")

	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected experiment root test to pass with Shared support folder: err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if strings.Contains(stdout, "SharedShouldNotRun") || strings.Contains(stdout, "MILESTONE Shared") {
		t.Fatalf("expected Shared support folder exclusion from milestone execution, got %q", stdout)
	}
}

func createExperimentRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := strings.Join([]string{
		"package Manifest",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Dependencies: Dependency[]",
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"}",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"Experiment\"",
		"        Version: \"0.1.0\"",
		"        Description: \"experiment root\"",
		"        Dependencies: []",
		"    }",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write experiment manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "REPORT.md"), []byte("# Report\n"), 0o644); err != nil {
		t.Fatalf("write experiment report: %v", err)
	}
	return root
}

func writeMilestoneFile(t *testing.T, root, milestone, name, content string) {
	t.Helper()
	dir := filepath.Join(root, milestone)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir milestone %s: %v", milestone, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s/%s: %v", milestone, name, err)
	}
}
