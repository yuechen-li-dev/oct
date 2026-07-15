package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestM38aKaijuMemoryPlacementDiagnostic is an audit-only controlled experiment.
// It forces each supported memory choice without altering shader or dispatch
// metadata, and optionally rewrites mapped inputs immediately before dispatch.
func TestM38aKaijuMemoryPlacementDiagnostic(t *testing.T) {
	requireKaijuHardware(t)
	sidecar, err := resolveKaijuSidecar()
	if err != nil {
		t.Fatal(err)
	}
	root := findM37bRepo(t)
	workloads := [][3]uint32{{512, 512, 8}, {512, 512, 512}, {127, 131, 129}}
	kernels := m37bKernels()[6:]
	type row struct {
		Kernel      string    `json:"kernel"`
		Memory      string    `json:"memory"`
		Reupload    bool      `json:"reupload_inputs"`
		M           uint32    `json:"m"`
		N           uint32    `json:"n"`
		K           uint32    `json:"k"`
		Groups      [3]uint32 `json:"groups"`
		SpirvSHA256 string    `json:"spirv_sha256"`
		ABytes      int       `json:"a_bytes"`
		BBytes      int       `json:"b_bytes"`
		CBytes      uint32    `json:"c_bytes"`
		SamplesNS   []uint64  `json:"samples_ns"`
		MedianNS    uint64    `json:"median_ns"`
	}
	var rows []row
	modes := []struct {
		memory   string
		reupload bool
	}{
		{memory: "host-visible-coherent"},
		{memory: "host-visible-device-local"},
		{memory: "host-visible-device-local", reupload: true},
	}
	for _, mode := range modes {
		t.Setenv("OCT_KAIJU_VULKAN_M38A_MEMORY", mode.memory)
		if mode.reupload {
			t.Setenv("OCT_KAIJU_VULKAN_M38A_REUPLOAD_INPUTS", "1")
		} else {
			t.Setenv("OCT_KAIJU_VULKAN_M38A_REUPLOAD_INPUTS", "")
		}
		for _, kernel := range kernels {
			spirv := m37bArtifact(t, filepath.Join(root, filepath.FromSlash(kernel.path)), kernel.symbol)
			for _, workload := range workloads {
				a, b := m37bInputs(workload[0], workload[1], workload[2])
				pa, pb, kind := m37bPayload(kernel.mode, a, b, workload[0], workload[1], workload[2])
				request := m37bBenchmarkRequest(kernel, workload, spirv, pa, pb, kind)
				request.Warmup = 1
				request.Iterations = 3
				response, err := invokeKaijuBenchmark(sidecar, request)
				if err != nil || !response.Success {
					t.Fatalf("%s reupload=%t %s %v: %v %#v", mode.memory, mode.reupload, kernel.name, workload, err, response.Errors)
				}
				stats := StatisticsFor(response.Timing.SamplesNS)
				rows = append(rows, row{
					Kernel: kernel.name, Memory: mode.memory, Reupload: mode.reupload,
					M: workload[0], N: workload[1], K: workload[2],
					Groups:      [3]uint32{request.DispatchGroups.X, request.DispatchGroups.Y, request.DispatchGroups.Z},
					SpirvSHA256: request.SpirvSHA256,
					ABytes:      len(pa), BBytes: len(pb), CBytes: workload[0] * workload[1] * 4,
					SamplesNS: response.Timing.SamplesNS, MedianNS: stats.Median,
				})
			}
		}
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "out", "test-artifacts", "m38a_kaiju_memory_diagnostic.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
