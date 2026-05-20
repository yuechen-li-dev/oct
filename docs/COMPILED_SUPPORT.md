# Compiled Octest Support Tracker (M0b/M2b) — 2026-05-20 (Assertions repair pass)

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
| Explicit file target isolation | Supported | CLI accepts file target + execution mode. |
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
| `String.*` wrappers | Supported | Not supported enough | Falls back (e.g., String auto compiled 0 fallback 5) | Symbol binding / wrapper compiled coverage | P1 |
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
  - assertion lowering is no longer the blocker; next blocker remains unsupported builtin `StringByteLength`.
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

## 7) 2026-05-20 Compiled String builtins M0 attempt (this pass)
- Added compiled-lowering coverage scaffolding for String builtins in `internal/build/compiler.go`:
  - return-type routing and Go emission paths for `StringByteLength`, `StringRuneCount`, `StringJoin`, `StringReplaceAll`, `StringContains`, `StringStartsWith`, `StringEndsWith`, `StringTrim`, `StringSplitLines`, `StringEscapeJSON`, `StringQuoteJSON`, plus namespaced spellings.
  - emitted helper bridges for compiled path: `__octStringSplitLines` and `__octStringEscapeJSON`.
- Added focused fixture:
  - `Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest`.

### Measured outcome (normalization repair)
- Exact rejection site was `internal/build/compiler.go` `resolveCall` (identifier builtin path): only `Random.*` builtins were normalized/typed there, while `StringByteLength` hit the default unsupported-builtin branch before emission.
- Fix applied:
  - added canonical builtin identity helper `canonicalCompiledBuiltinName(...)`.
  - compiled support/type routing now canonicalizes names before `compiledBuiltinReturnType(...)`.
  - generated-Go builtin emission switch canonicalizes callee identity before dispatch.
  - `resolveCall` now returns typed builtin entries for the compiled String builtin set and canonicalizes namespaced String aliases (`String.ByteLength` -> `StringByteLength`, etc.).
- `go run ./cmd/oct test Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest --execution compiled`
  - now progresses past the old `StringByteLength` unsupported-builtin gate.
  - current blocker is later Go build failure due to missing generated imports (`strings`, `strconv`, `utf8`) in emitted file.
- `go run ./cmd/oct test Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest --execution auto`
  - falls back interpreted and passes (`compiled: 0 interpreted fallback: 4`), with compiled unsupported reason now `duplicate declaration 'main' in package 'String'`.
- `go run ./cmd/oct test Libraries/String --execution compiled`
  - no longer blocked by `StringByteLength` unsupported; now fails at generated Go compile with undefined `strings/strconv/utf8` symbols.

### Next blocker (post-dispatch fix)
- Generated-Go import management is incomplete for newly wired String builtin emission paths (`strings`, `strconv`, `unicode/utf8` helpers referenced but not imported in generated output).
