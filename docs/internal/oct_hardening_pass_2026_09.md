# Focused Oct hardening pass — 2026-09

## Outcome

Success. The external review was revalidated against current `main` at
`62251ec6` rather than applied as an implementation plan. Verified hardening
gaps were fixed without changing Oct language design.

## Revalidated findings

| Area | Current finding | Outcome |
| --- | --- | --- |
| H1 path/case correctness | The historical `Examples/` versus `examples/` collision was already fixed. Git has one canonical `Examples/` root. The repository guard validates case-folded path components and module-ZIP portability, and Ubuntu CI runs it. | No duplicate fix. Guard rerun and retained. |
| H2 security path validation | `ValidateDisplayPath` still used host `filepath` semantics, so Windows drive and UNC syntax could evade the predicate on Unix. | Fixed with host-independent syntax validation and a focused matrix. |
| H3 builtin authority | Reserved names/namespace aliases existed centrally, but regular call shape and canonicalization were still repeated across typecheck/interpreter/compiled lowering. | Added backend-neutral definitions and moved canonical alias and regular core call-shape authority into `internal/builtin`. |
| H4 interpreted/compiled conformance | Per-feature parity coverage existed, but there was no one bounded eligible-corpus differential lane. | Added a continuously run differential harness over six deterministic, strictly compiled-capable Language fixtures. |
| H5 panic leakage | Compiled expected failures still surfaced generated-Go panic headers and stacks, including missing sidecars and explicit unwrap failures. | Added an expected-diagnostic containment boundary to normal and compiled-test entry points. |
| H6 FLOW checkpoint determinism | Current code was substantially ahead of the review: versioned logical checkpoints, deterministic bytes, construction/board/yield/utility/resume/history state, restore validation, and compiled host continuation parity already existed. | Added the missing interpreter serialize/restore differential proof and a Language contract fixture. |
| H7 scalar-loop performance | The older approximately 3.4x measurement needed reproduction. | Current measured gap is 1.84x-1.89x; no speculative optimization was made. |

## Portability and security changes

`ValidateDisplayPath` now normalizes both slash spellings as data syntax and
rejects Unix roots, Windows drive prefixes (including drive-relative syntax),
UNC/rooted paths, parent traversal with either separator, and empty paths on
every host. Ordinary and nested relative paths remain accepted.

The original case-collision finding was already resolved by the earlier
OCT-HARDEN-M0 work. `go run ./tools/check_case_collisions` remains the bounded
repository check; it covers unignored paths and constructs a valid Go module
ZIP. No filesystem abstraction was added.

## Builtin semantic authority

`internal/builtin.Definition` now provides one backend-neutral place for:

- canonical spelling and namespace aliases;
- regular argument and type-argument counts;
- parameter-constraint vocabulary;
- return rules or an explicit semantic hook for irregular cases;
- fallibility and semantic/capability traits.

Typechecking, interpretation, and compiled lowering consume shared
canonicalization. The regular core/string/array subset also consumes shared
call-shape validation. Backend execution functions and Go emission remain in
their owning packages.

Structural tests require every reserved public builtin to resolve to one
definition, every namespace alias to resolve to its canonical definition,
every public definition to have typechecker coverage, and every builtin marked
interpreted/compiled-capable to have a corresponding implementation reference.
Irregular builtin typing remains in small typechecker hooks instead of being
forced into a lossy schema.

## Differential execution conformance

`TestConformanceEligibleCorpusHasInterpretedCompiledParity` runs each selected
Language fixture once interpreted and once strictly compiled, then compares
the structured test outcome. Its current eligible set covers:

- core pure builtins;
- core string builtins;
- array lowering;
- typed callbacks;
- ordinary expressions inside FLOW;
- deterministic FLOW checkpoint/resume semantics.

The lane reports each entry as compiled-capable, compiled-pass, parity-pass,
intentional-fallback, and unexpected-fallback. Strict compiled execution makes
any fallback in this corpus unexpected. Native-, environment-, timing-, and
nondeterminism-dependent suites remain outside this eligible subset and keep
their existing explicit auto/fallback reporting.

## Runtime diagnostic containment

Generated executable entry points now recover only recognized Oct diagnostic
panic strings (`runtime error:`, `oct error:`, and `unwrap failed:`), print the
diagnostic without Go substrate details, and exit nonzero. Generated test
harness entry points use the same boundary.

Go `runtime.Error` values and unknown panic payloads are re-panicked. This
preserves developer-visible stacks for implementation bugs instead of broadly
suppressing failures. Focused tests cover explicit unwrap failure and a missing
Octxiliary sidecar with assertions that `panic:` and `goroutine` do not leak.

## FLOW deterministic continuation evidence

The new Language fixture uses current semantics only: state/goto, nested
control, board mutation, deterministic `when policy`, suspend,
remember/resume, and final result. It passes interpreted and compiled execution
with zero fallback.

The interpreter host test compares:

1. execute to suspend and continue in memory; and
2. execute to the same suspend, export, JSON serialize, JSON deserialize,
   restore, and continue.

It requires identical observable run results, state history, resume state, and
board values. Existing compiled generated-facade tests already compare original
and restored next-turn results and boards and verify deterministic checkpoint
bytes. Ambient time, external effects, cryptographic randomness, and RNG state
not explicitly stored in persistent flow data remain outside the deterministic
subset.

## Performance reconnaissance

The reproducible tool and specimens are documented in
`docs/internal/compiled_scalar_loop_recon.md`. Three seven-sample runs measured
compiled Oct at 41.84-42.48 ms and hand-written Go at 22.34-23.02 ms for the
same 20M-iteration result, a 1.84x-1.89x gap.

Generated code contains one required explicit integer-to-float conversion, no
redundant guard branch in the loop body, and one general MIR program-counter
switch. Structured emission for reducible loops is the only visible lead; it
is not a trivial hardening fix and was not implemented.

## Design ideas parked, not accepted

`docs/internal/language_design_backlog.md` records dimension variables,
rational exponents, quantity kinds, template inference, derived-unit aliases,
capture consistency, deterministic replay positioning, `when utility`, stdlib
scope, and performance positioning. Every item records usefulness, tradeoffs,
and no current commitment. No language/product proposal from the review was
implemented.

## Verification

- `go test ./...`: pass.
- `go test -tags integration ./internal/build`: pass.
- `go vet ./...`: pass.
- `go run ./tools/check_case_collisions`: pass.
- portable safetensors path matrix: pass.
- builtin definition/alias/implementation coverage tests: pass.
- six-fixture interpreted/compiled differential conformance lane: pass.
- FLOW checkpoint serialization continuation differential: pass.
- new FLOW Language fixture, interpreted: 1 passed.
- new FLOW Language fixture, compiled: 1 passed, compiled 1, fallback 0.
- compiled missing-sidecar containment toolchain test: pass.
- `git diff --check`: pass.

## Intentionally deferred

- language-design and standard-library restructuring proposals listed in the
  backlog;
- making every irregular builtin declarative when a semantic hook is clearer;
- treating intentional compiled fallback as parity failure;
- parity comparison of nondeterministic stdout, native effects, timing, or
  environment-dependent artifacts;
- RNG checkpoint machinery not promised by current FLOW semantics;
- structured-loop emission, SSA, or any new optimization framework;
- new backend or broader compiled capability.

## Behavior changes

- Artifact display paths reject unsafe Windows syntax on non-Windows hosts and
  reject drive-relative paths such as `C:model.safetensors` everywhere.
- Expected compiled Oct failures now exit with a concise diagnostic instead of
  a generated-Go panic stack.
- Regular core builtin arity/type-argument diagnostics are sourced from shared
  metadata; invalid compiled calls may gain corrected singular/plural wording.
- No valid Oct program semantics, syntax, type-system rule, package layout, or
  backend capability changed.
