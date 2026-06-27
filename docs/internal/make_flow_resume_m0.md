# MAKE-FLOW-RESUME-M0 persistent FlowTarget resume

M0 persists intentionally suspended interpreted `Make.FlowTarget` runs at:

```text
<StateDir>/flows/<sanitized-target>/checkpoint.octagon
```

The sanitizer is shared with failure artifacts. The wrapper records the original target name and refuses resume when the original target or sanitized target no longer matches, so sanitized-name collisions do not silently resume the wrong target.

## Checkpoint wrapper

The Make-owned file is valid Octagon with top-level `MakeFlowCheckpointFile` and version `0`. It stores target metadata (`Target`, `SanitizedTarget`, `Flow`), UTC creation/update times, `MakeFile`, `MakeFileHash`, `FlowTargetHash`, ordered input/output `MakePathState` snapshots, ordered dependencies, and the nested interpreter-owned `FlowCheckpoint` payload.

`MakeFileHash` is SHA-256 over the loaded Make file. `FlowTargetHash` is SHA-256 over target kind/name, flow name, `MaxSteps`, and ordered inputs, outputs, and deps. Path snapshots use the existing timestamp model: path, exists bit, modified unix nanoseconds, and an empty content hash.

## Resume and staleness policy

A valid checkpoint means the flow target is incomplete. Checkpoint-aware staleness runs before ordinary up-to-date decisions and reports reason `CheckpointPresent`. It applies both to explicitly selected targets and to flow targets selected by dependency closure. Dependents are blocked while the flow suspends because suspension remains a failed/incomplete make outcome.

When a valid checkpoint is used, `oct make` prints:

```text
resuming FlowTarget <Target> from <CheckpointPath> at state <State>
```

Successful completion deletes the checkpoint and then normal state update proceeds.

## Invalidation policy

M0 invalidates and starts fresh when the checkpoint cannot be parsed, has an unsupported version, targets a different original/sanitized target, targets a different flow, observes a changed Make file hash, observes a changed flow target hash, observes changed input snapshots, has a dependency that reran earlier in this invocation, or fails interpreter restore validation.

Invalid checkpoints are not deleted in M0. If tracing is enabled, the decision records `CheckpointInvalidated` and `CheckpointInvalidationReason`. The CLI prints:

```text
checkpoint for FlowTarget <Target> invalidated: <Reason>; restarting flow
```

An invalid checkpoint by itself does not force a rerun if ordinary staleness says the target is up to date.

## Suspension and failures

Intentional suspension exports an interpreter checkpoint and writes/updates the Make checkpoint file. The process still exits nonzero. On success the CLI prints:

```text
flow suspended at state <State>; checkpoint written to <Path>; re-run oct make <Target> to resume
```

If interpreter export or checkpoint write is unsupported, no checkpoint is written and the failure artifact records `ResumeSupported: false` plus `CheckpointError`. Non-suspension failures and MaxSteps failures do not advance checkpoints; resumed failures preserve the previous checkpoint for inspection.

## Trace and failure fields

Flow trace decisions now include resume/checkpoint fields: `Resumed`, `CheckpointPath`, `ResumeState`, `PriorSteps`, per-invocation `Steps`, `TotalSteps`, `MaxSteps`, `StateHistory`, `CheckpointWritten`, `CheckpointDeleted`, `CheckpointInvalidated`, `CheckpointInvalidationReason`, `ResumeSupported`, and `CheckpointError`, while preserving `FinalState`, `ResultCode`, `Suspended`, and `SuspendedIntentionally`.

Suspension failure artifacts include `ResumeSupported`, `CheckpointPath`, `CheckpointError`, `CurrentState`, `FinalState`, `StateHistory`, `PriorSteps`, `StepCount`, `TotalSteps`, and `MaxSteps` evidence.

## Scope and future work

M0 depends on the H1/H2 interpreter APIs: `ExportCheckpoint`, `InstantiateFlowFromCheckpoint`, and resumed execution from an interpreter-owned checkpoint payload. There are no new flags yet: no `--restart-flow`, `--no-resume`, pruning, doctor checkpoint status, Ninja backend support, compiled-flow checkpointing, or checkpoint clean command.
