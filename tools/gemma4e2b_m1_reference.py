"""Capture authoritative Gemma 4 E2B text-layer-0 reference boundaries.

This is a reference-only harness.  It uses the pinned Transformers 5.6.2
Gemma4 modules for the layer operators, reads only the exact M1 tensor rows
needed from the owner-local safetensors checkpoint, and writes compact JSON
summaries plus small numerical fixtures.  It is never a Prometheus fallback.
"""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path

import torch
import torch.nn.functional as F
from safetensors import safe_open
from transformers.models.gemma4.configuration_gemma4 import Gemma4TextConfig
from transformers.models.gemma4.modeling_gemma4 import (
    Gemma4RMSNorm,
    Gemma4TextDecoderLayer,
    apply_rotary_pos_emb,
    eager_attention_forward,
)


REPOSITORY = "google/gemma-4-E2B-it"
REVISION = "3e22461f65e89153144f8adb70e3b8c2cc9845a7"
CHECKPOINT_SHA256 = "2db5482b20d746879bb3ef79b5203e9075a2e2b98f54ec7c2f281c1477ddc550"
TOKEN_IDS = [2, 105, 2364, 107, 1567, 506, 1171, 8355, 1548, 236761, 106, 107, 105, 4368, 107]
WIDTH = 1536
PLE_WIDTH = 256
LAYERS = 35
HEADS = 8
HEAD_DIM = 256
EPSILON = 1e-6


def summary(value):
    """Return an explicitly FP32-reduced, representation-stable tensor summary."""
    x = value.detach().float().cpu().contiguous()
    finite = torch.isfinite(x)
    finite_values = x[finite]
    raw = x.numpy().astype("<f4", copy=False).tobytes()
    return {
        "shape": list(x.shape),
        "source_dtype": str(value.dtype).replace("torch.", ""),
        "summary_dtype": "float32",
        "accumulation_precision": "float32 summary reduction",
        "element_count": x.numel(),
        "minimum": float(finite_values.min()) if finite_values.numel() else None,
        "maximum": float(finite_values.max()) if finite_values.numel() else None,
        "finite_count": int(finite.sum()),
        "nan_count": int(torch.isnan(x).sum()),
        "infinity_count": int(torch.isinf(x).sum()),
        "l1_norm": float(torch.linalg.vector_norm(x, ord=1)),
        "l2_norm": float(torch.linalg.vector_norm(x)),
        "linf_norm": float(x.abs().max()),
        "sha256_f32_le": hashlib.sha256(raw).hexdigest(),
        "comparison_policy": "Prometheus FP32 activations compared elementwise against this BF16-source reference; abs<=0.25 or rel<=0.02 for ordinary values, abs<=0.01 where |reference|<0.5.",
    }


def bf16_precision_comparison(reference, actual):
    """Compare a complete captured BF16 tensor without accepting tolerance drift."""
    reference = reference.detach().float().contiguous().view(-1)
    actual = actual.detach().float().contiguous().view(-1)
    difference = (actual - reference).abs()
    worst = int(difference.argmax())
    nonzero_reference = reference.abs() >= 0.5
    relative = torch.zeros_like(difference)
    relative[nonzero_reference] = difference[nonzero_reference] / reference[nonzero_reference].abs()
    reference_bits = int(reference[worst].to(torch.bfloat16).view(torch.uint16).item())
    actual_bits = int(actual[worst].to(torch.bfloat16).view(torch.uint16).item())
    return {
        "element_count": int(reference.numel()),
        "exact_match_count": int((actual == reference).sum()),
        "differing_count": int((actual != reference).sum()),
        "max_absolute_error": float(difference[worst]),
        "max_relative_error": float(relative.max()),
        "relative_l2": float(torch.linalg.vector_norm(actual - reference) / torch.linalg.vector_norm(reference)),
        "worst_flat_index": worst,
        "reference": float(reference[worst]),
        "actual": float(actual[worst]),
        "reference_bf16_hex": f"0x{reference_bits:04x}",
        "actual_bf16_hex": f"0x{actual_bits:04x}",
    }


def projection_precision_diagnostics(layer, embeddings, initial_norm, projection_outputs):
    """Distinguish activation storage from CPU BF16 linear backend behavior.

    This is reference-only evidence.  Production Vulkan never imports this
    computation or its outputs.
    """
    raw_norm = layer.input_layernorm._norm(embeddings.float()) * layer.input_layernorm.weight.float()
    def strict_scalar(terms):
        accumulator = torch.tensor(0.0, dtype=torch.float32)
        for term in terms:
            accumulator = accumulator + term
        return accumulator

    def pairwise_tree(terms):
        values = list(terms.unbind())
        while len(values) > 1:
            values = [
                values[index] + values[index + 1] if index + 1 < len(values) else values[index]
                for index in range(0, len(values), 2)
            ]
        return values[0]

    compact_terms = torch.tensor([16777216.0, -8388608.0, -8388608.0, -0.5], dtype=torch.float32)
    compact_scalar = strict_scalar(compact_terms)
    compact_pairwise = pairwise_tree(compact_terms)
    result = {
        "reference_expression": "torch.nn.Linear(BF16 activation, BF16 checkpoint weight)",
        "operand_dtypes": {"activation": str(initial_norm.dtype), "weight": "torch.bfloat16"},
        "activation_fp32_before_output_cast": bf16_precision_comparison(initial_norm, raw_norm),
        "torch": torch.__version__,
        "mkldnn_available": bool(torch.backends.mkldnn.is_available()),
        "mkldnn_enabled": bool(torch.backends.mkldnn.enabled),
        "mkl_available": bool(torch.backends.mkl.is_available()),
        "thread_count": torch.get_num_threads(),
        "interop_thread_count": torch.get_num_interop_threads(),
        "environment": {name: os.environ.get(name) for name in ("OMP_NUM_THREADS", "MKL_NUM_THREADS", "ONEDNN_MAX_CPU_ISA")},
        "projections": {},
        "framework_stability": {},
        "compact_contraction_order_witness": {
            "terms": compact_terms.tolist(),
            "strict_scalar_fp32": float(compact_scalar),
            "strict_scalar_bf16_hex": f"0x{int(compact_scalar.to(torch.bfloat16).view(torch.uint16).item()):04x}",
            "pairwise_tree_fp32": float(compact_pairwise),
            "pairwise_tree_bf16_hex": f"0x{int(compact_pairwise.to(torch.bfloat16).view(torch.uint16).item()):04x}",
        },
    }
    original_threads = torch.get_num_threads()
    original_mkldnn = torch.backends.mkldnn.enabled
    try:
        for name, reference in projection_outputs.items():
            linear = getattr(layer.self_attn, f"{name}_proj")
            result["projections"][name] = {
                "A_fp32_precast_activation_bf16_weight_fp32_accum_bf16_output": bf16_precision_comparison(
                    reference, F.linear(raw_norm, linear.weight.float()).to(torch.bfloat16)
                ),
                "B_bf16_activation_bf16_weight_fp32_accum_bf16_output": bf16_precision_comparison(
                    reference, F.linear(initial_norm.float(), linear.weight.float()).to(torch.bfloat16)
                ),
                "C_framework_bf16_linear": bf16_precision_comparison(reference, linear(initial_norm)),
            }
        for enabled in (True, False):
            torch.backends.mkldnn.enabled = enabled
            for threads in (1, original_threads):
                torch.set_num_threads(threads)
                key = f"mkldnn={enabled},threads={threads}"
                result["framework_stability"][key] = {
                    name: {
                        "against_captured": bf16_precision_comparison(reference, getattr(layer.self_attn, f"{name}_proj")(initial_norm)),
                        "repeat": bf16_precision_comparison(
                            getattr(layer.self_attn, f"{name}_proj")(initial_norm),
                            getattr(layer.self_attn, f"{name}_proj")(initial_norm),
                        ),
                    }
                    for name, reference in projection_outputs.items()
                }
    finally:
        torch.set_num_threads(original_threads)
        torch.backends.mkldnn.enabled = original_mkldnn
    return result


def compact_rows(value):
    """Small deterministic attention witness: first and final row of heads 0/7."""
    x = value.detach().float().cpu()
    rows = {}
    for head, row in ((0, 0), (0, x.shape[-2] - 1), (x.shape[1] - 1, x.shape[-2] - 1)):
        key = f"head_{head}_row_{row}"
        row_value = x[0, head, row].contiguous()
        rows[key] = {
            "values_f32": [float(v) for v in row_value],
            "sha256_f32_le": hashlib.sha256(row_value.numpy().astype("<f4", copy=False).tobytes()).hexdigest(),
        }
    return rows


def attention_invariants(probabilities):
    """Record the causal invariants required by the bounded prefill witness."""
    x = probabilities.detach().float().cpu()
    sequence = x.shape[-1]
    forbidden = torch.triu(torch.ones(sequence, sequence, dtype=torch.bool), diagonal=1)
    forbidden_values = x[..., forbidden]
    sums = x.sum(dim=-1)
    return {
        "finite": bool(torch.isfinite(x).all()),
        "forbidden_future_probability_linf": float(forbidden_values.abs().max()),
        "row_sum_min": float(sums.min()),
        "row_sum_max": float(sums.max()),
        "row_sum_max_abs_error_from_one": float((sums - 1.0).abs().max()),
        "topology": "causal sliding-window 512; this 15-token witness is shorter than the window",
    }


def load_authority(path):
    record = json.loads(Path(path).read_text(encoding="utf-8"))
    if record.get("schema") != "oct.prometheus.g4-e2b.checkpoint-authority.v1":
        raise RuntimeError("checkpoint authority schema mismatch")
    if record.get("repository") != REPOSITORY or record.get("revision") != REVISION:
        raise RuntimeError("checkpoint authority repository or revision mismatch")
    safe = record.get("safetensors", {})
    if safe.get("sha256") != CHECKPOINT_SHA256 or safe.get("tensor_count") != 2011:
        raise RuntimeError("checkpoint authority payload identity mismatch")
    return {item["name"]: item for item in safe["tensors"]}


def require_tensor(authority, name, shape):
    item = authority.get(name)
    if item is None:
        raise RuntimeError(f"required authoritative tensor missing: {name}")
    if item["dtype"] != "BF16" or item["shape"] != shape:
        raise RuntimeError(f"authoritative tensor incompatible: {name}")


def selected_rows(reader, name, ids):
    # get_slice avoids materializing either 768 MiB embedding table.
    view = reader.get_slice(name)
    return torch.cat([view[token : token + 1] for token in ids], dim=0)


def load_layer0(reader, config, authority):
    layer = Gemma4TextDecoderLayer(config, 0).to(dtype=torch.bfloat16)
    state = {}
    prefix = "model.language_model.layers.0."
    for key in layer.state_dict():
        name = prefix + key
        item = authority.get(name)
        if item is None:
            raise RuntimeError(f"layer 0 state missing from authority: {name}")
        state[key] = reader.get_tensor(name)
    missing, unexpected = layer.load_state_dict(state, strict=True)
    if missing or unexpected:
        raise RuntimeError("layer 0 state loading was not exact")
    return layer.eval()


def set_weight(module, tensor):
    with torch.no_grad():
        module.weight.copy_(tensor)


def capture(root, authority_path, fixture_root):
    config_raw = json.loads((root / "config.json").read_text(encoding="utf-8"))["text_config"]
    config = Gemma4TextConfig(**config_raw)
    if (config.hidden_size, config.head_dim, config.num_attention_heads, config.sliding_window) != (WIDTH, HEAD_DIM, HEADS, 512):
        raise RuntimeError("pinned layer-0 configuration is incompatible")
    authority = load_authority(authority_path)
    required = {
        "model.language_model.embed_tokens.weight": [262144, WIDTH],
        "model.language_model.embed_tokens_per_layer.weight": [262144, LAYERS * PLE_WIDTH],
        "model.language_model.per_layer_model_projection.weight": [LAYERS * PLE_WIDTH, WIDTH],
        "model.language_model.per_layer_projection_norm.weight": [PLE_WIDTH],
    }
    for name, shape in required.items():
        require_tensor(authority, name, shape)
    ids = torch.tensor([TOKEN_IDS], dtype=torch.long)
    path = root / "model.safetensors"
    with safe_open(path, framework="pt", device="cpu") as reader:
        base_rows = selected_rows(reader, "model.language_model.embed_tokens.weight", TOKEN_IDS)
        packed_ple = selected_rows(reader, "model.language_model.embed_tokens_per_layer.weight", TOKEN_IDS)
        projection = torch.nn.Linear(WIDTH, LAYERS * PLE_WIDTH, bias=False, dtype=torch.bfloat16)
        set_weight(projection, reader.get_tensor("model.language_model.per_layer_model_projection.weight"))
        ple_norm = Gemma4RMSNorm(PLE_WIDTH, eps=EPSILON).to(dtype=torch.bfloat16)
        set_weight(ple_norm, reader.get_tensor("model.language_model.per_layer_projection_norm.weight"))
        layer = load_layer0(reader, config, authority)

    # Gemma4TextScaledWordEmbedding does this multiplication in BF16.
    embeddings = base_rows.unsqueeze(0) * torch.tensor(math.sqrt(WIDTH), dtype=torch.bfloat16)
    token_ple = (packed_ple * torch.tensor(math.sqrt(PLE_WIDTH), dtype=torch.bfloat16)).reshape(1, len(TOKEN_IDS), LAYERS, PLE_WIDTH)
    context_ple = projection(embeddings) * (WIDTH**-0.5)
    context_ple = context_ple.reshape(1, len(TOKEN_IDS), LAYERS, PLE_WIDTH)
    context_ple = ple_norm(context_ple)
    effective_ple = (context_ple + token_ple) * (2.0**-0.5)
    ple0 = effective_ple[:, :, 0, :]

    positions = torch.arange(len(TOKEN_IDS), dtype=torch.long).unsqueeze(0)
    # This is the pinned local/default 1-D RoPE formula at theta 10,000.
    inv_freq = 1.0 / (10000.0 ** (torch.arange(0, HEAD_DIM, 2, dtype=torch.float32) / HEAD_DIM))
    freqs = (inv_freq[None, :, None] @ positions[:, None, :].float()).transpose(1, 2)
    angles = torch.cat((freqs, freqs), dim=-1)
    cos, sin = angles.cos().to(torch.bfloat16), angles.sin().to(torch.bfloat16)

    initial_norm = layer.input_layernorm(embeddings)
    q_linear = layer.self_attn.q_proj(initial_norm)
    k_linear = layer.self_attn.k_proj(initial_norm)
    v_linear = layer.self_attn.v_proj(initial_norm)
    q_normalized = layer.self_attn.q_norm(q_linear.view(1, len(TOKEN_IDS), HEADS, HEAD_DIM))
    q = apply_rotary_pos_emb(q_normalized, cos, sin, unsqueeze_dim=2).transpose(1, 2)
    k_normalized = layer.self_attn.k_norm(k_linear.view(1, len(TOKEN_IDS), 1, HEAD_DIM))
    k = apply_rotary_pos_emb(k_normalized, cos, sin, unsqueeze_dim=2).transpose(1, 2)
    v = layer.self_attn.v_norm(v_linear.view(1, len(TOKEN_IDS), 1, HEAD_DIM)).transpose(1, 2)
    sequence = len(TOKEN_IDS)
    mask = torch.full((1, 1, sequence, sequence), torch.finfo(torch.bfloat16).min, dtype=torch.bfloat16)
    mask = torch.triu(mask, diagonal=1)
    pre_softmax = torch.matmul(q, k.expand(-1, HEADS, -1, -1).transpose(2, 3)) + mask
    attention_before_projection, probabilities = eager_attention_forward(
        layer.self_attn, q, k, v, mask, dropout=0.0, scaling=1.0
    )
    attention_before_projection = attention_before_projection.reshape(1, sequence, HEADS * HEAD_DIM).contiguous()
    attention_after_projection = layer.self_attn.o_proj(attention_before_projection)
    post_attention_norm = layer.post_attention_layernorm(attention_after_projection)
    post_attention_residual = embeddings + post_attention_norm
    ffn_norm = layer.pre_feedforward_layernorm(post_attention_residual)
    gate = layer.mlp.gate_proj(ffn_norm)
    up = layer.mlp.up_proj(ffn_norm)
    activated_gated_product = layer.mlp.act_fn(gate) * up
    down = layer.mlp.down_proj(activated_gated_product)
    post_ffn_norm = layer.post_feedforward_layernorm(down)
    post_ffn_residual = post_attention_residual + post_ffn_norm
    ple_gate = layer.per_layer_input_gate(post_ffn_residual)
    ple_product = layer.act_fn(ple_gate) * ple0
    ple_projection = layer.per_layer_projection(ple_product)
    ple_norm_out = layer.post_per_layer_input_norm(ple_projection)
    output = post_ffn_residual + ple_norm_out
    output = output * layer.layer_scalar

    # Suppressing just the PLE contribution produces its pre-PLE residual exactly.
    ablated = post_ffn_residual * layer.layer_scalar
    fixture_root.mkdir(parents=True, exist_ok=True)
    (fixture_root / "token_ids.u32le.bin").write_bytes(torch.tensor(TOKEN_IDS, dtype=torch.int32).numpy().astype("<u4").tobytes())
    (fixture_root / "base_token_embedding.bf16le.bin").write_bytes(embeddings.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_input_rmsnorm.bf16le.bin").write_bytes(initial_norm.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_ple.bf16le.bin").write_bytes(ple0.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    # Projection-storage witnesses make the resident FP32 -> BF16 -> FP32
    # boundary independently auditable before per-head normalization.
    (fixture_root / "layer0_q_linear.bf16le.bin").write_bytes(q_linear.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_k_linear.bf16le.bin").write_bytes(k_linear.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_v_linear.bf16le.bin").write_bytes(v_linear.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    # The head-normalization fixtures are comparison authority only.  They
    # preserve the pre-RoPE [token, head, channel] layout so the Vulkan path
    # cannot accidentally hide a transpose or position transform error.
    (fixture_root / "layer0_q_normalized.bf16le.bin").write_bytes(q_normalized.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_k_normalized.bf16le.bin").write_bytes(k_normalized.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    # RoPE authority is retained in the production [token, head, component]
    # layout.  cos/sin are BF16 storage values computed from FP32 angles.
    (fixture_root / "layer0_rope_cos.bf16le.bin").write_bytes(cos.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_rope_sin.bf16le.bin").write_bytes(sin.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_q_rope.bf16le.bin").write_bytes(q.transpose(1, 2).cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_k_rope.bf16le.bin").write_bytes(k.transpose(1, 2).cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    (fixture_root / "layer0_output.bf16le.bin").write_bytes(output.cpu().view(torch.uint16).numpy().astype("<u2").tobytes())
    fixture_hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(fixture_root.glob("*.bin"))}
    boundaries = {
        "token_ids": TOKEN_IDS,
        "base_token_embedding": summary(embeddings),
        "per_layer_token_lookup_packed": summary(token_ple),
        "per_layer_context_projection_normalized": summary(context_ple),
        "layer0_effective_ple": summary(ple0),
        "layer0_input_rmsnorm": summary(initial_norm),
        "layer0_q_linear": summary(q_linear), "layer0_k_linear": summary(k_linear), "layer0_v_linear": summary(v_linear),
        "layer0_q_normalized": summary(q_normalized), "layer0_k_normalized": summary(k_normalized),
        "layer0_q_normalized_rope": summary(q), "layer0_k_normalized_rope": summary(k), "layer0_v_normalized": summary(v),
        "layer0_pre_softmax": summary(pre_softmax), "layer0_attention_probabilities": summary(probabilities),
        "layer0_attention_before_output_projection": summary(attention_before_projection),
        "layer0_attention_after_output_projection": summary(attention_after_projection),
        "layer0_post_attention_residual": summary(post_attention_residual), "layer0_feed_forward_norm": summary(ffn_norm),
        "layer0_gate_projection": summary(gate), "layer0_up_projection": summary(up),
        "layer0_activated_gated_product": summary(activated_gated_product), "layer0_down_projection": summary(down),
        "layer0_post_ffn_residual": summary(post_ffn_residual), "layer0_ple_residual": summary(ple_norm_out),
        "layer0_output": summary(output), "layer0_output_ple_suppressed": summary(ablated),
    }
    projection_diagnostics = projection_precision_diagnostics(
        layer, embeddings, initial_norm, {"q": q_linear, "k": k_linear, "v": v_linear}
    )
    return {
        "schema": "oct.prometheus.g4-e2b.m1-reference.v1", "repository": REPOSITORY, "revision": REVISION,
        "reference": {"transformers": __import__("transformers").__version__, "torch": torch.__version__, "device": "cpu", "weights_dtype": "bfloat16", "attention_implementation": "eager"},
        "layer0_graph": ["scaled_token_embedding", "input_rmsnorm", "local_qkv", "qk_rmsnorm_then_rope", "v_rmsnorm", "causal_local_attention", "o_proj", "post_attention_rmsnorm", "residual", "pre_ffn_rmsnorm", "tanh_gelu_gate_times_up", "down_proj", "post_ffn_rmsnorm", "residual", "ple_gated_residual", "layer_scalar"],
        "ple": {"token_lookup": "embed_tokens_per_layer[id] * sqrt(256), packed [35,256]", "context": "RMSNorm((base_embedding @ W^T) / sqrt(1536))", "combine": "(token_lookup + context) / sqrt(2)", "layer0_injection": "after ordinary FFN residual via gelu(per_layer_input_gate(residual)) * ple0, projection, RMSNorm, residual", "initial_embedding_combination": "none; PLE does not alter the layer initial hidden state"},
        "attention": {"identity": "sliding_attention", "heads": HEADS, "kv_heads": 1, "head_dim": HEAD_DIM, "scale": 1.0, "rope_theta": 10000.0, "causal": True, "sliding_window": 512, "pre_softmax_rows": compact_rows(pre_softmax), "probability_rows": compact_rows(probabilities), "invariants": attention_invariants(probabilities)},
        "boundaries": boundaries, "ple_ablation": {"effective_layer_input_delta_linf": 0.0, "final_output_delta": summary(output - ablated)},
        "projection_precision_diagnostics": projection_diagnostics,
        "fixtures": {"root": str(fixture_root), "files_sha256": fixture_hashes},
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-root", type=Path, required=True)
    parser.add_argument("--authority", type=Path, required=True)
    parser.add_argument("--fixture-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    torch.manual_seed(0)
    torch.use_deterministic_algorithms(True)
    result = capture(args.model_root, args.authority, args.fixture_root)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
