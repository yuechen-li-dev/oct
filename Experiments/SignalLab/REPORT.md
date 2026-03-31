# SignalLab: An Experimental Evaluation of Octomata and Blackboard-Based Behavioral Control in Oct

## 1. Overview
SignalLab is a staged application experiment used to evaluate whether Oct can sustain a behavior-heavy, UI-backed system without runtime special cases. The project was designed to pressure-test three concerns together: Octomata as the primary control model, blackboard-mediated coordination, and day-to-day language ergonomics under immutable state.

The validation target was practical viability, not novelty: can control flow remain explicit, can data movement stay disciplined, and can code remain maintainable as milestones increase complexity from M0 through M9a.

## 2. Architecture Summary
SignalLab uses Octomata for control orchestration, combining hierarchical state flow structure with utility-style arbitration where multiple intents compete. Control policy is expressed in behavior logic, while application data remains in immutable records.

The blackboard is intentionally scoped to behavior-local policy memory (for interruption, arbitration, and coordination hints), not a general storage substrate for all app state. Core data travels through record-based immutable lanes, preserving explicit update paths.

M8 introduced a projection seam that removed mirrored fields and reduced representational duplication between behavior and display surfaces. Across milestones, the architecture remained separated by concern: behavior policy (Octomata + board), durable data (records), and view projection (UI/readout composition).

## 3. Key Language Features Validated
- Immutable records as the primary state model for deterministic updates.
- The `with` record-update expression (M9), which reduced structural copy boilerplate while preserving immutable semantics.
- Switch expressions for clearer decision mapping in label and mode-selection surfaces.
- Typing/error discipline remained sufficient for the experiment’s control/data boundaries without requiring ad hoc runtime exceptions.

## 4. Experiment Findings (High-Level)
- **M6–M7:** Blackboard integration proved viable for behavioral coordination, but also exposed pressure toward board-width growth and centralization if admission boundaries are loose.
- **M8:** Projection seam cleanup (including removal of mirrored fields) reduced artificial duplication and improved separation between policy and presentation concerns.
- **M9:** `with` materially improved maintainability by removing repetitive immutable reconstruction patterns, making record-centric state updates viable at larger surface area.
- **M9a:** Control-style signal shaping (low-pass filtering plus leaky accumulation) reduced decision chatter and mode thrash in this app, but the technique is narrow and should remain application-level until replicated elsewhere.

## 5. Final Conclusions
- Octomata is viable as a primary behavioral control model for this class of application.
- Blackboard is effective when constrained to policy/interruption memory rather than general-purpose state.
- Immutable records are workable at scale when paired with language support such as `with`.
- Over-centralization is still a concrete risk if board scope is allowed to grow indiscriminately.
- Control-theoretic shaping techniques are promising, but not yet candidates for core language features.

## 6. Future Directions
- Replicate M9a-style control shaping in additional applications before considering any standardization.
- Tighten board admission criteria to keep policy memory narrow and avoid accidental state centralization.
- Extract small reusable patterns where justified, while avoiding subsystem-scale abstractions.

---

# SignalLab Report

## M0 findings

### Was the current UI system sufficient to build this app?

Yes. The current UI primitives were sufficient to build a real, UI-backed application with explicit state, explicit event tokens, deterministic update logic, and a mount/emit/drain/update/patch loop. No runtime changes were needed.

### Where did layout feel natural vs awkward?

Natural:

- AnchoredBox worked well for high-level page slicing (header, control area, readout area, display placeholder).
- Row/Column worked well for local semantic grouping of controls and readouts.

Awkward:

- Keeping the same control intent represented in both grouped rows and explicit absolute placements adds duplication pressure.
- Cross-region alignment still requires manual visual arithmetic when two anchored areas need to line up with absolute clusters.

### Did coordinate hygiene help in a larger layout?

Yes. A local constants block (anchors, origins, spacing, shared sizes) significantly reduced drift and made edits local:

- Moving a control cluster is a one-constant change.
- Adjusting spacing is one-value tuning instead of many scattered literal edits.
- Readability improved because placements describe intent (`origin + offset`) instead of unrelated numbers.

### Were any UI primitives obviously missing?

Concrete gaps observed in M0:

1. A small first-class "labeled value" helper primitive (or equivalent compositional shortcut) would reduce repetitive readout row boilerplate.
2. Optional button-group helper semantics (author-side utility, not runtime feature) would reduce repeated event/enable wiring patterns for mutually exclusive selections.
3. A built-in lightweight numeric badge/readout element could reduce text-only formatting repetition.

None of these are blockers; they are ergonomic friction points.

### Did hybrid layout still feel like the right model?

Yes. Hybrid remained the right model:

- Anchors define macro composition.
- Row/Column handles local flow.
- AbsoluteBox handles precision touch-up points.

This combination gave predictable edit locality without introducing a heavier layout engine.

### What is the single most painful part of building this UI?

The most painful part was duplicated coordinate and control declarations when preserving both grouped semantic layout and precise absolute placements in one view. It stays manageable for M0 size, but becomes the dominant maintenance cost as control count increases.

## Recommendation

Proceed to **SignalLab M1** while keeping the existing runtime and layout model unchanged.

M1 should focus on:

- adding a minimal plot area implementation on top of the current model,
- evaluating whether the same coordinate hygiene conventions still scale,
- introducing only author-level helper conventions (not runtime features) for repeated control/readout patterns.

## M1 findings

### Did the current runtime/layout model remain sufficient?

Yes. M1 adds a richer surface (controls + readouts + mini plot region + sample history) while preserving the M0 architecture exactly:

- explicit external state,
- explicit event tokens,
- pure `Update(state, event) -> state`,
- pure `View(state) -> UI`,
- unchanged mount/emit/drain/update/patch loop.

No runtime API changes were required.

### Did the plot/display area fit naturally into the current UI system?

Yes. A minimal plot-like readout was implemented as an anchored display region with textual marker lines derived from bounded history state. This fit naturally into existing `AnchoredBox + Column + Text` composition, with no new rendering primitives.

### Did author-level helper conventions reduce duplication meaningfully?

Yes, at modest but real scale. Three helpers reduced repeated boilerplate:

- `LabeledValue(label, valueText)` for repeated readout rows,
- `SignalSelectButton(label, token, selected, enabled)` for mutually exclusive signal selection controls,
- `ReadoutBox(title, lines)` for stable readout grouping.

The helpers kept event wiring explicit while reducing copy-paste of label/event/enable patterns.

### Which helper patterns felt worth keeping?

Most useful in practice:

1. `SignalSelectButton` (highest value): centralizes selected/disabled behavior and makes intent obvious.
2. `LabeledValue`: lowers repetitive readout row construction.
3. `ReadoutBox`: helpful for structure, but lower leverage than the first two.

### Was any new runtime/widget feature actually justified?

Not yet. M1 did not justify runtime or widget expansion. Existing primitives were sufficient for this app growth step.

### What is now the single biggest pain point in building a richer app?

Manual record-copy updates are now the dominant cost. Because state is explicit and immutable, each event branch repeats full record reconstruction, including history fields. This is correct and transparent, but verbose as state shape grows.

## Updated recommendation

Proceed with one more app-level growth step before runtime changes.

- Continue validating richer app behavior in M2 with current runtime/layout.
- Prefer additional author-level helper conventions (especially for repetitive update/record wiring and grouped controls) before introducing new primitives.
- Revisit runtime changes only if multiple experiments converge on the same hard blocker.

## M2 findings

### What helper/state patterns were tried?

M2 kept the same state shape as M1 and introduced explicit author-level state-copy helpers:

- `WithRunning(model, value)`
- `WithNoiseEnabled(model, enabled)`
- `WithSelectedSignal(model, signal)`
- `WithTime(model, time)`
- `WithValue(model, value)`
- `WithTickCount(model, tickCount)`
- `WithHistory(model, count, h0, h1, h2, h3)`

`Update` now composes these helpers instead of repeating full record literals per event branch.

### Did they reduce Update verbosity?

Yes, materially inside `Update`. Event branches now read as intent-first transformations:

- start/stop: `Recompute(WithRunning(...))`
- toggle/select: `Recompute(WithNoiseEnabled(...))`, `Recompute(WithSelectedSignal(...))`
- step: unchanged high-level flow (`AdvanceOne -> Recompute -> RecordSample`) but `AdvanceOne` now uses helpers.

This removed repeated field-by-field reconstruction from the `Update` branches.

### Did readability improve?

Yes for event handling. The event logic is shorter and clearer because each branch names exactly which field changes.

Tradeoff: boilerplate did not disappear; it moved into helper definitions. This is acceptable for the probe because:

- boilerplate is centralized,
- naming carries intent,
- behavior remains explicit and pure,
- update flow is easier to scan quickly.

### Were helpers alone enough, or was state reshaping needed?

Helpers alone were enough for this milestone. No state reshaping was required to make `Update` readable at current app size.

### What is now the single biggest pain point?

The biggest remaining pain is maintaining many near-identical helper constructors as state fields grow. `Update` is cleaner, but helper maintenance still has record-copy drift risk.

### Is a language feature now justified, or can this stay author-level for now?

For current SignalLab scale, this can stay author-level for now.

M2 evidence suggests explicit immutable state remains workable with disciplined helper conventions, and immediate language/runtime changes are not yet justified. A language-level record update feature should only be reconsidered if multiple larger experiments show helper maintenance cost becoming dominant.

## M3 findings

### What control-state structure moved into Octomata?

M3 moved control-mode transitions out of plain helper branching into an explicit Octomata flow:

- `ControlModeTransition(current, event) -> String`
- states: `Idle`, `Running`, `Paused`
- transition selection uses `when` in each state with explicit guard ordering.

`Update` now delegates start/stop/pause/resume mode changes through `NextControlMode`, which steps the flow and applies the returned control mode.

### What remained ordinary record state?

The experiment kept numeric/display/application data in explicit immutable record fields:

- `Time`
- `Value`
- `TickCount`
- `SelectedSignal`
- `NoiseEnabled`
- `HistoryCount` + history slots

Step/recompute/history logic remained regular record-based state transformation.

### Did Octomata reduce complexity?

Partially.

- It made allowed control transitions materially clearer by concentrating them in one flow surface (`Idle/Running/Paused`) instead of scattering mode expectations across event branches.
- It reduced boolean-centric intent branching in `Update` for control events.

But total code volume did not drop dramatically because record-copy helpers still dominate data updates.

### Did readability of control transitions improve?

Yes.

The allowed transition graph is now explicit and reviewable in one place. It is easier to answer questions like “can paused resume?” or “what events are ignored in idle?” without scanning the whole reducer.

### Did update verbosity reduce enough to matter?

Moderately.

`Update` became cleaner for control events, but data-path verbosity is still primarily governed by explicit record updates and history management. So the gain is meaningful for control logic, not a broad verbosity solution.

### What became more awkward?

- A small adapter (`NextControlMode`) is needed to instantiate/step/read a flow result for each control event.
- The flow is excellent for control semantics but does not help with dense numeric/history record-copy boilerplate.

### Is this a better model than M2 helper-only reducer style?

For apps with non-trivial control modes, yes.

M3 suggests a hybrid split is cleaner than helper-only reducer style:

- Octomata for control-state progression/contracts.
- Records for numeric/data state.

### Should future interactive apps prefer this split?

Recommendation: **Yes, prefer the hybrid split by default when control-state transitions are a meaningful part of behavior.**

If an app has very shallow control state, helper-only style remains acceptable. But once mode transitions matter, Octomata provides clearer control contracts with low architectural disruption.

## M4 findings

### What parts were pushed into Octomata?

M4 intentionally moved far beyond M3’s control-mode-only flow. Octomata now owns:

- control progression plus interruption routing (`Idle`, `Running`, `Paused`, `Inspect`, `FaultHold`) via `ControlDirector`
- temporary interruption semantics (`Inspect`) using `suspend` + `remember` + `resume`
- fault hold and acknowledgement restoration path (`FaultHold`) using `suspend` + conditional `resume`
- display-mode arbitration via utility policy (`DisplayPolicy`)
- surfaced alert arbitration via utility policy (`AlertPolicy`)

The reducer still dispatches explicit events, but mode/display/alert decisions are now flow-driven rather than direct if/else field assignment.

### Which Octomata features were actually exercised?

M4 directly uses all required pressure features:

- `flow`
- `state`
- ordered `when`
- `utility when` with `hysteresis` and `min_commit`
- `suspend`
- `remember`
- `resume`

This was not decorative use: interruption and utility selection both affect live control mode and UI-facing derived state.

### What became dramatically clearer?

- The temporary interruption model became explicit: “remember current mode, detour, then resume” is represented directly instead of manually threading previous mode through record fields.
- Fault acknowledgement behavior became easier to reason about as a flow path than as ad-hoc reducer conditionals.
- Display/alert priority conflicts are now written as explicit scored policy instead of nested branching.

### What became worse?

- Integrating flow lifecycle into a one-event reducer required step orchestration (`Step` calls to completion), which is ceremony-heavy.
- Because app data still lives in immutable records, boilerplate `With*` helpers remain substantial.
- Using utility policies for presentation choices is powerful but can feel over-engineered for small UIs.

### Did this go beyond M3 in a genuinely useful way?

Yes. M3 proved Octomata for control transitions; M4 demonstrates Octomata can own broader behavioral surfaces (interruptions + policy arbitration) while keeping explicit UI/update architecture intact.

However, the gain is uneven: control/fault/interruption logic improved more than numeric/data handling.

### Where is the real line between Octomata and records?

Practical boundary from M4:

- **Octomata-owned**: mode transitions, interruption semantics, conflict-priority decisions, acknowledgment/resumption behavior.
- **Record-owned**: sampled numeric values, history buffers, raw readout payloads, deterministic signal generation.

Trying to push low-level numeric/sample storage into Octomata did not look promising; keeping it in records remains cleaner.

### Is Octomata now strong enough to be primary behavior model?

For interactive apps with meaningful mode logic and competing decisions: **yes, Octomata can be the primary behavior model**.

For numeric/data-heavy parts: **no, records should remain primary**.

Net recommendation after M4: use an Octomata-first behavioral core with explicit record data lanes, not an all-Octomata everything model.

## M5 findings

### What candidate blackboard shapes were compared?

M5 compared four narrow blackboard candidates against the exact M4 behavior-local awkward set:

1. **Flow-local mutable bindings** (machine-local mutable vars inside states)
2. **Special flow blackboard record** (explicit typed board attached to a flow)
3. **Typed slot/key model** (typed keys with get/set by slot)
4. **Specialized flow context object** (method-shaped context API; optional control)

### What concrete M4 pressure points were used?

All candidates were forced through the same behavior-local responsibilities that felt awkward in M4 record-heavy style:

- `ControlMode`
- `DisplayMode`
- `AlertStatus`
- `FaultLatched`
- interruption/resume machine memory (`Inspect`/`FaultHold` return target)
- policy/arbitration working outputs and commitment memory

Numeric/data-lane state (`Time`, sampled values, history payload) was intentionally held constant as immutable record state and treated as out-of-scope for the blackboard.

### Candidate A — Flow-local mutable bindings

**How it handled M4 responsibilities:**

- easy direct writes for `FaultLatched`, `DisplayMode`, `AlertStatus`
- interruption/resume memory can be stored in mutable locals
- policy counters can be incremented naturally

Pseudo shape:

```oct
state Running {
  mut FaultLatched: Bool = false
  mut DisplayMode: String = "Scope"
  mut AlertStatus: String = "Nominal"
  mut ResumeMode: String = "Running"

  when EventFaultTrip => {
    FaultLatched = true
    ResumeMode = current_state_name()
    goto FaultHold
  }
}
```

Evaluation:

- **Readability:** locally concise, globally risky (mutation surface is diffuse).
- **Boilerplate reduction:** high.
- **Explicitness:** medium-low; ownership boundaries blur quickly.
- **Abuse risk:** **very high** (becomes “mutable soup” by default).
- **Behavior vs data separation:** weak; encourages mutation patterns that can bleed into data-lanes conceptually.
- **Fit with Oct philosophy:** poor due to weak constraints.
- **Testability/reviewability:** degrades as mutable locals proliferate.

Verdict: **Rejected.** Convenient, but dangerously broad.

### Candidate B — Special flow blackboard record

**How it handled M4 responsibilities:**

- all awkward behavior-local fields become explicit board fields
- interruption/resume memory stored in `board.ResumeMode`
- policy/arbitration working memory stays with behavior owner (the flow)
- writes are explicit and searchable (`board.Field = ...`)

Pseudo shape:

```oct
record ControlBoard {
  FaultLatched: Bool
  DisplayMode: String
  AlertStatus: String
  ResumeMode: String
  PolicyCommitCount: Int
}

flow ControlDirector(board: ControlBoard) {
  state Running {
    when EventFaultTrip => {
      board.FaultLatched = true
      board.AlertStatus = "Fault"
      board.ResumeMode = "Running"
      suspend "FaultHold"
    }
  }
}
```

Evaluation:

- **Readability:** high; intent is explicit and scoped.
- **Boilerplate reduction:** meaningful (removes repeated immutable record copy churn for behavior-local scratch state).
- **Explicitness:** high; reads/writes and ownership are obvious.
- **Abuse risk:** medium-low if constrained to flow scope.
- **Behavior vs data separation:** strong; board is behavior-local by construction.
- **Fit with Oct philosophy:** strong (explicit, typed, constrained, reviewable).
- **Testability/reviewability:** strong; board fields form clear correctness surface.

Verdict: **Preferred.** Best balance of ergonomics and guardrails.

### Candidate C — Typed slot/key model

**How it handled M4 responsibilities:**

- pressure points map to slots (`FaultLatched`, `DisplayMode`, etc.)
- interruption/resume and policy counters become keyed entries

Pseudo shape:

```oct
board.set(FaultLatched, true)
board.set(AlertStatus, "Fault")
board.set(ResumeMode, current_state_name())
```

Evaluation:

- **Readability:** medium; semantics are less visible than named fields.
- **Boilerplate reduction:** medium.
- **Explicitness:** medium; type-safe keys help but indirection hurts scanability.
- **Abuse risk:** medium-high (slot growth and cross-cutting key usage invite framework sprawl).
- **Behavior vs data separation:** medium; technically possible but socially easier to erode.
- **Fit with Oct philosophy:** mixed; more framework-like than language-direct.
- **Testability/reviewability:** medium-low for larger machines due to key indirection.

Verdict: **Rejected.** Too indirect and framework-ish for Oct’s clarity goals.

### Candidate D — Specialized flow context object (optional)

**How it handled M4 responsibilities:**

- provides narrow behavior APIs (`latch_fault`, `capture_resume_mode`)
- can constrain writes tightly, but hides state changes behind methods

Evaluation:

- **Readability:** mixed; operations read nicely, but underlying state mutations become opaque.
- **Boilerplate reduction:** medium.
- **Explicitness:** medium-low (indirection through methods).
- **Abuse risk:** low-medium.
- **Behavior vs data separation:** strong.
- **Fit with Oct philosophy:** mixed; drifts toward framework surface rather than direct language model.
- **Testability/reviewability:** mixed; requires jumping between API and behavior code.

Verdict: **Not selected.** Safer than A/C, but less explicit and too API-mediated.

### Narrowing outcome and provisional recommendation

M5 materially narrows the space: **use an explicit, typed, flow-owned blackboard record model** (Candidate B).

Provisional blackboard shape for Octomata:

- one fixed-shape typed board per flow instance
- board access only inside flow/state transition contexts
- explicit field reads/writes (no string keys, no dynamic slot registration)
- explicit boundary handoff between reducer/app record and flow board
- preserve the M4 split: board for behavior-local working memory, immutable records for numeric/data lanes

### Open questions before any implementation

1. How boundary synchronization should work between reducer record state and flow board snapshots.
2. Whether utility-policy commitment/hysteresis memory should be visible board fields or private flow internals.
3. Initialization/reset lifecycle rules for board fields across start/stop/fault-clear cycles.
4. Preferred testing style for board-dependent behavior without overcoupling tests to incidental internals.

### M5 non-goal confirmation

No runtime changes were added. No language feature/syntax changes were added. No blackboard implementation was added. M5 remained a design/refactor thought experiment only.

## Preliminary Conclusion

From M0 through M5, SignalLab indicates that the current UI runtime model is sufficient for this class of app. The hybrid layout approach remains the working default: `AnchoredBox` for macro regions, `Row`/`Column` for local grouping, and `AbsoluteBox` for precision touch-ups. Coordinate hygiene plus author-level UI helpers reduced practical friction without requiring runtime changes.

For state ergonomics, explicit immutable record state remains workable for data-heavy lanes at current scale. Author-level helper conventions reduced update noise enough to keep reducers readable, so this experiment alone does not yet justify language-level record-update features.

For control behavior, Octomata showed clear value as a behavioral contract surface. It materially improved clarity for mode progression, interruption and fault handling, and prioritization/arbitration logic. At the same time, M4 reinforces that Octomata should not be treated as a universal replacement for all state.

The emerging default architecture from SignalLab is therefore a split model: Octomata for behavior/control progression, interruption, and policy decisions; ordinary records for numeric state, history buffers, and deterministic data pipelines. This is a preliminary conclusion from SignalLab, not a universal rule.

M5 further narrows the missing behavior-local working-state category toward a constrained, flow-owned typed blackboard record rather than general mutable locals or key/value slot frameworks.

Forward-looking guidance: future nontrivial interactive Oct apps should likely start from this split when control-state transitions are meaningful, while validating the pattern against at least one structurally different app before promoting it to broader doctrine.

## M6 findings

### What exact board shape was implemented?

M6 implemented a **fixed-shape typed flow board**:

- `SignalBoard { DisplayMode, AlertStatus, FaultLatched, ResumeMode }`
- one board snapshot is carried per SignalLab flow invocation and fed back into app state
- field writes use direct syntax (`board.Field = ...`) inside flow state bodies only

No dynamic slots, no key APIs, no framework wrapper methods were added.

### Which fields moved into the board?

Moved into `SignalBoard`:

- `DisplayMode`
- `AlertStatus`
- `FaultLatched`
- `ResumeMode`

Kept in ordinary records/data lanes:

- `Time`
- `Value`
- `TickCount`
- `SelectedSignal`
- `NoiseEnabled`
- history buffer payload fields

### Did board improve behavior-local clarity?

Partially yes.

- Fault/display/alert policy writes now sit in flow-local policy paths instead of reducer-side helper churn.
- Resume intent memory has an explicit home (`board.ResumeMode`) instead of ad-hoc cross-branch threading.

### Did board stay constrained enough?

Yes in this implementation:

- writes are restricted to flow state contexts
- only `board.<field> = <expr>` mutation is accepted
- non-board field mutation and non-flow use are rejected

This kept board from becoming generic mutable app state.

### Dirty tracking status

Included internally.

- runtime marks board fields dirty on board-field writes
- no public API was added for dirty inspection in M6
- behavior is internal-only and narrow

### What became cleaner vs riskier?

Cleaner:

- behavior-local fault/display/alert updates are easier to read in flow form
- less reducer-level helper noise for those fields

Riskier/awkward:

- explicit board handoff between reducer state and flow calls adds seam plumbing
- mixed model (board + mirrored UI-facing fields) introduces potential drift if discipline slips

### Did Candidate B survive first implementation?

**Yes, with caveats.**

The typed, flow-local board model is viable and useful, but it is only safe when constraints stay strict (flow-only writes, fixed shape, no dynamic access). It feels like a real missing piece for behavior-local working memory, not a blanket mutability model.

## M7 findings

### What additional responsibilities moved into the board?

M7 deliberately overloaded the same single flow-local board with more behavior-local memory than M6:

- richer display behavior metadata (`DisplayOverlay`)
- expanded alert memory (`AlertSeverity`, `AlertLatched`)
- split interruption targets (`ResumeModeInspect`, `ResumeModeFault`) while keeping one board
- policy/hysteresis-adjacent commit memory (`CooldownTicks`, `CooldownActive`, `JustTriggered`)
- UI-facing behavior digest (`BehaviorSummary`)

This was done without introducing new APIs/syntax/runtime features, and without dynamic keys.

### What explicitly stayed in records (and why)?

Data lanes stayed out of board and remained immutable record fields:

- `Time`, `Value`, `TickCount`
- `SelectedSignal`, `NoiseEnabled`
- sample history payload/buffer slots

Reason: these are numeric pipeline/state payload concerns, not behavior-local arbitration memory. Moving them into board would blur the behavioral boundary and collapse into generic mutable app state.

### Clarity impact

Mixed result:

- Improved: control/policy edge-memory intent is local and explicit in flow policy paths.
- Worse: board shape became much wider and harder to scan; “what matters for this transition?” is no longer obvious at a glance.

M7 shows that board clarity degrades once too many adjacent concerns are packed into one object, even if each field is technically behavior-local.

### Boilerplate impact

Board reduced some reducer-side helper churn for behavioral flags, but introduced board-field bookkeeping noise:

- initialize/reset touchpoints expanded
- policy flow writes expanded
- handoff synchronization burden increased

Net: boilerplate shifted, not eliminated.

### Centralization risk

Yes, visible in M7.

The board started to trend toward a local “god object” for anything behavioral, including fields that are only weakly coupled. This did not fully collapse architecture, but the pressure signal is real: once many categories accumulate, review and ownership boundaries get fuzzy.

### Seam plumbing pressure

Increased and now noticeable:

- every reevaluation path carries a larger board snapshot
- event-to-policy handoff became a key seam (`ReevaluateBehavior(..., event)`)
- reset/ack/resume paths now require broader board synchronization discipline

Still acceptable at this app size, but this is now a limiting factor.

### Boundary integrity (board vs records)

Still intact in M7 because numeric lanes were kept out by rule.  
However, pressure at the boundary increased: the temptation to move “just one more derived datum” into board is now high once board already contains many fields.

### Dirty tracking value under heavier board usage

Dirty tracking became more justified conceptually (many fields mutate transiently), but still mostly invisible in author workflow because no public inspection surface exists. M7 increases potential value, but not practical payoff yet.

### One-board-per-flow viability

One board remains viable but is now near its readability limit for this domain.

- No hard requirement for multiple boards emerged.
- Clear pressure emerged for sub-grouping or tighter board scope constraints.

### Refined boundary after pressure probe

**Board is good for:**

- interruption/resume intent memory
- policy commit edges and short-lived behavioral flags
- derived behavior summaries that are tightly coupled to control arbitration

**Board is bad for:**

- numeric data lanes and plot payload
- broad UI presentation state that is not directly tied to behavior contracts
- accumulating heterogeneous concerns “because it is convenient”

### Direct recommendation for next step

Do **not** expand board breadth further right now.  
Next step should be **refining seam/integration discipline and tightening board field admission rules**, not adding board categories.

If pressure continues to rise, restrict board usage further (narrower “behavior contract memory” only) before considering shape growth.

## M8 findings

### What board-owned fields were removed from outer app state?

M8 removed three mirrored behavior-local fields from `SignalState`:

- `DisplayMode`
- `AlertStatus`
- `FaultLatched`

These fields now live only on `SignalBoard` and are no longer copied into outer state during reevaluation.

### What explicit projection seam was introduced?

M8 introduced one narrow UI projection seam:

- `ProjectReadout(model: SignalState) -> ViewReadout`

`ViewReadout` carries only the board-owned values the view needs (`DisplayMode`, `AlertStatus`, `FaultLatched`) and `View` consumes that projection instead of directly mirroring or recomputing these values.

### Did seam collapse materially reduce duplication?

Yes, moderately.

- `WithBoardFields` was removed entirely.
- `WithDisplayMode`, `WithAlertStatus`, and `WithFaultLatched` were removed.
- All remaining `With*` constructors no longer need to carry those three mirrored fields.

This reduced both update-path wiring and constructor churn without introducing new language/runtime features.

### What became cleaner?

- Ownership is clearer: board policy fields are owned by `SignalBoard` only.
- `ReevaluateBehavior` is simpler (`UpdateBoardPolicy` then `WithBoard`).
- `NextControlMode` fault input now reads from the board source of truth directly.
- UI readout intent is explicit at one seam (`ProjectReadout`) instead of dispersed mirror access.

### What became more awkward?

- Some tests and call sites now explicitly dereference `model.Board.*` when asserting policy behavior.
- Projection seam introduces a small extra record/function pair, which is additional structure for a small app.

### After seam cleanup, what boilerplate still remains?

Significant immutable-record boilerplate remains in data lanes:

- `WithControlMode`, `WithBoard`, `WithNoiseEnabled`, `WithSelectedSignal`, `WithTime`, `WithValue`, `WithTickCount`, `WithHistory`
- step/recompute/history flows still require chained immutable updates

So the main remaining pressure is record-copy for numeric/data fields, not board mirror maintenance.

### Is `with` now clearly justified, or is more seam cleanup still needed?

M8 makes the pressure signal cleaner:

- seam cleanup removed artificial duplication due to dual ownership
- remaining verbosity is largely intrinsic immutable record update cost

Recommendation after M8: a `with`-style record update feature is now **better justified** than before, because seam discipline has already removed a clear architectural source of noise.

## M9 findings

### Implemented `with` shape

M9 implemented a narrow immutable record update expression:

- `sourceExpr with { Field: value ... }`

Semantics:

- evaluates `sourceExpr` exactly once
- requires `sourceExpr` to be a record value
- validates updated field names against that record type
- typechecks each updated value against the declared field type (including dimensions)
- returns a **new** record of the **same nominal type**
- copies unspecified fields from source as-is

This is not mutation and does not add dynamic patch objects.

### Zero-field decision

`with {}` is rejected in M9 as useless/no-op sugar.

### SignalLab helper families replaced

M9 removed the record-copy helper family from `SignalLab` M9 state updates:

- removed `WithControlMode`
- removed `WithBoard`
- removed `WithNoiseEnabled`
- removed `WithSelectedSignal`
- removed `WithTime`
- removed `WithValue`
- removed `WithTickCount`
- removed `WithHistory`

Call sites now update directly with explicit field lists, e.g.:

- `model with { NoiseEnabled: model.NoiseEnabled == false }`
- `model with { SelectedSignal: SignalSine() }`
- `model with { Time: model.Time + 0.25 TickCount: model.TickCount + 1 }`
- `model with { HistoryCount: nextCount ... }`

### Boilerplate/readability outcome

Material improvement.

- repetitive full-record reconstruction helpers disappeared
- update intent is now local at call sites
- multi-field updates read as one coherent state transition instead of helper chaining

Net: this removes the remaining “immutable-copy ceremony” that dominated M8’s data-lane updates.

### Edge-case behavior observed

Covered by language tests:

- basic valid update
- multi-field valid update
- unknown field rejection
- type mismatch rejection
- non-record source rejection
- source evaluated once
- chaining (`x with {..} with {..}`) works
- empty update rejected

Nested field-path sugar (e.g. `Child.X`) remains unsupported in M9 by design.

### Boundary interaction (records vs board)

No new board bypass was introduced.

- `with` updates ordinary immutable records only
- board ownership rules and flow-only board mutation remain unchanged
- M9 usage stayed in the existing record-update lanes and did not blur board boundaries

### Remaining ergonomics pain after `with`

Main remaining friction is not immutable record copying anymore. Remaining pain points are:

- some long update sets are still verbose when many fields must move together (explicitness cost)
- no nested-path update sugar (intentional for narrowness)

These are acceptable tradeoffs for a first feature shape.

### Recommendation

**Adopt `with` as a real language feature** with the M9 narrow shape.

Rationale:

- solves a concrete, isolated ergonomics problem
- preserves immutability and nominal typing
- keeps explicit field updates and strict checking
- does not force blackboard or mutability model expansion
- produces immediate clarity/boilerplate wins in SignalLab without architectural regression

## M9a findings

### Decision surface chosen

M9a targeted **alert surfacing priority** under noisy near-threshold input, because this was a concrete jitter-prone decision in prior milestones (rapid Watch/Normal flips when values hover around a threshold).

### Baseline vs controlled comparison

M9a keeps the same decision surface and runs two variants side-by-side inside the board:

- **Baseline**: direct threshold (`value <= -0.20` under noisy noise-signal mode).
- **Controlled**: same signal passed through tiny control memory before alert decision.

The app-facing `AlertStatus` now uses the controlled variant (except fault override), while baseline is retained as an internal comparator for measured chatter.

### Control primitives used

Two explicit tiny primitives were used:

1. **Low-pass filter** on raw pressure (`filtered = prev*0.60 + raw*0.40`)
2. **Leaky accumulator** (`leaky = prev*0.70 + filtered`)

No framework, PID subsystem, or generalized control layer was added.

### Where memory lived

Control memory was stored in the existing **board**:

- `AlertSignalRaw`
- `AlertSignalFiltered`
- `AlertSignalLeaky`
- baseline/controlled status and switch counters

No new state architecture was introduced.

### What improved

On an explicit near-threshold pressure sequence, the baseline flips multiple times while the controlled variant flips less. This reduced alert chatter and made policy behavior steadier without hiding logic behind a large abstraction.

### What got worse

- Source complexity increased modestly: there are now dual tracks (baseline and controlled) plus counters for comparison.
- For this narrow gain, extra fields are required on the board, which is honest but not free.

### Was it clearer or more obscure?

Mostly still clear. The control math is small and explicit, but readability would degrade quickly if many more primitives were layered in without discipline.

### Board as memory location

Yes. The board felt like the right place for this behavior-local control memory; it stayed close to policy and remained deterministic.

### Recommendation

**Promising but narrow**:

- Keep this as an application-level technique by default.
- A future tiny Octomata control-primitive layer is only justified if multiple experiments show the same pattern with similar small primitives.
- Do not expand toward generic control-theory infrastructure from this result alone.
