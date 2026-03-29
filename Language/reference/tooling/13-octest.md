# octest

## Overview

`oct test` executes Oct-native test contracts. `.octest` files define valid behavior. `.octfail` files define expected rejection behavior. Together they are the executable specification for Oct behavior.

## Rules

- `.octest` contains programs expected to parse, type-check, and run.
- `.octfail` contains programs expected to fail with declared diagnostics.
- `oct test <path>` discovers and runs both file kinds.
- A passing `.octfail` means rejection matched expectation.
- A failing `.octfail` means missing or mismatched rejection.
- Behavior correctness is defined by passing test suites.
- Reference text must align with `oct test` outcomes.

## Examples

Valid:

```text
oct test Language/Expressions/Arithmetic/valid
```

Invalid:

```text
oct test path/that/omits-required-failure-expectations
```
