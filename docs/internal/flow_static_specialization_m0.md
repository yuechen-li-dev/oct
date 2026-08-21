# Flow static specialization M0

## 1. Verdict

**Success**

The existing compiled-flow path now specializes small scalar controllers from
the same flow MIR. The persistent-policy specimen emits typed policy state,
performs zero allocations per decision after construction, and omits unused
history, resume, range, clone, reflection, and generic policy-map machinery.

## 2. Motivating pathology

External dogfood used a small scheduling controller with bounded candidates and
`when policy`. Persistent policy semantics were valid: hysteresis and minimum
commit behaved correctly when one flow instance survived across observations.
Per-decision reconstruction was semantically inappropriate because it discarded
that controller memory. The compiled representation also imposed large generic
runtime costs on a tiny controller.

This milestone contains no database-specific compiler logic and has no source or
build dependency on the external dogfood repository.

## 3. Current compiler architecture

The pre-optimization architecture was already closer to the desired direction
than a separate generic runtime universe:

1. parsing and type checking produce the ordinary Oct AST;
2. `lowerProgram` invokes `lowerFlow` for each flow;
3. `lowerFlow` creates `MIRFlow`, `MIRFlowState`, statement, and expression nodes;
4. `emitGoWithOptions` emits module support plus `emitGoFlow` output;
5. `emitGoFlow` creates a Go struct and an `__octStep` method containing nested
   state/instruction switches.

The generic tax was principally in unconditional fields and source helpers, not
in a second runtime interpreter. Flow policy expressions lowered to a site ID,
then used `map[int]__octUtilitySiteState`, `any`, `reflect.DeepEqual`, candidate
slices, and a second allocated slice of eligible candidates.

## 4. Feature-cost inventory

| Feature | Pre-optimization representation | Did every small flow pay? | M0 treatment | Safety fact |
|---|---|---:|---|---|
| State history | `history []string`, entry/transition appends, copying accessor | Yes | Storage and appends emitted only when reachable code uses `StateHistory` | With no reachable observer, stored entries cannot affect Oct behavior |
| Resume/remember | Boolean slot, integer target, state-name helper | Yes | Fields/helper emitted only when the flow contains `remember` or `resume` | A flow with neither statement can only report the documented empty target |
| Controller policy | `map[int]` of `any` state plus reflection | Every policy flow | Comparable scalar sites become typed struct fields | Bool, String, Int, Float, and dimensioned numeric Go representations are directly comparable; numeric equality matches the prior reflection comparison, including NaN inequality |
| Policy candidates | Candidate slice plus allocated filtered-valid slice | Every policy decision | Scalar helper scans the fixed candidate slice directly | Source order and strict-greater replacement preserve deterministic tie-breaking |
| Board state | Nested fixed Go struct | Only flows with a board | Retained | Board mutation is observable flow state |
| Board snapshot | Typed record returned through the flow interface's `any` boundary | Only flows with a board/API use | Retained | Snapshot typing and value semantics are language-visible |
| Array/value clone | Reflection-based `__octClone` and row helper | Emitted in every module | Emitted only when MIR contains a clone-producing operation or builtin | Lowering already marks array/value-copy sites; array corpus verifies copy behavior |
| Range support | `__octRange` type | Emitted in every module | Emitted only for a Range type/value/consumer | No generated reference exists otherwise |
| Array coercion | Int-to-Float array helper | Emitted in every module | Emitted only when MIR references the coercion | The helper is purely an implementation dependency of those expressions |
| Flow result/fallibility | Result fields, flow-result interface, fallible `Result` wrapper | Every flow | Retained | Completion and result-before-completion are observable semantics |
| Record returns | Direct Go record structs | Only used record shapes | Retained | This is already a static representation |
| Debug/status API | Interface methods for active/complete/history/resume/snapshot | Every flow result type | Compatibility methods retained; unused history/resume methods become constant stubs | Preserves the current internal interface without ABI redesign |

## 5. Optimization

`analyzeFlowFeatures` walks flow MIR and records history, resume, generic-policy,
and scalar-policy requirements. `analyzeGoSupportFeatures` inspects module MIR
and builtin use for clone, row assignment, range, and array coercion support.
The existing emitter consumes those results; no alternate flow lowering or
emitter was introduced.

Controller-bound policy sites with comparable scalar result types now emit one
field per site:

```go
utilitySite0 __octScalarUtilitySiteState[int]
```

and select through a typed pointer. The site contains only current-selection,
score, and commitment-age state. It has no map, interface conversion, or
reflection. General result types and standalone utility expressions retain the
existing generic path.

## 6. Semantic justification

- History removal is module-observation-driven. If `StateHistory` is reachable,
  history remains enabled for all flows because flow interfaces are grouped by
  return type and may not reveal a more precise target statically.
- Resume storage is syntax-driven per flow. `remember` or `resume` retains the
  full slot and transition behavior. Without either, `ResumeTarget` is always
  the documented empty string.
- Direct policy equality is restricted to Go-comparable scalar Oct types. Other
  shapes retain reflection and `any` state.
- The scalar selector visits candidates in source order, replaces a winner only
  for a strictly greater score, discards an ineligible current choice, and uses
  the same commitment-age and hysteresis comparisons as the general selector.
- Clone, row, range, and coercion helpers are removed only when MIR and builtin
  analysis prove that emitted code cannot reference them. The compiled array
  value-copy corpus remains the regression authority.

## 7. Persistent policy result

`persistent_policy_controller.octest` constructs one controller and calls
`Step(controller)` repeatedly. Its board snapshot sequence is:

| Step | Challenger | Result | Reason |
|---:|---|---:|---|
| 1 | ineligible | 1 | initial selection |
| 2 | score 65 vs 60 | 1 | gain is within hysteresis 8 |
| 3 | score 90 vs 60 | 1 | `min_commit: 3` is still active |
| 4 | score 90 vs 60 | 2 | commitment expired and threshold is exceeded |

The fixture passed both interpreted and required-compiled execution. A new flow
per step would produce `1, 1, 1, 1`, so the fourth assertion detects accidental
reconstruction.

## 8. Generated-Go comparison

Baseline is commit `94c4832722d9174a38cc168aad0bcfe4a19ba041`, measured before
changing lowering. Both measurements use the same canonical specimen and
`package flowbench` emission without a generated `main`.

| Metric | Baseline | Optimized | Change |
|---|---:|---:|---:|
| Source bytes | 6,227 | 4,070 | -34.6% |
| Source lines | 240 | 165 | -31.3% |
| Imports | 2 (`fmt`, `reflect`) | 0 | -2 |
| `__oct*` helper/method declarations | 13 | 8 | -38.5% |
| Flow struct fields | 11 | 8 | -27.3% |
| Reflection | yes | no | removed |
| Generic clone helper | yes | no | removed |
| Range type | yes | no | removed |
| History storage/appends | yes | no | removed |
| Resume fields/state-name helper | yes | no | removed |
| Generic utility map | yes | no | replaced by one typed site field |

History and resume compatibility methods remain as constant stubs because the
current generated flow-result interface requires them. Their backing state and
hot-path work are absent.

## 9. Runtime comparison

Windows/amd64, AMD Ryzen 7 7700X, Go benchmark tooling, five samples. Controller
construction and three warm-up decisions occur before the timer.

| Implementation | ns/op range | bytes/decision | allocations/decision |
|---|---:|---:|---:|
| Baseline compiled flow | 89.70-111.9 | 167-175 | 1 |
| Optimized compiled flow | 8.921-9.177 | 0 | 0 |
| Handwritten Go reference (same optimized run) | 1.880-1.895 | 0 | 0 |

In a representative baseline sample, 10,843,310 decisions therefore incurred
approximately 10,843,310 allocation events; optimized samples incurred zero
allocation events even at more than 132 million decisions. Total allocated
bytes are intentionally not compared across samples with different iteration
counts. The stable conclusion is the allocation result and approximately one
order-of-magnitude compiled-flow decision-cost reduction. Sub-nanosecond sample
variation is timer noise and is not interpreted.

## 10. Compatibility

Green verification:

- targeted `internal/build` flow core, utility, history, resume, structure,
  deterministic generation, independent generated-Go compile, and benchmark
  tests;
- full `go test -tags integration ./internal/build`;
- compiled persistent-policy specimen and interpreted parity;
- 5 compiled observability/history facts;
- 3 compiled ordered-`when` facts;
- 1 compiled scalar-board fact;
- 5 compiled board-indexed-assignment facts;
- 3 compiled flow-index-expression facts;
- 30 compiled array/range/value-copy facts;
- `go test ./...` outside unrelated Concept-Vulkan checked-output drift.

Baseline-verified or unrelated repository issues:

- two compiled resume corpus assertions fail identically on baseline commit
  `94c48327`; focused compiler integration resume tests pass;
- six utility-policy corpus fixtures use flow-local `var`, an already unsupported
  compiled-flow statement, while the new persistent specimen uses the supported
  board shape and passes compiled mode;
- some legacy multi-file flow-record and Core-A fixture directories do not load
  independently because of existing package/test-runner layout issues;
- full `go test ./...` reports stale Concept-Vulkan checked artifacts; the
  touched compiler packages pass;
- the stale Core-A top-level diagnostic expectation was updated to include the
  already-supported `concept` declaration and its invalid lane now passes 7/7.

No interpreted fallback was used to hide a compiled failure.

## 11. Remaining tax

The optimized flow still has generic lifecycle fields (`started`, `completed`,
`currentState`, `instruction`, `result`, and `hasResult`), compatibility methods,
a nested instruction switch, and a generic candidate slice literal. Escape
analysis keeps that slice on the stack, so it adds no allocations. The remaining
decision cost is about 4.8 times the handwritten controller in the same run, but
the absolute measured tax is about 7 ns/decision and zero bytes.

Further reduction would require either proving a narrower flow API surface per
instance or fusing straight-line state instructions. That is a distinct,
broader optimization pass and is not justified by the repaired motivating
shape.

## 12. Language/backend findings

| Classification | Finding |
|---|---|
| Compiler artifact | Unconditional history/resume fields and module clone/range helpers |
| Compiler artifact | Policy map, `any`, reflection, and filtered-candidate allocation for scalar sites |
| Necessary semantic cost | Persistent selected value, score, and commitment age |
| Necessary semantic cost | Result/completion behavior and board state |
| Optimization opportunity | Future instruction-switch fusion for straight-line single-state controllers |
| API ergonomic issue | Generated internal flow interfaces obscure the concrete controller type in Go, although Oct's `let controller = Flow(); Step(controller)` lifetime is natural |
| Not worth optimizing in M0 | Constant compatibility stubs and the stack-resident candidate literal |

## 13. Next recommendation

Return to Database-Scheduler M3 and integrate one long-lived generated flow
instance per controller lane; the motivating flow now has zero hot-path
allocations, persistent policy memory, and sufficiently small absolute decision
cost that another compiler specialization pass is not presently justified.
