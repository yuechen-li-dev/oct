# Judge quickstart

This is the shortest honest evaluation path. It does not require Vulkan
hardware. The public CI artifact path becomes available after this packet is
committed and its workflow succeeds; the tracked Linux server path works at the
audited head without a rebuild.

## Path A: Windows local Codex plugin from the CI test-build

Open the repository's [CI workflow](https://github.com/yuechen-li-dev/oct/actions/workflows/ci.yml),
select the successful run for the submission commit, and download
`oct-build-week-windows-amd64`. Extract it into a clone of the repository so
that `dist/oct.exe`, `dist/oct-mcp.exe`, and `plugins/oct` are present. In
PowerShell:

```powershell
$env:PATH = "$(Resolve-Path .\dist);$env:PATH"
.\dist\oct.exe --version
.\dist\oct-mcp.exe --help
```

The repository already contains `.agents/plugins/marketplace.json`. Restart the
ChatGPT desktop app, open **Plugins Directory**, choose **Personal**, and install
**Oct**. Open this repository as the workspace, then ask:

```text
Use the Oct plugin. Inspect the Build Week judge fixture, run its Oct tests,
then generate its artifact. Report execution mode and every artifact path,
MIME type, byte size, and SHA-256 hash.
```

Expected behavior: Codex selects the repository-native workflow, invokes
`oct test` and `oct artifact`, and returns structured `oct.cli.result.v1`
evidence. The plugin does not expose the host filesystem through hosted MCP;
local repository work deliberately delegates to the CLI.

The workflow configuration is actionlint-valid, but the new artifact is not
claimed to exist before the packet commit runs publicly. If it is absent, use
Path B or the exact source fallback in [TESTING_INSTRUCTIONS.md](TESTING_INSTRUCTIONS.md).

## Path B: tracked Linux x86-64 MCP server, no rebuild

The audited repository already tracks a Linux x86-64 `oct-mcp`. From a Linux
clone:

```bash
chmod +x ./oct-mcp
./oct-mcp --help
./oct-mcp serve --listen 127.0.0.1:8080
```

In a second terminal:

```bash
curl --fail http://127.0.0.1:8080/healthz
```

Expected response: `ok`. An MCP client can connect to
`http://127.0.0.1:8080/mcp` and inspect the five bounded virtual-workspace
tools. Do not expose this unauthenticated local process to the public internet.
The tracked binary is 38,925,822 bytes with SHA-256
`b9288f39fad803f42f49dc729985346f7943485a12db0fdd674218d85b096f5e` and was
introduced by eligible commit `b07c8849efa00fe0455e827e9a162856f389878f`.

## Path C: run the deterministic fixture directly

With an `oct` binary from the CI test-build artifact on `PATH`:

```bash
oct test docs/build-week/recording/fixtures/JudgeDemo --execution auto --json
oct artifact docs/build-week/recording/fixtures/JudgeDemo --execution interpreted --json
```

On Windows PowerShell use the same commands with `oct.exe`. The expected test
result has two passing facts and zero failures. The artifact result lists
`out/build-week/judge-demo/summary.json`, including its MIME type, byte size,
and SHA-256 hash. The fixture and expected values are committed, so the judge
does not need to improvise input.

The CI workflow builds `oct` and `oct-mcp` for Linux and Windows and uploads
them with the plugin folder as `oct-build-week-linux-amd64` and
`oct-build-week-windows-amd64`. Until the packet commit has a successful public
CI run, this is a documented test-build path, not a claimed published release.
There is no GitHub Release or hosted demo at packet time.

## Path D: inspect the specialized hardware evidence

No GPU is needed to review the recorded Vulkan result:

- fused reduction report:
  [`PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md`](../../internal/prometheus/DevelopmentReport/PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md)
- complete bounded transformer artifact:
  [`gated_ffn_complete_transformer_block_rtx3070.json`](../../internal/prometheus/DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json)
- four-block numerical audit:
  [`multi_block_golden_path_evt_closeout.json`](../../internal/prometheus/DevelopmentReport/artifacts/M48/multi_block_golden_path_evt_closeout.json)
- Shadow-HSFM artifact:
  [`numerical_shadow_hsfm_rtx3070.json`](../../internal/prometheus/DevelopmentReport/artifacts/M49b/numerical_shadow_hsfm_rtx3070.json)

These are authoritative only for the recorded Windows RTX 3070 environment.
See [TESTING_INSTRUCTIONS.md](TESTING_INSTRUCTIONS.md) for rebuild and hardware
lanes.
