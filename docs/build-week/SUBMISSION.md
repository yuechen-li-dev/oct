# Submission summary and claims review

## Judge thesis

Oct and SDSL-V are not week-old toy languages. They are pre-existing,
AI-assisted language/compiler projects. The Build Week submission is the
eligible GPT-5.6 Codex extension that joined their existing foundations to a
measured Vulkan transformer vertical, investigated its numerical behavior in
Oct, and then redesigned the Oct/Codex interface by using it for real work.

The judging unit is the Build Week delta documented in
[BUILD_WEEK_SCOPE.md](BUILD_WEEK_SCOPE.md), not all historical repository work
and not merely the MCP plugin.

## What judges can verify

1. Run one Oct correctness contract and one artifact lane through structured
   CLI output with [JUDGE_QUICKSTART.md](JUDGE_QUICKSTART.md).
2. Inspect the SDSL-V source, generated HLSL, validated SPIR-V, and manifest
   provenance committed for reduction, graphics, attention, and transformer
   stages.
3. Inspect machine-readable RTX 3070 artifacts for the bounded Vulkan path.
4. Re-run portable Go/compiler/plugin tests; live Prometheus GPU reproduction
   requires compatible Vulkan hardware and the documented native toolchain.
5. Trace every major claim to a commit, GPT-5.6 session, file, test, artifact,
   and limitation in [EVIDENCE_INDEX.md](EVIDENCE_INDEX.md).

## Final claims table

| Claim | Evidence and eligibility | Safe wording | Avoid |
| --- | --- | --- | --- |
| “built by AI” | Long-running Codex/Claude-authored repository history plus eligible GPT-5.6 session logs; the human committed and directed the work. | “AI-built under human orchestration” or “developed through a human-orchestrated AI engineering loop.” | “Autonomously built with no human decisions.” |
| “two programming languages” | Oct language/reference/corpus and Go implementation; SDSL-V spec, compiler, conformance corpus, HLSL/SPIR-V toolchain. Oct and SDSL-V predate Build Week. | “Two pre-existing AI-assisted programming-language projects, materially extended during Build Week.” | “Both languages were created during Build Week.” |
| “production fused reduction” | `adc527d…`; production registry/source ownership; validation and RTX corpus. | “Production-owned SDSL-V fused-reduction reactor within Prometheus, validated on the bounded recorded platform.” | “Production-ready on all GPUs.” |
| “graphics-capable shader language” | `f2d4ea8…`; vertex/pixel grammar, validation, conformance and golden artifacts. | “SDSL-V gained canonical vertex/pixel compilation and conformance output.” | “A complete graphics engine/runtime.” |
| “complete transformer block” | M42–M47 reports/artifacts; attention through second residual. | “A complete bounded experimental transformer block for the documented fixed topology.” | “A complete LLM inference engine.” |
| “multi-block stack” | M48 fixed four-block executor and artifact; later numerical audit postponed EVT closure. | “A fixed four-block device-resident stack with measured RTX evidence and explicit numerical caveats.” | “An arbitrary transformer graph or certified production model runtime.” |
| “numerical controller” | M49–M49b reports/artifacts and Shadow-HSFM state machine. | “An experimental bounded Shadow-HSFM observer/controller and checkpoint policy.” | “A generally validated stability controller.” |
| “beats CUDA” | No direct CUDA comparison exists. | “No CUDA superiority claim is made.” | “Beats CUDA.” |
| “replaces ComfyUI” | No such product or comparison is present. | Do not mention. | “Replaces ComfyUI.” |
| “portable across all hardware” | Windows RTX 3070 live evidence; Linux build/smoke; AMD DVT not complete. | “Portable compiler/tooling paths with one recorded live GPU platform.” | “Portable across all hardware.” |
| “hosted demo” | Deployment docs exist; no deployed endpoint is evidenced. | “A bounded MCP server and container path are included; no public hosted service is claimed.” | “Try the hosted demo.” |
| “GPT-5.6 use” | Session files record Sol and Terra across the eligible implementation threads. | “GPT-5.6 Sol performed most vertical work; Terra participated in named continuation/productization threads.” | “One GPT-5.6 thread built everything.” |
| “Claude contribution” | Original `oct-mcp` commit `0ffa09e…` and library manifests predate the event. | “Claude contributed to the pre-existing baseline; eligible plugin redesign was GPT-5.6 Codex work.” | “Claude built the Build Week transformer work.” |

## Selected factual limitations

- Oct is a pre-1.0 preview with incomplete compiled parity.
- SDSL-V graphics compilation does not provide a windowing or graphics runtime.
- Prometheus does not provide tokenizer, embeddings, rotary encoding, logits,
  sampling, KV cache, autoregressive decoding, model import, training, or an
  arbitrary graph scheduler.
- The recorded hardware results are one Windows RTX 3070/driver environment;
  they do not establish NVIDIA-wide, AMD, Linux-live, or cross-vendor behavior.
- Cooperative and other FP16 paths exhibited deterministic depth-correlated
  numerical drift in the bounded stack. The repository preserves that failure
  evidence rather than widening tolerances.
- The plugin is local and the MCP server is bounded; no public hosting or
  marketplace approval exists at submission-packet time.
