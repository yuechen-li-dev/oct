package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCanonicalRegistryDogfoodMathematicsAddSyncTestAndLockedSync(t *testing.T) {
	root := canonicalRegistryCLIRepoRoot(t)
	workspace := t.TempDir()
	consumerDir := filepath.Join(workspace, "PackageRegistryDogfood")
	copyDir(t, filepath.Join(root, "examples", "PackageRegistryDogfood"), consumerDir)

	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "oct", filepath.Join(root, "Registry"))
	if err != nil || !strings.Contains(stdout, "Added package registry oct") {
		t.Fatalf("registry add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "add", "Mathematics@0.1.0")
	if err != nil || !strings.Contains(stdout, "Added dependency Mathematics 0.1.0 from registry oct") {
		t.Fatalf("pkg add failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil || !strings.Contains(stdout, "Synced Mathematics 0.1.0") {
		t.Fatalf("pkg sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "Mathematics", "0.1.0", "manifest.oct")); err != nil {
		t.Fatalf("missing synced Mathematics manifest: %v", err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "test", ".")
	if err != nil || !strings.Contains(stdout, "PASS PackageRegistryDogfood.UsesCanonicalMathematicsPackage") {
		t.Fatalf("oct test failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}

	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "lock")
	if err != nil || !strings.Contains(stdout, "Wrote lock.octagon") {
		t.Fatalf("pkg lock failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if err := os.RemoveAll(filepath.Join(consumerDir, ".oct", "packages")); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "pkg", "sync", "--locked")
	if err != nil || !strings.Contains(stdout, "Synced Mathematics 0.1.0") {
		t.Fatalf("locked sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	stdout, stderr, err = executeCLIInDir(consumerDir, "test", ".")
	if err != nil || !strings.Contains(stdout, "PASS PackageRegistryDogfood.UsesCanonicalMathematicsPackage") {
		t.Fatalf("oct test after locked sync failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func TestCanonicalRegistryWrapperSyncDoesNotBuildSidecars(t *testing.T) {
	root := canonicalRegistryCLIRepoRoot(t)
	consumerDir := filepath.Join(t.TempDir(), "WrapperConsumer")
	copyDir(t, filepath.Join(root, "examples", "PackageRegistryDogfood"), consumerDir)

	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "registry", "add", "oct", filepath.Join(root, "Registry")); err != nil {
		t.Fatalf("registry add failed: %v %s", err, stderr)
	}
	if _, stderr, err := executeCLIInDir(consumerDir, "pkg", "add", "Archive@0.1.0"); err != nil {
		t.Fatalf("pkg add wrapper failed: %v %s", err, stderr)
	}
	stdout, stderr, err := executeCLIInDir(consumerDir, "pkg", "sync")
	if err != nil || !strings.Contains(stdout, "Synced Archive 0.1.0") {
		t.Fatalf("pkg sync wrapper failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "packages", "Archive", "0.1.0", "manifest.oct")); err != nil {
		t.Fatalf("expected wrapper source manifest to be synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(consumerDir, ".oct", "wrappers")); !os.IsNotExist(err) {
		t.Fatalf("pkg sync must not create .oct/wrappers, err=%v", err)
	}
}

func canonicalRegistryCLIRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
