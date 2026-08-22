# SDSL-V M14: Constrained `comptime match`

M14 adds constrained multi-way compile-time shader staging with `comptime match`.

This is not runtime branching and not Zig-style arbitrary comptime. Templates and configs choose concrete shader variants; `comptime match` shapes code inside an already specialized variant.

The pipeline remains:

```text
source
  -> lex
  -> parse
  -> validate
  -> template/config monomorphization
  -> comptime evaluation and expansion
  -> lower expanded concrete shaders to VD-MIR
  -> emit HLSL
  -> optional DXC / SPIR-V / generated header
```

VD-MIR remains the compiler boundary. HLSL emits from VD-MIR and does not understand `comptime match`.

## Syntax

```sdslv
comptime match C.Tile.K {
    8u => {
        static assert false;
    }
    16u => {
        comptime let KUnroll: u32 = 16u;
        static assert KUnroll == C.Tile.K;
    }
    else => {
        static assert false;
    }
}
```

The scrutinee must be a compile-time expression. Exactly one arm is selected during comptime expansion, and only the selected arm is spliced into the AST before VD-MIR lowering. Non-selected arms are parsed but dropped before VD-MIR and HLSL; `static assert` statements in non-selected arms do not fire.

`comptime match` may appear wherever `comptime if` may appear. Runtime statements may appear inside the selected arm and lower normally after expansion.

## Patterns

M14 supports a deliberately small pattern set:

- integer literals such as `8u`, `16u`, and `32u`;
- bool literals `true` and `false`;
- `else`.

M14 does not support ranges, guards, destructuring, payload matching, wildcard `_`, type patterns, multiple comma-separated patterns, generated identifiers, or comptime functions. `comptime for` arrives later in M16.

Enum/config-symbol patterns are deferred; use integer or bool config fields in M14.

## Else and Exhaustiveness

- Bool matches are exhaustive when both `true` and `false` arms are present.
- A bool match with only one bool arm requires `else`.
- Integer matches require `else`; M14 does not try to prove integer exhaustiveness.
- Duplicate literal arms are rejected.

Example diagnostics include:

- `comptime match over integer requires else arm`
- `duplicate comptime match arm for 16u`
- `comptime match scrutinee must be compile-time`
- `comptime match arm pattern must be compile-time literal`

## Compile-Time Expressions

`comptime match` reuses the M13 compile-time expression rules.

Allowed scrutinee inputs include literals, resolved config fields, prior `comptime let` values, and arithmetic/comparison/boolean operators already supported by consteval.

Boolean operators in those compile-time expressions use `and`, `or`, and `not`. `!=` remains available as the comparison operator.

Forbidden inputs include runtime parameters and push constants, shader resources, thread builtins, workgroup memory, runtime locals, matrix views, tile reads or writes, reductions, match payloads, and runtime function calls.

## Relationship to Other Branching

`comptime if` remains the binary compile-time branch form. `comptime match` is the multi-way form for structural shader selection inside a specialized variant.

Runtime `match` remains an expression that lowers to VD-MIR. `comptime match` is a statement that expands away before VD-MIR. The HLSL backend must not special-case it.

Before M16, `comptime match` could choose among fixed `reg_tile` shapes or fixed explicit accumulator writeouts inside one specialized shader variant. M16 adds constrained `comptime for` for structured repeated expansion without generated identifiers.

M14a adds `comptime when utility` as the utility-scored arbitration sibling to `comptime match`. Use `comptime match` when selecting by literal pattern over one scrutinee; use `comptime when utility` when multiple guarded candidates can qualify and should compete by compile-time score.

## Examples

- `Examples/SDSL-V/M14/ComptimeMatchInt.sdslv`
- `Examples/SDSL-V/M14/ComptimeMatchBool.sdslv`
- `Examples/SDSL-V/M14/ComptimeMatchTileConfig.sdslv`
