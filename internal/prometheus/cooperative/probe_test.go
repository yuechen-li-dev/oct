package cooperative

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBenchmarkArtifactHasUnambiguousPreparationRows(t *testing.T) {
	path := filepath.Join("..", "DevelopmentReport", "artifacts", "M40A", "cooperative_benchmark_rtx3070.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Rows []struct {
			Correct        bool   `json:"correct"`
			ReplayIdentity string `json:"replay_identity"`
		} `json:"rows"`
		PreparationModes []struct {
			Kernel         string `json:"kernel"`
			Mode           string `json:"mode"`
			ReplayIdentity string `json:"replay_identity"`
			KernelTiming   struct {
				MedianNS uint64 `json:"median_ns"`
			} `json:"kernel_timing"`
		} `json:"preparation_modes"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if len(artifact.Rows) != 24 {
		t.Fatalf("kernel row count = %d, want 24", len(artifact.Rows))
	}
	for _, row := range artifact.Rows {
		if !row.Correct || row.ReplayIdentity == "" {
			t.Fatalf("incomplete kernel row: %#v", row)
		}
	}
	if len(artifact.PreparationModes) != 9 {
		t.Fatalf("preparation row count = %d, want 9", len(artifact.PreparationModes))
	}
	for _, row := range artifact.PreparationModes {
		if row.Kernel == "" || row.Mode == "" || row.ReplayIdentity == "" || row.KernelTiming.MedianNS == 0 {
			t.Fatalf("incomplete preparation row: %#v", row)
		}
	}
}

func loadRTX3070Probe(t *testing.T) Probe {
	t.Helper()
	path := filepath.Join("..", "DevelopmentReport", "artifacts", "M40A", "capability_probe_rtx3070.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return probe
}

func TestParseSortAndSelectRTX3070Tuple(t *testing.T) {
	probe := loadRTX3070Probe(t)
	if probe.Device.VendorID != 0x10de || probe.Device.DeviceID != 0x2488 || len(probe.KHRTuples) != 11 {
		t.Fatalf("unexpected device probe: %#v tuple_count=%d", probe.Device, len(probe.KHRTuples))
	}
	first := SortedTuples(probe.KHRTuples)
	second := SortedTuples(slices.Clone(probe.KHRTuples))
	if !slices.Equal(first, second) {
		t.Fatal("tuple ordering is not deterministic")
	}
	selected, ok := SelectInitialTuple(probe)
	if !ok || selected.Scope != "subgroup" || selected.M != 16 || selected.N != 16 || selected.K != 16 ||
		selected.AType != "float16" || selected.CType != "float32" {
		t.Fatalf("selected tuple = %#v, %v", selected, ok)
	}
}

func TestSelectionRejectsMissingFeatureAndUnsupportedTuple(t *testing.T) {
	probe := loadRTX3070Probe(t)
	probe.Features.VulkanMemoryModel = false
	if _, ok := SelectInitialTuple(probe); ok {
		t.Fatal("missing memory-model feature was accepted")
	}
	probe = loadRTX3070Probe(t)
	probe.KHRTuples = []Tuple{{Scope: "subgroup", M: 8, N: 8, K: 16, AType: "float16", BType: "float16", CType: "float32", ResultType: "float32", MMAUsable: true}}
	if _, ok := SelectInitialTuple(probe); ok {
		t.Fatal("unsupported tuple was accepted")
	}
}

func TestAlignedTailAndReplayIdentity(t *testing.T) {
	tuple, ok := SelectInitialTuple(loadRTX3070Probe(t))
	if !ok {
		t.Fatal("fixture has no selected tuple")
	}
	if err := ValidateAlignedShape(tuple, 256, 512, 1024); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAlignedShape(tuple, 257, 259, 263); err == nil || !strings.Contains(err.Error(), "requires multiples") {
		t.Fatalf("tail validation error = %v", err)
	}
	first := ReplayIdentity("247e410e", tuple, 512, 512, 512, 99)
	second := ReplayIdentity("247e410e", tuple, 512, 512, 512, 99)
	different := ReplayIdentity("247e410e", tuple, 512, 512, 512, 100)
	if first != second || first == different {
		t.Fatal("replay identity is not stable and input-sensitive")
	}
}
