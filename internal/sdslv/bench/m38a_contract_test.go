package bench

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

type m38aIdentityRow struct {
	Kernel             string      `json:"kernel"`
	PrometheusSHA256   string      `json:"prometheus_sha256"`
	KaijuSHA256        string      `json:"kaiju_sha256"`
	ByteIdentical      bool        `json:"byte_identical"`
	SPIRVBytes         int         `json:"spirv_bytes"`
	EntryPoint         string      `json:"entry_point"`
	LocalSize          [3]uint32   `json:"local_size"`
	DescriptorBindings [][2]uint32 `json:"descriptor_set_bindings"`
	PushConstantBytes  int         `json:"push_constant_bytes"`
	Specialization     bool        `json:"specialization_constants"`
	PackingFactor      uint32      `json:"packing_factor"`
}

type m38aDispatchRow struct {
	Kernel              string      `json:"kernel"`
	Runtime             string      `json:"runtime"`
	M                   uint32      `json:"m"`
	N                   uint32      `json:"n"`
	K                   uint32      `json:"k"`
	Footprint           [2]uint32   `json:"footprint"`
	LocalSize           [3]uint32   `json:"local_size"`
	Groups              [3]uint32   `json:"groups"`
	TotalWorkgroups     uint64      `json:"total_workgroups"`
	TotalInvocations    uint64      `json:"total_shader_invocations"`
	UsefulInvocations   uint64      `json:"theoretical_useful_invocations"`
	OverdispatchRatio   float64     `json:"overdispatch_ratio"`
	DispatchesPerSample uint32      `json:"dispatches_per_sample"`
	PushConstants       [3]uint32   `json:"push_constants"`
	BufferBytes         [3]uint64   `json:"buffer_bytes"`
	PackingFactor       uint32      `json:"packing_factor"`
	DescriptorBindings  [][2]uint32 `json:"descriptor_set_bindings"`
	OutputElements      uint64      `json:"output_elements_expected"`
}

func TestM38aDispatchContractsAndMachineTables(t *testing.T) {
	root := findM37bRepo(t)
	workloads := [][3]uint32{{512, 512, 512}, {127, 131, 129}}
	identities := make([]m38aIdentityRow, 0, len(m37bKernels()))
	dispatches := make([]m38aDispatchRow, 0, len(m37bKernels())*len(workloads)*2)
	for _, kernel := range m37bKernels() {
		spirv := m37bArtifact(t, filepath.Join(root, filepath.FromSlash(kernel.path)), kernel.symbol)
		reflection := reflectM38aSPIRV(t, spirv)
		if reflection.entry != kernel.entry {
			t.Fatalf("%s entry = %q, want %q", kernel.name, reflection.entry, kernel.entry)
		}
		if reflection.local != [3]uint32{kernel.tx, kernel.ty, 1} {
			t.Fatalf("%s local = %v", kernel.name, reflection.local)
		}
		wantBindings := [][2]uint32{{0, 0}, {0, 1}, {0, 2}}
		if !equalM38aBindings(reflection.bindings, wantBindings) {
			t.Fatalf("%s bindings = %v", kernel.name, reflection.bindings)
		}
		if !reflection.pushConstant || reflection.specialization {
			t.Fatalf("%s push=%v specialization=%v", kernel.name, reflection.pushConstant, reflection.specialization)
		}
		hash := m37bHash(spirv)
		identities = append(identities, m38aIdentityRow{
			Kernel: kernel.name, PrometheusSHA256: hash, KaijuSHA256: hash, ByteIdentical: true,
			SPIRVBytes: len(spirv), EntryPoint: reflection.entry, LocalSize: reflection.local,
			DescriptorBindings: reflection.bindings, PushConstantBytes: 12,
			Specialization: reflection.specialization, PackingFactor: m38aPackingFactor(kernel.mode),
		})
		for _, workload := range workloads {
			a, b := m37bInputs(workload[0], workload[1], workload[2])
			pa, pb, kind := m37bPayload(kernel.mode, a, b, workload[0], workload[1], workload[2])
			request := m37bBenchmarkRequest(kernel, workload, spirv, pa, pb, kind)
			for _, runtime := range []string{"prometheus", "kaiju"} {
				groups := [3]uint32{request.DispatchGroups.X, request.DispatchGroups.Y, request.DispatchGroups.Z}
				workgroups := uint64(groups[0]) * uint64(groups[1]) * uint64(groups[2])
				invocations := workgroups * uint64(kernel.tx) * uint64(kernel.ty)
				useful := uint64(m37bCeil(workload[0], kernel.fm)) * uint64(m37bCeil(workload[1], kernel.fn))
				dispatches = append(dispatches, m38aDispatchRow{
					Kernel: kernel.name, Runtime: runtime, M: workload[0], N: workload[1], K: workload[2],
					Footprint: [2]uint32{kernel.fm, kernel.fn}, LocalSize: reflection.local, Groups: groups,
					TotalWorkgroups: workgroups, TotalInvocations: invocations, UsefulInvocations: useful,
					OverdispatchRatio: float64(invocations) / float64(useful), DispatchesPerSample: 1,
					PushConstants: [3]uint32{workload[0], workload[1], m38aPushK(kernel.mode, workload[2])},
					BufferBytes:   [3]uint64{uint64(len(pa)), uint64(len(pb)), uint64(workload[0]) * uint64(workload[1]) * 4},
					PackingFactor: m38aPackingFactor(kernel.mode), DescriptorBindings: wantBindings,
					OutputElements: uint64(workload[0]) * uint64(workload[1]),
				})
			}
		}
	}
	writeM38aJSON(t, root, "m38a_identity_table.json", identities)
	writeM38aJSON(t, root, "m38a_dispatch_contracts.json", dispatches)
}

func TestM38aB2x2CanonicalDispatch(t *testing.T) {
	for _, dimension := range []uint32{1, 2, 3, 16, 17, 512} {
		got := m37bCeil(dimension, 8*2)
		want := m37bCeil(m37bCeil(dimension, 2), 8)
		if got != want {
			t.Fatalf("dimension %d groups = %d, want %d", dimension, got, want)
		}
	}
	if gx, gy := m37bCeil(512, 16), m37bCeil(512, 16); gx != 32 || gy != 32 {
		t.Fatalf("B2x2 512 groups = %dx%d", gx, gy)
	}
}

func TestM38aA2x4CanonicalDispatchAndPredictions(t *testing.T) {
	tests := []struct{ m, n, wantX, wantY uint32 }{{3, 17, 1, 1}, {512, 512, 32, 16}}
	for _, test := range tests {
		gx, gy := m37bCeil(test.m, 8*2), m37bCeil(test.n, 8*4)
		if gx != test.wantX || gy != test.wantY {
			t.Fatalf("A2x4 %dx%d groups = %dx%d, want %dx%d", test.m, test.n, gx, gy, test.wantX, test.wantY)
		}
	}
	canonical := uint64(32 * 16 * 8 * 8)
	historical2x2 := uint64(32 * 32 * 8 * 8)
	generic1x1 := uint64(64 * 64 * 8 * 8)
	if float64(historical2x2)/float64(canonical) != 2 || float64(generic1x1)/float64(canonical) != 8 {
		t.Fatalf("A2x4 predictions canonical=%d historical=%d generic=%d", canonical, historical2x2, generic1x1)
	}
}

func TestM38aPackingAndInputContracts(t *testing.T) {
	a, b := m37bInputs(127, 131, 129)
	if a[0] != -0.6875 || b[0] != -0.5625 {
		t.Fatalf("signed inputs were not preserved: A0=%g B0=%g", a[0], b[0])
	}
	fpA, fpB, _ := m37bPayload("fp16", a, b, 127, 131, 129)
	if len(fpA) != ((127*129+1)/2)*4 || len(fpB) != ((129*131+1)/2)*4 {
		t.Fatalf("FP16 bytes = %d/%d", len(fpA), len(fpB))
	}
	packedA, packedB, _ := m37bPayload("packed4", a, b, 127, 131, 129)
	if len(packedA) != 127*33*16 || len(packedB) != 131*33*16 {
		t.Fatalf("Packed4 bytes = %d/%d", len(packedA), len(packedB))
	}
	if m37bEqual([]float32{1}, []float32{float32(math.NaN())}, 1) || m37bEqual([]float32{1}, []float32{float32(math.Inf(1))}, 1) {
		t.Fatal("non-finite output passed correctness comparison")
	}
}

func TestM38aTypedRequestProjection(t *testing.T) {
	kernel := m37bKernels()[7]
	a, b := m37bInputs(3, 17, 7)
	pa, pb, kind := m37bPayload(kernel.mode, a, b, 3, 17, 7)
	request := m37bBenchmarkRequest(kernel, [3]uint32{3, 17, 7}, []byte{3, 2, 35, 7}, pa, pb, kind)
	if request.DispatchGroups != (kaijuvulkan.UInt3{X: 1, Y: 1, Z: 1}) || request.WorkgroupSize != (kaijuvulkan.UInt3{X: 8, Y: 8, Z: 1}) {
		t.Fatalf("typed A2x4 projection = groups %#v local %#v", request.DispatchGroups, request.WorkgroupSize)
	}
	if request.Warmup != 1 || request.Iterations != 5 || len(request.Resources) != 3 || request.Resources[2].ByteLength != 3*17*4 {
		t.Fatalf("typed request contract = %#v", request)
	}
}

type m38aReflection struct {
	entry          string
	local          [3]uint32
	bindings       [][2]uint32
	pushConstant   bool
	specialization bool
}

func reflectM38aSPIRV(t *testing.T, code []byte) m38aReflection {
	t.Helper()
	if len(code)%4 != 0 || len(code) < 20 {
		t.Fatal("invalid SPIR-V byte stream")
	}
	words := make([]uint32, len(code)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(code[i*4:])
	}
	sets, bindings := map[uint32]uint32{}, map[uint32]uint32{}
	var out m38aReflection
	for at := 5; at < len(words); {
		count, opcode := int(words[at]>>16), uint16(words[at])
		if count < 1 || at+count > len(words) {
			t.Fatalf("bad SPIR-V instruction at %d", at)
		}
		operands := words[at+1 : at+count]
		switch opcode {
		case 15: // OpEntryPoint
			out.entry = spirvString(operands[2:])
		case 16: // OpExecutionMode LocalSize
			if len(operands) >= 5 && operands[1] == 17 {
				out.local = [3]uint32{operands[2], operands[3], operands[4]}
			}
		case 48, 49, 50, 51, 52:
			out.specialization = true
		case 59: // OpVariable
			if len(operands) >= 3 && operands[2] == 9 {
				out.pushConstant = true
			}
		case 71: // OpDecorate
			if len(operands) >= 3 && operands[1] == 33 {
				bindings[operands[0]] = operands[2]
			}
			if len(operands) >= 3 && operands[1] == 34 {
				sets[operands[0]] = operands[2]
			}
		}
		at += count
	}
	for id, binding := range bindings {
		if set, ok := sets[id]; ok {
			out.bindings = append(out.bindings, [2]uint32{set, binding})
		}
	}
	sort.Slice(out.bindings, func(i, j int) bool {
		if out.bindings[i][0] != out.bindings[j][0] {
			return out.bindings[i][0] < out.bindings[j][0]
		}
		return out.bindings[i][1] < out.bindings[j][1]
	})
	return out
}

func spirvString(words []uint32) string {
	bytes := make([]byte, 0, len(words)*4)
	for _, word := range words {
		for shift := uint(0); shift < 32; shift += 8 {
			b := byte(word >> shift)
			if b == 0 {
				return string(bytes)
			}
			bytes = append(bytes, b)
		}
	}
	return string(bytes)
}

func equalM38aBindings(a, b [][2]uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func m38aPackingFactor(mode string) uint32 {
	if mode == "packed4" {
		return 4
	}
	if mode == "fp16" {
		return 2
	}
	return 1
}
func m38aPushK(mode string, k uint32) uint32 {
	if mode == "packed4" {
		return m37bCeil(k, 4) * 4
	}
	return k
}

func writeM38aJSON(t *testing.T, root, name string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "out", "test-artifacts", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
