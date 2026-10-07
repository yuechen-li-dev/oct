package builtin

import (
	"path/filepath"
	"testing"
)

func TestJsonBuiltins(t *testing.T) {
	cases := []struct {
		name             string
		action           JsonAction
		writes           bool
		firstLibraryForm bool
	}{
		{"Json.Load", JsonReadFile, false, true},
		{"Json.Parse", JsonReadText, false, false},
		{"Json.Save", JsonWriteFile, true, true},
		{"Json.Text", JsonWriteText, true, false},
		{"Artifact.WriteJson", JsonWriteArtifact, true, true},
	}
	if len(JsonBuiltins()) != len(cases) {
		t.Errorf("the table has %d builtins, and %d are described here", len(JsonBuiltins()), len(cases))
	}
	for _, c := range cases {
		json, ok := LookupJson(c.name)
		if !ok {
			t.Errorf("LookupJson(%q) finds nothing", c.name)
			continue
		}
		if json.Name() != c.name || json.Action != c.action || json.Writes() != c.writes || json.HasFirstLibraryForm() != c.firstLibraryForm {
			t.Errorf("%s: action %q, writes %v, first library form %v; want %q, %v, %v",
				json.Name(), json.Action, json.Writes(), json.HasFirstLibraryForm(), c.action, c.writes, c.firstLibraryForm)
		}
	}
	for _, name := range []string{"Json.Object", "Json.Decode", "IO.Load", "Load", "Artifact.WriteText", "WriteJson", ""} {
		if json, ok := LookupJson(name); ok {
			t.Errorf("LookupJson(%q) = %+v, want nothing", name, json)
		}
	}
	table := JsonBuiltins()
	table[0].Symbol = "Changed"
	if _, ok := LookupJson("Json.Load"); !ok {
		t.Errorf("JsonBuiltins hands out the table itself")
	}
}

// The Json builtins are parsed, typed and run from the table in json.go, so
// each part of the compiler must consult it rather than name them.
func TestJsonBuiltinsHaveImplementationCoverage(t *testing.T) {
	for _, part := range []string{"parse", "typecheck", "interpret", "build"} {
		if _, ok := implementationSelectors(t, filepath.Join("..", part))["builtin.LookupJson"]; !ok {
			t.Errorf("internal/%s does not consult builtin.LookupJson, so the Json builtins are not implemented there", part)
		}
		literals := implementationStringLiterals(t, filepath.Join("..", part))
		for _, json := range JsonBuiltins() {
			if _, named := literals[json.Name()]; named {
				t.Errorf("internal/%s names builtin %q; it must come from the table in json.go", part, json.Name())
			}
		}
	}
}
