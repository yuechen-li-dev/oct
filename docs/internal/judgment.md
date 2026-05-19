# internal/judgment guidance

`internal/judgment` is the Go-layer analogue of Oct's language-level `when utility` idea.

Oct is intentionally **100% implemented in Go and 0% implemented in Oct**, so compiler/tooling implementation code cannot depend on Oct-layer utility libraries for internal decisions. This package exists because tooling also faces utility-shaped candidate selection and needs deterministic, inspectable resolution.

## Rule of thumb

> If it is a bounded choice with competing evidence, use judgment.
> If it is a hard semantic rule, use if/else.

## Use `internal/judgment` when

Use `internal/judgment` when all of these are true:

- Exactly one answer must be selected.
- Candidate choices are bounded and valid.
- Multiple competing signals are relevant.
- No single signal should always dominate.
- Deterministic/debuggable behavior is required.

## Use direct rule logic when

Use direct `if/else` or direct rule dispatch when:

- The decision is a hard semantic rule.
- A condition is authoritative.
- There is no meaningful tradeoff.
- Scoring or traceability does not add value.
- The logic is simple and not heuristic.

## Judgment pattern

1. Generate bounded candidates.
2. Mark unsafe/impossible candidates ineligible with explicit reasons.
3. Add named weighted considerations.
4. Score candidates deterministically.
5. Apply deterministic tie-breaks.
6. Preserve traces where debugging or inspection may be required.

This is **not** nondeterministic heuristic magic. It is deterministic, inspectable ambiguity resolution.

## Example shapes

### 1) Formatter layout selection (judgment-shaped)

Candidates:
- `inline`
- `multiline`
- `leaveUnchanged`

Signals:
- line width
- nesting depth
- callee kind
- comment risk
- diff stability

Why judgment-shaped: several plausible layouts can be valid, and their tradeoffs are signal-dependent.

### 2) Diagnostic suggestion ranking (judgment-shaped when ambiguous)

Candidates:
- unknown function
- missing import
- wrong namespace
- typo suggestion
- enum/type confusion

Signals:
- name similarity
- imported namespace set
- expression context
- available symbols
- prior parse/type context

Why judgment-shaped: when multiple explanations are plausible, deterministically ranking candidates with traces improves debuggability.

### 3) Import/path recovery (judgment-shaped when multiple roots are plausible)

Candidates:
- package root A
- package root B
- local fixture path
- repository-root fixture path

Signals:
- file exists
- package context
- invocation root
- stability preference

Why judgment-shaped: several candidate roots may be valid from partial evidence.

### 4) Artifact/report presentation (can be judgment-shaped)

Candidates:
- table
- key-value table
- callout
- compact summary
- leave raw

Signals:
- data shape
- row/column count
- human readability
- output format
- size guard

Why judgment-shaped: presentation quality depends on competing formatting constraints.

### 5) Non-example (hard semantic rule)

`Matrix<Int> has no field depth` is not judgment-shaped.

That is a direct semantic diagnostic and should use hard rule logic (`if/else`, direct rule tables, or equivalent deterministic semantic checks).

## Boundaries and ownership

Keep `internal/judgment` domain-neutral and dependency-light.

- Domain packages (for example `ocfmt`, diagnostics, import/path tooling) should generate candidates and considerations.
- `internal/judgment` should only score/select/trace.
- Do not move language semantics into judgment scoring.
