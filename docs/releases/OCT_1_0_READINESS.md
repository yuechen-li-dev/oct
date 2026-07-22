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

## RC2 update — 2026-07-21

The owner selected complete compiled parity for every stable construct; a
partial stable compiled subset is no longer an acceptable release outcome.

### Closed, bounded backend defects

| ID | Disposition | Evidence |
| --- | --- | --- |
| COMP-001a | Fixed | Stable `batch` expressions in separate functions emitted duplicate worker helpers. `Language/Concurrency/Batch/valid/batch_valid.octest --execution compiled` now passes 7 native cases. |
| COMP-001b | Fixed | A literal zero divisor in an unselected `switch` arm was rejected by Go at compile time despite Oct lazy-arm semantics. The division lowering now defers evaluation; `ConditionSwitch` compiled lane passes 6 cases. |
| COMP-001c | Fixed | Void-returning flows did not force emission of `__octVoid`; `OctomataCompiledBoundary` compiled lane passes. |
| COMP-001d | Fixed | Explicit `return` in a fallible `Void` function emitted no result. Cross-package fallible compiled tests now pass 4 cases. |
| COMP-001e | Fixed | Generated wrapper test sources in OS temp could not import `internal/octxiliary`. A transient module-local compile staging source preserves external test artifacts while satisfying Go's internal-package rule; focused slow wrapper tests pass. |
| GATE-001 | Fixed | `tools/Test-Oct10Conformance.ps1` runs 15 selected stable source contracts in both modes; compiled mode requires native cases and zero fallback. Current compiled result: 52 cases, 0 fallback, 0 failed. |
| API-001 | Progressed | `OCT_1_0_SURFACE_MANIFEST.md` establishes the authoritative boundary, but two established library candidates remain release-blocked. |

### Remaining release blockers

| ID | Severity | Evidence | Required next action |
| --- | --- | --- | --- |
| COMP-002 | P0 | `Libraries/Optimization --execution compiled` has four compiled Gauss-Newton/Levenberg-Marquardt failures: generated row assignment rejects a `Float[]` replacement with `row length mismatch: expected 1, got 2`. | Reconcile compiled row assignment with the already-passing interpreted semantics, then add a focused `Language/` parity contract. |
| COMP-003 | P0 | `Libraries/Numerics` cannot load because `Numerics.Optimization.oct` contains `else if`, which the authoritative reference rejects. | Rewrite this existing library source to the documented `switch`/nested-block form, then run interpreted and compiled package lanes. |
| PKG-001 | P1 | README and release guidance remain v0.1-oriented; no archive/checksum/install candidate proof has been produced. | Complete release-shaped artifact and install documentation/testing after P0 closure. |

The strict core language gate is green, but the candidate cannot yet be called
an Oct 1.0 RC because two established standard-library candidates lack the
owner-required compiled parity.

### RC2 parity closure — 2026-07-21

| ID | Disposition | Evidence |
| --- | --- | --- |
| COMP-002 | Fixed | `JacobianCD` and `JtJ` now construct their first rows at the required shape rather than relying on interpreter-only row resizing. The authoritative array contract requires same-length whole-row replacement. `Libraries/Optimization` passes 40 interpreted and 40 compiled cases, with compiled fallback `0`. |
| COMP-003 | Fixed | `Numerics.Optimization.oct` now uses the documented `switch` expression and flow `when` control form; its duplicate test declaration was removed. This exposed and closed a bounded native-flow callback lowering defect: function-valued flow parameters now call the stored callback and constructor storage cannot collide with parameter names. `Libraries/Numerics` passes 44 interpreted and 44 compiled cases, with compiled fallback `0`. |
| COMP-004 | Fixed | The compiled flow constructor used `f` for both its local instance and a valid flow parameter, producing invalid Go. The constructor now uses a reserved local, and function-valued flow parameters lower to direct stored-callback invocation. |

The declared stable-library sweep now passes all 28 packages through native
execution with no fallback. The positive 1.0 conformance driver remains green
in both modes (15 targets; compiled: 52 native cases, 0 fallback), and the
separate negative diagnostic baseline passes 299 contracts.

**Current RC2 status:** stable compiled parity is closed for the declared
language and standard-library surface. **PKG-001 remains P1:** archive,
checksum, and ordinary binary-install proof has not yet been completed, so the
repository is not yet a GA candidate.

The current Windows development binary was smoke-tested with `oct version`,
`--help`, native `oct build`, and execution of the resulting executable. It
reports `oct dev`, so this is compiler-path evidence only, not release-artifact
or version-verification evidence.

### RC3 artifact and installation verification — 2026-07-22

**PKG-001: fixed.** The release scripts inject `1.0.0-rc.1`, bundle the
compiler runtime and sidecars, create named archives, and write a conventional
`checksums.sha256` manifest. The manifest was independently verified with
`Get-FileHash` and `sha256sum -c`.

| Artifact | Size | SHA-256 | Built / executed |
| --- | ---: | --- | --- |
| `oct-1.0.0-rc.1-windows-amd64.zip` | 52,210,046 bytes | `0162a38c8bffbb8df8e66f87b63f7e7c76f1f6e1fa0c342fbdc30d73d81f2a44` | Native Windows build and extracted execution |
| `oct-1.0.0-rc.1-linux-amd64.tar.gz` | 52,531,781 bytes | `a41ea9ef9e606ae20776c467981d248f03a1300f1b2c5df6d504034475386611` | Native WSL Linux build and extracted execution |

Each archive was inspected after fresh extraction: one `oct` executable,
13 sidecars, `LICENSE`, `INSTALL.md`, and `runtime/go.mod`, `runtime/go.sum`,
and `runtime/internal/octxiliary/protocol.go`; no absolute or traversal paths
were found. Windows and Linux installed candidates both report `oct
1.0.0-rc.1`, run an interpreted program, build and execute a native program,
run a compiled `.octest` with zero fallback, and format source outside the
repository. Linux verification found and fixed an output/source collision;
non-Windows `oct build Main.oct` now emits `Main.oct.out`.

The extracted Windows candidate also passes the 15-target compiled conformance
gate (52 compiled cases, zero fallback). `go test -count=1 ./internal/build
./cmd/oct` passes after the packaging/runtime changes, as does `git diff
--check`.

### GA freeze verification — 2026-07-22

**PKG-001 remains closed.** The final version-injected candidate was rebuilt
from the frozen local release-preparation tree. The final approval revision is
recorded in the GA completion report.
No tag, GitHub release, or external publication was created.

| Artifact | Size | SHA-256 | Build / execution evidence |
| --- | ---: | --- | --- |
| `oct-1.0.0-windows-amd64.zip` | 52,209,711 bytes | `f4d82bc06a1ab35831769f0488d43ca077be8863b4131ce8b97e8addc3863328` | Built and extracted/executed natively on Windows. |
| `oct-1.0.0-linux-amd64.tar.gz` | 50,650,441 bytes | `b85cb3df85b41947bab80d5a8f1a57eca0e6f6e441e6b4eb6402c970bd66cd9f` | Built and extracted/executed under native WSL Linux. |

The SHA-256 manifest was verified independently with PowerShell `Get-FileHash`
and Linux `sha256sum -c`. Archive inspections found no absolute/traversal paths,
one compiler executable per archive, and the required `LICENSE`, `INSTALL.md`,
runtime module, protocol source, and 13 sidecars. Fresh external installation
directories on both hosts reported `oct 1.0.0`; `help`, interpreted `run`,
native `build` and execution, compiled `test` (one native case, zero fallback),
and `fmt` all passed.

The extracted Windows artifact also passed the authoritative 1.0 conformance
driver: 15 interpreted targets and 15 compiled targets / 52 native cases, with
zero fallback and zero failures. Source release lanes passed: 299 negative
diagnostic contracts; `go test -count=1 -parallel 8 ./internal/... ./cmd/oct`;
the 28-package compiled library sweep with zero fallback; and the slow wrapper
toolchain lane with the bundled sidecar-equivalent path. The ordinary and
integration-tagged broad Go suites both pass. `git diff --check` passes. No
stable-language, packaging, or installation blocker remains.

**GA recommendation: READY FOR OCT-1.0 GA.** The remaining operation is
procedural: perform one final clean checkout verification at this revision,
recreate the two archives and manifest, obtain human approval, then create and
push `v1.0.0` and publish the verified files.
