# Text

## Text.Regex

Thin wrappers over Go `regexp`.

- `IsMatch(pattern, text) -> Bool ! Error`
- `FindAll(pattern, text) -> String[] ! Error`
- `ReplaceAll(pattern, text, replacement) -> String ! Error`
- `Split(pattern, text) -> String[] ! Error`

Common failures:
- invalid regex pattern strings
