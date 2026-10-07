# Artifact Library

`Artifact.*` is the compiler-owned output capability for `[Artifact]` functions.
`oct artifact` loads, binds, and type-checks the selected program before it
evaluates artifact entry points through the shared typed interpreter. It does
not generate or compile the application backend.

## API

- `Artifact.WriteText(path, text) -> Void`
- `Artifact.WriteLines(path, lines) -> Void`
- `Artifact.WriteMarkdown(path, lines) -> Void`
- `Artifact.WriteCsv(path, table) -> Void`
- `Artifact.WriteJson(path, value) -> Void`
- `Artifact.WriteOctagon(path, value) -> Void`
- `Artifact.Markdown(path, doc) -> Void`
- `Artifact.Docx(path, doc) -> Void`
- `Artifact.Latex(path, doc) -> Void`
- `Artifact.Pdf(path, doc) -> Void`

All functions are valid only during the explicit artifact phase. They declare
paths relative to `--output-root` (the working directory by default), reject
absolute and escaping paths, and fail on duplicate output paths. Outputs are
staged until every selected entry point succeeds, then published in sorted path
order. Identical content is reported as unchanged and is not rewritten.

## JSON

- `Artifact.WriteJson(path, value)` publishes the JSON text of a typed value:
  a record, a `record table`, an array, a scalar. The rules are those of
  `Json.Save`; see [`Libraries/Json`](../Json/README.md).
- Give it the value. JSON text is not built by hand: a String is written as
  a JSON string, not taken as JSON.
- `Json.Save` is rejected during artifact evaluation. `Json.Load<T>(path)`
  may read an output this phase has already written, to assert on it.

## Example

```oct
import Artifact

[Artifact]
fn WriteOutputs() {
    Artifact.WriteOctagon("report.octagon", report)
    Artifact.WriteCsv("metrics.csv", metrics)
    Artifact.WriteJson("metrics.json", summary)
    Artifact.WriteMarkdown("report.md", lines)
}
```

Lower-level `IO.*`, `Csv.*`, `Json.*`, and `WriteOctagon` APIs still exist for
ordinary runtime code. During artifact evaluation, the legacy global
`WriteOctagon` call is a compatibility alias for the same output capability;
ordinary filesystem writes are rejected. `Directory.Make*` is accepted only as
a confined staging-directory compatibility operation.

The document artifact APIs accept the canonical `Document.Doc`. `Artifact.Latex`
publishes deterministic human-readable LaTeX plus content-addressed relative
assets. `Artifact.Pdf` preserves that exact sibling `.tex` bundle and compiles it
with the configured/discovered LaTeX engine; it does not invoke the direct-drawing
`Libraries/Pdf` API or create a second document IR.
