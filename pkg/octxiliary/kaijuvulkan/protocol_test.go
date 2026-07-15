package kaijuvulkan_test

import (
	"bytes"
	"testing"

	internal "github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

func TestBenchmarkRequestRoundTrip(t *testing.T) {
	request := kaijuvulkan.BenchmarkRequest{
		DispatchRequest: kaijuvulkan.DispatchRequest{
			BenchmarkID:    "bench-id",
			ReplayID:       "replay-id",
			Spirv:          []byte{3, 2, 35, 7},
			SpirvSHA256:    "abc",
			EntryPoint:     "entry",
			WorkgroupSize:  kaijuvulkan.UInt3{X: 1, Y: 2, Z: 3},
			DispatchGroups: kaijuvulkan.UInt3{X: 4, Y: 5, Z: 6},
			PushConstants:  []byte{1, 2, 3, 4},
			Resources: []kaijuvulkan.Resource{{
				Set: 0, Binding: 1, Access: kaijuvulkan.ResourceAccessReadwrite, Kind: kaijuvulkan.ResourceKindStorageBuffer,
				ElementType: kaijuvulkan.ElementTypeF32, ByteLength: 8, Payload: []byte{9, 8, 7, 6, 5, 4, 3, 2}, Readback: true,
			}},
		},
		Warmup:     2,
		Iterations: 8,
	}
	frame := internal.EncodeRequest(internal.Request{
		ID: 1, Family: kaijuvulkan.Family, Function: kaijuvulkan.OperationBenchmark, HasArgs: true,
		Args: []internal.Value{kaijuvulkan.BenchmarkRequestValue(request)},
	})
	parsed, err := internal.ParseRequest(frame)
	if err != nil {
		t.Fatal(err)
	}
	got, err := kaijuvulkan.ParseBenchmarkRequestArg(parsed, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.BenchmarkID != request.BenchmarkID || got.ReplayID != request.ReplayID || !bytes.Equal(got.Spirv, request.Spirv) || len(got.Resources) != 1 || !bytes.Equal(got.Resources[0].Payload, request.Resources[0].Payload) {
		t.Fatalf("unexpected roundtrip: %#v", got)
	}
}

func TestDispatchResponseRoundTrip(t *testing.T) {
	response := kaijuvulkan.DispatchResponse{
		Success:     true,
		BenchmarkID: "bench-id",
		ReplayID:    "replay-id",
		SpirvSHA256: "hash",
		Device:      kaijuvulkan.DeviceInfo{RuntimeName: "sidecar", RuntimeVersion: "v1", DeviceName: "gpu", VendorID: 1, DeviceID: 2, DriverVersion: 3, VulkanAPIVersion: "1.0", TimestampPeriodNS: 4.5, TimestampValidBits: 64, QueueFamilyIndex: 2, QueueFlags: 7, BufferMemoryTypeIndex: 4, BufferMemoryPropertyFlags: 7, BufferUsageFlags: 32, BufferSharingMode: "VK_SHARING_MODE_EXCLUSIVE", BufferMemoryAlignment: 256, BufferMemoryOffset: 0, Headless: true},
		Timing: kaijuvulkan.Timing{Source: kaijuvulkan.TimingSourceVulkanQueryPoolGPU, StageSpan: kaijuvulkan.TimingStageBoundsComputeDispatch, TimestampStartStage: kaijuvulkan.TimingStageTopOfPipe, TimestampEndStage: kaijuvulkan.TimingStageBottomOfPipe, IntervalCommands: []string{
			kaijuvulkan.TimingCommandBindPipeline,
			kaijuvulkan.TimingCommandBindDescriptorSets,
			kaijuvulkan.TimingCommandPushConstants,
			kaijuvulkan.TimingCommandDispatch,
		}, DispatchesPerSample: 1, QueryResetLocation: "before", FenceWaitLocation: "after", ResultRetrievalLocation: "after_wait", SamplesNS: []uint64{10, 12}},
		Validation: kaijuvulkan.ValidationStatus{Requested: true, Available: true, Enabled: true, Warnings: 1, Errors: 0},
		Readbacks:  []kaijuvulkan.Readback{{Set: 0, Binding: 1, Payload: []byte{1, 2, 3}}},
		Warnings:   []kaijuvulkan.Diagnostic{{Severity: kaijuvulkan.DiagnosticSeverityWarning, Type: kaijuvulkan.DiagnosticTypeValidation, MessageID: "warn", Message: "careful"}},
	}
	value := kaijuvulkan.DispatchResponseValue(response)
	got, err := kaijuvulkan.ParseDispatchResponseValue(value)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Success || got.ReplayID != response.ReplayID || got.Device.DeviceName != response.Device.DeviceName || got.Device.BufferMemoryPropertyFlags != 7 || len(got.Timing.SamplesNS) != 2 || got.Timing.DispatchesPerSample != 1 || len(got.Timing.IntervalCommands) != 4 || len(got.Readbacks) != 1 || len(got.Warnings) != 1 {
		t.Fatalf("unexpected response roundtrip: %#v", got)
	}
}
