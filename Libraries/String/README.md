# String library

Minimal deterministic helpers for report/artifact text composition.

## API

- `StringByteLength(s)` -> `Int` (UTF-8 byte length)
- `StringRuneCount(s)` -> `Int` (Unicode code-point count)
- `StringJoin(parts, separator)` -> `String` (join parts with separator)
- `StringReplaceAll(s, old, new)` -> `String` (replaces all non-overlapping occurrences; when `old == ""`, follows Go `strings.ReplaceAll` insertion semantics)
- `StringContains(s, needle)` -> `Bool`
- `StringStartsWith(s, prefix)` -> `Bool`
- `StringEndsWith(s, suffix)` -> `Bool`
- `StringTrim(s)` -> `String` (Go `strings.TrimSpace` / Unicode whitespace)
- `StringSplitLines(s)` -> `String[]` (normalizes `CRLF` to `LF`; splits on `LF`; preserves interior empties; suppresses terminal synthetic empty line)
- `StringEscapeJSON(s)` -> `String` (JSON escaped contents, no wrapping quotes)
- `StringQuoteJSON(s)` -> `String` (complete JSON string literal with wrapping quotes)

## Artifact guidance

- Use `IO.WriteLines` for markdown/text report files.
- Use `StringJoin` to build deterministic lines.
- Use `CsvWrite` for CSV.
- Use `JsonSave` for structured JSON.
- Use `StringQuoteJSON` only when manual JSON-shaped text is necessary.
