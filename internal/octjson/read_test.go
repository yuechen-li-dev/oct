package octjson

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A refined concept is its base type in the document. The lane says whether
// a value is admitted, and the refusal carries the place of the value.
func TestDecodeAsksForAdmission(t *testing.T) {
	port := &Schema{Kind: KindInt, Concept: "Net.Port"}
	weights := &Schema{Kind: KindArray, Elem: floatType, Concept: "Net.Weights"}
	event := &Schema{Kind: KindString, Concept: "Net.Event"}
	var asked []string
	admit := func(concept string, value Data) string {
		asked = append(asked, fmt.Sprintf("%s %+v", concept, value))
		switch {
		case concept == "Net.Port" && value.Int <= 0:
			return "refined concept Port: a port is positive"
		case concept == "Net.Weights" && len(value.Array) == 0:
			return "refined concept Weights: there is at least one weight"
		case concept == "Net.Event" && value.Text == "":
			return "refined concept Event: an event has a name"
		}
		return ""
	}
	cases := []struct {
		name      string
		schema    *Schema
		text      string
		want      Data
		wantError string
	}{
		{name: "admitted", schema: port, text: `8080`, want: i(8080)},
		{name: "refused", schema: port, text: `0`, wantError: `Json.Parse: $ (line 1, column 1): refined concept Port: a port is positive`},
		{name: "the base type is read first", schema: port, text: `"80"`, wantError: `Json.Parse: $ (line 1, column 1): expected Int, found a string`},
		{name: "a field", schema: record("Address", "Host", stringType, "Port", port), text: "{\n  \"host\": \"h\",\n  \"port\": -1\n}",
			wantError: `Json.Parse: $.port (line 3, column 11): refined concept Port: a port is positive`},
		{name: "Some of a refined value", schema: option(port), text: `0`, wantError: `Json.Parse: $ (line 1, column 1): refined concept Port: a port is positive`},
		{name: "None is not asked about", schema: option(port), text: `null`, want: none()},
		{name: "a refined array, whole", schema: weights, text: `[1, 2]`, want: list(f(1), f(2))},
		{name: "a refined array, refused", schema: record("Mix", "Weights", weights), text: `{"weights": []}`,
			wantError: `Json.Parse: $.weights (line 1, column 13): refined concept Weights: there is at least one weight`},
		{name: "an element", schema: arrayOf(port), text: `[1, 0]`, wantError: `Json.Parse: $[1] (line 1, column 5): refined concept Port: a port is positive`},
		{name: "a cell", schema: table("Ports", "Name", stringType, "Port", port), text: `[{"name": "a", "port": 1}, {"name": "b", "port": 0}]`,
			wantError: `Json.Parse: $[1].port (line 1, column 50): refined concept Port: a port is positive`},
		{name: "the single cell of a keyed table", schema: table("Ports", "Name", stringType, "Port", port), text: `{"a": 1, "b": 0}`,
			wantError: `Json.Parse: $.b (line 1, column 15): refined concept Port: a port is positive`},
		{name: "the key of a keyed table", schema: table("Handlers", "Event", event, "Handler", stringType), text: `{"created": "f", "": "g"}`,
			wantError: `Json.Parse: $[""] (line 1, column 18): refined concept Event: an event has a name`},
		{name: "the key of a keyed table, admitted", schema: table("Handlers", "Event", event, "Handler", stringType), text: `{"created": "f"}`,
			want: rec("Handlers", "Event", list(s("created")), "Handler", list(s("f")))},
	}
	for _, c := range cases {
		got, err := ParseAs(c.text, c.schema, admit)
		switch {
		case c.wantError != "":
			if err == nil {
				t.Errorf("%s: %s was accepted as %+v; want %s", c.name, c.text, got, c.wantError)
			} else if err.Error() != c.wantError {
				t.Errorf("%s: %s\n got  %s\n want %s", c.name, c.text, err, c.wantError)
			}
		case err != nil:
			t.Errorf("%s: %s was refused: %s", c.name, c.text, err)
		case !reflect.DeepEqual(got, c.want):
			t.Errorf("%s: %s\n got  %+v\n want %+v", c.name, c.text, got, c.want)
		}
	}

	// The lane is asked with the concept and the value of its base type.
	asked = nil
	if _, err := ParseAs(`[7]`, arrayOf(port), admit); err != nil {
		t.Fatal(err)
	}
	if want := []string{fmt.Sprintf("Net.Port %+v", i(7))}; !reflect.DeepEqual(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}

	// Without a lane to ask, nothing is refused.
	if got, err := ParseAs(`0`, port, nil); err != nil || !reflect.DeepEqual(got, i(0)) {
		t.Errorf("with no admission: %+v, %v", got, err)
	}
}

func TestParseAs(t *testing.T) {
	config := record("Config", "Name", stringType, "Port", intType)
	got, err := ParseAs(`{"name": "api", "port": 8080}`, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := rec("Config", "Name", s("api"), "Port", i(8080)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	for text, want := range map[string]string{
		`{"name": }`:                   `Json.Parse: (line 1, column 10): expected a value, found '}'`,
		`{"name": "api", "port": "x"}`: `Json.Parse: $.port (line 1, column 25): expected Int, found a string`,
	} {
		if _, err := ParseAs(text, config, nil); err == nil || err.Error() != want {
			t.Errorf("%s\n got  %v\n want %s", text, err, want)
		}
	}
}

func TestLoadAs(t *testing.T) {
	directory := t.TempDir()
	write := func(name string, text string) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	levels := arrayOf(floatType)

	good := write("levels.json", "\uFEFF[1, 2.5]\n")
	got, err := LoadAs(good, levels, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := list(f(1), f(2.5)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	broken := write("broken.json", "[1,\n 2,]")
	wrong := write("wrong.json", "[1,\n true]")
	missing := filepath.Join(directory, "missing.json")
	for path, want := range map[string]string{
		broken:    "Json.Load: " + broken + ": (line 2, column 4): expected a value, found ']'",
		wrong:     "Json.Load: " + wrong + ": $[1] (line 2, column 2): expected Float, found a boolean",
		missing:   "Json.Load: " + missing + ": the file does not exist",
		directory: "Json.Load: " + directory + ": this is a directory, not a file",
	} {
		if _, err := LoadAs(path, levels, nil); err == nil || err.Error() != want {
			t.Errorf("%s\n got  %v\n want %s", path, err, want)
		}
	}
}

// Why a file cannot be read is said in words of this package's own.
func TestUnreadable(t *testing.T) {
	cases := []struct {
		err         error
		isDirectory bool
		want        string
	}{
		{&fs.PathError{Op: "open", Path: "a.json", Err: fs.ErrNotExist}, false, "the file does not exist"},
		{&fs.PathError{Op: "open", Path: "a.json", Err: fs.ErrPermission}, false, "the file cannot be read: permission denied"},
		{&fs.PathError{Op: "read", Path: "a", Err: errors.New("is a directory")}, true, "this is a directory, not a file"},
		{errors.New("input/output error"), false, "the file cannot be read"},
	}
	for _, c := range cases {
		if got := unreadable(c.err, c.isDirectory); got != c.want {
			t.Errorf("unreadable(%v, %v) = %q, want %q", c.err, c.isDirectory, got, c.want)
		}
	}
}
