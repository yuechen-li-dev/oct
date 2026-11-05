package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/prometheus/shaderpackage"
)

func TestWorkspaceCheckUsesCurrentPackageOwnership(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// This CLI runs at the workspace root, including the canonical benchmark
	// identity boundary. Exercise that real entrypoint rather than changing cwd
	// process-wide inside a test package.
	command := exec.Command("go", "run", "./tools/sdslv_workspace_check")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("workspace command: %v\n%s", err, output)
	}
}

func TestNativeProjectionRetainsIdentityAndAuthorityChecks(t *testing.T) {
	const valid = `  META(1, "fixture", PROM_SHADER_SOURCE_SDSLV, "Fixture_CS", PROM_SHADER_AUTHORITY_PRODUCTION),`
	for _, name := range []string{"valid", "missing", "duplicate", "entry", "authority", "language", "name", "malformed", "package-missing", "package-authority"} {
		t.Run(name, func(t *testing.T) {
			manifest := shaderManifest{ShaderAssets: []shaderAsset{{ID: 1, Name: "fixture", SourceLanguage: "sdslv", EntryPoint: "Fixture_CS"}}}
			catalog := shaderpackage.Manifest{Tables: shaderpackage.Tables{Kernels: []shaderpackage.Kernel{{ID: 1, Name: "fixture", Authority: "production"}}}}
			registry := valid
			switch name {
			case "missing":
				registry = ""
			case "duplicate":
				registry += "\n" + valid
			case "entry":
				registry = strings.ReplaceAll(registry, "Fixture_CS", "Wrong_CS")
			case "authority":
				registry = strings.ReplaceAll(registry, "AUTHORITY_PRODUCTION", "AUTHORITY_EXPERIMENTAL")
			case "language":
				registry = strings.ReplaceAll(registry, "SOURCE_SDSLV", "SOURCE_HLSL")
			case "name":
				registry = strings.ReplaceAll(registry, "fixture", "wrong")
			case "malformed":
				registry = "  META(broken),"
			case "package-missing":
				catalog.Tables.Kernels = nil
			case "package-authority":
				catalog.Tables.Kernels[0].Authority = "experimental"
			}
			err := checkNativeProjection([]byte(registry), manifest, catalog)
			if name == "valid" && err != nil {
				t.Fatal(err)
			}
			if name != "valid" && err == nil {
				t.Fatalf("accepted %s selector drift", name)
			}
		})
	}
}
