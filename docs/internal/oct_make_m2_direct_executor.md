# `oct make` direct executor M0

## Synopsis

```text
oct make [target] [--file <path>] [--backend direct] [--list] [--dry-run] [--trace]
```

`--backend direct` is the only backend in M0. Other backend names fail clearly; Ninja is not implemented yet.

## Discovery

If `--file <path>` is provided, that file is loaded as the make source. Otherwise `oct make` walks upward from the current directory, preferring the nearest project root with `manifest.oct` and using `<project-root>/Make.oct`. If no make file is found, the command reports that the user should pass `--file <path>` or create `Make.oct` at the project root.

MAKE2 does not add manifest build schema. MAKE4 adds direct-backend FlowTarget execution without changing manifest schema.

## Plan schema

`Make.oct` defines `Plan() -> Make.Plan`:

```oct
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        CommandTargets: [
            Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["version"] Cwd: "" }
        ]
        FunctionTargets: []
        FlowTargets: []
        PhonyTargets: []
    }
}
```

The records live in the first-party `Make` library: `Plan`, `CommandTarget`, `FunctionTarget`, `FlowTarget`, and `PhonyTarget`. `FlowTarget` has `Name`, `Inputs`, `Outputs`, `Deps`, `Flow`, and positive `MaxSteps` fields.

## Target kinds

- `CommandTarget`: direct program plus args, inputs, outputs, deps, optional cwd. No shell string is accepted.
- `FunctionTarget`: named zero-argument function in `Make.oct`. It may be fallible; returned errors fail the target.
- `FlowTarget`: named zero-argument Octomata flow in `Make.oct`. The direct executor instantiates the flow, steps it until completion, suspension, runtime error, or `MaxSteps` exhaustion, and records flow evidence in `trace.octagon`. MAKE4 supports the narrow result convention `Int`: `0` succeeds and non-zero fails the target. A suspended flow fails clearly because persistent make-flow resume is not supported in MAKE4. Flow targets are for complex workflows such as configure/probe/retry/fallback/stage/test/package; ordinary file DAG actions should remain command or function targets.
- `PhonyTarget`: dependency-only target with no outputs and no action.

Function and flow targets are direct-backend concepts. Ninja is not implemented here and future Ninja lowering must not lower arbitrary flows; initial Ninja work should lower command targets only.

## Staleness

A command, function, or flow target runs when `Staleness.Always` is selected, it has no outputs, an output is missing, an input is newer than an output, or a dependency ran in the current invocation. Missing inputs fail. Phony targets always evaluate dependencies.

## Make authority

`oct make` automatically sets `OCT_MAKE_AUTHORITY=1` while evaluating the plan, function targets, and flow targets so Make sidecar APIs such as `Make.Exec` and `Make.WriteText` can run. `OCT_WRAPPER_PATH` is preserved for sidecar discovery. Ordinary `oct run` does not receive this authority.

## Trace

`--trace` writes `.octmake/trace.octagon` containing selected target, backend, make file, closure order, and target decisions. Flow decisions include the flow name, max steps, executed steps, final state when available, state history, result code, suspended flag, error text, and timing fields. Board snapshots are not required in make state/trace in MAKE4; state history is the primary flow evidence.
