package octagon

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "Language", "Data", "Octagon", "valid", "*.octagon"))
	if err != nil {
		t.Fatalf("glob valid fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected valid .octagon fixtures")
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			_, err := Load(path)
			if err != nil {
				t.Fatalf("load valid fixture: %v", err)
			}
		})
	}
}

func TestLoadInvalidFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "Language", "Data", "Octagon", "invalid", "*.octagon"))
	if err != nil {
		t.Fatalf("glob invalid fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected invalid .octagon fixtures")
	}

	expected := map[string]string{
		"package_declaration.octagon":       "expected expression",
		"function_declaration.octagon":      "expected expression",
		"function_call.octagon":             ".octagon does not allow function calls",
		"let_binding.octagon":               "expected expression",
		"if_expression.octagon":             ".octagon does not allow if expressions",
		"arithmetic_expression.octagon":     ".octagon does not allow arithmetic or computed expressions",
		"multiple_top_level_values.octagon": "expected end of file after top-level value",
		"malformed_mixed_exec_data.octagon": "expected end of file after top-level value",
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected invalid fixture to fail")
			}
			want, ok := expected[filepath.Base(path)]
			if !ok {
				t.Fatalf("missing expected error substring for %s", filepath.Base(path))
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected error to contain %q, got %q", want, err.Error())
			}
		})
	}
}

func TestLoadRejectsNonOctagonExtension(t *testing.T) {
	_, err := Load(filepath.Join("..", "..", "README.md"))
	if err == nil || !strings.Contains(err.Error(), ".octagon extension") {
		t.Fatalf("expected extension error, got %v", err)
	}
}
