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
