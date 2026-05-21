# String library

Minimal deterministic helpers for report/artifact text composition.

## API (canonical M0 namespace form)

```oct
import String
```

- `String.ByteLength(s)` -> `Int`
- `String.RuneCount(s)` -> `Int`
- `String.Join(parts, separator)` -> `String`
- `String.Concat(parts)` -> `String`
- `String.From<T>(value)` -> `String` (`T` in M0: `Int`, `Float`, `Bool`, `String`)
- `String.ReplaceAll(s, old, new)` -> `String`
- `String.Contains(s, needle)` -> `Bool`
- `String.StartsWith(s, prefix)` -> `Bool`
- `String.EndsWith(s, suffix)` -> `Bool`
- `String.Trim(s)` -> `String`
- `String.SplitLines(s)` -> `String[]`
- `String.EscapeJson(s)` -> `String`
- `String.QuoteJson(s)` -> `String`

Compatibility/backing globals remain available (`StringJoin`, `StringQuoteJSON`, etc.) but are transition/backing surface, not preferred authoring style.

## Canonical examples

```oct
import String

let sampleCount = FloorToInt(sampleRate * duration)
let summary = String.Concat(["samples=", String.From<Int>(sampleCount)])
let scalar = String.From<Float>(value)
let textBlob = String.Join(lines, "\n")
```

Use `String.Join(lines, "\n")` when one joined text blob is needed; otherwise keep line-oriented `String[]` and write with `Artifact.WriteLines`/`IO.WriteLines`.

## Constrained generic builtin note

Oct does not support user-defined generics.
A small number of compiler-known builtins use explicit type arguments for closed conversion/decoding contracts. `String.From<T>` (and `ReadOctagon<T>` in artifact/data lanes) are examples.

`ToString(...)` remains available, but namespaced `String.From<T>` is preferred in report/library code for explicitness and consistency.
