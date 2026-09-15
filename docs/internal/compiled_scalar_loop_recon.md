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

## 2026-09-14 Windows result

Three consecutive runs on the development host measured:

| Run | Compiled Oct median | Go median | Ratio |
| --- | ---: | ---: | ---: |
| 1 | 42.33 ms | 23.02 ms | 1.84x |
| 2 | 42.48 ms | 22.46 ms | 1.89x |
| 3 | 41.84 ms | 22.34 ms | 1.87x |

Both executables returned `1.9999999e+07` on every measured invocation.

## Generated-code differences

The current generated Go contains one necessary `float64(index)` conversion,
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

## Recommendation

If this gap matters to a real workload, use a separate milestone to profile the
generated executable and evaluate structured emission for reducible MIR loops.
Require semantic parity and generated-code tests before changing control-flow
lowering. No optimization pass or speculative benchmark patch is introduced
here.
