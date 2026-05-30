package pkgmgr

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildWrapperPlanForManifestPurePackageHasNoSidecars(t *testing.T) {
	metadata, err := loadManifestMetadata(writeManifest(t, validManifestWithEdits("    Dependencies: Dependency[]", "}", "        Dependencies: []")))
	if err != nil {
		t.Fatalf("load pure manifest: %v", err)
	}

	plan, err := BuildWrapperPlanForManifest(t.TempDir(), metadata)
	if err != nil {
		t.Fatalf("build wrapper plan: %v", err)
	}
	if len(plan.Packages) != 0 {
		t.Fatalf("expected no wrapper packages, got %#v", plan.Packages)
	}
	if len(plan.Sidecars) != 0 {
		t.Fatalf("expected no sidecars, got %#v", plan.Sidecars)
	}
	if plan.HasNativeWrappers || plan.RequiresNativeBuildPermission {
		t.Fatalf("expected no native wrapper flags, got HasNativeWrappers=%v RequiresNativeBuildPermission=%v", plan.HasNativeWrappers, plan.RequiresNativeBuildPermission)
	}
}

func TestBuildWrapperPlanForManifestWrapperPackage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "pkg")
	metadata := testWrapperManifestMetadata("Xlsx", "0.1.0", testWrapperMetadata("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary"))

	plan, err := BuildWrapperPlanForManifest(root, metadata)
	if err != nil {
		t.Fatalf("build wrapper plan: %v", err)
	}
	if plan.Root != root {
		t.Fatalf("Root = %q, want %q", plan.Root, root)
	}
	if !plan.HasNativeWrappers || !plan.RequiresNativeBuildPermission {
		t.Fatalf("expected native wrapper flags, got HasNativeWrappers=%v RequiresNativeBuildPermission=%v", plan.HasNativeWrappers, plan.RequiresNativeBuildPermission)
	}
	wantPackage := WrapperPackagePlan{PackageName: "Xlsx", Version: "0.1.0", CachePath: root, Kind: "wrapper", WrapperCount: 1}
	if !reflect.DeepEqual(plan.Packages, []WrapperPackagePlan{wantPackage}) {
		t.Fatalf("unexpected packages:\ngot  %#v\nwant %#v", plan.Packages, []WrapperPackagePlan{wantPackage})
	}
	wantFunctions := []WrapperFunctionMetadata{
		{OctName: "ReadSheetNames", WireName: "XlsxReadSheetNames", Args: []string{"String"}, Return: "String[]", Fallible: true},
		{OctName: "WriteBytes", WireName: "XlsxWriteBytes", Args: []string{"String", "Bytes"}, Return: "Int", Fallible: false},
	}
	wantSidecar := WrapperSidecarPlan{
		PackageName:    "Xlsx",
		WrapperName:    "xlsx",
		Family:         "Xlsx",
		Protocol:       "octxiliary.v0",
		SidecarCommand: "octxiliary-xlsx",
		GoModuleDir:    "octxiliary",
		GoModulePath:   filepath.Join(root, "octxiliary"),
		Functions:      wantFunctions,
	}
	if !reflect.DeepEqual(plan.Sidecars, []WrapperSidecarPlan{wantSidecar}) {
		t.Fatalf("unexpected sidecars:\ngot  %#v\nwant %#v", plan.Sidecars, []WrapperSidecarPlan{wantSidecar})
	}
}

func TestBuildWrapperPlanForManifestSortsSidecarsDeterministically(t *testing.T) {
	root := t.TempDir()
	metadata := testWrapperManifestMetadata("Adapters", "0.2.0",
		testWrapperMetadata("zeta", "Zeta", "octxiliary-zeta", "wrappers/zeta"),
		testWrapperMetadata("alpha", "Alpha", "octxiliary-alpha", "wrappers/alpha"),
	)

	plan, err := BuildWrapperPlanForManifest(root, metadata)
	if err != nil {
		t.Fatalf("build wrapper plan: %v", err)
	}
	got := []string{plan.Sidecars[0].WrapperName, plan.Sidecars[1].WrapperName}
	want := []string{"alpha", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sidecar order = %#v, want %#v", got, want)
	}
}

func TestBuildWrapperPlanForSyncResultIncludesOnlyWrapperDependencies(t *testing.T) {
	root := t.TempDir()
	wrapperPath := filepath.Join(root, "cache", "wrapper")
	purePath := filepath.Join(root, "cache", "pure")
	experimentPath := filepath.Join(root, "cache", "experiment")
	result := SyncResult{
		ProjectPath: root,
		Dependencies: []SyncDependencyResult{
			{Source: "file:///pure", GetResult: GetResult{Path: purePath, Source: "file:///pure", Manifest: ManifestMetadata{Name: "PureDep", Version: "1.0.0", Kind: "pure"}}},
			{Source: "file:///wrapper", GetResult: GetResult{Path: wrapperPath, Source: "file:///wrapper", Manifest: testWrapperManifestMetadata("Xlsx", "0.1.0", testWrapperMetadata("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary"))}},
			{Source: "file:///experiment", GetResult: GetResult{Path: experimentPath, Source: "file:///experiment", Manifest: ManifestMetadata{Name: "ExperimentDep", Version: "0.1.0", Kind: "experiment"}}},
		},
	}

	plan, err := BuildWrapperPlanForSyncResult(result)
	if err != nil {
		t.Fatalf("build sync wrapper plan: %v", err)
	}
	if plan.Root != root {
		t.Fatalf("Root = %q, want %q", plan.Root, root)
	}
	if len(plan.Packages) != 1 || plan.Packages[0].PackageName != "Xlsx" || plan.Packages[0].Source != "file:///wrapper" || plan.Packages[0].CachePath != wrapperPath {
		t.Fatalf("unexpected packages: %#v", plan.Packages)
	}
	if len(plan.Sidecars) != 1 || plan.Sidecars[0].GoModulePath != filepath.Join(wrapperPath, "octxiliary") {
		t.Fatalf("unexpected sidecars: %#v", plan.Sidecars)
	}
}

func TestBuildWrapperPlanDetectsConflicts(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name     string
		metadata ManifestMetadata
		want     string
	}{
		{
			name: "duplicate sidecar command",
			metadata: testWrapperManifestMetadata("Adapters", "0.1.0",
				testWrapperMetadata("xlsx", "Xlsx", "octxiliary-shared", "wrappers/xlsx"),
				testWrapperMetadata("csv", "Csv", "octxiliary-shared", "wrappers/csv"),
			),
			want: "duplicate sidecar command",
		},
		{
			name: "duplicate family",
			metadata: testWrapperManifestMetadata("Adapters", "0.1.0",
				testWrapperMetadata("xlsx", "Documents", "octxiliary-xlsx", "wrappers/xlsx"),
				testWrapperMetadata("csv", "Documents", "octxiliary-csv", "wrappers/csv"),
			),
			want: "duplicate wrapper family",
		},
		{
			name: "duplicate module path",
			metadata: testWrapperManifestMetadata("Adapters", "0.1.0",
				testWrapperMetadata("xlsx", "Xlsx", "octxiliary-xlsx", "wrappers/shared"),
				testWrapperMetadata("csv", "Csv", "octxiliary-csv", "wrappers/shared"),
			),
			want: "duplicate GoModulePath",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildWrapperPlanForManifest(root, tt.metadata)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestBuildWrapperPlanResolvesGoModulePathUnderPackagePath(t *testing.T) {
	root := t.TempDir()
	metadata := testWrapperManifestMetadata("Adapters", "0.1.0",
		testWrapperMetadata("csv", "Csv", "octxiliary-csv", "wrappers/csv"),
		testWrapperMetadata("xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary"),
	)

	plan, err := BuildWrapperPlanForManifest(root, metadata)
	if err != nil {
		t.Fatalf("build wrapper plan: %v", err)
	}
	got := map[string]string{}
	for _, sidecar := range plan.Sidecars {
		got[sidecar.WrapperName] = sidecar.GoModulePath
	}
	want := map[string]string{
		"csv":  filepath.Join(root, "wrappers", "csv"),
		"xlsx": filepath.Join(root, "octxiliary"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GoModulePath map = %#v, want %#v", got, want)
	}
}

func testWrapperManifestMetadata(name string, version string, wrappers ...WrapperMetadata) ManifestMetadata {
	return ManifestMetadata{Name: name, Version: version, Kind: "wrapper", Wrappers: wrappers}
}

func testWrapperMetadata(name string, family string, command string, moduleDir string) WrapperMetadata {
	return WrapperMetadata{
		Name:           name,
		Family:         family,
		Protocol:       "octxiliary.v0",
		SidecarCommand: command,
		GoModuleDir:    moduleDir,
		Functions: []WrapperFunctionMetadata{
			{OctName: "ReadSheetNames", WireName: "XlsxReadSheetNames", Args: []string{"String"}, Return: "String[]", Fallible: true},
			{OctName: "WriteBytes", WireName: "XlsxWriteBytes", Args: []string{"String", "Bytes"}, Return: "Int", Fallible: false},
		},
	}
}

func TestBuildWrapperPlanForProjectIncludesCurrentRoot(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.oct")
	if err := os.WriteFile(manifestPath, []byte(validWrapperManifestSource("Xlsx")), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	manager := &Manager{cacheDir: filepath.Join(t.TempDir(), "cache")}

	plan, err := manager.BuildWrapperPlanForProject(root)
	if err != nil {
		t.Fatalf("build project wrapper plan: %v", err)
	}
	if plan.Root != root {
		t.Fatalf("Root = %q, want %q", plan.Root, root)
	}
	if len(plan.Sidecars) != 1 || plan.Sidecars[0].PackageName != "Xlsx" || plan.Sidecars[0].GoModulePath != filepath.Join(root, "octxiliary") {
		t.Fatalf("unexpected sidecars: %#v", plan.Sidecars)
	}
}

func TestBuildWrapperPlanForProjectIncludesSyncedDependencies(t *testing.T) {
	requireGitForPkgMgr(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	wrapperSource := createPkgMgrGitRepoWithManifest(t, validWrapperManifestSource("Xlsx"))
	root := t.TempDir()
	manifest := projectManifestSourceForPkgMgr([]string{`Dependency { Name: "Xlsx" VersionRequirement: "^0.1.0" Source: "` + wrapperSource + `" }`})
	if err := os.WriteFile(filepath.Join(root, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write root manifest: %v", err)
	}
	manager := &Manager{cacheDir: cacheDir}

	plan, err := manager.BuildWrapperPlanForProject(root)
	if err != nil {
		t.Fatalf("build project wrapper plan: %v", err)
	}
	if len(plan.Sidecars) != 1 || plan.Sidecars[0].PackageName != "Xlsx" {
		t.Fatalf("unexpected sidecars: %#v", plan.Sidecars)
	}
	if !strings.Contains(plan.Sidecars[0].GoModulePath, cacheDir) {
		t.Fatalf("expected dependency module path under cache %q, got %q", cacheDir, plan.Sidecars[0].GoModulePath)
	}
}

func requireGitForPkgMgr(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

func createPkgMgrGitRepoWithManifest(t *testing.T, manifest string) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runPkgMgrCmd(t, repoDir, "git", "init")
	runPkgMgrCmd(t, repoDir, "git", "config", "user.name", "oct-test")
	runPkgMgrCmd(t, repoDir, "git", "config", "user.email", "oct-test@example.com")
	if err := os.WriteFile(filepath.Join(repoDir, "manifest.oct"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write repo manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "main.oct"), []byte("package Demo\n"), 0o644); err != nil {
		t.Fatalf("write repo source: %v", err)
	}
	runPkgMgrCmd(t, repoDir, "git", "add", ".")
	runPkgMgrCmd(t, repoDir, "git", "commit", "-m", "init")
	path := filepath.ToSlash(repoDir)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func runPkgMgrCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\noutput:%s", name, args, err, string(out))
	}
}

func projectManifestSourceForPkgMgr(depLiterals []string) string {
	deps := ""
	if len(depLiterals) > 0 {
		deps = "\n            " + strings.Join(depLiterals, ",\n            ") + "\n        "
	}
	return strings.Join([]string{
		"package Manifest",
		"",
		"record PackageManifest {",
		"    Name: String",
		"    Version: String",
		"    Description: String",
		"    Dependencies: Dependency[]",
		"}",
		"",
		"record Dependency {",
		"    Name: String",
		"    VersionRequirement: String",
		"    Source: String",
		"}",
		"",
		"fn Manifest() -> PackageManifest {",
		"    return PackageManifest {",
		"        Name: \"Main\"",
		"        Version: \"0.1.0\"",
		"        Description: \"main project\"",
		"        Dependencies: [" + deps + "]",
		"    }",
		"}",
	}, "\n") + "\n"
}
