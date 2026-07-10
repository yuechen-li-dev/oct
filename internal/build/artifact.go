package build

import (
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
