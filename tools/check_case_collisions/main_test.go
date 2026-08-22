package main

import (
	"reflect"
	"testing"
)

func TestCaseInsensitiveCollisions(t *testing.T) {
	paths := []string{"Examples/a.oct", "examples/A.oct", "docs/readme.md", "Docs/README.md", "unique.go"}
	want := [][]string{
		{"Docs", "docs"},
		{"Docs/README.md", "docs/readme.md"},
		{"Examples", "examples"},
		{"Examples/a.oct", "examples/A.oct"},
	}
	if got := caseInsensitiveCollisions(paths); !reflect.DeepEqual(got, want) {
		t.Fatalf("caseInsensitiveCollisions() = %#v, want %#v", got, want)
	}
}

func TestCaseInsensitiveCollisionsAllowsUniquePaths(t *testing.T) {
	if got := caseInsensitiveCollisions([]string{"Examples/a.oct", "Examples/b.oct"}); len(got) != 0 {
		t.Fatalf("unexpected collisions: %#v", got)
	}
}

func TestCaseInsensitiveCollisionsDetectsDirectoryOnlyCollision(t *testing.T) {
	want := [][]string{{"Examples", "examples"}}
	paths := []string{"Examples/first.oct", "examples/second.oct"}
	if got := caseInsensitiveCollisions(paths); !reflect.DeepEqual(got, want) {
		t.Fatalf("caseInsensitiveCollisions() = %#v, want %#v", got, want)
	}
}
