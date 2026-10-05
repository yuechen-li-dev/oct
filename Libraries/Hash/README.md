# Hash

## Hash.Core

Thin wrappers over Go `crypto/sha256`.

- `Sha256Bytes(data: Bytes) -> String ! Error`
- `Sha256Text(text: String) -> String ! Error`
- `Sha256File(path: String) -> String ! Error`

Hex strings are lowercase and deterministic.

Compiled mode runs the hash builtins these functions call in the `octxiliary-hash` sidecar.
