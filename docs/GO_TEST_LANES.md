# Go test lanes

Oct separates quick implementation feedback from compiler, executable, external-tool, and native-hardware validation. A test's build tag describes its dependency boundary; it is not a coverage waiver.

## Fast local unit lane

Run the two high-signal iteration commands:

```powershell
go test ./cmd/oct
go test ./internal/...
```

Or run `tools/Run-GoUnitTests.ps1`. The default lane requires no DXC, Vulkan device, native Prometheus reactor, Node, Git operation, sidecar, or separately built Oct executable.

Useful focused commands:

```powershell
# In-process CLI dispatch, diagnostics, and representative compiler boundary
go test ./cmd/oct -run 'Help|Version|Fmt|RunCommand|BuildCommand'

# SDSL-V implementation and in-process CLI emission
go test ./internal/sdslv/...
go test ./cmd/oct -run SDSLv

# Prometheus Go bridge logic without a native reactor
go test ./internal/prometheus

# One internal package during implementation
go test ./internal/parse
```

## Integration lane

The `integration` tag owns exhaustive CLI/compiler/corpus, generated-runner, artifact, benchmark, and real executable-boundary coverage:

```powershell
go test -tags=integration ./...
```

The CLI executable smoke uses the shared test binary; ordinary CLI assertions call `internal/cli.Execute` directly.

## External-tool lane

The `toolchain` tag owns tests that may invoke Git, the Go toolchain, Node, DXC, or Octxiliary sidecars:

```powershell
go test -tags=toolchain ./...
```

Real sidecars remain an explicit sub-lane:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

Missing optional tools cause a named toolchain test to skip with its discovery error. They never affect the default lane.

## Full CI and acceptance

Run all non-hardware coverage explicitly:

```powershell
go test -count=1 ./...
go test -count=1 -tags=integration ./...
go test -count=1 -tags=toolchain ./...
```

`tools/Run-GoIntegrationTests.ps1 -IncludeToolchain` runs the latter two tagged lanes. CI runs the fast lane first and attributes integration and toolchain failures separately.

Compiled `.octest`, artifact, and benchmark runners are lifecycle-scoped under
an owned temporary directory. Normal runs remove the entire scope on success,
compile/runtime failure, and timeout. Set `OCT_TEST_TEMP_ROOT` to choose the
parent directory. Set `OCT_KEEP_TEST_ARTIFACTS=1` only while debugging; Oct then
prints each retained directory instead of scattering runner files into the
working tree. This policy does not affect persistent `oct build` outputs.

To inspect or remove legacy leaked runners safely:

```powershell
# Dry-run is the default; only zz_oct_test_runner_*.octest.octbin is matched.
./tools/Clean-OctTestArtifacts.ps1 -Root . -OlderThan ([TimeSpan]::FromDays(1))

# Explicit deletion after reviewing the list.
./tools/Clean-OctTestArtifacts.ps1 -Root . -OlderThan ([TimeSpan]::FromDays(1)) -Delete
```

Use `-IncludeTempDirectories` only when the selected root contains abandoned
owned scopes named `octest-run-*`, `oct-artifact-run-*`, or
`oct-benchmark-run-*`. Arbitrary `.octbin` files are never matched.

## Native hardware lane

Windows cgo tests that load the Prometheus reactor are tagged `native` and require an actual reactor and compatible Vulkan hardware:

```powershell
$env:OCT_RUN_PROMETHEUS_INTEGRATION = "1"
$env:OCT_PROMETHEUS_REACTOR = "C:\path\to\prometheus-reactor.dll"
go test -count=1 -tags=native ./internal/prometheus ./cmd/oct
```

The Machina WebView smoke retains its existing `machina_desktop_webview` tag and platform prerequisites. Neither native lane belongs on generic runners.

## Profiling

Capture Go JSON and summarize it without changing test behavior:

```powershell
go test -count=1 -json ./cmd/oct > out/test-artifacts/cmd.json
./tools/Profile-GoTests.ps1 -JsonPath out/test-artifacts/cmd.json `
  -OutputJson out/test-artifacts/cmd-summary.json `
  -OutputMarkdown out/test-artifacts/cmd-summary.md
```

Tests using `executeCLIInDir` remain serial because package discovery currently depends on the process working directory. Sidecar tests also remain bounded by their shared executable/build setup. Do not add `t.Parallel()` to either family without first removing that shared state.
