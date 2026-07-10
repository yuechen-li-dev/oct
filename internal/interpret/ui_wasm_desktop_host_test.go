//go:build toolchain

package interpret

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachinaUIDesktopHostReusesWasmAndCanonicalABI(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for desktop host harness")
	}

	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed: %v", err)
	}

	hostPath := filepath.Join("..", "..", "tools", "machina-ui-host", "host.js")
	desktopPath := filepath.Join("..", "..", "tools", "machina-ui-desktop", "desktop_host.js")
	script := strings.TrimSpace(`
const fs = require('fs');
const hostModule = require(process.argv[2]);
const desktopModule = require(process.argv[3]);

class FakeElement {
  constructor(tagName) {
    this.tagName = tagName;
    this.className = '';
    this.textContent = '';
    this.disabled = false;
    this.dataset = {};
    this.style = {};
    this.children = [];
    this.listeners = {};
    this.parent = null;
  }

  appendChild(child) {
    this.children.push(child);
    child.parent = this;
    return child;
  }

  removeChild(child) {
    const idx = this.children.indexOf(child);
    if (idx >= 0) {
      this.children.splice(idx, 1);
      child.parent = null;
    }
  }

  get firstChild() {
    return this.children.length > 0 ? this.children[0] : null;
  }

  addEventListener(event, handler) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(handler);
  }

  click() {
    if (this.disabled) {
      return;
    }
    for (const handler of this.listeners.click || []) {
      handler({ type: 'click', target: this });
    }
  }
}

class FakeDocument {
  createElement(tagName) {
    return new FakeElement(tagName);
  }
}

function walk(root, fn) {
  fn(root);
  for (const child of root.children || []) {
    walk(child, fn);
  }
}

function collectText(root) {
  const out = [];
  walk(root, (node) => {
    if (node.textContent && node.textContent.length > 0) {
      out.push(node.textContent);
    }
  });
  return out;
}

function findButtonByToken(root, token) {
  let found = null;
  walk(root, (node) => {
    if (found) {
      return;
    }
    if (node.tagName === 'button' && node.dataset && node.dataset.eventToken === token) {
      found = node;
    }
  });
  return found;
}

function snapshotTree(root) {
  const lines = [];
  function rec(node, depth) {
    const indent = ' '.repeat(depth * 2);
    lines.push(indent + [
      node.tagName,
      node.dataset.kind || '',
      node.dataset.nodeId || '',
      JSON.stringify(node.textContent || ''),
      String(node.disabled)
    ].join('|'));
    for (const child of node.children || []) {
      rec(child, depth + 1);
    }
  }
  rec(root, 0);
  return lines.join('\n');
}

async function run(artifactPath) {
  globalThis.MachinaUIHost = hostModule;
  const wasmBytes = new Uint8Array(fs.readFileSync(artifactPath));
  const eventJSONs = [];
  const nativeBridge = {
    getWasmArtifactBase64: () => Buffer.from(wasmBytes).toString('base64'),
    onEventJSON: (eventJSON) => eventJSONs.push(eventJSON)
  };

  const desktopDoc = new FakeDocument();
  const desktopContainer = desktopDoc.createElement('div');
  const desktop = await desktopModule.bootDesktopHost({
    document: desktopDoc,
    container: desktopContainer,
    nativeBridge
  });

  findButtonByToken(desktopContainer.firstChild, 'counter.increment').click();
  findButtonByToken(desktopContainer.firstChild, 'route.stats').click();
  const statsTree = desktopContainer.firstChild;
  findButtonByToken(desktopContainer.firstChild, 'route.home').click();
  const finalDesktopTree = desktopContainer.firstChild;

  const browserDoc = new FakeDocument();
  const browserContainer = browserDoc.createElement('div');
  const browserHost = new hostModule.MachinaUIWasmHost({
    document: browserDoc,
    container: browserContainer,
    wasmBytes
  });
  await browserHost.load();
  browserHost.renderInto(browserContainer);
  findButtonByToken(browserContainer.firstChild, 'counter.increment').click();
  findButtonByToken(browserContainer.firstChild, 'route.stats').click();
  findButtonByToken(browserContainer.firstChild, 'route.home').click();

  const summary = {
    abi: desktop.initialDocument.abi,
    bootShowsHome: collectText(desktopContainer.firstChild).includes('route=home'),
    statsRouteVisible: collectText(statsTree).includes('route=stats'),
    homeAfterRouteBack: collectText(finalDesktopTree).includes('route=home'),
    countPersistsAcrossRoute: collectText(statsTree).includes('count=1') && collectText(finalDesktopTree).includes('count=1'),
    capturedCanonicalEvents: eventJSONs,
    deterministicParityWithBrowserHost: snapshotTree(finalDesktopTree) === snapshotTree(browserContainer.firstChild)
  };

  process.stdout.write(JSON.stringify(summary));
}

run(process.argv[1]).catch((err) => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
`)

	cmd := exec.Command("node", "-e", script, artifactPath, hostPath, desktopPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node desktop host harness failed: %v\n%s", err, string(out))
	}

	var summary struct {
		ABI                            string   `json:"abi"`
		BootShowsHome                  bool     `json:"bootShowsHome"`
		StatsRouteVisible              bool     `json:"statsRouteVisible"`
		HomeAfterRouteBack             bool     `json:"homeAfterRouteBack"`
		CountPersistsAcrossRoute       bool     `json:"countPersistsAcrossRoute"`
		CapturedCanonicalEvents        []string `json:"capturedCanonicalEvents"`
		DeterministicParityWithBrowser bool     `json:"deterministicParityWithBrowserHost"`
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("decode desktop harness summary: %v\noutput=%q", err, string(out))
	}

	if summary.ABI != uiirJSONABI {
		t.Fatalf("expected abi %q, got %q", uiirJSONABI, summary.ABI)
	}
	if !summary.BootShowsHome {
		t.Fatalf("expected desktop host boot render to contain route=home")
	}
	if !summary.StatsRouteVisible || !summary.HomeAfterRouteBack {
		t.Fatalf("expected route.stats and route.home transitions in desktop host")
	}
	if !summary.CountPersistsAcrossRoute {
		t.Fatalf("expected count state to remain source-of-truth across desktop route changes")
	}
	if len(summary.CapturedCanonicalEvents) != 3 {
		t.Fatalf("expected 3 canonical event JSON dispatches, got %d", len(summary.CapturedCanonicalEvents))
	}
	if !strings.Contains(summary.CapturedCanonicalEvents[0], `"token":"counter.increment"`) ||
		!strings.Contains(summary.CapturedCanonicalEvents[1], `"token":"route.stats"`) ||
		!strings.Contains(summary.CapturedCanonicalEvents[2], `"token":"route.home"`) {
		t.Fatalf("expected canonical event JSON token sequence, got %v", summary.CapturedCanonicalEvents)
	}
	if !summary.DeterministicParityWithBrowser {
		t.Fatalf("expected desktop webview host to match browser host for same event sequence")
	}
}
