# Oct

Oct is a scientific programming language and toolchain for reproducible research.

It is designed for the point where notebooks and scripts stop being enough: when an experiment needs tests, units, artifacts, packages, native binaries, and a distribution story. Oct's guiding principle is that the correct way should also be the easiest way.

## OpenAI Build Week 2026

**Submission:** *Oct and SDSL-V: Programming Languages Built by AI, Used by AI*
**Category:** Developer Tools

This submission is the eligible July 13–21 extension of a project that predates
Build Week. Oct is the correctness-oriented scientific language and toolchain;
SDSL-V is the typed compute/graphics shader language; Prometheus is the Vulkan
execution runtime in this repository; and ROALoop is the human-orchestrated
review/author loop. Persistent ChatGPT holds cross-milestone context, reviews
handoffs, and helps prepare each bounded prompt; fresh GPT-5.6 Codex author
tasks re-ground in the repository, implement a vertical, and leave durable
tests, reports, and artifacts for the next review. Occasional Claude work is
part of the older repository history. The Oct foundation, SDSL-V compute
foundation, Prometheus SGEMM/runtime,
and earlier ROALoop work all existed before the event and are not presented as
Build Week inventions.

From the official eligibility boundary, July 13, 2026 at 9:00 a.m. Pacific,
through the current submission state, GPT-5.6 Codex sessions materially added:

- the production SDSL-V fused-reduction reactor and its Vulkan lifecycle,
  correctness, benchmark, and machine-readable evidence;
- a direct same-harness GGML GLSL versus SDSL-V SPIR-V audit that exposed a
  short-row reduction weakness, then added one bounded packed-row plan with
  controlled RTX 3070 crossover evidence; no general GGML or matmul claim is
  made;
- the canonical SDSL-V vertex/pixel graphics language and conformance bundle
  (a compiler/toolchain feature, not a graphics runtime or renderer);
- a bounded device-resident transformer vertical: cooperative matrix SGEMM,
  attention, grouped multi-head attention, output projection, residual add,
  RMSNorm, gated FFN, one complete experimental transformer block, and a fixed
  four-block stack;
- numerical-heterogeneity experiments, stage-gain/mitigation evidence, and an
  experimental Shadow-HSFM observer/controller path on one RTX 3070;
- a Codex-native Oct plugin, bounded MCP server, structured `oct test` and
  `oct artifact` JSON, and a dogfooding campaign that removed speculative MCP
  tools and made the real CLI workflow authoritative.

GPT-5.6 Sol handled most one-shot vertical implementation and validation passes;
GPT-5.6 Terra also participated in the M40b, M48, M49b, and plugin/productization
threads. Codex accelerated repository inspection, cross-layer implementation,
test generation, hardware-corpus execution, artifact production, and
documentation. The human owner chose the product thesis, milestone order,
architecture and scope boundaries, hardware questions, stop/go criteria,
unsupported claims, and when failed numerical hypotheses required a course
correction. Claude's direct Build Week role was limited mainly to occasional
rubber-duck review and audit. Its original `oct-mcp` work and portions of earlier
libraries belong to the pre-event baseline; Build Week plugin productization and
redesign were done in GPT-5.6 Codex sessions.

The models now use the same user-facing contracts they helped improve: local
Codex edits Oct source, runs `oct test ... --json`, repairs the returned
diagnostic, then runs `oct artifact ... --json` and reports exact paths, types,
sizes, and SHA-256 hashes. SDSL-V sources compile to reviewable HLSL and
validated SPIR-V; Prometheus consumes production or explicitly experimental
shader assets under recorded authority boundaries.

Judges should start with the [Build Week judge quickstart](docs/build-week/JUDGE_QUICKSTART.md).
The no-rebuild path is the tracked Linux x86-64 `oct-mcp` server; the strongest
local desktop-plugin path is the Windows CI test-build artifact configured by
this revision once its public workflow run succeeds. The packet also includes a
minimal deterministic Oct test and artifact fixture. The
specialized Vulkan evidence is prerecorded and committed because live execution
was measured only on Windows with an NVIDIA RTX 3070 and Vulkan validation.

Supported and limited today:

- Oct CLI/compiler: developed and tested on Windows x86-64 and Linux x86-64;
  it is a pre-1.0 preview and compiled feature coverage is not universal.
- SDSL-V compiler/toolchain: Windows and Linux Go tests; graphics artifact
  compilation additionally requires DXC and `spirv-val`; there is no graphics
  runtime, window, swapchain, render pass, or pipeline-state engine.
- Prometheus native runtime: authoritative live Build Week hardware evidence is
  Windows x86-64 on one RTX 3070/driver; Linux compiles and smoke-tests, but no
  live Linux Vulkan result or AMD/cross-vendor DVT is claimed.
- Oct plugin/MCP: local plugin metadata and bounded stdio/HTTP server are
  present; the committed prebuilt server is Linux x86-64. CI builds Windows and
  Linux test binaries. No hosted public MCP deployment or marketplace approval
  is claimed.

The exact eligibility ledger, commits, sessions, claims, artifacts, judge
commands, Devpost copy, and video package are indexed in
[docs/build-week/README.md](docs/build-week/README.md).

## What is Oct?

Oct is an early scientific programming language/toolchain for portable computation, reproducible research, and AI-assisted experimentation.

Oct is built on Go as its systems substrate. Oct programs compile through Go, build quickly, run as native binaries, and target the platforms Go targets. Existing Go libraries can be exposed to Oct through explicit Octxiliary wrappers, letting researchers keep a high-level scientific language without losing access to the Go ecosystem.

The language includes first-class scientific features that are already represented in the repository's contracts and libraries: SI units, xUnit-style testing, arrays/vectors/matrices, native Einstein tensor notation, Octomata flow/state machines, utility scoring, fallible functions, package sync, optional `lock.octagon` reproducibility, and explicit native wrapper builds.

## Why Oct?

Oct is for research code that has outgrown throwaway scripts but still needs to stay close to the scientist's model of the problem.

- **Reproducibility by default:** tests, artifacts, package manifests, and optional lockfiles are part of the normal workflow.
- **Scientific language surface:** units, tensors, arrays, matrices, fallible functions, and experiment artifacts are language/toolchain concerns rather than notebook conventions.
- **Native distribution path:** the current implementation compiles through Go, so the compiled path can produce ordinary native binaries.
- **Explicit integration:** Octxiliary sidecars expose Go libraries through manifest-declared wrappers instead of hidden ambient bindings.
- **Agent-friendly workflow:** an LLM can create Oct experiments, run tests, sync packages, generate artifacts, and return reproducible code instead of a fragile transcript.

## Example
```oct
package ReadmeDemo

// Oct enforces physical units at compile time.
// Wrong units are a type error — not a runtime surprise.

fn KineticEnergy(mass: Float<kg>, velocity: Float<m/s>) -> Float<kg*m^2/s^2> {
    return 0.5 * mass * velocity * velocity
    // kg * (m/s)^2 = kg*m^2/s^2  ✓  compiler verifies this
}

fn StiffnessForce(K: Matrix<Float<kg/s^2>>, u: Vector<Float<m>>) -> Vector<Float<kg*m/s^2>> {
    return K @ u
    // Matrix<Float<kg/s^2>> @ Vector<Float<m>> → Vector<Float<kg*m/s^2>>  ✓  Newton's law
}

// Errors are values. ? propagates. match handles locally.
fn AverageSpeed(distance: Float<m>, time: Float<s>) -> Float<m/s> ! Error {
    if time <= 0.0s {
        return error("time must be positive")
    }
    return distance / time
}

// State machines are a language primitive — explicit, named, typed.
// Python's async/await secretly compiles to one of these.
// Oct makes the states, transitions, and mutable board visible.
flow HeatReactor(target: Float<K>, initial: Float<K>) -> Float<K> {
    board {
        Temp:  Float<K>
        Ticks: Int
    }

    state Initialize {
        board.Temp  = initial
        board.Ticks = 0
        goto Heating
    }

    state Heating {
        board.Temp  = board.Temp + 0.5K
        board.Ticks = board.Ticks + 1
        when {
            case board.Temp >= target -> goto Done
            case board.Ticks > 1000  -> goto Done
            else                     -> goto Heating
        }
    }

    state Done { return board.Temp }
}

```

## Current status: v0.1 preview

Oct 0.1 is an early preview: real enough to run, test, package, and compile scientific programs, but still pre-1.0 and evolving.

Current milestone capabilities include:

- core Oct language/toolchain;
- interpreted and compiled execution paths;
- package manager MVP with local/Git source sync, transitive exact dependency graph sync, and optional project-root `lock.octagon`;
- source-controlled canonical first-party registry at `Registry/registry.oct`;
- manifest-declared wrapper lifecycle with Octxiliary sidecars and explicit `oct pkg build-wrappers --allow-native`;
- tests and CI coverage across core compiler/tooling paths.

The language definition lives in Oct source contracts under `Language/`. The Go implementation (`cmd/`, `internal/`) is the current implementation/backend for those contracts.

## Install

After the `v0.1.0` tag is published, install the Oct CLI with Go:

```sh
go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
```

For development from a checkout, use:

```sh
go run ./cmd/oct --help
go install ./cmd/oct
```

Optional sidecar command for compiled programs that use the current IO sidecar path:

```sh
go install github.com/yuechen-li-dev/oct/cmd/octxiliary-io@v0.1.0
```

Ensure your Go bin directory is on `PATH` (commonly `$(go env GOPATH)/bin` or your configured `GOBIN`), then verify:

```sh
oct --help
oct version
```

Release builds can inject a version string with:

```sh
go build -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=0.1.0" ./cmd/oct
```

## Quick start

Create and test a small library package:

```sh
oct new library HelloScience
cd HelloScience
oct test .
```

The generated library contains an `Identity` function and an xUnit-style `[Fact]` test. Replace those with your package code as the experiment grows. For an existing directory that already contains Oct files, run `oct init experiment`, `oct init library`, `oct init application`, or `oct init wrapper-library` from that directory to add only `manifest.oct`; `oct init` refuses to overwrite an existing manifest.

From a repository checkout without installing first, the same flow is:

```sh
go run ./cmd/oct new library HelloScience
cd HelloScience
go run ../cmd/oct test .
```

## Package manager / canonical registry

Oct 0.1 includes a package manager MVP. The canonical first-party registry is source-controlled at:

```text
Registry/registry.oct
```

PM7 is intentionally local/source-controlled, not hosted. When using an installed `oct` outside this repository, point a project at a local checkout of the Oct repository:

```sh
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
oct test .
```

`Mathematics` is the canonical math package name. There is no `Math` alias in the canonical registry.

Optional lockfile workflow:

```sh
oct pkg lock
oct pkg sync --locked
```

Current package-manager boundaries for v0.1:

- registry entries are exact-version source entries;
- hosted registry, publishing, auth, signing, `.octpkg` artifacts, semver ranges, `latest`, and solver/backtracking behavior are not implemented;
- `lock.octagon` records the resolved graph but does not yet provide package tree digest or artifact integrity;
- wrapper package sync copies source and manifest metadata only; it does not build native sidecars.

## Wrapper / Octxiliary note

Octxiliary is the explicit sidecar bridge for exposing Go libraries to Oct. Wrapper packages declare sidecars in `manifest.oct`; `oct pkg wrappers` inspects that metadata without building or running native code.

Native sidecars are built only when requested explicitly:

```sh
oct pkg build-wrappers --allow-native
```

Built sidecars currently require `OCT_WRAPPER_PATH` or an existing sibling-discovery location at runtime. Package sync does not build sidecars, fetch arbitrary native dependencies, or run wrapper code.

## AI-assisted virtual laboratory note

Oct is designed to work well in agentic coding environments such as Codex Cloud or Claude Code. An LLM can write an experiment, run `oct test`, generate artifacts, sync exact package dependencies, and return a repository state that another user can reproduce locally.

This is a design goal, not a claim that every scientific workflow is complete in v0.1.

## Agent workflow

For repository work, the semantic authority is the CLI:

```sh
oct test <file-or-root> --execution auto --json
oct artifact <file-or-root> --execution interpreted --json
```

`oct test` reports compiled cases and any explicit interpreted fallbacks.
`oct artifact` is a separate lane and reports interpreted artifact paths,
types, sizes, and SHA-256 hashes. Local coding agents should edit and inspect
the repository directly; the bounded MCP server is for hosted virtual
workspaces, not a replacement filesystem or shell.

Artifacts that explicitly request one manifest wrapper operation through a
typed Concept provider may be invoked with
`--grant-native Package:Wrapper:Operation`. The request is descriptive, the
host grant is authoritative, and the broker checks it at dispatch. Sidecars
must already be built and remain trusted unsandboxed native processes.

Choose `oct new library Name` for stable reusable code. Choose
`oct new experiment Name` for a milestone-driven investigation: it creates
`REPORT.md` and M0, while root `oct test` / `oct artifact` run every canonical
`M<number>[letter]` milestone. See the bundled `oct-experiments` skill for the
focused-milestone then root-evidence loop.

## Oct 1.0 installation

Release archives, checksum verification, prerequisites, native build, test,
formatting, upgrade, and uninstall instructions are in
[`docs/releases/INSTALL_1_0.md`](docs/releases/INSTALL_1_0.md). An artifact
requires the Go toolchain declared by its bundled compiler runtime for `oct
build`; a repository checkout is not required.

## Stability notice / pre-1.0 warning

Oct 0.1 is a preview release. Language syntax, Go APIs, package registry format, standard-library APIs, wrapper metadata, and compiled-backend support may change before 1.0. Performance is not final, and no production-readiness promise is made for this prerelease.

## Development/test commands

Useful commands from the repository root:

```sh
go test ./pkg/octxiliary ./internal/octxiliary
go test ./internal/pkgmgr ./internal/project
go test ./cmd/oct -run 'Version|Help|Pkg|Registry|Lock|New|Init|Wrappers|BuildWrappers'
go test ./internal/... ./cmd/oct
go test -count=1 -parallel 8 ./...
go test -count=1 -parallel 8 -tags=integration ./...
go run ./tools/build_sidecars --out dist/sidecars
OCT_SLOW_TESTS=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" go test -count=1 -parallel 8 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
go run ./cmd/oct --help
go run ./cmd/oct pkg --help
go run ./cmd/oct version
```

Default `go test ./...` is the fast lane and skips sidecar-heavy Octxiliary wrapper tests. Build sidecars and set `OCT_SLOW_TESTS=1` when wrapper/octxiliary code changed, before release, or when that lane is explicitly requested.

On PowerShell, use the same sidecar build command and set the wrapper path with:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -parallel 8 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

For more details, start with:

- [`docs/science/README.md`](docs/science/README.md) — task- and discipline-oriented map of the scientific libraries;
- `docs/ARCHITECTURE.md` — architecture and execution model;
- `docs/CLI.md` — CLI quick reference;
- `docs/COMPILED_SUPPORT.md` — compiled-backend status;
- `docs/releases/OCT_1_0_CONTRACT.md` — proposed 1.0 stable-surface and compatibility contract;
- `docs/releases/OCT_1_0_READINESS.md` — RC1 evidence, discrepancies, and blocker ledger;
- `docs/releases/OCT_1_0_RELEASE_PLAN.md` — RC2/GA gates and non-goals;
- `docs/releases/OCT_1_0_SURFACE_MANIFEST.md` — authoritative stable and experimental API boundary;
- `Language/reference/` — canonical language/reference corpus;
- `docs/internal/canonical_registry_pm7.md` — canonical registry PM7 notes.
