package makecmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverySpec declares an action-local compiler discovery protocol.  The
// runtime output path is deliberately absent: it is allocated for one attempt
// by the executor and therefore must never affect the action identity.
type DiscoverySpec struct {
	Kind                           string
	SchemaVersion                  string
	OutputArgument                 RuntimeOutputArgument
	ExpectedSourceIdentity         string
	ExpectedPhysicalOutputIdentity string
}

// RuntimeOutputArgument describes where the executor inserts its ephemeral
// output path.  Prefix supports tools that require a joined flag/value while
// Position supports tools, such as cl.exe, with a separate value argument.
type RuntimeOutputArgument struct {
	Position int
	Prefix   string
}

type DiscoveryProvenance struct {
	Collector string
	Detail    string
}

type DiscoveredInputs struct {
	Paths      []string
	Provenance DiscoveryProvenance
}

type discoveryCollector interface {
	Supports(DiscoverySpec) bool
	Collect(DiscoverySpec, string) (DiscoveredInputs, error)
}

type discoveryCollectors map[string]discoveryCollector

func defaultDiscoveryCollectors() discoveryCollectors {
	return discoveryCollectors{MSVCSourceDependenciesKind: msvcSourceDependenciesCollector{}}
}

func (cs discoveryCollectors) collectorFor(spec DiscoverySpec) (discoveryCollector, error) {
	c, ok := cs[spec.Kind]
	if !ok || !c.Supports(spec) {
		return nil, fmt.Errorf("unsupported discovery kind/schema %q/%q", spec.Kind, spec.SchemaVersion)
	}
	return c, nil
}

func canonicalIdentity(root, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func sameIdentity(a, b string) bool {
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func canonicalDiscoveredInputs(root string, paths []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			return nil, fmt.Errorf("discovery returned an empty dependency path")
		}
		identity := canonicalIdentity(root, path)
		info, err := os.Stat(filepath.FromSlash(identity))
		if err != nil {
			return nil, fmt.Errorf("validate discovered dependency %q: %w", identity, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("validate discovered dependency %q: directory is not an input", identity)
		}
		if !seen[identity] {
			seen[identity] = true
			out = append(out, identity)
		}
	}
	sort.Strings(out)
	return out, nil
}

func discoverySpecMatchesAction(root string, c CommandTarget, spec DiscoverySpec) error {
	if spec.Kind == "" || spec.SchemaVersion == "" {
		return fmt.Errorf("discovery kind and schema version must be non-empty")
	}
	if spec.OutputArgument.Position < 0 || spec.OutputArgument.Position > len(c.Args) {
		return fmt.Errorf("discovery output argument position %d is outside command arguments", spec.OutputArgument.Position)
	}
	if spec.ExpectedSourceIdentity == "" || spec.ExpectedPhysicalOutputIdentity == "" {
		return fmt.Errorf("discovery source and physical output identities must be non-empty")
	}
	expectedSource := canonicalIdentity(root, spec.ExpectedSourceIdentity)
	expectedOutput := canonicalIdentity(root, spec.ExpectedPhysicalOutputIdentity)
	var sourceMatches, outputMatches bool
	for _, input := range c.Inputs {
		if sameIdentity(canonicalIdentity(root, input), expectedSource) {
			sourceMatches = true
		}
	}
	for _, output := range c.Outputs {
		if sameIdentity(canonicalIdentity(root, output), expectedOutput) {
			outputMatches = true
		}
	}
	if !sourceMatches || !outputMatches {
		return fmt.Errorf("discovery source/output identity does not belong to command action")
	}
	return nil
}

const (
	MSVCSourceDependenciesKind     = "msvc.sourceDependencies"
	MSVCSourceDependenciesSchemaV1 = "v1"
)

// msvcSourceDependenciesCollector is the only component that knows the JSON
// emitted by cl.exe.  The executor sees only validated, canonical paths.
type msvcSourceDependenciesCollector struct{}

func (msvcSourceDependenciesCollector) Supports(spec DiscoverySpec) bool {
	return spec.Kind == MSVCSourceDependenciesKind && spec.SchemaVersion == MSVCSourceDependenciesSchemaV1
}

func (msvcSourceDependenciesCollector) Collect(spec DiscoverySpec, outputPath string) (DiscoveredInputs, error) {
	body, err := os.ReadFile(outputPath)
	if err != nil {
		return DiscoveredInputs{}, fmt.Errorf("read MSVC sourceDependencies output: %w", err)
	}
	var document struct {
		Version string `json:"Version"`
		Data    struct {
			Source   string   `json:"Source"`
			Includes []string `json:"Includes"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return DiscoveredInputs{}, fmt.Errorf("parse MSVC sourceDependencies output: %w", err)
	}
	if document.Version == "" {
		return DiscoveredInputs{}, fmt.Errorf("MSVC sourceDependencies output has no Version")
	}
	if document.Data.Source == "" {
		return DiscoveredInputs{}, fmt.Errorf("MSVC sourceDependencies output has no Data.Source")
	}
	if !sameIdentity(document.Data.Source, spec.ExpectedSourceIdentity) {
		return DiscoveredInputs{}, fmt.Errorf("MSVC sourceDependencies source %q does not match expected %q", document.Data.Source, spec.ExpectedSourceIdentity)
	}
	return DiscoveredInputs{
		Paths: document.Data.Includes,
		Provenance: DiscoveryProvenance{
			Collector: MSVCSourceDependenciesKind,
			Detail:    "compiler schema " + document.Version,
		},
	}, nil
}
