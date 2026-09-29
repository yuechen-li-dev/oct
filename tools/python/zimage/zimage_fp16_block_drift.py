#!/usr/bin/env python3
"""Reference-only noise_refiner.0 FP16-cache substitution witness.

The original and substituted invocations use the captured M1 boundary.  The
FP16 invocation explicitly casts that boundary to FP16 because Comfy's
manual_cast linear layers require matching activation/weight dtypes; this is
therefore an execution witness, not the BF16 oracle authority.
"""
import argparse
import hashlib
import json
import sys
from pathlib import Path


CHECKPOINT_SHA256 = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
MODEL_REVISION = "f332072aa78be7aecdf3ee76d5c247082da564a6"


def digest(tensor):
    import torch
    value = tensor.detach().contiguous().cpu()
    data = value.view(torch.uint8).numpy().tobytes()
    return hashlib.sha256(data).hexdigest(), data


def load_tensor(path, shape, dtype, device):
    import torch
    data = path.read_bytes()
    return torch.frombuffer(bytearray(data), dtype=dtype).reshape(shape).clone().to(device)


def metrics(reference, candidate):
    import torch
    a, b = reference.detach().float(), candidate.detach().float()
    d = b - a
    flat_a, flat_b, flat_d = a.reshape(-1), b.reshape(-1), d.reshape(-1)
    denom = torch.linalg.vector_norm(flat_a).item()
    return {
        "shape": list(reference.shape), "reference_dtype": str(reference.dtype).replace("torch.", ""),
        "candidate_dtype": str(candidate.dtype).replace("torch.", ""),
        "l2": float(torch.linalg.vector_norm(flat_d).item()), "linf": float(flat_d.abs().max().item()),
        "rms": float(flat_d.square().mean().sqrt().item()), "relative_l2": float(torch.linalg.vector_norm(flat_d).item() / denom) if denom else 0.0,
        "cosine": float(torch.nn.functional.cosine_similarity(flat_a, flat_b, dim=0).item()),
        "finite": bool(torch.isfinite(b).all().item()),
    }


def install_summaries(block, output):
    import torch
    stages = {}
    def hook(name):
        def capture(_module, _inputs, value):
            if isinstance(value, tuple): value = value[0]
            stages[name] = value.detach().clone()
        return capture
    handles = [
        block.adaLN_modulation.register_forward_hook(hook("adaln_modulation")),
        block.attention_norm1.register_forward_hook(hook("attention_norm1")),
        block.attention.qkv.register_forward_hook(hook("qkv")),
        block.attention.q_norm.register_forward_hook(hook("q_norm")),
        block.attention.k_norm.register_forward_hook(hook("k_norm")),
        block.attention.out.register_forward_hook(hook("attention_out")),
        block.attention_norm2.register_forward_hook(hook("attention_norm2")),
        block.ffn_norm1.register_forward_hook(hook("ffn_norm1")),
        block.feed_forward.w1.register_forward_hook(hook("ffn_w1")),
        block.feed_forward.w3.register_forward_hook(hook("ffn_w3")),
        block.feed_forward.w2.register_forward_hook(hook("ffn_w2")),
        block.ffn_norm2.register_forward_hook(hook("ffn_norm2")),
    ]
    return stages, handles


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--comfy-root", required=True, type=Path)
    parser.add_argument("--data-root", required=True, type=Path)
    parser.add_argument("--oracle-root", required=True, type=Path)
    parser.add_argument("--cache-root", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    sys.path.insert(0, str(args.comfy_root))
    import torch
    import folder_paths
    import nodes
    from comfy_extras.nodes_model_advanced import ModelSamplingAuraFlow
    for kind, directory in (("diffusion_models", "diffusion_models"),):
        folder_paths.add_model_folder_path(kind, str(args.data_root / "models" / directory), is_default=True)
    run = args.oracle_root / "oracle" / MODEL_REVISION / "run_02"
    device = torch.device("cuda:0")
    x = load_tensor(run / "noise_refiner_0_input.bin", (1, 1024, 3840), torch.bfloat16, device)
    timestep = load_tensor(run / "noise_refiner_0_timestep.bin", (1, 256), torch.bfloat16, device)
    model = ModelSamplingAuraFlow().patch_aura(nodes.UNETLoader().load_unet("z_image_turbo_bf16.safetensors", "default")[0], 3.0)[0]
    diffusion = model.model.diffusion_model
    block = diffusion.noise_refiner[0]
    # Only the M1 block is resident for this isolated reference witness.
    block.to(device)
    ids = __import__("comfy.ldm.lumina.model", fromlist=["pos_ids_x"]).pos_ids_x(16, 32, 32, 1, device)
    freqs = diffusion.rope_embedder(ids).movedim(1, 2)
    # Original authority invocation.
    stages_a, handles = install_summaries(block, None)
    with torch.inference_mode():
        original = block(x, None, freqs, timestep)
    for handle in handles: handle.remove()
    original_hash, _ = digest(original)
    # First establish a Float32-compute control with the original BF16 values.
    # This keeps the activation boundary fixed while separating weight-format
    # drift from the much larger BF16-vs-FP16 activation arithmetic drift.
    for parameter in block.parameters():
        parameter.data = parameter.data.float()
    stages_control, handles = install_summaries(block, None)
    with torch.inference_mode():
        control = block(x.float(), None, freqs.float(), timestep.float())
    for handle in handles: handle.remove()
    # Restore all thirteen cached tensors in original PyTorch orientation.
    manifest_path = args.cache_root / "layers" / CHECKPOINT_SHA256 / "noise_refiner.0" / "manifest.json"
    manifest = json.loads(manifest_path.read_text())
    modules = dict(block.named_modules())
    for item in manifest["tensors"]:
        suffix = item["source_name"].split("noise_refiner.0.", 1)[1]
        module_name, parameter_name = suffix.rsplit(".", 1)
        values = torch.frombuffer(bytearray((manifest_path.parent / item["destination_name"]).read_bytes()), dtype=torch.float16).reshape(item["destination_shape"])
        if item["transpose"]: values = values.t()
        # Keep Float32 execution while retaining only the values representable
        # by the cache's FP16 bytes.
        setattr(modules[module_name], parameter_name, torch.nn.Parameter(values.contiguous().to(device).float(), requires_grad=False))
    # Comfy manual_cast converts FP16 weights to the Float32 input dtype here.
    # Thus the candidate has the exact cached FP16 values but unchanged Float32
    # activation arithmetic relative to the control above.
    stages_b, handles = install_summaries(block, None)
    with torch.inference_mode():
        substituted = block(x.float(), None, freqs.float(), timestep.float())
    for handle in handles: handle.remove()
    substitute_hash, data = digest(substituted)
    args.out.mkdir(parents=True, exist_ok=True)
    output = args.out / "noise_refiner_0_fp16_weight_output.bin"
    output.write_bytes(data)
    result = {
        "schema": "oct.prometheus.evt2m075.fp16-reference-drift.v1", "model_revision": MODEL_REVISION,
        "source_checkpoint_sha256": CHECKPOINT_SHA256, "cache_manifest_sha256": hashlib.sha256(manifest_path.read_bytes()).hexdigest(),
        "substitution": "all 13 cache tensors restored to original PyTorch orientation; Comfy manual_cast expands the exact cached FP16 values to Float32 for an activation-fixed weight-only comparison",
        "original_output_sha256": original_hash, "fp16_weight_output_sha256": substitute_hash,
        "output_cache_relative_path": "oracle/" + MODEL_REVISION + "/m075/noise_refiner_0_fp16_weight_output.bin",
        "bf16_oracle_to_fp16_weight_fp32_compute": metrics(original, substituted),
        "weight_only_output_metrics": metrics(control, substituted),
        "weight_only_stage_metrics": {name: metrics(stages_control[name], stages_b[name]) for name in stages_control if name in stages_b},
    }
    (args.out / "noise_refiner_0_fp16_reference_drift.json").write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(args.out / "noise_refiner_0_fp16_reference_drift.json")


if __name__ == "__main__": main()
