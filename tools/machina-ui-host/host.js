(function (globalScope) {
  const UIIR_ABI = 'machina.uiir.v1';
  const DEFAULT_WASM_GAP = 64;

  function assertCondition(condition, message) {
    if (!condition) {
      throw new Error(message);
    }
  }

  const utf8Decoder = new TextDecoder('utf-8');
  const utf8Encoder = new TextEncoder();

  function readUTF8(memory, ptr, len) {
    return utf8Decoder.decode(memory.slice(ptr, ptr + len));
  }

  function writeUTF8(memory, ptr, text) {
    const bytes = utf8Encoder.encode(text);
    memory.set(bytes, ptr);
    return bytes.length;
  }

  function styleFromLayout(layout) {
    if (!layout) {
      return null;
    }
    return {
      left: layout.x + 'px',
      top: layout.y + 'px',
      width: layout.width + 'px',
      height: layout.height + 'px'
    };
  }

  function applyStyles(target, styles) {
    if (!styles) {
      return;
    }
    for (const key of Object.keys(styles)) {
      target.style[key] = styles[key];
    }
  }

  function applyBoxLayout(target, node) {
    const layoutStyles = styleFromLayout(node.layout);
    if (layoutStyles) {
      target.style.position = 'absolute';
      applyStyles(target, layoutStyles);
      return;
    }

    if (!node.box) {
      return;
    }

    target.style.position = 'absolute';
    if (node.box.kind === 'absolute' && node.box.absolute) {
      const absolute = node.box.absolute;
      target.style.left = absolute.x + 'px';
      target.style.top = absolute.y + 'px';
      target.style.width = absolute.width + 'px';
      target.style.height = absolute.height + 'px';
      return;
    }
    if (node.box.kind === 'anchored' && node.box.anchored) {
      const anchored = node.box.anchored;
      target.style.left = anchored.left * 100 + '%';
      target.style.top = anchored.top * 100 + '%';
      target.style.right = (1 - anchored.right) * 100 + '%';
      target.style.bottom = (1 - anchored.bottom) * 100 + '%';
    }
  }

  function renderNode(documentRef, node, dispatchEvent) {
    const kind = node.kind;
    let el;
    switch (kind) {
      case 'Text':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-text';
        el.textContent = node.text || '';
        break;
      case 'Button':
        el = documentRef.createElement('button');
        el.className = 'machina-node machina-button';
        el.textContent = node.label || '';
        el.disabled = !node.enabled;
        if (node.event && node.event.token) {
          el.dataset.eventToken = node.event.token;
          if (node.enabled) {
            el.addEventListener('click', () => dispatchEvent(node.event));
          }
        }
        break;
      case 'Row':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-row';
        break;
      case 'Column':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-column';
        break;
      case 'Grid':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-grid';
        break;
      case 'Spacer':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-spacer';
        break;
      case 'AbsoluteBox':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-absolute-box';
        applyBoxLayout(el, node);
        break;
      case 'AnchorBox':
        el = documentRef.createElement('div');
        el.className = 'machina-node machina-anchor-box';
        applyBoxLayout(el, node);
        break;
      default:
        throw new Error('unsupported UIIR kind: ' + kind);
    }

    el.dataset.nodeId = node.id;
    el.dataset.kind = kind;

    if ((kind === 'AbsoluteBox' || kind === 'AnchorBox') && !el.style.position) {
      el.style.position = 'absolute';
    }

    for (const child of node.children || []) {
      el.appendChild(renderNode(documentRef, child, dispatchEvent));
    }
    return el;
  }


  function renderUIDocument(documentRef, container, documentPayload, dispatchEvent) {
    assertCondition(documentPayload && documentPayload.abi === UIIR_ABI, 'unsupported UIIR ABI: ' + (documentPayload && documentPayload.abi));
    while (container.firstChild) {
      container.removeChild(container.firstChild);
    }
    const rootEl = renderNode(documentRef, documentPayload.root, dispatchEvent);
    container.appendChild(rootEl);
    return rootEl;
  }

  class MachinaUIWasmHost {
    constructor(options) {
      this.document = options.document;
      this.container = options.container;
      this.fetchImpl = options.fetchImpl || null;
      this.wasmBytes = options.wasmBytes || null;
      this.wasmURL = options.wasmURL || null;
      this.wasmGap = options.wasmGap || DEFAULT_WASM_GAP;
      this.instance = null;
      this.exports = null;
      this.lastRenderJSON = '';
      this.lastRenderDocument = null;
    }

    async load() {
      assertCondition(this.wasmBytes || this.wasmURL, 'host requires wasm bytes or wasm URL');
      let wasmBytes = this.wasmBytes;
      if (!wasmBytes) {
        assertCondition(this.fetchImpl, 'fetch implementation is required for wasm URL loading');
        const response = await this.fetchImpl(this.wasmURL);
        wasmBytes = new Uint8Array(await response.arrayBuffer());
      }

      const instantiated = await WebAssembly.instantiate(wasmBytes, {});
      this.instance = instantiated.instance;
      this.exports = this.instance.exports;

      const initStatus = Number(this.exports.ExportMachinaUIInit());
      if (initStatus !== 0) {
        throw new Error('ExportMachinaUIInit failed with status=' + initStatus);
      }
    }

    currentMemory() {
      assertCondition(this.exports && this.exports.memory, 'wasm exports not loaded');
      return new Uint8Array(this.exports.memory.buffer);
    }

    readRenderJSON() {
      const mem = this.currentMemory();
      const ptr = Number(this.exports.ExportMachinaUIBufferPtr());
      const len = Number(this.exports.ExportMachinaUIBufferLen());
      return readUTF8(mem, ptr, len);
    }

    writeDispatchJSON(eventJSON) {
      const mem = this.currentMemory();
      const ptr = Number(this.exports.ExportMachinaUIBufferPtr()) + Number(this.exports.ExportMachinaUIBufferLen()) + this.wasmGap;
      const len = writeUTF8(mem, ptr, eventJSON);
      return { ptr, len };
    }

    dispatchEvent(event) {
      const token = event && event.token;
      assertCondition(!!token, 'event token is required');
      const eventJSON = JSON.stringify({ token, payload: event.payload ?? null });
      const written = this.writeDispatchJSON(eventJSON);
      const status = Number(this.exports.ExportMachinaUIDispatchEventJSON(written.ptr, written.len));
      if (status !== 0) {
        throw new Error('ExportMachinaUIDispatchEventJSON failed with status=' + status + ' token=' + token);
      }
      return status;
    }

    renderInto(container) {
      const target = container || this.container;
      assertCondition(target, 'render container is required');
      const renderStatus = Number(this.exports.ExportMachinaUIRender());
      if (renderStatus !== 0) {
        throw new Error('ExportMachinaUIRender failed with status=' + renderStatus);
      }
      const payloadJSON = this.readRenderJSON();
      const documentPayload = JSON.parse(payloadJSON);
      this.lastRenderJSON = payloadJSON;
      this.lastRenderDocument = documentPayload;
      renderUIDocument(this.document, target, documentPayload, (event) => {
        this.dispatchEvent(event);
        this.renderInto(target);
      });
      return documentPayload;
    }
  }

  async function bootBrowserHost(options) {
    const host = new MachinaUIWasmHost({
      document: options.document || globalScope.document,
      container: options.container,
      fetchImpl: options.fetchImpl || globalScope.fetch.bind(globalScope),
      wasmURL: options.wasmURL,
      wasmGap: options.wasmGap || DEFAULT_WASM_GAP
    });
    await host.load();
    host.renderInto(options.container);
    return host;
  }

  const exportsObject = {
    UIIR_ABI,
    MachinaUIWasmHost,
    bootBrowserHost,
    renderUIDocument
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = exportsObject;
    return;
  }
  globalScope.MachinaUIHost = exportsObject;
})(typeof window !== 'undefined' ? window : globalThis);
