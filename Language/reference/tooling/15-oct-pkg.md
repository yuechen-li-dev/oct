# oct pkg

## Overview

`oct pkg` manages Oct package workflows. Packages use explicit manifests and explicit dependency declarations. Package structure and commands are deterministic.

## Commands

- `oct pkg` subcommands operate on package manifests and package dependency state.

## Behavior

- Package manifests are explicit records in source.
- Dependency identity is explicit by package name and version requirement.
- Package resolution and sync behavior are command-driven, not implicit.
- Package boundaries are explicit via `package` and `import` declarations.

## Notes

- Keep package metadata and imports consistent.
- Treat package operations as reproducible build inputs.
