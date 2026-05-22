package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctTestWriteThenLoadOctagonRoundTrip(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "roundtrip.octagon")

	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "roundtrip.octest", strings.Join([]string{
		"package Main",
		"",
		"record Payload {",
		"    Name: String",
		"    Samples: Int[]",
		"}",
		"",
		"[Fact]",
		"fn WriteThenLoad() -> Void ! Error {",
		fmt.Sprintf("    WriteOctagon(%q, Payload { Name: \"Run\" Samples: [1, 2, 3] })", artifactPath),
		fmt.Sprintf("    let loaded = LoadOctagon<Payload>(%q)?", artifactPath),
		"    Assert.Equal(\"Run\", loaded.Name, \"roundtrip record field\")",
		"    Assert.Equal(3, Len(loaded.Samples), \"roundtrip array len\")",
		"}",
		"",
	}, "\n"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected test success, got err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Main.WriteThenLoad") {
		t.Fatalf("expected WriteThenLoad pass output, got stdout=%q", stdout.String())
	}
}
