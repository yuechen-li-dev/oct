package shaderpackage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReductionSourceAuthorityLivesInPackageInventory(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repo, "internal", "prometheus", "native", "shaders", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory sourceManifest
	if err := json.Unmarshal(data, &inventory); err != nil {
		t.Fatal(err)
	}
	found := map[uint32]bool{}
	for _, asset := range inventory.ShaderAssets {
		if asset.ID < 16 || asset.ID > 22 {
			continue
		}
		if found[asset.ID] || asset.Authority != "production" || asset.SourceLanguage != "sdslv" ||
			!strings.HasPrefix(asset.Source, "internal/prometheus/shaders/sdslv/production/reduction/") {
			t.Fatalf("invalid reduction source authority: %+v", asset)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(asset.Source))); err != nil {
			t.Fatalf("reduction %d source: %v", asset.ID, err)
		}
		found[asset.ID] = true
	}
	if len(found) != 7 {
		t.Fatalf("reduction source inventory has %d entries, want all seven", len(found))
	}
}

func TestImportedProductionSourceGraphRejectsStaleHeaders(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	asset := sourceAsset{ID: 67, SourceLanguage: "sdslv", Source: "internal/prometheus/shaders/sdslv/production/gemma4e2b/bf16_roundtrip.sdslv", Header: "../shaders/sdslv/production/gemma4e2b/bf16_roundtrip_spirv.h"}
	provenance, err := sourceProvenance(repo, asset)
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.Dependencies) != 2 || provenance.SourceSHA256 == "" {
		t.Fatalf("incomplete imported provenance: %#v", provenance)
	}
	if err := validateSourceGraph(provenance); err != nil {
		t.Fatal(err)
	}
	corrupt := provenance
	corrupt.DependencyIdentity = strings.Repeat("0", 64)
	if err := validateSourceGraph(corrupt); err == nil {
		t.Fatal("accepted a corrupt dependency identity")
	}
	copy := t.TempDir()
	paths := []string{asset.Source, "internal/prometheus/shaders/sdslv/production/shared/Bf16.sdslv", "internal/prometheus/shaders/sdslv/production/gemma4e2b/bf16_roundtrip_spirv.h"}
	for _, path := range paths {
		bytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(copy, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, bytes, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(copy, "internal", "prometheus", "native"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := sourceProvenance(copy, asset); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(copy, filepath.FromSlash(paths[1]))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n// New dependency snapshot.\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := sourceProvenance(copy, asset); err == nil || !strings.Contains(err.Error(), "stale source graph") {
		t.Fatalf("stale imported header admission: %v", err)
	}
}
