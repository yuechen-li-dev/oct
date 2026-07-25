# DragonGod M2 — Typed Automata Context and Guarded Transitions

Status: **SUCCESS**

Assessment: **DRAGONGOD M2: SUCCESS**

Date: 2026-07-25

## Starting point

- starting branch: `main`
- starting checkpoint: `66dd5f6b30e36ca6bb08fdb9e7fb48423b7a3f70`
- starting worktree: clean
- expected predecessor confirmed: `concept-vulkan: add typed automata runtime dispatch`
- M0 report read:
  `internal/prometheus/DevelopmentReport/DRAGONGOD_M0_TYPED_PUSHDOWN_AUTOMATA_DECLARATION_AND_DETERMINISTIC_STATIC_VALIDATION.md`
- M1 report read:
  `internal/prometheus/DevelopmentReport/DRAGONGOD_M1_TYPED_AUTOMATA_INSTANCES_AND_DETERMINISTIC_RUNTIME_DISPATCH.md`
- current reviewer handoff read:
  `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
- current constitution read:
  `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
- current living-status brief read:
  `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`

## Completion summary

DragonGod M2 extends the accepted M1 runtime slice with:

- one optional exact automata-wide borrowed context binding;
- exact `instance AutomataName localName(contextExpr);` binding for contextful
  automata;
- guarded handlers `on Signal::Value when BoolExpression => ControlAction;`;
- explicit fallback handlers `on Signal::Value otherwise => ControlAction;`;
- exact `bool` guard typing;
- compiler-verified pure and bounded guard helper calls;
- deterministic grouped candidate resolution per `(automata, machine, state,
  signal)`;
- exact `Unhandled` on zero true guards with no fallback;
- exact `Ambiguous` on more than one true guard;
- preserved M1 `goto`/`push`/`pop`/`finish` semantics only after unique
  selection;
- preserved no-mutation behavior for guard-caused `Unhandled` and `Ambiguous`;
- guard-aware MIR, graph identity, checked outputs, and strict-C11 lowering.

## Implemented source surface

### Contextful automata declaration

Accepted form:

```concept
automata GuardedLifecycle(
    LifecycleSignal,
    borrow context: LifecycleContext)
{
    ...
}
```

Rules implemented:

- the first automata parameter remains the exact signal enum type;
- the optional second parameter must use `borrow name: Type`;
- exactly zero or one context parameter is accepted;
- context type must be one exact plain runtime value type;
- pointer, owned, borrowed, imported, unsafe, array, and applied context types
  are rejected in M2;
- contextless M1 automata remain valid unchanged.

### Context-bound instance declaration

Accepted form:

```concept
instance GuardedLifecycle lifecycle(context);
```

Rules implemented:

- contextful automata require exactly one context argument;
- contextless automata reject a context argument;
- the context expression must be an assignable access path;
- initialization evaluates and binds the context exactly once at instance
  declaration time;
- the retained borrow is immutable for the lexical lifetime of the instance;
- overlapping later assignments are rejected.

### Guarded and fallback handlers

Accepted forms:

```concept
on LifecycleSignal::Submit when CanSubmit(context) => goto Submitted;
on LifecycleSignal::Submit otherwise => goto Deferred;
```

Rules implemented:

- an ordinary M1 unguarded handler remains valid;
- a candidate group is either one ordinary unguarded handler, or guarded
  candidates plus one optional final `otherwise`;
- `otherwise` cannot appear before a later guard;
- `otherwise` cannot stand alone;
- ordinary unguarded handlers cannot mix with guarded or fallback candidates for
  the same group;
- duplicate ordinary unguarded handlers keep the historical `CV4253` rejection.

## Guard typing and purity

Guard expressions are validated in a dedicated context scope:

- exact type must be `bool`;
- direct names may read the declared borrowed context binding;
- field access, literals, unary/binary Boolean/int/string expressions, `if`
  expressions, `match` expressions, struct/enum construction, array literals,
  and `Len(...)` remain admitted when they already lower as ordinary EVT1
  expressions;
- ordinary runtime function calls are admitted only when the called function is
  locally defined, non-recursive, bounded, expression-oriented, and free of
  mutable borrows, owned parameters, pointers, assignments, loops, instance
  declarations, statement-form `match`, `dispatch`, or template calls.

Explicit guard bounds implemented:

- maximum guard expression nodes: `128`
- maximum transitive guard-call depth: `8`
- maximum guard-call graph nodes: `16`
- maximum guard-call graph edges: `32`
- maximum candidates per state/signal group: `16`

## Runtime selection semantics

Generated dispatch now performs:

```text
signal match
    -> exact-once guard evaluation for that matched group
    -> unique selection / fallback / unhandled / ambiguous
    -> M1 control mutation
    -> M1 completion normalization
```

Observed behavior:

- `AlreadyFinished`: returned before any guard evaluation;
- missing signal group: returned as `Unhandled` with no guard evaluation;
- exactly one true guard: selected action runs;
- zero true guards plus fallback: fallback action runs;
- zero true guards plus no fallback: `Unhandled`;
- more than one true guard: `Ambiguous`;
- `Unhandled` and `Ambiguous` leave machine/state/continuation/finished/context
  storage unchanged;
- source order is used only for exact-once evaluation order, not as winner
  precedence.

`AutomataDispatchOutcome` now contains:

- `Transitioned`
- `Unhandled`
- `Ambiguous`
- `Finished`
- `AlreadyFinished`

## MIR, identity, and lowering

M2 extends automata MIR/source-map evidence with:

- optional `context_name` and exact `context_type`;
- per-handler `guard` text when present;
- per-handler `otherwise` flag when present.

Graph identity now includes:

- optional context binding name and exact context type;
- per-handler guard identity text;
- per-handler fallback presence.

Generated runtime representation remains private and fixed:

- no heap allocation;
- no reflection registry;
- no string dispatch;
- no function-pointer dispatch table;
- no public Prometheus ABI growth.

Contextful automata instances now carry one private retained immutable context
pointer in their generated instance struct and pass it to guard lowering only
through private specialized code.

## New specimens

- language specimen:
  `examples/Concept-Vulkan/evt1_dragongod_m2_language.concept`
- Vulkan-shaped specimen:
  `examples/Concept-Vulkan/evt1_dragongod_m2_vulkan.concept`

The language specimen proves:

- unique guarded selection;
- no-true fallback selection;
- no-true/no-fallback `Unhandled` without mutation;
- multi-true `Ambiguous` without mutation;
- preserved `AlreadyFinished`;
- preserved contextless M1 instance compatibility.

The Vulkan-shaped specimen proves the same legality/control semantics against
Vulkan-flavored types and names without introducing production Vulkan effects.

## Validation run on Saturday, July 25, 2026

### Go/compiler lanes

- `go test ./internal/conceptvulkan -count=1`
- `go test ./cmd/concept-vulkan ./internal/conceptvulkan -count=1`
- `go build ./cmd/concept-vulkan`
- focused sweep:
  `go test ./internal/conceptvulkan -run 'ParseEVT1SpecimensAndGenerateDeterministically|DragonGodM2|DragonGodM1GenerationIncludesRuntimeDispatch|DragonGodM0DiagnosticsAreStable' -count=1`

Result: **PASS**

### Native C11 lanes

Developer environment loaded through:

- `C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat`

Observed environment on Saturday, July 25, 2026:

- MSVC compiler: `19.51.36248`
- VC tools: `14.51.36231`
- Windows SDK: `10.0.26100.0`
- `VulkanSDK` environment variable: **unset in the rerun shell**
- locally installed Vulkan SDK directories observed:
  - `C:\VulkanSDK\1.4.341.1`
  - `C:\VulkanSDK\1.4.350.0`

Native rerun command:

```text
go test ./internal/conceptvulkan -run "DragonGodM0.*NativeC11|DragonGodM1.*NativeC11|DragonGodM2.*NativeC11" -count=1 -v
```

Observed native result:

- `TestDragonGodM0LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM0VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM1LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM1VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM2LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM2VulkanSpecimenNativeC11` — **PASS**

### Checked outputs and hygiene

- regenerated checked outputs for:
  - `evt1_dragongod_m1_language`
  - `evt1_dragongod_m1_vulkan`
  - `evt1_dragongod_m2_language`
  - `evt1_dragongod_m2_vulkan`
- `go test ./internal/conceptvulkan -count=1` revalidated checked outputs
- `git diff --check` run at the end of the pass

## Files changed in this pass

- parser / validation / MIR / lowering / tests:
  - `internal/conceptvulkan/evt1_automata.go`
  - `internal/conceptvulkan/evt1_generate.go`
  - `internal/conceptvulkan/evt1_parse.go`
  - `internal/conceptvulkan/evt1_test.go`
  - `internal/conceptvulkan/evt1_types.go`
  - `internal/conceptvulkan/evt1_validate.go`
- specimens and checked outputs:
  - `examples/Concept-Vulkan/evt1_dragongod_m1_language.concept`
  - `examples/Concept-Vulkan/evt1_dragongod_m1_vulkan.concept`
  - `examples/Concept-Vulkan/evt1_dragongod_m2_language.concept`
  - `examples/Concept-Vulkan/evt1_dragongod_m2_vulkan.concept`
  - corresponding generated outputs under `internal/conceptvulkan/generated/`
- docs / handoff:
  - `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
  - `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
  - `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`

## Scope preserved

- no transition effects, entry effects, exit effects, cleanup execution,
  rollback, compensation, scoring, `decide`, priority, source-order winner
  selection, `yield`, scheduler integration, Dominatus policy, heap allocation,
  reflection, or production Vulkan route migration were added;
- M0 and M1 topology, identity, reachability, maximum-depth, continuation, and
  normalization behavior remain in force after unique candidate selection;
- public Prometheus ABI, shader packages, manifests, locks, kernels, and paused
  work remain unchanged.

## Known limitations

- guard purity is a bounded compiler proof for the current EVT1 statement and
  expression subset, not a general effect system;
- guard helper functions must be local and definition-visible in this pass;
- retained-context immutability is enforced through overlapping assignment
  rejection in the current lexical model rather than a broader lifetime engine.

## Exact next milestone

Recommended next assignment:

```text
DragonGod M3 — Typed Ordered Transition Effects
```

The smallest coherent follow-on remains:

- explicit transition-attached effect calls;
- deliberate mutable access to the typed automata context;
- ordered effect execution after M2 legality selection;
- one exact control-commit boundary;
- fallible effect results with no rollback or compensation unless later forced
  by evidence.
