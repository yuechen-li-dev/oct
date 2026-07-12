package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
	"github.com/yuechen-li-dev/oct/internal/tester"
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

func TestSdslvM33cInvalidReasonFixtureCorpusUsesOctFailExpectations(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m33c-invalid", "*.sdslvinvalid"))
	if err != nil || len(paths) != 10 {
		t.Fatalf("M33c invalid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			expected, program, err := tester.ParseOctFailFixture(string(data))
			if err != nil {
				t.Fatalf("invalid expect header: %v", err)
			}
			file := source.File{Path: path, Text: program}
			tokens, err := lex.Analyze(file)
			if err != nil {
				t.Fatal(err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatal(err)
			}
			diagnostics := validate.Diagnostics(module)
			if len(diagnostics) == 0 {
				t.Fatal("expected validation diagnostic")
			}
			actual := diagnostic.Error(diagnostics).Error()
			if !strings.Contains(actual, expected) {
				t.Fatalf("expectation mismatch: expected diagnostic containing %q, got %s", expected, actual)
			}
			if diagnostics[0].Code != "SDSL-V1401" && diagnostics[0].Code != "SDSL-V1406" {
				t.Fatalf("unexpected M33c diagnostic code %s", diagnostics[0].Code)
			}
			if !diagnostics[0].Span.Known() {
				t.Fatal("M33c diagnostic lost source span")
			}
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

func TestSdslvM32aTensorFixtureCorpusValidates(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m32a-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M32a tensor fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := source.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := lex.Analyze(file)
			if err != nil {
				t.Fatal(err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatal(err)
			}
			if diagnostics := validate.Diagnostics(module); len(diagnostics) != 0 {
				t.Fatalf("unexpected validate failure: %v", diagnostic.Error(diagnostics))
			}
			metadata, diagnostics := validate.ValidatedTensorAssignments(module)
			if len(diagnostics) != 0 || len(metadata) != 1 || len(metadata[0].FreeIndices) == 0 {
				t.Fatalf("tensor metadata = %#v diagnostics = %v", metadata, diagnostic.Error(diagnostics))
			}
		})
	}
}

func TestSdslvM32aTensorInvalidFixtureCorpus(t *testing.T) {
	expectations := []invalidFixtureExpectation{
		{"DuplicateFreeIndex.sdslvinvalid", "validate", "SDSL-V3202", 3, 17},
		{"DuplicateReductionIndex.sdslvinvalid", "validate", "SDSL-V3208", 5, 26},
		{"FreeIndexScalarUse.sdslvinvalid", "validate", "SDSL-V3218", 3, 19},
		{"ReductionEscapesScope.sdslvinvalid", "validate", "SDSL-V3220", 5, 37},
		{"UnsafeTranspose.sdslvinvalid", "validate", "SDSL-V3216", 3, 22},
		{"OffsetSelfAlias.sdslvinvalid", "validate", "SDSL-V3216", 3, 22},
		{"BoolSum.sdslvinvalid", "validate", "SDSL-V3213", 5, 12},
		{"ImmutableDestination.sdslvinvalid", "validate", "SDSL-V3222", 4, 12},
		{"ConflictingReductionExtent.sdslvinvalid", "validate", "SDSL-V3210", 6, 41},
		{"UnsupportedAffineReductionIndex.sdslvinvalid", "validate", "SDSL-V3221", 5, 31},
	}
	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			path := filepath.Join(languageFixtureRoot(t), "m32a-invalid", expectation.file)
			assertInvalidFixtureExpectation(t, path, expectation)
		})
	}
}

func TestSdslvM33aNDArrayValidFixtureCorpusLowers(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m33a-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M33a ndarray valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := source.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := lex.Analyze(file)
			if err != nil {
				t.Fatal(err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatal(err)
			}
			if diagnostics := validate.Diagnostics(module); len(diagnostics) != 0 {
				t.Fatalf("unexpected validate failure: %v", diagnostic.Error(diagnostics))
			}
			if _, err := sdslv.EmitHLSLFile(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSdslvM33aNDArrayInvalidFixtureCorpus(t *testing.T) {
	expectations := []invalidFixtureExpectation{
		{"MissingShape.sdslvinvalid", "validate", "SDSL-V3309", 2, 12},
		{"EmptyShape.sdslvinvalid", "validate", "SDSL-V3310", 2, 25},
		{"NonconstantExtent.sdslvinvalid", "validate", "SDSL-V3301", 2, 26},
		{"NonintegerExtent.sdslvinvalid", "validate", "SDSL-V3301", 2, 26},
		{"ZeroExtent.sdslvinvalid", "validate", "SDSL-V3302", 2, 26},
		{"NegativeExtent.sdslvinvalid", "validate", "SDSL-V3302", 2, 26},
		{"ExtentOverflow.sdslvinvalid", "validate", "SDSL-V3303", 2, 26},
		{"TotalSizeOverflow.sdslvinvalid", "validate", "SDSL-V3308", 2, 34},
		{"UnsupportedElementType.sdslvinvalid", "validate", "SDSL-V3311", 2, 20},
		{"WrongIndexCount.sdslvinvalid", "validate", "SDSL-V3217", 3, 5},
		{"WrongIndexType.sdslvinvalid", "validate", "SDSL-V1507", 3, 7},
		{"TooFewLiteralElements.sdslvinvalid", "validate", "SDSL-V3305", 2, 37},
		{"TooManyLiteralElements.sdslvinvalid", "validate", "SDSL-V3305", 2, 37},
		{"WrongLiteralElementType.sdslvinvalid", "validate", "SDSL-V3306", 2, 38},
		{"NestedArrayToNDArrayImplicitConversion.sdslvinvalid", "validate", "SDSL-V1503", 3, 41},
		{"NDArrayToNestedArrayImplicitConversion.sdslvinvalid", "validate", "SDSL-V1503", 3, 45},
		{"ShapeMismatchAssignment.sdslvinvalid", "validate", "SDSL-V1503", 4, 9},
		{"ImmutableMutation.sdslvinvalid", "validate", "SDSL-V1000", 2, 5},
		{"TensorRankMismatch.sdslvinvalid", "validate", "SDSL-V3206", 4, 12},
		{"UnsupportedNestedLiteral.sdslvinvalid", "validate", "SDSL-V3312", 2, 38},
	}
	for _, expectation := range expectations {
		t.Run(expectation.file, func(t *testing.T) {
			path := filepath.Join(languageFixtureRoot(t), "m33a-invalid", expectation.file)
			assertInvalidFixtureExpectation(t, path, expectation)
		})
	}
}

func TestSdslvM33bTensorConstructionFixtureCorpus(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m33b-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M33b valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := sdslv.EmitHLSLFile(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	assertInvalidFixtureExpectation(t, filepath.Join(languageFixtureRoot(t), "m33b-invalid", "GenerateRankMismatch.sdslvinvalid"), invalidFixtureExpectation{"GenerateRankMismatch.sdslvinvalid", "validate", "SDSL-V3319", 2, 50})
}

func TestSdslvM35aValidFixtureCorpusCompiles(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m35a-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M35a valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := sdslv.EmitHLSLFile(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSdslvM35aInvalidFixtureCorpusUsesOctFailExpectations(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m35a-invalid", "*.sdslvinvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M35a invalid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			expected, program, err := tester.ParseOctFailFixture(string(data))
			if err != nil {
				t.Fatalf("invalid expect header: %v", err)
			}
			file := source.File{Path: path, Text: program}
			tokens, err := lex.Analyze(file)
			if err != nil {
				t.Fatal(err)
			}
			module, err := parse.BuildModule(tokens)
			if err != nil {
				t.Fatal(err)
			}
			diagnostics := validate.Diagnostics(module)
			if len(diagnostics) == 0 {
				t.Fatal("expected validation diagnostic")
			}
			actual := diagnostic.Error(diagnostics).Error()
			if !strings.Contains(actual, expected) {
				t.Fatalf("expectation mismatch: expected diagnostic containing %q, got %s", expected, actual)
			}
			foundM35a := false
			for _, d := range diagnostics {
				if strings.Contains(d.Message, expected) || strings.Contains(diagnostic.Error([]diagnostic.Diagnostic{d}).Error(), expected) {
					if strings.HasPrefix(d.Code, "SDSL-V35") || d.Code == "SDSL-V1506" || d.Code == "SDSL-V1508" {
						foundM35a = true
					}
					if !d.Span.Known() {
						t.Fatal("M35a diagnostic lost source span")
					}
				}
			}
			if !foundM35a {
				t.Fatalf("expected an M35a diagnostic code in %#v", diagnostics)
			}
		})
	}
}

func TestSdslvM31bValidFlowFixtureCorpusCompiles(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(languageFixtureRoot(t), "m31b-valid", "*.sdslvvalid"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("M31b valid fixture corpus: paths=%v err=%v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := sdslv.EmitHLSLFile(path); err != nil {
				t.Fatal(err)
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
		"normal.sdslvtest":     "[Fact]\nfn Normal() -> void { Assert.True(true, \"embedded SDSL-V fixture must preserve its asserted invariant\"); }\n",
		"valid.sdslvvalid":     "[Fact]\nfn Intentional() -> void { Assert.True(false, \"embedded SDSL-V fixture must preserve its asserted invariant\"); }\n",
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
