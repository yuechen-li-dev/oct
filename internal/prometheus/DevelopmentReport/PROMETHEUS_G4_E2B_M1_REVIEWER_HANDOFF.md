# PROMETHEUS G4-E2B-M1 — raw-score stabilization handoff

## Proven

- The authority-verified checkpoint-backed Vulkan path runs package-backed
  `kernel-69-default` against the exact resident kernel-68 Q and K outputs.
- Fresh-session Q-first and K-first chains each match all `1,800 / 1,800`
  FP32 raw scores bit-exactly against the accepted sequential stage-local
  authority.
- Q, K, and score use bounded reusable resident roles: Q slot 0 (122,880
  bytes), K slot 1 (15,360 bytes), and distinct FP32 score slot 3 (7,200
  bytes). The ring depth is 3. Q/K have no host detour; only the final score
  readback occurs.
- Kernel-69 arithmetic, geometry, indexing, GQA mapping, BF16 decoding,
  sequential coordinate order, and post-sum `1/16` scaling are accepted. Do
  not reopen them without contradictory evidence.

## Open boundary

In one reusable native session, the first chain succeeds. On the second chain,
M46 weight preparation succeeds, but the immediately following M49 required-
weight validation rejects with `PROM_M46_DETAIL_STALE_WEIGHT_GENERATION`
(`-7406`) before any positional dispatch. The failure is the M46-to-M49
required-weight generation/hash handoff after prior score completion; it is
not initial M46 admission, Q/K ordering or binding, resident-ring depth, or
kernel-69 arithmetic.

`observed_weight_generation` and `requested_weight_generation` have been added
to the closed HeadRmsNormRope and score results. They are zero at this M49
early-return boundary because current propagation covers M46 rejection or full
M46/M49 completion, not M49's required-weight rejection. Possible causes
remain intentionally unproved: completion invalidation, missing reacquisition,
cached source metadata, stale slot role metadata, or asymmetric generation
advancement. The exact handoff to inspect is M46 `weight_result.generation` /
`weight_result.hash` into M49 `required_weight_generation` /
`required_weight_hash`.

## Relevant files and commands

- Native lifecycle and diagnostics: `native/reactor_api.c`,
  `native/reactor_api.h`, `native/reactor_vulkan_transformer.c`, and
  `native/reactor_vulkan_runtime_internal.h`.
- Closed Go seam and live harness: `bridge.go`, `bridge_dlopen_windows.go`,
  `bridge_dlopen_linux.go`, `gemma4e2b_m1_rtx.go`, and
  `gemma4e2b_m1_rtx_test.go`.
- Resume command (with validated checkpoint, DLL, hardware, and validation
  environment variables configured):
  `go test -run TestGemma4E2BM1CanonicalQKVRTX -count=1 -v ./internal/prometheus`.

First regroup architecturally: decide whether to finish the handwritten Gemma
layer-0 specimen before extracting the semantic model compiler, or pause
model-specific Vulkan work and establish that compiler boundary now. Do not
automatically resume implementation of the `-7406` defect.
