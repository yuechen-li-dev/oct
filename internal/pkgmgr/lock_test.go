package pkgmgr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleLock() PackageLock {
	return PackageLock{LockVersion: CurrentLockVersion, GeneratedBy: LockGeneratedBy, Root: LockRoot{Name: "Consumer", Version: "0.1.0"}, Packages: []LockPackage{{Name: "MathCore", Version: "0.2.0", Kind: "library", SourceKind: "git", Source: "https://example.invalid/math.git", Ref: "v0.2.0", ResolvedCommit: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", Path: ".", Registry: "local", RegistryPath: "../registry", Mutable: false}, {Name: "SignalTools", Version: "0.1.0", Kind: "library", SourceKind: "local", Source: "../SignalTools", Path: ".", Registry: "local", RegistryPath: "../registry", Mutable: true, Dependencies: []LockDependency{{Name: "MathCore", Version: "0.2.0"}}}}}
}

func TestPackageLockRoundTripAndDeterministicOutput(t *testing.T) {
	lock := sampleLock()
	first, err := RenderPackageLockOctagon(lock)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderPackageLockOctagon(lock)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("lock output should be deterministic")
	}
	if strings.Contains(first, "package ") || strings.Contains(first, "GeneratedAt") || strings.Contains(first, "Timestamp") {
		t.Fatalf("lock should be data-only and timestamp-free:\n%s", first)
	}
	path := filepath.Join(t.TempDir(), LockFileName)
	if err := WritePackageLock(path, lock); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPackageLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Packages) != 2 || loaded.Packages[0].Name != "MathCore" || !loaded.Packages[1].Mutable {
		t.Fatalf("unexpected loaded lock: %#v", loaded)
	}
}

func TestPackageLockLoadValidation(t *testing.T) {
	cases := []struct{ name, mutate, want string }{
		{"missing field", strings.Replace(RenderLockForTest(t, sampleLock()), "    GeneratedBy: \"oct pkg lock\"\n", "", 1), "missing required field GeneratedBy"},
		{"unsupported version", strings.Replace(RenderLockForTest(t, sampleLock()), "LockVersion: 1", "LockVersion: 2", 1), "unsupported LockVersion"},
		{"invalid git commit", strings.Replace(RenderLockForTest(t, sampleLock()), "abcdefabcdefabcdefabcdefabcdefabcdefabcd", "abc", 1), "full 40-character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), LockFileName)
			if err := os.WriteFile(path, []byte(tc.mutate), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadPackageLock(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPackageLockGraphValidation(t *testing.T) {
	base := sampleLock()
	cases := []struct {
		name string
		edit func(*PackageLock)
		want string
	}{
		{"duplicate", func(l *PackageLock) { l.Packages = append(l.Packages, l.Packages[0]) }, "duplicate"},
		{"conflict", func(l *PackageLock) { p := l.Packages[0]; p.Version = "9.9.9"; l.Packages = append(l.Packages, p) }, "conflicting"},
		{"cycle", func(l *PackageLock) {
			l.Packages[0].Dependencies = []LockDependency{{Name: "SignalTools", Version: "0.1.0"}}
		}, "cycle"},
		{"missing", func(l *PackageLock) {
			l.Packages[1].Dependencies = []LockDependency{{Name: "Missing", Version: "0.1.0"}}
		}, "missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lock := base
			lock.Packages = append([]LockPackage(nil), base.Packages...)
			tc.edit(&lock)
			err := ValidateLock(lock)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPackageLockManifestDriftValidation(t *testing.T) {
	lock := sampleLock()
	manifest := ManifestMetadata{Name: "Consumer", Version: "0.1.0", Dependencies: []DependencyMetadata{{Name: "SignalTools", VersionRequirement: "0.1.0"}}}
	if err := ValidateLockAgainstManifest(lock, manifest); err != nil {
		t.Fatalf("valid manifest should match lock: %v", err)
	}
	cases := []struct {
		name     string
		manifest ManifestMetadata
		want     string
	}{
		{"added", ManifestMetadata{Name: "Consumer", Version: "0.1.0", Dependencies: []DependencyMetadata{{Name: "SignalTools", VersionRequirement: "0.1.0"}, {Name: "Extra", VersionRequirement: "0.1.0"}}}, "Extra@0.1.0 is not locked"},
		{"removed", ManifestMetadata{Name: "Consumer", Version: "0.1.0", Dependencies: nil}, "no longer reachable"},
		{"changed", ManifestMetadata{Name: "Consumer", Version: "0.1.0", Dependencies: []DependencyMetadata{{Name: "SignalTools", VersionRequirement: "0.2.0"}}}, "lock contains SignalTools@0.1.0"},
		{"root", ManifestMetadata{Name: "Other", Version: "0.1.0", Dependencies: []DependencyMetadata{{Name: "SignalTools", VersionRequirement: "0.1.0"}}}, "does not match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLockAgainstManifest(lock, tc.manifest)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPackageLockLocalAndGitPolicy(t *testing.T) {
	lock := sampleLock()
	if !lock.Packages[1].Mutable || lock.Packages[1].ResolvedCommit != "" {
		t.Fatalf("local package should be mutable and unpinned: %#v", lock.Packages[1])
	}
	if !IsFullCommitSHA(lock.Packages[0].ResolvedCommit) || lock.Packages[0].Mutable {
		t.Fatalf("git package should use resolved commit as authority: %#v", lock.Packages[0])
	}
}

func RenderLockForTest(t *testing.T, lock PackageLock) string {
	t.Helper()
	rendered, err := RenderPackageLockOctagon(lock)
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}
