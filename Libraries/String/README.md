# String library

Minimal deterministic helpers for report/artifact text composition.

## API (canonical M0 namespace form)

```oct
import String
```

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

Compatibility/backing globals remain available (`StringJoin`, `StringQuoteJSON`, etc.) but are not the preferred user-facing spelling. Compatibility namespace aliases (`String.EscapeJSON`, `String.QuoteJSON`) are also supported, but `EscapeJson`/`QuoteJson` are canonical.

## Artifact guidance

- Use `IO.WriteLines` for markdown/text report files.
- Use `String.Join(parts, separator)` only when a single joined string is specifically needed.
- Use `Csv.Write` for CSV.
- Use `Json.Save` for structured JSON.
- Use `String.QuoteJson` when manual JSON-shaped text is necessary.
