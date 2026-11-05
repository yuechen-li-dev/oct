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
	"sort"
	"strings"
)

const source = "internal/prometheus/reactors/PrometheusReduction.concept"
const destination = "internal/prometheus/native/generated"

func main() {
	conceptRoot := flag.String("concept-root", "../Concept", "Concept compiler checkout")
	check := flag.Bool("check", false, "compare regeneration without writing")
	buildPackage := flag.Bool("build-package", false, "build manifest.concept consumer archives without regenerating deployment")
	flag.Parse()
	var err error
	if *buildPackage {
		err = buildNativePackage(*conceptRoot)
	} else {
		err = run(*conceptRoot, *check)
	}
	if err != nil {
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
	identity := exec.Command("git", "rev-parse", "HEAD")
	identity.Dir = root
	revision, err := identity.Output()
	if err != nil {
		return err
	}
	compilerHash, err := compilerInputHash(root)
	if err != nil {
		return err
	}
	files := map[string][]byte{}
	for _, entry := range []struct{ Source, Stem string }{
		{source, "prometheusreduction"},
		{"internal/prometheus/reactors/PrometheusExecution.concept", "prometheusexecution"},
	} {
		compile := exec.Command(compiler, "emit-c", entry.Source, "--source-modules", "--module-root", "internal/prometheus/reactors", "--out-dir", temporary)
		if output, err := compile.CombinedOutput(); err != nil {
			return fmt.Errorf("compile reactor: %w\n%s", err, output)
		}
		inputs := map[string]string{}
		for _, path := range []string{source, entry.Source} {
			input, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(input)
			inputs[path] = hex.EncodeToString(sum[:])
		}
		outputHashes := map[string]string{}
		for _, name := range []string{entry.Stem + ".generated.c", entry.Stem + ".generated.h"} {
			files[name], err = os.ReadFile(filepath.Join(temporary, name))
			if err != nil {
				return err
			}
			sum := sha256.Sum256(files[name])
			outputHashes[name] = hex.EncodeToString(sum[:])
		}
		metadata := struct {
			Source         string            `json:"source"`
			SHA256         string            `json:"source_sha256"`
			Compiler       string            `json:"concept_revision"`
			CompilerInputs string            `json:"concept_source_tree_sha256"`
			Inputs         map[string]string `json:"inputs_sha256"`
			Command        string            `json:"regenerate"`
			Outputs        map[string]string `json:"outputs_sha256"`
		}{entry.Source, inputs[entry.Source], strings.TrimSpace(string(revision)), compilerHash, inputs,
			"go run ./tools/prometheus_concept -concept-root <Concept checkout>", outputHashes}
		encoded, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return err
		}
		files[entry.Stem+".provenance.json"] = append(encoded, '\n')
	}
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

// Hash actual compiler and library inputs, including uncommitted new files.
// HEAD alone cannot identify a compiler while fixing a newly exposed gap.
func compilerInputHash(root string) (string, error) {
	var paths []string
	for _, directory := range []string{"cmd/concept", "internal/concept", "libraries"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == ".native-build" || d.Name() == ".concept") {
				return filepath.SkipDir
			}
			ext := filepath.Ext(path)
			if !d.IsDir() && (ext == ".go" || ext == ".concept" || ext == ".h" || ext == ".c" || ext == ".tmpl") {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				paths = append(paths, filepath.ToSlash(relative))
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	paths = append(paths, "go.mod")
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(body)
		fmt.Fprintf(hash, "%s\x00%x\n", path, sum)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func buildNativePackage(conceptRoot string) error {
	compiler := os.Getenv("CONCEPT_BIN")
	if compiler == "" {
		if installed, err := exec.LookPath("concept"); err == nil {
			compiler = installed
		}
	}
	if compiler == "" {
		temporary, err := os.MkdirTemp("", "prometheus-concept-build-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temporary)
		compiler = filepath.Join(temporary, "concept")
		if runtime.GOOS == "windows" {
			compiler += ".exe"
		}
		root, err := filepath.Abs(conceptRoot)
		if err != nil {
			return err
		}
		build := exec.Command("go", "build", "-o", compiler, "./cmd/concept")
		build.Dir = root
		if output, err := build.CombinedOutput(); err != nil {
			return fmt.Errorf("build Concept driver (or set CONCEPT_BIN): %w\n%s", err, output)
		}
	}
	build := exec.Command(compiler, "package", "build", "internal/prometheus")
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build Prometheus manifest.concept: %w\n%s", err, output)
	}
	fmt.Println("Concept package archives built and input/output hashes validated")
	return nil
}
