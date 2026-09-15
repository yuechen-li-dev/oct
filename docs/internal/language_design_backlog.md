# Language design backlog

This is a neutral parking lot for ideas surfaced by external review. It is not
a roadmap, accepted direction, or commitment. None of these ideas is
implemented by the current hardening pass.

## Dimension-generic numerics

**Idea:** Allow dimension variables or dimension parameters in generic numeric
signatures, including relationships such as `Float<A>`, `Float<B>`, and
`Float<A*B>`.

**Why it may be useful:** Shared numerical infrastructure could retain units
through generic statistics, optimization, linear algebra, and differential
equation APIs instead of requiring dimension-specific wrappers.

**Known tradeoffs:** This requires a defined dimension-parameter kind,
substitution and unification rules, transformed result dimensions, useful
diagnostics, and a clear interaction with ordinary type parameters. It is a
type-system feature, not a library-only refactor.

**No current commitment:** Oct's present explicit template and integer-exponent
dimension semantics remain unchanged.

## Rational dimension exponents

**Idea:** Represent rational dimension exponents so types such as `V / sqrt(Hz)`
can be stated.

**Why it may be useful:** Spectral-density and related scientific quantities
can naturally require half powers.

**Known tradeoffs:** Canonical representation, equality, simplification,
parsing, `Sqrt` admission, formatting, and diagnostics all become more complex.

**No current commitment:** Dimensions continue to use the current integer
exponent model.

## Quantity kinds

**Idea:** Add an optional nominal distinction for quantities such as `Torque`
and `Energy` that share the same structural dimensions.

**Why it may be useful:** A nominal layer could catch assignments that are
dimensionally compatible but semantically wrong.

**Known tradeoffs:** The design would need explicit rules for arithmetic,
conversion, generic code, aliases, and when nominal identity is preserved or
erased. It must not weaken structural dimensional checking.

**No current commitment:** Dimensionally identical values remain structurally
compatible under current Oct semantics.

## Template argument inference

**Idea:** Infer simple template arguments when call arguments determine them
uniquely.

**Why it may be useful:** Common template calls could be less verbose while
retaining static specialization.

**Known tradeoffs:** Inference introduces ambiguity policy, diagnostic and
compatibility obligations, especially for nested types and future constraints.
Explicit template arguments are currently auditable and deterministic.

**No current commitment:** Explicit template arguments remain required.

## Derived unit aliases

**Idea:** Support parse/type-display aliases such as `N`, `Pa`, `J`, and `W`
for existing structural dimensions, without implicit scale conversion.

**Why it may be useful:** Public scientific signatures become shorter and more
recognizable without changing their dimensional meaning.

**Known tradeoffs:** Alias normalization, display stability, name collisions,
and round-trip formatting need rules. SI prefixes are a separate design
question and the current no-prefix philosophy may remain intentional.

**No current commitment:** Canonical expanded dimension syntax remains the
authority.

## Capture consistency

**Idea:** Revisit whether explicit anonymous-function capture and implicit batch
capture should remain intentionally different.

**Why it may be useful:** One capture policy may make lifetime and snapshot
behavior easier to teach and audit.

**Known tradeoffs:** Batch capture is tied to its bounded execution model;
changing either surface may create churn or lose useful explicitness. A review
must begin from current capture semantics rather than older workarounds.

**No current commitment:** Existing capture rules remain unchanged.

## FLOW deterministic replay as a product claim

**Idea:** Document deterministic checkpoint continuation more prominently for
the explicitly supported FLOW subset.

**Why it may be useful:** Resumable, reproducible long-running computation is a
distinctive capability when its limits and evidence are clear.

**Known tradeoffs:** The claim must stay narrower than general replay. External
effects, nondeterministic inputs, unsupported persistent values, and RNG state
need explicit boundaries.

**No current commitment:** Current checkpoint contracts and experimental ABI
status remain authoritative.

## `when utility`

**Idea:** Retain and further evaluate the unusual explicit arbitration form as
an auditable language construct.

**Why it may be useful:** The finite candidate set, eligibility, scores, and
tie behavior are visible and inspectable at the decision site.

**Known tradeoffs:** It occupies language syntax for behavior that could also
be expressed by library code. Keyword and long-term compatibility costs need to
be justified by real consumers.

**No current commitment:** This note does not reopen the existing syntax or
semantics decision.

## Standard-library scope

**Idea:** Reassess core-versus-domain-package boundaries before 1.0.

**Why it may be useful:** A smaller deep core plus independently versioned
domain packages may reduce long-term compatibility surface.

**Known tradeoffs:** Moving packages can harm discoverability, offline use, and
existing workflows. The package manager and release model must be ready before
scope changes.

**No current commitment:** No package is moved, renamed, or removed here.

## Performance positioning

**Idea:** Position Oct as correct and auditable, fast enough for ordinary work,
with explicit acceleration boundaries, rather than as a direct competitor to
optimized native numerical kernels.

**Why it may be useful:** This matches the current Go-compiled path and explicit
Octxiliary/Prometheus boundaries without overstating scalar-kernel performance.

**Known tradeoffs:** Product positioning should still demand removal of
avoidable lowering overhead and must not excuse unmeasured regressions.

**No current commitment:** This is a messaging question, not a relaxation of
performance engineering or correctness requirements.
