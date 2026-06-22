# oct make status after ATTR-MAKE4

This is the current internal status note after ATTR-MAKE4-H1. It supersedes
`docs/internal/oct_make_status_make5c.md` as the most recent status snapshot,
but does not delete or rewrite the older parking note.

## Status

`oct make` is ordinary Oct code returning typed `Make.Plan` data, executed by a
controlled Make runtime with explicit host capability boundaries and structured
evidence artifacts.

It is still an MVP/direct-backend feature: there is no Ninja backend, no plan
diff, no replay, no native helper family, and no hermetic build sandbox. It is
no longer a toy. It has a typed target model, state and trace files, command
identity hashing, read-only reporting, failure artifacts, C ABI metadata records,
Chimera dogfood examples, Make-file attributes, authority enforcement, and
advisory purity diagnostics.

## Current mental model

`Make.oct` is not a separate build-system language. It is normal Oct source in a
build entrypoint file. The file constructs typed records from `Libraries/Make`,
usually through `Plan() -> Make.Plan`, and the Go `oct make` implementation owns
loading, validation, target closure selection, staleness decisions, execution,
state, trace, and failure evidence.

Plan construction is data construction. Creating a `Make.CommandTarget`,
`Make.FunctionTarget`, `Make.FlowTarget`, `Make.PhonyTarget`, `Make.CAbiLibrary`,
or `Make.CAbiConsumer` does not execute host work. Host work happens only when
the make executor runs selected targets, or when a `[RequiresAuthority]` Oct
helper directly calls a Make host primitive such as `Make.Tool`, `Make.Env`, or
`Make.Remove`.

## Implemented CLI surface

Current useful surface:

```sh
oct make [target]
oct make --file <Make.oct> [target]
oct make --list
oct make --dry-run
oct make --trace
oct make --plan-out <file.octagon>
oct make explain [target]
oct make doctor
```

`--file <Make.oct>` selects an explicit make file. Without it, `oct make` uses
repository/project discovery for `Make.oct`.

`--list` reports available targets. `--dry-run` selects the target closure and
reports what would run without executing target actions. `--trace` forces trace
writing for execution or dry-run evidence. `--plan-out <file.octagon>` writes a
validated plan snapshot only; it does not add execution decisions, execute
commands, mutate state, or write failure artifacts. `explain` is read-only
would-run/would-skip reasoning over the selected closure. `doctor` is read-only
health and attribute diagnostics; invalid Make files may still fail during load
or typecheck before doctor can print advisory output.

The only executor backend is `direct`. Unsupported backends, including `ninja`,
are rejected rather than silently lowered.

## Make.Plan and target model

`Make.Plan` contains a default target name, `Make.Config`, and four target lists:
`CommandTargets`, `FunctionTargets`, `FlowTargets`, and `PhonyTargets`.

- `CommandTarget` represents a structured process action. Key fields are
  `Name`, `Inputs`, `Outputs`, `Deps`, `Program`, `Args`, `Cwd`, and explicit
  `Env`. Construction does not run a command. The direct executor runs
  `Program` with `Args`, optional `Cwd`, and explicit `Env` entries when the
  selected target is stale.
- `FunctionTarget` represents a call to a named Oct function. Key fields are
  `Name`, `Inputs`, `Outputs`, `Deps`, and `Function`. Construction does not
  call the function. The direct executor calls the named zero-argument function
  as the target action.
- `FlowTarget` represents a call to a named zero-argument Octomata flow. Key
  fields are `Name`, `Inputs`, `Outputs`, `Deps`, `Flow`, and `MaxSteps`.
  Construction does not start the flow. The direct executor runs the flow with a
  positive step bound; the current convention treats completed `Int` result `0`
  as success and non-zero as failure. Persistent suspended-flow resume is not
  implemented.
- `PhonyTarget` groups dependencies without producing an output artifact. Key
  fields are `Name` and `Deps`. It does not execute a command, function, or flow;
  it selects and orders dependency closure.

The executor validates the target graph, target names, dependencies, duplicate
names, selected/default target, and dependency closure before execution. The
current backend is direct only.

## Make.Config / state / trace / plan snapshots

`Make.Config` fields are:

- `Profile`: descriptive profile name.
- `StateDir`: state directory; empty falls back to `.octmake`.
- `Trace`: whether trace evidence should be written without requiring `--trace`.
- `Staleness`: `Make.Staleness.Timestamp` or `Make.Staleness.Always`.

Durable files:

```text
<StateDir>/state.octagon
<StateDir>/trace.octagon
--plan-out <file.octagon>
<StateDir>/failures/<sanitized-target>/<run-id>/failure.octagon
```

Distinctions:

- Plan snapshot: what the build declares after loading and validation. It is a
  static `MakePlanSnapshot` view and includes command hashes for command targets;
  it is not a run log.
- State: last observed successful target state, including output path status and
  command hashes for successful command targets.
- Trace: what happened or would happen in a specific execution or dry-run,
  including decisions, reasons, command hashes, previous command hashes, and
  failure artifact paths when tracing a failed execution.
- Failure artifact: durable extracted failure evidence for an actual failed
  target execution.

`Make.Staleness.Timestamp` uses missing input/output checks, command identity
hashing for command targets, and input/output modified-time comparisons.
`Make.Staleness.Always` reruns selected command/function/flow targets.

## Staleness and CommandHash

MAKE12 added command identity hashing:

```text
CommandHash
PreviousCommandHash
CommandHashMissing
CommandChanged
```

`CommandHash` is a deterministic SHA-256 over stable length-prefixed metadata:
target kind, target name, `Program`, `Args` in order, explicit `Env` entries in
order, `Cwd`, `Outputs` in order, `Inputs` in order, and `Deps` in order.

The hash deliberately excludes ambient environment, timestamps, command output,
durations, current time, temporary state paths, tool versions, and file content
hashes. A `Make.Env("NAME")` read is not automatically part of a
`CommandTarget` identity. If runtime environment should affect command identity,
put explicit `NAME=value` entries into `CommandTarget.Env`.

Older `state.octagon` files without command hashes still load. If outputs are
otherwise present and timestamp-fresh but previous state lacks a command hash,
the command is stale with `CommandHashMissing`. If the previous hash differs
from the current metadata, the command is stale with `CommandChanged`.

## Failure artifacts

MAKE14 added failure artifacts at:

```text
<StateDir>/failures/<sanitized-target>/<run-id>/failure.octagon
```

They are written for actual failed executions. They are not written by dry-run,
`oct make explain`, `oct make doctor`, or plan-only `--plan-out` runs.

A failure artifact includes run metadata, make file, state dir, trace path,
target name and kind, decision reason, failure reason/message, duration, command
hashes, and kind-specific evidence. Command failures include program, args,
environment, cwd, inputs, outputs, deps, exit code, stdout, stderr, and errors.
Function failures include function and error evidence. Flow failures include
flow name, max steps, state history, suspension status, result code, and errors
when available.

The CLI reports the failure artifact path. When trace is enabled, trace decisions
include `FailureArtifactPath`. Future `stdout.txt`/`stderr.txt` sidecars,
pruning, replay, and richer run capture are deferred.

## Make host capabilities and authority boundary

`Libraries/Make` exposes pure records/enums plus side-effectful host primitives.
Pure schema includes `ProcessResult`, `EnvValue`, `Staleness`, `Config`, `Plan`,
target records, and C ABI metadata records/enums. Host primitives are:

```text
Make.Exec
Make.ExecIn
Make.Tool
Make.Env
Make.Exists
Make.IsFile
Make.IsDir
Make.MkdirAll
Make.Remove
Make.Copy
Make.ReadText
Make.WriteText
Make.Glob
Make.ModifiedTime
Make.HashFile
```

Host primitives require Make host authority. Ordinary `oct test Libraries/Make`
does not get authority. The privileged fixture lives at
`Libraries/MakeHostPrivileged/Make.Primitives.octest`; it requires sidecar
build/discovery through `OCT_WRAPPER_PATH` and explicit
`OCT_MAKE_AUTHORITY=1`. `oct make` supplies authority for Make execution as part
of the designed make runtime boundary.

## Pure vs privileged Make tests

`Libraries/Make` is the pure library/schema lane. It tests records, enums,
`Make.CAbi*` metadata, and other authority-free behavior.

```sh
go run ./cmd/oct test Libraries/Make --execution interpreted
go run ./cmd/oct test Libraries/Make --execution compiled
```

`Libraries/MakeHostPrivileged/Make.Primitives.octest` is the explicit
side-effectful lane for process, environment, and filesystem primitives:

```sh
go run ./tools/build_sidecars --out dist/sidecars

OCT_MAKE_ENV_TEST_VALUE=hello OCT_MAKE_EMPTY_ENV_TEST_VALUE= \
OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted

OCT_MAKE_ENV_TEST_VALUE=hello OCT_MAKE_EMPTY_ENV_TEST_VALUE= \
OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution compiled
```

The split is intentional: Make host capabilities are not ambient test/runtime
capabilities.

## C ABI artifact records

MAKE11 records/enums:

```text
CAbiLibraryKind
CAbiCallingConvention
CAbiHeader
CAbiLibrary
CAbiConsumer
```

These are metadata only. They do not compile, link, copy runtime libraries,
change command execution, or provide Rust/Go/C/C++ helpers. They are intended as
the stable data shape for future typed C ABI helper expansion and plan/test
assertions.

The safety posture is M0 integer-only in examples. Deferred or unsafe areas
include strings, pointers, callbacks, heap ownership, structs-by-value,
unwind/panic crossing the ABI, generated headers, `cbindgen`, and `bindgen`.

## Chimera interop examples

`Examples/ChimeraHello` is an experimental C ABI dogfood example: a Go final
executable consumes a Rust `staticlib` through C ABI/cgo. The Make file also
constructs `Make.CAbiLibrary` metadata for the Rust static library. Real runs are
opt-in behind `OCT_CHIMERA_HELLO=1`; expected output is the Go executable's
formatted Chimera hello result from the Rust integer ABI function.

`Examples/ChimeraOctxHello` is an experimental Octxiliary-style dogfood example:
a Go final executable talks to a Rust sidecar through existing Octxiliary typed
DTO frames. It does not make raw JSON canonical. Real runs are opt-in behind
`OCT_CHIMERA_OCTX_HELLO=1`; expected output is the Go executable's formatted
result from the Rust sidecar exchange.

Both examples are dogfood for `oct make`. Their real native build/run targets
are not default CI lanes and should not be run casually by documentation-only
work.

## Rust Chimera SDK M0

`internal/chimera/rust-sdk` contains the std-only `chimera_rust_sdk` crate,
version `0.1.0`. It provides:

- `CHIMERA_ABI_VERSION`.
- `CHIMERA_PANIC_I32`, an `i32::MIN` panic sentinel.
- `ChimeraI32`, an alias for M0 integer values.
- `return_i32`, a panic-boundary helper that catches Rust panics and maps them
  to the sentinel rather than unwinding across `extern "C"`.

It is used by `Examples/ChimeraHello`. It is not Octxiliary, UIBridge,
`cbindgen`, `bindgen`, full FFI, a header generator, or a Make helper API.

## Make.oct attributes

ATTR-MAKE1/2/3/4-H1 support a closed Make-file attribute set:

```oct
[MakePlan]
[Pure]
[NoWhile]
[RequiresAuthority]
```

The Make attributes are `Make.oct`-only, closed names, functions-only, and have
no payloads. There are no user-defined attributes and no generic metaprogramming
system. Ordinary `.oct` still rejects attributes. `.octest` has its own separate
closed attributes such as `[Fact]` and `[Theory]`.

Semantics:

- `[MakePlan]` validates the make entrypoint shape as `Plan() -> Make.Plan`.
  Conventional unmarked `Plan()` is still accepted.
- `[NoWhile]` syntactically rejects `while` inside the marked function.
- `[RequiresAuthority]` is required for direct Make host primitive calls in
  `Make.oct`.
- `[Pure]` marks Make plan/data-construction helpers and feeds diagnostics. It
  is not language-wide purity, not transitive purity, and not a hard strict
  purity mode for all effects.

## Authority enforcement

ATTR-MAKE3 made direct Make host primitive calls in `Make.oct` require
`[RequiresAuthority]`. This is a hard load/typecheck rule for direct calls such
as `Make.Tool`, `Make.Env`, `Make.ReadText`, or `Make.Remove`.

The rule is direct-call-only. Target record constructors do not count as host
primitive calls. There is no transitive authority analysis yet, so a function
that calls a helper that calls `Make.Tool` is not analyzed as requiring authority
unless it directly calls the primitive. Invalid Make files fail typecheck/doctor
load rather than producing only a doctor warning.

## Purity judgment diagnostics

ATTR-MAKE4-H1 added advisory Make purity diagnostics around `[Pure]`. Internal
Go evidence categories are:

```text
PureData
HostAuthority
ObservableEffect
UnknownCall
DeterministicFailure
ControlFlow
```

Severity/utility labels are:

```text
Allow
Info
Warning
Error
```

`internal/judgment` is used for deterministic diagnostic ranking/explanation,
not to define hard language semantics. Hard semantics remain rule-based: for
example, direct Make host primitive calls still require `[RequiresAuthority]`,
and direct host calls from `[Pure]` functions get a clearer hard error.

Doctor can warn for unmarked or unknown helper calls and observable effects such
as `Print`. It allows normal Make data construction: records, arrays, enums,
target constructors, command strings as data, `error(...)`, fallible validation,
`match`, `switch`, and ordinary control flow diagnostics. Public
`Make.PurityJudgment` enums are deferred.

## Current real Make.oct files

- `Examples/ChimeraHello/Make.oct`: annotated with `[MakePlan]`, `[Pure]`,
  `[NoWhile]`, and `[RequiresAuthority]` where appropriate. It has real command
  targets for Rust build, Go/cgo build, and run; real execution is gated by
  `OCT_CHIMERA_HELLO=1`. It includes C ABI metadata for the Rust staticlib.
- `Examples/ChimeraOctxHello/Make.oct`: annotated with Make attributes and
  authority markers. It has real command targets for Rust sidecar, Go binary,
  run, and cleaning; real execution is gated by `OCT_CHIMERA_OCTX_HELLO=1`.
- `internal/prometheus/Make.oct`: annotated plan/config/target constructors for
  Prometheus dogfood. It includes real command targets that invoke existing
  platform/native scripts, but real native execution remains opt-in and not a
  default lane.
- `Libraries/MakeOctestPlan/Make.oct`: annotated pure plan/config companion
  fixture. It has plan/test targets for octest planning but no native dogfood
  command execution requirement.

Doctor status should be kept green for these files as part of future Make work;
when recording a specific doctor result, include the exact command and date.

## Known limitations / non-goals

- Direct backend only.
- No Ninja backend.
- No plan diff.
- No replay.
- No failure artifact pruning.
- No `stdout.txt`/`stderr.txt` sidecar artifacts yet.
- No input content hashing.
- No tool path/version hashing.
- No transitive purity or authority analysis.
- No hard strict purity mode.
- No public `Make.PurityJudgment` API.
- No C ABI helper APIs yet.
- No Go/Rust/C/C++ language-specific Make helpers yet.
- No `cbindgen`/`bindgen` integration.
- No sandbox/hermetic mode.
- No parallel execution.
- No package manifest `Build` section requirement.
- No automatic `Make.octest` execution before `oct make`.
- No persistent/resumable `FlowTarget` checkpointing.

## Recommended next work

Suggested staged work:

- MAKE17-DESIGN: decide Chimera helper library shape. Compare
  `Libraries/Chimera` versus language-specific helpers under `Libraries/Make`.
  Keep helpers pure data/functions for Rust static C ABI artifacts; do not add
  executor magic.
- MAKE17-H1: implement pure Chimera Rust staticlib helper records/functions and
  refactor `Examples/ChimeraHello` to use them.
- MAKE18: design and implement Go cgo consumer helper shape.
- MAKE19: add plan diff over plan snapshots and command identity changes.
- ATTR-MAKE5-DESIGN: design transitive authority/purity analysis without
  implementing it prematurely.
- MAKE20: add failure artifact `stdout.txt`/`stderr.txt` sidecars and pruning.

Keep the next changes narrow. Do not add a Ninja backend, C ABI helpers,
transitive purity, or plan diff as drive-by work while touching status docs.
