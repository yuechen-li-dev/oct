package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBuiltinDefinitionsHaveImplementationCoverage(t *testing.T) {
	typecheckLiterals := implementationStringLiterals(t, filepath.Join("..", "typecheck"))
	interpreterLiterals := implementationStringLiterals(t, filepath.Join("..", "interpret"))
	compiledLiterals := implementationStringLiterals(t, filepath.Join("..", "build"))

	for _, definition := range Definitions() {
		if _, tableDriven := LookupRandom(definition.Name); tableDriven {
			// Random builtins are covered by TestRandomBuiltinsHaveImplementationCoverage.
			continue
		}
		if !definitionMentioned(typecheckLiterals, definition) {
			t.Errorf("public builtin %q has no typechecker implementation coverage", definition.Name)
		}
		if hasTrait(definition, TraitInterpreted) && !definitionMentioned(interpreterLiterals, definition) {
			t.Errorf("interpreted-capable builtin %q has no interpreter implementation coverage", definition.Name)
		}
		if hasTrait(definition, TraitCompiled) && !definitionMentioned(compiledLiterals, definition) {
			t.Errorf("compiled-capable builtin %q has no compiled implementation coverage", definition.Name)
		}
	}
}

// Random builtins are typed from the table in random.go, so the typechecker
// names none of them. Its coverage is the table lookup itself; the execution
// lanes must each implement every builtin that has its own implementation.
func TestRandomBuiltinsHaveImplementationCoverage(t *testing.T) {
	typecheckSelectors := implementationSelectors(t, filepath.Join("..", "typecheck"))
	if _, ok := typecheckSelectors["builtin.LookupRandom"]; !ok {
		t.Errorf("typechecker does not consult builtin.LookupRandom, so Random builtins have no typechecker coverage")
	}
	typecheckLiterals := implementationStringLiterals(t, filepath.Join("..", "typecheck"))
	interpreterLiterals := implementationStringLiterals(t, filepath.Join("..", "interpret"))
	compiledLiterals := implementationStringLiterals(t, filepath.Join("..", "build"))

	for _, random := range RandomBuiltins() {
		// A v2 symbol such as "Unit" is an ordinary word that the typechecker
		// may use for something else, so only the spellings that are reserved
		// builtin names are checked.
		spellings := []string{random.Name()}
		if random.Legacy {
			spellings = append(spellings, random.Symbol)
		}
		for _, spelling := range spellings {
			if _, ok := typecheckLiterals[spelling]; ok {
				t.Errorf("typechecker names Random builtin %q; it must come from the table in random.go", spelling)
			}
		}
		if !random.HasOwnImplementation() {
			continue
		}
		if _, ok := interpreterLiterals[random.Implementation()]; !ok {
			t.Errorf("Random builtin %q has no interpreter implementation", random.Implementation())
		}
		if _, ok := compiledLiterals[random.Implementation()]; !ok {
			t.Errorf("Random builtin %q has no compiled implementation", random.Implementation())
		}
	}
}

// implementationSelectors collects every qualified identifier, such as
// "builtin.LookupRandom", used by the non-test Go files under root.
func implementationSelectors(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	selectors := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if qualifier, ok := selector.X.(*ast.Ident); ok {
				selectors[qualifier.Name+"."+selector.Sel.Name] = struct{}{}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return selectors
}

func implementationStringLiterals(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	literals := map[string]struct{}{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil {
				literals[value] = struct{}{}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return literals
}

func definitionMentioned(literals map[string]struct{}, definition Definition) bool {
	if _, ok := literals[definition.Name]; ok {
		return true
	}
	for _, alias := range definition.Aliases {
		if _, ok := literals[alias]; ok {
			return true
		}
	}
	return false
}

func hasTrait(definition Definition, trait Trait) bool {
	for _, candidate := range definition.Traits {
		if candidate == trait {
			return true
		}
	}
	return false
}
