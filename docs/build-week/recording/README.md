# Recording kit

Run all commands from the repository root. These scripts only read repository
history or write bounded demo output under `out/build-week/judge-demo` and
`.tmp/build-week-recording`; they do not change production behavior.

## Preflight

```powershell
powershell -File docs/build-week/recording/verify-recording-assets.ps1
```

Expected: every required source/report/artifact is found; the fixture test and
artifact lanes pass; the artifact metadata matches the recorded 62-byte hash.

## Ready-made terminal clips

```powershell
powershell -File docs/build-week/recording/show-timeline.ps1
powershell -File docs/build-week/recording/show-transformer.ps1
powershell -File docs/build-week/recording/run-judge-demo.ps1 -Lane test
powershell -File docs/build-week/recording/run-judge-demo.ps1 -Lane artifact
powershell -File docs/build-week/recording/run-repair-demo.ps1
```

The recording scripts use `go run ./cmd/oct` so they always capture the current
checkout. `run-judge-demo.ps1` also accepts `-OctPath <path>` for a selected
test-build binary. Judges should use the no-rebuild path in
`../JUDGE_QUICKSTART.md`.

## Editor clips

Open only these paths, in this order:

1. `README.md` — Build Week section.
2. `internal/prometheus/shaders/sdslv/production/reduction/softmax_fused.sdslv`.
3. `internal/prometheus/DevelopmentReport/PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md`.
4. `Examples/SDSL-V/conformance/graphics/CanonicalGraphicsProgram.sdslvvalid`.
5. `Examples/SDSL-V/conformance/artifacts/ForwardTextured.vertex.hlsl`.
6. `Examples/SDSL-V/conformance/artifacts/ForwardTextured.bundle.json`.
7. `internal/prometheus/DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json`.
8. `internal/prometheus/DevelopmentReport/artifacts/M48/multi_block_golden_path_evt_closeout.json`.
9. `docs/development/OCT_MCP_AGENT_DOGFOODING.md`.

Avoid opening `.spv` binary data. Show the `.sdslv`, generated `.hlsl`, and JSON
provenance instead.

## Codex plugin clip

Install the local Oct plugin as documented in `../JUDGE_QUICKSTART.md`. Paste
the exact prompt from `../VIDEO_SHOT_LIST.md`. Record the prompt and final
evidence only; cut waiting time. If the plugin result differs from the committed
fixture facts, stop and investigate rather than editing the narration.

## Cards

Open `title-card.html` and `closing-card.html` in a browser at 1920×1080. Use
full-screen mode, hide the browser chrome, and record six seconds of each. The
cards use only local HTML/CSS and the exact approved wording.
