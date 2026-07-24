// Command prometheus_stage0 checks the structured, current Prometheus
// generated/package authority surface. It is intentionally descriptive about
// the known generated successor drift and does not repair or reinterpret it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type sourceAsset struct {
	ID         uint32 `json:"id"`
	Name       string `json:"name"`
	EntryPoint string `json:"entry_point"`
}

type sourceImplementation struct {
	ID       uint32 `json:"id"`
	ShaderID uint32 `json:"shader_id"`
}

type sourceManifest struct {
	ShaderAssets           []sourceAsset          `json:"shader_assets"`
	ComputeImplementations []sourceImplementation `json:"compute_implementations"`
}

type packageKernel struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

type packageVariant struct {
	ID         string `json:"id"`
	KernelID   uint32 `json:"kernel_id"`
	EntryPoint string `json:"entry_point"`
}

type packageArtifact struct {
	Digest string `json:"digest"`
}

type packageImplementation struct {
	ID        uint32 `json:"id"`
	VariantID string `json:"variant_id"`
}

type packageManifest struct {
	Package struct {
		ID         string `json:"id"`
		Version    string `json:"version"`
		RuntimeABI uint32 `json:"runtime_abi"`
	} `json:"package"`
	Tables struct {
		Kernels         []packageKernel         `json:"kernels"`
		Variants        []packageVariant        `json:"variants"`
		Artifacts       []packageArtifact       `json:"artifacts"`
		Implementations []packageImplementation `json:"implementations"`
		Provenance      []map[string]any        `json:"provenance"`
	} `json:"tables"`
}

type report struct {
	Schema string `json:"schema"`
	Status string `json:"status"`
	Source struct {
		Assets          int `json:"assets"`
		Implementations int `json:"implementations"`
	} `json:"source"`
	Package struct {
		Identity         string `json:"identity"`
		Kernels          int    `json:"kernels"`
		Variants         int    `json:"variants"`
		Artifacts        int    `json:"artifacts"`
		Implementations  int    `json:"implementations"`
		Provenance       int    `json:"provenance"`
		ObjectFiles      int    `json:"object_files"`
		ExtraObjectFiles int    `json:"extra_object_files"`
	} `json:"package"`
	Projection struct {
		GeneratedKernelIDs     int      `json:"generated_kernel_ids"`
		PackageNotGeneratedIDs []uint32 `json:"package_not_generated_ids"`
		StaticRegistryIDs      int      `json:"static_registry_ids"`
		PackageOnlyIDs         []uint32 `json:"package_only_ids"`
		UnreferencedIDs        []uint32 `json:"unreferenced_package_only_ids"`
	} `json:"projection"`
	Gemma struct {
		Kernel68Variant string `json:"kernel_68_variant"`
		Kernel69Variant string `json:"kernel_69_variant"`
	} `json:"gemma"`
	Topology struct {
		MainTransformerBlocks    int    `json:"main_transformer_blocks"`
		RepeatedMainTransformer1 int    `json:"repeated_main_transformer1_successors"`
		Status                   string `json:"status"`
	} `json:"topology"`
	ABI struct {
		ExportedSymbols int `json:"exported_symbols"`
		StaleWeightCode int `json:"stale_weight_detail_code"`
	} `json:"abi"`
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

func mustJSON(path string, out any) {
	if err := json.Unmarshal(mustRead(path), out); err != nil {
		panic(fmt.Errorf("parse %s: %w", path, err))
	}
}

func idsFromMatches(text string, pattern string) map[uint32]bool {
	result := map[uint32]bool{}
	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1) {
		value, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil {
			panic(err)
		}
		result[uint32(value)] = true
	}
	return result
}

func sortedMissing(left, right map[uint32]bool) []uint32 {
	var result []uint32
	for id := range left {
		if !right[id] {
			result = append(result, id)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func main() {
	root, _ := os.Getwd()
	check := flag.Bool("check", false, "check the current Stage 0 authority surface")
	flag.Parse()
	if !*check {
		*check = true
	}

	var source sourceManifest
	var pkg packageManifest
	sourcePath := filepath.Join(root, "internal", "prometheus", "native", "shaders", "manifest.json")
	packagePath := filepath.Join(root, "out", "prometheus", "native", "SerialCanonical", "shaders", "manifest.json")
	mustJSON(sourcePath, &source)
	mustJSON(packagePath, &pkg)

	result := report{Schema: "prometheus.stage0.authority.v1", Status: "PASS"}
	result.Source.Assets = len(source.ShaderAssets)
	result.Source.Implementations = len(source.ComputeImplementations)
	result.Package.Identity = pkg.Package.ID + "@" + pkg.Package.Version
	result.Package.Kernels = len(pkg.Tables.Kernels)
	result.Package.Variants = len(pkg.Tables.Variants)
	result.Package.Artifacts = len(pkg.Tables.Artifacts)
	result.Package.Implementations = len(pkg.Tables.Implementations)
	result.Package.Provenance = len(pkg.Tables.Provenance)
	objects, err := os.ReadDir(filepath.Join(root, "out", "prometheus", "native", "SerialCanonical", "shaders", "objects", "sha256"))
	if err != nil {
		panic(err)
	}
	result.Package.ObjectFiles = len(objects)
	result.Package.ExtraObjectFiles = result.Package.ObjectFiles - result.Package.Artifacts

	sourceIDs := map[uint32]bool{}
	for _, asset := range source.ShaderAssets {
		sourceIDs[asset.ID] = true
	}
	packageIDs := map[uint32]bool{}
	for _, kernel := range pkg.Tables.Kernels {
		packageIDs[kernel.ID] = true
	}
	if result.Source.Assets != 69 || result.Source.Implementations != 18 || result.Package.Kernels != 69 ||
		result.Package.Variants != 69 || result.Package.Artifacts != 68 || result.Package.Implementations != 18 ||
		result.Package.Provenance != 69 || result.Package.ObjectFiles < result.Package.Artifacts || len(sortedMissing(sourceIDs, packageIDs)) != 0 {
		panic("normative manifest/package projection counts or membership changed")
	}

	generatedText := string(mustRead(filepath.Join(root, "internal", "prometheus", "native", "reactor_shader_ids.generated.h")))
	generatedIDs := idsFromMatches(generatedText, `PROMETHEUS_SHADER_KERNEL_[A-Z0-9_]+\s+(\d+)u`)
	result.Projection.GeneratedKernelIDs = len(generatedIDs)
	result.Projection.PackageNotGeneratedIDs = sortedMissing(packageIDs, generatedIDs)
	if result.Projection.GeneratedKernelIDs != 66 || len(result.Projection.PackageNotGeneratedIDs) != 3 ||
		result.Projection.PackageNotGeneratedIDs[0] != 67 || result.Projection.PackageNotGeneratedIDs[1] != 68 || result.Projection.PackageNotGeneratedIDs[2] != 69 {
		panic("generated shader ID projection drifted from the current descriptive snapshot")
	}

	registryText := string(mustRead(filepath.Join(root, "internal", "prometheus", "native", "reactor_shader_registry.c")))
	registryIDs := idsFromMatches(registryText, `(?:META|META_DETAIL|REDUCTION_ASSET)\((\d+),`)
	result.Projection.StaticRegistryIDs = len(registryIDs)
	result.Projection.PackageOnlyIDs = sortedMissing(packageIDs, registryIDs)
	if len(result.Projection.PackageOnlyIDs) == 0 {
		panic("static registry unexpectedly stopped being a partial projection")
	}

	nativeRoot := filepath.Join(root, "internal", "prometheus", "native")
	var nativeText strings.Builder
	_ = filepath.Walk(nativeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".c") {
			return nil
		}
		nativeText.Write(mustRead(path))
		return nil
	})
	for _, id := range result.Projection.PackageOnlyIDs {
		if !strings.Contains(nativeText.String(), fmt.Sprintf("kernel-%d-default", id)) {
			result.Projection.UnreferencedIDs = append(result.Projection.UnreferencedIDs, id)
		}
	}
	if len(result.Projection.UnreferencedIDs) != 0 {
		panic("package-only shader identities are unreachable from native sources")
	}

	variantByID := map[string]packageVariant{}
	for _, variant := range pkg.Tables.Variants {
		variantByID[variant.ID] = variant
	}
	for _, want := range []struct {
		id    string
		name  string
		entry string
	}{
		{"kernel-68-default", "kernel-68-default", "Gemma4E2BM1RopeHalfSplit_CS"},
		{"kernel-69-default", "kernel-69-default", "Gemma4E2BM1AttentionScores_CS"},
	} {
		variant, ok := variantByID[want.id]
		if !ok || variant.EntryPoint != want.entry {
			panic(fmt.Sprintf("missing or changed %s package variant", want.id))
		}
		if want.id == "kernel-68-default" {
			result.Gemma.Kernel68Variant = variant.ID
		} else {
			result.Gemma.Kernel69Variant = variant.ID
		}
	}

	lock := string(mustRead(filepath.Join(root, "internal", "prometheus", "models", "zimage-turbo", "lock-tagon.octagon")))
	mainLines := regexp.MustCompile(`Family: "ZImageTurbo\.MainTransformer"`).FindAllStringIndex(lock, -1)
	result.Topology.MainTransformerBlocks = len(mainLines)
	result.Topology.RepeatedMainTransformer1 = strings.Count(strings.Join(strings.Split(lock, "\n"), "\n"), `Successor: "MainTransformer1"`)
	result.Topology.Status = "descriptive_disputed_current_projection"
	if result.Topology.MainTransformerBlocks != 30 || result.Topology.RepeatedMainTransformer1 != 29 {
		panic("generated MainTransformer topology snapshot changed")
	}

	apiHeader := string(mustRead(filepath.Join(root, "internal", "prometheus", "native", "reactor_api.h")))
	result.ABI.ExportedSymbols = len(regexp.MustCompile(`PROM_REACTOR_API\s+[A-Za-z_][A-Za-z0-9_\s\*]*\([^;]*\);`).FindAllString(apiHeader, -1))
	result.ABI.StaleWeightCode = -7406
	if result.ABI.ExportedSymbols != 84 || !strings.Contains(string(mustRead(filepath.Join(root, "internal", "prometheus", "native", "reactor_vulkan.h"))), "PROM_M46_DETAIL_STALE_WEIGHT_GENERATION = -7406") {
		panic("public export or known detail-code snapshot changed")
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
