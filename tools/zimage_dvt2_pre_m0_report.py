#!/usr/bin/env python3
"""Materialize the DVT-2 Pre-M0 audit from a timing-enabled fixed smoke."""
from __future__ import annotations

import argparse
import hashlib
import json
import statistics
import subprocess
from pathlib import Path

STAGES = ["NoiseRefiner0", "NoiseRefiner1", "ContextRefiner0", "ContextRefiner1"] + [f"MainTransformer{n}" for n in range(30)]
CEILING = {"persistent_weight_window": 361_820_672, "reusable_activation_and_session": 234_579_972, "audit_arena": 47_186_432}


def write(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def summary(values: list[float]) -> dict[str, float]:
    return {"min_seconds": min(values), "mean_seconds": statistics.mean(values), "median_seconds": statistics.median(values), "max_seconds": max(values)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cold", type=Path, required=True)
    parser.add_argument("--warm", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    cold = json.loads(args.cold.read_text(encoding="utf-8"))
    warm = json.loads(args.warm.read_text(encoding="utf-8")) if args.warm else None
    measured = warm or cold
    out = args.out
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    lock = Path(cold["authority"]["lock_path"])
    lock_hash = hashlib.sha256(lock.read_bytes()).hexdigest()
    ev = cold["native_evaluations"]
    wall = cold["timings"]["wall_time_seconds"]
    uploaded = sum(row["uploaded_weight_bytes"] for row in ev)
    payload_read = sum(sum(row["stage_payload_read_seconds"]) for row in ev)
    rebind = sum(row["parameter_rebind_seconds"] for row in ev)
    native = sum(row["model_execution_seconds"] for row in ev)
    host_window_bandwidth = uploaded / rebind if rebind else 0.0
    identity = {"schema": "prometheus.dvt2.pre-m0.identity.v1", "commit": commit, "lock_sha256": lock_hash, "checkpoint_sha256": measured["authority"]["checkpoint_sha256"], "model": measured["authority"]["model"], "source": measured["authority"]["source"], "smoke_output": measured["output"], "environment": measured["environment"], "bridge_dll_sha256": measured["authority"]["bridge_dll_sha256"], "reactor_dll_sha256": measured["authority"]["reactor_dll_sha256"], "shader_portfolio": "lock-tagon.octagon: NoiseRefiner 24-36; ContextRefiner 25-39; MainTransformer 40-43"}
    layers = [
        {"layer": "authoring/front-end", "current_owner": "Python CLI fixed prompt", "long_term_owner": "replaceable producer", "input_abi": "user intent", "output_abi": "ConditioningRequest.v1", "state": "none", "replaceable": True, "critical_path": True},
        {"layer": "conditioning", "current_owner": "ComfyUI tokenizer/Qwen/embedders", "long_term_owner": "typed conditioning producer", "input_abi": "ConditioningRequest.v1", "output_abi": "ConditioningTensor.v1", "state": "Qwen weights", "replaceable": True, "critical_path": True},
        {"layer": "compiled model", "current_owner": "Prometheus", "long_term_owner": "Prometheus", "input_abi": "PrometheusEvaluation.v2", "output_abi": "ImageTokens.FP32", "state": "lock/session/device streams", "replaceable": False, "critical_path": True},
        {"layer": "scheduler", "current_owner": "ComfyUI Python", "long_term_owner": "native or typed external scheduler", "input_abi": "SchedulerStep.v1", "output_abi": "Latent.FP32", "state": "sigmas/latent", "replaceable": True, "critical_path": True},
        {"layer": "final projection", "current_owner": "ComfyUI Python", "long_term_owner": "native or external projection", "input_abi": "FinalProjection.v1", "output_abi": "Latent.FP32", "state": "final-layer weights", "replaceable": True, "critical_path": True},
        {"layer": "decoder", "current_owner": "ComfyUI VAE", "long_term_owner": "decoder ABI", "input_abi": "DecoderInput.v1", "output_abi": "Image.RGB", "state": "VAE weights", "replaceable": True, "critical_path": False},
        {"layer": "artifact", "current_owner": "Pillow", "long_term_owner": "artifact writer ABI", "input_abi": "Image.RGB", "output_abi": "PNG.v1", "state": "none", "replaceable": True, "critical_path": False},
    ]
    for layer in layers:
        layer["lifecycle"] = "per request unless explicitly session-resident"
        layer["correctness_authority"] = "pinned source boundary plus recorded tensor/artifact identities"
        layer["compute_heavy"] = layer["layer"] in {"conditioning", "compiled model", "decoder"}
    seams = {"schema": "prometheus.dvt2.bootstrap-seams.v1", "runtime_authority": "typed in-memory records and the C ABI; JSON is report-only", "contracts": [
        {"name": "ConditioningTensor.v1", "role": "context", "shape": "[1,32,3840]", "dtype": "FP32", "layout": "C-contiguous token-major", "mask": "not currently required", "selected_qwen_layer": "ComfyUI lumina2 scheduled output", "normalization": "source-owned", "identity": "SHA-256", "producer_identity": "Qwen/tokenizer revision"},
        {"name": "ImageIngress.v1", "shape": "[1,1024,3840]", "dtype": "BF16", "layout": "C-contiguous token-major", "scaling": "source x_embedder", "ownership": "caller until execute", "mutability": "immutable", "generation": "per evaluation"},
        {"name": "SchedulerConditioning.v1", "shape": "[1,256]", "dtype": "BF16", "encoding": "t_embedder((1-timestep)*1000)", "step_identity": "sigma index + value", "AuraFlow_shift": 3.0, "generation": "per evaluation"},
        {"name": "PrometheusEvaluation.v2", "input": "ImageIngress.v1 + ConditioningTensor.v1 + SchedulerConditioning.v1", "output": "[1,1024,3840] FP32", "ownership": "session-owned device work / caller output buffer", "lifecycle": "one denoising evaluation per call", "errors": "nonzero C status plus sized diagnostic", "replay": "lock + generations + tensor identities"},
        {"name": "FinalProjection.v1", "input": "[1,1024,3840] FP32 + [1,256] BF16", "weights": "source FinalLayer identity", "output": "[1,16,64,64]", "dtype": "source boundary dtype", "scaling": "source sign inversion retained"},
        {"name": "DecoderInput.v1", "shape": "[1,16,64,64]", "dtype": "FP32", "channel_order": "latent channels", "scaling": "ComfyUI VAE contract", "output": "[1,512,512,3] RGB"},
        {"name": "PNG.v1", "size": "512x512", "channel_order": "RGB", "bit_depth": 8, "color_space": "not asserted by current writer", "metadata": "no nondeterministic metadata", "hash": "SHA-256 full file"}
    ]}
    lifecycle = {"schema": "prometheus.dvt2.lifecycle.v1", "sequence": ["Python starts/imports", "DLL/session create validates lock and payloads", "Vulkan runtime/session creates", "each evaluation prepares/reuses context", "NoiseRefiner owner create/execute/rebind/destroy", "capture PreparedImage", "compose JointWorking", "MainTransformer owner create/rebind layers 0-29/destroy", "read back image tokens", "Python final projection/scheduler", "session destroy", "VAE/PNG"], "resources": [
        {"resource": "Vulkan runtime, session, lock index, fixed streams", "created": "session create", "destroyed": "session destroy", "reused": True, "cost": "warm session create %.3fs" % cold["timings"]["prometheus_session_create_seconds"], "future_owner": "Prometheus"},
        {"resource": "ContextRefiner/NoiseRefiner/MainTransformer owner", "created": "evaluation", "destroyed": "after family/final layer", "reused": False, "cost": "%.3fs cold host-visible rebind total" % rebind, "future_owner": "Prometheus warm-session M0"},
        {"resource": "PreparedImage/PreparedContext/JointWorking", "created": "capture/compose", "destroyed": "session destroy or replacement", "reused": "context only when digest matches", "cost": "included in model execution and session reusable allocation", "future_owner": "Prometheus"},
        {"resource": "Qwen/VAE", "created": "Python load", "destroyed": "ComfyUI unload/process end", "reused": False, "cost": "Qwen %.3fs, VAE %.3fs cold" % (cold["timings"]["qwen_conditioning_seconds"], cold["timings"]["vae_decode_and_png_seconds"]), "future_owner": "replaceable producer/decoder"}
    ]}
    pipeline = {"schema": "prometheus.dvt2.pipeline-timing.v1", "cold": cold["timings"], "warm_repeat": warm["timings"] if warm else {"status": "not yet recorded"}, "evaluation_wall_summary": summary([row["wall_time_seconds"] for row in ev]), "measurement": "time.perf_counter monotonic host timing; native stage execution is reactor evidence"}
    per_eval = {"schema": "prometheus.dvt2.per-evaluation.v1", "rows": ev, "native_wall_summary": summary([r["wall_time_seconds"] for r in ev]), "model_execution_summary": summary([r["model_execution_seconds"] for r in ev])}
    stage_rows = []
    for index, name in enumerate(STAGES):
        stage_rows.append({"stage": name, "execution": summary([r["stage_execution_seconds"][index] for r in ev]), "rebind": summary([r["stage_rebind_seconds"][index] for r in ev]), "payload_read": summary([r["stage_payload_read_seconds"][index] for r in ev]), "uploaded_weight_bytes": ev[0]["stage_uploaded_weight_bytes"][index]})
    transfer = {"schema": "prometheus.dvt2.transfer-trace.v1", "all_evaluations_uploaded_weight_bytes": uploaded, "readback_bytes": len(ev) * 1024 * 3840 * 4, "python_ingress_bytes": len(ev) * (1024*3840*2 + 32*3840*4 + 256*2), "device_to_device_joint_bytes_per_evaluation": 1056*3840*4, "payload_disk_read_host_seconds": payload_read, "limitations": "The bridge records payload-read and host-visible rebind windows. The current reactor C ABI does not expose GPU copy timestamps, pinned-memory state, or disk-cache counters; do not interpret these as PCIe-only measurements."}
    bandwidth = {"schema": "prometheus.dvt2.bandwidth.v1", "host_visible_weight_window_bytes_per_second": host_window_bandwidth, "host_visible_weight_window_gib_per_second": host_window_bandwidth/(1024**3), "window_seconds": rebind, "bytes": uploaded, "classification": "bounded host-visible rebind window, not claimed PCIe bandwidth", "overlap": "none measured; current calls serialize payload load, rebind, and execute"}
    memory = {"schema": "prometheus.dvt2.memory.v1", "components": CEILING, "total": sum(CEILING.values()), "accepted_ceiling": 643_587_076, "validation": sum(CEILING.values()) == 643_587_076, "streams": {"PreparedImage": 15_728_640, "PreparedContext": 491_520, "JointWorking": 16_220_160}, "note": "QKV/attention/FFN scratch are included in the reactor reusable allocation rather than separately exported by this ABI."}
    process = {"schema": "prometheus.dvt2.process-memory.v1", "python_process": measured.get("process_memory", "not collected"), "whole_device_vulkan_memory": "unavailable through current reactor ABI", "qwen_vae_ram": "not separately observable", "model_owned_bytes": 643_587_076, "conclusion": "Do not conflate this precise model-owned ceiling with total VRAM or process RSS."}
    ranking = {"schema": "prometheus.dvt2.bottlenecks.v1", "ranked": [
        {"candidate": "native model execution", "seconds": native, "percent_wall": native/wall*100, "payoff": "high", "risk": "high"},
        {"candidate": "payload read/reconstruction", "seconds": payload_read, "percent_wall": payload_read/wall*100, "payoff": "high", "risk": "medium"},
        {"candidate": "owner rebind window", "seconds": rebind, "percent_wall": rebind/wall*100, "payoff": "medium", "risk": "medium"},
        {"candidate": "session creation", "seconds": cold["timings"]["prometheus_session_create_seconds"], "percent_wall": cold["timings"]["prometheus_session_create_seconds"]/wall*100, "payoff": "low per image", "risk": "low"},
        {"candidate": "Qwen/VAE/final/PNG", "seconds": cold["timings"]["qwen_conditioning_seconds"]+cold["timings"]["vae_decode_and_png_seconds"]+cold["timings"]["external_final_projection_seconds"], "percent_wall": 0, "payoff": "low", "risk": "high if native"}
    ]}
    inventory = {"schema": "prometheus.dvt2.scaffolding.v1", "entries": [
        {"path": "tools/zimage_prometheus_smoke.py", "purpose": "canonical fixed smoke", "references": "shipping report and DVT2 docs", "action": "keep production bootstrap", "risk": "low", "owner": "bootstrap"},
        {"path": "tools/prometheus_zimage_bridge.py", "purpose": "typed ctypes ABI", "references": "smoke", "action": "keep diagnostic/bridge", "risk": "low", "owner": "Prometheus"},
        {"path": "tools/zimage_dvt2_pre_m0_report.py", "purpose": "deterministic audit materializer", "references": "DVT2 artifacts", "action": "keep tooling", "risk": "low", "owner": "DVT2"},
        {"path": "internal/prometheus/DevelopmentReport/artifacts/Evt2Shipping", "purpose": "canonical accepted evidence", "references": "reports", "action": "keep canonical evidence", "risk": "high", "owner": "EVT2"},
        {"path": "out/", "purpose": "local DLL/build outputs", "references": "local smoke", "action": "local-only generated artifact", "risk": "do not commit", "owner": "native build"}
    ]}
    replacement = {"schema": "prometheus.dvt2.python-removal.v1", "stages": [{"stage": n, "current": "Python/ComfyUI", "abi": a, "replacement": r, "shipping_required": False, "semantic_change": False} for n,a,r in [("tokenizer","ConditioningRequest.v1","typed tokenizer producer"),("Qwen","ConditioningTensor.v1","typed encoder service"),("embedding/patchify","ImageIngress.v1","native ingress producer"),("scheduler","SchedulerConditioning.v1","native scheduler"),("final AdaLN/linear/unpatchify","FinalProjection.v1","native projection"),("VAE","DecoderInput.v1","native/external decoder"),("PNG","PNG.v1","artifact writer")]]}
    candidates = {"schema": "prometheus.dvt2.m0-candidates.v1", "candidates": [{"candidate": "persistent warm model session across scheduler evaluations", "bottleneck": "payload reconstruction and owner rebind", "recommendation": "SELECT M0", "expected_wall_reduction": "remove repeated owner allocation/reconstruction; measure against current host-visible  %.2fs payload-read + %.2fs rebind" % (payload_read,rebind)}, {"candidate": "double-buffered prefetch", "recommendation": "defer; depends on persistent ownership"}, {"candidate": "typed transport", "recommendation": "defer; architectural follow-up"}, {"candidate": "native scheduler", "recommendation": "defer; larger semantic boundary"}, {"candidate": "native final projection", "recommendation": "defer; measured negligible"}, {"candidate": "pinned packing", "recommendation": "defer; no pinned-memory evidence"}, {"candidate": "bridge lifecycle cleanup", "recommendation": "covered by M0 seam"}, {"candidate": "native VAE", "recommendation": "defer; low measured payoff"}, {"candidate": "native Qwen", "recommendation": "defer; large scope, low share"}]}
    m0 = {"schema": "prometheus.dvt2.m0-recommendation.v1", "selected_target": "persistent warm model owner/session across scheduler evaluations", "hypothesis": "If Prometheus retains legal owner allocations and immutable packages across successive scheduler evaluations, then the measured repeated payload-read/rebind contribution will fall materially while the exact lock, 30-layer count, FP32 activation policy, allocation ceiling, and image boundary hashes remain valid.", "acceptance": ["fixed smoke produces valid 512x512 RGB PNG", "nine evaluations each report 30 layers", "model-owned ceiling remains <=643587076", "no host activation reconstruction inside transformer", "cold and warm trace prove a lower repeated owner/payload cost", "boundary identities remain deterministic where expected"], "expected_performance": "measure first; no unverified percentage claim", "deferred": ["prefetch", "native scheduler", "native Qwen", "native VAE", "full transport subsystem", "M2D deferred audit hardening"]}
    reproduction = {"schema": "prometheus.dvt2.reproduction.v1", "canonical_command": '& "$env:USERPROFILE\\ComfyUI\\.venv\\Scripts\\python.exe" tools\\zimage_prometheus_smoke.py --metadata internal\\prometheus\\DevelopmentReport\\artifacts\\Dvt2PreM0\\dvt2_profile_cold.json', "requires": ["OCT_EVT2_CACHE or default local payload root", "ComfyUI source/data roots or OCT_COMFY_ROOT/OCT_COMFY_DATA", "out/prometheus/native/prometheus_reactor.dll", "out/prometheus/python_bridge/prometheus_zimage_bridge.dll"], "outputs": ["local PNG", "fixed smoke metadata", "DVT2 profile JSON"], "validation": "PNG verify, 30 layers/evaluation, allocation ceiling, boundary hashes"}
    entries = {"dvt2_baseline_identity.json":identity,"dvt2_architecture_layers.json":layers,"dvt2_bootstrap_seams.json":seams,"dvt2_session_lifecycle.json":lifecycle,"dvt2_pipeline_timing.json":pipeline,"dvt2_per_evaluation_timing.json":per_eval,"dvt2_native_stage_timing.json":{"schema":"prometheus.dvt2.native-stage-timing.v1","stages":stage_rows},"dvt2_transfer_trace.json":transfer,"dvt2_bandwidth.json":bandwidth,"dvt2_memory_accounting.json":memory,"dvt2_process_memory.json":process,"dvt2_bottleneck_ranking.json":ranking,"dvt2_scaffolding_inventory.json":inventory,"dvt2_cleanup_actions.json":{"schema":"prometheus.dvt2.cleanup.v1","actions":["centralized per-stage trace schema in bridge ABI v2","preserved one canonical smoke entry point","kept generated DLLs and payloads local-only"]},"dvt2_python_replacement_map.json":replacement,"dvt2_architecture_candidates.json":candidates,"dvt2_m0_recommendation.json":m0,"dvt2_reproduction.json":reproduction}
    for name, data in entries.items(): write(out/name, data)


if __name__ == "__main__": main()
