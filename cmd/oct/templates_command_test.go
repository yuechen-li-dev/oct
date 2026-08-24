package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/templatecatalog"
)

func TestTemplatesListAndDescribeCanonicalCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "Libraries", "DatabaseTemplates")
	stdout, stderr, err := executeCLIArgs("templates", "list", root, "--json")
	if err != nil {
		t.Fatalf("templates list failed: %v stderr=%q", err, stderr)
	}
	var entries []templatecatalog.Entry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("decode catalog JSON: %v\n%s", err, stdout)
	}
	if len(entries) != 10 {
		t.Fatalf("expected 10 deliberately bounded templates, got %d: %+v", len(entries), entries)
	}
	stdout, stderr, err = executeCLIArgs("templates", "describe", "MaterializedFilter", root)
	if err != nil {
		t.Fatalf("templates describe failed: %v stderr=%q", err, stderr)
	}
	for _, required := range []string{"SpecializationPattern", "[Type]", "[Structure]", "[Require]", "Record", "Access:", "Predicate:", "rebuild/publication", "Database.Patterns.template.oct"} {
		if !strings.Contains(stdout, required) {
			t.Fatalf("description missing %q:\n%s", required, stdout)
		}
	}
}
