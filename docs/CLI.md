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
- `go run ./cmd/oct new library SignalTools`
- `go run ./cmd/oct new experiment BrownNoiseKalman`
- `go run ./cmd/oct new wrapper-library OpenCV`

`oct test` defaults to running tests only from the selected entry package/root. Imported packages are still loaded for typechecking, but their tests are excluded unless `--all-packages` is specified.

## `oct new` package scaffolding

Usage:

```sh
oct new <experiment|library|wrapper-library> <Name>
```

`oct new` creates deterministic package scaffolds in the current working directory at `./<Name>`. W5 supports exactly three no-flag forms: `library`, `experiment`, and `wrapper-library`. The command rejects missing or extra arguments, unknown scaffold kinds, invalid names, and any target directory that already exists.

Names must match strict PascalCase `[A-Z][A-Za-z0-9]*`; non-PascalCase inputs such as `oct-opencv`, `signal_tools`, and `openCV` are rejected instead of normalized. Reserved names such as `Manifest`, `Main`, built-in scalar/type family names, and top-level command family names are also rejected.

Wrapper-library scaffolds include manifest wrapper metadata and a package-local sidecar reference under `sidecars/octxiliary-<kebab>/`. `oct pkg wrappers` can inspect this metadata and render registry output, but `oct new wrapper-library` does not build or run native sidecars. The generated raw wrapper function is metadata only until future wrapper dispatch/build lifecycle milestones.
