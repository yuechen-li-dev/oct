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
	"regexp"
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
}

type Entry struct {
	Source    string `json:"source"`
	CacheKey  string `json:"cache_key"`
	Path      string `json:"path"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Head      string `json:"head,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

type index struct {
	Entries map[string]Entry `json:"entries"`
}

func NewManager() (*Manager, error) {
	cacheDir := os.Getenv(envCacheDir)
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

	result.Name, result.Version = readManifestIdentity(filepath.Join(repoPath, manifestFileName))
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
		Source:    result.Source,
		CacheKey:  result.CacheKey,
		Path:      result.Path,
		Name:      result.Name,
		Version:   result.Version,
		Head:      result.Head,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
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
	return source, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", source, repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git clone timed out for %s", source)
		}
		return fmt.Errorf("git fetch/clone failed for %s: %s", source, strings.TrimSpace(string(output)))
	}
	return nil
}

func gitHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read git HEAD: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

var (
	nameLine    = regexp.MustCompile(`(?m)\bName\s*:\s*"([^"]+)"`)
	versionLine = regexp.MustCompile(`(?m)\bVersion\s*:\s*"([^"]+)"`)
)

func readManifestIdentity(path string) (string, string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	name := ""
	version := ""
	if m := nameLine.FindSubmatch(body); len(m) == 2 {
		name = string(m[1])
	}
	if m := versionLine.FindSubmatch(body); len(m) == 2 {
		version = string(m[1])
	}
	return name, version
}
