# Evidence index

## Audit boundary and public state

- Official event start: **2026-07-13 09:00 Pacific**.
- Last repository commit before that boundary:
  `8c029d6d8f8d5f698276edfda138fa96f5fb305e`, 2026-07-12 12:39:01
  Pacific.
- First post-boundary commit: `32b93d3385ac89ac2a4fbf180472e8abbea465e8`,
  2026-07-15 16:40:50 Pacific.
- Audited submission head: `b07c8849efa00fe0455e827e9a162856f389878f`,
  2026-07-17 12:51:42 Pacific, matching `origin/main` during the audit.
- Public repository: <https://github.com/yuechen-li-dev/oct>.
- License: [`LICENSE`](../../LICENSE), GPL-3.0.
- Releases at audit time: tag `v0.1.0` predates the event; no GitHub Release is
  published. A tracked Linux x86-64 `oct-mcp` is present.
- Exhaustive 23-commit machine-readable ledger:
  [`BUILD_WEEK_COMMITS.json`](BUILD_WEEK_COMMITS.json).

## Capability-to-evidence map

The Codex task IDs below identify local task records. They are supplementary
evidence, not substitutes for the owner-obtained `/feedback` ID.

| Capability | Commit and date (Pacific) | Codex task / model | Relevant files | Tests and artifacts | Supported claim | Limitation |
| --- | --- | --- | --- | --- | --- | --- |
| Cross-runtime groundwork and production shader layout | `32b93d3`, `06fd2a5`, `ef1c24c`; Jul 15 | preceding M38/M39 records; GPT-5.6 Codex | `Sidecars/KaijuVulkan/vulkan.go`; `internal/prometheus/native/reactor_vulkan_sgemm.c`; `internal/prometheus/shaders/sdslv/production/sgemm/` | M37b/M38a reports and contract tests; workspace checker | Eligible preparatory cleanup put existing SGEMM assets under explicit production ownership. | SGEMM and the runtime foundation pre-existed; do not count them as week-created. |
| Fused reduction reactor | `adc527d`; Jul 15 20:07 | `019f689b…`; `gpt-5.6-sol` | `reactor_vulkan_fused_reduction.c`; production reduction `.sdslv`; generated headers; registry | `reactor_reduction_tests.cpp`, benchmarks, audit tests, `FusedReductionSemantics.sdslvtest`, M39b report | Production-owned fused row-sum/max and softmax reduction path within the bounded Prometheus runtime. | Recorded platform only; “production” means repository ownership/lifecycle, not universal deployment readiness. |
| Vulkan cooperative matrix proof | `c81bbcd`; Jul 15 22:38 | `019f68f5…`; `gpt-5.6-sol` | cooperative SDSL-V/HLSL/SPIR-V, probe, audit API | M40A probe/benchmark JSON and native/Go/toolchain tests | Experimental cooperative matrix capability was probed and measured on RTX 3070. | Experimental and default-off; no cross-vendor claim. |
| Device-resident cooperative inference path | `832e325`; Jul 15 23:59 | `019f6977…`; Sol + Terra | cooperative path plus fused softmax assets | `device_resident_inference_rtx3070.json`, M40B report | A bounded cooperative SGEMM/reduction inference chain stayed device resident. | Fixed shapes/path; not a general model runtime. |
| Canonical SDSL-V graphics language | `f2d4ea8`; Jul 16 02:07 | `019f69df…`; `gpt-5.6-sol` | lexer/AST/parser/validator/lowering/HLSL/toolchain; graphics corpus | conformance tests; committed vertex/pixel HLSL, SPIR-V, bundle JSON | SDSL-V gained canonical vertex/pixel compilation, validation, conformance, and artifacts. | No window, swapchain, render pass, graphics pipeline runtime, or renderer. |
| Device-resident attention | `0392276`; Jul 16 03:45 | `019f6a39…`; `gpt-5.6-sol` | attention transpose/scale/pack shaders and Vulkan owner | `reactor_attention_tests.cpp`; `device_resident_attention_rtx3070.json`; M42 report | Bounded device-resident attention operator with CPU oracle and hardware evidence. | Fixed topology and recorded GPU. |
| Grouped multi-head attention | `a5f0fd8`; Jul 16 08:04 | `019f6a88…`; `gpt-5.6-sol` | grouped attention runtime path | native tests; `bounded_grouped_attention_rtx3070.json`; M43 report | Four-head bounded grouped attention path. | Fixed head count/dimensions; not arbitrary MHA. |
| Output projection | `7de0809`; Jul 16 09:37 | `019f6b77…`; `gpt-5.6-sol` | interleave/direct projection SDSL-V, HLSL, SPIR-V | native tests; M44 RTX JSON and report | Multi-head aggregation and output projection were joined on device. | Experimental fixed shape. |
| Residual and RMSNorm | `1032853`, `2ecd200`; Jul 16 | `019f6bca…`, `019f6bff…`; `gpt-5.6-sol` | residual and RMSNorm SDSL-V/HLSL/SPIR-V; runtime owner | M45/M46 RTX JSON, native tests, reports | Device-resident residual add and RMSNorm milestones. | Bounded topology and witness device only. |
| Gated FFN and complete bounded block | `9fb3772`; Jul 16 13:48 | `019f6c45…`; `gpt-5.6-sol` | `reactor_vulkan_transformer.c`; SiLU gate/pack assets | native transformer tests; `gated_ffn_complete_transformer_block_rtx3070.json`; M47 report | Attention through second residual formed one complete bounded experimental transformer block. | Not an LLM inference engine; no tokenizer, KV cache, decode, model import, or arbitrary graph. |
| Four-block stack and stage audit | `00eab1b`, `b58cf6c`, `95e96ab`; Jul 16 | `019f6cb4-b438-70e2-b91c-487d7ad45bbd`; Sol + Terra | transformer fixed-stack owner/internal API and stage readbacks | native tests; M48 report and JSON | A live fixed four-block device-resident stack was measured and audited. | Deterministic FP16 depth drift prevented unconditional EVT closure. |
| Numerical heterogeneity research | `a1ab67a`; Jul 16 | `019f6e6f…`; `gpt-5.6-sol` | M49 native research plus Oct M0 package | native research tests; Oct facts/artifacts; M49 RTX JSON | Numerical failure modes were preserved and investigated using Oct-generated research artifacts. | Synthetic M0 is not hardware validation; fitted ideas are not certified controls. |
| Controlled stage-gain/mitigation | `58adba6`, `a8c5d5c`, `734522c`, `be4bfd1`; Jul 17 | `019f6ea6…`, `019f6f49…`, `019f70c4…`; `gpt-5.6-sol` | Oct M1 import/analysis; record tables; utility fit; checkpoint implementation | Oct M1 tests/artifacts, M49a native tests/report/RTX JSON | Stage gain was measured, imported into Oct, and used to compare explicit checkpoint policies. | The fitted utility has no held-out authority; selected policy remains tunable/experimental. |
| Shadow-HSFM observer/controller | `51b08bf`; Jul 17 11:20 | `019f70e8…`; `gpt-5.6-terra` | transformer control C/H and numerical research API | native controller tests; M49b report and RTX JSON | Experimental bounded shadow observer/controller with explicit state and authority boundaries. | Shadow/experimental; not generally validated or enabled as universal control. |
| Codex-native plugin and dogfood redesign | `b07c884`; Jul 17 12:51 | `019f714f…`, `019f717c…`; `gpt-5.6-terra` | plugin manifest/skills/MCP; `internal/cli/structured.go`; deployment docs | MCP/CLI/generator tests; `oct_mcp_agent_dogfooding.json`; tracked Linux MCP binary | Codex used Oct's real test/artifact workflow; dogfooding narrowed the interface and added structured evidence. | Local plugin; no hosted endpoint, marketplace approval, Windows tracked prebuilt, or public release claimed. |

## Primary task recommendation

The owner ran `/feedback` in the M48 task and confirmed the returned Session ID
as **`019f6cb4-b438-70e2-b91c-487d7ad45bbd`**. It contains the largest coherent Build Week
vertical: a planning stop, a live four-block continuation, hardware evidence,
and the numerical course correction. It used both GPT-5.6 Sol and Terra.

This confirmed primary ID does not imply one task built the entire submission;
the supplementary task index records the distributed verticals.

Supplementary task groups are indexed in
[`BUILD_WEEK_COMMITS.json`](BUILD_WEEK_COMMITS.json), including the fused
reduction, graphics, M42–M47, M49–M49b, and plugin dogfooding tasks.

### Supplementary Codex task index

| Capability | Local Codex task ID | Recorded model(s) |
| --- | --- | --- |
| Fused reduction | `019f689b-2c49-7570-86f6-87e5a89edfe9` | `gpt-5.6-sol` |
| Cooperative matrix proof | `019f68f5-b438-73c0-aa3d-739abdad8e09` | `gpt-5.6-sol` |
| Cooperative inference continuation | `019f6977-decf-7233-a278-60a1f8ea102f` | Sol, Terra |
| Canonical SDSL-V graphics | `019f69df-5475-7dc2-8e35-b8defd0346cd` | `gpt-5.6-sol` |
| M42 attention | `019f6a39-2086-7bf1-ba63-3a6032484377` | `gpt-5.6-sol` |
| M43 grouped attention | `019f6a88-e97a-7f03-a704-8b6c00379113` | `gpt-5.6-sol` |
| M44 output projection | `019f6b77-fbbb-73b2-927d-a434eefe91d0` | `gpt-5.6-sol` |
| M45 residual | `019f6bca-ea6d-79b3-9321-5268bd30595c` | `gpt-5.6-sol` |
| M46 RMSNorm | `019f6bff-42ea-7cc0-bdaf-046e4b2b83e1` | `gpt-5.6-sol` |
| M47 gated FFN/block | `019f6c45-f0bd-77b3-8b6b-b243a9ca6775` | `gpt-5.6-sol` |
| M48 four-block stack/audit | `019f6cb4-b438-70e2-b91c-487d7ad45bbd` | Sol, Terra |
| M49 numerical research | `019f6e6f-a49a-7193-8bc4-72356442c6d4` | `gpt-5.6-sol` |
| M49a measure/import | `019f6ea6-1f55-7892-aa9b-ee5b2c0cc08b` | `gpt-5.6-sol` |
| M49a continuation | `019f6f49-5f02-7d93-8368-c23881e232dd` | `gpt-5.6-sol` |
| M49a closeout | `019f70c4-ec82-7ee0-b056-2d4c5f2e676d` | `gpt-5.6-sol` |
| M49b Shadow-HSFM | `019f70e8-d60f-75a3-a142-8273e92009f8` | `gpt-5.6-terra` |
| Plugin productization | `019f714f-48d1-77f0-bcb0-6379a06eac19` | `gpt-5.6-terra` |
| Plugin dogfood redesign | `019f717c-d9e5-7c10-baf9-db461693a6fb` | `gpt-5.6-terra` |

## Claims not supported by the evidence

There is no direct CUDA comparison, no ComfyUI replacement, no all-hardware
portability result, no hosted demo, no complete graphics runtime, and no
general-purpose production LLM inference engine. These claims are intentionally
absent from submission copy.
