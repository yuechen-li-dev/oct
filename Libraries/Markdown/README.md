# Markdown Library

`Libraries/Markdown` provides deterministic Markdown helpers for report-like artifacts.

## Canonical usage

```oct
import Markdown

record MetricsTable {
    Method: String[]
    OutputSNR: String[]
}

fn Build() -> String[] {
    let table = MetricsTable {
        Method: ["Fixed", "Adaptive"]
        OutputSNR: ["-9.8", "-8.1"]
    }

    return Markdown.Report([
        Markdown.H1("Run Summary"),
        Markdown.KeyValueTable(["seed", "sampleCount"], ["42", "1200"]),
        Markdown.Table(table),
        Markdown.Callout("info", ["Compiled lane used for this run."])
    ])
}
```

## Shape contracts (authoring quick-reference)

- `Markdown.KeyValueTable(keys, values)` is the canonical scalar metadata/settings table helper.
  - Pass **two** arrays: keys then values.
  - Example: `Markdown.KeyValueTable(["sampleRate"], ["2000"])`.
- `Markdown.Table(table)` expects a **columnar record-of-string-columns** shape.
  - Example:
    - `record MetricsTable { Method: String[] OutputSNR: String[] }`
    - `let table = MetricsTable { Method: [...] OutputSNR: [...] }`
  - Row-major `header + rows` (`String[][]`) is not this helper's contract; convert row-major data to a columnar record first, or emit CSV / use a future row helper.
- `Markdown.TableWithColumns(table, columns)` provides explicit column-order control.
- `Markdown.Callout(kind, lines)` supports `note`, `info`, `warning`, `danger`, `success`.
  - `lines` must be `String[]`; for a single line use `Markdown.Callout("warning", ["message"])`.
