package toolchain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

func TestResolveEntryPointSingleDefault(t *testing.T) {
	entry, err := resolveEntryPoint(vdmir.Module{
		EntryPoints: []vdmir.ComputeEntryPoint{{EmittedName: "VectorAdd_CS"}},
	}, "")
	if err != nil {
		t.Fatalf("resolveEntryPoint() error = %v", err)
	}
	if entry.EmittedName != "VectorAdd_CS" {
		t.Fatalf("unexpected entry %q", entry.EmittedName)
	}
}

func TestResolveEntryPointRequiresExplicitChoice(t *testing.T) {
	_, err := resolveEntryPoint(vdmir.Module{
		EntryPoints: []vdmir.ComputeEntryPoint{{EmittedName: "A_CS"}, {EmittedName: "B_CS"}},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "pass --entry") {
		t.Fatalf("expected multiple-entry diagnostic, got %v", err)
	}
}

func TestResolveDXCPathMissingDiagnostic(t *testing.T) {
	_, err := resolveDXCPath(host{
		getenv:   func(string) string { return "" },
		lookPath: func(string) (string, error) { return "", errors.New("missing") },
		runner:   defaultCommandRunner,
	}, "")
	if err == nil || !strings.Contains(err.Error(), "dxc was not found") {
		t.Fatalf("expected clear dxc diagnostic, got %v", err)
	}
}

func TestBuildDXCArgs(t *testing.T) {
	args := buildDXCArgs("VectorAdd_CS", "out/vector_add.spv", "out/vector_add.hlsl", []string{"-fspv-extension=SPV_KHR_storage_buffer_storage_class"})
	want := []string{
		"-spirv",
		"-T", "cs_6_0",
		"-E", "VectorAdd_CS",
		"-Fo", "out/vector_add.spv",
		"-fspv-target-env=vulkan1.3",
		"-O3",
		"-fspv-extension=SPV_KHR_storage_buffer_storage_class",
		"out/vector_add.hlsl",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", args, want)
	}
}

func TestCooperativeMatrixTargetContract(t *testing.T) {
	requirement := vdmir.CapabilityRequirement{Kind: vdmir.CapabilityCooperativeMatrixF16F32M16N16K16Subgroup}
	target := targetContract(vdmir.Module{Requirements: []vdmir.CapabilityRequirement{requirement}})
	args := buildDXCArgsForTarget("main", "out.spv", "in.hlsl", nil, target)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-T cs_6_9", "-fspv-target-env=vulkan1.3", "-fspv-use-vulkan-memory-model", "-enable-16bit-types"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
	if strings.Join(target.vulkanExtensions, ",") != "VK_KHR_cooperative_matrix" ||
		strings.Join(target.spirvExtensions, ",") != "SPV_KHR_cooperative_matrix" ||
		!strings.Contains(strings.Join(target.spirvCapabilities, ","), "CooperativeMatrixKHR") {
		t.Fatalf("target capability manifest = %#v", target)
	}
	if target.name != "ProductionVulkan14" || target.validatorTarget != "vulkan1.4" || target.spirvVersion != "1.6" {
		t.Fatalf("unexpected production target authority = %#v", target)
	}
}

func TestRayQueryTargetContract(t *testing.T) {
	target := targetContract(vdmir.Module{Requirements: []vdmir.CapabilityRequirement{{Kind: vdmir.CapabilityRayQuery}}})
	args := buildDXCArgsForTarget("main", "out.spv", "in.hlsl", nil, target)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-T cs_6_5") || !strings.Contains(joined, "-fspv-target-env=vulkan1.3") {
		t.Fatalf("args = %q", joined)
	}
	if strings.Join(target.vulkanExtensions, ",") != "VK_KHR_acceleration_structure,VK_KHR_ray_query" ||
		strings.Join(target.spirvExtensions, ",") != "SPV_KHR_acceleration_structure,SPV_KHR_ray_query" ||
		strings.Join(target.spirvCapabilities, ",") != "RayQueryKHR" {
		t.Fatalf("target capability manifest = %#v", target)
	}
}

func TestCompileToSPIRVSmokeIfDXCAvailable(t *testing.T) {
	dxcPath, err := resolveDXCPath(defaultHost(), "")
	if err != nil {
		t.Skip("dxc not available:", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", "..", ".."))
	tmp := t.TempDir()
	result, err := CompileToSPIRV(CompileOptions{
		InputPath:  filepath.Join(root, "examples", "SDSL-V", "M0", "VectorAdd.sdslv"),
		OutputPath: filepath.Join(tmp, "vector_add.spv"),
		HLSLPath:   filepath.Join(tmp, "vector_add.hlsl"),
		DXCPath:    dxcPath,
		Validate:   false,
	})
	if err != nil {
		t.Fatalf("CompileToSPIRV() error = %v", err)
	}
	info, err := os.Stat(result.SPIRVPath)
	if err != nil {
		t.Fatalf("compiled SPIR-V missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("compiled SPIR-V was empty")
	}
}
