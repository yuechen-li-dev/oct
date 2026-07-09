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
		"-fspv-target-env=vulkan1.0",
		"-O3",
		"-fspv-extension=SPV_KHR_storage_buffer_storage_class",
		"out/vector_add.hlsl",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", args, want)
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
