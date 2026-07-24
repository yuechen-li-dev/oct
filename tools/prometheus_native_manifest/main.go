// Command prometheus_native_manifest renders and verifies the checked-in
// platform build fragments from the canonical native manifest.  It deliberately
// uses only the Go standard library so Windows and Linux builders can validate
// source parity before compiling.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type manifest struct {
	Production []string `json:"production_sources"`
	Tests      []string `json:"native_test_sources"`
	Slow       []string `json:"slow_only_test_sources"`
	Mains      struct {
		Normal     string `json:"normal"`
		Slow       string `json:"slow"`
		Benchmarks string `json:"benchmarks"`
	} `json:"test_mains"`
	TestHost              string                 `json:"sdslv_test_host_source"`
	GeneratedHeaders      []string               `json:"generated_headers"`
	GeneratedArtifactSets []generatedArtifactSet `json:"generated_artifact_sets"`
}

type generatedArtifactSet struct {
	ID                            string   `json:"id"`
	Paths                         []string `json:"paths"`
	Generator                     string   `json:"generator"`
	DeclaredInputAuthority        []string `json:"declared_input_authority"`
	RequiredForCleanClone         bool     `json:"required_for_clean_clone"`
	RequiredForBuild              bool     `json:"required_for_build"`
	ReproducibilityCheck          string   `json:"reproducibility_check"`
	SourceControlledIntentionally bool     `json:"source_controlled_intentionally"`
	Packaged                      bool     `json:"packaged"`
	ContentAddressed              bool     `json:"content_addressed"`
	BytesAuthoritative            string   `json:"bytes_authoritative"`
	KnownConsumers                []string `json:"known_consumers"`
	DeleteRegenerateResult        string   `json:"delete_regenerate_result"`
	Status                        string   `json:"status"`
}

func load(root string) (manifest, error) {
	var m manifest
	b, err := os.ReadFile(filepath.Join(root, "internal", "prometheus", "native", "native_manifest.json"))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

func checkUnique(kind string, paths []string) error {
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			return fmt.Errorf("duplicate %s entry: %s", kind, p)
		}
		seen[p] = true
		if !strings.HasSuffix(p, ".c") && !strings.HasSuffix(p, ".cpp") {
			return fmt.Errorf("invalid %s extension: %s", kind, p)
		}
	}
	return nil
}

func validate(root string, m manifest) error {
	if err := checkUnique("production", m.Production); err != nil {
		return err
	}
	if err := checkUnique("test", m.Tests); err != nil {
		return err
	}
	if err := checkUnique("slow test", m.Slow); err != nil {
		return err
	}
	all := append(append(append([]string{}, m.Production...), m.Tests...), m.Slow...)
	for _, main := range []string{m.Mains.Normal, m.Mains.Slow, m.Mains.Benchmarks} {
		if main != "" {
			all = append(all, main)
		}
	}
	for _, p := range all {
		if _, err := os.Stat(filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(p))); err != nil {
			return fmt.Errorf("manifest entry missing: %s", p)
		}
	}
	if m.TestHost != "" {
		if _, err := os.Stat(filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(m.TestHost))); err != nil {
			return fmt.Errorf("manifest test-host source missing: %s", m.TestHost)
		}
	}
	for _, slow := range m.Slow {
		for _, normal := range m.Tests {
			if slow == normal {
				return fmt.Errorf("slow test is also normal: %s", slow)
			}
		}
	}
	if err := validateGeneratedInventory(root, m); err != nil {
		return err
	}
	return nil
}

func validateGeneratedInventory(root string, m manifest) error {
	if len(m.GeneratedHeaders) == 0 {
		return fmt.Errorf("generated_headers must not be empty")
	}
	seenHeaders := map[string]bool{}
	for _, name := range m.GeneratedHeaders {
		if seenHeaders[name] {
			return fmt.Errorf("duplicate generated header entry: %s", name)
		}
		seenHeaders[name] = true
		if filepath.Base(name) != name || !strings.HasSuffix(name, ".h") {
			return fmt.Errorf("generated header must be a native-relative .h path: %s", name)
		}
		if _, err := os.Stat(filepath.Join(root, "internal", "prometheus", "native", filepath.FromSlash(name))); err != nil {
			return fmt.Errorf("generated header missing: %s", name)
		}
	}
	var discovered []string
	nativeRoot := filepath.Join(root, "internal", "prometheus", "native")
	if err := filepath.Walk(nativeRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if filepath.Ext(name) == ".h" && (strings.HasSuffix(name, "spirv.h") || strings.HasSuffix(name, "generated.h")) {
			discovered = append(discovered, name)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("scan native generated headers: %w", err)
	}
	sort.Strings(discovered)
	declared := append([]string(nil), m.GeneratedHeaders...)
	sort.Strings(declared)
	if len(discovered) != len(declared) {
		return fmt.Errorf("generated header inventory count mismatch: declared=%d discovered=%d", len(declared), len(discovered))
	}
	for index := range discovered {
		if discovered[index] != declared[index] {
			return fmt.Errorf("generated header inventory mismatch: declared=%s discovered=%s", declared[index], discovered[index])
		}
	}
	seenPaths := map[string]string{}
	for _, set := range m.GeneratedArtifactSets {
		if set.ID == "" || set.Generator == "" || len(set.DeclaredInputAuthority) == 0 || set.ReproducibilityCheck == "" || set.BytesAuthoritative == "" || set.Status == "" {
			return fmt.Errorf("generated artifact set %q is missing provenance/authority metadata", set.ID)
		}
		for _, path := range set.Paths {
			clean := filepath.ToSlash(filepath.Clean(path))
			if clean != path || filepath.IsAbs(path) || strings.HasPrefix(clean, "../") {
				return fmt.Errorf("generated artifact path is not repository-relative and normalized: %s", path)
			}
			if prior, ok := seenPaths[path]; ok {
				return fmt.Errorf("duplicate generated artifact declaration: %s (%s and %s)", path, prior, set.ID)
			}
			seenPaths[path] = set.ID
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
				return fmt.Errorf("generated artifact path missing: %s", path)
			}
		}
	}
	for _, name := range m.GeneratedHeaders {
		path := filepath.ToSlash(filepath.Join("internal", "prometheus", "native", name))
		if _, ok := seenPaths[path]; !ok {
			return fmt.Errorf("generated header is not covered by generated_artifact_sets: %s", path)
		}
	}
	return nil
}

func qWin(p string) string { return `"%NATIVE_DIR%\` + strings.ReplaceAll(p, "/", `\`) + `"` }
func qSh(p string) string  { return `"$NATIVE_DIR/` + p + `"` }

func windows(m manifest) []byte {
	var b strings.Builder
	b.WriteString("@rem Generated by tools/prometheus_native_manifest; do not edit.\r\n")
	write := func(name string, paths []string) {
		fmt.Fprintf(&b, "set \"%s=", name)
		for i, p := range paths {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(qWin(p))
		}
		b.WriteString("\"\r\n")
	}
	write("PROMETHEUS_COMMON_C_SRCS", m.Production)
	objs := make([]string, 0, len(m.Production))
	for _, p := range m.Production {
		objs = append(objs, `"%OBJ_DIR%\`+strings.TrimSuffix(filepath.Base(p), ".c")+`.obj"`)
	}
	fmt.Fprintf(&b, "set \"PROMETHEUS_COMMON_OBJS=%s\"\r\n", strings.Join(objs, " "))
	write("PROMETHEUS_MARIONETTE_CPP_SRCS", m.Tests)
	write("PROMETHEUS_MARIONETTE_SLOW_ONLY_SRCS", m.Slow)
	fmt.Fprintf(&b, "set \"PROMETHEUS_MARIONETTE_MAIN=%s\"\r\n", qWin(m.Mains.Normal))
	if m.Mains.Slow == "" {
		b.WriteString("set \"PROMETHEUS_MARIONETTE_SLOW_MAIN=\"\r\n")
	} else {
		fmt.Fprintf(&b, "set \"PROMETHEUS_MARIONETTE_SLOW_MAIN=%s\"\r\n", qWin(m.Mains.Slow))
	}
	fmt.Fprintf(&b, "set \"PROMETHEUS_MARIONETTE_BENCH_MAIN=%s\"\r\n", qWin(m.Mains.Benchmarks))
	fmt.Fprintf(&b, "set \"PROMETHEUS_SDSLV_TEST_HOST=%s\"\r\n", qWin(m.TestHost))
	return []byte(b.String())
}

func shell(m manifest) []byte {
	var b strings.Builder
	b.WriteString("# Generated by tools/prometheus_native_manifest; do not edit.\n")
	write := func(name string, paths []string) {
		fmt.Fprintf(&b, "%s=(\n", name)
		for _, p := range paths {
			fmt.Fprintf(&b, "  %s\n", qSh(p))
		}
		b.WriteString(")\n")
	}
	write("PROMETHEUS_COMMON_C", m.Production)
	write("PROMETHEUS_MARIONETTE_CPP", m.Tests)
	write("PROMETHEUS_MARIONETTE_SLOW_ONLY", m.Slow)
	fmt.Fprintf(&b, "PROMETHEUS_MARIONETTE_MAIN=%s\n", qSh(m.Mains.Normal))
	if m.Mains.Slow == "" {
		b.WriteString("PROMETHEUS_MARIONETTE_SLOW_MAIN=\n")
	} else {
		fmt.Fprintf(&b, "PROMETHEUS_MARIONETTE_SLOW_MAIN=%s\n", qSh(m.Mains.Slow))
	}
	fmt.Fprintf(&b, "PROMETHEUS_MARIONETTE_BENCH_MAIN=%s\n", qSh(m.Mains.Benchmarks))
	fmt.Fprintf(&b, "PROMETHEUS_SDSLV_TEST_HOST=%s\n", qSh(m.TestHost))
	return []byte(b.String())
}

func main() {
	write := flag.Bool("write", false, "write generated fragments")
	check := flag.Bool("check", false, "verify generated fragments")
	flag.Parse()
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	m, err := load(root)
	if err != nil {
		panic(err)
	}
	if err := validate(root, m); err != nil {
		panic(err)
	}
	files := []struct {
		path string
		data []byte
	}{
		{filepath.Join(root, "internal", "prometheus", "native", "native_sources_windows.cmd"), windows(m)},
		{filepath.Join(root, "internal", "prometheus", "native", "native_sources_linux.sh"), shell(m)},
	}
	if !*write && !*check {
		*check = true
	}
	for _, f := range files {
		got, readErr := os.ReadFile(f.path)
		if *check && (readErr != nil || !bytes.Equal(got, f.data)) {
			fmt.Fprintf(os.Stderr, "native manifest drift: %s (run: go run ./tools/prometheus_native_manifest -write)\n", f.path)
			os.Exit(1)
		}
		if *write {
			if err := os.WriteFile(f.path, f.data, 0644); err != nil {
				panic(err)
			}
		}
	}
}
