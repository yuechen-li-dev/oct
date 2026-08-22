# Interpreter-owned Octomata flow checkpoint API recon

Status: design/recon only. This document does **not** implement persistent Make flow resume, checkpoint files, `oct make` flags, Make staleness changes, failure-artifact schema changes, or Octomata language semantics changes.

## 1. Runtime audit

The previous Make-flow resume recon correctly stopped because Make cannot safely infer the complete continuation of an Octomata flow from trace-visible state alone. The runtime object that actually owns continuation is `*interpret.FlowRuntimeInstance`.

### Mutable continuation state that matters

Exact interpreter continuation from an intentional `suspend` needs these logical pieces:

- flow identity: `Package` plus `Decl.Name`;
- source/runtime declaration compatibility: enough of `Decl` to validate the current state and cursor against the current flow declaration;
- root bindings created at instantiation:
  - flow parameters, for general flow checkpointing;
  - the synthetic `board` record when the flow declares a board;
- current execution location:
  - `CurrentState`;
  - `InstructionIndex`;
  - `StateEnv`, because same-state `let` bindings live there across top-level statements in a state;
- resume slot:
  - `HasResumeTarget`;
  - `ResumeTarget`;
- observability/semantic state:
  - `StateHistory`;
  - `UtilityWhenSites` for controller-bound utility `when` expressions;
- run accounting for host callers:
  - prior step/transition count if a host wants a cumulative MaxSteps policy.

`Completed` and `Result` matter to a completed flow instance, but Make persistent resume should only export suspended incomplete flows in M0. A checkpoint export API should reject completed instances unless a future caller explicitly wants a completed-flow snapshot.

### Runtime fields that should not be persisted directly

These fields are implementation details, caches, or object-graph roots and must not be serialized as raw Go state:

- `Decl ast.FlowDecl`: use current source at restore time plus a stable flow fingerprint/structural validation, not a serialized AST object graph.
- `RootEnv *environment`: reconstruct from flow instantiation and logical board/parameter payloads.
- `StateEnv *environment`: either reconstruct from logical local bindings or reject the checkpoint shape.
- `DirtyBoardFields`: debug/incremental tracking for board writes; exact continuation can recompute future dirtiness from restored board and later execution.
- `Completed`/`Result`: should be reset for suspended checkpoint restore; completed snapshots are not an M0 resume goal.
- any `ValueFlow`, function handles, wrapper/sidecar handles, file/process handles, channels, or arbitrary interpreter heap references reachable from values.

### State execution and suspension boundary

The interpreter executes one top-level state-body statement at `InstructionIndex`. `suspend` returns a flow signal; `stepFlowWithTransitionLimit` increments `InstructionIndex` before returning `suspended=true`. Therefore suspend occurs at a statement boundary for top-level `suspend` statements and for the containing top-level statement when a nested `if`, loop, `match`, `when`, or action block propagates a suspend signal.

`InstructionIndex` identifies only the next **top-level** statement in the current state's body. It does not record nested block position, loop iterator position, or partially evaluated expression state. Today that is safe only because a suspend unwinds after the containing statement; any nested block/loop is not resumable in the middle. If the suspended top-level statement is a loop/conditional/match/when whose post-suspend semantics depend on nested execution context, `InstructionIndex` alone can skip or replay work. M0 must therefore define checkpointability around statement-boundary shapes that the interpreter can prove safe.

### `StateEnv` behavior and possible values

`StateEnv` is created when a state is entered and parented to `RootEnv`. It persists across top-level statements in that state until `goto`/`resume` enters a new state. Ordinary statements executed inside the state define `let` bindings in this state environment. Nested blocks create child environments, so locals defined inside nested block scopes are normally not stored in `StateEnv`, but top-level state locals remain live after a suspend if later statements reference them.

Values that can appear in `StateEnv` are normal interpreter `Value` instances produced by Oct expressions allowed in flow states: scalar numeric/dimensioned values, strings, bools, arrays, records/enums, ranges, vectors/matrices where allowed by ordinary expression typing, errors from fallible paths if bound, function values, flow instances, batches or UI values if admitted by current type rules, and the synthetic `__oct_flow_instance` binding. Not all of these are checkpointable.

Safe serialization of `StateEnv` requires a whitelist. The synthetic flow-instance binding must never be persisted. M0 should either reject non-empty user locals or allow only Octagon-compatible scalar/array/record/enum-without-payload data whose static type can be revalidated against the current state body.

### Resume slot

`remember` stores the current state in `HasResumeTarget`/`ResumeTarget`. `resume` requires the slot, clears it, and performs a `goto` to the remembered state. Losing this slot changes behavior and diagnostics, so it is part of exact continuation. Restore must validate that the stored target still exists when `HasResumeTarget` is true.

### Utility `when` controller state

`UtilityWhenSites` is a map keyed by AST-assigned utility site ID. Each entry stores:

- whether a current committed value exists;
- the current value;
- the score associated with that selection;
- commit age.

Controller-bound utility `when` uses this state to enforce hysteresis and minimum-commit behavior. Resetting it can change future choices even when board and state cursor are identical. M0 must not silently reset it. Either persist it as logical scalar/Octagon-compatible data and validate site compatibility, or reject checkpoints for flows with active controller-bound utility state.

### Board snapshots and Octagon representability

Flow board fields are constrained by the typechecker to `Bool`, `String`, `Int`/dimensioned `Int`, `Float`/dimensioned `Float`, and arrays of those types. The interpreted `BoardSnapshot` builtin copies the current synthetic board record into a typed snapshot record, and the Octagon writer/loader path supports records, arrays, scalar ints/floats with dimensions, bools, strings, and payload-free enums. For legal board fields today, board snapshots are logically representable as Octagon-compatible data, including arrays of scalar board fields.

The former documentation gap is closed: the runtime reference now matches the typechecker and fixtures by documenting arrays of supported scalar board fields, including nested arrays.

### Compiled Octomata support

The compiled backend lowers flow instances to generated Go structs with analogous fields: started/completed state, current state integer, instruction integer, result/history, resume slot, optional utility site map, parameters, and typed board fields. It exposes `Step`, `Active`, `Complete`, `Result`, `StateHistory`, `ResumeTarget`, and `BoardSnapshot`, but it does not expose a logical checkpoint export/restore path. M0 checkpointing should be interpreter-only unless a future Make path runs compiled FlowTargets. Compiled checkpointing needs a separate lowering design.

## 2. Ownership boundary

### Make owns

Make should own host/build concerns only:

- checkpoint file path and lifecycle;
- Make target identity;
- target fingerprints and staleness interaction;
- Make file hash and dependency/input snapshots;
- CLI visibility and user prompts;
- trace/failure artifact linkage;
- deleting a checkpoint after successful completion.

### Interpreter owns

The interpreter should own flow continuation concerns:

- export of logical resumable flow state;
- restore into a fresh runtime instance from current source;
- state/cursor/source/body compatibility validation;
- board encode/decode and schema validation;
- state-local policy and reconstruction;
- resume-slot validation;
- utility-controller export/restore or explicit refusal;
- explicit `CheckpointUnsupported` errors for unsafe shapes.

Make must not serialize raw interpreter object graphs, reconstruct `StateEnv`, decide how `InstructionIndex` maps to state bodies, or reset utility controllers.

## 3. Interpreter checkpoint payload

The interpreter should expose a logical payload, not a file format. A proposed internal Go shape is:

```go
type FlowCheckpoint struct {
    Version int
    Package string
    Flow string
    FlowFingerprint string
    CurrentState string
    Cursor FlowResumeCursor
    HasResumeTarget bool
    ResumeTarget string
    Board FlowBoardCheckpoint
    StateLocals []FlowLocalBinding
    StateHistory []string
    UtilityWhenSites []FlowUtilitySiteCheckpoint
    StepCount int
}

type FlowResumeCursor struct {
    InstructionIndex int
    CursorKind string // e.g. "top-level-statement-next"
    StateBodyFingerprint string
}
```

Additional helper types should use Octagon-compatible value shapes rather than raw `interpret.Value` where persistence is involved. In-process APIs may temporarily carry `Value`, but any persistence-ready export should convert to a stable data model first.

### Source fingerprint ownership

Make should own coarse Make-file and target/dependency fingerprints. The interpreter should still own a flow-level fingerprint because only it knows which AST body/cursor/utility sites are meaningful. For M0, the interpreter can use a deterministic fingerprint over the flow signature, board declaration, state names, state body statement fingerprints, and utility site IDs. Restore should validate both the whole-flow fingerprint when present and structural compatibility when fingerprints are absent or intentionally ignored.

### Versioning

Use a monotonically increasing `Version` with strict restore behavior:

- reject unknown future versions;
- accept the current version;
- optionally migrate older versions in interpreter-owned migration code;
- include a `CheckpointKind` or `CursorKind` if multiple cursor schemes are introduced.

## 4. State locals policy

### Option A: serialize checkpointable state locals

Pros:
- preserves normal mid-state suspends where later statements depend on earlier `let` values;
- most exact continuation model.

Cons:
- requires static liveness/type validation for locals after the cursor;
- requires stable serialization of local values;
- must reject functions, flow instances, UI/native/handle-like values, fallible/internal values, and arbitrary heap graphs;
- needs body compatibility precise enough to know the local still exists and has the same meaning.

### Option B: restrict to no-live-locals / boundary-safe suspends

Pros:
- simplest safe M0;
- avoids serializing arbitrary locals;
- aligns with Make's likely flow style, where durable progress belongs on the board.

Cons:
- rejects some valid in-process flows;
- users may need to restructure flows.

### Option C: style convention: suspend then resume at state boundary

Pros:
- encourages durable state through board fields and explicit states;
- checkpoint can be mostly board + state + cursor.

Cons:
- by itself it is a convention, not a guarantee;
- if `suspend` is followed by same-state statements, exact continuation still needs cursor and possibly locals.

### Recommendation

M0 should use a strict checkpointable subset:

1. Export only suspended, incomplete interpreter flow instances.
2. Allow top-level statement-boundary cursors only.
3. Reject checkpoints when user state locals are present unless an explicit local serializer has been implemented and each local is Octagon-compatible and statically revalidated.
4. Prefer durable progress via board fields and states; document `CheckpointUnsupported` with a reason such as `StateLocalsUnsupported` or `NestedSuspendCursorUnsupported`.

This removes the Make blocker without pretending that Make can infer liveness or serialize interpreter heaps.

## 5. Utility-state policy

M0 must not reset utility state. The recommended initial policy is:

- if `UtilityWhenSites` is empty, checkpointing may proceed;
- if it is non-empty, either:
  - serialize each site entry as `{SiteID, HasCurrent, Current, Score, CommitAge}` using the same Octagon-compatible value whitelist used for locals, and validate that the current flow still has the same site IDs/fingerprints; or
  - reject export with `UtilityStateUnsupported`.

For safest H1 staging, reject non-empty utility state. H2 can add serialization once utility site fingerprints are stable. If utility checkpointing is implemented, compatibility should validate site ID, site fingerprint, selected value type, and current/score/commit-age scalar ranges.

## 6. Board checkpoint serialization

Recommended API shape:

```go
func ExportBoardCheckpoint(instance *FlowRuntimeInstance) (FlowBoardCheckpoint, error)
func RestoreBoardCheckpoint(instance *FlowRuntimeInstance, checkpoint FlowBoardCheckpoint) error
```

`FlowBoardCheckpoint` should contain:

- board record type/name;
- ordered fields;
- static type descriptors including dimensions and array depth;
- Octagon-compatible values.

Export should:

1. locate the synthetic `board` binding;
2. validate every declared board field exists;
3. validate every runtime value matches the declared board type;
4. deep-copy into stable value data;
5. reject non-board flows if the checkpoint schema requires a board, or encode an explicit empty board for boardless flows.

Restore should:

1. instantiate a fresh flow;
2. compare declared board field names, order, types, dimensions, and array depths;
3. materialize checkpoint values;
4. assign the synthetic board record in `RootEnv`;
5. clear `DirtyBoardFields`.

## 7. Restore API and semantics

Proposed interpreter APIs:

```go
func ExportFlowCheckpoint(inst *FlowRuntimeInstance, opts FlowCheckpointOptions) (FlowCheckpoint, error)

func InstantiateFlowFromCheckpoint(program project.Program, pkg string, flow string, cp FlowCheckpoint, opts FlowRestoreOptions) (*FlowRuntimeInstance, error)

func RunFlowToCompletionFromCheckpointWithOptions(program project.Program, pkg string, flow string, cp FlowCheckpoint, maxSteps int, stdout io.Writer, options ExecuteOptions) (FlowRunResult, error)
```

Restore should:

1. create a fresh interpreter and instantiate the current flow declaration;
2. validate checkpoint version, package, flow, return shape, and M0 zero-parameter requirement when called from Make;
3. validate source/body fingerprint if present;
4. validate state exists;
5. validate `InstructionIndex` is in range and cursor kind is supported;
6. restore board values;
7. set `CurrentState`, `InstructionIndex`, `HasResumeTarget`, `ResumeTarget`, and `StateHistory`;
8. reconstruct `StateEnv` from `RootEnv`, the synthetic flow binding, and allowed logical locals;
9. restore utility sites or reject;
10. reset `Completed=false` and `Result=Value{}`;
11. continue from the post-suspend cursor without re-executing the suspended statement.

## 8. Compatibility validation

Interpreter-owned restore errors should be explicit and machine-readable. Suggested reason codes:

- `UnsupportedCheckpointVersion`
- `PackageMismatch`
- `FlowMismatch`
- `FlowFingerprintMismatch`
- `StateMissing`
- `InstructionIndexOutOfRange`
- `StateBodyChanged`
- `BoardSchemaMismatch`
- `BoardValueTypeMismatch`
- `StateLocalUnsupported`
- `StateLocalTypeMismatch`
- `UtilityStateUnsupported`
- `UtilitySiteMismatch`
- `ResumeCursorInvalid`
- `ResumeTargetMissing`
- `CompletedCheckpointUnsupported`

For M0, structural validation is required even when Make invalidates on coarse file hashes: state exists, cursor is in range, board schema matches, resume target exists, state history entries reference known states, and utility site count/IDs match if persisted. Flow/body fingerprints are recommended because they let the interpreter explain source incompatibility rather than leaving all invalidation to Make.

## 9. Run-result integration

Keep export explicit. `FlowRunResult` should not always carry serialized checkpoint data. A future run API can expose the suspended instance through an interpreter-owned callback or return a lightweight handle that permits:

```go
if result.Suspended {
    cp, err := result.ExportCheckpoint()
}
```

For the existing Make-oriented helper, a staged API can add an option like `CheckpointOnSuspend bool` and return either `Checkpoint *FlowCheckpoint` or `CheckpointUnsupportedReason`, but the cleaner design is to separate execution from checkpoint export so callers opt in.

## 10. Compiled support decision

Checkpointing is interpreter-only for M0. Compiled flows have analogous generated state but no stable logical restore seam. Future compiled support needs:

- MIR-level checkpoint metadata;
- generated export/restore methods;
- typed board/local/utility serialization;
- compatibility between generated state IDs and source fingerprints.

Make should use the interpreter checkpoint path for persistent FlowTarget resume until compiled support is explicitly designed and tested.

## 11. Staged implementation path

Recommended staging:

```text
FLOW-CHECKPOINT-API-H1:
  interpreter export only
  suspended incomplete instances only
  cursor + board + resume slot + history + flow fingerprint
  reject user StateEnv locals
  reject non-empty UtilityWhenSites
  tests for checkpointable and unsupported suspend shapes

FLOW-CHECKPOINT-API-H2:
  interpreter restore from checkpoint
  exact continuation tests for board/state/history/resume slot
  schema/source/cursor mismatch diagnostics

FLOW-CHECKPOINT-API-H3:
  optional checkpointable locals whitelist or static no-live-local checker
  optional utility-site serialization with site fingerprints

MAKE-FLOW-RESUME-M0:
  Make checkpoint file wrapper
  Make-owned invalidation/staleness/CLI/trace/failure integration
  no raw interpreter object graph serialization
```

If H1 audit during implementation proves local serialization is small and robust, H1 may include an explicit Octagon-compatible local whitelist, but it should still reject unsupported values with precise reasons.

## 12. Future tests

Future implementation should add or update tests for:

1. a flow suspending at a boundary-safe point exports a checkpoint;
2. restore from checkpoint completes the flow;
3. board values survive restore, including strings, bools, dimensions, and scalar arrays;
4. state history appends after resume rather than being reset;
5. the post-suspend same-state statement executes exactly once;
6. state locals needed after suspend are restored or produce `StateLocalUnsupported`;
7. utility `when` state is restored exactly or produces `UtilityStateUnsupported`;
8. board schema mismatch rejects restore;
9. missing state rejects restore;
10. instruction index mismatch rejects restore;
11. resume target mismatch rejects restore;
12. no raw Go object graph serialization appears in checkpoint output;
13. compiled flow checkpointing remains documented as unsupported for M0.

Language behavior tests should live in `Language/` when semantics are involved. Go tests should validate interpreter API boundaries and diagnostics without duplicating language semantics already covered by `.octest`/`.octfail` contracts.

## 13. Answers to audit questions

1. Exact continuation needs package/flow identity, current source compatibility, board/root values, current state, instruction cursor, state env locals, resume slot, state history, utility site state, and host step count.
2. Raw AST declarations, raw environments, dirty board tracking, completed/result state for suspended checkpoints, and heap/object references are implementation/cache/debug state and should not be persisted directly.
3. `StateEnv` can contain ordinary Oct runtime values plus the synthetic flow binding; the user-local subset can include scalars, arrays, records/enums, function/flow values, vectors/matrices, errors, and other values admitted by state expressions.
4. `StateEnv` can be serialized safely only with a strict whitelist and static compatibility validation; M0 should reject user locals unless that whitelist is implemented.
5. `suspend` returns at statement boundaries, but nested statements make the resumable cursor coarser than the nested execution point.
6. `InstructionIndex` identifies the next top-level state statement, not nested block/loop/expression continuation.
7. `HasResumeTarget`/`ResumeTarget` are semantic single-slot continuation data for `remember`/`resume` and must be restored or validated empty.
8. `UtilityWhenSites` stores committed value, score, and commit age per site; resetting it can change hysteresis/min-commit behavior.
9. Compiled flow execution has analogous generated fields but no checkpoint path; M0 should be interpreter-only.
10. Legal board fields are Octagon-representable under current implementation, including arrays of scalar board values, though the runtime reference has a scalar-only documentation gap.

## 14. Non-goals preserved

This recon does not implement persistent Make resume, write checkpoint files, add Make CLI flags, alter Make staleness, alter failure artifact schemas, change Octomata semantics, serialize Go object graphs, introduce coroutines, persist arbitrary heap values, support handles/files/processes/sidecars in checkpoints, or touch Chimera/Octxiliary/Rust SDK work.

## H1 implementation note: export-only interpreter checkpoints

Status: implemented for interpreter-owned export only. Restore, persistent `oct make` resume, checkpoint files, staleness changes, Make CLI flags, failure-artifact schema changes, compiled-flow checkpoints, and Octomata semantic changes remain deferred.

### API shape

H1 adds an interpreter logical checkpoint payload and export API in `internal/interpret`:

- `ExportFlowCheckpoint(instance *FlowRuntimeInstance, options FlowCheckpointOptions) (FlowCheckpoint, error)`;
- `(*FlowRuntimeInstance).ExportCheckpoint(options FlowCheckpointOptions) (FlowCheckpoint, error)` for in-package runtime owners;
- `RunFlowToSuspensionWithOptions(...) (SuspendedFlowRunResult, error)` for host callers that need to run a no-argument interpreted flow and then export a checkpoint without receiving a raw mutable instance;
- `SuspendedFlowRunResult.ExportCheckpoint(options FlowCheckpointOptions)`.

The existing `RunFlowToCompletionWithOptions` behavior is preserved and delegates through the new run helper.

### Exported payload

`FlowCheckpoint` version 1 contains:

- package name and flow name;
- a deterministic, non-security flow fingerprint;
- current state;
- top-level next-statement cursor with instruction index and state-body fingerprint;
- resume slot (`HasResumeTarget`, `ResumeTarget`);
- board checkpoint;
- state history;
- caller-provided or run-result step count.

The fingerprint is intentionally structural and cheap: package/flow identity, declared board fields and types, state names, top-level statement counts, and top-level statement Go AST node kinds. It is intended for future compatibility diagnostics, not tamper resistance.

### Safe subset and unsupported reasons

H1 exports only suspended, incomplete interpreter flows. It rejects unsupported shapes with `FlowCheckpointError` and a precise `FlowCheckpointUnsupportedReason`:

- `NotSuspended` for nil, unstarted, or otherwise not intentionally suspended instances;
- `CompletedCheckpointUnsupported` for completed flows;
- `StateMissing` when `CurrentState` is absent from the current declaration;
- `InstructionIndexOutOfRange` when the cursor is outside the current state's top-level body;
- `ResumeTargetMissing` when a populated resume slot names a missing state;
- `StateLocalsUnsupported` when `StateEnv` contains any user binding;
- `UtilityStateUnsupported` when controller-bound utility `when` state is non-empty;
- `BoardSnapshotUnsupported` for missing or malformed board bindings;
- `UnsupportedValueType` when a board value cannot be converted to the stable checkpoint value model.

The only ignored state binding is the runtime synthetic `__oct_flow_instance`. H1 does not serialize state locals and does not reset or serialize utility-controller state.

### Board value support

Boardless flows export an explicit empty board checkpoint. Boarded flows export declared board fields in declaration order with field names, static type strings, and stable values. H1 supports checkpoint values for:

- `Bool`;
- `String`;
- `Int`, including dimension metadata when present at runtime;
- `Float`, including dimension metadata when present at runtime;
- arrays recursively containing the same supported scalar values.

The checkpoint does not persist raw `Value` objects or interpreter object graphs. Current implementation and language fixtures permit arrays of scalar board fields; older prose that described board snapshots as scalar-only should be treated as stale relative to the current semantic contracts.

### Deferred work

Restore remains an interpreter-owned follow-up. Make still owns future checkpoint files, Make target staleness, CLI/user experience, and cleanup policy, but should consume this logical API rather than serializing trace-visible runtime fields. Compiled flow checkpointing remains a separate design because generated flow structs do not yet expose this logical checkpoint format.
