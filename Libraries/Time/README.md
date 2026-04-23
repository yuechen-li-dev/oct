# Time

## Time.Core

Thin wrappers over Go `time` using RFC3339/ISO-8601 strings.

- `NowIso8601() -> String`
- `ParseIso8601(text) -> String ! Error`
- `FormatIso8601(text) -> String ! Error`
- `UnixSecondsNow() -> Int`
- `FormatUnixSeconds(seconds) -> String ! Error`

Common failures:
- invalid time format strings for parse/format helpers
