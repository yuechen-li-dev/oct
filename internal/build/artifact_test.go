package build

import (
	"path/filepath"
	"testing"
)

func TestOutputPathUsesPlatformNativeExtensions(t *testing.T) {
	cases := []struct {
		name, goos, want string
		kind             ArtifactKind
	}{
		{"windows executable", "windows", "app.exe", ArtifactExecutable},
		{"windows test executable", "windows", "app.exe", ArtifactTestExecutable},
		{"windows shared", "windows", "app.dll", ArtifactSharedLibrary},
		{"windows static", "windows", "app.lib", ArtifactStaticLibrary},
		{"linux executable", "linux", "app", ArtifactExecutable},
		{"linux shared", "linux", "app.so", ArtifactSharedLibrary},
		{"linux static", "linux", "app.a", ArtifactStaticLibrary},
		{"mac shared", "darwin", "app.dylib", ArtifactSharedLibrary},
		{"portable", "windows", "app.octbin", ArtifactPortableBinary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OutputPath("app", tc.kind, Target{GOOS: tc.goos}); got != tc.want {
				t.Fatalf("OutputPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeriveTestArtifactPathsSeparatesSourceAndPlatformBinary(t *testing.T) {
	cases := []struct {
		goos, wantBinary string
	}{
		{"windows", "runner.octbin.exe"},
		{"linux", "runner.octbin"},
		{"darwin", "runner.octbin"},
	}
	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			paths, err := DeriveTestArtifactPaths("C:/owned/runner.octest", Target{GOOS: tc.goos})
			if err != nil {
				t.Fatal(err)
			}
			if got := filepath.Base(paths.GeneratedSource); got != "runner.generated.go" {
				t.Fatalf("generated source = %q", got)
			}
			if got := filepath.Base(paths.Binary); got != tc.wantBinary {
				t.Fatalf("binary = %q, want %q", got, tc.wantBinary)
			}
			if filepath.Clean(paths.GeneratedSource) == filepath.Clean(paths.Binary) {
				t.Fatal("source and binary paths collided")
			}
		})
	}
}

func TestProgramOutputPathNeverCollidesWithSource(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "Main.oct")
			output := programOutputPath(source, ArtifactExecutable, Target{GOOS: goos})
			if filepath.Clean(output) == filepath.Clean(source) {
				t.Fatalf("program output %q collides with source %q", output, source)
			}
		})
	}
}
