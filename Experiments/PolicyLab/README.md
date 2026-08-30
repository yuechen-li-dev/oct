# Policy Lab M0

Policy Lab M0 asks whether ordinary Oct can represent bounded policy as typed,
executable, testable knowledge without becoming a policy-specific DSL. The
experiment also probes whether policy applicability and template/operator
constraints share a deeper idea of admissibility.

This is research, not legal advice. The code is an executable interpretation of
cited authority, not the law itself, not a substitute for counsel, and not an
authoritative implementation for payroll or FOIA administration.

## Specimens

### A — FLSA overtime

The substantive specimen models one deliberately narrow interpretation of:

- [29 U.S.C. § 207(a)(1)](https://uscode.house.gov/view.xhtml?edition=2023&num=0&req=granuleid%3AUSC-2023-title29-section207): covered employment above 40 hours in a workweek receives compensation for excess hours at not less than one and one-half times the regular rate.
- [29 U.S.C. § 213(a)(1)](https://uscode.house.gov/view.xhtml?req=%28title%3A29+section%3A213+edition%3Aprelim%29): the bounded executive/administrative/professional exemption probe.

The caller supplies coverage, regular-rate, and exemption classifications. The
experiment does not derive those legally complex classifications. It uses whole
hours and whole cents. Because the narrowed statutory text does not specify
discrete-cent rounding, the selected interpretation rounds a half-cent upward;
flooring is retained as an explicit alternative.

### B — DOJ FOIA administrative appeal

The procedural specimen models the bounded adverse-response and administrative-
appeal path described by:

- [5 U.S.C. § 552(a)(6)(A)(i)–(ii)](https://www.justice.gov/oip/freedom-information-act-5-usc-552): an appeal period of at least 90 days after an adverse determination and a determination on appeal within 20 working days.
- The [DOJ FOIA Reference Guide](https://www.justice.gov/oip/department-justice-freedom-information-act-reference-guide): a DOJ appeal must be transmitted or postmarked within 90 calendar days and may be affirmed or remanded.

The flow starts with a request already filed. It models adverse notice, appeal
window, review, affirmance, remand, withdrawal, expiration, and full release. It
uses supplied elapsed calendar-day and working-day indexes. It does not calculate
holidays, tolling, extensions, unusual circumstances, litigation, or every appeal
ground.

Sources were checked on 2026-08-30. Statutes, regulations, and guidance can
change; current authority must be checked before any real-world use.

## What is explicit interpretation or synthetic material?

- Whole-hour inputs and whole-cent rates are bounded modeling assumptions.
- Half-cent ceiling is a selected interpretation; half-cent floor is preserved
  as an alternative.
- The 40-to-38-hour amendment is synthetic and exists only to measure change
  impact.
- The four conflict states use synthetic rule effects to test whether ordinary
  Oct exposes, rather than hides, rule conflict.
- Day indexes stand in for missing calendar/date/holiday domain support.

## Corpus

- `M0/policy_lab_m0.oct` — records, enums, refined Concepts, policy functions,
  explicit exception/conflict logic, templates, captured callables, Atlas graph,
  Octomata procedure, and deterministic artifact builder.
- `M0/policy_lab_m0.octest` — normative Facts, boundary Theories, negative
  propositions, amendment cases, procedural invariants, history, and artifact.
- `REPORT.md` — comparison, scorecard, phase analysis, verdicts, and the sole
  proposed next milestone.

`[Fact]` functions are executable propositions linked to rule IDs and authority
through doc comments and the source-map table. `[Theory]` plus `[InlineData]`
represents boundary and case families. They are evidence for this interpretation,
not judicial precedent or authoritative case law.

## Run

From the repository root:

```powershell
go run ./cmd/oct test Experiments/PolicyLab/M0 --execution interpreted --json
go run ./cmd/oct test Experiments/PolicyLab/M0 --execution compiled --json
go run ./cmd/oct test Experiments/PolicyLab/M0 --execution auto --json
go run ./cmd/oct artifact Experiments/PolicyLab/M0 --execution interpreted --json
```

The artifact is written deterministically to:

```text
out/test-artifacts/experiments_policy_lab_m0/policy_lab_audit.md
```

## Known ambiguities and gaps

- Literate source-law proximity remains absent; first-class project provenance
  now lives in the package's ordinary `AtlasDocument()` and is checked by
  `oct atlas verify`.
- Money, currency, calendar date, working-day, and holiday types are absent from
  the core surface used here.
- Default/exception priority and conflict resolution are ordinary explicit code,
  not a declarative semantic layer.
- Concept refinements validate admissible values; they intentionally do not turn
  ordinary policy ineligibility into a construction failure.
- Templates have concrete post-specialization operator checking but no behavioral
  Concept or operator bound.

## What this is not

This is not a Catala syntax port, legal NLP system, policy runtime, smart contract,
legal assistant, payroll system, FOIA case-management system, or claim that Oct
replaces Catala. No policy-specific parser, keyword, runtime, or core Concept
redesign was added.
