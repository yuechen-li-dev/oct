package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/octagon"
)

const (
	LockFileName       = "lock.octagon"
	CurrentLockVersion = 1
	LockGeneratedBy    = "oct pkg lock"
)

type PackageLock struct {
	LockVersion int
	GeneratedBy string
	Root        LockRoot
	Packages    []LockPackage
}

type LockRoot struct{ Name, Version string }

type LockPackage struct {
	Name           string
	Version        string
	Kind           string
	SourceKind     string
	Source         string
	Ref            string
	ResolvedCommit string
	Path           string
	Registry       string
	RegistryPath   string
	Mutable        bool
	Dependencies   []LockDependency
}

type LockDependency struct{ Name, Version string }

type LockResult struct {
	Lock          PackageLock
	Path          string
	LocalWarnings []string
}

func (m *Manager) Lock(projectRoot string) (LockResult, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return LockResult{}, fmt.Errorf("resolve project root: %w", err)
	}
	syncResult, err := m.Sync(absRoot)
	if err != nil {
		return LockResult{}, err
	}
	manifest, err := LoadManifestMetadata(filepath.Join(absRoot, manifestFileName))
	if err != nil {
		return LockResult{}, err
	}
	lock := PackageLock{LockVersion: CurrentLockVersion, GeneratedBy: LockGeneratedBy, Root: LockRoot{Name: manifest.Name, Version: manifest.Version}}
	for _, dep := range syncResult.RegistryDependencies {
		pkg := LockPackage{Name: dep.Name, Version: dep.Version, Kind: dep.Kind, SourceKind: dep.SourceKind, Source: dep.Source, Ref: dep.Ref, ResolvedCommit: dep.ResolvedCommit, Path: dep.Path, Registry: dep.Registry, RegistryPath: dep.RegistryPath, Mutable: dep.SourceKind == "local"}
		childManifest, err := LoadManifestMetadata(filepath.Join(dep.Destination, manifestFileName))
		if err != nil {
			return LockResult{}, fmt.Errorf("load synced manifest for lock %s@%s: %w", dep.Name, dep.Version, err)
		}
		for _, child := range sortedDependencies(childManifest.Dependencies) {
			if isBuiltinDependency(child) {
				continue
			}
			if strings.TrimSpace(child.Source) != "" {
				continue
			}
			pkg.Dependencies = append(pkg.Dependencies, LockDependency{Name: child.Name, Version: child.VersionRequirement})
		}
		sortLockDependencies(pkg.Dependencies)
		lock.Packages = append(lock.Packages, pkg)
	}
	sortLockPackages(lock.Packages)
	if err := ValidateLock(lock); err != nil {
		return LockResult{}, err
	}
	path := filepath.Join(absRoot, LockFileName)
	if err := WritePackageLock(path, lock); err != nil {
		return LockResult{}, err
	}
	var warnings []string
	for _, pkg := range lock.Packages {
		if pkg.SourceKind == "local" {
			warnings = append(warnings, fmt.Sprintf("warning: local source %s@%s is mutable; lock.octagon records source path but not content digest", pkg.Name, pkg.Version))
		}
	}
	return LockResult{Lock: lock, Path: path, LocalWarnings: warnings}, nil
}

func RenderPackageLockOctagon(lock PackageLock) (string, error) {
	if lock.LockVersion == 0 {
		lock.LockVersion = CurrentLockVersion
	}
	if lock.GeneratedBy == "" {
		lock.GeneratedBy = LockGeneratedBy
	}
	sortLockPackages(lock.Packages)
	var b strings.Builder
	b.WriteString("OctPackageLock {\n")
	renderRegistryField(&b, 1, "LockVersion", strconv.Itoa(lock.LockVersion))
	renderRegistryField(&b, 1, "GeneratedBy", strconv.Quote(lock.GeneratedBy))
	b.WriteString(lockIndent(1))
	b.WriteString("Root: LockRoot { Name: ")
	b.WriteString(strconv.Quote(lock.Root.Name))
	b.WriteString(" Version: ")
	b.WriteString(strconv.Quote(lock.Root.Version))
	b.WriteString(" }\n")
	b.WriteString(lockIndent(1))
	b.WriteString("Packages: ")
	renderLockPackageArray(&b, lock.Packages, 1)
	b.WriteString("\n}\n")
	return b.String(), nil
}

func WritePackageLock(path string, lock PackageLock) error {
	if filepath.Base(path) != LockFileName {
		return fmt.Errorf("package lockfile must be named lock.octagon")
	}
	rendered, err := RenderPackageLockOctagon(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(rendered), 0o644)
}

func LoadPackageLock(path string) (PackageLock, error) {
	expr, err := octagon.Load(path)
	if err != nil {
		return PackageLock{}, err
	}
	rec, ok := expr.(ast.RecordLiteralExpr)
	if !ok || rec.TypeName != "OctPackageLock" {
		return PackageLock{}, fmt.Errorf("lock.octagon must contain OctPackageLock")
	}
	lock, err := parseLockRecord(rec)
	if err != nil {
		return PackageLock{}, err
	}
	if err := ValidateLock(lock); err != nil {
		return PackageLock{}, err
	}
	return lock, nil
}

func ValidateLock(lock PackageLock) error {
	if lock.LockVersion != CurrentLockVersion {
		return fmt.Errorf("unsupported LockVersion %d", lock.LockVersion)
	}
	if strings.TrimSpace(lock.Root.Name) == "" || strings.TrimSpace(lock.Root.Version) == "" {
		return fmt.Errorf("lock root Name and Version are required")
	}
	byKey := map[string]LockPackage{}
	byName := map[string]string{}
	for _, pkg := range lock.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			return fmt.Errorf("lock package Name and Version are required")
		}
		key := nodeKey(pkg.Name, pkg.Version)
		if _, ok := byKey[key]; ok {
			return fmt.Errorf("duplicate lock package node %s", key)
		}
		if prior, ok := byName[pkg.Name]; ok && prior != pkg.Version {
			return fmt.Errorf("conflicting lock package versions for %s: %s and %s", pkg.Name, prior, pkg.Version)
		}
		byKey[key] = pkg
		byName[pkg.Name] = pkg.Version
		switch pkg.SourceKind {
		case "git":
			if !IsFullCommitSHA(pkg.ResolvedCommit) {
				return fmt.Errorf("git lock package %s requires full 40-character ResolvedCommit", key)
			}
			if pkg.Mutable {
				return fmt.Errorf("git lock package %s must have Mutable false", key)
			}
		case "local":
			if pkg.ResolvedCommit != "" || pkg.Ref != "" {
				return fmt.Errorf("local lock package %s must not record Ref or ResolvedCommit", key)
			}
			if !pkg.Mutable {
				return fmt.Errorf("local lock package %s must have Mutable true", key)
			}
		default:
			return fmt.Errorf("lock package %s has unsupported SourceKind %q", key, pkg.SourceKind)
		}
		seenDep := map[string]bool{}
		for _, dep := range pkg.Dependencies {
			dkey := nodeKey(dep.Name, dep.Version)
			if seenDep[dkey] {
				return fmt.Errorf("duplicate dependency %s in lock package %s", dkey, key)
			}
			seenDep[dkey] = true
		}
	}
	for _, pkg := range lock.Packages {
		for _, dep := range pkg.Dependencies {
			if _, ok := byKey[nodeKey(dep.Name, dep.Version)]; !ok {
				return fmt.Errorf("lock package %s depends on missing package %s", nodeKey(pkg.Name, pkg.Version), nodeKey(dep.Name, dep.Version))
			}
		}
	}
	state := map[string]string{}
	var visit func(LockPackage, []string) error
	visit = func(pkg LockPackage, chain []string) error {
		key := nodeKey(pkg.Name, pkg.Version)
		if state[key] == "done" {
			return nil
		}
		if state[key] == "visiting" {
			return fmt.Errorf("lock dependency cycle detected: %s", formatChain(append(chain, key)))
		}
		state[key] = "visiting"
		for _, dep := range pkg.Dependencies {
			if err := visit(byKey[nodeKey(dep.Name, dep.Version)], append(chain, key)); err != nil {
				return err
			}
		}
		state[key] = "done"
		return nil
	}
	for _, pkg := range lock.Packages {
		if err := visit(pkg, nil); err != nil {
			return err
		}
	}
	return nil
}

func ValidateLockAgainstManifest(lock PackageLock, manifest ManifestMetadata) error {
	if lock.Root.Name != manifest.Name || lock.Root.Version != manifest.Version {
		return fmt.Errorf("lock root %s@%s does not match current project %s@%s", lock.Root.Name, lock.Root.Version, manifest.Name, manifest.Version)
	}
	byKey := map[string]LockPackage{}
	byName := map[string]string{}
	for _, pkg := range lock.Packages {
		byKey[nodeKey(pkg.Name, pkg.Version)] = pkg
		byName[pkg.Name] = pkg.Version
	}
	rootKeys := map[string]bool{}
	for _, dep := range sortedDependencies(manifest.Dependencies) {
		if isBuiltinDependency(dep) || strings.TrimSpace(dep.Source) != "" {
			continue
		}
		key := nodeKey(dep.Name, dep.VersionRequirement)
		if _, ok := byKey[key]; !ok {
			if v, has := byName[dep.Name]; has {
				return fmt.Errorf("manifest dependency %s is not locked; lock contains %s@%s; run oct pkg lock", key, dep.Name, v)
			}
			return fmt.Errorf("manifest dependency %s is not locked; run oct pkg lock", key)
		}
		rootKeys[key] = true
	}
	reachable := map[string]bool{}
	var mark func(string)
	mark = func(key string) {
		if reachable[key] {
			return
		}
		reachable[key] = true
		for _, dep := range byKey[key].Dependencies {
			mark(nodeKey(dep.Name, dep.Version))
		}
	}
	for key := range rootKeys {
		mark(key)
	}
	for _, pkg := range lock.Packages {
		key := nodeKey(pkg.Name, pkg.Version)
		if !reachable[key] {
			return fmt.Errorf("lock package %s is no longer reachable from current manifest dependencies; run oct pkg lock", key)
		}
	}
	return nil
}

func renderLockPackageArray(b *strings.Builder, packages []LockPackage, depth int) {
	if len(packages) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, pkg := range packages {
		renderLockPackage(b, pkg, depth+1)
		if i != len(packages)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(lockIndent(depth))
	b.WriteString("]")
}
func renderLockPackage(b *strings.Builder, pkg LockPackage, depth int) {
	sortLockDependencies(pkg.Dependencies)
	b.WriteString(lockIndent(depth))
	b.WriteString("LockPackage {\n")
	renderRegistryField(b, depth+1, "Name", strconv.Quote(pkg.Name))
	renderRegistryField(b, depth+1, "Version", strconv.Quote(pkg.Version))
	renderRegistryField(b, depth+1, "Kind", strconv.Quote(pkg.Kind))
	renderRegistryField(b, depth+1, "SourceKind", strconv.Quote(pkg.SourceKind))
	renderRegistryField(b, depth+1, "Source", strconv.Quote(pkg.Source))
	renderRegistryField(b, depth+1, "Ref", strconv.Quote(pkg.Ref))
	renderRegistryField(b, depth+1, "ResolvedCommit", strconv.Quote(pkg.ResolvedCommit))
	renderRegistryField(b, depth+1, "Path", strconv.Quote(pkg.Path))
	renderRegistryField(b, depth+1, "Registry", strconv.Quote(pkg.Registry))
	renderRegistryField(b, depth+1, "RegistryPath", strconv.Quote(pkg.RegistryPath))
	renderRegistryField(b, depth+1, "Mutable", strconv.FormatBool(pkg.Mutable))
	b.WriteString(lockIndent(depth + 1))
	b.WriteString("Dependencies: ")
	renderLockDependencyArray(b, pkg.Dependencies, depth+1)
	b.WriteString("\n")
	b.WriteString(lockIndent(depth))
	b.WriteString("}")
}
func renderLockDependencyArray(b *strings.Builder, deps []LockDependency, depth int) {
	if len(deps) == 0 {
		b.WriteString("[]")
		return
	}
	b.WriteString("[\n")
	for i, d := range deps {
		b.WriteString(lockIndent(depth + 1))
		b.WriteString("LockDependency { Name: ")
		b.WriteString(strconv.Quote(d.Name))
		b.WriteString(" Version: ")
		b.WriteString(strconv.Quote(d.Version))
		b.WriteString(" }")
		if i != len(deps)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(lockIndent(depth))
	b.WriteString("]")
}
func lockIndent(depth int) string { return strings.Repeat("    ", depth) }
func sortLockPackages(packages []LockPackage) {
	sort.SliceStable(packages, func(i, j int) bool {
		if packages[i].Name != packages[j].Name {
			return packages[i].Name < packages[j].Name
		}
		return packages[i].Version < packages[j].Version
	})
}
func sortLockDependencies(deps []LockDependency) {
	sort.SliceStable(deps, func(i, j int) bool {
		if deps[i].Name != deps[j].Name {
			return deps[i].Name < deps[j].Name
		}
		return deps[i].Version < deps[j].Version
	})
}

func parseLockRecord(rec ast.RecordLiteralExpr) (PackageLock, error) {
	f, err := recordLiteralMap(rec)
	if err != nil {
		return PackageLock{}, err
	}
	lock := PackageLock{}
	if lock.LockVersion, err = intField(f, "LockVersion"); err != nil {
		return PackageLock{}, err
	}
	if lock.GeneratedBy, err = stringFieldExpr(f, "GeneratedBy"); err != nil {
		return PackageLock{}, err
	}
	rootRec, err := recordFieldExpr(f, "Root", "LockRoot")
	if err != nil {
		return PackageLock{}, err
	}
	rf, err := recordLiteralMap(rootRec)
	if err != nil {
		return PackageLock{}, err
	}
	if lock.Root.Name, err = stringFieldExpr(rf, "Name"); err != nil {
		return PackageLock{}, err
	}
	if lock.Root.Version, err = stringFieldExpr(rf, "Version"); err != nil {
		return PackageLock{}, err
	}
	arr, err := arrayFieldExpr(f, "Packages")
	if err != nil {
		return PackageLock{}, err
	}
	for idx, e := range arr.Elements {
		pr, ok := e.(ast.RecordLiteralExpr)
		if !ok || pr.TypeName != "LockPackage" {
			return PackageLock{}, fmt.Errorf("Packages[%d] must be LockPackage", idx)
		}
		p, err := parseLockPackage(pr)
		if err != nil {
			return PackageLock{}, fmt.Errorf("Packages[%d]: %w", idx, err)
		}
		lock.Packages = append(lock.Packages, p)
	}
	return lock, nil
}
func parseLockPackage(rec ast.RecordLiteralExpr) (LockPackage, error) {
	f, err := recordLiteralMap(rec)
	if err != nil {
		return LockPackage{}, err
	}
	p := LockPackage{}
	for _, item := range []struct {
		name string
		dst  *string
	}{{"Name", &p.Name}, {"Version", &p.Version}, {"Kind", &p.Kind}, {"SourceKind", &p.SourceKind}, {"Source", &p.Source}, {"Ref", &p.Ref}, {"ResolvedCommit", &p.ResolvedCommit}, {"Path", &p.Path}, {"Registry", &p.Registry}, {"RegistryPath", &p.RegistryPath}} {
		if *item.dst, err = stringFieldExpr(f, item.name); err != nil {
			return LockPackage{}, err
		}
	}
	if p.Mutable, err = boolFieldExpr(f, "Mutable"); err != nil {
		return LockPackage{}, err
	}
	arr, err := arrayFieldExpr(f, "Dependencies")
	if err != nil {
		return LockPackage{}, err
	}
	for idx, e := range arr.Elements {
		dr, ok := e.(ast.RecordLiteralExpr)
		if !ok || dr.TypeName != "LockDependency" {
			return LockPackage{}, fmt.Errorf("Dependencies[%d] must be LockDependency", idx)
		}
		df, err := recordLiteralMap(dr)
		if err != nil {
			return LockPackage{}, err
		}
		name, err := stringFieldExpr(df, "Name")
		if err != nil {
			return LockPackage{}, err
		}
		ver, err := stringFieldExpr(df, "Version")
		if err != nil {
			return LockPackage{}, err
		}
		p.Dependencies = append(p.Dependencies, LockDependency{Name: name, Version: ver})
	}
	return p, nil
}
func recordLiteralMap(rec ast.RecordLiteralExpr) (map[string]ast.Expr, error) {
	m := map[string]ast.Expr{}
	for _, fld := range rec.Fields {
		if _, ok := m[fld.Name]; ok {
			return nil, fmt.Errorf("duplicate field %s", fld.Name)
		}
		m[fld.Name] = fld.Value
	}
	return m, nil
}
func stringFieldExpr(f map[string]ast.Expr, name string) (string, error) {
	e, ok := f[name]
	if !ok {
		return "", fmt.Errorf("missing required field %s", name)
	}
	s, ok := e.(ast.StringLiteralExpr)
	if !ok {
		return "", fmt.Errorf("field %s must be String", name)
	}
	return s.Value, nil
}
func boolFieldExpr(f map[string]ast.Expr, name string) (bool, error) {
	e, ok := f[name]
	if !ok {
		return false, fmt.Errorf("missing required field %s", name)
	}
	v, ok := e.(ast.BoolLiteral)
	if !ok {
		return false, fmt.Errorf("field %s must be Bool", name)
	}
	return v.Value, nil
}
func intField(f map[string]ast.Expr, name string) (int, error) {
	e, ok := f[name]
	if !ok {
		return 0, fmt.Errorf("missing required field %s", name)
	}
	v, ok := e.(ast.IntegerLiteral)
	if !ok {
		return 0, fmt.Errorf("field %s must be Int", name)
	}
	n, err := strconv.Atoi(v.Value)
	if err != nil {
		return 0, fmt.Errorf("field %s must be Int", name)
	}
	return n, nil
}
func recordFieldExpr(f map[string]ast.Expr, name, typ string) (ast.RecordLiteralExpr, error) {
	e, ok := f[name]
	if !ok {
		return ast.RecordLiteralExpr{}, fmt.Errorf("missing required field %s", name)
	}
	r, ok := e.(ast.RecordLiteralExpr)
	if !ok || r.TypeName != typ {
		return ast.RecordLiteralExpr{}, fmt.Errorf("field %s must be %s", name, typ)
	}
	return r, nil
}
func arrayFieldExpr(f map[string]ast.Expr, name string) (ast.ArrayLiteralExpr, error) {
	e, ok := f[name]
	if !ok {
		return ast.ArrayLiteralExpr{}, fmt.Errorf("missing required field %s", name)
	}
	a, ok := e.(ast.ArrayLiteralExpr)
	if !ok {
		return ast.ArrayLiteralExpr{}, fmt.Errorf("field %s must be array", name)
	}
	return a, nil
}

type LockedSyncResult struct {
	ProjectPath  string
	ManifestPath string
	Packages     []RegistrySyncResult
}

func (m *Manager) SyncLocked(projectRoot string) (LockedSyncResult, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return LockedSyncResult{}, fmt.Errorf("resolve project root: %w", err)
	}
	lockPath := filepath.Join(absRoot, LockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		if os.IsNotExist(err) {
			return LockedSyncResult{}, fmt.Errorf("lock.octagon is required for --locked; run oct pkg lock to create it")
		}
		return LockedSyncResult{}, fmt.Errorf("read lock.octagon: %w", err)
	}
	lock, err := LoadPackageLock(lockPath)
	if err != nil {
		return LockedSyncResult{}, err
	}
	manifestPath := filepath.Join(absRoot, manifestFileName)
	manifest, err := LoadManifestMetadata(manifestPath)
	if err != nil {
		return LockedSyncResult{}, err
	}
	if err := ValidateLockAgainstManifest(lock, manifest); err != nil {
		return LockedSyncResult{}, err
	}
	result := LockedSyncResult{ProjectPath: absRoot, ManifestPath: manifestPath}
	packages := append([]LockPackage(nil), lock.Packages...)
	sortLockPackages(packages)
	for _, pkg := range packages {
		synced, err := syncLockedPackage(absRoot, pkg)
		if err != nil {
			return LockedSyncResult{}, err
		}
		result.Packages = append(result.Packages, synced)
	}
	return result, nil
}

func syncLockedPackage(projectRoot string, pkg LockPackage) (RegistrySyncResult, error) {
	resolved := ResolvedPackage{Registry: RegistrySource{Name: pkg.Registry, Path: pkg.RegistryPath}, Entry: PackageEntry{Name: pkg.Name, Version: pkg.Version, Kind: pkg.Kind, SourceKind: pkg.SourceKind, Source: pkg.Source, Ref: pkg.Ref, Path: pkg.Path}, ResolvedCommit: pkg.ResolvedCommit}
	switch pkg.SourceKind {
	case "local":
		sourceRoot := pkg.Source
		if !filepath.IsAbs(sourceRoot) {
			registryRoot := pkg.RegistryPath
			if !filepath.IsAbs(registryRoot) {
				registryRoot = filepath.Join(projectRoot, registryRoot)
			}
			sourceRoot = filepath.Join(registryRoot, sourceRoot)
		}
		sourceRoot = filepath.Clean(sourceRoot)
		packageRoot, err := safePackageSourcePath(sourceRoot, pkg.Path)
		if err != nil {
			return RegistrySyncResult{}, fmt.Errorf("locked local dependency %s %s source %s path %s: %w", pkg.Name, pkg.Version, pkg.Source, pkg.Path, err)
		}
		return installResolvedPackage(projectRoot, resolved, packageRoot)
	case "git":
		if !IsFullCommitSHA(pkg.ResolvedCommit) {
			return RegistrySyncResult{}, fmt.Errorf("locked git dependency %s %s source %s ref %s has invalid ResolvedCommit %q", pkg.Name, pkg.Version, pkg.Source, pkg.Ref, pkg.ResolvedCommit)
		}
		return syncLockedGitPackage(projectRoot, resolved)
	default:
		return RegistrySyncResult{}, fmt.Errorf("locked dependency %s %s has unsupported SourceKind %q", pkg.Name, pkg.Version, pkg.SourceKind)
	}
}

func syncLockedGitPackage(projectRoot string, resolved ResolvedPackage) (RegistrySyncResult, error) {
	entry := resolved.Entry
	baseDir := filepath.Join(projectRoot, ProjectPackagesRelDir, entry.Name)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create package cache directory: %w", err)
	}
	cloneDir, err := os.MkdirTemp(baseDir, ".tmp-git-*")
	if err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create temporary git clone directory: %w", err)
	}
	defer os.RemoveAll(cloneDir)
	checkoutEntry := entry
	checkoutEntry.Ref = resolved.ResolvedCommit
	runner := func(operation string, name string, args ...string) (string, error) {
		if operation == "checkout" {
			operation = "checkout locked commit"
			return runGitPackageOutput(checkoutEntry, resolved.Registry.Name, operation, name, args...)
		}
		return runGitPackageOutput(entry, resolved.Registry.Name, operation, name, args...)
	}
	if err := gitCloneConfigCheckout(entry.Source, cloneDir, resolved.ResolvedCommit, nil, runner); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("locked git clone/config/checkout failed for %s %s source %s ref %s resolved commit %s path %s: %w", entry.Name, entry.Version, entry.Source, entry.Ref, resolved.ResolvedCommit, entry.Path, err)
	}
	packageRoot, err := safePackageSourcePath(cloneDir, entry.Path)
	if err != nil {
		return RegistrySyncResult{}, fmt.Errorf("locked git dependency %s %s source %s ref %s resolved commit %s: %w", entry.Name, entry.Version, entry.Source, entry.Ref, resolved.ResolvedCommit, err)
	}
	return installResolvedPackage(projectRoot, resolved, packageRoot)
}
