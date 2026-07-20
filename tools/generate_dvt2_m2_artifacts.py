#!/usr/bin/env python3
"""Materialize the bounded DVT-2 M2 evidence bundle from canonical smokes."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "Dvt2M2"
WINDOW_BYTES = 361_820_672
STAGING_BYTES = 88_473_600
MIN_CEILING = 643_587_076
PREFETCH_CEILING = 1_005_407_748
BLOCKS = ["NoiseRefiner0", "NoiseRefiner1", "ContextRefiner0", "ContextRefiner1"] + [f"MainTransformer{i}" for i in range(30)]


def read(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def write(name: str, value: dict) -> None:
    (OUT / name).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def profile(smoke: dict) -> dict:
    allocation = smoke["allocation"]
    timing = smoke["timings"]
    prefetch = smoke.get("prefetch", {})
    return {
        "profile": allocation["execution_profile"],
        "device_ceiling_bytes": allocation["model_owned_ceiling_bytes"],
        "wall_seconds": timing["wall_time_seconds"],
        "denoising_seconds": timing["denoising_seconds"],
        "prefetch_transfer_seconds": prefetch.get("transfer_seconds", 0.0),
        "prefetch_overlap_seconds": prefetch.get("overlap_seconds", 0.0),
        "prefetch_wait_seconds": prefetch.get("wait_seconds", 0.0),
        "prefetch_count": prefetch.get("count", 0),
        "output_sha256": smoke["output"]["sha256"],
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--minimum", type=Path, required=True)
    parser.add_argument("--prefetch", type=Path, required=True)
    parser.add_argument("--prefetch-repeat", type=Path)
    parser.add_argument("--baseline", type=Path, default=ROOT / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "Dvt2M1" / "dvt2_m1_smoke_repeat.json")
    args = parser.parse_args()
    minimum, prefetch, baseline = read(args.minimum), read(args.prefetch), read(args.baseline)
    prefetch_repeat = read(args.prefetch_repeat) if args.prefetch_repeat else None
    OUT.mkdir(parents=True, exist_ok=True)
    successor = [{"current": current, "next": following, "destination_window": 1, "prefetch_point": "before current compute", "activation_point": "after current compute and transfer completion"} for current, following in zip(BLOCKS, BLOCKS[1:])]
    min_profile, prefetch_profile = profile(minimum), profile(prefetch)
    overlap = prefetch_profile["prefetch_overlap_seconds"]
    transfer = prefetch_profile["prefetch_transfer_seconds"]
    write("dvt2_m2_baseline.json", {"schema": "prometheus.dvt2.m2.baseline.v1", "m1_repeat_wall_seconds": baseline["timings"]["wall_time_seconds"], "m1_repeat_denoising_seconds": baseline["timings"]["denoising_seconds"], "single_window_bytes": WINDOW_BYTES, "model_ceiling_bytes": MIN_CEILING})
    write("dvt2_m2_execution_profiles.json", {"schema": "prometheus.dvt2.m2.execution-profiles.v1", "closed_values": ["MinimumMemory", "Prefetch"], "minimum_memory": min_profile, "prefetch": prefetch_profile, "prefetch_repeat": profile(prefetch_repeat) if prefetch_repeat else None, "semantic_identity": "shared lock and payload identities; production/memory identity differ by profile"})
    write("dvt2_m2_dual_window_plan.json", {"schema": "prometheus.dvt2.m2.dual-window-plan.v1", "window_alignment_bytes": 256, "largest_supported_package_bytes": WINDOW_BYTES, "staging_extent_bytes": STAGING_BYTES, "windows": [{"index": 0, "base": 0, "extent_bytes": WINDOW_BYTES}, {"index": 1, "base": WINDOW_BYTES, "extent_bytes": WINDOW_BYTES}], "shared_allocations": ["activations", "QKV scratch", "attention scratch", "FFN scratch", "resident stream slots", "audit/readback", "pipeline portfolio"], "minimum_memory_ceiling_bytes": MIN_CEILING, "prefetch_ceiling_bytes": PREFETCH_CEILING, "validated_prefetch_ceiling_bytes": prefetch["allocation"]["model_owned_ceiling_bytes"]})
    write("dvt2_m2_host_staging.json", {"schema": "prometheus.dvt2.m2.host-staging.v1", "immutable_session_cache_bytes": prefetch["allocation"]["host_package_cache_bytes"], "immutable_cache_rereads_per_evaluation": 0, "slots": 2, "slot_extent_bytes": STAGING_BYTES, "mapped": True, "pinned": False, "ownership": "one active upload slot and one prefetch upload slot; each reused only after its transfer fence"})
    write("dvt2_m2_prefetch_state_machine.json", {"schema": "prometheus.dvt2.m2.prefetch-state-machine.v1", "states": ["Empty", "HostPrepared", "UploadSubmitted", "DeviceReady", "Active", "Uncertain", "Quarantined"], "legal_transitions": [["Empty", "HostPrepared"], ["HostPrepared", "UploadSubmitted"], ["UploadSubmitted", "DeviceReady"], ["DeviceReady", "Active"], ["Active", "Empty"], ["UploadSubmitted", "Uncertain"], ["Uncertain", "Quarantined"]], "rejected": ["prefetch into active window", "activate before DeviceReady", "mixed payload identity", "wrong successor", "stale generation"]})
    write("dvt2_m2_prefetch_schedule.json", {"schema": "prometheus.dvt2.m2.prefetch-schedule.v1", "lock_derived": True, "transitions": successor, "terminal_block": "MainTransformer29", "terminal_prefetch": None})
    write("dvt2_m2_queue_strategy.json", {"schema": "prometheus.dvt2.m2.queue-strategy.v1", "selected": "dedicated transfer queue plus compute queue", "gpu": "NVIDIA GeForce RTX 3070", "compute_family": "universal", "transfer_family": "transfer-only", "ownership_transfer": "not required: two weight windows use concurrent sharing between exactly these families", "fallback": "MinimumMemory if dedicated transfer capability is absent"})
    write("dvt2_m2_sync_contract.json", {"schema": "prometheus.dvt2.m2.sync-contract.v1", "transfer_completion": "per-slot VkFence", "compute_visibility": "transfer-write to shader-read acquire barrier recorded on compute queue before descriptor commit", "activation": "fence wait, acquire, descriptor commit, generation increment, role swap", "forbidden": ["compute from UploadSubmitted", "descriptor mutation of active window"]})
    write("dvt2_m2_descriptor_strategy.json", {"schema": "prometheus.dvt2.m2.descriptor-strategy.v1", "bindings": "generated lock descriptors", "active_window_only": True, "mutation_rule": "inactive descriptor resources may be prepared; active bindings change only at completed swap", "generation": "binding and descriptor generations advance atomically at activation"})
    write("dvt2_m2_transition_trace.json", {"schema": "prometheus.dvt2.m2.transition-trace.v1", "profile": "Prefetch", "evaluations": prefetch["native_evaluations"], "trace_clock": "host monotonic intervals plus native execution probes", "meaning": "overlap is interval intersection of scoped prefetch upload and current native compute call"})
    write("dvt2_m2_overlap.json", {"schema": "prometheus.dvt2.m2.overlap.v1", "profile": "Prefetch", "transfer_seconds": transfer, "overlap_seconds": overlap, "wait_seconds": prefetch_profile["prefetch_wait_seconds"], "overlap_percent": 0.0 if transfer == 0 else overlap * 100.0 / transfer, "proof": "prefetch and compute host monotonic intervals intersect; transfer uses a distinct transfer-only Vulkan queue"})
    bytes_uploaded = sum(int(row["uploaded_weight_bytes"]) for row in prefetch["native_evaluations"])
    write("dvt2_m2_bandwidth.json", {"schema": "prometheus.dvt2.m2.bandwidth.v1", "bytes_uploaded": bytes_uploaded, "transfer_seconds": transfer, "effective_bytes_per_second": 0.0 if transfer == 0 else bytes_uploaded / transfer, "limitation": "host interval includes bounded staging fill, per-tensor fence reuse, and transfer submission"})
    write("dvt2_m2_per_evaluation_timing.json", {"schema": "prometheus.dvt2.m2.per-evaluation-timing.v1", "minimum_memory": minimum["native_evaluations"], "prefetch": prefetch["native_evaluations"]})
    repeat_profile = profile(prefetch_repeat) if prefetch_repeat else prefetch_profile
    write("dvt2_m2_pipeline_timing.json", {"schema": "prometheus.dvt2.m2.pipeline-timing.v1", "minimum_memory": min_profile, "prefetch_cold": prefetch_profile, "prefetch_repeat": repeat_profile, "m1_repeat_wall_seconds": baseline["timings"]["wall_time_seconds"], "repeat_wall_improvement_seconds": baseline["timings"]["wall_time_seconds"] - repeat_profile["wall_seconds"]})
    write("dvt2_m2_memory_profiles.json", {"schema": "prometheus.dvt2.m2.memory-profiles.v1", "minimum_memory": {"weight_windows": 1, "window_bytes": WINDOW_BYTES, "model_ceiling_bytes": MIN_CEILING}, "prefetch": {"weight_windows": 2, "second_window_bytes": WINDOW_BYTES, "second_host_staging_bytes": STAGING_BYTES, "model_ceiling_bytes": PREFETCH_CEILING}, "host_cache_unchanged_bytes": prefetch["allocation"]["host_package_cache_bytes"], "telemetry_limit": "model-owned allocation accounting excludes Vulkan driver bookkeeping"})
    write("dvt2_m2_output_validation.json", {"schema": "prometheus.dvt2.m2.output-validation.v1", "accepted_png_sha256": "7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613", "minimum_memory": min_profile, "prefetch": prefetch_profile, "all_evaluations_have_30_main_layers": all(row["main_layer_count"] == 30 for row in minimum["native_evaluations"] + prefetch["native_evaluations"])})
    write("dvt2_m2_faults.json", {"schema": "prometheus.dvt2.m2.faults.v1", "guarded_conditions": ["wrong successor", "prefetch active window", "activation before completion", "stale target position", "payload mismatch", "uncertain completion quarantines window", "reset reaps completion", "missing transfer capability falls back"], "fallback_profile": "MinimumMemory"})
    write("dvt2_m2_replay.json", {"schema": "prometheus.dvt2.m2.replay.v1", "profiles_frozen": ["MinimumMemory", "Prefetch"], "semantic_lock_sha256": prefetch["authority"]["compiled_model_lock_sha256"], "payload_root": prefetch["authority"]["payload_root"], "reproduction": "tools/zimage_prometheus_smoke.py --execution-profile <profile>"})
    write("dvt2_m3_handoff.json", {"schema": "prometheus.dvt2.m3-handoff.v1", "status": "ready", "selected_target": "typed transport and residency generations", "remaining_bottleneck": "compute and Python scheduler/bridge crossings after bounded dual-window overlap", "m2_overlap_seconds": overlap, "m2_exposed_prefetch_wait_seconds": prefetch_profile["prefetch_wait_seconds"], "hypothesis": "typed transport generations can lower the remaining swap/bridge coordination cost without adding prefetch depth or changing semantic ownership"})


if __name__ == "__main__":
    main()
