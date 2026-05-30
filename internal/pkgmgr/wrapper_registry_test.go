package pkgmgr

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestBuildOctxiliaryRegistryEmptyPlan(t *testing.T) {
	registry, err := BuildOctxiliaryRegistry(WrapperBuildPlan{})
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if registry.Version != OctxiliaryRegistryVersion {
		t.Fatalf("Version = %q, want %q", registry.Version, OctxiliaryRegistryVersion)
	}
	if len(registry.Sidecars) != 0 {
		t.Fatalf("expected no sidecars, got %#v", registry.Sidecars)
	}

	rendered, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render registry: %v", err)
	}
	want := "OctxiliaryRegistry {\n" +
		"    Version: \"octxiliary.registry.v0\"\n" +
		"    Sidecars: []\n" +
		"}\n"
	if rendered != want {
		t.Fatalf("unexpected rendered registry:\ngot:\n%s\nwant:\n%s", rendered, want)
	}
}

func TestBuildOctxiliaryRegistryCopiesSingleSidecar(t *testing.T) {
	plan := WrapperBuildPlan{Sidecars: []WrapperSidecarPlan{testRegistryPlanSidecar("Xlsx", "xlsx", "Xlsx", "octxiliary-xlsx", "octxiliary", "/cache/Xlsx/octxiliary")}}

	registry, err := BuildOctxiliaryRegistry(plan)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	want := OctxiliaryRegistry{
		Version: OctxiliaryRegistryVersion,
		Sidecars: []OctxiliarySidecar{
			{
				PackageName:    "Xlsx",
				WrapperName:    "xlsx",
				Family:         "Xlsx",
				Protocol:       "octxiliary.v0",
				SidecarCommand: "octxiliary-xlsx",
				GoModuleDir:    "octxiliary",
				GoModulePath:   "/cache/Xlsx/octxiliary",
				Functions: []OctxiliaryFunction{
					{OctName: "ReadSheetNames", WireName: "XlsxReadSheetNames", Args: []string{"String"}, Return: "String[]", Fallible: true},
					{OctName: "WriteBytes", WireName: "XlsxWriteBytes", Args: []string{"String", "Bytes"}, Return: "Int", Fallible: false},
				},
			},
		},
	}
	if !reflect.DeepEqual(registry, want) {
		t.Fatalf("unexpected registry:\ngot  %#v\nwant %#v", registry, want)
	}

	rendered, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render registry: %v", err)
	}
	wantRendered := "OctxiliaryRegistry {\n" +
		"    Version: \"octxiliary.registry.v0\"\n" +
		"    Sidecars: [\n" +
		"        OctxiliarySidecar {\n" +
		"            PackageName: \"Xlsx\"\n" +
		"            WrapperName: \"xlsx\"\n" +
		"            Family: \"Xlsx\"\n" +
		"            Protocol: \"octxiliary.v0\"\n" +
		"            SidecarCommand: \"octxiliary-xlsx\"\n" +
		"            GoModuleDir: \"octxiliary\"\n" +
		"            GoModulePath: \"/cache/Xlsx/octxiliary\"\n" +
		"            Functions: [\n" +
		"                OctxiliaryFunction {\n" +
		"                    OctName: \"ReadSheetNames\"\n" +
		"                    WireName: \"XlsxReadSheetNames\"\n" +
		"                    Args: [\"String\"]\n" +
		"                    Return: \"String[]\"\n" +
		"                    Fallible: true\n" +
		"                },\n" +
		"                OctxiliaryFunction {\n" +
		"                    OctName: \"WriteBytes\"\n" +
		"                    WireName: \"XlsxWriteBytes\"\n" +
		"                    Args: [\"String\", \"Bytes\"]\n" +
		"                    Return: \"Int\"\n" +
		"                    Fallible: false\n" +
		"                }\n" +
		"            ]\n" +
		"        }\n" +
		"    ]\n" +
		"}\n"
	if rendered != wantRendered {
		t.Fatalf("unexpected rendered registry:\ngot:\n%s\nwant:\n%s", rendered, wantRendered)
	}
	assertParseableOctagon(t, rendered)
}

func TestBuildOctxiliaryRegistrySortsMultipleSidecarsDeterministically(t *testing.T) {
	plan := WrapperBuildPlan{Sidecars: []WrapperSidecarPlan{
		testRegistryPlanSidecar("Zoo", "zeta", "Zeta", "octxiliary-zeta", "wrappers/zeta", "/cache/Zoo/wrappers/zeta"),
		testRegistryPlanSidecar("AlphaPkg", "beta", "Beta", "octxiliary-beta", "wrappers/beta", "/cache/AlphaPkg/wrappers/beta"),
		testRegistryPlanSidecar("AlphaPkg", "alpha", "Alpha", "octxiliary-alpha", "wrappers/alpha", "/cache/AlphaPkg/wrappers/alpha"),
	}}

	registry, err := BuildOctxiliaryRegistry(plan)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	gotOrder := []string{
		registry.Sidecars[0].PackageName + "/" + registry.Sidecars[0].WrapperName,
		registry.Sidecars[1].PackageName + "/" + registry.Sidecars[1].WrapperName,
		registry.Sidecars[2].PackageName + "/" + registry.Sidecars[2].WrapperName,
	}
	wantOrder := []string{"AlphaPkg/alpha", "AlphaPkg/beta", "Zoo/zeta"}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("order = %#v, want %#v", gotOrder, wantOrder)
	}

	first, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render first: %v", err)
	}
	second, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render second: %v", err)
	}
	if first != second {
		t.Fatalf("render output changed between runs:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	alphaIndex := strings.Index(first, "PackageName: \"AlphaPkg\"\n            WrapperName: \"alpha\"")
	betaIndex := strings.Index(first, "PackageName: \"AlphaPkg\"\n            WrapperName: \"beta\"")
	zetaIndex := strings.Index(first, "PackageName: \"Zoo\"\n            WrapperName: \"zeta\"")
	if alphaIndex < 0 || betaIndex < 0 || zetaIndex < 0 || !(alphaIndex < betaIndex && betaIndex < zetaIndex) {
		t.Fatalf("rendered sidecars are not sorted as expected:\n%s", first)
	}
}

func TestRenderOctxiliaryRegistryOctagonEscapesStrings(t *testing.T) {
	registry := OctxiliaryRegistry{
		Version: OctxiliaryRegistryVersion,
		Sidecars: []OctxiliarySidecar{
			{
				PackageName:    "Quote\"Pkg",
				WrapperName:    "slash\\wrapper",
				Family:         "Family\nLine",
				Protocol:       "octxiliary.v0",
				SidecarCommand: "cmd\"quoted",
				GoModuleDir:    "wrap\\dir",
				GoModulePath:   "C:\\cache\\Quote\"Pkg",
				Functions: []OctxiliaryFunction{
					{OctName: "Read\"Name", WireName: "Wire\\Name", Args: []string{"String", "String\\Path"}, Return: "String[]", Fallible: true},
				},
			},
		},
	}

	rendered, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render registry: %v", err)
	}
	checks := []string{
		`PackageName: "Quote\"Pkg"`,
		`WrapperName: "slash\\wrapper"`,
		`Family: "Family\nLine"`,
		`GoModulePath: "C:\\cache\\Quote\"Pkg"`,
		`OctName: "Read\"Name"`,
		`Args: ["String", "String\\Path"]`,
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("expected rendered registry to contain %s, got:\n%s", check, rendered)
		}
	}
	assertParseableOctagon(t, rendered)
}

func TestWriteOctxiliaryRegistryOctagonPathValidationAndParentCreation(t *testing.T) {
	registry := OctxiliaryRegistry{Version: OctxiliaryRegistryVersion}
	badPath := filepath.Join(t.TempDir(), "registry.txt")
	if err := WriteOctxiliaryRegistryOctagon(badPath, registry); err == nil || !strings.Contains(err.Error(), ".octagon") {
		t.Fatalf("expected .octagon path error, got %v", err)
	}

	path := filepath.Join(t.TempDir(), "nested", "registry.octagon")
	if err := WriteOctxiliaryRegistryOctagon(path, registry); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	want, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		t.Fatalf("render registry: %v", err)
	}
	if string(body) != want {
		t.Fatalf("written registry mismatch:\ngot:\n%s\nwant:\n%s", string(body), want)
	}
}

func testRegistryPlanSidecar(packageName string, wrapperName string, family string, command string, moduleDir string, modulePath string) WrapperSidecarPlan {
	return WrapperSidecarPlan{
		PackageName:    packageName,
		WrapperName:    wrapperName,
		Family:         family,
		Protocol:       "octxiliary.v0",
		SidecarCommand: command,
		GoModuleDir:    moduleDir,
		GoModulePath:   modulePath,
		Functions: []WrapperFunctionMetadata{
			{OctName: "ReadSheetNames", WireName: "XlsxReadSheetNames", Args: []string{"String"}, Return: "String[]", Fallible: true},
			{OctName: "WriteBytes", WireName: "XlsxWriteBytes", Args: []string{"String", "Bytes"}, Return: "Int", Fallible: false},
		},
	}
}

func assertParseableOctagon(t *testing.T, rendered string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.octagon")
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		t.Fatalf("write parse smoke file: %v", err)
	}
	if _, err := octagon.Load(path); err != nil {
		t.Fatalf("rendered registry should parse as Octagon: %v\n%s", err, rendered)
	}
}
