package build

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/jsontype"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// octjsonImportPath is the package that implements the Json builtins for
// both lanes. A generated program imports it directly, so the compiled lane
// runs the same Go code as the interpreter rather than an emitted copy.
const octjsonImportPath = "github.com/yuechen-li-dev/oct/internal/octjson"

// lowerJsonCall lowers `Json.Load<T>(path)` and `Json.Parse<T>(text)` to a
// builtin call that carries the schema of T.
func (c *lowerCtx) lowerJsonCall(json builtin.JsonBuiltin, call ast.CallExpr) (string, string, bool, error) {
	if len(call.TypeArguments) != 1 || len(call.Arguments) != 1 {
		return "", "", false, fmt.Errorf("function '%s' expects 1 type argument and 1 argument", json.Name())
	}
	argument, _, _, err := c.lowerExpr(call.Arguments[0])
	if err != nil {
		return "", "", false, err
	}
	ret := typeRefStringForPackage(c.pkg.Name, call.TypeArguments[0])
	tmp := c.temp(fallibleType(ret))
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
		Target:  tmp,
		Callee:  json.Name(),
		Args:    lowerMIRValues([]string{argument}, nil),
		Builtin: true,
		RetType: ret,
		JSON:    jsontype.Of(c.program, c.pkg.Name, call.TypeArguments[0]),
	})
	return tmp, ret, true, nil
}

// jsonReadName names the generated function for one Json builtin at one
// type, such as `__octJsonLoad_Main_Service`.
func jsonReadName(json builtin.JsonBuiltin, valueType string) string {
	return "__octJson" + json.Symbol + "_" + goSafeName(valueType)
}

// noteJsonRead records a Json call the program makes, by the function that
// will serve it.
func noteJsonRead(statement MIRStmt, reads map[string]MIRCall) {
	call, ok := statement.(MIRCall)
	if !ok || !call.Builtin {
		return
	}
	if json, ok := builtin.LookupJson(call.Callee); ok {
		reads[jsonReadName(json, call.RetType)] = call
	}
}

// emitJsonCall emits a call to a Json builtin, and reports whether the call
// is one. Its value is the Go result type of T, as for any fallible call.
func emitJsonCall(call MIRCall, args []string) (string, bool, error) {
	json, ok := builtin.LookupJson(call.Callee)
	if !ok || !call.Builtin {
		return "", false, nil
	}
	if len(args) != 1 {
		return "", true, fmt.Errorf("function '%s' expects 1 argument, got %d", json.Name(), len(args))
	}
	return fmt.Sprintf("%s = %s(%s)", call.Target, jsonReadName(json, call.RetType), args[0]), true, nil
}

// jsonHelpers adapts octjson to the generated program: it turns the Octagon
// data octjson gives into the value the Octagon materialiser takes. It holds
// no rule about JSON.
const jsonHelpers = `
func __octJsonParsed(data octjson.Data) __octParsedValue {
	switch data.Kind {
	case octjson.DataInt:
		return __octParsedValue{Kind: __octParsedInt, Int: int(data.Int), Dimension: data.Dimension}
	case octjson.DataFloat:
		return __octParsedValue{Kind: __octParsedFloat, Float: data.Float, Dimension: data.Dimension}
	case octjson.DataBool:
		return __octParsedValue{Kind: __octParsedBool, Bool: data.Bool}
	case octjson.DataString:
		return __octParsedValue{Kind: __octParsedString, Text: data.Text}
	case octjson.DataArray:
		items := make([]__octParsedValue, len(data.Array))
		for i, item := range data.Array {
			items[i] = __octJsonParsed(item)
		}
		return __octParsedValue{Kind: __octParsedArray, Array: items}
	case octjson.DataRecord:
		fields := make(map[string]__octParsedValue, len(data.Fields))
		for _, field := range data.Fields {
			fields[field.Name] = __octJsonParsed(field.Value)
		}
		return __octParsedValue{Kind: __octParsedRecord, RecordType: data.RecordType, RecordFields: fields}
	default:
		value := __octParsedValue{Kind: __octParsedEnum, EnumType: data.EnumType, EnumVariant: data.Variant}
		if data.Payload != nil {
			value.EnumHasPayload = true
			value.EnumPayload = []__octParsedValue{__octJsonParsed(*data.Payload)}
		}
		return value
	}
}

func __octJsonValue(operation string, data octjson.Data, refusal error, target reflect.Type, expectedType string) (any, error) {
	if refusal != nil {
		return nil, refusal
	}
	value, err := __octMaterialize(__octJsonParsed(data), target, expectedType)
	if err != nil {
		// octjson has read the text as the type, so the data is of the type.
		panic("runtime invariant violation: " + operation + " read a value its type does not take: " + err.Error())
	}
	return value.Interface(), nil
}
`

// emitJsonSupport writes what the program's Json calls need: the helpers,
// the admission of refined values, and for each call its schema and the
// function that serves it.
func emitJsonSupport(b *strings.Builder, m MIRModule, reads map[string]MIRCall) error {
	b.WriteString(jsonHelpers)

	// __octJsonAdmit is octjson.Admit: it runs the checked constructor of
	// the refined concept on the value, loaded as the concept's base type.
	b.WriteString("\nfunc __octJsonAdmit(concept string, value octjson.Data) string {\n\tswitch concept {\n")
	for _, refinement := range m.Refinements {
		fmt.Fprintf(b, "\tcase %q:\n", refinement.Package+"."+refinement.Name)
		fmt.Fprintf(b, "\t\tbase, err := __octMaterialize(__octJsonParsed(value), reflect.TypeOf((*%s)(nil)).Elem(), %q)\n", goType(refinement.Base), refinement.Base)
		b.WriteString("\t\tif err != nil { return err.Error() }\n")
		fmt.Fprintf(b, "\t\tif checked := fn_%s___oct_refine_%s(base.Interface().(%s)); checked.IsErr { return checked.Err }\n", refinement.Package, refinement.Name, goType(refinement.Base))
	}
	b.WriteString("\t}\n\treturn \"\"\n}\n\n")

	names := make([]string, 0, len(reads))
	for name := range reads {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		call := reads[name]
		json, _ := builtin.LookupJson(call.Callee)
		schema, err := goJsonSchema(call.JSON)
		if err != nil {
			return fmt.Errorf("%s<%s>: %w", json.Name(), call.RetType, err)
		}
		read := "octjson.ParseAs"
		if json.Source == builtin.JsonFromFile {
			read = "octjson.LoadAs"
		}
		result := goResultTypeName(call.RetType)
		fmt.Fprintf(b, "var %s_schema = %s\n\n", name, schema)
		fmt.Fprintf(b, "func %s(source string) %s {\n", name, result)
		fmt.Fprintf(b, "\tdata, refusal := %s(source, %s_schema, __octJsonAdmit)\n", read, name)
		fmt.Fprintf(b, "\tvalue, err := __octJsonValue(%q, data, refusal, reflect.TypeOf((*%s)(nil)).Elem(), %q)\n", json.Name(), goType(call.RetType), call.RetType)
		fmt.Fprintf(b, "\tif err != nil {\n\t\treturn %s{Err: err.Error(), IsErr: true}\n\t}\n", result)
		fmt.Fprintf(b, "\treturn %s{Value: value.(%s)}\n}\n\n", result, goType(call.RetType))
	}
	return nil
}

// goJsonSchema writes a schema as a Go expression that builds it. A schema
// may point back to itself, so the expression makes every node first and
// fills them in after.
func goJsonSchema(root *octjson.Schema) (string, error) {
	if root == nil {
		return "", fmt.Errorf("the call carries no schema")
	}
	index := map[*octjson.Schema]int{}
	var nodes []*octjson.Schema
	var number func(s *octjson.Schema)
	number = func(s *octjson.Schema) {
		if _, seen := index[s]; seen || s == nil {
			return
		}
		index[s] = len(nodes)
		nodes = append(nodes, s)
		number(s.Elem)
		for _, field := range s.Fields {
			number(field.Type)
		}
	}
	number(root)

	var b strings.Builder
	fmt.Fprintf(&b, "func() *octjson.Schema {\n\ts := make([]octjson.Schema, %d)\n", len(nodes))
	for i, node := range nodes {
		kind, err := goJsonKind(node.Kind)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\ts[%d] = octjson.Schema{Kind: octjson.%s", i, kind)
		if node.Name != "" {
			fmt.Fprintf(&b, ", Name: %q", node.Name)
		}
		if node.Dimension != "" {
			fmt.Fprintf(&b, ", Dimension: %q", node.Dimension)
		}
		if node.Concept != "" {
			fmt.Fprintf(&b, ", Concept: %q", node.Concept)
		}
		if node.Elem != nil {
			fmt.Fprintf(&b, ", Elem: &s[%d]", index[node.Elem])
		}
		if len(node.Fields) > 0 {
			b.WriteString(", Fields: []octjson.Field{")
			for j, field := range node.Fields {
				if j > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "{Name: %q, Type: &s[%d]}", field.Name, index[field.Type])
			}
			b.WriteString("}")
		}
		if len(node.Variants) > 0 {
			fmt.Fprintf(&b, ", Variants: %#v", node.Variants)
		}
		b.WriteString("}\n")
	}
	b.WriteString("\treturn &s[0]\n}()")
	return b.String(), nil
}

// goJsonKind is the name of a schema kind in package octjson.
func goJsonKind(kind octjson.Kind) (string, error) {
	switch kind {
	case octjson.KindBool:
		return "KindBool", nil
	case octjson.KindInt:
		return "KindInt", nil
	case octjson.KindFloat:
		return "KindFloat", nil
	case octjson.KindString:
		return "KindString", nil
	case octjson.KindEnum:
		return "KindEnum", nil
	case octjson.KindOption:
		return "KindOption", nil
	case octjson.KindRecord:
		return "KindRecord", nil
	case octjson.KindArray:
		return "KindArray", nil
	case octjson.KindVector:
		return "KindVector", nil
	case octjson.KindMatrix:
		return "KindMatrix", nil
	case octjson.KindTable:
		return "KindTable", nil
	default:
		// The typechecker refuses a type with no JSON form.
		return "", fmt.Errorf("a type with no JSON form reached the compiled lane")
	}
}
