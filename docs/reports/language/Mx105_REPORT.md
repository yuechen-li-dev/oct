# Mx105 Report — Canonical Repo-Wide Package Import Resolution

## 1) Root problem

Import resolution depended on a single active root (`<root>/<ImportName>`). In experiment milestone paths (for example `Experiments/.../M1`), this excluded repo libraries such as `Libraries/Octomata`, causing `unknown package 'Octomata'`.

## 2) Inconsistency found

- Resolver searched only active root siblings.
- `run`, `build`, `test`, and `artifact` were all routed through `project.Load` / `project.LoadForTest`, but the shared resolver model itself was too narrow for repo-wide package visibility.
- `.oct` vs `.octest` loading differed only by inclusion of test files, not by import algorithm, but real parity was still broken because both missed repo library roots when active root was a milestone directory.

## 3) Canonical model adopted

Single deterministic resolver order:

1. `<active-root>/<ImportName>`
2. `<repo-root>/Libraries/<ImportName>` (if present)
3. `<repo-root>/Packages/<ImportName>` (if present)

Repo root is discovered by walking upward from active root to nearest ancestor that contains `Libraries/` or `Packages/`.

## 4) Parity unification

- `.oct` and `.octest` both use the same resolver.
- `run`, `build`, `test`, and `artifact` continue to share the same loader path and now receive identical import behavior from the canonical resolver.

## 5) Diagnostics improvements

Missing-package errors now include deterministic context:

- requested package name
- importing package name
- active root
- searched directories
- manifest-mode/dependency/cache status when relevant

## 6) Regression tests added

- Library importing library (`Libraries/VectorOps` importing `Octomata`)
- Experiment importing library (`Experiments/.../M1/Main` importing `Octomata`)
- `.octest` importing library (`[Fact]` and `[Artifact]` importing `Octomata`)
- Command parity (`run`, `build`, `test`, `artifact` on same setup)
- Invocation-location robustness (same command succeeds from multiple cwd locations)
- Missing-package diagnostics include resolver context fields

## 7) Retained boundaries / limitations

- Package names remain directory-based and unqualified (`import Name`).
- Resolver remains deterministic and non-heuristic (no broad recursive search).
- Manifest and package-cache behavior is preserved; canonical repo roots are checked before cache fallback.
