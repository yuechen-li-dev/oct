# PROMETHEUS EVT-2 M1b0 — FP16 Pre-Attention Witnesses

## Outcome

Convergence outcome: **HONEST STOP**  
Milestone state: **IN PROGRESS**  
EVT-2 state: **REQUIRES RESCOPE**  
Reference status: **AUTHORITY BLOCKED**

The local ComfyUI reference environment is restored and the accepted
FP16-weight/FP32-compute block result reproduces exactly.  M1b0 cannot freeze
the requested Q/K RoPE witnesses because two pinned authorities disagree about
the image frame coordinate.  No GPU implementation work was added.

## Restored reference environment

The former `ComfyUI` directory retained models and its virtual environment but
not the source checkout; `folder_paths`, `nodes`, and the Lumina model module
were absent.  The recorded M0.5 Lumina source SHA-256 was matched against
ComfyUI history, recovering commit `635406e283e9c0c8964f2fde3ff1ff4a8b31201e`
in a separate reference checkout.  The raw Git blob of
`comfy/ldm/lumina/model.py` has the recorded SHA-256
`a22f2e9e4d4cb30f09063809eaec45763af24eda121ae955f5e04f26e1849ced`.

The restored checkout was run with the pre-existing environment, without
changing it or copying model weights:

- Python 3.12.9; PyTorch 2.9.1+cu130; CUDA 13.0; cuDNN 91200.
- NVIDIA GeForce RTX 3070; NumPy 2.2.6; einops 0.8.0; safetensors 0.5.2;
  transformers 4.57.3.
- The script registers `<data-root>/models/diffusion_models` through
  `folder_paths.add_model_folder_path`; the existing model directory is reused
  read-only.
- Import smoke test passed for `folder_paths`, `nodes`, and
  `comfy.ldm.lumina.model` from the restored checkout.

The environment lock is
[`fp16_reference_environment_lock.json`](artifacts/Evt2M1b0/fp16_reference_environment_lock.json).
It includes a path-independent command template and archive identity.  A
model-free source archive, `ComfyUI-EVT2-635406e283e9-source.zip`, was created
beside the separate reference checkout; SHA-256
`9c1806bedaa6c79af91c898b3304f997de222e70894d83f8f5a66a141a534978`.

## Accepted final-output reproduction

The unmodified accepted script
[`tools/zimage_fp16_block_drift.py`](../../../tools/zimage_fp16_block_drift.py)
(SHA-256 `2011cb1b4ecb9f4e4eeef2d93e3205f8de8ffdfd52f2e53c775bebf7f1cc6773`)
was run twice against the existing 13-tensor cache, captured input, and
captured timestep.  Both runs produced:

`7e1e6d3d802402a5f0055b6fb257ef04a57cff8166b5925dcce8f9a235f281a7`

The complete drift JSON was byte-identical across the two runs (SHA-256
`50c414c16fbfe2526b059aa097420015499519c52fca285a9770c1efaefe3fedd`).
This is the restored FP16-weight/FP32-compute final-output authority.  The
official BF16 output, `6dae8d91b2118e7c425ee16d5db214ec0d8df1e988487e855aebd1fe81575873`,
remains separate and informational only.

## Why witness generation stopped

The frozen M1b RoPE contract requires image coordinates with frame `33`, rows
and columns `0..31`, axes `[32,48,48]`, and theta `256`.  The M0.5 full BF16
capture's stored RoPE-frequency samples match exactly when the recovered
Comfy source evaluates `pos_ids_x(33, 32, 32, 1)`.

The accepted FP16 script instead explicitly evaluates
`pos_ids_x(16, 32, 32, 1)`.  Its first two frequency values are
`[-0.9576594829559326, 0.2879033088684082]`; the frozen full-capture values
are `[-0.013276747427880764, -0.9999118447303772]`, the frame-33 result.
Consequently, a stage bundle from the accepted isolated script would put the
wrong Q/K RoPE values in front of M1b.  Changing it to frame 33 would make it
a different final-output path, which this milestone is not authorized to
bless.

The complete evidence and required resolution are in
[`fp16_pre_attention_rope_contract.json`](artifacts/Evt2M1b0/fp16_pre_attention_rope_contract.json).
No pre-attention payload, projection, or tolerance was frozen; a
frame-16 bundle would be misleading numerical authority.

## M1b handoff

The restored environment is ready to execute the existing final-output
reference, but it is **not** an M1b stage authority until one invocation is
accepted for both the frame-33 RoPE contract and final-output identity.  M1b
must not implement or validate Q/K RoPE against either side alone.  The
machine-readable handoff is
[`evt2_m1b_handoff.json`](artifacts/Evt2M1b0/evt2_m1b_handoff.json).

The environment was restored successfully; the stage-witness bundle has not
resumed because the evidence exposes a real numerical-authority contradiction,
not a missing dependency.
