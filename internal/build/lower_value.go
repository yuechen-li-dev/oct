package build

import (
	goast "go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// lowerMIRValue is the temporary syntactic adapter used while the established
// lowering routines still assemble a few scalar expressions compositionally.
// It runs in lowering, never in an emitter, and produces only structured MIR.
func lowerMIRValue(expression, typ string) MIRValue {
	if expression == "" {
		return nil
	}
	if value, ok := lowerResultMarker(expression); ok {
		return value
	}
	node, err := parser.ParseExpr(expression)
	if err != nil {
		return MIRBackendValue{Backend: "go", Expression: expression, Type: typ, Reason: "legacy-unparsed"}
	}
	return lowerGoExprNode(node, expression, typ)
}

func lowerResultMarker(expression string) (MIRValue, bool) {
	isError := strings.HasPrefix(expression, "__oct_err(")
	if !isError && !strings.HasPrefix(expression, "__oct_ok(") {
		return nil, false
	}
	payload := strings.TrimSuffix(expression[strings.Index(expression, "(")+1:], ")")
	parts := strings.SplitN(payload, ",", 2)
	if len(parts) != 2 {
		return nil, false
	}
	resultType, valueText := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if isError {
		return MIRResultValue{ResultType: resultType, Error: lowerMIRValue(valueText, "Error"), IsError: true}, true
	}
	return MIRResultValue{ResultType: resultType, Value: lowerMIRValue(valueText, resultType)}, true
}

func lowerMIRValues(expressions []string, types []string) []MIRValue {
	values := make([]MIRValue, len(expressions))
	for i, expression := range expressions {
		typ := ""
		if i < len(types) {
			typ = types[i]
		}
		values[i] = lowerMIRValue(expression, typ)
	}
	return values
}

func lowerMIRValueWithClone(expression, typ string) MIRValue {
	value := lowerMIRValue(expression, typ)
	if compiledValueNeedsClone(typ) {
		return MIRClone{Value: value, Type: typ}
	}
	return value
}

func lowerGoExprNode(node goast.Expr, original, typ string) MIRValue {
	switch e := node.(type) {
	case *goast.ParenExpr:
		return lowerGoExprNode(e.X, original, typ)
	case *goast.Ident:
		switch e.Name {
		case "true", "false":
			return MIRLiteral{Type: "Bool", Value: e.Name}
		}
		if _, ok := parseCompiledFunctionType(typ); ok || strings.HasPrefix(e.Name, "fn_") {
			return MIRFunctionRef{Name: e.Name, Type: typ}
		}
		return MIRLocal{Name: e.Name, Type: typ}
	case *goast.BasicLit:
		if e.Kind == token.STRING {
			value, err := strconv.Unquote(e.Value)
			if err == nil {
				return MIRLiteral{Type: "String", Value: value}
			}
		}
		literalType := typ
		if literalType == "" {
			if e.Kind == token.FLOAT {
				literalType = "Float"
			} else {
				literalType = "Int"
			}
		}
		return MIRLiteral{Type: literalType, Value: e.Value}
	case *goast.UnaryExpr:
		return MIRUnary{Op: e.Op.String(), Value: lowerGoExprNode(e.X, original, typ), Type: typ}
	case *goast.BinaryExpr:
		return MIRBinary{Op: e.Op.String(), Left: lowerGoExprNode(e.X, original, ""), Right: lowerGoExprNode(e.Y, original, ""), Type: typ}
	case *goast.SelectorExpr:
		return MIRFieldAccess{Target: lowerGoExprNode(e.X, original, ""), Field: e.Sel.Name, Type: typ}
	case *goast.IndexExpr:
		return MIRIndex{Target: lowerGoExprNode(e.X, original, ""), Index: lowerGoExprNode(e.Index, original, "Int"), Type: typ}
	case *goast.TypeAssertExpr:
		return MIREnumPayload{Value: lowerGoExprNode(e.X, original, ""), PayloadType: typ}
	case *goast.CallExpr:
		if fun, ok := e.Fun.(*goast.Ident); ok {
			args := make([]MIRValue, len(e.Args))
			for i := range e.Args {
				args[i] = lowerGoExprNode(e.Args[i], original, "")
			}
			switch fun.Name {
			case "float64":
				return MIRConvert{TargetType: "Float", Value: args[0]}
			case "int":
				return MIRConvert{TargetType: "Int", Value: args[0]}
			case "complex":
				value := args[0]
				if converted, ok := value.(MIRConvert); ok && converted.TargetType == "Float" {
					value = converted.Value
				}
				return MIRConvert{TargetType: "Complex", Value: value}
			case "len":
				return MIRLength{Value: args[0]}
			case "__octClone":
				return MIRClone{Value: args[0], Type: typ}
			case "__octIntArrayToFloat":
				return MIRArrayConvert{Value: args[0], SourceType: "Int[]", TargetType: "Float[]"}
			}
		}
		if selector, ok := e.Fun.(*goast.SelectorExpr); ok {
			if base, ok := selector.X.(*goast.Ident); ok && base.Name == "fmt" && selector.Sel.Name == "Sprint" {
				return MIRIntrinsicValue{Kind: "stringify", Type: "String", Args: []MIRValue{lowerGoExprNode(e.Args[0], original, "")}}
			}
		}
		return MIRBackendValue{Backend: "go", Expression: original, Type: typ, Reason: "legacy-call-expression"}
	case *goast.CompositeLit:
		if id, ok := e.Type.(*goast.Ident); ok && id.Name == "__octRange" {
			values := map[string]MIRValue{}
			flags := map[string]bool{}
			for _, elt := range e.Elts {
				if kv, ok := elt.(*goast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*goast.Ident); ok {
						values[key.Name] = lowerGoExprNode(kv.Value, original, "Int")
						if key.Name == "HasStart" || key.Name == "HasEnd" || key.Name == "HasStep" {
							flags[key.Name] = true
						}
					}
				}
			}
			if values["Start"] == nil {
				values["Start"] = mirInt("0")
			}
			if values["End"] == nil {
				values["End"] = mirInt("0")
			}
			if values["Step"] == nil {
				values["Step"] = mirInt("1")
			}
			return MIRRangeValue{Start: values["Start"], End: values["End"], Step: values["Step"], HasStart: flags["HasStart"], HasEnd: flags["HasEnd"], HasStep: flags["HasStep"]}
		}
		fields := map[string]goast.Expr{}
		for _, elt := range e.Elts {
			if kv, ok := elt.(*goast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*goast.Ident); ok {
					fields[key.Name] = kv.Value
				}
			}
		}
		if tagExpr, ok := fields["Tag"]; ok && typ != "" {
			if tag, ok := tagExpr.(*goast.Ident); ok {
				prefix := enumShortName(typ) + "_"
				variant := strings.TrimSuffix(strings.TrimPrefix(tag.Name, prefix), "_tag")
				var payload MIRValue
				if payloadExpr, ok := fields["Payload"]; ok {
					payload = lowerGoExprNode(payloadExpr, original, "")
				}
				return MIREnumValue{EnumType: typ, Variant: variant, Payload: payload}
			}
		}
		return MIRBackendValue{Backend: "go", Expression: original, Type: typ, Reason: "legacy-composite-expression"}
	case *goast.FuncLit:
		return MIRBackendValue{Backend: "go", Expression: original, Type: typ, Reason: "legacy-closure-expression"}
	default:
		return MIRBackendValue{Backend: "go", Expression: original, Type: typ, Reason: "legacy-expression"}
	}
}
