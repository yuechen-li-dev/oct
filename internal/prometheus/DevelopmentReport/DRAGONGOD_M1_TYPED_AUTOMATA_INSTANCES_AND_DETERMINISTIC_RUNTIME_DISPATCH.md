# DragonGod M1 — Typed Automata Instances and Deterministic Runtime Dispatch

Status: **SUCCESS**

Intended commit message:
`concept-vulkan: add typed automata runtime dispatch`

## Completion assessment

Assessment: **DRAGONGOD M1: SUCCESS**

DragonGod M1 closes the first runtime vertical for Concept/Vulkan automata:

- fixed local `instance AutomataName localName;` declarations;
- exact typed `dispatch(instance, signal)` expressions returning
  `AutomataDispatchOutcome`;
- compiler-owned `AutomataDispatchOutcome::{Transitioned, Unhandled, Finished,
  AlreadyFinished}`;
- fixed private runtime storage with current machine, current state, finished
  flag, continuation count, and fixed continuation storage derived from the M0
  maximum active machine depth;
- deterministic runtime `goto`, `push`, `pop`, and `finish`;
- synchronous terminal normalization after initialization and after each
  handled dispatch;
- immediate normalization for pushed initial terminal states and resumed
  terminal continuations;
- strict-C11 private lowering with no heap allocation, no string lookup, no
  function-pointer table, no VM, and no public Prometheus ABI growth.

DragonGod M0 is also reconciled to **SUCCESS** in this repository state. The
original blocker was environmental, not semantic: on Friday, July 24, 2026,
both M0 native C11 specimen lanes were rerun from a fully loaded Visual Studio
developer environment and both passed.

## Starting point

- starting branch: `main`
- starting checkpoint: `5dbb4d5268b85458cc10a68a1b83ac1dba212554`
- starting worktree: clean
- expected predecessor confirmed: `concept-vulkan: add typed lifecycle automata declarations`
- repository authority read before edits:
  - `README.md`
  - `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
  - `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
  - `internal/prometheus/DevelopmentReport/DRAGONGOD_M0_TYPED_PUSHDOWN_AUTOMATA_DECLARATION_AND_DETERMINISTIC_STATIC_VALIDATION.md`
  - `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
  - relevant `Language/reference` slices for expressions, enums, and variables

## Implemented language surface

### Instance declaration

Exact local form:

```concept
instance ResourceLifecycle lifecycle;
```

Rules implemented:

- the first identifier must bind to a validated `automata` declaration;
- the second identifier becomes a local automatic-storage instance binding;
- initialization is compiler-generated;
- no initializer, constructor, factory, heap allocation, or user-sized stack
  exists;
- instance bindings are not ordinary runtime values and cannot be copied,
  assigned, returned, passed to ordinary functions, or used as plain
  expressions.

### Dispatch expression

Exact dedicated form:

```concept
dispatch(lifecycle, LifecycleSignal::Create)
```

Rules implemented:

- the first operand must name a local `instance`;
- the second operand must have the automata family's exact signal enum type;
- signal evaluation happens exactly once;
- the result type is the compiler-owned enum `AutomataDispatchOutcome`;
- dispatch is unavailable in comptime code;
- ordinary functions or templates named `dispatch` are rejected.

## Runtime representation and semantics

Per used automata family, the generator emits private strict-C11 runtime
support inside the `.generated.c` translation unit only:

- private machine ordinals in source order;
- private per-machine state ordinals in source order;
- one private instance struct containing:
  - `bool finished`
  - `uint8_t current_machine`
  - `uint8_t current_state`
  - `uint8_t continuation_count`
  - `continuations[maximum_active_depth - 1]` when capacity is nonzero;
- one private continuation struct containing:
  - caller machine ordinal
  - caller resume-state ordinal

Implementation choices:

- machine/state identities use `uint8_t`, which is sufficient for the existing
  M0 bounds (`<= 16` machines, `<= 32` states per machine, `<= 8` active depth);
- continuation capacity is exactly `maximum_active_depth - 1`;
- zero-capacity automata emit no continuation array at all and still lower as
  valid strict C11;
- private invariant traps remain for impossible invalid state, stack underflow,
  stack overflow, and completion-bound violations.

### Initialization

`instance` initialization:

1. sets `finished = false`;
2. sets the current machine to the declared root machine;
3. sets the current state to that machine's declared initial state;
4. clears the continuation count;
5. runs the same normalization logic used after handled dispatch.

This means an initial terminal `finish` state becomes immediately finished, and
its first later dispatch returns `AlreadyFinished`.

### Handler selection and outcomes

For active nonterminal states:

- missing handler: return `Unhandled` with no mutation;
- `goto`: update only the current state, then normalize;
- `push`: append caller machine plus continuation state, enter the pushed
  machine's declared initial state, then normalize;
- terminal `pop`: restore the retained continuation and continue normalization;
- terminal `finish`: mark the full instance finished, clear continuation state,
  and return `Finished`.

Outcome meanings in the implemented lowering:

- `Transitioned`: one handler matched and post-normalization quiescence is an
  active nonterminal state;
- `Unhandled`: no handler matched and the instance representation is unchanged;
- `Finished`: one handler matched and normalization reached `finish`;
- `AlreadyFinished`: the instance was already terminated before dispatch.

The completion-step bound used by the runtime normalizer is exactly
`maximum_active_depth`: each normalization step is either one `finish` or one
`pop`, and each `pop` strictly reduces active machine depth.

## Typed MIR and deterministic evidence

MIR/source-map automata entries now include:

- root machine;
- maximum active depth;
- continuation capacity;
- completion-step bound;
- deterministic graph identity;
- deterministic private machine ordinals;
- deterministic private state ordinals;
- statement/operation evidence for `instance_decl` and `dispatch`.

Declaration-only automata that are never used through `instance` still erase
fully from generated runtime C/H. Runtime helpers appear only for automata
families actually used through local instances.

## New specimens

- `examples/Concept-Vulkan/evt1_dragongod_m1_language.concept`
- `examples/Concept-Vulkan/evt1_dragongod_m1_vulkan.concept`

The language specimen proves:

- initial-terminal normalization to `AlreadyFinished`;
- zero-continuation-capacity lowering to `Finished`;
- unhandled non-mutation;
- self-`goto` handling;
- nested `push`/`pop` resumption;
- resumed-terminal continuation normalization to `Finished`;
- non-root `finish` terminating the whole instance;
- independent local instances.

The Vulkan-shaped specimen proves the same runtime control behavior against
Vulkan-flavored resource lifecycle names and real Vulkan headers without adding
any production Vulkan execution path.

## Diagnostics added

Focused M1 diagnostics now cover:

- compiler-owned `AutomataDispatchOutcome` redeclaration rejection;
- compiler-owned `dispatch` redeclaration rejection;
- payload-bearing signal-enum rejection for automata runtime use;
- `instance` on a non-automata symbol;
- ordinary value use of an instance binding;
- `dispatch` first operand not being an instance;
- wrong signal enum at dispatch.

## Validation run

### Go/compiler lanes

- `go test ./internal/conceptvulkan -count=1`
- `go test ./cmd/concept-vulkan ./internal/conceptvulkan -count=1`
- `go build ./cmd/concept-vulkan`
- `go test ./internal/conceptvulkan -run 'DragonGod|ParseEVT1Specimens|CheckedOutputsMatch' -count=1`

Result: **PASS**

### Native C11 lanes

Visual Studio developer environment loaded with:

- `C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat`

Native environment evidence recorded on Friday, July 24, 2026:

- MSVC compiler: `19.51.36248.0`
- VC tools: `14.51.36231`
- Windows SDK: `10.0.26100.0`
- Vulkan SDK: `C:\VulkanSDK\1.4.350.0`

Rerun commands:

- `cmd /c 'call "C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat" >nul && go test ./internal/conceptvulkan -run "DragonGodM0.*NativeC11" -count=1 -v'`
- `cmd /c 'call "C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat" >nul && go test ./internal/conceptvulkan -run "DragonGodM0.*NativeC11|DragonGodM1.*NativeC11" -count=1 -v'`

Observed result:

- `TestDragonGodM0LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM0VulkanSpecimenNativeC11` — **PASS**
- `TestDragonGodM1LanguageSpecimenNativeC11` — **PASS**
- `TestDragonGodM1VulkanSpecimenNativeC11` — **PASS**

### Generated-output and hygiene lanes

- regenerated checked outputs for M0 and M1 DragonGod specimens with
  `go run ./cmd/concept-vulkan -mode evt1-m1b-d -source ... -out internal/conceptvulkan/generated generate`
- `git diff --check`

Result:

- checked outputs: **PASS**
- `git diff --check`: **PASS** (line-ending warnings only)

## Scope preserved

- no state payloads, transition effects, entry/exit effects, cleanup effects,
  rollback, compensation, yield, scheduler integration, Dominatus policy, heap
  allocation, dynamic automata, reflection, or public framework API were added;
- no production Vulkan runtime path, shader package, manifest, lock, kernel, or
  public Prometheus ABI was widened;
- paused RQ-M1 and DVT-2 work remains paused.

## Known limitations

- machine/state/continuation identities remain private implementation details
  and are not surfaced as first-class Concept values;
- M1 still has no state-local data, automata context, signal payload runtime,
  or ordered transition effects;
- enum equality lowering for ordinary runtime enum values remains outside the
  scope of this milestone; the M1 specimens use `match`-based outcome encoding
  rather than widening the runtime-equality surface here.

## Exact next milestone

Recommended next assignment:

```text
DragonGod M2 — Typed Automata Context and Ordered Transition Effects
```

The smallest coherent next step is:

- one explicit automata-wide context type;
- ordered `exit -> transition -> entry` effect execution;
- deterministic state-commit boundary after successful effect completion;
- no rollback, compensation, scheduler, dynamic automata, or Dominatus policy
  expansion unless directly forced by the first bounded effectful specimen.
