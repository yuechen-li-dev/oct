# Testing instructions

## Portable judge lane

Run from the repository root with Go 1.24 or a current prebuilt `oct` binary:

```text
go run ./cmd/oct test docs/build-week/recording/fixtures/JudgeDemo --execution auto --json
go run ./cmd/oct artifact docs/build-week/recording/fixtures/JudgeDemo --execution interpreted --json
```

Using `go run` builds the CLI and is therefore the source fallback, not the
preferred no-rebuild judge path. Substitute `oct` (Linux) or `oct.exe`
(Windows) when using a CI test-build artifact.

Expected facts:

- `JudgeDemo.TwiceIsDeterministic` passes;
- `JudgeDemo.SummaryTextIsStable` passes;
- the structured result is `oct.cli.result.v1`;
- artifact execution is interpreted and writes exactly one JSON file under
  `out/build-week/judge-demo/`;
- the result reports path, `application/json`, size, and SHA-256.

## Plugin and MCP lanes

```text
go test ./cmd/oct-mcp ./internal/cli
go build -o dist/oct-mcp ./cmd/oct-mcp
dist/oct-mcp --help
```

On Windows the output is `dist/oct-mcp.exe`. The stable bounded hosted surface
is `oct_workspace_info`, `oct_test`, `oct_artifact`, `oct_run`, and
`oct_get_artifact`. The local plugin uses repository skills and the canonical
CLI; it is not a second compiler interface.

## SDSL-V compiler and conformance lanes

```text
go test ./internal/sdslv/...
go run ./cmd/sdslv conformance ./Examples/SDSL-V/conformance
```

Golden graphics outputs are already committed under
`Examples/SDSL-V/conformance/artifacts/`. Regenerating validated SPIR-V requires
DXC and `spirv-val`; compile-only Go tests remain portable.

## Prometheus source and hardware lanes

Read `primer/` before modifying native code. The portable test surface includes
the Go/native build orchestration; authoritative GPU reruns require the Vulkan
SDK, DXC, C/C++ toolchain, validation layers, and compatible hardware. Exact
commands and environment gates are in the milestone development reports under
`internal/prometheus/DevelopmentReport/`.

Do not interpret a skipped hardware lane as a reproduced GPU result. The
committed Build Week artifacts identify the witness as Windows x86-64, NVIDIA
RTX 3070, with the recorded driver/toolchain. Linux live Vulkan and AMD DVT are
not claimed.

## Repository validation used for this packet

```text
go test ./cmd/oct-mcp ./internal/cli ./internal/sdslv/...
go test ./...
actionlint .github/workflows/ci.yml
git diff --check
```

See [EVIDENCE_INDEX.md](EVIDENCE_INDEX.md) for claim-specific tests and
artifacts and [JUDGE_QUICKSTART.md](JUDGE_QUICKSTART.md) for the shortest path.
