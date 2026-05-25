# P16 M1 — Prometheus FFT Reactor Contract Audit

## Scope and outcome classification

This milestone is **documentation-only** and intentionally leaves runtime behavior unchanged.

Convergence classification: **Success** (architecture/contract audit completed with explicit next-step recommendation and no runtime modifications).

---

## 1) Current-state audit

### 1.1 Reactor family topology status

Current topology is family-split and already reserves FFT/fused-reduction files:

- `reactor_vulkan_common.c`: shared Vulkan helpers only.
- `reactor_vulkan_sgemm.c`: complete SGEMM family runtime.
- `reactor_vulkan_fft.c`: inert FFT placeholder.
- `reactor_vulkan_fused_reduction.c`: inert fused-reduction placeholder.
- `reactor_vulkan.h`: shared internal declarations + runtime impl declarations used by API forwarders.

This matches the documented topology rule: split by compute primitive family, not SGEMM-internal subsystems.

### 1.2 FFT stub status (truthful non-claim)

`reactor_vulkan_fft.c` is include + comment only, with explicit non-claims:

- no public ABI symbols,
- no capability reporting,
- no probe/runtime behavior changes.

So FFT is currently topology-reserved but contract-inert.

### 1.3 Current public API/caps shape

Public C API is SGEMM-centric and currently exposes:

- runtime lifecycle (`create/destroy/probe`),
- SGEMM entrypoints (sync, benchmark-variant seam, batch, async),
- SGEMM policy/batch diagnostics.

`PrometheusCaps` probe reports only overall backend/runtime availability semantics (available/backend/reason), not per-family FFT capability states.

### 1.4 Build integration status

Both native build helpers compile and link FFT/fused stubs in active source lists:

- Linux: `internal/prometheus/native/build_stub.sh`
- Windows: `internal/prometheus/native/build_windows.cmd`

This means topology presence is verified at build time without behavioral wiring.

### 1.5 Existing test conventions relevant to FFT planning

Marionette conventions already establish patterns we should reuse:

- **truthful availability/probe semantics** (available vs unavailable lane behavior),
- **CPU oracle parity tests** for SGEMM correctness lanes,
- **diagnostics-first milestones** with defaults and explicit non-enabled fields,
- **benchmark seam truthfulness** (requested vs executed path identity and fallback reasons),
- **selector cache reuse/recompute tests** driven by bounded dependency keys.

### 1.6 SGEMM patterns to reuse for FFT

Reuse these SGEMM-proven patterns:

1. **Benchmark-only seam before production enablement** (requested path can differ from executed path, reported truthfully).
2. **Default-off capability truthfulness** (no fake claims in probe/caps).
3. **Deterministic diagnostics export** with explicit fields and stable default values.
4. **Bounded explicit caches** (no generic dynamic policy soup).
5. **Clear family ownership** in one reactor family file with shared helpers only in `common`.

### 1.7 SGEMM patterns not to copy into FFT

Do **not** copy these SGEMM-specific internals into FFT baseline:

- Dominatus SGEMM adaptation/state projections as a prerequisite for first FFT correctness.
- SGEMM layout-precision policy machinery (Packed4/FP16 selector paths).
- SGEMM typed-arena artifact invalidation semantics tied to matrix roles A/B/C/upload.
- SGEMM-specific queue/transfer heuristics unless they become genuine cross-family common helpers.

FFT should start with FFT-native plan/execution contracts and only share true Vulkan plumbing.

---

## 2) Proposed FFT API surface (proposal only, no implementation in M1)

### 2.1 Types

```c
typedef struct PrometheusComplex32 {
  float real;
  float imag;
} PrometheusComplex32;
```

```c
typedef struct PrometheusFftRequest {
  const PrometheusComplex32* input;
  PrometheusComplex32* output;
  uint32_t element_count;     // per transform length N
  uint32_t batch_count;       // number of independent transforms
  uint32_t stride_elements;   // optional explicit stride (0 => contiguous N)
  uint32_t flags;             // direction/normalization/benchmark hints
} PrometheusFftRequest;
```

### 2.2 Flags (initial proposal)

- `PROM_FFT_FLAG_FORWARD` (default if neither direction flag set)
- `PROM_FFT_FLAG_INVERSE`
- `PROM_FFT_FLAG_INVERSE_NORMALIZE`
- `PROM_FFT_FLAG_BENCHMARK_ALLOW_NON_PRODUCTION_PATH`

Direction dual-flag should reject as invalid.

### 2.3 Functions (initial proposal)

- `prometheus_reactor_runtime_fft(...)`
- `prometheus_reactor_runtime_fft_benchmark_variant(...)`
- `prometheus_reactor_runtime_fft_diagnostics(...)`
- internal impl counterparts in `reactor_vulkan.h` / `reactor_vulkan_fft.c`.

### 2.4 Diagnostics struct (initial proposal)

`PrometheusFftDiagnostics` should include (minimum):

- request shape snapshot (N, batch, stride, flags)
- validation status/failure reason
- requested path/radix vs executed path/radix
- pass count
- ping/pong role flow summary
- arena reuse/grow counters
- benchmark/prod eligibility booleans
- selector cache reuse/recompute counters (future fields default zero)

### 2.5 Capability plumbing proposal

Preferred: **caps v2 extension** over family-specific ad-hoc probe, because existing probe already anchors runtime truth and tests. Add FFT family capability fields only when semantics are explicit and default-off.

Interim fallback if ABI pressure exists: separate `prometheus_reactor_runtime_probe_fft_family(...)` that is clearly optional and returns default-absent states until FFT milestones wire behavior.

---

## 3) Capability semantics (required truth model)

Define four explicit states:

1. **Absent**
   - No FFT API symbols in public ABI yet.
   - Probe/caps contain no FFT claim.

2. **API-declared but unavailable**
   - FFT API exists but returns unavailable/not-enabled for all runtime lanes.
   - Probe/caps FFT fields report declared-but-unavailable.

3. **Benchmark-wired**
   - Benchmark FFT entrypoint can execute limited FFT path(s).
   - Production FFT entrypoint remains unavailable or baseline-fallback-only.
   - Diagnostics must report requested/executed/fallback truth.

4. **Production-available**
   - Production FFT entrypoint executes supported contract path(s).
   - Caps report production eligibility only for actually wired modes.

No milestone may skip truthful intermediate semantics.

---

## 4) Plan model (deterministic)

Proposed internal FFT planning types:

- `prom_fft_direction` (forward/inverse)
- `prom_fft_radix` (2 baseline, 4/8 future)
- `prom_fft_buffer_role` (input, ping, pong, output, twiddle)
- `prom_fft_plan_pass`
  - pass_index
  - radix
  - span/stride metadata
  - source_role
  - destination_role
  - twiddle_mode (inline/precomputed)
- `prom_fft_plan`
  - element_count (N)
  - batch_count
  - direction
  - pass_count
  - passes[] (deterministic order)
  - final_output_role

Determinism rules:

- For given request shape + flags + capability lane, pass plan must be stable.
- Tie-breaks must be explicit (e.g., prefer lower radix in baseline milestone).
- Future adaptive selection should preserve inspectable reasoning traces.

---

## 5) Resource ownership model

FFT family should own its own arenas initially:

- **ping arena** (device-local, intermediate)
- **pong arena** (device-local, intermediate)
- **optional twiddle arena** (device-local or host-visible staging-backed)

Rules:

- No SGEMM arena struct reuse in P16 baseline.
- Any future sharing must happen via explicitly shared common infrastructure abstractions, not SGEMM-private coupling.
- Diagnostics must surface per-arena reuse/grow/rebuild/failure counters.

---

## 6) Diagnostics model

Required diagnostic fields across milestones:

1. last request shape (N, batch, stride, flags)
2. validation failure code/reason (e.g., non-power-of-two, overflow, null pointers)
3. selected path id/status
4. requested radix and executed radix (or per-pass sequences)
5. pass count
6. ping/pong role flow (start role, per-pass swap count, final role)
7. arena reuse/grow/rebuild counts
8. benchmark eligibility vs production eligibility booleans
9. future selector cache diagnostics (valid/reuse/recompute/invalidate + dependency mask summary)

These must have stable defaults in API-declared-but-unavailable state.

---

## 7) Test strategy (Marionette)

Proposed FFT test ladder:

1. **Inert/default state tests**
   - Verify no FFT capability claim while API absent.
2. **Invalid shape rejection tests**
   - null pointers, zero lengths, overflow.
3. **Power-of-two validation tests**
   - reject non-power-of-two lengths with explicit detail codes.
4. **CPU oracle round-trip tests**
   - forward+inverse complex32 with tolerance and optional normalization modes.
5. **Deterministic plan construction tests**
   - same shape/flags produce same pass plan.
6. **Benchmark-only path truthfulness tests**
   - requested variant vs executed variant/fallback reported correctly.
7. **Production capability truthfulness tests**
   - production flag only flips when true runtime path is wired.
8. **Arena diagnostics tests**
   - reuse/grow counters, ping/pong flow visibility.
9. **Selector cache tests (later)**
   - explicit dependency-driven reuse/recompute behavior.

---

## 8) Milestone ladder recommendation (P16 M2+)

Recommended sequence (kept unless later repo evidence dictates change):

- **P16 M2**: FFT ABI skeleton + diagnostics defaults, no capability claim.
- **P16 M3**: CPU oracle + deterministic plan builder.
- **P16 M4**: benchmark-only Vulkan radix-2 path.
- **P16 M5**: production complex32 radix-2 FFT.
- **P16 M6**: batch FFT + ping/pong arena reuse.
- **P16 M7**: radix-4/radix-8 benchmark variants.
- **P16 M8**: adaptive per-pass radix policy.
- **P16 M9**: twiddle strategy selector.
- **P16 M10**: real-to-complex.

Rationale: mirrors prior Prometheus maturation pattern (truthful seams first, production claims last), minimizes fake-capability risk, and keeps family-boundary hygiene.

---

## 9) Immediate next recommendation (exactly one)

**Choose P16 M2 next: FFT ABI skeleton + diagnostics defaults, no capability claim.**

Why this is next:

- It establishes public/internal contract surfaces and test scaffolding with default-off truth semantics.
- It enables CI-visible compatibility checks without forcing premature runtime behavior claims.
- It aligns directly with proven SGEMM pattern: benchmark/production wiring should follow after diagnostics/capability truth model exists.

---

## Inconsistency and documentation-gap audit notes

1. **No FFT contract in current API header despite topology reservation**: expected by design, but this is a deliberate documentation/API gap to close in M2.
2. **SGEMM benchmark seam semantics are explicit; FFT has none yet**: this is a useful precedent to replicate, not an inconsistency.
3. **Current caps are backend-level only**: per-family capability granularity is not yet represented; this is a planned extension point.

