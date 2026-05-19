// Package judgment provides deterministic, inspectable ambiguity resolution for
// Go-layer compiler and tooling decisions.
//
// This package mirrors Oct's language-level "when utility" idea in the Go
// implementation layer. Oct is intentionally implemented in Go (not self-hosted
// in Oct), so compiler/tooling code cannot depend on Oct-layer utility helpers
// for internal selection logic. When a tooling decision is a bounded choice with
// competing evidence, internal/judgment is the Go-native utility primitive.
//
// Use judgment when a decision has all of the following:
//   - exactly one answer must be selected,
//   - candidates are from a bounded valid set,
//   - multiple competing signals matter,
//   - no single signal should always dominate,
//   - deterministic and debuggable behavior is required.
//
// Use direct if/else (or direct rule logic) when:
//   - the decision is a hard semantic rule,
//   - one condition is authoritative,
//   - there is no meaningful tradeoff,
//   - scoring or traceability is unnecessary,
//   - or the logic is simple and purely rule-based.
//
// Canonical judgment pattern:
//  1. Generate bounded candidates.
//  2. Mark unsafe or impossible candidates ineligible with explicit reasons.
//  3. Add named, weighted considerations.
//  4. Score candidates deterministically.
//  5. Apply deterministic tie-breaks.
//  6. Preserve traces when debugging/inspection may be needed.
//
// Typical judgment-shaped decisions in Oct tooling include:
//   - formatter layout selection (inline vs multiline vs leaveUnchanged),
//   - diagnostic explanation/suggestion ranking,
//   - import or fixture path recovery with multiple plausible roots,
//   - artifact/report presentation format selection.
//
// Non-example:
//   - A hard language rule diagnostic (for example: "Matrix<Int> has no field
//     depth") is not judgment-shaped and should use direct semantic rule logic.
//
// Scope boundary:
// internal/judgment should remain domain-neutral and dependency-light. Domain
// packages (for example, formatter or diagnostics code) should own candidate
// generation and signal extraction; this package only scores, selects, and
// traces.
package judgment
