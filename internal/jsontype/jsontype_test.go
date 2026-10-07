package jsontype

import (
	"reflect"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/octjson"
	"github.com/yuechen-li-dev/oct/internal/project"
)

func named(name string) ast.TypeRef          { return ast.TypeRef{Name: name} }
func in(pkg string, name string) ast.TypeRef { return ast.TypeRef{Package: pkg, Name: name} }
func arrayOf(t ast.TypeRef) ast.TypeRef      { t.IsArray, t.ArrayDepth = true, t.ArrayDepth+1; return t }
func optionOf(t ast.TypeRef) ast.TypeRef {
	return ast.TypeRef{Name: ast.OptionTypeName, TypeArguments: []ast.TypeRef{t}}
}
func field(name string, t ast.TypeRef) ast.RecordField { return ast.RecordField{Name: name, Type: t} }

func seconds() dimension.Dimension {
	d, _ := dimension.FromBaseName("s")
	return d
}

// The program the tests describe types of: Main imports Net.
func program() project.Program {
	payload := named("Float")
	return project.Program{Packages: map[string]project.Package{
		"Main": {
			Name: "Main",
			Records: []ast.RecordDecl{
				{Name: "Service", Fields: []ast.RecordField{
					field("Name", named("String")),
					field("Listen", in("Net", "Address")),
					field("Timeout", ast.TypeRef{Name: "Float", Dimension: seconds(), HasUnit: true}),
					field("Mode", named("Mode")),
				}},
				{Name: "Ticket", IsTable: true, Fields: []ast.RecordField{
					field("Id", named("String")),
					field("Assignee", optionOf(named("String"))),
				}},
				{Name: "Tree", Fields: []ast.RecordField{
					field("Label", named("String")),
					field("Children", arrayOf(named("Tree"))),
				}},
				{Name: "Reading", Fields: []ast.RecordField{
					field("Phase", named("Complex")),
				}},
				{Name: "Drawing", Fields: []ast.RecordField{
					field("Shapes", arrayOf(named("Shape"))),
				}},
			},
			Enums: []ast.EnumDecl{
				{Name: "Mode", Variants: []ast.EnumVariantDecl{{Name: "Fast"}, {Name: "Safe"}}},
				{Name: "Shape", Variants: []ast.EnumVariantDecl{{Name: "Dot"}, {Name: "Circle", Payload: &payload}}},
			},
		},
		"Net": {
			Name: "Net",
			Records: []ast.RecordDecl{
				{Name: "Address", Fields: []ast.RecordField{
					field("Host", named("String")),
					field("Port", named("Port")),
					field("Backup", optionOf(named("Port"))),
					field("Weights", named("Weights")),
				}},
			},
			Concepts: []ast.ConceptDecl{
				{Name: "Port", Target: named("Int"), Requirements: []ast.RefinementRequirement{{Explanation: "a port is positive"}}},
				{Name: "Weights", Target: arrayOf(named("Float")), Requirements: []ast.RefinementRequirement{{Explanation: "there is at least one weight"}}},
				{Name: "Count", Target: named("Int")},
			},
		},
	}}
}

func scalar(kind octjson.Kind) *octjson.Schema { return &octjson.Schema{Kind: kind} }

func TestOf(t *testing.T) {
	declarations := program()
	port := &octjson.Schema{Kind: octjson.KindInt, Concept: "Net.Port"}
	address := &octjson.Schema{Kind: octjson.KindRecord, Name: "Address", Fields: []octjson.Field{
		{Name: "Host", Type: scalar(octjson.KindString)},
		{Name: "Port", Type: port},
		{Name: "Backup", Type: &octjson.Schema{Kind: octjson.KindOption, Elem: port}},
		{Name: "Weights", Type: &octjson.Schema{Kind: octjson.KindArray, Elem: scalar(octjson.KindFloat), Concept: "Net.Weights"}},
	}}
	cases := []struct {
		name string
		pkg  string
		t    ast.TypeRef
		want *octjson.Schema
	}{
		{"Bool", "Main", named("Bool"), scalar(octjson.KindBool)},
		{"Int", "Main", named("Int"), scalar(octjson.KindInt)},
		{"Float", "Main", named("Float"), scalar(octjson.KindFloat)},
		{"String", "Main", named("String"), scalar(octjson.KindString)},
		{"Float<s>", "Main", ast.TypeRef{Name: "Float", Dimension: seconds(), HasUnit: true}, &octjson.Schema{Kind: octjson.KindFloat, Dimension: "s"}},
		{"Int<s>", "Main", ast.TypeRef{Name: "Int", Dimension: seconds(), HasUnit: true}, &octjson.Schema{Kind: octjson.KindInt, Dimension: "s"}},
		{"Int[]", "Main", arrayOf(named("Int")), &octjson.Schema{Kind: octjson.KindArray, Elem: scalar(octjson.KindInt)}},
		{"Int[] with no depth written", "Main", ast.TypeRef{Name: "Int", IsArray: true}, &octjson.Schema{Kind: octjson.KindArray, Elem: scalar(octjson.KindInt)}},
		{"Int[][]", "Main", arrayOf(arrayOf(named("Int"))), &octjson.Schema{Kind: octjson.KindArray, Elem: &octjson.Schema{Kind: octjson.KindArray, Elem: scalar(octjson.KindInt)}}},
		{"Option<Int>[]", "Main", arrayOf(optionOf(named("Int"))), &octjson.Schema{Kind: octjson.KindArray, Elem: &octjson.Schema{Kind: octjson.KindOption, Elem: scalar(octjson.KindInt)}}},
		{"Vector<Float>", "Main", ast.TypeRef{VectorOf: &ast.TypeRef{Name: "Float"}}, &octjson.Schema{Kind: octjson.KindVector, Elem: scalar(octjson.KindFloat)}},
		{"Matrix<Int>", "Main", ast.TypeRef{MatrixOf: &ast.TypeRef{Name: "Int"}}, &octjson.Schema{Kind: octjson.KindMatrix, Elem: scalar(octjson.KindInt)}},
		{"an enum", "Main", named("Mode"), &octjson.Schema{Kind: octjson.KindEnum, Name: "Mode", Variants: []string{"Fast", "Safe"}}},
		{"a table", "Main", named("Ticket"), &octjson.Schema{Kind: octjson.KindTable, Name: "Ticket", Fields: []octjson.Field{
			{Name: "Id", Type: scalar(octjson.KindString)},
			{Name: "Assignee", Type: &octjson.Schema{Kind: octjson.KindOption, Elem: scalar(octjson.KindString)}},
		}}},
		{"one row of a table", "Main", named("__oct_table_row_Ticket"), &octjson.Schema{Kind: octjson.KindRecord, Name: "Ticket", Fields: []octjson.Field{
			{Name: "Id", Type: scalar(octjson.KindString)},
			{Name: "Assignee", Type: &octjson.Schema{Kind: octjson.KindOption, Elem: scalar(octjson.KindString)}},
		}}},
		{"a row of a record that is not a table", "Main", named("__oct_table_row_Service"), &octjson.Schema{Kind: octjson.KindUnsupported, Name: "__oct_table_row_Service"}},
		{"a record of another package, qualified", "Main", in("Net", "Address"), address},
		{"the same record from inside its package", "Net", named("Address"), address},
		{"a record with a field of another package", "Main", named("Service"), &octjson.Schema{Kind: octjson.KindRecord, Name: "Service", Fields: []octjson.Field{
			{Name: "Name", Type: scalar(octjson.KindString)},
			{Name: "Listen", Type: address},
			{Name: "Timeout", Type: &octjson.Schema{Kind: octjson.KindFloat, Dimension: "s"}},
			{Name: "Mode", Type: &octjson.Schema{Kind: octjson.KindEnum, Name: "Mode", Variants: []string{"Fast", "Safe"}}},
		}}},
		{"a refined concept", "Main", in("Net", "Port"), port},
		{"an alias left named", "Net", named("Count"), scalar(octjson.KindInt)},
		{"Complex", "Main", named("Complex"), &octjson.Schema{Kind: octjson.KindUnsupported, Name: "Complex"}},
		{"a name nothing declares", "Main", named("Address"), &octjson.Schema{Kind: octjson.KindUnsupported, Name: "Address"}},
		{"a function", "Main", ast.TypeRef{Function: &ast.FunctionTypeRef{ReturnType: named("Int")}}, &octjson.Schema{Kind: octjson.KindUnsupported, Name: "a function value"}},
		{"a tuple", "Main", ast.TypeRef{TupleOf: []ast.TypeRef{named("Int"), named("Int")}}, &octjson.Schema{Kind: octjson.KindUnsupported, Name: "a tuple"}},
		{"a flow instance", "Main", ast.TypeRef{FlowInstanceOf: &ast.TypeRef{Name: "Int"}}, &octjson.Schema{Kind: octjson.KindUnsupported, Name: "a flow instance"}},
	}
	for _, c := range cases {
		if got := Of(declarations, c.pkg, c.t); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got  %s\n want %s", c.name, show(got), show(c.want))
		}
	}
}

// What octjson.Check says of a type with a part that has no JSON form.
func TestOfNamesThePartWithNoJSONForm(t *testing.T) {
	declarations := program()
	for name, want := range map[string]string{
		"Reading": "Reading.Phase: Complex has no JSON form",
		"Drawing": "Drawing.Shapes: the enum Shape, whose variant Circle carries a payload, has no JSON form",
		"Service": "",
	} {
		got := ""
		if err := octjson.Check(Of(declarations, "Main", named(name))); err != nil {
			got = err.Error()
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestOfATypeThatNamesItself(t *testing.T) {
	tree := Of(program(), "Main", named("Tree"))
	if tree.Kind != octjson.KindRecord || len(tree.Fields) != 2 {
		t.Fatalf("got %+v", tree)
	}
	if children := tree.Fields[1].Type; children.Kind != octjson.KindArray || children.Elem != tree {
		t.Errorf("Tree.Children is %+v, want an array of the Tree schema itself", children)
	}
	if err := octjson.Check(tree); err != nil {
		t.Errorf("Check: %v", err)
	}
}

// A refined concept is a copy of its base, so the base stays unrefined.
func TestOfDoesNotRefineTheBase(t *testing.T) {
	declarations := program()
	address := Of(declarations, "Main", in("Net", "Address"))
	if port, backup := address.Fields[1].Type, address.Fields[2].Type.Elem; port.Concept != "Net.Port" || backup.Concept != "Net.Port" {
		t.Errorf("Port is %+v and Backup holds %+v", port, backup)
	}
	if weights := address.Fields[3].Type; weights.Concept != "Net.Weights" || weights.Elem.Concept != "" {
		t.Errorf("Weights is %+v of %+v", weights, weights.Elem)
	}
}

func show(s *octjson.Schema) string {
	return showAt(s, map[*octjson.Schema]bool{})
}

func showAt(s *octjson.Schema, seen map[*octjson.Schema]bool) string {
	if s == nil {
		return "nil"
	}
	if seen[s] {
		return "<" + s.Name + ">"
	}
	seen[s] = true
	text := []string{"kind", "Bool", "Int", "Float", "String", "Enum", "Option", "Record", "Array", "Vector", "Matrix", "Table"}[s.Kind]
	if s.Kind == octjson.KindUnsupported {
		text = "Unsupported"
	}
	if s.Name != "" {
		text += " " + s.Name
	}
	if s.Dimension != "" {
		text += "<" + s.Dimension + ">"
	}
	if s.Concept != "" {
		text += " admitted to " + s.Concept
	}
	if s.Elem != nil {
		text += " of (" + showAt(s.Elem, seen) + ")"
	}
	for _, field := range s.Fields {
		text += " " + field.Name + ":(" + showAt(field.Type, seen) + ")"
	}
	for _, variant := range s.Variants {
		text += " ." + variant
	}
	return text
}
