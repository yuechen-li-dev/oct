package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestArtifactScopeRemovesOwnedDirectoryAndPartialFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv(testArtifactRootEnv, root)
	scope, err := newArtifactScope("octest-run", nil)
	if err != nil {
		t.Fatalf("create scope: %v", err)
	}
	if err := os.WriteFile(scope.path("runner.octest.octbin"), []byte("partial"), 0o644); err != nil {
		t.Fatalf("write partial artifact: %v", err)
	}
	dir := scope.dir
	if err := scope.close(); err != nil {
		t.Fatalf("close scope: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("owned directory survived cleanup: %s (%v)", dir, err)
	}
}

func TestArtifactScopeRetentionIsExplicitAndReported(t *testing.T) {
	root := t.TempDir()
	t.Setenv(testArtifactRootEnv, root)
	t.Setenv(keepTestArtifactsEnv, "1")
	var diagnostic bytes.Buffer
	scope, err := newArtifactScope("octest-run", &diagnostic)
	if err != nil {
		t.Fatalf("create scope: %v", err)
	}
	if err := os.WriteFile(scope.path("runner.octbin"), []byte("debug"), 0o644); err != nil {
		t.Fatalf("write retained artifact: %v", err)
	}
	if err := scope.close(); err != nil {
		t.Fatalf("retain scope: %v", err)
	}
	if !strings.Contains(diagnostic.String(), scope.dir) {
		t.Fatalf("retained path was not reported: %q", diagnostic.String())
	}
	if _, err := os.Stat(scope.path("runner.octbin")); err != nil {
		t.Fatalf("retained artifact missing: %v", err)
	}
}

func TestArtifactScopesDoNotCollide(t *testing.T) {
	root := t.TempDir()
	t.Setenv(testArtifactRootEnv, root)
	const count = 8
	dirs := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scope, err := newArtifactScope("octest-run", nil)
			if err != nil {
				errs <- err
				return
			}
			dirs <- scope.dir
			errs <- scope.close()
		}()
	}
	wg.Wait()
	close(dirs)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel artifact scope: %v", err)
		}
	}
	seen := map[string]struct{}{}
	for dir := range dirs {
		if _, ok := seen[dir]; ok {
			t.Fatalf("artifact scope collision: %s", dir)
		}
		seen[dir] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("got %d unique scopes, want %d", len(seen), count)
	}
	left, err := filepath.Glob(filepath.Join(root, "octest-run-*"))
	if err != nil {
		t.Fatalf("glob scopes: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("parallel scopes leaked: %v", left)
	}
}
