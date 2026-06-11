# PM7 canonical Oct registry seed

PM7 seeds a source-controlled first-party registry at:

```text
Registry/registry.oct
```

The canonical registry makes the package manager dogfood real Oct packages without adding hosted registry behavior. In PM7 the registry is local project data: consumers configure it with `oct pkg registry add`, entries point at package source in this repository, and sync copies source into `.oct/packages/<Name>/<Version>/`.

## Registry entry policy

PM7 includes only package roots that are registry-ready without restructuring:

- the package root has a `manifest.oct`;
- the manifest package identity is the source of truth for `Name`, `Version`, and manifest `Kind`;
- omitted/empty/pure manifest kind maps to registry `library`;
- manifest `Kind: "wrapper"` maps to registry `wrapper`;
- manifest `Kind: "experiment"` maps to registry `experiment`;
- the registry entry passes the current registry loader validation;
- first-party transitive dependencies are themselves registry-ready or intentionally skipped by package-manager builtin dependency handling (`OctStd`).

PM7 intentionally does not add semver ranges, `latest`, publishing, auth, hosted registry resolution, mirrors, signing, `.octpkg` artifacts, package tree digests, binary sidecar distribution, P2P behavior, or wrapper/native build changes.

Registry entries use `SourceKind: "local"`, `Source: ".."`, and `Path: "Libraries/<PackageDir>"`. The source path is relative to `Registry/`, so entries remain portable inside a checkout and can later be moved into a dedicated registry repository or hosted mirror with a separate source strategy.

## Package audit

Package names, versions, kinds, and dependencies below were read from manifests rather than inferred from directory names. `Mathematics` remains the canonical math package name; PM7 does not create `Math` as an alias.

| Package | Path | Version | Kind | Registry-ready? | Notes |
| --- | --- | --- | --- | --- | --- |
| Analysis | Libraries/Analysis | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Archive | Libraries/Archive | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| Artifact | Libraries/Artifact | 0.1.0 | library | No | Manifest identity conflicts with current package-manager name validation because Artifact is reserved as a top-level command family name; exclude until name-policy/registry handling is resolved intentionally. |
| ArtifactUsage | Libraries/ArtifactUsage | 0.1.0 | library | No | Usage/test package rather than reusable library; it depends on Artifact, which is excluded by current registry name validation, and its tests import Markdown, whose package root has no manifest yet. |
| Complex | Libraries/Complex | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Compression | Libraries/Compression | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| Cooking | Libraries/Cooking | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Csv | Libraries/Csv | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd, IO; declares first-party dependencies IO. |
| Deployment | Libraries/Deployment | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| DifferentialEquations | Libraries/DifferentialEquations | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Distributions | Libraries/Distributions | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Geometry | Libraries/Geometry | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Hash | Libraries/Hash | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| IO | Libraries/IO | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| IfErrNotEqualNil | Libraries/IfErrNotEqualNil | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Image | Libraries/Image | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| Interpolation | Libraries/Interpolation | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Json | Libraries/Json | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| LinearAlgebra | Libraries/LinearAlgebra | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Markdown | Libraries/Markdown |  |  | No | No manifest.oct; not a standalone package root for PM7. |
| Mathematics | Libraries/Mathematics | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Mechanics | Libraries/Mechanics | 0.6.0 | library | Yes | pure library; depends on OctStd. |
| Numerics | Libraries/Numerics | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Octomata | Libraries/Octomata | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Optimization | Libraries/Optimization | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Pdf | Libraries/Pdf | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| Physics | Libraries/Physics | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Plot | Libraries/Plot | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| RF | Libraries/RF | 0.2.4 | library | Yes | pure library; depends on OctStd. |
| Random | Libraries/Random | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Signal | Libraries/Signal | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Simulation | Libraries/Simulation | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Statistics | Libraries/Statistics | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| String | Libraries/String | 0.1.0 | library | No | Manifest identity conflicts with current package-manager name validation because String is a built-in scalar/type family name; exclude until name-policy/registry handling is resolved intentionally. |
| Structures | Libraries/Structures | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Text | Libraries/Text | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| Thermofluids | Libraries/Thermofluids | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Time | Libraries/Time | 0.1.0 | wrapper | Yes | wrapper with native sidecar metadata; sync copies source only; depends on OctStd. |
| UI | Libraries/UI | 0.1.0 | library | Yes | pure library; depends on OctStd. |
| Wireless | Libraries/Wireless |  |  | No | No manifest.oct; not a standalone package root for PM7. |

## Included packages

`Registry/registry.oct` includes every audited package marked registry-ready above. The included set contains pure libraries and wrapper packages. Wrapper package entries are source entries only: `oct pkg sync` copies the wrapper package source and manifest metadata, but does not build native sidecars and does not create `.oct/wrappers`.

The canonical math entry is `Mathematics@0.1.0`. It is intentionally not named `Math`.

## Excluded packages and known inconsistencies

PM7 excludes the following package directories rather than silently reshaping them:

- `Libraries/Markdown`: no `manifest.oct`; not a standalone registry package yet.
- `Libraries/Wireless`: no `manifest.oct`; not a standalone registry package yet.
- `Libraries/Artifact`: manifest identity is `Artifact@0.1.0`, but current package-manager registry validation reuses `oct new` package-name validation, where `Artifact` is reserved as a top-level command family name. This is an implementation/name-policy inconsistency between existing first-party library manifests and registry admission rules.
- `Libraries/String`: manifest identity is `String@0.1.0`, but current package-manager registry validation reuses `oct new` package-name validation, where `String` is reserved as a built-in scalar/type family name. This is another implementation/name-policy inconsistency.
- `Libraries/ArtifactUsage`: usage/test package rather than reusable library. It also depends on `Artifact`, which is excluded by the validation inconsistency above, and its tests import `Markdown`, which has no manifest.

These exclusions should be revisited intentionally in a later milestone. PM7 does not rename packages, add aliases, or weaken registry validation as part of the registry seed.

## Dogfood tests

PM7 dogfood coverage verifies:

1. `Registry/registry.oct` parses through the registry loader.
2. Every registry entry points to an existing package root.
3. Every registry entry matches the package manifest name, version, and kind mapping.
4. No duplicate `Name@Version` entries exist.
5. `Mathematics@0.1.0` appears and no `Math` alias appears.
6. A temp consumer copied from `examples/PackageRegistryDogfood` can add the canonical registry, add `Mathematics@0.1.0`, sync, run an Oct test importing `Mathematics`, write `lock.octagon`, remove synced packages, sync with `--locked`, and run the Oct test again.
7. A wrapper package entry can sync as source without creating `.oct/wrappers`.

## Future hosting path

`Registry/registry.oct` can later be split into a dedicated registry repository or mirrored by a hosted registry. Hosted/HTTP registry behavior remains future work. Package signing, artifacts, P2P distribution, registry mirrors, auth, namespace reservation, publishing, `.octpkg` artifacts, tree digests, and binary sidecar distribution remain out of scope for PM7.
