package zimage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNoiseRefiner0PayloadBundleWhenLocalPayloadsAreAvailable(t *testing.T) {
	cacheRoot := os.Getenv("OCT_EVT2_CACHE")
	oracleRoot := os.Getenv("OCT_EVT2_ORACLE")
	if cacheRoot == "" || oracleRoot == "" {
		t.Skip("local EVT-2 payload roots are not configured")
	}
	bundle, err := LoadNoiseRefiner0PayloadBundle(NoiseRefiner0PayloadPaths{
		CacheRoot:  cacheRoot,
		OracleRoot: oracleRoot,
	})
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
