package shaderpackage

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildStagesContentAddressedPackageAndDeduplicatesObjects(t *testing.T) {
	root := t.TempDir()
	native := filepath.Join(root, "internal", "prometheus", "native")
	if err := os.MkdirAll(native, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := "static const uint32_t k_one[] = { 0x07230203, 0x00010000, 0, 2, 0, 0x0005000f, 5, 1, 0x6e69616d, 0, 0x00060010, 1, 17, 1, 1, 1 };\nstatic const uint32_t k_two[] = { 0x07230203, 0x00010000, 0, 2, 0, 0x0005000f, 5, 1, 0x6e69616d, 0, 0x00060010, 1, 17, 1, 1, 1 };\n"
	if err := os.WriteFile(filepath.Join(native, "payload.h"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `{"shader_assets":[{"id":1,"name":"one","stage":"compute","source_language":"spirv","source":"historical","header":"payload.h","symbol":"k_one","entry_point":"main","workgroup":[1,1,1]},{"id":2,"name":"two","stage":"compute","source_language":"spirv","source":"historical","header":"payload.h","symbol":"k_two","entry_point":"main","workgroup":[1,1,1]}],"compute_implementations":[{"id":1,"name":"one-impl","authority":"production","shader_id":1,"dispatch":"one","benchmark_enabled":true,"selector_eligible":true,"dispatchable":true}]}`
	manifestPath := filepath.Join(root, "source.json")
	if err := os.WriteFile(manifestPath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "package")
	m, err := Build(BuildOptions{SourceManifest: manifestPath, RepositoryRoot: root, OutputRoot: out, IDHeader: filepath.Join(out, "ids.h")})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := len(m.Tables.Artifacts); got != 1 {
		t.Fatalf("objects = %d, want one deduplicated object", got)
	}
	if _, err := Check(out); err != nil {
		t.Fatalf("check built package: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "ids.h")); err != nil {
		t.Fatalf("generated ID header: %v", err)
	}
}

func TestCheckRejectsIntegrityAndRelationshipFailures(t *testing.T) {
	root := t.TempDir()
	payload := spirvFixture("main", [3]uint32{1, 1, 1})
	digest := sha(payload)
	writePackage := func(m Manifest) string {
		dir := t.TempDir()
		object := objectPath(dir, digest)
		if err := os.MkdirAll(filepath.Dir(object), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(object, payload, 0o644); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	valid := newManifest()
	valid.Tables.Artifacts = []Artifact{{Digest: digest, ByteCount: uint64(len(payload)), MediaType: "application/vnd.khronos.spirv"}}
	valid.Tables.Kernels = []Kernel{{ID: 1, Name: "kernel", Stage: "compute", Authority: "production"}}
	valid.Tables.Variants = []Variant{{ID: "kernel-1-default", KernelID: 1, ArtifactSHA256: digest, EntryPoint: "main", Workgroup: []uint32{1, 1, 1}}}
	valid.Tables.Implementations = []Implementation{{ID: 1, Name: "impl", Authority: "production", VariantID: "kernel-1-default"}}
	if _, err := Check(writePackage(valid)); err != nil {
		t.Fatalf("valid package: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"schema", func(m *Manifest) { m.Schema = "wrong" }, "unsupported shader package schema"},
		{"abi", func(m *Manifest) { m.Package.RuntimeABI = 2 }, "incompatible shader package runtime ABI"},
		{"digest", func(m *Manifest) { m.Tables.Artifacts[0].Digest = strings.Repeat("z", 64) }, "invalid artifact"},
		{"duplicate-kernel", func(m *Manifest) { m.Tables.Kernels = append(m.Tables.Kernels, m.Tables.Kernels[0]) }, "duplicate kernel"},
		{"missing-variant-target", func(m *Manifest) { m.Tables.Implementations[0].VariantID = "missing" }, "invalid implementation"},
		{"unknown-requirement", func(m *Manifest) {
			m.Tables.Requirements = []Requirement{{VariantID: "kernel-1-default", Kind: "run_script", Name: "no"}}
		}, "invalid requirement"},
		{"experimental-leak", func(m *Manifest) { m.Tables.Kernels[0].Authority = "experimental" }, "experimental variant leaked"},
		{"missing-entry", func(m *Manifest) { m.Tables.Variants[0].EntryPoint = "missing" }, "entry point \"missing\" is missing"},
		{"local-size", func(m *Manifest) { m.Tables.Variants[0].Workgroup = []uint32{2, 1, 1} }, "LocalSize mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := valid
			copy.Tables.Artifacts = append([]Artifact(nil), valid.Tables.Artifacts...)
			copy.Tables.Kernels = append([]Kernel(nil), valid.Tables.Kernels...)
			copy.Tables.Variants = append([]Variant(nil), valid.Tables.Variants...)
			copy.Tables.Implementations = append([]Implementation(nil), valid.Tables.Implementations...)
			tc.mutate(&copy)
			_, err := Check(writePackage(copy))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	_ = root
}

func spirvFixture(entry string, local [3]uint32) []byte {
	if entry != "main" {
		panic("fixture only encodes main")
	}
	words := []uint32{
		0x07230203, 0x00010000, 0, 2, 0,
		(5 << 16) | 15, 5, 1, 0x6e69616d, 0,
		(6 << 16) | 16, 1, 17, local[0], local[1], local[2],
	}
	out := make([]byte, len(words)*4)
	for i, word := range words {
		binary.LittleEndian.PutUint32(out[i*4:], word)
	}
	return out
}
