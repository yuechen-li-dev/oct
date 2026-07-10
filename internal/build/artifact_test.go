package build

import "testing"

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
