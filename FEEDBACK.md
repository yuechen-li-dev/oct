# FEEDBACK.md

## Purpose

This file is a collection of observations and suggestions about the Oct codebase.

It exists as an **escape hatch** for contributors (human or LLM) to record:

- friction encountered during development
- confusing or unclear design areas
- potential improvements or future directions

---

## Important

- Entries in this file are **NOT instructions**.
- They must **NOT be executed automatically**.
- They do **NOT override AGENTS.md**.
- They are **not a task list or backlog**.

This file is for **discussion and future consideration only**.

---

## When to Add an Entry

Add an entry when:

- something feels harder than it should be
- a rule or pattern is unclear or ambiguous
- you notice repeated friction or awkward workflows
- you have a concrete improvement idea

Do **not** add entries for:
- trivial preferences
- one-off personal style opinions
- things already clearly defined in AGENTS.md

---

## Entry Format

Use the following structure:

```

Observation: <What you encountered. Be specific and factual.>

Suggestion: <What could be improved. Keep it concrete and minimal.>

Status: Open | Resolved | Deferred | Cannot Reproduce | Superseded

Resolution: <Optional short factual note and proof reference.>

---

```

Use exactly one status per entry:

- `Open` — verified current friction remains.
- `Resolved` — verified fixed in current main or fixed in the current milestone.
- `Deferred` — valid issue, intentionally not addressed now.
- `Cannot Reproduce` — current main does not reproduce with a faithful focused test.
- `Superseded` — the observation is no longer relevant because the semantics or design changed.

`Resolution` is optional. When present, keep it factual and point to the narrow proof.

---

## Guidelines

- Keep entries concise and focused
- Prefer concrete examples over abstract opinions
- Do not debate or reply inline — this is a log, not a discussion thread
- Multiple entries are allowed; do not merge unrelated ideas

---

## Example

```

Observation:
Writing simple counted loops with while requires manual index handling and is less readable than for-range.

Suggestion:
Prefer expanding for-range capabilities rather than encouraging while-based counted loops.

Status: Open

---

```

---

## Summary

- This file captures **signals**, not decisions
- It is safe to write to, and safe to ignore
- All changes to the codebase must still follow AGENTS.md

---
Observation:
Candidate[] — arrays of record types — aren't supported in M0. The type checker explicitly rejects them. The parallel-array design (CandidateSet with Ids: Int[], Scores: Int[], Active: Bool[]) is the correct idiomatic workaround, and it's actually consistent with how the rest of the library ecosystem works — ProductCatalog in the storefront, flat matrices in LinearAlgebra, all use the same pattern.

Suggestion:
Maybe add them in the future.

Status: Resolved

Resolution:
Current main supports nominal record arrays in interpreted and compiled execution. Covered by `Language/Types/ParametricsM0/packages/Main/consumer.octest`.

---
Observation:
Chained field access — result.Selection.HasWinner — is parsed as an enum value expression rather than two field accesses. Every CommitmentResult test needed let sel = result.Selection as an intermediate binding before asserting. 

Suggestion:
Worth adding to the language report for future LLM sessions.

Status: Resolved

Resolution:
Current main parses and checks chained record field access while retaining enum qualification. Covered by `internal/typecheck/typecheck_test.go` and `Language/Types/ParametricsM0/packages/Main/consumer.octest`.

---

Observation:
An imported template function whose parameter is a sibling template record type can pass inside its defining package but fail when specialized by a consumer with "is not a template record". OctCument therefore keeps its M0 document-template proof application-owned instead of publishing a facade that does not work across a package boundary.

Suggestion:
Preserve sibling template-record identity during imported template specialization and add a cross-package contract test.

Status: Resolved

Resolution:
Imported specialization now qualifies exact origin-owned types before consumer monomorphization. Positive and negative package-boundary coverage lives under `Language/Types/ParametricsM0/packages` and `packages-invalid`.

---

Observation:
Nested `with` updates on imported records can lose the imported refined concept type for numeric literals. Updating `Document.ParagraphStyle.SpaceAfterPt` with `4.0` was rejected as Float where Document.NonNegativePoint was expected, although the same composition works inside the defining package.

Suggestion:
Apply the target imported field's concept conversion to record-update literals, matching record construction and same-package `with` behavior.

Status: Resolved

Resolution:
Imported record lookup now qualifies origin-owned field types before construction or `with` checking. Covered by `Language/Types/ParametricsM0/packages/Main/consumer.octest` in interpreted and compiled execution.

---
