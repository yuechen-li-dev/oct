# SDSL-V M14a: Constrained `comptime when utility`

M14a adds constrained compile-time utility arbitration for shader staging.

This is not runtime branching, not `comptime match` over one scrutinee, and not Zig-style arbitrary comptime. Templates/configs still choose concrete shader variants; `comptime when utility` chooses among guarded, scored structural alternatives inside one already-specialized shader.

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

VD-MIR remains the compiler boundary. HLSL emits from VD-MIR and does not understand `comptime when utility`.

## Syntax

```sdslv
comptime when utility {
    case Vector4 when C.UseVectorizedLoad and C.Tile.K == 16u score 100 {
        static assert C.Tile.K == 16u;
    }

    case Scalar score 10 {
        static assert false;
    }

    else {
        static assert false;
    }
}
```

Each `case` has a metadata label, an optional compile-time guard, a required compile-time score, and a statement body. A case without `when` is always eligible. `else` has no score and is selected only when no case qualifies.

## Selection

Expansion evaluates cases in source order:

- evaluate the guard, or treat the case as eligible when the guard is omitted;
- evaluate the score for eligible cases;
- select the eligible case with the highest score;
- select `else` only when no case qualifies;
- reject when no case qualifies and no `else` exists;
- reject tied highest scores as ambiguous in M14a.

Only the selected case or selected `else` body is spliced into the AST. Non-selected bodies are parsed, then dropped before VD-MIR and HLSL. `static assert` statements in non-selected bodies do not fire.

Example tied-score diagnostic:

```text
ambiguous comptime when utility cases First and Second have tied score 10
```

## Labels

Case labels are identifiers or dotted identifier names:

```sdslv
case UseVector4 score 100 { ... }
case Policy.UseScalar score 10 { ... }
```

Labels are compile-time metadata for diagnostics and readability. They do not introduce runtime values and are not emitted to HLSL. Duplicate labels within one `comptime when utility` are rejected.

## Compile-Time Expressions

Guards must evaluate to compile-time `bool`. Scores must evaluate to compile-time numeric values; the M14a implementation accepts integer scores through the existing constant evaluator.

Allowed guard and score inputs include:

- integer and bool literals;
- resolved config fields after template/config specialization;
- prior `comptime let` values in scope;
- arithmetic, comparison, modulo, boolean, and parenthesized expressions already supported by consteval.

SDSL-V source uses `and`, `or`, and `not` in these compile-time guards and scores. Backend punctuation belongs only to emitted HLSL.

Forbidden inputs include runtime parameters and push constants, shader resources, thread builtins, workgroup memory, runtime locals, matrix views, tile reads or writes, reductions, match payloads, and runtime function calls.

Example diagnostics:

- `comptime when guard must be compile-time bool`
- `comptime when score must be compile-time numeric`
- `comptime when guard cannot reference runtime parameter 'params.M'`
- `comptime when score cannot reference runtime local 'x'`

## Relationship to Other Branching

`comptime if` is binary compile-time selection.

`comptime match` is compile-time selection by literal pattern against one scrutinee.

`comptime when utility` is compile-time utility-scored arbitration among multiple eligible candidates. Multiple cases may qualify; the highest score wins, and equal highest scores are ambiguous in M14a.

Ordinary/runtime `when utility` remains an expression that lowers through VD-MIR. `comptime when utility` is a statement that expands away before VD-MIR. The HLSL backend must not special-case it.

M19 adds a separate Oct-style runtime guard `when` statement:

```sdslv
when {
    case guard -> { ... }
    else -> { ... }
}
```

That form is source-order first-match control flow. It is not utility scoring and it does not disappear before VD-MIR; it lowers to VD-MIR `IfStmt` chains. See `docs/SDSL_V_M19_RUNTIME_GUARD_WHEN.md`.

## Limits

M14a intentionally does not add in this milestone:

- comptime functions;
- generated identifiers;
- payload labels or destructuring;
- arbitrary compile-time execution;
- runtime dispatch changes;
- selector retuning;
- P15/P14 changes;
- FFT/P16 changes.

M16 later adds constrained `comptime for` on the same staging pass; see `docs/SDSL_V_M16_COMPTIME_FOR.md`.

M15 keeps that boundary intact: `reg_tile` gives structured per-thread accumulator storage, and M16 later adds `comptime for` as the constrained mechanism for generating repeated indexed logic over that structure.

## Examples

- `Examples/SDSL-V/M14a/ComptimeWhenUtilityBasic.sdslv`
- `Examples/SDSL-V/M14a/ComptimeWhenUtilityTieReject.sdslv`
- `Examples/SDSL-V/M14a/ComptimeWhenUtilityTilePolicy.sdslv`
