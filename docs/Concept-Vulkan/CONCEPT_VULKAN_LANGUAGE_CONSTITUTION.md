# Concept/Vulkan language constitution

Status: **normative EVT1 M1B-D constitution in success state; kernel-54 proof accepted; payload enums, exhaustive match, mutable structs, named concept requirements, constrained template monomorphization, bounded pure comptime evaluation, foundational control flow, fixed-size compile-time arrays, and finite structural validation implemented; the current worktree also carries an experimental DragonGod M1 typed-automata runtime vertical with fixed local instances and deterministic dispatch; production remains handwritten**

Date: 2026-07-24

## 1. Identity and authority

The language is **Concept/Vulkan**. Its source extension is `.concept`, and a
source unit begins with:

```concept
profile Vulkan;
```

Concept/Vulkan is a real, imperative, C-family/Concept-like systems-language
profile for host-side Vulkan mechanisms. It is Concept-compatible in direction;
the current experimental Concept compiler is not its production dependency and
need not accept the M1 surface.

Concept/Vulkan is implemented through the working Go-based Oct/SDSL-V compiler
lineage. It initially supports only the surface proven necessary by current
Prometheus Vulkan mechanisms. It is not a general Concept revival, a declarative
reactor recipe, a Vulkan framework, or a policy language.

The permanent responsibility split is:

```text
Dominatus decides and coordinates.
Concept/Vulkan mechanisms execute committed work and report facts.
SDSL-V expresses shader-side computation.
Prometheus exposes semantic GPU capabilities and consumes generated C/H.
```

Generated C/H is the first backend and the native-build boundary. Prometheus
must not require a future general Concept compiler at downstream build time.

## 2. Governing rule

```text
Essential decisions remain explicit.
Mechanical consequences are generated.
```

An **essential decision** is a choice that can change correctness, ownership,
compatibility, admission, observable behavior, or the Vulkan contract. Current
examples are:

- the already-admitted package variant;
- logical resource role, byte range, usage, sharing, and required memory
  properties;
- whether a resource is imported, borrowed, or owned;
- descriptor binding contract and any binding not fixed by package metadata;
- dispatch dimensions when they are semantic inputs;
- declared reads, writes, host transfers, and required ordering;
- a non-derivable stage/access/layout override;
- the existing Prometheus error/detail mapping and observation boundary.

A **mechanical consequence** follows deterministically from those decisions and
repository-owned metadata. Current examples are:

- zero-initialized Vulkan create-info structures and `sType` fields;
- requirements queries, exact allocation-size propagation, zero-offset binding,
  and map/unmap calls through the Stage 5 helpers;
- descriptor pool sizing, allocation, and writes implied by typed bindings;
- a barrier whose stage/access pair is uniquely implied by adjacent declared
  accesses;
- command-buffer begin/end, pipeline/set binding, submission, fence wait, and
  reverse cleanup;
- failure branches that preserve an existing Prometheus error and cleanup order.

Generation must not erase an essential decision or invent policy.

## 2.1 M1 source naming

Concept/Vulkan follows the C++/Concept syntactic lineage. User-facing
functions, compiler-known operations, and type names use `PascalCase`; parameters
and locals use `camelCase`. Existing C ABI spelling is retained only at the
backend boundary, and MIR opcodes remain compiler snake_case. The canonical M1
source uses `Execute`, `CreateMappedEvidenceBuffer`, `BindDescriptor`,
`BeginCommands`, `DeclareAccess`, `Dispatch`, `SubmitAndWait`, and
`ReadObservation`. M1 enforces function/local naming in its bounded parser;
this is not a general style-lint subsystem.

## 2.2 M1 declaration grammar

Concept/Vulkan uses C++-shaped declarations, deliberately distinct from Oct,
SDSL-V, and Rust:

```concept
Result<ProbeEvidence, PrometheusError> Execute(
    borrow MechanismContext context,
    unsafe imported borrow AccelerationStructure admittedTlas)
{
    owned MappedEvidenceBuffer evidence =
        CreateMappedEvidenceBuffer(context)?;
}
```

Return types precede function names; parameter and local types precede their
names. `fn`, `name: Type`, `-> ReturnType`, `let`, and `var` are rejected;
there are no compatibility aliases. `borrow`, `owned`, `unsafe`, `imported`,
and `move` remain explicit ownership/boundary vocabulary, while `?` retains its
bounded fallibility meaning. `MappedEvidenceBuffer` is the narrow source
spelling of M1's existing mapped host-visible evidence-buffer capability;
`ComputePipeline`, `DescriptorSet`, `CommandRecording`, and `Submission` are
the existing M1 capability names, not new runtime abstractions.

## 2.3 EVT1 M1A payload enums and exhaustive match

EVT1 M1A adds payload enums and exhaustive `match` without changing the
established C++-shaped declaration form:

```concept
enum PipelineState
{
    Empty,
    LayoutCreated(PipelineLayout layout),
    Ready(PipelineLayout layout, Pipeline pipeline),
    Failed(VulkanError error)
}
```

Variant construction and patterns stay qualified by the enum type:

```concept
PipelineState::Ready(layout, pipeline)
PipelineState::Failed(error)
PipelineState::Empty
```

`match` uses Rust-style fat-arrow arms while keeping Concept/Vulkan source
otherwise C++-shaped:

```concept
return match (state)
{
    PipelineState::Empty => 0,
    PipelineState::Ready(layout, pipeline) => 2,
    PipelineState::Failed(error) => error.Code,
};
```

Statement-form `match` requires braced blocks, expression-form `match` requires
single expressions, the subject is evaluated exactly once, and every declared
variant must be covered exactly once. M1A does not add wildcard arms, guards,
or nested destructuring patterns.

## 2.4 EVT1 M1B-A structs and named concepts

EVT1 M1B-A adds ordinary mutable `struct`, bounded `immovable struct`, and
named compile-time-only `concept` requirements without changing the established
C++-shaped declaration direction:

```concept
struct BufferRange
{
    VkBuffer buffer;
    int offset;
    int size;
};

immovable struct CommandPoolState
{
    VkCommandPool pool;
    bool initialized;
};

concept Validatable<T>
{
    requires bool IsValid(borrow const T value);
}

requires Validatable<CommandPoolState>;
```

Ordinary structs are mutable value types with declaration-ordered fields and
transparent field-ordered C11 layout. Positional aggregate construction uses
exactly one canonical surface:

```concept
BufferRange range = BufferRange{buffer, 0, 4096};
```

Field access remains dotted and nested field access is preserved:

```concept
range.offset
allocation.range.size
```

`immovable struct` means mutable but non-relocatable under the current bounded
rules: it may be constructed directly in final local storage and mutated
through valid mutable borrow paths, but it may not be copied, whole-value
assigned, passed by value, returned by value, embedded by value, or placed
into enum payloads by value.

Named concepts are one-parameter compile-time propositions only. They describe
required free-function signatures and prerequisite named concepts. Concrete
assertions emit no runtime table, vtable, witness object, registry, or public
symbol.

## 2.5 EVT1 M1B-B constrained templates and deterministic monomorphization

EVT1 M1B-B adds one deliberately bounded compile-time generic facility:

```concept
template <typename T>
requires VulkanResource<T>
void DestroyResource(borrow T value)
{
    Destroy(value);
}
```

Invocation is explicit and concrete:

```concept
DestroyResource<PipelineState>(state);
```

M1B-B supports exactly one free-function template parameter, exactly one named
concept constraint over that same parameter, symbolic body checking against the
constraint closure, and deterministic monomorphization of each invoked
`(template identity, concrete type identity)` pair into one private C11
function instance.

Templates remain compile-time recipes only. Concepts remain compile-time
propositions only. There is no runtime generic machinery, deduction,
specialization, SFINAE, witness table, vtable, or public ABI growth.

## 2.6 EVT1 M1B-C bounded pure comptime evaluation and foundational control flow

EVT1 M1B-C adds a deliberately bounded compile-time evaluation surface:

```concept
comptime int ClampCount(int value, int maximum)
{
    return if (value < maximum) value else maximum;
}

comptime int LoopBound = 4;
static_assert(LoopBound > 0, "LoopBound must be positive");
```

Compile-time declarations and free functions use the explicit `comptime`
keyword. `static_assert` accepts a compile-time `bool` condition and optional
compile-time `string` message. Expression-valued `if` uses the exact form
`if (condition) then_expression else else_expression`. Ordinary `while`
becomes available as foundational runtime control flow, while compile-time
`while` is accepted only with an explicit `bounded(limit)` clause.

The accepted compile-time value domain now includes `int`, `bool`, `string`,
enums, and structs composed from those values. EVT1 M1B-D extends that domain
with fixed-size compile-time arrays and leaves M1B-C itself as the accepted
foundational evaluator/control-flow substrate.

## 2.7 EVT1 M1B-D fixed compile-time arrays and finite structural validation

EVT1 M1B-D extends the accepted M1B-C substrate with a deliberately bounded
fixed-array facility for compile-time structure:

```concept
comptime int[3] RetryBudgets = [1, 2, 4];
comptime int[2][3] RetryMatrix = [[1, 2], [3, 4], [5, 6]];
static_assert(Len(RetryBudgets) == 3);
static_assert(RetryMatrix[2][1] == 6);
```

Array types use the exact suffix form `ElementType[LengthExpression]`, where
the length expression is evaluated in compile-time context and must produce a
non-negative `int`. Array literals use bracket form in source order; empty
literals require explicit contextual type. Indexing uses `array[index]`, where
the index must be a compile-time `int` and out-of-range access is rejected
deterministically. `Len(array)` returns the exact declared length.

Fixed arrays remain compile-time-only in EVT1: they are accepted in top-level
and local `comptime` declarations and in compile-time free-function parameter
and return types, but runtime locals, runtime parameters, and runtime return
types containing arrays remain rejected. Arrays may nest subject to explicit
bounds, may contain accepted compile-time enums and structs, may participate in
structural equality when their element types already support equality, and may
be traversed only through the existing bounded `while` form. No runtime
collection, iterator, metadata table, or `for` loop is introduced, and all
compile-time array structure erases before runtime C11 lowering.

## 2.8 DragonGod M1 typed automata instances and deterministic dispatch

DragonGod now includes the validated M0 declaration surface plus the smallest
runtime heartbeat:

```concept
automata ResourceLifecycle(LifecycleSignal)
{
    initial machine Main
    {
        initial state Empty
        {
            on LifecycleSignal::Create goto Ready;
        }

        terminal state Finished
        {
            finish;
        }
    }
}
```

```concept
instance ResourceLifecycle lifecycle;
AutomataDispatchOutcome outcome =
    dispatch(lifecycle, LifecycleSignal::Create);
```

The source hierarchy is exact and non-aliasing: `automata` declares one
validated family, `machine` declares one pushable finite-state machine within
that family, and `state` declares one state within a machine. `goto` targets a
state in the current machine. `push` targets another machine and retains an
explicit caller continuation state. `pop` resumes that retained caller
continuation, and `finish` terminates the full automaton.

DragonGod M0 binds one exact enum signal type per automata declaration. State
handlers use exact qualified nullary enum members with no wildcard, guard,
priority, or implicit fallthrough. Machine-local state cycles remain legal, but
the machine-push graph must be acyclic in M0. The compiler derives exact
maximum active machine depth from the longest reachable push chain, records that
depth and a stable graph identity in MIR/source-map evidence, and rejects
unreachable machines, unreachable states, root-machine `pop`, pushed machines
with no reachable `pop`, and root machines with no reachable `finish`.

DragonGod M1 adds one dedicated local runtime form: `instance AutomataName
localName;`. Instance initialization is compiler-generated, fixed-size, rooted
at the declared initial machine/state, and immediately normalizes terminal
completion until the instance is waiting in a nonterminal state or is finished.

`dispatch(instance, signal)` accepts only the instance family's exact nullary
signal enum and returns the compiler-owned `AutomataDispatchOutcome` enum with
members `Transitioned`, `Unhandled`, `Finished`, and `AlreadyFinished`.
Unhandled dispatch is non-mutating. `goto` changes state in-place. `push`
stores the caller machine plus explicit continuation state in a fixed stack
whose capacity is exactly `maximum_active_machine_depth - 1`. `pop` restores
that retained continuation. `finish` terminates the complete instance and
discards continuation state. Terminal completion is synchronous, so pushed
initial terminal states and resumed terminal continuations normalize
immediately in the same dispatch.

DragonGod M1 still emits no reflection registry, heap allocation, dynamic
graph lookup, string dispatch, function-pointer dispatch table, payload
runtime, or public Prometheus ABI growth. Declaration-only automata with no
`instance` usage still erase completely before runtime C11 lowering.

## 3. Static and runtime facts

Compile time may consume only deterministic compiler-owned or
repository-owned inputs:

- source declarations, types, layouts, constants, and bounded pass structure;
- checked shader-package identity, variant metadata, descriptor count,
  entry-point name, workgroup size, push-constant size, and requirements;
- target Vulkan contract and native ABI declarations selected by the build;
- declared resource accesses and fixed kernel requirements.

Compile time must not query a live GPU, driver, queue, memory heap, environment,
clock, network, or ambient filesystem. M1 admits repository inputs by explicit
path/configuration supplied to the compiler; it has no general comptime I/O.

Live extensions, features, limits, queue families, devices, memory types, and
allocation results remain runtime admission or committed execution facts.
Package requirements may be checked statically for internal consistency, but
the current device still admits them at runtime. A memory-type index is a
runtime mechanism result selected from explicit required properties and an
already-committed placement rule; it is not a compile-time fact.

## 4. Values, ownership, and identity

### 4.1 Minimum value categories

M1 distinguishes:

- `borrow T`: non-owning use valid only for the lexical call/scope;
- `owned T`: affine, move-only ownership with exactly one live drop obligation;
- plain copy values: fixed scalars, handles explicitly declared as observations,
  and immutable package/static descriptors;
- `unsafe imported T`: a privileged borrowed Vulkan object admitted through a
  typed host boundary.

There is no implicit copy of `owned` values. `move` transfers the drop
obligation and use after move is rejected. M1 uses lexical local checking; it
does not promise universal Rust-style lifetime proof or infer safety across
arbitrary foreign storage.

### 4.2 Deterministic destruction

Owned locals drop in reverse successful-initialization order on success, early
return, propagated failure, and generated cleanup edges. Partially constructed
values drop only initialized members. Moved values are not dropped. Drop is
idempotent at the existing helper boundary where the production helper already
supports repeated cleanup.

Submission creates an in-flight ownership obligation. A resource used by an
in-flight command cannot be dropped, remapped, moved to an unrelated owner, or
rebound until the existing completion operation succeeds or the existing
failure path quarantines/retains it. Dependent pipeline, descriptor, command,
buffer, acceleration-structure, and scene objects drop before the borrowed
common runtime/device owner.

### 4.3 Identities are not interchangeable

The type system and MIR must not conflate:

- language ownership;
- logical tensor/resource identity;
- immutable content/weight identity and hash;
- mechanical Vulkan allocation identity;
- descriptor binding;
- committed execution facts;
- slot/generation or arena reuse epochs;
- in-flight submission ownership.

A `VkBuffer` allocation is not a tensor, a binding, content, or authorization.
A package variant is not a Dominatus decision. A descriptor does not own the
resource it names.

## 5. Fallibility

Fallible functions return `Result<T, PrometheusError>` (or the profile's
equivalent fallible return spelling) and use explicit propagation. `Result` is
must-use. M1 may use `?` as surface syntax, but its exact parser spelling is an
implementation detail until the M1 grammar is committed.

Lowering preserves existing C behavior:

- every failure maps to an existing `PROM_*` return, stage, and detail value;
- no public error, ABI code, or failure-ordering rule is added;
- the first current failure at a production call boundary remains the reported
  failure;
- cleanup runs in the same dependency-safe order before returning;
- Vulkan/package facts remain observations, not policy choices.

Generated helpers may use a single cleanup epilogue when that is behaviorally
equivalent and source-mapped. They may not collapse distinct existing failure
codes or turn a package/admission failure into a generic error.

## 6. M1 resource and pipeline types

M1 requires only:

- `borrow MechanismContext`: admitted runtime/device/queue/command-pool and
  package services;
- `borrow AccelerationStructure`: an already-created, already-admitted object
  with an explicit lifetime contract;
- `owned Buffer<HostVisibleCoherent, Storage, T>` for mapped observation;
- `PackageComputeEntry<Bindings, PushConstants>` checked against an exact
  package/variant identity;
- a typed `DescriptorSet<Bindings>`;
- `ComputePipeline<Entry>`;
- a lexical `CommandRecording`;
- an affine `Submission` completed by the existing synchronous wait.

Device-local buffers, transfer staging, push constants, and multiple storage
bindings are expected M1 types when required by the selected operation, but the
first conformance specimen does not need all of them. Images, layouts,
samplers, full ray-tracing pipelines/SBTs, multiple queues, and general Vulkan
object coverage are deferred until production evidence demands them.

Bindings are structural contracts with an exact set, binding number, descriptor
kind, access, and element/range type. Package metadata may prove entry point,
workgroup, descriptor count, push-constant byte count, and static requirements.
It does not currently encode every descriptor kind, so M1 source states the
kernel-54 binding kinds explicitly and validation cross-checks the package
count. Derivation is permitted only when the package becomes authoritative for
the missing fact.

## 7. Access and synchronization

M1 declares access at each operation boundary:

- `host_write`;
- `transfer_read` / `transfer_write`;
- `shader_read` / `shader_write`;
- `host_read`;
- `acceleration_structure_read`;
- `descriptor_read` as a binding property, not a memory barrier category.

The compiler constructs an ordered per-resource access chain. It may derive a
barrier only when the adjacent accesses, queue-family relationship, and layout
state select one safe current Vulkan mapping. Examples include host write to
transfer read, transfer write to shader read, shader write to transfer read,
and transfer write to host read as used by current Prometheus paths.

M1's mapped coherent kernel-54 observation uses submission/fence completion for
device-to-host availability, matching the current synchronous path. The source
still declares `shader_write -> host_read`; the MIR records how the existing
mechanism satisfies it.

Ambiguous cases require a visually explicit profile operation:

```concept
unsafe vulkan.sync_override(
    resource: evidence,
    src_stage: ComputeShader,
    src_access: ShaderWrite,
    dst_stage: Host,
    dst_access: HostRead,
);
```

An override is checked for resource ownership and scope, appears in MIR and
generated comments, and affects only the named transition. Images additionally
require explicit layouts until a later typed image model proves safe inference.
No barrier or layout transition is hidden behind an uninspectable default.

## 8. Effects decision

M1 does **not** introduce a general annotation-heavy effect system. The same
call-edge mistakes are caught more directly by typed capabilities and lexical
state:

- allocation requires `borrow MechanismContext` and returns `owned`;
- mapping requires the host-visible buffer capability;
- recording operations require `borrow mut CommandRecording`;
- submission consumes a finished recording and produces `Submission`;
- wait consumes or completes `Submission`;
- host observation requires completed device writes.

MIR records `allocate`, `map`, `record`, `submit`, `wait`, `observe`, and
`unsafe_vulkan` effects for audit and future checking. A general effect syntax
is deferred unless multiple real call sites demonstrate that capability/state
types do not make the constraint legible.

## 9. Static requirements, templates, and comptime

The three mechanisms remain separate:

- `concept` describes a static shape or requirement;
- templates provide bounded, compile-time generic reuse;
- `comptime` performs deterministic, bounded evaluation;
- runtime polymorphism is unrelated and absent from M1.

EVT1 M1B-A fixes the first user-visible static requirement surface:

- `concept Name<T> { ... }` declares one named proposition over one type
  parameter;
- `requires ReturnType Function(Params...);` describes an exact required
  free-function signature;
- `requires OtherConcept<T>;` composes prerequisite concepts;
- `requires ConceptName<ConcreteType>;` explicitly asks the compiler to prove
  a concrete type satisfies a named concept.

These checks are structural, deterministic, and compile-time-only. They do not
create runtime interface machinery or widen the public ABI.

EVT1 M1B-B adds the bounded constrained-template consumer:

- `template <typename T>` declares exactly one type parameter;
- `requires ConceptName<T>` is required and must name exactly one existing
  one-parameter concept over that same template parameter;
- template bodies are checked symbolically against the named concept's ordered
  prerequisite closure and required operation set;
- `TemplateName<ConcreteType>(...)` performs explicit-only instantiation;
- each unique `(template, concrete type)` pair emits one deterministic private
  C11 helper and is reused for repeated calls.

EVT1 M1B-C now adds bounded pure compile-time evaluation and foundational
control flow:

- `comptime Type Name = Expression;` declares deterministic compile-time
  values at module or statement scope;
- `comptime ReturnType Name(...) { ... }` declares compile-time-only free
  functions;
- `static_assert(condition, optionalMessage);` checks compile-time boolean
  facts without emitting runtime machinery;
- `if (condition) then_expression else else_expression` is the accepted
  expression-form conditional;
- `while (condition)` is accepted at runtime, and compile-time `while`
  additionally requires `bounded(limit)`;
- the evaluator is deterministic, bounded, and pure over the currently
  accepted compile-time value domain.

EVT1 M1B-D closes the remaining fixed-array gap with compile-time-only array
types, literals, indexing, exact `Len(...)`, structural equality where valid,
and finite structural validation over ordered arrays.

## 10. Escape hatch

The only M1 escape is a typed `unsafe vulkan.<operation>` declaration or call
from a compiler-maintained allowlist. It must name:

- the exact Vulkan operation;
- every borrowed/owned handle and affected resource;
- declared pre/post access state;
- the existing failure mapping;
- why the profile cannot yet express the operation.

It is source-visible, MIR-visible, generated-comment-visible, and counted in a
machine-readable generation summary. It cannot embed arbitrary C, bypass drops,
manufacture ownership, weaken another resource's synchronization, query
Dominatus state, or call unlisted Vulkan symbols. M1 acceptance sets an explicit
maximum escape count for its specimen; the chosen capability probe requires no
escape for its normal dispatch path and one typed import boundary for the
prebuilt acceleration structure.

## 11. Diagnostics and source mapping

Every AST, typed node, MIR operation, cleanup edge, and generated helper retains
the `.concept` source path and span. Validation errors name the source
construct, package/variant fact, and conflicting production contract.

Generated C/H is deterministic and readable. It contains:

- a generated-file marker and source digest;
- stable helper/resource names derived from source declarations;
- comments with source path and line before operation groups;
- a sidecar source map from generated line ranges and MIR operation IDs to
  source spans;
- `#line` directives only where they improve compiler diagnostics without
  obscuring review.

C compiler failures are reported with both the generated location and mapped
Concept/Vulkan location. Generated code never claims that a Vulkan runtime
failure is a compile-time validation failure.

## 12. M1 semantic minimum

The first compiler milestone implements exactly one complete packaged
compute-operation conformance slice:

1. parse `profile Vulkan;` and one function;
2. load and strictly validate one repository-supplied shader package;
3. type a borrowed mechanism context and imported admitted acceleration
   structure;
4. select exact package variant `kernel-54-default`;
5. create one mapped host-visible coherent storage buffer;
6. validate typed bindings 0 (acceleration structure, read) and 1 (storage
   observation, write);
7. create package-backed module, layout, descriptor resources, and compute
   pipeline;
8. record one dispatch `(1, 1, 1)` with declared accesses;
9. submit and wait through the current synchronous mechanics;
10. read the mapped observation;
11. preserve all fallible exits and reverse cleanup;
12. emit deterministic checked-in C/H plus source map and generation manifest.

The specimen mirrors the bounded execution inside
`prom_ray_create_compute_resources` and
`prom_ray_query_triangle_scene_probe_impl` in
`reactor_vulkan_ray_query.c`. The acceleration structure is imported because
building BLAS/TLAS is a separate, substantially larger mechanism. Full
ray-query batch resource growth, descriptor rebinding, `ray_count` dispatch,
and result conversion remain the M2 equivalence target.

M1 does not require topology, Dominatus progression, adaptive choice, multiple
queues, scheduling, graph compilation, general templates, dynamic interfaces,
reflection, async, package management, or the full Concept language.

## 12.1 EVT1 M1A semantic minimum

EVT1 M1A adds the smallest coherent language vertical above the accepted M1D
proof foundation:

1. mixed unit and payload enums;
2. qualified unit and payload variant construction;
3. positional payload destructuring in `match` arms;
4. exhaustive statement-form and expression-form `match`;
5. deterministic typed MIR that preserves enum identity, variant order, tags,
   constructor nodes, and explicit match/pattern nodes;
6. deterministic C11 lowering to one explicit tag plus one explicit payload
   union with source-ordered fields;
7. exactly-once subject evaluation and exactly-once payload evaluation;
8. explicit invalid-tag handling through a private abort path;
9. native strict-C11 compilation and executable behavior for one
   hardware-independent specimen and one Vulkan-shaped specimen.

This milestone is intentionally language-focused. It does not widen production
Prometheus routing, public ABI, or the earlier kernel-54 handwritten witness.

## 12.2 EVT1 M1B-A semantic minimum

EVT1 M1B-A extends the accepted M1A proof with the smallest coherent host-side
type-and-requirement vertical:

1. ordinary mutable user-defined structs;
2. positional aggregate construction with exact arity and type checking;
3. field reads, nested field reads, and field mutation;
4. ordinary struct value-copy where all fields remain copyable under existing
   ownership authority;
5. explicit rejection of ownership-illegal struct copying;
6. bounded `immovable struct` final-storage construction and mutable borrow use;
7. rejection of immovable copy, whole-value assignment, by-value
   parameter/return, embedding, and enum-payload placement;
8. one-parameter named concepts with required free-function signatures;
9. prerequisite concepts, deterministic prerequisite traversal, and cycle
   rejection;
10. declaration-level concrete satisfaction assertions with exact-signature
   checking and no runtime representation;
11. typed MIR that preserves struct identity, field order, immovability,
   concept identity, requirements, and concrete assertions;
12. deterministic strict-C11 lowering for ordinary and immovable structs plus
   compile-time-only concept erasure;
13. one hardware-independent native specimen and one Vulkan-shaped native
   specimen proving structs, immovability, concepts, and preserved M1A enum /
   `match` behavior.

## 12.3 EVT1 M1B-B semantic minimum

EVT1 M1B-B extends the accepted M1B-A proof with the smallest coherent
constraint-consumer vertical:

1. one-parameter free-function templates with the exact header spelling
   `template <typename T>`;
2. exactly one named concept constraint over that same template parameter;
3. symbolic template-body checking against the ordered prerequisite closure;
4. exact requirement-bound dependent free-function calls in typed MIR;
5. explicit concrete invocation `TemplateName<ConcreteType>(...)`;
6. satisfaction checking before instantiation using the existing named-concept
   engine;
7. deterministic concrete substitution and exact concrete operation binding;
8. revalidation of ownership, borrowing, copyability, and immovability after
   substitution;
9. rejection of nested template invocation, recursive instantiation, type
   deduction, specialization, and runtime generic machinery;
10. one private deterministic C11 function per unique `(template, concrete
    type)` key with deduplication of repeated calls;
11. typed MIR that preserves template identity, constraint closure,
    requirement-bound calls, concrete instances, and generated symbols;
12. one hardware-independent native specimen and one Vulkan-shaped native
    specimen proving explicit-only instantiation, deduplication, exact
    concrete operation binding, and immovable borrowed use.

## 12.4 EVT1 M1B-C semantic minimum

EVT1 M1B-C extends the accepted M1B-B proof with the smallest coherent bounded
compile-time and control-flow vertical:

1. module-scope and statement-scope `comptime` declarations with explicit
   types and deterministic evaluation;
2. compile-time-only free functions with compile-time-safe parameter and
   return types;
3. module-scope and statement-scope `static_assert` with compile-time `bool`
   conditions and optional compile-time `string` messages;
4. expression-valued `if (condition) then_expression else else_expression`
   with exact branch-type agreement;
5. ordinary runtime `while` plus compile-time `while` gated by explicit
   `bounded(limit)` syntax;
6. deterministic evaluator fuel, call-depth, and bounded-loop limits with
   rejection of runtime calls during compile-time evaluation;
7. structural rejection of direct or indirect compile-time recursion;
8. accepted compile-time values for `int`, `bool`, `string`, enums, and
   structs composed from accepted compile-time values;
9. runtime lowering that erases compile-time declarations and assertions while
   substituting final literal or aggregate values into generated C11;
10. typed MIR and source-map evidence for compile-time declarations, static
    asserts, `if_expr`, `while`, `bounded_while`, unary operations, and string
    literals;
11. one hardware-independent native specimen and one Vulkan-shaped native
    specimen proving compile-time evaluation, bounded loops, and preserved
    earlier EVT1 behavior.

EVT1 M1B-D extends this minimum with the fixed-array closure listed next.

## 12.5 EVT1 M1B-D semantic minimum

EVT1 M1B-D extends the accepted M1B-C proof with the smallest coherent fixed
array and finite-structural-validation vertical:

1. fixed-array type spelling `ElementType[LengthExpression]` with deterministic
   compile-time length evaluation and explicit nested-suffix association;
2. fixed-array literals in source order with exact contextual typing and
   rejection of heterogeneous fallback or context-free empty literals;
3. compile-time-only fixed-array declarations, locals, parameters, and return
   types, with explicit rejection of runtime arrays;
4. exact `array[index]` validation and evaluation with compile-time `int`
   indexes and deterministic out-of-range rejection;
5. exact `Len(array)` validation and evaluation with no runtime metadata
   emission;
6. nested arrays plus arrays of accepted enums and structs, and structs
   containing accepted arrays;
7. structural equality for arrays only when element equality already exists,
   and explicit rejection of array ordering comparisons;
8. bounded `while` traversal over arrays with no new iteration protocol and no
   `for` syntax;
9. typed MIR/source-map/generated-C evidence for array literals, indexing,
   length inspection, and erased compile-time array results;
10. one hardware-independent native specimen and one Vulkan-shaped native
    specimen proving finite ordered structural validation and complete runtime
    erasure without runtime collections.

## 12.6 DragonGod M1 semantic minimum

DragonGod M1 extends the accepted EVT1 substrate with the smallest coherent
typed automata runtime vertical:

1. exact `automata -> machine -> state` declaration nesting with no aliases;
2. one exact signal enum type per automata family;
3. exactly one initial/root machine and exactly one initial state per machine;
4. explicit terminal states whose sole completion is `pop;` or `finish;`;
5. exact typed `goto` within one machine and exact typed `push` to another
   machine with an explicit caller continuation state;
6. duplicate `(machine, state, signal)` rejection;
7. acyclic machine-push topology with machine-local state cycles still legal;
8. deterministic machine/state reachability, pushed-machine reachable-`pop`,
   and root reachable-`finish` validation;
9. exact maximum active machine depth derivation and deterministic graph
   identity in MIR/source-map evidence;
10. fixed local `instance AutomataName localName;` storage with private machine
    and state ordinals plus continuation capacity derived from the validated
    maximum active depth;
11. exact typed `dispatch(instance, signal)` returning compiler-owned
    `AutomataDispatchOutcome`;
12. synchronous terminal normalization covering initial terminal states, pushed
    initial terminal states, resumed terminal continuations, root `finish`, and
    non-root `finish`;
13. deterministic strict-C11 lowering with no heap allocation, string lookup,
    runtime reflection, or public ABI exposure of automata internals;
14. complete runtime erasure for declaration-only automata that are never used
    through `instance`.

## 13. Profile MIR boundary

The M1 MIR is profile-specific and typed. Its operation vocabulary is:

| MIR operation | Current production lowering target |
| --- | --- |
| `borrow_context` | `prom_reactor_runtime_get_vk_services` / borrowed `prom_vk_runtime_services` |
| `import_acceleration_structure` | typed scene-owned TLAS handle at the existing probe boundary |
| `resolve_package_entry` | `prom_reactor_runtime_get_shader_package` plus `prom_shader_package_create_module` |
| `create_buffer` | `prom_vk_create_buffer` using Stage 5 mechanics |
| `map_observation` | mapped result returned by the existing buffer helper |
| `create_descriptor_layout` | kernel-54 `vkCreateDescriptorSetLayout` sequence |
| `create_pipeline_layout` | kernel-54 `vkCreatePipelineLayout` sequence |
| `allocate_descriptor_set` | descriptor-pool/create/allocate sequence |
| `bind_descriptor` | kernel-54 acceleration-structure and storage writes |
| `create_compute_pipeline` | package module plus `vkCreateComputePipelines` |
| `begin_recording` | `prom_ray_begin_command` |
| `declare_access` | typed validation input; no direct Vulkan call |
| `bind_compute` | `vkCmdBindPipeline` / `vkCmdBindDescriptorSets` |
| `dispatch` | `vkCmdDispatch(1, 1, 1)` |
| `end_submit_wait` | `prom_ray_end_submit_and_free` / `prom_ray_submit_command` |
| `observe_mapped` | existing post-wait `memcpy` from the evidence buffer |
| `drop` | `prom_vk_destroy_buffer`, Vulkan dependent-object destroys, module destroy |
| `fail_to_cleanup` | existing return/detail mapping plus reverse initialized drops |

MIR contains explicit ownership states, access chains, package identity,
source spans, and cleanup successors. It contains no policy, scoring,
Dominatus blackboard, model progress, scheduler, graph optimizer, pooling,
topology, or shader computation.

## 14. Generated authority

After M1, `.concept` source and its checked package/ABI inputs are semantic
authority. Generated C/H is a deterministic, checked-in native build input and
must not be hand edited.

Regeneration occurs in a temporary directory, formats output with the repository
chosen pinned formatter, byte-compares all C/H/map/manifest outputs, and fails
on drift. A generated manifest records compiler version, source digests,
package identity/variant/artifact digest, ABI digest, output digests, and escape
count. Existing shader-package identities and generated shader authorities
remain separate inputs; Concept/Vulkan does not regenerate shaders.

Review compares generated code beside the handwritten witness until equivalence
is proven. The native build consumes checked-in C/H without invoking the
compiler. Rollback switches the build back to the retained handwritten file and
reverts the generated/source set; public ABI, packages, and shaders are not
part of that rollback.

## 15. Equivalence and migration gates

- **M1:** compiler vertical slice and capability-probe conformance; parser,
  type/MIR/C determinism; failure/drop goldens; native compile and real-path
  validation.
- **M2:** generate a physical ray-query batch beside the handwritten path and
  compare allocation, descriptor bindings/rebinding, command sequence, access
  synchronization, `ray_count` dispatch, results, diagnostics, failure order,
  repeated lifecycle, and Vulkan validation.
- **M3:** migrate the production ray-query mechanism only after M2 equivalence.
- **M4:** express one Stage 4 SGEMM handoff without changing its committed
  variant, dimensions, bindings, handles, offsets, dispatch, synchronization,
  or execution state.
- **Prometheus Stage 7:** later establish the separate Dominatus model-operation
  authorization/observation seam. Concept/Vulkan never fakes that seam.

## 16. Explicit exclusions

M1/EVT1 excludes Concept machines/transitions, `decide`, `yield`, dynamic
interfaces, vtables, runtime reflection, general heap/containers, general
async, exceptions, package-manager ambitions, DragonGod runtime instances,
runtime dispatch, effects, rollback, `yield`, scheduler integration, full RT
pipelines/SBT, shader mathematics, model topology, and lifecycle/progression
policy. EVT1 M1A exclusions for wildcard arms, guards, or-patterns,
literal/range/recursive patterns, and enum methods remain. EVT1 M1B-A further
excludes multiple concept parameters, concept specialization, compile-time
user functions, general compile-time evaluation, methods, constructors,
destructors, implicit cleanup, and any new move or borrow system. EVT1 M1B-B
still excludes template types, template structs/enums, multiple type
parameters, deduction, specialization, overload ranking, SFINAE, recursive or
nested template instantiation, and runtime generic machinery. PoC3 is design
history; only its local ownership,
explicit move, deterministic drop, error-as-value, unsafe quarantine, C ABI,
bounded comptime, profiles, and MIR-first principles are adopted here.
