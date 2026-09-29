"""Small, dependency-free presentation display for the canonical Z-Image run."""

from __future__ import annotations

import sys
import time
from dataclasses import dataclass
from pathlib import Path


EVALUATIONS = 9
BLOCKS_PER_EVALUATION = 34
TOTAL_BLOCKS = EVALUATIONS * BLOCKS_PER_EVALUATION
CANONICAL_PNG_SHA256 = "7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613"


def stage_label(stage_index: int) -> tuple[str, str | None]:
    if stage_index < 2:
        return "Noise Refiner", None
    if stage_index < 4:
        return "Context Refiner", None
    return "Main Transformer", f"{stage_index - 3} / 30"


def completed_blocks(evaluation: int, stage_index: int) -> int:
    """Return completed transformer blocks after a real stage completion."""
    return (evaluation - 1) * BLOCKS_PER_EVALUATION + stage_index + 1


def _elapsed(seconds: float) -> str:
    minutes, seconds = divmod(int(seconds), 60)
    return f"{minutes:02d}:{seconds:02d}"


@dataclass
class DemoProgress:
    gpu: str
    profile: str
    ceiling_bytes: int
    ansi: bool | None = None
    stream: object = sys.stdout

    def __post_init__(self) -> None:
        if self.ansi is None:
            self.ansi = bool(getattr(self.stream, "isatty", lambda: False)())
        self.started = time.perf_counter()
        self.evaluation = 0
        self.stage_index = -1
        self.status = "Loading canonical workflow"
        self._rendered = False

    def update(self, *, evaluation: int | None = None, stage_index: int | None = None, status: str | None = None) -> None:
        if evaluation is not None:
            self.evaluation = evaluation
        if stage_index is not None:
            self.stage_index = stage_index
        if status is not None:
            self.status = status
        self.render()

    def complete_stage(self, evaluation: int, stage_index: int) -> None:
        self.update(evaluation=evaluation, stage_index=stage_index, status="Executing")

    def _lines(self) -> list[str]:
        stage, layer = stage_label(max(self.stage_index, 0))
        done = completed_blocks(self.evaluation, self.stage_index) if self.evaluation and self.stage_index >= 0 else 0
        percent = done * 100 // TOTAL_BLOCKS
        width = 28
        filled = done * width // TOTAL_BLOCKS
        bar = "█" * filled + "░" * (width - filled)
        lines = [
            "Prometheus — Compiled Transformer Demo",
            "",
            "Prompt       A lighthouse in fog at dawn",
            "Seed         42",
            "Output       512×512",
            "Model        Z-Image-Turbo — official BF16 transformer",
            "Checkpoint   12,309,817,472 bytes",
            f"GPU          {self.gpu}",
            f"Profile      {self.profile} — {self.ceiling_bytes / 1_000_000_000:.3f} GB model-owned GPU memory",
            "",
            f"Overall      [{bar}] {percent}%",
            f"Evaluation   {self.evaluation} / {EVALUATIONS}" if self.evaluation else "Evaluation   preparing / 9",
            f"Stage        {stage}",
            f"Layer        {layer}" if layer else "Layer        —",
            f"Elapsed      {_elapsed(time.perf_counter() - self.started)}",
            f"Status       {self.status}",
        ]
        return lines

    def render(self) -> None:
        text = "\n".join(self._lines())
        if self.ansi:
            prefix = "\x1b[2J\x1b[H" if not self._rendered else "\x1b[H"
            print(prefix + text, file=self.stream, flush=True)
        else:
            if not self._rendered:
                print(text, file=self.stream, flush=True)
            else:
                done = completed_blocks(self.evaluation, self.stage_index) if self.evaluation and self.stage_index >= 0 else 0
                stage, layer = stage_label(max(self.stage_index, 0))
                layer_text = f", layer {layer}" if layer else ""
                print(f"progress {done}/{TOTAL_BLOCKS}: evaluation {self.evaluation}/{EVALUATIONS}, {stage}{layer_text}, elapsed {_elapsed(time.perf_counter() - self.started)} — {self.status}", file=self.stream, flush=True)
        self._rendered = True

    def complete(self, output: Path, png_hash: str, elapsed_seconds: float, ceiling_bytes: int | None) -> None:
        self.evaluation = EVALUATIONS
        self.stage_index = BLOCKS_PER_EVALUATION - 1
        self.status = "COMPLETE" if png_hash == CANONICAL_PNG_SHA256 else "HASH MISMATCH"
        self.started = time.perf_counter() - elapsed_seconds
        lines = self._lines() + [
            "",
            self.status,
            "Image        one complete 512×512 PNG",
            "Evaluations  9 / 9",
            f"Output PNG   {output.resolve()}",
            f"SHA-256      {png_hash}",
            "Canonical    MATCH" if png_hash == CANONICAL_PNG_SHA256 else f"Canonical    MISMATCH (expected {CANONICAL_PNG_SHA256})",
        ]
        if ceiling_bytes is not None:
            lines.append(f"Model GPU    {ceiling_bytes:,} bytes model-owned ceiling (reported by run)")
        text = "\n".join(lines)
        if self.ansi:
            print("\x1b[H" + text, file=self.stream, flush=True)
        else:
            print(text, file=self.stream, flush=True)
