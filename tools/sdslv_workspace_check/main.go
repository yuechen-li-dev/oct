// Command sdslv_workspace_check verifies the small ownership boundaries that
// keep SDSL-V production, audit, and canonical benchmark paths distinct.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/bench"
	"github.com/yuechen-li-dev/oct/internal/sdslv/conformance"
)

type workspace struct {
	ProductionSourceRoot   string `json:"production_source_root"`
	ExperimentalSourceRoot string `json:"experimental_source_root"`
	HistoricalAuditRoot    string `json:"historical_audit_root"`
	CanonicalBenchmarkRoot string `json:"canonical_benchmark_root"`
}

type shaderAsset struct {
	ID             uint32 `json:"id"`
	Name           string `json:"name"`
	Authority      string `json:"authority"`
	SourceLanguage string `json:"source_language"`
	Source         string `json:"source"`
	Header         string `json:"header"`
}

type computeImplementation struct {
	ID               uint32 `json:"id"`
	Name             string `json:"name"`
	Authority        string `json:"authority"`
	Operation        string `json:"operation"`
	ShaderID         uint32 `json:"shader_id"`
	SelectorEligible bool   `json:"selector_eligible"`
}

type experimentalShaderAsset struct {
	ID                  string `json:"id"`
	Authority           string `json:"authority"`
	ProductionAuthority string `json:"production_authority"`
	SelectorEligible    bool   `json:"selector_eligible"`
	SourceLanguage      string `json:"source_language"`
	Source              string `json:"source"`
	Output              string `json:"output"`
	GeneratedHLSL       string `json:"generated_hlsl"`
	GeneratedHeader     string `json:"generated_header"`
	Inspection          string `json:"inspection"`
	ShaderSHA256        string `json:"shader_sha256"`
}

type shaderManifest struct {
	Workspace                workspace                 `json:"workspace"`
	ShaderAssets             []shaderAsset             `json:"shader_assets"`
	ExperimentalShaderAssets []experimentalShaderAsset `json:"experimental_shader_assets"`
	ComputeImplementations   []computeImplementation   `json:"compute_implementations"`
}

type canonicalArtifact struct {
	Name         string `json:"name"`
	BenchmarkID  string `json:"benchmarkId"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"sourceSha256"`
	SPIRVPath    string `json:"spirvPath"`
	SPIRVSHA256  string `json:"spirvSha256"`
}

type canonicalManifest struct {
	Artifacts []canonicalArtifact `json:"artifacts"`
}

type m40bTiming struct {
	MedianNS uint64 `json:"median_ns"`
}

type m40bTimingSet struct {
	SGEMM    m40bTiming `json:"sgemm"`
	Softmax  m40bTiming `json:"softmax"`
	Combined m40bTiming `json:"combined"`
	Readback m40bTiming `json:"readback"`
	EndToEnd m40bTiming `json:"end_to_end"`
}

type m40bDeviceTimingSet struct {
	Combined m40bTiming `json:"combined"`
	EndToEnd m40bTiming `json:"end_to_end"`
}

type m40bTraceEntry struct {
	Operation              uint32 `json:"operation"`
	SubmitIndex            uint32 `json:"submit_index"`
	SourceStageMask        uint32 `json:"source_stage_mask"`
	DestinationStageMask   uint32 `json:"destination_stage_mask"`
	SourceAccessMask       uint32 `json:"source_access_mask"`
	DestinationAccessMask  uint32 `json:"destination_access_mask"`
	SourceQueueFamily      uint32 `json:"source_queue_family"`
	DestinationQueueFamily uint32 `json:"destination_queue_family"`
}

type m40bTrace struct {
	EntryCount                uint32           `json:"entry_count"`
	SubmitCount               uint32           `json:"submit_count"`
	IntermediateBufferCount   uint32           `json:"intermediate_buffer_count"`
	IntermediateHostCopyCount uint32           `json:"intermediate_host_copy_count"`
	FinalReadbackCopyCount    uint32           `json:"final_readback_copy_count"`
	ReplayID                  uint64           `json:"replay_id"`
	Entries                   []m40bTraceEntry `json:"entries"`
}

type m40bArtifactRow struct {
	Kernel                    string              `json:"kernel"`
	Precision                 string              `json:"precision"`
	Group                     string              `json:"group"`
	M                         uint32              `json:"m"`
	N                         uint32              `json:"n"`
	K                         uint32              `json:"k"`
	PaddedM                   uint32              `json:"padded_m"`
	PaddedN                   uint32              `json:"padded_n"`
	PaddedK                   uint32              `json:"padded_k"`
	Correct                   bool                `json:"correct"`
	PersistentBNewA           m40bTimingSet       `json:"persistent_b_new_a"`
	DeviceAB                  m40bDeviceTimingSet `json:"device_a_b"`
	CommandPlanReplayID       uint64              `json:"command_plan_replay_id"`
	ReductionReplayID         uint64              `json:"reduction_replay_id"`
	ShaderHash                uint64              `json:"shader_hash"`
	Repeat10MedianEndToEndNS  uint64              `json:"repeat_10_median_end_to_end_ns"`
	Repeat100MedianEndToEndNS uint64              `json:"repeat_100_median_end_to_end_ns"`
	CommandTraces             struct {
		HostOne     m40bTrace `json:"host_one"`
		ResidentOne m40bTrace `json:"resident_one"`
		ResidentTwo m40bTrace `json:"resident_two"`
	} `json:"command_traces"`
}

type m40bArtifact struct {
	Schema          string            `json:"schema"`
	Notation        string            `json:"notation"`
	WarmRepetitions uint32            `json:"warm_repetitions"`
	Rows            []m40bArtifactRow `json:"rows"`
	Capability      struct {
		State        uint32 `json:"state"`
		Tuple        string `json:"tuple"`
		SubgroupSize uint32 `json:"subgroup_size"`
	} `json:"capability"`
	Validation struct {
		Warnings uint32 `json:"warnings"`
		Errors   uint32 `json:"errors"`
	} `json:"validation"`
}

type m42ArtifactRecord struct {
	Workload            string `json:"workload"`
	Path                string `json:"path"`
	Tokens              uint32 `json:"tokens"`
	ModelWidth          uint32 `json:"model_width"`
	HeadDim             uint32 `json:"head_dim"`
	SelectedPath        uint32 `json:"selected_path"`
	ReplayID            uint64 `json:"replay_id"`
	ReductionReplayID   uint64 `json:"reduction_replay_id"`
	Correct             bool   `json:"correct"`
	QProjectionGPUNS    uint64 `json:"q_projection_gpu_ns"`
	KProjectionGPUNS    uint64 `json:"k_projection_gpu_ns"`
	VProjectionGPUNS    uint64 `json:"v_projection_gpu_ns"`
	KLayoutGPUNS        uint64 `json:"k_layout_gpu_ns"`
	QKGPUNS             uint64 `json:"qk_gpu_ns"`
	ScaleGPUNS          uint64 `json:"scale_gpu_ns"`
	SoftmaxGPUNS        uint64 `json:"softmax_gpu_ns"`
	PVGPUNS             uint64 `json:"pv_gpu_ns"`
	TotalAttentionGPUNS uint64 `json:"total_attention_gpu_ns"`
	HostFedEndToEndNS   uint64 `json:"host_fed_end_to_end_ns"`
	ResidentXEndToEndNS uint64 `json:"resident_x_end_to_end_ns"`
	FinalReadbackNS     uint64 `json:"final_readback_ns"`
	RetainedBytes       uint64 `json:"retained_bytes"`
}

type m42Artifact struct {
	Schema                 string              `json:"schema"`
	TensorConvention       string              `json:"tensor_convention"`
	PrecisionContract      string              `json:"precision_contract"`
	WarmRepetitions        uint32              `json:"warm_repetitions"`
	PrimaryWarm10MedianNS  uint64              `json:"primary_warm_10_median_end_to_end_ns"`
	PrimaryWarm100MedianNS uint64              `json:"primary_warm_100_median_end_to_end_ns"`
	Records                []m42ArtifactRecord `json:"records"`
	Validation             struct {
		Warnings uint32 `json:"warnings"`
		Errors   uint32 `json:"errors"`
	} `json:"validation"`
	Device struct {
		CooperativeState uint32 `json:"cooperative_state"`
		SubgroupSize     uint32 `json:"subgroup_size"`
	} `json:"device"`
}

type m44ArtifactRecord struct {
	Workload               string `json:"workload"`
	Strategy               string `json:"strategy"`
	Path                   string `json:"path"`
	SubmitPlan             string `json:"submit_plan"`
	Tokens                 uint32 `json:"tokens"`
	HeadDim                uint32 `json:"head_dim"`
	ModelWidth             uint32 `json:"model_width"`
	ReplayID               uint64 `json:"replay_id"`
	M43ReplayID            uint64 `json:"m43_replay_id"`
	Correct                bool   `json:"correct"`
	M44GPUNS               uint64 `json:"m44_gpu_ns"`
	TotalGPUNS             uint64 `json:"total_m43_m44_gpu_ns"`
	FinalReadbackNS        uint64 `json:"final_readback_ns"`
	EndToEndNS             uint64 `json:"end_to_end_ns"`
	CPUConcatenateNS       uint64 `json:"cpu_concatenate_ns"`
	CPUPackNS              uint64 `json:"cpu_pack_ns"`
	TemporaryBytes         uint64 `json:"temporary_bytes"`
	RetainedBytes          uint64 `json:"retained_bytes"`
	SourceHeadBytes        uint64 `json:"source_head_bytes"`
	ContiguousF32Bytes     uint64 `json:"contiguous_f32_bytes"`
	ContiguousPackedBytes  uint64 `json:"contiguous_packed_bytes"`
	PartialOutputBytes     uint64 `json:"partial_output_bytes"`
	AccumulationBytes      uint64 `json:"accumulation_bytes"`
	WoUploadBytes          uint64 `json:"wo_upload_bytes"`
	WoF32Bytes             uint64 `json:"wo_f32_bytes"`
	WoPackedBytes          uint64 `json:"wo_packed_bytes"`
	FinalYBytes            uint64 `json:"final_y_bytes"`
	FinalReadbackBytes     uint64 `json:"final_readback_bytes"`
	ReusableDescriptorSets uint32 `json:"reusable_descriptor_sets"`
	DescriptorBindings     uint32 `json:"descriptor_bindings"`
	SubmitCount            uint32 `json:"submit_count"`
	IntermediateHostCopies uint32 `json:"intermediate_host_copies"`
}

type m44ShaderArtifact struct {
	SourceSHA256 string `json:"source_sha256"`
	HLSLSHA256   string `json:"hlsl_sha256"`
	SPVSHA256    string `json:"spv_sha256"`
}

type m44Artifact struct {
	Schema                string `json:"schema"`
	HeadCount             uint32 `json:"head_count"`
	SourceLayout          string `json:"source_layout"`
	LogicalConcatenation  string `json:"logical_concatenation"`
	OutputLayout          string `json:"output_layout"`
	WarmupOperations      uint32 `json:"warmup_operations_per_plan"`
	MeasurementOperations uint32 `json:"measurement_operations_per_plan"`
	CapacityPrime         uint32 `json:"capacity_prime_operations_per_plan"`
	Precision             struct {
		CooperativeInput  string `json:"cooperative_input"`
		CooperativeWeight string `json:"cooperative_weight"`
		Accumulation      string `json:"accumulation"`
		Output            string `json:"output"`
	} `json:"precision"`
	ShaderArtifacts struct {
		DXC             string            `json:"dxc"`
		Interleave      m44ShaderArtifact `json:"interleave"`
		DirectSegmented m44ShaderArtifact `json:"direct_segmented"`
	} `json:"shader_artifacts"`
	PrimaryRepeats struct {
		Warm10GPUNS       uint64 `json:"warm_10_gpu_ns"`
		Warm10EndToEndNS  uint64 `json:"warm_10_end_to_end_ns"`
		Warm100GPUNS      uint64 `json:"warm_100_gpu_ns"`
		Warm100EndToEndNS uint64 `json:"warm_100_end_to_end_ns"`
	} `json:"primary_repeats"`
	Validation struct {
		Warnings uint32 `json:"warnings"`
		Errors   uint32 `json:"errors"`
	} `json:"validation"`
	Records []m44ArtifactRecord `json:"records"`
}

type m45ArtifactRecord struct {
	Workload          string `json:"workload"`
	Strategy          string `json:"strategy"`
	SubmitPolicy      string `json:"submit_policy"`
	Tokens            uint32 `json:"tokens"`
	ModelWidth        uint32 `json:"model_width"`
	HeadDim           uint32 `json:"head_dim"`
	Correct           bool   `json:"correct"`
	ReplayID          uint64 `json:"replay_id"`
	M44ReplayID       uint64 `json:"m44_replay_id"`
	ZGeneration       uint64 `json:"z_generation"`
	M43GPUNS          uint64 `json:"m43_gpu_ns"`
	M44GPUNS          uint64 `json:"m44_gpu_ns"`
	ResidualGPUNS     uint64 `json:"residual_gpu_ns"`
	TotalGPUNS        uint64 `json:"total_m43_m44_m45_gpu_ns"`
	FinalReadbackNS   uint64 `json:"final_readback_ns"`
	EndToEndNS        uint64 `json:"end_to_end_ns"`
	CPUAddNS          uint64 `json:"cpu_add_ns"`
	XReadbackNS       uint64 `json:"x_readback_ns"`
	RetainedBytes     uint64 `json:"retained_bytes"`
	ExactRequestBytes uint64 `json:"exact_request_bytes"`
	InPlaceSavedBytes uint64 `json:"in_place_saved_bytes"`
	SubmitCount       uint32 `json:"submit_count"`
}

type m45Artifact struct {
	Schema string `json:"schema"`
	Shader struct {
		SourceSHA256 string `json:"source_sha256"`
		HLSLSHA256   string `json:"hlsl_sha256"`
		SPVSHA256    string `json:"spirv_sha256"`
	} `json:"shader"`
	Validation struct {
		Warnings uint32 `json:"warnings"`
		Errors   uint32 `json:"errors"`
	} `json:"validation"`
	WarmupsPerPlan      uint32 `json:"warmups_per_plan"`
	MeasurementsPerPlan uint32 `json:"measurements_per_plan"`
	PrimaryRepeats      struct {
		Warm10GPUNS       uint64 `json:"warm_10_gpu_ns"`
		Warm10EndToEndNS  uint64 `json:"warm_10_end_to_end_ns"`
		Warm100GPUNS      uint64 `json:"warm_100_gpu_ns"`
		Warm100EndToEndNS uint64 `json:"warm_100_end_to_end_ns"`
	} `json:"primary_repeats"`
	Records []m45ArtifactRecord `json:"records"`
}

type m46ArtifactRecord struct {
	Workload          string `json:"workload"`
	Strategy          string `json:"strategy"`
	SubmitPolicy      string `json:"submit_policy"`
	ReductionPlan     string `json:"reduction_plan"`
	Tokens            uint32 `json:"tokens"`
	ModelWidth        uint32 `json:"model_width"`
	HeadDim           uint32 `json:"head_dim"`
	Correct           bool   `json:"correct"`
	SubmitCount       uint32 `json:"submit_count"`
	DispatchCount     uint32 `json:"dispatch_count"`
	BarrierCount      uint32 `json:"barrier_count"`
	ReplayID          uint64 `json:"replay_id"`
	M45ReplayID       uint64 `json:"m45_replay_id"`
	NGeneration       uint64 `json:"n_generation"`
	ReductionGPUNS    uint64 `json:"reduction_gpu_ns"`
	FinalReductionNS  uint64 `json:"final_reduction_gpu_ns"`
	InvRmsGPUNS       uint64 `json:"inv_rms_gpu_ns"`
	ApplyGPUNS        uint64 `json:"apply_gpu_ns"`
	M46GPUNS          uint64 `json:"m46_gpu_ns"`
	ResidualGPUNS     uint64 `json:"residual_gpu_ns"`
	CompleteGPUNS     uint64 `json:"complete_gpu_ns"`
	FinalReadbackNS   uint64 `json:"final_readback_ns"`
	HostCPUNormNS     uint64 `json:"host_cpu_norm_ns"`
	EndToEndNS        uint64 `json:"end_to_end_ns"`
	RetainedBytes     uint64 `json:"retained_bytes"`
	ExactRequestBytes uint64 `json:"exact_request_bytes"`
	PartialBytes      uint64 `json:"partial_bytes"`
	InvRmsBytes       uint64 `json:"inv_rms_bytes"`
	InPlaceSavedBytes uint64 `json:"in_place_saved_bytes"`
}

type m46Artifact struct {
	Schema              string  `json:"schema"`
	Epsilon             float32 `json:"epsilon"`
	WarmupsPerPlan      uint32  `json:"warmups_per_plan"`
	MeasurementsPerPlan uint32  `json:"measurements_per_plan"`
	Validation          struct {
		Warnings uint32 `json:"warnings"`
		Errors   uint32 `json:"errors"`
	} `json:"validation"`
	PrimaryRepeats struct {
		Warm10M46GPUNS       uint64 `json:"warm_10_m46_gpu_ns"`
		Warm10CompleteGPUNS  uint64 `json:"warm_10_complete_gpu_ns"`
		Warm10EndToEndNS     uint64 `json:"warm_10_end_to_end_ns"`
		Warm100M46GPUNS      uint64 `json:"warm_100_m46_gpu_ns"`
		Warm100CompleteGPUNS uint64 `json:"warm_100_complete_gpu_ns"`
		Warm100EndToEndNS    uint64 `json:"warm_100_end_to_end_ns"`
	} `json:"primary_repeats"`
	Records []m46ArtifactRecord `json:"records"`
}

var wordRE = regexp.MustCompile(`0x([0-9a-fA-F]{8})u`)

func main() {
	inventory := flag.Bool("inventory", false, "print production source and generated module hashes")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(fmt.Errorf("usage: go run ./tools/sdslv_workspace_check [-inventory]"))
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if err := check(root, *inventory); err != nil {
		fail(err)
	}
}

func check(root string, inventory bool) error {
	if err := conformance.Verify(root); err != nil {
		return fmt.Errorf("SDSL-V conformance: %w", err)
	}
	var m shaderManifest
	manifestPath := filepath.Join(root, "internal", "prometheus", "native", "shaders", "manifest.json")
	if err := readJSON(manifestPath, &m); err != nil {
		return err
	}
	if m.Workspace.ProductionSourceRoot == "" || m.Workspace.ExperimentalSourceRoot == "" || m.Workspace.HistoricalAuditRoot == "" || m.Workspace.CanonicalBenchmarkRoot == "" {
		return fmt.Errorf("shader manifest lacks complete workspace ownership roots")
	}
	for _, path := range []string{
		m.Workspace.ProductionSourceRoot,
		m.Workspace.ExperimentalSourceRoot,
		m.Workspace.HistoricalAuditRoot,
		m.Workspace.CanonicalBenchmarkRoot,
		"docs/SDSL_V_LANGUAGE_SPEC.md",
		"docs/SDSL_V_GRAPHICS_RECONCILIATION.md",
		"docs/SDSL_V_GRAPHICS_RECONCILIATION.json",
		"docs/SDSL_V_WORKSPACE.md",
		"examples/SDSL-V/conformance/manifest.json",
		"internal/prometheus/DevelopmentReport/SDSL_V_M41_CANONICAL_FULL_LANGUAGE_IMPLEMENTATION.md",
		"internal/prometheus/DevelopmentReport/SDSL_V_M39A_WORKSPACE_PRODUCTIZATION.md",
		"internal/prometheus/native/Marionette/reactor_shader_registry_tests.cpp",
	} {
		if err := mustExist(root, path); err != nil {
			return err
		}
	}
	registry, err := os.ReadFile(filepath.Join(root, "internal", "prometheus", "native", "reactor_shader_registry.c"))
	if err != nil {
		return err
	}
	seenIDs := map[uint32]string{}
	seenSources := map[string]string{}
	assetsByID := map[uint32]shaderAsset{}
	lines := make([]string, 0)
	for index, asset := range m.ShaderAssets {
		if prior, ok := seenIDs[asset.ID]; ok {
			return fmt.Errorf("duplicate shader id %d: %s and %s", asset.ID, prior, asset.Name)
		}
		if index != 0 && asset.ID <= m.ShaderAssets[index-1].ID {
			return fmt.Errorf("shader assets are not in deterministic ascending id order at %d", asset.ID)
		}
		seenIDs[asset.ID] = asset.Name
		assetsByID[asset.ID] = asset
		if asset.SourceLanguage != "sdslv" {
			continue
		}
		authority := asset.Authority
		if authority == "" {
			authority = "production"
		}
		switch authority {
		case "production":
			if !strings.HasPrefix(asset.Source, m.Workspace.ProductionSourceRoot+"/") {
				return fmt.Errorf("production registry asset %d (%s) points outside production source root: %s", asset.ID, asset.Name, asset.Source)
			}
			if strings.Contains(asset.Source, "/experimental/") || strings.Contains(asset.Source, "/historical/") {
				return fmt.Errorf("production registry asset %d points at non-production source: %s", asset.ID, asset.Source)
			}
		case "experimental":
			if !strings.HasPrefix(asset.Source, m.Workspace.ExperimentalSourceRoot+"/") {
				return fmt.Errorf("experimental registry asset %d (%s) points outside experimental source root: %s", asset.ID, asset.Name, asset.Source)
			}
		default:
			return fmt.Errorf("shader asset %d (%s) has unknown authority %q", asset.ID, asset.Name, authority)
		}
		if prior, ok := seenSources[asset.Source]; ok {
			return fmt.Errorf("duplicate SDSL-V source authority %s: %s and %s", asset.Source, prior, asset.Name)
		}
		seenSources[asset.Source] = asset.Name
		if err := mustExist(root, asset.Source); err != nil {
			return err
		}
		if !strings.Contains(string(registry), `"`+asset.Source+`"`) {
			return fmt.Errorf("registry does not own manifest source %s", asset.Source)
		}
		headerPath := filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(asset.Header))
		header, err := os.ReadFile(headerPath)
		if err != nil {
			return fmt.Errorf("asset %d generated header %s: %w", asset.ID, asset.Header, err)
		}
		if !strings.Contains(string(header), "// Source: "+asset.Source+"\n") {
			return fmt.Errorf("generated header %s does not identify source %s", asset.Header, asset.Source)
		}
		if !strings.Contains(string(header), "// Generated by: oct sdslv generate-header "+asset.Source) {
			return fmt.Errorf("generated header %s lacks regeneration command", asset.Header)
		}
		moduleHash, err := headerModuleHash(header)
		if err != nil {
			return fmt.Errorf("generated header %s: %w", asset.Header, err)
		}
		if inventory {
			lines = append(lines, fmt.Sprintf("id=%d name=%s authority=%s source_sha256=%s module_sha256=%s source=%s header=%s", asset.ID, asset.Name, authority, fileHash(filepath.Join(root, filepath.FromSlash(asset.Source))), moduleHash, asset.Source, asset.Header))
		}
	}
	seenExperimental := map[string]bool{}
	for _, asset := range m.ExperimentalShaderAssets {
		if asset.ID == "" || seenExperimental[asset.ID] {
			return fmt.Errorf("experimental shader asset has empty or duplicate id %q", asset.ID)
		}
		seenExperimental[asset.ID] = true
		if asset.Authority != "experimental" || asset.ProductionAuthority != "experimental" || asset.SelectorEligible {
			return fmt.Errorf("experimental shader asset %s must remain experimental and selector-ineligible", asset.ID)
		}
		if asset.SourceLanguage != "sdslv" || !strings.HasPrefix(asset.Source, m.Workspace.ExperimentalSourceRoot+"/") {
			return fmt.Errorf("experimental shader asset %s has invalid source ownership: %s", asset.ID, asset.Source)
		}
		for _, path := range []string{asset.Source, asset.Output, asset.GeneratedHLSL, asset.GeneratedHeader, asset.Inspection} {
			if err := mustExist(root, path); err != nil {
				return fmt.Errorf("experimental shader asset %s: %w", asset.ID, err)
			}
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(asset.Output))); got != strings.ToLower(asset.ShaderSHA256) {
			return fmt.Errorf("experimental shader asset %s hash mismatch: manifest=%s file=%s", asset.ID, asset.ShaderSHA256, got)
		}
		if strings.Contains(string(registry), asset.Source) {
			return fmt.Errorf("experimental shader asset %s leaked into the production runtime registry", asset.ID)
		}
	}
	seenImplementationIDs := map[uint32]string{}
	for index, implementation := range m.ComputeImplementations {
		if prior, ok := seenImplementationIDs[implementation.ID]; ok {
			return fmt.Errorf("duplicate compute implementation id %d: %s and %s", implementation.ID, prior, implementation.Name)
		}
		if index != 0 && implementation.ID <= m.ComputeImplementations[index-1].ID {
			return fmt.Errorf("compute implementations are not in deterministic ascending id order at %d", implementation.ID)
		}
		seenImplementationIDs[implementation.ID] = implementation.Name
		asset, ok := assetsByID[implementation.ShaderID]
		if !ok {
			return fmt.Errorf("compute implementation %d (%s) references unknown shader %d", implementation.ID, implementation.Name, implementation.ShaderID)
		}
		implementationAuthority := implementation.Authority
		if implementationAuthority == "" {
			implementationAuthority = "production"
		}
		assetAuthority := asset.Authority
		if assetAuthority == "" {
			assetAuthority = "production"
		}
		if implementationAuthority != assetAuthority {
			return fmt.Errorf("compute implementation %d authority %q disagrees with shader %d authority %q", implementation.ID, implementationAuthority, asset.ID, assetAuthority)
		}
		if implementationAuthority == "experimental" && implementation.SelectorEligible {
			return fmt.Errorf("experimental compute implementation %d (%s) must not be selector eligible", implementation.ID, implementation.Name)
		}
		isReduction := implementation.Operation == "row-sum" || implementation.Operation == "row-max" || implementation.Operation == "softmax"
		if isReduction {
			root := m.Workspace.ProductionSourceRoot
			if implementationAuthority == "experimental" {
				root = m.Workspace.ExperimentalSourceRoot
			}
			expectedPrefix := root + "/reduction/"
			if !strings.HasPrefix(asset.Source, expectedPrefix) {
				return fmt.Errorf("%s reduction implementation %d source must be confined to %s: %s", implementationAuthority, implementation.ID, expectedPrefix, asset.Source)
			}
		}
	}
	if err := checkCanonicalArtifacts(root); err != nil {
		return err
	}
	if err := checkBenchmarkIdentities(root); err != nil {
		return err
	}
	if err := checkM40bArtifact(root); err != nil {
		return err
	}
	if err := checkM42Artifact(root); err != nil {
		return err
	}
	if err := checkM44Artifact(root); err != nil {
		return err
	}
	if err := checkM45Artifact(root); err != nil {
		return err
	}
	if err := checkM46Artifact(root); err != nil {
		return err
	}
	if inventory {
		sort.Strings(lines)
		for _, line := range lines {
			fmt.Println(line)
		}
	}
	return nil
}

func checkM46Artifact(root string) error {
	var artifact m46Artifact
	path := filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "M46", "device_resident_rmsnorm_rtx3070.json")
	if err := readJSON(path, &artifact); err != nil {
		return fmt.Errorf("M46 RMSNorm artifact: %w", err)
	}
	if artifact.Schema != "prometheus.m46.device-resident-rmsnorm.v1" || artifact.Epsilon != 1.0e-5 ||
		artifact.WarmupsPerPlan != 16 || artifact.MeasurementsPerPlan != 5 ||
		artifact.PrimaryRepeats.Warm10M46GPUNS == 0 ||
		artifact.PrimaryRepeats.Warm10CompleteGPUNS == 0 ||
		artifact.PrimaryRepeats.Warm10EndToEndNS == 0 ||
		artifact.PrimaryRepeats.Warm100M46GPUNS == 0 ||
		artifact.PrimaryRepeats.Warm100CompleteGPUNS == 0 ||
		artifact.PrimaryRepeats.Warm100EndToEndNS == 0 {
		return fmt.Errorf("M46 RMSNorm artifact lacks identity or deterministic warm evidence")
	}
	if artifact.Validation.Warnings != 0 || artifact.Validation.Errors != 0 {
		return fmt.Errorf("M46 RMSNorm artifact is not validation-clean: warnings=%d errors=%d",
			artifact.Validation.Warnings, artifact.Validation.Errors)
	}
	workloads := map[string][3]uint32{
		"tiny": {16, 128, 16}, "primary": {128, 1024, 128},
		"more_tokens": {256, 1024, 128}, "wider": {128, 2048, 256},
		"awkward": {127, 1001, 127}, "staged_4096": {128, 4096, 512},
		"token_boundary": {1024, 1024, 128},
	}
	plans := [][2]string{
		{"separate_output", "one"}, {"separate_output", "two"},
		{"in_place_z", "one"}, {"in_place_z", "two"},
		{"m43_m44_m45_no_normalization", "one"},
		{"cpu_host_bounce", "final_z_readback_cpu_rmsnorm_no_reupload"},
	}
	want := make(map[string]bool, len(workloads)*len(plans))
	for workload := range workloads {
		for _, plan := range plans {
			want[workload+"/"+plan[0]+"/"+plan[1]] = false
		}
	}
	want["primary/in_place_z/one_forced_staged"] = false
	if len(artifact.Records) != len(want) {
		return fmt.Errorf("M46 RMSNorm artifact record count: got %d want %d", len(artifact.Records), len(want))
	}
	for _, record := range artifact.Records {
		key := record.Workload + "/" + record.Strategy + "/" + record.SubmitPolicy
		seen, known := want[key]
		if !known || seen {
			return fmt.Errorf("M46 RMSNorm artifact has unexpected or duplicate record %s", key)
		}
		want[key] = true
		shape := workloads[record.Workload]
		if record.Tokens != shape[0] || record.ModelWidth != shape[1] || record.HeadDim != shape[2] ||
			!record.Correct || record.ResidualGPUNS == 0 || record.CompleteGPUNS == 0 ||
			record.FinalReadbackNS == 0 || record.EndToEndNS == 0 {
			return fmt.Errorf("M46 RMSNorm artifact record %s lacks shape, timing, or correctness evidence", key)
		}
		saved := uint64(record.Tokens) * uint64(record.ModelWidth) * 4
		switch record.Strategy {
		case "separate_output", "in_place_z":
			wantSubmits := uint32(1)
			if record.SubmitPolicy == "two" {
				wantSubmits = 2
			}
			wantReduction := "fused"
			wantDispatches := uint32(2)
			wantBarriers := uint32(5)
			wantPartials := uint64(0)
			if record.ModelWidth > 1024 || record.SubmitPolicy == "one_forced_staged" {
				wantReduction = "staged"
				wantDispatches = 3
				wantBarriers = 6
				partials := (record.ModelWidth + 1023) / 1024
				wantPartials = uint64(record.Tokens) * uint64(partials) * 4
			}
			if record.SubmitCount != wantSubmits || record.ReductionPlan != wantReduction ||
				record.DispatchCount != wantDispatches || record.BarrierCount != wantBarriers ||
				record.ReplayID == 0 || record.M45ReplayID == 0 || record.NGeneration == 0 ||
				record.ReductionGPUNS == 0 || record.InvRmsGPUNS == 0 ||
				record.ApplyGPUNS == 0 || record.M46GPUNS == 0 ||
				record.RetainedBytes == 0 || record.ExactRequestBytes == 0 ||
				record.PartialBytes != wantPartials || record.InvRmsBytes != uint64(record.Tokens)*4 ||
				record.InPlaceSavedBytes != saved {
				return fmt.Errorf("M46 device record %s lacks plan, identity, timing, or memory evidence", key)
			}
			if wantReduction == "staged" && record.FinalReductionNS == 0 {
				return fmt.Errorf("M46 staged record %s lacks final reduction timing", key)
			}
		case "m43_m44_m45_no_normalization":
			if record.M46GPUNS != 0 || record.HostCPUNormNS != 0 || record.ReductionPlan != "none" {
				return fmt.Errorf("M46 no-normalization record %s contains normalization work", key)
			}
		case "cpu_host_bounce":
			if record.M46GPUNS != 0 || record.HostCPUNormNS == 0 || record.ReductionPlan != "none" {
				return fmt.Errorf("M46 host-bounce record %s lacks CPU normalization evidence", key)
			}
		}
	}
	for key, present := range want {
		if !present {
			return fmt.Errorf("M46 RMSNorm artifact lacks required record %s", key)
		}
	}
	return nil
}

func checkM45Artifact(root string) error {
	var artifact m45Artifact
	path := filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "M45", "device_resident_residual_rtx3070.json")
	if err := readJSON(path, &artifact); err != nil {
		return fmt.Errorf("M45 residual artifact: %w", err)
	}
	if artifact.Schema != "prometheus.m45.device-resident-residual.v1" ||
		artifact.WarmupsPerPlan != 32 || artifact.MeasurementsPerPlan != 5 ||
		artifact.PrimaryRepeats.Warm10GPUNS == 0 || artifact.PrimaryRepeats.Warm10EndToEndNS == 0 ||
		artifact.PrimaryRepeats.Warm100GPUNS == 0 || artifact.PrimaryRepeats.Warm100EndToEndNS == 0 {
		return fmt.Errorf("M45 residual artifact lacks identity or deterministic warm evidence")
	}
	if artifact.Validation.Warnings != 0 || artifact.Validation.Errors != 0 {
		return fmt.Errorf("M45 residual artifact is not validation-clean: warnings=%d errors=%d",
			artifact.Validation.Warnings, artifact.Validation.Errors)
	}
	shaderPaths := [3]string{
		"internal/prometheus/shaders/sdslv/experimental/transformer/residual_add.sdslv",
		"internal/prometheus/shaders/sdslv/experimental/transformer/residual_add.hlsl",
		"internal/prometheus/shaders/sdslv/experimental/transformer/residual_add.spv",
	}
	if artifact.Shader.SourceSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shaderPaths[0]))) ||
		artifact.Shader.HLSLSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shaderPaths[1]))) ||
		artifact.Shader.SPVSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shaderPaths[2]))) {
		return fmt.Errorf("M45 residual artifact shader provenance mismatch")
	}
	workloads := map[string][3]uint32{
		"tiny": {16, 128, 16}, "primary": {128, 1024, 128},
		"more_tokens": {256, 1024, 128}, "wider": {128, 2048, 256},
		"awkward": {127, 1001, 127}, "boundary": {1024, 1024, 64},
	}
	plans := [][2]string{
		{"separate_output", "one"}, {"separate_output", "two"},
		{"in_place_y", "one"}, {"in_place_y", "two"},
		{"m43_m44_no_residual", "one"},
		{"cpu_host_bounce", "x_y_readback_cpu_add_no_reupload"},
	}
	want := make(map[string]bool, len(workloads)*len(plans))
	for workload := range workloads {
		for _, plan := range plans {
			want[workload+"/"+plan[0]+"/"+plan[1]] = false
		}
	}
	if len(artifact.Records) != len(want) {
		return fmt.Errorf("M45 residual artifact record count: got %d want %d", len(artifact.Records), len(want))
	}
	for _, record := range artifact.Records {
		key := record.Workload + "/" + record.Strategy + "/" + record.SubmitPolicy
		seen, known := want[key]
		if !known || seen {
			return fmt.Errorf("M45 residual artifact has unexpected or duplicate record %s", key)
		}
		want[key] = true
		shape := workloads[record.Workload]
		if record.Tokens != shape[0] || record.ModelWidth != shape[1] || record.HeadDim != shape[2] ||
			!record.Correct || record.ReplayID == 0 || record.M44ReplayID == 0 ||
			record.M43GPUNS == 0 || record.M44GPUNS == 0 || record.TotalGPUNS == 0 ||
			record.FinalReadbackNS == 0 || record.EndToEndNS == 0 || record.RetainedBytes == 0 {
			return fmt.Errorf("M45 residual artifact record %s lacks shape, identity, timing, or correctness evidence", key)
		}
		saved := uint64(record.Tokens) * uint64(record.ModelWidth) * 4
		switch record.Strategy {
		case "separate_output", "in_place_y":
			wantSubmits := uint32(1)
			if record.SubmitPolicy == "two" {
				wantSubmits = 2
			}
			if record.ZGeneration == 0 || record.ResidualGPUNS == 0 ||
				record.InPlaceSavedBytes != saved || record.SubmitCount != wantSubmits {
				return fmt.Errorf("M45 device record %s lacks residual ownership evidence", key)
			}
		case "m43_m44_no_residual":
			if record.ZGeneration != 0 || record.ResidualGPUNS != 0 || record.CPUAddNS != 0 {
				return fmt.Errorf("M45 no-residual record %s contains residual work", key)
			}
		case "cpu_host_bounce":
			if record.ZGeneration != 0 || record.ResidualGPUNS != 0 || record.CPUAddNS == 0 ||
				record.XReadbackNS == 0 || record.SubmitCount != 2 {
				return fmt.Errorf("M45 host-bounce lower-bound record %s lacks CPU add evidence", key)
			}
		}
	}
	for key, present := range want {
		if !present {
			return fmt.Errorf("M45 residual artifact lacks required record %s", key)
		}
	}
	return nil
}

func checkM44Artifact(root string) error {
	var artifact m44Artifact
	path := filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "M44", "multihead_aggregation_output_projection_rtx3070.json")
	if err := readJSON(path, &artifact); err != nil {
		return fmt.Errorf("M44 output projection artifact: %w", err)
	}
	if artifact.Schema != "prometheus.m44.multihead-output-projection.v1" || artifact.HeadCount != 8 ||
		artifact.SourceLayout != "head_major_views" ||
		artifact.LogicalConcatenation != "C[token,head*HeadDim+column]" ||
		artifact.OutputLayout != "row_major_tokens_model_width" {
		return fmt.Errorf("M44 output projection artifact has incomplete tensor identity")
	}
	if artifact.WarmupOperations != 32 || artifact.MeasurementOperations != 5 || artifact.CapacityPrime != 2 ||
		artifact.PrimaryRepeats.Warm10GPUNS == 0 || artifact.PrimaryRepeats.Warm10EndToEndNS == 0 ||
		artifact.PrimaryRepeats.Warm100GPUNS == 0 || artifact.PrimaryRepeats.Warm100EndToEndNS == 0 {
		return fmt.Errorf("M44 output projection artifact lacks deterministic warm evidence")
	}
	if artifact.Precision.CooperativeInput != "f16_rne" || artifact.Precision.CooperativeWeight != "f16_rne" ||
		artifact.Precision.Accumulation != "fp32" || artifact.Precision.Output != "fp32" {
		return fmt.Errorf("M44 output projection artifact has invalid precision identity")
	}
	if artifact.Validation.Warnings != 0 || artifact.Validation.Errors != 0 {
		return fmt.Errorf("M44 output projection artifact is not validation-clean: warnings=%d errors=%d",
			artifact.Validation.Warnings, artifact.Validation.Errors)
	}
	shaderPaths := map[string]struct {
		artifact m44ShaderArtifact
		paths    [3]string
	}{
		"interleave": {artifact: artifact.ShaderArtifacts.Interleave, paths: [3]string{
			"internal/prometheus/shaders/sdslv/experimental/attention/interleave_heads.sdslv",
			"internal/prometheus/shaders/sdslv/experimental/attention/interleave_heads.hlsl",
			"internal/prometheus/shaders/sdslv/experimental/attention/interleave_heads.spv",
		}},
		"direct_segmented": {artifact: artifact.ShaderArtifacts.DirectSegmented, paths: [3]string{
			"internal/prometheus/shaders/sdslv/experimental/attention/direct_segmented_projection.sdslv",
			"internal/prometheus/shaders/sdslv/experimental/attention/direct_segmented_projection.hlsl",
			"internal/prometheus/shaders/sdslv/experimental/attention/direct_segmented_projection.spv",
		}},
	}
	if artifact.ShaderArtifacts.DXC == "" {
		return fmt.Errorf("M44 output projection artifact lacks DXC identity")
	}
	for identity, shader := range shaderPaths {
		if shader.artifact.SourceSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shader.paths[0]))) ||
			shader.artifact.HLSLSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shader.paths[1]))) ||
			shader.artifact.SPVSHA256 != fileHash(filepath.Join(root, filepath.FromSlash(shader.paths[2]))) {
			return fmt.Errorf("M44 output projection artifact shader provenance mismatch for %s", identity)
		}
	}
	workloads := map[string][3]uint32{
		"tiny": {16, 16, 128}, "primary": {128, 128, 1024},
		"more_tokens": {256, 128, 1024}, "larger_head": {128, 256, 1024},
		"awkward": {127, 127, 1001}, "softmax_boundary": {1024, 64, 128},
	}
	plans := [][3]string{
		{"interleave", "cooperative", "one"},
		{"interleave", "a2x4_fp32", "one"},
		{"interleave", "conventional_fp16", "one"},
		{"direct_segmented", "direct_fp16", "one"},
		{"interleave", "cooperative", "two"},
		{"host_bounce", "cooperative", "two_cpu_separated"},
		{"no_output_projection", "m43_only", "one"},
	}
	want := make(map[string]bool, len(workloads)*len(plans))
	for workload := range workloads {
		for _, plan := range plans {
			want[workload+"/"+strings.Join(plan[:], "/")] = false
		}
	}
	retainedByWorkload := make(map[string]uint64, len(workloads))
	if len(artifact.Records) != len(want) {
		return fmt.Errorf("M44 output projection artifact record count: got %d want %d", len(artifact.Records), len(want))
	}
	for _, record := range artifact.Records {
		key := record.Workload + "/" + record.Strategy + "/" + record.Path + "/" + record.SubmitPlan
		seen, known := want[key]
		if !known || seen {
			return fmt.Errorf("M44 output projection artifact has unexpected or duplicate record %s", key)
		}
		want[key] = true
		shape := workloads[record.Workload]
		if record.Tokens != shape[0] || record.HeadDim != shape[1] || record.ModelWidth != shape[2] ||
			!record.Correct || record.ReplayID == 0 || record.M43ReplayID == 0 ||
			record.TotalGPUNS == 0 || record.FinalReadbackNS == 0 || record.EndToEndNS == 0 || record.RetainedBytes == 0 {
			return fmt.Errorf("M44 output projection artifact record %s lacks shape, timing, identity, or correctness evidence", key)
		}
		switch record.Strategy {
		case "host_bounce":
			if record.IntermediateHostCopies != 1 || record.SubmitCount != 2 ||
				record.CPUConcatenateNS == 0 || record.CPUPackNS == 0 || record.M44GPUNS == 0 {
				return fmt.Errorf("M44 host-bounce record %s lacks its explicit residency violation", key)
			}
		case "no_output_projection":
			if record.M44GPUNS != 0 || record.IntermediateHostCopies != 0 {
				return fmt.Errorf("M44 no-projection record %s contains M44 work", key)
			}
		default:
			if record.IntermediateHostCopies != 0 || record.M44GPUNS == 0 || record.SubmitCount == 0 ||
				record.SourceHeadBytes == 0 || record.WoUploadBytes == 0 || record.WoF32Bytes == 0 ||
				record.WoPackedBytes == 0 || record.FinalYBytes == 0 || record.FinalReadbackBytes == 0 ||
				record.ReusableDescriptorSets != 2 || record.DescriptorBindings != 14 ||
				record.PartialOutputBytes != 0 || record.AccumulationBytes != 0 {
				return fmt.Errorf("M44 device record %s lacks memory or residency evidence", key)
			}
			if prior, ok := retainedByWorkload[record.Workload]; ok && prior != record.RetainedBytes {
				return fmt.Errorf("M44 device record %s uses mismatched retained capacity", key)
			}
			retainedByWorkload[record.Workload] = record.RetainedBytes
			if record.Path == "a2x4_fp32" && (record.ContiguousF32Bytes == 0 || record.ContiguousPackedBytes != 0) {
				return fmt.Errorf("M44 A2x4 record %s has invalid interleave storage", key)
			}
			if (record.Path == "cooperative" || record.Path == "conventional_fp16") &&
				(record.ContiguousPackedBytes == 0 || record.ContiguousF32Bytes != 0) {
				return fmt.Errorf("M44 rounded record %s has invalid packed interleave storage", key)
			}
			if record.Path == "direct_fp16" && (record.TemporaryBytes != 0 || record.ContiguousF32Bytes != 0 || record.ContiguousPackedBytes != 0) {
				return fmt.Errorf("M44 direct record %s unexpectedly materializes concatenation", key)
			}
		}
	}
	for key, present := range want {
		if !present {
			return fmt.Errorf("M44 output projection artifact lacks required record %s", key)
		}
	}
	return nil
}

func checkM42Artifact(root string) error {
	var artifact m42Artifact
	path := filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "M42", "device_resident_attention_rtx3070.json")
	if err := readJSON(path, &artifact); err != nil {
		return fmt.Errorf("M42 attention artifact: %w", err)
	}
	if artifact.Schema != "prometheus.m42.device-resident-attention.v1" ||
		artifact.TensorConvention == "" || artifact.PrecisionContract == "" {
		return fmt.Errorf("M42 attention artifact has incomplete identity")
	}
	if artifact.WarmRepetitions != 3 || artifact.PrimaryWarm10MedianNS == 0 ||
		artifact.PrimaryWarm100MedianNS == 0 || artifact.Device.CooperativeState != 5 ||
		artifact.Device.SubgroupSize != 32 {
		return fmt.Errorf("M42 attention artifact lacks warm/capability evidence")
	}
	if artifact.Validation.Warnings != 0 || artifact.Validation.Errors != 0 {
		return fmt.Errorf("M42 attention artifact is not validation-clean: warnings=%d errors=%d",
			artifact.Validation.Warnings, artifact.Validation.Errors)
	}
	workloads := map[string][3]uint32{
		"tiny":             {16, 64, 16},
		"primary":          {128, 1024, 128},
		"larger-head":      {128, 1024, 256},
		"more-tokens":      {256, 1024, 128},
		"awkward-padded":   {127, 1001, 127},
		"softmax-boundary": {1024, 128, 64},
	}
	paths := map[string]uint32{
		"cooperative-f16":   1,
		"a2x4-fp32":         2,
		"conventional-fp16": 3,
	}
	want := make(map[string]bool, len(workloads)*len(paths))
	for workload := range workloads {
		for path := range paths {
			want[workload+"/"+path] = false
		}
	}
	if len(artifact.Records) != len(want) {
		return fmt.Errorf("M42 attention artifact record count: got %d want %d", len(artifact.Records), len(want))
	}
	for _, record := range artifact.Records {
		key := record.Workload + "/" + record.Path
		seen, known := want[key]
		if !known || seen {
			return fmt.Errorf("M42 attention artifact has unexpected or duplicate record %s", key)
		}
		want[key] = true
		shape := workloads[record.Workload]
		if record.Tokens != shape[0] || record.ModelWidth != shape[1] || record.HeadDim != shape[2] ||
			record.SelectedPath != paths[record.Path] || !record.Correct {
			return fmt.Errorf("M42 attention artifact record %s has incorrect shape, path, or result", key)
		}
		if record.ReplayID == 0 || record.ReductionReplayID == 0 ||
			record.QProjectionGPUNS == 0 || record.KProjectionGPUNS == 0 || record.VProjectionGPUNS == 0 ||
			record.KLayoutGPUNS == 0 || record.QKGPUNS == 0 || record.ScaleGPUNS == 0 ||
			record.SoftmaxGPUNS == 0 || record.PVGPUNS == 0 || record.TotalAttentionGPUNS == 0 ||
			record.HostFedEndToEndNS == 0 || record.ResidentXEndToEndNS == 0 ||
			record.FinalReadbackNS == 0 || record.RetainedBytes == 0 {
			return fmt.Errorf("M42 attention artifact record %s lacks timing, identity, or ownership evidence", key)
		}
	}
	for key, present := range want {
		if !present {
			return fmt.Errorf("M42 attention artifact lacks required record %s", key)
		}
	}
	return nil
}

func checkM40bArtifact(root string) error {
	var artifact m40bArtifact
	path := filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "M40B", "device_resident_inference_rtx3070.json")
	if err := readJSON(path, &artifact); err != nil {
		return fmt.Errorf("M40b composed artifact: %w", err)
	}
	if artifact.Schema != "prometheus.m40b.device-resident-inference.v1" || artifact.Notation != "M x N x K" {
		return fmt.Errorf("M40b composed artifact has unexpected identity: schema=%q notation=%q", artifact.Schema, artifact.Notation)
	}
	if artifact.WarmRepetitions == 0 || artifact.Capability.State != 5 || artifact.Capability.Tuple != "subgroup-m16-n16-k16-f16-f16-f32-f32" || artifact.Capability.SubgroupSize != 32 {
		return fmt.Errorf("M40b composed artifact has incomplete capability or repetition evidence: warm=%d state=%d tuple=%q subgroup=%d", artifact.WarmRepetitions, artifact.Capability.State, artifact.Capability.Tuple, artifact.Capability.SubgroupSize)
	}
	if artifact.Validation.Warnings != 0 || artifact.Validation.Errors != 0 {
		return fmt.Errorf("M40b composed artifact is not validation-clean: warnings=%d errors=%d", artifact.Validation.Warnings, artifact.Validation.Errors)
	}

	shapes := [][3]uint32{
		{128, 1024, 1024}, {256, 1024, 1024}, {512, 1024, 1024}, {1024, 1024, 1024},
		{256, 4096, 1024}, {1024, 4096, 1024},
		{128, 320, 1024}, {128, 640, 1024}, {128, 768, 1024}, {128, 1280, 1024}, {128, 2048, 1024},
		{127, 1001, 1023}, {257, 769, 1025}, {511, 1281, 2049},
	}
	kernels := []string{"cooperative", "A2x4", "conventional-fp16"}
	want := make(map[string]bool, len(shapes)*len(kernels))
	for _, shape := range shapes {
		for _, kernel := range kernels {
			want[fmt.Sprintf("%dx%dx%d/%s", shape[0], shape[1], shape[2], kernel)] = false
		}
	}
	if len(artifact.Rows) != len(want) {
		return fmt.Errorf("M40b composed artifact row count: got %d want %d", len(artifact.Rows), len(want))
	}
	for _, row := range artifact.Rows {
		key := fmt.Sprintf("%dx%dx%d/%s", row.M, row.N, row.K, row.Kernel)
		seen, known := want[key]
		if !known {
			return fmt.Errorf("M40b composed artifact has unexpected workload %s", key)
		}
		if seen {
			return fmt.Errorf("M40b composed artifact has duplicate workload %s", key)
		}
		want[key] = true
		if !row.Correct || row.M == 0 || row.N == 0 || row.K == 0 {
			return fmt.Errorf("M40b composed artifact workload %s is incorrect or zero-sized", key)
		}
		if row.PaddedM < row.M || row.PaddedN < row.N || row.PaddedK < row.K || row.PaddedM%16 != 0 || row.PaddedN%16 != 0 || row.PaddedK%16 != 0 {
			return fmt.Errorf("M40b composed artifact workload %s has invalid padding %dx%dx%d", key, row.PaddedM, row.PaddedN, row.PaddedK)
		}
		if row.CommandPlanReplayID == 0 || row.ReductionReplayID == 0 {
			return fmt.Errorf("M40b composed artifact workload %s lacks replay identity", key)
		}
		if err := checkM40bTrace(key+" host-one", row.CommandTraces.HostOne, 1, false); err != nil {
			return err
		}
		if err := checkM40bTrace(key+" resident-one", row.CommandTraces.ResidentOne, 1, true); err != nil {
			return err
		}
		if err := checkM40bTrace(key+" resident-two", row.CommandTraces.ResidentTwo, 2, true); err != nil {
			return err
		}
		if row.PersistentBNewA.SGEMM.MedianNS == 0 || row.PersistentBNewA.Softmax.MedianNS == 0 || row.PersistentBNewA.Combined.MedianNS == 0 || row.PersistentBNewA.Readback.MedianNS == 0 || row.PersistentBNewA.EndToEnd.MedianNS == 0 || row.DeviceAB.Combined.MedianNS == 0 || row.DeviceAB.EndToEnd.MedianNS == 0 {
			return fmt.Errorf("M40b composed artifact workload %s lacks separated timing evidence", key)
		}
		switch row.Kernel {
		case "cooperative":
			if row.Precision != "f16-rounded-input-f32-accum-output" || row.ShaderHash == 0 {
				return fmt.Errorf("M40b cooperative workload %s has invalid precision or shader identity", key)
			}
		case "A2x4":
			if row.Precision != "fp32-input-accum-output" || row.ShaderHash != 0 {
				return fmt.Errorf("M40b A2x4 workload %s has invalid precision or authority", key)
			}
		case "conventional-fp16":
			if row.Precision != "f16-rounded-input-f32-accum-output" || row.ShaderHash != 0 {
				return fmt.Errorf("M40b conventional FP16 workload %s has invalid precision or authority", key)
			}
		}
	}
	for key, present := range want {
		if !present {
			return fmt.Errorf("M40b composed artifact lacks required workload %s", key)
		}
	}
	repeatKey := "128x1024x1024/cooperative"
	for _, row := range artifact.Rows {
		if fmt.Sprintf("%dx%dx%d/%s", row.M, row.N, row.K, row.Kernel) == repeatKey && (row.Repeat10MedianEndToEndNS == 0 || row.Repeat100MedianEndToEndNS == 0) {
			return fmt.Errorf("M40b composed artifact lacks 10/100-operation amortization for %s", repeatKey)
		}
	}
	return nil
}

func checkM40bTrace(label string, trace m40bTrace, submitCount uint32, resident bool) error {
	if trace.EntryCount == 0 || trace.EntryCount != uint32(len(trace.Entries)) || trace.SubmitCount != submitCount ||
		trace.IntermediateBufferCount != 1 || trace.IntermediateHostCopyCount != 0 || trace.FinalReadbackCopyCount != 1 || trace.ReplayID == 0 {
		return fmt.Errorf("M40b command trace %s has invalid ownership or identity", label)
	}
	seen := map[uint32]bool{}
	for _, entry := range trace.Entries {
		seen[entry.Operation] = true
		if entry.SourceQueueFamily != 0xffffffff || entry.DestinationQueueFamily != 0xffffffff {
			return fmt.Errorf("M40b command trace %s contains an accidental queue-family transfer", label)
		}
	}
	for _, operation := range []uint32{2, 3, 4, 6, 9, 12, 13, 14, 15, 18, 19} {
		if !seen[operation] {
			return fmt.Errorf("M40b command trace %s lacks operation %d", label, operation)
		}
	}
	if resident && seen[1] {
		return fmt.Errorf("M40b command trace %s unexpectedly uploads host A", label)
	}
	if !resident && !seen[1] {
		return fmt.Errorf("M40b command trace %s lacks host-A upload", label)
	}
	if submitCount == 2 && !seen[10] {
		return fmt.Errorf("M40b command trace %s lacks its explicit submit dependency", label)
	}
	return nil
}

func checkCanonicalArtifacts(root string) error {
	var m canonicalManifest
	path := filepath.Join(root, "examples", "SDSL-V", "M36a", "artifacts", "manifest.json")
	if err := readJSON(path, &m); err != nil {
		return err
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("canonical benchmark artifact manifest is empty")
	}
	for _, artifact := range m.Artifacts {
		if err := mustExist(root, artifact.Source); err != nil {
			return err
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(artifact.Source))); got != artifact.SourceSHA256 {
			return fmt.Errorf("canonical benchmark %s source hash: got %s want %s", artifact.Name, got, artifact.SourceSHA256)
		}
		if err := mustExist(root, artifact.SPIRVPath); err != nil {
			return err
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(artifact.SPIRVPath))); got != artifact.SPIRVSHA256 {
			return fmt.Errorf("canonical benchmark %s SPIR-V hash: got %s want %s", artifact.Name, got, artifact.SPIRVSHA256)
		}
	}
	return nil
}

func checkBenchmarkIdentities(root string) error {
	source := filepath.Join(root, "examples", "SDSL-V", "M36a", "BasicBenchmarks.sdslvbench")
	benchmarks, err := bench.Discover(source)
	if err != nil {
		return fmt.Errorf("discover permanent benchmark corpus: %w", err)
	}
	byID := make(map[string]bench.Case, len(benchmarks.Benchmarks))
	for _, benchmark := range benchmarks.Benchmarks {
		if benchmark.ReplayID == "" {
			return fmt.Errorf("benchmark %s has no replay id", benchmark.ID)
		}
		byID[benchmark.ID] = benchmark
	}
	var artifacts canonicalManifest
	artifactPath := filepath.Join(root, "examples", "SDSL-V", "M36a", "artifacts", "manifest.json")
	if err := readJSON(artifactPath, &artifacts); err != nil {
		return err
	}
	for _, artifact := range artifacts.Artifacts {
		if _, ok := byID[artifact.BenchmarkID]; !ok {
			return fmt.Errorf("canonical benchmark artifact %s references unknown benchmark id %s", artifact.Name, artifact.BenchmarkID)
		}
	}
	return nil
}

func headerModuleHash(header []byte) (string, error) {
	matches := wordRE.FindAllSubmatch(header, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("contains no SPIR-V words")
	}
	words := make([]byte, len(matches)*4)
	for i, match := range matches {
		var value uint32
		if _, err := fmt.Sscanf(string(match[1]), "%x", &value); err != nil {
			return "", err
		}
		binary.LittleEndian.PutUint32(words[i*4:], value)
	}
	sum := sha256.Sum256(words)
	return hex.EncodeToString(sum[:]), nil
}

func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func mustExist(root, path string) error {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		return fmt.Errorf("required workspace path missing: %s", path)
	}
	return nil
}

func fileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sdslv workspace check:", err)
	os.Exit(1)
}
