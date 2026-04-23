# Archive

## Archive.Zip

Thin wrappers over Go `archive/zip` for practical archive inspection and extraction.

- `ListEntries(path) -> String[] ! Error`
- `ExtractAll(path, destination) -> Int ! Error`
- `CreateFromFiles(outputPath, paths) -> Int ! Error`

Common failures:
- missing archive path
- invalid zip payload
- entry path escaping extraction root (rejected as invalid data)
