# SDSL-V grouped semantic spaces

## Outcome

- Convergence: **SUCCESS**
- Scope: parser/AST sugar over existing nominal semantic-space aliases
- Runtime or ABI effect: none

The accepted syntax is:

```sdslv
space zimage.attention {
    QueryHead: float4;
    KeyHead: float4;
    ValueHead: float4;
    PositionedQueryHead: float4;
    PositionedKeyHead: float4;
    Score: float4;
    Probability: float4;
    Output: float4;
}
```

The parser retains a `SpaceGroupDecl` long enough to preserve source spans and
then expands it before ordinary validation and lowering. Members remain normal
lexical type aliases. PascalCase ASCII member names become lower snake case;
acronym runs stay together. The first and sixth members above expand to:

```sdslv
type QueryHead = float4 @space(zimage.attention.query_head);
type Score = float4 @space(zimage.attention.score);
```

There is no nested or generic group, structural object type, relation
declaration, pairing rule, conversion, or tensor-axis meaning. Function
signatures remain the only transformation and pairing authority.

## Diagnostics

- `SDSL-V1509`: a generated type name duplicates another top-level name;
- `SDSL-V4124`: two direct declarations produce the same semantic space
  identity. The diagnostic points at the collision and relates the first
  declaration.

`QKVHead` and `QkvHead`, for example, both canonicalize to `qkv_head` and are
rejected rather than depending on declaration order.

## Equivalence proof

The attention PoC is the primary grouped migration fixture. A matched grouped
and explicit control additionally proves exact backend equivalence:

| Artifact | Grouped | Expanded | Result |
|---|---|---|---|
| HLSL SHA-256 | `51502080c6e6d5776cc5eb88aa9caa18b071ce406b16d2ab19eb4282cff93468` | same | byte-identical |
| SPIR-V SHA-256 | `7c821f28dd528bcaccc0dcbf6999d73732721520afbaa8afb81e377355122b3a` | same | byte-identical; both pass `spirv-val` |

VD-MIR dumps expose the same resolved `@space(...)` values in both forms. No
tag, branch, descriptor, field, storage, push constant, or layout is added.

The attention declaration block is materially clearer: eight repeated
`type ... @space(zimage.attention...)` declarations become one named family
with eight short members, while all legal transitions remain explicit
functions.
