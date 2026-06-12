# Oct v0.1 release readiness

## Release positioning

Oct 0.1 is an early preview of a scientific programming language and toolchain for reproducible research. It is real enough to run, test, package, and compile scientific programs, but it is still pre-1.0 and evolving.

Core message:

> Oct is a scientific programming language and toolchain for reproducible research. It targets the point where notebooks and scripts stop being enough: when an experiment needs tests, units, artifacts, packages, native binaries, and a distribution story.

Do not position v0.1 as production-stable. Do not make unmeasured performance claims.

## Install command after tag

```sh
go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
```

Optional IO sidecar command:

```sh
go install github.com/yuechen-li-dev/oct/cmd/octxiliary-io@v0.1.0
```

Development version check:

```sh
oct version
```

Release builds may inject the tag with:

```sh
go build -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=0.1.0" ./cmd/oct
```

## Key capabilities to mention

- Core Oct language/toolchain.
- Interpreted and compiled execution paths.
- SI units.
- Tests, artifacts, and benchmarks.
- Arrays, vectors, matrices, tensors, and Einstein notation.
- Octomata flow/state machines.
- Utility scoring and fallible functions.
- Package manager MVP with local/Git source sync and exact transitive registry sync.
- Canonical first-party registry at `Registry/registry.oct`.
- Optional project-root `lock.octagon`.
- Manifest-declared Octxiliary wrapper sidecars.
- Explicit native wrapper builds with `oct pkg build-wrappers --allow-native`.
- Go-based compiled/native binary path.

## Known limitations

- Language syntax and APIs may change before 1.0.
- Public Go APIs may evolve before 1.0.
- Standard library/package APIs may evolve.
- The canonical registry is local/source-controlled; hosted registry resolution is not implemented.
- Package publishing, auth, signing, mirrors, federation/P2P, and `.octpkg` artifacts are not implemented.
- Semver ranges, `latest`, and solver/backtracking package semantics are not implemented.
- `lock.octagon` has no package tree digest or artifact integrity guarantee yet.
- Wrapper sidecars require explicit build and runtime discovery through `OCT_WRAPPER_PATH` or sibling discovery.
- Package sync does not build sidecars.
- Performance is not final.

## Canonical registry notes

The canonical registry lives at:

```text
Registry/registry.oct
```

Use it from a project with:

```sh
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
```

`Mathematics` is the canonical package name. Do not introduce or document a `Math` alias.

Optional lockfile commands:

```sh
oct pkg lock
oct pkg sync --locked
```

## Pre-tag checklist

- Worktree is clean before tagging.
- README install and quickstart commands are accurate, including:

  ```sh
  go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
  ```

- Changelog includes v0.1.0 initial preview notes.
- `pkg/octxiliary` package comment renders sensibly on pkg.go.dev.
- CLI version surface works (`oct version`, or `go run ./cmd/oct version` from a checkout).
- Package-manager docs mention registry, lockfile, and wrapper build boundaries.
- Sidecars build successfully.
- Fast/default tests pass.
- Slow Octxiliary wrapper CI lane passes explicitly.
- CI is green on supported platforms.
- No major language semantics, package-manager semantics, or wrapper runtime behavior changed for release polish.

Required checks:

```sh
go test ./pkg/octxiliary ./internal/octxiliary
go test ./internal/pkgmgr ./internal/project
go test ./cmd/oct -run 'Init|New|Version|Help|Pkg|Registry|Lock|Language'
go test ./internal/... ./cmd/oct
go test -count=1 -parallel 8 ./...
go run ./tools/build_sidecars --out dist/sidecars
OCT_SLOW_TESTS=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" go test -count=1 -parallel 8 ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
go run ./cmd/oct version
```

PowerShell slow-wrapper lane:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -parallel 8 ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

## Suggested tag command

```sh
git tag v0.1.0
git push origin v0.1.0
```

Do not reuse or mutate a published tag. If a bad version is published, release a newer version and use Go module retractions if needed.

## pkg.go.dev verification

After the tag is pushed, request/index the module by visiting:

```text
https://pkg.go.dev/github.com/yuechen-li-dev/oct
```

or by requesting the module proxy metadata:

```sh
GOPROXY=https://proxy.golang.org GO111MODULE=on go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
```

Verify at least:

- module page loads;
- `pkg/octxiliary` page has the intended package comment;
- install command resolves at `v0.1.0`.

## CI requirements

Before tagging, CI should pass the normal Go test suite and the targeted release-readiness package-manager/wrapper/version checks. Windows and Linux hardening should remain green for command paths touched by this release pass.
