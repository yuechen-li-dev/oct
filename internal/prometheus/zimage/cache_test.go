package zimage

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNoiseRefiner0PayloadBundleWhenLocalPayloadsAreAvailable(t *testing.T) {
	paths, err := NoiseRefiner0PayloadPathsFromEnvironment()
	if err != nil {
		t.Skip(err)
	}
	bundle, err := LoadNoiseRefiner0PayloadBundle(paths)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.CacheBytes != 361820672 || bundle.CacheManifest.AggregateSHA256 != NoiseRefiner0CacheAggregateSHA256 {
		t.Fatalf("unexpected cache identity: bytes=%d aggregate=%s", bundle.CacheBytes, bundle.CacheManifest.AggregateSHA256)
	}
	if bundle.Input.SHA256 != "857cea75e69d665c43779c9bc860796e76ac8b78c5c70882e02a04940e78fded" ||
		bundle.Timestep.SHA256 != "bc0ba90e94f5ae98779c6f7c44e7d1346f8aa6aa1cc048f62a748d96076823b2" ||
		bundle.FP16Reference.SHA256 != NoiseRefiner0FP16ReferenceSHA256 {
		t.Fatal("unexpected local oracle identity")
	}
	if len(bundle.StageNames) < 20 {
		t.Fatalf("stage witness count = %d, want at least 20", len(bundle.StageNames))
	}
	contract, err := NewNoiseRefiner0ModuleContract(bundle, NoiseRefiner0ResidentProofShaderSHA256, "rtx-3070-vulkan", []NoiseRefiner0ShaderIdentity{{
		ID:         NoiseRefiner0ResidentProofShaderID,
		SHA256:     NoiseRefiner0ResidentProofShaderSHA256,
		PipelineID: NoiseRefiner0ResidentProofPipelineID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewNoiseRefiner0ResidentBlockPlan(bundle, contract)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Weights) != 13 || plan.Memory.PersistentWeightBytes != 361820672 || plan.Memory.TotalCommittedBytes > plan.Memory.MemoryCeilingBytes {
		t.Fatalf("unexpected M1a resident plan: weights=%d memory=%+v", len(plan.Weights), plan.Memory)
	}
	if len(plan.ExecutionSteps) != 7 || plan.ExecutionPlanID == "" || plan.ResidentReplaySeedID == "" {
		t.Fatalf("resident plan identity or fixed program missing: %+v", plan)
	}
}

func TestBF16ToFP16BoundarySemantics(t *testing.T) {
	tests := []struct {
		name string
		bf16 uint16
		want uint16
	}{
		{"positive zero", 0x0000, 0x0000},
		{"negative zero", 0x8000, 0x8000},
		{"one", 0x3f80, 0x3c00},
		{"negative one", 0xbf80, 0xbc00},
		// 0x477f is the largest BF16 value below FP16 overflow; BF16's
		// coarser mantissa makes its exact FP16 representation 0x7bf8.
		{"largest bf16 value below fp16 overflow", 0x477f, 0x7bf8},
		{"positive infinity", 0x7f80, 0x7c00},
		{"negative infinity", 0xff80, 0xfc00},
		{"positive nan canonicalized", 0x7fc1, 0x7e00},
		{"negative nan canonicalized", 0xffc1, 0xfe00},
		{"smallest bf16 normal underflows", 0x0080, 0x0000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BF16ToFP16(test.bf16); got != test.want {
				t.Fatalf("BF16ToFP16(%#04x) = %#04x, want %#04x", test.bf16, got, test.want)
			}
		})
	}
}

func TestFP16ToFloat32NormalAndSubnormalSemantics(t *testing.T) {
	tests := []struct {
		name string
		fp16 uint16
		want float32
	}{
		{"positive one", 0x3c00, 1},
		{"negative one", 0xbc00, -1},
		{"normal half", 0x3800, 0.5},
		{"smallest normal", 0x0400, 1.0 / 16384.0},
		{"smallest subnormal", 0x0001, 1.0 / 16777216.0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FP16ToFloat32(test.fp16); got != test.want {
				t.Fatalf("FP16ToFloat32(%#04x) = %g, want %g", test.fp16, got, test.want)
			}
		})
	}
}

func referenceFP16Finite(bits uint16) float32 {
	sign := 1.0
	if bits&0x8000 != 0 {
		sign = -1.0
	}
	exponent := int((bits >> 10) & 0x1f)
	fraction := int(bits & 0x03ff)
	if exponent == 0 {
		if fraction == 0 {
			return math.Float32frombits(uint32(bits&0x8000) << 16)
		}
		return float32(sign * math.Ldexp(float64(fraction), -24))
	}
	return float32(sign * math.Ldexp(float64(1024+fraction), exponent-25))
}

func TestFP16ToFloat32ExhaustiveFiniteReference(t *testing.T) {
	for raw := 0; raw <= 0xffff; raw++ {
		bits := uint16(raw)
		if bits&0x7c00 == 0x7c00 { // infinities and NaNs are covered separately.
			continue
		}
		got, want := FP16ToFloat32(bits), referenceFP16Finite(bits)
		if math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("FP16ToFloat32(%#04x) = %#08x, want %#08x", bits, math.Float32bits(got), math.Float32bits(want))
		}
	}
}

func TestFP16CacheDecodedValuesRemainFiniteAndUnscaledWhenLocalPayloadsAreAvailable(t *testing.T) {
	paths, err := NoiseRefiner0PayloadPathsFromEnvironment()
	if err != nil {
		t.Skip(err)
	}
	bundle, err := LoadNoiseRefiner0PayloadBundle(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, tensor := range bundle.CacheManifest.Tensors {
		values, err := canonicalRead16(filepath.Join(bundle.CacheBlockPath, tensor.DestinationName), FP16ToFloat32)
		if err != nil {
			t.Fatalf("read %s: %v", tensor.DestinationName, err)
		}
		for index, value := range values {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				t.Fatalf("%s[%d] is non-finite after FP16 decode: %g", tensor.DestinationName, index, value)
			}
			// O1's source census establishes that every cached weight has |x| < 2.61.
			// Four permits a small invariant margin while permanently detecting the
			// historical normal-exponent bug, which inflated ordinary values by 32768.
			if math.Abs(float64(value)) > 4 {
				t.Fatalf("%s[%d] = %g exceeds the O1 cache range bound; possible scaled FP16 decode", tensor.DestinationName, index, value)
			}
		}
	}
}

func TestConvertTensorTransposesDeterministically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.bin")
	input := make([]byte, 8)
	for i, value := range []uint16{0x3f80, 0x4000, 0x4040, 0x4080} {
		binary.LittleEndian.PutUint16(input[i*2:], value)
	}
	if err := os.WriteFile(path, input, 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	tensor := Tensor{Name: "test", DType: "BF16", Shape: []uint64{2, 2}, Elements: 4, Bytes: 8, FileRange: [2]uint64{0, 8}}
	first, err := convertTensor(source, tensor, cacheSpec{name: "test", transpose: true, shape: []uint64{2, 2}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := convertTensor(source, tensor, cacheSpec{name: "test", transpose: true, shape: []uint64{2, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("conversion changed between identical reads")
	}
	want := []uint16{0x3c00, 0x4200, 0x4000, 0x4400}
	for i, value := range want {
		if got := binary.LittleEndian.Uint16(first[i*2:]); got != value {
			t.Fatalf("output[%d] = %#04x, want %#04x", i, got, value)
		}
	}
}

func TestWriteAtomicCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := writeAtomic(path, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file survived atomic write: %v", err)
	}
}
