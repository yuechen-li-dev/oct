//go:build integration

package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/octagon"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// inspectProgram proves lowering and generated-source structure without
// crossing the native toolchain boundary. Executable tests call Compile
// explicitly and remain the representative end-to-end layer.
func inspectProgram(path string) (MIRModule, string, error) {
	program, err := project.Load(path)
	if err != nil {
		return MIRModule{}, "", err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return MIRModule{}, "", err
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		return MIRModule{}, "", err
	}
	source, err := emitGo(module)
	return module, source, err
}

func TestLowerProgramBuildsMIRShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let x = 2
    if x > 1 {
        return x + 3
    } else {
        return 0
    }
}

`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(mainPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}
	if len(module.Functions[0].Blocks) < 3 {
		t.Fatalf("expected control-flow blocks, got %d", len(module.Functions[0].Blocks))
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "branch") {
		t.Fatalf("expected branch in dump, got:\n%s", text)
	}
}

func TestConceptsAndTrueRequireEraseToConcreteGo(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.oct")
	src := `package Main
concept Length = Float<m>
concept Motion { Distance: Length }
fn main() -> Int {
    let stages = [1, 2, 3]
    Require(Len(stages) == 3, "must retain three stages")
    let motion = Motion { Distance: 2.0m }
    if motion.Distance == 2.0m { return 0 }
    return 1
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, generated, err := inspectProgram(path)
	if err != nil {
		t.Fatalf("inspect concept program: %v", err)
	}
	if !strings.Contains(generated, "type Main_Motion struct") {
		t.Fatalf("generated Go did not contain concrete record struct:\n%s", generated)
	}
	if strings.Contains(generated, "Require") || strings.Contains(generated, "must retain three stages") || strings.Contains(generated, "concept") {
		t.Fatalf("generated Go retained compile-time concept/requirement metadata:\n%s", generated)
	}
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, statement := range block.Statements {
				if call, ok := statement.(MIRCall); ok && call.Callee == "Require" {
					t.Fatal("Require survived into MIR")
				}
			}
		}
	}
}

func TestRefinedConceptStaticProofErasesAndCheckedConstructionIsSingleBoundary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.oct")
	src := `package Main
concept Channel = Int {
    Require(Self >= 0, "channel low")
    Require(Self <= 255, "channel high")
}
fn Source() -> Int { return 128 }
fn Use(value: Channel) -> Int { return value }
fn main() -> Int {
    let known: Channel = 255
    let checked = Channel(Source())!
    return Use(known) + Use(checked) - 383
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, generated, err := inspectProgram(path)
	if err != nil {
		t.Fatalf("inspect refined concept program: %v", err)
	}
	dump := dumpMIR(module)
	if !strings.Contains(generated, "type Main_Channel = int") {
		t.Fatalf("refined representation was not an alias of int:\n%s", generated)
	}
	if strings.Count(dump, "call Main.__oct_refine_Channel") != 1 {
		t.Fatalf("expected exactly one explicit checked boundary in MIR:\n%s", dump)
	}
	mainStart := strings.Index(dump, "fn Main.main")
	constructorStart := strings.Index(dump, "fn Main.__oct_refine_Channel")
	if mainStart < 0 || constructorStart < mainStart {
		t.Fatalf("missing refined functions:\n%s", dump)
	}
	mainMIR := dump[mainStart:constructorStart]
	if strings.Contains(mainMIR, "channel low") || strings.Contains(mainMIR, "channel high") {
		t.Fatalf("static proof or downstream use retained validation:\n%s", mainMIR)
	}
	if strings.Count(generated, "refined concept Channel:") != 2 {
		t.Fatalf("runtime constructor should emit exactly two requirement failures:\n%s", generated)
	}
}

func TestCompileWritesMIRDumpWhenRequested(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    return 7
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if result.MIRDumpPath == "" {
		t.Fatal("expected MIR dump path")
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	if !strings.Contains(string(data), "fn Main.main") {
		t.Fatalf("unexpected MIR dump:\n%s", string(data))
	}
}

func TestCompileAndRunSubsetProgram(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "Main"), 0o755)
	os.Mkdir(filepath.Join(root, "Math"), 0o755)
	mainSrc := `package Main

import Math

fn main() -> Int {
    let arr = [1, 2]
    let arr2 = Append(arr, 3)
    let n = Len(arr2)
    let pair = Math.Make(4, 5)
    if n == 3 {
        return pair.Left + n
    }
    return 0
}

`
	mathSrc := `package Math

record Pair {
    Left: Int
    Right: Int
}

fn Make(a: Int, b: Int) -> Pair {
    return Pair {
        Left: a
        Right: b
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Math", "math.oct"), []byte(mathSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(root)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileForTestLowersBenchmarkFunctionIntoMIRAndRuns(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte("package Main\nfn Main() -> Int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "bench.octest"), []byte("package Main\n[Benchmark]\nfn Bench() -> Void { let x = 1 + 1 Print(x) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join(root, "Main", "zz_compiled_bench_runner.oct")
	if err := os.WriteFile(runnerPath, []byte("package Main\nfn main() -> Int {\n    Bench()\n    return 0\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := CompileForTestWithSelectedFiles(runnerPath, []string{runnerPath, filepath.Join(root, "Main", "bench.octest")})
	if err != nil {
		t.Fatalf("compile for test: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "fn Main.Bench") {
		t.Fatalf("expected benchmark function in MIR dump, got:\n%s", text)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	normalized := strings.ReplaceAll(string(out), "\r\n", "\n")
	if strings.TrimSpace(normalized) != "2\n0" {
		t.Fatalf("expected benchmark print and runner return, got %q", string(out))
	}
}

func TestCompileForTestLowersBenchmarkPrometheusBlockIntoMIRAndRuns(t *testing.T) {
	skipUnlessSlow(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte("package Main\nfn Main() -> Int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	benchSource := "package Main\n[Benchmark]\nfn BenchPrometheus() -> Void {\n    let a = Matrix.fill(2, 2, 3.0)\n    let b = Matrix.fill(2, 2, 5.0)\n    PROMETHEUS {\n        let c = a @ b\n        Print(c)\n    }\n}\n"
	if err := os.WriteFile(filepath.Join(root, "Main", "bench.octest"), []byte(benchSource), 0o644); err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join(root, "Main", "zz_compiled_bench_runner_prometheus.oct")
	if err := os.WriteFile(runnerPath, []byte("package Main\nfn main() -> Int {\n    BenchPrometheus()\n    return 0\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	t.Setenv("OCT_PROMETHEUS_REACTOR", filepath.Join(t.TempDir(), "missing-reactor.so"))
	result, err := CompileForTestWithSelectedFiles(runnerPath, []string{runnerPath, filepath.Join(root, "Main", "bench.octest")})
	if err != nil {
		t.Fatalf("compile for test: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "PrometheusMatMulMM") {
		t.Fatalf("expected PROMETHEUS MIR builtin call, got:\n%s", text)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	normalized := strings.ReplaceAll(string(out), "\r\n", "\n")
	if !strings.Contains(normalized, "backend_requested=prometheus backend_used=cpu status=fallback(prometheus_unavailable)") {
		t.Fatalf("expected explicit fallback status in output, got %q", normalized)
	}
	if !strings.Contains(normalized, "vulkan_env=") {
		t.Fatalf("expected explicit environment classification in output, got %q", normalized)
	}
	if !strings.Contains(normalized, "[[30 30] [30 30]]") {
		t.Fatalf("expected matrix output, got %q", normalized)
	}
}

func TestCompileAndRunCrossPackageFallibleAndEnum(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Cooking"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainSrc := `package Main

import Cooking

fn Main() -> Int ! Error {
    let grams = Cooking.CupsToGrams(2.0, "bread_flour")?
    let flour = Cooking.Flour.Bread
    if flour == Cooking.PreferredFlour() and grams == 240.0 {
        return 1
    }
    return 0
}
`
	cookingSrc := `package Cooking

enum Flour {
    Bread
    Pastry
}

fn CupsToGrams(cups: Float, flour: String) -> Float ! Error {
    if flour == "bread_flour" {
        return cups * 120.0
    }
    if flour == "pastry_flour" {
        return cups * 110.0
    }
    return error("unknown flour")
}

fn PreferredFlour() -> Flour {
    return Flour.Bread
}
`
	manifestSrc := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Main"
        Version: "0.1.0"
        Description: "compiled cross-package test"
        Dependencies: [Dependency { Name: "Cooking" VersionRequirement: "0.1.0" }]
    }
}
`
	cookingManifestSrc := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Cooking"
        Version: "0.1.0"
        Description: "compiled cross-package cooking"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "manifest.oct"), []byte(manifestSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cooking", "cooking.oct"), []byte(cookingSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cooking", "manifest.oct"), []byte(cookingManifestSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(root)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunNamedRecordArraySurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

record Ingredient {
    Name: String
    Grams: Float
}

fn Ingredients() -> Ingredient[] {
    return [
        Ingredient { Name: "Flour" Grams: 500.0 },
        Ingredient { Name: "Salt" Grams: 10.0 }
    ]
}

fn Pick(xs: Ingredient[]) -> Ingredient {
    return xs[1]
}

fn main() -> Int {
    let xs = Ingredients()
    let picked = Pick(xs)
    if Len(xs) == 2 and picked.Name == "Salt" and picked.Grams == 10.0 {
        return 1
    }
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunLoopLoweringParity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn WhileAccumulation() -> Int {
    var i = 0
    var sum = 0
    while i < 5 {
        sum = sum + i
        i = i + 1
    }
    return sum
}

fn ForRangeAccumulation() -> Int {
    var sum = 0
    for i in 1..5 {
        sum = sum + i
    }
    return sum
}

fn ForRangeStepAccumulation() -> Int {
    var sum = 0
    for i in 0..10 step 2 {
        if i < 6 {
            sum = sum + i
        } else {
            sum = sum + (i + 1)
        }
    }
    return sum
}

fn ForRangeEvaluatesBoundsOnce() -> Int {
    var end = 5
    var sum = 0
    for i in 0..end {
        sum = sum + i
        end = end + 100
    }
    return sum
}

fn ForLoopScopeShadowing() -> Int {
    let i = 9
    for i in 0..1 {
        let x = i
        if x != 0 {
            return 0
        }
    }
    return i
}

fn main() -> Int {
    if WhileAccumulation() == 10 and
        ForRangeAccumulation() == 10 and
        ForRangeStepAccumulation() == 22 and
        ForRangeEvaluatesBoundsOnce() == 10 and
        ForLoopScopeShadowing() == 9 {
        return 1
    }
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFailsWhenImportedPackageMissingSymbol(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Cooking"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainSrc := `package Main

import Cooking

fn Main() -> Int {
    return Cooking.DoesNotExist()
}
`
	cookingSrc := `package Cooking

fn CupsToGrams(cups: Float, flour: String) -> Float {
    return cups
}
`
	manifestSrc := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Main"
        Version: "0.1.0"
        Description: "missing symbol test"
        Dependencies: [Dependency { Name: "Cooking" VersionRequirement: "0.1.0" }]
    }
}
`
	cookingManifestSrc := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Cooking"
        Version: "0.1.0"
        Description: "missing symbol test dependency"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "manifest.oct"), []byte(manifestSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cooking", "cooking.oct"), []byte(cookingSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Cooking", "manifest.oct"), []byte(cookingManifestSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Compile(root)
	if err == nil {
		t.Fatal("expected compile failure")
	}
	if !strings.Contains(err.Error(), "package 'Cooking' has no function 'DoesNotExist'") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompileResolvesManifestDependencyFromPackageCache(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainSrc := `package Main

import Cooking

fn Main() -> Int ! Error {
    let grams = Cooking.CupsToGrams(2.0, "bread_flour")?
    if grams == 240.0 {
        return 1
    }
    return 0
}
`
	manifestSrc := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Main"
        Version: "0.1.0"
        Description: "cache dependency test"
        Dependencies: [Dependency { Name: "Cooking" VersionRequirement: "0.1.0" }]
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main", "manifest.oct"), []byte(manifestSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(root, ".pkgcache")
	depDir := filepath.Join(cacheDir, "repos", "dep-cooking")
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	depSrc := `package Cooking

fn CupsToGrams(cups: Float, flour: String) -> Float ! Error {
    if flour == "bread_flour" {
        return cups * 120.0
    }
    return error("unknown flour")
}
`
	depManifest := `package Manifest

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
}

record Dependency {
    Name: String
    VersionRequirement: String
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "Cooking"
        Version: "0.1.0"
        Description: "cached dependency"
        Dependencies: [Dependency { Name: "OctStd" VersionRequirement: "0.1.0" }]
    }
}
`
	if err := os.WriteFile(filepath.Join(depDir, "cooking.oct"), []byte(depSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "manifest.oct"), []byte(depManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(cacheDir, "index.json")
	type packageCacheEntry struct {
		Source    string `json:"source"`
		CacheKey  string `json:"cache_key"`
		Path      string `json:"path"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		FetchedAt string `json:"fetched_at"`
	}
	type packageCacheIndex struct {
		Entries map[string]packageCacheEntry `json:"entries"`
	}
	indexData, err := json.MarshalIndent(packageCacheIndex{
		Entries: map[string]packageCacheEntry{
			"dep-cooking": {
				Source:    "https://example.com/cooking.git",
				CacheKey:  "dep-cooking",
				Path:      depDir,
				Name:      "Cooking",
				Version:   "0.1.0",
				FetchedAt: "2026-01-01T00:00:00Z",
			},
		},
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal cache index: %v", err)
	}
	if err := os.WriteFile(indexPath, indexData, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_PKG_CACHE_DIR", cacheDir)

	result, err := Compile(root)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchParameterSweepAndOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let values = batch [1, 2, 3, 4] as x {
        return x * x
    }
    return values[0] + values[1] + values[2] + values[3]
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "30" {
		t.Fatalf("expected 30, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunLogicalOperatorsShortCircuitIndexExpressions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let xs = [10, 20, 30]
    let j = 0
    if j > 0 and xs[j - 1] > 0 {
        return 1
    }
    if j == 0 or xs[j - 1] > 0 {
        return 2
    }
    return 3
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Fatalf("expected 2, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchDeterministicRepeatedRuns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let input = [2, 4, 6]
    let first = batch input as x {
        return x + 1
    }
    let second = batch input as x {
        return x + 1
    }
    if first[0] == second[0] and first[1] == second[1] and first[2] == second[2] {
        return 1
    }
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchEmptyAndSingleElement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	emptyPath := filepath.Join(root, "empty.octagon")
	if err := os.WriteFile(emptyPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

fn main() -> Int ! Error {
    let source = LoadOctagon<Int[]>(%q)?
    let empty = batch source as x {
        return x + 1
    }
    let one = batch [9] as x {
        return x * 2
    }
    if Len(empty) == 0 and Len(one) == 1 {
        return one[0]
    }
    return 0
}
`, emptyPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "18" {
		t.Fatalf("expected 18, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchFailWholeBatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn MaybeSample(n: Int) -> Int ! Error {
    if n == 3 {
        return error("sample failed")
    }
    return n + 10
}

fn Collect() -> Int[] ! Error {
    let values = [1, 2, 3, 4]
    let results = batch values as item {
        return MaybeSample(item)?
    }
    return results
}

fn main() -> Int {
    match Collect() {
        ok(v) => { return 0 }
        err(e) => { return 42 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Fatalf("expected 42, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFalliblePropagationAndMatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x + 10
    }
    return error("bad input")
}

fn Chain(x: Int) -> Int ! Error {
    let value = Parse(x)?
    return value + 1
}

fn main() -> Int {
    match Chain(5) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "16" {
		t.Fatalf("expected 16, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFallibleMatchErrBranch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    match Parse(0) {
        ok(v) => { return v }
        err(e) => { return 42 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Fatalf("expected 42, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFallibleUnwrap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    let value = Parse(5)!
    return value + 2
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFallibleUnwrapFailureIsFatal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    return Parse(0)!
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected fatal unwrap failure, got success output %q", strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "unwrap failed: bad input") {
		t.Fatalf("expected unwrap failure message, got %q", string(out))
	}
}

func TestCompileAndRunWriteOctagonSuccess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outPath := filepath.Join(root, "compiled-write.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn main() -> Int {
    let payload = Payload {
        Name: "run"
        Count: 3
    }
    WriteOctagon(%q, payload)
    return 0
}
`, outPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if out, err := exec.Command(result.ArtifactPath).CombinedOutput(); err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(string(body)) != "Payload {\n    Name: \"run\"\n    Count: (3)\n}" {
		t.Fatalf("unexpected .octagon body: %q", string(body))
	}
	if _, err := octagon.Load(outPath); err != nil {
		t.Fatalf("expected written .octagon to be loadable, got %v", err)
	}
}

func TestCompileAndRunLoadOctagonSuccessAndFallibleIntegration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inPath := filepath.Join(root, "input.octagon")
	if err := os.WriteFile(inPath, []byte("Payload { Name: \"run\" Count: 7 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn LoadCount(path: String) -> Int ! Error {
    let payload = LoadOctagon<Payload>(path)?
    return payload.Count
}

fn main() -> Int {
    match LoadCount(%q) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`, inPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunOctagonRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifactPath := filepath.Join(root, "roundtrip.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Samples: Int[]
}

fn main() -> Int ! Error {
    let payload = Payload {
        Name: "trial"
        Samples: [1, 2, 3]
    }
    WriteOctagon(%q, payload)
    let loaded = LoadOctagon<Payload>(%q)?
    return loaded.Samples[2]
}
`, artifactPath, artifactPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "3" {
		t.Fatalf("expected 3, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunLoadOctagonFailureReturnsErr(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inPath := filepath.Join(root, "bad.octagon")
	if err := os.WriteFile(inPath, []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn main() -> Int {
    match LoadOctagon<Payload>(%q) {
        ok(v) => { return 0 }
        err(e) => { return 1 }
    }
}
`, inPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected err-branch result 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestMIRDumpShowsExplicitOctagonRuntimeCalls(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "roundtrip.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Count: Int
}

fn main() -> Int ! Error {
    WriteOctagon(%q, Payload { Count: 1 })
    let loaded = LoadOctagon<Payload>(%q)?
    return loaded.Count
}
`, artifactPath, artifactPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, _, err := inspectProgram(mainPath)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "call WriteOctagon(") {
		t.Fatalf("expected WriteOctagon runtime call in MIR, got:\n%s", text)
	}
	if !strings.Contains(text, "call LoadOctagon(") {
		t.Fatalf("expected LoadOctagon runtime call in MIR, got:\n%s", text)
	}
}

func TestMIRDumpShowsExplicitBatchLowering(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let values = batch [1, 2] as x {
        return x + 1
    }
    return values[0]
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, _, err := inspectProgram(mainPath)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "batch_map") {
		t.Fatalf("expected batch_map in MIR, got:\n%s", text)
	}
	if !strings.Contains(text, "__oct_internal_batch_worker_0_main") {
		t.Fatalf("expected lowered batch worker in MIR, got:\n%s", text)
	}
}

func TestCompileAndRunFlowCoreRuntimeBuiltins(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine(input: Int) -> Int {
    state Start {
        if input > 0 {
            goto Track
        }
        suspend
        return 0
    }

    state Track {
        suspend
        return input
    }
}

fn main() -> Int ! Error {
    let f = Machine(4)
    if Active(f) != "" {
        return error("active should be empty before first step")
    }
    if Complete(f) {
        return error("new flow must start incomplete")
    }
    Step(f)
    if Active(f) != "Track" {
        return error("first step should enter Track")
    }
    let h1 = StateHistory(f)
    if h1[0] != "Start" or h1[1] != "Track" {
        return error("history should include Start and Track after goto")
    }
    Step(f)
    if not Complete(f) {
        return error("flow should be complete after second step")
    }
    if Active(f) != "" {
        return error("active should clear after completion")
    }
    Step(f)
    if Active(f) != "" {
        return error("step on completed flow should be no-op")
    }
    let value = Result(f)?
    return value
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "4" {
		t.Fatalf("expected 4, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunComplexBuiltinsAndArithmetic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn ShadowedComponentNames(z: Complex) -> Float {
    let real = 1.0
    let imag = 2.0
    return real + imag + Real(z) + Imag(z)
}

fn main() -> Float {
    let z = Complex(3.0, 4.0)
    let polar = ComplexPolar(Abs(z), Arg(z))
    let conjugate = Conj(z)
    let logged = Ln(Exp(Complex(0.25, 0.5)))
    let rotated = I() * z
    return ShadowedComponentNames(z) + Abs(z) + Real(conjugate) + Imag(conjugate) + Real(polar) + Imag(polar) + Real(logged) + Imag(logged) + Real(rotated) + Imag(rotated)
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "20.75" {
		t.Fatalf("expected 20.75, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFloatBuiltinConvertsInt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Float {
    let count = 3
    return Float(count) / 2.0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1.5" {
		t.Fatalf("expected 1.5, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileKeepsIntArgumentAfterFloatUse(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn UseSteps(n: Int) -> Int {
    return n + 1
}

fn SolveLike(steps: Int) -> Float {
    let dt = 1.0 / Float(steps)
    let count = UseSteps(steps)
    return dt + Float(count)
}

fn Deriv(t: Float, y: Float) -> Float {
    return t + y
}

fn CallbackStep(f: fn(Float, Float) -> Float, steps: Int) -> Float {
    let dt = 1.0 / Float(steps)
    return f(dt, 1.0) + Float(steps)
}

fn SolveWithCallback(steps: Int) -> Float {
    let dt = 0.5
    let horizon = dt * steps
    return CallbackStep(Deriv, steps) + horizon
}

fn main() -> Float {
    return SolveLike(4) + SolveWithCallback(4)
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "12.5" {
		t.Fatalf("expected 12.5, got %q", strings.TrimSpace(string(out)))
	}
	dump, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(dump)
	if strings.Contains(text, "call Main.UseSteps(float64(") || strings.Contains(text, "call Main.CallbackStep(fn_Main_Deriv, float64(") {
		t.Fatalf("expected Int arguments to remain binding-local Ints, got:\n%s", text)
	}
}

func TestCompileCoercesIntArrayLiteralsInFloatArrayContexts(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

record Samples {
    Values: Float[]
}

fn Sum(values: Float[]) -> Float {
    return values[0] + values[1] + values[2]
}

fn Returned() -> Float[] {
    return [1, 2, 3]
}

fn AnnotatedLocal() -> Float[] {
    let values: Float[] = [1, 2, 3]
    return values
}

fn RecordValues() -> Samples {
    return Samples { Values: [1, 2, 3] }
}

fn main() -> Float {
    let fromArg = Sum([1, 2, 3])
    let fromReturn = Returned()
    let fromLocal = AnnotatedLocal()
    let fromRecord = RecordValues()
    return fromArg + fromReturn[0] + fromLocal[1] + fromRecord.Values[2]
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "12" {
		t.Fatalf("expected 12, got %q", strings.TrimSpace(string(out)))
	}
	dump, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(dump)
	if strings.Contains(text, "call Main.Sum(_t0)") && strings.Contains(text, "_t0 = [1, 2, 3]") {
		t.Fatalf("expected Float[] argument literal to be lowered with Float element context, got:\n%s", text)
	}
}

func TestCompileUsesTemporaryForDiscardForLoopVariable(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Repeat(value: Float, count: Int) -> Float[] {
    var out = [value]
    for _ in 1 .. count {
        out = Append(out, value)
    }
    return out
}

fn main() -> Int {
    return Len(Repeat(2.0, 4))
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, text, err := inspectProgram(mainPath)
	if err != nil {
		t.Fatalf("inspect generated source: %v", err)
	}
	for _, bad := range []string{"if (_ <", "_ = (_ +"} {
		if strings.Contains(text, bad) {
			t.Fatalf("generated Go used blank identifier as a loop value (%q):\n%s", bad, text)
		}
	}
}

func TestCompileFlowResultBeforeCompletionFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow OneSuspend() -> Int {
    state Start {
        suspend
        return 7
    }
}

fn main() -> Int {
    let f = OneSuspend()
    match Result(f) {
        ok(v) => { return 0 }
        err(e) => {
            return 1
        }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFlowRememberResumeAndResumeTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow RememberThenSuspend() -> Int {
    state Patrol {
        goto Guard
    }

    state Guard {
        suspend
        remember
        goto Investigate
    }

    state Investigate {
        suspend
        resume
    }
}

flow OverwritePath() -> Int {
    state Start {
        remember
        goto Mid
    }

    state Mid {
        suspend
        remember
        goto End
    }

    state End {
        suspend
        resume
    }
}

fn main() -> Int ! Error {
    let a = RememberThenSuspend()
    Step(a)
    if Active(a) != "Guard" {
        return error("first step should pause in Guard")
    }
    if ResumeTarget(a) != "" {
        return error("resume slot should start empty")
    }
    Step(a)
    if Active(a) != "Investigate" {
        return error("second step should divert to Investigate")
    }
    if ResumeTarget(a) != "Guard" {
        return error("remember should capture Guard")
    }
    Step(a)
    if Active(a) != "Guard" {
        return error("resume should return to Guard")
    }
    if ResumeTarget(a) != "" {
        return error("successful resume should clear slot")
    }
    let h1 = StateHistory(a)
    if h1[0] != "Patrol" or h1[1] != "Guard" or h1[2] != "Investigate" or h1[3] != "Guard" {
        return error("history should show deterministic diversion and resume")
    }

    let b = OverwritePath()
    Step(b)
    if Active(b) != "Mid" {
        return error("first overwrite step should pause in Mid")
    }
    Step(b)
    if Active(b) != "End" {
        return error("second step should be in End")
    }
    if ResumeTarget(b) != "Mid" {
        return error("second remember should overwrite Start target")
    }
    Step(b)
    if Active(b) != "Mid" {
        return error("resume should return to overwritten Mid target")
    }
    if ResumeTarget(b) != "" {
        return error("overwrite flow should clear slot after resume")
    }
    let h2 = StateHistory(b)
    if h2[0] != "Start" or h2[1] != "Mid" or h2[2] != "End" or h2[3] != "Mid" {
        return error("overwrite flow history should remain deterministic")
    }

    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "0" {
		t.Fatalf("expected 0, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFlowResumeEmptySlotFailsDeterministically(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Broken() -> Int {
    state Start {
        resume
    }
}

fn main() -> Int {
    let f = Broken()
    Step(f)
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected runtime failure, got success output %q", strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "runtime error: resume called with empty resume slot") {
		t.Fatalf("expected deterministic empty-slot resume failure, got %q", string(out))
	}
}

func TestCompileFlowDecisionDoesNotUseSpecialCaseShimPath(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine(flag: Bool) -> Int {
    state Start {
        when {
            case flag -> return 1
            else -> return 0
        }
    }
}

fn main() -> Int {
    let f = Machine(true)
    Step(f)
    match Result(f) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, _, err := inspectProgram(mainPath)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "when { case flag -> return 1; else -> return 0 }") {
		t.Fatalf("expected lowered ordered when in MIR dump, got:\n%s", text)
	}
	if strings.Contains(text, "shim") {
		t.Fatalf("MIR dump should not reference shim paths, got:\n%s", text)
	}
}

func TestCompileFlowBoardAndWhenActionBlock(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

record Input {
    HazardActive: Bool
    Ack: Bool
}

flow Machine(input: Input) -> Int {
    board {
        FaultLatched: Bool
        Count: Int
    }

    state Start {
        when {
            case input.HazardActive -> {
                remember
                board.FaultLatched = true
                board.Count = board.Count + 1
                goto Hold
            }
            case input.Ack -> {
                board.FaultLatched = false
                return 7
            }
            else -> {
                suspend
            }
        }
    }

    state Hold {
        return board.Count
    }
}

fn main() -> Int {
    let f = Machine(Input { HazardActive: true Ack: false })
    Step(f)
    match Result(f) {
        ok(v) => { return v }
        err(e) => { return -1 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "board.Count = (board.Count + 1)") {
		t.Fatalf("expected board field reads/writes in MIR dump, got:\n%s", text)
	}
	if !strings.Contains(text, "case input.HazardActive -> { remember; board.FaultLatched = true; board.Count = (board.Count + 1); goto Hold }") {
		t.Fatalf("expected when action block in MIR dump, got:\n%s", text)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFlowRememberResumeMIRDump(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine() -> Int {
    state A {
        remember
        goto B
    }

    state B {
        resume
    }
}

fn main() -> String {
    let f = Machine()
    Step(f)
    return ResumeTarget(f)
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	module, _, err := inspectProgram(mainPath)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "remember") {
		t.Fatalf("expected remember in MIR dump, got:\n%s", text)
	}
	if !strings.Contains(text, "resume") {
		t.Fatalf("expected resume in MIR dump, got:\n%s", text)
	}
	if !strings.Contains(text, "call ResumeTarget") {
		t.Fatalf("expected ResumeTarget lowering in MIR dump, got:\n%s", text)
	}
}

func TestCompileFlowUtilityWhenUsesMIRAndRuntimeState(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine(high: Bool, mid: Bool) -> Int {
    state Start {
        return when policy {
            hysteresis: 8
            min_commit: 2
        } {
            case 1 when high score 100
            case 2 when mid score 95
            else 3
        }
    }
}

fn main() -> Int {
    let f = Machine(true, true)
    Step(f)
    match Result(f) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "utility_when[site=") {
		t.Fatalf("expected lowered utility when in MIR dump, got:\n%s", text)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunStandaloneUtilityWhenParity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn ChooseMode(temp: Int, pressure: Int, fault: Bool) -> Int {
    return when utility {
        case 900 when fault score 100
        case 200 when temp > 70 score temp
        case 100 when pressure > 50 score pressure
        case 300 when temp > 40 and pressure > 40 score temp + pressure
        else 0
    }
}

fn TieStable() -> Int {
    return when utility {
        case 10 when true score 5
        case 20 when true score 5
        else 0
    }
}

fn Main() -> Int {
    let basic = ChooseMode(80, 20, false)
    let ordered = when utility {
        case 1 when true score 7
        case 2 when true score 9
        case 3 when true score 9
        else 0
    }
    let realistic = ChooseMode(45, 60, false)
    let fallback = ChooseMode(0, 0, false)
    let tie = TieStable()
    return basic + ordered + realistic + fallback + tie
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(mainPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}

	interpValue, err := interpret.ExecuteMain(program, nil)
	if err != nil {
		t.Fatalf("interpreter run: %v", err)
	}
	want := strings.TrimSpace(interpValue.String())
	if want != "512" {
		t.Fatalf("expected interpreter baseline 512, got %q", want)
	}

	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	if got != want {
		t.Fatalf("compiled/interpreter mismatch: compiled=%q interpreter=%q", got, want)
	}
}

func TestRecordTableDynamicLengthValidationParity(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

record table Measurements { Stage: String Latency: Float }

fn Build(stages: String[], latencies: Float[]) -> Measurements {
    return Measurements { Stage: stages Latency: latencies }
}

fn Main() -> Int {
    return Len(Build(["A", "B"], [1.0]))
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(mainPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	if _, err := interpret.ExecuteMain(program, nil); err == nil || !strings.Contains(err.Error(), "OCT-RTBL003") {
		t.Fatalf("expected interpreted OCT-RTBL003, got %v", err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, runErr := exec.Command(result.ArtifactPath).CombinedOutput()
	if runErr == nil || !strings.Contains(string(out), "OCT-RTBL003") {
		t.Fatalf("expected compiled OCT-RTBL003, got err=%v output=%s", runErr, out)
	}
}

func TestCompileModeUnsupportedSurfaceDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name: "range step zero still rejected by typechecker",
			source: `package Main

fn main() -> Int {
    for i in 0..10 step 0 {
        return i
    }
    return 0
}
`,
			wantErr: "range step must be positive, got 0",
		},
		{
			name: "plot builtin",
			source: `package Main

fn main() -> Int {
    return PlotLine([0.0], [1.0], "out.png")
}
`,
			wantErr: "compiled mode does not yet support builtin PlotLine",
		},
		{
			name: "xlsx builtin",
			source: `package Main

fn main() -> Int {
    let wb = XlsxCreateWorkbook()
    return wb
}
`,
			wantErr: "compiled mode does not yet support builtin XlsxCreateWorkbook",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mainPath := filepath.Join(root, "main.oct")
			if err := os.WriteFile(mainPath, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, err := inspectProgram(mainPath)
			if err == nil {
				t.Fatalf("expected compile failure")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestCompileAndRunVectorsMatricesM93(t *testing.T) {
	skipUnlessSlow(t)
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "vector literal compiled",
			source: `package Main

fn Main() -> Vector<Int> {
    return vector[1, 2, 3]
}
`,
			want: "[1 2 3]",
		},
		{
			name: "matrix literal compiled",
			source: `package Main

fn Main() -> Matrix<Int> {
    return matrix[[1, 2] [3, 4]]
}
`,
			want: "[[1 2] [3 4]]",
		},
		{
			name: "matrix at vector compiled",
			source: `package Main

fn Main() -> Vector<Int> {
    let a = matrix[[1, 2] [3, 4]]
    let x = vector[10, 20]
    return a @ x
}
`,
			want: "[50 110]",
		},
		{
			name: "matrix at matrix compiled",
			source: `package Main

fn Main() -> Matrix<Int> {
    let a = matrix[[1, 2] [3, 4]]
    let b = matrix[[5, 6] [7, 8]]
    return a @ b
}
`,
			want: "[[19 22] [43 50]]",
		},
		{
			name: "vector at matrix compiled",
			source: `package Main

fn Main() -> Vector<Int> {
    let x = vector[10, 20]
    let a = matrix[[1, 2] [3, 4]]
    return x @ a
}
`,
			want: "[70 100]",
		},
		{
			name: "vector at vector compiled",
			source: `package Main

fn Main() -> Int {
    let a = vector[1, 2, 3]
    let b = vector[10, 20, 30]
    return a @ b
}
`,
			want: "140",
		},
		{
			name: "matrix tabulate compiled",
			source: `package Main

fn Elem(r: Int, c: Int) -> Int {
    return r * 10 + c
}

fn Main() -> Matrix<Int> {
    return Matrix.tabulate(2, 3, Elem)
}
`,
			want: "[[0 1 2] [10 11 12]]",
		},
		{
			name: "matrix shape and index access compiled",
			source: `package Main

fn Main() -> Int {
    let m = Matrix.fill(2, 3, 7)
    return m.rows + m.cols + m[1, 2]
}
`,
			want: "12",
		},
		{
			name: "matrix index assignment compiled",
			source: `package Main

fn Main() -> Matrix<Int> {
    var out = matrix[[1, 2] [3, 4]]
    out[0, 1] = 9
    out[1, 0] = 8
    return out
}
`,
			want: "[[1 9] [8 4]]",
		},
		{
			name: "dimensioned matrix at vector compiled",
			source: `package Main

fn Main() -> Vector<Float<m>> {
    let k = matrix[[1.0m/s, 0.0m/s] [0.0m/s, 2.0m/s]]
    let x = vector[3.0s, 4.0s]
    return k @ x
}
`,
			want: "[3 8]",
		},
		{
			name: "trace builtin compiled",
			source: `package Main

fn Main() -> Int {
    let a = matrix[[1, 2] [3, 4]]
    return Trace(a)
}
`,
			want: "5",
		},
		{
			name: "tensor differential helpers compiled",
			source: `package Main

fn Main() -> Int {
    let u = vector[2, 4]
    let eps = SymGrad(u)
    let gradU = Grad(u)
    let divU = Div(u)
    return Trace(eps) + gradU[1, 1] + divU
}
`,
			want: "16",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mainPath := filepath.Join(root, "main.oct")
			if err := os.WriteFile(mainPath, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := Compile(mainPath)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, err := exec.Command(result.ArtifactPath).CombinedOutput()
			if err != nil {
				t.Fatalf("run artifact: %v (%s)", err, string(out))
			}
			if strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, strings.TrimSpace(string(out)))
			}
		})
	}
}

func TestCompileRunParityVectorsMatricesM93(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Main() -> Matrix<Int> {
    let a = matrix[[2, 1] [0, 3]]
    let b = matrix[[1, 4] [2, 5]]
    return a @ b
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(mainPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	interpValue, err := interpret.ExecuteMain(program, nil)
	if err != nil {
		t.Fatalf("interpreter run: %v", err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	got := strings.TrimSpace(string(out))
	want := strings.TrimSpace(interpValue.String())
	want = strings.TrimPrefix(want, "matrix")
	want = strings.TrimPrefix(want, "vector")
	want = strings.ReplaceAll(want, ",", "")
	if got != want {
		t.Fatalf("compiled/interpreter mismatch: compiled=%q interpreter=%q", got, want)
	}
}

func TestCompileVectorsMatricesBoundaryErrorsM93(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name: "no implicit array to vector coercion",
			source: `package Main

fn Main() -> Vector<Int> {
    return [1, 2, 3]
}
`,
			wantErr: "function expects Vector<Int>, but return is Int[]",
		},
		{
			name: "no implicit nested array to matrix coercion",
			source: `package Main

fn Main() -> Matrix<Int> {
    return [[1, 2], [3, 4]]
}
`,
			wantErr: "function expects Matrix<Int>, but return is Int[][]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mainPath := filepath.Join(root, "main.oct")
			if err := os.WriteFile(mainPath, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, err := inspectProgram(mainPath)
			if err == nil {
				t.Fatal("expected compile failure")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestCompileAndRunIfExpressionConditionSwitchSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn tier(x: Int) -> String {
    return if x > 10 {
        "high"
    } else {
        if x > 5 {
            "mid"
        } else {
            "low"
        }
    }
}

fn main() -> Int {
    if tier(11) == "high" and tier(7) == "mid" and tier(1) == "low" {
        return 1
    }
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileForTestLowersPrometheusMatMulBuiltinOutsidePrometheusBlock(t *testing.T) {
	skipUnlessSlow(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package Main\nfn main() -> Int {\n    let a = Matrix.fill(2, 2, 3.0)\n    let b = Matrix.fill(2, 2, 5.0)\n    let c = PrometheusMatMul(a, b)\n    Print(c)\n    return 0\n}\n"
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	t.Setenv("OCT_PROMETHEUS_REACTOR", filepath.Join(t.TempDir(), "missing-reactor.so"))
	result, err := CompileForTest(filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("compile for test: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "PrometheusMatMulMM") {
		t.Fatalf("expected PrometheusMatMul builtin to lower into PrometheusMatMulMM, got:\n%s", text)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	normalized := strings.ReplaceAll(string(out), "\r\n", "\n")
	if !strings.Contains(normalized, "backend_requested=prometheus backend_used=cpu status=fallback(prometheus_unavailable)") {
		t.Fatalf("expected explicit fallback status in output, got %q", normalized)
	}
	if !strings.Contains(normalized, "[[30 30] [30 30]]") {
		t.Fatalf("expected matrix output, got %q", normalized)
	}
}

func TestCompileAndRunNumericShapeLoweringChimera(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Main"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One compiled fixture preserves the M24 numeric/array lowering regressions and
	// the matrix/scalar elementwise regression while paying the Go build cost once.
	// Failure codes are intentionally stable so a chimera failure still identifies
	// the original regression category.
	src := `package Main

record Samples {
    Values: Float[]
}

record FloatBox { Value: Float }
record BoolBox { Value: Bool }

fn Accept(values: Float[]) -> Float {
    return values[0] + values[1] + values[2]
}

fn MakeFloat() -> FloatBox { return FloatBox { Value: 1.25 } }
fn MakeBool() -> BoolBox { return BoolBox { Value: true } }

fn MixedIntFloatArithmeticCompilesAsFloat() -> Bool {
    let n = 4
    let sum = 10.0
    return sum / n + n * 0.5 - 1 == 3.5
}

fn TypedFloatArrayLiteralCoercesIntegerElements() -> Bool {
    return Accept([1, 2, 3]) == 6.0
}

fn RecordFloatArrayFieldCoercesIntegerLiteralElements() -> Bool {
    let samples = Samples { Values: [1, 2, 3] }
    return samples.Values[2] == 3.0
}

fn LoopIndexKeepsIntShapeWhenLaterFloatLocalShadowsName() -> Bool {
    let xs = [1.0, 2.0, 3.0]
    var total = 0.0
    for c in 0..Len(xs) {
        total = total + xs[c]
    }
    let c = 1.5
    return total + c == 7.5
}

fn SameLogicalNameCanHoldDistinctRecordResultShapes() -> Bool {
    let draw = MakeFloat()
    let first = draw.Value
    let draw = MakeBool()
    if draw.Value {
        return first == 1.25
    }
    return false
}

fn MatrixMatches(actual: Matrix<Float>, expected: Matrix<Float>) -> Bool {
    if actual.rows != expected.rows {
        return false
    }
    if actual.cols != expected.cols {
        return false
    }
    for r in 0..actual.rows {
        for c in 0..actual.cols {
            if actual[r, c] != expected[r, c] {
                return false
            }
        }
    }
    return true
}

fn StressLike(strain: Matrix<Float>, lambda: Float, mu: Float) -> Matrix<Float> {
    let vol = strain[0, 0] + strain[1, 1]
    let identity = matrix[[1.0, 0.0] [0.0, 1.0]]
    return (lambda * vol) * identity + (2.0 * mu) * strain
}

fn MatrixScalarElementwiseRegressionWorks() -> Bool {
    let base = matrix[[1.0, 2.0] [3.0, 4.0]]
    let scalarLeft = 2.0 * base
    if not MatrixMatches(scalarLeft, matrix[[2.0, 4.0] [6.0, 8.0]]) {
        return false
    }

    let scalarRight = base * 2.0
    if not MatrixMatches(scalarRight, matrix[[2.0, 4.0] [6.0, 8.0]]) {
        return false
    }

    let a = matrix[[1.0, 2.0] [3.0, 4.0]]
    let b = matrix[[10.0, 20.0] [30.0, 40.0]]
    let added = a + b
    if not MatrixMatches(added, matrix[[11.0, 22.0] [33.0, 44.0]]) {
        return false
    }

    let subtracted = b - a
    if not MatrixMatches(subtracted, matrix[[9.0, 18.0] [27.0, 36.0]]) {
        return false
    }

    let stress = StressLike(matrix[[1.0, 0.25] [0.25, 2.0]], 3.0, 4.0)
    return MatrixMatches(stress, matrix[[17.0, 2.0] [2.0, 25.0]])
}

fn Main() -> Int {
    if not MixedIntFloatArithmeticCompilesAsFloat() {
        return 1
    }
    if not TypedFloatArrayLiteralCoercesIntegerElements() {
        return 2
    }
    if not RecordFloatArrayFieldCoercesIntegerLiteralElements() {
        return 3
    }
    if not LoopIndexKeepsIntShapeWhenLaterFloatLocalShadowsName() {
        return 4
    }
    if not SameLogicalNameCanHoldDistinctRecordResultShapes() {
        return 5
    }
    if not MatrixScalarElementwiseRegressionWorks() {
        return 6
    }
    return 0
}
`
	mainPath := filepath.Join(root, "Main", "main.oct")
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(root)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	failureLabels := map[string]string{
		"1": "mixed int/float arithmetic compiles as float",
		"2": "typed float array literal coerces integer elements",
		"3": "record float array field coerces integer literal elements",
		"4": "loop index keeps int shape when later float local shadows name",
		"5": "same logical name can hold distinct record result shapes",
		"6": "matrix/scalar elementwise regression works",
	}
	got := strings.TrimSpace(string(out))
	if got != "0" {
		if label, ok := failureLabels[got]; ok {
			t.Fatalf("chimera regression failed at code %s (%s)", got, label)
		}
		t.Fatalf("expected 0, got %q", got)
	}
}

func TestCompileFirstClassRangeExpressionsOutsideForLoop(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Closed() -> Range { return 1..3 }
fn OpenEnd() -> Range { return 1.. }
fn OpenStart() -> Range { return ..3 }
fn AllOpen() -> Range { return .. }
fn Stepped() -> Range { return 1..10 step 2 }

fn main() -> Range {
    return Stepped()
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(mainPath); err != nil {
		t.Fatalf("compile: %v", err)
	}
}
