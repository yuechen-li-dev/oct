package tester

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type testCase struct {
	pkg         string
	filePath    string
	name        string
	displayName string
	caseIndex   int
	arguments   []interpret.Value
	isFallible  bool
	cycleTime   time.Duration
	suites      []string
}

type compiledHarnessGroup struct {
	id       string
	pkg      string
	files    []string
	testCase []testCase
}

type testMetrics struct {
	harnessGroups       int
	suiteGroups         int
	fileGroups          int
	nativeCompilations  int
	processLaunches     int
	singleCaseReruns    int
	artifactScopesMade  int
	artifactScopesClean int
}

var defaultTestCycleTime = 30 * time.Second

type TestOptions struct {
	Suite       string
	Execution   string
	AllPackages bool
	JSON        bool
	// WrapperPath is a host-selected sidecar directory for bounded integrations
	// such as OCTGO. It is not a public CLI flag and does not grant Oct code a
	// general process or filesystem capability.
	WrapperPath string
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
	executionMode := strings.TrimSpace(options.Execution)
	if executionMode == "" {
		executionMode = "auto"
	}
	if executionMode != "auto" && executionMode != "compiled" && executionMode != "interpreted" {
		return fmt.Errorf("invalid test execution mode %q (expected auto|compiled|interpreted)", executionMode)
	}
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
						isFallible:  fn.IsFallible,
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
							isFallible:  fn.IsFallible,
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
	if !options.AllPackages {
		filtered := make([]testCase, 0, len(tests))
		for _, tc := range tests {
			if tc.pkg == program.Entry {
				filtered = append(filtered, tc)
			}
		}
		tests = filtered
	}
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
	compiledCount := 0
	interpretedFallbackCount := 0
	compiled := map[string]error{}
	metrics := testMetrics{}
	if executionMode != "interpreted" {
		compiled, metrics = executeCompiledHarnessGroups(program, tests, stdout, options.WrapperPath)
	}
	for _, testCase := range tests {
		total++
		qualified := fmt.Sprintf("%s.%s", testCase.pkg, testCase.displayName)
		ranCompiled := false
		if executionMode != "interpreted" {
			err := compiled[testCaseID(testCase)]
			if err == nil {
				compiledCount++
				ranCompiled = true
			} else if executionMode == "compiled" {
				failed++
				_, _ = fmt.Fprintf(stdout, "FAIL %s (%s): compiled execution required: %v\n", qualified, shortPath(path, testCase.filePath), err)
				continue
			} else {
				interpretedFallbackCount++
				_, _ = fmt.Fprintf(stdout, "INFO %s (%s): compiled unsupported, falling back to interpreted: %v\n", qualified, shortPath(path, testCase.filePath), err)
			}
		}
		if !ranCompiled {
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
	_, _ = fmt.Fprintf(stdout, "Execution summary: compiled: %d interpreted fallback: %d\n", compiledCount, interpretedFallbackCount)
	if os.Getenv("OCT_TEST_METRICS") == "1" {
		_, _ = fmt.Fprintf(stdout, "Test metrics: cases=%d suite_groups=%d file_groups=%d harness_groups=%d native_compilations=%d process_launches=%d single_case_reruns=%d artifact_scopes_created=%d artifact_scopes_cleaned=%d\n",
			total, metrics.suiteGroups, metrics.fileGroups, metrics.harnessGroups, metrics.nativeCompilations, metrics.processLaunches, metrics.singleCaseReruns, metrics.artifactScopesMade, metrics.artifactScopesClean)
	}
	_, _ = fmt.Fprintf(stdout, "Result: %d passed, %d failed, %d skipped\n", passed, failed, skipped)
	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

func executeCompiledTestCase(program project.Program, tc testCase, diagnostic io.Writer) error {
	results, _ := executeCompiledHarnessGroups(program, []testCase{tc}, diagnostic, "")
	return results[testCaseID(tc)]
}

func executeCompiledHarnessGroups(program project.Program, tests []testCase, diagnostic io.Writer, wrapperPath string) (map[string]error, testMetrics) {
	results := make(map[string]error, len(tests))
	metrics := testMetrics{}
	groups := groupCompiledTestCases(tests)
	metrics.harnessGroups = len(groups)
	for _, group := range groups {
		if strings.HasPrefix(group.id, "suite:") {
			metrics.suiteGroups++
		} else {
			metrics.fileGroups++
		}
		groupResults, groupMetrics := executeCompiledHarnessGroup(program, group, diagnostic, wrapperPath)
		metrics.nativeCompilations += groupMetrics.nativeCompilations
		metrics.processLaunches += groupMetrics.processLaunches
		metrics.artifactScopesMade += groupMetrics.artifactScopesMade
		metrics.artifactScopesClean += groupMetrics.artifactScopesClean
		for id, err := range groupResults {
			results[id] = err
		}
	}
	return results, metrics
}

func groupCompiledTestCases(tests []testCase) []compiledHarnessGroup {
	byID := map[string]*compiledHarnessGroup{}
	for _, tc := range tests {
		id := "file:" + tc.pkg + ":" + filepath.Clean(tc.filePath)
		if len(tc.suites) > 0 {
			suites := append([]string{}, tc.suites...)
			sort.Strings(suites)
			id = "suite:" + tc.pkg + ":" + strings.Join(suites, "|")
		}
		group := byID[id]
		if group == nil {
			group = &compiledHarnessGroup{id: id, pkg: tc.pkg}
			byID[id] = group
		}
		group.testCase = append(group.testCase, tc)
		if !containsPath(group.files, tc.filePath) {
			group.files = append(group.files, tc.filePath)
		}
	}
	groups := make([]compiledHarnessGroup, 0, len(byID))
	for _, group := range byID {
		sort.Strings(group.files)
		sort.Slice(group.testCase, func(i, j int) bool { return testCaseID(group.testCase[i]) < testCaseID(group.testCase[j]) })
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].id < groups[j].id })
	return groups
}

func containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if item == path {
			return true
		}
	}
	return false
}

func testCaseID(tc testCase) string {
	return tc.pkg + "|" + filepath.Clean(tc.filePath) + "|" + tc.displayName
}

func executeCompiledHarnessGroup(program project.Program, group compiledHarnessGroup, diagnostic io.Writer, wrapperPath string) (results map[string]error, metrics testMetrics) {
	results = make(map[string]error, len(group.testCase))
	pkg, ok := program.Packages[group.pkg]
	if !ok {
		for _, tc := range group.testCase {
			results[testCaseID(tc)] = fmt.Errorf("unknown package %q", group.pkg)
		}
		return results, metrics
	}
	scope, err := newArtifactScope("octest-run", diagnostic)
	if err != nil {
		for _, tc := range group.testCase {
			results[testCaseID(tc)] = err
		}
		return results, metrics
	}
	metrics.artifactScopesMade++
	var closeErr error
	defer func() {
		closeArtifactScope(scope, &closeErr)
		if closeErr == nil && !scope.keep {
			metrics.artifactScopesClean++
		}
		if closeErr != nil {
			for _, tc := range group.testCase {
				if results[testCaseID(tc)] == nil {
					results[testCaseID(tc)] = closeErr
				}
			}
		}
	}()
	runnerPath, cases, err := writeCompiledTestHarness(scope.path(sanitizeHarnessName(group)+".octest"), group.pkg, group.testCase)
	if err != nil {
		for _, tc := range group.testCase {
			results[testCaseID(tc)] = err
		}
		return results, metrics
	}
	selected := append([]string{runnerPath}, group.files...)
	metrics.nativeCompilations++
	result, err := build.CompileTestHarnessWithSelectedFilesInPackage(runnerPath, pkg.Directory, selected, cases)
	if err != nil {
		for _, tc := range group.testCase {
			results[testCaseID(tc)] = err
		}
		return results, metrics
	}
	for _, tc := range group.testCase {
		ctx, cancel := context.WithTimeout(context.Background(), tc.cycleTime)
		cmd := exec.CommandContext(ctx, result.ArtifactPath, "--case", testCaseID(tc))
		metrics.processLaunches++
		cmd.Env = testCommandEnvironment(wrapperPath)
		output, runErr := cmd.CombinedOutput()
		cancel()
		if runErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				results[testCaseID(tc)] = context.DeadlineExceeded
				continue
			}
			msg := strings.TrimSpace(string(output))
			if msg == "" {
				results[testCaseID(tc)] = fmt.Errorf("compiled test run failed: %w", runErr)
			} else {
				results[testCaseID(tc)] = fmt.Errorf("compiled test run failed: %w: %s", runErr, msg)
			}
		}
	}
	return results, metrics
}

func testCommandEnvironment(wrapperPath string) []string {
	environment := os.Environ()
	if wrapperPath == "" {
		return append(environment, "OCT_ENFORCE_ASSERTIONS=1")
	}
	filtered := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "OCT_WRAPPER_PATH") || strings.EqualFold(name, "OCT_ENFORCE_ASSERTIONS") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "OCT_ENFORCE_ASSERTIONS=1", "OCT_WRAPPER_PATH="+wrapperPath)
}

func writeCompiledTestRunner(runnerPath string, pkgName string, testCase testCase) (string, error) {
	call := fmt.Sprintf("    %s()", testCase.name)
	if len(testCase.arguments) > 0 {
		parts := make([]string, 0, len(testCase.arguments))
		for _, arg := range testCase.arguments {
			parts = append(parts, inlineValueToSource(arg))
		}
		call = fmt.Sprintf("    %s(%s)", testCase.name, strings.Join(parts, ", "))
	}
	if testCase.isFallible {
		call += "!"
	}
	source := strings.Join([]string{
		"package " + pkgName,
		"fn main() -> Int {",
		call,
		"    return 0",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(runnerPath, []byte(source), 0o644); err != nil {
		return "", err
	}
	if os.Getenv("OCT_DEBUG_COMPILED_RUNNER") != "" {
		fmt.Fprintf(os.Stderr, "compiled runner debug: testCase.name=%q displayName=%q isFallible=%t arguments=%d runner=%s\n%s\n", testCase.name, testCase.displayName, testCase.isFallible, len(testCase.arguments), runnerPath, source)
	}
	return runnerPath, nil
}

func writeCompiledTestHarness(runnerPath string, pkgName string, tests []testCase) (string, []build.TestHarnessCase, error) {
	lines := []string{"package " + pkgName}
	cases := make([]build.TestHarnessCase, 0, len(tests))
	for i, tc := range tests {
		helper := fmt.Sprintf("OctestCase%d", i)
		call := "    " + tc.name + "()"
		if len(tc.arguments) > 0 {
			parts := make([]string, 0, len(tc.arguments))
			for _, arg := range tc.arguments {
				parts = append(parts, inlineValueToSource(arg))
			}
			call = fmt.Sprintf("    %s(%s)", tc.name, strings.Join(parts, ", "))
		}
		if tc.isFallible {
			call += "!"
		}
		lines = append(lines, "fn "+helper+"() -> Void {", call, "}")
		cases = append(cases, build.TestHarnessCase{ID: testCaseID(tc), Function: helper})
	}
	lines = append(lines, "fn main() -> Int {")
	for _, tc := range cases {
		lines = append(lines, "    "+tc.Function+"()")
	}
	lines = append(lines, "    return 0", "}")
	if err := os.WriteFile(runnerPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return "", nil, err
	}
	return runnerPath, cases, nil
}

func sanitizeHarnessName(group compiledHarnessGroup) string {
	name := group.id
	if strings.HasPrefix(name, "suite:") {
		name = "octest-suite-" + strings.TrimPrefix(name, "suite:")
	} else {
		name = "octest-file-" + strings.TrimPrefix(name, "file:")
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func inlineValueToSource(value interpret.Value) string {
	switch value.Kind {
	case interpret.ValueInt:
		return fmt.Sprintf("%d", value.Int)
	case interpret.ValueFloat:
		return strconv.FormatFloat(value.Float, 'f', -1, 64)
	case interpret.ValueBool:
		if value.Bool {
			return "true"
		}
		return "false"
	case interpret.ValueString:
		return strconv.Quote(value.Text)
	case interpret.ValueEnum:
		return fmt.Sprintf("%s.%s", value.Enum.TypeName, value.Enum.Variant)
	default:
		return "0"
	}
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
