# W6 wrapper manifest validation and registry artifact audit

## Summary

W6 is a wrapper metadata hardening/audit milestone. The implementation remains planning-only: `oct pkg wrappers` loads manifests, produces wrapper sidecar build-plan metadata, and optionally writes a deterministic `.octagon` registry artifact, but it does not build native modules, download sidecar source, execute sidecars, change runtime sidecar discovery, or add interpreted generic wrapper dispatch.

This pass found that most structural wrapper manifest checks already exist in the shared `internal/manifestwrapper` extractor and are reached by both project loading and package-manager metadata loading. W6 makes one small hardening change: duplicate `Wrapper.SidecarCommand` values are now rejected during manifest wrapper extraction, before package-manager plan conflict validation. Strict source-level stub agreement is intentionally **not** enabled yet because the W5 golden-path wrapper-library scaffold declares `EchoStringRaw` as manifest ABI metadata without a source-level Oct stub, and the current language/reference set does not provide a supported inert extern/raw-stub declaration syntax for sidecar-backed functions.

## Current schema and field requirements

### Package manifest fields

`PackageManifest` currently has these required fields for all packages:

- `Name: String`
- `Version: String`
- `Description: String`
- `Dependencies: Dependency[]`

The recognized optional fields are:

- `Kind: String`
- `EntryMilestone: String`
- `Wrappers: Wrapper[]`

The default package kind is `pure` when `Kind` is omitted or empty. `Kind: "wrapper"` requires present, non-empty `Wrappers`; `pure` and `experiment` reject non-empty wrappers. `EntryMilestone` is accepted only for `experiment` packages.

### Dependency fields

`Dependency` currently requires:

- `Name: String`
- `VersionRequirement: String`

It optionally accepts:

- `Source: String`

`oct pkg wrappers` deliberately ignores dependencies without `Source` while inspecting the current package, so non-fetchable dependencies such as `OctStd` do not block current-package wrapper inspection or registry rendering.

### Wrapper fields

`Wrapper` currently requires:

- `Name: String`
- `Family: String`
- `Protocol: String`
- `SidecarCommand: String`
- `GoModuleDir: String`
- `Functions: WrapperFunction[]`

It optionally accepts:

- `TransportTypes: WrapperTransportType[]`

`GoModuleDir` remains the only accepted native source/build location field in production schema. W6 does not add `SourceDir`, `BuildKind`, `OutputName`, permissions, platform lists, or system dependencies because adding them to manifests would require changing every manifest-local `record Wrapper` declaration and making registry semantics look more stable than the native build lifecycle actually is. The recommended future migration remains `SourceDir`/`BuildKind`, with `GoModuleDir` retained as a compatibility alias until a native build milestone can consume the new fields.

### WrapperFunction fields

`WrapperFunction` requires:

- `OctName: String`
- `WireName: String`
- `Args: String[]`
- `Return: String`
- `Fallible: Bool`

All values are metadata only in package-manager planning. `OctName` names the raw/stub function the manifest claims is sidecar-backed; `WireName` names the sidecar method used by the Octxiliary wire protocol.

### Transport type fields

`WrapperTransportType` requires:

- `Name: String`
- `Kind: String`
- `Fields: WrapperTransportField[]`

`WrapperTransportField` requires:

- `Name: String`
- `Type: String`

`TransportTypes` is optional at wrapper level. When present, the manifest must also declare the `WrapperTransportType` and `WrapperTransportField` records with the exact required fields.

## Current validation coverage

### Manifest declaration and literal validation

Project and package-manager manifest loaders both validate the manifest record declarations before reading wrapper metadata:

- `PackageManifest`, `Dependency`, `Wrapper`, `WrapperFunction`, and optional transport records must use the expected field names and types.
- The `Manifest` function must return a `PackageManifest` record literal.
- Manifest literals reject unsupported fields and duplicate fields.
- Required string fields must be string literals and non-empty.
- `Functions` and `TransportTypes` must be literal arrays of the expected record literals.
- `Functions` must be non-empty for each wrapper.
- Transport `Fields` must be non-empty.

The package-manager loader returns more specific literal errors such as unsupported field names, duplicate fields, and wrong literal types. The project loader intentionally maps some declaration/body failures to the broader project manifest diagnostics used by existing project load tests.

### Wrapper conflicts

| Conflict | Current W6 behavior | Stage |
|---|---|---|
| Duplicate `Wrapper.Name` in one manifest | Rejected by wrapper extraction. | Manifest metadata extraction / project load / package-manager load |
| Duplicate `Wrapper.Family` in one manifest | Rejected by wrapper extraction. | Manifest metadata extraction / project load / package-manager load |
| Duplicate `Wrapper.SidecarCommand` in one manifest | Rejected by wrapper extraction after W6. Previously caught only later as a package-manager plan conflict. | Manifest metadata extraction / project load / package-manager load |
| Duplicate sidecar command across planned packages | Rejected by wrapper plan conflict validation. | Package-manager wrapper planning |
| Duplicate wrapper family across planned packages | Rejected by wrapper plan conflict validation. | Package-manager wrapper planning |
| Duplicate resolved `GoModulePath` across planned packages | Rejected by wrapper plan conflict validation. | Package-manager wrapper planning |
| Duplicate `WrapperFunction.OctName` inside one wrapper | Rejected by wrapper function extraction. | Manifest metadata extraction / project load / package-manager load |
| Duplicate `WrapperFunction.WireName` inside one wrapper | Rejected by wrapper function extraction. | Manifest metadata extraction / project load / package-manager load |
| Duplicate transport type name inside one wrapper | Rejected by transport type extraction. | Manifest metadata extraction / project load / package-manager load |
| Duplicate transport field name inside one transport type | Rejected by transport field extraction. | Manifest metadata extraction / project load / package-manager load |

Gaps: duplicate `OctName`/`WireName` are scoped to a single wrapper, not the whole package graph. That is acceptable for now because families/sidecars are already unique in the plan, but future interpreted dispatch should decide whether package-wide raw stub names need stronger uniqueness diagnostics.

### Protocol and path validation

- `Protocol` must be exactly `octxiliary.v0`.
- `GoModuleDir` must be non-empty.
- `GoModuleDir` must be package-local and relative.
- Absolute Unix paths, absolute Windows-style drive paths, leading backslash paths, `..` path traversal segments, and cleaned paths that escape the package are rejected.

### Transport validation

Supported scalar/list transport names are currently:

- `Void`
- `Int`
- dimensioned `Int<...>`
- `Float`
- `Bool`
- `String`
- `String[]`
- `String[][]`
- `Float[]`
- `Bytes`

Function args and returns may use supported built-in transport types or a declared transport type name. Declared transport types support these kinds:

- `record`
- `handle`

Handle schemas are hardened and must be exactly one field:

```oct
WrapperTransportField { Name: "Handle" Type: "Int" }
```

Record transport fields may use supported field transport types. `Void` is not valid as a record field type. Declared record transport types are allowed in function args, but declared record returns remain rejected because generic wrapper lowering does not support record returns. Declared handle returns are allowed.

Current validation does **not** compare declared `record` transport field names/types against Oct package record definitions. It also does not verify that declared handle transport type names correspond to actual Oct record declarations. That is a W7/W8 blocker once raw stub definitions and interpreted dispatch have a stable representation.

## Where errors happen today

### Parse/type declaration time

Basic Oct syntax and record/function declaration shape errors happen when `manifest.oct` is lexed and parsed. Invalid manifest record declarations are then rejected by project/package manifest declaration validators.

### Project load time

`internal/project` validates `manifest.oct` while loading packages. This catches wrapper kind rules, required wrapper/transport record declarations, unsupported wrapper literal fields, duplicate wrapper names/families/sidecar commands, invalid protocol, invalid `GoModuleDir`, invalid transport schemas, unsupported transport types, duplicate functions, and invalid function metadata before project packages are returned.

Project loading stores wrapper metadata on loaded packages, but it does not perform source-level stub agreement against package `.oct` files.

### Package-manager metadata load time

`internal/pkgmgr` independently loads manifest metadata for package-manager commands. It shares `internal/manifestwrapper`, so the same structural wrapper checks run when `oct pkg wrappers` reads the current project or fetched dependency manifests.

### Package-manager wrapper planning time

`BuildWrapperPlanForProject` loads the current manifest and fetches only dependencies with explicit `Source`. Dependencies without source are skipped. The wrapper plan then sorts sidecars deterministically and rejects cross-planned-package conflicts in sidecar command, wrapper family, and resolved Go module path.

Planning remains inert. It does not download Go module dependencies, run `go build`, execute sidecars, write registries unless `--registry-out` is supplied, or change runtime discovery.

### Registry rendering time

`BuildOctxiliaryRegistry` copies plan metadata into a deterministic registry model. `RenderOctxiliaryRegistryOctagon` emits data-only Octagon text with one registry record and deterministic array ordering. `WriteOctxiliaryRegistryOctagon` requires a `.octagon` output path and creates parent directories if necessary.

### Compiler lowering time

Existing compiled generic wrapper lowering is limited to the currently supported wrapper transports and sidecar runtime behavior. It is not a W6 validation layer for third-party manifest/stub agreement. Some unsupported generic wrapper cases, such as record returns, are intentionally rejected earlier at manifest metadata extraction when represented in wrapper manifests.

## Manifest/stub agreement decision

W6 does **not** enable strict source-level stub agreement by default.

Reason:

- W5 intentionally generates a wrapper-library manifest with `WrapperFunction { OctName: "EchoStringRaw" ... }` but does not generate an Oct source stub for `EchoStringRaw`.
- Current supported/reference syntax does not provide an inert `extern`/raw sidecar declaration form. W6 non-goals explicitly reject adding `@extern`, `EXTERNAL { ... }`, interpreted generic dispatch, or native build lifecycle.
- Requiring every manifest `OctName` to exist now would make the W5 golden-path scaffold invalid even though it is the canonical current-compatible third-party wrapper fixture.

The staged validation model after W6 is therefore:

1. **Always-on structural ABI validation**: manifest declarations, kind rules, wrapper uniqueness, protocol, sidecar command, source path, transport schema, function metadata, unsupported transports, handle schema, and unsupported record returns.
2. **No always-on strict stub agreement yet**: missing source functions, arg count mismatch, arg type mismatch, return mismatch, and fallibility mismatch are documented gaps rather than hard failures.
3. **Future strict mode**: W7/W8 should add a validation mode or command once raw wrapper stubs have a supported source representation. That strict mode should compare `WrapperFunction.OctName` to actual package function declarations, including argument count, argument types, return type, and fallibility.

This keeps W5 scaffolds valid and avoids inventing language syntax.

## Registry artifact contents after W6

The registry artifact currently includes:

- registry version (`octxiliary.registry.v0`),
- package name,
- wrapper name,
- family,
- protocol,
- sidecar command,
- `GoModuleDir`,
- resolved `GoModulePath`,
- transport type names/kinds/fields,
- wrapper function `OctName`,
- wrapper function `WireName`,
- wrapper function `Args`,
- wrapper function `Return`,
- wrapper function `Fallible`.

The registry does not currently include package version or package kind even though those are present in `WrapperPackagePlan`. That is a small future registry-model extension. It also does not include `SourceDir`, `BuildKind`, output name, permission metadata, platforms, or system dependencies because those fields are not recognized by the production manifest schema yet.

The output order is deterministic:

- wrapper plan sidecars sort by package name, wrapper name, family, sidecar command;
- registry sidecars are sorted again with the same key;
- functions and transport types preserve manifest order within a sidecar;
- strings are rendered with safe quoting.

## W6 behavior changes made

- Duplicate `Wrapper.SidecarCommand` values in the same manifest are now rejected during shared wrapper extraction.
- A package-manager manifest test now covers same-manifest duplicate sidecar command rejection.
- This audit document records the current validation matrix, staged stub-agreement decision, registry contents, and deferred native build metadata work.

## Remaining gaps and recommended next steps

1. Add a strict wrapper ABI check once raw/stub functions have a supported source-level shape. It should validate `OctName` existence, arg count, arg types, return type, and fallibility.
2. Compare declared `record` and `handle` transport type names against actual Oct record declarations once strict package-source validation is available.
3. Compare declared record transport fields against actual Oct record fields where current type metadata permits it.
4. Add package version/kind to the registry model if downstream consumers need complete package identity.
5. Introduce `SourceDir`/`BuildKind` as additive manifest fields only when a native build lifecycle milestone can define their semantics without making `GoModuleDir` users invalid.
6. Consider a future `oct pkg wrapper-check` command after strict ABI validation exists. Adding a command in W6 would mostly duplicate `oct pkg wrappers` without resolving the missing raw-stub representation.

## Explicit inconsistencies surfaced

- W2 recommends future `SourceDir`/`BuildKind` metadata, but current production manifests and W5 scaffolds use only `GoModuleDir`. W6 keeps production schema unchanged and documents the migration as future work.
- W6's original goal asks for manifest/stub agreement validation, but W5's golden-path scaffold deliberately has manifest-only raw function metadata. W6 resolves this by keeping strict stub agreement staged rather than invalidating W5 scaffolds or adding unsupported syntax.
- Registry output includes ABI and sidecar source-path metadata, but it does not yet include package version/kind despite those being available in wrapper package plans. This is a registry-model gap, not a runtime blocker.
