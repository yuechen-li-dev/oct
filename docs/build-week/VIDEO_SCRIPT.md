# Video script

Target: **2:44** at approximately **146 words per minute**. Read the narration
verbatim. Brief pauses are already budgeted; do not add an intro animation.

## Exact narration

### 00:00–00:18 — Thesis and repository

This is Oct and SDSL-V: programming languages built by AI, used by AI. Oct is a
correctness-oriented scientific language. SDSL-V is a typed GPU language, and
Prometheus is their Vulkan runtime. Their foundations predate Build Week; this
demo is the evidence-backed extension built during it.

### 00:18–00:42 — ROALoop and eligible boundary

The unusual part is ROALoop. Persistent ChatGPT reviewed every handoff and
helped write the next bounded prompt. Fresh, ephemeral GPT-5.6 Codex tasks then
re-grounded in the repository, authored one vertical, ran its tests, and left
durable reports and artifacts. I chose architecture, scope, hardware questions,
and stop-or-continue decisions. Claude mainly rubber-ducked and audited; its
original MCP work was pre-event.

### 00:42–01:03 — Reduction and graphics

Build Week started with a production-owned fused-reduction reactor: SDSL-V
source, generated SPIR-V, Vulkan lifecycle, CPU oracle, tests, benchmarks, and
an RTX evidence report. Next, GPT-5.6 Codex added canonical vertex and pixel
compilation across parsing, validation, lowering, HLSL, SPIR-V, and conformance.
That is a graphics-capable compiler, not a graphics engine.

### 01:03–01:34 — Transformer vertical

Successive author tasks joined cooperative matrix work, device-resident
attention, four grouped heads, output projection, residual, RMSNorm, and gated
feed-forward into one complete bounded transformer block, then a fixed
four-block stack. Finite output was not enough: reduced precision drifted with
depth. We preserved the failure, added stage readbacks, tested mitigations, and
built an experimental Shadow-HSFM observer. This is one measured RTX 3070 path,
not a general LLM runtime or a cross-vendor claim.

### 01:34–02:00 — Oct test and artifact loop

The numerical work then became an Oct experiment. Here the real CLI compiles
two correctness facts. The same package runs its artifact lane separately and
returns structured evidence: exact path, JSON type, sixty-two bytes, and a
SHA-256 hash. Oct used native measurements to produce typed tables, plots, and
research reports. The toolchain is not just generated code; it is now part of
the engineering feedback loop.

### 02:00–02:30 — Codex plugin dogfooding

Finally, Codex became an Oct user. The first MCP design exposed compiler-shaped
operations. Dogfooding showed that a local coding agent naturally preferred
`oct test` and `oct artifact`, because those commands already own repository
semantics. GPT-5.6 Terra removed speculative tools, narrowed the hosted surface,
and added structured execution and artifact provenance. In the local plugin,
Codex can read a diagnostic, repair source, rerun the test, generate the
artifact, and report its hash.

### 02:30–02:44 — Closing thesis

The result is not a claim that two languages appeared in one week. It is a
human-directed, GPT-5.6 Codex extension of serious existing tools—and a workflow
where persistent AI review, ephemeral AI authors, and reproducible evidence
compound across milestones.

## Title card

```text
OCT + SDSL-V
PROGRAMMING LANGUAGES BUILT BY AI, USED BY AI

OpenAI Build Week 2026 · Developer Tools
github.com/yuechen-li-dev/oct
```

## Closing card

```text
PERSISTENT REVIEW · EPHEMERAL AUTHORS · DURABLE EVIDENCE

Oct + SDSL-V + Prometheus
github.com/yuechen-li-dev/oct
```

## YouTube package

**Title**

Oct and SDSL-V — Programming Languages Built by AI, Used by AI | Build Week

**Description**

```text
Oct is a correctness-oriented scientific programming language. SDSL-V is a
typed GPU language, and Prometheus is their Vulkan runtime.

For OpenAI Build Week 2026, a human-orchestrated workflow used persistent
ChatGPT review and ephemeral GPT-5.6 Codex author tasks to extend the existing
stack with fused reduction, canonical graphics compilation, a bounded Vulkan
transformer vertical, numerical research, and a Codex-native Oct workflow.

The language/runtime foundations predate Build Week. The repository documents
the exact eligible delta, commits, tests, task evidence, hardware artifacts,
supported platforms, and limitations.

Repository: https://github.com/yuechen-li-dev/oct
Judge quickstart: https://github.com/yuechen-li-dev/oct/blob/main/docs/build-week/JUDGE_QUICKSTART.md

No CUDA superiority, all-hardware portability, hosted demo, or general LLM
runtime claim is made. Recorded live GPU evidence is one Windows RTX 3070
environment.
```

**Thumbnail text**

```text
AI BUILT THE LANGUAGES.
THEN USED THEM.
```

Use a repository/code background and the Oct mark only. Do not add unrelated
vendor logos.
