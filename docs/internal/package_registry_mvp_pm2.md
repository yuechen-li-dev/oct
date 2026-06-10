# PM2 local package registry/source-resolution MVP

## Summary

PM2 implements the local, source-oriented package registry frame designed in PM1. It intentionally proves only the direct golden path:

```text
local registry.oct
→ oct pkg registry add
→ oct pkg add SignalTools@0.1.0
→ oct pkg sync
→ .oct/packages/SignalTools/0.1.0
→ project loader can import/use SignalTools
→ oct test .
```

This is not federation, publishing, trust, HTTP registry support, Git registry cloning, artifact distribution, binary sidecar distribution, or lockfile work.

## Project-local registry config

Configured registries live at:

```text
.oct/registries.oct
```

The file is project-local and has no user-global equivalent. `oct pkg registry add <name> <path>` creates it when missing, appends new registries after existing entries, rejects duplicate names, and stores relative paths exactly as provided. Registry paths are resolved relative to the project root when loaded.

Registry names are validated as stable simple names matching `[A-Za-z][A-Za-z0-9_-]*`.

## Registry index

A registry root contains:

```text
registry.oct
```

PM2 supports this registry schema:

```oct
package Registry

record RegistryIndex {
    Packages: PackageEntry[]
}

record PackageEntry {
    Name: String
    Version: String
    Kind: String
    SourceKind: String
    Source: String
    Path: String
    Description: String
}

fn Registry() -> RegistryIndex {
    return RegistryIndex {
        Packages: [
            PackageEntry {
                Name: "SignalTools"
                Version: "0.1.0"
                Kind: "library"
                SourceKind: "local"
                Source: "../SignalTools"
                Path: "."
                Description: "Signal helper functions"
            }
        ]
    }
}
```

Validation is deliberately strict: package names use the existing strict PascalCase package-name rules, versions must be exact non-empty text, `Kind` is `library`, `experiment`, or `wrapper`, and `SourceKind` is only `local`. `Description` may be empty because it is descriptive metadata rather than a sync safety field. Duplicate `Name` + `Version` entries inside one registry are rejected.

## Resolution behavior

`oct pkg add <Name>@<exact-version> [--registry <name>]` resolves the package before editing `manifest.oct`.

Rules:

- versions are exact strings only;
- `latest`, ranges, comparison constraints, wildcards, and empty versions are rejected;
- package names are case-sensitive;
- no solver or semver interpretation exists in PM2;
- without `--registry`, all configured registries are scanned in project-local order;
- zero matches report a missing package/version error listing searched registries;
- multiple matches report ambiguity and ask for `--registry <name>`;
- `--registry <name>` narrows lookup to one configured registry and rejects unknown registry names.

`oct pkg add` writes a manifest dependency without `Source`:

```oct
Dependency { Name: "SignalTools" VersionRequirement: "0.1.0" }
```

The implementation uses a conservative text edit for canonical manifest dependency lists. Unsupported formatting fails clearly rather than corrupting the manifest.

## Sync behavior

`oct pkg sync` preserves legacy explicit `Dependency.Source` behavior. Dependencies with `Source` still use the existing package-manager cache/fetch path.

Dependencies without `Source` are registry-resolved by `Name` and exact `VersionRequirement`. PM2 copies `SourceKind: "local"` packages into:

```text
.oct/packages/<Name>/<Version>/
```

The local source path is resolved relative to the registry root unless absolute. The registry `Path` field is resolved inside that source root. PM2 rejects absolute `Path` values and obvious `..` traversal. Symlinks inside copied packages are rejected. The copy skips `.git`, `.oct/wrappers`, and `.oct/packages`.

Existing synced package directories are replaced via a temporary directory under `.oct/packages/<Name>/.tmp-*` followed by rename to reduce half-written final-cache risk.

After copy, PM2 validates the copied `manifest.oct`: manifest name and version must match the requested dependency, and registry kind must match manifest kind (`library` maps to missing/empty/normalized pure manifests; `experiment` maps to `Kind: "experiment"`; `wrapper` maps to `Kind: "wrapper"`).

PM2 writes source metadata beside each synced package:

```text
.oct/packages/<Name>/<Version>/.oct-package-source.oct
```

The metadata records package name, version, registry name/path, source kind, source, and path. It intentionally has no timestamp and is not a lockfile.

## Loader integration

The project loader can load manifest-declared dependencies from exact project-local synced package paths:

```text
<project-root>/.oct/packages/<Name>/<Version>/
```

The requested version comes from the importing package manifest's `VersionRequirement`. If a different version exists in `.oct/packages/<Name>/`, it is not used as a fallback. Existing repository search roots and legacy package-manager cache fallback remain in place.

## Wrapper packages

Wrapper packages can be registry-synced as source. `oct pkg sync` does not build sidecars and does not create `.oct/wrappers`. Package source sidecar directories such as `sidecars/...` are copied into `.oct/packages/<Name>/<Version>/` as ordinary source. The explicit `oct pkg build-wrappers --allow-native` lifecycle remains current-package only, and dependency wrapper sidecar build selection remains deferred.

## Deferred work

PM2 intentionally does not implement federation, P2P, HTTP registries, registry Git clone, Git source entries, publishing, auth, namespace reservation, mirrors, lockfiles, `oct.lock`, content-addressed `.octpkg` artifacts, registry signing, binary sidecar distribution, semver ranges, `latest`, solver behavior, wrapper runtime discovery changes, wrapper sync builds, or dependency sidecar build selection.

## Notes and limitations

- `OctStd` remains treated as a built-in/non-fetchable dependency for sync purposes.
- Transitive registry resolution is not generalized in PM2; the implemented path is direct dependencies from the current project manifest.
- Path traversal rejection is lexical/clean-path based for PM2. Symlinks are rejected during copy rather than followed.
