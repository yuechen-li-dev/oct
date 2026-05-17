# Artifact Library

`Artifact.*` is the canonical API for writing artifact outputs from `[Artifact]` functions.

## API

- `Artifact.WriteText(path, text) -> Void`
- `Artifact.WriteLines(path, lines) -> Void`
- `Artifact.WriteMarkdown(path, lines) -> Void`
- `Artifact.WriteCsv(path, table) -> Void`
- `Artifact.WriteJson(path, value) -> Void`
- `Artifact.WriteOctagon(path, value) -> Void`

All functions fail the artifact run immediately on write errors.

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

Lower-level `IO.*`, `Csv.*`, `Json.*`, and `WriteOctagon` APIs still exist for code that needs explicit fallible handling.
