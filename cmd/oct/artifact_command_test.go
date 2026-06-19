package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctArtifactRunsDiscoveredArtifacts(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", "package Main\n[Artifact]\nfn Generate() -> Void { Print(\"csv,row\") }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "RUN  Main.Generate") || !strings.Contains(stdout.String(), "PASS Main.Generate") {
		t.Fatalf("expected artifact run/pass output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "csv,row") {
		t.Fatalf("expected artifact body output, got %q", stdout.String())
	}
}

func TestOctArtifactUsesDeterministicOrdering(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Alpha", "a2.octest", "package Alpha\n[Artifact]\nfn Beta() -> Void { return }\n")
	writeOctPkgFile(t, root, "Alpha", "a1.octest", "package Alpha\n[Artifact]\nfn Alpha() -> Void { return }\n")
	writeOctPkgFile(t, root, "Main", "m.octest", "package Main\n[Artifact]\nfn Omega() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	output := stdout.String()
	alpha := strings.Index(output, "RUN  Alpha.Alpha")
	beta := strings.Index(output, "RUN  Alpha.Beta")
	omega := strings.Index(output, "RUN  Main.Omega")
	if alpha < 0 || beta < 0 || omega < 0 || !(alpha < beta && beta < omega) {
		t.Fatalf("expected deterministic package/file/function order, got %q", output)
	}
}

func TestOctArtifactRejectsInvalidArtifactShapes(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{name: "outside octest", source: "package Main\n[Artifact]\nfn Bad() -> Void { return }\n", wantErr: "[Artifact] is only valid in .octest files"},
		{name: "parameterized", source: "package Main\n[Artifact]\nfn Bad(x: Int) -> Void { return }\n", wantErr: "[Artifact] function must not declare parameters"},
		{name: "non void", source: "package Main\n[Artifact]\nfn Bad() -> Int { return 1 }\n", wantErr: "[Artifact] function must return Void"},
		{name: "fact combined", source: "package Main\n[Artifact]\n[Fact]\nfn Bad() -> Void { return }\n", wantErr: "[Artifact] cannot be combined with [Fact]"},
		{name: "theory combined", source: "package Main\n[Artifact]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", wantErr: "[Artifact] cannot be combined with [Theory]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
			name := "bad.octest"
			if tc.name == "outside octest" {
				name = "bad.oct"
			}
			writeOctPkgFile(t, root, "Main", name, tc.source)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := cli.Execute([]string{"artifact", root}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got err=%v stderr=%q stdout=%q", tc.wantErr, err, stderr.String(), stdout.String())
			}
		})
	}
}

func TestOctArtifactFailurePropagatesAndTestCommandUnaffected(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "suite.octest", strings.Join([]string{
		"package Main",
		"[Fact]",
		"fn FactStillRuns() -> Void { Assert.True(true, \"fact pass\") }",
		"[Artifact]",
		"fn Harness() -> Void { let x = 1 / 0 Print(x) }",
	}, "\n")+"\n")

	var testStdout bytes.Buffer
	var testStderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &testStdout, &testStderr); err != nil {
		t.Fatalf("expected oct test success, got err=%v stderr=%q stdout=%q", err, testStderr.String(), testStdout.String())
	}
	if !strings.Contains(testStdout.String(), "PASS Main.FactStillRuns") {
		t.Fatalf("expected fact to run, got %q", testStdout.String())
	}
	if strings.Contains(testStdout.String(), "Harness") {
		t.Fatalf("artifact should not run in oct test, got %q", testStdout.String())
	}

	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	err := cli.Execute([]string{"artifact", root}, &artifactStdout, &artifactStderr)
	if err == nil {
		t.Fatalf("expected artifact command failure")
	}
	if !strings.Contains(artifactStdout.String(), "FAIL Main.Harness") {
		t.Fatalf("expected artifact failure output, got %q", artifactStdout.String())
	}
	if !strings.Contains(artifactStderr.String(), "artifact failed: 1 artifact(s) failed") {
		t.Fatalf("expected command failure summary, got %q", artifactStderr.String())
	}
}

func TestOctArtifactRealFixtureTemplateExample(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join("..", "..", "testdata", "m24h", "valid")
	err := cli.Execute([]string{"artifact", root}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected m24h artifact example to pass, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Fixtures.GenerateSignalCsvTemplate") {
		t.Fatalf("expected real artifact pass output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "time_s,value") {
		t.Fatalf("expected CSV template output from artifact, got %q", stdout.String())
	}
}

func TestOctArtifactExecutionModesAndMetadata(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", "package Main\n[Artifact]\nfn Generate() -> Void { Print(\"artifact-mode-ok\") }\n")

	for _, mode := range []string{"interpreted", "compiled"} {
		t.Run(mode, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := cli.Execute([]string{"artifact", root, "--execution", mode}, &stdout, &stderr); err != nil {
				t.Fatalf("expected artifact %s success, got err=%v stderr=%q stdout=%q", mode, err, stderr.String(), stdout.String())
			}
			output := stdout.String()
			if !strings.Contains(output, "Execution: "+mode) {
				t.Fatalf("expected execution metadata for %s, got %q", mode, output)
			}
			if !strings.Contains(output, "artifact-mode-ok") || !strings.Contains(output, "PASS Main.Generate") {
				t.Fatalf("expected artifact output/pass for %s, got %q", mode, output)
			}
		})
	}
}

func TestOctArtifactRejectsInvalidExecutionMode(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", "package Main\n[Artifact]\nfn Generate() -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"artifact", root, "--execution", "banana"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected invalid execution mode failure")
	}
	if !strings.Contains(stderr.String(), "invalid artifact execution mode \"banana\" (expected compiled|interpreted)") {
		t.Fatalf("expected clear invalid mode diagnostic, got stderr=%q stdout=%q err=%v", stderr.String(), stdout.String(), err)
	}
}
