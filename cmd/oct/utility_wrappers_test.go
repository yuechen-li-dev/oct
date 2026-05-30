package main

import (
	"strings"
	"testing"
)

func TestUtilityWrappers(t *testing.T) {
	testCases := []struct {
		root    string
		markers []string
	}{
		{
			root: "../../Libraries/Archive",
			markers: []string{
				"PASS Archive.ZipListEntriesAndExtractAllRoundTrip",
				"PASS Archive.ZipListEntriesMissingArchiveFails",
			},
		},
		{
			root: "../../Libraries/Compression",
			markers: []string{
				"PASS Compression.GzipCompressAndDecompressBytesRoundTrip",
				"PASS Compression.GzipCompressAndDecompressFileRoundTrip",
				"PASS Compression.GzipDecompressRejectsInvalidPayload",
			},
		},
		{
			root: "../../Libraries/Hash",
			markers: []string{
				"PASS Hash.Sha256TextKnownValueChecks",
				"PASS Hash.Sha256BytesKnownValueChecks",
				"PASS Hash.Sha256FileKnownValueChecks",
				"PASS Hash.Sha256FileMissingFails",
			},
		},
		{
			root: "../../Libraries/Text",
			markers: []string{
				"PASS Text.RegexMatchFindReplaceSplit",
				"PASS Text.RegexInvalidPatternFails",
			},
		},
		{
			root: "../../Libraries/Time",
			markers: []string{
				"PASS Time.TimeParseFormatIso8601Sanity",
				"PASS Time.TimeUnixSecondsFormattingSanity",
				"PASS Time.TimeInvalidIso8601Fails",
			},
		},
	}

	for _, tc := range testCases {
		stdout, stderr, err := executeCLI("test", tc.root)
		if err != nil {
			t.Fatalf("oct test failed for %s: %v stderr=%s stdout=%s", tc.root, err, stderr, stdout)
		}
		for _, marker := range tc.markers {
			if !strings.Contains(stdout, marker) {
				t.Fatalf("expected marker %q for root %s, got %q", marker, tc.root, stdout)
			}
		}
	}
}
