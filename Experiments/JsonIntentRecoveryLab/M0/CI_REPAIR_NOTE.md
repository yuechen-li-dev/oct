# CI Repair Note — IO.Json test boundary realignment (2026-04-23)

## What was incorrectly coupled

Production library tests in `Libraries/IO/IO.Json.octest` directly loaded corpus assets under `Experiments/JsonIntentRecoveryLab/M0/corpus/...`.

This made `oct test Libraries/IO` depend on experiment-owned paths and fail under normal library test execution roots.

## Boundary fix

- Kept production `IO.Json` tests in `Libraries/IO/IO.Json.octest`, but switched them to stable local fixtures in `Libraries/IO/testdata/`.
- Moved corpus-oriented validation ownership to an experiment-local test entrypoint: `Experiments/JsonIntentRecoveryLab/M0/corpus_validation.octest`.
- Added CLI wrapper coverage in `cmd/oct/m103c_json_intent_recovery_lab_test.go` so experiment corpus validation remains exercised from Go tests without recoupling production smoke tests.

## Stable production IO.Json smoke surface now covered

- Raw import + structured graph import on local fixture.
- Structured lowering for scalar/bool/null/object/array/string compatibility.
- Intent recovery smoke coverage for:
  - table-like (`smoke_people.json`)
  - mapping-like (`simple_mapping.json`)
  - config-like (`smoke_config.json`)
  - grid-like (`smoke_grid.json`)
- deterministic recovery and conservative ambiguous fallback checks.

## Experiment corpus validation location

`Experiments/JsonIntentRecoveryLab/M0/corpus_validation.octest`

Covers all M0 corpus files for load + normalize determinism and representative root-shape stability checks.

## Stale expectation updates

- Updated `Len(...)` CLI expectation strings from:
  - `String or array type`
  to:
  - `String, Bytes, or array type`
- Updated deterministic syntax-error expectation from legacy `expected '->' before return type` to parser’s current arrow-unified message (`expected arrow before return type ...`).

## Inconsistency surfaced explicitly

`Language/reference/language/09-builtins.md` still points to `Libraries/IO/IO.Json.octest` for corpus-oriented validation language that was historically experiment-heavy. After this repair, corpus validation ownership is now split with explicit experiment-local tests at `Experiments/JsonIntentRecoveryLab/M0/corpus_validation.octest`.
