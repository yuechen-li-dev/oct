# Devpost fields

All text below is ready to paste. Bracketed owner fields are the only values
that must be replaced. The narrative sections are intentionally compact and do
not depend on unpublished performance claims.

## Project name

Oct and SDSL-V: Programming Languages Built by AI, Used by AI

## Tagline

Persistent ChatGPT review and ephemeral GPT-5.6 Codex author tasks extended two AI-built languages into a measured Vulkan transformer stack—then Codex dogfooded Oct itself.

## Category

Developer Tools

## Short description

Oct is a correctness-oriented scientific language; SDSL-V is a typed GPU
language; Prometheus executes their shaders through Vulkan. During Build Week,
a human-orchestrated GPT-5.6 Codex workflow added fused reduction, canonical
graphics compilation, a bounded transformer vertical, numerical research, and
a Codex-native Oct test/artifact workflow. The language foundations predate the
event; this submission documents and demonstrates the eligible extension.

## Long description

Oct and SDSL-V are long-running, AI-assisted programming-language projects—not
week-old prototypes. Oct provides correctness-oriented scientific programming,
tests, units, packages, and reproducible artifacts. SDSL-V provides typed GPU
programs that lower to reviewable HLSL and validated SPIR-V. Prometheus is the
Vulkan execution runtime built around those languages.

Build Week extended that foundation through ROALoop, an unusual division of AI
labor: persistent ChatGPT retained cross-milestone context, reviewed evidence,
and helped prepare bounded prompts; fresh GPT-5.6 Codex author tasks repeatedly
re-grounded in the repository, implemented one vertical, ran its real tests and
hardware lanes, and left reports and machine-readable artifacts for the next
review. The human owner chose product direction, architecture, scope, evidence
standards, and stop/go decisions.

The eligible result includes a production-owned fused-reduction reactor;
canonical SDSL-V vertex/pixel compilation and conformance; an experimental
device-resident path from cooperative matrix and attention through grouped
multi-head attention, output projection, residual, RMSNorm, gated FFN, a
complete bounded block, and a fixed four-block stack; and numerical research
that preserved depth-related FP16 drift instead of hiding it. Oct then imported
that evidence for typed research artifacts. Finally, Codex dogfooded Oct,
causing the plugin to remove speculative tools and standardize structured
`oct test` and `oct artifact` results.

The repository clearly separates the pre-existing foundation from the 23
eligible commits. Judges can install the local Linux plugin without rebuilding,
run a deterministic Oct test/artifact fixture, and inspect committed RTX 3070
evidence without specialized hardware. No hosted demo, CUDA superiority,
cross-vendor portability, or general LLM runtime is claimed.

## Inspiration

Most AI coding workflows make one model carry both product memory and
implementation context. We wanted to test a different architecture: persistent
ChatGPT as reviewer and prompt author, ephemeral Codex tasks as repository-
grounded implementers, and a human as product manager and final authority. The
languages and their artifacts become the durable memory between tasks. Build
Week was the chance to see whether that loop could extend a serious existing
stack—and whether Codex could eventually use the language it helped improve.

## What it does

Oct lets developers express tested scientific programs and reproducible
artifacts. SDSL-V lets them express typed compute, vertex, and pixel GPU
programs that compile to HLSL and validated SPIR-V. Prometheus consumes bounded
SDSL-V assets in a Vulkan runtime. The Build Week vertical demonstrates fused
reductions and a fixed experimental transformer stack, records numerical and
hardware evidence, then exposes Oct's canonical test/artifact workflow to Codex
through a local plugin and bounded MCP server.

## How it was built

The repository uses Go for the Oct and SDSL-V compiler/tooling layers, Oct for
language contracts and scientific experiments, SDSL-V for GPU programs, and
C/C++ plus Vulkan for Prometheus. Each milestone was implemented vertically:
source language, lowering, generated HLSL/SPIR-V, runtime ownership, CPU oracle,
tests, hardware run, machine-readable artifact, and a development report. The
submission ledger maps every eligible commit to those files, tests, artifacts,
Codex tasks, supported claims, and limitations.

## How Codex and GPT-5.6 were used

GPT-5.6 ran inside Codex as the implementation author. Session records identify
GPT-5.6 Sol across fused reduction, graphics, the transformer vertical, and
numerical research; Terra participated in named continuations and authored the
Shadow-HSFM and plugin productization/dogfooding tasks. Codex inspected the real
repository, changed multiple layers, executed tests and RTX lanes, preserved
failed numerical evidence, and wrote auditable handoffs. Persistent ChatGPT
reviewed those handoffs and helped author the next bounded prompt. GPT-5.6 is a
build-time engineering dependency here, not a hidden runtime API call.

## Challenges

The hardest challenge was refusing false closure. A four-block FP16 path could
produce finite output while drifting outside stage bounds with depth. We added
layer and stage readbacks, tested hypotheses, rejected an ineffective retained-
FP32 branch, and kept the failing evidence. A second challenge was interface
design: the first MCP surface mirrored compiler operations, but real Codex
dogfooding preferred the repository's canonical CLI. We narrowed the tool set
and made execution fallbacks and artifact provenance explicit.

## Accomplishments

- Added a production-owned fused-reduction reactor with tests and evidence.
- Used direct GGML GLSL versus SDSL-V SPIR-V measurements to find a short-row
  weakness, then added and measured one bounded packed-row plan; the report
  includes wins, losses, and no matmul claim.
- Added canonical SDSL-V vertex/pixel compilation and conformance artifacts.
- Joined a bounded device-resident transformer block and fixed four-block stack.
- Used Oct to analyze native numerical evidence and generate reproducible
  research artifacts.
- Built and dogfooded an Oct plugin whose design changed from observed Codex
  behavior.
- Preserved an exact pre-event/eligible-work boundary and an exhaustive commit
  ledger rather than attributing the whole project to one week.

## Lessons learned

Persistent review plus disposable implementation context can work when the
repository carries durable contracts: tests, reports, generated artifacts, and
explicit limitations. One-shot vertical prompts are powerful, but only when a
human owns scope and acceptance. Finite output is not numerical correctness.
And the best agent interface may be the boring, canonical developer workflow—
not a larger set of AI-specific tools.

## What is next

Close numerical DVT across more shapes, depths, vendors, and live Linux Vulkan;
improve compiled parity for Oct's newer research surfaces; package signed
Windows/Linux releases; and evaluate a bounded hosted MCP deployment only after
security, privacy, support, and legal owner gates are complete. SDSL-V graphics
runtime work—windowing, swapchain, render pass, and pipeline state—remains a
separate future milestone.

## Repository URL

https://github.com/yuechen-li-dev/oct

## Testing instructions

No-rebuild server path: on Linux x86-64, clone the repository, run `chmod +x
./oct-mcp`, start `./oct-mcp serve --listen 127.0.0.1:8080`, and verify
`http://127.0.0.1:8080/healthz`. The strongest desktop-plugin path is the
`oct-build-week-windows-amd64` artifact from the successful submission CI run;
put its `dist` directory on `PATH`, restart the ChatGPT desktop app, and install
Oct from the repository's Personal marketplace. Direct fixture commands are
`oct test docs/build-week/recording/fixtures/JudgeDemo
--execution auto --json` and `oct artifact
docs/build-week/recording/fixtures/JudgeDemo --execution interpreted --json`.
Full steps and committed GPU evidence: `docs/build-week/JUDGE_QUICKSTART.md`.

## Supported platforms

Oct compiler/CLI and SDSL-V compiler tests support Windows x86-64 and Linux
x86-64. The tracked no-rebuild MCP binary is Linux x86-64; CI is configured to
produce Windows/Linux `oct`, `oct-mcp`, and plugin test artifacts. Authoritative
live Prometheus Build Week evidence is Windows x86-64 on one NVIDIA RTX 3070.

## Known limitations

Oct is pre-1.0 and does not have universal compiled parity. SDSL-V graphics is
a compiler/toolchain feature, not a graphics engine. The transformer is a fixed
experimental topology, not an LLM inference runtime. Cooperative/FP16 paths are
experimental and include disclosed depth-related drift. No AMD or live Linux
Vulkan DVT, hosted MCP endpoint, approved marketplace listing, public GitHub
Release, CUDA comparison, or all-hardware portability claim exists.

## Video URL

[VIDEO_URL_OWNER_REQUIRED]

## `/feedback` Session ID

019f6cb4-b438-70e2-b91c-487d7ad45bbd

Confirmed by the owner from `/feedback` in the M48 primary task.
