// Command prometheus_concept regenerates the production Concept reactor seam.
// Ordinary native builds consume the checked C11; regeneration needs Concept.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const source = "internal/prometheus/reactors/PrometheusReduction.concept"
const destination = "internal/prometheus/native/generated"

func main() {
	conceptRoot := flag.String("concept-root", "../Concept", "Concept compiler checkout")
	check := flag.Bool("check", false, "compare regeneration without writing")
	flag.Parse()
	if err := run(*conceptRoot, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(conceptRoot string, check bool) error {
	root, err := filepath.Abs(conceptRoot)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "prometheus-concept-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	compiler := filepath.Join(temporary, "concept")
	if runtime.GOOS == "windows" {
		compiler += ".exe"
	}
	build := exec.Command("go", "build", "-o", compiler, "./cmd/concept")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build Concept: %w\n%s", err, output)
	}
	compile := exec.Command(compiler, "emit-c", source, "--out-dir", temporary)
	if output, err := compile.CombinedOutput(); err != nil {
		return fmt.Errorf("compile reactor: %w\n%s", err, output)
	}
	identity := exec.Command("git", "rev-parse", "HEAD")
	identity.Dir = root
	revision, err := identity.Output()
	if err != nil {
		return err
	}
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(input)
	files := map[string][]byte{}
	outputHashes := map[string]string{}
	for _, name := range []string{"prometheusreduction.generated.c", "prometheusreduction.generated.h"} {
		files[name], err = os.ReadFile(filepath.Join(temporary, name))
		if err != nil {
			return err
		}
		outputSum := sha256.Sum256(files[name])
		outputHashes[name] = hex.EncodeToString(outputSum[:])
	}
	metadata := struct {
		Source   string            `json:"source"`
		SHA256   string            `json:"source_sha256"`
		Compiler string            `json:"concept_revision"`
		Command  string            `json:"regenerate"`
		Outputs  map[string]string `json:"outputs_sha256"`
	}{source, hex.EncodeToString(sum[:]), strings.TrimSpace(string(revision)), "go run ./tools/prometheus_concept -concept-root <Concept checkout>", outputHashes}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	files["prometheusreduction.provenance.json"] = append(encoded, '\n')
	if !check {
		if err := os.MkdirAll(destination, 0755); err != nil {
			return err
		}
	}
	for name, generated := range files {
		path := filepath.Join(destination, name)
		if check {
			current, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(current, generated) {
				return fmt.Errorf("stale Concept artifact %s; regenerate with the recorded compiler", path)
			}
		} else if err := os.WriteFile(path, generated, 0644); err != nil {
			return err
		}
	}
	fmt.Println("Concept reactor artifacts verified")
	return nil
}
