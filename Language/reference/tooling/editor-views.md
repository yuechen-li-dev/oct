# Editor views and canonical source

[Reference home](../README.md) · [Coverage](../coverage.md)

## Overview

Canonical Oct source is `en-llm`.
VSCode may render `en-human` as a view-only projection.
`en-human` does not introduce a second source format.

## Rules

- Source text on disk is canonical `en-llm`.
- `en-human` is editor presentation only.
- Save, git diff, and commit operate on canonical source.
- `oct fmt`, `oct build`, `oct run`, and `oct test` operate on canonical source.
- `en-human` does not change parser/typechecker/runtime semantics.
- The VSCode extension starts the language server (`oct lsp`) when `oct.lsp.enabled` is true (the default), using the executable named by `oct.lsp.path` (default `oct` on PATH). The server then owns diagnostics, formatting, semantic highlighting and the outline; the CLI-based formatter is used only when the server is disabled.

## Example

```text
Use "Oct: Toggle en-human View" in VSCode to switch display projection while keeping source canonical.
```
