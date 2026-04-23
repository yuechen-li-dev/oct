# Hash

## Hash.Core

Thin wrappers over Go `crypto/sha256`.

- `Sha256Bytes(data: Bytes) -> String`
- `Sha256Text(text: String) -> String`
- `Sha256File(path: String) -> String ! Error`

Hex strings are lowercase and deterministic.
