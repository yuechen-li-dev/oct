package octjson

// DataKind is the kind of an Octagon data value.
type DataKind int

const (
	DataInt DataKind = iota
	DataFloat
	DataBool
	DataString
	DataArray
	DataRecord
	DataEnum
)

// Data is an Octagon data value: what `.octagon` text denotes, and what each
// lane's LoadOctagon materialiser turns into a value of the declared type. A
// decode makes one from a JSON document; an encode writes one.
//
// A vector is a DataArray of numbers and a matrix a DataArray of rows. A
// record table is a DataRecord whose fields are whole columns, as its
// Octagon literal is. An option is a DataEnum of type `Option`.
type Data struct {
	Kind  DataKind
	Int   int64
	Float float64
	// Dimension is the dimension of a DataInt or a DataFloat, as the
	// schema gave it.
	Dimension string
	Bool      bool
	Text      string
	Array     []Data
	// RecordType and Fields are a DataRecord, fields in declaration order.
	RecordType string
	Fields     []DataField
	// EnumType, Variant and Payload are a DataEnum. Payload is nil for a
	// variant that carries none.
	EnumType string
	Variant  string
	Payload  *Data
}

// DataField is one field of a DataRecord.
type DataField struct {
	Name  string
	Value Data
}

// The builtin Option<T>, as Octagon data names it.
const (
	OptionType = "Option"
	OptionNone = "None"
	OptionSome = "Some"
)

// Field answers with the named field of a DataRecord.
func (d Data) Field(name string) (Data, bool) {
	for _, field := range d.Fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return Data{}, false
}

func none() Data { return Data{Kind: DataEnum, EnumType: OptionType, Variant: OptionNone} }

func some(payload Data) Data {
	return Data{Kind: DataEnum, EnumType: OptionType, Variant: OptionSome, Payload: &payload}
}
