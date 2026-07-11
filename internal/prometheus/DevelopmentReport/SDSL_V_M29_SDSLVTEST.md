# SDSL-V M29 — `.sdslvtest` design authority

## Scope and ownership

M29 is a test-only compute contract. Discovery, compilation, manifest
creation, process orchestration, timeout enforcement, Vulkan preflight,
readback, and formatting belong to the host. Test bodies and assertion
evaluation belong to GPU code. The GPU writes a fixed record per invocation;
the host chooses the lowest failing invocation deterministically.

The fixed generated interface is descriptor set `0`, binding `0`, a compiler
owned structured result buffer. Its record is version `1`, fixed width, and
contains failure state, assertion/source identity, invocation coordinates,
kind/component metadata, and expected/actual/tolerance bit lanes. It has no
pointers, strings, variable payloads, or user-visible resource declaration.

Facts create one case. Theories create one case for each `InlineData` row;
their values are compile-time constants and rows share a compiler group by
workgroup size. The manifest is the durable artifact: stable IDs are SHA-256
prefixes of canonical source path, function name, case kind, and row identity.
`--case` addresses that ID and is independent of transient ordering.

Assertions keep a local first-failure state and never early-return. Generated
epilogues write the final invocation record. Initial meaningful assertions are
`True`, `False`, `Equal`, `NotEqual`, and `Near` for Bool/Int/UInt/Float and
unambiguous small vectors; operands must be evaluated once.

The eventual native `sdslv_test_host` runs one selected case per process. The
parent compiles once, supplies the manifest and selected case, applies a
conservative deadline, and classifies abnormal exits, timeout, and device loss.
`PROMETHEUS_REQUIRE_VULKAN_HARDWARE=1` converts absent loader/device/compute
queue into an explicit failure; normal CI may compile and validate manifests
while honestly skipping execution.

## Current implementation boundary

This pass adds a first executable narrow path for the current M29 inline-HLSL
fixture. The compiler groups cases by workgroup size, emits a compiler-owned
HLSL dispatcher, compiles it through DXC, and invokes the separate
`sdslv_test_host` process per case. The host owns a set-0/binding-0 storage
buffer, four-word push constants (`case`, `row`, `width`, `height`), checked
result allocation, pipeline creation, polling, ABI readback, and JSON output.
The host is built by the canonical native manifest/build scripts but is not
linked to the reactor API or shader registry.

On this Windows checkout, `InlineHlslFacts.sdslvtest` produced four PASS JSON
results through real Vulkan after DXC SPIR-V generation: the inline `asuint`
Fact, two independently dispatched Theory rows, and the explicit 32-thread
launch Fact. This is hardware execution evidence for the bounded foreign HLSL
path, not a claim that arbitrary SDSL-V test bodies are supported.

## Explicitly deferred

Arbitrary user resources, descriptor schemas, textures/samplers, graphics
tests, whole-buffer host scripting, production registry IDs, arbitrary public
compute dispatch, pools/parallel test execution, and async/batch redesign are
all deferred. These limits keep M29 test-owned and prevent pollution of the
production shader registry.

## M28 handoff

M28 remains accepted for compiler/toolchain scope. M29 now supplies real
hardware dispatch/readback evidence for its `asuint` inline-HLSL fixture.

## M29a.1c canonical declaration closure

**SDSL-V M29a is complete.** The compiler-owned authority chain is now:

```text
.sdslvtest -> lexer/parser -> AST -> validator -> validate.ValidatedTestDecl
    -> validate.ValidatedTestCase -> canonical grouping projection
        -> bootstrap compiler -> HLSL/SPIR-V
        -> manifest projection -> native test host
    -> discovery/listing/replay
```

`ValidatedTestDecl` is produced only after `validate.Diagnostics` succeeds.
It owns typed Fact/Theory classification, function identity and exact spans,
typed InlineData rows (with row/value spans and source order), validated launch
metadata (with attribute/argument spans), and lexical Assert call metadata.
`ValidatedTestCases` is the only stable-ID builder. Its identity is normalized
source-relative path, function name, validated kind, then row index and typed
value text. Consequently group order and generated HLSL order cannot affect
`--case` replay. Discovery is a lossless manifest/CLI DTO projection and no
longer formats attribute expressions or builds IDs.

The bootstrap HLSL emitter remains deliberately temporary M29 scaffolding for
the committed fixtures. It consumes canonical grouping/case input, never the
host manifest;
its existing fixture branches are not semantic authority and must be deleted,
not extended, when M29b lowers validated bodies. M29b's exact entry point is:

```text
ValidatedAssertCall -> VD-MIR Assert op -> local first-failure state
-> generated epilogue -> fixed result ABI v1
```

Theory lowering likewise consumes `ValidatedTheoryRow` directly: parameter
declarations/types remain on the shared `Function`, values are typed constants,
and row identity, launch metadata, and stable case identity are already
available. No attribute reparse, source scan, or host-side arbitrary row
serialization is part of that contract. M29b does not begin in this change.

The native host contract is preserved: set 0/binding 0 result buffer, four-word
push constants, ABI v1 records, deterministic JSON, and per-selected-case
process isolation. Manifest schema v2 is deterministic and serialization-only:
it adds canonical function/attribute/row/value/launch spans, typed row values,
Assert metadata, stable identity, group/artifact identity, and foreign-target/
GPU-capability metadata. The host continues to consume its ABI-v1 fields;
schema v2 is backwards-compatible enrichment, not a host redesign.

Validation for this closure includes validator/declaration and discovery tests,
stable-ID preservation for the committed M29 fixture, generated-HLSL regression
coverage, CLI listing/replay coverage, and the preserved native/host lanes
recorded below. The next permitted implementation work is M29b Assert/body
lowering; it must retain this front-end handoff and must not redesign the host.
`GroupValidatedCases` derives groups solely from canonical cases and validated
launch metadata. Neither grouping nor bootstrap compilation accepts `Manifest`;
source guards and projection tests cover that one-way boundary, deterministic
grouping, Theory-row group sharing, typed/span/Assert preservation, unchanged
IDs, and deterministic JSON.

### Preserved native and Vulkan evidence (Windows)

The canonical Windows launcher built successfully from the VS x64 developer
environment. Native manifest parity (`prometheus_native_manifest -check`) and
`bash -n internal/prometheus/native/build_linux.sh` passed. On 2026-07-10 the
native host executed the committed M29 suite on **NVIDIA GeForce RTX 3070**,
NVIDIA driver **596.36**, Vulkan device API **1.4.329** (loader 1.4.350). All
four ABI-v1 result-buffer cases emitted deterministic PASS JSON:

- `sdslv-11e3deb3d1ad94f0071f3d8d` — inline-HLSL Fact
- `sdslv-5664efcb0ab3deb7eb8c871b` and `sdslv-a20bf18c1aa6672e75d2b267` — Theory rows 0/1
- `sdslv-ea0387cf37037ceec9e4083d` — explicit workgroup `[32,1,1]`, dispatch `[1,1,1]`

Exact replay of `sdslv-5664efcb0ab3deb7eb8c871b` also passed. This proves the
preserved fixed result ABI, deterministic JSON readback, per-row execution,
and explicit 32-thread launch for the current bounded bootstrap path.

## M29a first-class declaration boundary

### M29a.1 span retrofit status

M29a.1 requires normal compiler-owned source positions because diagnostics for
an InlineData value or a single Assert operand cannot be derived safely from a
test package without introducing a second scanner. The canonical representation
is `source.Position` (zero-based UTF-8 byte offset, one-based Unicode-code-point
line and column) and `source.Span`, a half-open `[Start, End)` interval. The
zero span is unknown. Lexer tokens now receive complete cursor-derived spans,
including raw HLSL and EOF, and the parser propagates those spans through the
ordinary literal, unary, binary, call, field/index, guarded-read, paren, and
foreign-expression forms. The validator also exposes a compiler-owned
`ValidatedTestDecl` handoff whose rows, launch arguments, Assert calls, and
Assert operands refer to normal AST expressions and spans.

This is intentionally only infrastructure progress: the full compiler-wide
declaration/statement/expression retrofit and removal of the retained legacy
regex migration helper are still required before M29a.1 can be accepted. M29b
lowering has not begun.

M29a.1a now has parser-owned token/child span composition across the major
declaration, statement, type, and expression paths, plus a permanent AST
struct inventory test and representative recursive parsed-tree span audit.
The audit is deliberately compiler-side and performs no SDSL-V source
reconstruction. Exhaustive family fixtures and the full containment matrix
remain the next mechanical closeout before this slice is complete.

The audit is now promoted over the parseable shipped SDSL-V example corpus.
It checks every reached span-bearing AST node for a known in-bounds interval
and verifies containment through structural nesting. Omitted loop/reduction
step literals are explicit compiler-synthesized `1` values and are the sole
documented unknown-span exemption. Exact slicing coverage includes attributes,
functions, compound types, guarded reads/indexing, foreign HLSL, and Assert
calls. The remaining M29a.1 work begins structured diagnostics and test-model
consolidation; no M29b lowering has begun.

#### AST inventory and span policy

The source-originating concrete AST inventory is: Module; TemplateParam;
Attribute; TypeAliasDecl; RecordDecl; BoardDecl; StreamDecl; ConceptDecl;
ConceptField/ConceptGroup; ConfigField/ConfigDecl; EnumDecl/EnumVariant;
ShaderDecl; CompileDecl; FunctionDecl; NumThreads; ResourceDecl;
WorkgroupDecl; Field; Parameter; TypeRef; Block; LetStmt; ComptimeLetStmt;
AssignStmt; GuardedWriteStmt; ReturnStmt; ExprStmt; ForeignShaderStmt;
IfStmt; GuardWhenStmt/GuardWhenCase; FlowStmt/FlowBoardDecl/StateBlock;
ComptimeIfStmt; ComptimeMatchStmt/ComptimeMatchArm;
ComptimeWhenUtilityStmt/ComptimeWhenUtilityCase; ComptimeForStmt; ForStmt;
RequireStmt; StaticAssertStmt; IntegerLiteral; FloatLiteral; BoolLiteral;
StringLiteral; IdentifierExpr; ForeignShaderExpr; FieldAccessExpr; IndexExpr;
GuardedReadExpr; CallExpr; BinaryExpr; UnaryExpr; ParenExpr;
WhenUtilityExpr/UtilityCase; WithExpr; DeriveExpr/DeriveField; ReductionExpr;
FieldUpdate; EnumConstructExpr; BoardLiteralExpr; FieldInit; and
MatchExpr/MatchArm. Decl, Stmt, Expr, and ConceptMember are abstract storage
interfaces and therefore have no span field. UnsupportedDecl is a parser
recovery/synthetic node and may be unknown. Every other listed concrete node
must ultimately carry an exact source span; parents must contain their parsed
children. `Span.Contains` and `Span.Merge` make the unknown policy explicit.

Regex extraction is no longer discovery authority. The ordinary SDSL-V lexer
and parser now parse function-level attributes into `ast.FunctionDecl.Attributes`
using the existing typed `ast.Attribute` expression arguments. `.sdslvtest`
discovery loads the file through `source -> lex -> parse`, derives Fact/Theory
metadata from real declarations, validates launch constants and typed scalar
theory literals, and then derives the existing stable IDs and manifest cases.
Comment and string text cannot create test cases because lexer ownership comes
before parser attribute construction.

Function test attributes are rejected by normal validation in non-`.sdslvtest`
sources. The fixture-specific HLSL emitter remains a temporary M29 bootstrap,
but it now consumes AST-derived cases and must not gain new fixture branches.
M29b owns its deletion once Assert intrinsics, test-body lowering, source spans,
and epilogue state are represented in VD-MIR.

**M29a BLOCKED.** The AST/parser authority is established, but full M29a still
needs validator-owned duplicate/placement diagnostics, precise typed metadata
source spans, normal body validation with unlowered Assert intrinsics, and the
requested parser/validator coverage before it can be accepted independently.

### M29a validator follow-up

The normal validator now owns core function-attribute conflicts and test
function contracts, and `.sdslvtest` discovery invokes normal module
validation before deriving cases. `Assert.True`, `Assert.False`, `Assert.Equal`,
`Assert.NotEqual`, and `Assert.Near` are recognized only in test source and
have front-end arity/type validation; they remain `void` intrinsics with no
VD-MIR, HLSL, or GPU behavior. M29b replaces the bootstrap emitter by lowering
these validated call sites into local failure state and the fixed result ABI.

**SDSL-V M29a BLOCKED.** Exact attribute/argument source spans in typed
metadata, validator-owned launch/InlineData constant diagnostics, a single
exported validated-declaration handoff model, and the requested dedicated
coverage remain incomplete.

### M29a.1b structured validator diagnostics

M29a.1b introduces `internal/diagnostic.Diagnostic` as the compiler-wide
diagnostic schema. A diagnostic has a stable code (`SDSL-Vxxxx` for this
validator), severity, source path, complete `source.Span`, human message, and
ordered related locations. The validator's authoritative API is now
`validate.Diagnostics(ast.Module) []diagnostic.Diagnostic`; its older
`validate.Module` API is deliberately only a one-way renderer adapter for
lowering and toolchain callers that still accept `error`.

Diagnostics sort by path, byte offset, severity, code, and stable insertion
order. The CLI-compatible renderer prints `path:line:column: error CODE:
message`, followed by structured related notes. Unknown spans render as
`<unknown>` and are limited to genuinely source-independent failures.

M29 test-front-end rules now use exact AST-owned locations: duplicate and
conflicting test/launch attributes carry primary plus related spans; Fact and
Theory contracts identify the offending attribute/parameter/type; InlineData
identifies its row or value; launch validation identifies the precise argument;
and Assert arity/type failures identify the call or operand. Core body
validation likewise supplies precise spans for unknown identifiers, calls,
argument and assignment mismatches, operators, indexes, fields, returns, and
conditions. Legacy validator sites use the same structured collector and
module span while their individual minimum-span tightening is continued; they
are not a second string-error system. Parser and lexer failures remain on
their existing error paths for this milestone, but the direction is one
compiler diagnostic model.

The validator now establishes a scoped compiler-owned AST span through its
declaration, function, statement, type, and expression validation paths. This
means retained `errorf` call sites write structured diagnostics at the active
smallest enclosing node rather than using module-start fallbacks. Exact
subexpression spans are used where a distinct offending operand, call,
attribute, value, or condition exists. Duplicate declarations and fields use
the later declaration as primary and retain the first as a structured related
location. A module span is reserved for malformed synthetic recovery input for
which no source node exists.

`.sdslvtest` discovery now calls `validate.ValidatedTests` and converts its
validated AST declarations into manifest cases. The legacy regex scanner and
all independent parsed-discovery validation of test attributes, rows, launch
metadata, and parameter types have been removed. Consequently the validator
is the sole semantic authority before manifest preparation.

Permanent coverage exercises structured data and rendering (including unknown
and multiline spans), ordering, related locations, precise M29/core spans,
the legacy adapter, discovery authority, and CLI nonzero error rendering.
Generated HLSL and stable test IDs remain regression-covered. Parser and lexer
still return their established Go errors in this milestone: this is an explicit
deferred compiler-wide migration, not a competing validator model. M29a.1c may
adopt the existing `internal/diagnostic` schema at those boundaries. M29b
assertion lowering, GPU behavior, and Vulkan-host work have not begun.

## M29b VD-MIR lowering seam

M29b removes the bootstrap fixture-body emitter. `test.Prepare` now sends the
normal parsed module through `lower.ModuleForTarget(..., "HLSL")`; the suite
retains that compiler-owned VD-MIR result and `test.Compile` consumes it when
emitting a compatible compilation group. The test backend owns only generated
test functions, the case/row dispatcher, hidden ABI-v1 result buffer, local
failure state, and the single epilogue write. It neither reparses source nor
uses test names, source filenames, or hard-coded expected values to decide
generated behavior.

Assert calls are retained as a VD-MIR intrinsic call shape and lowered in
lexical order. Each operand is first assigned to a fresh local, left-to-right,
then compared once. A failed assertion records local first-failure state and
does not return; ordinary control flow continues and the generated epilogue
writes exactly one ABI-v1 record. Float and integer values use `asuint`, Bool
uses 0/1, and unused lanes remain initialized to zero. The linear result index
is `x + y * width + z * width * height`, matching host allocation order.

Theory functions are emitted once per function in a group. Dispatcher cases
materialize each selected typed row as function arguments, so rows do not
cause recompilation. This implementation preserves the fixed native host
contract and leaves barrier behavior unchanged: assertions add no early-return
or discard and cannot make an otherwise invalid divergent-barrier program
valid. Fresh M29b RTX execution evidence is recorded below.

### Dedicated assertion operations

The transitional call shape is now consumed at the lowering boundary and is
replaced by `vdmir.AssertStmt`. It contains the validated assertion kind,
lowered expected/actual/tolerance operands, call and operand spans, lexical
index, scalar value kind, and component count. Consequently the HLSL test
emitter does not receive `ValidatedAssertCall`, inspect `Assert.*` AST syntax,
or infer assertion behavior from any manifest field. `Assert.Near` rejects a
negative literal tolerance (`SDSL-V1404`); at runtime negative tolerances and
NaNs fail deterministically, while infinities pass only when exactly equal.

### Fresh real-lowering execution evidence

On 2026-07-10, the native Vulkan host executed the new source-derived
`RealAssertions.sdslvtest` suite: `ScalarAssertions` exercised True, False,
exact Bool/Int/UInt/Float Equal, NotEqual, and Near; two Theory arithmetic
rows (`[1u,2u]` and `[4u,5u]`) both passed through one lowered body. The
existing `InlineHlslFacts.sdslvtest` stable IDs also remained green. A focused
run of intentionally failing `FirstFailureWins` returned
`ASSERTION_FAILED`, `assertion_id: 0`, source `[28,33]`, expected bits
`[1,0,0,0]`, and actual bits `[2,0,0,0]`, proving ABI-v1 readback and local
first-failure ownership for a real lowered body. The same RTX 3070 host and
ABI-v1 transport described above were used.

## M30 handoff

M29's fixed ABI-v1 result path and native host ownership remain intact in M30.
The new milestone extends that boundary with exactly one compiler-owned
read-only test input resource at set `0`, binding `1`. Parser, validator,
validated metadata, VD-MIR, shared HLSL, manifest projection, and the native
host all preserve the M29 ownership rule: compiler layers define semantics,
the manifest serializes them, and the host only uploads and binds the payload.
M30 must not reopen fixture-specific emitters, host-owned semantic validation,
or arbitrary descriptor/resource declaration design.

## M29b XYZ execution hardening (2026-07-10)

The generated compute wrapper now threads its ordinary
`SV_DispatchThreadID` value into every lowered `.sdslvtest` body. This closes a
real shared-path gap: validator and VD-MIR builtin references previously
survived lowering, but the test wrapper emitted the lowered function as a
helper without supplying the builtin. No test-only coordinate operation or
second ordinary emitter was added.

The original XYZ hardening pass used one suite with a `[2,3,2]` workgroup and
`[2,2,2]` dispatch (48 invocations). On the RTX 3070 native host:

- `sdslv-41ce8cd46dad8ba707888b6f` passed while checking all three dispatched
  coordinate bounds in the ordinary body;
- `sdslv-c4848b19d591e0f9060513d3` deterministically reported the sole failing
  invocation `[3,4,2]`;
- `sdslv-7ec9b74a55e73b1c28ee22a0` had failures at two coordinates and
  deterministically reported `[1,1,0]`, the lower record under
  `x + y * width + z * width * height` scan order.

The passing case remains in `XYZInvocationIndexing.sdslvtest`. The two
intentional failures subsequently moved to
`testdata/language/valid/XYZAssertionFailures.sdslvvalid`; their historical
IDs above document the pre-migration run and are not public replay identity.

Each failure was replayed twice with byte-identical ABI-v1 JSON. Permanent
native integration tests distinguish a normally completed Vulkan process
returning `ASSERTION_FAILED` from host/process failure, verify structured
stable ID, coordinates, assertion/source identity, and deterministic zero
unused lanes. `FirstFailureWins` is now also a first-class expected-failure
integration contract with exact UInt expected/actual words.

This slice does not declare the full M29b matrix complete. Remaining closure
still includes the full Near special-value executable matrix, stronger
exactly-once side-effect proofs, broader ordinary control-flow/type coverage,
and consolidated fresh hardware evidence for every required case.

## M29b fixture corpus and native launcher hardening

M29b now uses three extensions with separate ownership. `.sdslvtest` remains
the only normal user-runner input. `.sdslvvalid` is a focused valid-language
fixture whose runtime expectation may be PASS or `ASSERTION_FAILED`, while
`.sdslvinvalid` carries Go-table-owned expected phase, diagnostic code, and
source location. Normal directory discovery is permanently tested to ignore
both fixture extensions. Intentional assertion and XYZ failures moved under
`internal/sdslv/testdata/language/valid`; malformed Assert, Theory, and
TestInput examples live under the corresponding `invalid` corpus. Existing
public M29 `.sdslvtest` stable IDs remain unchanged.

The canonical Windows build body now captures every required command's status
before control transfer, names the failed stage, exits with the original code,
and verifies the reactor, host, and required Marionette executables exist. A
Windows Go regression injects a deterministic compiler command returning 17
and proves the build reports `compile common native sources` and exits 17.
The authoritative launcher then completed from a normal PowerShell session via
VS 2026 x64 tools in 101.455 seconds and freshly built
`sdslv_test_host.exe`, `prometheus_reactor.dll`, and the required Marionette
executables.

The valid fixture corpus includes the complete scalar `Assert.Near`
special-value contract. Fresh RTX 3070 execution passed finite-within,
equal-infinity, and zero-tolerance cases and returned exact ABI-v1 bit words
for finite-outside, opposite infinities, NaN expected, NaN actual, and a
runtime negative tolerance. NaN payload `0x7fc00001` was preserved exactly.

`OrdinaryBodyCoverage.sdslvtest` adds real shared-emitter hardware coverage for
helper calls, runtime loops, assignments, nested if/else, records, payload
enums and match, foreign HLSL statements, and multiple assertions. The test
emitter now emits ordinary module type/helper declarations while attaching
the test invocation/failure parameters only to actual test functions; no
duplicate ordinary statement or expression emitter was introduced.

## M29b final assertion/ABI closure (2026-07-11)

The final closeout pass made assertion operand materialization explicit in the
shared HLSL backend. Every `vdmir.AssertStmt` operand now passes through one
backend-owned materialization point. Ordinary expressions emit one local
initializer; inline HLSL expression operands declare one local and assign it
inside the preserved foreign block; supported guarded-read operands declare
one local, use the normal guarded-read lowering once, then compare the local.
The validator was widened only for top-level Assert operands, so general
guarded-read placement remains bounded.

Permanent compiler evidence now covers VD-MIR source-order ownership for
Assert.True, Assert.False, Equal, NotEqual, and Near operands, including inline
HLSL and guarded-read operands. Emitted HLSL contains `sdslv_once`
temporaries in expected-then-actual order, and Near
expected-then-actual-then-tolerance order. Artifact tests also guard against
inline-HLSL or guarded-read fallback placeholders, assertion-introduced
`return`/`discard`, and epilogue bypass. Theory rows continue to dispatch
through one lowered body and materialize row arguments without recompilation.

`ExactlyOnceOperands.sdslvvalid` is the executable proof fixture. Its passing
cases use inline HLSL expression operands that mutate local counters; any
duplicated evaluation or reversed operand order changes the counter/order word
and fails on hardware. The Theory case combines row arguments with
`TestInput.UInt[index]`, a guarded read from the same fixed input buffer, and a
Near tolerance operand with observable counter mutation. The first-failure
case records an earlier UInt failure, then contains later inline-HLSL operand
evaluation and a final counter assertion in ordinary control flow; emitted
HLSL proves those later operands remain after the first failure and the native
result proves the first failure record is not overwritten.

The native host now serializes all ABI-v1 fields for both passing and failing
records: `abi_version`, `failed`, invocation coordinates, assertion/source
identity, value kind, component count, and expected/actual/tolerance words.
Passing records report ABI version 1, `failed = 0`, assertion/source zero
sentinels, exact invocation coordinates, value kind 0, component count 1, and
zeroed value lanes. Failing records report ABI version 1, `failed = 1`, exact
assertion ID/source location/invocation, value kind (`1` Bool, `2` Int, `3`
UInt, `4` Float), component count 1, exact bit words, and zeroed unused lanes.
Repeated pass and failure runs compare byte-identical JSON.

Fresh hardware execution ran on NVIDIA GeForce RTX 3070, NVIDIA driver
596.36, Vulkan loader/instance version 1.4.350, and Vulkan device API version
1.4.329. The result ABI was v1 throughout.

Fresh RTX 3070 evidence:

- normal M29 `.sdslvtest` discovery executed only user suites and passed
  `InlineHlslFacts`, `OrdinaryBodyCoverage`, `RealAssertions`, and
  `XYZInvocationIndexing`;
- M30 fixed input execution passed all seven existing stable cases, preserving
  binding 0 result and binding 1 fixed `TestInput`;
- the Assert matrix verified True/False, Bool/Int/UInt/Float Equal,
  NotEqual, finite Near pass/fail, zero tolerance, NaN expected, NaN actual,
  equal infinities, opposite infinities, and runtime negative tolerance with
  exact ABI words;
- `ExactlyOnceOperands.sdslvvalid` passed the exactly-once,
  left-to-right, Theory/materialization, indexed-resource, and guarded-read
  cases, and returned the expected first-failure record for the non-aborting
  continuation case;
- XYZ execution retained the `[2,3,2]` workgroup, `[2,2,2]` dispatch, 48
  invocation geometry, deterministic sole-failure coordinate `[3,4,2]`, and
  deterministic multi-failure lowest coordinate `[1,1,0]`;
- expected-failure integration continues to treat a normally completed Vulkan
  dispatch returning `ASSERTION_FAILED` as a successful integration contract;
- stable replay with M30 `GuardedReadUsesSource` preserved exact selected-case
  behavior.

Final validation for this closure included `go test ./internal/source`,
`go test ./internal/diagnostic`, `go test ./internal/sdslv/...`,
`go test ./cmd/oct`, focused valid and invalid language fixture corpus tests,
focused expected-failure/XYZ/Near/exactly-once/first-failure/ABI/stable replay
native host tests, `go run ./cmd/oct sdslv test examples/SDSL-V/M29`,
`go run ./cmd/oct sdslv test examples/SDSL-V/M30`, and the canonical Windows
native build through `internal\prometheus\native\build_windows_launcher.cmd`,
which rebuilt `sdslv_test_host.exe`, `prometheus_reactor.dll`, and the
Marionette executables with verified outputs.

At this point M29b has closed its motivating contract: `.sdslvtest` executes
normal GPU-native SDSL-V bodies through VD-MIR and the shared HLSL emitter,
with source-mapped, typed, deterministic, replayable assertion records. M31
has not begun.
