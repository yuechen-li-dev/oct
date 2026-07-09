package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestSDSLvEmitVDMIRCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M0", "VectorAdd.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"vdmir module Prometheus.Kernels",
		"resource readonly A: array<f32>",
		"entry compute VectorAdd_CS numthreads(16,16,1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}
}
