// dvt2_m6a_cooperative_contract turns the live Vulkan property probe into a
// machine-readable GEMM contract without pretending that device properties
// report load/store layouts or frontend-specific storage rules.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type tuple struct {
	Scope      string `json:"scope"`
	M          uint32 `json:"m"`
	N          uint32 `json:"n"`
	K          uint32 `json:"k"`
	AType      string `json:"a_type"`
	BType      string `json:"b_type"`
	CType      string `json:"c_type"`
	ResultType string `json:"result_type"`
	Saturating bool   `json:"saturating_accumulation"`
	MMAUsable  bool   `json:"mma_usable"`
}

type probe struct {
	Schema string `json:"schema"`
	Device struct {
		Name       string `json:"name"`
		DriverInfo string `json:"driver_info"`
		Limits     struct {
			MinStorageBufferOffsetAlignment uint64 `json:"min_storage_buffer_offset_alignment"`
		} `json:"limits"`
	} `json:"device"`
	Extensions map[string]uint32 `json:"extensions"`
	Features   map[string]bool   `json:"features"`
	Subgroup   struct {
		Size                             uint32 `json:"size"`
		CooperativeMatrixSupportedStages uint32 `json:"cooperative_matrix_supported_stages"`
	} `json:"subgroup"`
	KHRTuples []tuple `json:"khr_tuples"`
}

type gemmContract struct {
	Configuration                  tuple    `json:"configuration"`
	VulkanExtension                string   `json:"vulkan_extension"`
	VulkanFeature                  string   `json:"vulkan_feature"`
	RequiredSubgroupSize           uint32   `json:"required_subgroup_size"`
	PropertyReportedLayouts        []string `json:"property_reported_layouts"`
	SPIRVLoadStoreLayoutEnumerants []string `json:"spirv_load_store_layout_enumerants"`
	M6AValidatedLayouts            []string `json:"m6a_validated_layouts"`
	StorageContract                string   `json:"storage_contract"`
	BaseOffsetAlignmentBytes       uint64   `json:"base_offset_alignment_bytes"`
	ShapeAlignment                 string   `json:"shape_alignment"`
	SDSLVRouteSupported            bool     `json:"sdslv_route_supported"`
}

func main() {
	in := flag.String("in", "", "live capability probe JSON")
	out := flag.String("out", "", "M6A contract JSON")
	flag.Parse()
	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var p probe
	if err = json.Unmarshal(data, &p); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if p.Schema != "prometheus.m40a.cooperative-matrix-probe.v1" || p.Device.Name == "" || len(p.KHRTuples) == 0 {
		fmt.Fprintln(os.Stderr, "incomplete cooperative-matrix probe")
		os.Exit(1)
	}
	contracts := make([]gemmContract, 0, len(p.KHRTuples))
	for _, item := range p.KHRTuples {
		selected := item.Scope == "subgroup" && item.M == 16 && item.N == 16 && item.K == 16 &&
			item.AType == "float16" && item.BType == "float16" && item.CType == "float32" && item.ResultType == "float32"
		validated := []string{}
		storage := "not implemented by the bounded SDSL-V cooperative route"
		shape := "no M6A route admission contract"
		if selected {
			validated = []string{"row-major"}
			storage = "A and B are row-major IEEE F16 pairs packed low-lane-first in u32 storage buffers; C/result is row-major F32; cooperative tiles stage through Workgroup memory"
			shape = "logical M, N, and K must each be divisible by 16; buffer ranges must cover the complete packed matrices"
		}
		contracts = append(contracts, gemmContract{
			Configuration: item, VulkanExtension: "VK_KHR_cooperative_matrix", VulkanFeature: "cooperativeMatrix",
			RequiredSubgroupSize: p.Subgroup.Size, PropertyReportedLayouts: []string{},
			SPIRVLoadStoreLayoutEnumerants: []string{"row-major", "column-major"}, M6AValidatedLayouts: validated,
			StorageContract: storage, BaseOffsetAlignmentBytes: p.Device.Limits.MinStorageBufferOffsetAlignment,
			ShapeAlignment: shape, SDSLVRouteSupported: selected,
		})
	}
	result := struct {
		Schema                string         `json:"schema"`
		SourceProbeSchema     string         `json:"source_probe_schema"`
		Device                string         `json:"device"`
		Driver                string         `json:"driver"`
		ExtensionSpecVersion  uint32         `json:"extension_spec_version"`
		FeatureEnabled        bool           `json:"feature_enabled"`
		ComputeStageSupported bool           `json:"compute_stage_supported"`
		LayoutReportingNote   string         `json:"layout_reporting_note"`
		F16F16F32F32Available bool           `json:"f16_f16_f32_f32_available"`
		BF16Configurations    []tuple        `json:"bf16_configurations"`
		GEMMConfigurations    []gemmContract `json:"gemm_configurations"`
	}{
		Schema: "prometheus.dvt2.m6a.cooperative-matrix-contract.v1", SourceProbeSchema: p.Schema,
		Device: p.Device.Name, Driver: p.Device.DriverInfo,
		ExtensionSpecVersion:  p.Extensions["VK_KHR_cooperative_matrix"],
		FeatureEnabled:        p.Features["khr_cooperative_matrix"],
		ComputeStageSupported: p.Subgroup.CooperativeMatrixSupportedStages&32 != 0,
		LayoutReportingNote:   "VkCooperativeMatrixPropertiesKHR reports component types, scope, dimensions, and saturation; it does not enumerate matrix layouts. Row/column are SPIR-V load/store operands, and M6A validates only row-major.",
		GEMMConfigurations:    contracts,
	}
	for _, item := range p.KHRTuples {
		if item.AType == "float16" && item.BType == "float16" && item.CType == "float32" && item.ResultType == "float32" {
			result.F16F16F32F32Available = true
		}
		if item.AType == "bfloat16" || item.BType == "bfloat16" {
			result.BF16Configurations = append(result.BF16Configurations, item)
		}
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s configurations=%d f16_f16_f32_f32=%t bf16=%d\n", result.Schema, len(contracts), result.F16F16F32F32Available, len(result.BF16Configurations))
}
