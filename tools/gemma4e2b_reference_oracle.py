"""Pinned, compact numerical oracle for google/gemma-4-E2B-it.

This script deliberately records only deterministic summaries and SHA-256
digests of activations. It never copies checkpoint weights or full activation
dumps into the repository.
"""

import argparse
import hashlib
import json
from pathlib import Path

import numpy as np
import torch
from PIL import Image
from transformers import AutoModelForMultimodalLM, AutoProcessor


REPOSITORY = "google/gemma-4-E2B-it"
REVISION = "3e22461f65e89153144f8adb70e3b8c2cc9845a7"
TEXT_MESSAGES = [{"role": "user", "content": [{"type": "text", "text": "Name the first prime number."}]}]
IMAGE_TEXT = "What are the two dominant colors in this image?"


def summary(value):
    if isinstance(value, (tuple, list)):
        value = value[0]
    if hasattr(value, "last_hidden_state"):
        value = value.last_hidden_state
    x = value.detach().float().cpu().contiguous()
    raw = x.numpy().tobytes()
    return {
        "shape": list(x.shape),
        "source_dtype": str(value.dtype).replace("torch.", ""),
        "summary_dtype": "float32",
        "accumulation_precision": "float32 summary reduction",
        "absolute_max": float(x.abs().max()),
        "l2_norm": float(torch.linalg.vector_norm(x)),
        "sha256_f32_le": hashlib.sha256(raw).hexdigest(),
    }


def deterministic_image():
    y, x = np.mgrid[0:48, 0:48]
    pixels = np.zeros((48, 48, 3), dtype=np.uint8)
    pixels[..., 0] = np.where(x < 24, 240, 16)
    pixels[..., 2] = np.where(x < 24, 20, 235)
    pixels[..., 1] = ((x + y) % 12) * 4
    return Image.fromarray(pixels, "RGB")


def capture_text(model, processor):
    inputs = processor.apply_chat_template(
        TEXT_MESSAGES,
        tokenize=True,
        return_dict=True,
        return_tensors="pt",
        add_generation_prompt=True,
        enable_thinking=False,
    )
    captures = {}
    handles = []

    def hook(name):
        def save(_module, _args, output):
            captures[name] = summary(output)
        return save

    def attention_hook(_module, _args, output):
        if len(output) > 1 and output[1] is not None:
            captures["layer0_attention_probabilities"] = summary(output[1])

    language = model.model.language_model
    layer0 = language.layers[0]
    middle = language.layers[len(language.layers) // 2]
    handles.extend([
        language.embed_tokens.register_forward_hook(hook("text_embedding")),
        language.embed_tokens_per_layer.register_forward_hook(hook("per_layer_token_embedding")),
        layer0.input_layernorm.register_forward_hook(hook("layer0_input_rmsnorm")),
        layer0.self_attn.q_proj.register_forward_hook(hook("layer0_q_linear")),
        layer0.self_attn.k_proj.register_forward_hook(hook("layer0_k_linear")),
        layer0.self_attn.v_proj.register_forward_hook(hook("layer0_v_linear")),
        layer0.self_attn.register_forward_hook(hook("layer0_attention_output")),
        layer0.self_attn.register_forward_hook(attention_hook),
        layer0.mlp.register_forward_hook(hook("layer0_feed_forward_output")),
        layer0.register_forward_hook(hook("layer0_output")),
        middle.register_forward_hook(hook("middle_layer_output")),
        language.norm.register_forward_hook(hook("final_rmsnorm")),
    ])
    try:
        with torch.inference_mode():
            out = model(**inputs, use_cache=True, output_attentions=True, return_dict=True)
    finally:
        for handle in handles:
            handle.remove()
    captures["final_logits"] = summary(out.logits)
    captures["token_ids"] = inputs["input_ids"][0].tolist()
    captures["greedy_next_token"] = int(out.logits[0, -1].argmax())
    captures["greedy_next_token_text"] = processor.decode([captures["greedy_next_token"]], skip_special_tokens=False)
    captures["attention_note"] = "The pinned eager reference returns post-softmax attention weights only when requested; this compact M0 harness records the attention output and does not claim a pre-softmax capture."
    return captures


def capture_image(model, processor):
    image = deterministic_image()
    rendered = processor.apply_chat_template(
        [{"role": "user", "content": [{"type": "image"}, {"type": "text", "text": IMAGE_TEXT}]}],
        tokenize=False,
        add_generation_prompt=True,
        enable_thinking=False,
    )
    inputs = processor(images=image, text=rendered, return_tensors="pt", return_mm_token_type_ids=True)
    captures = {
        "fixture": {
            "kind": "generated 48x48 RGB split red/blue with deterministic green checker ramp",
            "sha256_rgb": hashlib.sha256(np.asarray(image).tobytes()).hexdigest(),
        },
        "preprocessed_pixel_values": summary(inputs["pixel_values"]),
        "image_position_ids": summary(inputs["image_position_ids"]),
        "input_token_count": int(inputs["input_ids"].shape[-1]),
        "input_token_ids_sha256_u32_le": hashlib.sha256(inputs["input_ids"].cpu().numpy().astype("<u4").tobytes()).hexdigest(),
    }
    handles = []

    def hook(name):
        def save(_module, _args, output):
            captures[name] = summary(output)
        return save

    handles.extend([
        model.model.vision_tower.register_forward_hook(hook("vision_encoder_output")),
        model.model.embed_vision.register_forward_hook(hook("projected_multimodal_tokens")),
        model.model.language_model.layers[0].register_forward_hook(hook("first_decoder_multimodal_boundary")),
    ])
    try:
        with torch.inference_mode():
            out = model(**inputs, use_cache=True, return_dict=True)
    finally:
        for handle in handles:
            handle.remove()
    captures["final_logits"] = summary(out.logits)
    captures["greedy_next_token"] = int(out.logits[0, -1].argmax())
    captures["greedy_next_token_text"] = processor.decode([captures["greedy_next_token"]], skip_special_tokens=False)
    captures["image_placeholder_token_id"] = model.config.image_token_id
    positions = (inputs["input_ids"][0] == model.config.image_token_id).nonzero().flatten().tolist()
    captures["image_placeholder_positions"] = {"start": positions[0], "count": len(positions), "end_inclusive": positions[-1]}
    return captures


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-root", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--mode", choices=("text", "image", "both"), default="both")
    args = parser.parse_args()
    torch.manual_seed(0)
    torch.use_deterministic_algorithms(True)
    root = Path(args.model_root)
    processor = AutoProcessor.from_pretrained(root)
    model = AutoModelForMultimodalLM.from_pretrained(root, dtype=torch.bfloat16, attn_implementation="eager")
    model.eval()
    result = {
        "schema": "oct.prometheus.g4-e2b.reference-oracle.v1",
        "repository": REPOSITORY,
        "revision": REVISION,
        "reference": {"transformers": __import__("transformers").__version__, "torch": torch.__version__, "device": "cpu", "weights_dtype": "bfloat16", "attention_implementation": "eager", "sampling": "disabled; forward argmax"},
    }
    if args.mode in ("text", "both"):
        result["text"] = capture_text(model, processor)
    if args.mode in ("image", "both"):
        result["image"] = capture_image(model, processor)
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    Path(args.out).write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
