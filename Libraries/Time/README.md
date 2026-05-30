# Time

## Time.Core

Thin wrappers over Go `time` using RFC3339/ISO-8601 strings.

- `NowIso8601() -> String`
- `ParseIso8601(text: String) -> String ! Error`
- `FormatIso8601(text: String) -> String ! Error`
- `UnixSecondsNow() -> Int`
- `FormatUnixSeconds(seconds: Int) -> String ! Error`

Common failures:
- invalid time format strings for parse/format helpers

Compiled mode lowers these functions through the generic Octxiliary `octxiliary-time` sidecar declared in `manifest.oct`. The public parse and format APIs preserve the existing normalized RFC3339 string return values; Unix-second conversion remains available through `UnixSecondsNow` and `FormatUnixSeconds`.
