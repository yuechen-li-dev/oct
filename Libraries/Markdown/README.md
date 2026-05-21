# Markdown Library (M1)

Markdown is a deterministic report-writing helper, not a Markdown parser.

All block helpers return `String[]` lines suitable for `IO.WriteLines` or `Artifact.WriteMarkdown`.

For report ergonomics, prefer semantic heading aliases:

- `Markdown.Title(text)` as the canonical report title helper (same output as `Markdown.H1(text)`).
- `Markdown.Subtitle(text)` as the canonical report subtitle helper (same output as `Markdown.H2(text)`).
- `Markdown.H1` / `Markdown.H2` / `Markdown.H3` remain supported lower-level heading helpers.

`Markdown.Table` expects a columnar record-of-string-columns table.
Use `Markdown.TableWithColumns(table, columns)` for explicit output column order.
Use `Markdown.KeyValueTable(keys, values)` for scalar metadata/settings sections.

`Markdown.CodeBlock(language, lines)` uses dynamic backtick fence lengths: it scans content (and language) for the longest backtick run and emits a fence of `max(3, longestRun + 1)`.

M1 adds report ergonomics:

- `Markdown.Callout(kind, lines)` for deterministic blockquote callouts (`note`, `info`, `warning`, `danger`, `success`).
- `Markdown.Image(path, altText)` for single-line image output.
- `Markdown.Figure(path, caption)` for image + caption output.
- `Markdown.KeyValueTable(keys, values)` for two-column key/value tables.
- `Markdown.Section(title, blocks)` and `Markdown.Subsection(title, blocks)` for heading + flattened report blocks.

No CommonMark compliance is claimed in M1.
Markdown remains line-oriented output only; no parser, renderer, or nested document model is introduced.

Known limitation: `Markdown.Image`/`Markdown.Figure` only perform minimal alt-text safety normalization and do not implement full URL/path escaping.

Future direction (not implemented in M1): an Oct-owned `.md` dialect could later parse structured tags such as `<Code>`, `<Figure>`, and `<Callout>` for PDF/UI report pipelines.

Canonical namespaced authoring pattern:

```oct
import Markdown
let report = Markdown.Report([
    Markdown.Title("Experiment Report"),
    Markdown.Subtitle("Overview"),
    Markdown.Paragraph("Body.")
])

import String
let text = String.Join(lines, "\n")

import IO
IO.WriteLines("out/report.md", lines)!

import Artifact
Artifact.WriteMarkdown("out/report.md", lines)
```
