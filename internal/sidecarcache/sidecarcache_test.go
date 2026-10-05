package sidecarcache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRepo is a repository with one source file per command.
func fakeRepo(t *testing.T, commands ...string) string {
	t.Helper()
	repo := t.TempDir()
	write(t, filepath.Join(repo, "go.mod"), "module example\n")
	write(t, filepath.Join(repo, "go.sum"), "")
	for _, command := range commands {
		write(t, filepath.Join(repo, "cmd", command, "main.go"), "package main // "+command+"\n")
	}
	return repo
}

func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// countingToolchain records each build and writes a recognisable binary.
func countingToolchain(builds *[]string) Toolchain {
	return Toolchain{
		Version: "go-test linux/amd64",
		Sources: func(repo string, commands []string) ([]string, error) {
			var files []string
			for _, command := range commands {
				files = append(files, filepath.Join(repo, "cmd", command, "main.go"))
			}
			return files, nil
		},
		Build: func(repo string, command string, outPath string) error {
			*builds = append(*builds, command)
			source, err := os.ReadFile(filepath.Join(repo, "cmd", command, "main.go"))
			if err != nil {
				return err
			}
			return os.WriteFile(outPath, append([]byte("built from: "), source...), 0o644)
		},
	}
}

func TestEnsureBuildsOnceAndThenUsesTheCache(t *testing.T) {
	repo := fakeRepo(t, "octxiliary-a", "octxiliary-b")
	root := t.TempDir()
	var builds []string
	toolchain := countingToolchain(&builds)

	first, err := EnsureIn(root, repo, []string{"octxiliary-b", "octxiliary-a"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(first.Built, ","); got != "octxiliary-a,octxiliary-b" {
		t.Fatalf("first call built %q", got)
	}
	for _, command := range []string{"octxiliary-a", "octxiliary-b"} {
		info, err := os.Stat(filepath.Join(first.Dir, BinaryName(command)))
		if err != nil {
			t.Fatalf("%s is not in the cache: %v", command, err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("%s is not executable: %v", command, info.Mode())
		}
	}

	second, err := EnsureIn(root, repo, []string{"octxiliary-a", "octxiliary-b"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if second.Dir != first.Dir || len(second.Built) != 0 || len(builds) != 2 {
		t.Fatalf("second call: dir %q built %v, %d builds in total; want the same directory and no new build", second.Dir, second.Built, len(builds))
	}
	entries, _ := os.ReadDir(first.Dir)
	if len(entries) != 2 {
		t.Errorf("cache directory holds %d entries, want the two binaries and no staging file", len(entries))
	}
}

// Every input of a build changes the key. A stale binary must not be reused.
func TestEnsureRebuildsWhenAnInputChanges(t *testing.T) {
	changes := map[string]func(repo string, toolchain *Toolchain){
		"a source file": func(repo string, _ *Toolchain) {
			_ = os.WriteFile(filepath.Join(repo, "cmd", "octxiliary-a", "main.go"), []byte("package main // edited\n"), 0o644)
		},
		"go.mod": func(repo string, _ *Toolchain) {
			_ = os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example\n\ngo 1.25\n"), 0o644)
		},
		"go.sum": func(repo string, _ *Toolchain) {
			_ = os.WriteFile(filepath.Join(repo, "go.sum"), []byte("dep v1 h1:x\n"), 0o644)
		},
		"the toolchain": func(_ string, toolchain *Toolchain) { toolchain.Version = "go-next linux/amd64" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			repo := fakeRepo(t, "octxiliary-a")
			root := t.TempDir()
			var builds []string
			toolchain := countingToolchain(&builds)
			first, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain)
			if err != nil {
				t.Fatal(err)
			}
			change(repo, &toolchain)
			second, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain)
			if err != nil {
				t.Fatal(err)
			}
			if second.Dir == first.Dir || len(builds) != 2 {
				t.Fatalf("after changing %s: same directory %v, %d builds", name, second.Dir == first.Dir, len(builds))
			}
			if _, err := os.Stat(first.Dir); !os.IsNotExist(err) {
				t.Errorf("the directory of the earlier key was not removed: %v", err)
			}
		})
	}
}

// The key does not depend on which sidecars are asked for: callers at one
// source state share a directory, and asking for one more sidecar builds only
// that one.
func TestEnsureSharesOneDirectoryAcrossCommandSets(t *testing.T) {
	repo := fakeRepo(t, "octxiliary-a", "octxiliary-b")
	root := t.TempDir()
	var builds []string
	toolchain := countingToolchain(&builds)
	one, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	two, err := EnsureIn(root, repo, []string{"octxiliary-a", "octxiliary-b"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if one.Dir != two.Dir {
		t.Fatalf("two command sets use different directories")
	}
	if got := strings.Join(two.Built, ","); got != "octxiliary-b" || len(builds) != 2 {
		t.Fatalf("second call built %q, %d builds in total; want only octxiliary-b", got, len(builds))
	}
	if _, err := os.Stat(filepath.Join(one.Dir, BinaryName("octxiliary-a"))); err != nil {
		t.Errorf("the first sidecar was removed: %v", err)
	}
}

// A sidecar another command depends on is part of the key even when it is not
// the one asked for.
func TestEnsureRebuildsWhenAnotherSidecarChanges(t *testing.T) {
	repo := fakeRepo(t, "octxiliary-a", "octxiliary-b")
	root := t.TempDir()
	var builds []string
	toolchain := countingToolchain(&builds)
	first, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(repo, "cmd", "octxiliary-b", "main.go"), "package main // edited\n")
	second, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain)
	if err != nil {
		t.Fatal(err)
	}
	if second.Dir == first.Dir {
		t.Fatalf("the key ignored a change to another sidecar")
	}
}

func TestEnsureRefusesACommandTheRepositoryDoesNotHave(t *testing.T) {
	repo := fakeRepo(t, "octxiliary-a")
	if _, err := EnsureIn(t.TempDir(), repo, []string{"octxiliary-missing"}, countingToolchain(new([]string))); err == nil || !strings.Contains(err.Error(), "octxiliary-missing is not a sidecar command") {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureLeavesNoPartialBinaryWhenABuildFails(t *testing.T) {
	repo := fakeRepo(t, "octxiliary-a")
	root := t.TempDir()
	toolchain := countingToolchain(new([]string))
	toolchain.Build = func(string, string, string) error { return errors.New("compiler said no") }
	if _, err := EnsureIn(root, repo, []string{"octxiliary-a"}, toolchain); err == nil || !strings.Contains(err.Error(), "build octxiliary-a: compiler said no") {
		t.Fatalf("err = %v", err)
	}
	var left []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			left = append(left, path)
		}
		return nil
	})
	if len(left) != 0 {
		t.Errorf("a failed build left %v", left)
	}
}

func TestRootHonoursTheEnvironment(t *testing.T) {
	t.Setenv(EnvDir, filepath.Join("some", "where"))
	if root, err := Root(); err != nil || root != filepath.Join("some", "where") {
		t.Fatalf("Root() = %q, %v", root, err)
	}
}
