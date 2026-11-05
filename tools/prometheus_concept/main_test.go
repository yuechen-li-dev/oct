package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Checking deployment integrity needs no compiler checkout. Native builds
// resolve the Concept library dependency separately through manifest.concept.
func TestCheckedDeploymentMatchesConceptSourceAndOutputs(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, stem := range []string{"prometheusreduction", "prometheusexecution"} {
		t.Run(stem, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, destination, stem+".provenance.json"))
			if err != nil {
				t.Fatal(err)
			}
			var provenance struct {
				Source         string            `json:"source"`
				SHA256         string            `json:"source_sha256"`
				Revision       string            `json:"concept_revision"`
				CompilerInputs string            `json:"concept_source_tree_sha256"`
				Inputs         map[string]string `json:"inputs_sha256"`
				Outputs        map[string]string `json:"outputs_sha256"`
			}
			if err := json.Unmarshal(data, &provenance); err != nil {
				t.Fatal(err)
			}
			if provenance.Source == "" || len(provenance.Revision) != 40 || len(provenance.CompilerInputs) != 64 || len(provenance.Outputs) != 2 || len(provenance.Inputs) == 0 {
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
			for path, expected := range provenance.Inputs {
				check(path, expected)
			}
			for _, name := range []string{stem + ".generated.c", stem + ".generated.h"} {
				check(filepath.Join(destination, name), provenance.Outputs[name])
			}
		})
	}
}
