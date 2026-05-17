# String library

Minimal deterministic helpers for report/artifact text composition.

## API (canonical M0 namespace form)

- `String.ByteLength(s)` -> `Int`
- `String.RuneCount(s)` -> `Int`
- `String.Join(parts, separator)` -> `String`
- `String.ReplaceAll(s, old, new)` -> `String`
- `String.Contains(s, needle)` -> `Bool`
- `String.StartsWith(s, prefix)` -> `Bool`
- `String.EndsWith(s, suffix)` -> `Bool`
- `String.Trim(s)` -> `String`
- `String.SplitLines(s)` -> `String[]`
- `String.EscapeJson(s)` -> `String`
- `String.QuoteJson(s)` -> `String`

Compatibility/backing globals remain available (`StringJoin`, `StringQuoteJSON`, etc.) but are not the preferred user-facing spelling.

## Artifact guidance

- Use `IO.WriteLines` for markdown/text report files.
- Use `String.Join` to build deterministic lines.
- Use `CsvWrite` for CSV.
- Use `JsonSave` for structured JSON.
- Use `StringQuoteJSON` only when manual JSON-shaped text is necessary.
