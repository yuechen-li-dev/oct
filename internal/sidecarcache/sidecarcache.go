// Package sidecarcache keeps built Octxiliary sidecars between test runs.
//
// A sidecar is built once into a directory named by a key, and later runs use
// the binaries in place. The key covers everything a sidecar is built from:
// the Go toolchain, the platform, the module's go.mod and go.sum, and the
// source files of every package of this module that any sidecar depends on.
// A change to any of them is a different key, so a stale binary is never
// reused; nothing has to be invalidated by hand.
//
// The key does not depend on which sidecars a caller asks for. Every caller
// at one source state shares one directory and fills it as needed.
package sidecarcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// EnvDir overrides where the cache lives. The default is the user's cache
// directory.
const EnvDir = "OCT_SIDECAR_CACHE_DIR"

// Result reports what Ensure did.
type Result struct {
	// Dir holds the requested sidecars; pass it as OCT_WRAPPER_PATH.
	Dir string
	// Built names the sidecars that were compiled by this call. It is empty
	// when every sidecar came from the cache.
	Built []string
}

// Toolchain is the part of the work that runs the Go toolchain. Tests replace
// it; DefaultToolchain is the real one.
type Toolchain struct {
	// Sources lists the files the named commands are built from, as absolute
	// paths.
	Sources func(repo string, commands []string) ([]string, error)
	// Build compiles one command of the repository to outPath.
	Build func(repo string, command string, outPath string) error
	// Version identifies the compiler and the platform it targets.
	Version string
}

// DefaultToolchain builds with the `go` command found on PATH.
func DefaultToolchain() Toolchain {
	return Toolchain{
		Sources: goSources,
		Build:   goBuild,
		Version: runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// Ensure returns a directory that holds the named sidecar commands (for
// example "octxiliary-io"), building the ones that are missing.
func Ensure(repo string, commands []string) (Result, error) {
	root, err := Root()
	if err != nil {
		return Result{}, err
	}
	return EnsureIn(root, repo, commands, DefaultToolchain())
}

// Root is the directory that holds one subdirectory per cache key.
func Root() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(EnvDir)); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate the user cache directory (set %s to choose one): %w", EnvDir, err)
	}
	return filepath.Join(base, "oct", "sidecars"), nil
}

// EnsureIn is Ensure with the cache root and the toolchain given.
func EnsureIn(root string, repo string, commands []string, toolchain Toolchain) (Result, error) {
	repoAbs, err := filepath.Abs(repo)
	if err != nil {
		return Result{}, fmt.Errorf("resolve repository %s: %w", repo, err)
	}
	commands = append([]string(nil), commands...)
	sort.Strings(commands)

	all, err := sidecarCommands(repoAbs)
	if err != nil {
		return Result{}, err
	}
	for _, command := range commands {
		if !contains(all, command) {
			return Result{}, fmt.Errorf("%s is not a sidecar command of this repository (no cmd/%s)", command, command)
		}
	}
	key, err := cacheKey(repoAbs, all, toolchain)
	if err != nil {
		return Result{}, err
	}
	dir := filepath.Join(root, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create sidecar cache %s: %w", dir, err)
	}

	result := Result{Dir: dir}
	for _, command := range commands {
		target := filepath.Join(dir, BinaryName(command))
		if info, statErr := os.Stat(target); statErr == nil && info.Mode().IsRegular() {
			continue
		}
		// Build beside the target and rename, so that a run that reads the
		// cache while another fills it never sees half a binary.
		staging, err := os.CreateTemp(dir, command+".building-*")
		if err != nil {
			return Result{}, fmt.Errorf("stage %s: %w", command, err)
		}
		stagingPath := staging.Name()
		_ = staging.Close()
		if err := toolchain.Build(repoAbs, command, stagingPath); err != nil {
			_ = os.Remove(stagingPath)
			return Result{}, fmt.Errorf("build %s: %w", command, err)
		}
		if err := os.Chmod(stagingPath, 0o755); err != nil {
			_ = os.Remove(stagingPath)
			return Result{}, fmt.Errorf("mark %s executable: %w", command, err)
		}
		if err := os.Rename(stagingPath, target); err != nil {
			_ = os.Remove(stagingPath)
			return Result{}, fmt.Errorf("publish %s: %w", command, err)
		}
		result.Built = append(result.Built, command)
	}
	if len(result.Built) > 0 {
		pruneOtherKeys(root, key)
	}
	return result, nil
}

// sidecarCommands lists the sidecar commands of the repository: the
// directories under cmd/ whose name starts with "octxiliary-".
func sidecarCommands(repo string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(repo, "cmd"))
	if err != nil {
		return nil, fmt.Errorf("list sidecar commands: %w", err)
	}
	var commands []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "octxiliary-") {
			commands = append(commands, entry.Name())
		}
	}
	sort.Strings(commands)
	return commands, nil
}

func contains(items []string, item string) bool {
	for _, candidate := range items {
		if candidate == item {
			return true
		}
	}
	return false
}

// BinaryName is the file name of a sidecar command on this platform.
func BinaryName(command string) string {
	if runtime.GOOS == "windows" {
		return command + ".exe"
	}
	return command
}

// cacheKey hashes everything the sidecar commands are built from.
func cacheKey(repo string, commands []string, toolchain Toolchain) (string, error) {
	files, err := toolchain.Sources(repo, commands)
	if err != nil {
		return "", fmt.Errorf("list sidecar sources: %w", err)
	}
	files = append(files, filepath.Join(repo, "go.mod"), filepath.Join(repo, "go.sum"))
	sort.Strings(files)

	hash := sha256.New()
	fmt.Fprintf(hash, "toolchain %s\ncommands %s\n", toolchain.Version, strings.Join(commands, ","))
	previous := ""
	for _, file := range files {
		if file == previous {
			continue
		}
		previous = file
		rel, relErr := filepath.Rel(repo, file)
		if relErr != nil {
			rel = file
		}
		handle, err := os.Open(file)
		if err != nil {
			if os.IsNotExist(err) && (strings.HasSuffix(file, "go.sum")) {
				continue
			}
			return "", fmt.Errorf("read sidecar source %s: %w", file, err)
		}
		fmt.Fprintf(hash, "file %s\n", filepath.ToSlash(rel))
		_, copyErr := io.Copy(hash, handle)
		_ = handle.Close()
		if copyErr != nil {
			return "", fmt.Errorf("read sidecar source %s: %w", file, copyErr)
		}
		fmt.Fprint(hash, "\n")
	}
	return hex.EncodeToString(hash.Sum(nil))[:24], nil
}

// pruneOtherKeys removes the directories of earlier keys. It runs only after
// a build, so a cache that is being read is left alone, and it is best
// effort: a directory another process still uses is simply kept.
func pruneOtherKeys(root string, keep string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != keep {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

func goSources(repo string, commands []string) ([]string, error) {
	args := []string{"list", "-deps", "-f", `{{if .Module}}{{if .Module.Main}}{{.Dir}}|{{join .GoFiles ","}}|{{join .CgoFiles ","}}|{{join .EmbedFiles ","}}{{end}}{{end}}`}
	for _, command := range commands {
		args = append(args, "./cmd/"+command)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("go list: %w\n%s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("go list: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 4 || parts[0] == "" {
			continue
		}
		for _, group := range parts[1:] {
			for _, name := range strings.Split(group, ",") {
				if name != "" {
					files = append(files, filepath.Join(parts[0], name))
				}
			}
		}
	}
	return files, nil
}

func goBuild(repo string, command string, outPath string) error {
	cmd := exec.Command("go", "build", "-o", outPath, "./cmd/"+command)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
