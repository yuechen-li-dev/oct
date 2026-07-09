package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestSDSLvHelpMentionsSPIRVCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"sdslv", "--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("sdslv --help failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"compile-spv", "generate-header"} {
		if !strings.Contains(out, want) {
			t.Fatalf("sdslv help missing %q:\n%s", want, out)
		}
	}
}

func TestSDSLvGenerateHeaderCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	tmp := t.TempDir()
	input := repoPath(t, "examples", "SDSL-V", "M0", "VectorAdd.sdslv")
	output := filepath.Join(tmp, "vector_add_spirv.h")
	args := []string{
		"sdslv", "generate-header", input,
		"-o", output,
		"--symbol", "k_sdslv_vector_add_spirv",
	}
	if err := cli.Execute(args, &stdout, &stderr); err != nil {
		if !strings.Contains(err.Error(), "dxc was not found") && !strings.Contains(stderr.String(), "dxc was not found") {
			t.Fatalf("generate-header failed unexpectedly: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
		}
		return
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("expected header output at %s: %v", output, err)
	}
	text, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read header output: %v", err)
	}
	if !strings.Contains(string(text), "k_sdslv_vector_add_spirv") {
		t.Fatalf("header output missing symbol:\n%s", string(text))
	}
}

func TestSDSLvM4EmitCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M4", "WorkgroupTileCopy.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"workgroup Tile: array<f32,256> shader TileCopy",
		"expr WorkgroupMemoryBarrierWithSync()",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "workgroup_tile_copy.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"groupshared float Tile[256];",
		"GroupMemoryBarrierWithGroupSync();",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestSDSLvPrometheusSgemmScalarPlusSourceEmits(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "internal", "prometheus", "shaders", "sdslv", "sgemm_scalar_baseline_plus.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"resource readonly A: array<f32> bundle SgemmIO binding(0,0)",
		"resource readonly B: array<f32> bundle SgemmIO binding(1,0)",
		"resource readwrite C: array<f32> bundle SgemmIO binding(2,0)",
		"entry compute SgemmScalarBaselinePlus8x8_CS numthreads(8,8,1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "sgemm_scalar_baseline_plus.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"[[vk::binding(0, 0)]] Buffer<float> A;",
		"[[vk::binding(1, 0)]] Buffer<float> B;",
		"[[vk::binding(2, 0)]] RWBuffer<float> C;",
		"[[vk::push_constant]] ConstantBuffer<SgemmParams> params;",
		"void SgemmScalarBaselinePlus8x8_CS(",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestPrometheusSgemmScalarPlusHeaderCheckedIn(t *testing.T) {
	path := repoPath(t, "internal", "prometheus", "native", "reactor_vulkan_sgemm_scalar_plus_spirv.h")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in header: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"// Source: internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv",
		"// Entry point: SgemmScalarBaselinePlus8x8_CS",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv[] = {",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv_word_count = ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("checked-in header missing %q:\n%s", want, body)
		}
	}
}
