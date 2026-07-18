# Codex collaboration

## ROALoop

ROALoop is the distinctive collaboration architecture behind the submission,
not a separate autonomous agent product. Its unusual feature is the separation
between a **persistent reviewer** and **ephemeral author tasks**: ChatGPT retains
the cross-milestone product context and helps write the next bounded prompt;
each fresh Codex task enters the real repository as an implementation author,
produces code/tests/artifacts, and ends with an evidence-bearing handoff.

1. The human acts as product manager and technical owner: selects the next
   bounded question, fixes acceptance criteria, approves scope, chooses hardware
   and evidence standards, and decides whether a result warrants continuation.
2. Persistent ChatGPT acts as reviewer and prompt editor across milestones: it
   retains the program narrative, compares the handoff against prior evidence,
   and helps frame architecture, hypotheses, and the next acceptance contract.
3. Ephemeral GPT-5.6 Codex author tasks read the real repository, implement
   across layers, run tests/hardware lanes, record artifacts, and report
   limitations.
4. The human reviews the evidence and redirects the next author task. Occasional
   Claude work or review exists in the pre-event history; repository tests and
   measured artifacts, not model agreement, are the final authority.

This is deliberately different from a single long autonomous coding session.
Persistence belongs to the review and product loop; implementation context is
re-grounded from repository truth in each author task. The committed reports,
tests, and artifacts are the durable memory between tasks.

## Exact GPT-5.6 use

Local Codex session records show `gpt-5.6-sol` for the fused reduction, M40a,
SDSL-V M41, M42–M47, M49, and M49a verticals. `gpt-5.6-terra` appears in the
M40b and M48 continuations and is the recorded model for M49b and the two plugin
productization/dogfooding tasks. The primary M48 task used both Sol and Terra.

This is build-time use of GPT-5.6 inside Codex. The shipped language/runtime
does not call the GPT-5.6 API at runtime, and the submission does not pretend it
does. The event FAQ says GPT-5.6 may be used for part of the project; here it was
used materially across the eligible engineering work.

## One-shot vertical examples

- **M39b:** one task moved from an existing stub through Vulkan planning,
  shaders, registry, lifecycle, CPU oracle, tests, benchmarks, and report.
- **M41:** one task reconciled SDSL-V compute/graphics language law and added
  parser/AST/validator/lowering/HLSL/toolchain/conformance/golden artifacts.
- **M42–M47:** successive sessions implemented a real device-resident path from
  attention through second residual, each with permanent tests and hardware
  artifacts rather than isolated code generation.
- **M48:** one long task first stopped truthfully at a planning/reference
  boundary, then continued into a live four-layer executor, consolidation,
  hardware evidence, and a numerical audit instead of asserting success from
  finite outputs.
- **Plugin dogfood:** one task tested the agent interface as its intended user,
  removed four speculative tools, and changed CLI/artifact semantics based on
  observed friction.

## Human decisions that changed the product

- The project to submit is the Oct/SDSL-V/Prometheus Build Week extension, not
  merely the MCP wrapper.
- Existing foundations must be disclosed and not counted as new.
- Vulkan work proceeds as bounded milestones with permanent evidence and a
  fixed RTX witness before cross-vendor claims.
- SDSL-V graphics is a compiler/toolchain milestone; a runtime/engine was kept
  out of scope.
- M48 EVT could not close merely because final values were finite. The human
  directed a deeper numerical audit when FP16 paths diverged by depth.
- Post-hoc scalar correction and broad tolerance widening were rejected.
- M49a could select a tunable MVP checkpoint/control architecture without
  pretending its parameters were certified.
- Local Codex should use the repository and canonical CLI; hosted MCP is a
  different, bounded virtual-workspace boundary.

## Codex changed its own interface after dogfooding

The original MCP exposed compiler-shaped operations such as `oct_check`,
`oct_build`, and `oct_explain_diagnostic`. The dogfooding campaign found that a
local coding agent naturally used `oct test` and `oct artifact` because those
commands already owned repository semantics. The redesign:

- added `oct.cli.result.v1` structured output;
- made compiled/interpreted fallback counts explicit;
- added exact artifact path/type/size/SHA-256 metadata;
- narrowed artifact default scope to the selected entry package;
- reduced the hosted tool set to `oct_workspace_info`, `oct_test`,
  `oct_artifact`, bounded `oct_run`, and scoped `oct_get_artifact`;
- replaced five overlapping MCP-first skills with two repository-workflow
  skills.

The deterministic evidence is
[`docs/development/artifacts/oct_mcp_agent_dogfooding.json`](../development/artifacts/oct_mcp_agent_dogfooding.json).

## Failure modes and course corrections

| Failure | Course correction |
| --- | --- |
| M48 planning contract existed before a live fixed-stack owner. | Reported “meaningful progression,” retained null timings, then implemented the real owner in the continuation. |
| Reduced-precision final output was finite but violated stage bounds by depth. | Preserved failing evidence, postponed EVT, added stage readbacks and M49 research. |
| Retaining FP32 Hidden did not help once Wdown recreated the FP16 boundary. | Rejected that branch; measured matched-input stage gain and complete-block checkpoint policies. |
| A fitted utility model lacked held-out authority. | Kept fitted scores shadow-only; selected explicit bounded/tunable policy. |
| The original MCP guessed what an LLM needed. | Dogfooded the product, removed speculative operations, and delegated local semantics to the CLI. |
| Compiled record-table support was incomplete. | Structured results disclose interpreted fallbacks; compiled mode fails rather than hiding the gap. |

## Claude boundary

Claude is an occasional historical contributor, not the author of the eligible
transformer vertical. The repository explicitly records an original pre-event
`oct-mcp` contribution in commit `0ffa09e10c45116141047c69541e90b50b276da8`
and author metadata in several libraries. GPT-5.6 Codex productized and then
redesigned the plugin during Build Week. No eligible-period Claude session is
claimed without session evidence.
