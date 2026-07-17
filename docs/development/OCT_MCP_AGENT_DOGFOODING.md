# Oct MCP agent dogfooding

## Convergence outcome

**SUCCESS** — the plugin now follows the workflows that worked in the Oct
repository instead of exposing a speculative compiler API. The durable record
is [oct_mcp_agent_dogfooding.json](artifacts/oct_mcp_agent_dogfooding.json).
Its generator is `go run ./tools/generate_oct_mcp_dogfooding`; its test and two
successive generation runs were byte-identical.

## What was inspected

This pass read the root instructions, README, CLI dispatch, `internal/cli`,
`internal/newpkg`, project discovery, parser/typechecker/interpreter/build
paths, test and artifact runners, diagnostic rendering, manifests, artifact writers,
`Language/reference`, representative Language contracts, Libraries, examples,
the ArrayMapGenerics experiment, OctErgonomicsLab, and the Prometheus M49 and
numerical-heterogeneity M0/M1 labs. It also read the original `cmd/oct-mcp`,
plugin metadata/skills, and existing publication material.

Repository facts that determined the design:

- `.octest` is the behavior contract lane; `oct test` owns `[Fact]`,
  `[Theory]`, and `.octfail` execution.
- `[Artifact]` is intentionally a different lane owned by `oct artifact`.
- `oct new library` creates a reusable package shape; `oct new experiment`
  creates `manifest.oct`, `REPORT.md`, and `M0/`. An experiment root is
  recognized by the presence of both manifest and report, not by its name.
- Root experiment `test`/`artifact` runs select every canonical
  `M<number>[letter]` directory in deterministic order. `Mx...` and `Shared`
  are excluded until directly targeted; `EntryMilestone` is not that selection
  authority.
- `auto` test mode first attempts compiled execution and then reports every
  interpreted fallback. `compiled` must fail rather than silently fall back.
- Source tests default to the selected entry package; imported test contracts
  require `--all-packages`. Artifacts now follow the same scoping rule.
- `Artifact.Write*` produces local evidence; interpreted artifact metadata can
  be captured from the writer boundary without inventing a second artifact
  implementation.
- The M49/M49a-related labs are bounded evidence experiments with explicit
  authority limits, not a GPU or generic remote-compute justification.

## Dogfooding campaign

The campaign used a disposable `.dogfood/McpDogfood` package, existing
Language/Libraries fixtures, an existing experiment, stdio MCP, and streamable
HTTP MCP discovery. It deliberately did not use Python.

| Scenario | Path used naturally | Repairs | Result and friction |
| --- | --- | ---: | --- |
| Default experiment scaffold | `oct new experiment DogfoodDefaultExperiment` | 0 | chose `Experiments/DogfoodDefaultExperiment` because `Experiments/` exists; M0 passed compiled. `oct init experiment` would have been insufficient because it writes no `REPORT.md`. |
| Experiment milestone and artifact | focused M0 test, then root `oct artifact` | 0 source / 1 CLI | a CSV artifact passed with a SHA-256. Initial JSON incorrectly reported zero milestone-prefixed passes; the parser was repaired and now reports the one selected M0 case. |
| Minimal `.octest` | direct `oct test` | 0 | scaffold passed once the generated file name was inspected; guessed scaffold names were wrong. |
| Syntax mistake | direct `oct test` | 1 | useful standalone-expression guidance repaired a trailing invalid expression. |
| Type mistake | direct `oct test` | 1 | `Identity` reported expected `Int`, got `String`. |
| Unit mistake | direct `oct test` | 1 | exact `Float<s>` versus `Float<m/s>` mismatch. |
| Record table | direct `oct test --execution auto --json` | 0 | passed interpreted with two explicit compiled fallbacks; compiled mode failed honestly. |
| Artifact production | direct `oct artifact --json` | 2 | initial wrong assumption omitted `import Artifact`; a second repair added Oct array commas in manifest dependencies. CSV/Markdown/JSON paths and hashes were returned. |
| Artifact scope | direct `oct artifact` | 0 | an imported package's artifact ran unexpectedly under the old behavior; default scope was narrowed and `--all-packages` made explicit. |
| Existing experiment | focused ArrayMapGenerics `.octest` | 0 | a doc-comment edit and focused auto run passed two compiled cases. |
| Compiled success | SmartGreenhouseController | 0 | seven cases passed compiled with zero fallbacks. |
| Empty workspace | direct `oct test` | 0 | correctly failed with `unknown package 'Main'`; no hidden scaffolding. |
| Nested/multi-file hosted project | `oct_artifact` | 0 | OctErgonomicsLab source + suite submitted as virtual files generated four scoped artifacts. |
| Invalid-to-repaired hosted flow | `oct_test` then `oct_artifact` | 1 | an existing invalid fixture produced a structured diagnostic; an existing valid fixture passed; artifact retrieval verified scoped hash metadata. |
| HTTP | streamable MCP | 0 | tool discovery passed. |

The deterministic JSON record lists every scenario's activated skill, MCP calls,
CLI calls, repair count, diagnostic, fallback, and outcome.

## Direct CLI, original MCP, and final mixed workflow

| Property | Direct CLI | Original six MCP tools | Final workflow |
| --- | --- | --- | --- |
| Routine correctness | `oct test` already authoritative | `oct_check` built `.oct`; could not run `.octest` | local `oct test --json`; hosted `oct_test` |
| Repair loop | one command after each edit | required `check` then guessed whether `run` was needed | test result is the single repair authority |
| Compiled/fallback status | human summary only | lost behind check/build abstraction | stable JSON fields and embedded MCP result |
| Artifacts | `oct artifact`, but paths/hashes were not structured | generic workspace scan and retrieval | CLI writer metadata; hosted scoped ID retrieval |
| Workspace semantics | native repository discovery | source-only temporary workspace, no `.octest` path | explicit local versus hosted split |
| Duplication | none | duplicated build/check/run semantics | MCP invokes canonical CLI lanes |

The original MCP-only path was unsuitable for ordinary Oct development: it
accepted only `.oct` entries, exposed `oct_check`/`oct_build` as separate
authorities, and scanned all workspace files for artifacts. The final local
path is mixed only in the useful sense: skill-guided repository inspection and
CLI execution. MCP is for the genuinely different hosted boundary.

## Final workflows

### Local Codex

1. Decide the ownership: stable reusable code starts with `oct new library`;
   milestone learning/evidence starts with `oct new experiment` and the
   `oct-experiments` skill.
2. Inspect the repository instructions, manifest, reference, and nearby
   contracts. For an experiment, work one canonical M0/M1 directory first and
   keep shared helpers out of milestone-to-milestone imports.
3. Edit `.oct` helpers and `.octest` test/artifact entries, then run
   `oct test <target> --execution auto --json`.
4. Repair the diagnostic and repeat. Report any `interpretedFallbacks`.
5. Run `oct artifact <target> --execution interpreted --json`, first on the
   focused milestone and then at the experiment root before claiming it.
6. Inspect the returned local paths, hashes, types, and update the experiment
   `REPORT.md` with exact evidence and remaining limitation.

`oct build` remains available as a CLI feature when a user specifically needs a
native program artifact. It is not a routine agent validation step. `oct run`
is likewise for program execution, not the contract authority.

### Hosted ChatGPT

1. Call `oct_workspace_info` once.
2. Submit bounded virtual `.oct`, `.octest`, and `.octfail` files to `oct_test`.
3. Repair from the returned `oct.cli.result.v1` diagnostic/result.
4. Submit the artifact entry to `oct_artifact`.
5. Retrieve only `oct_get_artifact` IDs returned by that execution.

The server creates and removes an owned workspace for each request. It copies
the reviewed runtime `Libraries/` source only for hosted test/artifact lanes;
there is no host-path, shell, package-install, network, or native-library
parameter. `oct_run` remains a bounded hosted playground command for a `.oct`
entry, not the ordinary workflow.

## Final MCP surface

- `oct_workspace_info`
- `oct_test`
- `oct_artifact`
- `oct_run` — hosted-playground-only
- `oct_get_artifact` — hosted scoped retrieval only

Removed before publication: `oct_describe`, `oct_check`, `oct_build`, and
`oct_explain_diagnostic`. `oct_run` was retained only because a hosted client
without a repository can have a genuine one-off program-execution need. Local
Codex should normally use the CLI instead.

## CLI and artifact changes

`oct test --json` now emits `oct.cli.result.v1`: command/version identity,
target, discovered test files, normalized command diagnostic, pass/fail/skip
counts, compiled case count, interpreted fallback count, timing, exit status,
and preserved human output. `oct artifact --json` uses the same envelope and
adds exact interpreted artifact path, MIME type, bytes, and SHA-256. Human
output is unchanged without `--json`. The milestone dogfood exposed that
prefixed `MILESTONE` output was initially omitted from the summary; JSON now
counts prefixed outcomes and reports only the canonical milestone files it
actually selects.

Artifact execution now defaults to the selected entry package, matching test
selection. `--all-packages` makes imported artifact lanes explicit. This avoids
an import turning a targeted evidence action into an unrelated package sweep.

## Skills

Five overlapping MCP-first skills were removed. Two non-overlapping skills now
cover the real local choices: `plugins/oct/skills/oct-workflow/SKILL.md` for
ordinary libraries/repository work and
`plugins/oct/skills/oct-experiments/SKILL.md` for experiment scaffolding,
canonical versus scratch milestones, focused/root validation, artifacts, and
`REPORT.md` evidence. They cite current reference paths instead of copying a
speculative manual.

## Remaining limitations and owner actions

- CLI diagnostics are stable command-level structured entries but many compiler
  errors still lack a source span/code at the CLI boundary; the original
  compiler message is preserved rather than invented.
- Artifact metadata is complete for interpreted `Artifact.Write*` operations.
  Compiled artifact mode explicitly marks metadata incomplete in this preview.
- Runtime package copying is intentionally bounded to the hosted test/artifact
  server image; hosted package installation, network access, sidecars, GPU, and
  arbitrary native libraries remain unsupported.
- An owner still must complete hosted deployment, legal/privacy/support fields,
  ingress controls, and marketplace/submission UI work. Nothing here claims
  public listing approval.
