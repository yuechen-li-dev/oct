(function (globalScope) {
  function assertCondition(condition, message) {
    if (!condition) {
      throw new Error(message);
    }
  }

  function decodeBase64ToBytes(base64) {
    if (typeof Buffer !== 'undefined') {
      return new Uint8Array(Buffer.from(base64, 'base64'));
    }
    const binary = globalScope.atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let idx = 0; idx < binary.length; idx++) {
      bytes[idx] = binary.charCodeAt(idx);
    }
    return bytes;
  }

  async function resolveWasmBytes(nativeBridge) {
    assertCondition(nativeBridge, 'native bridge is required');

    if (typeof nativeBridge.getWasmBytes === 'function') {
      const direct = await nativeBridge.getWasmBytes();
      if (direct instanceof Uint8Array) {
        return direct;
      }
      return new Uint8Array(direct);
    }

    if (typeof nativeBridge.getWasmArtifactBase64 === 'function') {
      const encoded = await nativeBridge.getWasmArtifactBase64();
      assertCondition(typeof encoded === 'string' && encoded.length > 0, 'native bridge returned empty wasm base64');
      return decodeBase64ToBytes(encoded);
    }

    throw new Error('native bridge must provide getWasmBytes() or getWasmArtifactBase64()');
  }

  async function bootDesktopHost(options) {
    assertCondition(globalScope.MachinaUIHost, 'MachinaUIHost is required; load tools/machina-ui-host/host.js first');

    const documentRef = options.document || globalScope.document;
    const container = options.container;
    const nativeBridge = options.nativeBridge || globalScope.MachinaDesktopBridge;
    assertCondition(container, 'desktop host requires a container');
    assertCondition(nativeBridge, 'desktop host requires native bridge');

    const wasmBytes = await resolveWasmBytes(nativeBridge);
    const dispatchedEvents = [];

    const host = new globalScope.MachinaUIHost.MachinaUIWasmHost({
      document: documentRef,
      container,
      wasmBytes,
      onBeforeDispatch: (eventJSON) => {
        dispatchedEvents.push(eventJSON);
        if (typeof nativeBridge.onEventJSON === 'function') {
          nativeBridge.onEventJSON(eventJSON);
        }
      }
    });

    await host.load();
    const initialDocument = host.renderInto(container);

    return {
      host,
      initialDocument,
      dispatchedEvents,
      rerender: () => host.renderInto(container)
    };
  }

  const exportsObject = {
    bootDesktopHost,
    resolveWasmBytes
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = exportsObject;
  }

  globalScope.MachinaUIDesktopHost = exportsObject;
})(typeof window !== 'undefined' ? window : globalThis);
