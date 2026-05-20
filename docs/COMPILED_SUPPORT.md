# Compiled Octest Support Tracker (M0b/M2b) — 2026-05-20 (FmBrownNoiseKalman M2/M2b re-measure + artifact-lane split)

## 1) Overview
- Compiled pipeline: **Oct -> MIR -> generated Go -> .octbin**.
- Used by benchmark-oriented compiled tests and now exposed for Octest via:
  - `oct test --execution auto|compiled|interpreted`
- `auto` prefers compiled and falls back to interpreted.
- `compiled` requires compiled support and fails fast with explicit diagnostics.
- `interpreted` stays on the legacy interpreter path.

## 2) Test execution support status
| Surface | Status | Notes / evidence |
|---|---|---|
| `[Fact]` | Partial | Control-plane works; compiled failures still common on unsupported builtins / lowering. |
| `[Theory]` + `[InlineData]` | Partial | Harness now binds to emitted test symbols (no `undefined fn_String_*`), but wrapper/assert coverage still blocks many suites. |
| `[CycleTime]` | Partial | CLI mode plumbing exists; no claim of broad compiled semantic parity yet. |
| `[Suite]` | Partial | Suite selection works; compiled path still blocked in M2/M2b cases. |
| Explicit file target isolation | Supported | Compiled explicit `.octest` target now isolates selection to the chosen file in its package (imports still load normally). |
| Directory/package targets | Supported | `oct test <dir> --execution ...` works. |
| Imported test isolation | Partial | Works in many cases, but generated symbol binding is still a blocker for some namespaced cases. |
| Auto fallback reporting | Supported | Summary prints compiled/fallback counts. |
| `[Artifact]` in compiled path | Unsupported/Partial | Artifact-heavy tests currently hit fallible-expression and builtin support gaps. |

## 3) Core language/lowering support (compiled)
| Feature | Status | Notes |
|---|---|---|
| Arithmetic/comparisons/if/loops | Supported (core) | Present in compiled benchmark/loop tests; still subject to unsupported builtin paths. |
| Arrays/records/field access/chained access | Partial | Core lowering exists; wrapper-heavy suites still fall back/fail. |
| Functions/imports/namespaces | Partial | Symbol binding bug fixed for compiled octest harness; remaining gaps are wrapper/assert builtin coverage. |
| Scientific float literals | Partial | Present in M2 code paths, but not fully isolated from other compiled blockers. |
| Fallible calls (`?`, `!`, `match`) | Partial | Language supports these; compiled tests still fail if expression statements are fallible. |
| Fallible expression statements | Unsupported by rule | Enforced diagnostic: `expression statement must not be fallible; handle it with '?', '!', or match`. |

## 4) Builtin / wrapper support snapshot
| Surface | Interpreted | Compiled | Auto behavior | Blocker | Priority |
|---|---|---|---|---|---|
| `String.*` wrappers | Supported | Supported for core fixture + Libraries/String | Compiles in auto with fallback 0 on verified targets | broader wrapper parity still tracked separately | P0 closed for missing-import blocker |
| `Markdown.*` wrappers | Supported | Not supported enough | Falls back (compiled 0 fallback 4) | Wrapper lowering/runtime gaps | P1 |
| `IO.*` wrappers | Supported | Not supported enough | Falls back (compiled 0 fallback 35) | CSV + IO wrapper gaps | P1 |
| `Csv.*` | Supported | Partial/unsupported | Often fallback/fail | `CsvRead` not compiled-supported in current path | P1 |
| `Json.*` | Supported | Partial/unsupported | likely fallback/fail in wrapper paths | missing compiled bridges | P1 |
| `Artifact.*` | Supported | Partial/unsupported | artifact suites not reliably compiled | fallible-expression + bridge gaps | P2 |
| Numeric helpers (`FloorToInt`) | Supported | Not observed in current M2 failure diagnostics | Current M2 blockers are fallible statement + Markdown wrapper | no new FloorToInt repro in this pass | P1 |
| Test assertion helpers (`Assert.True/False/Equal`) | Supported | Supported (core fixtures) | compiled valid fixtures run with compiled=3 fallback=0 | failing fixture exits nonzero with `assertion failed: ...`; diagnostics currently plain stderr text via `os.Exit(1)` | P0 closed |

## 5) Current command evidence (2026-05-20 (Assertions repair pass))
- `go run ./cmd/oct test Language/Functions/Calls --execution compiled`
  - pass; this target currently contains invalid `.octfail` coverage only in this run.
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2b --execution compiled`
  - fails with non-zero exit; first failure is fallible expression-statement in generated runner entry, plus `unknown function 'Markdown.Report'` on artifact path.
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2 --execution compiled`
  - fails with non-zero exit; blockers are fallible expression statement + `Markdown.Report` unsupported.
- `go run ./cmd/oct test Libraries/String --execution compiled`
  - now passes: compiled 5, interpreted fallback 0.
- Existing verification (prior pass):
  - Libraries/String auto: compiled 0 fallback 5
  - Libraries/Markdown auto: compiled 0 fallback 4
  - Libraries/IO auto: compiled 0 fallback 35
  - M2 suite auto: compiled 0 fallback 3
  - M2b auto: timeout / no compiled evidence

## 6) Priority backlog
### P0 (current critical path)
1. Resolve compiled support path for `FloorToInt` used in M2/M2b numerical flow.
2. Resolve M2 fallible expression-statement failures (`?`, `!`, or `match` required).
3. Make M2b suite compile/run enough to avoid interpreted timeout behavior in auto mode.

### P1
1. String wrappers.
2. Markdown pure wrappers.
3. IO text/line wrappers.
4. CSV read-row/matrix bridges as needed.
5. JSON load/save bridges as needed.

### P2
1. Artifact write support in compiled mode.
2. CSV table/row-major write bridges.
3. Broader wrapper parity hardening.

---
This tracker is descriptive (not aspirational): if auto reports fallback or compiled fails, compiled support remains partial/unsupported until measured evidence changes.

## 7) 2026-05-20 Compiled String builtins M0c (generated-Go import plumbing)
- Implemented centralized compiled-builtin import mapping in `internal/build/compiler.go` via `builtinImportDeps(...)`.
- Generated Go import-set population now derives String-related imports from builtin usage metadata instead of ad-hoc scattered checks.

### String builtin -> import mapping (compiled Go emission)
- `StringRuneCount` -> `unicode/utf8`
- `StringTrim` -> `strings`
- `StringReplaceAll` -> `strings`
- `StringContains` -> `strings`
- `StringStartsWith` -> `strings`
- `StringEndsWith` -> `strings`
- `StringSplitLines` -> `strings` (helper `__octStringSplitLines` uses `strings`)
- `StringEscapeJSON` -> `strconv` (helper `__octStringEscapeJSON` uses `strconv.Quote`)
- `StringQuoteJSON` -> `strconv`
- `StringJoin` -> `strings`
- `StringByteLength` -> no extra import (`len(s)`)

### Measured outcome
- `go run ./cmd/oct test Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest --execution compiled`
  - passes with `compiled: 4 interpreted fallback: 0`.
- `go run ./cmd/oct test Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest --execution auto`
  - passes with `compiled: 4 interpreted fallback: 0`.
- `go run ./cmd/oct test Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest --execution interpreted`
  - passes.
- `go run ./cmd/oct test Libraries/String --execution compiled`
  - passes with `compiled: 5 interpreted fallback: 0`.
- `go run ./cmd/oct test Libraries/String --execution auto`
  - passes with `compiled: 5 interpreted fallback: 0`.
- `go run ./cmd/oct test Libraries/String --execution interpreted`
  - passes.

### File-target/harness repair (M0d)
- `project.LoadForTest` file-path loading now applies `.octest` file-selection filtering only to the explicitly selected entry package file, preventing parse/selection widening to sibling `.octest` files in compiled file-target runs.


## 8) 2026-05-20 M2/M2b re-measure (post assertions/string/file-target repairs)
### Initial measurements (before this pass change)
- `M2 --execution compiled`: failed, compiled 0 / fallback 0; first blocker `This standalone expression cannot be ran` then `unknown function Markdown.Report`.
- `M2 --execution auto`: passed, compiled 0 / fallback 3; first fallback blocker `duplicate declaration 'main'` and `unknown function Markdown.Report`.
- `M2b --execution compiled`: failed, compiled 0 / fallback 0; first blocker `duplicate declaration 'main'`.
- `M2b --execution auto`: failed by cycle-time timeout on artifact row, compiled 0 / fallback 4; non-artifact rows fallback on `unknown function Markdown.Report`.

### Blocker classification
- Classified as **Test/artifact workload separation issue** (class 3), with wrapper/reporting coupling (class 2): `[Theory]` lane invoked artifact/report generation paths that require unsupported `Markdown.Report` compiled bridges and provoke timeout-heavy interpreted fallback.

### Smallest fix applied
- Moved `M2ArtifactFilesWrite` and `M2bArtifactsWrite` from `[Theory]` suite lanes into `[Artifact]`-only lanes (`Experiments.FmBrownNoiseKalman.M2.Artifacts` and `Experiments.FmBrownNoiseKalman.M2b.Artifacts`) to keep compiled `[Theory]` lane focused on numerical checks.
- No FM science, Kalman parameters, or sweep logic changed.

### Post-fix measurements
- `M2 --execution compiled`: still fails (compiled 0 / fallback 0) with runner/source-file and `Markdown.Report` blockers.
- `M2 --execution auto`: passes with compiled 0 / fallback 2.
- `M2b --execution compiled`: fails with compiled 0 / fallback 0 (`source file not found` + `Markdown.Report` blockers).
- `M2b --execution auto`: improved fallback count to compiled 0 / fallback 3, but still fails due `M2bSingleCaseFinite` cycle-time timeout (30s).

### Interpretation
- M2b remains interpreted-path dominated and still not converged for auto-mode performance due timeout under fallback execution.
- Remaining concrete next blocker for compiled parity is still wrapper/harness boundary around `Markdown.Report` reachability and intermittent runner source-file ownership (`zz_oct_test_runner_*.oct` not found / duplicate main).

### Inconsistency surfaced
- `Libraries/String --execution compiled` is green (`compiled 5 fallback 0`), but `Libraries/String --execution auto` in this pass fell back on duplicate `main` in package `String` (`compiled 0 fallback 5`). This indicates an auto-lane harness ownership inconsistency rather than String builtin lowering regression.

## 9) 2026-05-20 M0f test-lane cleanup attempt (M2/M2b)
- Added explicit M2b smoke helpers (`M2bSmokeGrid`, `M2bRunSmokeGrid`) so `[Theory]` tests can stay tiny (2 cases/mode) while artifact lane keeps the bounded full grid.
- Split M2b artifact construction into helper functions:
  - `M2bBuildCsvRows`
  - `M2bBuildJsonSummary`
  - `M2bBuildMarkdownReport`
  - `M2bBuildOctagonSummary`
- Updated M2b theory test to use smoke-only path (`M2bSmokeGridFinite`) and not call artifact builders directly.

### Re-measurement outcome after split
- `M2 --execution compiled`: still nonzero; compiled unsupported due `unknown function 'Markdown.Report'` during package compile.
- `M2 --execution auto`: still nonzero for same compile-time blocker.
- `M2b --execution compiled`: still nonzero for same compile-time blocker.
- `M2b --execution auto`: compiled 0 / interpreted fallback 3; one row passes, two rows timeout at 30s in interpreted fallback.

### Classification (current)
- Remaining blocker is **wrapper/report contamination at package compilation boundary** (not numeric lowering yet): compiled mode appears to typecheck/compile whole package and still sees artifact/report helpers even when theory lanes are smoke-only.
- Therefore, next focused task should isolate artifact/report functions into a separate package/file-ownership lane that is excluded from compiled theory package build, before pursuing numeric lowering fixes.

## 10) 2026-05-20 M0g selected-function lowering boundary repair
- Root cause confirmed in `internal/build/compiler.go`: `lowerProgram(...)` lowered almost every package function unconditionally (excluding only `[Artifact]`-tagged funcs and some test-file filters), so compiled octest runner builds still traversed unrelated package helpers.
- First unwanted inclusion for `M2b` compiled suite was `FmBrownNoiseKalman.M2ArtifactWriteAll`, which then reached `Markdown.Report` and failed before numeric/theory lowering could execute.

### Fix shipped
- Added `compileOptions{selectedReachableOnly:true}` for `CompileForTest(...)` only.
- Added selected reachability collection rooted from compiled runner entry (`main`/`Main`) and traversed function-call graph across local/imported package calls before lowering.
- Compiled test lowering now includes only reachable functions for compiled octest/benchmark runners; normal `Compile(...)` behavior is unchanged.

### Fixture evidence
- Added `Language/Testing/CompiledSelectedReachable/unreachable`:
  - `[Fact]` does not call `Markdown.Report`; unreachable artifact-only helper exists.
  - Expected compiled pass (unreachable unsupported path no longer blocks).
- Added `Language/Testing/CompiledSelectedReachable/reachable`:
  - `[Fact]` calls helper that calls `Markdown.Report`.
  - Expected compiled fail with explicit unsupported wrapper diagnostic.

### Re-measure (post-fix)
- `M2 --execution compiled`: no longer blocked by `Markdown.Report`; next blocker is `Random.RngSeed` using unsupported compiled builtin `Require`.
- `M2b --execution compiled`: no longer first-blocked by `Markdown.Report`; next blockers are `Require` and a generated-Go type mismatch in M2b grid lowering.
- `M2b --execution auto`: compiled unsupported reasons now report `Require`/type mismatch, then interpreted fallback where cycle-time timeout still occurs on two rows.

### Artifact lane status
- `oct artifact Experiments/FmBrownNoiseKalman/M2` still runs on artifact lane; this change does not require compiled artifact support and does not change interpreted artifact behavior.
