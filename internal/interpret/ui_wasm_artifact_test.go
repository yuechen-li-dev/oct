package interpret

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitMachinaUIWasmArtifactAndExecuteBoundary(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for real wasm execution harness")
	}

	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed: %v", err)
	}

	script := strings.TrimSpace(`
const fs = require('fs');

async function run(path) {
  const bytes = fs.readFileSync(path);
  const { instance } = await WebAssembly.instantiate(bytes, {});
  const e = instance.exports;
  const mem = new Uint8Array(e.memory.buffer);

  function readOutput() {
    const ptr = Number(e.ExportMachinaUIBufferPtr());
    const len = Number(e.ExportMachinaUIBufferLen());
    return Buffer.from(mem.slice(ptr, ptr + len)).toString('utf8');
  }

  function writeInput(json) {
    const ptr = Number(e.ExportMachinaUIBufferPtr()) + Number(e.ExportMachinaUIBufferLen()) + 64;
    const bytes = Buffer.from(json, 'utf8');
    mem.set(bytes, ptr);
    return { ptr, len: bytes.length };
  }

  const statusInit = Number(e.ExportMachinaUIInit());
  const statusRender0 = Number(e.ExportMachinaUIRender());
  const firstJSON = readOutput();
  const firstDoc = JSON.parse(firstJSON);

  const increment = writeInput('{"token":"counter.increment","payload":null}');
  const statusDispatchIncrement = Number(e.ExportMachinaUIDispatchEventJSON(increment.ptr, increment.len));
  const statusRender1 = Number(e.ExportMachinaUIRender());
  const secondJSON = readOutput();

  const malformed = writeInput('{"token":');
  const statusMalformed = Number(e.ExportMachinaUIDispatchEventJSON(malformed.ptr, malformed.len));

  const invalidShape = writeInput('{"payload":null}');
  const statusInvalidShape = Number(e.ExportMachinaUIDispatchEventJSON(invalidShape.ptr, invalidShape.len));

  const routeStats = writeInput('{"token":"route.stats","payload":null}');
  const statusDispatchRoute = Number(e.ExportMachinaUIDispatchEventJSON(routeStats.ptr, routeStats.len));
  const statusRenderStats = Number(e.ExportMachinaUIRender());
  const statsJSON = readOutput();

  // Determinism: run same sequence on fresh instance and compare final JSON bytes.
  const { instance: right } = await WebAssembly.instantiate(bytes, {});
  const e2 = right.exports;
  const mem2 = new Uint8Array(e2.memory.buffer);
  function writeInput2(json) {
    const ptr = Number(e2.ExportMachinaUIBufferPtr()) + Number(e2.ExportMachinaUIBufferLen()) + 64;
    const bytes = Buffer.from(json, 'utf8');
    mem2.set(bytes, ptr);
    return { ptr, len: bytes.length };
  }
  function readOutput2() {
    const ptr = Number(e2.ExportMachinaUIBufferPtr());
    const len = Number(e2.ExportMachinaUIBufferLen());
    return Buffer.from(mem2.slice(ptr, ptr + len)).toString('utf8');
  }
  e2.ExportMachinaUIInit();
  e2.ExportMachinaUIRender();
  let ev = writeInput2('{"token":"counter.increment","payload":null}');
  e2.ExportMachinaUIDispatchEventJSON(ev.ptr, ev.len);
  ev = writeInput2('{"token":"route.stats","payload":null}');
  e2.ExportMachinaUIDispatchEventJSON(ev.ptr, ev.len);
  e2.ExportMachinaUIRender();
  const rightJSON = readOutput2();

  const summary = {
    statusInit,
    statusRender0,
    statusDispatchIncrement,
    statusRender1,
    statusMalformed,
    statusInvalidShape,
    statusDispatchRoute,
    statusRenderStats,
    abi: firstDoc.abi,
    firstRootKind: firstDoc.root && firstDoc.root.kind,
    hasIncrementedCount: secondJSON.includes('"count=1"'),
    statsHasRoute: statsJSON.includes('"route=stats"'),
    statsHasHomeEvent: statsJSON.includes('"route.home"'),
    deterministic: rightJSON === statsJSON
  };

  process.stdout.write(JSON.stringify(summary));
}

run(process.argv[1]).catch((err) => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
`)

	cmd := exec.Command("node", "-e", script, artifactPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node wasm harness failed: %v\n%s", err, string(out))
	}

	var summary struct {
		StatusInit              int    `json:"statusInit"`
		StatusRender0           int    `json:"statusRender0"`
		StatusDispatchIncrement int    `json:"statusDispatchIncrement"`
		StatusRender1           int    `json:"statusRender1"`
		StatusMalformed         int    `json:"statusMalformed"`
		StatusInvalidShape      int    `json:"statusInvalidShape"`
		StatusDispatchRoute     int    `json:"statusDispatchRoute"`
		StatusRenderStats       int    `json:"statusRenderStats"`
		ABI                     string `json:"abi"`
		FirstRootKind           string `json:"firstRootKind"`
		HasIncrementedCount     bool   `json:"hasIncrementedCount"`
		StatsHasRoute           bool   `json:"statsHasRoute"`
		StatsHasHomeEvent       bool   `json:"statsHasHomeEvent"`
		Deterministic           bool   `json:"deterministic"`
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("decode harness summary: %v\noutput=%q", err, string(out))
	}

	if summary.StatusInit != int(uiWasmStatusOK) || summary.StatusRender0 != int(uiWasmStatusOK) {
		t.Fatalf("init/render status mismatch: %+v", summary)
	}
	if summary.StatusDispatchIncrement != int(uiWasmStatusOK) || summary.StatusRender1 != int(uiWasmStatusOK) {
		t.Fatalf("dispatch increment status mismatch: %+v", summary)
	}
	if summary.StatusMalformed != int(uiWasmStatusInvalidInput) || summary.StatusInvalidShape != int(uiWasmStatusInvalidInput) {
		t.Fatalf("invalid input status mismatch: %+v", summary)
	}
	if summary.StatusDispatchRoute != int(uiWasmStatusOK) || summary.StatusRenderStats != int(uiWasmStatusOK) {
		t.Fatalf("route/render status mismatch: %+v", summary)
	}
	if summary.ABI != uiirJSONABI {
		t.Fatalf("expected abi %q, got %q", uiirJSONABI, summary.ABI)
	}
	if summary.FirstRootKind != string(uiirNodeColumn) {
		t.Fatalf("expected root kind %q, got %q", uiirNodeColumn, summary.FirstRootKind)
	}
	if !summary.HasIncrementedCount {
		t.Fatalf("expected incremented render payload")
	}
	if !summary.StatsHasRoute || !summary.StatsHasHomeEvent {
		t.Fatalf("expected routed stats payload with route.home token")
	}
	if !summary.Deterministic {
		t.Fatalf("expected deterministic output for identical execution sequence")
	}
}

func TestEmitMachinaUIWasmArtifactRequiresPath(t *testing.T) {
	if err := EmitMachinaUIWasmArtifact(""); err == nil {
		t.Fatalf("expected empty output path to fail")
	}
}

func TestEmitMachinaUIWasmArtifactIsNotFixtureBytes(t *testing.T) {
	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed: %v", err)
	}
	emitted, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read emitted wasm artifact: %v", err)
	}
	fixture, err := decodeMachinaUIWasmArtifactFixture()
	if err != nil {
		t.Fatalf("decode fixture bytes: %v", err)
	}
	if bytes.Equal(emitted, fixture) {
		t.Fatalf("expected production emission to differ from reference fixture bytes")
	}
}

func TestEmitMachinaUIWasmArtifactDoesNotRequireClangInPATH(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for wasm artifact execution check")
	}
	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed without PATH tools: %v", err)
	}
	t.Setenv("PATH", originalPath)
	cmd := exec.Command("node", "-e", `
const fs = require('fs');
const bytes = fs.readFileSync(process.argv[1]);
WebAssembly.instantiate(bytes, {}).then(() => process.exit(0)).catch((err) => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
`, artifactPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed to instantiate emitted wasm: %v\n%s", err, string(out))
	}
}

func TestEmitMachinaUIWasmArtifactDoesNotUseLegacyFixtureDecodeHook(t *testing.T) {
	orig := legacyFixtureDecodeHook
	legacyFixtureDecodeHook = func() ([]byte, error) {
		t.Fatalf("legacy fixture decode hook must not be called in production emission")
		return nil, nil
	}
	t.Cleanup(func() { legacyFixtureDecodeHook = orig })

	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed: %v", err)
	}
}
