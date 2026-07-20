package test

// This package orchestrates compiler artifacts only. Shared HLSL emission,
// including ordinary VD-MIR bodies and AssertStmt, belongs to emit/hlsl.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

type Group struct {
	ID            string    `json:"id"`
	WorkgroupSize [3]uint32 `json:"workgroup_size"`
	HLSLPath      string    `json:"hlsl_path"`
	SPIRVPath     string    `json:"spirv_path"`
	Cases         []string  `json:"case_ids"`
}

func Compile(suite Suite, artifactRoot string) ([]Group, error) {
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return nil, err
	}
	program := BuildTestProgram(suite)
	groups := make([]Group, 0, len(suite.Groups))
	for _, input := range suite.Groups {
		g := Group{ID: input.ID, WorkgroupSize: input.WorkgroupSize, HLSLPath: filepath.Join(artifactRoot, input.ID+".hlsl"), SPIRVPath: filepath.Join(artifactRoot, input.ID+".spv")}
		for _, c := range input.Cases {
			g.Cases = append(g.Cases, c.Test.StableID)
		}
		tg := findTestGroup(program, input.ID)
		source, err := hlsl.EmitTestGroup(program, tg)
		if err != nil {
			return nil, err
		}
		if err = os.WriteFile(g.HLSLPath, []byte(source), 0o644); err != nil {
			return nil, err
		}
		dxc, err := exec.LookPath("dxc")
		if err != nil {
			return nil, fmt.Errorf("DXC not found: %w", err)
		}
		// DXC's highest spelling is vulkan1.3; test artifacts are SPIR-V 1.6
		// and the production validator applies Vulkan 1.4 semantics.
		out, err := exec.Command(dxc, "-T", "cs_6_0", "-E", "main", "-spirv", "-fspv-target-env=vulkan1.3", "-Fo", g.SPIRVPath, g.HLSLPath).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("DXC compile %s: %w: %s", g.ID, err, out)
		}
		groups = append(groups, g)
	}
	return groups, nil
}
func findTestGroup(p vdmir.TestProgram, id string) vdmir.TestCompilationGroup {
	for _, g := range p.Groups {
		if g.ID == id {
			return g
		}
	}
	return vdmir.TestCompilationGroup{}
}
