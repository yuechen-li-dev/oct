package pkgmgr

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalRegistryMatchesFirstPartyManifests(t *testing.T) {
	root := canonicalRegistryRepoRoot(t)
	registryRoot := filepath.Join(root, "Registry")
	idx, err := LoadRegistryIndex(registryRoot)
	if err != nil {
		t.Fatalf("load canonical registry: %v", err)
	}
	if len(idx.Packages) == 0 {
		t.Fatalf("canonical registry must contain at least one package")
	}

	seen := map[string]bool{}
	foundMathematics := false
	for _, entry := range idx.Packages {
		key := entry.Name + "@" + entry.Version
		if seen[key] {
			t.Fatalf("duplicate canonical registry entry %s", key)
		}
		seen[key] = true
		if entry.Name == "Math" {
			t.Fatalf("canonical registry must not define Math alias")
		}
		if entry.Name == "Mathematics" {
			foundMathematics = true
		}
		if entry.SourceKind != "local" {
			t.Fatalf("canonical registry entry %s has non-local SourceKind %q", key, entry.SourceKind)
		}
		if entry.Source != ".." {
			t.Fatalf("canonical registry entry %s has Source %q, want ..", key, entry.Source)
		}
		packageRoot, err := safePackageSourcePath(root, entry.Path)
		if err != nil {
			t.Fatalf("canonical registry entry %s path: %v", key, err)
		}
		manifest, err := LoadManifestMetadata(filepath.Join(packageRoot, manifestFileName))
		if err != nil {
			t.Fatalf("canonical registry entry %s manifest: %v", key, err)
		}
		if manifest.Name != entry.Name {
			t.Fatalf("canonical registry entry %s manifest name mismatch: got %q", key, manifest.Name)
		}
		if manifest.Version != entry.Version {
			t.Fatalf("canonical registry entry %s manifest version mismatch: got %q", key, manifest.Version)
		}
		if registryKindForManifestKind(manifest.Kind) != entry.Kind {
			t.Fatalf("canonical registry entry %s kind mismatch: manifest %q maps to %q, registry has %q", key, manifest.Kind, registryKindForManifestKind(manifest.Kind), entry.Kind)
		}
	}
	if !foundMathematics {
		t.Fatalf("canonical registry must include Mathematics when registry-ready")
	}
}

func registryKindForManifestKind(kind string) string {
	switch kind {
	case "", "pure":
		return "library"
	case "wrapper":
		return "wrapper"
	case "experiment":
		return "experiment"
	default:
		return kind
	}
}

func canonicalRegistryRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
