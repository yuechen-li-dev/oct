package pkgmgr

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// WrapperBuildTarget describes one package-local native wrapper sidecar build.
// Targets are derived from manifest wrapper metadata only; constructing targets
// must not compile or execute sidecars.
type WrapperBuildTarget struct {
	PackageName    string
	PackageVersion string
	WrapperName    string
	Family         string
	Protocol       string
	SidecarCommand string
	GoModuleDir    string
	SourceDir      string
	OutputPath     string
	Platform       string
	FunctionCount  int
}

// WrapperBuildResult records a completed wrapper sidecar build.
type WrapperBuildResult struct {
	Targets []WrapperBuildTarget
}

// BuildWrapperBuildTargetsForProject returns native wrapper build targets for
// the current package only. It does not fetch dependencies, build Go modules, or
// execute sidecars.
func (m *Manager) BuildWrapperBuildTargetsForProject(projectRoot string) ([]WrapperBuildTarget, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	manifestPath := filepath.Join(absRoot, manifestFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project manifest not found: %s", manifestPath)
		}
		return nil, fmt.Errorf("read project manifest %s: %w", manifestPath, err)
	}
	metadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		return nil, err
	}
	plan, err := BuildWrapperPlanForManifest(absRoot, metadata)
	if err != nil {
		return nil, err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	outputDir := filepath.Join(absRoot, ".oct", "wrappers", platform)
	versionByPackage := map[string]string{}
	for _, pkg := range plan.Packages {
		versionByPackage[pkg.PackageName] = pkg.Version
	}
	targets := make([]WrapperBuildTarget, 0, len(plan.Sidecars))
	for _, sidecar := range plan.Sidecars {
		outputName := sidecar.SidecarCommand
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(outputName), ".exe") {
			outputName += ".exe"
		}
		targets = append(targets, WrapperBuildTarget{
			PackageName:    sidecar.PackageName,
			PackageVersion: versionByPackage[sidecar.PackageName],
			WrapperName:    sidecar.WrapperName,
			Family:         sidecar.Family,
			Protocol:       sidecar.Protocol,
			SidecarCommand: sidecar.SidecarCommand,
			GoModuleDir:    sidecar.GoModuleDir,
			SourceDir:      sidecar.GoModulePath,
			OutputPath:     filepath.Join(outputDir, outputName),
			Platform:       platform,
			FunctionCount:  len(sidecar.Functions),
		})
	}
	sort.SliceStable(targets, func(i, j int) bool {
		return wrapperBuildTargetSortKey(targets[i]) < wrapperBuildTargetSortKey(targets[j])
	})
	return targets, nil
}

// BuildWrapperTargets compiles each target with `go build -o <output> .` from
// the target source directory. It never executes the resulting sidecar binary.
func BuildWrapperTargets(targets []WrapperBuildTarget) (WrapperBuildResult, error) {
	for _, target := range targets {
		if err := buildWrapperTarget(target); err != nil {
			return WrapperBuildResult{}, err
		}
	}
	return WrapperBuildResult{Targets: append([]WrapperBuildTarget(nil), targets...)}, nil
}

func buildWrapperTarget(target WrapperBuildTarget) error {
	info, err := os.Stat(target.SourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return wrapperBuildError(target, fmt.Errorf("source dir does not exist"), "")
		}
		return wrapperBuildError(target, fmt.Errorf("stat source dir: %w", err), "")
	}
	if !info.IsDir() {
		return wrapperBuildError(target, fmt.Errorf("source dir is not a directory"), "")
	}
	if _, err := os.Stat(filepath.Join(target.SourceDir, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return wrapperBuildError(target, fmt.Errorf("source dir does not contain go.mod"), "")
		}
		return wrapperBuildError(target, fmt.Errorf("read go.mod: %w", err), "")
	}
	if err := os.MkdirAll(filepath.Dir(target.OutputPath), 0o755); err != nil {
		return wrapperBuildError(target, fmt.Errorf("create output dir: %w", err), "")
	}
	cmd := exec.Command("go", "build", "-o", target.OutputPath, ".")
	cmd.Dir = target.SourceDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(strings.TrimSpace(stdout.String()) + "\n" + strings.TrimSpace(stderr.String()))
		return wrapperBuildError(target, fmt.Errorf("go build failed: %w", err), combined)
	}
	return nil
}

func wrapperBuildError(target WrapperBuildTarget, cause error, goOutput string) error {
	var b strings.Builder
	b.WriteString("wrapper sidecar build failed")
	fmt.Fprintf(&b, ": package %s", target.PackageName)
	if target.PackageVersion != "" {
		fmt.Fprintf(&b, " %s", target.PackageVersion)
	}
	fmt.Fprintf(&b, ", wrapper %s, command %s, source dir %s, output path %s: %v", target.WrapperName, target.SidecarCommand, target.SourceDir, target.OutputPath, cause)
	if goOutput != "" {
		fmt.Fprintf(&b, "\ngo output:\n%s", goOutput)
	}
	return fmt.Errorf("%s", b.String())
}

func wrapperBuildTargetSortKey(target WrapperBuildTarget) string {
	return target.PackageName + "\x00" + target.WrapperName + "\x00" + target.Family + "\x00" + target.SidecarCommand
}
