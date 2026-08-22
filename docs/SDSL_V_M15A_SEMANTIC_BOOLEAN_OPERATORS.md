# SDSL-V M15a: Semantic Boolean Operators

M15a aligns SDSL-V source with Oct conditional spelling.

Shader authors write:

```sdslv
and
or
not
```

not C-style logical punctuation:

```sdslv
&&
||
!
```

The backend boundary does not change. VD-MIR remains the compiler boundary, and emitted HLSL still uses `&&`, `||`, and `!`.

## Source rules

Semantic boolean operators are available in both runtime and compile-time expressions:

```sdslv
let runtimeFlag: bool = params.M > 0u and params.N > 0u;

comptime let CanVectorize: bool =
    C.UseVectorizedLoad and not C.DisableVectorLoads and C.Tile.K == 16u;
```

`!=` remains the comparison spelling. The rejected forms are logical `&&`, logical `||`, and unary logical `!`.

## Precedence

M15a follows Oct-style semantic precedence:

- comparisons bind tighter than `and` and `or`;
- `not` binds tighter than `and`;
- `and` binds tighter than `or`.

## Type rules

- `and` requires bool operands and produces `bool`.
- `or` requires bool operands and produces `bool`.
- `not` requires a bool operand and produces `bool`.

No bitwise operators are added in M15a.

## Comptime and lowering

The same semantic operators work in `require`, `static assert`, `comptime let`, `comptime if`, `comptime match`, `comptime when utility`, and `comptime for` bounds or nested compile-time guards.

They do not relax existing comptime dependency restrictions.

Lowering normalizes semantic spellings onto the existing logical ops for backend output:

- `and` -> `&&`
- `or` -> `||`
- `not` -> `!`

This keeps source style Oct-like while keeping backend output target-native.

## Examples

- `Examples/SDSL-V/M15a/SemanticBooleanOperators.sdslv`
- `Examples/SDSL-V/M15a/SemanticComptimeGuard.sdslv`

M16a guarded memory access uses the same source-level boolean spelling:

```sdslv
let a: f32 =
    read AView[row, k] when row < params.M and k < params.K else 0.0;

write CView[row, col] = a when row < params.M and col < params.N;
```
