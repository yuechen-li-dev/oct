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

## Acceptance status

**SDSL-V M29 BLOCKED.** The fixed execution path works for the current bounded
inline-HLSL fixture, but completion requires real SDSL-V AST/VD-MIR lowering of
assertions and bodies (rather than the fixture-specific emitter), intentional
failure and invalid-ABI fixtures/tests, M27 guarded-read execution without
introducing user resources, robust source locations, process-level timeout
enforcement by the parent runner, and the requested committed evidence files.

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
