package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/toolchain"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const Schema = "sdslv.conformance.v1"

type Manifest struct {
	Schema    string           `json:"schema"`
	Authority string           `json:"authority"`
	Tiers     []string         `json:"tiers"`
	Valid     []ValidFixture   `json:"valid"`
	Invalid   []InvalidFixture `json:"invalid"`
}

type ValidFixture struct {
	ID                   string              `json:"id"`
	Source               string              `json:"source"`
	Profile              string              `json:"profile"`
	EntryPoints          []string            `json:"entry_points"`
	Stages               []string            `json:"stages"`
	StreamRoles          map[string]string   `json:"stream_roles"`
	CoordinateSpaces     []string            `json:"coordinate_spaces"`
	Bindings             []string            `json:"bindings"`
	Locations            []string            `json:"locations"`
	Builtins             []string            `json:"builtins"`
	RequiredCapabilities []string            `json:"required_capabilities"`
	Bundle               string              `json:"bundle,omitempty"`
	ReferenceArtifacts   []ReferenceArtifact `json:"reference_artifacts"`
}

type ReferenceArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type InvalidFixture struct {
	Source        string `json:"source"`
	Code          string `json:"code"`
	Line          uint32 `json:"line"`
	Column        uint32 `json:"column"`
	RelatedLine   uint32 `json:"related_line,omitempty"`
	RelatedColumn uint32 `json:"related_column,omitempty"`
	Category      string `json:"category"`
}

var syntaxLocationRE = regexp.MustCompile(`\bat ([0-9]+):([0-9]+)(?:\s|$)`)
var diagnosticCodeRE = regexp.MustCompile(`SDSL-V[0-9]+`)

func Load(root string) (Manifest, error) {
	path := filepath.Join(root, "examples", "SDSL-V", "conformance", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return manifest, nil
}

func Verify(root string) error {
	manifest, err := Load(root)
	if err != nil {
		return err
	}
	if manifest.Schema != Schema || strings.TrimSpace(manifest.Authority) == "" || len(manifest.Tiers) != 6 {
		return fmt.Errorf("incomplete conformance manifest identity")
	}
	if len(manifest.Valid) == 0 || len(manifest.Invalid) == 0 {
		return fmt.Errorf("conformance manifest must contain valid and invalid fixtures")
	}
	seenIDs, seenEntries := map[string]bool{}, map[string]string{}
	for _, fixture := range manifest.Valid {
		if fixture.ID == "" || seenIDs[fixture.ID] {
			return fmt.Errorf("empty or duplicate conformance fixture id %q", fixture.ID)
		}
		seenIDs[fixture.ID] = true
		path := filepath.Join(root, filepath.FromSlash(fixture.Source))
		if err := sdslv.CheckFile(path); err != nil {
			return fmt.Errorf("valid conformance fixture %s: %w", fixture.ID, err)
		}
		module, err := sdslv.LowerFile(path)
		if err != nil {
			return fmt.Errorf("lower conformance fixture %s: %w", fixture.ID, err)
		}
		actualEntries, actualStages := moduleEntryFacts(module)
		if !equalStrings(actualEntries, fixture.EntryPoints) || !equalStrings(actualStages, fixture.Stages) {
			return fmt.Errorf("conformance fixture %s entry facts = %v/%v, want %v/%v", fixture.ID, actualEntries, actualStages, fixture.EntryPoints, fixture.Stages)
		}
		spaces := make([]string, 0)
		for _, alias := range module.TypeAliases {
			if alias.Target.Space != "" {
				spaces = append(spaces, alias.Target.Space)
			}
		}
		if !equalStrings(spaces, fixture.CoordinateSpaces) {
			return fmt.Errorf("conformance fixture %s coordinate spaces = %v, want %v", fixture.ID, spaces, fixture.CoordinateSpaces)
		}
		capabilities := make([]string, 0, len(module.Requirements))
		for _, requirement := range module.Requirements {
			capabilities = append(capabilities, requirement.Kind)
		}
		if !equalStrings(capabilities, fixture.RequiredCapabilities) {
			return fmt.Errorf("conformance fixture %s capabilities = %v, want %v", fixture.ID, capabilities, fixture.RequiredCapabilities)
		}
		for _, entry := range fixture.EntryPoints {
			if prior, exists := seenEntries[entry]; exists {
				return fmt.Errorf("duplicate conformance entry identity %s in %s and %s", entry, prior, fixture.ID)
			}
			seenEntries[entry] = fixture.ID
		}
		roles := map[string]string{}
		for _, stream := range module.Streams {
			roles[stream.Name] = string(stream.Role)
		}
		for name, want := range fixture.StreamRoles {
			if roles[name] != want {
				return fmt.Errorf("conformance fixture %s stream %s role = %q, want %q", fixture.ID, name, roles[name], want)
			}
		}
		actualBindings := make([]string, 0, len(module.Resources)+len(module.Materials))
		for _, resource := range module.Resources {
			actualBindings = append(actualBindings, fmt.Sprintf("%s:set%d/binding%d", resource.Name, resource.Binding.Set, resource.Binding.Binding))
		}
		if len(module.Materials) > 0 {
			material := module.Materials[0]
			actualBindings = append(actualBindings, fmt.Sprintf("Material:set%d/binding%d", material.Binding.Set, material.Binding.Binding))
		}
		if !bindingPrefixesEqual(actualBindings, fixture.Bindings) {
			return fmt.Errorf("conformance fixture %s ordered bindings = %v, want %v", fixture.ID, actualBindings, fixture.Bindings)
		}
		for _, artifact := range fixture.ReferenceArtifacts {
			if got := fileHash(filepath.Join(root, filepath.FromSlash(artifact.Path))); got != strings.ToLower(artifact.SHA256) {
				return fmt.Errorf("conformance artifact %s hash = %s, want %s", artifact.Path, got, artifact.SHA256)
			}
		}
		if fixture.Bundle != "" {
			if err := verifyBundle(root, filepath.Join(root, filepath.FromSlash(fixture.Bundle)), fixture); err != nil {
				return fmt.Errorf("conformance bundle %s: %w", fixture.ID, err)
			}
		}
	}
	for _, fixture := range manifest.Invalid {
		if err := verifyInvalid(root, fixture); err != nil {
			return err
		}
	}
	return nil
}

func moduleEntryFacts(module vdmir.Module) ([]string, []string) {
	entries := make([]string, 0, len(module.EntryPoints)+len(module.GraphicsEntryPoints))
	stages := make([]string, 0, len(module.EntryPoints)+len(module.GraphicsEntryPoints))
	for _, entry := range module.EntryPoints {
		entries = append(entries, entry.EmittedName)
		stages = append(stages, string(vdmir.StageCompute))
	}
	for _, entry := range module.GraphicsEntryPoints {
		entries = append(entries, entry.EmittedName)
		stages = append(stages, string(entry.Stage))
	}
	return entries, stages
}

func verifyInvalid(root string, fixture InvalidFixture) error {
	path := filepath.Join(root, filepath.FromSlash(fixture.Source))
	file, err := source.Load(path)
	if err != nil {
		return err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return verifySyntaxError(fixture, err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return verifySyntaxError(fixture, err)
	}
	for _, actual := range validate.Diagnostics(module) {
		if actual.Code != fixture.Code {
			continue
		}
		if actual.Span.Start.Line != fixture.Line || actual.Span.Start.Column != fixture.Column {
			return fmt.Errorf("invalid conformance fixture %s diagnostic %s at %d:%d, want %d:%d", fixture.Source, fixture.Code, actual.Span.Start.Line, actual.Span.Start.Column, fixture.Line, fixture.Column)
		}
		if fixture.RelatedLine != 0 {
			if len(actual.Related) == 0 || actual.Related[0].Span.Start.Line != fixture.RelatedLine || actual.Related[0].Span.Start.Column != fixture.RelatedColumn {
				return fmt.Errorf("invalid conformance fixture %s related span mismatch", fixture.Source)
			}
		}
		return nil
	}
	return fmt.Errorf("invalid conformance fixture %s did not produce %s", fixture.Source, fixture.Code)
}

func verifySyntaxError(fixture InvalidFixture, err error) error {
	text := err.Error()
	code := diagnosticCodeRE.FindString(text)
	location := syntaxLocationRE.FindStringSubmatch(text)
	if code != fixture.Code || len(location) != 3 {
		return fmt.Errorf("invalid conformance fixture %s syntax diagnostic = %q", fixture.Source, text)
	}
	var line, column uint32
	if _, scanErr := fmt.Sscanf(location[0], "at %d:%d", &line, &column); scanErr != nil {
		return scanErr
	}
	if line != fixture.Line || column != fixture.Column {
		return fmt.Errorf("invalid conformance fixture %s diagnostic %s at %d:%d, want %d:%d", fixture.Source, code, line, column, fixture.Line, fixture.Column)
	}
	return nil
}

func verifyBundle(repositoryRoot, path string, fixture ValidFixture) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var bundle toolchain.GraphicsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return err
	}
	if bundle.Schema != "sdslv.graphics-program-bundle.v1" || bundle.Program == "" || bundle.ReplayIdentity == "" || !bundle.Vertex.SPIRVValidated || !bundle.Pixel.SPIRVValidated {
		return fmt.Errorf("graphics bundle identity or validation evidence is incomplete")
	}
	if filepath.Clean(filepath.FromSlash(bundle.Source.Path)) != filepath.Clean(filepath.FromSlash(fixture.Source)) {
		return fmt.Errorf("graphics bundle source = %s, want %s", bundle.Source.Path, fixture.Source)
	}
	if got := fileHash(filepath.Join(repositoryRoot, filepath.FromSlash(bundle.Source.Path))); got != bundle.Source.SHA256 {
		return fmt.Errorf("graphics bundle source hash = %s, want %s", got, bundle.Source.SHA256)
	}
	if bundle.Compiler.Name == "" || bundle.Compiler.SHA256 == "" || bundle.Compiler.SHA256 == "unreadable" {
		return fmt.Errorf("graphics bundle compiler provenance is incomplete")
	}
	if !equalStrings([]string{bundle.Vertex.EntryPoint, bundle.Pixel.EntryPoint}, fixture.EntryPoints) || !equalStrings([]string{bundle.Vertex.Stage, bundle.Pixel.Stage}, fixture.Stages) {
		return fmt.Errorf("graphics bundle stage entries disagree with conformance manifest")
	}
	if bundle.Vertex.Profile != "vs_6_0" || bundle.Pixel.Profile != "ps_6_0" || bundle.Vertex.TargetEnvironment != bundle.Pixel.TargetEnvironment {
		return fmt.Errorf("graphics bundle profile or target policy is invalid")
	}
	if !reflect.DeepEqual(bundle.Interface.Varyings, bundle.Interface.PixelInputs) {
		return fmt.Errorf("graphics bundle vertex/pixel interface parity failed")
	}
	if !equalStrings(bundle.RequiredCapabilities, fixture.RequiredCapabilities) {
		return fmt.Errorf("graphics bundle capabilities = %v, want %v", bundle.RequiredCapabilities, fixture.RequiredCapabilities)
	}
	identity := bundle.ReplayIdentity
	bundle.ReplayIdentity = ""
	bundle.ManifestPath = ""
	projection, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	if got := hashBytes(projection); got != identity {
		return fmt.Errorf("graphics bundle replay identity = %s, want %s", identity, got)
	}
	root := filepath.Dir(path)
	for _, artifact := range []struct {
		path, hash string
		stage      toolchain.BundleStageArtifact
	}{
		{bundle.Vertex.HLSLPath, bundle.Vertex.HLSLSHA256, toolchain.BundleStageArtifact{}},
		{bundle.Vertex.SPIRVPath, bundle.Vertex.SPIRVSHA256, bundle.Vertex},
		{bundle.Pixel.HLSLPath, bundle.Pixel.HLSLSHA256, toolchain.BundleStageArtifact{}},
		{bundle.Pixel.SPIRVPath, bundle.Pixel.SPIRVSHA256, bundle.Pixel},
	} {
		artifactPath := filepath.Join(root, filepath.FromSlash(artifact.path))
		if got := fileHash(artifactPath); got != artifact.hash {
			return fmt.Errorf("bundle artifact %s hash = %s, want %s", artifact.path, got, artifact.hash)
		}
		if artifact.stage.Stage != "" {
			facts, err := toolchain.InspectGraphicsSPIRV(artifactPath, artifact.stage.EntryPoint, artifact.stage.Stage)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(facts, artifact.stage.StructuralFacts) {
				return fmt.Errorf("bundle artifact %s structural SPIR-V facts changed", artifact.path)
			}
		}
	}
	return nil
}

func equalStrings(actual, expected []string) bool {
	a, b := append([]string(nil), actual...), append([]string(nil), expected...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}

func bindingPrefixesEqual(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if expected[i] != actual[i] && !strings.HasPrefix(expected[i], actual[i]+":") {
			return false
		}
	}
	return true
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
