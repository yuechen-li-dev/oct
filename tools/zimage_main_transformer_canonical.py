"""Generate the local, source-pinned M2C MainTransformer FP32 authority.

This is intentionally a closed laboratory for `layers.0`, not a transformer
framework. It consumes the accepted NoiseRefiner1 and ContextRefiner1 FP32
resident boundaries, the captured 256-wide conditioning vector, and the
representative's immutable FP16 cache. The source order is preserved as
Joint = Concat(Image[1024], Context[32]).
"""

import argparse
import hashlib
import json
import os
from pathlib import Path

# CUBLAS must see the determinism configuration before torch initializes CUDA.
os.environ.setdefault("CUBLAS_WORKSPACE_CONFIG", ":4096:8")

import numpy as np
import torch


CHECKPOINT = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
REVISION = "f332072aa78be7aecdf3ee76d5c247082da564a6"
CACHE_AGGREGATE = "48e987811885741ae5f1bf16b28db33ca7f23e09f1e99c1c2fe3d81bdd1caeb6"
WIDTH, HEADS, HEAD_WIDTH, HIDDEN = 3840, 30, 128, 10240
IMAGE_TOKENS, CONTEXT_TOKENS = 1024, 32
TOKENS = IMAGE_TOKENS + CONTEXT_TOKENS


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def write(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def float32_file(path: Path, expected_elements: int) -> np.ndarray:
    values = np.fromfile(path, dtype="<f4")
    if values.size != expected_elements:
        raise RuntimeError(f"{path} has {values.size} values; want {expected_elements}")
    if not np.isfinite(values).all():
        raise RuntimeError(f"{path} contains a non-finite value")
    return values


def bf16_file(path: Path, expected_elements: int) -> np.ndarray:
    bits = np.fromfile(path, dtype="<u2")
    if bits.size != expected_elements:
        raise RuntimeError(f"{path} has {bits.size} BF16 values; want {expected_elements}")
    values = (bits.astype("<u4") << 16).view("<f4")
    if not np.isfinite(values).all():
        raise RuntimeError(f"{path} contains a non-finite value")
    return values


class Authority:
    def __init__(self, cache_root: Path, oracle_root: Path, out: Path, block: str,
                 input_joint: Path | None, summary_only: bool):
        self.cache_root = cache_root
        self.oracle_root = oracle_root
        self.out = out
        self.block = block
        self.input_joint = input_joint
        self.summary_only = summary_only
        self.cache_dir = cache_root / "layers" / CHECKPOINT / block
        self.stage_dir = out / "stages"
        self.stages: dict[str, dict] = {}
        self.selected: dict[str, dict] = {}
        self.cache_manifest = json.loads((self.cache_dir / "manifest.json").read_text(encoding="utf-8"))
        if (self.cache_manifest.get("schema") not in {"oct.prometheus.evt2.m2c.fp16-cache.v1", "oct.prometheus.evt2.m2d.fp16-cache.v1"} or
                self.cache_manifest.get("block") != block or
                len(self.cache_manifest.get("tensors", [])) != 13):
            raise RuntimeError(f"{block} cache manifest is not a closed MainTransformer package")
        self.tensors = {item["source_name"]: item for item in self.cache_manifest["tensors"]}
        self.device = torch.device("cuda")

    def weight(self, suffix: str) -> torch.Tensor:
        name = self.block + suffix
        entry = self.tensors.get(name)
        if entry is None:
            raise RuntimeError(f"cache lacks {name}")
        values = np.fromfile(self.cache_dir / entry["destination_name"], dtype="<f2")
        shape = tuple(entry["destination_shape"])
        if values.size != int(np.prod(shape)):
            raise RuntimeError(f"cache tensor is truncated: {name}")
        # Every immutable packed FP16 operand expands to FP32 at its use.
        return torch.from_numpy(values.reshape(shape).astype("<f4", copy=False)).to(self.device)

    def record(self, name: str, tensor: torch.Tensor, selected: bool = False) -> None:
        if self.summary_only and name not in {"final_joint_output", "final_image_output", "final_context_output"}:
            return
        values = tensor.detach().to("cpu", dtype=torch.float32).contiguous().numpy().astype("<f4", copy=False)
        if not np.isfinite(values).all():
            raise RuntimeError(f"non-finite canonical stage: {name}")
        payload = values.tobytes(order="C")
        path = self.stage_dir / f"{name}.f32.bin"
        write(path, payload)
        record = {
            "relative_path": f"stages/{name}.f32.bin",
            "sha256": sha256(payload),
            "bytes": len(payload),
            "dtype": "float32",
            "shape": list(values.shape),
            "finite": True,
            "min": float(values.min()),
            "max": float(values.max()),
            "absolute_max": float(np.abs(values).max()),
        }
        if selected:
            self.selected[name] = record
        else:
            self.stages[name] = record

    def provenance(self, path: Path) -> dict:
        data = path.read_bytes()
        return {"relative_path": path.name, "sha256": sha256(data), "bytes": len(data), "dtype": "float32"}


def rmsnorm(value: torch.Tensor, scale: torch.Tensor) -> torch.Tensor:
    return value * torch.rsqrt(torch.mean(value * value, dim=-1, keepdim=True) + 1.0e-5) * scale


def rope(value: torch.Tensor, coordinates: torch.Tensor) -> torch.Tensor:
    # The source computes three independently partitioned complex rotations
    # [32,48,48], theta=256, after Q/K RMSNorm. `value` is [T,30,128].
    start = 0
    for axis, width in enumerate((32, 48, 48)):
        pairs = width // 2
        frequencies = 1.0 / torch.pow(
            torch.tensor(256.0, device=value.device),
            torch.arange(0, width, 2, device=value.device, dtype=torch.float32) / float(width),
        )
        angles = coordinates[:, axis:axis + 1] * frequencies[None, :]
        cosine, sine = torch.cos(angles)[:, None, :], torch.sin(angles)[:, None, :]
        even = value[:, :, start:start + width:2]
        odd = value[:, :, start + 1:start + width:2]
        value[:, :, start:start + width:2] = even * cosine - odd * sine
        value[:, :, start + 1:start + width:2] = even * sine + odd * cosine
        start += width
    return value


def joint_coordinates(device: torch.device) -> torch.Tensor:
    image = torch.arange(IMAGE_TOKENS, device=device, dtype=torch.float32)
    image_coordinates = torch.stack((
        torch.full((IMAGE_TOKENS,), 33.0, device=device),
        torch.floor_divide(image, 32.0),
        torch.remainder(image, 32.0),
    ), dim=1)
    context = torch.arange(1, CONTEXT_TOKENS + 1, device=device, dtype=torch.float32)
    context_coordinates = torch.stack((context, torch.zeros_like(context), torch.zeros_like(context)), dim=1)
    return torch.cat((image_coordinates, context_coordinates), dim=0)


def attention(authority: Authority, q: torch.Tensor, k: torch.Tensor, v: torch.Tensor) -> torch.Tensor:
    output = torch.empty_like(q)
    scale = HEAD_WIDTH ** -0.5
    selected = {0: "image_query0_head0", IMAGE_TOKENS: "context_query0_head0"}
    for head in range(HEADS):
        scores = torch.matmul(q[:, head, :], k[:, head, :].transpose(0, 1)) * scale
        probabilities = torch.softmax(scores, dim=1)
        if head == 0:
            for token, prefix in selected.items():
                authority.record(f"attention_logits_{prefix}", scores[token], selected=True)
                authority.record(f"attention_probabilities_{prefix}", probabilities[token], selected=True)
        output[:, head, :] = torch.matmul(probabilities, v[:, head, :])
    return output.reshape(TOKENS, WIDTH)


def build(authority: Authority) -> dict:
    cache = authority.cache_root
    revision_root = cache / "canonical" / REVISION
    image_path = revision_root / "o19-fp32-reference" / "noise_refiner.1" / "final_output.f32.bin"
    context_path = revision_root / "m2b-fp32-reference" / "context_refiner.1" / "final_output.f32.bin"
    timestep_path = authority.oracle_root / "run_02" / "noise_refiner_0_timestep.bin"
    timestep = bf16_file(timestep_path, 256).reshape(1, 256)
    if authority.input_joint is None:
        image = float32_file(image_path, IMAGE_TOKENS * WIDTH).reshape(IMAGE_TOKENS, WIDTH)
        context = float32_file(context_path, CONTEXT_TOKENS * WIDTH).reshape(CONTEXT_TOKENS, WIDTH)
        joint = torch.from_numpy(np.concatenate((image, context), axis=0)).to(authority.device)
        source_provenance = {
            "prepared_image_stream": authority.provenance(image_path),
            "prepared_context_stream": authority.provenance(context_path),
        }
    else:
        joint = torch.from_numpy(float32_file(authority.input_joint, TOKENS * WIDTH).reshape(TOKENS, WIDTH)).to(authority.device)
        source_provenance = {"previous_joint_state": authority.provenance(authority.input_joint)}
    source_provenance["conditioning"] = {"relative_path": "run_02/noise_refiner_0_timestep.bin", "sha256": sha256(timestep_path.read_bytes()), "bytes": timestep_path.stat().st_size, "dtype": "bfloat16"}
    condition = torch.from_numpy(timestep).to(authority.device)
    authority.record("image_input", joint[:IMAGE_TOKENS])
    authority.record("context_input", joint[IMAGE_TOKENS:])
    authority.record("joint_input", joint)

    adaln_weight, adaln_bias = authority.weight(".adaLN_modulation.0.weight"), authority.weight(".adaLN_modulation.0.bias")
    adaln = condition @ adaln_weight + adaln_bias
    authority.record("adaln_projection", adaln)
    attention_scale_raw, attention_gate_raw, mlp_scale_raw, mlp_gate_raw = torch.chunk(adaln, 4, dim=1)
    attention_scale, mlp_scale = 1.0 + attention_scale_raw, 1.0 + mlp_scale_raw
    attention_gate, mlp_gate = torch.tanh(attention_gate_raw), torch.tanh(mlp_gate_raw)
    authority.record("attention_scale", attention_scale)
    authority.record("attention_gate", attention_gate)
    authority.record("mlp_scale", mlp_scale)
    authority.record("mlp_gate", mlp_gate)
    del adaln_weight, adaln_bias, adaln, attention_scale_raw, attention_gate_raw, mlp_scale_raw, mlp_gate_raw

    attn_norm1 = authority.weight(".attention_norm1.weight")
    norm = rmsnorm(joint, attn_norm1)
    authority.record("attention_norm", norm)
    modulated = norm * attention_scale
    authority.record("attention_modulated", modulated)
    del attn_norm1, norm, attention_scale

    qkv_weight = authority.weight(".attention.qkv.weight")
    qkv = modulated @ qkv_weight
    authority.record("qkv", qkv)
    del qkv_weight, modulated
    q, k, v = qkv.reshape(TOKENS, 3, HEADS, HEAD_WIDTH).unbind(dim=1)
    authority.record("q", q.reshape(TOKENS, WIDTH))
    authority.record("k", k.reshape(TOKENS, WIDTH))
    authority.record("v", v.reshape(TOKENS, WIDTH))
    del qkv
    q_norm, k_norm = authority.weight(".attention.q_norm.weight"), authority.weight(".attention.k_norm.weight")
    q, k = rmsnorm(q, q_norm), rmsnorm(k, k_norm)
    authority.record("q_norm", q.reshape(TOKENS, WIDTH))
    authority.record("k_norm", k.reshape(TOKENS, WIDTH))
    del q_norm, k_norm
    coordinates = joint_coordinates(authority.device)
    q, k = rope(q, coordinates), rope(k, coordinates)
    authority.record("positioned_q", q.reshape(TOKENS, WIDTH))
    authority.record("positioned_k", k.reshape(TOKENS, WIDTH))
    del coordinates

    aggregated = attention(authority, q, k, v)
    authority.record("attention_aggregation", aggregated)
    del q, k, v
    attention_out = authority.weight(".attention.out.weight")
    projected = aggregated @ attention_out
    authority.record("attention_projection", projected)
    del aggregated, attention_out
    attn_norm2 = authority.weight(".attention_norm2.weight")
    attention_residual = joint + attention_gate * rmsnorm(projected, attn_norm2)
    authority.record("attention_residual", attention_residual)
    del projected, attn_norm2, attention_gate, joint

    ffn_norm1 = authority.weight(".ffn_norm1.weight")
    ffn_normed = rmsnorm(attention_residual, ffn_norm1)
    authority.record("ffn_norm", ffn_normed)
    ffn_modulated = ffn_normed * mlp_scale
    authority.record("ffn_modulated", ffn_modulated)
    del ffn_norm1, ffn_normed, mlp_scale
    w1_weight, w3_weight = authority.weight(".feed_forward.w1.weight"), authority.weight(".feed_forward.w3.weight")
    w1, w3 = ffn_modulated @ w1_weight, ffn_modulated @ w3_weight
    authority.record("w1", w1)
    authority.record("w3", w3)
    del ffn_modulated, w1_weight, w3_weight
    hidden = torch.nn.functional.silu(w1) * w3
    authority.record("gated_hidden", hidden)
    del w1, w3
    w2_weight = authority.weight(".feed_forward.w2.weight")
    w2 = hidden @ w2_weight
    authority.record("w2", w2)
    del hidden, w2_weight
    ffn_norm2 = authority.weight(".ffn_norm2.weight")
    final = attention_residual + mlp_gate * rmsnorm(w2, ffn_norm2)
    authority.record("final_joint_output", final)
    authority.record("final_image_output", final[:IMAGE_TOKENS])
    authority.record("final_context_output", final[IMAGE_TOKENS:])
    del attention_residual, mlp_gate, w2, ffn_norm2
    torch.cuda.synchronize()
    return source_provenance


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--cache-root", type=Path, required=True)
    parser.add_argument("--oracle-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--block", default="layers.0")
    parser.add_argument("--input-joint", type=Path)
    parser.add_argument("--summary-only", action="store_true")
    args = parser.parse_args()
    if not torch.cuda.is_available():
        raise RuntimeError("M2C canonical authority requires the validated CUDA RTX laboratory")
    torch.use_deterministic_algorithms(True)
    torch.set_float32_matmul_precision("highest")
    torch.backends.cuda.matmul.allow_tf32 = False
    torch.backends.cudnn.allow_tf32 = False
    authority = Authority(args.cache_root, args.oracle_root, args.out, args.block, args.input_joint, args.summary_only)
    provenance = build(authority)
    final_image = authority.stages["final_image_output"]
    final_context = authority.stages["final_context_output"]
    manifest = {
        "schema": "oct.prometheus.evt2.m2c.main-transformer-canonical.v1" if args.block == "layers.0" and args.input_joint is None else "oct.prometheus.evt2.m2d.main-transformer-canonical.v1",
        "representative": args.block,
        "family": "ZImageTurbo.MainTransformer",
        "source_revision": "26f23eda626ffadda020b04ff79488e1d72004cd",
        "model_revision": REVISION,
        "checkpoint_sha256": CHECKPOINT,
        "cache_aggregate_sha256": authority.cache_manifest["aggregate_sha256"],
        "precision_policy": "FP16 immutable packaged weights expanded to FP32 at use; FP32 activations, reductions, RoPE, softmax, projections, and residuals; no activation FP16",
        "stream_contract": {
            "composition": "Joint = Concat(Image, Context)",
            "image_offset": 0,
            "image_tokens": IMAGE_TOKENS,
            "context_offset": IMAGE_TOKENS,
            "context_tokens": CONTEXT_TOKENS,
            "joint_tokens": TOKENS,
            "width": WIDTH,
            "dtype": "float32",
            "image_coordinates": "[33, token//32, token%32] for token 0..1023",
            "context_coordinates": "[token+1, 0, 0] for token 0..31",
        },
        "inputs": provenance,
        "stages": dict(sorted(authority.stages.items())),
        "selected_attention": dict(sorted(authority.selected.items())),
        "final_image_output": final_image,
        "final_context_output": final_context,
        "execution": {"device": torch.cuda.get_device_name(0), "torch": torch.__version__, "tf32": False, "deterministic_algorithms": True},
    }
    encoded = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode("utf-8")
    write(args.out / "manifest.json", encoded)
    print(json.dumps({"manifest_sha256": sha256(encoded), "final_image_sha256": final_image["sha256"], "final_context_sha256": final_context["sha256"], "stage_count": len(authority.stages), "selected_attention_count": len(authority.selected)}, sort_keys=True))


if __name__ == "__main__":
    main()
