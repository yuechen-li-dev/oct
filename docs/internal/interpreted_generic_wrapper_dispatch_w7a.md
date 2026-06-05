# W7a interpreted generic wrapper dispatch design

## 1. Executive summary

W7a is a design and audit milestone only. It does not implement interpreted generic wrapper dispatch, change language syntax, change manifest schema, build sidecars, run sidecars, or alter package-manager/runtime discovery behavior.

### Current interpreted wrapper behavior

Interpreted wrapper calls are handled by an internal builtin registry. The registry maps builtin names to Go handlers and is assembled when the interpreter is constructed. Those handlers evaluate Oct argument expressions directly inside the interpreter, decode `interpret.Value` values, perform in-process Go work, and map fallible failures to `ValueError` through the standard wrapper error helper.

This means interpreted standard-library wrappers are currently not generic Octxiliary calls. They are host/runtime builtins. A manifest-declared wrapper function is not a callable interpreted fallback unless it also happens to be an ordinary Oct function or an internal builtin.

### Current compiled wrapper behavior

Compiled mode has a generic Octxiliary lowering path for manifest-declared wrapper functions. During lowering, the compiler looks at `project.Package.Wrappers`, matches a callee by `WrapperFunction.OctName`, validates source stub signature information against manifest metadata when a source function exists, packs arguments into typed `octxiliary.Value` envelopes, sends a request to the sidecar identified by `Wrapper.SidecarCommand`, validates the response kind, and either returns a compiled fallible result or panics for non-fallible boundary errors.

Compiled generic wrapper lowering coexists with older hardcoded IO file/directory sidecar helpers and with direct compiled helpers for some non-sidecar builtins.

### Gap for third-party wrappers

The W5 `oct new wrapper-library <Name>` scaffold intentionally declares raw wrapper metadata in `manifest.oct`, such as `WrapperFunction { OctName: "EchoStringRaw" ... }`, but does not generate an Oct raw function/stub named `EchoStringRaw`. W6 kept strict source-level stub agreement deferred because Oct does not currently have a supported inert extern/raw-stub declaration form.

That leaves a mode gap:

- compiled mode can use manifest metadata for generic wrapper calls when there is an Oct source function for the manifest `OctName`;
- interpreted mode cannot call manifest-only raw functions at all;
- adding unsupported syntax now would invalidate the staged W5/W6 design.

### Recommended dispatch design

W7b should implement **manifest-only raw function resolution** for interpreted dispatch:

1. Ordinary source functions keep normal precedence.
2. If no ordinary source function exists for the resolved package/name, the interpreter may resolve a matching manifest `WrapperFunction.OctName` in that package.
3. That manifest-only raw function routes through a generic Octxiliary interpreted client.
4. Source/function collisions with manifest raw names should be validation errors, not silent overrides.
5. Raw manifest functions should be treated as package-local implementation details by convention and by diagnostics. External users should normally call public Oct wrapper APIs, not raw names.

This design keeps W5 scaffolds valid, avoids inventing `@extern`, `EXTERNAL { ... }`, or raw stub syntax, and gives interpreted and compiled modes a convergent generic Octxiliary ABI.

### Recommended next implementation milestone

The single recommended next milestone is:

> **W7b — implement interpreted generic wrapper dispatch M0 with manifest-only raw function resolution and `OCT_WRAPPER_PATH` test sidecars.**

W7b should include the minimum generic interpreted client needed for manifest-only raw calls in third-party-style packages, plus focused tests using a small sidecar fixture. It should not migrate standard-library interpreted wrappers.

### Deferred features

Deferred beyond W7b:

- new source syntax (`@extern`, `EXTERNAL { ... }`, raw stubs);
- strict source-level stub agreement for missing raw names;
- native sidecar build lifecycle;
- package cache discovery and lockfiles;
- native permission prompts;
- PATH search broadening;
- sidecar registry/federation/P2P;
- stdlib interpreted wrapper migration;
- record returns and advanced transport expansion beyond compiled parity;
- package-manager sidecar execution.

## 2. Current interpreted wrapper architecture inventory

### Registry location and composition

The interpreted wrapper registry is `wrapperBuiltinRegistry` in `internal/interpret/wrapper_bridge.go`. It stores `map[string]wrapperBuiltinHandler`, where each handler has the shape:

```go
type wrapperBuiltinHandler func(i *interpreter, env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error)
```

The registry is composed with `newWrapperBuiltinRegistry(...)`. Interpreter construction currently passes family-specific handler maps:

- `xlsxWrapperBuiltins()`
- `imageWrapperBuiltins()`
- `plotWrapperBuiltins()`
- `pdfWrapperBuiltins()`
- `jsonWrapperBuiltins()`
- `fileWrapperBuiltins()`
- `pathWrapperBuiltins()`
- `directoryWrapperBuiltins()`
- `csvWrapperBuiltins()`
- `artifactWrapperBuiltins()`
- `archiveWrapperBuiltins()`
- `compressionWrapperBuiltins()`
- `hashWrapperBuiltins()`
- `regexWrapperBuiltins()`
- `timeWrapperBuiltins()`

This composition happens in both `ExecuteMain`'s inline interpreter construction and `newInterpreter(...)` for test/function entrypoints.

### How raw wrapper builtins resolve today

Interpreted calls are first flattened to a direct callee name when possible. If the name is recognized by `internal/builtin`, `evalBuiltinCallExpr(...)` handles it. Inside `evalBuiltinCallExpr`, after several bespoke builtin families and random helpers, the interpreter checks `i.wrappers.has(callee)`. If the internal wrapper registry contains the name, the call is dispatched to `i.wrappers.eval(...)`.

That path is builtin-name based. It does not inspect package manifest metadata. It also does not require a sidecar process.

### Fallible wrapper return shape

Wrapper handlers return fallible Oct errors with `wrapperErrorResult(callee, err)`. The helper produces `evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: ...}}}`. The visible message shape is:

```text
<Callee>: <WrapperErrorKind>: <message>
```

where current wrapper error categories include `InvalidArgument`, `InvalidHandle`, `NotFound`, `Conflict`, `InvalidData`, and `BackendFailure`.

Function execution treats a fallible function returning a `ValueError` as `callResult{hasError: true}`. Callers using `?` then propagate that error through the ordinary fallible machinery.

### Generic Octxiliary client path in interpreted mode

There is no current interpreted generic Octxiliary client path. Interpreted wrappers use internal Go handlers and in-process stores, not `internal/octxiliary` request/response framing.

### Sidecar missing/error conditions in interpreted mode

For the internal interpreted wrapper registry, sidecar missing/error conditions generally do not exist because no sidecar is spawned. Missing files, bad regexes, invalid handles, codec failures, and similar operational problems are represented with wrapper error values produced in process.

The older compiled sidecar path documents missing sidecar as a fallible `Error`, but that is compiled runtime behavior, not interpreted behavior.

### Stdlib wrappers that bypass Octxiliary in interpreted mode

All standard-library interpreted wrapper handlers currently bypass Octxiliary and run in process through the registry. This includes the current wrapper families represented by the registered handler maps above: Xlsx, Image, Plot, Pdf, Json, File, Path, Directory, Csv, Artifact, Archive, Compression, Hash, Regex/Text, and Time.

Some compiled standard-library functions also bypass Octxiliary by design, such as simple path helpers and `FileExists`, but that is separate from interpreted wrapper dispatch.

## 3. Current compiled generic wrapper architecture inventory

### Metadata consumption during lowering

Compiled generic wrapper lowering uses manifest metadata loaded into `project.Package.Wrappers`. The lowerer calls `genericWrapperMetadataForCallee(...)`, which resolves either:

- an unqualified identifier against the current package's wrappers; or
- a package-qualified field access against an imported package's wrappers.

`findGenericWrapperFunction(...)` walks the package wrappers and returns metadata containing package name, wrapper family, sidecar command, `OctName`, `WireName`, args, return, fallibility, and declared transport types.

When metadata is found, lowering validates:

- manifest return type versus the Oct source function's inferred return type;
- manifest fallibility versus the Oct source function's fallibility;
- argument count;
- each manifest argument type against the lowered argument type, with `Int<unit>` allowed to match runtime `Int`;
- whether each argument/return transport is supported or declared;
- record returns are rejected unless the declared transport return is a handle.

It then emits `MIRGenericOctxiliaryCall` rather than a normal source-function body call.

### Sidecar discovery in generated code

Generated code uses `__octOctxiliarySidecarPath(sidecarCommand)`. The search order is:

1. sibling executable beside the generated `.octbin` / current executable (`filepath.Dir(os.Args[0])`);
2. `OCT_WRAPPER_PATH`, interpreted as either a directory containing the sidecar command or an explicit executable whose basename matches the sidecar command;
3. failure with an error message naming the sidecar and telling the user to set `OCT_WRAPPER_PATH` or place it beside `.octbin`.

It does not search arbitrary `PATH` by default.

### Client/process lifecycle in generated code

Generated code caches generic clients by `SidecarCommand` in a global map. A client owns one sidecar process, stdin pipe, stdout pipe, request id counter, mutex, and persistent error state. Each generic call locks the client, increments the request id, writes one frame, reads one response, and validates it.

The helper is process-per-sidecar-command and reuse-per-call, not spawn-per-call.

### Request argument encoding

Generated generic wrapper calls pack arguments into `[]octxiliary.Value` and send:

```go
octxiliary.Request{ID: id, Family: family, Function: wireName, Args: args, HasArgs: true}
```

Current generic encoders cover:

- `Void` as `ValueVoid`;
- `Int` and `Int<unit>` as `ValueInt`;
- `Float` as `ValueFloat`;
- `Bool` as `ValueBool`;
- `String` as `ValueString`;
- `String[]` as `ValueStringArray`;
- `String[][]` as `ValueStringMatrix`;
- `Float[]` as `ValueFloatArray`;
- `Bytes` as `ValueBytes`;
- declared handle records as `ValueHandle` with `HandleFamily`, `HandleType`, and positive handle id;
- declared record arguments as `ValueRecord` with ordered named fields, recursively using supported field transports.

### Response decoding

Generated code calls `__octOctxiliaryGenericCall(...)` with the expected response kind. The helper:

1. validates the request before writing;
2. writes a frame and reads a response frame;
3. parses and validates the response;
4. converts `ok:false` to an error;
5. requires a typed value on success;
6. checks the response `Value.Kind` against the expected kind.

Return extraction maps typed values back to generated Go values. Handle returns get extra validation of family, handle type, and positive id. Record returns remain rejected by metadata/lowering policy, except handle records represented through `ValueHandle`.

### Missing sidecar diagnostics

Generic sidecar discovery failure currently produces:

```text
Octxiliary sidecar "<sidecarCommand>" not found; set OCT_WRAPPER_PATH or place it beside .octbin
```

The older hardcoded IO helper has a similar message for `octxiliary-io`.

### Conceptual pieces to share with interpreted dispatch

W7b should mirror, not import generated code:

- metadata lookup by package and `OctName`;
- sidecar discovery order and missing-sidecar wording;
- request/response validation using `internal/octxiliary`;
- typed value packing/extraction rules;
- sidecar client cache keyed by sidecar command;
- handle validation;
- mapping `ok:false`, protocol errors, EOF, and missing sidecar to fallible `Error` where the manifest function is fallible.

## 4. Current project/package metadata path

### Where wrapper metadata lives after load

`internal/pkgmgr.LoadManifestMetadata(...)` extracts `ManifestMetadata.Wrappers` from `manifest.oct`. The wrapper metadata type aliases `internal/manifestwrapper.Metadata`, with functions represented by `FunctionMetadata`.

`internal/project` aliases that metadata as `project.WrapperMetadata` and stores it on each loaded `project.Package`:

```go
type Package struct {
    ...
    Wrappers []WrapperMetadata
}
```

`project.Program` then contains all loaded packages in `Program.Packages`.

### Interpreter entrypoint metadata availability

The interpreter entrypoints receive a full `project.Program`. `newInterpreter(program, stdout)` already iterates `program.Packages` and has access to each `project.Package`, but it currently copies only imports, records, enums, functions, and flows into interpreter maps. It does not copy or index `pkg.Wrappers`.

Therefore the metadata exists at interpreter construction time, but no interpreted dispatch table is currently built from it.

### Package-manager/compiler-only paths

Wrapper metadata is currently used by:

- package-manager manifest extraction and planning (`oct pkg wrappers`);
- registry artifact rendering;
- compiled lowering.

Interpreted runtime ignores it. W7b needs to thread/index wrapper metadata into the interpreter object, not invent a new package-manager path.

### What needs to be threaded through

W7b should add an interpreted wrapper metadata index built from `program.Packages` during interpreter construction. A useful shape is:

```text
package name -> OctName -> wrapper call metadata
```

The metadata should retain at least:

- package name;
- wrapper name;
- family;
- protocol;
- sidecar command;
- transport type declarations;
- `OctName`;
- `WireName`;
- manifest arg types;
- manifest return type;
- manifest fallibility.

It should also preserve enough origin context for diagnostics: package, wrapper name, family, sidecar command, wire name, and raw `OctName`.

## 5. Raw wrapper function representation problem

### W5/W6 tension

The current representation tension is intentional:

- `WrapperFunction.OctName` declares the Oct-facing raw wrapper function name.
- W5 wrapper scaffolds may not define an Oct source function with that name.
- Current Oct has no supported inert extern/raw-stub declaration.
- W6 deferred strict manifest/source stub agreement so W5 scaffolds remain valid.

A W7 design must not require syntax that does not exist and must not make W5 scaffolds invalid.

### Option A: manifest-only raw function resolution

The interpreter treats manifest `OctName` as a callable raw function when no ordinary source-level function exists in that package.

A future wrapper package can write:

```oct
package Echo

fn EchoString(text: String) -> String ! Error {
    return EchoStringRaw(text)?
}
```

with manifest metadata declaring `EchoStringRaw` even though no source-level `fn EchoStringRaw(...)` exists.

Pros:

- requires no syntax change;
- preserves W5 scaffolds;
- aligns with W6's staged validation decision;
- gives interpreted third-party wrappers a generic dispatch route;
- keeps the raw function shape data-only in `manifest.oct` until Oct has a real native-boundary declaration syntax.

Cons:

- source readers cannot jump to a raw function declaration;
- strict stub agreement remains incomplete;
- raw manifest names can look like ordinary functions unless tooling makes them visible;
- collision validation becomes important.

### Option B: generated raw stub source

`oct new wrapper-library` could eventually generate source-level raw stubs after a supported syntax exists.

Pros:

- source declares the native boundary explicitly;
- typechecker/tooling can validate signatures in a familiar source location;
- package authors get better editor/navigation support.

Cons:

- impossible to do correctly today without inventing unsupported syntax;
- would invalidate W5's current “valid metadata but not callable yet” posture if required too early;
- risks duplicating ABI metadata across source and manifest before a strict agreement model exists.

### Option C: future `EXTERNAL { ... }` block

A future explicit native-boundary block could be sugar over manifest ABI declarations.

Pros:

- clear native boundary in source;
- could group raw declarations and permissions;
- could become the right place for strict source/manifest agreement.

Cons:

- language design work, not W7a/W7b implementation scope;
- requires reference documentation, parser/typechecker work, and migration policy;
- still needs a manifest/package metadata relationship.

Do not implement this in W7.

### Option D: `[External]` attribute

An attribute on a source function could mark it as externally implemented.

This is likely not preferred because current Oct attributes read primarily as test/tool metadata rather than core native-boundary syntax. Reusing attributes for a process/native boundary would make an operational capability look like incidental annotation and would not naturally solve manifest agreement, sidecar permissions, or transport declarations.

Do not implement this in W7.

### Recommendation for W7b

Pick exactly one target: **manifest-only raw function resolution**.

W7b should implement manifest-only raw function resolution because it is the only option that:

- does not change syntax;
- keeps W5 scaffolds valid;
- respects W6's deferred strict-stub decision;
- lets public Oct functions call raw manifest functions once interpreted dispatch exists;
- moves interpreted and compiled modes toward the same Octxiliary ABI.

Strict source-level agreement can remain future work until Oct has `EXTERNAL { ... }` or another supported raw-stub representation.

## 6. Interpreted dispatch design

### Desired future package shape

Source:

```oct
package Echo

fn EchoString(text: String) -> String ! Error {
    return EchoStringRaw(text)?
}
```

Manifest:

```oct
WrapperFunction { OctName: "EchoStringRaw" WireName: "EchoString" Args: ["String"] Return: "String" Fallible: true }
```

### Resolution order

Recommended W7b resolution order:

1. Preserve all existing special forms and ordinary builtin behavior.
2. Resolve ordinary source functions and flow constructors normally.
3. If no ordinary source function/flow exists for the resolved package/name, check manifest wrapper raw functions in that package.
4. If one manifest raw function exists, route through generic Octxiliary interpreted dispatch.
5. If none exists, report the existing undefined-function style error.

Ordinary source functions should not be silently overridden. Existing code should keep running unchanged.

### Collision policy

If a source function and manifest wrapper function have the same name in the same package, W7b should reject the package/program with a validation error. It should not choose source sometimes and manifest other times.

The preferred validation point is project load or interpreter metadata-index construction, because collisions are package facts and should be discovered before the first call where possible.

### Package qualification and imports

For an unqualified call inside package `Echo`, `EchoStringRaw(...)` resolves against package `Echo` after source functions fail.

For a qualified call, `Echo.EchoStringRaw(...)`, the interpreter should require that `Echo` is an imported/loaded package just like ordinary package-qualified source function calls. W7b should not create a global flat namespace of raw wrappers.

Raw manifest functions are **package-local implementation details by convention**, not intended public APIs. However, until Oct has visibility/private syntax, a package-qualified raw name may be technically callable from another package if the package is imported and the raw name is present. Diagnostics and docs should direct users to call public wrapper APIs.

### Missing sidecar and sidecar errors

Missing sidecar should return a fallible `Error` for fallible manifest functions. Sidecar `ok:false` should also map to `Error`.

For non-fallible manifest functions, W7b must mirror compiled mode carefully: process/protocol/sidecar errors cannot be represented as `T ! Error`, so they should become runtime errors. New third-party wrapper authoring guidance should recommend fallible public APIs for process-boundary operations.

### Type/transport mismatch

There are two classes:

- **Program/manifest ABI mismatch before making the sidecar request**: argument count/type incompatibility between runtime values and manifest transport should be an interpreter/runtime invariant or package validation error. It indicates broken wrapper package metadata or a call that passed through typechecking incorrectly.
- **Sidecar/protocol mismatch after the request**: malformed response, wrong response kind, bad handle family/type, missing typed value, or invalid record fields should be converted to `Error` for fallible manifest functions and runtime error for non-fallible functions.

This mirrors compiled behavior while preserving fallible process-boundary semantics.

## 7. Function resolution and collision policy

### No silent override

Manifest raw wrapper names must not silently override real source functions. Silent override would make source behavior depend on manifest metadata in a surprising way and could break existing standard-library packages whose public functions intentionally have the same names as manifest `OctName` for compiled lowering.

### Source collision policy

For W7b, if a manifest-only raw dispatch table is enabled for a package and a manifest `OctName` equals a source function in the same package, report a validation error.

Important staging detail: current stdlib manifests often declare public API names as `OctName` because compiled lowering intercepts those public functions. W7b must not apply the new collision error globally to existing stdlib packages unless it would break current behavior. The safe M0 policy is:

- enable manifest-only raw resolution for calls whose source function is missing;
- add collision diagnostics for third-party-style manifest raw names in new tests;
- avoid stdlib migration and avoid invalidating existing stdlib public-name manifests.

A stricter package-wide collision validator can follow once stdlib manifests are migrated or a raw/public distinction exists.

### Duplicate `OctName` policy

W6 rejects duplicate wrapper names, families, and sidecar commands within the same manifest. It validates wrapper/function structure and unsupported transports, but package-wide `OctName` uniqueness across wrappers is not yet a documented hard guarantee.

W7b should require package-wide uniqueness of manifest raw `OctName` values for interpreted dispatch. Otherwise a deterministic call target cannot be selected safely. If two wrappers in the same package expose the same `OctName`, W7b should reject the interpreted raw-dispatch index with a diagnostic naming both wrappers.

### Imports and accidental public exposure

Imported package raw wrapper names should not become unqualified names in the importing package. They should only be reachable through explicit package qualification, and wrapper authoring docs should say that raw names are implementation details. Public APIs should be ordinary Oct functions that call raw manifest functions inside their package.

## 8. Fallibility policy

Process-boundary wrapper functions should normally be fallible. Spawn, discovery, handshake, frame IO, sidecar runtime failures, and protocol mismatches are operationally fallible even when the logical operation appears deterministic.

W7b has to account for existing metadata: standard-library manifests currently include both fallible and non-fallible functions, such as time “now” helpers and image width/height/format helpers. Therefore W7b should support both manifest `Fallible: true` and `Fallible: false` in the generic client if it shares metadata code broadly.

Recommended conservative policy:

- W7b supports both fallible and non-fallible manifest functions only to match existing wrapper metadata and compiled behavior.
- New third-party wrapper-library guidance should recommend fallible raw functions and fallible public APIs for sidecar calls.
- For `Fallible: true`, missing sidecar, sidecar `ok:false`, handshake failures, malformed responses, EOF/crash, and response type mismatches map to `ValueError`.
- For `Fallible: false`, those same failures become runtime errors, because the Oct signature has no `Error` channel.
- Non-fallible third-party raw wrappers should be treated as advanced/exceptional, not the default scaffold recommendation.

## 9. Transport support scope for W7b

W7b should match current compiled generic wrapper transport support as closely as the interpreter value model permits.

Recommended W7b supported set:

- `Void`
- `Int`
- `Int<unit>` erased to runtime `Int`
- `Float`
- `Bool`
- `String`
- `String[]`
- `String[][]`
- `Float[]`
- `Bytes`
- declared record arguments if the interpreter can pack `ValueRecord` fields in declared order;
- declared handles as arguments and returns, represented as single-field records with `Handle: Int` and transported as `ValueHandle` with family/type/id metadata.

Recommended W7b deferrals/gaps:

- declared record returns remain deferred while generic compiled lowering still rejects record returns;
- nested/recursive records remain out of scope;
- dynamic `Any`, Complex, matrices/vectors as generic transports, callbacks, and cross-family handles remain out of scope;
- `Void` arguments should be accepted only where manifest metadata and call arity make sense; ordinary user-facing raw functions should not require users to pass a `Void` value.

If implementation pressure requires a smaller W7b M0, the minimum acceptable transport proof should cover `String`, `Bytes`, `Int`, `Bool`, and `Void` plus fallible error propagation, then explicitly leave record/handle parity for W7c. The preferred target remains compiled-parity for existing generic transports.

## 10. Sidecar client lifecycle in interpreter

### Spawn/reuse policy

The interpreter should spawn one sidecar process per sidecar command and reuse it for subsequent calls. Spawn-per-call would be slow, would not preserve sidecar-owned handles, and would diverge from compiled behavior.

### Cache key

Cache clients by sidecar command, with metadata checks ensuring one command is not ambiguously associated with incompatible families in the same interpreted program. W6 already rejects duplicate sidecar commands within one manifest; package-graph conflicts should follow the package-manager plan policy over time.

### EOF/crash handling

If a sidecar exits, closes stdout, returns EOF, or produces invalid frames:

- the active fallible call returns an `Error` naming the sidecar command, family, wire name, and raw `OctName`;
- non-fallible calls report a runtime error;
- the client should be marked failed and not reused silently;
- restart policy should be deferred unless W7b tests need a minimal “drop bad client and respawn next call” behavior.

A no-restart sticky error is simpler and deterministic for W7b.

### Cleanup

Interpreter-owned sidecar clients should be closed when `ExecuteMain` or `ExecuteFunction...` exits. This can be a `defer interpreter.closeWrapperClients()` in entrypoints. Cleanup should close stdin and wait/kill as needed to avoid leaked processes. W7b should test this enough to avoid orphan sidecars in Go tests.

### Test isolation

Tests should isolate sidecar clients per interpreter execution. Do not share sidecars across unrelated test files/packages. This matches the current interpreter construction model and prevents handle state leakage.

### Discovery behavior

For W7b, interpreted sidecar discovery should match compiled generic discovery where feasible:

1. sidecar beside the `oct` executable or current executable;
2. `OCT_WRAPPER_PATH` as a directory or explicit sidecar executable path;
3. no general `PATH` search.

For tests/dev, `OCT_WRAPPER_PATH` should be the primary mechanism.

## 11. Sidecar discovery policy for W7b

W7b should not implement W8 package build lifecycle or package-cache sidecar discovery. It should support exactly:

1. **Sibling executable lookup** using the current executable directory, because compiled mode uses sibling lookup and this gives local installations a consistent behavior.
2. **`OCT_WRAPPER_PATH` lookup** as either:
   - a directory containing `SidecarCommand`; or
   - an explicit executable path whose basename equals `SidecarCommand`.
3. **No package-cache lookup** because there is no native build/install lifecycle yet.
4. **No default `PATH` lookup** to avoid PATH hijacking and accidental native execution.
5. **No package-manager execution**. `oct pkg wrappers` remains planning-only and must not execute native code.

The missing-sidecar message should remain close to compiled mode:

```text
Octxiliary sidecar "<sidecarCommand>" not found; set OCT_WRAPPER_PATH or place it beside oct
```

or, if W7b chooses to reuse exact compiled wording, “beside .octbin” is acceptable but less precise for interpreted mode. A mode-specific phrase such as “beside the oct executable” is clearer.

## 12. Error and diagnostic policy

Diagnostics should include package name, sidecar command, family, wire name, and raw `OctName` where possible.

Recommended messages/categories:

- **Missing sidecar**: `wrapper <Package>.<OctName> (family <Family>, wire <WireName>): Octxiliary sidecar "<SidecarCommand>" not found; set OCT_WRAPPER_PATH or place it beside the oct executable`.
- **Unsupported function from sidecar** (`ok:false` with unsupported): return sidecar error as `Error`, prefixed with wrapper context.
- **Handshake failure**: `wrapper <Package>.<OctName> sidecar <SidecarCommand>: handshake failed: <detail>`.
- **Malformed response**: `wrapper <Package>.<OctName> sidecar <SidecarCommand>: malformed Octxiliary response: <detail>`.
- **Transport type mismatch before request**: runtime invariant/package validation error: `wrapper <Package>.<OctName> argument <n> expects <ManifestType>, got <RuntimeKind>`.
- **Response type mismatch**: for fallible functions, `Error`: `wrapper <Package>.<OctName> expected <ReturnKind>, got <ResponseKind>`; for non-fallible functions, runtime error.
- **Non-fallible sidecar error**: runtime error: `runtime error: wrapper <Package>.<OctName> sidecar error: <message>`.
- **Crash/EOF**: for fallible functions, `Error`: `wrapper <Package>.<OctName> sidecar <SidecarCommand> closed unexpectedly`; for non-fallible functions, runtime error.
- **Invalid manifest raw function call**: package validation or runtime invariant depending on when detected: `wrapper <Package>.<OctName> has unsupported transport type <Type>`.
- **Collision with source function**: validation error: `wrapper manifest raw function <Package>.<OctName> conflicts with source function of the same name`.
- **Duplicate `OctName` across wrappers**: validation error naming both wrappers and the duplicate raw name.

The error string returned through Oct's `Error` channel should be concise, but logs/test failures should have enough context to diagnose sidecar selection.

## 13. Relationship to stdlib wrappers

W7b should not migrate standard-library interpreted wrappers. It should leave the internal wrapper builtin registry and all existing interpreted stdlib tests alone.

Recommended relationship:

- Internal registry remains the first implementation path for standard-library interpreted builtins.
- W7b adds a generic fallback path for manifest-declared wrapper raw functions that are not handled by ordinary source functions or internal builtins.
- Existing stdlib fast paths remain for performance, behavior stability, and avoiding broad regression risk.
- Later milestones may migrate selected stdlib interpreted wrappers to generic Octxiliary dispatch after third-party behavior is proven and diagnostics match compiled mode.

This staged approach avoids disrupting stdlib packages whose manifests currently use public API names as `OctName` for compiled lowering.

## 14. Test plan for W7b

W7a does not add these tests. W7b should.

### Fixtures

Add a third-party-style wrapper package fixture with:

- `manifest.oct` declaring raw manifest functions such as `EchoStringRaw`;
- ordinary public Oct functions that call those raw names;
- no source-level raw stub function;
- no generated sidecar build lifecycle.

Add a small Go sidecar fixture using `pkg/octxiliary`, for example `octxiliary-echo`, built by Go tests into a temporary directory. Tests set `OCT_WRAPPER_PATH` to that directory.

### Interpreted success/error tests

Focused tests should cover:

- successful manifest-only raw call through a public wrapper function;
- sidecar `ok:false` maps to `Error` and propagates through `?`;
- missing sidecar diagnostic includes sidecar command and package/raw function context;
- unsupported sidecar function maps to `Error`;
- `String` argument/return;
- `Bytes` argument/return or bytes length;
- `Int` argument/return;
- `Bool` argument/return;
- `Void` return;
- `String[]`, `String[][]`, and `Float[]` if included in W7b parity scope;
- declared record argument packing if included;
- declared handle return/argument round trip if included;
- source/manifest collision validation;
- duplicate manifest `OctName` validation across wrappers;
- cleanup/no leaked sidecar process where test infrastructure can observe it.

### Compiled mode regression tests

Compiled mode should remain passing. If W7b adds new fixtures, existing compiled generic tests under `Language/Testing/CompiledOctxiliary` should remain green. Do not require package manager to build sidecars.

### Package-manager behavior tests

`oct pkg wrappers` should remain planning-only. Tests should assert that it reports sidecars/registry output without executing native code.

## 15. Candidate next milestones

### A. W7b — implement interpreted generic wrapper dispatch M0

Pros:

- directly closes the W7a design gap;
- proves W5 scaffold direction without new syntax;
- gives third-party wrappers a real interpreted path;
- can be tested with `OCT_WRAPPER_PATH` and a tiny fixture sidecar;
- converges interpreted mode toward compiled generic Octxiliary behavior.

Cons:

- touches interpreter call resolution and runtime process lifecycle;
- requires careful collision policy to avoid stdlib regressions.

### B. W7b — first add package-wide raw wrapper name conflict validation

Pros:

- lowers ambiguity before dispatch work;
- safer metadata foundation.

Cons:

- does not make wrappers callable;
- may require stdlib manifest migration or staging exceptions because current stdlib compiled metadata uses public names;
- risks failing the motivating W7 capability without improving runtime behavior.

### C. W7b — implement a test-only fixture before real dispatch

Pros:

- can clarify sidecar build/test mechanics;
- low production risk.

Cons:

- no user-visible capability;
- can become throwaway scaffolding;
- still leaves the dispatch design unproven.

### D. W8 — native wrapper build lifecycle

Pros:

- addresses real package installation/build UX.

Cons:

- too broad before interpreted dispatch exists;
- adds permission, cache, lockfile, and lifecycle questions explicitly outside W7 scope;
- would not solve manifest-only raw function representation by itself.

### Recommendation

Recommend exactly one next implementation milestone:

> **W7b — implement interpreted generic wrapper dispatch M0 with manifest-only raw function resolution and `OCT_WRAPPER_PATH` test sidecars.**

## 16. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Raw wrapper names accidentally become public API | Document raw names as implementation details; prefer public Oct wrapper functions; require explicit package qualification for imported raw names. |
| Source/manifest name collisions | Do not silently override; validate collisions for raw-dispatch packages and stage stricter package-wide checks carefully around current stdlib manifests. |
| Sidecar process leaks | Interpreter-owned client manager with explicit close in entrypoint defers; tests verify cleanup where practical. |
| Test flakiness from sidecar lifecycle | Build sidecars into temp dirs, set `OCT_WRAPPER_PATH`, isolate interpreter executions, avoid shared global clients. |
| PATH hijacking | Do not search `PATH`; use sibling executable and `OCT_WRAPPER_PATH` only. |
| Native code execution in tests | Keep sidecar tests focused, explicit, local, and built from repository test fixtures; keep `oct pkg wrappers` inert. |
| Diverging interpreted vs compiled diagnostics | Reuse compiled wording where sensible and add wrapper context consistently. |
| Record/handle transport mismatch | Start from compiled transport rules; validate records/handles before request and after response; defer record returns. |
| Non-fallible wrapper error semantics | Support only for parity; map boundary failures to runtime errors; recommend fallible third-party APIs. |
| Stdlib interpreted wrapper regressions | Do not migrate stdlib interpreted wrappers in W7b; add generic fallback only for manifest-only raw calls. |
| No strict stub validation yet | Keep manifest-only resolution staged; surface this as a known validation gap until explicit raw syntax exists. |
| Duplicate `OctName` across wrappers | W7b interpreted index rejects ambiguous names package-wide. |
| Compiled/interpreted metadata drift | Build one conceptual metadata structure and test against existing manifest fixtures. |

## 17. Final recommendation

### Exact recommended W7b scope

Implement interpreted generic wrapper dispatch M0 with:

- manifest-only raw function resolution;
- ordinary source functions and current builtins preserved;
- package/name metadata index built from `project.Program.Packages[*].Wrappers`;
- package-wide manifest raw `OctName` uniqueness for interpreted dispatch;
- collision diagnostics that do not invalidate existing stdlib interpreted paths;
- generic Octxiliary client cache keyed by `SidecarCommand`;
- sibling executable plus `OCT_WRAPPER_PATH` discovery only;
- typed value packing/extraction matching compiled generic support where practical;
- fallible error mapping for `Fallible: true` and runtime errors for `Fallible: false`;
- focused third-party-style tests using a tiny `pkg/octxiliary` sidecar fixture.

### Exact W7b non-goals

Do not:

- change Oct syntax;
- add `@extern`;
- add `EXTERNAL { ... }`;
- add raw wrapper stub syntax;
- change manifest schema;
- implement sidecar build/download/install lifecycle;
- execute native code from `oct pkg wrappers`;
- add PATH lookup;
- add package lockfiles;
- add permission prompts;
- migrate standard-library interpreted wrappers;
- broaden registry/federation/P2P/package-cache behavior.

### Deferred features

Defer:

- strict source-level manifest/stub agreement;
- explicit raw/extern syntax design;
- package-cache sidecar discovery;
- native build lifecycle and permissions;
- stdlib interpreted generic migration;
- record returns;
- cross-family handle sharing;
- sidecar restart/health policy beyond deterministic cleanup;
- broad transport expansion beyond compiled parity.

W7a ends in **success** for the design milestone if this document is accepted: it identifies the current architectural gap, selects manifest-only raw function resolution for W7b, and constrains the next implementation to a convergent but narrow interpreted Octxiliary dispatch path.

## W7b follow-up status

W7b implements the recommended M0 path described here: interpreted execution can resolve manifest-only raw `WrapperFunction.OctName` entries as a fallback after ordinary source-function resolution, dispatch them through Octxiliary, discover sidecars beside the `oct` executable or through `OCT_WRAPPER_PATH`, and keep `oct pkg wrappers` planning-only. See `docs/internal/interpreted_generic_wrapper_dispatch_w7b.md` for the implemented scope and deferrals.
