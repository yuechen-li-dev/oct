// Command scalar_loop_recon compares one generated Oct scalar loop with the
// equivalent hand-written Go loop. It is reconnaissance, not a performance
// gate: host load and Go toolchain changes can move the measurements.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/build"
)

const fixture = "testdata/performance/scalar_loop_recon/main.oct"
const baseline = "testdata/performance/scalar_loop_recon/baseline.go"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	repo, err := os.Getwd()
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "oct-scalar-loop-recon-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)

	source, err := os.ReadFile(filepath.Join(repo, fixture))
	if err != nil {
		return err
	}
	octSource := filepath.Join(temporary, "main.oct")
	if err := os.WriteFile(octSource, source, 0o644); err != nil {
		return err
	}

	priorKeep, hadKeep := os.LookupEnv("OCT_KEEP_GEN")
	if err := os.Setenv("OCT_KEEP_GEN", "1"); err != nil {
		return err
	}
	compiled, compileErr := build.Compile(octSource)
	if hadKeep {
		_ = os.Setenv("OCT_KEEP_GEN", priorKeep)
	} else {
		_ = os.Unsetenv("OCT_KEEP_GEN")
	}
	if compileErr != nil {
		return compileErr
	}
	generatedDirectory := filepath.Dir(compiled.GeneratedSourcePath)
	if safelyWithin(filepath.Join(repo, ".octbuild"), generatedDirectory) {
		defer os.RemoveAll(generatedDirectory)
	}

	baselineBinary := filepath.Join(temporary, "baseline")
	if build.HostTarget().GOOS == "windows" {
		baselineBinary += ".exe"
	}
	command := exec.Command("go", "build", "-o", baselineBinary, filepath.Join(repo, baseline))
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build Go baseline: %w: %s", err, strings.TrimSpace(string(output)))
	}

	generatedSource, err := os.ReadFile(compiled.GeneratedSourcePath)
	if err != nil {
		return err
	}
	octSamples, octOutput, err := measure(compiled.ArtifactPath, 7)
	if err != nil {
		return err
	}
	goSamples, goOutput, err := measure(baselineBinary, 7)
	if err != nil {
		return err
	}
	if octOutput != goOutput {
		return fmt.Errorf("observable result mismatch: Oct %q, Go %q", octOutput, goOutput)
	}

	octMedian := median(octSamples)
	goMedian := median(goSamples)
	fmt.Printf("iterations=20000000 samples=7\n")
	fmt.Printf("oct_median=%s go_median=%s ratio=%.2fx result=%s\n", octMedian, goMedian, float64(octMedian)/float64(goMedian), octOutput)
	fmt.Printf("generated_float64_conversions=%d generated_guard_branches=%d generated_program_counter_switches=%d\n",
		bytes.Count(generatedSource, []byte("float64(")),
		bytes.Count(generatedSource, []byte("if !")),
		bytes.Count(generatedSource, []byte("switch __oct")),
	)
	return nil
}

func measure(path string, count int) ([]time.Duration, string, error) {
	for index := 0; index < 2; index++ {
		if _, err := exec.Command(path).CombinedOutput(); err != nil {
			return nil, "", fmt.Errorf("warm %s: %w", path, err)
		}
	}
	samples := make([]time.Duration, 0, count)
	output := ""
	for index := 0; index < count; index++ {
		started := time.Now()
		body, err := exec.Command(path).CombinedOutput()
		elapsed := time.Since(started)
		if err != nil {
			return nil, "", fmt.Errorf("run %s: %w: %s", path, err, strings.TrimSpace(string(body)))
		}
		current := strings.TrimSpace(string(body))
		if output != "" && current != output {
			return nil, "", fmt.Errorf("nondeterministic output from %s: %q then %q", path, output, current)
		}
		output = current
		samples = append(samples, elapsed)
	}
	return samples, output, nil
}

func median(samples []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered[len(ordered)/2]
}

func safelyWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != "." && relative != "" && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
