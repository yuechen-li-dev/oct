# Compiled scalar-loop performance reconnaissance

## Scope

This is a bounded measurement of one 20,000,000-iteration floating-point
accumulation loop. It is not a benchmark claim for Oct generally and is not a
performance gate.

The reproducible specimens are:

- `testdata/performance/scalar_loop_recon/main.oct`
- `testdata/performance/scalar_loop_recon/baseline.go`
- `go run ./tools/scalar_loop_recon`

The tool compiles both programs, warms each executable twice, takes seven
process-wall-time samples, verifies identical output, and reports medians plus
simple generated-code shape counts.

Pass `--out .tmp/scalar-loop/after` to retain the generated Go and both
executables for inspection, plus `kernel_test.go` from the host benchmark
fixture. Raw wall-time samples are also printed.

## 2026-09-14 Windows result

Three consecutive runs on the development host measured:

| Run | Compiled Oct median | Go median | Ratio |
| --- | ---: | ---: | ---: |
| 1 | 42.33 ms | 23.02 ms | 1.84x |
| 2 | 42.48 ms | 22.46 ms | 1.89x |
| 3 | 41.84 ms | 22.34 ms | 1.87x |

Both executables returned `1.9999999e+07` on every measured invocation.

## Generated-code differences at the September baseline

The baseline generated Go contains one necessary `float64(index)` conversion,
no redundant `if !...` guard branch in this specimen, and one MIR program
counter `switch` inside the generated function. The hand-written Go baseline
uses a direct structured `for` loop.

The earlier external review's approximately 3.4x result does not reproduce on
this host/current revision; the current observed gap is approximately
1.84x-1.89x. Process startup is included equally and makes this a conservative
microbenchmark rather than a kernel-only profile.

## Likely causes and actionability

- The conversion is required by the source's explicit `Float(index)` and is not
  redundant.
- The prior suspected redundant guard blocks are absent in this fixture.
- The remaining visible structural difference is the general MIR
  program-counter loop/switch used for ordinary control flow. Replacing it with
  structured-loop emission is not a trivial local fix; it is lowering or
  optimization work and is intentionally outside this hardening pass.
- No cloning, collection allocation, or avoidable range/index checks appear in
  the loop body.

## September recommendation

If this gap matters to a real workload, use a separate milestone to profile the
generated executable and evaluate structured emission for reducible MIR loops.
Require semantic parity and generated-code tests before changing control-flow
lowering. No optimization pass or speculative benchmark patch is introduced
here.

## 2026-11-04 direct-branch optimization

The reported issue is reproducible on the current development host (AMD Ryzen
7 7700X, Windows AMD64, Go 1.27.0). Before changing the emitter, the original
specimen measured 41.18 ms versus Go's 21.78 ms (1.89x). A retained baseline
measured 42.35 ms versus 22.62 ms (1.87x), with identical output throughout.

The ordinary Go emitter now translates static MIR edges directly to Go labels
and `goto`, reusing `BuildCFG` for validation and predecessor information.
Each MIR block retains its own Go lexical scope. Compiler-owned labels use
the existing internal naming authority. Legacy implicit fallthrough is made
explicit on a copy of the block list; emission does not mutate MIR.

This requires no loop reconstruction, source rewrite, new MIR optimization,
arithmetic reassociation, or removal of runtime checks. Ordinary functions
receive the improvement through the default `build.Compile` path. Resumable
FLOW control and its shared-expression dispatchers keep their existing model.

The first changed run measured 22.85 ms versus Go's 21.89 ms (1.04x). Disassembly
of the retained binaries confirms the mechanism: the old loop repeatedly
compares and writes the program counter; the new one is a compact arithmetic
loop with one conditional backedge. The required integer-to-float conversion
is still present, and both programs return `1.9999999e+07`.

### Final measurements

The original compiler was retained before editing (base revision
`62b02c79e4519ea8066a4b7d6cecfdb3149ca1f0`). The following kernel-only
measurements compile the retained generated Go with the same Go host benchmark,
check its result against the handwritten loop, and exclude process startup.
Five benchmark repetitions use one CPU, one second per benchmark, and no
concurrent regression work. All variants report zero bytes and zero allocations
per operation. Each operation performs all 20,000,000 iterations.

| Generated code | Oct kernel median | Go control median | Oct / Go |
| --- | ---: | ---: | ---: |
| Original dispatcher | 29.843 ms | 11.070 ms | 2.696x |
| Direct branches | 11.203 ms | 11.077 ms | 1.011x |

The generated kernel improves **2.664x** (62.5% less time) on this specimen.
This is evidence for removing scalar-loop dispatch overhead, not a general
claim about arrays, library algorithms, callbacks, or all Oct programs.

Three final seven-sample process-wall-time runs, also without overlapping test
work, measured:

| Run | Compiled Oct median | Go median | Ratio |
| --- | ---: | ---: | ---: |
| 1 | 23.373 ms | 22.057 ms | 1.060x |
| 2 | 23.368 ms | 22.647 ms | 1.032x |
| 3 | 23.304 ms | 22.656 ms | 1.029x |

Process startup is roughly half this short program's total time and masks
some of the loop improvement. Occasional ~100 ms scheduling/startup outliers
remain in the printed raw samples; the table uses medians, not minima.

Reproduce the current process and kernel measurements from the repository root:

```powershell
go run ./tools/scalar_loop_recon --out .tmp/scalar-loop/current
Push-Location .tmp/scalar-loop/current
go test -run '^$' -bench '^BenchmarkScalar' -benchmem -benchtime=1s -count=5 -cpu=1
Pop-Location
```

Machine-local evidence is retained under `.tmp/scalar-loop/before` and
`.tmp/scalar-loop/after`: generated Go, binaries, kernel benchmark/output,
disassembly, and final process samples. Generated evidence is not committed.

### Regression coverage

- Backend boundary tests parse and Go-typecheck the actual generated scalar
  function, require a direct backedge without a dispatcher, check malformed
  graph diagnostics, and check fallthrough/unreferenced-block handling without
  mutating input MIR.
- `Language/ControlFlow/Loops/valid/nested_loop_exits.octest` covers branch joins,
  early returns from nested ascending/descending loops, and fallible calls
  that either rejoin or propagate out of both loops.
- Run `Loops/valid` explicitly: the parent `Loops` target currently executes
  only the negative contracts, despite listing the positive source files in
  its JSON discovery output. The positive lane contains 25 cases, passing in
  both interpreted and strict compiled execution, with zero fallback.

Verification passed:

```text
go test ./internal/build ./tools/scalar_loop_recon
go test ./internal/tester
go test -tags integration ./internal/build
go run ./cmd/oct test Language/ControlFlow/Loops --execution compiled --json
go run ./cmd/oct test Language/ControlFlow/Loops/valid --execution interpreted --json
go run ./cmd/oct test Language/ControlFlow/Loops/valid --execution compiled --json
go test -tags integration ./cmd/oct -run 'TestLanguageCorpusRunsInBothLanes/Language/(ControlFlow|Errors|Expressions|Functions)/' -count=1 -timeout 8m
go test -tags integration ./cmd/oct -run 'TestLanguageCorpusRunsInBothLanes/Language/(Types|Testing)/' -count=1 -timeout 8m
```

The full repository suite, GPU/native lanes, and slow wrapper lanes were not
run for this Go control-flow emission change.

Source-only floating-point emission has a separate pre-existing helper omission,
recorded in `FEEDBACK.md`. It does not affect the executable path measured here.
