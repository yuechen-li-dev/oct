# Plot

## Role

`Plot.Core` is the advanced plotting surface for Oct standard libraries.

- **Convenience tier (no import required):** builtins `PlotLine` and `PlotScatter` stay available directly for quick one-liners.
- **Library tier (import required):** `Plot.Core` provides richer plot controls (size, titles, labels, legend text, and histogram support).

This tier split intentionally mirrors the `Image.Core`/artifact foundation: plots render directly to output image files, with explicit output geometry in `Int<px>`.

## Current MVP surface

- `record Size { Width: Int<px>, Height: Int<px> }`
- `record Labels { Title: String, X: String, Y: String, Legend: String }`
- `Line(x: Float[], y: Float[], outputPath: String, size: Size, labels: Labels) -> Int ! Error`
- `Scatter(x: Float[], y: Float[], outputPath: String, size: Size, labels: Labels) -> Int ! Error`
- `Histogram(values: Float[], bins: Int, outputPath: String, size: Size, labels: Labels) -> Int ! Error`
- `DefaultSize() -> Size`
- `DefaultLabels() -> Labels`

## Notes

- `outputPath` must end in `.png`.
- `Line`/`Scatter` require equal-length x/y arrays.
- `Histogram` requires positive bin count.
- This MVP intentionally does not expose every backend `gonum/plot` knob.
