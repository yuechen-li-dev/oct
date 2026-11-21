package interpret

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	machinauiir "github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// Explicit placement builtins behind Libraries/UI UI.Place.oct
// (DESIGN.md §5.4): UIPlaceSpan (UI.At), UI.Region and UI.Arrange.

var uiSpanKinds = map[string]machinauiir.SpanKind{
	"Start":    machinauiir.SpanStart,
	"End":      machinauiir.SpanEnd,
	"Stretch":  machinauiir.SpanStretch,
	"Center":   machinauiir.SpanCenter,
	"Fraction": machinauiir.SpanFraction,
}

// uiLayerRanks maps UI.Layer variants to ranks relative to Content.
var uiLayerRanks = map[string]int64{
	"Base": -1, "Content": 0, "Chrome": 1, "Overlay": 2, "Modal": 3, "Tooltip": 4,
}

const uiMinZ, uiMaxZ = -5, 5

func uiQualifiedTypeName(pkgName, typeName string) string {
	if strings.Contains(typeName, ".") {
		return typeName
	}
	return pkgName + "." + typeName
}

func uiLocalTypeName(pkgName, qualified string) string {
	return strings.TrimPrefix(qualified, pkgName+".")
}

// uiTypeNameAs names a Libraries/UI declaration as package pkgName sees it.
func uiTypeNameAs(pkgName, name string) string {
	if pkgName == "UI" {
		return name
	}
	return "UI." + name
}

// decodeUISpan reads a UI.XSpan or UI.YSpan record value.
func decodeUISpan(v Value, axis string) (machinauiir.SpanSpec, error) {
	if v.Kind != ValueRecord {
		return machinauiir.SpanSpec{}, fmt.Errorf("runtime invariant violation: %s span expects a record", axis)
	}
	kindValue := v.Record.Fields["Kind"]
	kind, ok := uiSpanKinds[kindValue.Enum.Variant]
	if kindValue.Kind != ValueEnum || !ok {
		return machinauiir.SpanSpec{}, fmt.Errorf("runtime invariant violation: %s span has no valid Kind", axis)
	}
	if kind == machinauiir.SpanFraction {
		return machinauiir.SpanSpec{Kind: kind, A: v.Record.Fields["From"].Float, B: v.Record.Fields["To"].Float}, nil
	}
	return machinauiir.SpanSpec{Kind: kind, A: v.Record.Fields["A"].Float, B: v.Record.Fields["B"].Float}, nil
}

func (i interpreter) evalUIArgs(env *environment, pkgName string, argumentExprs []ast.Expr) ([]Value, *evalResult, error) {
	values := make([]Value, len(argumentExprs))
	for idx, expr := range argumentExprs {
		r, err := i.evalExpr(env, pkgName, expr)
		if err != nil {
			return nil, nil, err
		}
		if r.hasError {
			return nil, &evalResult{hasError: true, errorVal: r.errorVal}, nil
		}
		values[idx] = r.value
	}
	return values, nil, nil
}

func (i interpreter) evalUIPlacementBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 3 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 3 arguments", callee)
	}
	args, errResult, err := i.evalUIArgs(env, pkgName, argumentExprs)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	switch callee {
	case "UIPlaceSpan":
		x, err := decodeUISpan(args[0], "x")
		if err != nil {
			return evalResult{}, err
		}
		y, err := decodeUISpan(args[1], "y")
		if err != nil {
			return evalResult{}, err
		}
		if args[2].Kind != ValueUI || args[2].UI == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UIPlaceSpan expects UI child")
		}
		box := &machinauiir.BoxSpec{Kind: machinauiir.BoxSpan, XSpan: x, YSpan: y}
		node := &uiirNode{Kind: uiirNodeAnchorBox, Box: box, Children: []*uiirNode{cloneUIIR(args[2].UI)}}
		return evalResult{value: Value{Kind: ValueUI, UI: node}}, nil
	case "UIRegion":
		if args[0].Kind != ValueEnum {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Region expects a region enum value")
		}
		key := uiQualifiedTypeName(pkgName, args[0].Enum.TypeName) + "." + args[0].Enum.Variant
		layer := Value{Kind: ValueEnum, Enum: EnumValue{TypeName: uiTypeNameAs(pkgName, "Layer"), Variant: "Content"}}
		record := RecordValue{
			TypeName:   uiTypeNameAs(pkgName, "Placement"),
			FieldOrder: []string{"Region", "X", "Y", "Layer", "Z"},
			Fields: map[string]Value{
				"Region": {Kind: ValueString, Text: key},
				"X":      args[1],
				"Y":      args[2],
				"Layer":  layer,
				"Z":      {Kind: ValueInt, Int: 0},
			},
		}
		return evalResult{value: Value{Kind: ValueRecord, Record: record}}, nil
	default:
		return i.evalUIArrange(pkgName, args[0], args[1], args[2])
	}
}

// evalUIArrange places each region's content with its layout-table row.
// The table must place every variant of one region enum exactly once, and
// that enum must be the content function's region parameter.
func (i interpreter) evalUIArrange(pkgName string, layout, context, content Value) (evalResult, error) {
	if layout.Kind != ValueRecord || layout.Record.Fields["Rows"].Kind != ValueArray {
		return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Arrange expects a UI.LayoutTable")
	}
	if content.Kind != ValueFunc {
		return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Arrange expects a content function")
	}
	rows := layout.Record.Fields["Rows"].Array
	if len(rows) == 0 {
		return evalResult{}, fmt.Errorf("UI.Arrange: the layout table has no rows")
	}
	enumName := ""
	seen := map[string]bool{}
	type placed struct {
		variant string
		box     *machinauiir.BoxSpec
	}
	placements := make([]placed, 0, len(rows))
	for idx, row := range rows {
		fields := row.Record.Fields
		key := fields["Region"].Text
		dot := strings.LastIndex(key, ".")
		if dot <= 0 {
			return evalResult{}, fmt.Errorf("UI.Arrange: row %d has no region; build rows with UI.Region", idx)
		}
		rowEnum, variant := key[:dot], key[dot+1:]
		if enumName == "" {
			enumName = rowEnum
		} else if rowEnum != enumName {
			return evalResult{}, fmt.Errorf("UI.Arrange: row %d places %s, but the table's regions are %s", idx, uiLocalTypeName(pkgName, key), uiLocalTypeName(pkgName, enumName))
		}
		if seen[variant] {
			return evalResult{}, fmt.Errorf("UI.Arrange: region %s is placed twice", uiLocalTypeName(pkgName, key))
		}
		seen[variant] = true
		x, err := decodeUISpan(fields["X"], "x")
		if err != nil {
			return evalResult{}, err
		}
		y, err := decodeUISpan(fields["Y"], "y")
		if err != nil {
			return evalResult{}, err
		}
		layer, ok := uiLayerRanks[fields["Layer"].Enum.Variant]
		if !ok {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Arrange row %d has no valid Layer", idx)
		}
		z := fields["Z"].Int
		if z < uiMinZ || z > uiMaxZ {
			return evalResult{}, fmt.Errorf("UI.Arrange: region %s has z %d, outside %d..%d; use a Layer to paint above other regions", uiLocalTypeName(pkgName, key), z, uiMinZ, uiMaxZ)
		}
		placements = append(placements, placed{variant: variant, box: &machinauiir.BoxSpec{Kind: machinauiir.BoxSpan, XSpan: x, YSpan: y, Layer: layer, ZOrder: z, Region: uiLocalTypeName(pkgName, key)}})
	}
	enumDecl, ok := i.enums[enumName]
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Arrange region enum %s is not declared", enumName)
	}
	for _, variant := range enumDecl.Variants {
		if !seen[variant.Name] {
			return evalResult{}, fmt.Errorf("UI.Arrange: the layout table does not place region %s.%s", uiLocalTypeName(pkgName, enumName), variant.Name)
		}
	}
	if param, ok := i.contentRegionEnum(content.Function); ok && param != enumName {
		return evalResult{}, fmt.Errorf("UI.Arrange: the layout table places %s regions, but the content function takes %s", uiLocalTypeName(pkgName, enumName), uiLocalTypeName(pkgName, param))
	}
	children := make([]*uiirNode, 0, len(placements))
	for _, p := range placements {
		region := Value{Kind: ValueEnum, Enum: EnumValue{TypeName: uiLocalTypeName(pkgName, enumName), Variant: p.variant}}
		result, err := i.invokeFunctionValue(content.Function, pkgName, []Value{cloneValue(context), region})
		if err != nil {
			return evalResult{}, err
		}
		if result.hasError {
			return evalResult{hasError: true, errorVal: result.errorVal}, nil
		}
		if result.value.Kind != ValueUI || result.value.UI == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: UI.Arrange content for %s did not return UI", p.box.Region)
		}
		children = append(children, &uiirNode{Kind: uiirNodeAnchorBox, Box: p.box, Children: []*uiirNode{cloneUIIR(result.value.UI)}})
	}
	return evalResult{value: Value{Kind: ValueUI, UI: &uiirNode{Kind: uiirNodeAbsoluteBox, Children: children}}}, nil
}

// contentRegionEnum names the region enum a content function declares as
// its second parameter, qualified by package.
func (i interpreter) contentRegionEnum(fn FunctionValue) (string, bool) {
	var params []ast.Parameter
	pkg := ""
	if fn.Anonymous != nil {
		params, pkg = fn.Anonymous.Expression.Parameters, fn.Anonymous.Package
	} else {
		decl, ok := i.functions[fn.Key]
		if !ok {
			return "", false
		}
		params = decl.Parameters
		if dot := strings.Index(fn.Key, "."); dot >= 0 {
			pkg = fn.Key[:dot]
		}
	}
	if len(params) != 2 {
		return "", false
	}
	_, name, ok := i.lookupEnumDecl(pkg, qualifiedTypeRefName(params[1].Type))
	if !ok {
		return "", false
	}
	return uiQualifiedTypeName(pkg, name), true
}
