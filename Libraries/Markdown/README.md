# Markdown Library (M1)

Markdown is a deterministic report-writing helper, not a Markdown parser.

All block helpers return `String[]` lines suitable for `Artifact.WriteMarkdown(path, lines)` (preferred) or `IO.WriteLines(path, lines)!`.

## Canonical heading posture

- `Markdown.Title(text)` is the preferred report title helper and aliases `Markdown.H1(text)`.
- `Markdown.Subtitle(text)` is the preferred report subtitle helper and aliases `Markdown.H2(text)`.
- `Markdown.H1` / `Markdown.H2` / `Markdown.H3` remain available lower-level heading helpers.

## Canonical report pattern

`Markdown.Report` takes a list of blocks.

```oct
import Markdown
import Artifact

let lines = Markdown.Report([
    Markdown.Title("Experiment Report"),
    Markdown.Subtitle("Run Summary"),
    Markdown.Section("Config", [
        Markdown.KeyValueTable(["seed", "sampleCount"], ["42", "1200"])
    ]),
    Markdown.Section("Results", [
        Markdown.Table(table),
        Markdown.Callout("info", ["Compiled lane used for this run."])
    ])
])

Artifact.WriteMarkdown("out/report.md", lines)
```

## Surface notes

- `Markdown.Section(title, blocks)` and `Markdown.Subsection(title, blocks)` flatten lists of blocks.
- `Markdown.KeyValueTable(keys, values)` is the canonical scalar metadata/settings table helper.
- `Markdown.Table(table)` expects a columnar record-of-string-columns shape.
- `Markdown.TableWithColumns(table, columns)` provides explicit column-order control.
- `Markdown.Callout(kind, lines)` supports `note`, `info`, `warning`, `danger`, `success`.
- `Markdown.Image(path, altText)` and `Markdown.Figure(path, caption)` are line-oriented media helpers.
- `Markdown.CodeBlock(language, lines)` uses dynamic backtick fences to avoid premature fence closure.

No CommonMark compliance is claimed in M1.
