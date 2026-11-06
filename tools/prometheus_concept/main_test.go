package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Runtime provenance keeps its integrity contract even though its files no
// longer belong in Git. Exercise stale and missing products in isolation.
func TestDeploymentProvenanceRejectsStaleInputsAndOutputs(t *testing.T) {
	for _, corrupt := range []string{"input", "c", "header", "exports", "missing exports", "missing"} {
		t.Run(corrupt, func(t *testing.T) {
			root, output := t.TempDir(), t.TempDir()
			sum := func(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			for _, stem := range []string{"prometheusreduction", "prometheusexecution", "prometheusrmsnorm", "prometheusmodelblock"} {
				input := []byte("opaque source hash fixture")
				inputPath := filepath.Join("inputs", stem+".concept")
				write(filepath.Join(root, inputPath), input)
				outputs := map[string]string{}
				names := []string{stem + ".generated.c", stem + ".generated.h"}
				if stem == "prometheusmodelblock" {
					names = append(names, stem+".exports.h")
				}
				for _, name := range names {
					data := []byte("opaque generated " + name + " fixture")
					write(filepath.Join(output, name), data)
					outputs[name] = sum(data)
				}
				metadata := map[string]any{
					"compiler_binary_sha256": strings.Repeat("0", 64),
					"inputs_sha256":          map[string]string{filepath.ToSlash(inputPath): sum(input)},
					"outputs_sha256":         outputs,
				}
				data, err := json.Marshal(metadata)
				if err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(output, stem+".provenance.json"), data)
			}
			if err := verifyDeployment(root, output); err != nil {
				t.Fatal(err)
			}
			switch corrupt {
			case "input":
				write(filepath.Join(root, "inputs", "prometheusexecution.concept"), []byte("changed"))
			case "c":
				write(filepath.Join(output, "prometheusexecution.generated.c"), []byte("changed"))
			case "header":
				write(filepath.Join(output, "prometheusexecution.generated.h"), []byte("changed"))
			case "exports":
				write(filepath.Join(output, "prometheusmodelblock.exports.h"), []byte("changed"))
			case "missing exports":
				if err := os.Remove(filepath.Join(output, "prometheusmodelblock.exports.h")); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(filepath.Join(output, "prometheusexecution.generated.c")); err != nil {
					t.Fatal(err)
				}
			}
			if err := verifyDeployment(root, output); err == nil {
				t.Fatal("stale or missing deployment accepted")
			}
		})
	}
}
