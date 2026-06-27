# FLOW-CHECKPOINT-API-H2: interpreter restore from flow checkpoints

Status: implemented for the interpreter-owned H1 checkpoint subset. This does **not** implement checkpoint files, persistent `oct make` resume, Make staleness integration, CLI flags, compiled-flow checkpointing, failure-artifact schema changes, or Octomata language semantics changes.

## Restore API

H2 adds interpreter-owned restore entry points in `internal/interpret`:

```go
func InstantiateFlowFromCheckpoint(
    program project.Program,
    pkg string,
    flow string,
    checkpoint FlowCheckpoint,
    opts FlowRestoreOptions,
) (*FlowRuntimeInstance, error)

func RunFlowToCompletionFromCheckpointWithOptions(
    program project.Program,
    pkg string,
    flow string,
    checkpoint FlowCheckpoint,
    maxSteps int,
    stdout io.Writer,
    options ExecuteOptions,
) (FlowRunResult, error)
```

`InstantiateFlowFromCheckpoint` validates the logical H1 payload against the current source, creates a fresh interpreter flow instance, restores only the supported logical continuation state, and returns the instance to interpreter callers. It does not expose raw mutable interpreter internals to Make as a persistence format.

`RunFlowToCompletionFromCheckpointWithOptions` is a convenience helper that restores and then steps the flow with the same run-result shape used by fresh flow execution. `Steps` reports transitions executed by the resumed invocation. `StateHistory` is restored from the checkpoint and appended as the resumed flow transitions.

## Validation rules

Restore validates before execution:

- checkpoint version must equal the current `FlowCheckpointVersion`;
- checkpoint package and flow must match the requested package and flow;
- the current flow declaration must exist;
- H2 stays within the H1 Make subset: no flow parameters and an `Int` return shape;
- when present, the whole-flow structural fingerprint must match the current flow;
- the current state must exist;
- the cursor kind must be `top-level-statement-next`;
- the instruction index must identify a resumable next top-level statement in the current state body;
- when present, the state-body fingerprint must match the current state body;
- resume target must exist when `HasResumeTarget` is true;
- every `StateHistory` entry must reference a known state;
- board checkpoint schema must match the current board declaration by field name, order, and expected type string;
- board checkpoint values must match the declared board field types.

Validation uses `FlowCheckpointError` reason codes for incompatible checkpoints, including version, identity, fingerprint, state, cursor, board schema/value, resume target, and history failures.

## Board restore support

H2 restores the stable H1 checkpoint value model only:

- `Bool`;
- `String`;
- `Int`, including dimensioned `Int` where the checkpoint dimension string matches the declaration;
- `Float`, including dimensioned `Float` where the checkpoint dimension string matches the declaration;
- arrays of supported board values.

Restore instantiates the flow normally, locates the synthetic `board` binding, materializes checkpoint values into interpreter `Value` instances, assigns a new board record preserving declaration order, and clears dirty-board tracking. Boardless flows accept an explicit empty board checkpoint and reject non-empty board data.

## Cursor and continuation behavior

H2 restores:

- `CurrentState`;
- `InstructionIndex`;
- `HasResumeTarget` and `ResumeTarget`;
- `StateHistory`;
- board values;
- a safe reconstructed `StateEnv` containing only the synthetic flow instance binding.

The restored instance is incomplete, has a cleared result, and starts from the post-suspend next top-level statement recorded by H1. It does not re-execute the `suspend` statement and does not skip the next statement. Exact continuation tests cover board mutation before suspend, post-suspend mutation exactly once, return after resume, state-history append behavior, and resume-slot preservation.

## Deferred policy remains conservative

H2 keeps H1's conservative unsupported policy:

- user state-local serialization remains deferred;
- utility `when` controller state serialization remains deferred and must not be silently reset from an encoded checkpoint shape;
- compiled-flow checkpoint export/restore remains unsupported;
- Make checkpoint files, FlowTarget persistent resume, staleness integration, and CLI resume behavior remain deferred.
