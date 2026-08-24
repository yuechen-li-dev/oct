package templatecatalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRequiresConceptCategoryAndEnforcementClassification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		category  string
		requires  string
		wantError string
	}{
		{name: "unknown category", category: "MissingCategory", requires: "Requires[Type]: Value has exactly T.", wantError: "is not a catalog Concept"},
		{name: "bare requirement", category: "KnownCategory", requires: "Requires: Value has exactly T.", wantError: "must classify Requires"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := "package Catalog\n\nconcept KnownCategory = String {\n    Require(Self == \"known\", \"known category\")\n}\n\n/// A test template.\n/// Category: " + tc.category + "\n/// " + tc.requires + "\n/// Provides: a test value.\ntemplate record Box<T> {Value: T}\n"
			if err := os.WriteFile(filepath.Join(root, "Box.template.oct"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected %q, got %v", tc.wantError, err)
			}
		})
	}

	t.Run("category concept requires Require", func(t *testing.T) {
		root := t.TempDir()
		source := "package Catalog\n\nconcept UnconstrainedCategory = String\n\n/// A test template.\n/// Category: UnconstrainedCategory\n/// Requires[Type]: Value has exactly T.\n/// Provides: a test value.\ntemplate record Box<T> {Value: T}\n"
		if err := os.WriteFile(filepath.Join(root, "Box.template.oct"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(root)
		if err == nil || !strings.Contains(err.Error(), "must refine String with Require") {
			t.Fatalf("expected constrained category diagnostic, got %v", err)
		}
	})
}

func BenchmarkLoadCanonicalCatalog(b *testing.B) {
	root := filepath.Join("..", "..", "Libraries", "DatabaseTemplates")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		entries, err := Load(root)
		if err != nil {
			b.Fatal(err)
		}
		if len(entries) != 10 {
			b.Fatalf("got %d entries", len(entries))
		}
	}
}
