package octjson

import (
	"reflect"
	"testing"
)

type decodeCase struct {
	name   string
	schema *Schema
	text   string
	// want is the value, when wantError is "".
	want Data
	// wantError is the whole message of Json.Parse.
	wantError string
}

func runDecodeCases(t *testing.T, cases []decodeCase) {
	t.Helper()
	for _, c := range cases {
		got, err := decode(t, c.text, c.schema)
		switch {
		case c.wantError != "":
			if err == nil {
				t.Errorf("%s: %s was accepted as %+v; want %s", c.name, c.text, got, c.wantError)
			} else if text := err.Text("Json.Parse", ""); text != c.wantError {
				t.Errorf("%s: %s\n got  %s\n want %s", c.name, c.text, text, c.wantError)
			}
		case err != nil:
			t.Errorf("%s: %s was refused: %s", c.name, c.text, err.Text("Json.Parse", ""))
		case !reflect.DeepEqual(got, c.want):
			t.Errorf("%s: %s\n got  %+v\n want %+v", c.name, c.text, got, c.want)
		}
	}
}

var (
	seconds = &Schema{Kind: KindFloat, Dimension: "s"}
	metres  = &Schema{Kind: KindInt, Dimension: "m"}
	rank    = enum("Rank", "Low", "VeryHigh")
)

// Ladder 3.3: each representable type, what it accepts and what it refuses.
// There is no conversion between kinds.
func TestDecodeScalars(t *testing.T) {
	runDecodeCases(t, []decodeCase{
		{name: "Bool true", schema: boolType, text: `true`, want: b(true)},
		{name: "Bool false", schema: boolType, text: `false`, want: b(false)},
		{name: "Bool from a number", schema: boolType, text: `1`, wantError: `Json.Parse: $ (line 1, column 1): expected Bool, found a number`},
		{name: "Bool from a string", schema: boolType, text: `"true"`, wantError: `Json.Parse: $ (line 1, column 1): expected Bool, found a string`},
		{name: "Bool from null", schema: boolType, text: `null`, wantError: `Json.Parse: $ (line 1, column 1): expected Bool, found null`},

		{name: "Int", schema: intType, text: `42`, want: i(42)},
		{name: "Int negative", schema: intType, text: `-7`, want: i(-7)},
		{name: "Int minus zero", schema: intType, text: `-0`, want: i(0)},
		{name: "Int largest", schema: intType, text: `9223372036854775807`, want: i(9223372036854775807)},
		{name: "Int smallest", schema: intType, text: `-9223372036854775808`, want: i(-9223372036854775808)},
		{name: "Int above 2^53 is exact", schema: intType, text: `9007199254740993`, want: i(9007199254740993)},
		{name: "Int with a dimension", schema: metres, text: `12`, want: Data{Kind: DataInt, Int: 12, Dimension: "m"}},
		{name: "Int past the largest", schema: intType, text: `9223372036854775808`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found 9223372036854775808, which is outside the 64-bit range`},
		{name: "Int past the smallest", schema: intType, text: `-9223372036854775809`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found -9223372036854775809, which is outside the 64-bit range`},
		{name: "Int with a fraction", schema: intType, text: `1.0`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found 1.0, which has a fraction`},
		{name: "Int with an exponent", schema: intType, text: `1e3`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found 1e3, which has an exponent`},
		{name: "Int with a capital exponent", schema: intType, text: `1E3`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found 1E3, which has an exponent`},
		{name: "Int from a string", schema: intType, text: `"42"`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found a string`},
		{name: "Int from a boolean", schema: intType, text: `true`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found a boolean`},
		{name: "Int from a long number", schema: intType, text: `123456789012345678901234567890123456789012345`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found 1234567890123456789012345678901234567890…, which is outside the 64-bit range`},

		{name: "Float", schema: floatType, text: `1.5`, want: f(1.5)},
		{name: "Float from a whole number", schema: floatType, text: `3`, want: f(3)},
		{name: "Float with an exponent", schema: floatType, text: `-2.5e-3`, want: f(-0.0025)},
		{name: "Float largest", schema: floatType, text: `1.7976931348623157e308`, want: f(1.7976931348623157e308)},
		{name: "Float below the smallest is zero", schema: floatType, text: `1e-400`, want: f(0)},
		{name: "Float with a dimension", schema: seconds, text: `1.5`, want: Data{Kind: DataFloat, Float: 1.5, Dimension: "s"}},
		{name: "Float that is not finite", schema: floatType, text: `1e999`, wantError: `Json.Parse: $ (line 1, column 1): expected Float, found 1e999, which is not finite as a 64-bit float`},
		{name: "Float that is not finite, negative", schema: floatType, text: `-1e999`, wantError: `Json.Parse: $ (line 1, column 1): expected Float, found -1e999, which is not finite as a 64-bit float`},
		{name: "Float from a string", schema: floatType, text: `"1.5"`, wantError: `Json.Parse: $ (line 1, column 1): expected Float, found a string`},

		{name: "String", schema: stringType, text: `"a\nb"`, want: s("a\nb")},
		{name: "String empty", schema: stringType, text: `""`, want: s("")},
		{name: "String from a number", schema: stringType, text: `7`, wantError: `Json.Parse: $ (line 1, column 1): expected String, found a number`},
		{name: "String from an object", schema: stringType, text: `{}`, wantError: `Json.Parse: $ (line 1, column 1): expected String, found an object`},
		{name: "String from an array", schema: stringType, text: `[]`, wantError: `Json.Parse: $ (line 1, column 1): expected String, found an array`},

		{name: "enum", schema: rank, text: `"Low"`, want: variant("Rank", "Low")},
		{name: "enum, as keys match", schema: rank, text: `"very_high"`, want: variant("Rank", "VeryHigh")},
		{name: "enum, another case", schema: rank, text: `"VERY-HIGH"`, want: variant("Rank", "VeryHigh")},
		{name: "enum, no such variant", schema: rank, text: `"Medium"`, wantError: `Json.Parse: $ (line 1, column 1): expected one of "Low", "VeryHigh", found "Medium"`},
		{name: "enum from a number", schema: rank, text: `0`, wantError: `Json.Parse: $ (line 1, column 1): expected one of "Low", "VeryHigh", found a number`},
	})
}

func TestDecodeOptionsArraysVectorsAndMatrices(t *testing.T) {
	runDecodeCases(t, []decodeCase{
		{name: "Option null", schema: option(stringType), text: `null`, want: none()},
		{name: "Option of a value", schema: option(stringType), text: `"sam"`, want: some(s("sam"))},
		{name: "Option of a record", schema: option(record("P", "X", intType)), text: `{"x": 1}`, want: some(rec("P", "X", i(1)))},
		{name: "Option of another kind", schema: option(stringType), text: `7`, wantError: `Json.Parse: $ (line 1, column 1): expected String or null, found a number`},
		{name: "Option of an Int with a fraction", schema: option(intType), text: `1.5`, wantError: `Json.Parse: $ (line 1, column 1): expected Int or null, found 1.5, which has a fraction`},
		{name: "Option of an enum", schema: option(rank), text: `"none"`, wantError: `Json.Parse: $ (line 1, column 1): expected one of "Low", "VeryHigh" or null, found "none"`},

		{name: "array", schema: arrayOf(intType), text: `[1, 2, 3]`, want: list(i(1), i(2), i(3))},
		{name: "array empty", schema: arrayOf(intType), text: `[]`, want: list()},
		{name: "array of options", schema: arrayOf(option(intType)), text: `[1, null]`, want: list(some(i(1)), none())},
		{name: "array of arrays", schema: arrayOf(arrayOf(stringType)), text: `[["a"], []]`, want: list(list(s("a")), list())},
		{name: "array from an object", schema: arrayOf(intType), text: `{}`, wantError: `Json.Parse: $ (line 1, column 1): expected an array, found an object`},
		{name: "array element of another kind", schema: arrayOf(intType), text: "[1,\n \"2\"]", wantError: `Json.Parse: $[1] (line 2, column 2): expected Int, found a string`},
		{name: "array of floats holds floats", schema: arrayOf(floatType), text: `[1, 2.5]`, want: list(f(1), f(2.5))},

		{name: "vector", schema: vector(floatType), text: `[1, 2.5]`, want: list(f(1), f(2.5))},
		{name: "vector from a number", schema: vector(floatType), text: `1`, wantError: `Json.Parse: $ (line 1, column 1): expected an array of numbers, found a number`},
		{name: "vector element", schema: vector(floatType), text: `[1, "x"]`, wantError: `Json.Parse: $[1] (line 1, column 5): expected Float, found a string`},

		{name: "matrix", schema: matrix(floatType), text: `[[1, 2], [3, 4.5]]`, want: list(list(f(1), f(2)), list(f(3), f(4.5)))},
		{name: "matrix empty", schema: matrix(floatType), text: `[]`, want: list()},
		{name: "matrix of Int", schema: matrix(intType), text: `[[1], [2]]`, want: list(list(i(1)), list(i(2)))},
		{name: "matrix with a short row", schema: matrix(floatType), text: `[[1, 2], [3]]`, wantError: `Json.Parse: $[1] (line 1, column 10): this row has 1 numbers and the first row has 2; the rows of a matrix are one length`},
		{name: "matrix with a row that is no array", schema: matrix(floatType), text: `[[1], 2]`, wantError: `Json.Parse: $[1] (line 1, column 7): expected an array of numbers, a row of the matrix, found a number`},
		{name: "matrix from a flat array", schema: matrix(floatType), text: `[1, 2]`, wantError: `Json.Parse: $[0] (line 1, column 2): expected an array of numbers, a row of the matrix, found a number`},
		{name: "matrix from an object", schema: matrix(floatType), text: `{}`, wantError: `Json.Parse: $ (line 1, column 1): expected an array of arrays of numbers, found an object`},
		{name: "matrix element", schema: matrix(intType), text: `[[1, 2.5]]`, wantError: `Json.Parse: $[0][1] (line 1, column 6): expected Int, found 2.5, which has a fraction`},
	})
}

var httpConfig = record("HttpConfig", "Host", stringType, "Port", intType, "ReadTimeoutMs", floatType)

// Ladder 3.4: how the members of an object meet the fields of a record.
func TestDecodeRecords(t *testing.T) {
	want := rec("HttpConfig", "Host", s("h"), "Port", i(80), "ReadTimeoutMs", f(1500))
	optional := record("Ticket", "Id", stringType, "Assignee", option(stringType), "Tags", option(arrayOf(stringType)))
	runDecodeCases(t, []decodeCase{
		{name: "fields as declared", schema: httpConfig, text: `{"Host": "h", "Port": 80, "ReadTimeoutMs": 1500}`, want: want},
		{name: "snake case keys, in another order", schema: httpConfig, text: `{"read_timeout_ms": 1500, "port": 80, "host": "h"}`, want: want},
		{name: "camel case keys", schema: httpConfig, text: `{"host": "h", "port": 80, "readTimeoutMs": 1500}`, want: want},
		{name: "keys with dashes, dots and spaces", schema: httpConfig, text: `{"HOST": "h", "p-o-r-t": 80, "read.timeout ms": 1500}`, want: want},
		{name: "nested record", schema: record("Outer", "Inner", record("Inner", "N", intType)), text: `{"inner": {"n": 1}}`, want: rec("Outer", "Inner", rec("Inner", "N", i(1)))},
		{name: "record with no fields", schema: record("Empty"), text: `{}`, want: rec("Empty")},

		{name: "an option without a member is None", schema: optional, text: `{"id": "T-3"}`, want: rec("Ticket", "Id", s("T-3"), "Assignee", none(), "Tags", none())},
		{name: "an option with null is None", schema: optional, text: `{"id": "T-2", "assignee": null, "tags": []}`, want: rec("Ticket", "Id", s("T-2"), "Assignee", none(), "Tags", some(list()))},
		{name: "an option with a value is Some", schema: optional, text: `{"id": "T-1", "assignee": "sam"}`, want: rec("Ticket", "Id", s("T-1"), "Assignee", some(s("sam")), "Tags", none())},

		{name: "a missing field", schema: httpConfig, text: `{"host": "h", "port": 80}`, wantError: `Json.Parse: $ (line 1, column 1): missing "ReadTimeoutMs"`},
		{name: "several missing fields", schema: httpConfig, text: `{"port": 80}`, wantError: `Json.Parse: $ (line 1, column 1): missing "Host", "ReadTimeoutMs"`},
		{name: "null is not a missing-able value", schema: httpConfig, text: `{"host": null, "port": 80, "readTimeoutMs": 1}`, wantError: `Json.Parse: $.host (line 1, column 10): expected String, found null`},
		{name: "an unknown member", schema: httpConfig, text: `{"host": "h", "port": 80, "readTimeoutMs": 1, "tls": true}`, wantError: `Json.Parse: $ (line 1, column 1): unknown member "tls"; HttpConfig has Host, Port, ReadTimeoutMs`},
		{name: "several unknown members, in document order", schema: httpConfig, text: `{"tls": true, "host": "h", "prot": 1}`, wantError: `Json.Parse: $ (line 1, column 1): unknown members "tls", "prot"; HttpConfig has Host, Port, ReadTimeoutMs`},
		{name: "unknown members are reported before missing ones", schema: httpConfig, text: `{"prot": 1}`, wantError: `Json.Parse: $ (line 1, column 1): unknown member "prot"; HttpConfig has Host, Port, ReadTimeoutMs`},
		{name: "two members for one field", schema: httpConfig, text: `{"host": "a", "Host": "b", "port": 1, "readTimeoutMs": 1}`, wantError: `Json.Parse: $ (line 1, column 15): "host" and "Host" both name the field Host`},
		{name: "a key written twice", schema: httpConfig, text: `{"host": "a", "host": "b", "port": 1, "readTimeoutMs": 1}`, wantError: `Json.Parse: $ (line 1, column 15): the key "host" is written twice`},
		{name: "an unknown key written twice", schema: httpConfig, text: `{"x": 1, "x": 2}`, wantError: `Json.Parse: $ (line 1, column 10): the key "x" is written twice`},
		{name: "a record from an array", schema: httpConfig, text: `[]`, wantError: `Json.Parse: $ (line 1, column 1): expected an object, found an array`},
		{name: "a field of another kind", schema: httpConfig, text: `{"host": "h", "port": "80", "readTimeoutMs": 1}`, wantError: `Json.Parse: $.port (line 1, column 23): expected Int, found a string`},
		{name: "fields are read in declaration order", schema: httpConfig, text: `{"readTimeoutMs": "x", "port": "y", "host": "h"}`, wantError: `Json.Parse: $.port (line 1, column 32): expected Int, found a string`},
		{name: "a key that needs quoting in a path", schema: record("R", "ReadTimeout", record("T", "Ms", intType)), text: `{"read timeout": {"ms": 1.5}}`, wantError: `Json.Parse: $["read timeout"].ms (line 1, column 25): expected Int, found 1.5, which has a fraction`},
	})
}

var (
	ticket = table("Ticket", "Id", stringType, "Assignee", option(stringType))
	retry  = table("Retry", "Event", stringType, "Retries", intType)
	person = table("Person", "Id", stringType, "Name", stringType, "Active", boolType)
)

func columns(typeName string, pairs ...any) Data { return rec(typeName, pairs...) }

// Ladder 3.5: a record table reads from an array of objects, or from a keyed
// object when its first column is a String. The value is the table's columns.
func TestDecodeTables(t *testing.T) {
	site := record("Site", "Code", stringType)
	located := table("Located", "Id", stringType, "Site", site)
	wrapped := table("Wrapped", "Id", stringType, "Site", option(site))
	counted := table("Counted", "Rank", intType, "Count", intType)
	runDecodeCases(t, []decodeCase{
		{name: "array of objects", schema: ticket,
			text: `[ {"id": "T-1", "assignee": "sam"}, {"id": "T-2", "assignee": null}, {"id": "T-3"} ]`,
			want: columns("Ticket", "Id", list(s("T-1"), s("T-2"), s("T-3")), "Assignee", list(some(s("sam")), none(), none()))},
		{name: "empty array", schema: ticket, text: `[]`, want: columns("Ticket", "Id", list(), "Assignee", list())},
		{name: "keyed object, one other column", schema: retry,
			text: `{ "invoice.failed": 5, "user.deleted": 1 }`,
			want: columns("Retry", "Event", list(s("invoice.failed"), s("user.deleted")), "Retries", list(i(5), i(1)))},
		{name: "keyed object, rows are objects", schema: person,
			text: `{ "u-100": {"name": "Avery", "active": true}, "u-101": {"name": "Mina", "active": false} }`,
			want: columns("Person", "Id", list(s("u-100"), s("u-101")), "Name", list(s("Avery"), s("Mina")), "Active", list(b(true), b(false)))},
		{name: "keyed object, one other column written as an object", schema: retry,
			text: `{ "a": {"retries": 2} }`,
			want: columns("Retry", "Event", list(s("a")), "Retries", list(i(2)))},
		{name: "keyed object, the other column is a record", schema: located,
			text: `{ "a": {"code": "D1"} }`,
			want: columns("Located", "Id", list(s("a")), "Site", list(rec("Site", "Code", s("D1"))))},
		{name: "keyed object, the other column is an option of a record", schema: wrapped,
			text: `{ "a": {"code": "D1"}, "b": null }`,
			want: columns("Wrapped", "Id", list(s("a"), s("b")), "Site", list(some(rec("Site", "Code", s("D1"))), none()))},
		{name: "keyed object, empty", schema: retry, text: `{}`, want: columns("Retry", "Event", list(), "Retries", list())},
		{name: "keyed object, an option cell without a member", schema: table("T", "Id", stringType, "A", option(intType), "B", intType),
			text: `{ "k": {"b": 1} }`,
			want: columns("T", "Id", list(s("k")), "A", list(none()), "B", list(i(1)))},

		{name: "a row that is no object", schema: ticket, text: `[{"id": "T-1"}, 7]`, wantError: `Json.Parse: $[1] (line 1, column 17): expected an object, a row of Ticket, found a number`},
		{name: "a row with a missing cell", schema: person, text: "[\n  {\"id\": \"a\", \"name\": \"n\", \"active\": true},\n  {\"id\": \"b\", \"name\": \"m\"}\n]", wantError: `Json.Parse: $[1] (line 3, column 3): missing "Active"`},
		{name: "a row with an unknown member", schema: ticket, text: `[{"id": "T-1", "owner": "x"}]`, wantError: `Json.Parse: $[0] (line 1, column 2): unknown member "owner"; Ticket has Id, Assignee`},
		{name: "a cell of another kind", schema: ticket, text: `[{"id": "T-1"}, {"id": "T-2", "assignee": 7}]`, wantError: `Json.Parse: $[1].assignee (line 1, column 43): expected String or null, found a number`},
		{name: "keyed object, a cell of another kind", schema: retry, text: `{ "invoice.failed": "5" }`, wantError: `Json.Parse: $["invoice.failed"] (line 1, column 21): expected Int, found a string`},
		{name: "keyed object, a row that is no object", schema: person, text: `{ "u-100": true }`, wantError: `Json.Parse: $["u-100"] (line 1, column 12): expected an object, a row of Person, found a boolean`},
		{name: "keyed object, a row that names the key column", schema: person, text: `{ "u": {"id": "u", "name": "n", "active": true} }`, wantError: `Json.Parse: $.u (line 1, column 8): unknown member "id"; Person has Name, Active`},
		{name: "keyed object, a key written twice", schema: retry, text: `{ "a": 1, "a": 2 }`, wantError: `Json.Parse: $ (line 1, column 11): the key "a" is written twice`},
		{name: "an object for a table whose first column is no String", schema: counted, text: `{ "1": 2 }`, wantError: `Json.Parse: $ (line 1, column 1): expected an array of objects, found an object`},
		{name: "a table from a number", schema: retry, text: `3`, wantError: `Json.Parse: $ (line 1, column 1): expected an array of objects or a keyed object, found a number`},
		{name: "the columnar form is not a table", schema: retry, text: `{"event": ["a"], "retries": [1]}`, wantError: `Json.Parse: $.event (line 1, column 11): expected Int, found an array`},
	})
}

// The three load messages of ladder section 3.8, at the positions it shows.
func TestDecodeErrorTextOfTheLadder(t *testing.T) {
	tickets := "[\n  {\n    \"id\": \"T-1\",\n    \"assignee\": \"sam\"\n  },\n  {\n    \"id\": \"T-2\",\n\n    \"assignee\": 7\n  }\n]"
	_, err := decode(t, tickets, ticket)
	if want := `Json.Load: tickets.json: $[1].assignee (line 9, column 17): expected String or null, found a number`; err == nil || err.Text("Json.Load", "tickets.json") != want {
		t.Errorf("got  %v\nwant %s", textOf(err, "Json.Load", "tickets.json"), want)
	}

	config := record("Config", "Service", record("Service", "Http", httpConfig))
	configText := "{\n  \"service\": {\n\n    \"http\": {\"host\": \"h\", \"prot\": 1, \"port\": 80, \"tls\": false, \"read_timeout_ms\": 5}\n  }\n}"
	_, err = decode(t, configText, config)
	if want := `Json.Load: config.json: $.service.http (line 4, column 13): unknown members "prot", "tls"; HttpConfig has Host, Port, ReadTimeoutMs`; err == nil || err.Text("Json.Load", "config.json") != want {
		t.Errorf("got  %v\nwant %s", textOf(err, "Json.Load", "config.json"), want)
	}

	people := record("Directory", "People", table("Person", "Name", stringType, "Active", boolType))
	peopleText := "{\n  \"people\": [\n    {\n      \"name\": \"a\",\n      \"active\": true\n    },\n    {\n      \"name\": \"b\",\n      \"active\": true\n    },\n\n\n\n    {\"name\": \"c\"}\n  ]\n}"
	_, err = decode(t, peopleText, people)
	// The ladder wrote `missing "active"`. The message names the field as
	// the record declares it; the document does not say how it would have
	// spelled a key it left out.
	if want := `Json.Load: people.json: $.people[2] (line 14, column 5): missing "Active"`; err == nil || err.Text("Json.Load", "people.json") != want {
		t.Errorf("got  %v\nwant %s", textOf(err, "Json.Load", "people.json"), want)
	}
}

func textOf(err *Error, operation string, source string) string {
	if err == nil {
		return "no error"
	}
	return err.Text(operation, source)
}
