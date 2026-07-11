package build

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ArtifactKind describes the ownership and operating-system format of compiler output.
// Native artifacts deliberately use their target platform's conventional suffix;
// .octbin is reserved for a future Oct-owned portable artifact format.
type ArtifactKind int

const (
	ArtifactExecutable ArtifactKind = iota
	ArtifactSharedLibrary
	ArtifactStaticLibrary
	ArtifactPortableBinary
	ArtifactMIR
	ArtifactObject
	ArtifactTestExecutable
)

// Target identifies the output platform used when naming an artifact.
type Target struct{ GOOS string }

func HostTarget() Target { return Target{GOOS: runtime.GOOS} }

// OutputPath returns the native output path for base. base is a stem, not a
// source filename: callers that pass "Main.oct" therefore get "Main.oct.exe".
func OutputPath(base string, kind ArtifactKind, target Target) string {
	ext := artifactExtension(kind, target.GOOS)
	if ext == "" || strings.EqualFold(filepath.Ext(base), ext) {
		return base
	}
	return base + ext
}

func artifactExtension(kind ArtifactKind, goos string) string {
	switch kind {
	case ArtifactPortableBinary:
		return ".octbin"
	case ArtifactSharedLibrary:
		if goos == "windows" {
			return ".dll"
		}
		if goos == "darwin" {
			return ".dylib"
		}
		return ".so"
	case ArtifactStaticLibrary:
		if goos == "windows" {
			return ".lib"
		}
		return ".a"
	case ArtifactExecutable, ArtifactTestExecutable:
		if goos == "windows" {
			return ".exe"
		}
	}
	return ""
}

// TestArtifactPaths is the owned, per-run layout for compiled test helpers.
// The generated Go source and native executable intentionally use different
// names so retaining diagnostics can never make go build overwrite its input.
type TestArtifactPaths struct {
	GeneratedSource string
	Binary          string
}

func DeriveTestArtifactPaths(runnerPath string, target Target) (TestArtifactPaths, error) {
	clean := filepath.Clean(runnerPath)
	dir := filepath.Dir(clean)
	name := strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean))
	if name == "" || name == "." {
		return TestArtifactPaths{}, fmt.Errorf("invalid compiled test runner path %q", runnerPath)
	}
	paths := TestArtifactPaths{
		GeneratedSource: filepath.Join(dir, name+".generated.go"),
		Binary:          OutputPath(filepath.Join(dir, name+".octbin"), ArtifactTestExecutable, target),
	}
	if filepath.Clean(paths.GeneratedSource) == filepath.Clean(paths.Binary) {
		return TestArtifactPaths{}, fmt.Errorf("generated source path collides with binary output path: %s", paths.Binary)
	}
	return paths, nil
}
