package validate

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// ValidatedTestDecl is the compiler-owned M29a handoff. Its attributes,
// literals, assertions, and positions are normal AST data; consumers must not
// rescan source text to rediscover them.
type ValidatedTestDecl struct {
	Function     ast.FunctionDecl
	Span         source.Span
	Fact, Theory *ast.Attribute
	Rows         []ValidatedTheoryRow
	Launch       ValidatedLaunchMetadata
	Assertions   []ValidatedAssertCall
}

type ValidatedTheoryRow struct {
	Attribute ast.Attribute
	Values    []ast.Expr
	Span      source.Span
}

type ValidatedLaunchMetadata struct {
	WorkgroupSize, DispatchGroups [3]uint32
	WorkgroupAttribute            *ast.Attribute
	DispatchAttribute             *ast.Attribute
}

type ValidatedAssertCall struct {
	Call     ast.CallExpr
	Operands []ast.Expr
	Span     source.Span
}

// ValidatedTests converts the already parsed, validated test declarations into
// the sole downstream test metadata contract.
func ValidatedTests(module ast.Module) ([]ValidatedTestDecl, error) {
	if err := Module(module); err != nil {
		return nil, err
	}
	var out []ValidatedTestDecl
	for _, decl := range module.Decls {
		fn, ok := decl.(ast.FunctionDecl)
		if !ok {
			continue
		}
		d := ValidatedTestDecl{Function: fn, Span: fn.Span, Launch: ValidatedLaunchMetadata{WorkgroupSize: [3]uint32{1, 1, 1}, DispatchGroups: [3]uint32{1, 1, 1}}}
		for i := range fn.Attributes {
			a := &fn.Attributes[i]
			switch a.Name {
			case "Fact":
				d.Fact = a
			case "Theory":
				d.Theory = a
			case "InlineData":
				d.Rows = append(d.Rows, ValidatedTheoryRow{Attribute: *a, Values: a.Arguments, Span: a.Span})
			case "WorkgroupSize":
				if err := launch(&d.Launch.WorkgroupSize, a.Arguments); err != nil {
					return nil, fmt.Errorf("%d:%d: %w", a.Span.Start.Line, a.Span.Start.Column, err)
				}
				d.Launch.WorkgroupAttribute = a
			case "DispatchGroups":
				if err := launch(&d.Launch.DispatchGroups, a.Arguments); err != nil {
					return nil, fmt.Errorf("%d:%d: %w", a.Span.Start.Line, a.Span.Start.Column, err)
				}
				d.Launch.DispatchAttribute = a
			}
		}
		if d.Fact != nil || d.Theory != nil {
			d.Assertions = assertionCalls(fn.Body)
			out = append(out, d)
		}
	}
	return out, nil
}

func launch(dst *[3]uint32, args []ast.Expr) error {
	if len(args) != 3 {
		return fmt.Errorf("launch attribute requires three positive integer constants")
	}
	for i, e := range args {
		x, ok := e.(ast.IntegerLiteral)
		if !ok {
			return fmt.Errorf("launch attribute arguments must be integer constants")
		}
		var n uint64
		if _, err := fmt.Sscanf(x.Value, "%du", &n); err != nil {
			if _, err := fmt.Sscanf(x.Value, "%d", &n); err != nil {
				return fmt.Errorf("launch attribute arguments must be positive integer constants")
			}
		}
		if n == 0 || n > 0xffffffff {
			return fmt.Errorf("launch attribute arguments must be positive integer constants")
		}
		dst[i] = uint32(n)
	}
	return nil
}

func assertionCalls(b ast.Block) []ValidatedAssertCall {
	var out []ValidatedAssertCall
	for _, s := range b.Statements {
		if x, ok := s.(ast.ExprStmt); ok {
			if c, ok := x.Value.(ast.CallExpr); ok && testAssertName(c.Callee) != "" {
				out = append(out, ValidatedAssertCall{Call: c, Operands: c.Arguments, Span: c.Span})
			}
		}
	}
	return out
}
