package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"oct/internal/ast"
	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type testCase struct {
	pkg         string
	filePath    string
	name        string
	displayName string
	caseIndex   int
	arguments   []interpret.Value
	cycleTime   time.Duration
	suites      []string
}

var defaultTestCycleTime = 30 * time.Second

type TestOptions struct {
	Suite string
}

func Execute(path string, stdout io.Writer) error {
	return ExecuteWithOptions(path, stdout, TestOptions{})
}

func ExecuteWithOptions(path string, stdout io.Writer, options TestOptions) error {
	return executeForPathOrExperiment(path, stdout, "test", func(singlePath string, singleStdout io.Writer) error {
		return executeTestsSingleRoot(singlePath, singleStdout, options)
	})
}

func executeTestsSingleRoot(path string, stdout io.Writer, options TestOptions) error {
	var tests []testCase
	selectedSources, err := selectedTestSources(path)
	if err != nil {
		return err
	}
	octFailCases, err := discoverOctFailCases(path)
	if err != nil {
		return err
	}

	program, loadErr := project.LoadForTest(path)
	if loadErr == nil {
		if err := typecheck.CheckProgram(program); err != nil {
			return err
		}
		for pkgName, pkg := range program.Packages {
			for _, fn := range pkg.Functions {
				if !isSelectedSource(selectedSources, fn.SourcePath) {
					continue
				}
				if fn.IsFact {
					tests = append(tests, testCase{
						pkg:         pkgName,
						filePath:    fn.SourcePath,
						name:        fn.Name,
						displayName: fn.Name,
						cycleTime:   defaultTestCycleTime,
						suites:      append([]string{}, fn.Suites...),
					})
				}
				if fn.IsTheory {
					cycleTime, err := resolveTheoryCycleTime(fn)
					if err != nil {
						return fmt.Errorf("%s.%s: %w", pkgName, fn.Name, err)
					}
					for i, row := range fn.InlineData {
						args, err := inlineDataArgumentsToValues(row.Values, pkgName)
						if err != nil {
							return fmt.Errorf("%s.%s inline case %d: %w", pkgName, fn.Name, i, err)
						}
						tests = append(tests, testCase{
							pkg:         pkgName,
							filePath:    fn.SourcePath,
							name:        fn.Name,
							displayName: fmt.Sprintf("%s[%d]", fn.Name, i),
							caseIndex:   i,
							arguments:   args,
							cycleTime:   cycleTime,
							suites:      append([]string{}, fn.Suites...),
						})
					}
				}
			}
		}
	} else if len(octFailCases) == 0 || !strings.Contains(loadErr.Error(), "unknown package") {
		return loadErr
	}
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].pkg != tests[j].pkg {
			return tests[i].pkg < tests[j].pkg
		}
		if tests[i].filePath != tests[j].filePath {
			return tests[i].filePath < tests[j].filePath
		}
		if tests[i].name != tests[j].name {
			return tests[i].name < tests[j].name
		}
		return tests[i].caseIndex < tests[j].caseIndex
	})
	if options.Suite != "" {
		filtered := make([]testCase, 0, len(tests))
		for _, tc := range tests {
			for _, suite := range tc.suites {
				if suite == options.Suite {
					filtered = append(filtered, tc)
					break
				}
			}
		}
		tests = filtered
	}

	if len(tests) == 0 && len(octFailCases) == 0 {
		if options.Suite != "" {
			return fmt.Errorf("no tests found for suite `%s`", options.Suite)
		}
		return fmt.Errorf("no [Fact], [Theory], or .octfail tests found")
	}

	failed := 0
	skipped := 0
	total := 0
	for _, testCase := range tests {
		total++
		qualified := fmt.Sprintf("%s.%s", testCase.pkg, testCase.displayName)
		assertionCount := 0
		ctx, cancel := context.WithTimeout(context.Background(), testCase.cycleTime)
		err := interpret.ExecuteFunctionWithArgsAndOptions(
			program,
			testCase.pkg,
			testCase.name,
			testCase.arguments,
			io.Discard,
			interpret.ExecuteOptions{
				AssertionRecorder: func() {
					assertionCount++
				},
				Context: ctx,
			},
		)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				failed++
				_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): exceeded cycle time of %.1f<s>\n", qualified, shortPath(path, testCase.filePath), testCase.cycleTime.Seconds())
				continue
			}
			var skipErr interpret.SkipTestError
			if errors.As(err, &skipErr) {
				skipped++
				_, _ = fmt.Fprintf(stdout, "SKIP %s (%s): %s\n", qualified, shortPath(path, testCase.filePath), skipErr.Reason)
				continue
			}
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): %v\n", qualified, shortPath(path, testCase.filePath), err)
			continue
		}
		if assertionCount == 0 {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): test completed with zero assertions\n", qualified, shortPath(path, testCase.filePath))
			continue
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s (%s)\n", qualified, shortPath(path, testCase.filePath))
	}
	for _, octFailCase := range octFailCases {
		total++
		actual, err := runOctFailCase(octFailCase)
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s\n", octFailCase.displayName)
			_, _ = fmt.Fprintf(stdout, "  expected error containing: %q\n", octFailCase.expectedError)
			if actual == "" {
				actual = err.Error()
			}
			_, _ = fmt.Fprintf(stdout, "  actual: %q\n", actual)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s\n", octFailCase.displayName)
	}

	passed := total - failed - skipped
	_, _ = fmt.Fprintf(stdout, "Result: %d passed, %d failed, %d skipped\n", passed, failed, skipped)
	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func resolveTheoryCycleTime(fn ast.FunctionDecl) (time.Duration, error) {
	if fn.CycleTime == nil {
		return defaultTestCycleTime, nil
	}
	literal, ok := fn.CycleTime.(ast.FloatLiteral)
	if !ok {
		return 0, fmt.Errorf("[CycleTime] currently requires a Float unit literal")
	}
	if literal.Dimension.String() != "s" {
		return 0, fmt.Errorf("[CycleTime] expects Float<s>")
	}
	value := parseFloatLiteral(literal.Value)
	if value <= 0 {
		return 0, fmt.Errorf("[CycleTime] must be > 0<s>")
	}
	return time.Duration(value * float64(time.Second)), nil
}

func inlineDataArgumentsToValues(values []ast.Expr, currentPackage string) ([]interpret.Value, error) {
	args := make([]interpret.Value, 0, len(values))
	for _, value := range values {
		converted, err := inlineDataValueToRuntimeValue(value, currentPackage)
		if err != nil {
			return nil, err
		}
		args = append(args, converted)
	}
	return args, nil
}

func inlineDataValueToRuntimeValue(value ast.Expr, currentPackage string) (interpret.Value, error) {
	switch node := value.(type) {
	case ast.IntegerLiteral:
		return interpret.Value{Kind: interpret.ValueInt, Int: parseIntLiteral(node.Value), Dimension: node.Dimension}, nil
	case ast.FloatLiteral:
		return interpret.Value{Kind: interpret.ValueFloat, Float: parseFloatLiteral(node.Value), Dimension: node.Dimension}, nil
	case ast.BoolLiteral:
		return interpret.Value{Kind: interpret.ValueBool, Bool: node.Value}, nil
	case ast.StringLiteralExpr:
		return interpret.Value{Kind: interpret.ValueString, Text: node.Value}, nil
	case ast.FieldAccessExpr:
		enumType, variant, ok := flattenEnumValueExpr(node)
		if !ok {
			return interpret.Value{}, fmt.Errorf("invalid enum inline value")
		}
		if !strings.Contains(enumType, ".") {
			enumType = currentPackage + "." + enumType
		}
		return interpret.Value{Kind: interpret.ValueEnum, Enum: interpret.EnumValue{TypeName: enumType, Variant: variant}}, nil
	default:
		return interpret.Value{}, fmt.Errorf("unsupported inline value type %T", value)
	}
}

func flattenEnumValueExpr(expr ast.FieldAccessExpr) (string, string, bool) {
	switch target := expr.Target.(type) {
	case ast.IdentifierExpr:
		return target.Name, expr.Field, true
	case ast.FieldAccessExpr:
		pkg, enumType, ok := flattenEnumTypeNameFromFieldAccess(target)
		if !ok {
			return "", "", false
		}
		return pkg + "." + enumType, expr.Field, true
	default:
		return "", "", false
	}
}

func flattenEnumTypeNameFromFieldAccess(expr ast.FieldAccessExpr) (string, string, bool) {
	pkgIdentifier, ok := expr.Target.(ast.IdentifierExpr)
	if !ok {
		return "", "", false
	}
	return pkgIdentifier.Name, expr.Field, true
}

func parseIntLiteral(text string) int64 {
	value, _ := strconv.ParseInt(text, 10, 64)
	return value
}

func parseFloatLiteral(text string) float64 {
	value, _ := strconv.ParseFloat(text, 64)
	return value
}

func shortPath(root string, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return full
	}
	return rel
}

func selectedTestSources(path string) (map[string]struct{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return map[string]struct{}{filepath.Clean(abs): {}}, nil
}

func isSelectedSource(selected map[string]struct{}, sourcePath string) bool {
	if selected == nil {
		return true
	}
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return false
	}
	_, ok := selected[filepath.Clean(abs)]
	return ok
}
