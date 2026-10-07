package builtin

import (
	"path/filepath"
	"testing"
)

func TestJsonBuiltinsResolve(t *testing.T) {
	cases := []struct {
		callee        string
		typeArguments int
		want          string
	}{
		{"Json.Load", 1, "Json.Load"},
		{"Json.Parse", 1, "Json.Parse"},
		// A missing type argument is the builtin's own error to report.
		{"Json.Parse", 0, "Json.Parse"},
		{"Json.Load", 2, "Json.Load"},
		// Until the first Json library leaves, `Json.Load(path)` is its.
		{"Json.Load", 0, ""},
		{"Json.Save", 1, ""},
		{"Json.Object", 0, ""},
		{"IO.Load", 1, ""},
		{"Load", 1, ""},
	}
	for _, c := range cases {
		got, ok := ResolveJsonCall(c.callee, c.typeArguments)
		if name := got.Name(); (c.want == "") == ok || (ok && name != c.want) {
			t.Errorf("ResolveJsonCall(%q, %d) = %q, %v; want %q", c.callee, c.typeArguments, name, ok, c.want)
		}
	}
	if load, ok := LookupJson("Json.Load"); !ok || load.Source != JsonFromFile {
		t.Errorf("Json.Load = %+v, %v; want a builtin that reads a file", load, ok)
	}
	if parse, ok := LookupJson("Json.Parse"); !ok || parse.Source != JsonFromText {
		t.Errorf("Json.Parse = %+v, %v; want a builtin that reads text", parse, ok)
	}
	table := JsonBuiltins()
	table[0].Symbol = "Changed"
	if _, ok := LookupJson("Json.Load"); !ok {
		t.Errorf("JsonBuiltins hands out the table itself")
	}
}

// The Json builtins are typed and run from the table in json.go, so each
// part of the compiler must consult it rather than name them.
func TestJsonBuiltinsHaveImplementationCoverage(t *testing.T) {
	for part, selector := range map[string]string{
		"typecheck": "builtin.ResolveJsonCall",
		"interpret": "builtin.ResolveJsonCall",
		"build":     "builtin.ResolveJsonCall",
	} {
		if _, ok := implementationSelectors(t, filepath.Join("..", part))[selector]; !ok {
			t.Errorf("internal/%s does not consult %s, so the Json builtins are not implemented there", part, selector)
		}
	}
	for _, part := range []string{"typecheck", "interpret", "build"} {
		literals := implementationStringLiterals(t, filepath.Join("..", part))
		for _, json := range JsonBuiltins() {
			if _, named := literals[json.Name()]; named {
				t.Errorf("internal/%s names builtin %q; it must come from the table in json.go", part, json.Name())
			}
		}
	}
}
