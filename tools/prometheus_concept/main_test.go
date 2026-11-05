package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Clean-clone builds need no sibling checkout, but cannot silently deploy a
// different Concept source or handwritten edits to its generated implementation.
func TestCheckedDeploymentMatchesConceptSourceAndOutputs(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, destination, "prometheusreduction.provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Source   string            `json:"source"`
		SHA256   string            `json:"source_sha256"`
		Revision string            `json:"concept_revision"`
		Outputs  map[string]string `json:"outputs_sha256"`
	}
	if err := json.Unmarshal(data, &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.Source != source || len(provenance.Revision) != 40 || len(provenance.Outputs) != 2 {
		t.Fatal("incomplete Concept deployment provenance")
	}
	check := func(path, expected string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("stale Concept deployment %s; run go run ./tools/prometheus_concept", path)
		}
	}
	check(provenance.Source, provenance.SHA256)
	for _, name := range []string{"prometheusreduction.generated.c", "prometheusreduction.generated.h"} {
		check(filepath.Join(destination, name), provenance.Outputs[name])
	}
}
