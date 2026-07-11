# Compiled identifier hygiene and test artifact paths

## Outcome

Compiled Go emission separates source bindings from compiler-owned bindings.
Every function-local source binding (parameters, locals, and loop bindings) is
emitted as a deterministic `__oct_user_<id>` name. Compiler-owned bindings are
emitted only through `internalName(kind, id)`, currently including the state
machine program counter, temporaries, and batch items as
`__oct_internal_<kind>[_<id>]`.

Oct permits underscore-prefixed identifiers, including double underscores, so
the internal prefix is not a language reservation. The collision-proof
property comes from emitting source and internal symbols through distinct
identity paths, not from rejecting or specially treating source spellings.

## Code generation audit

| Generated category | Status | Policy |
| --- | --- | --- |
| State-machine program counter and dispatch updates | Required hotfix | `internalName(ProgramCounter, -1)` |
| Expression, call, result, error, loop, and matrix temporaries | Required hotfix | `internalName(Temporary, id)` |
| Batch worker item, forwarding bindings, and helper functions | Required hotfix | internal batch-item identity plus source-name captures; package helper name avoids a source function collision |
| Source parameters, locals, and loop bindings | Potentially colliding before this change | `sourceName` deterministic source-symbol namespace |
| Runtime helpers, helper types, generated entry functions, labels | Structurally separate/global or already Go-mangled | Existing `__oct*`, `fn_<package>_<function>`, and numeric switch cases; they do not share a function-local source namespace |

This is intentionally a narrow backend naming contract. It changes neither
Oct identifier validity nor interpreter semantics.

## Compiled test artifacts

Compiled test runners derive one owned pair of paths from their generated
runner source:

```text
<scope>/<case>.generated.go
<scope>/<case>.octbin[.exe]
```

The source is always a valid Go filename. The executable uses `.octbin` as
the logical compiled-test artifact kind, plus `.exe` on Windows. On Linux and
macOS it ends in `.octbin`.

`DeriveTestArtifactPaths` is the single path derivation point. It rejects an
impossible source/binary collision. The compiler removes an owned stale test
binary immediately before `go build`; a failed rebuild therefore cannot be
followed by execution of an older binary. After a successful build it verifies
that the expected output exists. The test scope owns cleanup; with
`OCT_KEEP_TEST_ARTIFACTS=1`, both the generated source and binary remain for
inspection and reruns receive a fresh owned scope.

Earlier `zz_*.octbin` naming was merely organizational. Correctness now comes
from explicit distinct artifact roles and paths, not a prefix.
