package octjson

import "testing"

// Schemas for the tests, written the way a lane would build them.
var (
	boolType   = &Schema{Kind: KindBool}
	intType    = &Schema{Kind: KindInt}
	floatType  = &Schema{Kind: KindFloat}
	stringType = &Schema{Kind: KindString}
)

func option(of *Schema) *Schema  { return &Schema{Kind: KindOption, Elem: of} }
func arrayOf(of *Schema) *Schema { return &Schema{Kind: KindArray, Elem: of} }
func vector(of *Schema) *Schema  { return &Schema{Kind: KindVector, Elem: of} }
func matrix(of *Schema) *Schema  { return &Schema{Kind: KindMatrix, Elem: of} }

func enum(name string, variants ...string) *Schema {
	return &Schema{Kind: KindEnum, Name: name, Variants: variants}
}

// fields takes name, type, name, type, ...
func fields(pairs ...any) []Field {
	list := make([]Field, 0, len(pairs)/2)
	for index := 0; index < len(pairs); index += 2 {
		list = append(list, Field{Name: pairs[index].(string), Type: pairs[index+1].(*Schema)})
	}
	return list
}

func record(name string, pairs ...any) *Schema {
	return &Schema{Kind: KindRecord, Name: name, Fields: fields(pairs...)}
}

func table(name string, pairs ...any) *Schema {
	return &Schema{Kind: KindTable, Name: name, Fields: fields(pairs...)}
}

// Data values for the tests.
func i(value int64) Data   { return Data{Kind: DataInt, Int: value} }
func f(value float64) Data { return Data{Kind: DataFloat, Float: value} }
func b(value bool) Data    { return Data{Kind: DataBool, Bool: value} }
func s(value string) Data  { return Data{Kind: DataString, Text: value} }
func list(items ...Data) Data {
	if items == nil {
		items = []Data{}
	}
	return Data{Kind: DataArray, Array: items}
}
func variant(enumType string, name string) Data {
	return Data{Kind: DataEnum, EnumType: enumType, Variant: name}
}

// rec takes the type name, then name, value, name, value, ...
func rec(typeName string, pairs ...any) Data {
	value := Data{Kind: DataRecord, RecordType: typeName, Fields: []DataField{}}
	for index := 0; index < len(pairs); index += 2 {
		value.Fields = append(value.Fields, DataField{Name: pairs[index].(string), Value: pairs[index+1].(Data)})
	}
	return value
}

func decode(t *testing.T, text string, schema *Schema) (Data, *Error) {
	t.Helper()
	doc, err := Parse([]byte(text))
	if err != nil {
		t.Fatalf("Parse(%q): %s", text, err.Text("Json.Parse", ""))
	}
	return Decode(doc, schema, nil)
}
