# Oct CLI Quick Reference

Usage:

- `oct <command> [options]`
- `oct --help`
- `oct <command> --help`

Common commands:

- `go run ./cmd/oct test <path>`
- `go run ./cmd/oct test <path> --suite <suite>`
- `go run ./cmd/oct test <path> --execution compiled`
- `go run ./cmd/oct test <path> --all-packages`
- `go run ./cmd/oct artifact <path>`
- `go run ./cmd/oct fmt <path> --mode en-llm --check`
- `go run ./cmd/oct fmt <path> --mode en-llm-compact --check`
- `go run ./cmd/oct bench <path> --profile`

`oct test` defaults to running tests only from the selected entry package/root. Imported packages are still loaded for typechecking, but their tests are excluded unless `--all-packages` is specified.
