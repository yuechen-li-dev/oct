package builtin

import "testing"

func TestEveryReservedBuiltinNameHasOneCanonicalDefinition(t *testing.T) {
	for name := range names {
		definition, ok := Lookup(name)
		if !ok {
			t.Errorf("reserved builtin %q has no semantic definition", name)
			continue
		}
		if definition.Name != CanonicalName(name) {
			t.Errorf("builtin %q resolves to %q, want %q", name, definition.Name, CanonicalName(name))
		}
	}
}

func TestEveryNamespaceAliasResolvesToCanonicalDefinition(t *testing.T) {
	for namespace, aliases := range namespaceAliases {
		for symbol, canonical := range aliases {
			alias := namespace + "." + symbol
			definition, ok := Lookup(alias)
			if !ok {
				t.Errorf("alias %q has no semantic definition", alias)
				continue
			}
			if definition.Name != canonical {
				t.Errorf("alias %q resolves to %q, want %q", alias, definition.Name, canonical)
			}
		}
	}
}

func TestRegularBuiltinCallShapesAreValid(t *testing.T) {
	known := 0
	for _, definition := range Definitions() {
		shape := definition.CallShape
		if !shape.Known {
			continue
		}
		known++
		if shape.MinimumArguments < 0 || shape.MaximumArguments < shape.MinimumArguments {
			t.Errorf("builtin %q has invalid argument shape %#v", definition.Name, shape)
		}
		if shape.MinimumTypeArgs < 0 || shape.MaximumTypeArgs < shape.MinimumTypeArgs {
			t.Errorf("builtin %q has invalid type-argument shape %#v", definition.Name, shape)
		}
		if definition.ReturnRule == "" {
			t.Errorf("builtin %q has no return-type rule or semantic hook", definition.Name)
		}
		if len(definition.ParameterConstraints) > 0 && shape.MinimumArguments == shape.MaximumArguments && len(definition.ParameterConstraints) != shape.MinimumArguments {
			t.Errorf("builtin %q has %d parameter constraints for %d arguments", definition.Name, len(definition.ParameterConstraints), shape.MinimumArguments)
		}
	}
	if known != len(regularCallShapes) {
		t.Fatalf("registered %d regular call shapes, want %d", known, len(regularCallShapes))
	}
}
