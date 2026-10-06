package octjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The `i_*` files of JSONTestSuite are the ones a parser may accept or
// reject. This parser accepts the ones listed here and rejects the rest:
// it keeps the text of a number however large, so every well-formed number
// parses, and it reads only UTF-8 in which every escape is a character.
var acceptedImplementationDefined = map[string]bool{
	"i_number_double_huge_neg_exp.json":       true,
	"i_number_huge_exp.json":                  true,
	"i_number_neg_int_huge_exp.json":          true,
	"i_number_pos_double_huge_exp.json":       true,
	"i_number_real_neg_overflow.json":         true,
	"i_number_real_pos_overflow.json":         true,
	"i_number_real_underflow.json":            true,
	"i_number_too_big_neg_int.json":           true,
	"i_number_too_big_pos_int.json":           true,
	"i_number_very_big_negative_int.json":     true,
	"i_structure_500_nested_arrays.json":      true,
	"i_structure_UTF-8_BOM_empty_object.json": true,
}

func TestParseJSONTestSuite(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "JSONTestSuite", "test_parsing", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 318 {
		t.Fatalf("found %d JSONTestSuite files, want 318", len(paths))
	}
	counts := map[byte]int{}
	for _, path := range paths {
		name := filepath.Base(path)
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		_, parseErr := Parse(text)
		accepted := parseErr == nil
		counts[name[0]]++
		switch name[0] {
		case 'y':
			if !accepted {
				t.Errorf("%s must be accepted: %s", name, parseErr.Text("Json.Parse", ""))
			}
		case 'n':
			if accepted {
				t.Errorf("%s must be rejected", name)
			}
		case 'i':
			if accepted != acceptedImplementationDefined[name] {
				t.Errorf("%s: accepted = %v, want %v", name, accepted, acceptedImplementationDefined[name])
			}
		default:
			t.Errorf("unexpected JSONTestSuite file %s", name)
		}
		if parseErr != nil && (parseErr.Line < 1 || parseErr.Column < 1 || parseErr.Message == "") {
			t.Errorf("%s: a syntax error without a position or a message: %+v", name, parseErr)
		}
	}
	if counts['y'] != 95 || counts['n'] != 188 || counts['i'] != 35 {
		t.Fatalf("JSONTestSuite has %d accept, %d reject and %d free files; want 95, 188 and 35", counts['y'], counts['n'], counts['i'])
	}
	for name := range acceptedImplementationDefined {
		if _, err := os.Stat(filepath.Join("testdata", "JSONTestSuite", "test_parsing", name)); err != nil {
			t.Errorf("%s is listed as accepted and is not in the suite", name)
		}
	}
}

func mustParse(t *testing.T, text string) *Document {
	t.Helper()
	doc, err := Parse([]byte(text))
	if err != nil {
		t.Fatalf("Parse(%q): %s", text, err.Text("Json.Parse", ""))
	}
	return doc
}

// The tree keeps what a load must not lose: the order of members, a key
// that is written twice, and the text of every number.
func TestParseKeepsOrderRepeatedKeysAndNumberText(t *testing.T) {
	doc := mustParse(t, `{"b": 1, "a": 9007199254740993, "b": 1.50, "z": -0, "e": 1E+2}`)
	var keys, numbers []string
	for _, member := range doc.Root.Members {
		keys = append(keys, member.Key)
		numbers = append(numbers, member.Value.Text)
	}
	if got := strings.Join(keys, ","); got != "b,a,b,z,e" {
		t.Errorf("member keys = %s, want b,a,b,z,e in document order", got)
	}
	if got := strings.Join(numbers, ","); got != "1,9007199254740993,1.50,-0,1E+2" {
		t.Errorf("number texts = %s, want them exactly as written", got)
	}
}

func TestParseValues(t *testing.T) {
	if replaced := mustParse(t, "\"a�b\""); replaced.Root.Text != "a�b" {
		t.Errorf("U+FFFD written as itself was not kept: %q", replaced.Root.Text)
	}
	doc := mustParse(t, ` [null, true, false, "a\"b\\c\/d\b\f\n\r\té😀é😀", [], {}, {"k": [1]}] `)
	elements := doc.Root.Elements
	kinds := []NodeKind{NodeNull, NodeBool, NodeBool, NodeString, NodeArray, NodeObject, NodeObject}
	if len(elements) != len(kinds) {
		t.Fatalf("parsed %d elements, want %d", len(elements), len(kinds))
	}
	for index, kind := range kinds {
		if elements[index].Kind != kind {
			t.Errorf("element %d has kind %d, want %d", index, elements[index].Kind, kind)
		}
	}
	if !elements[1].Bool || elements[2].Bool {
		t.Error("true and false were not read as themselves")
	}
	if want := "a\"b\\c/d\b\f\n\r\té😀é😀"; elements[3].Text != want {
		t.Errorf("string value = %q, want %q", elements[3].Text, want)
	}
	if elements[6].Members[0].Key != "k" || elements[6].Members[0].Value.Elements[0].Text != "1" {
		t.Errorf("nested value was not kept: %+v", elements[6])
	}
}

// A position is a line and a column counted from one, and a column counts
// characters: the `é` before the value below is one column, though it is
// two bytes.
func TestParsePositions(t *testing.T) {
	doc := mustParse(t, "{\n  \"é\": [10,\n\t\"x\"],\r\n  \"k\": true\n}")
	type position struct{ line, column int }
	at := func(offset int) position {
		line, column := doc.Position(offset)
		return position{line, column}
	}
	members := doc.Root.Members
	checks := []struct {
		name string
		got  position
		want position
	}{
		{"the object", at(doc.Root.Offset), position{1, 1}},
		{"the first key", at(members[0].KeyOffset), position{2, 3}},
		{"the array", at(members[0].Value.Offset), position{2, 8}},
		{"10", at(members[0].Value.Elements[0].Offset), position{2, 9}},
		{`"x"`, at(members[0].Value.Elements[1].Offset), position{3, 2}},
		{"the second key", at(members[1].KeyOffset), position{4, 3}},
		{"true", at(members[1].Value.Offset), position{4, 8}},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s is at line %d, column %d; want line %d, column %d", check.name, check.got.line, check.got.column, check.want.line, check.want.column)
		}
	}
}

func TestParseSkipsAByteOrderMarkAndCountsFromTheTextAfterIt(t *testing.T) {
	doc, err := Parse([]byte("\xEF\xBB\xBF{}"))
	if err != nil {
		t.Fatalf("a leading byte order mark was not skipped: %s", err.Text("Json.Parse", ""))
	}
	if doc.Root.Kind != NodeObject {
		t.Fatalf("root kind %d, want an object", doc.Root.Kind)
	}
	if _, err := Parse([]byte("{}\xEF\xBB\xBF")); err == nil {
		t.Error("a byte order mark after the value was accepted")
	}
	// The mark is not a column: the first character after it is column 1.
	for text, want := range map[string]string{
		"\xEF\xBB\xBF":       `Json.Parse: (line 1, column 1): expected a value, found the end of the text`,
		"\xEF\xBB\xBF[1,]":   `Json.Parse: (line 1, column 4): expected a value, found ']'`,
		"\xEF\xBB\xBF\n[1,]": `Json.Parse: (line 2, column 4): expected a value, found ']'`,
	} {
		_, err := Parse([]byte(text))
		if err == nil || err.Text("Json.Parse", "") != want {
			t.Errorf("Parse(%q)\n got  %s\n want %s", text, textOf(err, "Json.Parse", ""), want)
		}
	}
}

func TestParseDepthLimit(t *testing.T) {
	nested := func(depth int) []byte {
		return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
	}
	if _, err := Parse(nested(MaxDepth)); err != nil {
		t.Errorf("%d nested arrays were rejected: %s", MaxDepth, err.Text("Json.Parse", ""))
	}
	_, err := Parse(nested(MaxDepth + 1))
	if err == nil {
		t.Fatalf("%d nested arrays were accepted", MaxDepth+1)
	}
	if want := "Json.Parse: (line 1, column 513): arrays and objects are nested more than 512 deep"; err.Text("Json.Parse", "") != want {
		t.Errorf("got  %s\nwant %s", err.Text("Json.Parse", ""), want)
	}
	objects := func(depth int) []byte {
		return []byte(strings.Repeat(`{"a":`, depth) + "1" + strings.Repeat("}", depth))
	}
	if _, err := Parse(objects(MaxDepth)); err != nil {
		t.Errorf("%d nested objects were rejected: %s", MaxDepth, err.Text("Json.Parse", ""))
	}
	if _, err := Parse(objects(MaxDepth + 1)); err == nil || !strings.Contains(err.Message, "nested more than 512 deep") {
		t.Errorf("%d nested objects: %v", MaxDepth+1, err)
	}
	mixed := []byte(strings.Repeat(`{"a":[`, MaxDepth/2+1))
	if _, err := Parse(mixed); err == nil || !strings.Contains(err.Message, "nested more than 512 deep") {
		t.Errorf("objects and arrays nested together past the limit: %v", err)
	}
}

// Every syntax error says where it is and what was found there, in Oct's
// words. The first case is the example of ladder section 3.8.
func TestParseErrorText(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{`{"a":}`, `Json.Parse: (line 1, column 6): expected a value, found '}'`},
		{`{"a": }`, `Json.Parse: (line 1, column 7): expected a value, found '}'`},
		{``, `Json.Parse: (line 1, column 1): expected a value, found the end of the text`},
		{"  \n ", `Json.Parse: (line 2, column 2): expected a value, found the end of the text`},
		{`[1, 2,]`, `Json.Parse: (line 1, column 7): expected a value, found ']'`},
		{`[1 2]`, `Json.Parse: (line 1, column 4): expected ',' or ']' after an array element, found '2'`},
		{"[1,\n 2", `Json.Parse: (line 2, column 3): expected ',' or ']' after an array element, found the end of the text`},
		{`{"a": 1,}`, `Json.Parse: (line 1, column 9): expected a key in double quotes, found '}'`},
		{`{a: 1}`, `Json.Parse: (line 1, column 2): expected a key in double quotes, found 'a'`},
		{`{'a': 1}`, `Json.Parse: (line 1, column 2): expected a key in double quotes, found "'"`},
		{`{"a" 1}`, `Json.Parse: (line 1, column 6): expected ':' after a key, found '1'`},
		{`{"a": 1 "b": 2}`, `Json.Parse: (line 1, column 9): expected ',' or '}' after a member, found '"'`},
		{`{} x`, `Json.Parse: (line 1, column 4): expected the end of the text, found 'x'`},
		{`[1] // done`, `Json.Parse: (line 1, column 5): expected the end of the text, found '/'`},
		{`tru`, `Json.Parse: (line 1, column 1): expected a value, found 't'`},
		{`nulll`, `Json.Parse: (line 1, column 5): expected the end of the text, found 'l'`},
		{`NaN`, `Json.Parse: (line 1, column 1): expected a value, found 'N'`},
		{`01`, `Json.Parse: (line 1, column 1): a number does not begin with a zero that other digits follow`},
		{`00`, `Json.Parse: (line 1, column 1): a number does not begin with a zero that other digits follow`},
		{`09`, `Json.Parse: (line 1, column 1): a number does not begin with a zero that other digits follow`},
		{`[-01]`, `Json.Parse: (line 1, column 2): a number does not begin with a zero that other digits follow`},
		{`1. 5`, `Json.Parse: (line 1, column 3): expected a digit after the decimal point, found ' '`},
		{"\"a\x80b\"", `Json.Parse: (line 1, column 3): the text is not UTF-8 here`},
		{`"\uD800\`, `Json.Parse: (line 1, column 2): the escape \uD800 is half of a surrogate pair and the other half does not follow it`},
		{`"\u0041`, `Json.Parse: (line 1, column 1): this string has no closing quote`},
		{`"\u004`, `Json.Parse: (line 1, column 2): \u is followed by four hexadecimal digits`},
		{`-`, `Json.Parse: (line 1, column 2): expected a digit, found the end of the text`},
		{`+1`, `Json.Parse: (line 1, column 1): expected a value, found '+'`},
		{`1.`, `Json.Parse: (line 1, column 3): expected a digit after the decimal point, found the end of the text`},
		{`.5`, `Json.Parse: (line 1, column 1): expected a value, found '.'`},
		{`1e`, `Json.Parse: (line 1, column 3): expected a digit in the exponent, found the end of the text`},
		{`1e+x`, `Json.Parse: (line 1, column 4): expected a digit in the exponent, found 'x'`},
		{`"abc`, `Json.Parse: (line 1, column 1): this string has no closing quote`},
		{"\"a\nb\"", `Json.Parse: (line 1, column 3): a string cannot hold the end of the line; write it as an escape`},
		{"\"a\tb\"", `Json.Parse: (line 1, column 3): a string cannot hold the control character U+0009; write it as an escape`},
		{`"a\qb"`, `Json.Parse: (line 1, column 3): '\' is followed by 'q'; the escapes are \" \\ \/ \b \f \n \r \t and \uXXXX`},
		{`"\u12G4"`, `Json.Parse: (line 1, column 2): \u is followed by four hexadecimal digits`},
		{`"\uD800"`, `Json.Parse: (line 1, column 2): the escape \uD800 is half of a surrogate pair and the other half does not follow it`},
		{`"\uDC00"`, `Json.Parse: (line 1, column 2): the escape \uDC00 is half of a surrogate pair and the other half does not follow it`},
		{`"\uD800A"`, `Json.Parse: (line 1, column 2): the escape \uD800 is half of a surrogate pair and the other half does not follow it`},
		{"\"a\xFFb\"", `Json.Parse: (line 1, column 3): the text is not UTF-8 here`},
		{"[\xFF]", `Json.Parse: (line 1, column 2): expected a value, found a byte that is not UTF-8`},
		{"[\x01]", `Json.Parse: (line 1, column 2): expected a value, found the control character U+0001`},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.text))
		if err == nil {
			t.Errorf("Parse(%q) was accepted; want %s", c.text, c.want)
			continue
		}
		if got := err.Text("Json.Parse", ""); got != c.want {
			t.Errorf("Parse(%q)\n got  %s\n want %s", c.text, got, c.want)
		}
	}
}
