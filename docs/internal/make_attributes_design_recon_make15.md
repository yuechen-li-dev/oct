# MAKE15-DESIGN: Make.oct attributes and typed idiom audit

## Status

Design/recon only. This pass does not implement Make attributes or new Make host primitives.

## Non-goals

- Do not add general-purpose attributes for ordinary Oct code.
- Do not add user-defined attributes, macros, reflection, compiler plugins, or an open decorator ecosystem.
- Do not change Oct syntax, Make execution semantics, Make authority boundaries, or the availability of `Make.Exec`.
- Do not require native toolchains in default tests.

## Existing attribute support audit

### Where attributes are currently allowed

Attributes are currently parsed only for `.octest` files. `parseFile` marks a file as a test file when the source path ends with `.octest`; any top-level `[` in a non-`.octest` file is rejected before parsing an attribute body.

Within `.octest`, attributes are pending top-level markers that must attach to the next function declaration. If a pending attribute is followed by `record`, `enum`, `flow`, EOF, or another invalid top-level shape, parsing fails with a function-declaration requirement.

### Existing syntax

Current attribute syntax is closed and first-party only:

```oct
[Fact]
[Theory]
[Artifact]
[Benchmark]
[InlineData(1, "x", true, Some.EnumValue)]
[Suite("Name")]
[CycleTime(1.0s)]
```

The parser accepts square-bracket attributes with an identifier name. Some attributes are bare markers; `InlineData`, `Suite`, and `CycleTime` have parenthesized payloads with special validation. `InlineData` supports scalar literals and enum-value field access in the current parser lane.

### AST representation

Attributes are not represented as a general AST node list. They are lowered directly into dedicated fields on `ast.FunctionDecl`:

- `IsTestFile`
- `IsFact`
- `IsTheory`
- `IsArtifact`
- `IsBenchmark`
- `InlineData`
- `Suites`
- `CycleTime`

This is intentionally specialized. There is no `Attribute` struct and no declaration-wide metadata bag.

### Typechecker treatment

The typechecker consumes these dedicated function fields. It validates `InlineData` rows against theory parameters and carries test context through `functionContext` so test-only builtins such as `SkipTest` are only available in `[Fact]` or `[Theory]` bodies. Attribute shape constraints such as parameter count and `Void` return type are mostly enforced in the parser before typechecking.

### Octest use

Octest uses the function flags and metadata to discover facts, theories, artifacts, and benchmarks; organize suites; provide inline data rows; and handle cycle-time metadata for theories. Project-generated runner coverage also uses `[Fact]` in generated `.octest` sources.

### Unknown attributes and current restrictions

Unknown attribute names are rejected by `parseTestAttribute` with `unsupported attribute [Name]`. Known attributes are also restricted by file suffix and declaration context. For example, `[Fact]` in a normal `.oct` file is rejected; `[Fact]` before a `record` in `.octest` is rejected; duplicate and incompatible test attributes are rejected.

### Availability to non-octest files

Attributes are not currently available to non-`.octest` files. That includes `Make.oct`. A `Make.oct` file cannot currently spell `[MakePlan]`, `[Pure]`, `[NoWhile]`, or `[RequiresAuthority]` without parser changes.

### Minimal safe recognition path for Make.oct-only attributes

The lowest-risk path is to mirror Octest's closed, first-party model rather than adding generic attributes:

1. Treat a source path whose base name is exactly `Make.oct` as a Make file in the parser.
2. Keep the parser's top-level attribute gate closed: allow attributes only in `.octest` or `Make.oct` files.
3. Add a separate `parseMakeAttribute` with a closed name set and no user-defined extension mechanism.
4. Lower Make attributes into dedicated `ast.FunctionDecl` boolean fields, such as `IsMakePlan`, `IsMakePure`, `IsMakeNoWhile`, and `RequiresMakeAuthority`, instead of adding a generic attribute list.
5. Require Make attributes to attach only to function declarations.
6. Validate only minimal syntactic/type shape in H1: duplicate rejection, `[MakePlan]` zero parameters and `Make.Plan` return, `[NoWhile]` syntactic body walk, and no effect analysis yet.
7. Keep `.octest` attributes and `Make.oct` attributes disjoint. `[Fact]` remains invalid in `Make.oct`; `[MakePlan]` remains invalid in `.octest` unless a future release explicitly changes that.

## Make.oct attribute policy

Make attributes should be compiler/tool-owned semantic markers for the Make execution surface. They should not be a reusable annotation language.

### Proposed minimal attribute set

```oct
[MakePlan]
fn Plan() -> Make.Plan { ... }

[Pure]
fn RustArtifact() -> Make.CAbiLibrary { ... }

[NoWhile]
fn Plan() -> Make.Plan { ... }

[RequiresAuthority]
fn CheckTools() -> Int ! Error { ... }
```

Names are intentionally plain and closed. If implemented, they should be accepted only in files whose base name is `Make.oct`.

### `[MakePlan]`

Marks the Make plan entrypoint. M0 compatibility should keep plain `fn Plan() -> Make.Plan` valid. If `[MakePlan]` is present, the checker should require zero parameters and return type `Make.Plan`. Future `oct make` may prefer or validate this function, but should not break existing conventional `Plan()` files during the migration window.

### `[Pure]`

Marks deterministic data/plan construction. It should be Make.oct-only for the foreseeable future. H1 can store it as metadata and optionally warn if obvious Make host primitives are called. A later checker can enforce that the function body does not call `Make.Exec`, `Make.ExecIn`, `Make.Tool`, filesystem mutation primitives, host reads, or `Print`-style effects.

### `[NoWhile]`

Marks functions whose body must not contain `while`. This is useful for plan construction and static metadata helpers where unbounded loops are a smell. It is syntactic and can be enforced before effect analysis exists.

### `[RequiresAuthority]`

Marks functions that intentionally use Make host authority, such as process execution, tool lookup, filesystem mutation, filesystem reads, hashing, globbing, or environment access. It documents that the function belongs to the execution/action side rather than the pure plan-construction side.

## Design answers

### 1. Should `[MakePlan]` imply `[Pure]` and `[NoWhile]`?

Not in H1. `[MakePlan]` should initially imply only the entrypoint shape: zero arguments and `Make.Plan` return. It should be legal but redundant to write `[MakePlan] [Pure] [NoWhile]`. After Make files are migrated and diagnostics are proven, `oct make doctor` can recommend `[Pure]` and `[NoWhile]` on `[MakePlan]` functions.

Long term, `[MakePlan]` should behave as if it is pure and no-while for validation, but the staged migration should avoid surprising existing Make files.

### 2. Should `Plan()` eventually require `[MakePlan]`?

No, not for compatibility. The conventional `fn Plan() -> Make.Plan` should remain accepted indefinitely. `[MakePlan]` should be an opt-in marker that improves validation, diagnostics, plan-out metadata, and future ambiguity handling if Make ever supports multiple candidate plan functions.

### 3. Should `[Pure]` be Make.oct-only or eventually general?

Make.oct-only. General purity is a language-wide effect system and should not be smuggled in through attributes. If Oct later needs purity, it deserves a separate language design, not a Make-specific marker promoted by accident.

### 4. Should `[RequiresAuthority]` be required for any function calling Make host primitives?

Eventually yes for direct calls in Make.oct function bodies, but not in H1. The staged rule should be:

1. H1: marker is parsed and surfaced; no requirement.
2. H2: `doctor` warns on unmarked functions that directly call known host primitives.
3. H3: checker error for direct host primitive calls outside `[RequiresAuthority]`, except inside the Make action execution surface if such a distinction is represented.

### 5. Should `FunctionTarget.Function` be required to reference a `[RequiresAuthority]` function when it uses host capabilities?

Not solely by name. The correct future rule is body-based: if the referenced function directly or transitively uses host capabilities, it should be marked `[RequiresAuthority]`. `FunctionTarget.Function` can then validate that the target function exists and, if it is authority-using, that the marker is present. Pure function targets should remain legal for generated outputs or synthetic checks that do not need host access.

### 6. How should attributes interact with interpreted and compiled execution?

They should be front-end metadata with identical validation for interpreted and compiled execution. The interpreter and compiler should receive already-validated AST/programs. Attributes must not change runtime dispatch semantics except where `oct make` explicitly uses `[MakePlan]` for entrypoint selection or diagnostics.

### 7. How should attributes appear in `--plan-out`, trace, doctor, or failure artifacts?

- `--plan-out`: include a small first-party metadata section only after a schema/version bump, for example `MakePlanMetadata` with entrypoint name and marker list. Do not serialize arbitrary user attributes.
- `trace.octagon`: record selected plan function and whether Make attribute validation was enabled.
- `doctor`: best first surface. Report plan marker presence, unmarked authority functions, `[NoWhile]` violations, and shell-shaped probe suggestions.
- failure artifacts: include attribute validation diagnostics when failure is caused by attribute enforcement. Do not dump an open annotation map.

### 8. How do we avoid annotation/metaprogramming creep?

- Only parser-known names are accepted.
- Only `Make.oct` gets Make attributes.
- Attributes attach only to functions.
- Attribute payloads should be avoided for Make H1; use bare markers.
- No reflection API exposes attributes to Oct code.
- No user-defined attributes, macros, compile-time execution, plugin hooks, or behavior injection.
- Each new attribute must identify the owning tool, allowed file kind, allowed declaration kind, validation rule, and non-goal.

## Make.oct shell-shaped logic audit

| File | Function | Current pattern | Problem | Preferred typed idiom | Requires new primitive? | Suggested priority |
| --- | --- | --- | --- | --- | --- | --- |
| `Examples/ChimeraHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "test \"$OCT_CHIMERA_HELLO\" = 1"])` | Shell required for a basic environment gate; quoting is platform-specific and hard to inspect. | `Make.Getenv("OCT_CHIMERA_HELLO")` plus Oct comparison, or a small `Make.Env` record if env presence must be distinguished from empty value. | Yes | MAKE16-H1 |
| `Examples/ChimeraHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "command -v cargo >/dev/null"])` | Shell-shaped tool probe duplicates existing `Make.Tool`. | `let _cargo = Make.Tool("cargo")?`, optionally wrapped by a local helper to preserve custom error text. | No | MAKE15-H1 |
| `Examples/ChimeraHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "command -v go >/dev/null"])` | Same shell-shaped tool probe. | `let _goTool = Make.Tool("go")?`. | No | MAKE15-H1 |
| `Examples/ChimeraHello/Make.oct` | `BuildGoBinaryTarget` | Command target runs `bash -c` with `mkdir -p`, `go env`, and `go build`. | This is a real shell command target, not just a probe. `mkdir` could be typed, but dynamic `go env` path construction is currently shell-driven. | Keep as `Make.Exec`/command target for now; future split could use a function target with `Make.MkdirAll` plus a typed `Make.GoEnv`/host env primitive, or use stable output paths. | Maybe | MAKE17 |
| `Examples/ChimeraHello/Make.oct` | `RunChimeraTarget` | Command target runs `bash -c ./out/$(go env GOOS)-$(go env GOARCH)/chimera-hello`. | Shell is used to compute the same dynamic path. | Keep for now, or pair with the same future path/platform primitive used by build. | Maybe | MAKE17 |
| `Examples/ChimeraHello/Make.oct` | `Clean` | `Make.Remove` | Already typed. | Keep. | No | None |
| `Examples/ChimeraOctxHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "test \"$OCT_CHIMERA_OCTX_HELLO\" = 1"])` | Shell required for basic environment gate. | `Make.Getenv("OCT_CHIMERA_OCTX_HELLO")` plus Oct comparison, or `Make.Env`. | Yes | MAKE16-H1 |
| `Examples/ChimeraOctxHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "command -v cargo >/dev/null"])` | Shell-shaped tool probe duplicates existing `Make.Tool`. | `let _cargo = Make.Tool("cargo")?`. | No | MAKE15-H1 |
| `Examples/ChimeraOctxHello/Make.oct` | `CheckTools` | `Make.Exec("bash", ["-c", "command -v go >/dev/null"])` | Same shell-shaped tool probe. | `let _goTool = Make.Tool("go")?`. | No | MAKE15-H1 |
| `Examples/ChimeraOctxHello/Make.oct` | `Clean` | `Make.Remove` | Already typed. | Keep. | No | None |
| `internal/prometheus/Make.oct` | `Plan` / `BuildPrometheusTarget*` | `Program: "bash"`, script path argument | This is a command target invoking an existing platform build script. It is not a probe and should remain a command target until the native build itself is modeled. | Keep. If later typed, model as a target-specific native build helper rather than generic shell prohibition. | No | None |
| `internal/prometheus/Make.oct` | `ListEnvironment`, `CheckTools`, `Clean` | Stubs return `10` | No shell-shaped logic in current file. | Future real implementation should use `Make.Tool`, `Make.Exists`/`IsFile`/`IsDir`, and `Make.Getenv` rather than shell probes. | Maybe | When implemented |
| `Libraries/MakeOctestPlan/Make.oct` | `Build` | Stub returns `0` | No shell-shaped logic. | Keep. | No | None |


## MAKE15-H1 follow-up: typed Make.oct idiom cleanup

MAKE15-H1 implemented the Chimera example tool-probe cleanup without adding Make attributes, parser support, or Make execution semantics. MAKE16 then added the missing typed environment primitive and finished the Chimera gate cleanup.

- `Examples/ChimeraHello/Make.oct` now uses `Make.Tool("cargo")` and `Make.Tool("go")` through small fallible `RequireCargo` and `RequireGo` helpers. The helpers use fallible `match` to preserve the example-specific custom error messages.
- `Examples/ChimeraOctxHello/Make.oct` uses the same fallible helper idiom for its Cargo and Go probes.
- `Libraries/Make` now exposes `Make.Env(name) -> Make.EnvValue ! Error`, where `EnvValue` carries `Name`, `Present`, and `Value` so missing variables and present-empty variables are distinguishable.
- The Chimera `OCT_CHIMERA_*` opt-in gates now use local Oct helpers over `Make.Env` instead of `bash -c test ...`.
- Real shell command targets remain unchanged: the Chimera build/run commands and Prometheus native script invocation are command targets, not tool-discovery or environment-gate probes.

The MAKE16 environment-gate cleanup is implemented. `Make.Env("NAME")` is an explicit make-authorized host read of one variable; it is not ambient environment hashing, and it is not automatically part of `CommandTarget` identity unless the value is explicitly put in `CommandTarget.Env`.

## Missing Make primitive recommendations

### 1. Is `Make.Getenv` needed?

Yes. The current examples need to read a single environment variable to preserve a human-controlled gate without invoking a shell. This is the most obvious missing primitive.

Recommended smallest API:

```oct
record EnvValue {
    Name: String
    Present: Bool
    Value: String
}

fn Env(name: String) -> EnvValue ! Error
```

If the language/library convention prefers simpler names, `Make.Getenv(name: String) -> Make.EnvValue ! Error` is also fine. A record is preferable to returning only `String` because it distinguishes missing from present-but-empty, which matters for build gates and diagnostics.

### 2. Is `Make.RequireTool` useful or too magical?

Too magical for H1. Existing `Make.Tool(name)` is the right composable primitive. If examples need custom messages, they can use a tiny local helper once match/error ergonomics permit it, or `doctor` can improve `Make.Tool` diagnostics. Avoid growing one-off `Require*` helpers before there are repeated patterns.

### 3. Should `Make.Tool` return a record instead of relying on exit codes?

`Make.Tool` already returns a `String ! Error`, which is more typed than `ProcessResult` exit-code inspection. A future `ToolInfo` record could include `Name`, `Path`, and maybe `Version`, but changing `Tool` now is unnecessary and disruptive. If richer metadata is needed, add `Make.ToolInfo(name: String) -> Make.ToolInfo ! Error` instead of changing `Tool`.

### 4. Smallest primitive/helper set to eliminate current Bash probes

- Use existing `Make.Tool` for `cargo` and `go` probes.
- Use implemented `Make.Env` returning a presence-aware record.

That pair eliminates the current `command -v` and `test "$ENV" = 1` probes without adding broad shell restrictions or one-off require helpers.

## Recommended staged roadmap

### MAKE15-H1: typed probe cleanup with existing APIs

Replace shell-shaped `command -v` probes in Make files with `Make.Tool` where custom error behavior can be preserved or acceptable diagnostics are agreed. Do not touch shell command targets that perform real builds.

### MAKE16-H1: environment primitive

Implemented: `Make.Env` returns a presence-aware `EnvValue` record, and Chimera gates compare the returned value in Oct rather than through `bash -c test ...`.

### ATTR-MAKE1: parser and AST metadata only

Implement Make.oct-only bare attributes `[MakePlan]`, `[Pure]`, `[NoWhile]`, and `[RequiresAuthority]` with closed parsing, duplicate rejection, function-only attachment, `.octest`/`Make.oct` separation, and dedicated AST fields. Enforce `[MakePlan]` shape only.

### ATTR-MAKE2: syntactic enforcement and doctor output

Enforce `[NoWhile]` by walking function bodies. Add `oct make doctor` output for plan marker presence, no-while status, and direct host primitive calls in unmarked functions as warnings.

### ATTR-MAKE3: authority marker validation

Require direct Make host primitive calls in Make.oct functions to appear only in `[RequiresAuthority]` functions, initially with direct-call analysis. Add transitive analysis after call graph handling is reliable.

### ATTR-MAKE4: Pure checker

Enforce `[Pure]` for Make.oct functions by rejecting direct calls to known Make host primitives, filesystem mutation/read primitives, process execution, tool lookup, environment reads, and obvious output effects. Keep this Make.oct-only.

### MAKE17: platform/path idiom improvements

Investigate whether Chimera command targets should stop using `bash -c` for `go env GOOS/GOARCH` path interpolation. This may require a portable platform/path primitive or a decision to keep stable declared output paths.

## Tiny cleanup performed after design

MAKE15-H1 later replaced the Chimera `command -v` probes with `Make.Tool` while preserving custom error strings through local fallible helpers. The environment gate still needs a new primitive to remove the remaining shell-shaped check cleanly.

## ATTR-MAKE1 implementation note: closed Make.oct-only attributes

ATTR-MAKE1 implements the minimal closed Make attribute surface described above without adding a general attribute system.

Accepted Make attributes are limited to files whose base name is exactly `Make.oct`:

```oct
[MakePlan]
[Pure]
[NoWhile]
fn Plan() -> Make.Plan {
    return Make.Plan { ... }
}

[RequiresAuthority]
fn CheckTools() -> Int ! Error {
    let _go = Make.Tool("go")?
    return 0
}
```

The parser still accepts Octest attributes only in `.octest` files. Ordinary `.oct` files, including `Main.oct`, `manifest.oct`, library files, and experiment files, continue to reject top-level attributes. Make attributes and Octest attributes are deliberately disjoint: `[Fact]`, `[Theory]`, `[Artifact]`, `[Benchmark]`, `[InlineData]`, `[Suite]`, and `[CycleTime]` are invalid in `Make.oct`, and `[MakePlan]`, `[Pure]`, `[NoWhile]`, and `[RequiresAuthority]` are invalid in `.octest`.

Make attributes are closed, first-party, and payload-free in ATTR-MAKE1. `[Pure]` is valid, while `[Pure("x")]` is invalid; the same no-payload rule applies to `[MakePlan]`, `[NoWhile]`, and `[RequiresAuthority]`. Unknown Make attributes are rejected with an `unsupported Make attribute [Name]` diagnostic.

Make attributes attach only to the next function declaration. They may not precede `record`, `enum`, `flow`, EOF, or any other non-function top-level shape. The AST representation remains specialized: Make markers lower into dedicated `FunctionDecl` booleans rather than a generic attribute list or metadata bag.

Implemented validation is intentionally small:

- Duplicate Make attributes are rejected.
- `[Pure]` and `[RequiresAuthority]` on the same function are rejected as contradictory in this first pass.
- `[MakePlan]` requires the function to be named `Plan`, accept zero parameters, and return exactly `Make.Plan`. Conventional unmarked `fn Plan() -> Make.Plan` remains accepted.
- `[NoWhile]` walks the function body AST and rejects nested `while` statements anywhere inside that marked function.
- `[Pure]` and `[RequiresAuthority]` are stored as metadata only. ATTR-MAKE1 does not enforce effect purity, does not require authority markers for host primitive calls, and does not change Make execution semantics or host authority behavior.

Deferred work remains intentionally scoped: ATTR-MAKE2 can surface marker metadata in doctor, plan snapshots, traces, or failure artifacts; ATTR-MAKE3 can validate authority use; ATTR-MAKE4 can implement real purity checks. This pass does not add user-defined attributes, macros, reflection, compiler plugins, decorators, attribute payloads, or a general purity system.
