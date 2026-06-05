# W7b interpreted generic wrapper dispatch M0

## Summary

W7b implements interpreted generic Octxiliary dispatch for third-party-style wrapper packages whose public Oct functions call manifest-declared raw wrapper functions that do not have source-level `fn` declarations.

A package can now use this shape in interpreted execution:

```oct
package Main

fn EchoString(text: String) -> String ! Error {
    return EchoStringRaw(text)?
}
```

with `manifest.oct` declaring `EchoStringRaw` as a `WrapperFunction`. The raw manifest function is resolved only after ordinary source-function resolution fails, then dispatched through an Octxiliary sidecar.

## Baseline before W7b

The W7b fixture at `Language/Testing/InterpretedOctxiliary/valid` records the motivating baseline. Its manifest declares raw functions such as `EchoStringRaw`, `ByteLengthRaw`, `BoolNotRaw`, and `NotOkRaw`, while `TestWrapper.oct` exposes public Oct functions that call those raw names. Before W7b, interpreted execution failed at the first raw call because `EchoStringRaw` was not an ordinary source function or an internal interpreted wrapper builtin.

The failure mode was an unresolved raw-name diagnostic during interpreted test execution, for example an undefined-variable/function resolution error for `EchoStringRaw` rather than an Octxiliary request.

## Implemented interpreter metadata index

Interpreter construction now builds an `interpretedWrapperIndex` from `project.Program.Packages[*].Wrappers`. The index is package-scoped:

- lookup key: package name + `WrapperFunction.OctName`;
- value: wrapper name, family, sidecar command, protocol, transport metadata, raw Oct name, wire name, argument transport types, return transport type, and fallibility;
- duplicate manifest raw `OctName` values across wrappers in the same package are rejected for interpreted dispatch;
- source functions are not inserted into or overridden by the wrapper index.

This keeps raw wrapper names package-local and avoids a global flat raw-function namespace.

## Resolution order

Interpreted call resolution now follows this order:

1. Existing special forms, builtins, namespaced builtin aliases, assertions, `SkipTest`, and the internal interpreted wrapper builtin registry keep their current behavior.
2. Ordinary source functions and flows resolve normally.
3. If no ordinary source function exists for the resolved package/name, the interpreter checks the package-scoped manifest raw wrapper index.
4. A matching manifest-only raw function dispatches through generic Octxiliary interpreted transport.
5. Missing names use the existing undefined-function diagnostics.

If a source function and manifest raw function share a name, the source function wins in W7b. This preserves current stdlib interpreted wrapper behavior and avoids invalidating packages whose manifests still use public API names. A stricter collision validator remains future work once raw stubs or stdlib manifest migration are ready.

## Generic Octxiliary interpreted client

The M0 client is interpreter-owned and process-scoped for one interpreter execution:

- one sidecar process is spawned per `Wrapper.SidecarCommand`;
- clients are cached by sidecar command and reused for subsequent calls;
- requests use `octxiliary.Request{Family, Function: WireName, Args, HasArgs: true}`;
- responses are parsed and validated with `internal/octxiliary`;
- `ok:false`, response kind mismatches, protocol errors, process errors, and missing sidecars become Oct `Error` values for fallible wrapper functions;
- the same boundary failures are runtime errors for non-fallible wrapper functions.

The client cache is closed at interpreter execution end for `ExecuteMain` and test/function execution entrypoints, closing sidecar stdin and waiting for process exit so tests do not leave orphan sidecars.

Restart-after-crash policy remains deferred. A crashed/EOF client is marked unusable and returns deterministic errors for later calls in the same interpreter execution.

## Sidecar discovery

W7b intentionally supports only two discovery locations:

1. an executable with the sidecar command name beside the current `oct` executable;
2. `OCT_WRAPPER_PATH`, either:
   - a directory containing the sidecar command; or
   - an explicit executable path whose basename equals the sidecar command.

W7b does **not** add PATH lookup, package-cache lookup, build/install lookup, lockfiles, native permission prompts, registry discovery, federation, or P2P discovery.

Missing sidecar diagnostics include package, raw name, family, wire name, command, and search guidance, for example:

```text
wrapper Main.EchoStringRaw (family TestWrapper, wire TestEchoString): Octxiliary sidecar "octxiliary-test-wrapper" not found; set OCT_WRAPPER_PATH or place it beside the oct executable
```

## Transport support

W7b interpreted generic transport packing and unpacking supports the current M0 parity set:

- `Void`;
- `Int`;
- `Int<unit>` as runtime `Int`;
- `Float`;
- `Bool`;
- `String`;
- `String[]`;
- `String[][]`;
- `Float[]`;
- `Bytes`;
- declared handle arguments and returns;
- declared record arguments.

Argument packing validates argument count, runtime kinds, declared handle records (`Handle: Int` and positive ID), and declared record fields in manifest-declared order. Bytes and slice-like values are defensively copied where they cross the boundary.

Record returns remain unsupported, matching compiled generic wrapper behavior. Dynamic `Any`, callbacks/function values, streams, matrices/vectors as generic transport values, Complex generic transport, cross-family handles, native build lifecycle, and stdlib wrapper migration are deferred.

## Fallibility

W7b keeps the process boundary policy explicit:

- `Fallible: true` wrapper metadata maps sidecar/process/protocol failures to Oct `Error` values that can be propagated with `?` or handled with `match`.
- `Fallible: false` wrapper metadata maps sidecar/process/protocol failures to deterministic runtime errors.

Third-party wrapper docs should continue to recommend fallible public raw/process-boundary functions unless a non-fallible boundary is deliberately safe.

## Test fixture

The focused interpreted fixture is `Language/Testing/InterpretedOctxiliary/valid`:

- `manifest.oct` declares a wrapper package with raw manifest functions and sidecar command `octxiliary-test-wrapper`;
- `TestWrapper.oct` exposes public Oct functions that call manifest-only raw names and intentionally omits source-level raw declarations;
- `interpreted_generic_wrapper_w7b.octest` covers `String`, `Bytes -> Int`, `Bool`, fallible sidecar errors, and source-function precedence.

The Go tests build the existing deterministic sidecar fixture `cmd/octxiliary-test-wrapper` into a temporary directory, set `OCT_WRAPPER_PATH`, and run interpreted Oct tests against the fixture. A missing-sidecar test verifies the contextual diagnostic. The package-manager wrapper command remains planning-only and is not used to execute sidecars.

## Stdlib relationship

Existing interpreted standard-library wrappers continue to use the internal wrapper builtin registry. W7b generic dispatch is a fallback for manifest-only raw functions; it is not a replacement for current stdlib interpreted builtins and does not migrate Hash, Compression, Time, Text, Archive, Json, Csv, Plot, Xlsx, Image, Pdf, or IO wrappers.

## Package-manager behavior

`oct pkg wrappers` remains planning-only. It reports wrapper sidecar build plans and can write registries, but W7b does not make it build, download, install, discover, or execute sidecars. Sidecars are used only when interpreted execution calls a manifest raw wrapper function and discovery finds the sidecar.

## Deferred features

Deferred beyond W7b:

- new Oct syntax such as `@extern`, `EXTERNAL { ... }`, or raw wrapper stubs;
- strict source-level raw-stub agreement;
- source/manifest collision rejection for stdlib-compatible public-name manifests;
- native sidecar build/download/install lifecycle;
- package-cache sidecar discovery;
- PATH lookup;
- package lockfiles;
- native permission prompts;
- registry, federation, or P2P lookup;
- interpreted stdlib wrapper migration;
- record returns;
- dynamic `Any`, callbacks, streams, matrix/vector generic transport, and Complex generic transport;
- cross-family handles;
- sidecar restart policy.
