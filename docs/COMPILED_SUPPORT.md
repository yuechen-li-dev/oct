# Compiled Octest Support Tracker (M0b/M2b) — 2026-05-20

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
| `[Theory]` + `[InlineData]` | Partial | Harness runs, but compiled symbol-binding failures observed in verification (`undefined: fn_String_StringCoreBasicsNamespaced`). |
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
| Functions/imports/namespaces | Partial | Works broadly, but symbol binding failures still present for some generated test harness names. |
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
| Numeric helpers (`FloorToInt`) | Supported | **Currently blocked in M2/M2b path** | Causes compiled failure, no fallback in `--execution compiled` | unresolved compiled builtin handling in this path | **P0** |
| Test assertion helpers | Supported | Partial | mixed; depends on called wrappers/builtins | depends on underlying builtin coverage | P1 |

## 5) Current command evidence (2026-05-20)
- `go run ./cmd/oct test Language/Functions/Calls --execution compiled`
  - pass; this target currently contains invalid `.octfail` coverage only in this run.
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2b --execution compiled`
  - fails: `FloorToInt` unsupported in `M2BuildCleanMessage`; also fallible expression statement in artifact test.
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2 --execution compiled`
  - fails: same `FloorToInt` blocker + fallible expression statement.
- Existing verification (prior pass):
  - Libraries/String auto: compiled 0 fallback 5
  - Libraries/Markdown auto: compiled 0 fallback 4
  - Libraries/IO auto: compiled 0 fallback 35
  - M2 suite auto: compiled 0 fallback 3
  - M2b auto: timeout / no compiled evidence

## 6) Priority backlog
### P0 (current critical path)
1. Fix generated symbol binding for selected compiled test functions (namespaced harness linkage).
2. Resolve compiled support path for `FloorToInt` used in M2/M2b numerical flow.
3. Resolve M2 fallible expression-statement failures (`?`, `!`, or `match` required).
4. Make M2b suite compile/run enough to avoid interpreted timeout behavior in auto mode.

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
