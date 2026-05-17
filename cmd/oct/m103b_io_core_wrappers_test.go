package main

import (
	"strings"
	"testing"
)

func TestMx103bIOCoreWrappers(t *testing.T) {
	root := "../../Libraries/IO"
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

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
		"PASS IO.CsvReadRejectsInvalidCsv",
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
}
