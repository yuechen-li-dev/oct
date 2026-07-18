# PROMETHEUS EVT-2 M1b0-R — Canonical Z-Image Reference

## Current state

Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **RESCOPED**  
EVT-2 state: **READY WITH REQUIRED ADAPTATIONS**  
Reference status: **DIAGNOSTIC AUTHORITY ONLY**

ComfyUI is demoted to compatibility evidence. The canonical source is now the
pinned Tongyi-MAI/Z-Image revision `26f23eda626ffadda020b04ff79488e1d72004cd`.
The repository contains a dependency-free coordinate, RoPE-pair, and RMSNorm
contract seam; it imports neither Torch nor ComfyUI.

## Frame-coordinate resolution

The official `patchify_and_embed` program derives image coordinates before the
`noise_refiner` loop. For the frozen run, 15 text tokens are padded to the
source multiple of 32, adding 17 tokens. Image `create_coordinate_grid` starts
at `cap_ori_len + cap_padding_len + 1`, hence frame `15 + 17 + 1 = 33`.

The canonical image mapping is row-major: token 0 is `[33,0,0]`, token 31 is
`[33,0,31]`, token 32 is `[33,1,0]`, and token 1023 is `[33,31,31]`.
Frame 16 was caused by the historical isolated Comfy script calling
`pos_ids_x(16, ...)` directly, without the pinned source's padded-text
semantics. It is not canonical.

RoPE scalar widths are `[32,48,48]`, theta is 256, and the source rotates
contiguous even/odd pairs as complex values. Q and K are normalized before
RoPE; V bypasses both Q/K RMSNorm and RoPE.

## Derived block program

The source contract freezes AdaLN split order as attention scale, attention
gate, MLP scale, MLP gate. It then uses uncentered RMSNorm, fused logical
Q/K/V ordering, FP32 scaled non-causal attention, output projection, gated
attention residual, SiLU(W1) times W3, W2, and the final gated residual.
The exact shape/orientation contract is recorded in
[`canonical_noise_refiner0_contract.json`](artifacts/Evt2M1b0R/canonical_noise_refiner0_contract.json).

## Remaining work

The narrow Go executor now runs end-to-end from the real cache and captured
boundaries, with no framework imports. The diagnostic final payload hash is
`ea247df111e3230fec9aaf85a742c1635f707a65217818d5a354e09defba83db`.
However, the first projection pass found non-finite values at W2 and the final
output under the declared FP16-cache/fixed-order-FP32 policy. That hash is not
canonical authority. The source-derived frame-33 contract remains valid; the
next bounded seam is to establish the source-justified FP16 overflow policy
before a final witness can be blessed.

## Rescope and successor path

M1b0-R closes as **RESCOPED**. It established the canonical frame-33 RoPE
contract, explicit block operation trace, deterministic cache loading, and the
first non-finite stage under the proposed all-FP16-weight/fixed-order-FP32
policy: W2. The Go executor is retained only as a diagnostic differential
implementation. Its final byte hash is non-authoritative, and no saturation or
clamping policy is inferred from it.

The normative authority path moves to Oct experiments:

- M1b-O0 — Oct experiment vessel and Prometheus primitive seam
- M1b-O1 — per-tensor precision and W2 range study
- M1b-O2 — canonical pre-attention oracle
- M1b-O3 — canonical attention oracle
- M1b-O4 — canonical FFN and complete-block output
- M1b-O5 — reproducible oracle package and differential proof

ComfyUI remains historical compatibility evidence only. M1b-O0 is the exact
next EVT-2 milestone.
