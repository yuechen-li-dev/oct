# ARTIFACTS-M0 — Build-Time Artifact Evaluation

## Verdict

`[Artifact]` is an explicit compiler/build phase backed by the existing typed
interpreter. Artifact generation no longer has an independent compiled-runner
engine and never needs Go generation, Go compilation, or application startup.
Compatible source syntax is retained and awkward runtime-era directory setup is
migrated lazily.

## System before M0

1. The `.octest` parser stored `[Artifact]` as
   `ast.FunctionDecl.IsArtifact`. It rejected parameters, non-`Void` base return
   types, and conflicting Fact/Theory/Benchmark attributes. `Void ! Error` was
   already accepted.
2. `project.LoadForTest` loaded `.oct` and `.octest` packages and
   `typecheck.CheckProgram` checked the entire graph. Only then did
   `internal/tester/artifact.go` scan `IsArtifact`, apply selected-file and
   entry-package/`--all-packages` scope, and sort package/file/function.
3. `oct artifact` selected either `interpreted` (default) or `compiled`.
4. Interpreted mode called `interpret.ExecuteFunctionWithArgsAndOptions` on the
   typed program.
5. Compiled mode generated a temporary `.octest` runner that directly called
   the artifact, generated Go through `CompileForTestWithSelectedFilesInPackage`,
   compiled a host executable, and launched it. Wrapper consumers could also
   require sidecars. Thus this option did require backend generation, host
   compilation, and runtime execution.
6. Normal application lowering already skipped `IsArtifact`, but the selected
   reachable compiled runner deliberately pulled its target into generated
   runtime code.
7. `Artifact.Write*` names were aliases over ordinary File/Csv/Json/Octagon
   runtime functions. The library package was a marker, not the authority.
   Writes occurred immediately; a recorder observed paths afterward.
8. Paths were caller-controlled. There was no artifact output-root contract,
   traversal/absolute-path check, duplicate check, staging transaction, or
   write-if-changed behavior. Experiment orchestration could additionally apply
   a global output prefix.
9. `Artifact.Write*` could be called in ordinary runtime code. Existing
   generators also used `IO.MakeAll`, global `WriteOctagon`, and occasional
   read-after-write verification because artifact execution looked like a
   runtime lane.
10. Reusable pieces were the parser flag/signature validation, package loading,
    type checking, deterministic discovery, typed interpreter, progress events,
    Octagon serialization, and command partitioning.

## M0 semantics and orchestration

The lifecycle is:

```text
parse
→ bind/resolve packages
→ type-check the complete selected program
→ discover [Artifact] entries
→ evaluate entries with the typed interpreter and artifact capability
→ collect staged outputs
→ publish outputs deterministically
→ optionally run an independently requested ordinary backend workflow
```

An entry must be in `.octest`, have no parameters, return `Void` or
`Void ! Error`, and not carry another lane attribute. Discovery occurs only
after successful checking, in package/file/function order. The entry package is
the default scope; `--all-packages` opts into imported-package entries. An entry
does not run on import or application startup, is excluded from ordinary MIR/Go
lowering, and direct Oct calls to it are diagnosed.

The explicit route is:

```text
oct artifact <file-or-root> [--output-root <directory>] [--all-packages] [--json]
```

The output root defaults to the working directory. The actual execution mode is
`build-time-interpreted`. `--execution interpreted` remains accepted.
`--execution compiled` is a temporary compatibility spelling that delegates to
the same evaluator and reports that no backend was generated or compiled. It is
not a second engine.

Human output reports `RUN`, `PASS`/`FAIL`, `PRODUCED`/`UNCHANGED`, and totals.
JSON reports requested and actual execution, source provenance, relative path,
status, MIME type, bytes, SHA-256, timing, diagnostics, and exit status.

## Capability, publication, and determinism boundary

`Artifact.WriteText`, `WriteLines`, `WriteMarkdown`, `WriteCsv`, `WriteJson`,
and `WriteOctagon` are compiler-owned during artifact evaluation. Calls outside
the phase fail. The legacy global `WriteOctagon` spelling delegates to this same
capability during the phase. `Directory.Make`/`MakeAll` is a confined temporary
compatibility operation so existing generators need not pre-create output
directories.

Each output path must be non-empty, relative, volume-free, and lexically inside
the output root. Existing symbolic-link destinations are rejected. Duplicate
paths are rejected case-insensitively with both declaring functions and source
paths. Entries evaluate against a private staging directory; read-after-write is
limited to a path already declared in that phase. Evaluation failure publishes
nothing. Successful outputs are sorted by path, equal bytes are left untouched,
and changed files are written to same-directory temporary files before rename.
Each file replacement is atomic where the host filesystem supports rename
replacement. M0 does not roll back an earlier file if a later publication rename
fails; adding a portable directory-wide transaction would be disproportionate
to the current filesystem layer.

No ambient network, process, environment, clock, crypto-random, unrestricted
filesystem, or generic wrapper/sidecar effect is granted. Seeded Oct random
functions remain available because their state is explicit and deterministic.
Progress/checkpoint output is diagnostic only and ordered with entry execution.

Concepts M2 preserves this default. An entry may now opt into one exact,
manifest-declared wrapper operation with `[Artifact(RequestProvider)]` plus a
matching host `--grant-native Package:Wrapper:Operation`. The compiler-owned
grant is checked at the existing generic dispatcher; request records are not
authority. See `CONCEPTS_M2.md`. Authorized sidecars remain trusted,
unsandboxed native processes and are outside the M0 confinement guarantee.

## Concepts, models, and staged generation

Artifact code is ordinary typed Oct. Alias, refined, record, and named concepts;
records; arrays; enums and exhaustive `match`; units; pure helpers; `?`, `!`,
and fallible matching all use the existing interpreter implementation. Concepts
are useful for composing a model but are not mandatory, and M0 adds no template
language.

OctGen is not `[Artifact]`: it evaluates a typed `Generate()` model, decodes it
in Go, renders Go, and feeds that result to a later explicit build stage. Its
staging and atomic-write ideas informed M0, but its host renderer remains a
separate generator consumer rather than a second artifact engine.

Artifact publication starts only after the whole Oct program has been bound and
checked. Published output is never rediscovered as source in that invocation.
The audit found no `[Artifact]` output used as input to the same semantic
compilation. Later-stage Go/TypeScript/HTML/JSON/Markdown/Octagon/report outputs
are supported; a future Oct-source generator must use an explicit earlier build
graph stage.

## Lazy migration audit

The pre-M0 tree contained 243 declarations in 72 entry files, 116
`Artifact.Write*` calls, 219 legacy global `WriteOctagon` calls, and 100 source
files containing an entry or an artifact-write helper. The table groups only
identical path families; every pre-M0 declaration file is covered by an explicit
location or milestone list.

Classification: 1 already compatible; 2 small delegation/shim; 3 awkward old
runtime assumption; 4 unrestricted ambient effect; 5 same-compilation input;
6 fixture/unused; 7 not actually `[Artifact]` generation.

| Location (all `.octest` entry files unless noted) | Declarations | Outputs / effects | Before M0 engine and backend | Class | M0 action |
|---|---:|---|---|---:|---|
| `docs/build-week/recording/fixtures/JudgeDemo/JudgeDemo.octest` | 1 | JSON; redundant `IO.MakeAll` | interpreter or compiled runner; backend only when compiled selected | 3 | Migrated: removed runtime directory setup; retained entry/API |
| `Experiments/FmBrownNoiseKalman/{M0,M1,M2,M3,M4,M4b,M5,M6}` | 12 | Octagon/Markdown/CSV/JSON; seeded random; some helper calls | interpreter or compiled runner | 1/2 | Retained; global `WriteOctagon` and helper writes delegate to one capability |
| `Experiments/OctErgonomicsLab/{M0,M1}` | 2 | Octagon/Markdown/CSV/JSON; helper, mkdir, staged verification | interpreter or compiled runner | 2 | Retained; confined mkdir and declared-output reads adapted |
| `Experiments/PrometheusFftAlgorithmLab/M1` | 4 | Octagon/Markdown; mkdir and read-after-write | interpreter or compiled runner | 2 | Retained statically; same confined compatibility path; workload not run |
| `Experiments/PrometheusMeasurementFilteringLab/{M1,M2,M3,M4}` | 19 | Octagon; explicit seeded random | interpreter or compiled runner | 2 | Retained; legacy Octagon sink delegates; workload not run |
| `Experiments/PrometheusNumericalHeterogeneityLab/{M0,M1}` | 2 | Octagon/Markdown/CSV/JSON via helpers | interpreter or compiled runner | 2 | Retained; helper writes delegate; workload not run |
| `Experiments/PrometheusPredictiveLeaseAheadLab/{M1,M6a}` | 7 | Octagon | interpreter or compiled runner | 2 | Retained through legacy Octagon delegation; workload not run |
| `Experiments/PrometheusSgemmAlgorithmLab/{M4,M4d,M12,M13,M14,M15,M17,M18,M19,M22,M23,M25,M26,M27,M28,M30,M32,M33,M34,M36,M37,M38,M39,M40,M41,M44,M45,M46,M47,M48,M49,M50,M51,M52}` | 177 | Octagon; global legacy sink | interpreter or compiled runner | 2 | Retained through same capability; no Prometheus workload run |
| `Experiments/PrometheusShadowAuthorityRakeLab/{M1,M2,M3,M4,M5}` | 5 | Octagon/Markdown/CSV/JSON/findings; staged verification | interpreter or compiled runner | 2 | Retained; declared-output reads adapted; workload not run |
| `Experiments/ZImageTurboMainTransformer0/M0` | 1 | JSON | interpreter or compiled runner | 1 | Retained unchanged |
| `Experiments/ZImageTurboNoiseRefiner0/{M0,M2,M3,M4,M5,M6,M7}` | 7 | JSON; redundant `IO.MakeAll` | interpreter or compiled runner | 2 | Retained through confined mkdir compatibility; model workloads not run |
| `Libraries/ArtifactUsage/Artifact.Usage.octest` | 1 | text/lines/Markdown/CSV/JSON/Octagon | interpreter or compiled runner | 1/3 | Entry retained; ordinary Fact migrated from `Artifact.Write*` to fallible Csv/Json runtime APIs |
| `Libraries/Markdown/Markdown.Core.octest` | 2 | Markdown/JSON | interpreter or compiled runner | 1 | Retained unchanged |
| `Libraries/Artifact/Artifact.Core.octest` | 1 | semantic smoke, no file | interpreter or compiled runner | 6 | Retained as compatibility fixture |
| `Language/Testing/CompiledSelectedReachable/unreachable/suite.octest` | 1 | reachability fixture | test/compiler fixture | 6 | Retained; ordinary backend exclusion remains tested |
| `testdata/m24h/valid/Fixtures/fixtures.octest` | 1 | stdout fixture | interpreter or compiled runner | 6 | Retained; verified with new evaluator |
| OctGen (`internal/octgen`, `experimental/octgen`, `tools/octgen`) | 0 | typed model → generated Go in later stage | interpreter model plus host renderer | 7 | Deferred as intentionally separate staged generator; no semantic duplication |
| Make plan snapshots, SDSL-V shader/header commands, formatter outputs | 0 | tool-owned terminal/later-stage artifacts | their existing tool-specific paths | 7 | Not reclassified as `[Artifact]` |

No supported consumer was classified 4 or 5. Code that attempts those patterns
now gets a focused diagnostic. The Prometheus and model entries were statically
audited but intentionally not executed under the no-workload boundary.

## Diagnostics

Existing parser diagnostics continue to cover invalid targets, conflicting
attributes, parameters, and return signatures. M0 adds or strengthens:

- direct calls to an artifact entry point;
- `Artifact.Write*` outside artifact evaluation;
- absolute, empty, escaping, symbolic-link, duplicate, and conflicting paths;
- source-attributed staged-output failures;
- ordinary file writes or undeclared reads during the phase;
- unsupported clock, crypto-random, filesystem, archive/image/PDF/plot effects;
- generic wrapper/sidecar operations reached during artifact evaluation;
- propagated fallible evaluation failures without success publication.

Direct artifact calls are prohibited, so artifact-entry cycles are impossible.
Backend-only operations are stopped at the generic-wrapper boundary rather than
leaking sidecar or host-language errors.

## Compatibility and intentionally retained seams

| Seam | Status | Reason / removal direction |
|---|---|---|
| `import Artifact` | Retained, optional | Existing sources remain valid; namespace is compiler-owned |
| `--execution interpreted` | Retained | Harmless explicit spelling |
| `--execution compiled` | Delegating compatibility alias | Prevents breakage while eliminating the independent backend engine; deprecate after callers update |
| global `WriteOctagon` in artifact phase | Delegating compatibility alias | 219 existing calls; maps exactly to the same staged output semantics |
| `Directory.Make*` in artifact phase | Narrow temporary shim | Removes redundant runtime-era mkdir without requiring repository-wide churn |
| read-after-write of declared output | Narrow compatibility | Supports existing verification without granting ambient reads; consumers should prefer host/report verification over time |

## Tests and evidence

The canonical contract fixture is
`Language/Tooling/Artifacts/valid/build_time_artifact_evaluation.octest`. It
uses M0 alias concepts, M1 refined and record concepts, arrays, records, enums,
units, functions, exhaustive matching, `?`, handled fallibility, and every
currently supported structured/text `Artifact.Write*` operation. There is no
binary write API today, so no new binary API was invented.

Negative Language fixtures cover traversal, absolute paths, duplicate output,
ambient writes, failed fallible evaluation, direct entry calls, and phase misuse.
Go tests orchestrate these files rather than duplicating language semantics.
Focused tests also prove sorted discovery, changed and unchanged publication,
modification-time stability, no partial publication after evaluation failure,
the compiled-option delegation, and ordinary MIR/generated-Go exclusion.

## Measurements

Pre-M0 JudgeDemo measurements on this workstation were approximately 298 ms
for `go run ./cmd/oct artifact` interpreted and 227 ms for the compiled attempt;
the compiled attempt failed after entering backend machinery. The M0 JSON report
measured 20 ms inside the already-built command path and produced the same
62-byte JSON output; the second run reported `UNCHANGED`. These timings are
diagnostic, not a benchmark, because `go run` build-cache state differs.

For every `--execution compiled` artifact invocation, M0 eliminates one Go
generation, one Go compilation, one temporary artifact runner, one host
executable, and one host process. Artifact-specific compiled-runner code was
removed rather than hidden below the new command. One evaluator and two small
source compatibility adaptations remain.

Verification on 2026-07-22:

- focused parser/project/typechecker/interpreter/tester/CLI packages: pass;
- artifact CLI integration suite: pass;
- ordinary MIR/generated-Go exclusion integration test: pass;
- Concepts M0 valid `1/1` and invalid `12/12`, Concepts M1 valid `2/2`
  and invalid `10/10`, Units M1 valid `1/1` and invalid `5/5`: pass;
- those four valid Concepts/Units cases in compiled mode: `4` compiled,
  `0` fallback, all pass;
- Artifact `1/1`, Markdown `6/6`, and migrated ArtifactUsage `2/2`
  interpreted library contracts: pass;
- ArtifactUsage artifact command: one entry and six outputs pass;
- OctGen package tests, both `octgen check` commands, compiled model lock,
  and formatter package: pass;
- final `go test ./...`: pass in 33.4 s (earlier cold run 50.2 s); no GPU/model workload was invoked;
- `git diff --check`: pass.

The focused M0 artifact generated six deterministic files (301 total bytes).
Its second run reported all six unchanged and preserved modification times. The
migrated JudgeDemo generated the same 62-byte JSON with SHA-256
`4efc9d55ed0eac0e8401f92f1d6b320e7e4c4b7e09778f1c5b7f9917c484209c`.
Generated OctGen files remained unchanged.

Lines changed (added/deleted, including new files): parser `0/0`;
binder/typechecker `9/1`; interpreter/runtime `194/26`; compiler/backend
production `0/0`; backend exclusion test `27/0`; CLI/MCP production `22/4`;
artifact orchestration `281/140`; Go tests `176/22`; Language contracts
`106/0`; migrated consumer Oct `13/13`; documentation `332/34`; builtin
namespace classification `1/1`. Total: `1,161/241` across 43 files.

## Limitations and next milestone

- Attributes remain restricted to `.octest`; moving build declarations into
  ordinary package source is not part of M0.
- Publication is collect-before-write and per-file atomic, not a portable
  directory-wide rollback transaction.
- The M0 effect boundary is an explicit blocklist at current interpreter
  builtin/wrapper seams, not a general language capability type system.
- There is no binary `Artifact.WriteBytes` API.
- Existing generated-output assertions that run later as Facts remain separate
  staged workflows; `oct artifact` does not automatically run those tests.

The recommended next milestone is ARTIFACTS-M1: remove the compiled spelling
after downstream migration, replace remaining mkdir/read-back shims in touched
consumers, add an explicit binary output API only if a real consumer requires
it, and decide whether a journaled multi-file publication transaction is worth
its cross-platform complexity.

Rejected alternatives were preserving compiled and interpreted artifact
engines, running artifacts on import/startup, embedding a second VM, creating a
template language, granting ordinary runtime I/O as artifact authority, and
resuming binding after generation.
