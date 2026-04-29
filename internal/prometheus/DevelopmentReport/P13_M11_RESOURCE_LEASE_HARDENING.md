# P13 M11 — Resource Lease Hardening

## 1) M10 handoff summary
M10 integrated request/grant/deny/execute/yield hooks into single-SGEMM and batch paths with diagnostics-first behavior.

## 2) Invariants defined
- no execution without grant
- no yield without prior grant
- no double-yield
- outstanding depth bounded by configured slots per worker
- slot invalid/fail clears active lease
- per-run diagnostics coherence (request/grant/deny/yield)

## 3) Issues found
- Batch path had no explicit held-lease tracking, so yield relied on decision shape instead of runtime-held state.
- Outstanding depth relied only on `resources->in_flight` and lacked explicit lease-depth invariant tracking.
- Deny-path diagnostics were tested in smoke form but not explicitly cross-checked for output immutability and counter coherence.

## 4) Fixes applied
- Added local batch-runtime `lease_held_mask` + `lease_outstanding_depth` tracking.
- Grant path now marks held lease and increments depth exactly once.
- Failure/slot-failed path now clears held lease and decrements depth if needed.
- Yield path now gated by held lease bit; successful yield clears held bit and decrements depth.
- Added depth overflow hard-stop guard.

## 5) Tests added
- `PrometheusReactor_P13_M11_ResourceLease_DiagnosticsCoherentOnDeny`
- `PrometheusReactor_P13_M11_ResourceLease_RepeatRunCountersStable`

## 6) Diagnostics validation
Validated that deny-only path reports request==deny, grant==0, yield==0, and explicit deny reason. Also validated no output mutation on deny.

## 7) Single-SGEMM behavior clarification
Single-SGEMM lease diagnostics remain conditionally surfaced by backend/runtime availability and should be treated as best-effort in current wiring (existing M10 test keeps explicit SKIP path when unavailable).

## 8) Remaining gaps
- No explicit exported outstanding-depth gauge in batch diagnostics yet.
- Single-SGEMM does not yet enforce a strict public contract that diagnostics must always be non-zero in all configurations.

## 9) Deferred scope
Still deferred: utility policy, lookahead actuation, kernel variant dispatch switching, autotune, work stealing, and performance claims.
