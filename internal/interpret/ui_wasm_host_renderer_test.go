//go:build toolchain

package interpret

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachinaUIHostRendererRendersWasmAndRoundTripsEvents(t *testing.T) {
	requireNodeTool(t)

	artifactPath := filepath.Join(t.TempDir(), machinaUIWasmArtifactName)
	if err := EmitMachinaUIWasmArtifact(artifactPath); err != nil {
		t.Fatalf("EmitMachinaUIWasmArtifact failed: %v", err)
	}

	hostPath := filepath.Join("..", "..", "tools", "machina-ui-host", "host.js")
	script := strings.TrimSpace(`
const fs = require('fs');
const hostModule = require(process.argv[2]);

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
    const attrs = [
      'tag=' + node.tagName,
      'kind=' + (node.dataset.kind || ''),
      'id=' + (node.dataset.nodeId || ''),
      'token=' + (node.dataset.eventToken || ''),
      'unit=' + (node.dataset.boxUnit || ''),
      'z=' + (node.dataset.zOrder || ''),
      'disabled=' + String(node.disabled),
      'text=' + JSON.stringify(node.textContent || '')
    ];
    lines.push(indent + attrs.join('|'));
    for (const child of node.children || []) {
      rec(child, depth + 1);
    }
  }
  rec(root, 0);
  return lines.join('\n');
}

async function run(artifactPath) {
  const wasmBytes = new Uint8Array(fs.readFileSync(artifactPath));
  const fakeDocument = new FakeDocument();
  const root = fakeDocument.createElement('div');

  const host = new hostModule.MachinaUIWasmHost({
    document: fakeDocument,
    container: root,
    wasmBytes
  });

  await host.load();
  const firstDoc = host.renderInto(root);
  const firstTree = root.firstChild;

  const increment = findButtonByToken(firstTree, 'counter.increment');
  increment.click();

  const afterIncrementTree = root.firstChild;
  const stats = findButtonByToken(afterIncrementTree, 'route.stats');
  stats.click();

  const afterRouteTree = root.firstChild;
  const home = findButtonByToken(afterRouteTree, 'route.home');
  home.click();

  const deterministicHost = new hostModule.MachinaUIWasmHost({
    document: new FakeDocument(),
    container: new FakeDocument().createElement('div'),
    wasmBytes
  });
  await deterministicHost.load();
  deterministicHost.renderInto(deterministicHost.container);
  findButtonByToken(deterministicHost.container.firstChild, 'counter.increment').click();
  findButtonByToken(deterministicHost.container.firstChild, 'route.stats').click();
  findButtonByToken(deterministicHost.container.firstChild, 'route.home').click();

  const syntheticDocument = {
    abi: hostModule.UIIR_ABI,
    root: {
      id: '0',
      kind: 'Column',
      key: null,
      text: null,
      label: null,
      enabled: null,
      event: null,
      box: null,
      layout: null,
      children: [
        { id: '0.0', kind: 'Text', key: null, text: 'x', label: null, enabled: null, event: null, box: null, layout: null, children: [] },
        { id: '0.1', kind: 'Spacer', key: null, text: null, label: null, enabled: null, event: null, box: null, layout: null, children: [] },
        {
          id: '0.2', kind: 'Row', key: null, text: null, label: null, enabled: null, event: null, box: null, layout: null,
          children: [
            { id: '0.2.0', kind: 'Button', key: null, text: null, label: 'ok', enabled: true, event: { token: 'synthetic.ok', payload: null }, box: null, layout: null, children: [] }
          ]
        },
        {
          id: '0.3', kind: 'Grid', key: null, text: null, label: null, enabled: null, event: null, box: null, layout: null,
          children: [
            { id: '0.3.0', kind: 'Text', key: null, text: 'g', label: null, enabled: null, event: null, box: null, layout: null, children: [] }
          ]
        },
        {
          id: '0.4', kind: 'AbsoluteBox', key: null, text: null, label: null, enabled: null, event: null,
          box: { kind: 'absolute', z: 2, absolute: { x: 1, y: 2, width: 3, height: 4 }, anchored: null },
          layout: null,
          children: []
        },
        {
          id: '0.5', kind: 'AnchorBox', key: null, text: null, label: null, enabled: null, event: null,
          box: { kind: 'anchored', z: -1, absolute: null, anchored: { left: 0.1, top: 0.2, right: 0.9, bottom: 0.8 } },
          layout: null,
          children: []
        },
        {
          id: '0.6', kind: 'AbsoluteBox', key: null, text: null, label: null, enabled: null, event: null,
          box: { kind: 'absolute', absolute: { x: 9, y: 9, width: 9, height: 9 }, anchored: null },
          layout: null,
          children: []
        }
      ]
    }
  };
  const syntheticRoot = fakeDocument.createElement('div');
  const syntheticEvents = [];
  hostModule.renderUIDocument(fakeDocument, syntheticRoot, syntheticDocument, (event) => syntheticEvents.push(event.token));
  findButtonByToken(syntheticRoot.firstChild, 'synthetic.ok').click();
  const syntheticKinds = syntheticRoot.firstChild.children.filter((c) => c.dataset && (c.dataset.kind === 'AbsoluteBox' || c.dataset.kind === 'AnchorBox'));
  const syntheticOrder = syntheticKinds.map((node) => node.dataset.nodeId);
  const absWithZ = syntheticKinds.find((node) => node.dataset.nodeId === '0.4');
  const anchoredWithZ = syntheticKinds.find((node) => node.dataset.nodeId === '0.5');
  const absDefaultZ = syntheticKinds.find((node) => node.dataset.nodeId === '0.6');
  let syntheticRejectsOutOfRangeZ = false;
  try {
    hostModule.renderUIDocument(fakeDocument, fakeDocument.createElement('div'), {
      abi: hostModule.UIIR_ABI,
      root: {
        id: '0',
        kind: 'Column',
        key: null,
        text: null,
        label: null,
        enabled: null,
        event: null,
        box: null,
        layout: null,
        children: [
          {
            id: '0.0', kind: 'AbsoluteBox', key: null, text: null, label: null, enabled: null, event: null,
            box: { kind: 'absolute', z: 6, absolute: { x: 0, y: 0, width: 1, height: 1 }, anchored: null },
            layout: null,
            children: []
          }
        ]
      }
    }, () => {});
  } catch (err) {
    syntheticRejectsOutOfRangeZ = String(err).includes('z-order must be within [-5, 5]');
  }

  const summary = {
    abi: firstDoc.abi,
    initialHasHome: collectText(firstTree).includes('route=home'),
    initialHasCount0: collectText(firstTree).includes('count=0'),
    incrementHasCount1: collectText(afterIncrementTree).includes('count=1'),
    routedHasStats: collectText(afterRouteTree).includes('route=stats'),
    routedHasCount1: collectText(afterRouteTree).includes('count=1'),
    backHomeHasRoute: collectText(root.firstChild).includes('route=home'),
    deterministic: snapshotTree(root.firstChild) === snapshotTree(deterministicHost.container.firstChild),
    syntheticKinds: [
      !!findButtonByToken(syntheticRoot.firstChild, 'synthetic.ok'),
      !!syntheticRoot.firstChild.children.find((c) => c.dataset && c.dataset.kind === 'Spacer'),
      !!syntheticRoot.firstChild.children.find((c) => c.dataset && c.dataset.kind === 'Grid'),
      !!syntheticRoot.firstChild.children.find((c) => c.dataset && c.dataset.kind === 'AbsoluteBox'),
      !!syntheticRoot.firstChild.children.find((c) => c.dataset && c.dataset.kind === 'AnchorBox')
    ].every(Boolean),
    syntheticEventRoundtrip: syntheticEvents.length === 1 && syntheticEvents[0] === 'synthetic.ok',
    syntheticAbsoluteUsesPX: absWithZ && absWithZ.dataset.boxUnit === 'px' && absWithZ.style.left === '1px' && absWithZ.style.width === '3px',
    syntheticAnchoredUsesUI: anchoredWithZ && anchoredWithZ.dataset.boxUnit === 'ui' && anchoredWithZ.style.left === '10%' && anchoredWithZ.style.right === '9.999999999999998%',
    syntheticZOrdering: JSON.stringify(syntheticOrder) === JSON.stringify(['0.5', '0.6', '0.4']),
    syntheticDefaultZZero: absDefaultZ && absDefaultZ.dataset.zOrder === '0' && absDefaultZ.style.zIndex === '0',
    syntheticRejectsOutOfRangeZ
  };

  process.stdout.write(JSON.stringify(summary));
}

run(process.argv[1]).catch((err) => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
`)

	cmd := exec.Command("node", "-e", script, artifactPath, hostPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node host renderer harness failed: %v\n%s", err, string(out))
	}

	var summary struct {
		ABI                  string `json:"abi"`
		InitialHasHome       bool   `json:"initialHasHome"`
		InitialHasCount0     bool   `json:"initialHasCount0"`
		IncrementHasCount1   bool   `json:"incrementHasCount1"`
		RoutedHasStats       bool   `json:"routedHasStats"`
		RoutedHasCount1      bool   `json:"routedHasCount1"`
		BackHomeHasRoute     bool   `json:"backHomeHasRoute"`
		Deterministic        bool   `json:"deterministic"`
		SyntheticKinds       bool   `json:"syntheticKinds"`
		SyntheticEventBridge bool   `json:"syntheticEventRoundtrip"`
		SyntheticAbsolutePX  bool   `json:"syntheticAbsoluteUsesPX"`
		SyntheticAnchoredUI  bool   `json:"syntheticAnchoredUsesUI"`
		SyntheticZOrder      bool   `json:"syntheticZOrdering"`
		SyntheticDefaultZ    bool   `json:"syntheticDefaultZZero"`
		SyntheticRejectsZ    bool   `json:"syntheticRejectsOutOfRangeZ"`
	}
	if err := json.Unmarshal(out, &summary); err != nil {
		t.Fatalf("decode harness summary: %v\noutput=%q", err, string(out))
	}

	if summary.ABI != uiirJSONABI {
		t.Fatalf("expected abi %q, got %q", uiirJSONABI, summary.ABI)
	}
	if !summary.InitialHasHome || !summary.InitialHasCount0 {
		t.Fatalf("expected initial render to contain route=home and count=0")
	}
	if !summary.IncrementHasCount1 {
		t.Fatalf("expected click increment to render count=1")
	}
	if !summary.RoutedHasStats || !summary.RoutedHasCount1 {
		t.Fatalf("expected route.stats click to switch view and preserve count")
	}
	if !summary.BackHomeHasRoute {
		t.Fatalf("expected route.home click to return to home screen")
	}
	if !summary.Deterministic {
		t.Fatalf("expected deterministic host tree for same interaction sequence")
	}
	if !summary.SyntheticKinds {
		t.Fatalf("expected synthetic renderer coverage for Row/Column/Grid/Spacer/AbsoluteBox/AnchorBox/Button/Text")
	}
	if !summary.SyntheticEventBridge {
		t.Fatalf("expected synthetic button click to invoke canonical host dispatch callback")
	}
	if !summary.SyntheticAbsolutePX || !summary.SyntheticAnchoredUI {
		t.Fatalf("expected host renderer to preserve px/ui unit semantics for absolute/anchored boxes")
	}
	if !summary.SyntheticZOrder || !summary.SyntheticDefaultZ {
		t.Fatalf("expected deterministic z-order rendering with default z=0")
	}
	if !summary.SyntheticRejectsZ {
		t.Fatalf("expected host renderer to reject out-of-range z-order")
	}
}
