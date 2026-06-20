# `oct make` direct executor M0

## Synopsis

```text
oct make [target] [--file <path>] [--backend direct] [--list] [--dry-run] [--trace]
```

`--backend direct` is the only backend in M0. Other backend names fail clearly; Ninja is not implemented yet.

## Discovery

If `--file <path>` is provided, that file is loaded as the make source. Otherwise `oct make` walks upward from the current directory, preferring the nearest project root with `manifest.oct` and using `<project-root>/Make.oct`. If no make file is found, the command reports that the user should pass `--file <path>` or create `Make.oct` at the project root.

MAKE2 does not add manifest build schema.

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
        PhonyTargets: []
    }
}
```

The M0 records live in the first-party `Make` library: `Plan`, `CommandTarget`, `FunctionTarget`, and `PhonyTarget`.

## Target kinds

- `CommandTarget`: direct program plus args, inputs, outputs, deps, optional cwd. No shell string is accepted.
- `FunctionTarget`: named zero-argument function in `Make.oct`. It may be fallible; returned errors fail the target.
- `PhonyTarget`: dependency-only target with no outputs and no action.

Flow targets are deferred. Function and flow targets are direct-backend concepts and are not lowerable to Ninja in this milestone.

## Staleness

A command or function target runs when it has no outputs, an output is missing, an input is newer than an output, or a dependency ran in the current invocation. Missing inputs fail in M0. Phony targets always evaluate dependencies.

## Make authority

`oct make` automatically sets `OCT_MAKE_AUTHORITY=1` while evaluating the plan and function targets so Make sidecar APIs such as `Make.Exec` and `Make.WriteText` can run. `OCT_WRAPPER_PATH` is preserved for sidecar discovery. Ordinary `oct run` does not receive this authority.

## Trace

`--trace` writes `.octmake/trace.octagon` containing selected target, backend, make file, closure order, and target decisions. The M0 trace is valid Octagon-shaped text but intentionally minimal.
