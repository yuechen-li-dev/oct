package tester

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
}

func Execute(path string, stdout io.Writer) error {
	program, err := project.LoadForTest(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	var tests []testCase
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if fn.IsFact {
				tests = append(tests, testCase{
					pkg:         pkgName,
					filePath:    fn.SourcePath,
					name:        fn.Name,
					displayName: fn.Name,
				})
			}
			if fn.IsTheory {
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
					})
				}
			}
		}
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

	if len(tests) == 0 {
		return fmt.Errorf("no [Fact] or [Theory] tests found")
	}

	failed := 0
	for _, testCase := range tests {
		qualified := fmt.Sprintf("%s.%s", testCase.pkg, testCase.displayName)
		err := interpret.ExecuteFunctionWithArgs(program, testCase.pkg, testCase.name, testCase.arguments, io.Discard)
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
