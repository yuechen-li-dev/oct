# Mx101b report — compiled builtin parity sweep (core + tensor)

## Audited builtin families

- Core numeric builtins: `Abs`, `Pi`, `E`, `Sqrt`, `Sin`, `Cos`, `Tan`, `Asin`, `Acos`, `Atan`, `Atan2`, `Exp`, `Ln`, `Log10`, `Sinh`, `Cosh`, `Tanh`.
- Existing core helpers already compiled: `Len`, `Append`, `Print`, `ToString`, `Float`, string helpers, matrix construction helpers, octagon I/O helpers, flow helpers.
- Tensor builtins: `Trace`, `Grad`, `Div`, `SymGrad`.
- Explicitly reviewed but deferred/out-of-scope families: `EinMul`/`EinAdd`, plotting, XLSX, UI builtins.

## Newly lowered in this pass

### Core builtins (lowered and supported)

- Added compiled lowering and return-type resolution for:
  - `Sqrt`, `Sin`, `Cos`, `Tan`, `Asin`, `Acos`, `Atan`, `Atan2`, `Exp`, `Ln`, `Log10`, `Sinh`, `Cosh`, `Tanh`.
- These now emit direct Go `math` calls in compiled artifacts.

### Tensor builtins (lowered and supported)

- Added compiled lowering and return-type resolution for:
  - `Trace`
  - `Grad`
  - `Div`
  - `SymGrad`
- Added compiled runtime helper implementations in linear algebra helper block:
  - `__octTrace`
  - `__octGrad` / `__octGradScalar`
  - `__octDiv` / `__octDivVector`
  - `__octSymGrad`

## CI blockers resolved

- The compiled-mode blocker class around unsupported tensor builtin lowering (e.g. `Trace(...)`) is addressed by direct lowering and runtime helper support.
- The prior compiled-mode `Sqrt` gap used in package integration tests is also addressed.

## Classification summary

### 1) Lowered and supported

- Core: `Abs`, `Pi`, `E`, `Sqrt`, trig/hyperbolic/log numeric set listed above.
- Tensor: `Trace`, `Grad`, `Div`, `SymGrad`.

### 2) Intentionally deferred

- `EinMul`, `EinAdd`: deferred from this pass to keep scope focused on CI-blocking compiled parity and avoid over-expanding into full Einstein contraction lowering complexity.
- Plot builtins (`PlotLine`, `PlotScatter`): left deferred in compiled mode (existing deterministic diagnostics retained).
- XLSX builtins: left deferred in compiled mode (existing deterministic diagnostics retained).

### 3) Out of scope

- UI builtins and UI lowering pipeline (UIIR/WASM).

## Notes

- This pass intentionally preserves language semantics ownership split: Go implements compiler/runtime lowering; language behavior remains in `Language/` contracts.
- Deferred families are explicitly classified to avoid ambiguous parity claims.
