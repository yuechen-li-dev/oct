# Testing Guide

## Semantic Contracts

Language semantics are specified as:

- `.octest` for valid behavior
- `.octfail` for invalid/rejected behavior

These contracts live under `Language/` and are the canonical source for language behavior.

## Implementation and Integration Tests

Go-side tests should validate implementation and integration boundaries (CLI/runtime/backend mechanics), not duplicate language semantics already expressed in `Language/`.

## Practical Commands

In this environment, prefer:

- `go run ./cmd/oct test <path>` for contract execution

When the `oct` binary is available, `oct test <path>` is equivalent workflow intent.

## Test Placement Rules

- language behavior contract → `Language/`
- reusable package behavior → `Libraries/` package tests/contracts
- fixture/transitional input data → `testdata/`

Do not treat `testdata/` as the canonical owner of language semantics.
