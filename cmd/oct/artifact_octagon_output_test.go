package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestOctArtifactWriteOctagonEmitsDeterministicValidOutput(t *testing.T) {
	root := t.TempDir()
	outputOne := filepath.Join(root, "first.octagon")
	outputTwo := filepath.Join(root, "second.octagon")
	outputEnum := filepath.Join(root, "mode.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", strings.Join([]string{
		"package Main",
		"",
		"enum SolverMode {",
		"    Explicit",
		"    Implicit",
		"}",
		"",
		"record SimulationConfig {",
		"    Name: String",
		"    Enabled: Bool",
		"    Samples: Float<m/s^2>[]",
		"}",
		"",
		"[Artifact]",
		"fn Emit() -> Void {",
		fmt.Sprintf("    WriteOctagon(%q, SimulationConfig {", outputOne),
		"        Name: \"Run A\"",
		"        Enabled: true",
		"        Samples: [9.81m/s^2, 1.23m/s^2]",
		"    })",
		fmt.Sprintf("    WriteOctagon(%q, SimulationConfig {", outputTwo),
		"        Name: \"Run A\"",
		"        Enabled: true",
		"        Samples: [9.81m/s^2, 1.23m/s^2]",
		"    })",
		fmt.Sprintf("    WriteOctagon(%q, SolverMode.Explicit)", outputEnum),
		"}",
		"",
	}, "\n"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected artifact success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}

	firstBody, err := os.ReadFile(outputOne)
	if err != nil {
		t.Fatalf("read first .octagon artifact: %v", err)
	}
	secondBody, err := os.ReadFile(outputTwo)
	if err != nil {
		t.Fatalf("read second .octagon artifact: %v", err)
	}

	if string(firstBody) != string(secondBody) {
		t.Fatalf("expected deterministic .octagon emission, got first=%q second=%q", firstBody, secondBody)
	}

	expected := "SimulationConfig {\n    Name: \"Run A\"\n    Enabled: true\n    Samples: [9.81m/s^2, 1.23m/s^2]\n}\n"
	if string(firstBody) != expected {
		t.Fatalf("unexpected .octagon artifact body\nwant: %q\ngot:  %q", expected, firstBody)
	}

	if _, err := octagon.Load(outputOne); err != nil {
		t.Fatalf("expected emitted .octagon to pass validator, got %v", err)
	}
	if _, err := octagon.Load(outputEnum); err != nil {
		t.Fatalf("expected emitted enum .octagon to pass validator, got %v", err)
	}
}

func TestOctArtifactWriteOctagonRejectsBadPathAtRuntime(t *testing.T) {
	root := t.TempDir()

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", strings.Join([]string{
		"package Main",
		"",
		"[Artifact]",
		"fn Emit() -> Void {",
		"    let suffix = if true { \"txt\" } else { \"octagon\" }",
		"    let path = \"artifact.\" + suffix",
		"    WriteOctagon(path, 1)",
		"}",
		"",
	}, "\n"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"artifact", root}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected oct artifact to fail when WriteOctagon path suffix is invalid")
	}
	if !strings.Contains(stdout.String(), "WriteOctagon path must end with .octagon") {
		t.Fatalf("expected runtime failure output, got stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
}
