package octjson

import (
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
)

func encodeText(t *testing.T, value Data, schema *Schema) string {
	t.Helper()
	text, err := Encode(value, schema)
	if err != nil {
		t.Fatalf("Encode: %s", err.Text("Json.Text", ""))
	}
	return string(text)
}

// Ladder 3.6: the layout of what is written.
func TestEncodeLayout(t *testing.T) {
	site := record("Site", "Code", stringType, "Rank", option(rank))
	survey := record("Survey",
		"Name", stringType,
		"Depth", option(seconds),
		"Flow", option(floatType),
		"Readings", arrayOf(option(intType)),
		"Active", boolType,
		"Rank", rank,
		"Sites", arrayOf(site),
		"Grid", matrix(floatType),
		"Axis", vector(intType),
		"Empty", arrayOf(site),
		"Tickets", ticket,
		"NoTickets", ticket,
		"Nothing", record("Nothing"),
	)
	value := rec("Survey",
		"Name", s("delta"),
		"Depth", some(Data{Kind: DataFloat, Float: 2.5, Dimension: "s"}),
		"Flow", none(),
		"Readings", list(some(i(1)), none(), some(i(3))),
		"Active", b(true),
		"Rank", variant("Rank", "VeryHigh"),
		"Sites", list(rec("Site", "Code", s("D1"), "Rank", some(variant("Rank", "Low"))), rec("Site", "Code", s("D2"), "Rank", none())),
		"Grid", list(list(f(1), f(2)), list(f(3), f(4.5))),
		"Axis", list(i(1), i(2)),
		"Empty", list(),
		"Tickets", columns("Ticket", "Id", list(s("T-1"), s("T-2")), "Assignee", list(some(s("sam")), none())),
		"NoTickets", columns("Ticket", "Id", list(), "Assignee", list()),
		"Nothing", rec("Nothing"),
	)
	want := `{
  "Name": "delta",
  "Depth": 2.5,
  "Flow": null,
  "Readings": [1, null, 3],
  "Active": true,
  "Rank": "VeryHigh",
  "Sites": [
    {
      "Code": "D1",
      "Rank": "Low"
    },
    {
      "Code": "D2",
      "Rank": null
    }
  ],
  "Grid": [
    [1.0, 2.0],
    [3.0, 4.5]
  ],
  "Axis": [1, 2],
  "Empty": [],
  "Tickets": [
    {
      "Id": "T-1",
      "Assignee": "sam"
    },
    {
      "Id": "T-2",
      "Assignee": null
    }
  ],
  "NoTickets": [],
  "Nothing": {}
}
`
	if got := encodeText(t, value, survey); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEncodeScalarsAtTheTop(t *testing.T) {
	cases := []struct {
		value  Data
		schema *Schema
		want   string
	}{
		{b(false), boolType, "false\n"},
		{i(-9223372036854775808), intType, "-9223372036854775808\n"},
		{s("x"), stringType, "\"x\"\n"},
		{none(), option(intType), "null\n"},
		{some(i(4)), option(intType), "4\n"},
		{list(), arrayOf(intType), "[]\n"},
		{list(list(i(1)), list()), arrayOf(arrayOf(intType)), "[\n  [1],\n  []\n]\n"},
	}
	for _, c := range cases {
		if got := encodeText(t, c.value, c.schema); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

// A Float is written in the shortest form that reads back as the same value,
// and always with a fraction or an exponent, so that it reads back as a
// Float.
func TestEncodeFloats(t *testing.T) {
	cases := map[float64]string{
		1:                       "1.0",
		-1:                      "-1.0",
		0:                       "0.0",
		1.5:                     "1.5",
		0.1:                     "0.1",
		100:                     "100.0",
		1e20:                    "100000000000000000000.0",
		1e21:                    "1e+21",
		1e-7:                    "1e-07",
		123456789.125:           "123456789.125",
		1500000:                 "1500000.0",
		0.000001:                "0.000001",
		1.7976931348623157e308:  "1.7976931348623157e+308",
		5e-324:                  "5e-324",
		9007199254740993:        "9007199254740992.0",
		0.30000000000000004:     "0.30000000000000004",
		math.Copysign(0, -1):    "-0.0",
		float64(float32(16.25)): "16.25",
	}
	for value, want := range cases {
		got := formatFloat(value)
		if got != want {
			t.Errorf("formatFloat(%v) = %s, want %s", value, got, want)
		}
		back, err := strconv.ParseFloat(got, 64)
		if err != nil || math.Float64bits(back) != math.Float64bits(value) {
			t.Errorf("formatFloat(%v) = %s reads back as %v", value, got, back)
		}
	}
}

// A string escapes `"`, `\` and the control characters, and nothing else.
func TestEncodeStrings(t *testing.T) {
	cases := map[string]string{
		"plain":           `"plain"`,
		"":                `""`,
		"a\"b\\c":         `"a\"b\\c"`,
		"\b\f\n\r\t":      `"\b\f\n\r\t"`,
		"\x00\x01\x1f":    `"\u0000\u0001\u001f"`,
		"é 😀 / <tag> &  ": "\"é 😀 / <tag> &  \"",
		"\x7f":            "\"\x7f\"",
	}
	for value, want := range cases {
		if got := quoteString(value); got != want {
			t.Errorf("quoteString(%q) = %s, want %s", value, got, want)
		}
		doc, err := Parse([]byte(quoteString(value)))
		if err != nil || doc.Root.Text != value {
			t.Errorf("quoteString(%q) does not read back: %v", value, err)
		}
	}
}

// A value JSON cannot hold is not written, and the error says where it is.
// Json.Save stops the program on such an error.
func TestEncodeRefusesWhatJSONCannotHold(t *testing.T) {
	series := record("Series", "Name", stringType, "Levels", arrayOf(option(floatType)))
	cases := []struct {
		value Data
		want  string
	}{
		{rec("Series", "Name", s("a"), "Levels", list(some(f(1)), some(f(math.NaN())))), `Json.Save: out.json: $.Levels[1]: NaN has no JSON form`},
		{rec("Series", "Name", s("a"), "Levels", list(some(f(math.Inf(1))))), `Json.Save: out.json: $.Levels[0]: an infinity has no JSON form`},
		{rec("Series", "Name", s("a"), "Levels", list(some(f(math.Inf(-1))))), `Json.Save: out.json: $.Levels[0]: an infinity has no JSON form`},
		{rec("Series", "Name", s("a\xffb"), "Levels", list()), `Json.Save: out.json: $.Name: this String holds bytes that are not UTF-8`},
	}
	for _, c := range cases {
		text, err := Encode(c.value, series)
		if err == nil {
			t.Errorf("wrote %q; want %s", text, c.want)
			continue
		}
		if got := err.Text("Json.Save", "out.json"); got != c.want || !err.Unwritable {
			t.Errorf("got  %s (unwritable %v)\nwant %s", got, err.Unwritable, c.want)
		}
	}
	levels := table("Levels", "Station", stringType, "Level", floatType)
	_, err := Encode(columns("Levels", "Station", list(s("a"), s("b")), "Level", list(f(1), f(math.NaN()))), levels)
	if want := `Json.Save: out.json: $[1].Level: NaN has no JSON form`; err == nil || err.Text("Json.Save", "out.json") != want {
		t.Errorf("got  %s\nwant %s", textOf(err, "Json.Save", "out.json"), want)
	}
}

// A value that is not what its schema says is a defect of the lane that
// handed it over. It is reported as one, and nothing is written.
func TestEncodeReportsAValueOfTheWrongShape(t *testing.T) {
	cases := []struct {
		value  Data
		schema *Schema
	}{
		{i(1), stringType},
		{s("x"), intType},
		{i(1), floatType},
		{i(1), boolType},
		{s("Low"), rank},
		{i(1), option(intType)},
		{Data{Kind: DataEnum, EnumType: OptionType, Variant: OptionSome}, option(intType)},
		{Data{Kind: DataEnum, EnumType: "Other", Variant: OptionSome, Payload: &Data{Kind: DataInt, Int: 1}}, option(intType)},
		{rec("P"), record("P", "X", intType)},
		{rec("HttpConfig", "Host", s("h")), httpConfig},
		{i(1), httpConfig},
		{i(1), arrayOf(intType)},
		{i(1), matrix(floatType)},
		{i(1), ticket},
		{columns("Ticket", "Id", list(s("a")), "Assignee", list()), ticket},
		{columns("Ticket", "Id", list(s("a"))), ticket},
		{columns("Ticket", "Id", s("a"), "Assignee", list()), ticket},
	}
	for index, c := range cases {
		text, err := Encode(c.value, c.schema)
		if err == nil || err.Unwritable || len(text) != 0 {
			t.Errorf("case %d: wrote %q with error %v; want an internal error", index, text, err)
		}
	}
}

// I2: for every representable value v of a type T, reading what is written
// gives v. The values are generated from generated types.
func TestRoundTripOfGeneratedValues(t *testing.T) {
	random := rand.New(rand.NewSource(20261006))
	for round := 0; round < 3000; round++ {
		schema := randomSchema(random, 0, true)
		if err := Check(schema); err != nil {
			t.Fatalf("round %d: generated a type that is not representable: %v", round, err)
		}
		value := randomValue(random, schema)
		text, encodeErr := Encode(value, schema)
		if encodeErr != nil {
			t.Fatalf("round %d: Encode: %s", round, encodeErr.Text("Json.Text", ""))
		}
		doc, parseErr := Parse(text)
		if parseErr != nil {
			t.Fatalf("round %d: what was written does not parse: %s\n%s", round, parseErr.Text("Json.Parse", ""), text)
		}
		back, decodeErr := Decode(doc, schema, nil)
		if decodeErr != nil {
			t.Fatalf("round %d: what was written does not load: %s\n%s", round, decodeErr.Text("Json.Parse", ""), text)
		}
		if !reflect.DeepEqual(back, value) {
			t.Fatalf("round %d: read back another value\n text %s\n got  %+v\n want %+v", round, text, back, value)
		}
		again, _ := Encode(back, schema)
		if string(again) != string(text) {
			t.Fatalf("round %d: writing the value read back gives other text\n%s\n%s", round, text, again)
		}
	}
}

var generatedNames = []string{"Id", "Name", "Level", "ReadTimeoutMs", "Active", "Tags", "Site", "Rank"}

func randomSchema(random *rand.Rand, depth int, allowOption bool) *Schema {
	scalars := 5
	choices := scalars
	if depth < 3 {
		choices += 6
	}
	for {
		switch random.Intn(choices) {
		case 0:
			return boolType
		case 1:
			if random.Intn(2) == 0 {
				return &Schema{Kind: KindInt, Dimension: "m"}
			}
			return intType
		case 2:
			if random.Intn(2) == 0 {
				return &Schema{Kind: KindFloat, Dimension: "s"}
			}
			return floatType
		case 3:
			return stringType
		case 4:
			return enum("Rank", "Low", "VeryHigh", "read_only")
		case 5:
			if !allowOption {
				continue
			}
			return option(randomSchema(random, depth+1, false))
		case 6:
			return arrayOf(randomSchema(random, depth+1, true))
		case 7:
			return vector(floatType)
		case 8:
			return matrix(intType)
		case 9:
			return &Schema{Kind: KindRecord, Name: "Record", Fields: randomFields(random, depth, 0)}
		default:
			return &Schema{Kind: KindTable, Name: "Table", Fields: randomFields(random, depth, 1)}
		}
	}
}

func randomFields(random *rand.Rand, depth int, atLeast int) []Field {
	count := atLeast + random.Intn(len(generatedNames)-atLeast)
	order := random.Perm(len(generatedNames))
	list := make([]Field, count)
	for index := range list {
		list[index] = Field{Name: generatedNames[order[index]], Type: randomSchema(random, depth+1, true)}
	}
	return list
}

var generatedStrings = []string{"", "a", "u-100", "é 😀", "line\nbreak", "quote\"and\\slash", "\x00\x1f", "null", "1.5", " spaced "}

func randomValue(random *rand.Rand, schema *Schema) Data {
	switch schema.Kind {
	case KindBool:
		return b(random.Intn(2) == 0)
	case KindInt:
		values := []int64{0, 1, -1, 42, 9007199254740993, math.MaxInt64, math.MinInt64, random.Int63()}
		return Data{Kind: DataInt, Int: values[random.Intn(len(values))], Dimension: schema.Dimension}
	case KindFloat:
		values := []float64{0, 1, -1, 0.1, 1.5, 1e21, 1e-7, 5e-324, math.MaxFloat64, 9007199254740993, random.NormFloat64() * 1e6, random.Float64()}
		return Data{Kind: DataFloat, Float: values[random.Intn(len(values))], Dimension: schema.Dimension}
	case KindString:
		return s(generatedStrings[random.Intn(len(generatedStrings))])
	case KindEnum:
		return variant(schema.Name, schema.Variants[random.Intn(len(schema.Variants))])
	case KindOption:
		if random.Intn(3) == 0 {
			return none()
		}
		return some(randomValue(random, schema.Elem))
	case KindArray, KindVector:
		items := make([]Data, random.Intn(4))
		for index := range items {
			items[index] = randomValue(random, schema.Elem)
		}
		return list(items...)
	case KindMatrix:
		rows, width := random.Intn(4), random.Intn(4)
		items := make([]Data, rows)
		for row := range items {
			cells := make([]Data, width)
			for index := range cells {
				cells[index] = randomValue(random, schema.Elem)
			}
			items[row] = list(cells...)
		}
		return list(items...)
	case KindRecord:
		value := Data{Kind: DataRecord, RecordType: schema.Name, Fields: make([]DataField, len(schema.Fields))}
		for index, field := range schema.Fields {
			value.Fields[index] = DataField{Name: field.Name, Value: randomValue(random, field.Type)}
		}
		return value
	default:
		rows := random.Intn(4)
		value := Data{Kind: DataRecord, RecordType: schema.Name, Fields: make([]DataField, len(schema.Fields))}
		for index, field := range schema.Fields {
			cells := make([]Data, rows)
			for row := range cells {
				cells[row] = randomValue(random, field.Type)
			}
			value.Fields[index] = DataField{Name: field.Name, Value: list(cells...)}
		}
		return value
	}
}
