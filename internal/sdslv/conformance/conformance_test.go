package conformance

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv"
	"github.com/yuechen-li-dev/oct/internal/sdslv/toolchain"
)

func TestRepositoryManifest(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate conformance package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	if err := Verify(root); err != nil {
		t.Fatal(err)
	}
}

func TestGroupedSemanticSpacesAreExactlyExpandedEquivalent(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate conformance package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	grouped := filepath.Join(root, "examples", "SDSL-V", "AttentionSpacePoc", "GroupedSpaceEquivalence.sdslv")
	expanded := filepath.Join(root, "examples", "SDSL-V", "AttentionSpacePoc", "ExpandedSpaceEquivalence.sdslv")
	groupedHLSL, err := sdslv.EmitHLSLFile(grouped)
	if err != nil {
		t.Fatal(err)
	}
	expandedHLSL, err := sdslv.EmitHLSLFile(expanded)
	if err != nil {
		t.Fatal(err)
	}
	if groupedHLSL != expandedHLSL {
		t.Fatal("grouped semantic-space HLSL differs from its explicit expansion")
	}
	if _, err := exec.LookPath("dxc"); err != nil {
		t.Skip("dxc unavailable after exact HLSL equivalence proof")
	}
	if _, err := exec.LookPath("spirv-val"); err != nil {
		t.Skip("spirv-val unavailable after exact HLSL equivalence proof")
	}
	out := t.TempDir()
	groupedResult, err := sdslv.CompileSPIRV(toolchain.CompileOptions{
		InputPath: grouped, OutputPath: filepath.Join(out, "grouped.spv"), HLSLPath: filepath.Join(out, "grouped.hlsl"), RequireSPIRVVal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	expandedResult, err := sdslv.CompileSPIRV(toolchain.CompileOptions{
		InputPath: expanded, OutputPath: filepath.Join(out, "expanded.spv"), HLSLPath: filepath.Join(out, "expanded.hlsl"), RequireSPIRVVal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	groupedSPIRV, err := os.ReadFile(groupedResult.SPIRVPath)
	if err != nil {
		t.Fatal(err)
	}
	expandedSPIRV, err := os.ReadFile(expandedResult.SPIRVPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(groupedSPIRV, expandedSPIRV) {
		t.Fatal("grouped semantic-space SPIR-V differs from its explicit expansion")
	}
}
