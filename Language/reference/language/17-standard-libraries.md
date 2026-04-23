# Standard Libraries

## Overview

Oct standard libraries are the intended practical API surface for most programs.
Use these modules before reaching for low-level wrapper builtins directly.

This page summarizes the standard-library surface and its ownership boundaries.
For core language/runtime builtins, see [09 builtins](./09-builtins.md).
For Prometheus experimental APIs, see [23 Prometheus](../runtime/23-prometheus.md).

## Core practical modules

Common modules in the standard-library path include:

- `IO.File`
- `IO.Path`
- `IO.Directory`
- `IO.Json`
- `IO.Csv`
- `IO.Xlsx`
- `Archive.Zip`
- `Compression.Gzip`
- `Hash.Core`
- `Text.Regex`
- `Time.Core`

Module source locations (canonical in-repo docs/tests live with library code):

- `Libraries/IO/`
- `Libraries/Archive/`
- `Libraries/Compression/`
- `Libraries/Hash/`
- `Libraries/Text/`
- `Libraries/Time/`

## Usage posture

- Prefer module functions from `Libraries/*` for day-to-day application code.
- Treat direct wrapper builtin calls as low-level boundary tools.
- Keep business logic in Oct library/module code, not in builtin-specific glue.

## Backend support builtins (implementation detail)

Some builtins exist primarily to support standard-library modules and wrapper boundaries.
Examples include file/path/directory/json/csv/zip/gzip/hash/regex/time/xlsx/plotting-oriented builtins.

These are valid runtime primitives, but they are not the primary user-level programming story.
The primary user-facing story is the module layer (`IO.*`, `Archive.*`, `Compression.*`, `Hash.*`, `Text.*`, `Time.*`).

## Notes on current documentation boundaries

- The builtin reference intentionally no longer carries the full wrapper catalog; that content is conceptually owned by this page.
- If a library module exists in `Libraries/` but lacks matching detailed reference coverage under `Language/reference`, treat that as a documentation gap to close incrementally.
