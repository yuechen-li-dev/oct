package tester

import (
	"bytes"
	"strings"
	"testing"
)

func TestLanePartitionMixedFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "mixed_lanes.octest", `package Main

[Fact]
fn CorrectnessFact() -> Void {
    Assert.True(true, "fact lane assertion")
}

[Benchmark]
fn ThroughputProbe() -> Void {
    let value = 1 + 1
    Print(value)
}

[Artifact]
fn EmitEvidence() -> Void {
    let data = [1, 2, 3]
    WriteOctagon("lane_partition.octagon", data)
}
`)

	t.Run("oct test runs facts only", func(t *testing.T) {
		var out bytes.Buffer
		if err := Execute(root, &out); err != nil {
			t.Fatalf("expected pass, got %v (%s)", err, out.String())
		}
		log := out.String()
		if !strings.Contains(log, "PASS Main.CorrectnessFact") {
			t.Fatalf("expected fact lane execution, got %q", log)
		}
		if strings.Contains(log, "ThroughputProbe") || strings.Contains(log, "EmitEvidence") {
			t.Fatalf("expected test lane to ignore benchmark/artifact, got %q", log)
		}
	})

	t.Run("oct bench runs benchmarks only without assertions", func(t *testing.T) {
		var out bytes.Buffer
		if err := ExecuteBenchmarks(root, &out, BenchmarkOptions{}); err != nil {
			t.Fatalf("expected benchmark pass, got %v (%s)", err, out.String())
		}
		log := out.String()
		if !strings.Contains(log, "PASS Main.ThroughputProbe") {
			t.Fatalf("expected benchmark execution, got %q", log)
		}
		if strings.Contains(log, "CorrectnessFact") || strings.Contains(log, "EmitEvidence") {
			t.Fatalf("expected bench lane isolation, got %q", log)
		}
	})

	t.Run("oct artifact runs artifacts only without assertions", func(t *testing.T) {
		var out bytes.Buffer
		if err := ExecuteArtifacts(root, &out); err != nil {
			t.Fatalf("expected artifact pass, got %v (%s)", err, out.String())
		}
		log := out.String()
		if !strings.Contains(log, "PASS Main.EmitEvidence") {
			t.Fatalf("expected artifact execution, got %q", log)
		}
		if strings.Contains(log, "CorrectnessFact") || strings.Contains(log, "ThroughputProbe") {
			t.Fatalf("expected artifact lane isolation, got %q", log)
		}
	})
}
