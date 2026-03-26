package tester

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type factCase struct {
	pkg      string
	filePath string
	name     string
}

func Execute(path string, stdout io.Writer) error {
	program, err := project.LoadForTest(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	var tests []factCase
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if !fn.IsFact {
				continue
			}
			tests = append(tests, factCase{pkg: pkgName, filePath: fn.SourcePath, name: fn.Name})
		}
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].pkg != tests[j].pkg {
			return tests[i].pkg < tests[j].pkg
		}
		if tests[i].filePath != tests[j].filePath {
			return tests[i].filePath < tests[j].filePath
		}
		return tests[i].name < tests[j].name
	})

	if len(tests) == 0 {
		return fmt.Errorf("no [Fact] tests found")
	}

	failed := 0
	for _, testCase := range tests {
		qualified := fmt.Sprintf("%s.%s", testCase.pkg, testCase.name)
		err := interpret.ExecuteFunction(program, testCase.pkg, testCase.name, io.Discard)
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, testCase.filePath), err)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, testCase.filePath))
	}

	_, _ = fmt.Fprintf(stdout, "Result: %d passed, %d failed\n", len(tests)-failed, failed)
	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func shortPath(root string, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return full
	}
	return rel
}
