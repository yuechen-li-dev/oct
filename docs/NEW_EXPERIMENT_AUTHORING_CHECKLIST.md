# New Experiment Authoring Checklist

- Use `.oct` for helpers/modules and `.octest` for `[Fact]`/`[Theory]`/`[Artifact]` entry functions.
- `Assert.*` is implicit in `.octest` files (no explicit import needed).
- Do not replace `[Fact]`/`Assert.*` behavior tests with `fn Main() -> Int` smoke tests to work around fixture wiring issues.
- Only add `manifest.oct` when you are intentionally using package-manifest mode; a stub `manifest.oct` like `package Main` is invalid and will fail directory targets.
- Put `[Artifact]` functions in `.octest` files (artifact lane ownership).
- Milestones should not import each other; place shared helpers under `Shared`.
- Use Markdown report pattern: `Markdown.Title` + `Markdown.Subtitle` + `Markdown.Report([...])`.
- In artifact functions, prefer `Artifact.Write*` sinks.
- Use `String.From<T>` / `String.Concat` for report-string construction.
- Use `FloorToInt` / `CeilToInt` / `RoundToInt` instead of `Int(...)` conversion-style calls.
- Use `Result(machine)!` only when completion is guaranteed; otherwise propagate/handle fallibility.
- Start with milestone-local smoke commands before full matrix runs.
- Use `--execution auto` first to probe compiled support posture.
