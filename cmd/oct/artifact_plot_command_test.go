//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
	"github.com/yuechen-li-dev/oct/internal/tester"
)

func TestArtifactPlotIsOwnedByOutputRootAndAttributed(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "plots.octest", strings.Join([]string{
		"package Main",
		"[Fact]",
		"fn OrdinaryFact() -> Void { Assert.True(true, \"fact\") }",
		"[Artifact]",
		"fn FirstPlot() -> Void { let _plot = PlotLine([0.0, 1.0], [0.0, 1.0], \"plots/first.png\") }",
		"[Artifact]",
		"fn SecondPlot() -> Void { let _plot = PlotLine([0.0, 1.0], [1.0, 0.0], \"plots/second.png\") }",
	}, "\n")+"\n")

	for _, mode := range []string{"interpreted", "compiled"} {
		t.Run(mode, func(t *testing.T) {
			outputRoot := filepath.Join(t.TempDir(), "artifacts")
			var stdout, stderr bytes.Buffer
			if err := cli.Execute([]string{"artifact", root, "--output-root", outputRoot, "--execution", mode, "--json"}, &stdout, &stderr); err != nil {
				t.Fatalf("artifact plot failed: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
			}
			var result struct {
				Stdout    string                     `json:"stdout"`
				Artifacts []tester.GeneratedArtifact `json:"artifacts"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode artifact report: %v; output=%q", err, stdout.String())
			}
			if len(result.Artifacts) != 2 || !strings.Contains(result.Stdout, "Outputs: 2 produced, 0 unchanged") {
				t.Fatalf("expected two produced plots, got report=%+v stdout=%q", result.Artifacts, result.Stdout)
			}
			for index, name := range []string{"first.png", "second.png"} {
				artifact := result.Artifacts[index]
				if artifact.Package != "Main" || artifact.Kind != "plot.line" || artifact.Execution != "build-time-interpreted" || artifact.Identity == "" {
					t.Fatalf("incomplete deterministic plot metadata: %+v", artifact)
				}
				if artifact.Path != "plots/"+name {
					t.Fatalf("unexpected deterministic path: %+v", artifact)
				}
				if _, err := os.Stat(filepath.Join(outputRoot, "plots", name)); err != nil {
					t.Fatalf("plot missing below output root: %v", err)
				}
			}
			if strings.Contains(result.Stdout, "OrdinaryFact") {
				t.Fatalf("ordinary [Fact] entered artifact lane: %q", result.Stdout)
			}
		})
	}
}

func TestArtifactPlotFailurePreservesFailureAndDoesNotPublish(t *testing.T) {
	root := t.TempDir()
	outputRoot := filepath.Join(t.TempDir(), "artifacts")
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "plot.octest", "package Main\n[Artifact]\nfn BrokenPlot() -> Void { let _plot = PlotLine([0.0, 1.0], [0.0, 1.0], \"broken.png\") let failure = 1 / 0 Print(failure) }\n")
	var stdout, stderr bytes.Buffer
	err := cli.Execute([]string{"artifact", root, "--output-root", outputRoot}, &stdout, &stderr)
	if err == nil || !strings.Contains(stdout.String(), "FAIL Main.BrokenPlot") || !strings.Contains(stderr.String(), "1 artifact(s) failed") {
		t.Fatalf("expected normal artifact failure, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if _, statErr := os.Stat(filepath.Join(outputRoot, "broken.png")); !os.IsNotExist(statErr) {
		t.Fatalf("failed artifact must not publish, stat err=%v", statErr)
	}
}

func TestArtifactPlotRejectsAmbientAbsolutePath(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "plot.octest", "package Main\n[Artifact]\nfn UnsafePlot() -> Void { let _plot = PlotLine([0.0], [0.0], \"C:/outside.png\") }\n")
	var stdout, stderr bytes.Buffer
	err := cli.Execute([]string{"artifact", root, "--output-root", filepath.Join(t.TempDir(), "artifacts")}, &stdout, &stderr)
	if err == nil || !strings.Contains(stdout.String(), "path must be non-empty and relative to the artifact output root") {
		t.Fatalf("expected absolute plot path rejection, err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
}
