package bench

// Canonical artifact generation is deliberately separate from benchmark
// execution. It preserves isolated compiler output as a reviewable M36b input.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
)

const CanonicalArtifactSchemaVersion = 1

type CanonicalArtifact struct {
	SchemaVersion  int        `json:"schemaVersion"`
	Name           string     `json:"name"`
	BenchmarkID    string     `json:"benchmarkId"`
	Source         string     `json:"source"`
	SourceSHA256   string     `json:"sourceSha256"`
	Compiler       string     `json:"compiler"`
	DXCPath        string     `json:"dxcPath"`
	DXCVersion     string     `json:"dxcVersion"`
	DXCArgs        []string   `json:"dxcArgs"`
	VulkanTarget   string     `json:"vulkanTarget"`
	EntryPoint     string     `json:"entryPoint"`
	WorkgroupSize  [3]uint32  `json:"workgroupSize"`
	DispatchGroups [3]uint32  `json:"dispatchGroups"`
	Resources      []Resource `json:"resources"`
	PushConstants  int        `json:"pushConstantsBytes"`
	SPIRVPath      string     `json:"spirvPath"`
	SPIRVBytes     int        `json:"spirvBytes"`
	SPIRVSHA256    string     `json:"spirvSha256"`
}
type CanonicalManifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Artifacts     []CanonicalArtifact `json:"artifacts"`
}

// GenerateCanonicalArtifacts emits the M36b ndarray/tensor authorities. The
// caller commits the resulting SPIR-V and manifest only after review.
func GenerateCanonicalArtifacts(sourcePath, outDir string) (CanonicalManifest, error) {
	m, err := Discover(sourcePath)
	if err != nil {
		return CanonicalManifest{}, err
	}
	module, err := loadBenchmarkModule(sourcePath)
	if err != nil {
		return CanonicalManifest{}, err
	}
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return CanonicalManifest{}, err
	}
	sourceSum := sha256.Sum256(sourceBytes)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return CanonicalManifest{}, err
	}
	selected := map[string]bool{"NDArrayMaterializeStorage": true, "TensorContractionStorage": true}
	manifest := CanonicalManifest{SchemaVersion: CanonicalArtifactSchemaVersion}
	for _, c := range m.Benchmarks {
		if !selected[c.Name] {
			continue
		}
		mir, err := lower.Module(isolatedModule(module, c))
		if err != nil {
			return CanonicalManifest{}, err
		}
		hlslText, err := hlsl.EmitBenchmark(mir, c.EntryPoint, c.WorkgroupSize)
		if err != nil {
			return CanonicalManifest{}, err
		}
		base := strings.ToLower(strings.TrimSuffix(c.Name, "Storage"))
		hlslPath, spvPath := filepath.Join(outDir, base+".hlsl"), filepath.Join(outDir, base+".spv")
		if err := os.WriteFile(hlslPath, []byte(hlslText), 0o644); err != nil {
			return CanonicalManifest{}, err
		}
		if err := compileSPIRV(hlslPath, spvPath); err != nil {
			return CanonicalManifest{}, err
		}
		if err := validateSPIRV(spvPath); err != nil {
			return CanonicalManifest{}, err
		}
		spv, err := os.ReadFile(spvPath)
		if err != nil {
			return CanonicalManifest{}, err
		}
		sum := sha256.Sum256(spv)
		dxc, _ := exec.LookPath("dxc")
		manifest.Artifacts = append(manifest.Artifacts, CanonicalArtifact{SchemaVersion: CanonicalArtifactSchemaVersion, Name: c.Name, BenchmarkID: c.ID, Source: sourceIdentity(sourcePath), SourceSHA256: hex.EncodeToString(sourceSum[:]), Compiler: compilerRevision(), DXCPath: filepath.ToSlash(dxc), DXCVersion: toolVersion(dxc), DXCArgs: []string{"-T", "cs_6_0", "-E", "main", "-spirv", "-fspv-target-env=vulkan1.0", "-Fo", filepath.ToSlash(spvPath), filepath.ToSlash(hlslPath)}, VulkanTarget: "vulkan1.0", EntryPoint: "main", WorkgroupSize: c.WorkgroupSize, DispatchGroups: c.DispatchGroups, Resources: c.Resources, PushConstants: 0, SPIRVPath: filepath.ToSlash(filepath.Join("examples", "SDSL-V", "M36a", "artifacts", filepath.Base(spvPath))), SPIRVBytes: len(spv), SPIRVSHA256: hex.EncodeToString(sum[:])})
	}
	if len(manifest.Artifacts) != 2 {
		return CanonicalManifest{}, fmt.Errorf("canonical M36b cases missing from %s", sourcePath)
	}
	sort.Slice(manifest.Artifacts, func(i, j int) bool { return manifest.Artifacts[i].Name < manifest.Artifacts[j].Name })
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return CanonicalManifest{}, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(encoded, '\n'), 0o644); err != nil {
		return CanonicalManifest{}, err
	}
	return manifest, nil
}
func validateSPIRV(path string) error {
	p, err := exec.LookPath("spirv-val")
	if err != nil {
		return fmt.Errorf("spirv-val is required for canonical M36b artifacts: %w", err)
	}
	out, err := exec.Command(p, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("spirv-val %s: %w: %s", path, err, out)
	}
	return nil
}
func compilerRevision() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Path + "@" + info.Main.Version
	}
	return "github.com/yuechen-li-dev/oct@unknown"
}
func toolVersion(path string) string {
	if path == "" {
		return "unavailable"
	}
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(out))
}
