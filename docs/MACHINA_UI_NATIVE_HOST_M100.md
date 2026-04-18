# Machina UI Native Host (M100 + M100b + M100c)

M100 established the first native-host trajectory for Machina UI with a **webview-backed host layer**.

M100b completes the first real native shell slice by adding a **real OS webview binding** around the existing shared host runtime.

M100c completes the provisioning/automation gap by making the tagged real-webview path reproducible and smoke-tested in automation.

These milestones keep the architectural boundary locked:

- same emitted Wasm artifact (`.ui.wasm`) as browser host
- same M96 JSON ABI (`machina.uiir.v1`)
- same canonical event JSON dispatch contract
- same layout semantics (`AbsoluteBox`=`px`, `AnchorBox`=`ui`, bounded deterministic z-order)

## What landed in M100

- `tools/machina-ui-desktop/desktop_host.js`
  - desktop/webview adapter that asks a native bridge for real Wasm bytes
  - reuses `tools/machina-ui-host/host.js` (`MachinaUIWasmHost`) directly
  - optionally forwards canonical dispatch JSON to host diagnostics hook
- `tools/machina-ui-desktop/index.html`
  - minimal desktop webview page wiring
- `internal/interpret/ui_wasm_desktop_host_test.go`
  - end-to-end harness proving desktop-host path consumes real emitted Wasm and M96 ABI

## What landed in M100b

- `internal/machina/desktophost/`
  - bounded native shell launcher that wraps and feeds the shared host runtime
  - native bridge injection for `.ui.wasm` bytes (`window.MachinaDesktopBridge.getWasmArtifactBase64`)
  - inlined host-page generation that embeds shared `tools/machina-ui-host/host.js` + desktop adapter `tools/machina-ui-desktop/desktop_host.js`
- `cmd/machina-ui-desktop-host`
  - launchable native desktop host entrypoint
  - CLI flags for artifact path, optional asset root, and window sizing/title
- `internal/machina/desktophost/driver_webview.go`
  - real OS webview binding via `github.com/webview/webview_go`
  - intentionally bounded to `linux`/`darwin`, `cgo`, and explicit build tag `machina_desktop_webview`
- `internal/machina/desktophost/driver_stub.go`
  - explicit unsupported-mode message in default builds without the tag/toolchain

## What landed in M100c

- `tools/machina-ui-desktop/setup_linux_webview_deps.sh`
  - one-command Ubuntu/Debian setup for required native deps
  - verifies `gtk+-3.0` + `webkit2gtk-4.0` with `pkg-config`
- `internal/machina/desktophost/driver_webview_smoke_test.go`
  - bounded native smoke that constructs the real `webview_go` driver and performs minimal initialization (`SetTitle`, `SetSize`, `Init`, `Navigate`) without fragile GUI scripting
  - gated by both build tags and env opt-in (`MACHINA_WEBVIEW_SMOKE=1`)
- `.github/workflows/ci.yml` (`native-webview-smoke-linux`)
  - installs Linux webview deps
  - builds tagged native desktop host
  - runs smoke test under `xvfb-run`

## Source-of-truth boundary

Wasm remains source of truth for:

- state
- transitions
- UIIR generation

Desktop shell remains only:

- native window/container
- webview bootstrapping + asset loading
- Wasm bytes bridge

No second UI runtime or native-widget semantic fork is introduced.

## Build/launch scope (intentional)

M100b lands a coherent real-shell slice without cross-platform productization churn.

Real native shell path is enabled when built with:

- build tag: `machina_desktop_webview`
- platform: `linux` or `darwin`
- `cgo` enabled
- system webview dependencies installed for `webview_go`

## Supported platform slice (M100c)

Bounded, automated support is currently provided for:

- Linux (`ubuntu-22.04` in CI) with:
  - `CGO_ENABLED=1`
  - build tag `machina_desktop_webview`
  - GTK + WebKitGTK development libraries

`darwin` remains supported by build constraints for local development, but is not yet exercised in CI.

## Linux dependency list (exact packages used in CI/dev path)

Install on Ubuntu/Debian:

- `pkg-config`
- `libgtk-3-dev`
- `libwebkit2gtk-4.0-dev`
- `xvfb` (CI/headless smoke execution)

## Reproducible setup (Linux)

```bash
sudo ./tools/machina-ui-desktop/setup_linux_webview_deps.sh
```

The script installs dependencies and validates host toolchain visibility via:

```bash
pkg-config --modversion gtk+-3.0
pkg-config --modversion webkit2gtk-4.0
```

## How to run locally (Linux)

Build and launch real native host:

```bash
CGO_ENABLED=1 go run -tags machina_desktop_webview ./cmd/machina-ui-desktop-host \
  -wasm /path/to/counter.ui.wasm
```

Run bounded native smoke (no GUI automation):

```bash
CGO_ENABLED=1 MACHINA_WEBVIEW_SMOKE=1 \
  xvfb-run -a go test -tags machina_desktop_webview ./internal/machina/desktophost \
  -run TestWebviewDriverFactoryConstructsAndInitializesNativeBinding -count=1
```

## CI status

CI now runs a dedicated `native-webview-smoke-linux` lane that:

1. provisions GTK/WebKit deps,
2. builds `./cmd/machina-ui-desktop-host` with `-tags machina_desktop_webview`,
3. executes the native-binding smoke test under `xvfb-run`.

This keeps default untagged paths unchanged while preventing silent regressions in the real native shell binding.

Example launch:

```bash
go run -tags machina_desktop_webview ./cmd/machina-ui-desktop-host \
  -wasm /path/to/counter.ui.wasm
```

If the tag/toolchain/platform preconditions are not met, the command exits with an explicit bounded unsupported message instead of pretending full coverage.
