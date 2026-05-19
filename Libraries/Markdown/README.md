# Markdown Library (M0)

Markdown is a deterministic report-writing helper, not a Markdown parser.

All block helpers return `String[]` lines suitable for `IO.WriteLines` or `Artifact.WriteMarkdown`.

`Markdown.Table` expects a columnar record-of-string-columns table.
Use `Markdown.TableWithColumns(table, columns)` for explicit output column order.

No CommonMark compliance is claimed in M0.
