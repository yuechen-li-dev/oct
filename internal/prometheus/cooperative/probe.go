// Package cooperative owns the deterministic host-side interpretation of the
// M40a capability probe. It does not participate in production selection.
package cooperative

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

type Probe struct {
	Schema     string `json:"schema"`
	Device     Device `json:"device"`
	Extensions struct {
		KHR uint32 `json:"VK_KHR_cooperative_matrix"`
		NV  uint32 `json:"VK_NV_cooperative_matrix"`
		NV2 uint32 `json:"VK_NV_cooperative_matrix2"`
	} `json:"extensions"`
	Features struct {
		ShaderFloat16     bool `json:"shader_float16"`
		VulkanMemoryModel bool `json:"vulkan_memory_model"`
		KHRCooperative    bool `json:"khr_cooperative_matrix"`
	} `json:"features"`
	Subgroup struct {
		Size uint32 `json:"size"`
	} `json:"subgroup"`
	KHRTuples []Tuple `json:"khr_tuples"`
}

type Device struct {
	Name          string `json:"name"`
	APIVersion    uint32 `json:"api_version"`
	VendorID      uint32 `json:"vendor_id"`
	DeviceID      uint32 `json:"device_id"`
	DriverVersion uint32 `json:"driver_version"`
}

type Tuple struct {
	Scope                  string `json:"scope"`
	M                      uint32 `json:"m"`
	N                      uint32 `json:"n"`
	K                      uint32 `json:"k"`
	AType                  string `json:"a_type"`
	BType                  string `json:"b_type"`
	CType                  string `json:"c_type"`
	ResultType             string `json:"result_type"`
	SaturatingAccumulation bool   `json:"saturating_accumulation"`
	MMAUsable              bool   `json:"mma_usable"`
	Usefulness             string `json:"usefulness"`
}

func Parse(data []byte) (Probe, error) {
	var probe Probe
	if err := json.Unmarshal(data, &probe); err != nil {
		return Probe{}, fmt.Errorf("decode cooperative matrix probe: %w", err)
	}
	if probe.Schema != "prometheus.m40a.cooperative-matrix-probe.v1" {
		return Probe{}, fmt.Errorf("unsupported cooperative matrix probe schema %q", probe.Schema)
	}
	if probe.Device.Name == "" {
		return Probe{}, fmt.Errorf("cooperative matrix probe lacks device identity")
	}
	return probe, nil
}

func SortedTuples(in []Tuple) []Tuple {
	out := append([]Tuple(nil), in...)
	slices.SortFunc(out, func(a, b Tuple) int {
		for _, values := range [][2]string{{a.Scope, b.Scope}, {a.AType, b.AType}, {a.BType, b.BType}, {a.CType, b.CType}, {a.ResultType, b.ResultType}} {
			if values[0] < values[1] {
				return -1
			}
			if values[0] > values[1] {
				return 1
			}
		}
		for _, values := range [][2]uint32{{a.M, b.M}, {a.N, b.N}, {a.K, b.K}} {
			if values[0] < values[1] {
				return -1
			}
			if values[0] > values[1] {
				return 1
			}
		}
		if !a.SaturatingAccumulation && b.SaturatingAccumulation {
			return -1
		}
		if a.SaturatingAccumulation && !b.SaturatingAccumulation {
			return 1
		}
		return 0
	})
	return out
}

func SelectInitialTuple(probe Probe) (Tuple, bool) {
	if probe.Extensions.KHR == 0 || !probe.Features.KHRCooperative ||
		!probe.Features.ShaderFloat16 || !probe.Features.VulkanMemoryModel ||
		probe.Subgroup.Size != 32 {
		return Tuple{}, false
	}
	for _, tuple := range SortedTuples(probe.KHRTuples) {
		if tuple.Scope == "subgroup" && tuple.M == 16 && tuple.N == 16 && tuple.K == 16 &&
			tuple.AType == "float16" && tuple.BType == "float16" &&
			tuple.CType == "float32" && tuple.ResultType == "float32" && tuple.MMAUsable {
			return tuple, true
		}
	}
	return Tuple{}, false
}

func ValidateAlignedShape(tuple Tuple, m, n, k uint32) error {
	if tuple.M == 0 || tuple.N == 0 || tuple.K == 0 {
		return fmt.Errorf("cooperative tuple has a zero dimension")
	}
	if m%tuple.M != 0 || n%tuple.N != 0 || k%tuple.K != 0 {
		return fmt.Errorf("cooperative shape %dx%dx%d requires multiples of %dx%dx%d", m, n, k, tuple.M, tuple.N, tuple.K)
	}
	return nil
}

func ReplayIdentity(shaderHash string, tuple Tuple, m, n, k uint32, seed uint64) string {
	payload := fmt.Sprintf("%s|%s|%d|%d|%d|%s|%s|%s|%s|%d|%d|%d|%d",
		shaderHash, tuple.Scope, tuple.M, tuple.N, tuple.K, tuple.AType, tuple.BType,
		tuple.CType, tuple.ResultType, m, n, k, seed)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
