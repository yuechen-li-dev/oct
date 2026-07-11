package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type invalidFixtureExpectation struct {
	file, phase, code string
	line, column      uint32
}

func languageFixtureRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "internal", "sdslv", "testdata", "language")
}

func TestSdslvValidLanguageFixtureCorpusCompiles(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			suite, err := Prepare(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Compile(suite, filepath.Join(t.TempDir(), "artifacts")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSdslvInvalidLanguageFixtureCorpus(t *testing.T) {
	expectations := []invalidFixtureExpectation{
		{"AssertWrongArity.sdslvinvalid", "validate", "SDSL-V1401", 3, 5},
		{"TheoryWithoutRows.sdslvinvalid", "validate", "SDSL-V1107", 1, 1},
		{"TestInputKindMismatch.sdslvinvalid", "validate", "SDSL-V1218", 4, 22},
	}
	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			path := filepath.Join(languageFixtureRoot(t), "invalid", expectation.file)
			file, err := source.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := lex.Analyze(file)
			if err != nil {
				if expectation.phase == "lex" {
					return
				}
				t.Fatalf("unexpected lex failure: %v", err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				if expectation.phase == "parse" {
					return
				}
				t.Fatalf("unexpected parse failure: %v", err)
			}
			if expectation.phase != "validate" {
				t.Fatalf("fixture reached validate, expected phase %s", expectation.phase)
			}
			for _, diagnostic := range validate.Diagnostics(module) {
				if diagnostic.Code == expectation.code {
					if diagnostic.Span.Start.Line != expectation.line || diagnostic.Span.Start.Column != expectation.column {
						t.Fatalf("%s location = %d:%d, want %d:%d", expectation.code, diagnostic.Span.Start.Line, diagnostic.Span.Start.Column, expectation.line, expectation.column)
					}
					return
				}
			}
			t.Fatalf("missing expected diagnostic %s", expectation.code)
		})
	}
}

func TestSdslvM31aValidFlowFixtureCorpusValidates(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m31a-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M31a valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := source.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := lex.Analyze(file)
			if err != nil {
				t.Fatalf("unexpected lex failure: %v", err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatalf("unexpected parse failure: %v", err)
			}
			if diagnostics := validate.Diagnostics(module); len(diagnostics) != 0 {
				t.Fatalf("unexpected validate failure: %v", diagnostic.Error(diagnostics))
			}
		})
	}
}

func TestSdslvM31aInvalidFlowFixtureCorpus(t *testing.T) {
	expectations := []invalidFixtureExpectation{
		{"UnknownPushTarget.sdslvinvalid", "validate", "SDSL-V3104", 4, 14},
		{"UnknownGotoTarget.sdslvinvalid", "validate", "SDSL-V3104", 4, 14},
		{"DuplicateState.sdslvinvalid", "validate", "SDSL-V3101", 4, 7},
		{"PopAtEntry.sdslvinvalid", "validate", "SDSL-V3108", 4, 9},
		{"PopReachableWithoutPush.sdslvinvalid", "validate", "SDSL-V3108", 6, 9},
		{"MixedDepthPopReachability.sdslvinvalid", "validate", "SDSL-V3110", 9, 7},
		{"DirectPushCycle.sdslvinvalid", "validate", "SDSL-V3105", 4, 9},
		{"IndirectPushCycle.sdslvinvalid", "validate", "SDSL-V3105", 7, 9},
		{"LongPushCycle.sdslvinvalid", "validate", "SDSL-V3105", 10, 9},
		{"StatementAfterPush.sdslvinvalid", "validate", "SDSL-V3103", 5, 9},
		{"StatementAfterPop.sdslvinvalid", "validate", "SDSL-V3103", 8, 9},
		{"StatementAfterGoto.sdslvinvalid", "validate", "SDSL-V3103", 5, 9},
		{"StatementAfterFinish.sdslvinvalid", "validate", "SDSL-V3103", 5, 9},
		{"PushInsideNestedIf.sdslvinvalid", "validate", "SDSL-V3102", 4, 9},
		{"PopInsideLoop.sdslvinvalid", "validate", "SDSL-V3102", 4, 9},
		{"FinishInsideNestedBranch.sdslvinvalid", "validate", "SDSL-V3102", 4, 9},
		{"GotoWithNonzeroStack.sdslvinvalid", "validate", "SDSL-V3109", 8, 9},
		{"PushedFallthroughToCompletion.sdslvinvalid", "validate", "SDSL-V3107", 7, 1},
		{"AmbiguousBarrierStackPath.sdslvinvalid", "validate", "SDSL-V3114", 7, 7},
		{"CrossFlowTarget.sdslvinvalid", "validate", "SDSL-V3104", 4, 14},
	}
	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			path := filepath.Join(languageFixtureRoot(t), "m31a-invalid", expectation.file)
			assertInvalidFixtureExpectation(t, path, expectation)
		})
	}
}

func assertInvalidFixtureExpectation(t *testing.T, path string, expectation invalidFixtureExpectation) {
	t.Helper()
	file, err := source.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		if expectation.phase == "lex" {
			return
		}
		t.Fatalf("unexpected lex failure: %v", err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		if expectation.phase == "parse" {
			return
		}
		t.Fatalf("unexpected parse failure: %v", err)
	}
	if expectation.phase != "validate" {
		t.Fatalf("fixture reached validate, expected phase %s", expectation.phase)
	}
	for _, d := range validate.Diagnostics(module) {
		if d.Code == expectation.code {
			if d.Span.Start.Line != expectation.line || d.Span.Start.Column != expectation.column {
				t.Fatalf("%s location = %d:%d, want %d:%d", expectation.code, d.Span.Start.Line, d.Span.Start.Column, expectation.line, expectation.column)
			}
			return
		}
	}
	t.Fatalf("missing expected diagnostic %s", expectation.code)
}

func TestSdslvNormalDirectoryDiscoveryExcludesLanguageFixtures(t *testing.T) {
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"normal.sdslvtest":     "[Fact]\nfn Normal() -> void { Assert.True(true); }\n",
		"valid.sdslvvalid":     "[Fact]\nfn Intentional() -> void { Assert.True(false); }\n",
		"invalid.sdslvinvalid": "[Theory]\nfn Invalid(x: u32) -> void {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := suitePaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "normal.sdslvtest" {
		t.Fatalf("normal discovery included language fixtures: %v", paths)
	}
	if _, err := suitePaths(filepath.Join(dir, "valid.sdslvvalid")); err == nil {
		t.Fatal("normal user runner accepted .sdslvvalid without fixture mode")
	}
}
