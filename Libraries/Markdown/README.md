# Markdown Library (M0)

Markdown is a deterministic report-writing helper, not a Markdown parser.

All block helpers return `String[]` lines suitable for `IO.WriteLines` or `Artifact.WriteMarkdown`.

`Markdown.Table` expects a columnar record-of-string-columns table.
Use `Markdown.TableWithColumns(table, columns)` for explicit output column order.

`Markdown.CodeBlock(language, lines)` uses dynamic backtick fence lengths: it scans content (and language) for the longest backtick run and emits a fence of `max(3, longestRun + 1)`. This prevents generated reports with nested fences from prematurely closing outer blocks.

No CommonMark compliance is claimed in M0.
