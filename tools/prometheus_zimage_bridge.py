#!/usr/bin/env python3
"""Minimal ctypes binding for the closed Prometheus Z-Image session ABI."""

from __future__ import annotations

import ctypes
from dataclasses import dataclass
from pathlib import Path

import numpy as np


IMAGE_SHAPE = (1, 1024, 3840)
CONTEXT_SHAPE = (1, 32, 3840)
TIMESTEP_SHAPE = (1, 256)
OUTPUT_SHAPE = IMAGE_SHAPE
MODEL_EXECUTION_PROFILE_CEILINGS = frozenset((643_587_076, 1_005_407_748))
MODEL_EXECUTION_PROFILES = {"MinimumMemory": 1, "Prefetch": 2}
MAIN_ATTENTION_ROUTES = {"Auto": 1, "SerialCanonical": 2, "SubgroupOwned32": 3, "BuiltinTopology": 4}
MAIN_STAGE_NAMES = (
    "ingress_cast", "adaln_modulation", "attention_norm1", "qkv",
    "q_norm_rope", "k_norm_rope", "attention", "projection_residual",
    "attention_norm2", "ffn_norm", "ffn_w1_w3", "gate", "w2_residual",
)
NATIVE_COUNTER_NAMES = (
    "vkCreateBuffer", "vkDestroyBuffer", "vkAllocateMemory", "vkFreeMemory",
    "vkCreateShaderModule", "vkDestroyShaderModule", "vkCreateComputePipelines",
    "vkAllocateDescriptorSets", "vkUpdateDescriptorSets", "vkCreateCommandPool",
    "vkAllocateCommandBuffers", "vkResetCommandBuffer", "vkQueueSubmit",
    "vkWaitForFences", "timelineWait", "vkMapMemory", "vkUnmapMemory",
    "vkFlushMappedMemoryRanges", "vkInvalidateMappedMemoryRanges",
)


class _CreateRequest(ctypes.Structure):
    _fields_ = [
        ("struct_size", ctypes.c_uint32),
        ("reactor_dll_path", ctypes.c_char_p),
        ("compiled_model_lock_path", ctypes.c_char_p),
        ("payload_root", ctypes.c_char_p),
        ("device_index", ctypes.c_int32),
        ("execution_profile", ctypes.c_uint32),
        ("main_attention_route_policy", ctypes.c_uint32),
    ]


class _ExecuteRequest(ctypes.Structure):
    _fields_ = [
        ("struct_size", ctypes.c_uint32),
        ("image_bf16", ctypes.c_void_p),
        ("image_bytes", ctypes.c_uint64),
        ("image_batch", ctypes.c_uint32),
        ("image_tokens", ctypes.c_uint32),
        ("image_width", ctypes.c_uint32),
        ("context_fp32", ctypes.POINTER(ctypes.c_float)),
        ("context_bytes", ctypes.c_uint64),
        ("context_batch", ctypes.c_uint32),
        ("context_tokens", ctypes.c_uint32),
        ("context_width", ctypes.c_uint32),
        ("timestep_bf16", ctypes.c_void_p),
        ("timestep_bytes", ctypes.c_uint64),
        ("timestep_batch", ctypes.c_uint32),
        ("timestep_width", ctypes.c_uint32),
        ("output_image_fp32", ctypes.POINTER(ctypes.c_float)),
        ("output_image_bytes", ctypes.c_uint64),
    ]


class _ExecuteEvidence(ctypes.Structure):
    _fields_ = [
        ("struct_size", ctypes.c_uint32),
        ("evaluation_index", ctypes.c_uint32),
        ("main_layer_count", ctypes.c_uint32),
        ("context_reused", ctypes.c_uint32),
        ("wall_time_ns", ctypes.c_uint64),
        ("model_execution_ns", ctypes.c_uint64),
        ("parameter_rebind_ns", ctypes.c_uint64),
        ("uploaded_weight_bytes", ctypes.c_uint64),
        ("model_allocation_ceiling_bytes", ctypes.c_uint64),
        ("persistent_bytes", ctypes.c_uint64),
        ("reusable_bytes", ctypes.c_uint64),
        ("audit_bytes", ctypes.c_uint64),
        ("host_package_cache_bytes", ctypes.c_uint64),
        ("host_package_cache_hits", ctypes.c_uint64),
        ("prefetch_transfer_ns", ctypes.c_uint64),
        ("prefetch_overlap_ns", ctypes.c_uint64),
        ("prefetch_wait_ns", ctypes.c_uint64),
        ("prefetch_count", ctypes.c_uint32),
        ("reserved0", ctypes.c_uint32),
        ("stage_execution_ns", ctypes.c_uint64 * 34),
        ("stage_gpu_execution_ns", ctypes.c_uint64 * 34),
        ("stage_rebind_ns", ctypes.c_uint64 * 34),
        ("stage_payload_read_ns", ctypes.c_uint64 * 34),
        ("stage_uploaded_weight_bytes", ctypes.c_uint64 * 34),
        ("main_correlation_id", ctypes.c_uint64 * 30),
        ("main_cpu_begin_ns", ctypes.c_uint64 * 30),
        ("main_cpu_end_ns", ctypes.c_uint64 * 30),
        ("main_parameter_generation", ctypes.c_uint64 * 30),
        ("main_execution_generation", ctypes.c_uint64 * 30),
        ("main_active_weight_window", ctypes.c_uint32 * 30),
        ("main_gpu_total_begin_tick", ctypes.c_uint64 * 30),
        ("main_gpu_total_end_tick", ctypes.c_uint64 * 30),
        ("main_gpu_total_ns", ctypes.c_uint64 * 30),
        ("main_gpu_compute_begin_tick", ctypes.c_uint64 * 30),
        ("main_gpu_compute_end_tick", ctypes.c_uint64 * 30),
        ("main_gpu_compute_ns", ctypes.c_uint64 * 30),
        ("main_gpu_ingress_transfer_ns", ctypes.c_uint64 * 30),
        ("main_gpu_joint_copy_ns", ctypes.c_uint64 * 30),
        ("main_stage_gpu_begin_tick", ctypes.c_uint64 * 390),
        ("main_stage_gpu_end_tick", ctypes.c_uint64 * 390),
        ("main_stage_gpu_ns", ctypes.c_uint64 * 390),
        ("main_active_target_validation_ns", ctypes.c_uint64 * 30),
        ("main_command_reset_ns", ctypes.c_uint64 * 30),
        ("main_command_begin_ns", ctypes.c_uint64 * 30),
        ("main_command_record_ns", ctypes.c_uint64 * 30),
        ("main_command_end_ns", ctypes.c_uint64 * 30),
        ("main_queue_submit_ns", ctypes.c_uint64 * 30),
        ("main_fence_wait_ns", ctypes.c_uint64 * 30),
        ("main_descriptor_update_ns", ctypes.c_uint64 * 30),
        ("main_staging_memcpy_ns", ctypes.c_uint64 * 30),
        ("main_native_counters", ctypes.c_uint64 * 570),
        ("final_readback_gpu_ns", ctypes.c_uint64),
        ("final_readback_host_ns", ctypes.c_uint64),
    ]


@dataclass(frozen=True)
class ExecuteEvidence:
    evaluation_index: int
    main_layer_count: int
    context_reused: bool
    wall_time_seconds: float
    model_execution_seconds: float
    parameter_rebind_seconds: float
    uploaded_weight_bytes: int
    model_allocation_ceiling_bytes: int
    persistent_bytes: int
    reusable_bytes: int
    audit_bytes: int
    host_package_cache_bytes: int
    host_package_cache_hits: int
    prefetch_transfer_seconds: float
    prefetch_overlap_seconds: float
    prefetch_wait_seconds: float
    prefetch_count: int
    stage_execution_seconds: tuple[float, ...]
    stage_gpu_execution_seconds: tuple[float, ...]
    stage_rebind_seconds: tuple[float, ...]
    stage_payload_read_seconds: tuple[float, ...]
    stage_uploaded_weight_bytes: tuple[int, ...]
    main_layer_trace: tuple[dict[str, object], ...]
    final_readback_gpu_seconds: float
    final_readback_host_seconds: float


def _absolute_existing(path: Path | str, label: str, directory: bool = False) -> Path:
    resolved = Path(path).expanduser().resolve(strict=True)
    if directory != resolved.is_dir():
        expected = "directory" if directory else "file"
        raise ValueError(f"{label} must be an existing {expected}: {resolved}")
    return resolved


def _require_array(value: np.ndarray, label: str, dtype: np.dtype, shape: tuple[int, ...]) -> np.ndarray:
    if not isinstance(value, np.ndarray):
        raise TypeError(f"{label} must be a NumPy array")
    if value.dtype != dtype:
        raise TypeError(f"{label} dtype={value.dtype}; require {dtype}")
    if value.ndim != len(shape) or value.shape != shape:
        raise ValueError(f"{label} shape={value.shape}; require {shape}")
    if not value.flags.c_contiguous:
        raise ValueError(f"{label} must use C-contiguous token-major storage")
    if value.nbytes != int(np.prod(shape)) * dtype.itemsize:
        raise ValueError(f"{label} byte count does not match its shape and dtype")
    return value


class PrometheusZImageSession:
    """One serialized, long-lived Vulkan session; Python never supplies topology."""

    def __init__(self, bridge_dll: Path | str, reactor_dll: Path | str, lock_path: Path | str, payload_root: Path | str, device_index: int = -1, execution_profile: str = "MinimumMemory", main_attention_route: str = "Auto"):
        self._bridge_path = _absolute_existing(bridge_dll, "bridge DLL")
        reactor_path = _absolute_existing(reactor_dll, "reactor DLL")
        lock = _absolute_existing(lock_path, "compiled-model lock")
        payload = _absolute_existing(payload_root, "payload root", directory=True)
        self._dll = ctypes.CDLL(str(self._bridge_path))
        self._dll.prometheus_zimage_bridge_abi_version.argtypes = []
        self._dll.prometheus_zimage_bridge_abi_version.restype = ctypes.c_uint32
        self._dll.prometheus_zimage_session_create.argtypes = [ctypes.POINTER(_CreateRequest), ctypes.POINTER(ctypes.c_uint64)]
        self._dll.prometheus_zimage_session_create.restype = ctypes.c_int
        self._dll.prometheus_zimage_session_execute.argtypes = [ctypes.c_uint64, ctypes.POINTER(_ExecuteRequest), ctypes.POINTER(_ExecuteEvidence)]
        self._dll.prometheus_zimage_session_execute.restype = ctypes.c_int
        self._dll.prometheus_zimage_session_destroy.argtypes = [ctypes.c_uint64]
        self._dll.prometheus_zimage_session_destroy.restype = ctypes.c_int
        self._dll.prometheus_zimage_last_error.argtypes = [ctypes.c_uint64, ctypes.c_char_p, ctypes.c_uint64]
        self._dll.prometheus_zimage_last_error.restype = ctypes.c_uint64
        if self._dll.prometheus_zimage_bridge_abi_version() != 5:
            raise RuntimeError("unsupported Prometheus Z-Image bridge ABI")
        if execution_profile not in MODEL_EXECUTION_PROFILES:
            raise ValueError("execution_profile must be MinimumMemory or Prefetch")
        if main_attention_route not in MAIN_ATTENTION_ROUTES:
            raise ValueError("main_attention_route must be Auto, SerialCanonical, SubgroupOwned32, or BuiltinTopology")
        encoded = [str(path).encode("utf-8") for path in (reactor_path, lock, payload)]
        request = _CreateRequest(ctypes.sizeof(_CreateRequest), encoded[0], encoded[1], encoded[2], device_index, MODEL_EXECUTION_PROFILES[execution_profile], MAIN_ATTENTION_ROUTES[main_attention_route])
        handle = ctypes.c_uint64()
        if self._dll.prometheus_zimage_session_create(ctypes.byref(request), ctypes.byref(handle)) != 0:
            raise RuntimeError(self._last_error(0))
        if handle.value == 0:
            raise RuntimeError("Prometheus returned a null Z-Image session")
        self._handle = handle.value

    def _last_error(self, handle: int | None = None) -> str:
        selected = getattr(self, "_handle", 0) if handle is None else handle
        needed = int(self._dll.prometheus_zimage_last_error(selected, None, 0))
        if needed <= 1:
            return "Prometheus Z-Image bridge failed without diagnostic text"
        buffer = ctypes.create_string_buffer(needed)
        self._dll.prometheus_zimage_last_error(selected, buffer, needed)
        return buffer.value.decode("utf-8", errors="replace")

    def evaluate(self, image_bf16_bits: np.ndarray, context_fp32: np.ndarray, timestep_bf16_bits: np.ndarray) -> tuple[np.ndarray, ExecuteEvidence]:
        if not getattr(self, "_handle", 0):
            raise RuntimeError("Prometheus Z-Image session is closed")
        image = _require_array(image_bf16_bits, "image_bf16_bits", np.dtype(np.uint16), IMAGE_SHAPE)
        context = _require_array(context_fp32, "context_fp32", np.dtype(np.float32), CONTEXT_SHAPE)
        timestep = _require_array(timestep_bf16_bits, "timestep_bf16_bits", np.dtype(np.uint16), TIMESTEP_SHAPE)
        output = np.empty(OUTPUT_SHAPE, dtype=np.float32, order="C")
        request = _ExecuteRequest(
            ctypes.sizeof(_ExecuteRequest),
            image.ctypes.data,
            image.nbytes,
            *IMAGE_SHAPE,
            context.ctypes.data_as(ctypes.POINTER(ctypes.c_float)),
            context.nbytes,
            *CONTEXT_SHAPE,
            timestep.ctypes.data,
            timestep.nbytes,
            *TIMESTEP_SHAPE,
            output.ctypes.data_as(ctypes.POINTER(ctypes.c_float)),
            output.nbytes,
        )
        raw = _ExecuteEvidence()
        raw.struct_size = ctypes.sizeof(_ExecuteEvidence)
        if self._dll.prometheus_zimage_session_execute(self._handle, ctypes.byref(request), ctypes.byref(raw)) != 0:
            raise RuntimeError(self._last_error())
        if raw.main_layer_count != 30 or raw.model_allocation_ceiling_bytes not in MODEL_EXECUTION_PROFILE_CEILINGS:
            raise RuntimeError(f"incomplete native evidence: layers={raw.main_layer_count}, allocation={raw.model_allocation_ceiling_bytes}")
        if not np.isfinite(output).all():
            raise RuntimeError("Prometheus returned a non-finite image stream")
        main_layer_trace = []
        for layer in range(30):
            stage_offset = layer * len(MAIN_STAGE_NAMES)
            counter_offset = layer * len(NATIVE_COUNTER_NAMES)
            main_layer_trace.append({
                "layer": layer,
                "correlation_id": f"{raw.main_correlation_id[layer]:016x}",
                "cpu_begin_ns": int(raw.main_cpu_begin_ns[layer]),
                "cpu_end_ns": int(raw.main_cpu_end_ns[layer]),
                "parameter_generation": int(raw.main_parameter_generation[layer]),
                "execution_generation": int(raw.main_execution_generation[layer]),
                "active_weight_window": int(raw.main_active_weight_window[layer]),
                "gpu_total_begin_tick": int(raw.main_gpu_total_begin_tick[layer]),
                "gpu_total_end_tick": int(raw.main_gpu_total_end_tick[layer]),
                "gpu_total_seconds": raw.main_gpu_total_ns[layer] / 1e9,
                "gpu_compute_begin_tick": int(raw.main_gpu_compute_begin_tick[layer]),
                "gpu_compute_end_tick": int(raw.main_gpu_compute_end_tick[layer]),
                "gpu_compute_seconds": raw.main_gpu_compute_ns[layer] / 1e9,
                "gpu_ingress_transfer_seconds": raw.main_gpu_ingress_transfer_ns[layer] / 1e9,
                "gpu_joint_copy_seconds": raw.main_gpu_joint_copy_ns[layer] / 1e9,
                "gpu_stages": {
                    name: {
                        "begin_tick": int(raw.main_stage_gpu_begin_tick[stage_offset + stage]),
                        "end_tick": int(raw.main_stage_gpu_end_tick[stage_offset + stage]),
                        "seconds": raw.main_stage_gpu_ns[stage_offset + stage] / 1e9,
                    }
                    for stage, name in enumerate(MAIN_STAGE_NAMES)
                },
                "host_phases_seconds": {
                    "active_target_validation": raw.main_active_target_validation_ns[layer] / 1e9,
                    "command_reset": raw.main_command_reset_ns[layer] / 1e9,
                    "command_begin": raw.main_command_begin_ns[layer] / 1e9,
                    "command_record": raw.main_command_record_ns[layer] / 1e9,
                    "command_end": raw.main_command_end_ns[layer] / 1e9,
                    "queue_submit": raw.main_queue_submit_ns[layer] / 1e9,
                    "fence_wait": raw.main_fence_wait_ns[layer] / 1e9,
                    "descriptor_update": raw.main_descriptor_update_ns[layer] / 1e9,
                    "staging_memcpy": raw.main_staging_memcpy_ns[layer] / 1e9,
                },
                "native_counters_cumulative": {
                    name: int(raw.main_native_counters[counter_offset + counter])
                    for counter, name in enumerate(NATIVE_COUNTER_NAMES)
                },
            })
        evidence = ExecuteEvidence(
            evaluation_index=raw.evaluation_index,
            main_layer_count=raw.main_layer_count,
            context_reused=bool(raw.context_reused),
            wall_time_seconds=raw.wall_time_ns / 1e9,
            model_execution_seconds=raw.model_execution_ns / 1e9,
            parameter_rebind_seconds=raw.parameter_rebind_ns / 1e9,
            uploaded_weight_bytes=raw.uploaded_weight_bytes,
            model_allocation_ceiling_bytes=raw.model_allocation_ceiling_bytes,
            persistent_bytes=raw.persistent_bytes,
            reusable_bytes=raw.reusable_bytes,
            audit_bytes=raw.audit_bytes,
            host_package_cache_bytes=raw.host_package_cache_bytes,
            host_package_cache_hits=raw.host_package_cache_hits,
            prefetch_transfer_seconds=raw.prefetch_transfer_ns / 1e9,
            prefetch_overlap_seconds=raw.prefetch_overlap_ns / 1e9,
            prefetch_wait_seconds=raw.prefetch_wait_ns / 1e9,
            prefetch_count=raw.prefetch_count,
            stage_execution_seconds=tuple(value / 1e9 for value in raw.stage_execution_ns),
            stage_gpu_execution_seconds=tuple(value / 1e9 for value in raw.stage_gpu_execution_ns),
            stage_rebind_seconds=tuple(value / 1e9 for value in raw.stage_rebind_ns),
            stage_payload_read_seconds=tuple(value / 1e9 for value in raw.stage_payload_read_ns),
            stage_uploaded_weight_bytes=tuple(int(value) for value in raw.stage_uploaded_weight_bytes),
            main_layer_trace=tuple(main_layer_trace),
            final_readback_gpu_seconds=raw.final_readback_gpu_ns / 1e9,
            final_readback_host_seconds=raw.final_readback_host_ns / 1e9,
        )
        return output, evidence

    def close(self) -> None:
        handle = getattr(self, "_handle", 0)
        if handle:
            if self._dll.prometheus_zimage_session_destroy(handle) != 0:
                message = self._last_error(0)
                self._handle = 0
                raise RuntimeError(message)
            self._handle = 0

    def __enter__(self) -> "PrometheusZImageSession":
        return self

    def __exit__(self, exc_type, exc_value, traceback) -> None:
        self.close()
