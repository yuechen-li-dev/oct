# Build Week scope

## Eligibility boundary

The official submission period began **2026-07-13 09:00:00 -07:00**. The last
pre-boundary commit is `8c029d6d8f8d5f698276edfda138fa96f5fb305e`
(2026-07-12 12:39:01 -07:00). The first eligible repository commit in this
checkout is `32b93d3385ac89ac2a4fbf180472e8abbea465e8`
(2026-07-15 16:40:50 -07:00). No July 13–14 commit is being inferred from
calendar framing.

## Pre-existing foundation — disclosed, not judged as new

- Oct language, parser, typechecker, interpreter, Go-compiled backend,
  Octomata, units, tensors, package/wrapper foundations, libraries, tests, and
  release tag `v0.1.0`.
- SDSL-V compute-language syntax, compiler pipeline, HLSL/DXC/SPIR-V route,
  test language, and SGEMM shader portfolio.
- Prometheus native Vulkan runtime, persistent-ring lifecycle, SGEMM,
  benchmarks, validation, and pre-M37 reports.
- ROALoop conventions: the human sets objectives and scope; model sessions
  implement/review; repository tests, artifacts, and human decisions arbitrate.
- Original `oct-mcp` baseline from Claude (`0ffa09e10c45116141047c69541e90b50b276da8`,
  2026-07-01) and earlier Claude-authored libraries.

## Eligible Build Week additions

All commits below are after the official start. Exact per-commit files and
limitations are in [BUILD_WEEK_COMMITS.json](BUILD_WEEK_COMMITS.json).

| Capability | Commit range / anchor | Result |
| --- | --- | --- |
| Cross-runtime SGEMM diagnosis and packed-memory experiment | `32b93d3…`, `06fd2a5…` | Bounded cause/placement evidence preceding the new reactor path. |
| SDSL-V workspace production ownership | `ef1c24c…` | Canonical production/experimental/historical shader layout. |
| Fused reduction reactor | `adc527d…` | Production-owned row sum/max/stable-softmax family with lifecycle, tests and RTX evidence. |
| Cooperative matrix proof and device-resident composition | `c81bbcd…`, `832e325…` | KHR cooperative-matrix proof plus bounded SGEMM→softmax handoff; experimental/default-off. |
| Canonical SDSL-V graphics | `f2d4ea8…` | Vertex/pixel compilation, diagnostics, conformance corpus, HLSL/SPIR-V golden bundle. |
| Attention through complete block | `0392276…` through `9fb3772…` | M42 attention, M43 grouped multi-head, M44 output projection, M45 residual, M46 RMSNorm, M47 gated FFN and second residual. |
| Fixed four-block stack | `00eab1b…`, `b58cf6c…`, `95e96ab…` | Device-resident fixed stack, working-set reuse, audit readbacks, and truthful numerical contradiction. |
| Numerical research in Oct | `a1ab67a…` through `be4bfd1…` | M49/M49a experiments, record tables/utility fitting, hardware evidence, and an explicit bounded checkpoint policy. |
| Shadow-HSFM/controller | `51b08bf…` | Experimental observer/controller integration and pre-DVT baseline. |
| Codex-native Oct workflow/plugin | `b07c884…` | Structured CLI results, bounded MCP tools, two focused skills, deterministic dogfooding report. |
| Submission/test-build repair | submission packet commit | Build Week docs, recording kit, CI parse fix and Windows/Linux `oct` + `oct-mcp` test artifacts. |

## Post-deadline or unsupported work

At packet preparation time (2026-07-17), the submission deadline had not
passed. Therefore no known post-deadline commit is included. Before submitting,
re-run the ledger validation and exclude any commit later than
**2026-07-21 17:00:00 -07:00**.

Unsupported or incomplete work is not converted into a claim:

- no hosted MCP endpoint;
- no public plugin-directory approval;
- no live Linux Vulkan or AMD/cross-vendor DVT;
- no CUDA/PTX comparison;
- no general transformer/LLM inference product;
- no graphics runtime;
- no universal numerical threshold/certification;
- no claim that all of Oct, SDSL-V, Prometheus, or ROALoop was created in the
  event window.
