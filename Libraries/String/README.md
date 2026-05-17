# String library

Minimal deterministic helpers for report/artifact text composition.

## API

- `IsEmpty(s)` -> `Bool` (`true` when UTF-8 byte length is zero)
- `ByteLength(s)` -> `Int` (UTF-8 byte length)
- `RuneCount(s)` -> `Int` (Unicode code-point count)
- `Concat(parts)` -> `String` (concatenate all parts)
- `Join(separator, parts)` -> `String` (join parts with separator)
- `Replace(s, old, new)` -> `String ! Error` (replaces all non-overlapping occurrences; rejects `old == ""`)
- `Contains(s, needle)` -> `Bool`
- `StartsWith(s, prefix)` -> `Bool`
- `EndsWith(s, suffix)` -> `Bool`
- `Trim(s)` -> `String` (Go `strings.TrimSpace` / Unicode whitespace)
- `SplitLines(s)` -> `String[]` (normalizes `CRLF` to `LF`; splits on `LF`; preserves interior empties; suppresses terminal synthetic empty line)
- `EscapeJson(s)` -> `String` (JSON escaped contents, no wrapping quotes)
- `QuoteJson(s)` -> `String` (complete JSON string literal with wrapping quotes)

## Artifact guidance

- Use `IO.WriteLines` for markdown/text report files.
- Use `String.Join`/`String.Concat` to build deterministic lines.
- Use `IO.CsvWrite` for CSV.
- Use `IO.JsonSave` for structured JSON.
- Avoid hand-built JSON unless `QuoteJson` is specifically needed for text payloads.
