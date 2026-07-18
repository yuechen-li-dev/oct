# Video shot list

Record at 1920×1080, 100% UI scale, with a 16–18 point editor font and no
notifications. Record clips in the order below; edit into timeline order after.
Keep the mouse still unless it is identifying a line.

| Final time | Record this screen | Exact path or command | Expected visible result | Edit note |
| --- | --- | --- | --- | --- |
| 00:00–00:07 | Title card | text in `VIDEO_SCRIPT.md` | Project title, category, repository | Fade in for 6 frames; no logo animation. |
| 00:07–00:18 | Repository README | `README.md`, Build Week section | Oct/SDSL-V/Prometheus definitions and the pre-existing-foundation paragraph | Slow vertical move only. |
| 00:18–00:31 | Clean eligible timeline | `powershell -File docs/build-week/recording/show-timeline.ps1` | Boundary, last pre-event SHA, then 23 eligible commits | Highlight the boundary and first/last eligible rows. |
| 00:31–00:42 | ROALoop explanation | `docs/build-week/CODEX_COLLABORATION.md`, first 35 lines | Persistent ChatGPT reviewer → fresh Codex author task → durable evidence → human decision | Add four small text labels; do not show third-party branding. |
| 00:42–00:53 | Fused reduction source/report | split view: `internal/prometheus/shaders/sdslv/production/reduction/softmax_fused.sdslv` and `internal/prometheus/DevelopmentReport/PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md` | Typed source beside M39b evidence summary | Hold on entry point and evidence table. |
| 00:53–01:03 | Graphics source/generated outputs | `Examples/SDSL-V/conformance/graphics/CanonicalGraphicsProgram.sdslvvalid`; then `Examples/SDSL-V/conformance/artifacts/ForwardTextured.vertex.hlsl` and `.bundle.json` | Vertex/pixel program, generated HLSL, SHA/provenance bundle | Use two straight cuts, no scrolling through blobs. |
| 01:03–01:18 | Milestone ladder | `powershell -File docs/build-week/recording/show-transformer.ps1` | M42 attention through M49b Shadow-HSFM with SHAs | Emphasize M47 complete bounded block and M48 fixed stack. |
| 01:18–01:34 | Hardware evidence | `internal/prometheus/DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json`, then M48 JSON | Device, validation, stage data, and numerical caveat | Zoom on `RTX 3070`; then on drift/acceptance field. |
| 01:34–01:47 | Real Oct test | `powershell -File docs/build-week/recording/run-judge-demo.ps1 -Lane test` | `oct.cli.result.v1`; 2 passed; compiled 2; fallback 0 | Keep command and final JSON visible. |
| 01:47–02:00 | Real artifact result | `powershell -File docs/build-week/recording/run-judge-demo.ps1 -Lane artifact` | `summary.json`, `application/json`, 62 bytes, SHA-256 | Freeze last frame for 2 seconds. |
| 02:00–02:11 | Before/after dogfood report | `docs/development/OCT_MCP_AGENT_DOGFOODING.md`, sections “Before” and “After” | Compiler-shaped tools reduced to canonical workflow | Highlight removed tools and schema name. |
| 02:11–02:23 | Codex plugin interaction | In this repository, paste the prompt below into a Codex task with the Oct plugin installed | Codex identifies fixture, invokes test/artifact, and reports the same metadata | Record live; trim model thinking/wait time. |
| 02:23–02:30 | Diagnostic/repair proof | `powershell -File docs/build-week/recording/run-repair-demo.ps1` | Normalized type diagnostic followed by a passing repaired run | Use the prepared clip if live plugin timing is long. |
| 02:30–02:44 | Closing card | text in `VIDEO_SCRIPT.md` | Persistent review · ephemeral authors · durable evidence | End on repository URL; 8-frame fade out. |

## Exact plugin prompt

```text
Use the Oct plugin and repository-native workflow. Inspect
docs/build-week/recording/fixtures/JudgeDemo, run its tests, generate its
artifact, and report execution mode plus each artifact path, MIME type, byte
size, and SHA-256. Do not change the fixture.
```

Expected final facts: two compiled tests, zero interpreted fallbacks, one
interpreted artifact, `out/build-week/judge-demo/summary.json`,
`application/json`, 62 bytes, SHA-256
`4efc9d55ed0eac0e8401f92f1d6b320e7e4c4b7e09778f1c5b7f9917c484209c`.

## Recording order

1. Run `verify-recording-assets.ps1`.
2. Record timeline and transformer scripts.
3. Record the test, artifact, and repair terminal clips.
4. Record source/report/editor clips.
5. Record the live Codex plugin clip.
6. Generate title/closing cards from the exact text.
7. Assemble in final-time order, add narration, import `VIDEO_CAPTIONS.srt`, and
   verify the uploaded cut is no longer than 2:50.
