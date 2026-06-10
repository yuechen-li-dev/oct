# W8b local Go-module native wrapper build M0

## Summary

W8b adds the first explicit package-managed native wrapper build path:

```sh
oct pkg build-wrappers --allow-native
```

The command is intentionally narrow. It reads the current package manifest, selects wrapper sidecars declared by that current package only, and builds Go-module sidecars from each wrapper's existing `GoModuleDir` metadata. It writes binaries to:

```text
.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe]
```

The build command never executes sidecars. The `go` tool is invoked only after the user supplies the explicit `--allow-native` flag.

## Behavior

- `oct pkg build-wrappers` without `--allow-native` fails before planning or compiling native sidecars.
- Unknown flags and extra arguments fail with usage text.
- Non-wrapper packages and wrapper packages with no sidecars succeed as a no-op and print `No wrapper sidecars to build.`
- Current-package wrapper metadata is validated through the existing manifest wrapper validation path.
- Dependency graph sidecars are not built, and the command does not run `oct pkg sync` or fetch package dependencies.
- Each sidecar is built with:

  ```sh
  go build -o <output-path> .
  ```

  using `<project-root>/<GoModuleDir>` as the working directory.
- The source directory must exist and contain `go.mod`.
- Failures include package, wrapper, sidecar command, source directory, output path, and captured Go output when available.

## Deterministic output

The command prints a deterministic summary containing:

- platform;
- output directory;
- sidecar count;
- native permission status;
- package name/version;
- wrapper name;
- family;
- sidecar command;
- protocol;
- module / `GoModuleDir`;
- resolved source directory;
- output path;
- function count.

After success it prints:

```text
Built wrapper sidecars: <n>
Set OCT_WRAPPER_PATH=.oct/wrappers/<platform> to use these sidecars with current runtime discovery.
```

That guidance is required because W8b deliberately does **not** add project-local `.oct/wrappers` runtime lookup. Current runtime sidecar discovery still requires sibling placement or `OCT_WRAPPER_PATH`.

## Preserved inert inspection lane

`oct pkg wrappers` remains planning-only. It may inspect manifests and render a registry artifact with `--registry-out`, but it does not build Go modules, create `.oct/wrappers`, execute sidecars, alter runtime discovery, or download native dependencies.

## Explicit deferrals

W8b does not add:

- dependency/package graph sidecar builds;
- package-cache or user-cache wrapper installation;
- package fetches solely for wrapper builds;
- lockfiles or digest binding;
- registry/federation/P2P native build flows;
- prompts, sandboxing, or permission persistence;
- arbitrary build scripts;
- non-Go build systems;
- cross-compilation;
- manifest schema changes such as `BuildKind`, `SourceDir`, or `OutputName`;
- Oct syntax changes;
- `@extern` or `EXTERNAL { ... }`;
- stdlib sidecar migration;
- broadened `PATH` lookup;
- Octxiliary wire protocol changes.
