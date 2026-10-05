//go:build integration

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The Language corpus is the language's contract, and a contract nobody runs
// stops being one. This test runs every directory under Language that holds
// `.octest` files, in the interpreted lane and in the compiled lane, and
// runs every `.octfail` under Language.
//
// A directory that is not run that way is listed below with the reason. The
// list is checked as well: an entry for a directory that no longer exists, or
// for a failure that no longer happens, fails the test, so the list cannot
// outlive what it describes.

type corpusLane string

const (
	laneInterpreted corpusLane = "interpreted"
	laneCompiled    corpusLane = "compiled"
)

// A package set holds several packages that import one another. It is run
// from its root; its member directories are not targets on their own.
var languagePackageSets = []string{
	"Language/Packages/CrossPackageM81/valid",
	"Language/Types/ParametricsM0/packages",
	"Language/Types/TemplateTortureM0/packages",
}

// Directories another test owns, with the reason this test cannot run them.
var languageRunElsewhere = map[string]string{
	"Language/Tooling/ConceptCapabilitiesM2/valid": "artifact entry points that are run with native grants, one of them to be refused; internal/tester/artifact_phase_test.go",
}

// Directories whose tests call a wrapper sidecar. They are run with
// OCT_WRAPPER_PATH set to the sidecar cache (internal/sidecarcache), which
// builds a sidecar the first time it is asked for and reuses it afterwards.
// Every other directory runs with no sidecar available.
var languageSidecars = map[string][]string{
	"Language/Runtime/LibraryBuiltins/valid":       {"octxiliary-hash", "octxiliary-text"},
	"Language/Testing/CompiledOctxiliary/valid":    {"octxiliary-test-wrapper", "octxiliary-io"},
	"Language/Testing/InterpretedOctxiliary/valid": {"octxiliary-test-wrapper"},
	"Language/Types/Bytes/valid":                   {"octxiliary-io"},
}

// A known gap is a directory that fails in one lane because that lane lacks a
// feature. The failure is required: when the lane gains the feature the entry
// fails this test and has to be removed.
type corpusGap struct {
	directory string
	lane      corpusLane
	contains  string
}

var languageKnownGaps = []corpusGap{}

var corpusFactAttribute = regexp.MustCompile(`(?m)^\s*\[(Fact|Theory)\b`)

func TestLanguageCorpusRunsInBothLanes(t *testing.T) {
	repo := filepath.Join("..", "..")
	directories := languageOctestDirectories(t, repo)

	isSetMember := func(directory string) bool {
		for _, set := range languagePackageSets {
			if strings.HasPrefix(directory, set+"/") {
				return true
			}
		}
		return false
	}
	gapFor := func(directory string, lane corpusLane) (corpusGap, bool) {
		for _, gap := range languageKnownGaps {
			if gap.directory == directory && gap.lane == lane {
				return gap, true
			}
		}
		return corpusGap{}, false
	}

	seen := map[string]bool{}
	var targets []string
	for _, directory := range directories {
		seen[directory] = true
		if _, elsewhere := languageRunElsewhere[directory]; elsewhere || isSetMember(directory) {
			continue
		}
		targets = append(targets, directory)
	}
	for _, set := range languagePackageSets {
		if info, err := os.Stat(filepath.Join(repo, filepath.FromSlash(set))); err != nil || !info.IsDir() {
			t.Errorf("package set %s does not exist; remove it from languagePackageSets", set)
			continue
		}
		targets = append(targets, set)
	}
	for directory := range languageRunElsewhere {
		if !seen[directory] {
			t.Errorf("%s holds no .octest files; remove it from languageRunElsewhere", directory)
		}
	}
	for _, gap := range languageKnownGaps {
		if !seen[gap.directory] {
			t.Errorf("%s holds no .octest files; remove it from languageKnownGaps", gap.directory)
		}
	}
	for directory := range languageSidecars {
		if !seen[directory] {
			t.Errorf("%s holds no .octest files; remove it from languageSidecars", directory)
		}
	}
	sort.Strings(targets)

	for _, directory := range targets {
		// Fixtures name data files relative to the repository root, so every
		// command runs as a process started there.
		path := filepath.FromSlash(directory)
		if !corpusDirectoryHasFacts(t, filepath.Join(repo, path)) {
			// A directory of [Artifact] entry points has nothing for
			// `oct test` to run. `oct artifact` evaluates it, into a scratch
			// output root.
			t.Run(directory+"/artifact", func(t *testing.T) {
				stdout, stderr, err := runOctInRepository(t, repo, "artifact", path, "--output-root", t.TempDir())
				if err != nil {
					t.Fatalf("oct artifact failed: %v\n%s\n%s", err, stdout, stderr)
				}
			})
			continue
		}
		for _, lane := range []corpusLane{laneInterpreted, laneCompiled} {
			t.Run(directory+"/"+string(lane), func(t *testing.T) {
				wrapperPath := ""
				if sidecars, needed := languageSidecars[directory]; needed {
					wrapperPath = absoluteTestPath(t, cachedTestSidecarDir(t, sidecars...))
				}
				stdout, stderr, err := runOctWithWrapperPath(t, repo, wrapperPath, "test", path, "--execution", string(lane))
				if gap, known := gapFor(directory, lane); known {
					if err == nil {
						t.Fatalf("%s now passes in the %s lane; remove it from languageKnownGaps", directory, lane)
					}
					if !strings.Contains(stdout+stderr, gap.contains) {
						t.Fatalf("%s fails in the %s lane, but not with %q:\n%s\n%s", directory, lane, gap.contains, stdout, stderr)
					}
					return
				}
				if err != nil {
					t.Fatalf("%v\n%s\n%s", err, corpusFailureLines(stdout), stderr)
				}
				if strings.Contains(stdout, "interpreted fallback:") && lane == laneCompiled && !strings.Contains(stdout, "interpreted fallback: 0") {
					t.Fatalf("the compiled lane fell back to the interpreter:\n%s", stdout)
				}
			})
		}
	}

	// Every .octfail under Language, compile-time and runtime. Run from the
	// root, `oct test` checks .octfail files only.
	t.Run("octfail", func(t *testing.T) {
		stdout, stderr, err := runOctInRepository(t, repo, "test", "Language")
		if err != nil {
			t.Fatalf("%v\n%s\n%s", err, corpusFailureLines(stdout), stderr)
		}
	})
}

// languageOctestDirectories lists, with forward slashes and relative to the
// repository, every directory under Language that holds a .octest file.
func languageOctestDirectories(t *testing.T, repo string) []string {
	t.Helper()
	found := map[string]bool{}
	root := filepath.Join(repo, "Language")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && filepath.Ext(path) == ".octest" {
			relative, err := filepath.Rel(repo, filepath.Dir(path))
			if err != nil {
				return err
			}
			found[filepath.ToSlash(relative)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	directories := make([]string, 0, len(found))
	for directory := range found {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	if len(directories) == 0 {
		t.Fatal("no .octest files found under Language")
	}
	return directories
}

// corpusDirectoryHasFacts reports whether any .octest under the directory
// declares a [Fact] or a [Theory].
func corpusDirectoryHasFacts(t *testing.T, directory string) bool {
	t.Helper()
	hasFacts := false
	err := filepath.WalkDir(directory, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".octest" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if corpusFactAttribute.Match(data) {
			hasFacts = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hasFacts
}

// corpusFailureLines keeps the lines of a test run that say what failed.
func corpusFailureLines(stdout string) string {
	var kept []string
	lines := strings.Split(stdout, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "FAIL ") {
			kept = append(kept, line)
			for j := i + 1; j < len(lines) && strings.HasPrefix(lines[j], "  "); j++ {
				kept = append(kept, lines[j])
			}
		}
		if strings.HasPrefix(line, "Result: ") || strings.HasPrefix(line, "test failed") {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return stdout
	}
	return strings.Join(kept, "\n")
}

// runOctInRepository runs the oct binary with the repository root as its
// working directory, which is where a person runs it from. No wrapper sidecar
// is available to the run.
func runOctInRepository(t *testing.T, repo string, args ...string) (string, string, error) {
	t.Helper()
	return runOctWithWrapperPath(t, repo, "", args...)
}

// runOctWithWrapperPath is runOctInRepository with OCT_WRAPPER_PATH set to
// wrapperPath. An empty wrapperPath removes the variable, so that a sidecar
// directory in the caller's environment cannot make a fixture pass.
func runOctWithWrapperPath(t *testing.T, repo string, wrapperPath string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(absoluteTestPath(t, sharedTestOctBinary(t)), args...)
	cmd.Dir = repo
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "OCT_WRAPPER_PATH=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if wrapperPath != "" {
		cmd.Env = append(cmd.Env, "OCT_WRAPPER_PATH="+wrapperPath)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func absoluteTestPath(t *testing.T, path string) string {
	t.Helper()
	if filepath.IsAbs(path) {
		return path
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}
