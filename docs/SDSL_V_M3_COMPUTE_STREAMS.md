# SDSL-V M3: Compute Streams and `with`

SDSL-V M3 improves the compute authoring model without changing Prometheus runtime dispatch, selector logic, FFT/P16 work, or wiring SGEMM kernels.

The pipeline remains:

```text
SDSL-V source
  -> lex
  -> parse
  -> validate
  -> lower to VD-MIR
  -> emit HLSL
  -> optional DXC / SPIR-V / generated header
```

## What M3 adds

- top-level `stream` declarations for compute-focused structured payloads;
- named resource bundle syntax: `resources BundleName;`;
- conventional `ComputeThread` stream mapping to HLSL system values;
- immutable `with` updates for records and streams;
- validation that rejects direct field mutation through record/stream parameters.

## Compute stream semantics

The language spec originally described `stream` as graphics stage I/O with HLSL semantics.

M3 adds a second meaning for the compute subset:

- graphics stream:
  stage I/O with semantics such as `SV_Position` and `TEXCOORDn`;
- compute stream:
  a structured value used for compute-thread payloads, helper payloads, or resource bundles.

GoOct SDSL-V M3 implements the compute meaning. Full graphics-stream emission remains deferred.

## Compute thread stream

M3 recognizes the conventional stream:

```sdslv
stream ComputeThread {
    DispatchId: uint3;
    GroupId: uint3;
    GroupThreadId: uint3;
    GroupIndex: u32;
}
```

An entry point may accept it as a parameter:

```sdslv
stage compute [numthreads(16, 16, 1)] fn CS(thread: ComputeThread, params: Params) -> void {
    let x: u32 = thread.DispatchId.x;
    return;
}
```

VD-MIR records the thread binding separately from the push-constant/resource interface, and the HLSL backend lowers it to:

- `DispatchThreadID : SV_DispatchThreadID`
- `GroupID : SV_GroupID`
- `GroupThreadID : SV_GroupThreadID`
- `GroupIndex : SV_GroupIndex`

The backend then materializes a local `ComputeThread thread;` value and copies those builtins into its fields so ordinary field access, helper calls, and `with` updates continue to behave like value code.

The legacy builtin names `DispatchThreadID`, `GroupID`, `GroupThreadID`, and `GroupIndex` still remain available in M3.

## Resource bundles

M3 keeps the existing block form:

```sdslv
resources {
    A: readonly array<f32>;
    C: readwrite array<f32>;
}
```

It also adds a named bundle form:

```sdslv
stream VectorAddIO {
    A: readonly array<f32>;
    C: readwrite array<f32>;
}

shader VectorAdd {
    resources VectorAddIO;
}
```

Current M3 rule:

- a named resource bundle must refer to a `stream`;
- every bundle field used as a resource must carry `readonly` or `readwrite`;
- bundle resource fields must currently use `array<T>`;
- binding order remains deterministic declaration order in set `0`.

Current limitation:

- mixed bundle streams that combine access-qualified resources with plain payload fields such as `Params: SgemmParams;` are still deferred.

That keeps M3 narrow while still removing boilerplate from future compute kernels.

## `with` updates

M3 implements immutable updates for records and streams:

```sdslv
let adjusted: SgemmTile = tile with {
    Acc0: tile.Acc0 + value,
};
```

Rules:

- the base must be a record or stream value;
- updated fields must exist on that type;
- duplicate updated fields are rejected;
- each updated value must match the field type;
- the result type matches the base type.

Current supported positions:

- direct `let` initializer;
- direct assignment RHS;
- direct `return` value.

Nested `with` expressions inside call arguments or deeper expression trees are rejected with a clear M3 diagnostic instead of being lowered ambiguously.

## Parameter immutability

Record and stream parameters are immutable for field/path updates.

Rejected:

```sdslv
fn Bad(s: Surface) -> Surface {
    s.Roughness = 0.5;
    return s;
}
```

Allowed:

```sdslv
fn Good(s: Surface) -> Surface {
    return s with { Roughness: 0.5 };
}
```

The same rule applies to stream parameters, including `ComputeThread`.

Local values remain assignable, so a helper may still build or mutate a local struct and then return it.

## HLSL lowering strategy

HLSL has no native record-update expression, so the backend lowers `with` using a deterministic copy/update pattern:

```hlsl
Tile __sdslv_with_0 = tile;
__sdslv_with_0.Acc0 = A[thread.DispatchId.x];
return __sdslv_with_0;
```

The temp names are deterministic within an emitted file.

## Path toward SGEMM kernels

M3 does not wire SGEMM kernels yet.

It gives the compute subset the missing ergonomics needed to author them cleanly:

- explicit compute-thread payloads;
- reusable named resource bundles;
- immutable copy-update style for shader helper records and streams.
