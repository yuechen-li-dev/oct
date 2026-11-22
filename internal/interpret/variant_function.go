package interpret

import "github.com/yuechen-li-dev/oct/internal/ast"

// variantFunction is a payload variant named without its payload,
// `Msg.SetAudio`, as a function value (DESIGN.md §7 L2 in Libraries/UI).
func variantFunction(typeName, variant string) Value {
	return Value{Kind: ValueFunc, Function: FunctionValue{Variant: &EnumValue{TypeName: typeName, Variant: variant}}}
}

// buildVariant calls a variant function: the variant around its one
// argument.
func buildVariant(function FunctionValue, arguments []Value) Value {
	payload := cloneValue(arguments[0])
	return Value{Kind: ValueEnum, Enum: EnumValue{TypeName: function.Variant.TypeName, Variant: function.Variant.Variant, Payload: &payload}}
}

func (i interpreter) callVariantFunction(env *environment, pkgName string, function FunctionValue, arguments []ast.Expr) (evalResult, error) {
	values := make([]Value, 0, len(arguments))
	for _, argumentExpr := range arguments {
		argument, err := i.evalExpr(env, pkgName, argumentExpr)
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		values = append(values, argument.value)
	}
	return evalResult{value: buildVariant(function, values)}, nil
}
