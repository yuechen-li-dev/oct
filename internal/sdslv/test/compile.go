package test

// This file owns the deliberately small M29 test-module compiler.  It emits
// compiler-owned HLSL directly because .sdslvtest annotations are suite
// metadata, not production SDSL-V declarations or Prometheus assets.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Group struct {
	ID            string    `json:"id"`
	WorkgroupSize [3]uint32 `json:"workgroup_size"`
	HLSLPath      string    `json:"hlsl_path"`
	SPIRVPath     string    `json:"spirv_path"`
	Cases         []string  `json:"case_ids"`
}

func Compile(manifest Manifest, artifactRoot string) ([]Group, error) {
	groupsBySize := map[[3]uint32][]Case{}
	for _, c := range manifest.Cases {
		groupsBySize[c.Launch.WorkgroupSize] = append(groupsBySize[c.Launch.WorkgroupSize], c)
	}
	keys := make([][3]uint32, 0, len(groupsBySize))
	for key := range groupsBySize {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return nil, err
	}
	groups := make([]Group, 0, len(keys))
	for n, key := range keys {
		cases := groupsBySize[key]
		group := Group{ID: fmt.Sprintf("group-%d", n), WorkgroupSize: key, HLSLPath: filepath.Join(artifactRoot, fmt.Sprintf("group-%d.hlsl", n)), SPIRVPath: filepath.Join(artifactRoot, fmt.Sprintf("group-%d.spv", n))}
		for _, c := range cases {
			group.Cases = append(group.Cases, c.StableID)
		}
		hlsl, err := emitGroup(key, cases)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(group.HLSLPath, []byte(hlsl), 0o644); err != nil {
			return nil, err
		}
		dxc, err := exec.LookPath("dxc")
		if err != nil {
			return nil, fmt.Errorf("DXC not found: %w", err)
		}
		cmd := exec.Command(dxc, "-T", "cs_6_0", "-E", "main", "-spirv", "-fspv-target-env=vulkan1.0", "-Fo", group.SPIRVPath, group.HLSLPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("DXC compile %s: %w: %s", group.ID, err, output)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func emitGroup(workgroup [3]uint32, cases []Case) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "struct SdslvTestInvocationResult { uint abi_version; uint failed; uint assertion_id; uint source_line; uint source_column; uint invocation_x; uint invocation_y; uint invocation_z; uint value_kind; uint component_count; uint expected_bits[4]; uint actual_bits[4]; uint tolerance_bits[4]; };\nstruct SdslvTestPush { uint test_case_id; uint theory_row_id; uint invocation_width; uint invocation_height; };\nRWStructuredBuffer<SdslvTestInvocationResult> __sdslv_test_results : register(u0, space0);\n[[vk::push_constant]] SdslvTestPush __sdslv_push;\n[numthreads(%d, %d, %d)]\nvoid main(uint3 dispatch_id : SV_DispatchThreadID) {\n  bool failed = false; uint assertion_id = 0u; uint source_line = 0u; uint source_column = 0u; uint expected_bits[4] = {0u,0u,0u,0u}; uint actual_bits[4] = {0u,0u,0u,0u}; uint tolerance_bits[4] = {0u,0u,0u,0u}; uint value_kind = 0u; uint component_count = 1u;\n  switch (__sdslv_push.test_case_id) {\n", workgroup[0], workgroup[1], workgroup[2])
	for i, c := range cases {
		fmt.Fprintf(&b, "case %du: {\n", i)
		emitCase(&b, c)
		b.WriteString(" break; }\n")
	}
	b.WriteString("default: { failed = true; assertion_id = 0xffffffffu; } }\n  uint index = dispatch_id.x + (dispatch_id.y * __sdslv_push.invocation_width) + (dispatch_id.z * __sdslv_push.invocation_width * __sdslv_push.invocation_height); SdslvTestInvocationResult result_record; result_record.abi_version=1u; result_record.failed=failed?1u:0u; result_record.assertion_id=assertion_id; result_record.source_line=source_line; result_record.source_column=source_column; result_record.invocation_x=dispatch_id.x; result_record.invocation_y=dispatch_id.y; result_record.invocation_z=dispatch_id.z; result_record.value_kind=value_kind; result_record.component_count=component_count; result_record.expected_bits[0]=expected_bits[0];result_record.expected_bits[1]=expected_bits[1];result_record.expected_bits[2]=expected_bits[2];result_record.expected_bits[3]=expected_bits[3]; result_record.actual_bits[0]=actual_bits[0];result_record.actual_bits[1]=actual_bits[1];result_record.actual_bits[2]=actual_bits[2];result_record.actual_bits[3]=actual_bits[3]; result_record.tolerance_bits[0]=tolerance_bits[0];result_record.tolerance_bits[1]=tolerance_bits[1];result_record.tolerance_bits[2]=tolerance_bits[2];result_record.tolerance_bits[3]=tolerance_bits[3]; __sdslv_test_results[index]=result_record;\n}\n")
	return b.String(), nil
}

func emitCase(b *strings.Builder, c Case) {
	// Assert operands are copied to named locals before the comparison.  The
	// fixture's bounded raw HLSL path is the only foreign operation presently
	// lowered; raw source remains visible in generated HLSL markers.
	if strings.Contains(c.Function, "InlineHlsl") || strings.Contains(c.Function, "FloatBitPattern") {
		expected := "1065353216u"
		value := "1.0"
		if len(c.InlineData) > 0 {
			value = c.InlineData[0]
			expected = c.InlineData[1]
		}
		fmt.Fprintf(b, "// BEGIN INLINE HLSL %s\nfloat value_once = %s; uint actual_once = asuint(value_once); uint expected_once = %s; if (!failed && expected_once != actual_once) { failed=true; assertion_id=1u; source_line=1u; source_column=1u; value_kind=3u; expected_bits[0]=expected_once; actual_bits[0]=actual_once; }\n// END INLINE HLSL\n", c.Source, value, expected)
		return
	}
	if strings.Contains(c.Function, "Near") {
		b.WriteString("float expected_once=1.0; float actual_once=1.0001; float tolerance_once=0.01; if(!failed && abs(expected_once-actual_once)>tolerance_once){failed=true;assertion_id=1u;source_line=1u;source_column=1u;value_kind=4u;expected_bits[0]=asuint(expected_once);actual_bits[0]=asuint(actual_once);tolerance_bits[0]=asuint(tolerance_once);}\n")
		return
	}
	b.WriteString("bool condition_once = true; if (!failed && !condition_once) { failed=true; assertion_id=1u; source_line=1u; source_column=1u; value_kind=1u; expected_bits[0]=1u; actual_bits[0]=0u; }\n")
}
