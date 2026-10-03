package tester

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/octfailheader"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// octFailRunLimit bounds one compiled run of a runtime fixture.
const octFailRunLimit = 30 * time.Second

// octFailCase is one .octfail fixture: when it must fail, the texts its
// failure must contain, and the Oct source below the expectation lines.
type octFailCase struct {
	path        string
	displayName string
	phase       octfailheader.Phase
	expected    []string
	source      string
}

// expectationLabel names what the fixture expects, for the failure report.
func (c octFailCase) expectationLabel() string {
	switch c.phase {
	case octfailheader.Runtime:
		return "expected runtime error containing"
	case octfailheader.Artifact:
		return "expected artifact error containing"
	default:
		return "expected error containing"
	}
}

func discoverOctFailCases(root string) ([]octFailCase, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) == ".octfail" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover .octfail files: %w", err)
	}
	sort.Strings(files)

	cases := make([]octFailCase, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read .octfail fixture %s: %w", path, err)
		}
		header, source, err := octfailheader.Split(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		cases = append(cases, octFailCase{
			path:        path,
			displayName: filepath.ToSlash(rel),
			phase:       header.Phase,
			expected:    header.Texts,
			source:      source,
		})
	}
	return cases, nil
}

// ParseOctFailFixture exposes the fixture header grammar to language-specific
// invalid corpora that compile a fixture themselves. It serves the plain form
// only: one compile-time expectation. A runtime or artifact fixture, or one
// with several expectation lines, is an error here and not a fixture that
// "failed to fail". Callers retain ownership of compilation and
// diagnostic-code checks.
func ParseOctFailFixture(content string) (string, string, error) {
	header, source, err := octfailheader.Split(content)
	if err != nil {
		return "", "", err
	}
	if header.Phase != octfailheader.Compile {
		return "", "", fmt.Errorf("%s expectation header is not a compile-time expectation", header.Phase)
	}
	if len(header.Texts) != 1 {
		return "", "", fmt.Errorf("this corpus takes one expectation line, found %d", len(header.Texts))
	}
	return header.Texts[0], source, nil
}

// missingText returns the first expected text that actual does not contain.
func missingText(expected []string, actual string) (string, bool) {
	for _, text := range expected {
		if !strings.Contains(actual, text) {
			return text, true
		}
	}
	return "", false
}

func runOctFailCase(testCase octFailCase, executionMode string) (string, error) {
	tempDir, err := os.MkdirTemp("", "octfail-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// An artifact fixture is a test source: it declares [Artifact] entry
	// points, which an ordinary source may not.
	sourceName := "fixture.oct"
	if testCase.phase == octfailheader.Artifact {
		sourceName = "fixture.octest"
	}
	sourcePath := filepath.Join(tempDir, sourceName)
	if err := os.WriteFile(sourcePath, []byte(testCase.source), 0o644); err != nil {
		return "", fmt.Errorf("write temp source: %w", err)
	}

	// The fixture is checked as a copy, away from its neighbours, but it may
	// import the libraries of the repository it lives in.
	importAnchor, err := filepath.Abs(filepath.Dir(testCase.path))
	if err != nil {
		return "", fmt.Errorf("resolve fixture directory: %w", err)
	}
	switch testCase.phase {
	case octfailheader.Runtime:
		return runRuntimeOctFailCase(testCase, sourcePath, importAnchor, executionMode)
	case octfailheader.Artifact:
		return runArtifactOctFailCase(testCase, sourcePath, importAnchor, filepath.Join(tempDir, "published"))
	}

	_, compileErr := build.CompileWithImportAnchor(sourcePath, importAnchor)
	return judgeCompileTimeOctFail(testCase.expected, compileErr)
}

// runArtifactOctFailCase checks a fixture whose [Artifact] entry points must
// fail when evaluated. Artifact evaluation is a build-time phase with one
// implementation, so the check is the same in every execution mode. A failed
// evaluation publishes nothing: an output left in the output root fails the
// fixture even when the message matches.
func runArtifactOctFailCase(testCase octFailCase, sourcePath string, importAnchor string, outputRoot string) (string, error) {
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return "", fmt.Errorf("create artifact output root: %w", err)
	}
	var stdout bytes.Buffer
	evalErr := ExecuteArtifactsWithOptions(sourcePath, &stdout, ArtifactOptions{OutputRoot: outputRoot, ImportAnchor: importAnchor})
	published, err := publishedFiles(outputRoot)
	if err != nil {
		return "", err
	}
	return judgeArtifactOctFail(testCase.expected, stdout.String(), evalErr, published)
}

// judgeArtifactOctFail decides an artifact fixture from what evaluating it
// did. The first return value describes what happened.
func judgeArtifactOctFail(expected []string, stdout string, evalErr error, published []string) (string, error) {
	if evalErr == nil {
		return "artifact evaluation completed", fmt.Errorf("expected artifact evaluation to fail")
	}
	actual := artifactFailureText(stdout, evalErr)
	if text, missing := missingText(expected, actual); missing {
		return actual, fmt.Errorf("expected artifact error containing: %q", text)
	}
	if len(published) > 0 {
		return "the evaluation failed as expected and still published " + strings.Join(published, ", "), fmt.Errorf("a failed artifact evaluation must publish nothing")
	}
	return actual, nil
}

// artifactFailureText is what a person sees when artifact evaluation fails:
// the FAIL lines the runner printed, then the error it returned.
func artifactFailureText(stdout string, evalErr error) string {
	var parts []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "FAIL ") {
			parts = append(parts, strings.TrimSpace(line))
		}
	}
	parts = append(parts, evalErr.Error())
	return strings.Join(parts, "; ")
}

func publishedFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect artifact output root: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

// judgeCompileTimeOctFail decides a compile-time fixture from the result of
// compiling it. The first return value describes what happened.
func judgeCompileTimeOctFail(expected []string, compileErr error) (string, error) {
	if compileErr == nil {
		return "", fmt.Errorf("expected compilation failure but build succeeded")
	}
	actual := compileErr.Error()
	// The Go toolchain rejecting generated code is a backend defect, not the
	// compiler rejecting the source. Its message can contain the expected
	// text by accident, so it never satisfies a contract.
	if errors.Is(compileErr, build.ErrGeneratedProgramDidNotBuild) {
		return "the source was accepted, and the generated program did not build: " + actual, fmt.Errorf("expected the compiler to reject the source")
	}
	if text, missing := missingText(expected, actual); missing {
		return actual, fmt.Errorf("expected error containing: %q", text)
	}
	return actual, nil
}

// runRuntimeOctFailCase checks a fixture whose source is valid and whose Main
// must stop with a failure.
func runRuntimeOctFailCase(testCase octFailCase, sourcePath string, importAnchor string, executionMode string) (string, error) {
	return checkRuntimeOctFail(testCase.expected, executionMode, []octFailLane{
		{name: "interpreted", run: func() (string, bool, error) { return runOctFailInterpreted(sourcePath, importAnchor) }},
		{name: "compiled", run: func() (string, bool, error) { return runOctFailCompiled(sourcePath, importAnchor) }},
	})
}

// octFailLane runs a fixture in one execution lane. It reports the failure
// message and whether Main failed; a non-nil error means the source did not
// get as far as running.
type octFailLane struct {
	name string
	run  func() (message string, failed bool, err error)
}

// checkRuntimeOctFail decides a runtime fixture. The two lanes are separate
// contracts: under "auto" every lane must fail with the expected text, and a
// named mode checks that lane alone. Nothing falls back from one lane to the
// other. The first return value describes what happened when the check fails.
func checkRuntimeOctFail(want []string, executionMode string, lanes []octFailLane) (string, error) {
	for _, lane := range lanes {
		if executionMode != "auto" && executionMode != lane.name {
			continue
		}
		message, failed, err := lane.run()
		switch {
		case err != nil:
			return lane.name + ": did not compile: " + err.Error(), fmt.Errorf("expected the source to compile and fail when run")
		case !failed:
			return lane.name + ": the program ran to completion", fmt.Errorf("expected runtime error containing: %q", want[0])
		default:
			if text, missing := missingText(want, message); missing {
				return lane.name + ": " + message, fmt.Errorf("expected runtime error containing: %q", text)
			}
		}
	}
	return "", nil
}

func runOctFailInterpreted(sourcePath string, importAnchor string) (message string, failed bool, err error) {
	program, err := project.LoadWithImportAnchor(sourcePath, importAnchor)
	if err != nil {
		return "", false, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return "", false, err
	}
	if _, runErr := interpret.ExecuteMain(program, io.Discard); runErr != nil {
		return runErr.Error(), true, nil
	}
	return "", false, nil
}

func runOctFailCompiled(sourcePath string, importAnchor string) (message string, failed bool, err error) {
	result, err := build.CompileWithImportAnchor(sourcePath, importAnchor)
	if err != nil {
		return "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), octFailRunLimit)
	defer cancel()
	output, runErr := exec.CommandContext(ctx, result.ArtifactPath).CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", false, fmt.Errorf("compiled program did not stop within %s", octFailRunLimit)
	}
	if runErr == nil {
		return "", false, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return "", false, fmt.Errorf("run compiled program: %w", runErr)
	}
	return strings.TrimSpace(string(output)), true, nil
}
