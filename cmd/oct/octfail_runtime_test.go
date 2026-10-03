//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// An .octfail with `expect runtime error:` holds a program that compiles and
// whose Main must stop with the named failure. The fixtures under
// testdata/octfail_runtime are about the test form itself: `accepted` holds
// fixtures the runner must pass, `refused` holds fixtures it must fail.
func TestRuntimeOctFailFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "octfail_runtime")
	for _, mode := range []string{"interpreted", "compiled", "auto"} {
		t.Run(mode, func(t *testing.T) {
			stdout, stderr, err := executeCLIArgs("test", filepath.Join(root, "accepted"), "--execution", mode)
			if err != nil {
				t.Fatalf("accepted fixtures failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
			}
			for _, name := range []string{"assertion_failure", "fallible_main_failure", "runtime_error", "unwrap_failure"} {
				if !strings.Contains(stdout, "PASS "+name+".octfail") {
					t.Errorf("missing PASS for %s:\n%s", name, stdout)
				}
			}
			if !strings.Contains(stdout, "Result: 4 passed, 0 failed, 0 skipped") {
				t.Errorf("unexpected result line:\n%s", stdout)
			}

			stdout, _, err = executeCLIArgs("test", filepath.Join(root, "refused"), "--execution", mode)
			if err == nil {
				t.Fatalf("refused fixtures passed:\n%s", stdout)
			}
			lane := mode
			if mode == "auto" {
				lane = "interpreted"
			}
			for _, want := range []string{
				"FAIL completes.octfail",
				`actual: "` + lane + `: the program ran to completion"`,
				"FAIL wrong_message.octfail",
				`expected runtime error containing: "index out of range"`,
				`actual: "` + lane + `: assertion failed: a different failure"`,
				"FAIL does_not_compile.octfail",
				`actual: "` + lane + `: did not compile: function Main: function expects Int, but return is String"`,
				"Result: 0 passed, 3 failed, 0 skipped",
			} {
				if !strings.Contains(stdout, want) {
					t.Errorf("missing %q in:\n%s", want, stdout)
				}
			}
		})
	}
}
