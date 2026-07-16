// Command sdslv_workspace_check verifies the small ownership boundaries that
// keep SDSL-V production, audit, and canonical benchmark paths distinct.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/bench"
)

type workspace struct {
	ProductionSourceRoot   string `json:"production_source_root"`
	ExperimentalSourceRoot string `json:"experimental_source_root"`
	HistoricalAuditRoot    string `json:"historical_audit_root"`
	CanonicalBenchmarkRoot string `json:"canonical_benchmark_root"`
}

type shaderAsset struct {
	ID             uint32 `json:"id"`
	Name           string `json:"name"`
	Authority      string `json:"authority"`
	SourceLanguage string `json:"source_language"`
	Source         string `json:"source"`
	Header         string `json:"header"`
}

type computeImplementation struct {
	ID               uint32 `json:"id"`
	Name             string `json:"name"`
	Authority        string `json:"authority"`
	Operation        string `json:"operation"`
	ShaderID         uint32 `json:"shader_id"`
	SelectorEligible bool   `json:"selector_eligible"`
}

type experimentalShaderAsset struct {
	ID                  string `json:"id"`
	Authority           string `json:"authority"`
	ProductionAuthority string `json:"production_authority"`
	SelectorEligible    bool   `json:"selector_eligible"`
	SourceLanguage      string `json:"source_language"`
	Source              string `json:"source"`
	Output              string `json:"output"`
	GeneratedHLSL       string `json:"generated_hlsl"`
	GeneratedHeader     string `json:"generated_header"`
	Inspection          string `json:"inspection"`
	ShaderSHA256        string `json:"shader_sha256"`
}

type shaderManifest struct {
	Workspace                workspace                 `json:"workspace"`
	ShaderAssets             []shaderAsset             `json:"shader_assets"`
	ExperimentalShaderAssets []experimentalShaderAsset `json:"experimental_shader_assets"`
	ComputeImplementations   []computeImplementation   `json:"compute_implementations"`
}

type canonicalArtifact struct {
	Name         string `json:"name"`
	BenchmarkID  string `json:"benchmarkId"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"sourceSha256"`
	SPIRVPath    string `json:"spirvPath"`
	SPIRVSHA256  string `json:"spirvSha256"`
}

type canonicalManifest struct {
	Artifacts []canonicalArtifact `json:"artifacts"`
}

var wordRE = regexp.MustCompile(`0x([0-9a-fA-F]{8})u`)

func main() {
	inventory := flag.Bool("inventory", false, "print production source and generated module hashes")
	flag.Parse()
	if flag.NArg() != 0 {
		fail(fmt.Errorf("usage: go run ./tools/sdslv_workspace_check [-inventory]"))
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if err := check(root, *inventory); err != nil {
		fail(err)
	}
}

func check(root string, inventory bool) error {
	var m shaderManifest
	manifestPath := filepath.Join(root, "internal", "prometheus", "native", "shaders", "manifest.json")
	if err := readJSON(manifestPath, &m); err != nil {
		return err
	}
	if m.Workspace.ProductionSourceRoot == "" || m.Workspace.ExperimentalSourceRoot == "" || m.Workspace.HistoricalAuditRoot == "" || m.Workspace.CanonicalBenchmarkRoot == "" {
		return fmt.Errorf("shader manifest lacks complete workspace ownership roots")
	}
	for _, path := range []string{
		m.Workspace.ProductionSourceRoot,
		m.Workspace.ExperimentalSourceRoot,
		m.Workspace.HistoricalAuditRoot,
		m.Workspace.CanonicalBenchmarkRoot,
		"docs/SDSL_V_WORKSPACE.md",
		"internal/prometheus/DevelopmentReport/SDSL_V_M39A_WORKSPACE_PRODUCTIZATION.md",
		"internal/prometheus/native/Marionette/reactor_shader_registry_tests.cpp",
	} {
		if err := mustExist(root, path); err != nil {
			return err
		}
	}
	registry, err := os.ReadFile(filepath.Join(root, "internal", "prometheus", "native", "reactor_shader_registry.c"))
	if err != nil {
		return err
	}
	seenIDs := map[uint32]string{}
	seenSources := map[string]string{}
	assetsByID := map[uint32]shaderAsset{}
	lines := make([]string, 0)
	for index, asset := range m.ShaderAssets {
		if prior, ok := seenIDs[asset.ID]; ok {
			return fmt.Errorf("duplicate shader id %d: %s and %s", asset.ID, prior, asset.Name)
		}
		if index != 0 && asset.ID <= m.ShaderAssets[index-1].ID {
			return fmt.Errorf("shader assets are not in deterministic ascending id order at %d", asset.ID)
		}
		seenIDs[asset.ID] = asset.Name
		assetsByID[asset.ID] = asset
		if asset.SourceLanguage != "sdslv" {
			continue
		}
		authority := asset.Authority
		if authority == "" {
			authority = "production"
		}
		switch authority {
		case "production":
			if !strings.HasPrefix(asset.Source, m.Workspace.ProductionSourceRoot+"/") {
				return fmt.Errorf("production registry asset %d (%s) points outside production source root: %s", asset.ID, asset.Name, asset.Source)
			}
			if strings.Contains(asset.Source, "/experimental/") || strings.Contains(asset.Source, "/historical/") {
				return fmt.Errorf("production registry asset %d points at non-production source: %s", asset.ID, asset.Source)
			}
		case "experimental":
			if !strings.HasPrefix(asset.Source, m.Workspace.ExperimentalSourceRoot+"/") {
				return fmt.Errorf("experimental registry asset %d (%s) points outside experimental source root: %s", asset.ID, asset.Name, asset.Source)
			}
		default:
			return fmt.Errorf("shader asset %d (%s) has unknown authority %q", asset.ID, asset.Name, authority)
		}
		if prior, ok := seenSources[asset.Source]; ok {
			return fmt.Errorf("duplicate SDSL-V source authority %s: %s and %s", asset.Source, prior, asset.Name)
		}
		seenSources[asset.Source] = asset.Name
		if err := mustExist(root, asset.Source); err != nil {
			return err
		}
		if !strings.Contains(string(registry), `"`+asset.Source+`"`) {
			return fmt.Errorf("registry does not own manifest source %s", asset.Source)
		}
		headerPath := filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(asset.Header))
		header, err := os.ReadFile(headerPath)
		if err != nil {
			return fmt.Errorf("asset %d generated header %s: %w", asset.ID, asset.Header, err)
		}
		if !strings.Contains(string(header), "// Source: "+asset.Source+"\n") {
			return fmt.Errorf("generated header %s does not identify source %s", asset.Header, asset.Source)
		}
		if !strings.Contains(string(header), "// Generated by: oct sdslv generate-header "+asset.Source) {
			return fmt.Errorf("generated header %s lacks regeneration command", asset.Header)
		}
		moduleHash, err := headerModuleHash(header)
		if err != nil {
			return fmt.Errorf("generated header %s: %w", asset.Header, err)
		}
		if inventory {
			lines = append(lines, fmt.Sprintf("id=%d name=%s authority=%s source_sha256=%s module_sha256=%s source=%s header=%s", asset.ID, asset.Name, authority, fileHash(filepath.Join(root, filepath.FromSlash(asset.Source))), moduleHash, asset.Source, asset.Header))
		}
	}
	seenExperimental := map[string]bool{}
	for _, asset := range m.ExperimentalShaderAssets {
		if asset.ID == "" || seenExperimental[asset.ID] {
			return fmt.Errorf("experimental shader asset has empty or duplicate id %q", asset.ID)
		}
		seenExperimental[asset.ID] = true
		if asset.Authority != "experimental" || asset.ProductionAuthority != "experimental" || asset.SelectorEligible {
			return fmt.Errorf("experimental shader asset %s must remain experimental and selector-ineligible", asset.ID)
		}
		if asset.SourceLanguage != "sdslv" || !strings.HasPrefix(asset.Source, m.Workspace.ExperimentalSourceRoot+"/") {
			return fmt.Errorf("experimental shader asset %s has invalid source ownership: %s", asset.ID, asset.Source)
		}
		for _, path := range []string{asset.Source, asset.Output, asset.GeneratedHLSL, asset.GeneratedHeader, asset.Inspection} {
			if err := mustExist(root, path); err != nil {
				return fmt.Errorf("experimental shader asset %s: %w", asset.ID, err)
			}
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(asset.Output))); got != strings.ToLower(asset.ShaderSHA256) {
			return fmt.Errorf("experimental shader asset %s hash mismatch: manifest=%s file=%s", asset.ID, asset.ShaderSHA256, got)
		}
		if strings.Contains(string(registry), asset.Source) {
			return fmt.Errorf("experimental shader asset %s leaked into the production runtime registry", asset.ID)
		}
	}
	seenImplementationIDs := map[uint32]string{}
	for index, implementation := range m.ComputeImplementations {
		if prior, ok := seenImplementationIDs[implementation.ID]; ok {
			return fmt.Errorf("duplicate compute implementation id %d: %s and %s", implementation.ID, prior, implementation.Name)
		}
		if index != 0 && implementation.ID <= m.ComputeImplementations[index-1].ID {
			return fmt.Errorf("compute implementations are not in deterministic ascending id order at %d", implementation.ID)
		}
		seenImplementationIDs[implementation.ID] = implementation.Name
		asset, ok := assetsByID[implementation.ShaderID]
		if !ok {
			return fmt.Errorf("compute implementation %d (%s) references unknown shader %d", implementation.ID, implementation.Name, implementation.ShaderID)
		}
		implementationAuthority := implementation.Authority
		if implementationAuthority == "" {
			implementationAuthority = "production"
		}
		assetAuthority := asset.Authority
		if assetAuthority == "" {
			assetAuthority = "production"
		}
		if implementationAuthority != assetAuthority {
			return fmt.Errorf("compute implementation %d authority %q disagrees with shader %d authority %q", implementation.ID, implementationAuthority, asset.ID, assetAuthority)
		}
		if implementationAuthority == "experimental" && implementation.SelectorEligible {
			return fmt.Errorf("experimental compute implementation %d (%s) must not be selector eligible", implementation.ID, implementation.Name)
		}
		isReduction := implementation.Operation == "row-sum" || implementation.Operation == "row-max" || implementation.Operation == "softmax"
		if isReduction {
			root := m.Workspace.ProductionSourceRoot
			if implementationAuthority == "experimental" {
				root = m.Workspace.ExperimentalSourceRoot
			}
			expectedPrefix := root + "/reduction/"
			if !strings.HasPrefix(asset.Source, expectedPrefix) {
				return fmt.Errorf("%s reduction implementation %d source must be confined to %s: %s", implementationAuthority, implementation.ID, expectedPrefix, asset.Source)
			}
		}
	}
	if err := checkCanonicalArtifacts(root); err != nil {
		return err
	}
	if err := checkBenchmarkIdentities(root); err != nil {
		return err
	}
	if inventory {
		sort.Strings(lines)
		for _, line := range lines {
			fmt.Println(line)
		}
	}
	return nil
}

func checkCanonicalArtifacts(root string) error {
	var m canonicalManifest
	path := filepath.Join(root, "examples", "SDSL-V", "M36a", "artifacts", "manifest.json")
	if err := readJSON(path, &m); err != nil {
		return err
	}
	if len(m.Artifacts) == 0 {
		return fmt.Errorf("canonical benchmark artifact manifest is empty")
	}
	for _, artifact := range m.Artifacts {
		if err := mustExist(root, artifact.Source); err != nil {
			return err
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(artifact.Source))); got != artifact.SourceSHA256 {
			return fmt.Errorf("canonical benchmark %s source hash: got %s want %s", artifact.Name, got, artifact.SourceSHA256)
		}
		if err := mustExist(root, artifact.SPIRVPath); err != nil {
			return err
		}
		if got := fileHash(filepath.Join(root, filepath.FromSlash(artifact.SPIRVPath))); got != artifact.SPIRVSHA256 {
			return fmt.Errorf("canonical benchmark %s SPIR-V hash: got %s want %s", artifact.Name, got, artifact.SPIRVSHA256)
		}
	}
	return nil
}

func checkBenchmarkIdentities(root string) error {
	source := filepath.Join(root, "examples", "SDSL-V", "M36a", "BasicBenchmarks.sdslvbench")
	benchmarks, err := bench.Discover(source)
	if err != nil {
		return fmt.Errorf("discover permanent benchmark corpus: %w", err)
	}
	byID := make(map[string]bench.Case, len(benchmarks.Benchmarks))
	for _, benchmark := range benchmarks.Benchmarks {
		if benchmark.ReplayID == "" {
			return fmt.Errorf("benchmark %s has no replay id", benchmark.ID)
		}
		byID[benchmark.ID] = benchmark
	}
	var artifacts canonicalManifest
	artifactPath := filepath.Join(root, "examples", "SDSL-V", "M36a", "artifacts", "manifest.json")
	if err := readJSON(artifactPath, &artifacts); err != nil {
		return err
	}
	for _, artifact := range artifacts.Artifacts {
		if _, ok := byID[artifact.BenchmarkID]; !ok {
			return fmt.Errorf("canonical benchmark artifact %s references unknown benchmark id %s", artifact.Name, artifact.BenchmarkID)
		}
	}
	return nil
}

func headerModuleHash(header []byte) (string, error) {
	matches := wordRE.FindAllSubmatch(header, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("contains no SPIR-V words")
	}
	words := make([]byte, len(matches)*4)
	for i, match := range matches {
		var value uint32
		if _, err := fmt.Sscanf(string(match[1]), "%x", &value); err != nil {
			return "", err
		}
		binary.LittleEndian.PutUint32(words[i*4:], value)
	}
	sum := sha256.Sum256(words)
	return hex.EncodeToString(sum[:]), nil
}

func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func mustExist(root, path string) error {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		return fmt.Errorf("required workspace path missing: %s", path)
	}
	return nil
}

func fileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sdslv workspace check:", err)
	os.Exit(1)
}
