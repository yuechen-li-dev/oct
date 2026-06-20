# MAKE0-DESIGN-RECON — `oct make` architecture and repo-shape audit

Status: design reconnaissance only. No `oct make` implementation, process/file builtins, Ninja backend, toolchain helpers, package-manager changes, manifest behavior changes, or Octomata semantic changes are included in this milestone.

## Executive recommendation

`oct make` should be a first-class CLI command implemented in Go, but its build logic should be authored in a normal Oct source file named `Make.oct`. The project manifest should remain package/project metadata and should only gain a small optional build entrypoint section later. The recommended architecture is the hybrid path:

1. `manifest.oct` optionally points to a build system, build file, and default target.
2. `Make.oct` defines `Plan() -> Make.Plan` using ordinary records and functions.
3. M0 execution is a direct executor over a typed target/action plan.
4. Ninja is a later backend for command targets only.
5. Host authority lives in a compiler/host-owned `Make` capability package available only under `oct make`, not ambiently to `oct run`.
6. Octomata is supported as an action style for complex workflows, not required for ordinary file DAGs.

This keeps Oct away from CMake's central failure mode: mixing configuration, dependency graph construction, shell command composition, tool discovery, cache mutation, backend emission, and platform policy into one untyped string soup.

## Current repo survey findings

### CLI and command architecture

The CLI is centralized in `internal/cli.Execute`. It handles top-level help/version directly, then dispatches subcommands through a `switch` over the first argument. Existing commands include `new`, `init`, `pkg`, `exp`, `run`, `build`, `fmt`, `test`, `artifact`, `bench`, and a few specialized Prometheus commands. `cmd/oct/main.go` is intentionally thin and the real behavior is under `internal/cli` and subpackages.

Current command shapes are simple and explicit:

- `oct run <file-or-root>` loads a project and calls `interpret.ExecuteMain`.
- `oct build <file-or-root>` calls `build.Compile` and prints the `.octbin` path.
- `oct test <file-or-root> [--suite ...] [--execution ...] [--all-packages]` parses options manually.
- `oct artifact <file-or-root> [--execution ...]` uses the same test runtime lane partitioning style.
- `oct pkg ...` has nested dispatch and manually parsed subcommand options.
- `oct new` and `oct init` are scaffold commands with strict argument counts and explicit help.

The repo does not currently use a command framework. Flags are parsed with direct loops over `args`, validation is local to each command, and errors are normalized through `reportCommandError` as `<command> failed: <message>`.

A new `oct make` should fit beside `run`, `build`, `test`, `artifact`, and `bench`, not under `pkg`, because it is a project execution/build lane rather than package dependency management. It should be implemented by adding a new `case "make"` in `internal/cli.Execute`, with the substantive logic in a new internal package such as `internal/makecmd` or `internal/makeexec` once implementation begins.

`oct make [target]` is a natural shape for this CLI. Most existing commands take a path first, but `make` is project-root oriented. Because target selection is its core positional argument, the command should reserve at most one positional target and parse flags anywhere only if the manual parser explicitly supports it. To stay close to current style, prefer:

```text
oct make [target] [--file <path>] [--manifest <path>] [--backend direct] [--dry-run] [--trace] [--list]
```

For M0, however, the minimal implementation should be smaller: `oct make [target]`, `--file`, `--list`, `--dry-run`, `--trace`, and `--backend direct`. `--manifest` should be deferred unless manifest lookup actually creates ambiguity in MAKE2.

CLI tests should follow existing command tests in `cmd/oct/*_command_test.go`: create temporary projects, invoke the CLI entrypoint or `go run ./cmd/oct` only when full process behavior is required, assert stdout/stderr/error text, and keep semantic Oct behavior in `.octest` or `.octfail` fixtures under `Language/` rather than embedded Oct strings in Go tests.

### Manifest and package metadata shape

`manifest.oct` is parsed as Oct source, but with a constrained schema. It must declare `package Manifest`, a `PackageManifest` record, a `Dependency` record, and a `fn Manifest() -> PackageManifest` whose body is a single return of a literal `PackageManifest`. Current manifest metadata includes `Name`, `Version`, `Description`, optional ordered `Authors`, ISO `Date`, `Kind`, `EntryMilestone`, `Dependencies`, and optional wrapper metadata.

Package kinds are intentionally narrow: normalized manifest `Kind` values are `pure`, `experiment`, and `wrapper`. `EntryMilestone` is only valid for `experiment`. `oct new` uses user-facing scaffold kinds `library`, `experiment`, and `wrapper-library`, but writes manifest kinds that normalize into those manifest-level identities.

`oct make` should not add `Kind: "make"`, `Kind: "production"`, or any new package identity for build behavior. Production is a target/profile concern, not a package kind.

Adding a small optional `Build` or `Make` section to `PackageManifest` would be idiomatic if kept as project metadata and validated with the same literal-shape discipline. The recommended future shape is:

```oct
record BuildConfig {
    System: String
    File: String
    Default: String
}

record PackageManifest {
    Name: String
    Version: String
    Description: String
    Dependencies: Dependency[]
    Build: BuildConfig
}

fn Manifest() -> PackageManifest {
    return PackageManifest {
        Name: "MyProject"
        Kind: "experiment"
        Version: "0.1.0"
        Description: "..."
        Dependencies: []
        Build: BuildConfig {
            System: "oct.make"
            File: "Make.oct"
            Default: "Build"
        }
    }
}
```

For compatibility with current manifest strictness, MAKE2 should not require this shape. It should discover `Make.oct` by default when manifest build config is absent.

Recommended lookup order:

1. If `--file <path>` is provided, use that file and do not infer a different build file.
2. Else discover project root using existing manifest/project-root conventions.
3. If `manifest.oct` has future `Build.System == "oct.make"` and `Build.File`, use that file relative to project root.
4. Else use `<project-root>/Make.oct` if present.
5. Else fail with a diagnostic explaining the attempted locations and suggesting `oct make --file Make.oct`.
6. Target default is future manifest `Build.Default`, else plan default, else `Build` if present, else the first eligible target only if the plan explicitly marks it default.

Package manager and registry should treat build configuration as package metadata only. `oct pkg sync`, registry resolution, and lockfile semantics should not build sidecars, run make files, or mutate build state. This follows the existing wrapper split: `oct pkg wrappers` inspects wrapper metadata, while `oct pkg build-wrappers --allow-native` explicitly builds native sidecars.

### Oct execution and evaluation shape

The current execution path is suitable but needs one small plumbing layer for `oct make`.

`project.Load` loads source, imports, and manifest context. `run.Execute` typechecks the program and calls `interpret.ExecuteMain`. The interpreter already has exported helpers to invoke a named function: `ExecuteFunction`, `ExecuteFunctionWithArgs`, and `ExecuteFunctionWithArgsAndOptions`. Those helpers construct an interpreter, look up `pkgName.functionName`, validate argument count, execute the function, and convert fallible return errors into fatal errors.

Therefore:

- `oct make` can load a `Make.oct` file and call `Plan()` if the file is loaded as an entry package and `Plan` has an ordinary function signature.
- It can call named action functions such as `Build()` using `interpret.ExecuteFunctionWithArgsAndOptions` for interpreted execution.
- It cannot currently receive a returned `Make.Plan` value from that helper because the exported function-call helper returns only `error`. MAKE2 likely needs a narrow exported `CallFunction`-style helper returning `interpret.Value` or a typed bridge that converts a returned record into Go plan structs.
- The direct executor can call action functions with no arguments first; arguments/target context records can be added later.

`Make.oct` should use `Plan()`, not `Main()`. `Main()` is already the ordinary program entrypoint for `oct run`; using `Plan()` keeps build graph declaration distinct from program execution and avoids making build planning look like normal scientific program output. Function/flow action targets can still be named `Build`, `Test`, `Clean`, etc.

Fallible build actions should be allowed. A fallible function action returning `Void ! Error` should fail the target when it returns `err`. A fallible `Make.Exec` should return a `ProcessResult` on success and `Error` on launch/capability failure; non-zero process exit should normally be represented in `ProcessResult.ExitCode`, with helper policies later deciding whether non-zero is fatal. For M0, command targets should fail on non-zero unless the plan explicitly allows non-zero in a future field.

Diagnostics should preserve source locations from existing parser/typechecker errors. Plan validation diagnostics produced by the make executor should include the build file path, target name, field name, and enough plan context to compensate for the current runtime value bridge not carrying rich source spans for record literal fields.

### Builtins and host capability model

There are already many Go-backed builtins exposed to Oct:

- Octagon: `WriteOctagon(path, value)` and `LoadOctagon<T>(path)`.
- IO namespace aliases: `IO.ReadText`, `IO.WriteText`, `IO.ReadLines`, `IO.WriteLines` map to file builtins.
- Directory builtins include list, make, make-all, and remove-all.
- Path helpers include join/base/ext/stem/parent/clean.
- Hash helpers include SHA-256 bytes/text/file.
- Artifact namespace helpers wrap file/Octagon writes for artifact functions.

These are implemented as interpreter-side handlers and compiled-code lowering/runtime helpers. Fallible builtins are represented as `evalResult{hasError:true, errorVal: ...}` in the interpreter, and compiled output uses generated result structs for fallible values. Wrapper-style helpers commonly return `Int` status for mutation success today, which is usable but not ideal for future make APIs.

There is no general Oct process execution builtin. Existing process execution in the repository is Go-owned and toolchain-specific:

- `oct build` invokes `go build` internally.
- compiled test/artifact/benchmark lanes execute generated binaries.
- wrapper sidecar build invokes `go build -o <output> .` with explicit native permission.
- interpreted generic wrapper dispatch starts declared sidecar commands.
- package manager invokes `git` for clone/checkout operations.

Future `Make` capabilities should not be ordinary always-available builtins. They should be a host-capability package made available only under `oct make`, with explicit authority and project-root scoping. That package can still use the same Go-backed builtin machinery, but typechecking should reject it outside make mode or require an explicit host-capability option later.

Candidate M0 capabilities should be split into primitives and deferred helpers:

M0 primitives:

```oct
Make.Exec(program: String, args: String[]) ! ProcessResult
Make.ExecIn(cwd: String, program: String, args: String[]) ! ProcessResult
Make.Tool(name: String) ! String
Make.Exists(path: String) -> Bool
Make.IsFile(path: String) -> Bool
Make.IsDir(path: String) -> Bool
Make.MkdirAll(path: String) ! Error
Make.Remove(path: String) ! Error
Make.Copy(src: String, dst: String) ! Error
Make.ReadText(path: String) ! String
Make.WriteText(path: String, text: String) ! Error
Make.Glob(pattern: String) ! String[]
Make.ModifiedTime(path: String) ! Int
Make.HashFile(path: String) ! String
```

M0 records:

```oct
record ProcessResult {
    ExitCode: Int
    Stdout: String
    Stderr: String
}
```

Prefer new `Make`-specific return types over reusing existing `Int` status conventions. This makes failures explicit and avoids carrying old IO-wrapper compromises into build orchestration.

### Octomata integration possibilities

Octomata is mature enough to shape the design but should not be made mandatory in the first executor.

Current Octomata facts:

- Flows are declarations with states, `goto`, `suspend`, `return`, optional board memory, and state history.
- A flow call returns a flow instance; it does not directly run to completion.
- `Step(flow)` advances one scheduling step.
- `Result(flow)` is fallible because a flow may not have completed.
- `StateHistory(flow)` returns transition history.
- `BoardSnapshot(flow)` can expose read-only typed board state for scalar board fields.
- Flows can be called from ordinary Oct expressions; the interpreter instantiates flow values and supports stepping/result builtins.
- Compiled support exists in the build MIR and compiler, but command design should not require compiled flow support for M0 make.

Make actions can call Octomata flows today from ordinary Oct functions by creating a flow instance and stepping it. Flows are not directly callable as complete build actions unless the make executor defines a convention such as "step until Complete or max steps". That convention should be deferred until MAKE4 because it needs failure policy, trace shape, and infinite-loop protection.

Clean boundary:

- Target DAGs model file dependencies, outputs, and stale/ready scheduling.
- Octomata models complex workflows inside an action: configure/probe/retry/fallback/stage/package/test.
- A future `FlowTarget` should be direct-executor-only initially and should expose state history into make traces.
- A Ninja backend should not attempt to lower arbitrary flow bodies.

### Build model options

| Option | Cost | Fit with repo | Ninja future | C/C++ + Go future | Octomata | CMake-soup risk | Suitability |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A: `Make.oct` ordinary script, `oct make target` calls function | Low | Good because named function invocation exists | Weak; no declared inputs/outputs | Weak; helpers become ad hoc | Good inside functions | Medium-high because dependencies/cache are implicit | Acceptable only as prototype, not recommended for v0.1 design |
| B: `Make.oct` `Plan() -> Make.Plan` | Medium | Good; records/functions fit current language | Strong for command targets | Strong; typed helpers can emit targets | Good as action type | Low if plan stays typed and capability APIs are separated | Recommended foundation |
| C: manifest-only build declarations | Medium | Poor; manifest parser is intentionally literal metadata | Medium for command-only targets | Poor for generated/probed builds | Poor | Medium because command strings creep into metadata | Not recommended |
| D: special `target Build { ... }` syntax | High | Poor for M0; requires parser/typechecker/language reference changes | Potentially strong later | Potentially strong later | Unknown | Low if designed well, but high language churn | Defer beyond MAKE6 |
| E: hybrid manifest points to `Make.oct`; `Make.oct` defines `Plan()`; direct first; Ninja later | Medium | Best fit | Strong for command targets; explicit rejection/callback for others | Strong | Strong but optional | Lowest | Recommended v0.1/v0.2 architecture |

Recommended model is Option E with Option B as the semantic core.

### Target/action model

Ninja can lower command targets easily. It cannot lower arbitrary Oct function bodies or Octomata flows without calling back into Oct, so target kinds should be explicit from the start.

Recommended target kinds:

```text
CommandTarget:
  name + program + args + inputs + outputs + deps
  lowerable to Ninja later

FunctionTarget:
  name + function + inputs + outputs + deps
  direct backend only initially

FlowTarget:
  name + flow + inputs + outputs + deps
  direct backend only initially; defer execution convention

PhonyTarget:
  name + deps
  dependency grouping / always-run target
```

This split fits the current type system better as separate arrays than as one enum-with-payload array:

```oct
record Plan {
    Default: String
    CommandTargets: CommandTarget[]
    FunctionTargets: FunctionTarget[]
    FlowTargets: FlowTarget[]
    PhonyTargets: PhonyTarget[]
}
```

Associated enum payloads exist in the language reference and codebase, but separate arrays are simpler to validate, easier to lower incrementally, and avoid requiring generic/union ergonomics that are not needed for M0. Dependencies should refer to target names as `String[]`; the executor validates uniqueness and dangling references after `Plan()` returns.

Direct backend staleness:

1. Phony targets are stale/always considered runnable unless only used for grouping and all deps are up-to-date.
2. Command/function/flow targets with no outputs are always stale.
3. If any output is missing, target is stale.
4. If any input is newer than the oldest output, target is stale in M0.
5. If any dependency target ran, dependent target is stale unless future plan fields specify otherwise.
6. Hash-based staleness should be deferred to MAKE3 state because it requires cache records and reproducibility metadata.

Future Ninja backend policy:

- Lower `CommandTarget` directly.
- Lower `PhonyTarget` directly.
- For `FunctionTarget` and `FlowTarget`, either reject by default or emit explicit callback commands that invoke `oct make <target> --backend direct`. The default should be reject in the first Ninja milestone to avoid surprising recursive make behavior. Callback wrappers can be added when trace/state locking semantics are clear.

### C/C++ + Go mixed build stretch goal

Current repo evidence includes Go module builds, `go build` for the Oct compiler output, explicit wrapper sidecar build lifecycle, native Prometheus C/C++/Vulkan experiments, platform-specific sidecar discovery, and CGO/native host notes. This confirms that mixed native + Go builds are a real future motivator.

Minimum primitive APIs needed first:

- `Make.Exec` / `Make.ExecIn` with stdout/stderr/exit capture.
- `Make.Tool` with deterministic lookup and traceable path result.
- project-root-scoped path normalization.
- file stat/existence/read/write/copy/mkdir/remove/glob/hash/mtime.
- environment selection for commands, probably deferred from M0 but designed into `CommandTarget`.

Typed helpers can come later:

```oct
record CToolchain {
    Cc: String
    Cxx: String
    Ar: String
    Platform: String
}

record CLibrary {
    Name: String
    Sources: String[]
    IncludeDirs: String[]
    Defines: String[]
    Output: String
}

record GoBinary {
    Name: String
    Package: String
    Output: String
    Cgo: Bool
    CgoCFlags: String[]
    CgoLdFlags: String[]
}
```

`oct make` can improve on CGo shell soup by making C compile objects, library artifacts, Go build env (`CGO_CFLAGS`, `CGO_LDFLAGS`), generated headers, shared-library copying, and dependency ordering typed plan entries rather than strings embedded in shell invocations.

The direct executor should own early tool discovery, platform probing, and typed helper expansion. The future Ninja backend should receive already-expanded command targets for simple compile/link/build steps. Complex probing/fallback flows should remain direct.

Zig CC should be supported later as a toolchain option, not as the default semantic model. It is useful for cross-platform C/C++ but should fit behind `CToolchain` discovery and explicit target selection.

### Build cache, trace, and Octagon artifacts

Use `.octmake/` as the future build state directory, not `.oct/`, because `.oct/` is already used for wrapper outputs and project-local tool state. Keep `.octmake/` out of commits like other scratch/generated artifacts.

Recommended layout:

```text
.octmake/
  state.octagon
  trace.octagon
  plan.octagon
  logs/<target>/<run-id>.stdout
  logs/<target>/<run-id>.stderr
```

M0 direct executor should use timestamps because they require no state file and are understandable. MAKE3 should add optional hashes and exact input/output records. Trace should always be available when `--trace` is set and should write an Octagon artifact with target decisions, command lines without shell re-quoting ambiguity, cwd, env policy, exit code, stdout/stderr paths or captured snippets, input mtimes/hashes, outputs, and duration.

Target run records should use Octagon because `.octagon` is already Oct's typed artifact format and current tooling knows how to write/read it. This complements `oct artifact`: `oct artifact` is user-authored evidence generation, while `oct make --trace` is tool-authored build execution evidence. They should share the Octagon format but not the artifact-lane execution semantics.

### Security and authority model

Build scripts are side-effectful and need host authority. That authority should be explicit:

1. `Make` APIs should be available only in `oct make` mode, not all Oct programs.
2. `Make.Exec` should require make-mode host authority.
3. Paths should be normalized with lexical clean + project-root resolution; symlink escape policy should be decided before broad writes are allowed.
4. Default authority should be project-root scoped for file mutation and globbing.
5. Absolute paths should be allowed only for tool paths returned by `Make.Tool`, explicit inputs outside the project, or future `--allow` grants.
6. Environment variables should not be ambiently inherited wholesale forever. M0 can inherit by default for practicality, but trace the environment policy and design toward explicit `Env` fields.
7. Future `--allow` flags are likely needed, for example `--allow exec`, `--allow write=dist`, `--allow read=/opt/sdk`, or `--allow env=CC,CXX,PATH`.

`oct make` is explicitly side-effectful; `oct run` should remain ordinary program execution and should not gain arbitrary process execution accidentally.

## CLI UX proposal

Recommended M0 UX:

```sh
oct make
oct make Build
oct make Test
oct make Clean
oct make --list
oct make --dry-run
oct make --trace
oct make --file Make.oct
oct make --backend direct
```

M0 flags:

- `--file <path>`: choose build file explicitly.
- `--list`: print targets with kind and default marker; do not run.
- `--dry-run`: compute plan and print execution decisions; do not run commands/functions.
- `--trace`: write `.octmake/trace.octagon` and print path.
- `--backend direct`: accept only `direct` initially, reject anything else clearly.

Deferred flags:

- `--manifest <path>` until manifest build config exists.
- `--backend ninja` until MAKE6.
- `--profile`, `--watch`, `--jobs`, `--allow`, `--env`, `--state-dir`, `--plan-out` until state/backend policy is designed.

Default target order:

1. CLI target if provided.
2. Future manifest build default.
3. `Plan.Default` if non-empty.
4. Target named `Build` if present.
5. Error with target list if ambiguous.

Target names should be case-sensitive because Oct identifiers are case-sensitive and generated target names should match declared names exactly.

Errors should follow current CLI style (`make failed: ...`) but should include structured context after the command prefix:

```text
make failed: target "Build": command "cc": exit 1
  cwd: /path/project
  stderr: .octmake/logs/Build/....stderr
```

`--list` output should be stable and simple:

```text
Targets:
* Build    command  default
* Test     function
* Clean    function
* Package  phony
```

## Staged implementation plan

### MAKE0 — design reconnaissance only

- Add this report.
- No implementation.
- No manifest schema changes.
- No CLI behavior changes.

### MAKE1 — make-mode host capability primitives

- Add a host-owned `Make` capability package/typecheck mode available only to `oct make` tests or a temporary internal harness.
- Implement file/process/tool primitives with fallible results.
- Add `.octest`/`.octfail` language contracts for exposed behavior only if the surface is user-visible; otherwise use Go tests for host capability plumbing.
- Do not add target DAG execution yet.

### MAKE2 — `oct make` direct command M0

- Add CLI command and help.
- Discover `Make.oct`.
- Load/typecheck make file in make mode.
- Call `Plan()` and bridge returned plan records into Go structs.
- Validate target graph.
- Run direct `CommandTarget` and no-argument `FunctionTarget`.
- Implement `--list`, `--dry-run`, `--file`, `--backend direct`.
- No Ninja.

### MAKE3 — build state and trace

- Add `.octmake/state.octagon` and `.octmake/trace.octagon`.
- Record timestamp decisions first; add optional file hashes where useful.
- Capture stdout/stderr paths and process exit records.
- Add plan dump support for diagnostics.

### MAKE4 — Octomata build-flow support

- Add direct `FlowTarget` support with explicit step-to-completion policy, max-step guard, failure reporting, and state-history trace.
- Add examples showing configure/probe/retry/fallback flows.
- Keep DAG scheduling separate from flow progression.

### MAKE5 — C/C++ and Go typed helper records/functions

- Add typed helper expansion for C libraries, object files, Go binaries, CGO flags, generated headers, and runtime shared-library placement.
- Support platform suffixes and toolchain discovery.
- Consider Zig CC as an explicit toolchain option.

### MAKE6 — Ninja backend for command targets

- Emit Ninja for `CommandTarget` and `PhonyTarget`.
- Reject function/flow targets by default.
- Later consider callback wrappers only after trace/state locking semantics are robust.

## Exact recommended next prompt for MAKE1

```text
You are working in the Oct repository.

Task: MAKE1-HOST-CAP — make-mode host capability primitives for future `oct make`

Use docs/internal/oct_make_design_recon_make0.md as the design authority for this milestone.

Goal:
Implement the minimum host capability plumbing needed for future `oct make`, without implementing `oct make` target DAG execution.

Scope:
- Add a make-mode-only `Make` capability surface for primitives such as Exec, ExecIn, Tool, Exists, IsFile, IsDir, MkdirAll, Remove, Copy, ReadText, WriteText, Glob, ModifiedTime, HashFile.
- Model `ProcessResult` with ExitCode, Stdout, Stderr.
- Ensure process launch failures are fallible errors and non-zero process exits are inspectable through ProcessResult.
- Keep `Make` unavailable to ordinary `oct run` unless an explicit make/test harness enables make authority.
- Add tests for typecheck/runtime authority boundaries and primitive behavior.

Non-goals:
- Do not implement the `oct make` command.
- Do not implement `Make.oct` discovery.
- Do not implement `Plan()` execution or target DAGs.
- Do not implement Ninja.
- Do not add C/C++ or Go typed build helpers.
- Do not change package-manager behavior.
- Do not change manifest schema.
- Do not add language syntax.

Testing:
- Put user-visible language behavior in `.octest` / `.octfail` under `Language/` where appropriate.
- Use Go tests only for host plumbing/CLI authority boundaries.
- Run targeted tests plus `go test -count=1 -parallel 8 ./...` if Go code changes.
```

## PR summary checklist for this milestone

- No implementation included.
- Recommended manifest-vs-`Make.oct` split: manifest metadata may point to `Make.oct`; `Make.oct` owns orchestration.
- Recommended target/action split: command/function/flow/phony targets, with separate arrays initially.
- Recommended host capability model: make-mode-only `Make` package, not ambient process execution.
- C/C++ + Go implication: start with typed primitives and leave toolchain helpers to a later stage.
- Next milestone: MAKE1 host capability primitives, not `oct make` DAG execution.
