# Oct 1.0 RC1 Readiness Audit

_Audit date: 2026-07-21. This is an implementation-backed RC1 assessment, not
a 1.0 release declaration._

## Authorities and inventory

| Area | Public contract / documentation authority | Implementation authority | Test authority | RC1 status |
| --- | --- | --- | --- | --- |
| Lexical syntax, types, expressions, conversions | `Language/reference/language/01-03` | `internal/lex`, `parse`, `typecheck`, `interpret`, `build` | `Language/` lexical/types/expressions corpus | Ready |
| Functions, bindings, control flow, errors | `04-06`, `14-15` | parser/typecheck/interpreter/compiler | `Language/Functions`, `ControlFlow`, `Errors` | Ready |
| Arrays, ranges, records, enums | `07`, `11-12` | same, plus `builtin` | `Language/Types`, `Language/Expressions` | Ready with documented limits |
| SI units and scientific linear algebra | `08`, `16`, `tensors.md` | `dimension`, typecheck, interpret, build | units/vector-matrix/tensor corpus | Ready with documented limits |
| Packages and manifests | `13`, tooling `33` | `project`, `pkgmgr`, CLI | package corpus and CLI integration tests | Ready with documented limits |
| Octomata | runtime `21-22` | AST/typecheck/interpreter/build | Octomata corpus, compiled boundary fixtures | Ready with documented limits |
| Builtins and standard libraries | `09`, `17` and library READMEs | `builtin`, `libraries`, sidecars | library/corpus and wrapper lanes | Ready interpreted; stable API list pending |
| GoOct run/build/test/fmt | tooling `31-35`, `docs/TESTING.md` | `run`, `build`, `tester`, `ocfmt`, CLI | CLI/build/integration tests | Compiled parity blocker |
| Installation/version/CI artifacts | README, `docs/RELEASE.md`, workflows | CLI/version/build scripts | CI plus local smoke | RC2 packaging proof required |

The repository contains 205 `.octest` and 299 `.octfail` files under
`Language/` at audit time. They are the semantic conformance source, rather
than Go unit tests that duplicate language behavior.

### Material findings

1. **DOC-001 (fixed, P1):** CLI reference said `oct build` emits `.octbin`,
   while `internal/build.OutputPath` and the compiled-support tracker establish
   native executable output. The reference now states the actual behavior.
2. **DOC-002 (fixed, P1):** the expression reference omitted the documented
   implementation behavior that `and`/`or` short-circuit. The rule is now
   explicit for both GoOct paths.
3. **COMP-001 (P0):** the current documented language accepts valid shapes for
   which the compiled backend remains partial (notably broad function values,
   artifact paths, wrapper/library paths, and selected flow expressions).
   `auto` fallback is useful development behavior, but cannot be the 1.0
   compiled compatibility promise. RC2 must either close a bounded set of
   core-language gaps or explicitly constrain the stable compiled promise with
   an owner-approved language/tooling boundary.
4. **API-001 (P1):** `17-standard-libraries.md` identifies practical modules,
   but no release manifest separates stable library APIs from experimental or
   wrapper-dependent APIs. RC2 needs that list; it does not require library
   expansion.
5. **PKG-001 (P1):** README/release instructions retain v0.1.0 installation
   and tag text. This is expected pre-GA, but RC2 must provide versioned 1.0
   installation/artifact commands and verify them without tagging.
6. **GATE-001 (P1):** `oct test Language` recursively finds the 299 invalid
   contracts but reports zero compiled cases, because `Language/` is a corpus
   container rather than one package root. It is not an aggregate valid-program
   conformance command. RC2 must enumerate/package the valid corpus targets or
   add a small authoritative release-gate driver; a green container invocation
   must not be presented as full compiled conformance.

## Blocker ledger

| ID | Severity | Surface | Evidence / impact | Criterion and disposition | Scope | RC2? |
| --- | --- | --- | --- | --- | --- | --- |
| COMP-001 | P0 | Compiled GoOct | `docs/COMPILED_SUPPORT.md` says accepted surfaces remain partial; CLI `auto` falls back | Valid programs cannot honestly be said to compile/run predictably. Define and prove the stable compiled surface, or close bounded core gaps. | Substantial | Yes |
| API-001 | P1 | Standard libraries | Reference names modules but does not version/classify APIs | Compatibility promise cannot yet be stated honestly. Publish stable/experimental manifest. | Bounded | Yes |
| PKG-001 | P1 | Install/release artifacts | README and `docs/RELEASE.md` are v0.1-oriented | Essential 1.0 artifacts/install path need a no-tag proof. Update during RC2 packaging. | Bounded | Yes |
| GATE-001 | P1 | Release conformance gate | `oct test Language --execution compiled` reports `compiled: 0`, despite valid `.octest` files below the container | A release gate must demonstrate the surface it claims. Add an explicit target manifest/driver. | Bounded | Yes |
| DOC-001 | P1 | Build artifact contract | Reference/implementation disagreement | Fixed in RC1 and protected by existing build artifact tests. | Tiny | No |
| DOC-002 | P1 | Logical evaluation | Omitted parity-critical semantic rule | Fixed in RC1; add/confirm corpus evidence in RC2 gate. | Tiny | No |
| EXP-001 | P2 | Experimental command/API enumeration | Prometheus command surface is not fully enumerated in CLI reference | Experimental exclusion makes this non-blocking; document when publicizing. | Tiny | No |
| FUT-001 | Deferred | Metaprogramming/generics/system completion | No concrete repository evidence makes these necessary for existing systems | Explicitly outside 1.0. | Substantial | No |

## RC1 conclusion

**Classification: MEANINGFUL PROGRESSION.** The audit establishes a written
contract, fixes two unambiguous documentation contradictions, and isolates the
one P0 release decision with repository evidence. **Recommendation: READY FOR
RC2 WITH OWNER DECISIONS.** The owner must choose whether 1.0 means complete
compiled parity for every stable language construct, or a precisely constrained
stable compiled subset with an explicit supported-program definition. A release
cannot responsibly leave that boundary implicit.
