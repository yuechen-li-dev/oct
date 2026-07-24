# DragonGod M0 — Typed Pushdown-Automata Declaration and Deterministic Static Validation

Status: **MEANINGFUL PROGRESSION**

Intended commit message if/when the full milestone closes cleanly:
`concept-vulkan: add typed lifecycle automata declarations`

## Completion assessment

Assessment: **DRAGONGOD M0: MEANINGFUL PROGRESSION**

This pass establishes the first working DragonGod M0 compiler vertical inside
the EVT1 Concept/Vulkan path:

- exact `automata -> machine -> state` parsing;
- exact typed signal-enum association per automata family;
- exact typed `goto`, `push`, `pop`, and `finish` declarations;
- deterministic validation for initial/root rules, duplicate transition keys,
  machine-push acyclicity, machine/state reachability, reachable `pop`, and
  reachable root `finish`;
- deterministic maximum active machine depth derivation;
- stable graph identity in typed MIR/source-map output;
- complete runtime erasure of automata declarations from generated C/H;
- hardware-independent and Vulkan-shaped DragonGod specimens in the checked
  EVT1 corpus.

The remaining blocker to honest milestone closure is evidence closure rather
than a newly isolated parser/type bug: the full requested native/doc/report
matrix was not completed in this shell, and the MSVC developer include
environment was unavailable for the new native specimen lane.

## Starting point

- starting branch: `main`
- starting checkpoint: `cc49838a49abf866d5f2be54abee4c3ca25a7692`
- starting worktree: clean
- M1B-D authority preflight: confirmed in
  `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_EVT1_M1B_D_FIXED_COMPILE_TIME_ARRAYS_AND_FINITE_STRUCTURAL_VALIDATION.md`
- living-status reconciliation: DragonGod remained the intended first serious
  post-substrate direction in `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
  and `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`

## Implemented surface

- top-level `automata Name(SignalEnumType) { ... }` declarations;
- nested `initial machine Name { ... }` / `machine Name { ... }`;
- nested `initial state`, `terminal state`, and `initial terminal state`;
- state-local `on Enum::Member goto State;`;
- state-local `on Enum::Member push Machine goto ContinuationState;`;
- terminal `pop;` and `finish;` completions;
- rejection of ordinary statements inside state bodies;
- exact signal-member validation against the automata's declared enum;
- rejection of payload-bearing signal members in handlers;
- rejection of duplicate `(machine, state, signal)` handler keys;
- rejection of direct and indirect machine-push cycles;
- rejection of unreachable machines and unreachable states;
- rejection of pushed machines with no reachable `pop`;
- rejection of root machines with no reachable `finish`;
- rejection of root-machine `pop`;
- rejection of automata names used as runtime expressions or types.

## Deterministic evidence

- MIR now contains one `automata` section with machine/state/transition order,
  reachability, completion kind, maximum active machine depth, and graph
  identity;
- source-map output now includes the same automata MIR section;
- graph identity is a digest of a canonical source-order declaration shape and
  therefore ignores comment/whitespace/location-only changes;
- generated C/H remains ignorant of automata declarations, proving complete
  erasure in the current lowering path.

## New specimens

- `examples/Concept-Vulkan/evt1_dragongod_m0_language.concept`
- `examples/Concept-Vulkan/evt1_dragongod_m0_vulkan.concept`

The language specimen proves:

- two pushed machines;
- repeated local state names across different machines;
- explicit `goto`;
- explicit self-`goto`;
- explicit `push ... goto continuation`;
- nested push depth greater than one;
- reachable `pop` in every pushed machine;
- reachable root `finish`;
- non-root `finish`;
- unrelated runtime functions that still lower cleanly.

The Vulkan-shaped specimen proves the same declaration model against Vulkan-ish
resource lifecycle names without widening the runtime mechanism surface.

## Validation actually run

### Compiler/test lanes

- `go test ./internal/conceptvulkan -count=1`
- `go test ./cmd/concept-vulkan ./internal/conceptvulkan -count=1`
- `go build ./cmd/concept-vulkan`
- `go test ./internal/conceptvulkan -run 'DragonGodM0.*NativeC11' -count=1 -v`

### Generated-output lanes

- regenerated EVT1 checked outputs for existing specimens plus the two new
  DragonGod specimens with `go run ./cmd/concept-vulkan -mode evt1-m1b-d ... generate`
- `git diff --check`

## Validation result

- focused EVT1 Go compiler/test lanes: **PASS**
- checked-output regeneration: **PASS**
- `git diff --check`: **PASS** (line-ending warnings only)
- DragonGod native specimen lane: **SKIPPED** in this shell because the MSVC
  developer include environment was unavailable

## Scope preserved

- no runtime dispatcher, continuation stack, effects engine, rollback path,
  scheduler integration, or Dominatus bridge was added;
- no production Prometheus route, shader source, package metadata, public ABI,
  or paused RQ-M1 / DVT-2 work was intentionally widened;
- DragonGod remains declaration-and-validation only in this pass.

## Files added or changed in this pass

- parser / validation / MIR / tests:
  `internal/conceptvulkan/evt1_automata.go`,
  `internal/conceptvulkan/evt1_parse.go`,
  `internal/conceptvulkan/evt1_validate.go`,
  `internal/conceptvulkan/evt1_types.go`,
  `internal/conceptvulkan/evt1_generate.go`,
  `internal/conceptvulkan/evt1_test.go`
- specimens / checked outputs:
  `examples/Concept-Vulkan/evt1_dragongod_m0_language.concept`,
  `examples/Concept-Vulkan/evt1_dragongod_m0_vulkan.concept`,
  corresponding checked outputs under `internal/conceptvulkan/generated/`
- docs / handoff:
  `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`,
  `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`,
  `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`

## Known remaining blocker

The current blocker is evidence closure:

- the requested Windows-native DragonGod lane did not run to completion in this
  shell because the MSVC developer include environment was unavailable;
- the broader milestone-sized documentation and regression matrix from the
  assignment remains larger than the focused compiler lanes executed here.

That leaves the codebase in a materially better state than the starting point:
the language feature exists end-to-end in parser/validator/MIR/erasure form,
the key graph rules are enforced deterministically, the checked specimens are
added, and the next missing proof is isolated to validation closure rather than
to an unresolved compiler-design ambiguity.
