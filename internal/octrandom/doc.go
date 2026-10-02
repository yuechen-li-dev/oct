// Package octrandom is the single native implementation behind Oct's
// Random v2 library: a counter-based generator in which every draw is a pure
// function of (stream key, index, parameters).
//
// The normative specification is internal/random/RANDOM_V2_LADDER.md, section
// 3. This package implements sections 3.2 (bit generator and key derivation)
// and 3.3 (native primitives). Both the interpreter and generated programs
// call this package, so interpreted and compiled execution cannot drift.
//
// Determinism tiers (ladder invariant I5):
//
//   - Seeded, Fork, Child, Unit, Between and IntBetween use only integer
//     arithmetic and exactly rounded float operations. Their results are
//     bit-identical on every platform.
//   - Normal calls math.Log and math.Cos. Its results are bit-identical
//     between lanes on one GOARCH, but may differ in the last bits across
//     architectures where the Go compiler fuses multiply-adds inside the math
//     package.
//
// Expressions in this package wrap intermediate products in explicit
// float64(...) conversions. Per the Go specification an explicit conversion
// rounds to the target type and therefore prevents fusing x*y + z into a
// single FMA instruction, so this package's own arithmetic is the same on
// every architecture.
package octrandom
