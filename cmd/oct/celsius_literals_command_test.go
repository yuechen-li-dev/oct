package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestCelsiusLiteralRuntimeAndBuildParity(t *testing.T) {
	sourcePath := writeSourceFile(t, "m82_celsius_parity.oct", "fn Main() -> Float<K> {\n    let room = 22C\n    let oven = 180C\n    if oven > room {\n        return oven\n    }\n    return room\n}\n")

	runStdout, runStderr, runErr := executeCLI("run", sourcePath)
	if runErr != nil {
		t.Fatalf("run failed: %v stderr=%s", runErr, runStderr)
	}

	trimmed := strings.TrimSpace(runStdout)
	trimmed = strings.TrimSuffix(trimmed, "K")
	got, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		t.Fatalf("expected float output, got %q parseErr=%v", runStdout, err)
	}
	if got != 453.15 {
		t.Fatalf("expected 453.15K from 180C, got %v", got)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", sourcePath)
	if buildErr != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
	}
}

func TestCelsiusLiteralRejectsTypeLevelC(t *testing.T) {
	sourcePath := writeSourceFile(t, "m82_invalid_type_level_c.oct", "fn Main() -> Float<C> {\n    return 273.15K\n}\n")

	stdout, stderr, err := executeCLI("build", sourcePath)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "unknown base unit: C") {
		t.Fatalf("expected unknown base unit diagnostic, got %q", stderr)
	}
	if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
	}
}
