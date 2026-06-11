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


## `oct pkg` wrapper tooling

Inspection remains inert:

```sh
oct pkg wrappers
oct pkg wrappers --registry-out wrappers.octagon
```

`oct pkg wrappers` reports wrapper metadata and can write a data-only registry artifact. It does not build Go modules, create `.oct/wrappers`, run sidecars, or change runtime discovery.

Native sidecars are built only through the explicit command:

```sh
oct pkg build-wrappers --allow-native
```

The command builds current-package Go-module sidecars declared by `GoModuleDir` and writes binaries to `.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe]`. It does not run sidecars, does not build dependency/package-cache/registry sidecars, does not fetch packages, and does not add lockfiles or arbitrary build scripts.

Current runtime discovery does not automatically search `.oct/wrappers/<platform>`. After a successful build, use the printed guidance, for example:

```sh
OCT_WRAPPER_PATH=.oct/wrappers/linux-amd64 oct test .
```

or place the sidecar in an existing sibling-discovery location.

### Optional package lockfiles

`oct pkg lock` writes an optional project-root `lock.octagon` from the current exact-version registry dependency graph. The file is deterministic and timestamp-free. Git entries record the original `Ref` plus a full `ResolvedCommit`; local entries are allowed but marked mutable and are not reproducible because PM6 records no content digest. Wrapper packages are locked as source only; this command does not build sidecars or create `.oct/wrappers`.

`oct pkg sync --locked` requires `lock.octagon`, validates it against the current manifest, and syncs exactly the locked graph. Git packages are checked out at `ResolvedCommit`, not a mutable ref. Local packages are copied from the locked source/path.

Plain `oct pkg sync` remains rolling: it does not read or write `lock.octagon`, and it never creates `oct.lock` or `lock.oct`. Lockfiles do not add `.octpkg` artifacts, package tree/source/registry digests, signing, federation/P2P, publishing, auth, mirrors, binary sidecar distribution, semver ranges, `latest`, or solver/backtracking behavior.
