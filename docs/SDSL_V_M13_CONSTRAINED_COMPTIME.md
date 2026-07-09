# SDSL-V M13: Constrained Comptime M0

SDSL-V M13 adds constrained compile-time shader staging for code inside an already specialized shader variant.

This is not arbitrary compile-time execution. Templates/configs choose which concrete shader variants exist; `comptime` shapes code inside one concrete specialized shader.

The pipeline is:

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

VD-MIR remains the compiler boundary. HLSL emits from VD-MIR and does not understand `comptime`.

## `comptime let`

```sdslv
comptime let TileElements: u32 = C.Tile.M * C.Tile.K;
comptime let UseTailGuard: bool = C.Tile.K % C.Unroll.K != 0u;
```

Rules:

- `comptime let` must have an initializer.
- The initializer must be a compile-time expression.
- The binding follows lexical block scope.
- Runtime code may read the binding as a constant.
- The statement is removed before VD-MIR lowering.

## `comptime if`

```sdslv
comptime if C.Tile.K == 16u {
    static assert C.Unroll.K == 16u;
} else {
    static assert C.Unroll.K <= C.Tile.K;
}
```

The condition must evaluate to a compile-time `bool`. Only the selected branch remains after expansion. The non-selected branch is parsed but does not lower to VD-MIR or HLSL, and `static assert` statements in that branch do not fire.

Runtime statements may appear inside the selected branch:

```sdslv
comptime if C.UseFastPath {
    let scale: f32 = 1.0;
} else {
    let scale: f32 = 0.5;
}
```

Nested `comptime if` is supported. `comptime for`, comptime functions, generated identifiers, reflection, file I/O, process calls, heap objects, recursion, and runtime resource inspection are not part of M13.

## Compile-Time Expressions

M13 reuses the existing constant-expression machinery used by `require` and `static assert`.

Allowed inputs include:

- integer and bool literals;
- config fields after template/config specialization;
- prior `comptime let` values in scope;
- arithmetic, comparison, modulo, boolean, and parenthesized expressions already supported by consteval.

Forbidden inputs include:

- runtime parameters and push constants;
- shader resources;
- thread/system builtins such as `DispatchThreadID`, `GroupID`, `GroupThreadID`, and `GroupIndex`;
- workgroup memory values;
- runtime local `let` values;
- matrix views, tile reads/writes, reductions, match payloads, and runtime function results.

If an expression is not provably compile-time, it is rejected.

Examples of diagnostics:

- `comptime expression cannot reference runtime parameter params.M`
- `comptime expression cannot reference thread builtin GroupThreadID`
- `comptime if condition must be compile-time bool`
- `comptime let initializer must be compile-time`

## Relationship to `require` and `static assert`

`require` remains concept/config-level and is not general shader code.

`static assert` remains a compile-time fact check. M13 additionally allows `static assert` inside shader function bodies so it can be guarded by `comptime if`. A selected branch assertion is evaluated during expansion. A non-selected branch assertion is dropped.

## Limits

M13 intentionally does not add:

- Zig-style arbitrary comptime;
- `comptime for`;
- comptime functions;
- generated identifiers;
- reflection;
- runtime resource inspection;
- selector retuning;
- Prometheus runtime dispatch changes;
- P15/P14 changes;
- FFT/P16 changes.

Future `comptime for` can build on the same constrained staging pass, but it is deferred.

M14 extends the same constrained staging model with `comptime match` for multi-way compile-time selection. See `docs/SDSL_V_M14_COMPTIME_MATCH.md`.

## Examples

- `examples/SDSL-V/M13/ComptimeLet.sdslv`
- `examples/SDSL-V/M13/ComptimeIf.sdslv`
- `examples/SDSL-V/M13/ComptimeTileConfig.sdslv`
