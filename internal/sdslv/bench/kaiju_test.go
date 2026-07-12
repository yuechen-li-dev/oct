package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func requireKaijuHardware(t *testing.T) {
	t.Helper()
	if os.Getenv("OCT_KAIJU_VULKAN_TESTS") != "1" {
		t.Skip("set OCT_KAIJU_VULKAN_TESTS=1 to run Kaiju Vulkan hardware tests")
	}
}

func TestKaijuCapabilitiesHardware(t *testing.T) {
	requireKaijuHardware(t)
	sidecar, err := resolveKaijuSidecar()
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := invokeKaijuCapabilities(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if !capabilities.DispatchSupported || !capabilities.BenchmarkSupported {
		t.Fatalf("unexpected capabilities: %#v", capabilities)
	}
}

func TestKaijuBenchCanonicalNDArrayHardware(t *testing.T) {
	requireKaijuHardware(t)
	path := findBenchSource(t)
	report, err := runKaiju(path, Manifest{
		SchemaVersion: 1,
		Source: "examples/SDSL-V/M36a/BasicBenchmarks.sdslvbench",
		Benchmarks: []Case{{
			ID: "sdslvbench-8b1f66233dd54390f518e9c7",
			Name: "NDArrayMaterializeStorage",
			EntryPoint: "NDArrayMaterializeStorage",
			WorkgroupSize: [3]uint32{1, 1, 1},
			DispatchGroups: [3]uint32{4, 1, 1},
			Warmup: 2,
			Iterations: 8,
			ReplayID: "sdslvbench-replay-c15d871f78df565436b4d384",
			Resources: []Resource{
				{Set: 0, Binding: 0, Access: "readonly", ElementType: "f32", ByteLength: 16, PayloadBase64: "AAAAAAAAAAAAAAAAAAAAAA=="},
				{Set: 0, Binding: 1, Access: "readwrite", ElementType: "f32", ByteLength: 16, PayloadBase64: "AAAAAAAAAAAAAAAAAAAAAA==", Readback: true},
			},
			Shader: "CanonicalNDArrayBench",
		}},
	}, []Case{{
		ID: "sdslvbench-8b1f66233dd54390f518e9c7",
		Name: "NDArrayMaterializeStorage",
		EntryPoint: "NDArrayMaterializeStorage",
		WorkgroupSize: [3]uint32{1, 1, 1},
		DispatchGroups: [3]uint32{4, 1, 1},
		Warmup: 2,
		Iterations: 8,
		ReplayID: "sdslvbench-replay-c15d871f78df565436b4d384",
		Resources: []Resource{
			{Set: 0, Binding: 0, Access: "readonly", ElementType: "f32", ByteLength: 16, PayloadBase64: "AAAAAAAAAAAAAAAAAAAAAA=="},
			{Set: 0, Binding: 1, Access: "readwrite", ElementType: "f32", ByteLength: 16, PayloadBase64: "AAAAAAAAAAAAAAAAAAAAAA==", Readback: true},
		},
		Shader: "CanonicalNDArrayBench",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Benchmarks) != 1 || report.Benchmarks[0].SPIRVHash != "bd3ea90711adaad03e98923d7397d5b3e259497e437918bd444c46a0c46dc083" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func findBenchSource(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		path := filepath.Join(dir, "examples", "SDSL-V", "M36a", "BasicBenchmarks.sdslvbench")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("source file not found: %s", filepath.Join("examples", "SDSL-V", "M36a", "BasicBenchmarks.sdslvbench"))
	return ""
}
