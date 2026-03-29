# oct pkg

## Overview

`oct pkg` manages Oct package workflows.
Package manifests and dependency declarations are explicit.
Package commands define package state transitions.

## Rules

- `oct pkg` subcommands operate on package manifests and dependency state.
- Package manifests are explicit source records.
- Dependency identity is explicit by package name and version requirement.
- Package resolution and sync are command-driven.
- Package boundaries are explicit through `package` and `import` declarations.
- Package operations should be treated as reproducible build inputs.

## Examples

Valid:

```text
oct pkg <subcommand>
```

Invalid:

```text
# expecting imports to resolve without manifest or dependency updates
```
