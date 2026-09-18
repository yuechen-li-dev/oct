package wasm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/run"
)

const wasmComputeExample = "../../Examples/WasmCompute"

func TestCurrentMIRProducesDeterministicExecutableModule(t *testing.T) {
	example := copyWasmComputeExample(t)
	module, _, err := build.LoadMIR(example)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Encode(module)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Encode(module)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same current MIR produced different module bytes")
	}
	optimizedModule, _, err := build.OptimizeMIR(module)
	if err != nil {
		t.Fatal(err)
	}
	optimized, err := Encode(optimizedModule)
	if err != nil {
		t.Fatal(err)
	}
	optimizedAgain, err := Encode(optimizedModule)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(optimized, optimizedAgain) {
		t.Fatal("same optimized MIR produced different module bytes")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node WebAssembly runtime is not installed")
	}
	modulePath := filepath.Join(t.TempDir(), "compute.wasm")
	if err := os.WriteFile(modulePath, first, 0o644); err != nil {
		t.Fatal(err)
	}
	optimizedPath := filepath.Join(t.TempDir(), "compute-optimized.wasm")
	if err := os.WriteFile(optimizedPath, optimized, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `const fs=require("fs");Promise.all(process.argv.slice(1).map(async p=>{const b=fs.readFileSync(p);if(!WebAssembly.validate(b))throw new Error("invalid module");const {instance:{exports:e}}=await WebAssembly.instantiate(b,{});const got=[String(e.Add(20n,22n)),String(e.Max(20n,22n)),String(e.Choose(0)),String(e.Choose(1)),String(e.SumTo(10n)),String(e.ModeCode(1)),String(e.FloatKernel(2)),String(e.BoolKernel(1,0)),String(e.SignedKernel(5n)),String(e.ConstantDemo(0)),String(e.ConstantDemo(1)),String(e.Main())].join(",");if(got!=="42,22,1,2,55,7,4.5,1,-14,7,7,114")throw new Error(got);return got})).then(x=>console.log(x.join("\n")));`
	output, err := exec.Command(node, "-e", script, modulePath, optimizedPath).CombinedOutput()
	if err != nil {
		t.Fatalf("Node WebAssembly execution failed: %v\n%s", err, output)
	}
	wantWasm := "42,22,1,2,55,7,4.5,1,-14,7,7,114\n42,22,1,2,55,7,4.5,1,-14,7,7,114"
	if strings.TrimSpace(string(output)) != wantWasm {
		t.Fatalf("unexpected WebAssembly results: %s", output)
	}

	var interpreted bytes.Buffer
	if err := run.Execute(example, &interpreted); err != nil {
		t.Fatalf("interpreted lane: %v", err)
	}
	native, err := build.Compile(example)
	if err != nil {
		t.Fatalf("Go backend lane: %v", err)
	}
	nativeOutput, err := exec.Command(native.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("Go backend execution: %v\n%s", err, nativeOutput)
	}
	optimizedNative, err := build.CompileOptimized(example)
	if err != nil {
		t.Fatalf("optimized Go backend lane: %v", err)
	}
	optimizedNativeOutput, err := exec.Command(optimizedNative.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("optimized Go backend execution: %v\n%s", err, optimizedNativeOutput)
	}
	if strings.TrimSpace(interpreted.String()) != "114" || strings.TrimSpace(string(nativeOutput)) != "114" || strings.TrimSpace(string(optimizedNativeOutput)) != "114" {
		t.Fatalf("three-lane parity failed: interpreter=%q Go=%q WASM=%q", interpreted.String(), nativeOutput, output)
	}
}

func TestCompilerOptimizationBookMetricsSnapshot(t *testing.T) {
	module, _, err := build.LoadMIR(wasmComputeExample)
	if err != nil {
		t.Fatal(err)
	}
	optimizedModule, _, err := build.OptimizeMIR(module)
	if err != nil {
		t.Fatal(err)
	}
	beforeBytes, err := Encode(module)
	if err != nil {
		t.Fatal(err)
	}
	afterBytes, err := Encode(optimizedModule)
	if err != nil {
		t.Fatal(err)
	}
	beforeFn := findFunction(t, module, "ConstantDemo")
	afterFn := findFunction(t, optimizedModule, "ConstantDemo")
	metrics := fmt.Sprintf("program: ConstantDemo (within WasmCompute module)\nfold-or-branch MIR ops before: %d\nfold-or-branch MIR ops after: %d\nWASM module bytes before: %d\nWASM module bytes after: %d\nWASM code section bytes before: %d\nWASM code section bytes after: %d\n",
		countFoldBranchOps(beforeFn), countFoldBranchOps(afterFn), len(beforeBytes), len(afterBytes), wasmSectionPayloadSize(t, beforeBytes, 10), wasmSectionPayloadSize(t, afterBytes, 10))
	path := filepath.Join("..", "..", "book", "compiler-optimization-by-example", "snapshots", "constant-demo.metrics")
	if os.Getenv("OCT_UPDATE_BOOK_SNAPSHOTS") == "1" {
		if err := os.WriteFile(path, []byte(metrics), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != metrics {
		t.Fatalf("%s is stale\n--- snapshot ---\n%s--- current metrics ---\n%s", path, want, metrics)
	}
}

func findFunction(t *testing.T, module build.MIRModule, name string) build.MIRFunction {
	t.Helper()
	for _, fn := range module.Functions {
		if fn.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return build.MIRFunction{}
}

func countFoldBranchOps(fn build.MIRFunction) int {
	count := 0
	var walk func(build.MIRValue)
	walk = func(value build.MIRValue) {
		switch v := value.(type) {
		case build.MIRUnary:
			count++
			walk(v.Value)
		case build.MIRBinary:
			count++
			walk(v.Left)
			walk(v.Right)
		case build.MIRConvert:
			count++
			walk(v.Value)
		case build.MIRIntrinsicValue:
			count++
			for _, arg := range v.Args {
				walk(arg)
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, stmt := range block.Statements {
			if assign, ok := stmt.(build.MIRAssign); ok {
				walk(assign.Value)
			}
		}
		if branch, ok := block.Terminator.(build.MIRBranch); ok {
			count++
			walk(branch.Cond)
		}
	}
	return count
}

func wasmSectionPayloadSize(t *testing.T, module []byte, wanted byte) int {
	t.Helper()
	for offset := 8; offset < len(module); {
		id := module[offset]
		offset++
		size, width := decodeU32(module[offset:])
		offset += width
		if id == wanted {
			return int(size)
		}
		offset += int(size)
	}
	t.Fatalf("WebAssembly section %d not found", wanted)
	return 0
}

func decodeU32(input []byte) (uint32, int) {
	var value uint32
	for i, b := range input {
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i + 1
		}
	}
	return 0, 0
}

func TestUnsupportedMIRFailsBeforeEmission(t *testing.T) {
	module, _, err := build.LoadMIR("testdata/unsupported_string.oct")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Encode(module)
	if err == nil || !strings.Contains(err.Error(), "type String is unsupported in M0") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEncodeNeedsNoExternalToolchain(t *testing.T) {
	module, _, err := build.LoadMIR(wasmComputeExample)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")
	if _, err := Encode(module); err != nil {
		t.Fatal(err)
	}
}

func copyWasmComputeExample(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "WasmCompute")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"WasmCompute.oct", "manifest.oct"} {
		contents, err := os.ReadFile(filepath.Join(wasmComputeExample, file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
