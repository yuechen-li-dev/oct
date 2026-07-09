package test

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func Execute(path string, out io.Writer) error {
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
