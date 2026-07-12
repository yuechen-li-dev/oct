package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

func TestCapabilitiesDirectHardware(t *testing.T) {
	if os.Getenv("OCT_KAIJU_VULKAN_TESTS") != "1" {
		t.Skip("set OCT_KAIJU_VULKAN_TESTS=1 to run Kaiju Vulkan hardware tests")
	}
	response := capabilities(octxiliary.Request{ID: 1, Family: kaijuvulkan.Family, Function: kaijuvulkan.OperationCapabilities})
	if !response.OK || !response.HasValue {
		t.Fatalf("unexpected response: %#v", response)
	}
	value, err := kaijuvulkan.ParseCapabilitiesValue(response.Value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("validation available=%t enabled=%t device=%s", value.ValidationAvailable, value.ValidationEnabled, value.Device.DeviceName)
}

func TestBenchmarkDirectHardware(t *testing.T) {
	if os.Getenv("OCT_KAIJU_VULKAN_TESTS") != "1" {
		t.Skip("set OCT_KAIJU_VULKAN_TESTS=1 to run Kaiju Vulkan hardware tests")
	}
	spvPath := filepath.Join("..", "..", "examples", "SDSL-V", "M36a", "artifacts", "ndarraymaterialize.spv")
	spirv, err := os.ReadFile(spvPath)
	if err != nil {
		t.Fatal(err)
	}
	req := kaijuvulkan.BenchmarkRequest{
		DispatchRequest: kaijuvulkan.DispatchRequest{
			BenchmarkID: "sdslvbench-8b1f66233dd54390f518e9c7",
			ReplayID: "test-replay",
			Spirv: spirv,
			SpirvSHA256: "bd3ea90711adaad03e98923d7397d5b3e259497e437918bd444c46a0c46dc083",
			EntryPoint: "main",
			WorkgroupSize: kaijuvulkan.UInt3{X: 1, Y: 1, Z: 1},
			DispatchGroups: kaijuvulkan.UInt3{X: 4, Y: 1, Z: 1},
			Resources: []kaijuvulkan.Resource{
				{Set: 0, Binding: 0, Access: kaijuvulkan.ResourceAccessReadonly, Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: kaijuvulkan.ElementTypeF32, ByteLength: 16, Payload: make([]byte, 16)},
				{Set: 0, Binding: 1, Access: kaijuvulkan.ResourceAccessReadwrite, Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: kaijuvulkan.ElementTypeF32, ByteLength: 16, Payload: make([]byte, 16), Readback: true},
			},
		},
		Warmup: 2,
		Iterations: 8,
	}
	resp := benchmark(1, req)
	if !resp.OK || !resp.HasValue {
		t.Fatalf("unexpected response: %#v", resp)
	}
	decoded, err := kaijuvulkan.ParseDispatchResponseValue(resp.Value)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Success {
		t.Fatalf("benchmark failed: %#v", decoded)
	}
	t.Logf("validation=%#v", decoded.Validation)
	if len(decoded.Timing.SamplesNS) != 8 {
		t.Fatalf("unexpected timing samples: %#v", decoded.Timing.SamplesNS)
	}
	if len(decoded.Readbacks) != 1 {
		t.Fatalf("unexpected readbacks: %#v", decoded.Readbacks)
	}
	wantPrefix, _ := hex.DecodeString("0000803f0000803f0000803f0000803f")
	if string(decoded.Readbacks[0].Payload) != string(wantPrefix) {
		t.Fatalf("unexpected readback payload: %x", decoded.Readbacks[0].Payload)
	}
}

func TestDispatchDirectHardware(t *testing.T) {
	if os.Getenv("OCT_KAIJU_VULKAN_TESTS") != "1" {
		t.Skip("set OCT_KAIJU_VULKAN_TESTS=1 to run Kaiju Vulkan hardware tests")
	}
	spvPath := filepath.Join("..", "..", "examples", "SDSL-V", "M36a", "artifacts", "tensorcontraction.spv")
	spirv, err := os.ReadFile(spvPath)
	if err != nil {
		t.Fatal(err)
	}
	req := kaijuvulkan.DispatchRequest{
		BenchmarkID: "sdslvbench-a2b7fd8383074dd673a365d5",
		ReplayID: "dispatch-replay",
		Spirv: spirv,
		SpirvSHA256: "9c14708fb37490d3f0f776a2cd4b156dbf00936fb8a4d6f5db159718f393a3a7",
		EntryPoint: "main",
		WorkgroupSize: kaijuvulkan.UInt3{X: 1, Y: 1, Z: 1},
		DispatchGroups: kaijuvulkan.UInt3{X: 4, Y: 1, Z: 1},
		Resources: []kaijuvulkan.Resource{
			{Set: 0, Binding: 0, Access: kaijuvulkan.ResourceAccessReadonly, Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: kaijuvulkan.ElementTypeF32, ByteLength: 16, Payload: make([]byte, 16)},
			{Set: 0, Binding: 1, Access: kaijuvulkan.ResourceAccessReadwrite, Kind: kaijuvulkan.ResourceKindStorageBuffer, ElementType: kaijuvulkan.ElementTypeF32, ByteLength: 16, Payload: make([]byte, 16), Readback: true},
		},
	}
	resp := dispatch(1, req)
	if !resp.OK || !resp.HasValue {
		t.Fatalf("unexpected response: %#v", resp)
	}
	decoded, err := kaijuvulkan.ParseDispatchResponseValue(resp.Value)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Success || len(decoded.Readbacks) != 1 || len(decoded.Timing.SamplesNS) != 0 {
		t.Fatalf("unexpected dispatch response: %#v", decoded)
	}
	t.Logf("validation=%#v", decoded.Validation)
}
