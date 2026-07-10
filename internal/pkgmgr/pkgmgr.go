package pkgmgr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	manifestFileName = "manifest.oct"
	envCacheDir      = "OCT_PKG_CACHE_DIR"
)

type Manager struct {
	cacheDir string
}

type GetResult struct {
	Source   string
	CacheKey string
	Path     string
	Hit      bool
	Name     string
	Version  string
	Head     string
	Manifest ManifestMetadata
}

type Entry struct {
	Source       string               `json:"source"`
	CacheKey     string               `json:"cache_key"`
	Path         string               `json:"path"`
	Name         string               `json:"name,omitempty"`
	Version      string               `json:"version,omitempty"`
	Dependencies []DependencyMetadata `json:"dependencies,omitempty"`
	Head         string               `json:"head,omitempty"`
	FetchedAt    string               `json:"fetched_at"`
}

type SyncDependencyResult struct {
	Name               string
	VersionRequirement string
	Source             string
	GetResult          GetResult
}

type SyncResult struct {
	ProjectPath            string
	ManifestPath           string
	Dependencies           []SyncDependencyResult
	RegistryDependencies   []RegistrySyncResult
	SkippedBuiltinPackages []DependencyMetadata
}

type index struct {
	Entries map[string]Entry `json:"entries"`
}

func NewManager() (*Manager, error) {
	return NewManagerWithCacheDir("")
}

// NewManagerWithCacheDir creates a manager with an explicit cache root. An
// empty root preserves the process-boundary environment/default behavior.
func NewManagerWithCacheDir(cacheDir string) (*Manager, error) {
	if cacheDir == "" {
		cacheDir = os.Getenv(envCacheDir)
	}
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user cache dir: %w", err)
		}
		cacheDir = filepath.Join(userCache, "oct", "pkg")
	}
	return &Manager{cacheDir: cacheDir}, nil
}

func (m *Manager) Get(source string) (GetResult, error) {
	normSource, err := normalizeSource(source)
	if err != nil {
		return GetResult{}, err
	}
	if err := m.ensureCacheLayout(); err != nil {
		return GetResult{}, err
	}
	key := cacheKey(normSource)
	repoPath := filepath.Join(m.reposDir(), key)
	result := GetResult{Source: normSource, CacheKey: key, Path: repoPath}

	hit, err := hasManifest(repoPath)
	if err != nil {
		return GetResult{}, fmt.Errorf("check cached package: %w", err)
	}
	if !hit {
		if err := os.RemoveAll(repoPath); err != nil {
			return GetResult{}, fmt.Errorf("reset cache entry %s: %w", repoPath, err)
		}
		if err := gitClone(normSource, repoPath); err != nil {
			return GetResult{}, err
		}
		manifestPath := filepath.Join(repoPath, manifestFileName)
		if _, err := os.Stat(manifestPath); err != nil {
			if os.IsNotExist(err) {
				_ = os.RemoveAll(repoPath)
				return GetResult{}, fmt.Errorf("fetched repository has no %s", manifestFileName)
			}
			return GetResult{}, fmt.Errorf("read %s: %w", manifestPath, err)
		}
		result.Hit = false
	} else {
		result.Hit = true
	}

	manifestPath := filepath.Join(repoPath, manifestFileName)
	metadata, err := loadManifestMetadata(manifestPath)
	if err != nil {
		if !result.Hit {
			_ = os.RemoveAll(repoPath)
		}
		return GetResult{}, err
	}
	result.Manifest = metadata
	result.Name = metadata.Name
	result.Version = metadata.Version
	result.Head, _ = gitHead(repoPath)

	if err := m.writeIndexEntry(result); err != nil {
		return GetResult{}, err
	}
	return result, nil
}

func (m *Manager) List() ([]Entry, error) {
	if err := m.ensureCacheLayout(); err != nil {
		return nil, err
	}
	idx, err := m.readIndex()
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(idx.Entries))
	for _, entry := range idx.Entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Source < entries[j].Source
	})
	return entries, nil
}

func (m *Manager) CacheDir() string {
	return m.cacheDir
}

func (m *Manager) Sync(projectRoot string) (SyncResult, error) {
	if projectRoot == "" {
		projectRoot = "."
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return SyncResult{}, fmt.Errorf("resolve project root: %w", err)
	}
	manifestPath := filepath.Join(absRoot, manifestFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return SyncResult{}, fmt.Errorf("project manifest not found: %s", manifestPath)
		}
		return SyncResult{}, fmt.Errorf("read project manifest %s: %w", manifestPath, err)
	}
	manifest, err := loadManifestMetadata(manifestPath)
	if err != nil {
		return SyncResult{}, err
	}
	if err := validateSyncDependencies(manifest.Dependencies); err != nil {
		return SyncResult{}, err
	}
	result := SyncResult{
		ProjectPath:          absRoot,
		ManifestPath:         manifestPath,
		Dependencies:         make([]SyncDependencyResult, 0, len(manifest.Dependencies)),
		RegistryDependencies: make([]RegistrySyncResult, 0, len(manifest.Dependencies)),
	}
	planner := &syncPlanner{
		manager:      m,
		projectRoot:  absRoot,
		rootName:     manifest.Name,
		result:       &result,
		seenName:     map[string]plannedNode{},
		state:        map[string]string{},
		explicitSeen: map[string]bool{},
	}
	if planner.rootName == "" {
		planner.rootName = "App"
	}
	deps := sortedDependencies(manifest.Dependencies)
	for _, dep := range deps {
		if isBuiltinDependency(dep) {
			result.SkippedBuiltinPackages = append(result.SkippedBuiltinPackages, dep)
			continue
		}
		chain := []string{planner.rootName, depLabel(dep)}
		if strings.TrimSpace(dep.Source) != "" {
			if err := planner.syncExplicitDependency(dep, chain); err != nil {
				return SyncResult{}, err
			}
			continue
		}
		if err := planner.syncRegistryDependency(dep, chain); err != nil {
			return SyncResult{}, err
		}
	}
	return result, nil
}

type plannedNode struct {
	Name    string
	Version string
	Chain   []string
}

type syncPlanner struct {
	manager      *Manager
	projectRoot  string
	rootName     string
	result       *SyncResult
	seenName     map[string]plannedNode
	state        map[string]string
	explicitSeen map[string]bool
}

func (p *syncPlanner) syncRegistryDependency(dep DependencyMetadata, chain []string) error {
	if err := ValidateExactVersion(dep.VersionRequirement); err != nil {
		return fmt.Errorf("dependency %s required by %s: %w", dep.Name, formatChain(chain), err)
	}
	node := plannedNode{Name: dep.Name, Version: dep.VersionRequirement, Chain: append([]string(nil), chain...)}
	if prior, ok := p.seenName[node.Name]; ok && prior.Version != node.Version {
		return fmt.Errorf("conflicting exact versions for dependency %s: %s required by %s and %s required by %s", node.Name, prior.Version, formatChain(prior.Chain), node.Version, formatChain(node.Chain))
	}
	p.seenName[node.Name] = node
	key := nodeKey(node.Name, node.Version)
	if p.state[key] == "done" {
		return nil
	}
	if p.state[key] == "visiting" {
		return fmt.Errorf("dependency cycle detected: %s", formatCycle(chain, node.Name, node.Version))
	}
	p.state[key] = "visiting"
	resolved, err := ResolveRegistryPackage(p.projectRoot, node.Name, node.Version, "")
	if err != nil {
		return fmt.Errorf("registry dependency %s required by %s: %w", key, formatChain(chain), err)
	}
	synced, err := syncResolvedRegistryPackage(p.projectRoot, resolved)
	if err != nil {
		return fmt.Errorf("dependency %s required by %s: %w", key, formatChain(chain), err)
	}
	manifest, err := LoadManifestMetadata(filepath.Join(synced.Destination, manifestFileName))
	if err != nil {
		return fmt.Errorf("load synced manifest for %s required by %s: %w", key, formatChain(chain), err)
	}
	for _, child := range sortedDependencies(manifest.Dependencies) {
		if isBuiltinDependency(child) {
			p.result.SkippedBuiltinPackages = append(p.result.SkippedBuiltinPackages, child)
			continue
		}
		childChain := append(append([]string(nil), chain...), depLabel(child))
		if strings.TrimSpace(child.Source) != "" {
			if err := p.syncExplicitDependency(child, childChain); err != nil {
				return err
			}
			continue
		}
		if err := p.syncRegistryDependency(child, childChain); err != nil {
			return err
		}
	}
	p.state[key] = "done"
	synced.Chain = append([]string(nil), chain...)
	p.result.RegistryDependencies = append(p.result.RegistryDependencies, synced)
	return nil
}

func (p *syncPlanner) syncExplicitDependency(dep DependencyMetadata, chain []string) error {
	key := dep.Name + "\x00" + dep.Source
	if p.explicitSeen[key] {
		return nil
	}
	p.explicitSeen[key] = true
	getResult, err := p.manager.Get(dep.Source)
	if err != nil {
		return fmt.Errorf("dependency %s required by %s: %w", dep.Name, formatChain(chain), err)
	}
	p.result.Dependencies = append(p.result.Dependencies, SyncDependencyResult{Name: dep.Name, VersionRequirement: dep.VersionRequirement, Source: dep.Source, GetResult: getResult})
	return nil
}

func sortedDependencies(deps []DependencyMetadata) []DependencyMetadata {
	out := append([]DependencyMetadata(nil), deps...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].VersionRequirement != out[j].VersionRequirement {
			return out[i].VersionRequirement < out[j].VersionRequirement
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func depLabel(dep DependencyMetadata) string {
	if strings.TrimSpace(dep.VersionRequirement) == "" {
		return dep.Name
	}
	return dep.Name + "@" + dep.VersionRequirement
}

func nodeKey(name string, version string) string {
	return name + "@" + version
}

func formatChain(chain []string) string {
	return strings.Join(chain, " -> ")
}

func formatCycle(chain []string, name string, version string) string {
	target := nodeKey(name, version)
	for idx, part := range chain {
		if part == target {
			return formatChain(append(append([]string(nil), chain[idx:]...), target))
		}
	}
	return formatChain(append(append([]string(nil), chain...), target))
}

func validateSyncDependencies(deps []DependencyMetadata) error {
	seen := map[string]DependencyMetadata{}
	for idx, dep := range deps {
		if strings.TrimSpace(dep.Name) == "" {
			return fmt.Errorf("dependency at index %d has empty Name", idx)
		}
		if strings.TrimSpace(dep.VersionRequirement) == "" {
			return fmt.Errorf("dependency %q has empty VersionRequirement", dep.Name)
		}
		if prior, ok := seen[dep.Name]; ok {
			if prior.VersionRequirement != dep.VersionRequirement || prior.Source != dep.Source {
				return fmt.Errorf("dependency %q declared more than once with conflicting metadata", dep.Name)
			}
			return fmt.Errorf("dependency %q declared more than once", dep.Name)
		}
		seen[dep.Name] = dep
	}
	return nil
}

func (m *Manager) ensureCacheLayout() error {
	if err := os.MkdirAll(m.reposDir(), 0o755); err != nil {
		return fmt.Errorf("create package cache directory %s: %w", m.reposDir(), err)
	}
	if _, err := os.Stat(m.indexPath()); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read package cache index: %w", err)
		}
		empty := index{Entries: map[string]Entry{}}
		if err := writeIndex(m.indexPath(), empty); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) reposDir() string {
	return filepath.Join(m.cacheDir, "repos")
}

func (m *Manager) indexPath() string {
	return filepath.Join(m.cacheDir, "index.json")
}

func (m *Manager) readIndex() (index, error) {
	body, err := os.ReadFile(m.indexPath())
	if err != nil {
		return index{}, fmt.Errorf("read package cache index %s: %w", m.indexPath(), err)
	}
	var idx index
	if err := json.Unmarshal(body, &idx); err != nil {
		return index{}, fmt.Errorf("package cache metadata corrupted: %w", err)
	}
	if idx.Entries == nil {
		idx.Entries = map[string]Entry{}
	}
	return idx, nil
}

func (m *Manager) writeIndexEntry(result GetResult) error {
	idx, err := m.readIndex()
	if err != nil {
		return err
	}
	idx.Entries[result.CacheKey] = Entry{
		Source:       result.Source,
		CacheKey:     result.CacheKey,
		Path:         result.Path,
		Name:         result.Name,
		Version:      result.Version,
		Dependencies: append([]DependencyMetadata(nil), result.Manifest.Dependencies...),
		Head:         result.Head,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	return writeIndex(m.indexPath(), idx)
}

func writeIndex(path string, idx index) error {
	body, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("encode package cache index: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write package cache index: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("update package cache index: %w", err)
	}
	return nil
}

func normalizeSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("invalid package source URL: source is empty")
	}
	if normalizedFileSource, ok := normalizeWindowsFileSource(source); ok {
		source = normalizedFileSource
	}
	u, err := url.Parse(source)
	if err != nil {
		return "", fmt.Errorf("invalid package source URL: %w", err)
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("invalid package source URL: missing scheme")
	}
	switch u.Scheme {
	case "https", "http", "ssh", "git", "file":
	default:
		return "", fmt.Errorf("invalid package source URL: unsupported scheme %q", u.Scheme)
	}
	if u.Scheme == "file" {
		return normalizeFileURL(u), nil
	}
	return source, nil
}

func normalizeWindowsFileSource(source string) (string, bool) {
	const filePrefix = "file://"
	if !strings.HasPrefix(strings.ToLower(source), filePrefix) {
		return "", false
	}
	pathPart := source[len(filePrefix):]
	if len(pathPart) < 3 {
		return "", false
	}
	if !isWindowsDrivePath(pathPart) {
		return "", false
	}
	return fileURLFromPath(pathPart), true
}

func isWindowsDrivePath(path string) bool {
	drive := path[0]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) {
		return false
	}
	if path[1] != ':' {
		return false
	}
	return path[2] == '\\' || path[2] == '/'
}

func normalizeFileURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	if u.Host != "" && isWindowsDrivePath(u.Host+u.Path) {
		return fileURLFromPath(u.Host + u.Path)
	}
	return u.String()
}

func fileURLFromPath(path string) string {
	slashed := strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
	if isWindowsDrivePath(path) && !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed}).String()
}

func cacheKey(source string) string {
	hash := sha256.Sum256([]byte(source))
	return hex.EncodeToString(hash[:16])
}

func hasManifest(repoPath string) (bool, error) {
	_, err := os.Stat(filepath.Join(repoPath, manifestFileName))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func gitClone(source string, repoPath string) error {
	return gitCloneConfigCheckout(source, repoPath, "HEAD", []string{"--depth", "1"}, directGitCommandRunner(source))
}

type gitCommandRunner func(operation string, name string, args ...string) (string, error)

func gitCloneConfigCheckout(source string, repoPath string, ref string, cloneArgs []string, runner gitCommandRunner) error {
	if strings.TrimSpace(ref) == "" {
		ref = "HEAD"
	}
	args := []string{"-c", "core.autocrlf=false", "clone", "--no-checkout"}
	args = append(args, cloneArgs...)
	args = append(args, source, repoPath)
	if _, err := runner("clone", "git", args...); err != nil {
		return err
	}
	if _, err := runner("configure core.autocrlf", "git", "-C", repoPath, "config", "core.autocrlf", "false"); err != nil {
		return err
	}
	if _, err := runner("configure core.eol", "git", "-C", repoPath, "config", "core.eol", "lf"); err != nil {
		return err
	}
	if _, err := runner("checkout", "git", "-C", repoPath, "checkout", "--detach", ref); err != nil {
		return err
	}
	return nil
}

func directGitCommandRunner(source string) gitCommandRunner {
	return func(operation string, name string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		output, err := cmd.CombinedOutput()
		trimmed := strings.TrimSpace(string(output))
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				return "", fmt.Errorf("git executable not found while fetching %s during %s", source, operation)
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return "", fmt.Errorf("git %s timed out for %s", operation, source)
			}
			if trimmed == "" {
				trimmed = err.Error()
			}
			return "", fmt.Errorf("git %s failed for %s: %s", operation, source, trimmed)
		}
		return trimmed, nil
	}
}

func gitHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read git HEAD: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func isBuiltinDependency(dep DependencyMetadata) bool {
	return dep.Name == "OctStd" && strings.TrimSpace(dep.Source) == ""
}
