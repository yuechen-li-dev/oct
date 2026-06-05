package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/manifestkind"
)

// WrapperBuildPlan describes native wrapper sidecars that would need to be built.
// It is planning metadata only; creating a plan must not download modules, build
// binaries, generate registries, or execute sidecars.
type WrapperBuildPlan struct {
	Root                          string
	Packages                      []WrapperPackagePlan
	Sidecars                      []WrapperSidecarPlan
	HasNativeWrappers             bool
	RequiresNativeBuildPermission bool
}

// WrapperPackagePlan identifies a wrapper package in a package graph.
type WrapperPackagePlan struct {
	PackageName  string
	Version      string
	Source       string
	CachePath    string
	Kind         string
	WrapperCount int
}

// WrapperSidecarPlan identifies a package-local native wrapper sidecar.
type WrapperSidecarPlan struct {
	PackageName    string
	WrapperName    string
	Family         string
	Protocol       string
	SidecarCommand string
	GoModuleDir    string
	GoModulePath   string
	Functions      []WrapperFunctionMetadata
	TransportTypes []TransportTypeMetadata
}

// BuildWrapperPlanForProject creates a planning-only wrapper sidecar plan for
// the current root plus direct dependencies that declare fetchable Source
// metadata. Dependencies without Source are ignored for wrapper planning so
// local package wrapper metadata remains inspectable without requiring a package
// sync. It does not build Go modules, download Go module contents, execute
// sidecars, generate registries, or perform runtime discovery.
func (m *Manager) BuildWrapperPlanForProject(projectRoot string) (WrapperBuildPlan, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return WrapperBuildPlan{}, fmt.Errorf("resolve project root: %w", err)
	}
	manifestPath := filepath.Join(absRoot, manifestFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return WrapperBuildPlan{}, fmt.Errorf("project manifest not found: %s", manifestPath)
		}
		return WrapperBuildPlan{}, fmt.Errorf("read project manifest %s: %w", manifestPath, err)
	}
	rootMetadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		return WrapperBuildPlan{}, err
	}
	inputs := []wrapperPlanInput{{
		root:      absRoot,
		cachePath: absRoot,
		metadata:  rootMetadata,
	}}
	for _, dep := range rootMetadata.Dependencies {
		if strings.TrimSpace(dep.Source) == "" {
			continue
		}
		getResult, err := m.Get(dep.Source)
		if err != nil {
			return WrapperBuildPlan{}, fmt.Errorf("dependency %s: %w", dep.Name, err)
		}
		inputs = append(inputs, wrapperPlanInput{
			root:      getResult.Path,
			source:    dep.Source,
			cachePath: getResult.Path,
			metadata:  getResult.Manifest,
		})
	}
	return buildWrapperPlan(inputs, absRoot)
}

// BuildWrapperPlanForManifest creates a planning-only wrapper sidecar plan for a
// single manifest rooted at rootPath.
func BuildWrapperPlanForManifest(rootPath string, metadata ManifestMetadata) (WrapperBuildPlan, error) {
	return buildWrapperPlan([]wrapperPlanInput{{root: rootPath, cachePath: rootPath, metadata: metadata}}, rootPath)
}

// BuildWrapperPlanForGetResult creates a planning-only wrapper sidecar plan for
// a fetched or cached package result.
func BuildWrapperPlanForGetResult(result GetResult) (WrapperBuildPlan, error) {
	return buildWrapperPlan([]wrapperPlanInput{{root: result.Path, source: result.Source, cachePath: result.Path, metadata: result.Manifest}}, result.Path)
}

// BuildWrapperPlanForSyncResult creates a planning-only wrapper sidecar plan for
// the dependency graph fetched by pkg sync. The current project manifest is not
// included because SyncResult currently records fetched dependencies only.
func BuildWrapperPlanForSyncResult(result SyncResult) (WrapperBuildPlan, error) {
	inputs := make([]wrapperPlanInput, 0, len(result.Dependencies))
	for _, dep := range result.Dependencies {
		inputs = append(inputs, wrapperPlanInput{
			root:      dep.GetResult.Path,
			source:    dep.Source,
			cachePath: dep.GetResult.Path,
			metadata:  dep.GetResult.Manifest,
		})
	}
	return buildWrapperPlan(inputs, result.ProjectPath)
}

type wrapperPlanInput struct {
	root      string
	source    string
	cachePath string
	metadata  ManifestMetadata
}

func buildWrapperPlan(inputs []wrapperPlanInput, root string) (WrapperBuildPlan, error) {
	plan := WrapperBuildPlan{Root: root}
	for _, input := range inputs {
		metadata := input.metadata
		if metadata.Kind != manifestkind.Wrapper {
			continue
		}
		plan.Packages = append(plan.Packages, WrapperPackagePlan{
			PackageName:  metadata.Name,
			Version:      metadata.Version,
			Source:       input.source,
			CachePath:    input.cachePath,
			Kind:         metadata.Kind,
			WrapperCount: len(metadata.Wrappers),
		})
		for _, wrapper := range metadata.Wrappers {
			functions := append([]WrapperFunctionMetadata(nil), wrapper.Functions...)
			transportTypes := append([]TransportTypeMetadata(nil), wrapper.TransportTypes...)
			modulePath := wrapperGoModulePath(input.root, wrapper.GoModuleDir)
			plan.Sidecars = append(plan.Sidecars, WrapperSidecarPlan{
				PackageName:    metadata.Name,
				WrapperName:    wrapper.Name,
				Family:         wrapper.Family,
				Protocol:       wrapper.Protocol,
				SidecarCommand: wrapper.SidecarCommand,
				GoModuleDir:    wrapper.GoModuleDir,
				GoModulePath:   modulePath,
				Functions:      functions,
				TransportTypes: transportTypes,
			})
		}
	}
	sort.SliceStable(plan.Sidecars, func(i, j int) bool {
		left := plan.Sidecars[i]
		right := plan.Sidecars[j]
		return wrapperSidecarSortKey(left) < wrapperSidecarSortKey(right)
	})
	if err := validateWrapperPlanConflicts(plan.Sidecars); err != nil {
		return WrapperBuildPlan{}, err
	}
	plan.HasNativeWrappers = len(plan.Sidecars) > 0
	plan.RequiresNativeBuildPermission = plan.HasNativeWrappers
	return plan, nil
}

func wrapperGoModulePath(root string, goModuleDir string) string {
	normalizedDir := strings.ReplaceAll(goModuleDir, "\\", "/")
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(normalizedDir)))
}

func wrapperSidecarSortKey(sidecar WrapperSidecarPlan) string {
	return sidecar.PackageName + "\x00" + sidecar.WrapperName + "\x00" + sidecar.Family + "\x00" + sidecar.SidecarCommand
}

func validateWrapperPlanConflicts(sidecars []WrapperSidecarPlan) error {
	commands := map[string]WrapperSidecarPlan{}
	families := map[string]WrapperSidecarPlan{}
	modulePaths := map[string]WrapperSidecarPlan{}
	for _, sidecar := range sidecars {
		if sidecar.SidecarCommand == "" {
			return fmt.Errorf("wrapper plan sidecar %s/%s has empty SidecarCommand", sidecar.PackageName, sidecar.WrapperName)
		}
		if prior, ok := commands[sidecar.SidecarCommand]; ok {
			return fmt.Errorf("wrapper plan conflict: duplicate sidecar command %q for %s/%s and %s/%s", sidecar.SidecarCommand, prior.PackageName, prior.WrapperName, sidecar.PackageName, sidecar.WrapperName)
		}
		commands[sidecar.SidecarCommand] = sidecar

		if sidecar.Family == "" {
			return fmt.Errorf("wrapper plan sidecar %s/%s has empty Family", sidecar.PackageName, sidecar.WrapperName)
		}
		if prior, ok := families[sidecar.Family]; ok {
			return fmt.Errorf("wrapper plan conflict: duplicate wrapper family %q for %s/%s and %s/%s", sidecar.Family, prior.PackageName, prior.WrapperName, sidecar.PackageName, sidecar.WrapperName)
		}
		families[sidecar.Family] = sidecar

		if sidecar.GoModulePath == "" {
			return fmt.Errorf("wrapper plan sidecar %s/%s has empty GoModulePath", sidecar.PackageName, sidecar.WrapperName)
		}
		if prior, ok := modulePaths[sidecar.GoModulePath]; ok {
			return fmt.Errorf("wrapper plan conflict: duplicate GoModulePath %q for %s/%s and %s/%s", sidecar.GoModulePath, prior.PackageName, prior.WrapperName, sidecar.PackageName, sidecar.WrapperName)
		}
		modulePaths[sidecar.GoModulePath] = sidecar
	}
	return nil
}
