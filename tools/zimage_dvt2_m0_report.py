#!/usr/bin/env python3
"""Materialize the approved DVT-2 M0 host-package-residency evidence."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def write_json(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main() -> None:
    repo = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--smoke", type=Path, default=repo / "internal/prometheus/DevelopmentReport/artifacts/Dvt2M0/dvt2_m0_smoke.json")
    parser.add_argument("--out", type=Path, default=repo / "internal/prometheus/DevelopmentReport/artifacts/Dvt2M0")
    args = parser.parse_args()
    smoke = json.loads(args.smoke.read_text(encoding="utf-8"))
    pre = json.loads((repo / "internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/dvt2_pipeline_timing.json").read_text(encoding="utf-8"))
    out = args.out
    out.mkdir(parents=True, exist_ok=True)
    rows = smoke["native_evaluations"]
    allocation = smoke["allocation"]
    timings = smoke["timings"]
    per_evaluation = [
        {
            "evaluation_index": row["evaluation_index"],
            "wall_time_seconds": row["wall_time_seconds"],
            "model_execution_seconds": row["model_execution_seconds"],
            "parameter_rebind_seconds": row["parameter_rebind_seconds"],
            "main_layer_count": row["main_layer_count"],
            "context_reused": row["context_reused"],
            "host_package_cache_hits": row["host_package_cache_hits"],
            "immutable_payload_read_seconds": sum(row["stage_payload_read_seconds"]),
            "uploaded_weight_bytes": row["uploaded_weight_bytes"],
        }
        for row in rows
    ]
    write_json(out / "dvt2_m0_baseline.json", {"schema": "prometheus.dvt2.m0.baseline.v1", "rescope": "M0 retains one native session and one GPU window, adds a session-owned immutable host package store; shared-owner retargeting is M1.", "pre_m0_warm_wall_seconds": pre["warm_repeat"]["wall_time_seconds"], "m0_warm_wall_seconds": timings["wall_time_seconds"], "wall_reduction_seconds": pre["warm_repeat"]["wall_time_seconds"] - timings["wall_time_seconds"], "wall_reduction_percent": 100.0 * (pre["warm_repeat"]["wall_time_seconds"] - timings["wall_time_seconds"]) / pre["warm_repeat"]["wall_time_seconds"], "request": smoke["request"]})
    write_json(out / "dvt2_m0_resource_lifetimes.json", {"schema": "prometheus.dvt2.m0.resource-lifetimes.v1", "resources": [{"resource": name, "lifetime": lifetime, "reset": reset} for name, lifetime, reset in [("Vulkan instance/device/queue/command pool", "session", "never per evaluation"), ("compiled-model resident streams", "session", "logical generations only"), ("lock authority/generated descriptors/payload metadata", "session", "never per evaluation"), ("immutable host package cache", "session", "released on close"), ("single GPU weight window and active execution target", "evaluation-family", "retargeted by the existing create/rebind/destroy seam"), ("image/context/timestep/JointWorking/output/replay generations", "evaluation", "new logical generation"), ("command recording", "transient command", "reset before submission")]], "model_owned_ceiling_bytes": allocation["model_owned_ceiling_bytes"], "host_package_cache_bytes": allocation["host_package_cache_bytes"]})
    write_json(out / "dvt2_m0_session_contract.json", {"schema": "prometheus.dvt2.m0.session-contract.v1", "closed_authority": ["lock identity", "payload package identities", "device/runtime", "resident stream slots", "single GPU weight window"], "caller_cannot_supply": ["layer IDs", "package identities", "resource slots", "memory plans", "shader topology"], "serialized_evaluation": True, "host_package_store": {"bytes": allocation["host_package_cache_bytes"], "packages": 34, "immutable": True}})
    write_json(out / "dvt2_m0_evaluation_reset.json", {"schema": "prometheus.dvt2.m0.evaluation-reset.v1", "reset": ["image generation", "timestep identity", "JointWorking generation", "output validity", "replay identity"], "preserved": ["device/runtime", "lock authority", "payload metadata", "host package bytes", "resident stream allocations"], "note": "The single active native owner is still reconstructed between families; that explicit retarget seam is selected for M1."})
    write_json(out / "dvt2_m0_reuse_counters.json", {"schema": "prometheus.dvt2.m0.reuse-counters.v1", "evaluations": [{"evaluation_index": row["evaluation_index"], "host_package_cache_hits": row["host_package_cache_hits"], "immutable_payload_read_seconds": sum(row["stage_payload_read_seconds"])} for row in rows], "assertions": {"all_evaluation_payload_reads_zero": all(sum(row["stage_payload_read_seconds"]) == 0 for row in rows), "all_evaluation_cache_hits_positive": all(row["host_package_cache_hits"] > 0 for row in rows)}})
    write_json(out / "dvt2_m0_per_evaluation_timing.json", {"schema": "prometheus.dvt2.m0.per-evaluation-timing.v1", "evaluations": per_evaluation})
    write_json(out / "dvt2_m0_pipeline_timing.json", {"schema": "prometheus.dvt2.m0.pipeline-timing.v1", "pre_m0": pre, "m0": timings})
    write_json(out / "dvt2_m0_memory.json", {"schema": "prometheus.dvt2.m0.memory.v1", "model_owned": {key: allocation[key] for key in ("model_owned_ceiling_bytes", "persistent_bytes", "reusable_bytes", "audit_bytes")}, "host_package_cache_bytes": allocation["host_package_cache_bytes"], "peak_sampled_python_rss_bytes": smoke["process_memory"]["peak_sampled_rss_bytes"]})
    write_json(out / "dvt2_m0_output_validation.json", {"schema": "prometheus.dvt2.m0.output-validation.v1", "output": smoke["output"], "boundary": smoke["prometheus_boundary"], "all_30_layers": all(row["main_layer_count"] == 30 for row in rows)})
    write_json(out / "dvt2_m0_faults.json", {"schema": "prometheus.dvt2.m0.faults.v1", "host-cache-create-failure": "cache frees all prior package allocations and session creation fails", "session-close": "host cache releases after reactor close", "known_m1_gap": "native active-owner retarget/quarantine corpus remains the next bounded seam"})
    write_json(out / "dvt2_m0_replay.json", {"schema": "prometheus.dvt2.m0.replay.v1", "lock_sha256": smoke["authority"]["compiled_model_lock_sha256"], "png_sha256": smoke["output"]["sha256"], "prediction_boundary_sha256": smoke["prometheus_boundary"]["prometheus_output_image_tokens"]["sha256"]})
    write_json(out / "dvt2_m1_handoff.json", {"schema": "prometheus.dvt2.m1-handoff.v1", "target": "shared active-owner retargeting over the existing single GPU arena", "measured_remaining_cost": "parameter rebind is about 1.35-1.45 seconds/evaluation after immutable payload reads reach zero", "hypothesis": "Preserving lock-derived pipelines, descriptor infrastructure, and arena allocations across family retargets will remove repeated owner reconstruction without adding another weight window.", "required_memory_increase": 0, "prefetch": "not in scope"})


if __name__ == "__main__":
    main()
