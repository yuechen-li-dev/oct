// vulkan_shader_comparison produces the Build Week direct shader evidence.
//
// It deliberately compares generated SPIR-V in the same Kaiju Vulkan sidecar;
// it does not involve llama.cpp graph selection, CUDA, or model throughput.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	internaloctx "github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

const (
	ggmlCommit = "9be313313c8ecb9488911bd64550190e3ed80f38"
	iterations = 30
	warmups    = 5
)

type module struct {
	Family, Operation, Name, Entry, Source, SourceSHA256 string
	SPV                                                  []byte
	CompileNS                                            int64
	Command                                              []string
	Workgroup                                            kaijuvulkan.UInt3
	Includes                                             []string
}

type run struct {
	Family, Operation, Mode string
	Width, Rows             uint32
	Epsilon                 float32
	Pass                    int
	GPU                     []uint64
	GPUStats                timingStats
	HostWallNS              int64
	L2, Linf, Relative      float64
	Finite, InPlace         bool
	Validation              kaijuvulkan.ValidationStatus
}

type timingStats struct {
	MedianNS, MinimumNS, P95NS uint64
	MeanNS, StdDevNS           float64
}

type report struct {
	Schema       string `json:"schema"`
	Status       string `json:"status"`
	GeneratedUTC string `json:"generated_utc"`
	GGML         struct {
		Commit string `json:"commit"`
		Source string `json:"source"`
	} `json:"ggml"`
	Environment                   map[string]string `json:"environment"`
	Modules                       []moduleRecord    `json:"modules"`
	Runs                          []run             `json:"runs"`
	Rejections                    []string          `json:"rejected_comparisons"`
	Limits                        []string          `json:"limits"`
	DeterministicModuleIdentity   bool              `json:"deterministic_module_identity"`
	DeterministicArtifactIdentity bool              `json:"deterministic_artifact_identity"`
}

type moduleRecord struct {
	Family, Operation, Name, Entry, Source, SourceSHA256, SHA256 string
	Bytes                                                        int
	Workgroup                                                    kaijuvulkan.UInt3
	CompileNS                                                    int64
	Command                                                      []string
	Includes                                                     []string
	SpirvValidation                                              string
	OpcodeCount                                                  int
	Capabilities, Extensions, StorageClasses                     []string
	WorkgroupVariables, Barriers                                 int
}

func main() {
	runHardware := flag.Bool("run", false, "run the direct RMSNorm and single-dispatch reduction matrices")
	probePacked := flag.Bool("probe-packed", false, "run the bounded packed-short-row comparison probe")
	out := flag.String("out", filepath.Join("docs", "build-week", "artifacts", "ggml_sdslv_spirv_comparison.json"), "committed JSON evidence output")
	ggmlRoot := flag.String("ggml-root", "", "fresh GGML checkout; defaults to out/vulkan_shader_comparison/ggml")
	flag.Parse()
	root, err := repoRoot()
	die(err)
	if *ggmlRoot == "" {
		*ggmlRoot = filepath.Join(root, "out", "vulkan_shader_comparison", "ggml")
	}
	if err := prepareGGML(root, *ggmlRoot); err != nil {
		die(err)
	}
	modules, err := generateModules(root, *ggmlRoot)
	if err != nil {
		die(err)
	}
	report := report{Schema: "oct.build_week.ggml_sdslv_spirv_comparison.v1", Status: "audit_only", GeneratedUTC: time.Now().UTC().Format(time.RFC3339Nano), Environment: environment()}
	report.GGML.Commit, report.GGML.Source = ggmlCommit, "https://github.com/ggml-org/ggml/tree/"+ggmlCommit+"/src/ggml-vulkan/vulkan-shaders"
	for _, item := range modules {
		inspected, err := inspectModule(root, item)
		if err != nil {
			die(err)
		}
		report.Modules = append(report.Modules, inspected)
	}
	report.Rejections = []string{
		"GGML has no standalone row-wise numeric max shader; argmax is not a max-value match.",
		"GGML soft_max_f32 is eligible only with KY=0, scale=1, max_bias=0, and has_sinks=0; mask, sink, and ALiBi cases are excluded.",
		"GGML F16/F32 matmul is excluded: the public shader ABI/layout cannot be matched to the current SDSL-V FP16 production layout without introducing asymmetric packing.",
	}
	report.Limits = []string{
		"Kaiju is intentionally used only for one-dispatch plans. Widths above 1024 need the production SDSL-V multi-stage reduction plan and are deferred to a Prometheus adapter; they are not represented as a false single-dispatch match.",
		"Kaiju v1 reports GPU timestamp samples. Its current public response does not isolate Vulkan pipeline-creation or host submit/wait timing, so those fields are explicitly absent rather than inferred from process wall time.",
		"The SDSL-V RMSNorm module is the existing llama.cpp ABI adapter, not a Prometheus production registry asset; the row-sum and fused-softmax modules are production SDSL-V assets.",
	}
	if *probePacked {
		if err := runPackedProbe(root, modules, &report); err != nil {
			die(err)
		}
		report.Status = "meaningful_progression"
	}
	if *runHardware {
		if err := runMatrices(root, modules, &report); err != nil {
			die(err)
		}
		report.Status = "meaningful_progression"
	}
	repeated, err := generateModules(root, *ggmlRoot)
	if err != nil {
		die(err)
	}
	if !sameModules(modules, repeated) {
		die(errors.New("second GGML/SDSL-V generation changed module identity"))
	}
	report.DeterministicModuleIdentity = true
	second := report
	second.Modules = nil
	for _, item := range repeated {
		inspected, err := inspectModule(root, item)
		if err != nil {
			die(err)
		}
		second.Modules = append(second.Modules, inspected)
	}
	firstIdentity, err := deterministicArtifactIdentity(report)
	if err != nil {
		die(err)
	}
	secondIdentity, err := deterministicArtifactIdentity(second)
	if err != nil {
		die(err)
	}
	if firstIdentity != secondIdentity {
		die(errors.New("second generated artifact changed after excluding volatile fields"))
	}
	report.DeterministicArtifactIdentity = true
	if err := archive(root, *ggmlRoot, modules); err != nil {
		die(err)
	}
	path := filepath.Join(root, *out)
	if err := writeJSON(path, report); err != nil {
		die(err)
	}
	fmt.Printf("wrote %s\n", path)
}

func repoRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		p := filepath.Dir(d)
		if p == d {
			return "", errors.New("repository root not found")
		}
		d = p
	}
}

func prepareGGML(root, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := command(root, "git", "clone", "--filter=blob:none", "https://github.com/ggml-org/ggml.git", dir); err != nil {
			return err
		}
	}
	if err := command(root, "git", "-C", dir, "fetch", "--depth", "1", "origin", ggmlCommit); err != nil {
		return err
	}
	if err := command(root, "git", "-C", dir, "checkout", "--detach", ggmlCommit); err != nil {
		return err
	}
	b, err := commandOutput(root, "git", "-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) != ggmlCommit {
		return fmt.Errorf("GGML checkout is %s, expected %s", strings.TrimSpace(string(b)), ggmlCommit)
	}
	return nil
}

func generateModules(root, ggml string) ([]module, error) {
	out := filepath.Join(root, "out", "vulkan_shader_comparison", "generated")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	shaderDir := filepath.Join(ggml, "src", "ggml-vulkan", "vulkan-shaders")
	generator := filepath.Join(out, "vulkan-shaders-gen.exe")
	cxx := os.Getenv("CXX")
	if cxx == "" {
		cxx = `C:\msys64\ucrt64\bin\g++.exe`
	}
	if _, err := os.Stat(cxx); err != nil {
		return nil, fmt.Errorf("CXX not found at %s; set CXX to a C++17 compiler", cxx)
	}
	if err := command(root, cxx, "-std=c++17", "-O2", "-o", generator, filepath.Join(shaderDir, "vulkan-shaders-gen.cpp")); err != nil {
		return nil, err
	}
	glslc := tool("glslc.exe")
	if err := requireFile(glslc); err != nil {
		return nil, err
	}
	ggmlModules := []struct {
		operation, source, output string
		workgroup                 kaijuvulkan.UInt3
	}{
		{"rmsnorm", "rms_norm.comp", "rms_norm_f32.spv", kaijuvulkan.UInt3{X: 512, Y: 1, Z: 1}},
		{"row_sum", "sum_rows.comp", "sum_rows_f32.spv", kaijuvulkan.UInt3{X: 32, Y: 1, Z: 1}},
		{"softmax", "soft_max.comp", "soft_max_f32.spv", kaijuvulkan.UInt3{X: 32, Y: 1, Z: 1}},
	}
	var result []module
	for _, candidate := range ggmlModules {
		start := time.Now()
		args := []string{"--glslc", glslc, "--source", filepath.Join(shaderDir, candidate.source), "--output-dir", out, "--target-cpp", filepath.Join(out, "ggml_"+strings.TrimSuffix(candidate.source, ".comp")+".cpp")}
		if err := command(root, generator, args...); err != nil {
			return nil, err
		}
		spv, err := os.ReadFile(filepath.Join(out, candidate.output))
		if err != nil {
			return nil, err
		}
		result = append(result, module{Family: "ggml_glsl", Operation: candidate.operation, Name: strings.TrimSuffix(candidate.output, ".spv"), Entry: "main", Source: filepath.ToSlash(filepath.Join("src", "ggml-vulkan", "vulkan-shaders", candidate.source)), SourceSHA256: fileHash(filepath.Join(shaderDir, candidate.source)), SPV: spv, CompileNS: time.Since(start).Nanoseconds(), Command: append([]string{generator}, args...), Workgroup: candidate.workgroup, Includes: includeClosure(shaderDir, candidate.source)})
	}
	sdsl := []struct {
		family, operation, source, name, entry string
		workgroup                              kaijuvulkan.UInt3
	}{
		{"sdslv", "rmsnorm", filepath.Join("tools", "llama_cpp_benchmark", "shaders", "rms_norm_f32_ggml_abi.sdslv"), "sdslv_rms_norm_f32", "GgmlRmsNormF32_CS", kaijuvulkan.UInt3{X: 512, Y: 1, Z: 1}},
		{"sdslv", "row_sum", filepath.Join("internal", "prometheus", "shaders", "sdslv", "production", "reduction", "row_sum_stage.sdslv"), "sdslv_row_sum", "RowSumStage_CS", kaijuvulkan.UInt3{X: 256, Y: 1, Z: 1}},
		{"sdslv", "softmax", filepath.Join("internal", "prometheus", "shaders", "sdslv", "production", "reduction", "softmax_fused.sdslv"), "sdslv_softmax", "SoftmaxFused_CS", kaijuvulkan.UInt3{X: 256, Y: 1, Z: 1}},
		{"sdslv_packed_short", "row_sum", filepath.Join("internal", "prometheus", "shaders", "sdslv", "production", "reduction", "row_sum_packed_short.sdslv"), "sdslv_row_sum_packed_short", "RowSumPackedShort_CS", kaijuvulkan.UInt3{X: 256, Y: 1, Z: 1}},
		{"sdslv_packed_short", "softmax", filepath.Join("internal", "prometheus", "shaders", "sdslv", "production", "reduction", "softmax_packed_short.sdslv"), "sdslv_softmax_packed_short", "SoftmaxPackedShort_CS", kaijuvulkan.UInt3{X: 256, Y: 1, Z: 1}},
	}
	for _, candidate := range sdsl {
		spvPath := filepath.Join(out, candidate.name+".spv")
		start := time.Now()
		args := []string{"run", "./cmd/oct", "sdslv", "compile-spv", candidate.source, "-o", spvPath, "--validate", "--require-spirv-val"}
		if err := command(root, "go", args...); err != nil {
			return nil, err
		}
		spv, err := os.ReadFile(spvPath)
		if err != nil {
			return nil, err
		}
		result = append(result, module{Family: candidate.family, Operation: candidate.operation, Name: candidate.name, Entry: candidate.entry, Source: filepath.ToSlash(candidate.source), SourceSHA256: fileHash(filepath.Join(root, candidate.source)), SPV: spv, CompileNS: time.Since(start).Nanoseconds(), Command: append([]string{"go"}, args...), Workgroup: candidate.workgroup})
	}
	return result, nil
}

func runMatrices(root string, modules []module, r *report) error {
	sidecar, err := resolveSidecar(root)
	if err != nil {
		return err
	}
	if _, err := callKaiju(sidecar, kaijuvulkan.OperationCapabilities, nil); err != nil {
		return err
	}
	previousValidation, hadValidation := os.LookupEnv("OCT_KAIJU_VULKAN_VALIDATION")
	defer func() {
		if hadValidation {
			_ = os.Setenv("OCT_KAIJU_VULKAN_VALIDATION", previousValidation)
		} else {
			_ = os.Unsetenv("OCT_KAIJU_VULKAN_VALIDATION")
		}
	}()
	if err := os.Setenv("OCT_KAIJU_VULKAN_VALIDATION", "1"); err != nil {
		return err
	}
	if err := runMatrix(sidecar, modules, r, "correctness", 1, 1); err != nil {
		return err
	}
	if err := os.Unsetenv("OCT_KAIJU_VULKAN_VALIDATION"); err != nil {
		return err
	}
	return runMatrix(sidecar, modules, r, "performance", warmups, iterations)
}

func runPackedProbe(root string, modules []module, r *report) error {
	sidecar, err := resolveSidecar(root)
	if err != nil {
		return err
	}
	if _, err := callKaiju(sidecar, kaijuvulkan.OperationCapabilities, nil); err != nil {
		return err
	}
	for _, mode := range []struct {
		name            string
		validation      bool
		warmup, samples uint32
	}{{"correctness", true, 1, 1}, {"performance", false, 10, 50}} {
		if mode.validation {
			_ = os.Setenv("OCT_KAIJU_VULKAN_VALIDATION", "1")
		} else {
			_ = os.Unsetenv("OCT_KAIJU_VULKAN_VALIDATION")
		}
		passes := 3
		if mode.validation {
			passes = 1
		}
		for pass := 0; pass < passes; pass++ {
			families := []string{"sdslv", "sdslv_packed_short", "ggml_glsl"}
			if pass%3 == 1 {
				families[0], families[2] = families[2], families[0]
			} else if pass%3 == 2 {
				families[0], families[1], families[2] = families[1], families[2], families[0]
			}
			for _, family := range families {
				for _, shape := range []struct{ width, rows uint32 }{
					{16, 1}, {16, 8}, {16, 32}, {16, 128}, {16, 512}, {16, 1024},
					{32, 1}, {32, 8}, {32, 32}, {32, 128}, {32, 512}, {32, 1024},
					{64, 1}, {64, 8}, {64, 32}, {64, 128}, {64, 512}, {64, 1024},
					{96, 1}, {96, 8}, {96, 32}, {96, 128}, {96, 512}, {96, 1024},
					{128, 1}, {128, 8}, {128, 32}, {128, 128}, {128, 512}, {128, 1024},
					{257, 1}, {257, 8}, {257, 32}, {257, 128}, {257, 512}, {257, 1024},
					{1024, 1}, {1024, 8}, {1024, 32}, {1024, 128}, {1024, 512}, {1024, 1024},
				} {
					if family == "sdslv_packed_short" && shape.width > 128 {
						continue
					}
					for _, operation := range []string{"row_sum", "softmax"} {
						entry, err := oneRun(sidecar, find(modules, family, operation), shape.width, shape.rows, 0, pass, true, mode.name, mode.warmup, mode.samples)
						if err != nil {
							return err
						}
						r.Runs = append(r.Runs, entry)
					}
				}
			}
		}
	}
	_ = os.Unsetenv("OCT_KAIJU_VULKAN_VALIDATION")
	return nil
}

func runMatrix(sidecar string, modules []module, r *report, mode string, warmup, samples uint32) error {
	for pass := 0; pass < 2; pass++ {
		families := []string{"sdslv", "ggml_glsl"}
		if pass%2 == 1 {
			families[0], families[1] = families[1], families[0]
		}
		for _, family := range families {
			for _, shape := range []struct{ w, rows uint32 }{{2048, 1}, {2048, 7}, {2048, 128}, {2048, 512}, {2053, 21}, {1025, 1}, {1025, 128}} {
				for _, epsilon := range []float32{1e-5, 1e-6, 1e-3} {
					entry, err := oneRun(sidecar, find(modules, family, "rmsnorm"), shape.w, shape.rows, epsilon, pass, true, mode, warmup, samples)
					if err != nil {
						return err
					}
					r.Runs = append(r.Runs, entry)
				}
			}
			for _, width := range []uint32{64, 257, 1024} {
				for _, rows := range []uint32{1, 8, 128, 512} {
					for _, operation := range []string{"row_sum", "softmax"} {
						entry, err := oneRun(sidecar, find(modules, family, operation), width, rows, 0, pass, true, mode, warmup, samples)
						if err != nil {
							return err
						}
						r.Runs = append(r.Runs, entry)
					}
				}
			}
		}
	}
	return nil
}

func oneRun(sidecar string, item module, width, rows uint32, epsilon float32, pass int, readback bool, mode string, warmup, samples uint32) (run, error) {
	input := deterministicInput(width, rows)
	request, oracle := requestFor(item, input, width, rows, epsilon, readback)
	request.BenchmarkID, request.ReplayID = fmt.Sprintf("%s-%s-%dx%d-p%d", item.Family, item.Operation, width, rows, pass), "direct-neutral-v1"
	request.Warmup, request.Iterations = warmup, samples
	start := time.Now()
	value, err := callKaiju(sidecar, kaijuvulkan.OperationBenchmark, &request)
	wall := time.Since(start).Nanoseconds()
	if err != nil {
		return run{}, err
	}
	response, err := kaijuvulkan.ParseDispatchResponseValue(value)
	if err != nil {
		return run{}, err
	}
	if !response.Success {
		return run{}, fmt.Errorf("%s: %#v", item.Name, response.Errors)
	}
	actual, err := outputReadback(response.Readbacks)
	if err != nil {
		return run{}, err
	}
	l2, linf, rel, finite := errorsFor(oracle, actual)
	if !finite || linf > tolerance(item.Operation) {
		return run{}, fmt.Errorf("%s %dx%d epsilon %g mismatch: linf=%g relative=%g", item.Name, width, rows, epsilon, linf, rel)
	}
	return run{Family: item.Family, Operation: item.Operation, Mode: mode, Width: width, Rows: rows, Epsilon: epsilon, Pass: pass, GPU: response.Timing.SamplesNS, GPUStats: summarizeTiming(response.Timing.SamplesNS), HostWallNS: wall, L2: l2, Linf: linf, Relative: rel, Finite: finite, InPlace: false, Validation: response.Validation}, nil
}

func summarizeTiming(samples []uint64) timingStats {
	if len(samples) == 0 {
		return timingStats{}
	}
	ordered := append([]uint64(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var sum float64
	for _, sample := range ordered {
		sum += float64(sample)
	}
	mean := sum / float64(len(ordered))
	if len(ordered) == 1 {
		return timingStats{MedianNS: ordered[0], MinimumNS: ordered[0], P95NS: ordered[0], MeanNS: mean}
	}
	var squared float64
	for _, sample := range ordered {
		delta := float64(sample) - mean
		squared += delta * delta
	}
	return timingStats{
		MedianNS:  ordered[(len(ordered)-1)/2],
		MinimumNS: ordered[0],
		P95NS:     ordered[int(math.Ceil(float64(len(ordered))*0.95))-1],
		MeanNS:    mean,
		StdDevNS:  math.Sqrt(squared / float64(len(ordered)-1)),
	}
}

func requestFor(item module, input []float32, width, rows uint32, epsilon float32, readback bool) (kaijuvulkan.BenchmarkRequest, []float32) {
	bytesIn := f32Bytes(input)
	output := make([]byte, len(bytesIn))
	resources := []kaijuvulkan.Resource{}
	push := []byte{}
	switch item.Operation {
	case "rmsnorm":
		push = rmsPush(width, rows, epsilon)
		resources = []kaijuvulkan.Resource{{Set: 0, Binding: 0, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(bytesIn)), Payload: bytesIn}, {Set: 0, Binding: 1, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(bytesIn)), Payload: bytesIn}, {Set: 0, Binding: 2, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(output)), Payload: output, Readback: readback}}
		if item.Family == "sdslv" {
			resources = append(resources, kaijuvulkan.Resource{Set: 0, Binding: 3, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: 4, Payload: make([]byte, 4)})
		}
	case "row_sum":
		push = sumPush(width, rows)
		if item.Family == "ggml_glsl" {
			resources = []kaijuvulkan.Resource{{Set: 0, Binding: 0, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(bytesIn)), Payload: bytesIn}, {Set: 0, Binding: 1, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: rows * 4, Payload: make([]byte, rows*4), Readback: readback}}
		} else {
			push = reductionPush(width, rows)
			resources = reductionResources(bytesIn, rows, readback)
		}
	case "softmax":
		push = softmaxPush(width, rows)
		if item.Family == "ggml_glsl" {
			resources = []kaijuvulkan.Resource{{Set: 0, Binding: 0, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(bytesIn)), Payload: bytesIn}, {Set: 0, Binding: 1, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: 4, Payload: make([]byte, 4)}, {Set: 0, Binding: 2, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: 4, Payload: make([]byte, 4)}, {Set: 0, Binding: 3, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(output)), Payload: output, Readback: readback}}
		} else {
			push = reductionPush(width, rows)
			resources = reductionResources(bytesIn, uint32(len(input)), readback)
		}
	}
	specializations := []kaijuvulkan.SpecializationConstant{}
	if item.Family == "ggml_glsl" && (item.Operation == "row_sum" || item.Operation == "softmax") {
		// GGML creates these ordinary paths with its device subgroup size (32 on
		// this RTX 3070), rather than the SPIR-V LocalSize placeholder of one.
		specializations = append(specializations, kaijuvulkan.SpecializationConstant{ID: 0, Value: 32})
	}
	dispatchRows := rows
	if item.Family == "sdslv_packed_short" {
		dispatchRows = (rows + 7) / 8
	}
	return kaijuvulkan.BenchmarkRequest{DispatchRequest: kaijuvulkan.DispatchRequest{Spirv: item.SPV, SpirvSHA256: hash(item.SPV), EntryPoint: item.Entry, WorkgroupSize: item.Workgroup, DispatchGroups: kaijuvulkan.UInt3{X: dispatchRows, Y: 1, Z: 1}, PushConstants: push, SpecializationConstants: specializations, Resources: resources}, Warmup: warmups, Iterations: iterations}, oracleFor(item.Operation, input, width, rows, epsilon)
}

func reductionResources(input []byte, outputCount uint32, readback bool) []kaijuvulkan.Resource {
	return []kaijuvulkan.Resource{{Set: 0, Binding: 0, Access: "readonly", Kind: "storage_buffer", ElementType: "f32", ByteLength: uint32(len(input)), Payload: input}, {Set: 0, Binding: 1, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: 4, Payload: make([]byte, 4)}, {Set: 0, Binding: 2, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: 4, Payload: make([]byte, 4)}, {Set: 0, Binding: 3, Access: "readwrite", Kind: "storage_buffer", ElementType: "f32", ByteLength: outputCount * 4, Payload: make([]byte, outputCount*4), Readback: readback}}
}

func rmsPush(w, rows uint32, epsilon float32) []byte {
	p := make([]byte, 116)
	words := []uint32{0, w, rows, 1, 1, 1, w, w * rows, w * rows, w, 1, 1, 1, 1, 1, 1, 1, w, rows, 1, 1, 1, w, w * rows, w * rows, 0}
	for i, v := range words {
		binary.LittleEndian.PutUint32(p[i*4:], v)
	}
	binary.LittleEndian.PutUint32(p[26*4:], math.Float32bits(epsilon))
	return p
}
func sumPush(w, rows uint32) []byte {
	p := make([]byte, 60)
	// Model rows as GGML's third logical dimension: i03 is the row, nb03 is
	// the contiguous FP32 row stride, and nb13 is one scalar output per row.
	// The two fast-divisor records encode division by one.
	values := []uint32{w, 1, 1, 0, 0, w, 0, 0, 1, math.Float32bits(1), 0, 0, 0, 0, 0}
	for i, v := range values {
		binary.LittleEndian.PutUint32(p[i*4:], v)
	}
	return p
}
func softmaxPush(w, rows uint32) []byte {
	p := make([]byte, 68)
	values := []uint32{w, 0, w, rows, 1, 0, 0, 0, 0, 0, math.Float32bits(1), 0, math.Float32bits(1), math.Float32bits(1), 1, rows, 0}
	for i, v := range values {
		binary.LittleEndian.PutUint32(p[i*4:], v)
	}
	return p
}
func reductionPush(w, rows uint32) []byte {
	p := make([]byte, 32)
	values := []uint32{rows, w, 1, w, 1024, w, 1, 0}
	for i, v := range values {
		binary.LittleEndian.PutUint32(p[i*4:], v)
	}
	return p
}

func oracleFor(operation string, input []float32, w, rows uint32, epsilon float32) []float32 {
	out := make([]float32, 0, len(input))
	for row := uint32(0); row < rows; row++ {
		values := input[row*w : (row+1)*w]
		switch operation {
		case "rmsnorm":
			var sum float64
			for _, x := range values {
				sum += float64(x) * float64(x)
			}
			scale := float32(1 / math.Sqrt(sum/float64(w)+float64(epsilon)))
			for _, x := range values {
				out = append(out, x*scale)
			}
		case "row_sum":
			var sum float64
			for _, x := range values {
				sum += float64(x)
			}
			out = append(out, float32(sum))
		case "softmax":
			max := values[0]
			for _, x := range values[1:] {
				if x > max {
					max = x
				}
			}
			var sum float64
			for _, x := range values {
				sum += math.Exp(float64(x - max))
			}
			for _, x := range values {
				out = append(out, float32(math.Exp(float64(x-max))/sum))
			}
		}
	}
	return out
}
func deterministicInput(w, rows uint32) []float32 {
	out := make([]float32, w*rows)
	for i := range out {
		out[i] = float32(int((uint32(i)*1664525+1013904223)%65521)-32760) / 4096
	}
	return out
}
func errorsFor(want, got []float32) (float64, float64, float64, bool) {
	if len(want) != len(got) {
		return 0, math.Inf(1), math.Inf(1), false
	}
	var sum, norm, linf float64
	finite := true
	for i, x := range got {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			finite = false
		}
		d := float64(x - want[i])
		sum += d * d
		norm += float64(want[i]) * float64(want[i])
		if math.Abs(d) > linf {
			linf = math.Abs(d)
		}
	}
	return math.Sqrt(sum), linf, math.Sqrt(sum) / math.Max(math.Sqrt(norm), 1e-30), finite
}
func tolerance(operation string) float64 {
	if operation == "row_sum" {
		return 2e-3
	}
	return 3e-4
}
func f32Bytes(v []float32) []byte {
	out := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(x))
	}
	return out
}
func outputReadback(v []kaijuvulkan.Readback) ([]float32, error) {
	if len(v) != 1 || len(v[0].Payload)%4 != 0 {
		return nil, fmt.Errorf("unexpected readback %#v", v)
	}
	out := make([]float32, len(v[0].Payload)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(v[0].Payload[i*4:]))
	}
	return out, nil
}
func find(values []module, family, operation string) module {
	for _, v := range values {
		if v.Family == family && v.Operation == operation {
			return v
		}
	}
	panic(family + "/" + operation + " missing")
}
func sameModules(a, b []module) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Family != b[i].Family || a[i].Operation != b[i].Operation || !bytes.Equal(a[i].SPV, b[i].SPV) {
			return false
		}
	}
	return true
}

func inspectModule(root string, item module) (moduleRecord, error) {
	path := filepath.Join(root, "out", "vulkan_shader_comparison", "generated", item.Name+".spv")
	if item.Family == "ggml_glsl" {
		path = filepath.Join(root, "out", "vulkan_shader_comparison", "generated", item.Name+".spv")
	}
	val := tool("spirv-val.exe")
	dis := tool("spirv-dis.exe")
	if err := command(root, val, "--target-env", map[bool]string{true: "vulkan1.2", false: "vulkan1.0"}[item.Family == "ggml_glsl"], path); err != nil {
		return moduleRecord{}, err
	}
	b, err := commandOutput(root, dis, path)
	if err != nil {
		return moduleRecord{}, err
	}
	lines := strings.Split(string(b), "\n")
	caps, exts, storage := []string{}, []string{}, []string{}
	barrier, workgroup, ops := 0, 0, 0
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		for _, token := range f {
			if strings.HasPrefix(token, "Op") {
				ops++
				break
			}
		}
		if strings.Contains(line, "OpCapability") {
			caps = append(caps, strings.TrimSpace(strings.TrimPrefix(line[strings.Index(line, "OpCapability"):], "OpCapability")))
		}
		if strings.Contains(line, "OpExtension") {
			exts = append(exts, strings.TrimSpace(strings.TrimPrefix(line[strings.Index(line, "OpExtension"):], "OpExtension")))
		}
		if strings.Contains(line, "Workgroup") {
			workgroup++
		}
		if strings.Contains(line, "OpControlBarrier") || strings.Contains(line, "OpMemoryBarrier") {
			barrier++
		}
		if strings.Contains(line, "OpVariable") && len(f) > 2 {
			storage = append(storage, f[len(f)-1])
		}
	}
	sort.Strings(caps)
	sort.Strings(exts)
	sort.Strings(storage)
	return moduleRecord{Family: item.Family, Operation: item.Operation, Name: item.Name, Entry: item.Entry, Source: item.Source, SourceSHA256: item.SourceSHA256, SHA256: hash(item.SPV), Bytes: len(item.SPV), Workgroup: item.Workgroup, CompileNS: item.CompileNS, Command: item.Command, Includes: item.Includes, SpirvValidation: "passed", OpcodeCount: ops, Capabilities: caps, Extensions: exts, StorageClasses: storage, WorkgroupVariables: workgroup, Barriers: barrier}, nil
}

func includeClosure(dir, source string) []string {
	seen := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return
		}
		for _, m := range regexp.MustCompile(`#include\s+"([^"]+)"`).FindAllStringSubmatch(string(b), -1) {
			visit(m[1])
		}
	}
	visit(source)
	result := make([]string, 0, len(seen))
	for x := range seen {
		result = append(result, filepath.ToSlash(x))
	}
	sort.Strings(result)
	return result
}
func archive(root, ggmlRoot string, modules []module) error {
	base := filepath.Join(root, "docs", "build-week", "artifacts", "ggml_sdslv_spirv_modules")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	for _, item := range modules {
		if err := os.WriteFile(filepath.Join(base, item.Family+"_"+item.Name+".spv"), item.SPV, 0o644); err != nil {
			return err
		}
	}
	sourceBase := filepath.Join(root, "docs", "build-week", "artifacts", "ggml_sdslv_spirv_sources")
	shaderDir := filepath.Join(ggmlRoot, "src", "ggml-vulkan", "vulkan-shaders")
	for _, item := range modules {
		if item.Family == "ggml_glsl" {
			for _, include := range item.Includes {
				if err := copyArtifact(filepath.Join(shaderDir, include), filepath.Join(sourceBase, "ggml", "vulkan-shaders", include)); err != nil {
					return err
				}
			}
			continue
		}
		if err := copyArtifact(filepath.Join(root, filepath.FromSlash(item.Source)), filepath.Join(sourceBase, "sdslv", filepath.FromSlash(item.Source))); err != nil {
			return err
		}
	}
	return nil
}
func copyArtifact(source, destination string) error {
	b, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, b, 0o644)
}
func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
func environment() map[string]string {
	out := map[string]string{"go": runtime.Version(), "os": runtime.GOOS + "/" + runtime.GOARCH, "glslc": version(tool("glslc.exe")), "dxc": version(tool("dxc.exe")), "spirv_tools": version(tool("spirv-val.exe"))}
	if b, err := commandOutput("", "nvidia-smi", "--query-gpu=name,driver_version,pstate,power.limit,temperature.gpu,clocks.sm,clocks.mem", "--format=csv,noheader"); err == nil {
		out["nvidia_smi"] = strings.TrimSpace(string(b))
	}
	return out
}
func tool(name string) string { return filepath.Join(`C:\VulkanSDK\1.4.350.0\Bin`, name) }
func version(path string) string {
	b, err := commandOutput("", path, "--version")
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(b))
}
func requireFile(path string) error { _, err := os.Stat(path); return err }
func hash(v []byte) string          { s := sha256.Sum256(v); return hex.EncodeToString(s[:]) }
func deterministicArtifactIdentity(r report) (string, error) {
	projection := r
	projection.GeneratedUTC = ""
	projection.Environment = nil
	projection.Runs = nil
	projection.DeterministicArtifactIdentity = false
	projection.Modules = append([]moduleRecord(nil), r.Modules...)
	for i := range projection.Modules {
		projection.Modules[i].CompileNS = 0
	}
	b, err := json.Marshal(projection)
	if err != nil {
		return "", err
	}
	return hash(b), nil
}
func fileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return hash(b)
}
func command(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}
func commandOutput(dir, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	return c.CombinedOutput()
}
func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveSidecar(root string) (string, error) {
	if fromEnv := os.Getenv("OCT_WRAPPER_PATH"); fromEnv != "" {
		candidate := filepath.Join(fromEnv, "octxiliary-kaiju-vulkan.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	candidate := filepath.Join(root, "dist", "sidecars", "octxiliary-kaiju-vulkan.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", errors.New("Kaiju sidecar missing; build Sidecars/KaijuVulkan into dist/sidecars")
}
func callKaiju(sidecar, function string, benchmark *kaijuvulkan.BenchmarkRequest) (internaloctx.Value, error) {
	cmd := exec.Command(sidecar)
	in, err := cmd.StdinPipe()
	if err != nil {
		return internaloctx.Value{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return internaloctx.Value{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		return internaloctx.Value{}, err
	}
	waitDone := make(chan error, 1)
	waited := false
	go func() { waitDone <- cmd.Wait() }()
	defer func() {
		if waited {
			return
		}
		select {
		case <-waitDone:
		case <-time.After(100 * time.Millisecond):
			_ = cmd.Process.Kill()
			<-waitDone
		}
	}()
	if err = internaloctx.WriteHandshake(in); err != nil {
		return internaloctx.Value{}, err
	}
	handshakeResult := make(chan error, 1)
	go func() { handshakeResult <- internaloctx.ReadHandshake(out) }()
	select {
	case err = <-handshakeResult:
		if err != nil {
			return internaloctx.Value{}, fmt.Errorf("Kaiju handshake: %w; stderr=%s", err, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
		return internaloctx.Value{}, fmt.Errorf("Kaiju handshake timed out: %s", strings.TrimSpace(stderr.String()))
	}
	args := []internaloctx.Value{}
	if benchmark != nil {
		args = append(args, kaijuvulkan.BenchmarkRequestValue(*benchmark))
	}
	if err = internaloctx.WriteFrame(in, internaloctx.EncodeRequest(internaloctx.Request{ID: 1, Family: kaijuvulkan.Family, Function: function, HasArgs: benchmark != nil, Args: args})); err != nil {
		return internaloctx.Value{}, err
	}
	_ = in.Close()
	frameResult := make(chan struct {
		frame string
		err   error
	}, 1)
	go func() {
		frame, readErr := internaloctx.ReadFrame(out)
		frameResult <- struct {
			frame string
			err   error
		}{frame, readErr}
	}()
	var frame string
	select {
	case result := <-frameResult:
		if result.err != nil {
			return internaloctx.Value{}, fmt.Errorf("Kaiju %s response: %w; stderr=%s", function, result.err, strings.TrimSpace(stderr.String()))
		}
		frame = result.frame
	case <-time.After(35 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
		return internaloctx.Value{}, fmt.Errorf("Kaiju response timed out: %s", strings.TrimSpace(stderr.String()))
	}
	response, err := internaloctx.ParseResponse(frame)
	if err != nil {
		return internaloctx.Value{}, err
	}
	select {
	case err = <-waitDone:
		waited = true
		if err == nil {
			break
		}
		return internaloctx.Value{}, fmt.Errorf("%w: %s", err, stderr.String())
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		return internaloctx.Value{}, errors.New("Kaiju sidecar did not terminate")
	}
	if !response.OK {
		return internaloctx.Value{}, errors.New(response.Error)
	}
	return response.Value, nil
}
