# Maintaining the reference

[Reference home](README.md) · [Coverage inventory](coverage.md)

## One readable owner per rule

The reference owns human-readable syntax, supported behavior, and preferred
style. `Language/` owns executable valid/invalid specimens. Compiler Go owns
implementation. Follow [AGENTS.md](../../AGENTS.md) when those disagree and
record a concrete gap instead of copying an old experiment's workaround.

Keep beginner lessons focused on an outcome and runnable as whole files. Keep
exact typing, evaluation order, and edge cases in the owning reference chapter.
Keep native protocols, backend subsets, and experimental tools in their explicit
advanced sections. Historical reports belong under `docs/internal`, including
the [earlier builtin split](../../docs/internal/reference_builtin_split_history.md).

## When adding or changing a feature

1. Update the owning chapter and its preferred example. Explain when the feature helps.
2. Add or update `.octest` / `.octfail` contracts in the semantic area.
3. Update the home index, glossary, and coverage owner where discovery changes.
4. Add a coverage row for a new AST form or contract area; do not invent semantics from an internal node name.
5. Run the documentation checker and the affected execution lanes.

The library directory includes experimental and usage packages as such. Add
new packages there without silently promising stability. Release classification
is separately owned by the [surface manifest](../../docs/releases/OCT_1_0_SURFACE_MANIFEST.md).

## Check navigation and runnable lessons

```text
python tools/check_reference.py
go run ./cmd/oct test Language/ReferenceExamples/valid --execution interpreted
go run ./cmd/oct test Language/ReferenceExamples/valid --execution compiled
go run ./cmd/oct artifact Language/ReferenceExamples/valid --output-root .tmp/reference-output --json
go run ./cmd/oct bench Language/ReferenceExamples/valid --filter MeasureReferenceCalculation
```

The Python checker is read-only and uses only the standard library. It checks
local links and Markdown anchors, chapter reachability, exact lesson-source
blocks, coverage of ordinary statement/expression node kinds, contract areas,
and library directories. It does not typecheck prose or prove language behavior;
the Oct commands do the executable checks. A new milestone needs its own
appropriate qualification beyond a successful documentation check.

Full-file lesson blocks have a `verified-example` comment pointing to their
canonical `.octest` source. Keep their text equal to that source, including
the package and test function. Declaration fragments elsewhere are identified
by context and are not asserted to be standalone programs.
