package builtin

import (
	"path/filepath"
	"testing"
)

func TestJsonBuiltins(t *testing.T) {
	cases := []struct {
		name   string
		action JsonAction
		writes bool
	}{
		{"Json.Load", JsonReadFile, false},
		{"Json.Parse", JsonReadText, false},
		{"Json.Save", JsonWriteFile, true},
		{"Json.Text", JsonWriteText, true},
		{"Artifact.WriteJson", JsonWriteArtifact, true},
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
		if json.Name() != c.name || json.Action != c.action || json.Writes() != c.writes {
			t.Errorf("%s: action %q, writes %v; want %q, %v", json.Name(), json.Action, json.Writes(), c.action, c.writes)
		}
		// Inside its own package a builtin is found by its bare name, which
		// is how a declaration of that name is refused.
		if in, ok := LookupJsonIn(json.Namespace, json.Symbol); !ok || in != json {
			t.Errorf("LookupJsonIn(%q, %q) = %+v, %v", json.Namespace, json.Symbol, in, ok)
		}
		if _, ok := LookupJsonIn("Main", json.Symbol); ok {
			t.Errorf("LookupJsonIn finds %s in package Main", json.Symbol)
		}
	}
	for _, name := range []string{"Json.Decode", "Json.load", "Load", "Artifact.WriteText", "WriteJson", ""} {
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
