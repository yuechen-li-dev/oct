package shaderpackage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/modules"
)

func sourceProvenance(root string, asset sourceAsset) (Provenance, error) {
	provenance := Provenance{Source: asset.Source, SourceSHA256: asset.SourceSHA256, Generator: "oct sdslv package build"}
	if asset.SourceLanguage != "sdslv" {
		return provenance, nil
	}
	module, err := modules.Load(filepath.Join(root, filepath.FromSlash(asset.Source)))
	if err != nil {
		return Provenance{}, fmt.Errorf("shader %d source graph: %w", asset.ID, err)
	}
	if module.DependencyIdentity == "" {
		return provenance, nil
	}
	header, err := os.ReadFile(filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(asset.Header)))
	if err != nil {
		return Provenance{}, err
	}
	want := "// Source graph SHA-256: " + module.DependencyIdentity
	matched := false
	for _, line := range strings.Split(string(header), "\n") {
		if strings.TrimSpace(line) == want {
			matched = true
			break
		}
	}
	if !matched {
		return Provenance{}, fmt.Errorf("shader %d header has a stale source graph; regenerate %s", asset.ID, asset.Header)
	}
	entry, err := os.ReadFile(module.Source.Path)
	if err != nil {
		return Provenance{}, err
	}
	provenance.SourceSHA256 = sha(entry)
	provenance.DependencyIdentity = module.DependencyIdentity
	for _, input := range module.Dependencies {
		provenance.Dependencies = append(provenance.Dependencies, SourceDependency{input.Path, input.Namespace, input.SHA256})
	}
	return provenance, nil
}

func validateSourceGraph(provenance Provenance) error {
	if provenance.DependencyIdentity == "" && len(provenance.Dependencies) == 0 {
		return nil
	}
	if !validDigest(provenance.DependencyIdentity) || len(provenance.Dependencies) == 0 {
		return fmt.Errorf("invalid source graph for %s", provenance.VariantID)
	}
	previous := ""
	for _, input := range provenance.Dependencies {
		if input.Path == "" || !validDigest(input.SHA256) || (previous != "" && input.Path <= previous) {
			return fmt.Errorf("invalid source graph input for %s", provenance.VariantID)
		}
		previous = input.Path
	}
	encoded, err := json.Marshal(provenance.Dependencies)
	if err != nil {
		return err
	}
	if sha(encoded) != provenance.DependencyIdentity {
		return fmt.Errorf("source graph identity mismatch for %s", provenance.VariantID)
	}
	return nil
}
