# Language and execution support

[Reference home](README.md) · [Targets](tooling/targets.md)

This reference describes the implemented authoring surface, including unusual
and experimental features. Documentation alone does not promote a feature to
a release compatibility promise. The [1.0 surface manifest](../../docs/releases/OCT_1_0_SURFACE_MANIFEST.md)
and [1.1 release notes](../../docs/releases/OCT_1_1_RELEASE_NOTES.md) own those
release statements.

Milestone names such as M0/M1 and codes in contract directories identify the
history and bounded qualification of a feature. They are not import names,
language versions to select, or extra stages a learner must complete. The
chapter's stated current rules and limits are the authoring contract.

## Execution boundaries

| Surface | Intended path and limits |
| --- | --- |
| Ordinary Oct | Interpreter or native compiled Go path; exact feature contracts live under `Language/` |
| Native tests | `--execution compiled` requires native cases and forbids interpreter fallback |
| Auto tests | May fall back where supported; inspect the execution summary |
| Wrapper-backed operations | Require the documented sidecar/native prerequisites in the compiled path |
| Artifact publication | Explicit build-time typed interpreter; no application backend generation is implied |
| `Require` and refinements | Bounded checking/admission, with explicit runtime constructors for unknown refined values |
| `StaticAssert` | Evaluated under a compiler-owned static phase, currently artifact evaluation; not a replacement runtime assertion |
| Source-only generated Go | Experimental host integration; generated names and checkpoint encodings are not stable ABIs |
| WebAssembly | Direct MIR backend admitting a bounded scalar subset, with explicit rejection outside it |
| `profile Verilog` | Compile-only hardware subset, including bounded sequential FLOW; not a general software-to-hardware promise |
| SDSL-V / Prometheus | Separately governed GPU language and acceleration research; not ordinary Oct syntax or stable Oct conformance |
| Machina UI / UIBridge | Experimental authoring/host integration with an explicit protocol |
| Make / OctGen | Experimental explicit build/source-generation tools; no hidden execution on import |

The [coverage map](coverage.md) links every language-contract area to its
reference owner. A passing contract demonstrates its stated case and execution
lane. It does not establish all numerical, platform, native-library, or
performance properties of the containing feature.

## Things to write explicitly

Choose `let` unless rebinding is needed. Give scientific quantities units.
Name product values with concepts or records, and alternatives with enums.
Use fallible results for recoverable failure and options for normal absence.
Write captures and template type arguments explicitly.

There is no general dynamic value type, implicit truthiness, structural duck
typing, runtime reflection over types, trait search, user-defined operator
overloading, macro language, or arbitrary template-time VM. Ordinary loops
have no `break` or `continue`. Numeric widening is the bounded contextual
admission documented in [Types](language/types.md), not unrestricted coercion.

If one of these boundaries blocks a real program, preserve a reproducer and
record it in [FEEDBACK](../../FEEDBACK.md). Do not teach a brittle workaround
as the correct modeling pattern or silently describe a proposed feature as
implemented.
