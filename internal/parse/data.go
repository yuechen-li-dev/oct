package parse

import (
	"fmt"

	"oct/internal/ast"
	"oct/internal/lex"
)

func BuildDataValue(result lex.Result) (ast.Expr, error) {
	parser := parser{
		sourcePath: result.Source.Path,
		tokens:     result.Tokens,
	}

	value, err := parser.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", result.Source.Path, err)
	}
	if parser.current().Kind != lex.EOF {
		return nil, fmt.Errorf("parse %s: %w", result.Source.Path, parser.errorAtCurrent("expected end of file after top-level value"))
	}
	if err := validateDataValue(value); err != nil {
		return nil, fmt.Errorf("parse %s: %w", result.Source.Path, err)
	}
	return value, nil
}

func validateDataValue(expr ast.Expr) error {
	switch node := expr.(type) {
	case ast.IntegerLiteral, ast.FloatLiteral, ast.BoolLiteral, ast.StringLiteralExpr:
		return nil
	case ast.ArrayLiteralExpr:
		for _, element := range node.Elements {
			if err := validateDataValue(element); err != nil {
				return err
			}
		}
		return nil
	case ast.RecordLiteralExpr:
		for _, field := range node.Fields {
			if err := validateDataValue(field.Value); err != nil {
				return err
			}
		}
		return nil
	case ast.FieldAccessExpr:
		if _, ok := node.Target.(ast.IdentifierExpr); ok {
			return nil
		}
		return fmt.Errorf(".octagon enum values must use explicit Enum.Variant form")
	case ast.ParenExpr:
		return validateDataValue(node.Inner)
	case ast.IdentifierExpr:
		return fmt.Errorf(".octagon does not allow bare identifiers")
	case ast.CallExpr:
		return fmt.Errorf(".octagon does not allow function calls")
	case ast.IndexExpr:
		return fmt.Errorf(".octagon does not allow index expressions")
	case ast.BinaryExpr:
		return fmt.Errorf(".octagon does not allow arithmetic or computed expressions")
	case ast.UnaryExpr:
		return fmt.Errorf(".octagon does not allow unary computed expressions")
	case ast.RangeExpr:
		return fmt.Errorf(".octagon does not allow range expressions")
	case ast.PropagateExpr, ast.UnwrapExpr:
		return fmt.Errorf(".octagon does not allow fallibility operators")
	case ast.SwitchExpr:
		return fmt.Errorf(".octagon does not allow switch expressions")
	case ast.IfExpr:
		return fmt.Errorf(".octagon does not allow if expressions")
	case ast.VectorLiteralExpr, ast.MatrixLiteralExpr:
		return fmt.Errorf(".octagon does not allow vector or matrix literals")
	default:
		return fmt.Errorf(".octagon contains unsupported syntax %T", expr)
	}
}
