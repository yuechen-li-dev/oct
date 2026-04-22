# ocfmt

## Overview

`oct fmt` is Oct's canonical formatter.
It produces one normalized source form.
Formatting is deterministic and idempotent.
Formatter profiles are not supported.

## Rules

- Command form is `oct fmt <file-or-directory>`.
- Formatting rules are non-configurable.
- Running the formatter twice produces no further changes.
- Canonical spacing and indentation are enforced.
- Declaration-adjacent comments are preserved.
- Arrow spellings are canonicalized to `->` in formatter output (even when input uses `=>`).
- Formatter output is the committed style.

## Examples

Valid:

```text
oct fmt Language/reference
```

Invalid:

```text
oct fmt --profile team-style Language/reference
```
