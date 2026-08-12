package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOctGoCheckAndTestUseDistinctRealPaths(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	specimen := filepath.Join(root, "experimental", "octgo", "specimen")
	stdout, stderr, err := executeCLIArgs("check", specimen)
	if err != nil {
		t.Fatalf("oct check failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	for _, want := range []string{"check succeeded", "selected contracts: 1 concepts, 2 functions, 1 constant witnesses", "bridge: fresh"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("check output missing %q: %s", want, stdout)
		}
	}
	if strings.Contains(stdout, "PASS Specimen") {
		t.Fatalf("oct check executed Octest: %s", stdout)
	}

	stdout, stderr, err = executeCLIArgs("test", specimen)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	for _, want := range []string{"PASS Specimen.EqualityIsExcludedByStrictThreshold", "PASS Specimen.DimensionFreeResidualIsARecordedFalseGeneralization", "compiled: 6 interpreted fallback: 0", "Result: 6 passed"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("test output missing %q: %s", want, stdout)
		}
	}
}

func TestOctGoSpecimenRemainsOrdinaryGo(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"build", "./experimental/octgo/specimen"}, {"test", "./experimental/octgo/specimen"}} {
		command := exec.Command("go", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("go %s failed without Oct execution: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
}
