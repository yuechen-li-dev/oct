// Package shaderpackage owns the inert Prometheus M0 shader package format.
// It deliberately has no transport, resolver, or execution hooks.
package shaderpackage

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	Schema     = "prometheus.shader-package.v1"
	PackageID  = "prometheus.core"
	RuntimeABI = 1
)

type sourceManifest struct {
	ShaderAssets    []sourceAsset          `json:"shader_assets"`
	Implementations []sourceImplementation `json:"compute_implementations"`
}

type sourceAsset struct {
	ID                        uint32   `json:"id"`
	Name                      string   `json:"name"`
	Authority                 string   `json:"authority"`
	Stage                     string   `json:"stage"`
	SourceLanguage            string   `json:"source_language"`
	Source                    string   `json:"source"`
	SourceSHA256              string   `json:"source_sha256"`
	Header                    string   `json:"header"`
	Symbol                    string   `json:"symbol"`
	EntryPoint                string   `json:"entry_point"`
	Workgroup                 []uint32 `json:"workgroup"`
	DescriptorBindings        uint32   `json:"descriptor_bindings"`
	PushConstantBytes         uint32   `json:"push_constant_bytes"`
	StageRole                 string   `json:"stage_role"`
	RowWidthEnvelope          []uint32 `json:"row_width_envelope"`
	RequiredVulkanExtensions  []string `json:"required_vulkan_extensions"`
	RequiredVulkanFeatures    []string `json:"required_vulkan_features"`
	RequiredSPIRVExtensions   []string `json:"required_spirv_extensions"`
	RequiredSPIRVCapabilities []string `json:"required_spirv_capabilities"`
}

type sourceImplementation struct {
	ID               uint32 `json:"id"`
	Name             string `json:"name"`
	Authority        string `json:"authority"`
	Operation        string `json:"operation"`
	ShaderID         uint32 `json:"shader_id"`
	Dispatch         string `json:"dispatch"`
	BenchmarkEnabled bool   `json:"benchmark_enabled"`
	SelectorEligible bool   `json:"selector_eligible"`
	Dispatchable     bool   `json:"dispatchable"`
}

// Manifest is deliberately a tree of tables. Runtime decisions stay in native code.
type Manifest struct {
	Schema  string `json:"schema"`
	Package struct {
		ID         string `json:"id"`
		Version    string `json:"version"`
		RuntimeABI uint32 `json:"runtime_abi"`
	} `json:"package"`
	Target struct {
		ObjectLayout string `json:"object_layout"`
		MediaType    string `json:"media_type"`
	} `json:"target"`
	Tables Tables `json:"tables"`
}
type Tables struct {
	Artifacts       []Artifact       `json:"artifacts"`
	Kernels         []Kernel         `json:"kernels"`
	Variants        []Variant        `json:"variants"`
	Requirements    []Requirement    `json:"requirements"`
	Implementations []Implementation `json:"implementations"`
	Provenance      []Provenance     `json:"provenance"`
}
type Artifact struct {
	Digest    string `json:"sha256"`
	ByteCount uint64 `json:"byte_count"`
	MediaType string `json:"media_type"`
}
type Kernel struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	Stage     string `json:"stage"`
	Authority string `json:"authority"`
}
type Variant struct {
	ID                 string   `json:"id"`
	KernelID           uint32   `json:"kernel_id"`
	ArtifactSHA256     string   `json:"artifact_sha256"`
	EntryPoint         string   `json:"entry_point"`
	Workgroup          []uint32 `json:"workgroup,omitempty"`
	DescriptorBindings uint32   `json:"descriptor_bindings"`
	PushConstantBytes  uint32   `json:"push_constant_bytes"`
	StageRole          string   `json:"stage_role,omitempty"`
	RowWidthEnvelope   []uint32 `json:"row_width_envelope,omitempty"`
}
type Requirement struct {
	VariantID string `json:"variant_id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}
type Implementation struct {
	ID               uint32 `json:"id"`
	Name             string `json:"name"`
	Authority        string `json:"authority"`
	Operation        string `json:"operation"`
	VariantID        string `json:"variant_id"`
	Dispatch         string `json:"dispatch"`
	BenchmarkEnabled bool   `json:"benchmark_enabled"`
	SelectorEligible bool   `json:"selector_eligible"`
	Dispatchable     bool   `json:"dispatchable"`
}
type Provenance struct {
	VariantID    string `json:"variant_id"`
	Source       string `json:"source,omitempty"`
	SourceSHA256 string `json:"source_sha256,omitempty"`
	Generator    string `json:"generator"`
}

type BuildOptions struct {
	SourceManifest string
	RepositoryRoot string
	OutputRoot     string
	IDHeader       string
}

func Build(opts BuildOptions) (Manifest, error) {
	if opts.SourceManifest == "" || opts.RepositoryRoot == "" || opts.OutputRoot == "" {
		return Manifest{}, fmt.Errorf("source manifest, repository root, and output root are required")
	}
	data, err := os.ReadFile(opts.SourceManifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("read source manifest: %w", err)
	}
	var source sourceManifest
	if err := json.Unmarshal(data, &source); err != nil {
		return Manifest{}, fmt.Errorf("parse source manifest: %w", err)
	}
	if len(source.ShaderAssets) == 0 {
		return Manifest{}, fmt.Errorf("source manifest has no shader assets")
	}
	objects := filepath.Join(opts.OutputRoot, "objects", "sha256")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create package objects: %w", err)
	}
	m := newManifest()
	byKernel := make(map[uint32]string, len(source.ShaderAssets))
	seenArtifact := map[string]bool{}
	for _, asset := range source.ShaderAssets {
		if asset.ID == 0 || asset.Name == "" || asset.Symbol == "" || asset.EntryPoint == "" {
			return Manifest{}, fmt.Errorf("invalid source asset %d", asset.ID)
		}
		bytes, err := readPayload(opts.RepositoryRoot, asset)
		if err != nil {
			return Manifest{}, err
		}
		digest := sha(bytes)
		object := filepath.Join(objects, digest+".spv")
		if !seenArtifact[digest] {
			if err := os.WriteFile(object, bytes, 0o644); err != nil {
				return Manifest{}, fmt.Errorf("write object %s: %w", digest, err)
			}
			seenArtifact[digest] = true
			m.Tables.Artifacts = append(m.Tables.Artifacts, Artifact{digest, uint64(len(bytes)), "application/vnd.khronos.spirv"})
		}
		authority := asset.Authority
		if authority == "" {
			authority = "production"
		}
		variantID := fmt.Sprintf("kernel-%d-default", asset.ID)
		byKernel[asset.ID] = variantID
		m.Tables.Kernels = append(m.Tables.Kernels, Kernel{asset.ID, asset.Name, asset.Stage, authority})
		m.Tables.Variants = append(m.Tables.Variants, Variant{variantID, asset.ID, digest, asset.EntryPoint, asset.Workgroup, asset.DescriptorBindings, asset.PushConstantBytes, asset.StageRole, asset.RowWidthEnvelope})
		m.Tables.Provenance = append(m.Tables.Provenance, Provenance{variantID, asset.Source, asset.SourceSHA256, "oct sdslv package build"})
		for _, item := range asset.RequiredVulkanExtensions {
			m.Tables.Requirements = append(m.Tables.Requirements, Requirement{variantID, "vulkan_extension", item})
		}
		for _, item := range asset.RequiredVulkanFeatures {
			m.Tables.Requirements = append(m.Tables.Requirements, Requirement{variantID, "vulkan_feature", item})
		}
		for _, item := range asset.RequiredSPIRVExtensions {
			m.Tables.Requirements = append(m.Tables.Requirements, Requirement{variantID, "spirv_extension", item})
		}
		for _, item := range asset.RequiredSPIRVCapabilities {
			m.Tables.Requirements = append(m.Tables.Requirements, Requirement{variantID, "spirv_capability", item})
		}
	}
	for _, impl := range source.Implementations {
		variantID, ok := byKernel[impl.ShaderID]
		if !ok {
			return Manifest{}, fmt.Errorf("implementation %d references missing shader %d", impl.ID, impl.ShaderID)
		}
		authority := impl.Authority
		if authority == "" {
			authority = "production"
		}
		m.Tables.Implementations = append(m.Tables.Implementations, Implementation{impl.ID, impl.Name, authority, impl.Operation, variantID, impl.Dispatch, impl.BenchmarkEnabled, impl.SelectorEligible, impl.Dispatchable})
	}
	if err := Validate(opts.OutputRoot, m); err != nil {
		return Manifest{}, err
	}
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(opts.OutputRoot, "manifest.json"), append(encoded, '\n'), 0o644); err != nil {
		return Manifest{}, fmt.Errorf("write package manifest: %w", err)
	}
	if opts.IDHeader != "" {
		if err := writeIDHeader(opts.IDHeader, m); err != nil {
			return Manifest{}, err
		}
	}
	return m, nil
}

func Check(root string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read package manifest: %w", err)
	}
	var m Manifest
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("parse strict package manifest: %w", err)
	}
	if err := Validate(root, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func Validate(root string, m Manifest) error {
	if m.Schema != Schema {
		return fmt.Errorf("unsupported shader package schema %q", m.Schema)
	}
	if m.Package.ID != PackageID || m.Package.Version != "1" {
		return fmt.Errorf("unsupported shader package identity %q@%q", m.Package.ID, m.Package.Version)
	}
	if m.Package.RuntimeABI != RuntimeABI {
		return fmt.Errorf("incompatible shader package runtime ABI %d", m.Package.RuntimeABI)
	}
	if m.Target.ObjectLayout != "objects/sha256/<sha256>.spv" || m.Target.MediaType != "application/vnd.khronos.spirv" {
		return fmt.Errorf("unsupported shader package target layout")
	}
	artifacts := map[string]Artifact{}
	payloads := map[string][]byte{}
	for _, a := range m.Tables.Artifacts {
		if !validDigest(a.Digest) || a.ByteCount == 0 || a.MediaType != m.Target.MediaType {
			return fmt.Errorf("invalid artifact %q", a.Digest)
		}
		if _, ok := artifacts[a.Digest]; ok {
			return fmt.Errorf("duplicate artifact %q", a.Digest)
		}
		artifacts[a.Digest] = a
		path := objectPath(root, a.Digest)
		bytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read artifact %s: %w", a.Digest, err)
		}
		if uint64(len(bytes)) != a.ByteCount {
			return fmt.Errorf("artifact %s byte count mismatch", a.Digest)
		}
		if sha(bytes) != a.Digest {
			return fmt.Errorf("artifact %s SHA-256 mismatch", a.Digest)
		}
		if len(bytes)%4 != 0 || len(bytes) < 4 || binary.LittleEndian.Uint32(bytes[:4]) != 0x07230203 {
			return fmt.Errorf("artifact %s is not aligned SPIR-V", a.Digest)
		}
		payloads[a.Digest] = bytes
	}
	kernels := map[uint32]Kernel{}
	for _, k := range m.Tables.Kernels {
		if k.ID == 0 || k.Name == "" || k.Stage != "compute" || (k.Authority != "production" && k.Authority != "experimental") {
			return fmt.Errorf("invalid kernel %d", k.ID)
		}
		if _, ok := kernels[k.ID]; ok {
			return fmt.Errorf("duplicate kernel %d", k.ID)
		}
		kernels[k.ID] = k
	}
	variants := map[string]Variant{}
	for _, v := range m.Tables.Variants {
		if v.ID == "" || v.EntryPoint == "" || kernels[v.KernelID].ID == 0 || artifacts[v.ArtifactSHA256].Digest == "" {
			return fmt.Errorf("invalid variant %q", v.ID)
		}
		if _, ok := variants[v.ID]; ok {
			return fmt.Errorf("duplicate variant %q", v.ID)
		}
		if len(v.Workgroup) != 0 && len(v.Workgroup) != 3 {
			return fmt.Errorf("invalid workgroup for variant %q", v.ID)
		}
		localSize, found := spirvEntryLocalSize(payloads[v.ArtifactSHA256], v.EntryPoint)
		if !found {
			return fmt.Errorf("SPIR-V entry point %q is missing for variant %q", v.EntryPoint, v.ID)
		}
		if len(v.Workgroup) == 3 && (localSize[0] != v.Workgroup[0] || localSize[1] != v.Workgroup[1] || localSize[2] != v.Workgroup[2]) {
			return fmt.Errorf("SPIR-V LocalSize mismatch for variant %q", v.ID)
		}
		variants[v.ID] = v
	}
	validRequirement := map[string]bool{"vulkan_extension": true, "vulkan_feature": true, "spirv_extension": true, "spirv_capability": true}
	for _, r := range m.Tables.Requirements {
		if variants[r.VariantID].ID == "" || !validRequirement[r.Kind] || r.Name == "" {
			return fmt.Errorf("invalid requirement for variant %q", r.VariantID)
		}
	}
	impls := map[uint32]bool{}
	for _, i := range m.Tables.Implementations {
		if i.ID == 0 || i.Name == "" || variants[i.VariantID].ID == "" || (i.Authority != "production" && i.Authority != "experimental") {
			return fmt.Errorf("invalid implementation %d", i.ID)
		}
		if impls[i.ID] {
			return fmt.Errorf("duplicate implementation %d", i.ID)
		}
		if i.Authority == "production" && kernels[variants[i.VariantID].KernelID].Authority != "production" {
			return fmt.Errorf("experimental variant leaked into production implementation %d", i.ID)
		}
		impls[i.ID] = true
	}
	return nil
}

func newManifest() Manifest {
	var m Manifest
	m.Schema = Schema
	m.Package.ID = PackageID
	m.Package.Version = "1"
	m.Package.RuntimeABI = RuntimeABI
	m.Target.ObjectLayout = "objects/sha256/<sha256>.spv"
	m.Target.MediaType = "application/vnd.khronos.spirv"
	return m
}
func sha(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func validDigest(s string) bool {
	return len(s) == 64 && s == strings.ToLower(s) && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s)
}
func objectPath(root, digest string) string {
	return filepath.Join(root, "objects", "sha256", digest+".spv")
}

// spirvEntryLocalSize reads only the two declarative facts required by the M0
// package contract. It never executes SPIR-V and deliberately rejects malformed
// instruction boundaries by returning not found.
func spirvEntryLocalSize(data []byte, entry string) ([3]uint32, bool) {
	var local [3]uint32
	if len(data) < 20 || len(data)%4 != 0 {
		return local, false
	}
	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	entryIDs := map[uint32]bool{}
	for at := 5; at < len(words); {
		wordCount := int(words[at] >> 16)
		opcode := words[at] & 0xffff
		if wordCount == 0 || at+wordCount > len(words) {
			return local, false
		}
		operands := words[at+1 : at+wordCount]
		if opcode == 15 && len(operands) >= 3 && decodeSPIRVString(operands[2:]) == entry {
			entryIDs[operands[1]] = true
		}
		at += wordCount
	}
	if len(entryIDs) == 0 {
		return local, false
	}
	for at := 5; at < len(words); {
		wordCount := int(words[at] >> 16)
		opcode := words[at] & 0xffff
		if wordCount == 0 || at+wordCount > len(words) {
			return local, false
		}
		operands := words[at+1 : at+wordCount]
		if opcode == 16 && len(operands) == 5 && operands[1] == 17 && entryIDs[operands[0]] {
			return [3]uint32{operands[2], operands[3], operands[4]}, true
		}
		at += wordCount
	}
	return local, false
}

func decodeSPIRVString(words []uint32) string {
	var out []byte
	for _, word := range words {
		for shift := uint(0); shift < 32; shift += 8 {
			b := byte(word >> shift)
			if b == 0 {
				return string(out)
			}
			out = append(out, b)
		}
	}
	return string(out)
}
func readPayload(root string, asset sourceAsset) ([]byte, error) {
	path := asset.Header
	if path == "embedded" {
		path = asset.Source
	}
	full := filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(path))
	if path == asset.Source {
		full = filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(path))
	}
	text, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read payload source for shader %d: %w", asset.ID, err)
	}
	if strings.HasSuffix(path, ".spv.base64") {
		payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(text)))
		if err != nil || len(payload) == 0 {
			return nil, fmt.Errorf("decode checked SPIR-V payload for shader %d", asset.ID)
		}
		return payload, nil
	}
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(asset.Symbol) + `\s*\[\s*\]\s*=\s*\{(.*?)\};`)
	match := pattern.FindSubmatch(text)
	if len(match) != 2 {
		return nil, fmt.Errorf("extract payload symbol %q from %s", asset.Symbol, full)
	}
	numbers := regexp.MustCompile(`0x[0-9A-Fa-f]+|[0-9]+`).FindAllString(string(match[1]), -1)
	if len(numbers) == 0 {
		return nil, fmt.Errorf("payload %q is empty", asset.Symbol)
	}
	out := make([]byte, len(numbers)*4)
	for i, n := range numbers {
		v, e := strconv.ParseUint(n, 0, 32)
		if e != nil {
			return nil, fmt.Errorf("parse payload %q: %w", asset.Symbol, e)
		}
		binary.LittleEndian.PutUint32(out[i*4:], uint32(v))
	}
	return out, nil
}
func writeIDHeader(path string, m Manifest) error {
	sort.Slice(m.Tables.Kernels, func(i, j int) bool { return m.Tables.Kernels[i].ID < m.Tables.Kernels[j].ID })
	var b strings.Builder
	b.WriteString("/* Generated by oct sdslv package build. No payloads. */\n#ifndef OCT_PROMETHEUS_REACTOR_SHADER_IDS_GENERATED_H\n#define OCT_PROMETHEUS_REACTOR_SHADER_IDS_GENERATED_H\n\n#define PROMETHEUS_SHADER_PACKAGE_RUNTIME_ABI 1u\n")
	for _, k := range m.Tables.Kernels {
		n := strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(k.Name, "_"))
		b.WriteString(fmt.Sprintf("#define PROMETHEUS_SHADER_KERNEL_%s %du\n", n, k.ID))
	}
	b.WriteString("\n#endif\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
