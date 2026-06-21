# Oct CLI Quick Reference

Usage:

- `oct <command> [options]`
- `oct --help`
- `oct <command> --help`

Common commands:

- `go run ./cmd/oct test <path>`
- `go run ./cmd/oct test <path> --suite <suite>`
- `go run ./cmd/oct test <path> --execution compiled`
- `go run ./cmd/oct test <path> --all-packages`
- `go run ./cmd/oct artifact <path>`
- `go run ./cmd/oct artifact <path> --execution compiled`
- `go run ./cmd/oct fmt <path> --mode en-llm --check`
- `go run ./cmd/oct fmt <path> --mode en-llm-compact --check`
- `go run ./cmd/oct bench <path> --profile`
- `go run ./cmd/oct new library SignalTools`
- `go run ./cmd/oct new experiment BrownNoiseKalman`
- `go run ./cmd/oct new wrapper-library OpenCV`
- `go run ./cmd/oct new application MyApp`

`oct test` defaults to running tests only from the selected entry package/root. Imported packages are still loaded for typechecking, but their tests are excluded unless `--all-packages` is specified.

## `oct make` and `Make.octest`

`Make.oct` is ordinary Oct source for build plan, config, target metadata, action helper functions, and direct-backend `FlowTarget` Octomata actions. The build lane is explicit: `oct make` evaluates `Plan() -> Make.Plan` and executes selected build targets, but it does not automatically run `Make.octest`.

`Make.octest` is the normal xUnit-style same-package companion for `Make.oct`. Run pure plan/config checks directly with:

```sh
oct test Make.octest
```

Use `[Fact]` and `[Theory]` for ordinary assertions over `Plan()`, config helpers, target metadata, the default target, inputs/outputs/deps, and `with`-based profile composition. `[Artifact]` may later be useful for pure plan snapshots, but it must not hide build execution; `[Benchmark]` is not important for ordinary Make plan tests. Side-effectful Make primitive coverage requires explicit make authority and belongs in `Libraries/Make`, explicit sidecar/integration lanes, or Go CLI tests for `oct make`.

### `oct make` reporting

`oct make --plan-out <file.octagon>` evaluates `Plan()`, validates the typed plan, and writes a read-only `MakePlanSnapshot` Octagon artifact. The snapshot records `Version`, `MakeFile`, `Default`, `Config`, and all command/function/flow/phony target metadata (`Name`, `Inputs`, `Outputs`, `Deps`, `Program`, `Args`, `Env`, `Cwd`, `Function`, `Flow`, and `MaxSteps` where applicable). It is plan evidence only; execution decisions belong in `trace.octagon`.

`oct make explain [target] [--file <path>]` evaluates and validates the plan, selects the default target when no target is supplied, computes the selected dependency closure, and prints stable would-run/would-skip decisions without executing command targets, function targets, or flow targets. Reasons use the same canonical staleness names as traces: `NoOutputs`, `MissingOutput`, `MissingInput`, `InputNewerThanOutput`, `DependencyRan`, `UpToDate`, `Always`, `Phony`, and `DryRunWouldRun`.

`oct make doctor [--file <path>]` is a read-only project health report. It prints the make file path, profile, state directory, direct backend, default target, target counts by kind, validation/dependency status, state and trace paths with existence, referenced command programs, and Make attribute diagnostics. The attribute section reports whether `Plan()` is conventional or marked with `[MakePlan]`, lists explicit `[Pure]`, `[NoWhile]`, and `[RequiresAuthority]` markers, warns when a function directly calls Make host primitives without `[RequiresAuthority]`, warns when a `[Pure]` function directly calls a Make host primitive, and offers best-effort typed Make idiom suggestions for obvious shell-shaped probes such as `Make.Exec("bash", ["-c", "command -v ..."])` or shell env gates that should use `Make.Tool` or `Make.Env`. These diagnostics are advisory in ATTR-MAKE2: unmarked `Plan()` remains valid, direct-call warnings do not fail doctor, and no transitive authority/purity analysis or enforcement is performed yet. Doctor does not run target commands or require native toolchains beyond loading the make file.

When an actually executed target fails, `oct make` writes a durable Octagon diagnostic at `<StateDir>/failures/<target-name>/<run-id>/failure.octagon` (default `.octmake/failures/<target-name>/<run-id>/failure.octagon`). The path uses the active `Make.Config.StateDir`; target names are sanitized for path safety by replacing separators and unsafe characters with `_`, while the original target name remains inside the artifact. `failure.octagon` records the run id, UTC time, make file, state dir, trace path, target name/kind, failure reason/message, staleness decision reason, duration, and target-kind evidence such as command program/args/env/cwd, inputs/outputs/deps, exit code, stdout/stderr, `CommandHash`, function name/error, or flow state/result evidence. If tracing is enabled, `trace.octagon` also records `FailureArtifactPath` on the failed decision. Dry runs, `oct make explain`, `oct make doctor`, and plan-only `--plan-out` invocations do not execute targets and do not write failure artifact directories.

Example failed run:

```sh
oct make Build
# make failed: target Build failed
# failure artifact: .octmake/failures/Build/20260621T153012Z/failure.octagon
```

Examples:

```sh
oct make --file Examples/ChimeraHello/Make.oct --plan-out .octmake/plan.octagon
oct make explain --file Examples/ChimeraHello/Make.oct TestChimera
oct make doctor --file internal/prometheus/Make.oct
```

Future reporting work includes stdout/stderr sidecar files, replay, failure artifact pruning, plan diffing, hash-based staleness, and richer tool/environment snapshots.


## `oct artifact` execution modes

Usage:

```sh
oct artifact <file-or-root> [--execution <interpreted|compiled>]
oct artifact path/to/file.oct --execution compiled
```

`oct artifact` runs discovered `[Artifact]` entrypoints. The default remains interpreted execution. Passing `--execution interpreted` makes that default explicit; passing `--execution compiled` requires the compiled path and fails instead of silently falling back to interpreted execution if compilation is unsupported. Artifact command output includes stable execution metadata, for example `Execution: compiled`, so a later reader can tell which path generated the artifact output.

Artifact-producing labs should still validate scientific correctness separately from successful execution. Prefer comparing headline outputs against analytic solutions, Python/reference implementations, published values, or fixed golden outputs, and use `Assert.Close` or the closest available assertion before trusting artifact headline numbers.

## `oct new` package scaffolding

Usage:

```sh
oct new <experiment|library|wrapper-library|application|app> <Name> [path]
```

`oct new` creates deterministic package scaffolds. By default, `oct new experiment Name` creates `Experiments/Name` when `Experiments/` exists in the current project/root, while `oct new library Name` and `oct new wrapper-library Name` create `Libraries/Name` when `Libraries/` exists. `oct new application Name` and shorthand `oct new app Name` create `Applications/Name` when `Applications/` exists. If those collection directories do not exist, the fallback remains `./Name`. Passing an explicit `[path]` preserves that path. The command rejects missing arguments, unknown scaffold kinds, invalid names, and any target directory that already exists.

Names must match strict PascalCase `[A-Z][A-Za-z0-9]*`; non-PascalCase inputs such as `oct-opencv`, `signal_tools`, and `openCV` are rejected instead of normalized. Reserved names such as `Manifest`, `Main`, built-in scalar/type family names, and top-level command family names are also rejected.

Generated manifests include ordered `Authors: String[]` metadata and `Date: String` in ISO `YYYY-MM-DD` form. Public scaffolds default to `Authors: ["Unknown"]` because user-created packages are not assumed to be Codex-authored. The first author is the first array element; multiple authors are represented as a 1D string array.

Application scaffolds create `manifest.oct`, `README.md`, `Main.oct`, and `Main.octest`. The manifest uses canonical `Kind: "application"`; `app` is only a CLI shorthand. Application packages are for runnable Oct programs/services/UIs/CLIs, distinct from experiments whose primary output is evidence/artifacts, reusable libraries imported by other packages, and wrapper libraries exposing external sidecars. APP1 does not add application packaging/build output conventions, deployment profiles, containers, optional `Make.oct` application templates, or UIBridge/Machina runtime integration.

Wrapper-library scaffolds include manifest wrapper metadata and a package-local sidecar reference under `sidecars/octxiliary-<kebab>/`. `oct pkg wrappers` can inspect this metadata and render registry output, but `oct new wrapper-library` does not build or run native sidecars. The generated raw wrapper function is metadata only until future wrapper dispatch/build lifecycle milestones.


## `oct init` existing package manifests

Usage:

```sh
oct init <experiment|library|wrapper-library|application|app>
```

`oct new` creates a new package directory. `oct init` initializes the current existing directory by creating `manifest.oct` only. The package name is derived from the current directory basename using the same strict PascalCase validation as `oct new`; invalid directory names should be renamed until future explicit-name support exists.

`oct init` writes the same `Authors: ["Unknown"]` and ISO `Date` metadata as `oct new`, but it always initializes the current directory and never moves into `Experiments/` or `Libraries/`. `oct init` refuses to overwrite an existing `manifest.oct`. Use `oct init experiment` for existing experiment folders, `oct init library` for reusable libraries, `oct init application` (or shorthand `oct init app`) for runnable programs/services/UIs/CLIs, and `oct init wrapper-library` for wrapper-library manifests.

## `oct pkg` wrapper tooling

Inspection remains inert:

```sh
oct pkg wrappers
oct pkg wrappers --registry-out wrappers.octagon
```

`oct pkg wrappers` reports wrapper metadata and can write a data-only registry artifact. It does not build Go modules, create `.oct/wrappers`, run sidecars, or change runtime discovery.

Native sidecars are built only through the explicit command:

```sh
oct pkg build-wrappers --allow-native
```

The command builds current-package Go-module sidecars declared by `GoModuleDir` and writes binaries to `.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe]`. It does not run sidecars, does not build dependency/package-cache/registry sidecars, does not fetch packages, and does not add lockfiles or arbitrary build scripts.

Current runtime discovery does not automatically search `.oct/wrappers/<platform>`. After a successful build, use the printed guidance, for example:

```sh
OCT_WRAPPER_PATH=.oct/wrappers/linux-amd64 oct test .
```

or place the sidecar in an existing sibling-discovery location.

### Optional package lockfiles

`oct pkg lock` writes an optional project-root `lock.octagon` from the current exact-version registry dependency graph. The file is deterministic and timestamp-free. Git entries record the original `Ref` plus a full `ResolvedCommit`; local entries are allowed but marked mutable and are not reproducible because PM6 records no content digest. Wrapper packages are locked as source only; this command does not build sidecars or create `.oct/wrappers`.

`oct pkg sync --locked` requires `lock.octagon`, validates it against the current manifest, and syncs exactly the locked graph. Git packages are checked out at `ResolvedCommit`, not a mutable ref. Local packages are copied from the locked source/path.

Plain `oct pkg sync` remains rolling: it does not read or write `lock.octagon`, and it never creates `oct.lock` or `lock.oct`. Lockfiles do not add `.octpkg` artifacts, package tree/source/registry digests, signing, federation/P2P, publishing, auth, mirrors, binary sidecar distribution, semver ranges, `latest`, or solver/backtracking behavior.

## Version surface

```sh
oct version
oct --version
```

Development builds print `oct dev`. Release builds may inject the tag with Go linker flags, for example:

```sh
go build -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=0.1.0" ./cmd/oct
```

## Package manager / canonical registry quick reference

The v0.1 canonical first-party registry is local/source-controlled at `Registry/registry.oct`; it is not a hosted registry service. Configure a project to use a local Oct checkout with:

```sh
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg registry list
oct pkg registry remove oct
```

Add and sync an exact dependency from that registry:

```sh
oct pkg add Mathematics@0.1.0
oct pkg sync
```

`Mathematics` is the canonical math package name. There is no `Math` alias.

Optional lockfile workflow:

```sh
oct pkg lock
oct pkg sync --locked
```

`lock.octagon` is optional and project-root local. It records the resolved exact dependency graph; it does not yet provide package tree digests, artifact integrity, package signing, hosted publishing/auth, semver ranges, `latest`, or solver behavior.

Wrapper packages are synced as source packages. Syncing a package that declares wrappers does not build or run native sidecars. Use inert inspection first:

```sh
oct pkg wrappers
oct pkg wrappers --registry-out wrappers.octagon
```

Native wrapper sidecars require an explicit build permission:

```sh
oct pkg build-wrappers --allow-native
```

After building, use the printed `OCT_WRAPPER_PATH=.oct/wrappers/<platform>` guidance or place sidecars in an existing sibling-discovery location.
