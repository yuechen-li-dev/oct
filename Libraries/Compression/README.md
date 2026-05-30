# Compression

## Compression.Gzip

Thin wrappers over Go `compress/gzip`.

- `CompressBytes(data: Bytes) -> Bytes ! Error`
- `DecompressBytes(data: Bytes) -> Bytes ! Error`
- `CompressFile(inputPath, outputPath) -> Int ! Error`
- `DecompressFile(inputPath, outputPath) -> Int ! Error`

Common failures:
- missing file paths
- invalid gzip payloads for decompression

Compiled mode lowers these functions through the generic Octxiliary `octxiliary-compression` sidecar declared in `manifest.oct`.
