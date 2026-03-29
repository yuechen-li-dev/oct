# octest

## Overview

`oct test` executes Oct test contracts.
`.octest` files define accepted behavior.
`.octfail` files define required rejection behavior.
Together they are the executable language specification.

## Rules

- `.octest` files contain programs expected to parse, type-check, and run.
- `.octfail` files contain programs expected to fail with declared diagnostics.
- `oct test <path>` discovers and runs both `.octest` and `.octfail` files.
- A passing `.octfail` means rejection matched expectation.
- A failing `.octfail` means rejection was missing or mismatched.
- Language correctness is defined by passing test suites.
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
