package zimage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, header string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.safetensors")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var length [8]byte
	binary.LittleEndian.PutUint64(length[:], uint64(len(header)))
	if _, err := f.Write(length[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(header); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadManifestDeterministic(t *testing.T) {
	path := fixture(t, `{"b":{"dtype":"F16","shape":[2],"data_offsets":[2,6]},"a":{"dtype":"F16","shape":[1],"data_offsets":[0,2]}}`, make([]byte, 6))
	a, err := ReadManifest(path, "redacted/model.safetensors")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadManifest(path, "redacted/model.safetensors")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tensors) != 2 || a.Tensors[0].Name != "a" || a.HeaderSHA256 != b.HeaderSHA256 {
		t.Fatalf("non-deterministic manifest: %#v", a)
	}
	if a.Tensors[0].FileRange != [2]uint64{uint64(8 + len(`{"b":{"dtype":"F16","shape":[2],"data_offsets":[2,6]},"a":{"dtype":"F16","shape":[1],"data_offsets":[0,2]}}`)), uint64(10 + len(`{"b":{"dtype":"F16","shape":[2],"data_offsets":[2,6]},"a":{"dtype":"F16","shape":[1],"data_offsets":[0,2]}}`))} {
		t.Fatal("absolute file range was not recorded")
	}
}

func TestReadManifestRejectsMalformedAndUnsafeHeaders(t *testing.T) {
	cases := []struct {
		name, header string
		data         []byte
		want         string
	}{
		{"duplicate", `{"x":{"dtype":"F16","shape":[1],"data_offsets":[0,2]},"x":{"dtype":"F16","shape":[1],"data_offsets":[2,4]}}`, make([]byte, 4), "duplicate"},
		{"overlap", `{"a":{"dtype":"F16","shape":[1],"data_offsets":[0,2]},"b":{"dtype":"F16","shape":[1],"data_offsets":[1,3]}}`, make([]byte, 3), "overlapping"},
		{"unsupported", `{"a":{"dtype":"X16","shape":[1],"data_offsets":[0,2]}}`, make([]byte, 2), "unsupported"},
		{"range", `{"a":{"dtype":"F16","shape":[1],"data_offsets":[0,4]}}`, make([]byte, 2), "extends"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadManifest(fixture(t, tc.header, tc.data), "x")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
		})
	}
}

func TestIdentityAndPathRedaction(t *testing.T) {
	path := fixture(t, `{"a":{"dtype":"F16","shape":[1],"data_offsets":[0,2]}}`, make([]byte, 2))
	hash, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, hash); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("expected identity mismatch, got %v", err)
	}
	if err := ValidateDisplayPath("local-model-cache/z_image.safetensors"); err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{`C:\\Users\\person\\model.safetensors`, "/home/person/model.safetensors", "../model.safetensors"} {
		if err := ValidateDisplayPath(unsafe); err == nil {
			t.Fatalf("accepted unsafe path %q", unsafe)
		}
	}
}
