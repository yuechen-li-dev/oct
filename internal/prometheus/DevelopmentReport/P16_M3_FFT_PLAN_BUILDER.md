# P16 M3 — Prometheus FFT Native Validation + Deterministic Plan Builder

## Scope
This milestone adds C-native FFT request validation and deterministic radix-2 plan metadata construction for Prometheus FFT requests. Execution remains unavailable.

## Semantic Authorities
- `Libraries/Mathematics/Mathematics.Transforms.oct` (`FastFourierTransform`, `IFFT`) remains semantic authority.
- `internal/interpret/fft.go` remains production CPU execution authority for current FFT math behavior.
- `Experiments/PrometheusFftAlgorithmLab/M1` remains algorithm tapeout/audit authority.

## What M3 Implements
- Centralized FFT request validation in native runtime.
- Deterministic radix-2 plan construction metadata for valid requests.
- Deterministic role alternation model (INPUT -> PING, then PING/PONG alternation).
- FFT diagnostics extensions exposing validation and plan truth.
- Marionette coverage for N=1/2/8/16 planning, direction defaults, stride semantics, freshness, benchmark request metadata behavior.

## What M3 Does Not Implement
- No C FFT math implementation.
- No CPU FFT implementation in C.
- No Vulkan FFT shader or Vulkan FFT execution.
- No FFT capability claim or production enablement.

## Request Validation Rules
- request pointer required.
- request struct size must be at least known struct size.
- input/output pointers required.
- element_count > 0 and power-of-two.
- batch_count > 0.
- forward+inverse simultaneously invalid.
- default direction is forward when neither forward/inverse flag set.
- inverse-normalize requires inverse flag.
- stride 0 means effective stride equals element_count.
- non-zero stride must be >= element_count.
- overflow guards for derived count/size calculations.

## Plan Construction Rules
- Radix baseline = 2.
- `N=1` => pass_count=0, log2=0, deterministic output role.
- `N>1` => pass_count=`log2(N)`, spans are 2,4,8,...,N.
- bit-reversal requirement recorded for `N>1`.
- role alternation deterministic: first pass INPUT->PING, then alternate PING/PONG.
- final output role and ping-pong swap count recorded.

## Diagnostics Fields Used
Added plan and validation summary fields to `PrometheusFftDiagnostics`:
- `plan_valid`, `plan_element_count`, `plan_log2_element_count`
- `plan_pass_count`, `plan_first_span`, `plan_last_span`
- `plan_radix_mask`, `plan_bit_reversal_required`
- `plan_first_source_role`, `plan_first_destination_role`
- `plan_direction`, `plan_twiddle_mode`
- existing `ping_pong_swap_count`, `final_output_role` retained for summary.

## Tests Added/Extended
- Expanded `reactor_fft_api_tests.cpp` with deterministic plan checks for:
  - N=1, N=2, N=8, N=16
  - direction handling including inverse defaulting and invalid dual-direction flags
  - inverse-normalize rule
  - stride handling and diagnostics freshness
  - benchmark variant requested-path behavior while execution remains unavailable

## Deferred Scope
- Vulkan radix-2 benchmark execution path.
- Real FFT execution path.
- Batch arena ownership behaviors.
- Radix-4/radix-8 and adaptive per-pass radix.
- Twiddle strategy implementation.
- Real-to-complex transforms.
