# MACHINA UI Gio Backend Spike (M109)

M109 adds an optional Gio native backend spike for Machina UI.

## Scope and ownership

Gio is backend-only in this milestone:

- owns window lifecycle, pointer input, and drawing submission.
- consumes Machina render commands.
- sends pointer coordinates to `session.PointerDown`.

Gio does **not** own:

- app state
- UIIR lowering
- layout resolution
- hit-testing semantics
- action dispatch semantics

Those remain in the existing deterministic session pipeline from M108.

## What this spike proves

- A real native window loop can consume Machina render output.
- Pointer click coordinates can flow back through session hit testing and dispatch.
- A counter demo can rerender through state changes without handing semantic ownership to Gio.

## Build tags and dependency gating

Native Gio windowing code is isolated behind `machina_gio`:

- `internal/machina/backend/gio/window_gio.go` uses `//go:build machina_gio`.
- `cmd/machina-gio-smoke/main.go` uses `//go:build machina_gio`.

This means untagged/default builds do not compile native Gio window dependencies.

The headless translation layer and tests stay in the untagged lane:

- `internal/machina/backend/gio/translate.go`
- `internal/machina/backend/gio/translate_test.go`

## Native dependencies (Linux)

When you run tagged Gio commands (`-tags machina_gio`) on Linux, system native dependencies are required in addition to Go modules.

Common requirements include:

- `pkg-config`
- xkbcommon development package (pkg-config module: `xkbcommon`)
- Wayland client development package (pkg-config module: `wayland-client`)
- graphics/window stack packages as required by Gio/OpenGL/EGL/Vulkan on your distro

Distro package names vary. Use your distro documentation and Gio guidance for exact package names.

### Local preflight helper

Check Linux pkg-config modules before running tagged Gio commands:

```bash
./tools/machina-ui-gio/check_linux_deps.sh
```

Equivalent direct check:

```bash
pkg-config --exists xkbcommon wayland-client
```

If prerequisites are missing, tagged Gio commands are expected to fail with pkg-config/native linker errors.

## Build and run

This backend is isolated behind the `machina_gio` build tag.

Run backend tests:

```bash
go test -tags machina_gio ./internal/machina/backend/gio
```

Run demo:

```bash
go run -tags machina_gio ./cmd/machina-gio-smoke
```

## CI posture

Default CI lanes do not require a native window or GPU.

- Core packages continue to build and test without `machina_gio`.
- Gio backend code compiles and tests only in an explicit tagged/manual lane.
- Normal CI should not add `-tags machina_gio` unless the lane first installs Gio native Linux dependencies.

Baseline untagged verification command:

```bash
go test ./internal/... ./cmd/oct
```

## Current limitations

- Drawing is intentionally minimal (rectangles + basic text label drawing).
- Theme/styling fidelity is not production-targeted in this spike.
- No keyboard/focus/text-input path is provided beyond pointer/click flow.
- Automated GUI interaction tests are not added to normal CI.

## Future work before production backend

- Map backend drawing closer to style semantics when style contracts are stabilized.
- Harden clip-stack behavior and visual parity checks against snapshots.
- Add resize/viewport plumbing from native window size events.
- Expand input coverage (hover, keyboard, focus) without changing semantic ownership boundaries.
