package octjson

import (
	"reflect"
	"testing"
)

func TestFoldName(t *testing.T) {
	same := [][]string{
		{"read_timeout_ms", "readTimeoutMs", "ReadTimeoutMs", "READ-TIMEOUT-MS", "read.timeout ms", "r_e_a_d_timeoutms"},
		{"Id", "id", "ID", "_id", "i-d"},
		{"Größe", "GRÖSSE"[:0] + "größe", "GRÖßE"},
	}
	for _, group := range same {
		for _, name := range group {
			if FoldName(name) != FoldName(group[0]) {
				t.Errorf("%q and %q are one name and fold to %q and %q", group[0], name, FoldName(group[0]), FoldName(name))
			}
		}
	}
	different := [][2]string{{"Host", "Hosts"}, {"a1", "a_2"}, {"Name", "Nome"}, {"", "a"}}
	for _, pair := range different {
		if FoldName(pair[0]) == FoldName(pair[1]) {
			t.Errorf("%q and %q are two names and both fold to %q", pair[0], pair[1], FoldName(pair[0]))
		}
	}
}

// Check names the part of a type that has no JSON form.
func TestCheck(t *testing.T) {
	node := &Schema{Kind: KindRecord, Name: "Node"}
	node.Fields = fields("Label", stringType, "Children", arrayOf(node))
	representable := []*Schema{
		boolType, intType, floatType, stringType, rank,
		option(rank), arrayOf(option(intType)), vector(floatType), matrix(intType),
		httpConfig, ticket, record("Empty"), node,
		record("Outer", "Rows", ticket, "Config", option(httpConfig)),
	}
	for index, schema := range representable {
		if err := Check(schema); err != nil {
			t.Errorf("representable type %d was refused: %v", index, err)
		}
	}

	complexType := &Schema{Kind: KindUnsupported, Name: "Complex"}
	payload := &Schema{Kind: KindUnsupported, Name: "the payload enum Shape"}
	refused := []struct {
		schema *Schema
		want   string
	}{
		{complexType, `Complex has no JSON form`},
		{record("Reading", "Station", stringType, "Phase", complexType), `Reading.Phase: Complex has no JSON form`},
		{record("Reading", "Shapes", arrayOf(option(payload))), `Reading.Shapes: the payload enum Shape has no JSON form`},
		{record("Outer", "Inner", record("Inner", "Phase", complexType)), `Inner.Phase: Complex has no JSON form`},
		{table("Rows", "Id", stringType, "Phase", complexType), `Rows.Phase: Complex has no JSON form`},
		{option(option(intType)), "an Option of an Option has no JSON form: `null` could be either None"},
		{record("R", "Maybe", option(option(intType))), "R.Maybe: an Option of an Option has no JSON form: `null` could be either None"},
		{record("R", "ReadTimeout", intType, "read_timeout", intType), `the fields ReadTimeout and read_timeout of R are one name in JSON, where case and separators are ignored`},
		{table("T", "Id", stringType, "ID", intType), `the fields Id and ID of T are one name in JSON, where case and separators are ignored`},
		{record("Outer", "Inner", record("R", "A_b", intType, "AB", intType)), `Outer.Inner: the fields A_b and AB of R are one name in JSON, where case and separators are ignored`},
		{enum("Mode", "ReadOnly", "read_only"), `the variants ReadOnly and read_only of Mode are one name in JSON, where case and separators are ignored`},
		{vector(stringType), `a vector or a matrix of anything but Int or Float has no JSON form`},
		{record("R", "Grid", matrix(boolType)), `R.Grid: a vector or a matrix of anything but Int or Float has no JSON form`},
	}
	for _, c := range refused {
		err := Check(c.schema)
		if err == nil {
			t.Errorf("accepted a type that is not representable; want %s", c.want)
		} else if err.Error() != c.want {
			t.Errorf("got  %s\nwant %s", err.Error(), c.want)
		}
	}
}

// A type that names itself is a schema that points back to itself. It loads
// and writes like any other.
func TestRecursiveTypes(t *testing.T) {
	node := &Schema{Kind: KindRecord, Name: "Node"}
	node.Fields = fields("Label", stringType, "Children", arrayOf(node))
	text := `{"label": "root", "children": [{"label": "leaf", "children": []}]}`
	value, err := decode(t, text, node)
	if err != nil {
		t.Fatalf("a recursive type did not load: %s", err.Text("Json.Parse", ""))
	}
	written := encodeText(t, value, node)
	want := "{\n  \"Label\": \"root\",\n  \"Children\": [\n    {\n      \"Label\": \"leaf\",\n      \"Children\": []\n    }\n  ]\n}\n"
	if written != want {
		t.Errorf("got:\n%s\nwant:\n%s", written, want)
	}
}

func TestErrorTextForms(t *testing.T) {
	cases := []struct {
		err       Error
		operation string
		source    string
		want      string
	}{
		{Error{Path: "$[1].assignee", Line: 9, Column: 17, Message: "m"}, "Json.Load", "tickets.json", "Json.Load: tickets.json: $[1].assignee (line 9, column 17): m"},
		{Error{Line: 1, Column: 7, Message: "m"}, "Json.Parse", "", "Json.Parse: (line 1, column 7): m"},
		{Error{Line: 1, Column: 7, Message: "m"}, "Json.Load", "a.json", "Json.Load: a.json: (line 1, column 7): m"},
		{Error{Path: "$.Levels[1]", Message: "m"}, "Json.Save", "out.json", "Json.Save: out.json: $.Levels[1]: m"},
		{Error{Path: "$", Message: "m"}, "Json.Text", "", "Json.Text: $: m"},
		{Error{Message: "m"}, "Json.Load", "a.json", "Json.Load: a.json: m"},
	}
	for _, c := range cases {
		if got := c.err.Text(c.operation, c.source); got != c.want {
			t.Errorf("got  %s\nwant %s", got, c.want)
		}
	}
}

func TestCheckReportsAMalformedSchema(t *testing.T) {
	for _, schema := range []*Schema{nil, {Kind: Kind(99)}, arrayOf(nil), {Kind: KindRecord, Name: "R", Fields: []Field{{Name: "A"}}}} {
		if err := Check(schema); err == nil {
			t.Errorf("a malformed schema %+v was accepted", schema)
		}
	}
}

// A type with no JSON form is refused when the program is compiled. If one
// reaches a load or a write anyway, it is an error there and not a guess.
func TestUnsupportedTypesAreErrorsAtRunTimeToo(t *testing.T) {
	complexType := &Schema{Kind: KindUnsupported, Name: "Complex"}
	_, err := decode(t, `1`, complexType)
	if want := "Json.Parse: $ (line 1, column 1): Complex has no JSON form"; err == nil || err.Text("Json.Parse", "") != want {
		t.Errorf("got  %s\nwant %s", textOf(err, "Json.Parse", ""), want)
	}
	_, err = Encode(i(1), complexType)
	if want := "Json.Text: $: Complex has no JSON form"; err == nil || err.Text("Json.Text", "") != want || err.Unwritable {
		t.Errorf("got  %s\nwant %s", textOf(err, "Json.Text", ""), want)
	}
	if got := option(complexType).expects(); got != "Complex or null" {
		t.Errorf("expects() = %q", got)
	}
}

func TestErrorIsAnError(t *testing.T) {
	var err error = &Error{Path: "$.a", Line: 2, Column: 3, Message: "m"}
	if got := err.Error(); got != "Json: $.a (line 2, column 3): m" {
		t.Errorf("Error() = %q", got)
	}
}

func TestPathsAndPositionsAtTheEdges(t *testing.T) {
	inner := record("Inner", "N", intType)
	for key, want := range map[string]string{"z": "$.z.n", "Z": "$.Z.n", "a0": "$.a0.n", "a9": "$.a9.n", "0a": `$["0a"].n`, "a b": `$["a b"].n`} {
		_, err := decode(t, `{"`+key+`": {"n": 1.5}}`, record("One", key, inner))
		if err == nil || err.Path != want {
			t.Errorf("%s: path %v, want %s", key, err, want)
		}
	}
	outer := record("Outer", "", inner, "A1", inner, "_b", inner, "1c", inner, "d-e", inner)
	cases := map[string]string{
		`{"": {"n": 1.5}, "A1": {"n": 1}, "_b": {"n": 1}, "1c": {"n": 1}, "d-e": {"n": 1}}`: `$[""].n`,
		`{"": {"n": 1}, "A1": {"n": 1.5}, "_b": {"n": 1}, "1c": {"n": 1}, "d-e": {"n": 1}}`: `$.A1.n`,
		`{"": {"n": 1}, "A1": {"n": 1}, "_b": {"n": 1.5}, "1c": {"n": 1}, "d-e": {"n": 1}}`: `$._b.n`,
		`{"": {"n": 1}, "A1": {"n": 1}, "_b": {"n": 1}, "1c": {"n": 1.5}, "d-e": {"n": 1}}`: `$["1c"].n`,
		`{"": {"n": 1}, "A1": {"n": 1}, "_b": {"n": 1}, "1c": {"n": 1}, "d-e": {"n": 1.5}}`: `$["d-e"].n`,
	}
	for text, want := range cases {
		_, err := decode(t, text, outer)
		if err == nil || err.Path != want {
			t.Errorf("%s: path %v, want %s", text, err, want)
		}
	}

	doc := mustParse(t, "[1,\n22]")
	for offset, want := range map[int][2]int{0: {1, 1}, 3: {1, 4}, 4: {2, 1}, 7: {2, 4}, 99: {2, 4}} {
		if line, column := doc.Position(offset); line != want[0] || column != want[1] {
			t.Errorf("offset %d is at line %d, column %d; want line %d, column %d", offset, line, column, want[0], want[1])
		}
	}
}

// A long piece of the document is shortened in a message, and never in the
// middle of a character.
func TestClip(t *testing.T) {
	long := `"` + "éééééééééééééééééééééééééééééé" + `"`
	_, err := decode(t, long, rank)
	want := `Json.Parse: $ (line 1, column 1): expected one of "Low", "VeryHigh", found "éééééééééééééééééééé…"`
	if err == nil || err.Text("Json.Parse", "") != want {
		t.Errorf("got  %s\nwant %s", textOf(err, "Json.Parse", ""), want)
	}
	if got := clip("a" + long[1:]); got != "aééééééééééééééééééé…" {
		t.Errorf("a cut inside a character: %q", got)
	}
	if got := clip("0123456789012345678901234567890123456789"); got != "0123456789012345678901234567890123456789" {
		t.Errorf("a text of exactly the limit was shortened: %q", got)
	}
}

// A keyed table whose one other column is itself a keyed table takes each
// member's object as that cell.
func TestDecodeAKeyedTableOfKeyedTables(t *testing.T) {
	inner := table("Inner", "Key", stringType, "Count", intType)
	outer := table("Outer", "Group", stringType, "Members", inner)
	got, err := decode(t, `{"g": {"a": 1, "b": 2}}`, outer)
	if err != nil {
		t.Fatalf("refused: %s", err.Text("Json.Parse", ""))
	}
	want := columns("Outer", "Group", list(s("g")), "Members", list(columns("Inner", "Key", list(s("a"), s("b")), "Count", list(i(1), i(2)))))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	// An array-only table as the one other column is not read from an
	// object, so the object holds the cell under its name.
	plain := table("Plain", "Rank", intType, "Count", intType)
	holder := table("Holder", "Group", stringType, "Rows", plain)
	got, err = decode(t, `{"g": {"rows": [{"rank": 1, "count": 2}]}}`, holder)
	if err != nil {
		t.Fatalf("refused: %s", err.Text("Json.Parse", ""))
	}
	want = columns("Holder", "Group", list(s("g")), "Rows", list(columns("Plain", "Rank", list(i(1)), "Count", list(i(2)))))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
