# Policy Lab M0 report

## Outcome

**Task end state: Success.** Ordinary Oct, Octest, Algorithms, Markdown artifacts,
and Octomata express both bounded specimens through the real interpreted and
compiled paths. All 34 discovered cases pass in interpreted, compiled, and auto
execution; compiled and auto each run 34 compiled cases with zero interpreted
fallback. The deterministic Markdown artifact is produced through the real
artifact path.

**Central answer:** ordinary Oct can represent these bounded policies as
executable knowledge without becoming a policy DSL, but the experiment exposes
general provenance, explanation, domain-value, and default/conflict ergonomics
that Catala addresses more directly.

**Convergence verdict: B.** Behavioral/operator constraints and policy
applicability share admissibility vocabulary, but require separate static and
runtime forms with different subjects, phases, and failure semantics.

**Law-as-code verdict: Success B.** Oct expresses the computation and procedure
well; the meaningful missing abstraction is general provenance/explanation, with
default/conflict semantics a secondary scaling question. No policy syntax is
justified by M0.

## 1–4. Specimens, authority, and interpretation scope

1. **Specimen A:** bounded FLSA overtime classification and calculation.
2. **Authority:** 29 U.S.C. § 207(a)(1), with one caller-supplied exemption probe
   from 29 U.S.C. § 213(a)(1).
3. **Interpretation scope:** whole hours, whole cents, supplied coverage/regular-
   rate/exemption classification, explicit half-cent ceiling. The implementation
   excludes the rest of the FLSA and its regulations.
4. **Specimen B:** bounded DOJ FOIA administrative appeal from adverse response
   through appeal window, review, affirmance/remand, withdrawal, or expiration.
   Authority is 5 U.S.C. § 552(a)(6)(A)(i)–(ii), cross-checked against the DOJ
   FOIA Reference Guide.

## 5–14. Model inventory

5. **Substantive model:** `EmploymentCase -> OvertimeDecision`, with coverage,
   exception, threshold, calculation, trace, and provenance explicit.
6. **Procedural model:** `DOJAppealProcess` is a typed input/yield/result flow;
   state transitions, board flags, completion, and history are inspectable.
7. **Records/enums:** case facts, decisions, provenance, source-map columns,
   appeal outcomes, reasons, exemptions, events, timeliness, rule effects, and
   conflict results are nominal data.
8. **Concepts:** `WorkweekHours` admits 0–168; `RegularRateCents` admits positive
   whole cents. Neither encodes eligibility.
9. **Templates:** `AtOrBelow<T>` is one small reusable threshold family and a
   behavioral-bound probe. It succeeds for `Int` only after specialization; it
   cannot state an operator requirement over `T`.
10. **Captured callables:** `MakeAtOrBelowPredicate` and the quantifier Fact make
    thresholds explicit dependencies rather than ambient policy state.
11. **Algorithms mapping:** `Any` = exists, `All` = for all, `Filter` = select,
    `Map` = transform, and `Fold` = aggregate. The corpus uses all five.
12. **Base rule:** `BaseOvertimeApplies` expresses assumed coverage and hours
    above an explicit threshold.
13. **Exception:** `Section213A1ExceptionApplies` is evaluated before calculation
    and returns a rich reason, not a hidden control-flow side effect.
14. **Conflict representation:** `ResolveRuleEffects` deterministically exposes
    no rule, one rule, compatible overlap, or conflict. It never relies on map,
    file, or declaration order.

## 15–23. Executable knowledge and change evidence

15. **Facts:** source-derived rule Facts, domain-admission Facts, negative policy
    Facts, invariants, interpretation-selection Facts, procedural Facts, source-
    map Facts, and artifact determinism Facts.
16. **Theories:** threshold boundaries, nonnegative calculations, four conflict
    states, amendment impact, appeal deadline, and decision timeliness.
17. **Boundary cases:** 39/40/41 work hours, rates that create half cents,
    90/91 appeal days, and 20/21 decision working days.
18. **Procedural Facts:** timely appeal enters review, decision cannot precede
    notice, remand is explicit, an unappealed response expires only on the
    explicit deadline event, and history is deterministic.
19. **History/audit result:** `Filed -> NoticeIssued -> AppealWindow -> Review ->
    Remanded`. Removing implementation-only wait states was necessary to keep the
    legal audit trail semantically clean.
20. **Traceability:** doc comments carry rule IDs at each Fact; `BuildSourceMap`
    retains rule ID, authority, section, interpretation, symbol, and Fact name.
21. **Explanation artifact:** deterministic Markdown includes sources, source
    map, one substantive decision trace, one procedural history, ambiguity,
    amendment, concepts, tests, and scope limits.
22. **Amendment:** a clearly synthetic 40-to-38-hour threshold changes the 39-
    and 40-hour cases while leaving the source-derived rule and tests intact.
23. **Ambiguity:** whole-cent rounding is not specified by the narrowed source.
    The model selects half-cent ceiling and retains floor as an executable
    alternative. It does not pretend the source settles that choice.

## 24. Money, date, and rounding result

Oct has numeric units and duration-shaped values, but this corpus found no
authoritative currency/date/working-day/holiday domain library suitable for the
specimens. Money is therefore labeled whole cents (`Int`), not a fake precision
claim. Calendar and working days are supplied indexes. Rounding is an explicit
integer algorithm and has boundary Facts; no incidental `Float` formatting is
used.

## 25–27. Catala comparison

Catala uses literate source, scopes, conditional definitions, labeled exceptions,
assertions, states, and prioritized default logic. Oct uses ordinary nominal
data, functions, explicit control flow, Facts/Theories, artifacts, and Octomata.

| Concern | Catala approach | Oct approach | Oct result | Gap |
|---|---|---|---|---|
| Source-law traceability | Literate law and code co-located | Doc comments, source-map values, Markdown | Workable | Provenance can drift from code |
| Conditional rules | Definitions under conditions | Bool functions and condition switch/if | Clear | More implementation-shaped |
| Exceptions | Labeled prioritized exceptions | Explicit exception predicate before base consequence | Clear at M0 scale | No declarative grouping or priority graph |
| Conflict/default semantics | Prioritized default logic with conflict semantics | Explicit `ConflictResult` and ordered evaluation | Honest and testable | Boilerplate grows with distributed rules |
| Assertions | Source assertions | `[Fact]` and assertions | Strong | Fact-to-authority linkage is conventional |
| Case tables | Examples/tests | `[Theory]` + `[InlineData]` | Natural | No first-class legal-example metadata |
| State/procedure | Explicit state variables | Octomata events, board, result, history | Natural | History exposes source state names only |
| Quantifiers | Domain list operators | `Algorithms.Any/All/Map/Filter/Fold` | Readable | Explicit type arguments add noise |
| Money/date | Domain types | Whole cents and day indexes in this corpus | Bounded only | General domain libraries missing |
| Explanation | Literate program structure | Rich decision record + artifact | Useful | Separate artifact rather than source-native explanation |
| Change impact | Exceptions aligned to text structure | Facts/Theories and explicit amendment function | Detectable | No source citation dependency graph |
| Domain-expert readability | Lawyer-oriented syntax | Ordinary programming plus report | Mixed | Catala retains a meaningful literate advantage |

25. The matrix above is the conceptual Catala comparison.
26. Catala clearly handles source proximity, labeled/distributed exceptions,
    default/conflict semantics, legal-domain values, and domain-expert reading
    better.
27. Ordinary Oct cleanly handles typed case facts, rich results, executable
    propositions, boundary families, explicit ambiguity alternatives, generic
    list operations, deterministic reports, and procedural state/history.

## 28–30. Syntax, provenance, and explanation needs

28. **Policy-specific syntax:** not justified. No `law`, `rule`, `exception`,
    parser mode, policy runtime, or Claim AST was added.
29. **General provenance:** yes, this is the largest gap. A general source/
    authority/citation/requirement relationship could also serve engineering
    standards, scientific papers, contracts, and product requirements.
30. **General explanation:** useful, but rich ordinary result records plus
    deterministic artifacts are enough for M0. A tracing engine is not justified.

## 31–34. Concept/admissibility result

| Concept category | Subject | Predicate/contract | Checked when | Failure meaning |
|---|---|---|---|---|
| Value | Runtime value | 0 <= hours <= 168 | Static proof or checked construction | Invalid input value |
| Structural | Nominal record value | Required named fields and types | Type checking/construction | Invalid case shape |
| Behavioral | Concrete template type | Expression/operator must type-check | After specialization today | Compile-time specialization error |
| Policy | Valid case value | Rule prerequisites/applicability | Policy evaluation | Ordinary ineligible/not-applicable result |

31. **Policy-admissibility result:** Concepts belong at the valid-input boundary.
    `EligibleApplicant` as the only function input would erase explainable
    ineligible cases and is therefore the wrong default.
32. **Behavioral/operator probe:** `AtOrBelow<T>` demonstrates the pressure:
    `T` needs `<=`, but current templates cannot declare that contract. The body
    fails only after a concrete unsupported specialization.
33. **Static/runtime distinction:** shared vocabulary does not erase phase.
34. **Failure semantics:** Concept admission must not convert a policy-negative
    outcome into a constructor error or runtime failure.

| Constraint | Compile time? | Construction time? | Policy evaluation time? |
|---|---:|---:|---:|
| Operator `<=` available on `T` | Yes | No | No |
| Record has required field | Yes | No | No |
| Hours in 0–168 when literal | Yes | No | No |
| Dynamic hours in 0–168 | No | Yes | No |
| Income/hours below policy threshold | Sometimes known, not authoritative | No | Yes |
| Appeal deadline not expired | No | No | Yes |

| Failure kind | Correct use in this experiment |
|---|---|
| Compile-time rejection | Missing field/operator, statically invalid refinement |
| Checked constructor rejection | Dynamic structurally invalid hours/rate |
| Policy result = ineligible | Valid case outside coverage, exempt, or below threshold |
| Policy exception | Rich explicit reason/result, not a thrown program exception |
| Runtime error | Infrastructure/unchecked failure only; not ordinary policy outcome |

## 35. Behavioral Concept design options

| Option | Illustrative syntax | Constrains | Phase | Templates/values | Existing Concepts | Complexity/failure |
|---|---|---|---|---|---|---|
| Operator requirement | `RequireOperator(T, "<=")` | Named operator availability | Specialization/compile time | Direct template bound; no value effect | Separate from refinements | Low surface, stringly and weakly composable; compile error |
| Behavior Concept | `concept Ordered<T> { fn <=(T,T)->Bool }` | A closed behavior set | Compile time | Reusable bound over templates | Shared Concept vocabulary, distinct form | Higher design/runtime-erasure work; conformance error |
| Expression validity | `Require(ValidExpression(T <= T))` | One exact expression shape | Compile time | Precise local bound | Compiler-owned extension | Risks comptime/meta surface; proof diagnostic |
| Inferred body requirements | no new source syntax | Operations used by specialization | Specialization time | Current behavior with improved trace | No new Concept form | Lowest language cost, weakest reusable contract; late specialization error |

No option is selected in M0. The experiment supplies evidence for a reusable
static behavior contract but not enough to choose its syntax or conformance model.

## 36. Policy applicability design options

| Option | Example | Phase | Result/failure | Core language? |
|---|---|---|---|---|
| Ordinary Bool/result function | `EvaluateOvertime(case)` | Runtime | Rich decision | **M0 default; no change** |
| Refined constructor | `EligibleApplicant(raw)?` | Construction | Error on ineligibility | Usually wrong for explainable negatives |
| Concept predicate over case | `Concept(case)` | Construction/runtime boundary | Admission failure | Risks collapsing validity and eligibility |
| Rule value with `Applies/Consequence` | `PolicyRule { Applies, Consequence }` | Runtime | Explicit not-applicable/conflict | Possible library pattern; no core need yet |

## 37–46. Verdicts and feature conclusions

37. **Convergence:** B — shared admissibility vocabulary, separate static/runtime
    forms.
38. **Law-as-code:** Success B — computation works; general provenance is missing.
39. **`[Fact]`:** useful as normative executable knowledge when names read as
    propositions and citations remain adjacent.
40. **`[Theory]`:** maps naturally to synthetic case/boundary families. It must
    not be called actual case law without real cited cases.
41. **Octomata:** maps naturally to procedural policy because state, events,
    result, board, and history are real semantics rather than decoration.
42. **Templates:** modest help for policy families; the main evidence is the
    missing behavioral/operator contract, not a need to template all rules.
43. **Explicit captures:** improve auditability by naming threshold dependencies.
44. **Default logic:** no language support yet. Explicit base-plus-exception is
    readable at this scale; distributed exceptions may justify a later general
    default/conflict experiment.
45. **Conflict resolution:** no language support yet. Explicit conflict values
    are sufficient and safer than hidden priority for M0.
46. **Provenance:** deserves a general Oct feature experiment, not legal syntax.

## 47. Catala’s literate advantage

Catala-style literate source remains a meaningful advantage. The Oct artifact is
human-readable, but it is generated after the fact and can drift from cited text.
Catala aligns source text, definitions, and exceptions more directly with legal
structure. Oct’s advantage here is reuse of general typed/tested program features,
not superior legal authoring ergonomics.

## 48–52. Corpus and execution evidence

48. **Corpus:** `Experiments/PolicyLab/` with `M0`, README, report, source, tests,
    and a generated artifact under `out/test-artifacts`.
49. **Tests:** 34 discovered, 34 passed, 0 failed, 0 skipped in every mode.
50. **Parity:** interpreted, compiled, and auto all pass the same 34 cases.
51. **Fallback:** compiled = 0; auto = 0.
52. **Artifact:** one Markdown file, 3,244 bytes, SHA-256
    `51a6bfa3263648205020a0f6f81ae829167a250ae484d3755c7ad2f6fb7e3a4b` on the
    first run. The second run reported `unchanged`, 3,244 bytes, and the same
    hash, completing the determinism qualification.

Execution identity was `gooct-cli` at Oct version `dev`. Observed local CLI
timings were 17 ms interpreted, 1,073 ms compiled, 1,080 ms auto, 21 ms for the
first artifact run, and 16 ms for the unchanged artifact run. These timings are
diagnostic observations from this machine, not performance claims.

## Auditability scorecard

This is a qualitative evaluation of the two language/tooling approaches in the
bounded experiment, not an assessment of the underlying public policies.

| Dimension | Ordinary Oct M0 | Catala conceptual baseline | Evidence |
|---|---|---|---|
| Correctness | Strong for bounded executable behavior | Strong domain semantics | Typed decisions + 34 parity cases; Catala formal/default semantics |
| Traceability | Workable | Strong | Source map vs literate co-location |
| Readability | Strong for programmers | Strong for legal-domain readers | Ordinary functions vs lawyer-oriented constructs |
| Composability | Strong | Domain-focused | General records/templates/Algorithms |
| Testability | Strong | Strong | Facts/Theories vs assertions/examples |
| Change impact | Strong at test boundary | Strong at text/rule boundary | Synthetic amendment vs legal-structure alignment |
| Procedural modeling | Strong | Domain-dependent explicit states | Octomata history and results |
| Exception modeling | Clear at small scale | Strong at distributed scale | Explicit predicate vs labeled exceptions |
| Ambiguity handling | Strong when modeled explicitly | Strong when alternatives are explicit | Separate rounding interpretations |
| Domain-expert accessibility | Mixed | Strong | Generated report vs literate source |

## 53–56. Architectural friction and lessons

53. **Friction:** no currency/calendar domain values; provenance is conventional;
    Fact metadata cannot directly retain authority; artifact construction is
    separate from source; template behavioral requirements are implicit; flow
    helper states can pollute audit history if authors are careless.
54. **Concept lesson:** “admissibility contract” is a useful umbrella, but the
    subject, phase, and failure carrier are part of the semantics. Unifying the
    word does not justify unifying syntax or runtime behavior.
55. **Fact lesson:** a Fact can carry normative executable knowledge, but names
    and adjacent source citations are doing work that the current test metadata
    cannot enforce.
56. **Compiler Theory lesson:** phase separation is authoritative. Static shape
    and operator availability are compiler judgments; refined runtime inputs cross
    a checked constructor boundary; policy applicability remains program data.
    A sound generalized model must preserve those different proof obligations and
    diagnostics rather than route all failures through one Concept mechanism.

## 57. One proposed next milestone only

**Policy Lab M1 — General Provenance and Explanation Vertical.** Test one small,
domain-neutral provenance contract across this policy corpus, one engineering-
standard requirement, and one scientific-paper-derived Fact. Measure citation
drift detection, source-to-symbol/source-to-Fact navigation, deterministic
artifact generation, and compiled erasure. Start with library/tooling metadata;
add no legal syntax and do not begin behavioral Concepts or default logic in that
milestone.

M1 is proposed only and is not begun here.
