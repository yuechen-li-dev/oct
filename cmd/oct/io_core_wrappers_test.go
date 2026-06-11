package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestIOCoreWrappers(t *testing.T) {
	t.Parallel()
	root := "../../Libraries/IO"
	stdout, stderr, err := executeCLIWithSidecars(t, "test", root, "octxiliary-io", "octxiliary-csv", "octxiliary-json", "octxiliary-xlsx")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)

	expectedPasses := []string{
		"PASS IO.FileReadWriteTextAndDeleteRoundTrip",
		"PASS IO.FileReadWriteBytesRoundTrip",
		"PASS IO.FileReadMissingFails",
		"PASS IO.PathWrappersCoverJoinAndSegments",
		"PASS IO.DirectoryMakeListAndRemoveAllRoundTrip",
		"PASS IO.DirectoryListMissingFails",
		"PASS IO.JsonParseStringifyAndSaveLoadRoundTrip",
		"PASS IO.JsonParseRejectsInvalidDocument",
		"PASS IO.CsvReadWriteRoundTrip",
		"PASS IO.CsvReadPreservesRaggedRows",
		"PASS IO.CsvReadRowsRoundTripQuotedCommaQuoteAndEmpty",
		"PASS IO.CsvReadTableImportsHeaderIntoColumnarRecord",
		"PASS IO.CsvReadTableRejectsDuplicateHeadersAndRaggedRows",
		"PASS IO.CsvReadMatrixImportsNumericGridAndRejectsNonNumeric",
		"PASS IO.FileWriteTextReadTextRoundTripAndOverwrite",
		"PASS IO.FileReadMissingReportsError",
		"PASS IO.FileWriteLinesReadLinesPreservesEmptyLines",
		"PASS IO.CsvWriteEscapesAndArtifactRoundTrip",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}

	unsupportedBuiltins := []string{"CsvReadMatrix", "CsvReadRows", "CsvReadTable", "JsonParse", "JsonLoad"}
	combined := stdout + stderr
	for _, name := range unsupportedBuiltins {
		if unsupportedBuiltinMessagePresent(combined, name) {
			t.Fatalf("expected %s to have compiled wrapper support, got output:\nstdout:\n%s\nstderr:\n%s", name, stdout, stderr)
		}
	}
}

func TestCsvReadMatrixCsvReadRowsCsvReadTableJsonParseJsonLoadAutoCompiledWithoutFallback(t *testing.T) {
	stdout, stderr, err := executeCLIWithSidecars(t, "test", "../../Libraries/IO/IO.CoreWrappers.octest", "octxiliary-io", "octxiliary-csv", "octxiliary-json")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS IO.JsonParseStringifyAndSaveLoadRoundTrip",
		"PASS IO.CsvReadRowsRoundTripQuotedCommaQuoteAndEmpty",
		"PASS IO.CsvReadTableImportsHeaderIntoColumnarRecord",
		"PASS IO.CsvReadMatrixImportsNumericGridAndRejectsNonNumeric",
		"Execution summary: compiled: 18 interpreted fallback: 0",
	)
	combined := stdout + stderr
	unsupportedBuiltins := []string{"CsvReadMatrix", "CsvReadRows", "CsvReadTable", "JsonParse", "JsonLoad"}
	for _, name := range unsupportedBuiltins {
		if unsupportedBuiltinMessagePresent(combined, name) {
			t.Fatalf("expected %s to compile without unsupported fallback, got output:\nstdout:\n%s\nstderr:\n%s", name, stdout, stderr)
		}
	}
}

func unsupportedBuiltinMessagePresent(output string, name string) bool {
	pattern := regexp.MustCompile(`compiled mode does not yet support builtin ` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_]|$)`)
	return pattern.MatchString(output)
}
