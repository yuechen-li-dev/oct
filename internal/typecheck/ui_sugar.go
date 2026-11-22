package typecheck

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// checkUISugarCall types the stack/grid sugar, keyed identity and typed
// button builtins behind Libraries/UI (DESIGN.md §5.2, §5.5, §5.6, §5.8).
// All but UIButtonMsg are backing builtins for ordinary UI package
// functions; UIButtonMsg is UI.Button, which is generic over the message
// enum, so it is reached through the UI namespace alias.
func (c checker) checkUISugarCall(scope *scope, callee string, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	written := callee
	if callee == "UIButtonMsg" {
		written = "UI.Button"
	}
	if len(typeArguments) > 0 {
		return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", written)
	}
	ui := uiOf(uiMessageAny, 0)
	float := Type{Base: BaseTypeFloat}
	integer := Type{Base: BaseTypeInt}
	str := Type{Base: BaseTypeString}
	boolean := Type{Base: BaseTypeBool}
	params := map[string][]Type{
		"UIStack":      {c.uiPackageType("StackSpec"), withArrayDepth(ui, 1)},
		"UISizing":     {integer, float, float, ui},
		"UINudge":      {float, float, ui},
		"UIKeyed":      {str, ui},
		"UIGridMatrix": {c.uiPackageType("GridSpec"), withArrayDepth(ui, 2)},
		"UISkip":       {},
		"UISpan":       {integer, integer, ui},
		"UIButtonMsg":  {str, {}, boolean},
		"UITextWith":   {c.uiPackageType("TextSpec"), str},
	}[callee]
	if len(arguments) != len(params) {
		return ExprType{}, fmt.Errorf("function '%s' expects %d arguments, got %d", written, len(params), len(arguments))
	}
	join := newUIMessageJoin(written)
	for idx, argument := range arguments {
		var expected *Type
		if params[idx].Base == BaseTypeUI {
			expected = &params[idx]
		}
		t, err := c.checkUIArgumentWithExpected(scope, argument, ctx, expected)
		if err != nil {
			return ExprType{}, err
		}
		if callee == "UIButtonMsg" && idx == 1 {
			if !c.isMessageEnum(t) {
				return ExprType{}, fmt.Errorf("function 'UI.Button' argument 2 expects a value of the app's message enum (for example Msg.Raise), got %s", t)
			}
			join.addMessage(idx, t.Name)
			continue
		}
		if params[idx].Base == BaseTypeUI {
			if err := c.checkUIValueArgument(scope, join, idx, params[idx].ArrayDepth, t); err != nil {
				return ExprType{}, err
			}
			continue
		}
		if t != params[idx] {
			return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", written, idx+1, params[idx], t)
		}
	}
	return join.result(0)
}

// isMessageEnum reports a user enum (not Option) usable as a UI message.
func (c checker) isMessageEnum(t Type) bool {
	if t.Name == "" || t.IsArray || t.IsFunction || isOptionTypeName(t.Name) {
		return false
	}
	_, ok := c.lookupEnum(t.Name)
	return ok
}
