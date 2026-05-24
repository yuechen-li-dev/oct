# New Experiment Authoring Checklist

- Use `.oct` for helpers/modules and `.octest` for `[Fact]`/`[Theory]`/`[Artifact]` entry functions.
- `Assert.*` is implicit in `.octest` files (no explicit import needed).
- Do not replace `[Fact]`/`Assert.*` behavior tests with `fn Main() -> Int` smoke tests to work around fixture wiring issues.
- Only add `manifest.oct` when you are intentionally using package-manifest mode; a stub `manifest.oct` like `package Main` is invalid and will fail directory targets.
- Put `[Artifact]` functions in `.octest` files (artifact lane ownership).
- Keep pure compute helpers (for example `Run*Sweep`, `BuildSummary`, `BuildRows`) side-effect free: no `Artifact.*`, no `IO.Write*`, no file/directory writes.
- Keep artifact side effects in dedicated `[Artifact]` entrypoints only (for example `*ArtifactWriteAll`).
- Milestones should not import each other; place shared helpers under `Shared`.
- Use Markdown report pattern: `Markdown.Title` + `Markdown.Subtitle` + `Markdown.Report([...])`.
- In artifact functions, prefer `Artifact.Write*` sinks.
- For prose artifact output (including `FINDINGS.md`), use `Artifact.WriteMarkdown(path, markdownLines)` with `Markdown.Report([...])` content; do not use `IO.Write*` unless you intentionally need non-artifact low-level file APIs.
- For long-running `[Artifact]` loops/phases, emit explicit progress with `Artifact.Progress(label, current, total)` and phase boundaries with `Artifact.Checkpoint(label)`. `Artifact.Progress` is numeric progress only; for message-only phase markers use `Artifact.Checkpoint(label)`.
- Avoid invoking artifact generation from regular `[Fact]`/`[Theory]` test paths; keep FlowSmoke suites compute-only and keep artifact IO in artifact suites.
- Use `String.From<T>` / `String.Concat` for report-string construction.
- Use `FloorToInt` / `CeilToInt` / `RoundToInt` instead of `Int(...)` conversion-style calls.
- Use `Result(machine)!` only when completion is guaranteed; otherwise propagate/handle fallibility.
- Start with milestone-local smoke commands before full matrix runs.
- For long-running science milestones, run small compiled smoke suites first, then artifact suite, then broader milestone suite to stay inside cycle budgets.
- Use `--execution auto` first to probe compiled support posture.
- Oct syntax uses one statement per line/block statement; semicolon-separated inline statements are not supported.

- For Octomata scalar board observation between steps, use `BoardSnapshot(machine)!` (or `?`/`match`) and keep per-step arrays in external accumulators.

- Prefer `switch` expressions for multi-way classification; Oct does not support `else if`.
- Use semantic boolean operators: `and`, `or`, `not` (not `&&`, `||`, `!`).
