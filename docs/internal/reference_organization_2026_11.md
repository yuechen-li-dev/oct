# Reference organization and coverage audit — November 2026

## Result

The reference now starts with five runnable lessons, followed by grouped
lookup tables for everyday language, scientific/domain programming,
decisions/behavior, and tooling. Thirty-four existing chapters were retitled
and renamed to descriptive paths; repository links and the typechecker
reference-consistency test were updated. The learning path deliberately uses
ordered filenames; lookup chapters no longer encode an obsolete reading order.

FLOW, reactive turns/checkpoints, one-shot utility, controller commitment, and
async have distinct owners. Artifact/static-phase and benchmark workflows,
Go/native interoperability, target subsets, and a complete library directory
are discoverable without interrupting the basic language lessons. UIBridge
protocol detail lives with specialized tooling. The old builtin split report
moved to implementation history, and redundant pointer files were removed.

## Coverage evidence

`python tools/check_reference.py` is read-only. The final inventory has:

- 52 Markdown pages reachable from the reference home;
- 736 checked local links, including incoming repository Markdown links;
- 7 full source blocks equal to canonical `.octest` files;
- all 50 current ordinary AST statement/expression forms assigned to a
  reference owner, explicitly identifying internal/rejected forms;
- all 117 current Oct contract areas assigned to a reference owner;
- all 64 current library directories linked through the directory.

These are navigation and ownership checks. They do not prove every program,
historical snippet, numerical routine, or execution target correct. The
coverage chapter records the precise inventory and the disagreements repaired
in the audit, including contextual numeric admission, mask filtering,
table-query support, templates, guard action blocks, scaffolding, and FFT.

## Executable validation

The seven sources under `Language/ReferenceExamples/valid` contain six Facts,
one artifact entry, and one benchmark entry. Validation passed:

```text
python tools/check_reference.py
go run ./cmd/oct fmt Language/ReferenceExamples/valid --check
go run ./cmd/oct test Language/ReferenceExamples/valid --execution interpreted --json
go run ./cmd/oct test Language/ReferenceExamples/valid --execution compiled --json
go run ./cmd/oct artifact Language/ReferenceExamples/valid --output-root .tmp/reference-output --json
go run ./cmd/oct bench Language/ReferenceExamples/valid --filter MeasureReferenceCalculation
go test ./internal/typecheck -run TestOctomataReferenceDocumentsPersistentBoardValues -count=1
git diff --check
```

All six Facts pass in each software lane; the compiled run reports six native
cases and zero fallback. Artifact evaluation publishes two outputs through the
build-time interpreter with zero backend generations. The benchmark runs
through the compiled runner; no timing performance claim is attached to this
documentation example. CLI help and manifest implementation were consulted to
correct application scaffolding, explicit paths, default directories, and
author/date guidance.

The full repository suite and native/GPU/slow-wrapper qualification were not
rerun for this documentation and fixture change. Production compiler/runtime
logic was not changed; the only Go edit updates a renamed reference path in an
existing test. Detailed feature contracts and existing limitations remain
authoritative; ownership coverage is not a new compatibility promise.
