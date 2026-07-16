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

func TestSDSLvTestListsStableTheoryCases(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "suite.sdslvtest")
	source := "[Fact]\nfn FactCase() -> void {}\n[Theory]\n[InlineData(1u)]\n[InlineData(2u)]\nfn TheoryCase(value: u32) -> void {}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := cli.Execute([]string{"sdslv", "test", path, "--list"}, &stdout, &stderr); err != nil {
		t.Fatalf("sdslv test --list: %v stderr=%s", err, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "FactCase") || !strings.Contains(got, "TheoryCase[0]") || !strings.Contains(got, "sdslv-") {
		t.Fatalf("unexpected list output: %s", got)
	}
}

func TestSDSLvCheckRendersStructuredDiagnostic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.sdslv")
	if err := os.WriteFile(path, []byte("fn Bad() -> void { let x: u32 = missing; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := cli.Execute([]string{"sdslv", "check", path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("sdslv check accepted invalid source")
	}
	got := stderr.String()
	for _, want := range []string{path + ":1:33: error SDSL-V1501: unknown identifier missing", "sdslv check failed:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic output = %q, want %q", got, want)
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

func TestSDSLvM33bRepresentativeCompileSPVCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	tmp := t.TempDir()
	input := repoPath(t, "examples", "SDSL-V", "M33b", "TensorConstructionProofs.sdslv")
	cases := []struct {
		entry string
		name  string
	}{
		{"FillProof_CS", "fill"},
		{"GenerateRank2Proof_CS", "generate_rank2"},
		{"GenerateRank4Proof_CS", "generate_rank4"},
		{"GeneratedMatmulProof_CS", "generated_matmul"},
	}
	for _, tc := range cases {
		stdout.Reset()
		stderr.Reset()
		output := filepath.Join(tmp, tc.name+".spv")
		args := []string{
			"sdslv", "compile-spv", input,
			"-o", output,
			"--entry", tc.entry,
		}
		if err := cli.Execute(args, &stdout, &stderr); err != nil {
			if !strings.Contains(err.Error(), "dxc was not found") && !strings.Contains(stderr.String(), "dxc was not found") {
				t.Fatalf("compile-spv %s failed unexpectedly: %v stderr=%q stdout=%q", tc.entry, err, stderr.String(), stdout.String())
			}
			return
		}
		if info, err := os.Stat(output); err != nil || info.Size() == 0 {
			t.Fatalf("expected non-empty SPIR-V output for %s at %s: stat=%v info=%v", tc.entry, output, err, info)
		}
		hlslPath := strings.TrimSuffix(output, ".spv") + ".hlsl"
		text, err := os.ReadFile(hlslPath)
		if err != nil {
			t.Fatalf("read HLSL output for %s: %v", tc.entry, err)
		}
		body := string(text)
		for _, banned := range []string{"inline HLSL expressions require", "unsupported guarded read", "malformed fixed-shape"} {
			if strings.Contains(body, banned) {
				t.Fatalf("HLSL output for %s contains placeholder %q:\n%s", tc.entry, banned, body)
			}
		}
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
	path := repoPath(t, "internal", "prometheus", "shaders", "sdslv", "production", "sgemm", "sgemm_scalar_baseline_plus.sdslv")
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
		"[[vk::binding(0, 0)]] StructuredBuffer<float> A;",
		"[[vk::binding(1, 0)]] StructuredBuffer<float> B;",
		"[[vk::binding(2, 0)]] RWStructuredBuffer<float> C;",
		"[[vk::push_constant]] ConstantBuffer<SgemmParams> params;",
		"void SgemmScalarBaselinePlus8x8_CS(",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestSDSLvPrometheusSgemmTile16x16SharedSourceEmits(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "internal", "prometheus", "shaders", "sdslv", "production", "sgemm", "sgemm_tile16x16_shared_fp32.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"resource readonly A: array<f32> bundle SgemmIO binding(0,0)",
		"resource readonly B: array<f32> bundle SgemmIO binding(1,0)",
		"resource readwrite C: array<f32> bundle SgemmIO binding(2,0)",
		"workgroup TileA: tile<f32,16,16> shader SgemmTile16x16SharedFp32",
		"workgroup TileB: tile<f32,16,16> shader SgemmTile16x16SharedFp32",
		"let AView: readonly matrix_view<f32> = row_major(A, params.m, params.k)",
		"assign CView[row, col] = acc",
		"entry compute SgemmTile16x16SharedFp32_CS numthreads(16,16,1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "sgemm_tile16x16_shared_fp32.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"groupshared float TileA[16 * 16];",
		"groupshared float TileB[16 * 16];",
		"C[((row) * (params.n)) + (col)] = acc;",
		"GroupMemoryBarrierWithGroupSync();",
		"void SgemmTile16x16SharedFp32_CS(",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestSDSLvPrometheusSgemmShadersDoNotGainFlowDispatcherOverhead(t *testing.T) {
	shaders := []string{
		"sgemm_scalar_baseline_plus.sdslv",
		"sgemm_tile16x16_shared_fp32.sdslv",
		"sgemm_reg2x2_tile16x16_fp32.sdslv",
		"sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv",
		"sgemm_reg2x2_tile16x16_derive_fp32.sdslv",
		"sgemm_reg2x2_tile16x16_flowboard_fp32.sdslv",
	}
	for _, name := range shaders {
		t.Run(name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			path := repoPath(t, "internal", "prometheus", "shaders", "sdslv", "production", "sgemm", name)
			hlslPath := filepath.Join(t.TempDir(), strings.TrimSuffix(name, ".sdslv")+".hlsl")
			if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
				t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
			}
			text, err := os.ReadFile(hlslPath)
			if err != nil {
				t.Fatalf("read hlsl output: %v", err)
			}
			body := string(text)
			for _, forbidden := range []string{"flow dispatcher", "__flow_", "return_stack", "stack_top"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("%s gained flow runtime machinery %q:\n%s", name, forbidden, body)
				}
			}
		})
	}
}

func TestSDSLvM15RegTileEmitCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M15", "RegTileBasic.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"let Acc: reg_tile<f32,2,2> = reg_tile_zero()",
		"assign Acc[0u, 0u] = (Acc[0u, 0u] + 1.0)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "reg_tile_basic.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"float Acc[4];",
		"Acc[0] = 0.0;",
		"Acc[((1u) * (2)) + (1u)] = (Acc[((1u) * (2)) + (1u)] + 4.0);",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestSDSLvM15aSemanticBooleanEmitCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M15a", "SemanticBooleanOperators.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"let runtimeFlag: bool = ((params.M > 0u) && (params.N > 0u))",
		"if (runtimeFlag || (params.K == 0u))",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, " and ") || strings.Contains(out, " or ") || strings.Contains(out, " not ") {
		t.Fatalf("emit-vdmir output should lower semantic operators to logical ops:\n%s", out)
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "semantic_boolean_operators.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"bool runtimeFlag = ((params.M > 0u) && (params.N > 0u));",
		"if ((runtimeFlag || (params.K == 0u)))",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, " and ") || strings.Contains(body, " or ") {
		t.Fatalf("emit-hlsl output should use punctuation operators:\n%s", body)
	}
}

func TestSDSLvM16ComptimeForEmitCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M16", "ComptimeForRegTile.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "comptime") {
		t.Fatalf("emit-vdmir output should not mention comptime:\n%s", out)
	}
	for _, want := range []string{
		"assign Acc[0u, 0u] = (Acc[0u, 0u] + 1.0)",
		"assign Acc[1u, 1u] = (Acc[1u, 1u] + 1.0)",
		"assign CView[(params.Row + 0u), (params.Col + 1u)] = Acc[0u, 1u]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "m16_comptime_for_reg_tile.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	if strings.Contains(body, "comptime") {
		t.Fatalf("emit-hlsl output should not mention comptime:\n%s", body)
	}
	for _, want := range []string{
		"float Acc[4];",
		"Acc[((0u) * (2)) + (0u)] = (Acc[((0u) * (2)) + (0u)] + 1.0);",
		"C[(((params.Row + 1u)) * (params.N)) + ((params.Col + 1u))] = Acc[((1u) * (2)) + (1u)];",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
}

func TestSDSLvM22FlowStateEmitCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	path := repoPath(t, "examples", "SDSL-V", "M22", "FlowStateGuardWhenTileLoad.sdslv")
	if err := cli.Execute([]string{"sdslv", "emit-vdmir", path}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-vdmir failed: %v stderr=%q", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"let p__ct0: board LoadCoord = MakeLoadCoord(localThreadLinear, 0u, 16u)",
		"expr WorkgroupMemoryBarrierWithSync()",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("emit-vdmir output missing %q:\n%s", want, out)
		}
	}
	for _, banned := range []string{"flow ", "state "} {
		if strings.Contains(out, banned) {
			t.Fatalf("emit-vdmir output should not retain %q:\n%s", banned, out)
		}
	}

	stdout.Reset()
	stderr.Reset()
	tmp := t.TempDir()
	hlslPath := filepath.Join(tmp, "m22_flow_state_guard_when_tile_load.hlsl")
	if err := cli.Execute([]string{"sdslv", "emit-hlsl", path, "-o", hlslPath}, &stdout, &stderr); err != nil {
		t.Fatalf("emit-hlsl failed: %v stderr=%q", err, stderr.String())
	}
	text, err := os.ReadFile(hlslPath)
	if err != nil {
		t.Fatalf("read hlsl output: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"if (fullTile)",
		"GroupMemoryBarrierWithGroupSync();",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("emit-hlsl output missing %q:\n%s", want, body)
		}
	}
	for _, banned := range []string{"flow TileLoad", "state Load", "state Sync"} {
		if strings.Contains(body, banned) {
			t.Fatalf("emit-hlsl output should not retain %q:\n%s", banned, body)
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
		"// Source: internal/prometheus/shaders/sdslv/production/sgemm/sgemm_scalar_baseline_plus.sdslv",
		"// Entry point: SgemmScalarBaselinePlus8x8_CS",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv[] = {",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv_word_count = ",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv_numthreads_x = ",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_m = ",
		"static const uint32_t k_prom_sgemm_scalar_plus_spirv_config_unroll_k = ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("checked-in header missing %q:\n%s", want, body)
		}
	}
}

func TestPrometheusSgemmTile16x16SharedHeaderCheckedIn(t *testing.T) {
	path := repoPath(t, "internal", "prometheus", "native", "reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in header: %v", err)
	}
	body := string(text)
	for _, want := range []string{
		"// Source: internal/prometheus/shaders/sdslv/production/sgemm/sgemm_tile16x16_shared_fp32.sdslv",
		"// Entry point: SgemmTile16x16SharedFp32_CS",
		"static const uint32_t k_prom_sgemm_tile16x16_shared_fp32_spirv[] = {",
		"static const uint32_t k_prom_sgemm_tile16x16_shared_fp32_spirv_word_count = ",
		"static const uint32_t k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_x = ",
		"static const uint32_t k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_k = ",
		"static const uint32_t k_prom_sgemm_tile16x16_shared_fp32_spirv_config_tile_k = ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("checked-in header missing %q:\n%s", want, body)
		}
	}
}
