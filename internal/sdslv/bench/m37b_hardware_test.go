package bench

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

// TestM37bKaijuProductionRows is the bounded Kaiju half of the M37b adapter.
// It consumes exact native registry words; it is intentionally hardware-gated.
func TestM37bKaijuProductionRows(t *testing.T) {
	requireKaijuHardware(t)
	sidecar, err := resolveKaijuSidecar()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := invokeKaijuCapabilities(sidecar); err != nil {
		t.Fatal(err)
	}
	root := findM37bRepo(t)
	kernels := m37bKernels()
	workloads := [][3]uint32{{512, 512, 512}, {127, 131, 129}}
	rows := make([]m37bRow, 0, len(kernels)*len(workloads))
	for _, k := range kernels {
		for _, w := range workloads {
			spirv := m37bArtifact(t, filepath.Join(root, filepath.FromSlash(k.path)), k.symbol)
			a, b := m37bInputs(w[0], w[1], w[2])
			expected := m37bReference(a, b, w[0], w[1], w[2])
			pa, pb, kind := m37bPayload(k.mode, a, b, w[0], w[1], w[2])
			request := m37bBenchmarkRequest(k, w, spirv, pa, pb, kind)
			response, err := invokeKaijuBenchmark(sidecar, request)
			if err != nil || !response.Success {
				t.Fatalf("%s %v: %v %#v", k.name, w, err, response.Errors)
			}
			if !response.Validation.Requested || !response.Validation.Available || !response.Validation.Enabled ||
				response.Validation.Warnings != 0 || response.Validation.Errors != 0 || response.Validation.DeviceLost {
				t.Fatalf("%s validation %#v", k.name, response.Validation)
			}
			wantCommands := []string{
				kaijuvulkan.TimingCommandBindPipeline,
				kaijuvulkan.TimingCommandBindDescriptorSets,
				kaijuvulkan.TimingCommandPushConstants,
				kaijuvulkan.TimingCommandDispatch,
			}
			if response.SpirvSHA256 != request.SpirvSHA256 ||
				response.Timing.TimestampStartStage != kaijuvulkan.TimingStageTopOfPipe ||
				response.Timing.TimestampEndStage != kaijuvulkan.TimingStageBottomOfPipe ||
				response.Timing.DispatchesPerSample != 1 ||
				len(response.Timing.IntervalCommands) != len(wantCommands) ||
				response.Timing.QueryResetLocation != "command_buffer_before_start_timestamp" ||
				response.Timing.FenceWaitLocation != "host_after_queue_submit" ||
				response.Timing.ResultRetrievalLocation != "host_after_fence_wait" {
				t.Fatalf("%s identity/timing trace mismatch: %#v", k.name, response)
			}
			for i := range wantCommands {
				if response.Timing.IntervalCommands[i] != wantCommands[i] {
					t.Fatalf("%s interval commands = %v", k.name, response.Timing.IntervalCommands)
				}
			}
			actual := m37bReadback(t, response.Readbacks, w[0]*w[1])
			correct := m37bEqual(expected, actual, k.tolerance)
			if !correct {
				t.Fatalf("%s %v output mismatch", k.name, w)
			}
			stats := StatisticsFor(response.Timing.SamplesNS)
			rows = append(rows, m37bRow{
				Kernel: k.name, M: w[0], N: w[1], K: w[2], Correct: correct,
				SPIRVSHA256: request.SpirvSHA256, SPIRVBytes: len(request.Spirv), EntryPoint: request.EntryPoint,
				LocalSize: request.WorkgroupSize, Footprint: kaijuvulkan.UInt3{X: k.fm, Y: k.fn, Z: 1},
				Groups: request.DispatchGroups, PushConstantsHex: hex.EncodeToString(request.PushConstants),
				BufferBytes:        []uint32{request.Resources[0].ByteLength, request.Resources[1].ByteLength, request.Resources[2].ByteLength},
				DescriptorBindings: []uint32{0, 1, 2}, OutputElements: w[0] * w[1], OutputInitiallyZero: true,
				DispatchesPerSample: response.Timing.DispatchesPerSample, Device: response.Device, Timing: response.Timing, Validation: response.Validation,
				Min: stats.Min, Median: stats.Median, Max: stats.Max,
			})
		}
	}
	data, err := json.MarshalIndent(struct {
		Backend    string    `json:"backend"`
		Warmup     int       `json:"warmup"`
		Iterations int       `json:"iterations"`
		Rows       []m37bRow `json:"rows"`
	}{"kaiju", 1, 5, rows}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "out", "test-artifacts", "m37b_kaiju_rows.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func m37bKernels() []m37bKernel {
	return []m37bKernel{
		{name: "scalar", path: "internal/prometheus/native/shaders/historical/sgemm_baseline_scalar.spv.base64", symbol: "", entry: "main", tx: 8, ty: 8, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "tiled", path: "internal/prometheus/native/reactor_vulkan_tiled_spirv.h", symbol: "k_prom_sgemm_tiled_spirv", entry: "main", tx: 8, ty: 8, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "memory-conservative", path: "internal/prometheus/native/reactor_vulkan_memory_conservative_spirv.h", symbol: "k_prom_sgemm_memory_conservative_spirv", entry: "main", tx: 8, ty: 8, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "scalar-plus", path: "internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h", symbol: "k_prom_sgemm_scalar_plus_spirv", entry: "SgemmScalarBaselinePlus8x8_CS", tx: 8, ty: 8, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "tile16", path: "internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h", symbol: "k_prom_sgemm_tile16x16_shared_fp32_spirv", entry: "SgemmTile16x16SharedFp32_CS", tx: 16, ty: 16, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "SRT", path: "internal/prometheus/native/reactor_vulkan_sgemm_srt_2accum_k_spirv.h", symbol: "k_prom_sgemm_srt_2accum_k_spirv", entry: "SgemmSrt2AccumK_CS", tx: 8, ty: 8, fm: 1, fn: 1, mode: "f32", tolerance: .002},
		{name: "B2x2", path: "internal/prometheus/native/reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h", symbol: "k_prom_sgemm_b2x2_row_major_biased_spirv", entry: "SgemmB2x2_CS", tx: 8, ty: 8, fm: 2, fn: 2, mode: "f32", tolerance: .002},
		{name: "A2x4", path: "internal/prometheus/native/reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h", symbol: "k_prom_sgemm_a2x4_row_biased_accum8_spirv", entry: "SgemmA2x4_CS", tx: 8, ty: 8, fm: 2, fn: 4, mode: "f32", tolerance: .002},
		{name: "Packed4", path: "internal/prometheus/native/reactor_vulkan_packed4_spirv.h", symbol: "k_prom_sgemm_packed4_spirv", entry: "SgemmPacked4_CS", tx: 8, ty: 8, fm: 1, fn: 1, mode: "packed4", tolerance: .002},
		{name: "FP16", path: "internal/prometheus/native/reactor_vulkan_fp16_spirv.h", symbol: "k_prom_sgemm_fp16_storage_fp32accum_spirv", entry: "SgemmFp16StorageFp32Accum_CS", tx: 8, ty: 8, fm: 1, fn: 1, mode: "fp16", tolerance: .03},
	}
}

func m37bBenchmarkRequest(kernel m37bKernel, workload [3]uint32, spirv, a, b []byte, elementType string) kaijuvulkan.BenchmarkRequest {
	return kaijuvulkan.BenchmarkRequest{
		DispatchRequest: kaijuvulkan.DispatchRequest{
			BenchmarkID:    "m37b-" + kernel.name,
			ReplayID:       "m37b-" + kernel.name,
			Spirv:          spirv,
			SpirvSHA256:    m37bHash(spirv),
			EntryPoint:     kernel.entry,
			WorkgroupSize:  kaijuvulkan.UInt3{X: kernel.tx, Y: kernel.ty, Z: 1},
			DispatchGroups: kaijuvulkan.UInt3{X: m37bCeil(workload[0], kernel.tx*kernel.fm), Y: m37bCeil(workload[1], kernel.ty*kernel.fn), Z: 1},
			PushConstants:  m37bPush(workload),
			Resources: []kaijuvulkan.Resource{
				{Set: 0, Binding: 0, Access: "readonly", Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: elementType, ByteLength: uint32(len(a)), Payload: a},
				{Set: 0, Binding: 1, Access: "readonly", Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: elementType, ByteLength: uint32(len(b)), Payload: b},
				{Set: 0, Binding: 2, Access: "readwrite", Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: "f32", ByteLength: workload[0] * workload[1] * 4, Payload: make([]byte, workload[0]*workload[1]*4), Readback: true},
			},
		},
		Warmup:     1,
		Iterations: 5,
	}
}

type m37bKernel struct {
	name, path, symbol, entry string
	tx, ty, fm, fn            uint32
	mode                      string
	tolerance                 float32
}
type m37bRow struct {
	Kernel              string                       `json:"kernel"`
	M                   uint32                       `json:"m"`
	N                   uint32                       `json:"n"`
	K                   uint32                       `json:"k"`
	Correct             bool                         `json:"correct"`
	SPIRVSHA256         string                       `json:"spirv_sha256"`
	SPIRVBytes          int                          `json:"spirv_bytes"`
	EntryPoint          string                       `json:"entry_point"`
	LocalSize           kaijuvulkan.UInt3            `json:"local_size"`
	Footprint           kaijuvulkan.UInt3            `json:"footprint"`
	Groups              kaijuvulkan.UInt3            `json:"groups"`
	PushConstantsHex    string                       `json:"push_constants_hex"`
	BufferBytes         []uint32                     `json:"buffer_bytes"`
	DescriptorBindings  []uint32                     `json:"descriptor_bindings"`
	OutputElements      uint32                       `json:"output_elements"`
	OutputInitiallyZero bool                         `json:"output_initially_zero"`
	DispatchesPerSample uint32                       `json:"dispatches_per_sample"`
	Device              kaijuvulkan.DeviceInfo       `json:"device"`
	Timing              kaijuvulkan.Timing           `json:"timing"`
	Validation          kaijuvulkan.ValidationStatus `json:"validation"`
	Min                 uint64                       `json:"min"`
	Median              uint64                       `json:"median"`
	Max                 uint64                       `json:"max"`
}

func findM37bRepo(t *testing.T) string {
	t.Helper()
	d, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, e := os.Stat(filepath.Join(d, "go.mod")); e == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	t.Fatal("repo not found")
	return ""
}
func m37bArtifact(t *testing.T, path, symbol string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if filepath.Ext(path) == ".base64" {
		out, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(b)))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	m := regexp.MustCompile(`(?s)(?:static\s+)?const uint32_t\s+` + regexp.QuoteMeta(symbol) + `\[\]\s*=\s*\{(.*?)\};`).FindSubmatch(b)
	if len(m) != 2 {
		t.Fatalf("%s missing %s", path, symbol)
	}
	words := regexp.MustCompile(`0x([0-9a-fA-F]+)u?`).FindAllSubmatch(m[1], -1)
	out := make([]byte, len(words)*4)
	for i, w := range words {
		v, _ := strconv.ParseUint(string(w[1]), 16, 32)
		binary.LittleEndian.PutUint32(out[i*4:], uint32(v))
	}
	return out
}
func m37bInputs(m, n, k uint32) ([]float32, []float32) {
	a := make([]float32, m*k)
	b := make([]float32, k*n)
	for i := range a {
		a[i] = float32(int((uint32(i)*17)%23)-11) * .0625
	}
	for i := range b {
		b[i] = float32(int((uint32(i)*11)%19)-9) * .0625
	}
	return a, b
}
func m37bReference(a, b []float32, m, n, k uint32) []float32 {
	o := make([]float32, m*n)
	for r := uint32(0); r < m; r++ {
		for c := uint32(0); c < n; c++ {
			for x := uint32(0); x < k; x++ {
				o[r*n+c] += a[r*k+x] * b[x*n+c]
			}
		}
	}
	return o
}
func m37bPayload(mode string, a, b []float32, m, n, k uint32) ([]byte, []byte, string) {
	if mode == "f32" {
		return m37bF32(a), m37bF32(b), "f32"
	}
	if mode == "fp16" {
		return m37bF16(a), m37bF16(b), "u32"
	}
	lanes := uint32(4)
	packs := m37bCeil(k, lanes)
	av := make([]float32, m*packs*lanes)
	bv := make([]float32, n*packs*lanes)
	for r := uint32(0); r < m; r++ {
		for x := uint32(0); x < k; x++ {
			av[(r*packs+x/lanes)*lanes+x%lanes] = a[r*k+x]
		}
	}
	for c := uint32(0); c < n; c++ {
		for x := uint32(0); x < k; x++ {
			bv[(c*packs+x/lanes)*lanes+x%lanes] = b[x*n+c]
		}
	}
	return m37bF32(av), m37bF32(bv), "float4"
}
func m37bF32(v []float32) []byte {
	o := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(o[i*4:], math.Float32bits(x))
	}
	return o
}
func m37bF16(v []float32) []byte {
	o := make([]byte, ((len(v)+1)/2)*4)
	for i, x := range v {
		shift := uint(0)
		if i%2 == 1 {
			shift = 16
		}
		word := binary.LittleEndian.Uint32(o[i/2*4:])
		word |= uint32(m37bHalf(x)) << shift
		binary.LittleEndian.PutUint32(o[i/2*4:], word)
	}
	return o
}
func m37bHalf(value float32) uint16 {
	bits := math.Float32bits(value)
	sign := (bits >> 16) & 0x8000
	exp := int((bits>>23)&0xff) - 127 + 15
	mant := bits & 0x7fffff
	if exp <= 0 {
		return uint16(sign)
	}
	if exp >= 31 {
		return uint16(sign | 0x7c00)
	}
	return uint16(sign | uint32(exp<<10) | (mant >> 13))
}
func m37bPush(w [3]uint32) []byte {
	o := make([]byte, 12)
	for i, v := range w {
		binary.LittleEndian.PutUint32(o[i*4:], v)
	}
	return o
}
func m37bCeil(a, b uint32) uint32 { return (a + b - 1) / b }
func m37bHash(v []byte) string    { sum := sha256.Sum256(v); return hex.EncodeToString(sum[:]) }
func m37bReadback(t *testing.T, v []kaijuvulkan.Readback, count uint32) []float32 {
	t.Helper()
	if len(v) != 1 {
		t.Fatalf("readbacks %#v", v)
	}
	o := make([]float32, count)
	for i := range o {
		o[i] = math.Float32frombits(binary.LittleEndian.Uint32(v[0].Payload[i*4:]))
	}
	return o
}
func m37bEqual(a, b []float32, tol float32) bool {
	for i := range a {
		if math.IsNaN(float64(a[i])) || math.IsNaN(float64(b[i])) || math.IsInf(float64(a[i]), 0) || math.IsInf(float64(b[i]), 0) {
			return false
		}
		if math.Abs(float64(a[i]-b[i])) > float64(tol) {
			return false
		}
	}
	return true
}
