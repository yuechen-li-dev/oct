# PM6 optional package lockfile M0

PM6 implements the optional package lockfile path designed in PM5.

## Commands

```sh
oct pkg lock
oct pkg sync --locked
```

Plain `oct pkg sync` remains rolling. It does not read, write, update, or require `lock.octagon`.

## File

The lockfile is written at the project root:

```text
lock.octagon
```

No `oct.lock` or `lock.oct` file is produced.

The file is a data-only Octagon literal, not Oct source. It has no `package` declaration, records, functions, or timestamps.

## Model

The root fields are:

- `LockVersion`, currently `1`;
- `GeneratedBy`, deterministically `"oct pkg lock"`;
- `Root`, with the current project `Name` and `Version`;
- `Packages`, the resolved package graph.

The root project is not a package node. Package nodes represent exact `Name@Version` registry packages and exact dependency edges only.

Package nodes record source and diagnostic metadata:

- `Name`, `Version`, `Kind`;
- `SourceKind`, `Source`, `Ref`, `ResolvedCommit`, `Path`;
- `Registry`, `RegistryPath`;
- `Mutable`;
- `Dependencies`.

## Source policies

Git packages record the original `Ref` and the resolved full 40-character `ResolvedCommit`. Locked sync checks out `ResolvedCommit` rather than the mutable `Ref`.

Local packages are allowed, but they are marked `Mutable: true`, record no commit, and are not reproducible. PM6 does not compute content digests for local source trees.

Wrapper packages are ordinary package nodes. Locking and locked syncing materialize wrapper source only; PM6 does not build sidecars or create `.oct/wrappers`.

## Locked sync validation

`oct pkg sync --locked` requires `lock.octagon`, rejects unsupported lock versions, validates the lock graph, and compares the current manifest root identity and direct dependencies to the lock graph before installing.

Locked sync fails when the manifest adds a dependency, removes a dependency leaving stale lock packages, changes a dependency version, or no longer matches the lock root identity.

Locked sync uses lock entries as source authority and does not consult registries to choose versions or sources for locked packages. Registry fields remain diagnostic.

## Deferred work

PM6 does not implement content-addressed `.octpkg` artifacts, source/tree/registry digests, signing, P2P, federation, publishing, auth, registry mirrors, binary sidecar distribution, wrapper build changes, wrapper runtime discovery changes, semver ranges, `latest`, or solver/backtracking behavior.
