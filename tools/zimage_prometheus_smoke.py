#!/usr/bin/env python3
"""EVT-2 shipping smoke: trusted Comfy preprocessing/scheduler/VAE around Prometheus."""

from __future__ import annotations

import argparse
import gc
import hashlib
import json
import os
import platform
import subprocess
import sys
import time
from pathlib import Path


MODULE_IMPORT_START = time.perf_counter()


PROMPT = "A lighthouse in fog at dawn"
SEED = 42
WIDTH = 512
HEIGHT = 512
STEPS = 9
MODEL_REVISION = "f332072aa78be7aecdf3ee76d5c247082da564a6"
SOURCE_REVISION = "26f23eda626ffadda020b04ff79488e1d72004cd"
CHECKPOINT_SHA256 = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
TEXT_ENCODER_SHA256 = "6c671498573ac2f7a5501502ccce8d2b08ea6ca2f661c458e708f36b36edfc5a"
VAE_SHA256 = "afc8e28272cd15db3919bacdb6918ce9c1ed22e96cb12c4d5ed0fba823529e38"
LOCK_SHA256 = "71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e"
MODEL_EXECUTION_PROFILES = {
    643_587_076: "MinimumMemory",
    1_005_407_748: "Prefetch",
}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(8 * 1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def configure_model_paths(data_root: Path) -> None:
    import folder_paths

    models = data_root / "models"
    for name, directory in (("diffusion_models", "diffusion_models"), ("text_encoders", "text_encoders"), ("vae", "vae"), ("embeddings", "embeddings")):
        folder_paths.add_model_folder_path(name, str(models / directory), is_default=True)


def torch_bf16_bits(tensor):
    import numpy as np
    import torch

    value = tensor.detach().to(dtype=torch.bfloat16).contiguous().cpu()
    result = value.view(torch.uint16).numpy()
    if result.dtype != np.uint16 or not result.flags.c_contiguous:
        raise RuntimeError("failed to construct contiguous NumPy BF16 payload")
    return result


def main() -> None:
    repo = Path(__file__).resolve().parents[1]
    default_payload = Path(os.environ.get("LOCALAPPDATA", "")) / "Oct" / "evt2-z-image-turbo"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--comfy-root", type=Path, default=Path(os.environ.get("OCT_COMFY_ROOT", r"C:\Users\yuech\AppData\Local\Programs\ComfyUI\resources\ComfyUI")))
    parser.add_argument("--data-root", type=Path, default=Path(os.environ.get("OCT_COMFY_DATA", r"C:\Users\yuech\ComfyUI")))
    parser.add_argument("--payload-root", type=Path, default=Path(os.environ.get("OCT_EVT2_CACHE", default_payload)))
    parser.add_argument("--bridge-dll", type=Path, default=repo / "out" / "prometheus" / "python_bridge" / "prometheus_zimage_bridge.dll")
    parser.add_argument("--reactor-dll", type=Path, default=repo / "out" / "prometheus" / "native" / "prometheus_reactor.dll")
    parser.add_argument("--lock", type=Path, default=repo / "internal" / "prometheus" / "models" / "zimage-turbo" / "lock-tagon.octagon")
    parser.add_argument("--execution-profile", choices=("MinimumMemory", "Prefetch"))
    parser.add_argument("--main-attention-route", choices=("Auto", "SerialCanonical", "SubgroupOwned32"), default="Auto")
    parser.add_argument("--demo", action="store_true", help="present the canonical Prefetch workflow in a compact recording-friendly display")
    parser.add_argument("--verbose", action="store_true", help="keep structured diagnostic event output while using --demo")
    parser.add_argument("--no-ansi", action="store_true", help="emit line-based demo updates instead of terminal redraws")
    parser.add_argument("--preflight-one-evaluation", action="store_true")
    parser.add_argument("--output", type=Path, default=default_payload / "shipping_smoke" / "zimage_turbo_prometheus_seed42.png")
    parser.add_argument("--metadata", type=Path, default=repo / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "Evt2Shipping" / "zimage_python_smoke.json")
    args = parser.parse_args()
    if args.execution_profile is None:
        args.execution_profile = "Prefetch" if args.demo else "MinimumMemory"
    if not args.demo or args.verbose:
        print(json.dumps({"event": "process_start", "route": args.main_attention_route,
                          "profile": args.execution_profile, "output": str(args.output),
                          "metadata": str(args.metadata)}), flush=True)

    comfy_root = args.comfy_root.resolve(strict=True)
    data_root = args.data_root.resolve(strict=True)
    payload_root = args.payload_root.resolve(strict=True)
    if not (comfy_root / "nodes.py").is_file():
        raise SystemExit(f"invalid ComfyUI source root: {comfy_root}")
    sys.path.insert(0, str(comfy_root))
    sys.path.insert(0, str(Path(__file__).resolve().parent))

    import numpy as np
    import psutil
    import torch
    import comfy.ldm.common_dit
    import comfy.model_management
    import comfy.sample
    import comfy.samplers
    import nodes
    from PIL import Image
    from comfy.ldm.lumina.model import pad_zimage
    from comfy.ldm.lumina.model import modulate
    from comfy_extras.nodes_model_advanced import ModelSamplingAuraFlow
    from prometheus_zimage_bridge import PrometheusZImageSession
    from zimage_demo_progress import DemoProgress

    gpu_name = torch.cuda.get_device_name(0) if torch.cuda.is_available() else "No CUDA GPU detected"
    if torch.cuda.is_available():
        gpu_name += f" — {torch.cuda.get_device_properties(0).total_memory / (1024 ** 3):.0f} GiB"
    display = DemoProgress(gpu_name, args.execution_profile, {"MinimumMemory": 643_587_076, "Prefetch": 1_005_407_748}[args.execution_profile], ansi=False if args.no_ansi else None) if args.demo else None
    if display:
        display.render()

    torch.manual_seed(SEED)
    wall_start = time.perf_counter()
    timings: dict[str, float] = {"python_module_import_seconds": wall_start - MODULE_IMPORT_START}
    boundary: dict[str, object] = {}
    evidence_rows: list[dict[str, object]] = []
    process = psutil.Process()
    memory_samples: list[dict[str, int | str]] = []
    gpu_telemetry: list[dict[str, object]] = []
    gc_events: list[dict[str, object]] = []
    gc_active: dict[int, float] = {}

    def gc_probe(phase: str, info: dict[str, int]) -> None:
        generation = int(info.get("generation", -1))
        if phase == "start":
            gc_active[generation] = time.perf_counter()
        elif phase == "stop":
            begin = gc_active.pop(generation, time.perf_counter())
            gc_events.append({"generation": generation, "duration_seconds": time.perf_counter() - begin, "collected": int(info.get("collected", 0)), "uncollectable": int(info.get("uncollectable", 0))})

    gc.callbacks.append(gc_probe)

    def sample_gpu(label: str, evaluation_index: int | None = None) -> None:
        if os.environ.get("PROMETHEUS_DVT2_M3_TELEMETRY") != "1":
            return
        query = "timestamp,clocks.gr,clocks.mem,utilization.gpu,power.draw,temperature.gpu,memory.used,clocks_throttle_reasons.active"
        started = time.perf_counter()
        try:
            result = subprocess.run(["nvidia-smi", f"--query-gpu={query}", "--format=csv,noheader,nounits"], capture_output=True, text=True, timeout=5, check=True)
            values = [value.strip() for value in result.stdout.splitlines()[0].split(",")]
            gpu_telemetry.append({"label": label, "evaluation_index": evaluation_index, "sample_overhead_seconds": time.perf_counter() - started, "fields": query.split(","), "values": values, "status": "ok"})
        except (OSError, subprocess.SubprocessError, IndexError) as error:
            gpu_telemetry.append({"label": label, "evaluation_index": evaluation_index, "sample_overhead_seconds": time.perf_counter() - started, "status": "unavailable", "detail": str(error)})

    def sample_memory(label: str) -> None:
        info = process.memory_info()
        virtual = psutil.virtual_memory()
        memory_samples.append({"label": label, "rss_bytes": int(info.rss), "vms_bytes": int(info.vms), "system_available_bytes": int(virtual.available), "system_used_bytes": int(virtual.used)})

    sample_memory("after_python_imports")
    sample_gpu("after_python_imports")

    conditioning_start = time.perf_counter()
    configure_model_paths(data_root)
    timings["model_discovery_seconds"] = time.perf_counter() - conditioning_start
    qwen_load_start = time.perf_counter()
    clip = nodes.CLIPLoader().load_clip("qwen_3_4b.safetensors", "lumina2", "default")[0]
    timings["qwen_load_seconds"] = time.perf_counter() - qwen_load_start
    tokenization_start = time.perf_counter()
    tokens = clip.tokenize(PROMPT)
    timings["tokenization_seconds"] = time.perf_counter() - tokenization_start
    qwen_encode_start = time.perf_counter()
    positive = clip.encode_from_tokens_scheduled(tokens)
    timings["qwen_encode_seconds"] = time.perf_counter() - qwen_encode_start
    (negative,) = nodes.ConditioningZeroOut().zero_out(positive)
    prompt_embedding = positive[0][0]
    if tuple(prompt_embedding.shape) != (1, 15, 2560) or prompt_embedding.dtype != torch.float32:
        raise RuntimeError(f"unexpected Qwen boundary: shape={tuple(prompt_embedding.shape)} dtype={prompt_embedding.dtype}")
    boundary["qwen_prompt"] = {
        "shape": list(prompt_embedding.shape),
        "dtype": str(prompt_embedding.dtype).replace("torch.", ""),
        "sha256": sha256_bytes(prompt_embedding.detach().contiguous().cpu().numpy().tobytes()),
    }
    del clip
    comfy.model_management.unload_all_models()
    comfy.model_management.soft_empty_cache()
    timings["qwen_conditioning_seconds"] = time.perf_counter() - conditioning_start
    sample_memory("after_qwen_conditioning")

    model_load_start = time.perf_counter()
    model = nodes.UNETLoader().load_unet("z_image_turbo_bf16.safetensors", "default")[0]
    model = ModelSamplingAuraFlow().patch_aura(model, 3.0)[0]
    diffusion_model = model.model.diffusion_model

    # The strangler keeps only source-owned boundary modules in PyTorch. The
    # heavyweight refiners and main stack are removed before Comfy plans GPU
    # residency; their implementation and weights are owned by Prometheus.
    diffusion_model.noise_refiner = torch.nn.ModuleList()
    diffusion_model.context_refiner = torch.nn.ModuleList()
    diffusion_model.layers = torch.nn.ModuleList()
    gc.collect()
    timings["reference_boundary_model_load_seconds"] = time.perf_counter() - model_load_start
    sample_memory("after_boundary_model_load")

    bridge_start = time.perf_counter()
    session = PrometheusZImageSession(args.bridge_dll, args.reactor_dll, args.lock, payload_root,
                                     execution_profile=args.execution_profile,
                                     main_attention_route=args.main_attention_route)
    if display:
        display.update(status="Loading compiled transformer")
    if not args.demo or args.verbose:
        print(json.dumps({"event": "native_session_created", "route": args.main_attention_route}), flush=True)
    timings["prometheus_session_create_seconds"] = time.perf_counter() - bridge_start
    sample_memory("after_prometheus_session_create")
    final_projection_seconds = 0.0
    invocation_count = 0

    def bridge_forward(x, timesteps, context, num_tokens, attention_mask=None, **kwargs):
        nonlocal final_projection_seconds, invocation_count
        if x.shape[0] != 1 or context.shape[0] != 1 or timesteps.shape != (1,):
            raise RuntimeError(f"closed smoke supports batch one only: x={tuple(x.shape)} context={tuple(context.shape)} timestep={tuple(timesteps.shape)}")
        invocation_count += 1
        if display:
            display.update(evaluation=invocation_count, status="Executing")
        evaluation_begin = time.perf_counter()
        gc_begin_index = len(gc_events)
        evaluation_timing: dict[str, float | int] = {"evaluation_index": invocation_count, "scheduler_iteration_begin_monotonic_seconds": evaluation_begin}
        bs, channels, height, width = x.shape
        pre_call_start = time.perf_counter()
        x = comfy.ldm.common_dit.pad_to_patch_size(x, (diffusion_model.patch_size, diffusion_model.patch_size))
        t = diffusion_model.t_embedder((1.0 - timesteps) * diffusion_model.time_scale, dtype=x.dtype)
        if diffusion_model.clip_text_pooled_proj is not None:
            pooled = kwargs.get("clip_text_pooled")
            if pooled is None:
                pooled = torch.zeros((1, diffusion_model.clip_text_dim), device=x.device, dtype=x.dtype)
            else:
                pooled = diffusion_model.clip_text_pooled_proj(pooled)
            t = diffusion_model.time_text_embed(torch.cat((t, pooled), dim=-1))

        cap = diffusion_model.cap_embedder(context)
        if diffusion_model.pad_tokens_multiple is not None:
            cap, _ = pad_zimage(cap, diffusion_model.cap_pad_token, diffusion_model.pad_tokens_multiple)
        patch = diffusion_model.patch_size
        image = diffusion_model.x_embedder(
            x.view(bs, channels, height // patch, patch, width // patch, patch)
            .permute(0, 2, 4, 3, 5, 1)
            .flatten(3)
            .flatten(1, 2)
        )
        if diffusion_model.pad_tokens_multiple is not None:
            image, _ = pad_zimage(image, diffusion_model.x_pad_token, diffusion_model.pad_tokens_multiple)
        if tuple(image.shape) != (1, 1024, 3840) or tuple(cap.shape) != (1, 32, 3840) or tuple(t.shape) != (1, 256):
            raise RuntimeError(f"Python/native shape contradiction: image={tuple(image.shape)}, context={tuple(cap.shape)}, timestep={tuple(t.shape)}")
        evaluation_timing["python_pre_call_seconds"] = time.perf_counter() - pre_call_start

        marshal_start = time.perf_counter()
        image_bits = torch_bf16_bits(image)
        context_fp32 = cap.detach().float().contiguous().cpu().numpy()
        timestep_bits = torch_bf16_bits(t)
        evaluation_timing["python_to_native_marshaling_seconds"] = time.perf_counter() - marshal_start
        native_call_start = time.perf_counter()
        output, native = session.evaluate(image_bits, context_fp32, timestep_bits,
                                          progress_callback=(lambda stage: display.complete_stage(invocation_count, stage)) if display else None)
        evaluation_timing["native_call_and_readback_seconds"] = time.perf_counter() - native_call_start
        if invocation_count == 1:
            boundary.update({
                "image_tokens": {"shape": list(image.shape), "dtype": "bfloat16", "layout": "contiguous token-major [batch,token,channel]", "sha256": sha256_bytes(image_bits.tobytes())},
                "context_tokens": {"shape": list(cap.shape), "dtype": "float32", "layout": "contiguous token-major [batch,token,channel]", "sha256": sha256_bytes(context_fp32.tobytes())},
                "timestep_embedding": {"shape": list(t.shape), "dtype": "bfloat16", "layout": "contiguous [batch,channel]", "sha256": sha256_bytes(timestep_bits.tobytes())},
                "prometheus_output_image_tokens": {"shape": list(output.shape), "dtype": "float32", "layout": "contiguous token-major [batch,token,channel]", "sha256": sha256_bytes(output.tobytes())},
            })
        conversion_start = time.perf_counter()
        projected_input = torch.from_numpy(output).to(device=image.device, dtype=image.dtype)
        evaluation_timing["native_to_python_latent_conversion_seconds"] = time.perf_counter() - conversion_start
        adaln_start = time.perf_counter()
        scale = diffusion_model.final_layer.adaLN_modulation(t)
        projected_input = modulate(diffusion_model.final_layer.norm_final(projected_input), scale, timestep_zero_index=None)
        evaluation_timing["final_adaln_seconds"] = time.perf_counter() - adaln_start
        projection_start = time.perf_counter()
        projected = diffusion_model.final_layer.linear(projected_input)
        evaluation_timing["final_3840_to_64_projection_seconds"] = time.perf_counter() - projection_start
        unpatchify_start = time.perf_counter()
        result = diffusion_model.unpatchify(projected, [(height, width)], [0], return_tensor=True)[:, :, :height, :width]
        evaluation_timing["unpatchify_seconds"] = time.perf_counter() - unpatchify_start
        projection_seconds = evaluation_timing["final_adaln_seconds"] + evaluation_timing["final_3840_to_64_projection_seconds"] + evaluation_timing["unpatchify_seconds"] + evaluation_timing["native_to_python_latent_conversion_seconds"]
        final_projection_seconds += projection_seconds
        evaluation_timing["final_projection_unpatchify_seconds"] = projection_seconds
        evaluation_timing["scheduler_iteration_end_monotonic_seconds"] = time.perf_counter()
        evaluation_timing["bridge_forward_wall_seconds"] = evaluation_timing["scheduler_iteration_end_monotonic_seconds"] - evaluation_begin
        evaluation_timing["garbage_collection_count"] = len(gc_events) - gc_begin_index
        evaluation_timing["garbage_collection_seconds"] = sum(float(event["duration_seconds"]) for event in gc_events[gc_begin_index:])
        evidence_rows.append(native.__dict__ | {"python_timing": evaluation_timing})
        if not args.demo or args.verbose:
            print(json.dumps({"event": "native_evaluation_complete", "index": invocation_count,
                              "route": args.main_attention_route, "finite": bool(torch.isfinite(result).all())}), flush=True)
        if args.preflight_one_evaluation:
            print(json.dumps({"event": "preflight_complete", "route": args.main_attention_route}), flush=True)
            raise SystemExit(0)
        sample_gpu("after_native_evaluation", invocation_count)
        if tuple(result.shape) != (1, 16, 64, 64) or not torch.isfinite(result).all():
            raise RuntimeError(f"invalid external final projection: {tuple(result.shape)}")
        return -result

    diffusion_model.forward = bridge_forward
    latent = {"samples": torch.zeros((1, 16, HEIGHT // 8, WIDTH // 8), dtype=torch.float32, device="cpu")}
    noise = comfy.sample.prepare_noise(latent["samples"], SEED)
    sampler = comfy.samplers.KSampler(model, steps=STEPS, device=model.load_device, sampler="res_multistep", scheduler="simple", denoise=1.0, model_options=model.model_options)
    sigmas = sampler.sigmas.detach().cpu().tolist()

    denoise_start = time.perf_counter()
    try:
        final = sampler.sample(noise, positive, negative, cfg=1.0, latent_image=latent["samples"], disable_pbar=True, seed=SEED)
    finally:
        session_destroy_start = time.perf_counter()
        session.close()
        timings["prometheus_session_destroy_seconds"] = time.perf_counter() - session_destroy_start
    timings["denoising_seconds"] = time.perf_counter() - denoise_start
    for index, row in enumerate(evidence_rows):
        timing = row["python_timing"]
        next_begin = evidence_rows[index + 1]["python_timing"]["scheduler_iteration_begin_monotonic_seconds"] if index + 1 < len(evidence_rows) else denoise_start + timings["denoising_seconds"] - timings["prometheus_session_destroy_seconds"]
        timing["scheduler_update_and_outer_loop_seconds"] = max(0.0, float(next_begin) - float(timing["scheduler_iteration_end_monotonic_seconds"]))
    sample_memory("after_denoising")
    timings["external_final_projection_seconds"] = final_projection_seconds
    if invocation_count != len(evidence_rows) or invocation_count == 0:
        raise RuntimeError(f"scheduler/native invocation accounting failed: calls={invocation_count}, evidence={len(evidence_rows)}")
    if any(row["main_layer_count"] != 30 for row in evidence_rows):
        raise RuntimeError("a denoising evaluation did not execute all 30 native layers")
    if any(any(value != 0.0 for value in row["stage_payload_read_seconds"]) for row in evidence_rows):
        raise RuntimeError("persistent session reread an immutable package after creation")

    del model, diffusion_model
    comfy.model_management.unload_all_models()
    comfy.model_management.soft_empty_cache()
    gc.collect()

    vae_start = time.perf_counter()
    if display:
        display.update(status="Decoding")
    if not args.demo or args.verbose:
        print(json.dumps({"event": "python_postprocessing_start", "route": args.main_attention_route}), flush=True)
    vae = nodes.VAELoader().load_vae("ae.safetensors")[0]
    timings["vae_load_seconds"] = time.perf_counter() - vae_start
    vae_decode_start = time.perf_counter()
    decoded = nodes.VAEDecode().decode(vae, {"samples": final})[0]
    timings["vae_decode_seconds"] = time.perf_counter() - vae_decode_start
    image_array = (decoded[0].detach().clamp(0, 1).cpu().numpy() * 255.0).round().astype("uint8")
    if image_array.shape != (HEIGHT, WIDTH, 3) or image_array.min() == image_array.max():
        raise RuntimeError(f"decoded image is invalid or constant: shape={image_array.shape}, range=[{image_array.min()},{image_array.max()}]")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    if display:
        display.update(status="Writing PNG")
    png_start = time.perf_counter()
    Image.fromarray(image_array, mode="RGB").save(args.output, format="PNG")
    with Image.open(args.output) as check:
        check.verify()
    with Image.open(args.output) as check:
        if check.size != (WIDTH, HEIGHT) or check.mode != "RGB":
            raise RuntimeError(f"PNG verification failed: size={check.size}, mode={check.mode}")
    timings["png_encode_write_seconds"] = time.perf_counter() - png_start
    timings["vae_decode_and_png_seconds"] = time.perf_counter() - vae_start
    timings["wall_time_seconds"] = time.perf_counter() - wall_start
    sample_memory("after_png")

    png_hash = sha256_file(args.output)
    model_ceiling = int(evidence_rows[-1]["model_allocation_ceiling_bytes"])
    execution_profile = MODEL_EXECUTION_PROFILES.get(model_ceiling)
    if execution_profile is None:
        raise RuntimeError(f"unknown Prometheus model allocation ceiling: {model_ceiling}")
    metadata = {
        "schema": "prometheus.evt2.zimage.python-image-smoke.v1",
        "status": "success",
        "request": {
            "prompt": PROMPT,
            "negative_prompt": None,
            "negative_prompt_policy": "not evaluated; CFG=1 uses the single positive branch",
            "seed": SEED,
            "width": WIDTH,
            "height": HEIGHT,
            "sampler": "res_multistep",
            "scheduler": "simple",
            "configured_steps": STEPS,
            "actual_model_evaluations": invocation_count,
            "sigmas": sigmas,
            "aura_flow_shift": 3.0,
            "main_attention_route": args.main_attention_route,
        },
        "authority": {
            "model": f"Tongyi-MAI/Z-Image-Turbo@{MODEL_REVISION}",
            "source": f"Tongyi-MAI/Z-Image@{SOURCE_REVISION}",
            "checkpoint_sha256": CHECKPOINT_SHA256,
            "text_encoder_sha256": TEXT_ENCODER_SHA256,
            "vae_sha256": VAE_SHA256,
            "compiled_model_lock_sha256": LOCK_SHA256,
            "lock_path": str(args.lock.resolve()),
            "payload_root": str(payload_root),
            "reactor_dll_sha256": sha256_file(args.reactor_dll.resolve()),
            "bridge_dll_sha256": sha256_file(args.bridge_dll.resolve()),
        },
        "python_components_retained": ["ComfyUI Qwen tokenizer/encoder", "t_embedder", "x_embedder", "cap_embedder and padding", "FinalLayer AdaLN/linear", "unpatchify", "ComfyUI res_multistep/simple scheduler", "ComfyUI VAE decoder", "Pillow PNG writer"],
        "prometheus_boundary": boundary,
        "boundary_accounting": {
            "python_to_native_calls_per_evaluation": 1,
            "python_to_native_calls_full_image": invocation_count,
            "boundary_crossings_full_image": invocation_count,
            "crossings_inside_main_transformer_chain": 0,
            "bytes_to_native_per_evaluation": 8_356_352,
            "bytes_from_native_per_evaluation": 15_728_640,
            "bytes_to_native_full_image": 8_356_352 * invocation_count,
            "bytes_from_native_full_image": 15_728_640 * invocation_count,
        },
        "native_evaluations": evidence_rows,
        "main_attention_payloads": {
            "requested_route": args.main_attention_route,
            "selection_contract": "an explicit route is admitted during each MainTransformer owner creation; an unavailable route aborts the run",
            "expected_selected_route": {"SerialCanonical": 2, "SubgroupOwned32": 3}.get(args.main_attention_route),
            "expected_shader_id": {"SerialCanonical": 41, "SubgroupOwned32": 49}.get(args.main_attention_route),
        },
        "timings": timings,
        "allocation": {
            "execution_profile": execution_profile,
            "model_owned_ceiling_bytes": model_ceiling,
            "persistent_bytes": evidence_rows[-1]["persistent_bytes"],
            "reusable_bytes": evidence_rows[-1]["reusable_bytes"],
            "audit_bytes": evidence_rows[-1]["audit_bytes"],
            "host_package_cache_bytes": evidence_rows[-1]["host_package_cache_bytes"],
            "host_package_cache_hits_per_evaluation": [row["host_package_cache_hits"] for row in evidence_rows],
        },
        "prefetch": {
            "transfer_seconds": sum(float(row["prefetch_transfer_seconds"]) for row in evidence_rows),
            "overlap_seconds": sum(float(row["prefetch_overlap_seconds"]) for row in evidence_rows),
            "wait_seconds": sum(float(row["prefetch_wait_seconds"]) for row in evidence_rows),
            "count": sum(int(row["prefetch_count"]) for row in evidence_rows),
        },
        "output": {
            "path": str(args.output.resolve()),
            "sha256": png_hash,
            "bytes": args.output.stat().st_size,
            "width": WIDTH,
            "height": HEIGHT,
            "mode": "RGB",
            "pixel_min": int(image_array.min()),
            "pixel_max": int(image_array.max()),
        },
        "environment": {
            "python": sys.version,
            "python_executable": sys.executable,
            "platform": platform.platform(),
            "torch": torch.__version__,
            "cuda": torch.version.cuda,
            "gpu": torch.cuda.get_device_name(0) if torch.cuda.is_available() else None,
            "vulkan_validation": os.environ.get("PROMETHEUS_VK_VALIDATION") == "1",
            "static_audit": "final_layer_29_readback_only",
            "timing_probes": "dvt2_m3_full",
        },
        "process_memory": {"samples": memory_samples, "peak_sampled_rss_bytes": max(sample["rss_bytes"] for sample in memory_samples)},
        "gpu_telemetry": gpu_telemetry,
        "garbage_collection": {"events": gc_events, "total_seconds": sum(float(event["duration_seconds"]) for event in gc_events)},
        "deferred_native_work": "M2D 29-stage audit wiring and exhaustive 30-layer lifecycle matrix; explicitly not a blocker for this shipping smoke.",
        "reproduction_command": f'& "{sys.executable}" "{Path(__file__).resolve()}"',
    }
    args.metadata.parent.mkdir(parents=True, exist_ok=True)
    temporary = args.metadata.with_suffix(args.metadata.suffix + ".tmp")
    temporary.write_text(json.dumps(metadata, indent=2, sort_keys=True), encoding="utf-8")
    temporary.replace(args.metadata)
    if display:
        display.complete(args.output, png_hash, timings["wall_time_seconds"], model_ceiling)
    if not args.demo or args.verbose:
        print(json.dumps({"event": "output_complete", "route": args.main_attention_route,
                          "png": str(args.output.resolve()), "metadata": str(args.metadata.resolve())}), flush=True)
    gc.callbacks.remove(gc_probe)
    if not args.demo or args.verbose:
        print(json.dumps({"png": str(args.output.resolve()), "sha256": png_hash, "metadata": str(args.metadata.resolve()), "evaluations": invocation_count, "wall_time_seconds": timings["wall_time_seconds"]}, indent=2))


if __name__ == "__main__":
    main()
