package zimage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

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
