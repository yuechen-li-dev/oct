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
	if w, ok := map[string]string{"UIButtonMsg": "UI.Button", "UICheckbox": "UI.Checkbox", "UITextField": "UI.TextField", "UISlider": "UI.Slider"}[callee]; ok {
		written = w
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
		// An input's last argument is its callback; see callbacks below.
		"UICheckbox":    {str, boolean, {}},
		"UITextField":   {str, {}},
		"UISlider":      {float, float, float, {}},
		"UIStyleStates": {c.uiPackageType("StyleStates"), ui},
	}[callee]
	// callbacks gives an input's callback position and the value it is
	// called with: `on: fn(Bool) -> M` builds the message for a proposed
	// value (DESIGN.md §5.9).
	callback, isInput := map[string]struct {
		index int
		value Type
	}{
		"UICheckbox":  {2, boolean},
		"UITextField": {1, str},
		"UISlider":    {3, float},
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
		if isInput && idx == callback.index {
			message, err := c.inputCallbackMessage(t, callback.value)
			if err != nil {
				return ExprType{}, fmt.Errorf("function '%s' argument %d %v", written, idx+1, err)
			}
			join.addMessage(idx, message)
			continue
		}
		if params[idx].Base == BaseTypeUI {
			if err := c.checkUIValueArgument(scope, join, idx, params[idx].ArrayDepth, t); err != nil {
				return ExprType{}, err
			}
			continue
		}
		if !isAssignable(t, params[idx]) {
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

// inputCallbackMessage checks an input's callback, fn(value) -> M, and
// names M. A payload variant such as Msg.SetAudio is one.
func (c checker) inputCallbackMessage(t Type, value Type) (string, error) {
	want := fmt.Sprintf("expects a callback fn(%s) -> Msg that builds the app's message (for example a payload variant such as Msg.Set)", value)
	if !t.IsFunction {
		return "", fmt.Errorf("%s, got %s", want, t)
	}
	signature, ok := c.functionTypes[t.FunctionSignature]
	if !ok {
		return "", fmt.Errorf("internal error: missing function type metadata for %s", t.FunctionSignature)
	}
	if len(signature.parameters) != 1 || signature.parameters[0] != value || signature.isFallible || !c.isMessageEnum(signature.returnType) {
		return "", fmt.Errorf("%s, got %s", want, t)
	}
	return signature.returnType.Name, nil
}
