# Doc Refactor Report: Builtins vs Standard Libraries vs Prometheus

## Why split was needed

The previous builtin page mixed core language primitives, standard-library wrapper surfaces, and Prometheus-specific surfaces.
That made ownership boundaries unclear and increased cognitive load for new readers.

## Audit outcome

### Stayed in core builtins (`language/09-builtins.md`)

- Core utilities: `Print`, `Len`, `Append`
- Numeric/math core family (`Abs`, `Sqrt`, trig/hyperbolic, `Exp`, `Ln`, `Log10`, `Pi`, `E`)
- Complex core family (`Complex`, `ComplexPolar`, `I`, `Real`, `Imag`, `Conj`, `Arg`)
- Core conversion/formatting (`Float`, `ToString`, `FormatFloat`)
- Core string helpers (`Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`)

### Moved to standard libraries ownership (`language/17-standard-libraries.md`)

- Wrapper/library-backed practical modules (`IO.*`, `Archive.Zip`, `Compression.Gzip`, `Hash.Core`, `Image.Core`, `Text.Regex`, `Time.Core`, `IO.Xlsx`)
- Backend-support builtin category explanation for wrapper machinery
- Practical guidance to prefer module APIs over low-level wrapper calls

### Moved to Prometheus ownership (`runtime/23-prometheus.md`)

- `PROMETHEUS { ... }` block surface
- `PrometheusMatMul(a, b)` surface
- Prometheus CLI surfaces (`oct prometheus-sgemm`, `oct prometheus-m1-async`)
- Experimental/fallback posture and caveats

## Numbering, order, and navigation changes

- Added `language/17-standard-libraries.md` after `16-vectors-and-matrices.md`.
- Added `runtime/23-prometheus.md` after `22-batch.md`.
- Updated `Language/reference/README.md` section lists to include both pages in numbered order.
- Updated `Language/reference/00-overview.md` practical reading paths to include standard libraries and Prometheus path.

## Cross-links added

- Builtins page now links to standard libraries and Prometheus pages.
- Standard libraries page links back to builtins and to Prometheus.
- Prometheus page links back to builtins and standard libraries.

## Explicit inconsistencies / documentation gaps surfaced

- Prometheus language surfaces (`PROMETHEUS` blocks and `PrometheusMatMul`) exist in implementation/tests but were previously undocumented in `Language/reference`; this refactor adds explicit reference coverage.
- CLI reference (`tooling/35-cli.md`) still does not enumerate Prometheus commands even though they exist in implementation; this remains a follow-up documentation gap.
