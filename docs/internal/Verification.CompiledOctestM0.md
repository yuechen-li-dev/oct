# Compiled Octest M0 Verification (2026-05-20)

## Commands executed
- `go test ./internal/... ./cmd/oct -run 'Compiled|Octest|Benchmark|Execution|Suite|Theory|InlineData|CycleTime' -count=1` (pass)
- `go run ./cmd/oct test Language/Functions/Calls --execution auto|compiled|interpreted` (all pass; compiled/fallback counters 0/0)
- `go run ./cmd/oct test Libraries/String --execution auto|compiled|interpreted`
- `go run ./cmd/oct test Libraries/Markdown --execution auto|compiled|interpreted`
- `go run ./cmd/oct test Libraries/IO --execution auto|compiled|interpreted`
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --execution auto` (timed out after 240s while inside M2b)
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2 --execution auto` (pass)
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2b --execution auto` (timed out after 240s)
- `go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M2 --suite Experiments.FmBrownNoiseKalman.M2b --execution compiled` (fails clearly)
- `go test ./cmd/oct -run 'FmBrownNoiseKalman|M2b|Compiled|Octest|Benchmark|Suite|Markdown|Artifact|Namespace|Csv|Json|String|Record|Field' -count=1` (pass)
- `go run ./cmd/oct test Language/Functions/Calls` (default mode check)
- `go run ./cmd/oct test Language/Functions/Calls --execution banana` (invalid mode diagnostic)

## CLI behavior
- Accepted values observed: `auto`, `compiled`, `interpreted`.
- Default (no `--execution`) behaves as `auto` (same output/summary shape and success as explicit auto on same target).
- Invalid mode diagnostic: `invalid test execution mode "banana" (expected auto|compiled|interpreted)`.

## Coverage by suite
- Language/Functions/Calls
  - auto: pass, `compiled: 0`, `interpreted fallback: 0`
  - compiled: pass, `compiled: 0`, `interpreted fallback: 0`
  - interpreted: pass, `compiled: 0`, `interpreted fallback: 0`
  - note: this directory is `.octfail` heavy and does not exercise compiled [Fact]/[Theory] positive execution.

- Libraries/String
  - auto: pass, `compiled: 0`, `interpreted fallback: 5`
  - compiled: fail all 5, clear message: go-generated symbol missing, e.g. `undefined: fn_String_StringCoreBasicsNamespaced`
  - interpreted: pass all 5

- Libraries/Markdown
  - auto: pass, `compiled: 0`, `interpreted fallback: 4`
  - compiled: fail all 4 with clear blockers:
    - Artifact test: undefined generated symbol (`fn_Artifact_ArtifactPackageCompiles`)
    - Markdown tests: `package manifest missing`
  - interpreted: pass all 4

- Libraries/IO
  - auto: pass, `compiled: 0`, `interpreted fallback: 35`
  - compiled: fail all 35 with repeated clear builtin blocker:
    - `function IO.Read: compiled mode does not yet support builtin CsvRead`
  - interpreted: pass all 35

## M2/M2b status
- `Experiments/FmBrownNoiseKalman/M2 --execution auto`
  - timed out at 240s while in M2b path; observed fallback diagnostics before timeout.
  - observed blockers:
    - `function main: expression statement must not be fallible; handle it with '?', '!', or match`
    - `function FmBrownNoiseKalman.M2BuildCleanMessage: compiled mode does not yet support builtin FloorToInt`

- `--suite Experiments.FmBrownNoiseKalman.M2 --execution auto`
  - pass, `compiled: 0`, `interpreted fallback: 3`

- `--suite Experiments.FmBrownNoiseKalman.M2b --execution auto`
  - timed out at 240s after first fallback message; suggests heavy interpreted runtime and no evidence of compiled execution.

- `--suite Experiments.FmBrownNoiseKalman.M2b --execution compiled`
  - fail all 4, clear blockers:
    - fallible expression statement in `main`
    - unsupported builtin `FloorToInt` via `M2BuildCleanMessage`

Conclusion: M2/M2b are currently not compiling under compiled-octest M0 in this tree; performance concerns are very likely still interpreted-path dominated.

## Wrapper bridge coverage snapshot
- String.Join / Trim / ReplaceAll / SplitLines: currently not compiling in `Libraries/String` test lane due to generated symbol resolution failures; auto falls back.
- Markdown.Report / Table / KeyValueTable / Callout: not compiling in `Libraries/Markdown`; blockers include missing package manifest and artifact symbol generation.
- IO.WriteLines / ReadLines / WriteText / ReadText: in practice fall back in `Libraries/IO` because suite trips `CsvRead` unsupported builtin in compiled mode.
- Csv.ReadRows / ReadMatrix / Write* : not compiled; explicit blocker `builtin CsvRead` unsupported.
- Json.Save / Json.Load path in IO wrappers: currently falls back in auto for same `CsvRead` blocker when running `Libraries/IO` aggregate.

## Benchmark/compiled regression check
- `go test` focused compiled/octest/benchmark patterns passed for `./internal/...` and `./cmd/oct`.
- Additional focused `go test ./cmd/oct -run ...` (including benchmark/compiled/FM related names) passed.

## Notes on semantics checks
- Go test filters covering `Suite|Theory|InlineData|CycleTime|Compiled|Execution|Octest` passed, indicating no immediate regression in unit/integration expectations.
- This pass did not add new fixtures; evidence comes from existing test corpus + CLI behavior.
