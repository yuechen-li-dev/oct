#!/usr/bin/env python3
"""Reference-only Z-Image Turbo capture through the installed Comfy runtime.

This script deliberately imports ComfyUI's real model classes.  It is not a
Prometheus runtime dependency and writes all tensor payloads below OCT_EVT2_CACHE.
"""

import argparse
import hashlib
import json
import os
import platform
import subprocess
import sys
import time
from pathlib import Path


PROMPT = "A lighthouse in fog at dawn"
MODEL_REVISION = "f332072aa78be7aecdf3ee76d5c247082da564a6"
CHECKPOINT_SHA256 = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def tensor_bytes(tensor):
    import torch
    value = tensor.detach().contiguous()
    if value.device.type != "cpu":
        value = value.to("cpu")
    return value.view(torch.uint8).numpy().tobytes()


class Capture:
    def __init__(self, root):
        self.root = root
        self.root.mkdir(parents=True, exist_ok=True)
        self.artifacts = {}
        self.summaries = {}

    def save(self, name, tensor, required_full=False, stage=None):
        data = tensor_bytes(tensor)
        safe_name = name.replace("/", "_")
        path = self.root / (safe_name + ".bin")
        temporary = path.with_suffix(".bin.tmp")
        temporary.write_bytes(data)
        temporary.replace(path)
        self.artifacts[name] = {
            "cache_relative_path": path.relative_to(self.root.parents[2]).as_posix(),
            "sha256": sha256_bytes(data),
            "shape": list(tensor.shape),
            "dtype": str(tensor.dtype).replace("torch.", ""),
            "stride": list(tensor.stride()),
            "contiguous": tensor.is_contiguous(),
            "device": str(tensor.device),
            "element_count": tensor.numel(),
            "byte_count": len(data),
            "required_full": required_full,
            "stage": stage or name,
        }
        return self.artifacts[name]

    def summarize(self, name, tensor, stage=None):
        import torch
        value = tensor.detach().float()
        flat = value.reshape(-1)
        count = int(flat.numel())
        indices = sorted(set([0, min(1, count - 1), count // 3, count // 2, max(0, count - 2), count - 1]))
        samples = [float(flat[index].item()) for index in indices]
        # Fixed signed/absolute projections avoid recording another large tensor.
        weights = ((torch.arange(count, device=value.device, dtype=torch.float32) % 29.0) - 14.0) / 14.0
        signed = float(torch.dot(flat, weights).item())
        absolute = float(torch.dot(flat.abs(), weights.abs()).item())
        self.summaries[name] = {
            "stage": stage or name,
            "shape": list(tensor.shape),
            "dtype": str(tensor.dtype).replace("torch.", ""),
            "stride": list(tensor.stride()),
            "contiguous": tensor.is_contiguous(),
            "finite": bool(torch.isfinite(value).all().item()),
            "rms": float(value.square().mean().sqrt().item()),
            "l2": float(torch.linalg.vector_norm(flat).item()),
            "linf": float(flat.abs().max().item()),
            "sample_indices": indices,
            "sample_values": samples,
            "signed_projection": signed,
            "absolute_projection": absolute,
        }


def jsonable(value):
    import torch
    if isinstance(value, torch.Tensor):
        return {"shape": list(value.shape), "dtype": str(value.dtype), "values": value.detach().cpu().reshape(-1).tolist()}
    if isinstance(value, dict):
        return {str(key): jsonable(item) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [jsonable(item) for item in value]
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    return repr(value)


def environment_identity(comfy_root):
    import safetensors
    import torch
    import transformers
    identity = {
        "python_executable": sys.executable,
        "python_version": sys.version,
        "platform": platform.platform(),
        "torch": torch.__version__,
        "cuda_runtime": torch.version.cuda,
        "cudnn": torch.backends.cudnn.version(),
        "transformers": transformers.__version__,
        "safetensors": safetensors.__version__,
        "comfy_root_sha256": sha256_bytes((comfy_root / "comfy" / "ldm" / "lumina" / "model.py").read_bytes()),
        "comfy_model_source": "installed ComfyUI Lumina/Z-Image implementation",
        "gpu": torch.cuda.get_device_name(0) if torch.cuda.is_available() else None,
        "gpu_capability": list(torch.cuda.get_device_capability(0)) if torch.cuda.is_available() else None,
        "attention_backend_environment": {
            "sage": os.environ.get("COMFYUI_SAGE_ATTENTION"),
            "xformers": os.environ.get("COMFYUI_XFORMERS"),
            "flash": os.environ.get("COMFYUI_FLASH_ATTENTION"),
        },
        "determinism": {
            "torch_deterministic_algorithms": torch.are_deterministic_algorithms_enabled(),
            "cudnn_deterministic": torch.backends.cudnn.deterministic,
            "cudnn_benchmark": torch.backends.cudnn.benchmark,
        },
    }
    try:
        identity["nvidia_smi"] = subprocess.check_output(
            ["nvidia-smi", "--query-gpu=driver_version,name,uuid", "--format=csv,noheader"], text=True).strip()
    except Exception as error:
        identity["nvidia_smi_error"] = repr(error)
    return identity


def configure_model_paths(data_root):
    import folder_paths
    models = data_root / "models"
    for name, directory in (("diffusion_models", "diffusion_models"), ("text_encoders", "text_encoders"), ("vae", "vae"), ("embeddings", "embeddings")):
        folder_paths.add_model_folder_path(name, str(models / directory), is_default=True)


def install_block_hooks(diffusion_model, capture, first_call):
    import comfy.ldm.lumina.model as lumina_model
    block = diffusion_model.noise_refiner[0]
    original_rope = lumina_model.apply_rope
    original_attention = lumina_model.optimized_attention_masked

    def once_summary(name, tensor, stage):
        if not first_call["active"] or name in capture.summaries:
            return
        capture.summarize(name, tensor, stage)

    def module_pre(name, stage):
        def hook(_module, arguments):
            if arguments:
                once_summary(name, arguments[0], stage)
        return hook

    def module_post(name, stage):
        def hook(_module, _arguments, output):
            if isinstance(output, tuple):
                output = output[0]
            once_summary(name, output, stage)
        return hook

    def block_pre(_module, arguments):
        if first_call["seen"]:
            return
        first_call["active"] = True
        first_call["seen"] = True
        capture.save("noise_refiner_0_input", arguments[0], required_full=True, stage="noise_refiner.0 pre-modulation input")
        capture.save("noise_refiner_0_timestep", arguments[3], required_full=True, stage="noise_refiner.0 timestep conditioning")
        once_summary("noise_refiner_0_rope", arguments[2], "noise_refiner.0 three-axis RoPE frequencies")

    def block_post(_module, _arguments, output):
        if first_call["active"] and "noise_refiner_0_output" not in capture.artifacts:
            capture.save("noise_refiner_0_output", output, required_full=True, stage="noise_refiner.0 final gated residual output")
        first_call["active"] = False

    def rope_hook(q, k, freqs):
        q_out, k_out = original_rope(q, k, freqs)
        once_summary("q_after_rope", q_out, "noise_refiner.0 Q after three-axis RoPE")
        once_summary("k_after_rope", k_out, "noise_refiner.0 K after three-axis RoPE")
        return q_out, k_out

    def attention_hook(q, k, v, heads, *arguments, **kwargs):
        output = original_attention(q, k, v, heads, *arguments, **kwargs)
        once_summary("attention_output", output, "noise_refiner.0 attention output before output projection")
        if first_call["active"] and "attention_selected_logits" not in capture.summaries:
            # This is an explicitly derived witness; the backend may fuse its real logits.
            q_row = q[0, 0, 0].float()
            k_rows = k[0, 0].float()
            logits = (k_rows @ q_row) * (q.shape[-1] ** -0.5)
            probabilities = logits.softmax(dim=-1)
            capture.summarize("attention_selected_logits", logits, "derived head-0 token-0 FP32 attention logits")
            capture.summarize("attention_selected_probabilities", probabilities, "derived head-0 token-0 FP32 softmax probabilities")
        return output

    hooks = [block.register_forward_pre_hook(block_pre), block.register_forward_hook(block_post)]
    hooks += [
        block.adaLN_modulation.register_forward_hook(module_post("adaln_modulation", "noise_refiner.0 AdaLN linear output")),
        block.attention_norm1.register_forward_pre_hook(module_pre("attention_norm1_input", "noise_refiner.0 pre-attention normalization input")),
        block.attention_norm1.register_forward_hook(module_post("attention_norm1_output", "noise_refiner.0 normalized attention input")),
        block.attention.qkv.register_forward_pre_hook(module_pre("qkv_projection_input", "noise_refiner.0 modulated attention input")),
        block.attention.qkv.register_forward_hook(module_post("qkv_projection", "noise_refiner.0 fused QKV projection output")),
        block.attention.q_norm.register_forward_hook(module_post("q_after_qk_rmsnorm", "noise_refiner.0 Q after QK RMSNorm")),
        block.attention.k_norm.register_forward_hook(module_post("k_after_qk_rmsnorm", "noise_refiner.0 K after QK RMSNorm")),
        block.attention.out.register_forward_hook(module_post("attention_out_projection", "noise_refiner.0 attention output projection")),
        block.attention_norm2.register_forward_hook(module_post("attention_output_rmsnorm", "noise_refiner.0 attention output RMSNorm")),
        block.ffn_norm1.register_forward_pre_hook(module_pre("ffn_norm1_input", "noise_refiner.0 gated attention residual result")),
        block.ffn_norm1.register_forward_hook(module_post("ffn_norm1_output", "noise_refiner.0 normalized FFN input")),
        block.feed_forward.w1.register_forward_hook(module_post("ffn_w1", "noise_refiner.0 FFN W1 output")),
        block.feed_forward.w3.register_forward_hook(module_post("ffn_w3", "noise_refiner.0 FFN W3 output")),
        block.feed_forward.w2.register_forward_pre_hook(module_pre("ffn_silu_w1_times_w3", "noise_refiner.0 SiLU(W1) times W3")),
        block.feed_forward.w2.register_forward_hook(module_post("ffn_w2", "noise_refiner.0 FFN W2 output")),
        block.ffn_norm2.register_forward_hook(module_post("ffn_output_rmsnorm", "noise_refiner.0 FFN output RMSNorm")),
    ]
    lumina_model.apply_rope = rope_hook
    lumina_model.optimized_attention_masked = attention_hook
    return hooks, lumina_model, original_rope, original_attention


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--comfy-root", required=True, type=Path)
    parser.add_argument("--data-root", required=True, type=Path)
    parser.add_argument("--cache-root", required=True, type=Path)
    parser.add_argument("--run", required=True, type=int)
    arguments = parser.parse_args()
    if not (arguments.comfy_root / "nodes.py").is_file():
        raise SystemExit("--comfy-root must name the installed ComfyUI source root")
    sys.path.insert(0, str(arguments.comfy_root))

    import torch
    import comfy.model_management
    import comfy.sample
    import comfy.samplers
    import nodes
    from comfy_extras.nodes_model_advanced import ModelSamplingAuraFlow

    run_root = arguments.cache_root / "oracle" / MODEL_REVISION / ("run_%02d" % arguments.run)
    capture = Capture(run_root)
    configure_model_paths(arguments.data_root)
    metadata = {
        "schema": "prometheus.evt2.zimage.reference-capture.v1",
        "model_revision": MODEL_REVISION,
        "checkpoint_sha256": CHECKPOINT_SHA256,
        "prompt": PROMPT,
        "seed": 42,
        "resolution": [512, 512],
        "guidance": 0.0,
        "comfy_cfg": 1.0,
        "guidance_mapping": "ComfyUI CFG=1 selects the single conditional branch; frozen Turbo guidance=0 does not request a negative/unconditional branch.",
        "sampler": "res_multistep",
        "scheduler": "simple",
        "scheduler_steps": 9,
        "expected_denoiser_evaluations": 8,
        "aura_flow_shift": 3.0,
        "negative_prompt": "default ConditioningZeroOut behavior",
        "lora": "disabled",
        "controlnet": "disabled",
        "environment": environment_identity(arguments.comfy_root),
    }

    # Text encoder is deliberately released before the denoiser is loaded.
    clip = nodes.CLIPLoader().load_clip("qwen_3_4b.safetensors", "lumina2", "default")[0]
    tokens = clip.tokenize(PROMPT)
    positive = clip.encode_from_tokens_scheduled(tokens)
    (negative,) = nodes.ConditioningZeroOut().zero_out(positive)
    (arguments.cache_root / "oracle" / MODEL_REVISION / ("run_%02d_tokens.json" % arguments.run)).write_text(json.dumps(jsonable(tokens), indent=2, sort_keys=True), encoding="utf-8")
    capture.save("prompt_embeddings", positive[0][0], required_full=True, stage="text conditioning consumed by denoiser")
    if positive[0][1].get("pooled_output") is not None:
        capture.save("pooled_text_conditioning", positive[0][1]["pooled_output"], required_full=True, stage="pooled text conditioning")
    del clip
    comfy.model_management.unload_all_models()
    comfy.model_management.soft_empty_cache()

    latent = {"samples": torch.zeros((1, 16, 64, 64), dtype=torch.float32, device="cpu")}
    model = nodes.UNETLoader().load_unet("z_image_turbo_bf16.safetensors", "default")[0]
    model = ModelSamplingAuraFlow().patch_aura(model, 3.0)[0]
    diffusion_model = model.model.diffusion_model
    first_call = {"active": False, "seen": False}
    hooks, lumina_model, original_rope, original_attention = install_block_hooks(diffusion_model, capture, first_call)

    original_noise = comfy.sample.prepare_noise
    def noise_hook(latent_image, seed, noise_inds=None):
        noise = original_noise(latent_image, seed, noise_inds)
        if "initial_noise" not in capture.artifacts:
            capture.save("initial_noise", noise, required_full=True, stage="initial CPU-seeded noise latent")
        return noise
    comfy.sample.prepare_noise = noise_hook
    original_forward = diffusion_model.forward
    def denoiser_forward(*forward_args, **forward_kwargs):
        if "denoiser_input_step0" not in capture.artifacts:
            capture.save("denoiser_input_step0", forward_args[0], required_full=True, stage="diffusion model input at step 0")
        output = original_forward(*forward_args, **forward_kwargs)
        if "denoiser_output_step0" not in capture.artifacts:
            capture.save("denoiser_output_step0", output, required_full=True, stage="diffusion model output at step 0")
        return output
    diffusion_model.forward = denoiser_forward

    noise = comfy.sample.prepare_noise(latent["samples"], 42)
    sampler = comfy.samplers.KSampler(model, steps=9, device=model.load_device, sampler="res_multistep", scheduler="simple", denoise=1.0, model_options=model.model_options)
    metadata["sigmas"] = sampler.sigmas.detach().cpu().tolist()
    capture.save("initial_patchified_image_tokens", latent["samples"], required_full=True, stage="initial latent before noise scaling")
    start = time.perf_counter()
    step0 = sampler.sample(noise, positive, negative, cfg=1.0, latent_image=latent["samples"], start_step=0, last_step=1, force_full_denoise=False, disable_pbar=True, seed=42)
    capture.save("latent_after_step0", step0, required_full=True, stage="latent after scheduler update step 0")
    final = sampler.sample(noise, positive, negative, cfg=1.0, latent_image=latent["samples"], disable_pbar=True, seed=42)
    metadata["denoiser_execution_seconds"] = time.perf_counter() - start
    capture.save("final_latent", final, required_full=True, stage="latent after all denoising evaluations")

    diffusion_model.forward = original_forward
    comfy.sample.prepare_noise = original_noise
    lumina_model.apply_rope = original_rope
    lumina_model.optimized_attention_masked = original_attention
    for hook in hooks:
        hook.remove()
    del model
    comfy.model_management.unload_all_models()
    comfy.model_management.soft_empty_cache()

    vae = nodes.VAELoader().load_vae("ae.safetensors")[0]
    final_dict = {"samples": final}
    capture.save("vae_input", final, required_full=True, stage="VAE input latent")
    image = nodes.VAEDecode().decode(vae, final_dict)[0]
    capture.save("decoded_image_tensor", image, required_full=True, stage="decoded image tensor")
    from PIL import Image
    png_path = run_root / "final.png"
    Image.fromarray((image[0].detach().clamp(0, 1).cpu().numpy() * 255.0).round().astype("uint8")).save(png_path)
    metadata["final_png"] = {"cache_relative_path": png_path.relative_to(arguments.cache_root).as_posix(), "sha256": sha256_bytes(png_path.read_bytes()), "bytes": png_path.stat().st_size}
    metadata["artifacts"] = capture.artifacts
    metadata["internal_stage_summaries"] = capture.summaries
    metadata_path = run_root / "capture.json"
    metadata_path.write_text(json.dumps(metadata, indent=2, sort_keys=True), encoding="utf-8")
    print(metadata_path)


if __name__ == "__main__":
    main()
