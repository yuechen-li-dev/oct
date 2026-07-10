package test

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func Execute(path string, out io.Writer) error {
	return ExecuteWithOptions(path, out, Options{})
}

// Options deliberately exposes replay selection only.  It is not a generic
// compute API; the native host is responsible for eventual GPU execution.
type Options struct {
	List   bool
	CaseID string
}

func ExecuteWithOptions(path string, out io.Writer, options Options) error {
	paths, err := suitePaths(path)
	if err != nil {
		return err
	}
	for _, suite := range paths {
		manifest, err := Discover(suite)
		if err != nil {
			return err
		}
		selected := make([]Case, 0, len(manifest.Cases))
		for _, c := range manifest.Cases {
			if options.CaseID == "" || options.CaseID == c.StableID {
				selected = append(selected, c)
			}
		}
		if options.CaseID != "" && len(selected) == 0 {
			return fmt.Errorf("no .sdslvtest case with stable id %q", options.CaseID)
		}
		if options.List {
			for _, c := range selected {
				_, _ = fmt.Fprintf(out, "%s %s\n", c.StableID, c.DisplayName)
			}
			continue
		}
		// M0's evaluator remains only as a compile-free compatibility seam for
		// the original scalar fixture.  M29 discovery explicitly rejects malformed
		// tests before this path; a case that needs GPU lowering fails truthfully.
		if usesM29Syntax(suite) {
			if err := executeGPU(manifest, selected, out); err != nil {
				return err
			}
			continue
		}
		if len(manifest.Cases) != len(selected) || hasTheory(selected) {
			return fmt.Errorf("SDSL-V GPU test execution requires sdslv_test_host; discovered %d selected case(s) in %s", len(selected), suite)
		}
		if err := executeLegacyFacts(suite, out); err != nil {
			return err
		}
	}
	return nil
}

func usesM29Syntax(path string) bool {
	text, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(text), "HLSL<")
}
func executeGPU(manifest Manifest, selected []Case, out io.Writer) error {
	root := filepath.Join("out", "sdslvtest", strings.ReplaceAll(strings.TrimSuffix(filepath.Base(manifest.Source), filepath.Ext(manifest.Source)), " ", "_"))
	groups, err := Compile(manifest, root)
	if err != nil {
		return fmt.Errorf("COMPILE_FAILED: %w", err)
	}
	if err := WriteManifest(filepath.Join(root, "manifest.json"), manifest); err != nil {
		return err
	}
	host := os.Getenv("SDSLV_TEST_HOST")
	if host == "" {
		host = filepath.Join("out", "prometheus", "native", "sdslv_test_host.exe")
	}
	if _, err := os.Stat(host); err != nil {
		return fmt.Errorf("HOST_FAILURE: sdslv_test_host unavailable at %s", host)
	}
	for _, c := range selected {
		var group Group
		var index int
		found := false
		for _, g := range groups {
			for i, id := range g.Cases {
				if id == c.StableID {
					group, index = g, i
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return fmt.Errorf("HOST_FAILURE: compiled group missing %s", c.StableID)
		}
		args := []string{"--manifest", filepath.Join(root, "manifest.json"), "--spv", group.SPIRVPath, "--case", c.StableID, "--case-index", fmt.Sprint(index), "--row-index", fmt.Sprint(rowNumber(c)), "--groups", fmt.Sprint(c.Launch.DispatchGroups[0]), fmt.Sprint(c.Launch.DispatchGroups[1]), fmt.Sprint(c.Launch.DispatchGroups[2]), "--workgroup", fmt.Sprint(c.Launch.WorkgroupSize[0]), fmt.Sprint(c.Launch.WorkgroupSize[1]), fmt.Sprint(c.Launch.WorkgroupSize[2])}
		data, e := exec.Command(host, args...).CombinedOutput()
		_, _ = out.Write(data)
		if e != nil {
			return fmt.Errorf("HOST_FAILURE %s: %w", c.DisplayName, e)
		}
		if strings.Contains(string(data), "\"status\":\"PASS\"") {
			continue
		}
		return fmt.Errorf("GPU test failure: %s", c.DisplayName)
	}
	return nil
}
func rowNumber(c Case) int {
	if c.TheoryRow == nil {
		return 0
	}
	return *c.TheoryRow
}

func suitePaths(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".sdslvtest" {
			return nil, fmt.Errorf("sdslv test expects a .sdslvtest file or directory")
		}
		return []string{path}, nil
	}
	var paths []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if !info.IsDir() && filepath.Ext(p) == ".sdslvtest" {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .sdslvtest files found in %s", path)
	}
	sort.Strings(paths)
	return paths, nil
}

func hasTheory(cases []Case) bool {
	for _, c := range cases {
		if c.Kind == "Theory" {
			return true
		}
	}
	return false
}

func executeLegacyFacts(path string, out io.Writer) error {
	file, err := source.Load(path)
	if err != nil {
		return err
	}
	names := factNames(file.Text)
	if len(names) == 0 {
		return fmt.Errorf("no [Fact] tests found; [Theory] is deferred in GoOct SDSL-V M0")
	}
	moduleText := stripFactAttributes(file.Text)
	tokens, err := lex.Analyze(source.File{Path: file.Path, Text: moduleText})
	if err != nil {
		return err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return err
	}
	functions := map[string]ast.FunctionDecl{}
	for _, decl := range module.Decls {
		if fn, ok := decl.(ast.FunctionDecl); ok {
			functions[fn.Name] = fn
		}
	}
	failures := 0
	for _, name := range names {
		fn, ok := functions[name]
		if !ok {
			failures++
			_, _ = fmt.Fprintf(out, "FAIL %s: missing function for [Fact]\n", name)
			continue
		}
		if len(fn.Parameters) != 0 {
			failures++
			_, _ = fmt.Fprintf(out, "FAIL %s: [Fact] must not declare parameters\n", name)
			continue
		}
		if err := runFact(fn); err != nil {
			failures++
			_, _ = fmt.Fprintf(out, "FAIL %s: %v\n", name, err)
			continue
		}
		_, _ = fmt.Fprintf(out, "PASS %s\n", name)
	}
	if failures > 0 {
		return fmt.Errorf("sdslvtest failed: %d failure(s)", failures)
	}
	_, _ = fmt.Fprintf(out, "sdslvtest ok: %d fact(s)\n", len(names))
	return nil
}

func factNames(text string) []string {
	re := regexp.MustCompile(`(?m)^\s*\[Fact\]\s*\r?\n\s*fn\s+([A-Za-z_][A-Za-z0-9_]*)`)
	matches := re.FindAllStringSubmatch(text, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match[1])
	}
	return names
}

func stripFactAttributes(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "[Fact]" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "[Theory]") || strings.HasPrefix(strings.TrimSpace(line), "[InlineData") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

type value struct {
	kind string
	num  float64
	b    bool
	s    string
}

func runFact(fn ast.FunctionDecl) error {
	env := map[string]value{}
	for _, stmt := range fn.Body.Statements {
		if err := evalStmt(stmt, env); err != nil {
			return err
		}
	}
	return nil
}

func evalStmt(stmt ast.Stmt, env map[string]value) error {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if s.Value == nil {
			env[s.Name] = value{kind: "num"}
			return nil
		}
		v, err := evalExpr(s.Value, env)
		if err != nil {
			return err
		}
		env[s.Name] = v
		return nil
	case ast.ExprStmt:
		_, err := evalExpr(s.Value, env)
		return err
	default:
		return fmt.Errorf("unsupported .sdslvtest statement in M0")
	}
}

func evalExpr(expr ast.Expr, env map[string]value) (value, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		var n float64
		_, err := fmt.Sscanf(strings.TrimRight(e.Value, "uU"), "%f", &n)
		return value{kind: "num", num: n}, err
	case ast.FloatLiteral:
		var n float64
		_, err := fmt.Sscanf(e.Value, "%f", &n)
		return value{kind: "num", num: n}, err
	case ast.BoolLiteral:
		return value{kind: "bool", b: e.Value}, nil
	case ast.StringLiteral:
		return value{kind: "string", s: e.Value}, nil
	case ast.IdentifierExpr:
		v, ok := env[e.Name]
		if !ok {
			return value{}, fmt.Errorf("unknown test local %s", e.Name)
		}
		return v, nil
	case ast.BinaryExpr:
		left, err := evalExpr(e.Left, env)
		if err != nil {
			return value{}, err
		}
		right, err := evalExpr(e.Right, env)
		if err != nil {
			return value{}, err
		}
		return evalBinary(left, e.Operator, right)
	case ast.ParenExpr:
		return evalExpr(e.Inner, env)
	case ast.CallExpr:
		return evalCall(e, env)
	default:
		return value{}, fmt.Errorf("unsupported .sdslvtest expression in M0")
	}
}

func evalBinary(left value, op string, right value) (value, error) {
	switch op {
	case "+":
		return value{kind: "num", num: left.num + right.num}, nil
	case "-":
		return value{kind: "num", num: left.num - right.num}, nil
	case "*":
		return value{kind: "num", num: left.num * right.num}, nil
	case "/":
		return value{kind: "num", num: left.num / right.num}, nil
	case ">":
		return value{kind: "bool", b: left.num > right.num}, nil
	case "<":
		return value{kind: "bool", b: left.num < right.num}, nil
	case ">=":
		return value{kind: "bool", b: left.num >= right.num}, nil
	case "<=":
		return value{kind: "bool", b: left.num <= right.num}, nil
	case "==":
		if left.kind == "bool" || right.kind == "bool" {
			return value{kind: "bool", b: left.b == right.b}, nil
		}
		return value{kind: "bool", b: left.num == right.num}, nil
	case "!=":
		if left.kind == "bool" || right.kind == "bool" {
			return value{kind: "bool", b: left.b != right.b}, nil
		}
		return value{kind: "bool", b: left.num != right.num}, nil
	default:
		return value{}, fmt.Errorf("unsupported operator %s", op)
	}
}

func evalCall(call ast.CallExpr, env map[string]value) (value, error) {
	name := callName(call.Callee)
	args := make([]value, 0, len(call.Arguments))
	for _, arg := range call.Arguments {
		v, err := evalExpr(arg, env)
		if err != nil {
			return value{}, err
		}
		args = append(args, v)
	}
	switch name {
	case "Assert.True":
		if len(args) < 1 || args[0].kind != "bool" {
			return value{}, fmt.Errorf("Assert.True expects a bool")
		}
		if !args[0].b {
			return value{}, fmt.Errorf("assertion failed")
		}
		return value{kind: "void"}, nil
	case "Assert.Equals":
		if len(args) < 2 {
			return value{}, fmt.Errorf("Assert.Equals expects actual and expected")
		}
		if args[0].kind == "bool" || args[1].kind == "bool" {
			if args[0].b != args[1].b {
				return value{}, fmt.Errorf("expected %v, got %v", args[1].b, args[0].b)
			}
		} else if args[0].num != args[1].num {
			return value{}, fmt.Errorf("expected %v, got %v", args[1].num, args[0].num)
		}
		return value{kind: "void"}, nil
	case "Assert.Near":
		if len(args) < 3 {
			return value{}, fmt.Errorf("Assert.Near expects actual, expected, tolerance")
		}
		if math.Abs(args[0].num-args[1].num) > args[2].num {
			return value{}, fmt.Errorf("expected %v near %v within %v", args[0].num, args[1].num, args[2].num)
		}
		return value{kind: "void"}, nil
	case "saturate":
		if len(args) != 1 {
			return value{}, fmt.Errorf("saturate expects one argument")
		}
		return value{kind: "num", num: math.Max(0, math.Min(1, args[0].num))}, nil
	default:
		return value{}, fmt.Errorf("unsupported function call %s", name)
	}
}

func callName(expr ast.Expr) string {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name
	case ast.FieldAccessExpr:
		return callName(e.Target) + "." + e.Field
	default:
		return "<unknown>"
	}
}
