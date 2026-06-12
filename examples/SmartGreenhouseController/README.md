# SmartGreenhouseController (Oct v0.1 demo)

This example is a small, runnable greenhouse control demo showing multiple core Oct features in one place:

- SI units (`K`, `s`, `m`, `m*s^-1`)
- enums with payload + `match`
- immutable record updates with `with`
- `switch` expressions
- arrays vs vectors/matrices as distinct concepts
- `batch` mapping over arrays
- Octomata `flow`/`state` with scalar `board` fields, including `Float<K>` temperature state
- `when policy` with `hysteresis` and `min_commit`
- runtime flow inspection via `StateHistory` and `BoardSnapshot(machine)!`

## Run

```bash
go run ./cmd/oct test examples/SmartGreenhouseController
go run ./cmd/oct test examples/SmartGreenhouseController --execution compiled
go run ./cmd/oct test examples/SmartGreenhouseController --execution auto
```

## Notes

- Compiled mode is currently green for this example (`go run ./cmd/oct test examples/SmartGreenhouseController --execution compiled`).
- The flow keeps temperature as a dimensioned `Float<K>` board field; board snapshots preserve the unit-qualified scalar type instead of forcing manual unit stripping.
- Candidate selection in `when policy` is intentionally static; dynamic candidate sets are demonstrated elsewhere via `Libraries/Octomata` helpers.
