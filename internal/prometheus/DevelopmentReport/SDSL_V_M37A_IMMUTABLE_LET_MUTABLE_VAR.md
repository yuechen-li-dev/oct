# SDSL-V M37a: immutable `let`, mutable `var`

M37a is a deliberate source compatibility break. `let` now creates an
immutable local binding and `var` creates a mutable local binding. Both forms
require an initializer; there is no `let mut`, `comptime var`, mutable
parameter, or uninitialized-local syntax.

Assignment validation follows the root local binding through field and index
paths. A local aggregate can therefore be mutated only when its root was
declared with `var`. Shader resources and access-qualified matrix views remain
separate: a `let` index may address a `readwrite` resource, while readonly
resources remain unwritable. Flow-owned board fields retain their existing
compiler-owned mutation rule.

The validator enforces the source contract before VD-MIR lowering. VD-MIR and
HLSL continue to represent source locals as ordinary backend locals; M37a does
not require HLSL `const` emission or imply a code-generation change.

The migration converts only local declarations whose values are reassigned or
mutated. Old mutable-`let` source is rejected rather than retained behind a
compatibility path.

To preserve the universal initializer rule without restoring uninitialized
locals, M37a also extends the compiler-owned fixed-shape construction path to
fixed-size `array<T, N>` locals. `Fill(...)` now target-types both `array` and
`ndarray`, nested fixed arrays derive shape recursively, and rank-1 fixed-array
dense literals remain exact-count, exact-type initializers. `array` and
`ndarray` stay distinct public types; no implicit conversion is introduced.

Canonical M36a benchmark IDs remained stable across the migration, while their
authoritative source SHA values changed because the committed benchmark source
now uses explicit whole-value initialization. The canonical NDArray benchmark
SPIR-V hash changed from `bd3ea90711adaad03e98923d7397d5b3e259497e437918bd444c46a0c46dc083`
to `7ee4a36be544f4742c30aa13d159ce8312d31a527403c00d7b67ab0d377a397a`
after the source was rewritten from a tensor copy loop to direct `Generate`
construction. The tensor contraction canonical artifact remains
`9c14708fb37490d3f0f776a2cd4b156dbf00936fb8a4d6f5db159718f393a3a7`.
