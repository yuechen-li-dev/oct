# PM4 — Git registry sources and deterministic transitive sync

PM4 extends the PM2 local registry MVP without introducing federation, publishing, lockfiles, artifacts, signing, mirrors, auth, P2P, semver ranges, `latest`, or solver/backtracking behavior.

## Registry schema

`PackageEntry` keeps the PM2 fields and adds optional `Ref`:

```text
Name, Version, Kind, SourceKind, Source, Ref, Path, Description
```

Compatibility rules:

- PM2 local registry entries that omit `Ref` remain valid.
- `SourceKind: "local"` accepts omitted or empty `Ref` only.
- `SourceKind: "git"` requires non-empty `Ref`.
- Unsupported source kinds fail and list `local` and `git`.

## Git acquisition

For Git entries, sync creates a temporary clone under the project-local package cache area, runs `git clone <Source> <tmp-repo>`, checks out `Ref` detached, reads `git rev-parse HEAD`, resolves `Path` inside the checked-out repository, and copies that package root into `.oct/packages/<Name>/<Version>/`.

The copy path uses the same source-copy constraints as local registry sync: skip `.git`, `.oct/wrappers`, and `.oct/packages`; reject symlinks; validate manifest name, version, and kind; then write `.oct-package-source.oct` metadata before replacing the final package directory.

Git diagnostics include package name/version, registry, source, ref, and operation for clone, checkout, and rev-parse failures. Missing `git` reports a clear error.

## Metadata

`.oct-package-source.oct` now records:

```text
Ref
ResolvedCommit
```

Local sources write empty strings for both. Git sources write the registry ref and the resolved commit. Mutable refs are allowed for PM4; when the ref is not a full 40-character hex commit SHA, sync prints a warning and records the resolved commit. This metadata is diagnostic only and is not a lockfile.

## Deterministic graph sync

`oct pkg sync` now plans and syncs exact direct plus transitive dependency graphs for source-less registry dependencies.

Rules:

- graph nodes are `<Name>@<exact-version>`;
- direct and transitive dependencies are processed in deterministic name/version/source order;
- identical nodes are deduped;
- same package name with different exact versions errors with dependency chains;
- cycles error with cycle paths;
- missing and ambiguous registry dependencies include the requiring chain and searched registry diagnostics from registry resolution;
- exact versions only; no ranges, `latest`, solver, or backtracking;
- direct `oct pkg add --registry <name>` narrows only that add operation; sync uses project registry config for all dependencies;
- explicit `Dependency.Source` behavior remains supported and may appear transitively.

Final sync order is dependency-before-importer for registry nodes discovered through manifests. The root project is never copied. Wrapper package source sync remains source acquisition only and does not build sidecars or alter wrapper runtime discovery/build lifecycle.
