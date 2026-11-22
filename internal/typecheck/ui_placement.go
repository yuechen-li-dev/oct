package typecheck

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// uiPackageType names a record or enum declared in Libraries/UI as the
// checked package sees it.
func (c checker) uiPackageType(name string) Type {
	if c.packageName == "UI" {
		return Type{Name: name}
	}
	return Type{Name: "UI." + name}
}

func (c checker) checkUIArgument(scope *scope, callee string, index int, argument ast.Expr, ctx functionContext) (Type, error) {
	return c.checkUIArgumentWithExpected(scope, argument, ctx, nil)
}

func (c checker) checkUIArgumentWithExpected(scope *scope, argument ast.Expr, ctx functionContext, expected *Type) (Type, error) {
	argType, err := c.checkExprWithExpected(scope, argument, ctx, expected)
	if err != nil {
		return Type{}, err
	}
	if argType.Fallible {
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	return argType.ValueType, nil
}

// regionEnum reports whether t names a region enum: a user enum (not
// Option) whose variants carry no payload.
func (c checker) regionEnum(t Type) (string, error) {
	if t.Name == "" || t.IsArray || t.IsFunction || isOptionTypeName(t.Name) {
		return "", fmt.Errorf("expects a region enum value, got %s", t)
	}
	enum, ok := c.lookupEnum(t.Name)
	if !ok {
		return "", fmt.Errorf("expects a region enum value, got %s", t)
	}
	for name, variant := range enum.variants {
		if variant.payload != nil {
			return "", fmt.Errorf("expects a region enum whose variants carry no payload; %s.%s has one", t.Name, name)
		}
	}
	return t.Name, nil
}

// checkUIPlacementCall types the explicit-placement builtins behind
// Libraries/UI (DESIGN.md §5.4):
//
//	UIPlaceSpan(x: UI.XSpan, y: UI.YSpan, child: UI) -> UI   (UI.At)
//	UI.Region(region: R, x: UI.XSpan, y: UI.YSpan) -> UI.Placement
//	UI.Arrange(layout: UI.LayoutTable, context: C, content: fn(C, R) -> UI) -> UI
//
// R is any payload-free enum. Region and Arrange are compiler-polymorphic
// over R and C the way Array.Where is over its element type; whether a
// table places every region of the content function's R exactly once is
// checked when UI.Arrange runs.
func (c checker) checkUIPlacementCall(scope *scope, callee string, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	written := map[string]string{"UIPlaceSpan": "UIPlaceSpan", "UIRegion": "UI.Region", "UIArrange": "UI.Arrange"}[callee]
	if len(typeArguments) > 0 {
		return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", written)
	}
	if len(arguments) != 3 {
		return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", written, len(arguments))
	}
	args := make([]Type, 3)
	for idx, argument := range arguments {
		t, err := c.checkUIArgument(scope, written, idx, argument, ctx)
		if err != nil {
			return ExprType{}, err
		}
		args[idx] = t
	}
	expect := func(idx int, want Type, label string) error {
		if args[idx] != want {
			return fmt.Errorf("function '%s' argument %d expects %s, got %s", written, idx+1, label, args[idx])
		}
		return nil
	}
	xSpan, ySpan := c.uiPackageType("XSpan"), c.uiPackageType("YSpan")
	switch callee {
	case "UIPlaceSpan":
		if err := expect(0, xSpan, "UI.XSpan"); err != nil {
			return ExprType{}, err
		}
		if err := expect(1, ySpan, "UI.YSpan"); err != nil {
			return ExprType{}, err
		}
		if !isUIShape(args[2], 0) {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects UI, got %s", written, args[2])
		}
		return ExprType{ValueType: uiOf(args[2].UIMessage, 0)}, nil
	case "UIRegion":
		if _, err := c.regionEnum(args[0]); err != nil {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 %v", written, err)
		}
		if err := expect(1, xSpan, "UI.XSpan (for example UI.Left(16px, 240px))"); err != nil {
			return ExprType{}, err
		}
		if err := expect(2, ySpan, "UI.YSpan (for example UI.Top(16px, 56px))"); err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: c.uiPackageType("Placement")}, nil
	default: // UIArrange
		if err := expect(0, c.uiPackageType("LayoutTable"), "UI.LayoutTable"); err != nil {
			return ExprType{}, err
		}
		content := args[2]
		if !content.IsFunction {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects a content function (C, R) -> UI, got %s", written, content)
		}
		signature, ok := c.functionTypes[content.FunctionSignature]
		if !ok {
			return ExprType{}, fmt.Errorf("internal error: missing function type metadata for %s", content.FunctionSignature)
		}
		if len(signature.parameters) != 2 || signature.isFallible || !isUIShape(signature.returnType, 0) {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects a content function (%s, R) -> UI, got %s", written, args[1], content)
		}
		if signature.parameters[0] != args[1] {
			return ExprType{}, fmt.Errorf("function '%s' content function takes %s first, but the context argument is %s", written, signature.parameters[0], args[1])
		}
		if _, err := c.regionEnum(signature.parameters[1]); err != nil {
			return ExprType{}, fmt.Errorf("function '%s' content function's second parameter %v", written, err)
		}
		return ExprType{ValueType: uiOf(c.uiMessageOfFunctionResult(signature), 0)}, nil
	}
}
