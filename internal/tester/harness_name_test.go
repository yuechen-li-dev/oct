package tester

import (
	"strings"
	"testing"
)

func TestHarnessNamesAreBoundedAndDistinct(t *testing.T) {
	prefix := "file:" + strings.Repeat("long-checkout/", 40)
	a := sanitizeHarnessName(compiledHarnessGroup{id: prefix + "first.octest"})
	b := sanitizeHarnessName(compiledHarnessGroup{id: prefix + "second.octest"})
	if len(a) > 81 || len(b) > 81 {
		t.Fatalf("unbounded names: %d, %d", len(a), len(b))
	}
	if a == b {
		t.Fatal("different paths have the same name")
	}
	if a != sanitizeHarnessName(compiledHarnessGroup{id: prefix + "first.octest"}) {
		t.Fatal("name is not deterministic")
	}
}
