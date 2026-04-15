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

---

```

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

---
Observation:
Chained field access — result.Selection.HasWinner — is parsed as an enum value expression rather than two field accesses. Every CommitmentResult test needed let sel = result.Selection as an intermediate binding before asserting. 

Suggestion:
Worth adding to the language report for future LLM sessions.
