package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/newpkg"
)

type AddDependencyResult struct {
	Name     string
	Version  string
	Registry string
}

func AddDependency(projectRoot string, spec string, registryName string) (AddDependencyResult, error) {
	name, version, err := ParsePackageSpec(spec)
	if err != nil {
		return AddDependencyResult{}, err
	}
	resolved, err := ResolveRegistryPackage(projectRoot, name, version, registryName)
	if err != nil {
		return AddDependencyResult{}, err
	}
	manifestPath := filepath.Join(projectRoot, manifestFileName)
	metadata, err := LoadManifestMetadata(manifestPath)
	if err != nil {
		return AddDependencyResult{}, err
	}
	for _, dep := range metadata.Dependencies {
		if dep.Name != name {
			continue
		}
		if dep.VersionRequirement == version {
			return AddDependencyResult{}, fmt.Errorf("dependency %s %s is already present", name, version)
		}
		return AddDependencyResult{}, fmt.Errorf("dependency %s already exists with version %s; cannot add %s", name, dep.VersionRequirement, version)
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return AddDependencyResult{}, err
	}
	updated, err := addDependencyToCanonicalManifest(string(body), name, version)
	if err != nil {
		return AddDependencyResult{}, err
	}
	if err := os.WriteFile(manifestPath, []byte(updated), 0o644); err != nil {
		return AddDependencyResult{}, err
	}
	return AddDependencyResult{Name: name, Version: version, Registry: resolved.Registry.Name}, nil
}

func ParsePackageSpec(spec string) (string, string, error) {
	if strings.Count(spec, "@") != 1 {
		return "", "", fmt.Errorf("package spec must be <Name>@<exact-version>")
	}
	parts := strings.SplitN(spec, "@", 2)
	name := strings.TrimSpace(parts[0])
	version := strings.TrimSpace(parts[1])
	if name == "" || version == "" {
		return "", "", fmt.Errorf("package spec must be <Name>@<exact-version>")
	}
	if err := newpkg.ValidateName(name); err != nil {
		return "", "", err
	}
	if err := ValidateExactVersion(version); err != nil {
		return "", "", err
	}
	return name, version, nil
}

const manifestEditError = "oct pkg add can only edit canonical manifest dependency lists in PM2; edit manifest.oct manually or reformat with oct new scaffold style"

func addDependencyToCanonicalManifest(body string, name string, version string) (string, error) {
	needle := "Dependencies: ["
	idx := strings.Index(body, needle)
	if idx < 0 {
		return "", fmt.Errorf(manifestEditError)
	}
	start := idx + len(needle)
	endRel := strings.Index(body[start:], "]")
	if endRel < 0 {
		return "", fmt.Errorf(manifestEditError)
	}
	end := start + endRel
	inside := strings.TrimSpace(body[start:end])
	newDep := "Dependency { Name: " + strconv.Quote(name) + " VersionRequirement: " + strconv.Quote(version) + " }"
	if inside == "" {
		return body[:start] + newDep + body[end:], nil
	}
	if strings.Contains(inside, "\n") {
		insert := "            " + newDep + ",\n"
		return body[:start] + "\n" + insert + body[start:], nil
	}
	replacement := inside + ", " + newDep
	return body[:start] + replacement + body[end:], nil
}
