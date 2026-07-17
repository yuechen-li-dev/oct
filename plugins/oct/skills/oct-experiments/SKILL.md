---
name: oct-experiments
description: Create, modify, validate, and report milestone-driven Oct experiments. Use for work under Experiments/, requests to run oct new experiment, formal M0/M1 milestone work, exploratory Mx milestones, experiment reports, or evidence artifacts; use the general oct-workflow skill for ordinary libraries.
---

# Oct experiments and milestones

Choose the package shape before writing code.

| Goal | Create | Shape |
| --- | --- | --- |
| Reusable, stable importable code | `oct new library Name` | `Name.Core.oct`, `Name.Core.octest`, `manifest.oct`, `README.md`; no milestone runner. |
| Bounded investigation that produces learning and evidence | `oct new experiment Name` | `manifest.oct`, `README.md`, `REPORT.md`, and `M0/` source/test files; root commands use milestone behavior. |

From this checkout before installing `oct`, prefix commands with
`go run ./cmd/oct`. With an `Experiments/` directory in the current workspace,
`oct new experiment Name` creates `Experiments/Name`; otherwise it creates
`Name`. Pass an explicit third path only when that placement is intentional.

Do not start an experiment with `oct init experiment` unless you will add the
rest of the shape yourself: it writes only `manifest.oct`. Root experiment
execution requires both `manifest.oct` and `REPORT.md`; `oct new experiment`
creates both.

## Milestone loop

1. Update the scaffold's ordered `Authors` and ISO `Date` manifest metadata.
   Keep `Kind: "experiment"` and the scaffolded `EntryMilestone: "M0"` unless
   changing the separate `oct exp run` entry behavior. Root `oct test` and
   `oct artifact` do **not** select only `EntryMilestone`.
2. Put M0 work in its generated `.oct` helper and `.octest` contract. For a
   new formal step, add `M1`, `M2`, or a lettered insertion such as `M2a`.
   A root experiment run selects canonical `M<number>[letter]` directories in
   numeric/letter order. `Mx<number>[letter]` is an exploratory scratch
   milestone: target it directly when needed, but root runs exclude it.
   `Shared/` is support code, not a root-executed milestone. Do not make
   milestones import each other; put shared helpers there.
3. Put pure calculations in `.oct`; put `[Fact]`, `[Theory]`, and `[Artifact]`
   entries in `.octest`. Keep artifact side effects in `[Artifact]` entries;
   start with `Artifact.Write*`, and use `Artifact.Progress` or
   `Artifact.Checkpoint` for a long artifact lane.
4. During a narrow edit, validate only the milestone:

   ```powershell
   oct test Experiments/Name/M1 --execution auto --json
   ```

   Repair from `diagnostics`, then repeat the same command. A success is
   `"ok":true`, no failed cases, and a stated fallback count. `auto` tries
   compiled execution first; an interpreted fallback is a real coverage gap,
   not compiled success. Use `--execution compiled` when that is required.
5. Generate evidence only after the contracts pass:

   ```powershell
   oct artifact Experiments/Name/M1 --execution interpreted --json
   ```

   Inspect each reported local path and its MIME type, byte count, and
   SHA-256. `oct test` never invokes `[Artifact]` entries.
6. Before claiming a milestone, validate the root too:

   ```powershell
   oct test Experiments/Name --execution auto --json
   oct artifact Experiments/Name --execution interpreted --json
   ```

   Root execution reports `MILESTONE ...` headings and runs every canonical
   milestone, not just the one being edited. Update `REPORT.md` with the
   question, method, result, artifact paths/hashes, compiled/interpreted
   execution counts, and the unresolved limitation.

Use `--all-packages` only when imported package tests or artifacts are
intentionally part of the run. Do not replace the loop with Python, a generic
shell/MCP workflow, or a `Main()` smoke test. Local Codex should use the CLI;
bounded hosted source submission is the separate MCP path in `oct-workflow`.

Read `docs/REPO_STRUCTURE.md`, `docs/NEW_EXPERIMENT_AUTHORING_CHECKLIST.md`,
and `Language/reference/tooling/31-octest.md` before extending a nontrivial
experiment.
