# ocfmt

## Overview

`oct fmt` is Oct's canonical formatter. It produces a single normalized source shape. Formatting is deterministic and idempotent. No formatter profile system is supported.

## Commands

- `oct fmt <file-or-directory>`

## Behavior

- Formatting rules are non-configurable.
- Running formatter twice produces no further changes.
- Canonical spacing and indentation are enforced.
- Declaration-adjacent comments are preserved.

## Notes

- Use formatter output as the committed style.
- Do not maintain alternate local style variants.
