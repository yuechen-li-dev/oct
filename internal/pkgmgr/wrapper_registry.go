package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const OctxiliaryRegistryVersion = "octxiliary.registry.v0"

// OctxiliaryRegistry is an inert, deterministic artifact model for wrapper
// sidecars planned by the package manager. It records resolved planning metadata
// only; constructing or writing a registry must not build modules, download
// dependencies, execute sidecars, or discover runtime sidecars.
type OctxiliaryRegistry struct {
	Version  string
	Sidecars []OctxiliarySidecar
}

// OctxiliarySidecar records one package-local native wrapper sidecar.
type OctxiliarySidecar struct {
	PackageName    string
	WrapperName    string
	Family         string
	Protocol       string
	SidecarCommand string
	GoModuleDir    string
	GoModulePath   string
	Functions      []OctxiliaryFunction
}

// OctxiliaryFunction records the Oct-visible and wire-visible metadata for one
// wrapper function.
type OctxiliaryFunction struct {
	OctName  string
	WireName string
	Args     []string
	Return   string
	Fallible bool
}

// BuildOctxiliaryRegistry converts wrapper build-plan metadata into the inert
// registry artifact consumed by future build/runtime/compiler integration.
func BuildOctxiliaryRegistry(plan WrapperBuildPlan) (OctxiliaryRegistry, error) {
	registry := OctxiliaryRegistry{Version: OctxiliaryRegistryVersion}
	registry.Sidecars = make([]OctxiliarySidecar, 0, len(plan.Sidecars))
	for _, sidecar := range plan.Sidecars {
		functions := make([]OctxiliaryFunction, 0, len(sidecar.Functions))
		for _, function := range sidecar.Functions {
			functions = append(functions, OctxiliaryFunction{
				OctName:  function.OctName,
				WireName: function.WireName,
				Args:     append([]string(nil), function.Args...),
				Return:   function.Return,
				Fallible: function.Fallible,
			})
		}
		registry.Sidecars = append(registry.Sidecars, OctxiliarySidecar{
			PackageName:    sidecar.PackageName,
			WrapperName:    sidecar.WrapperName,
			Family:         sidecar.Family,
			Protocol:       sidecar.Protocol,
			SidecarCommand: sidecar.SidecarCommand,
			GoModuleDir:    sidecar.GoModuleDir,
			GoModulePath:   sidecar.GoModulePath,
			Functions:      functions,
		})
	}
	sort.SliceStable(registry.Sidecars, func(i, j int) bool {
		return octxiliarySidecarRegistrySortKey(registry.Sidecars[i]) < octxiliarySidecarRegistrySortKey(registry.Sidecars[j])
	})
	return registry, nil
}

func octxiliarySidecarRegistrySortKey(sidecar OctxiliarySidecar) string {
	return sidecar.PackageName + "\x00" + sidecar.WrapperName + "\x00" + sidecar.Family + "\x00" + sidecar.SidecarCommand
}

// RenderOctxiliaryRegistryOctagon renders a registry as deterministic, data-only
// Octagon text. The output contains no package declarations and ends with
// exactly one newline.
func RenderOctxiliaryRegistryOctagon(registry OctxiliaryRegistry) (string, error) {
	if registry.Version == "" {
		registry.Version = OctxiliaryRegistryVersion
	}
	var b strings.Builder
	b.WriteString("OctxiliaryRegistry {\n")
	renderRegistryField(&b, 1, "Version", strconv.Quote(registry.Version))
	b.WriteString(octxiliaryRegistryIndent(1))
	b.WriteString("Sidecars: ")
	renderSidecarArray(&b, registry.Sidecars, 1)
	b.WriteString("\n}")
	b.WriteString("\n")
	return b.String(), nil
}

// WriteOctxiliaryRegistryOctagon writes a rendered registry artifact to a
// .octagon path, creating parent directories when needed.
func WriteOctxiliaryRegistryOctagon(path string, registry OctxiliaryRegistry) error {
	if !strings.HasSuffix(path, ".octagon") {
		return fmt.Errorf("Octxiliary registry path must end with .octagon")
	}
	rendered, err := RenderOctxiliaryRegistryOctagon(registry)
	if err != nil {
		return err
	}
	if parent := filepath.Dir(path); parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("create Octxiliary registry directory %s: %w", parent, err)
		}
	}
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write Octxiliary registry %s: %w", path, err)
	}
	return nil
}

func renderSidecarArray(b *strings.Builder, sidecars []OctxiliarySidecar, depth int) {
	if len(sidecars) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, sidecar := range sidecars {
		renderSidecar(b, sidecar, depth+1)
		if i != len(sidecars)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("]")
}

func renderSidecar(b *strings.Builder, sidecar OctxiliarySidecar, depth int) {
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("OctxiliarySidecar {\n")
	renderRegistryField(b, depth+1, "PackageName", strconv.Quote(sidecar.PackageName))
	renderRegistryField(b, depth+1, "WrapperName", strconv.Quote(sidecar.WrapperName))
	renderRegistryField(b, depth+1, "Family", strconv.Quote(sidecar.Family))
	renderRegistryField(b, depth+1, "Protocol", strconv.Quote(sidecar.Protocol))
	renderRegistryField(b, depth+1, "SidecarCommand", strconv.Quote(sidecar.SidecarCommand))
	renderRegistryField(b, depth+1, "GoModuleDir", strconv.Quote(sidecar.GoModuleDir))
	renderRegistryField(b, depth+1, "GoModulePath", strconv.Quote(sidecar.GoModulePath))
	b.WriteString(octxiliaryRegistryIndent(depth + 1))
	b.WriteString("Functions: ")
	renderFunctionArray(b, sidecar.Functions, depth+1)
	b.WriteString("\n")
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("}")
}

func renderFunctionArray(b *strings.Builder, functions []OctxiliaryFunction, depth int) {
	if len(functions) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, function := range functions {
		renderFunction(b, function, depth+1)
		if i != len(functions)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("]")
}

func renderFunction(b *strings.Builder, function OctxiliaryFunction, depth int) {
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("OctxiliaryFunction {\n")
	renderRegistryField(b, depth+1, "OctName", strconv.Quote(function.OctName))
	renderRegistryField(b, depth+1, "WireName", strconv.Quote(function.WireName))
	b.WriteString(octxiliaryRegistryIndent(depth + 1))
	b.WriteString("Args: ")
	renderStringArray(b, function.Args)
	b.WriteString("\n")
	renderRegistryField(b, depth+1, "Return", strconv.Quote(function.Return))
	renderRegistryField(b, depth+1, "Fallible", strconv.FormatBool(function.Fallible))
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString("}")
}

func renderStringArray(b *strings.Builder, values []string) {
	b.WriteString("[")
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(value))
	}
	b.WriteString("]")
}

func renderRegistryField(b *strings.Builder, depth int, name string, renderedValue string) {
	b.WriteString(octxiliaryRegistryIndent(depth))
	b.WriteString(name)
	b.WriteString(": ")
	b.WriteString(renderedValue)
	b.WriteString("\n")
}

func octxiliaryRegistryIndent(depth int) string {
	return strings.Repeat("    ", depth)
}
