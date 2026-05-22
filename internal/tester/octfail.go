package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/build"
)

var octFailHeaderPattern = regexp.MustCompile(`^expect error:\s*"(.*)"\s*$`)

type octFailCase struct {
	path          string
	displayName   string
	expectedError string
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
		expectedError, source, err := parseOctFailFixture(string(data))
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
			source:        source,
		})
	}
	return cases, nil
}

func parseOctFailFixture(content string) (string, string, error) {
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
		return "", "", fmt.Errorf("missing expectation header")
	}

	header := strings.TrimSpace(lines[headerIndex])
	match := octFailHeaderPattern.FindStringSubmatch(header)
	if match == nil {
		return "", "", fmt.Errorf("malformed expectation header")
	}
	expected := match[1]
	if expected == "" {
		return "", "", fmt.Errorf("expected error substring must be non-empty")
	}

	for i := headerIndex + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "expect error:") {
			return "", "", fmt.Errorf("multiple expectation headers are not allowed")
		}
	}

	source := strings.Join(lines[headerIndex+1:], "\n")
	return expected, source, nil
}

func runOctFailCase(testCase octFailCase) (string, error) {
	tempDir, err := os.MkdirTemp("", "octfail-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "fixture.oct")
	if err := os.WriteFile(sourcePath, []byte(testCase.source), 0o644); err != nil {
		return "", fmt.Errorf("write temp source: %w", err)
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
