package wasm

import (
	"bytes"
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

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node WebAssembly runtime is not installed")
	}
	modulePath := filepath.Join(t.TempDir(), "compute.wasm")
	if err := os.WriteFile(modulePath, first, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `const fs=require("fs");const b=fs.readFileSync(process.argv[1]);if(!WebAssembly.validate(b))throw new Error("invalid module");WebAssembly.instantiate(b,{}).then(({instance:{exports:e}})=>{const got=[String(e.Add(20n,22n)),String(e.SumTo(10n)),String(e.ModeCode(1)),String(e.FloatKernel(2)),String(e.BoolKernel(1,0)),String(e.SignedKernel(5n)),String(e.Main())].join(",");if(got!=="42,55,7,4.5,1,-14,104")throw new Error(got);console.log(got)});`
	output, err := exec.Command(node, "-e", script, modulePath).CombinedOutput()
	if err != nil {
		t.Fatalf("Node WebAssembly execution failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "42,55,7,4.5,1,-14,104" {
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
	if strings.TrimSpace(interpreted.String()) != "104" || strings.TrimSpace(string(nativeOutput)) != "104" {
		t.Fatalf("three-lane parity failed: interpreter=%q Go=%q WASM=%q", interpreted.String(), nativeOutput, output)
	}
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
