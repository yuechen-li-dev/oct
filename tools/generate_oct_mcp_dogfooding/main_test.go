package main

import (
	"bytes"
	"testing"
)

func TestGenerateIsDeterministic(t *testing.T) {
	root, err := findRepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	first, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("dogfood artifact generation was not byte-identical")
	}
}
