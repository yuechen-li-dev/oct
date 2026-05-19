# ocfmt

## Overview

`oct fmt` is Oct's canonical formatter.
It supports explicit modes for human-readable source and compact LLM payload generation.
Formatting is syntax-validated (parse-backed), deterministic, and idempotent per mode.

## Rules

- Command form is `oct fmt <file-or-directory> [--mode readable|compact|en-llm] [--check]`.
- `readable` is the committed/review style.
- `compact` is a dense one-line-ish style for prompt payloads/handoff snippets.
- `en-llm` is preserved for compatibility and currently aliases `readable`.
- Running the formatter twice in the same mode produces no further changes.
- Arrow spellings are canonicalized to `->` in formatter output (even when input uses `=>`).
- Formatter preserves comments; source that fails parse is refused.

## Examples

```text
oct fmt Language/reference --mode readable
oct fmt Experiments/OctErgonomicsLab/M0 --mode compact
oct fmt Language/reference --mode readable --check
```
