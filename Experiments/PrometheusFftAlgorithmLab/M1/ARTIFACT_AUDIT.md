# PrometheusFftAlgorithmLab M1 Artifact Audit

Date: 2026-05-25 (UTC)

## Scope
Independent rerun of the M1 test + artifact lanes and audit of emitted Octagon artifacts under `out/prometheus_fft_algorithm_lab/m1/`.

## Commands run
1. `rm -rf out/prometheus_fft_algorithm_lab/m1`
2. Manifest/layout checks:
   - `[ -f Experiments/PrometheusFftAlgorithmLab/manifest.oct ]`
   - `[ -f Experiments/PrometheusFftAlgorithmLab/REPORT.md ]`
   - `[ ! -f Experiments/PrometheusFftAlgorithmLab/M1/manifest.oct ]`
3. `go run ./cmd/oct test Experiments/PrometheusFftAlgorithmLab/M1`
4. `go run ./cmd/oct artifact Experiments/PrometheusFftAlgorithmLab/M1`
5. `find out/prometheus_fft_algorithm_lab/m1 -maxdepth 1 -type f -print`
6. `stat -c '%n %s' out/prometheus_fft_algorithm_lab/m1/*`
7. Parse evidence via focused regression: `go test ./cmd/oct -run 'PrometheusFft|Octagon|Artifact|Manifest' -count=1`
8. Wider regression:
   - `go test ./internal/... ./cmd/oct`
9. Content spot checks:
   - `sed -n '1,220p' out/prometheus_fft_algorithm_lab/m1/m1_fft_report.md`
   - `rg -n ... out/prometheus_fft_algorithm_lab/m1/*.octagon`

## Manifest/layout verification
- `Experiments/PrometheusFftAlgorithmLab/manifest.oct`: present
- `Experiments/PrometheusFftAlgorithmLab/REPORT.md`: present
- `Experiments/PrometheusFftAlgorithmLab/M1/manifest.oct`: absent (as expected)

## Emitted files and sizes
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_cases.octagon` — 7636 bytes
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_results.octagon` — 10122 bytes
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_plan_traces.octagon` — 17935 bytes
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_report.md` — 727 bytes

All four expected files exist and are non-empty.

## Octagon parse audit
Parse requirement was validated by focused cmd/oct regression, including `TestPrometheusFftAlgorithmLabM1ArtifactWritesDeterministicVisibleOutputs`, which explicitly loads:
- `m1_fft_cases.octagon`
- `m1_fft_results.octagon`
- `m1_fft_plan_traces.octagon`

Result: pass (`ok github.com/yuechen-li-dev/oct/cmd/oct`).

## Semantic plausibility summary
- Case count represented: report summary shows `total=28`, `passed=28`, `failed=0`.
- Required N values: report states `N in {1,2,4,8,16}`.
- Rejection evidence: report explicitly states rejection checks for `N=0` and non-power-of-two; summary includes `rejectN0=true` and `rejectN3=true`.
- Result summary: indicates all validation cases pass.
- Error metrics: `m1_fft_results.octagon` contains forward/roundtrip max error fields with very small values (scientific-notation near machine epsilon); no obvious anomaly.
- Plan traces: `m1_fft_plan_traces.octagon` includes representative traces with `N: (8)` and `N: (16)` entries.
- Negative numeric evidence: negative numeric values are present across Octagon payloads (e.g., negative exponents/signals/snapshots) and parse path succeeds.
- Normalization convention: report states forward FFT unnormalized and inverse divides by N.

## Deviations / suspicious findings
- No artifact-path or manifest-layout regression observed.
- `oct test` included informational compiled fallback lines (`unknown function 'String.From'`) for two FFT tests, but tests passed via interpreted fallback and overall lane passed. This is noted as informational only.

## Conclusion
PASS.

Acceptance criteria 1–10 are satisfied in this rerun:
- M1 test lane passes.
- M1 artifact lane passes.
- Expected 4 output files emitted in expected directory.
- Files are non-empty.
- Required `.octagon` artifacts parse successfully (validated by focused regression test).
- Artifact contents are semantically plausible for the intended FFT M1 matrix.
- `ARTIFACT_AUDIT.md` added.
- No algorithm or schema changes required.
- Focused regression passes.
- Broader `go test ./internal/... ./cmd/oct` passes.

## Code changes required
- Only this audit report file was added.
