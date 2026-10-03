package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRepoRootSkipsLowercaseImplementationDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "libraries", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Libraries"), 0755); err != nil {
		t.Fatal(err)
	}
	got := detectRepoRoot(filepath.Join(root, "internal", "libraries", "nested"))
	if got != root {
		t.Fatalf("detectRepoRoot() = %q, want %q", got, root)
	}
}

func TestDetectRepoRootsStopsAtTheNearestLibraries(t *testing.T) {
	mk := func(parts ...string) string {
		path := filepath.Join(parts...)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	same := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// A repository with Libraries/, and a fixture domain inside it that
	// happens to be named Packages.
	repo := t.TempDir()
	mk(repo, "Libraries", "Markdown")
	mk(repo, "Language", "Packages", "Local")
	start := mk(repo, "Language", "Functions", "Calls", "valid")
	if got, want := detectRepoRoots(start), []string{filepath.Join(repo, "Language"), repo}; !same(got, want) {
		t.Errorf("nested Packages under Libraries: got %v, want %v", got, want)
	}
	if got := detectRepoRoot(start); got != filepath.Join(repo, "Language") {
		t.Errorf("nearest root: got %q", got)
	}
	if got, want := detectRepoRoots(mk(repo, "Experiments", "Lab", "M0")), []string{repo}; !same(got, want) {
		t.Errorf("ordinary directory: got %v, want %v", got, want)
	}

	// The walk does not go past the first Libraries/.
	outer := t.TempDir()
	mk(outer, "Libraries")
	mk(outer, "Packages")
	inner := mk(outer, "vendor", "project")
	mk(inner, "Libraries")
	if got, want := detectRepoRoots(mk(inner, "src")), []string{inner}; !same(got, want) {
		t.Errorf("nested repository: got %v, want %v", got, want)
	}

	// Without any Libraries/, only the nearest Packages/ counts.
	plain := t.TempDir()
	mk(plain, "Packages")
	project := mk(plain, "work", "project")
	mk(project, "Packages")
	if got, want := detectRepoRoots(mk(project, "src")), []string{project}; !same(got, want) {
		t.Errorf("no Libraries anywhere: got %v, want %v", got, want)
	}
}
