# M104 Plot.Core report

## 1) Existing plotting surface before this wave

- Direct builtins `PlotLine(x, y, path)` and `PlotScatter(x, y, path)` already existed.
- These functions are no-import convenience helpers and still render PNG output through the existing `gonum/plot` backend path.

## 2) What remains convenience builtins

- `PlotLine`
- `PlotScatter`

These remain low-ceremony and do not require `import Plot`.

## 3) What moved into `Plot.Core`

New advanced layer is provided by `Libraries/Plot/Plot.Core.oct`:

- `Plot.Line(...)` with explicit `Size` and `Labels`
- `Plot.Scatter(...)` with explicit `Size` and `Labels`
- `Plot.Histogram(...)` with explicit `Size`, `Labels`, and bin count
- `Plot.DefaultSize()` and `Plot.DefaultLabels()` helpers

Under the hood these call new backend-support plot builtins:

- `PlotRenderLine`
- `PlotRenderScatter`
- `PlotRenderHistogram`

## 4) Documentation updates for the split

Updated `Language/reference/language/17-standard-libraries.md` and `Language/reference/language/09-builtins.md` to explicitly state:

- convenience plotting builtins are direct/no-import
- advanced plotting requires `Plot.Core`
- examples for both paths

## 5) Backend functionality reused

- Reused existing `gonum/plot` dependency and plotting artifact save path.
- Refactored plotting runtime path into a shared renderer so convenience and advanced surfaces share backend behavior.

## 6) Intentionally left out of MVP

- Multi-series APIs
- Bar chart API
- Output formats beyond PNG
- Full backend style knob exposure
- PDF output work

## Inconsistency surfaced

- Existing direct builtins remain interpreter-centric and non-fallible from the language signature perspective, while new `Plot.Core` wrappers are fallible (`! Error`) and wrapper-style.
- This is intentional for the two-tier model but means plotting currently has mixed error-shape ergonomics between convenience and library tiers.
