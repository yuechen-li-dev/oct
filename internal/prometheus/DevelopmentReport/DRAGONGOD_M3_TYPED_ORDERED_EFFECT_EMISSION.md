# DragonGod M3 — Typed Ordered Effect Emission

Status: **SUCCESS**

Assessment: **DRAGONGOD M3: SUCCESS**

Date: 2026-07-25

Intended commit message:
`concept-vulkan: add typed automata effect emission`

## Starting point

- starting branch: `main`
- starting checkpoint:
  `38daf7a033cd6dac62bf46417e913c033a855819`
- starting worktree: clean
- expected predecessor confirmed:
  `concept-vulkan: add typed automata guards`
- M0 report read:
  `internal/prometheus/DevelopmentReport/DRAGONGOD_M0_TYPED_PUSHDOWN_AUTOMATA_DECLARATION_AND_DETERMINISTIC_STATIC_VALIDATION.md`
- M1 report read:
  `internal/prometheus/DevelopmentReport/DRAGONGOD_M1_TYPED_AUTOMATA_INSTANCES_AND_DETERMINISTIC_RUNTIME_DISPATCH.md`
- M2 report read:
  `internal/prometheus/DevelopmentReport/DRAGONGOD_M2_TYPED_AUTOMATA_CONTEXT_AND_GUARDED_TRANSITIONS.md`
- current reviewer handoff read:
  `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
- current constitution read:
  `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
- current living-status brief read:
  `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`

## Completion summary

DragonGod M3 extends the accepted M2 legality/runtime slice with:

- top-level typed `effect` declarations;
- local exact `effects AutomataName localName;` batch declarations;
- block-bodied transition handlers containing ordered `emit` statements followed
  by exactly one existing DragonGod control action;
- exact three-operand `dispatch(instance, signal, emitted)` for automata whose
  validated effect set is non-empty;
- preserved two-operand `dispatch(instance, signal)` for automata whose
  validated effect set is empty;
- compiler-derived closed effect-tag/payload unions and exact maximum emitted
  batch length per automata family;
- exact typed payload checking against declared effect parameter types;
- pure nonfallible exactly-once payload evaluation during selected dispatch
  only;
- complete-batch staging plus atomic publication of both settled control state
  and ordered effect batch;
- empty-batch publication on unhandled, ambiguous, no-selected-candidate, and
  already-finished paths;
- effect-aware MIR, semantic identity, checked outputs, strict-C11 lowering,
  and native evidence.

## Implemented source surface

### Typed effect declarations

Accepted form:

```concept
effect RecordSubmission(int submission);
effect BeginSubmission(int submission, QueueClass queue);
```

Rules implemented:

- effects are top-level declarations only;
- each effect name is globally unique within the module;
- effect parameter types must be exact accepted payload types;
- accepted payload types in M3 are:
  - `int`
  - `bool`
  - `uint64`
  - nullary enums
  - structs recursively composed from accepted payload types
- strings, pointers, arrays, borrowed/owned/imported/unsafe forms, applied
  types, and Vulkan handle types are rejected.

### Effect batches and effectful dispatch

Accepted forms:

```concept
instance ResourceLifecycle lifecycle(context);
effects ResourceLifecycle emitted;

AutomataDispatchOutcome outcome =
    dispatch(lifecycle, LifecycleSignal::Submit, emitted);
```

Rules implemented:

- `effects AutomataName localName;` declares one compiler-owned fixed local
  batch associated with that exact automata family;
- batches are not ordinary runtime values and cannot be copied, assigned,
  returned, or used as plain expressions;
- effectful automata require the exact third `effects` operand at dispatch;
- effect-free automata reject a third operand;
- the third operand must bind to the same automata family as the instance.

### Ordered transition emission

Accepted form:

```concept
on LifecycleSignal::Submit
    when context.queueAvailable
    => {
        emit RecordSubmission(context.submission);
        emit BeginSubmission(context.submission, context.queue);
        goto Submitting;
    }
```

Rules implemented:

- ordered `emit` statements are accepted only inside block-bodied handlers;
- every effect body must end with exactly one existing DragonGod control action;
- emit order is source order and is preserved exactly in the emitted batch;
- emit payload expressions are validated against the declared parameter list;
- payload expressions must belong to the current pure bounded subset;
- calls, dispatch, templates, and other side-effecting or non-bounded forms are
  rejected in effect payload position;
- effects describe work only; dispatch does not execute Vulkan, invoke an
  actuator, or report physical success.

## Runtime and publication semantics

The governing M3 semantic unit is now:

```text
(control state, signal, immutable context)
    -> (settled next control state, ordered typed effect batch)
```

Generated effectful dispatch performs:

```text
signal match
    -> exact-once guard evaluation for the matched group
    -> unique candidate selection / fallback / unhandled / ambiguous
    -> exactly-once payload evaluation for selected emits
    -> staged control mutation
    -> synchronous completion normalization
    -> atomic publication of settled control state and full batch
```

Observed behavior:

- `AlreadyFinished`: returns immediately and publishes `batch.count = 0`;
- no selected candidate: returns `Unhandled` or `Ambiguous` and publishes
  `batch.count = 0`;
- zero-emit selected transition: publishes the settled control state and an
  empty batch;
- selected effectful transition: publishes the settled control state and the
  full ordered batch only after successful staging;
- emitted payload expressions run exactly once and only for the uniquely
  selected transition;
- batch publication never leaks partially constructed entries.

The generated effect runtime representation remains private and fixed:

- one private tag enum per used effectful automata family;
- one private batch-entry struct with a tag plus closed payload union;
- one private batch struct containing exact fixed `entries[max_batch]` storage
  and `uint8_t count`;
- no heap allocation;
- no reflection registry;
- no function-pointer dispatch table;
- no public Prometheus ABI growth.

## MIR, identity, and lowering

M3 extends typed MIR/source-map evidence with:

- top-level ordered effect declarations;
- effectful transition `emit` operations and payload texts;
- per-automata closed effect set and exact maximum emitted batch length;
- effect-aware dispatch identity including the exact batch operand when present;
- separate deterministic topology, guard, effect, and runtime identities.

Runtime identity now changes when any private runtime-relevant fact changes,
including the derived effect union or maximum batch length, while topology
identity remains stable across pure emission-order changes.

## New specimens

- language specimen:
  `Examples/Concept-Vulkan/evt1_dragongod_m3_language.concept`
- Vulkan-shaped specimen:
  `Examples/Concept-Vulkan/evt1_dragongod_m3_vulkan.concept`

The language specimen proves:

- exact ordered multi-effect emission;
- exact payload projection from immutable context;
- zero-emit selected transition with empty published batch;
- explicit fallback emission;
- ambiguous non-mutation with empty batch;
- already-finished empty batch;
- preserved contextless compatibility path.

The Vulkan-shaped specimen proves the same M3 control-and-emission semantics
against Vulkan-flavored types and naming without introducing production Vulkan
execution.

## Diagnostics added

Focused M3 diagnostics now cover:

- duplicate effect name rejection;
- unsupported effect payload type rejection;
- unknown emitted effect rejection;
- missing batch on effectful dispatch;
- wrong or non-batch third operand;
- third operand on effect-free dispatch;
- ordinary use or assignment of batch bindings;
- effect payload expressions leaving the accepted pure bounded subset.

## Validation run on Saturday, July 25, 2026

### Go/compiler lanes

- `go test ./internal/conceptvulkan -count=1`
- `go test ./cmd/concept-vulkan ./internal/conceptvulkan -count=1`
- focused sweep:
  `go test ./internal/conceptvulkan -run 'CheckedOutputsMatch|DragonGodM0|DragonGodM1|DragonGodM2|DragonGodM3|ParseEVT1SpecimensAndGenerateDeterministically' -count=1`

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
go test ./internal/conceptvulkan -run "DragonGodM0.*NativeC11|DragonGodM1.*NativeC11|DragonGodM2.*NativeC11|DragonGodM3.*NativeC11" -count=1 -v
```

Observed native result:

- `TestDragonGodM0LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM0VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM1LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM1VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM2LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM2VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM3LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM3VulkanSpecimenNativeC11` — **PASS**

### Checked outputs and hygiene

- regenerated checked outputs for:
  - `evt1_dragongod_m0_language`
  - `evt1_dragongod_m0_vulkan`
  - `evt1_dragongod_m1_language`
  - `evt1_dragongod_m1_vulkan`
  - `evt1_dragongod_m2_language`
  - `evt1_dragongod_m2_vulkan`
  - `evt1_dragongod_m3_language`
  - `evt1_dragongod_m3_vulkan`
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
  - `Examples/Concept-Vulkan/evt1_dragongod_m3_language.concept`
  - `Examples/Concept-Vulkan/evt1_dragongod_m3_vulkan.concept`
  - corresponding generated outputs under `internal/conceptvulkan/generated/`
- docs / handoff:
  - `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
  - `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
  - `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
  - `internal/prometheus/DevelopmentReport/DRAGONGOD_M3_TYPED_ORDERED_EFFECT_EMISSION.md`

## Scope preserved

- no effect execution, actuator registration, callback routing, physical
  success/failure semantics, implicit completion signals, rollback,
  compensation, cleanup execution, state entry/exit effects, signal payloads,
  scoring, `decide`, priorities, telemetry policy, `yield`, suspension,
  Dominatus policy, dynamic automata, heap allocation, or production Vulkan
  route migration were added;
- M2 guard semantics, deterministic ambiguity handling, and non-mutation on
  non-selection remain in force;
- public Prometheus ABI, shader packages, manifests, locks, kernels, and paused
  Prometheus work remain unchanged.

## Known limitations

- effect payload types remain deliberately small and closed in M3;
- effect payload expressions use the current bounded pure-expression validator
  rather than a general effect system;
- only block-bodied handlers may emit in this pass;
- effect batches are compiler-owned local storage only and are not yet a public
  reusable user type.

## Exact next milestone

Recommended next assignment:

```text
DragonGod M4 — Typed Signal Payload Feedback
```

The smallest coherent follow-on appears to be:

- typed signal payloads for success/failure feedback after external actuation;
- exact handler binding and dispatch typing for payload-bearing signals;
- preserved separation between emitted work descriptions and physical execution;
- no rollback, implicit completion, or policy machinery unless later evidence
  forces it.
