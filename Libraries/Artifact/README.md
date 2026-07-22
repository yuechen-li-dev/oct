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

All functions are valid only during the explicit artifact phase. They declare
paths relative to `--output-root` (the working directory by default), reject
absolute and escaping paths, and fail on duplicate output paths. Outputs are
staged until every selected entry point succeeds, then published in sorted path
order. Identical content is reported as unchanged and is not rewritten.

## JSON authoring guidance (canonical)

- In `[Artifact]` functions, prefer `Artifact.WriteJson(path, value)`.
- Use `Json.Save(path, value)!` only when you intentionally need lower-level fallible control flow.
- Do **not** manually stringify JSON for artifact output unless there is a specific reason.
- `Json.Stringify(...)` is **not** currently exposed as a namespaced `Json.*` alias (`Json` namespace exposes `Load` and `Save`).

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
