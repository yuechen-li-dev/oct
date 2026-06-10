package pkgmgr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWrapperBuildTargetsCurrentPackageOnly(t *testing.T) {
	t.Setenv("OCT_PKG_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	manifest := strings.Join([]string{
		"package Manifest",
		"",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Kind: String",
		"    Dependencies: Dependency[]",
		"    Wrappers: Wrapper[]",
		"}",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"    Source: String",
		"}",
		"record Wrapper {",
		"    Name: String",
		"    Family: String",
		"    Protocol: String",
		"    SidecarCommand: String",
		"    GoModuleDir: String",
		"    Functions: WrapperFunction[]",
		"}",
		"record WrapperFunction {",
		"    OctName: String",
		"    WireName: String",
		"    Args: String[]",
		"    Return: String",
		"    Fallible: Bool",
		"}",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"LocalWrapper\"",
		"        Version: \"0.1.0\"",
		"        Description: \"local wrapper\"",
		"        Kind: \"wrapper\"",
		"        Dependencies: [Dependency { Name: \"DepWrapper\" VersionRequirement: \"0.1.0\" Source: \"https://example.invalid/dep.git\" }]",
		"        Wrappers: [Wrapper { Name: \"local\" Family: \"Local\" Protocol: \"octxiliary.v0\" SidecarCommand: \"octxiliary-local\" GoModuleDir: \"sidecar\" Functions: [WrapperFunction { OctName: \"Ping\" WireName: \"Ping\" Args: [] Return: \"Void\" Fallible: false }] }]",
		"    }",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manager, err := NewManager()
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	targets, err := manager.BuildWrapperBuildTargetsForProject(root)
	if err != nil {
		t.Fatalf("build targets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected one current-package target, got %#v", targets)
	}
	target := targets[0]
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if target.PackageName != "LocalWrapper" || target.PackageVersion != "0.1.0" || target.WrapperName != "local" || target.FunctionCount != 1 {
		t.Fatalf("unexpected target metadata: %#v", target)
	}
	if target.SourceDir != filepath.Join(root, "sidecar") {
		t.Fatalf("unexpected source dir: %s", target.SourceDir)
	}
	wantOutput := filepath.Join(root, ".oct", "wrappers", platform, wrapperBuildTestBinaryName("octxiliary-local"))
	if target.OutputPath != wantOutput {
		t.Fatalf("unexpected output path: got %s want %s", target.OutputPath, wantOutput)
	}
	if target.Platform != platform {
		t.Fatalf("unexpected platform: %s", target.Platform)
	}
}

func wrapperBuildTestBinaryName(command string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(command), ".exe") {
		return command + ".exe"
	}
	return command
}
