# AGENTS.md

## Quick field manual

Before editing, read the smallest relevant slice of the repo truth:

- `README.md` for user-facing workflow and release positioning.
- First-party `manifest.oct` files should include ordered `Authors: String[]` and ISO `Date: String` metadata; preserve explicit author policy when updating package manifests.
- `Language/reference/...` for Oct syntax, style, and supported language features.
- Relevant `Language/...` fixtures; they are examples **and** semantic contracts.
- Relevant `docs/internal/...` notes for release, package-manager, wrapper, or compiled-backend work.

Testing guidance:

- Language behavior belongs in `.octest` / `.octfail` corpus tests under `Language/`; prefer adding or updating those contracts over embedding Oct programs in Go tests.
- If you change Go code, run targeted Go tests plus the relevant Oct language/library corpus lane.
- If you only change docs, fixtures, or Oct examples, a full `go test ./...` is usually unnecessary; run the relevant `oct test` corpus/examples instead.
- Do not force tests to interpreted mode to hide compiled failures. Do not weaken no-fallback or missing-sidecar assertions. If compiled support is absent, document that explicitly.

Octxiliary/wrapper workflow:

- Build sidecars explicitly with `go run ./tools/build_sidecars --out dist/sidecars`.
- Run slow wrapper lanes explicitly with `OCT_WRAPPER_PATH` and `OCT_SLOW_TESTS=1` (or the legacy `OCT_RUN_SLOW_TESTS=1`).
- Normal feature PRs should avoid broad slow wrapper lanes unless wrapper/octxiliary code changed or the lane was requested.
- Keep generated and scratch artifacts out of commits: do not commit local `dist/`, temporary outputs, `.oct/` caches, or test artifacts.

Release hygiene:

- Prefer small surgical commits.
- If touching release/tagging, follow `docs/internal/release_readiness_0_1.md`.
- Do not create or push `v0.1.0` unless the release checklist is green and a human explicitly approves tagging.


## Language Reference Authority Rule (Oct Code)

* The `Language/reference` directory is the **single source of truth** for:

  * syntax
  * style conventions
  * supported language features

* When writing or modifying Oct code, **always follow `Language/reference`**, even if:

  * existing experiments
  * older library code
  * prior generated code

  use different patterns.

* Treat older code as potentially outdated. Do **not** cargo-cult:

  * deprecated syntax
  * superseded patterns (e.g., while-loops used as for-loops)
  * pre-fix workarounds

* If you encounter inconsistencies between:

  * `Language/reference`
  * existing code
  * experiments
  * tests

  you must:

  1. **Follow `Language/reference`**
  2. **Explicitly surface the inconsistency** in your report or summary

* If a feature appears in code but is **not documented** in `Language/reference`, or vice versa:

  * treat this as a documentation gap
  * call it out explicitly

* Do not silently resolve inconsistencies.
  Always make them visible so they can be corrected intentionally.

## Primer Rule (Native Code)

Read `primer/` before writing or editing native code (C, C++, Vulkan, etc.).

The files in `primer/` are the authoritative coding rules for native code in this repository.
Do not write native code that conflicts with them.
Do not substitute your own preferred style for the primer rules.
Do not skip testing. If Vulkan is not availble inside sandbox, attempt to download Vulkan from apt. Surface issue explicitly if testing is truly impossible. 

If instructions and primers appear to disagree, surface the conflict explicitly.

## Convergence rule

Every substantial task must end in exactly one of three states:

1. **Success**  
The intended capability works in the real path and the real motivating case materially improves.
2. **Meaningful progression**  
The capability is not complete, but one genuine blocker is removed and the next blocker is isolated with evidence.
3. **Honest stop**  
Further work would require overbroad scope expansion, excessive debt, brittle patching, or tangled logic. Stop and report the reason with concrete evidence.

Do not continue producing patches once the work stops converging.

Do not confuse activity with progress.
A failed attempt is only acceptable if it leaves behind a narrower problem, stronger evidence, or a justified stop.

Any partial work must leave the codebase in a cleaner, more legible, and more diagnosable state than before.

## Judgment Rule

When a compiler/tooling decision has:
- one selected answer,
- a bounded set of valid candidate choices,
- multiple competing input signals,
- no single signal that should always dominate,
- and a need for deterministic/debuggable behavior,

prefer internal/judgment over fragile nested if/else ladders.

Examples:
- formatter layout choice:
  inline vs multiline vs leaveUnchanged

- diagnostic suggestion choice:
  unknown function vs missing import vs wrong namespace vs typo suggestion

- import/path recovery:
  candidate package roots or fixture paths

- artifact/report presentation:
  whether to emit table, key-value table, callout, or compact summary

- future geometry/recovery-style heuristics if mirrored in Oct tooling:
  candidate interpretation selection from partial evidence

Use direct if/else only when:
- the decision is truly binary and rule-based,
- one condition is semantically authoritative,
- ordering is obvious and not heuristic,
- or the choice is not worth tracing.

Judgment pattern:
1. Generate bounded candidates.
2. Mark unsafe/impossible candidates ineligible with explicit reasons.
3. Add named considerations with weights.
4. Score candidates deterministically.
5. Use deterministic tie-breaks.
6. Preserve a trace where the decision may be inspected later.

The goal is not “AI vibes.” The goal is deterministic, inspectable ambiguity resolution.

Avoid this shape for multi-signal candidate selection:

if width > maxWidth {
    return multiline
}
if hasNestedCall && isMarkdown {
    return multiline
}
if hasComment {
    return leaveUnchanged
}
...

Prefer:

candidates:
- inline
- multiline
- leaveUnchanged

considerations:
- widthFit
- nestingReadability
- heavyCalleePreference
- commentSafety
- diffStability

Then let internal/judgment choose and record why.

This mirrors Oct’s language-level `when utility` concept in the Go implementation layer. Oct is not self-hosted, so compiler/tooling decisions need a Go-native utility primitive instead of relying on Oct-layer code.

## Separation of Concerns (Anti Self-Host) Rule

Oct has a strict, non-negotiable separation of concerns:

> Go implements the Oct language.  
> Oct expresses programs, tests, and language contracts.

Do not blur this boundary.

\---

### Mental Model

Think of the repository as two layers:

* **Go = compiler/runtime (implementation layer)**
* **Oct = user space (programs and contracts)**

Go defines how Oct works.  
Oct is never used to define how Oct works.

\---

### 1\. Do not embed Oct inside Go

Writing Oct programs as strings inside Go code is almost always a bug.

❌ Forbidden:

```go
src := `
fn main() {
    return 42
}
`
````

✅ Correct:

* Place code in `.octest` or `.octfail`
* Execute tests via `go run ./cmd/oct test ...`, or `oct test ...` when the oct binary is available.

#### Exception (narrow and explicit)

Embedded Oct in Go is allowed only for **host-side implementation validation** where Oct cannot be the sole validator without circularity.

This includes:

* parser and lexer validation
* typechecker validation
* runtime/compiler boundary checks

This exception must remain:

* small in scope
* focused on implementation correctness
* not user-facing

It does **not** allow:

* duplicating language semantics already expressed in `Language/`
* writing general program tests in Go

\---

### 2\. Do not implement Oct in Oct

Oct is **100% implemented in Go and 0% implemented in Oct**.

Do not:

* implement evaluation, typing, or execution logic in Oct
* introduce meta-programming layers to simulate language features
* build helper runtimes in Oct

If behavior belongs to the language, it belongs in Go.

\---

### 3\. Language semantics belong in Oct tests, not Go

All user-visible behavior must be expressed as:

* `.octest` (valid behavior)
* `.octfail` (invalid/rejected behavior)

These live under:

* `Language/`

❌ Forbidden:

* encoding language semantics in Go test assertions
* using embedded Oct in Go to define behavior

\---

### 4\. Do not duplicate semantics across Go and Language/

If a language contract exists in `Language/`, it must not be re-expressed in Go tests.

Go tests must not:

* mirror `.octest` or `.octfail` behavior
* act as a second specification of the same semantics

There must be a **single source of truth** for language behavior:

> `Language/`

\---

### 5\. Go tests orchestrate, not define semantics

Go test code should:

* invoke `oct test`, `oct bench`, `oct artifact`
* validate CLI boundaries and integration behavior

Go test code must not:

* define or reimplement language semantics
* encode evaluation logic that belongs to Oct tests

\---

### 6\. Migrate legacy semantic tests out of Go

If existing Go tests encode language semantics:

* do not extend them
* migrate the behavior into `.octest` or `.octfail` under `Language/`

Go should not remain the owner of semantic contracts.

\---

### 7\. Keep the repository roles clean

Each top-level area has a strict purpose:

* `Language/`
Canonical language contracts (semantic truth)
* `Packages/`
Reusable Oct packages (user-level code)
* `testdata/`
Fixtures, synthetic inputs, transitional data (not semantic contracts)
* Go code (`cmd/`, `internal/`)
Language implementation only

\---

### 8\. Do not use testdata for language semantics

`testdata/` must not contain language contracts.

Do not:

* place `.octest` or `.octfail` there as canonical behavior
* hide semantic tests as fixtures

Use `testdata/` only for:

* synthetic inputs
* invalid parsing cases
* temporary or transitional data

\---

## Placement Rule

When adding code or tests:

* If it defines language behavior → `Language/`
* If it is reusable user code → `Packages/`
* If it is synthetic or temporary → `testdata/`
* If it implements the language → Go

\---

## Allowed vs Forbidden Patterns

### Adding a new language behavior

✅ Correct:

* implement behavior in Go
* add `.octest` / `.octfail` under `Language/...`

❌ Forbidden:

* define behavior in Go tests using embedded Oct

\---

### Testing behavior

✅ Correct:

* use `Language/<Domain>/<Concept>/valid|invalid`

❌ Forbidden:

* duplicate the same behavior in Go tests

\---

### Reusing logic

✅ Correct:

* move reusable logic into `Packages/`

❌ Forbidden:

* simulate reuse via embedded Oct strings in Go
* build meta-systems in Oct to compensate for missing language features

\---

## If You Are Unsure

Default to preserving the boundary:

* If it feels like “this could be written in Oct instead of Go” → it probably should NOT be
* If it feels like “this test could live in Language/ instead of Go” → it probably SHOULD
* If something feels like a limitation, friction point, or missing feature → record it in `FEEDBACK.md` instead of working around the rules

When in doubt:

> keep implementation in Go, and semantics in Oct

\---

## Summary

* Go defines the language
* Oct expresses the language
* Language contracts live in `Language/`
* Go orchestrates, not specifies

This separation is fundamental to the architecture.
Do not violate it.

