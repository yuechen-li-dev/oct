# Oct

Oct is a scientific programming language and toolchain for reproducible research.

It is designed for the point where notebooks and scripts stop being enough: when an experiment needs tests, units, artifacts, packages, native binaries, and a distribution story. Oct's guiding principle is that the correct way should also be the easiest way.

## What is Oct?

Oct is an early scientific programming language/toolchain for portable computation, reproducible research, and AI-assisted experimentation.

Oct is built on Go as its systems substrate. Oct programs compile through Go, build quickly, run as native binaries, and target the platforms Go targets. Existing Go libraries can be exposed to Oct through explicit Octxiliary wrappers, letting researchers keep a high-level scientific language without losing access to the Go ecosystem.

The language includes first-class scientific features that are already represented in the repository's contracts and libraries: SI units, xUnit-style testing, arrays/vectors/matrices, native Einstein tensor notation, Octomata flow/state machines, utility scoring, fallible functions, package sync, optional `lock.octagon` reproducibility, and explicit native wrapper builds.

## Why Oct?

Oct is for research code that has outgrown throwaway scripts but still needs to stay close to the scientist's model of the problem.

- **Reproducibility by default:** tests, artifacts, package manifests, and optional lockfiles are part of the normal workflow.
- **Scientific language surface:** units, tensors, arrays, matrices, fallible functions, and experiment artifacts are language/toolchain concerns rather than notebook conventions.
- **Native distribution path:** the current implementation compiles through Go, so the compiled path can produce ordinary native binaries.
- **Explicit integration:** Octxiliary sidecars expose Go libraries through manifest-declared wrappers instead of hidden ambient bindings.
- **Agent-friendly workflow:** an LLM can create Oct experiments, run tests, sync packages, generate artifacts, and return reproducible code instead of a fragile transcript.

## Current status: v0.1 preview

Oct 0.1 is an early preview: real enough to run, test, package, and compile scientific programs, but still pre-1.0 and evolving.

Current milestone capabilities include:

- core Oct language/toolchain;
- interpreted and compiled execution paths;
- package manager MVP with local/Git source sync, transitive exact dependency graph sync, and optional project-root `lock.octagon`;
- source-controlled canonical first-party registry at `Registry/registry.oct`;
- manifest-declared wrapper lifecycle with Octxiliary sidecars and explicit `oct pkg build-wrappers --allow-native`;
- tests and CI coverage across core compiler/tooling paths.

The language definition lives in Oct source contracts under `Language/`. The Go implementation (`cmd/`, `internal/`) is the current implementation/backend for those contracts.

## Install

After the `v0.1.0` tag is published, install the Oct CLI with Go:

```sh
go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
```

For development from a checkout, use:

```sh
go run ./cmd/oct --help
go install ./cmd/oct
```

Optional sidecar command for compiled programs that use the current IO sidecar path:

```sh
go install github.com/yuechen-li-dev/oct/cmd/octxiliary-io@v0.1.0
```

Ensure your Go bin directory is on `PATH` (commonly `$(go env GOPATH)/bin` or your configured `GOBIN`), then verify:

```sh
oct --help
oct version
```

Release builds can inject a version string with:

```sh
go build -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=0.1.0" ./cmd/oct
```

## Quick start

Create and test a small library package:

```sh
oct new library HelloScience
cd HelloScience
oct test .
```

The generated library contains an `Identity` function and an xUnit-style `[Fact]` test. Replace those with your package code as the experiment grows.

From a repository checkout without installing first, the same flow is:

```sh
go run ./cmd/oct new library HelloScience
cd HelloScience
go run ../cmd/oct test .
```

## Package manager / canonical registry

Oct 0.1 includes a package manager MVP. The canonical first-party registry is source-controlled at:

```text
Registry/registry.oct
```

PM7 is intentionally local/source-controlled, not hosted. When using an installed `oct` outside this repository, point a project at a local checkout of the Oct repository:

```sh
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
oct test .
```

`Mathematics` is the canonical math package name. There is no `Math` alias in the canonical registry.

Optional lockfile workflow:

```sh
oct pkg lock
oct pkg sync --locked
```

Current package-manager boundaries for v0.1:

- registry entries are exact-version source entries;
- hosted registry, publishing, auth, signing, `.octpkg` artifacts, semver ranges, `latest`, and solver/backtracking behavior are not implemented;
- `lock.octagon` records the resolved graph but does not yet provide package tree digest or artifact integrity;
- wrapper package sync copies source and manifest metadata only; it does not build native sidecars.

## Wrapper / Octxiliary note

Octxiliary is the explicit sidecar bridge for exposing Go libraries to Oct. Wrapper packages declare sidecars in `manifest.oct`; `oct pkg wrappers` inspects that metadata without building or running native code.

Native sidecars are built only when requested explicitly:

```sh
oct pkg build-wrappers --allow-native
```

Built sidecars currently require `OCT_WRAPPER_PATH` or an existing sibling-discovery location at runtime. Package sync does not build sidecars, fetch arbitrary native dependencies, or run wrapper code.

## AI-assisted virtual laboratory note

Oct is designed to work well in agentic coding environments such as Codex Cloud or Claude Code. An LLM can write an experiment, run `oct test`, generate artifacts, sync exact package dependencies, and return a repository state that another user can reproduce locally.

This is a design goal, not a claim that every scientific workflow is complete in v0.1.

## Stability notice / pre-1.0 warning

Oct 0.1 is a preview release. Language syntax, Go APIs, package registry format, standard-library APIs, wrapper metadata, and compiled-backend support may change before 1.0. Performance is not final, and no production-readiness promise is made for this prerelease.

## Development/test commands

Useful commands from the repository root:

```sh
go test ./pkg/octxiliary ./internal/octxiliary
go test ./internal/pkgmgr ./internal/project
go test ./cmd/oct -run 'Version|Help|Pkg|Registry|Lock|New|Wrappers|BuildWrappers'
go test ./internal/... ./cmd/oct
go run ./tools/build_sidecars --out dist/sidecars
OCT_WRAPPER_PATH="$PWD/dist/sidecars" go test ./... -count=1
go run ./cmd/oct --help
go run ./cmd/oct pkg --help
go run ./cmd/oct version
```

On PowerShell, use the same sidecar build command and set the wrapper path with:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test ./... -count=1
```

For more details, start with:

- `docs/ARCHITECTURE.md` — architecture and execution model;
- `docs/CLI.md` — CLI quick reference;
- `docs/COMPILED_SUPPORT.md` — compiled-backend status;
- `Language/reference/` — canonical language/reference corpus;
- `docs/internal/canonical_registry_pm7.md` — canonical registry PM7 notes.
