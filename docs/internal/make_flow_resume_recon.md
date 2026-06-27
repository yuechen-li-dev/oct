# Persistent `FlowTarget` resume recon

Status: design-only recon. This document does **not** implement persistent resume, checkpointing, new CLI flags, new Make APIs, new attributes, or schema changes in code.

## Summary recommendation

Persistent Make-flow resume should persist an intentionally suspended `Make.FlowTarget` as visible `oct make` state:

```text
<StateDir>/flows/<sanitized-target>/checkpoint.octagon
```

A valid checkpoint means the target is incomplete. It should make the target stale, block dependents, and resume automatically when the target is selected or required by dependency closure. Resume must be visible in stdout and trace evidence. Successful completion deletes the checkpoint. Only intentional suspension writes or advances a checkpoint in M0.

## Current implementation audit

### `Make.Plan` representation

`Libraries/Make/Make.Core.oct` defines `Plan.FlowTargets: FlowTarget[]`. `FlowTarget` contains exactly `Name`, `Inputs`, `Outputs`, `Deps`, `Flow`, and `MaxSteps`.

`internal/makecmd` mirrors that shape with Go structs:

- `Plan.FlowTargets []FlowTarget`
- `FlowTarget{Name, Inputs, Outputs, Deps, Flow, MaxSteps}`
- an internal `target` union where `flow *FlowTarget` marks a flow target.

The plan is obtained by loading/typechecking `Make.oct`, calling `Plan()`, and converting the returned Oct record into the Go `Plan` in `convertPlan`, `flows`, and `config`.

### Validation

`validate` checks global Make invariants and FlowTarget-specific M0 constraints:

- `Flow` must be non-empty.
- `MaxSteps` must be positive.
- target names must be non-empty and unique across command/function/flow/phony targets.
- all dependencies must refer to existing targets.
- the dependency graph must be acyclic via `closureAll`/`closure`.

The interpreter entry point performs additional FlowTarget runtime validation before execution:

- the named flow must exist in the loaded package.
- it must take zero parameters.
- it must return `Int`.

### Direct backend flow start

The direct backend computes dependency closure, iterates targets in dependency order, checks staleness, and runs stale targets. For a flow target it calls:

```go
interpret.RunFlowToCompletionWithOptions(program, program.Entry, ft.Flow, int(ft.MaxSteps), stdout, interpret.ExecuteOptions{})
```

That function creates a fresh interpreter, looks up the flow by package/name, instantiates a new flow instance with `instantiateFlow`, then repeatedly steps it until completion, suspension, error, or `MaxSteps` exhaustion. There is no resume entry point today; every direct backend FlowTarget starts from a fresh instance.

### Runtime object that represents a flow instance

The interpreter runtime object is `*interpret.FlowRuntimeInstance`. Its checkpoint-relevant fields are:

- `Decl ast.FlowDecl`
- `Package string`
- `RootEnv *environment`
- `StateEnv *environment`
- `CurrentState string`
- `HasResumeTarget bool`
- `ResumeTarget string`
- `InstructionIndex int`
- `Completed bool`
- `Result Value`
- `StateHistory []string`
- `UtilityWhenSites map[int]utilityWhenSiteState`
- `DirtyBoardFields map[string]struct{}`

The exact runtime object that would need to be serialized for bit-for-bit continuation is therefore the flow instance plus its mutable environments, current state, instruction index, resume slot, history, board binding, utility-when site state, dirty board tracking, completion/result state, and enough flow identity to reconnect it to current source.

M0 should **not** serialize the raw object graph. Instead, it should serialize a constrained logical checkpoint: target identity, flow identity, current state, board snapshot, state history, prior step count, and validity fingerprints. That implies a future implementation needs a reconstruction path from those scalars into a fresh `FlowRuntimeInstance`.

### Board, state, and history representation

A flow board is stored as the `board` binding in the flow instance root environment. It is an interpreter `ValueRecord` with type name `__flow_board_<FlowName>`, field map, and field order. Defaults are created in `instantiateFlow`.

`StateHistory` is stored directly on `FlowRuntimeInstance` as `[]string`. The current active state is `CurrentState`; on first step it is set to the flow entry state and appended to history, and every `goto` appends the new state. A `suspend` advances `InstructionIndex` and returns a suspended signal without appending a new state.

`flowRunResult` copies `StateHistory`, `CurrentState`, `Completed`, `Result`, and step count into `FlowRunResult`. `makecmd` then copies those fields into its trace/failure `decision`.

### BoardSnapshot path

There is already a first-class scalar snapshot path for Oct code: `BoardSnapshot(flowInstance)`.

- Typechecking synthesizes a `<FlowName>BoardSnapshot` record type for flows with boards.
- The interpreted builtin returns an Oct `ValueRecord` copied from the current board binding.
- The compiled backend generates a typed snapshot struct and `__octBoardSnapshot()` method.
- `.octagon` representability already exists through `WriteOctagon`/`LoadOctagon`, but Make does not currently call that path for FlowTarget checkpoints.

So yes: there is a `BoardSnapshot` concept and scalar snapshot value path, but no Make-owned persistent checkpoint writer/loader today.

### Board field constraints

Flow board fields are constrained at typecheck time by `resolveFlowBoardFieldType`. Current rules allow `Bool`, `String`, `Int`/dimensioned `Int`, `Float`/dimensioned `Float`, and arrays of those types. They reject vectors, matrices, named records/enums, errors, void, complex, range, UI, index, functions, flow instances, and other non-scalar types.

This means suspended flows should be checkpointable by construction for board values under the current board model, including arrays of scalar board fields. A future checkpoint writer should still validate every value before writing and fail loudly if runtime/value support diverges from the type rule.

### Can a flow be reconstructed from state name plus board snapshot?

Not completely under the current runtime API.

A future implementation can likely reconstruct the logical flow by instantiating a fresh flow, loading the board binding from `BoardSnapshot`, setting `CurrentState`, setting `StateHistory`, and clearing completion/result state. However, current suspension resumes at the **next statement after `suspend`**, because `stepFlowWithTransitionLimit` increments `InstructionIndex` before reporting suspension. Therefore `CurrentState` alone is insufficient for exact continuation. The checkpoint also needs either:

- the current `InstructionIndex`; or
- a restricted reconstruction contract that resumes at a state boundary only, which would change semantics for suspends that occur mid-state.

M0 must preserve FlowTarget semantics, so the design recommends persisting a small internal resume cursor, not just a state name. The public schema should expose `CurrentState`; an M0 implementation field such as `InstructionIndex` or `ResumeCursor` should be considered required implementation data even if users usually reason in state names.

Also, utility `when` state is stored separately in `UtilityWhenSites`. If a flow can suspend after utility decisions whose hysteresis/commit-age matters later, exact continuation requires persisting that state as well or restricting/diagnosing such flows. M0 should audit this before implementation and treat missing utility state as non-checkpointable rather than silently changing behavior.

### Data currently lost on suspension

When a FlowTarget intentionally suspends, `oct make` currently records only trace/failure/state evidence. It loses all live continuation data:

- board values;
- `InstructionIndex` within the state;
- resume slot (`HasResumeTarget`, `ResumeTarget`);
- utility-when site state;
- dirty board field tracking;
- interpreter environments and locals;
- any transient values from the suspended state body;
- the ability to continue in a later `oct make` process.

Trace/failure evidence keeps `FinalState`, `StateHistory`, `Steps`, `MaxSteps`, `Suspended`, `SuspendedIntentionally`, `ResultCode` when present, and error/message text, but this is diagnostic evidence rather than a resumable checkpoint.

### Trace and failure evidence today

`decision` carries flow fields `Flow`, `MaxSteps`, `Steps`, `FinalState`, `StateHistory`, `ResultCode`, `Suspended`, and `SuspendedIntentionally`.

`trace.octagon` writes those fields for flow decisions. Current semantics are:

- `Steps`: actual executed state-transition count in this invocation.
- `MaxSteps`: configured cap.
- `StateHistory`: observed state path.
- `ResultCode`: integer result when available.
- `Suspended`: whether the flow ended suspended.
- `SuspendedIntentionally`: currently equal to `Suspended` for FlowTarget suspension.

Failure artifacts are written under:

```text
<StateDir>/failures/<sanitized-target>/<run-id>/failure.octagon
```

The target sanitizer preserves ASCII letters, digits, `.`, `-`, and `_`, and replaces other characters with `_`. The original target name is stored inside the artifact.

Flow failure artifacts include target metadata, inputs, outputs, deps, `FinalState`, `StepCount`, `StateHistory`, `Suspended`, `SuspendedIntentionally`, `ResultCode`, error, decision reason, and duration.

Checkpoint schema should reuse the established field names where they mean the same thing: `Target`, `Flow`, `CurrentState`/`FinalState` as appropriate, `StateHistory`, `MaxSteps`, `ResultCode`, `Suspended`, `SuspendedIntentionally`, `Inputs`, `Outputs`, `Deps`, and path snapshot field names from `MakePathState` (`Path`, `Exists`, `ModifiedUnixNano`, `Hash`). Prefer `StepCount` or `PriorSteps` rather than overloading existing per-invocation `Steps`.

### `state.octagon` update

`maybeState` writes `<StateDir>/state.octagon` after non-dry-run execution. It marks each decision as `Succeeded`, `Failed`, or `Skipped`; flow targets additionally record `FinalState` and `ResultCode` when a decision exists. Path states for inputs and outputs use `MakePathState { Path, Exists, ModifiedUnixNano, Hash }` with empty `Hash`.

A suspended FlowTarget is currently a failed target, so it is not success and dependents do not proceed in the same invocation because execution returns immediately on failure.

### Staleness and dependency closure

Dependency closure is computed by DFS in target declaration order. Staleness currently uses:

1. `Config.Staleness == Always` for non-phony targets;
2. whether any dependency ran in the current invocation (`DependencyRan`);
3. phony targets always stale;
4. no outputs (`NoOutputs`);
5. missing output (`MissingOutput`);
6. missing input is an error;
7. command hash missing/changed for command targets;
8. input modified time newer than oldest output;
9. otherwise `UpToDate`.

Flow targets share the generic input/output/dependency staleness path. There is no checkpoint-aware stale reason today.

## Mental model

A suspended Make flow is not a successful target and not an ordinary crash. It is an intentionally paused state machine with enough persisted state to continue in a later `oct make` invocation.

Consequences:

- A suspended target must not be marked successful.
- Dependents must not run while the target remains suspended.
- In M0 the process exit should remain nonzero on suspension unless a future explicit `--allow-suspend` mode is designed.
- The message must be actionable: name the target, flow, state, and checkpoint path, and say to rerun `oct make` when the external condition is ready.
- Resume must be visible, not CMake-cache-style hidden state. Stdout, trace, and failure artifacts should say when a checkpoint was used or written.

## Checkpoint location

Recommended M0 path:

```text
<StateDir>/flows/<sanitized-target>/checkpoint.octagon
```

Example:

```text
.octmake/flows/BuildFlow/checkpoint.octagon
```

Use the per-flow directory rather than `<StateDir>/checkpoints/<sanitized-target>.octagon` because it leaves room for future per-flow artifacts such as validation reports, stale checkpoint tombstones, or event logs.

Rules:

- Target-name sanitization should match or reuse the existing failure artifact sanitizer.
- The original target name must be stored inside the checkpoint.
- The sanitized target name may be stored for diagnostics, but it is not authoritative.
- The checkpoint is owned by `oct make`; users should not manually edit it.
- The checkpoint should be valid Octagon and loadable by existing Octagon tooling.
- `StateDir` controls the root, including custom absolute or relative state dirs.

## M0 checkpoint schema

Recommended top-level shape:

```octagon
MakeFlowCheckpoint {
    Version: 0
    Target: "BuildFlow"
    SanitizedTarget: "BuildFlow"
    Flow: "BuildMachine"
    CurrentState: "WaitForApproval"
    ResumeCursor: FlowResumeCursor { InstructionIndex: 3 HasResumeTarget: false ResumeTarget: "" }
    Board: BuildMachineBoardSnapshot { GateSeen: false Count: 2 }
    StateHistory: ["Setup", "CheckGate", "WaitForApproval"]
    PriorSteps: 3
    CreatedAtUtc: "2026-06-27T12:00:00Z"
    UpdatedAtUtc: "2026-06-27T12:05:00Z"
    MakeFile: "Make.oct"
    MakeFileHash: "..."
    FlowTargetHash: "..."
    Inputs: [
        InputSnapshot { Path: "gate.txt" Exists: true ModifiedUnixNano: 1790000000000000000 Hash: "" }
    ]
    Outputs: [
        OutputSnapshot { Path: "out.txt" Exists: false ModifiedUnixNano: 0 Hash: "" }
    ]
    Deps: ["Prepare"]
}
```

Required fields:

- `Version`: checkpoint schema version; M0 is `0`.
- `Target`: original target name.
- `SanitizedTarget`: diagnostic/path hint.
- `Flow`: named Octomata flow.
- `CurrentState`: active state at suspension.
- `ResumeCursor`: internal continuation data needed to avoid changing suspend semantics. At minimum this needs `InstructionIndex`, `HasResumeTarget`, and `ResumeTarget`; utility state may also be needed if used by the flow.
- `Board`: typed `<FlowName>BoardSnapshot` record, or an explicit empty board marker for boardless flows.
- `StateHistory`: full path before suspension.
- `PriorSteps`: step count already executed before the next invocation.
- `CreatedAtUtc` and `UpdatedAtUtc`.
- validity fingerprints: `MakeFile`, `MakeFileHash`, `FlowTargetHash`, and input/dependency snapshots.

Field naming should align with existing evidence: use `StateHistory`, `MaxSteps`, `Target`, `Flow`, `Inputs`, `Outputs`, `Deps`, `Path`, `Exists`, `ModifiedUnixNano`, and `Hash` where applicable. Use `PriorSteps` in checkpoints because `Steps` already means steps executed in one invocation.

## Checkpointability

M0 should checkpoint only intentional suspension.

Checkpointable values:

- scalar board fields and arrays of scalar board fields already allowed by `resolveFlowBoardFieldType`;
- current state name;
- state history strings;
- integer step counts;
- input/output path snapshots;
- small internal resume cursor fields.

Not checkpointable in M0:

- handles;
- files/processes;
- closures/functions;
- flow instances inside board fields;
- sidecar references;
- arbitrary heap values;
- matrices/vectors/complex/ranges/UI/index values;
- utility-when state unless a concrete serializer is added.

Do not silently drop board or cursor fields. If a suspended board or continuation contains a non-checkpointable value, checkpoint writing should fail clearly. The failure artifact should say `ResumeSupported: false` and include a checkpoint error explaining why persistent resume is unavailable for that suspension.

Given the current board type rule, board values are checkpointable by construction. Exact flow continuation as a whole is not guaranteed by board snapshot alone because of `InstructionIndex`, resume slot, and possible utility-when state; those must be serialized or diagnosed.

## Resume behavior

M0 policy:

- If a valid checkpoint exists for the selected or dependency-required FlowTarget, automatically resume from it.
- If no valid checkpoint exists, start a new flow instance normally.
- Print a clear one-line resume message, for example:

  ```text
  resuming FlowTarget BuildFlow from .octmake/flows/BuildFlow/checkpoint.octagon at state WaitForApproval
  ```

- On successful completion, delete the checkpoint and then update normal target state.
- On intentional suspension, write or update the checkpoint.
- On non-suspension failure, max-steps failure, or runtime error, do not create or advance a resumable checkpoint.
- If a resumed run fails for a non-suspension reason, preserve the previous checkpoint for inspection but do not claim it advanced. Trace/failure evidence should make clear that the checkpoint was not updated.

This simple policy minimizes surprising mutation and avoids turning crashes into resumable state.

## Invalidation rules

A checkpoint is invalid if any of the following are true:

- checkpoint version unsupported;
- `Target` differs from the current target;
- `Flow` differs from the current FlowTarget flow;
- `Make.oct` content hash changed;
- FlowTarget metadata hash changed;
- declared input snapshot changed;
- dependency target reran before the flow target in the current invocation;
- dependency set changed;
- current flow no longer contains the checkpoint state;
- current board schema no longer matches the checkpoint board;
- resume cursor is not compatible with the current state body;
- explicit future restart/clean control requested.

M0 fingerprints:

- `MakeFileHash`: content hash of the loaded make file. M0 should invalidate/restart rather than resume from a different source universe.
- `FlowTargetHash`: deterministic hash of target kind, target name, `Flow`, `MaxSteps`, `Inputs`, `Outputs`, and `Deps`.
- `InputSnapshots`: existence plus modified time, matching current timestamp staleness. Content hash can remain future work unless it becomes cheap through existing state.

`FlowTargetHash` should not include `StateDir`; moving the state directory changes where checkpoints live, not target semantics. It should not include `Config.Profile` unless profile later changes target semantics beyond selecting different plan content. The full `MakeFileHash` should remain separate from `FlowTargetHash`.

Dependency interaction:

- If any dependency ran before the flow target, invalidate the checkpoint and start fresh.
- `run` already has a `ran` map used for `DependencyRan`, so the information should be available at the point the flow starts.
- If implementation cannot safely distinguish dependency reruns in an edge case, use a conservative fallback: invalidate rather than resume.

Invalidation behavior should be visible. Trace should record `CheckpointInvalidated: true` and `CheckpointInvalidationReason`. CLI output should say it is restarting the FlowTarget when invalidating a checkpoint.

## Trace and failure artifact integration

Future trace fields for flow decisions:

```text
Resumed: true/false
CheckpointPath: "..."
ResumeState: "WaitForApproval"
PriorSteps: 3
Steps: 2
TotalSteps: 5
MaxSteps: 100
StateHistory: ["Setup", "CheckGate", "WaitForApproval", "Build", "Done"]
CheckpointWritten: true/false
CheckpointInvalidated: true/false
CheckpointInvalidationReason: "InputChanged"
```

Semantics:

- `Steps`: steps executed in this invocation only.
- `PriorSteps`: steps already present in the checkpoint before this invocation.
- `TotalSteps`: `PriorSteps + Steps`.
- `MaxSteps`: configured cap for this invocation, not lifetime, unless a later design explicitly changes it.
- `StateHistory`: full combined state history, including prior and current states. This keeps decision-path evidence first-class and avoids making users join two fields for the common case.

Failure artifact fields on intentional suspension:

```text
Suspended: true
SuspendedIntentionally: true
ResumeSupported: true
CheckpointPath: "..."
CurrentState: "WaitForApproval"
StateHistory: [...]
```

If checkpoint writing fails:

```text
ResumeSupported: false
CheckpointError: "board field X is not checkpointable"
```

Existing failure artifact behavior should otherwise remain unchanged.

## CLI behavior and controls

M0 should require no new CLI flag for basic behavior:

- automatic resume when a valid checkpoint exists;
- visible stdout message on resume;
- visible stdout message on checkpoint invalidation/restart;
- trace field `Resumed: true` when resume occurs.

Future controls, not part of M0:

```sh
oct make <target> --restart-flow
oct make <target> --no-resume
oct make clean-checkpoints
oct make doctor
```

Automatic resume can surprise users if it is silent. Mitigations are visible CLI messages, trace evidence, future doctor reporting, and conservative invalidation on Make file, target metadata, input, or dependency changes.

## State and staleness interaction

Recommended answers:

1. If a FlowTarget is otherwise up-to-date but has a valid checkpoint, the checkpoint wins. The target is incomplete and should be considered stale.
2. If outputs are already up-to-date but a checkpoint exists, resume rather than skip, because a checkpoint means the prior target action did not complete successfully.
3. If the target is selected explicitly and has a valid checkpoint, resume even if outputs exist.
4. If the target is required as a dependency and has a valid checkpoint, resume it; if it suspends again, block dependents and exit nonzero.

Implementation implication: staleness needs a checkpoint-aware reason such as `CheckpointPresent` before generic output freshness returns `UpToDate`. Successful flow completion should update `state.octagon` normally and delete the checkpoint.

## Cleanup behavior

M0 cleanup:

- success deletes the checkpoint;
- intentional suspension writes/updates the checkpoint;
- non-suspension failure leaves any previous checkpoint untouched for inspection;
- existing user-defined clean functions may remove `StateDir` manually;
- no built-in `clean-checkpoints` command yet.

Future cleanup:

- `oct make clean-checkpoints`;
- `oct make doctor` reports checkpoint presence, state, validity, and age;
- failure artifact pruning may eventually prune stale checkpoints, but only with explicit care because checkpoints are live build state.

## Implementation plan

1. Add Make-owned checkpoint path helpers reusing target sanitization.
2. Add deterministic `flowTargetHash` for kind/name/flow/max-steps/inputs/outputs/deps.
3. Add Make file hashing.
4. Add input snapshot collection using current path state fields.
5. Add checkpoint load/validate logic before flow staleness can return `UpToDate`.
6. Add interpreter support to instantiate a flow from logical checkpoint data without changing ordinary `RunFlowToCompletionWithOptions` semantics.
7. Add board snapshot serialization/deserialization using the existing `BoardSnapshot` record shape and `.octagon` representability rules.
8. Add resume cursor serialization for `InstructionIndex`, resume slot, and any required utility-when state; diagnose unsupported cursor state.
9. Add checkpoint write/update on intentional suspension only.
10. Add checkpoint delete on success.
11. Extend trace/failure evidence with resume/checkpoint fields.
12. Keep process exit nonzero on suspension in M0.

## Future test plan

1. Flow suspends and writes checkpoint.
2. Rerun resumes from checkpoint and completes after condition changes.
3. Success deletes checkpoint.
4. Checkpoint invalidates when `Make.oct` changes.
5. Checkpoint invalidates when FlowTarget metadata changes.
6. Checkpoint invalidates when input modified time changes.
7. Dependents do not run while checkpoint target remains suspended.
8. Trace records resumed/prior/current/total steps correctly.
9. Failure artifact links checkpoint path.
10. Non-suspension failure does not advance checkpoint.
11. Non-checkpointable continuation state fails clearly if such a state is possible.
12. Custom `StateDir` controls checkpoint path.
13. A checkpoint for one target does not resume a different target with the same sanitized name collision; original `Target` must be checked.
14. Board schema change invalidates the checkpoint.
15. State body/cursor incompatibility invalidates the checkpoint.

## Deferred work

- Checkpoint pruning and retention policy.
- `--restart-flow` and `--no-resume` controls.
- `clean-checkpoints` command.
- `doctor` checkpoint status reporting.
- Content-hash input snapshots.
- Rich checkpoint validation reports.
- Cross-backend/Ninja lowering, if FlowTarget ever gains non-direct backend support.
- Any Chimera, Octxiliary, Rust SDK, native toolchain, or wrapper changes.

## No behavior changed

This recon is documentation only. It intentionally does not implement persistent resume, checkpoint writing/loading, interpreter reconstruction, trace schema changes, failure artifact schema changes, CLI flags, Make APIs, attributes, or tests.
