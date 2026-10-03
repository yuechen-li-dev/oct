package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// An .octfail fixture begins with one expectation header.
//
//	expect error: "text"          the source is rejected before it can run
//	expect runtime error: "text"  the source compiles, and running Main stops
//	                              with a failure whose message contains text
var octFailHeaderPattern = regexp.MustCompile(`^expect (runtime )?error:\s*"(.*)"\s*$`)

// octFailRunLimit bounds one compiled run of a runtime fixture.
const octFailRunLimit = 30 * time.Second

type octFailCase struct {
	path          string
	displayName   string
	expectedError string
	runtime       bool
	source        string
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
		expectedError, runtime, source, err := parseOctFailExpectation(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		cases = append(cases, octFailCase{
			path:          path,
			displayName:   filepath.ToSlash(rel),
			expectedError: expectedError,
			runtime:       runtime,
			source:        source,
		})
	}
	return cases, nil
}

// parseOctFailFixture reads a compile-time fixture. A runtime fixture is not
// one: its source is valid, so a caller that only compiles would report it as
// a fixture that failed to fail.
func parseOctFailFixture(content string) (string, string, error) {
	expected, runtime, source, err := parseOctFailExpectation(content)
	if err != nil {
		return "", "", err
	}
	if runtime {
		return "", "", fmt.Errorf("runtime expectation header is not a compile-time expectation")
	}
	return expected, source, nil
}

func parseOctFailExpectation(content string) (string, bool, string, error) {
	lines := strings.Split(content, "\n")

	headerIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		headerIndex = i
		break
	}
	if headerIndex == -1 {
		return "", false, "", fmt.Errorf("missing expectation header")
	}

	header := strings.TrimSpace(lines[headerIndex])
	match := octFailHeaderPattern.FindStringSubmatch(header)
	if match == nil {
		return "", false, "", fmt.Errorf("malformed expectation header")
	}
	runtime := match[1] != ""
	expected := match[2]
	if expected == "" {
		return "", false, "", fmt.Errorf("expected error substring must be non-empty")
	}

	for i := headerIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "expect error:") || strings.HasPrefix(trimmed, "expect runtime error:") {
			return "", false, "", fmt.Errorf("multiple expectation headers are not allowed")
		}
	}

	source := strings.Join(lines[headerIndex+1:], "\n")
	return expected, runtime, source, nil
}

// ParseOctFailFixture exposes the established fixture header grammar to
// language-specific invalid corpora. Callers retain ownership of compilation
// and diagnostic-code checks; this helper owns only expectation parsing.
func ParseOctFailFixture(content string) (string, string, error) {
	return parseOctFailFixture(content)
}

func runOctFailCase(testCase octFailCase, executionMode string) (string, error) {
	tempDir, err := os.MkdirTemp("", "octfail-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "fixture.oct")
	if err := os.WriteFile(sourcePath, []byte(testCase.source), 0o644); err != nil {
		return "", fmt.Errorf("write temp source: %w", err)
	}

	if testCase.runtime {
		return runRuntimeOctFailCase(testCase, sourcePath, executionMode)
	}

	_, compileErr := build.Compile(sourcePath)
	if compileErr == nil {
		return "", fmt.Errorf("expected compilation failure but build succeeded")
	}

	actual := compileErr.Error()
	if !strings.Contains(actual, testCase.expectedError) {
		return actual, fmt.Errorf("expected error containing: %q", testCase.expectedError)
	}
	return actual, nil
}

// runRuntimeOctFailCase checks a fixture whose source is valid and whose Main
// must stop with a failure.
func runRuntimeOctFailCase(testCase octFailCase, sourcePath string, executionMode string) (string, error) {
	return checkRuntimeOctFail(testCase.expectedError, executionMode, []octFailLane{
		{name: "interpreted", run: func() (string, bool, error) { return runOctFailInterpreted(sourcePath) }},
		{name: "compiled", run: func() (string, bool, error) { return runOctFailCompiled(sourcePath) }},
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
func checkRuntimeOctFail(want string, executionMode string, lanes []octFailLane) (string, error) {
	for _, lane := range lanes {
		if executionMode != "auto" && executionMode != lane.name {
			continue
		}
		message, failed, err := lane.run()
		switch {
		case err != nil:
			return lane.name + ": did not compile: " + err.Error(), fmt.Errorf("expected the source to compile and fail when run")
		case !failed:
			return lane.name + ": the program ran to completion", fmt.Errorf("expected runtime error containing: %q", want)
		case !strings.Contains(message, want):
			return lane.name + ": " + message, fmt.Errorf("expected runtime error containing: %q", want)
		}
	}
	return "", nil
}

func runOctFailInterpreted(sourcePath string) (message string, failed bool, err error) {
	program, err := project.Load(sourcePath)
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

func runOctFailCompiled(sourcePath string) (message string, failed bool, err error) {
	result, err := build.Compile(sourcePath)
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
