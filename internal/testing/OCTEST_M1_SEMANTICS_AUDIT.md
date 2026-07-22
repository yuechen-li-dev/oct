# OCTEST M1 Semantics Audit

Date: 2026-04-30
Scope: audit/design only (no runner behavior changes)

## 0) Sources audited

- CLI command routing in `internal/cli/cli.go`.
- Test/benchmark/artifact executors in `internal/tester/`.
- Project/package loading in `internal/project/project.go`.
- Attribute parse/validation in `internal/parse/parse.go`.
- Language reference contracts in `Language/reference/tooling/31-octest.md` and `35-cli.md`.
- Representative command-level tests under `cmd/oct/` (including mixed discovery and execution-mode tests).
- Attribute usage across `Libraries/**/*.octest` and `Experiments/**/*.octest`.

---

## 1) Discovery model (current)

### 1.1 How `.octest` files are discovered

- `oct test`, `oct bench`, and `oct artifact` all load source via `project.LoadForTest(path)`.
- `LoadForTest` sets `includeTests=true` and package loading admits both `.oct` and `.octest` extensions.
- Discovery is package-oriented (by directories/packages), not a separate independent `.octest` filesystem walker.

### 1.2 How `[Fact]`, `[Theory]`, `[Benchmark]`, `[Artifact]` are discovered

- Attributes are parsed onto `ast.FunctionDecl` flags (`IsFact`, `IsTheory`, `IsBenchmark`, `IsArtifact`) in parser stage.
- `tester.Execute` includes only `IsFact` and `IsTheory` functions.
- `tester.ExecuteBenchmarks` includes only `IsBenchmark` functions.
- `tester.ExecuteArtifacts` includes only `IsArtifact` functions.

### 1.3 Mixed attributes/files and multiple attributes per function

- A single `.octest` file can contain multiple functions of different categories (fact/theory/artifact/benchmark), and commands partition execution by function flags.
- A single function cannot combine test-lane attributes in disallowed pairs; parser rejects combinations (`[Fact]+[Theory]`, `[Artifact]+[Fact/Theory/Benchmark]`, `[Benchmark]+[Fact/Theory/Artifact]`).

Conclusion: mixed-file is supported today at file level, but not mixed-attribute on one function.

---

## 2) Execution model (current)

### 2.1 `[Fact]` / `[Theory]`

- Interpreted execution path (`interpret.ExecuteFunctionWithArgs`) under `oct test`.
- Language reference explicitly states interpreter-path execution and no compiled parity contract for octest helper behavior.

### 2.2 `[Benchmark]`

- Executed by `oct bench` only.
- Benchmarks are compiled-and-run per case via `build.CompileForTest(...)` + spawned binary (`exec.Command(result.ArtifactPath)`).

### 2.3 `[Artifact]`

- Executed by `oct artifact` only.
- After loading and type checking, artifacts use the shared typed interpreter with a compiler-owned staged-output capability. Backend generation and host compilation are not involved.

### 2.4 Command partitioning

- `oct test`: runs `[Fact]`, `[Theory]`, and `.octfail`.
- `oct bench`: runs `[Benchmark]` only.
- `oct artifact`: runs `[Artifact]` only.

### 2.5 Package loading path parity

- All three commands use `project.LoadForTest(path)` + `typecheck.CheckProgram(program)` before lane-specific filtering, so package loading/typecheck entry is shared.

---

## 3) Assertion model (current)

### 3.1 Implementation

- Assertions are implemented as special-cased `Assert.*` call handling in interpreter/typechecker.
- Documented helpers: `Assert.True`, `Assert.False`, `Assert.Equal`, `Assert.Near`, `Assert.Error`, `Assert.LGTM`.

### 3.2 Counting and zero-assert behavior

- Runner does not track assertion counts per test/theory case.
- Pass/fail is currently “function returned without runtime/assertion error” for facts/theories.
- Therefore `[Fact]` with zero assertions passes today if body completes successfully.
- `[Theory]` with zero assertions passes today if each row completes successfully.

### 3.3 Policy fit analysis

Target policy:
- `[Fact]/[Theory]` require at least one `Assert.*` or `SkipTest(...)` terminal skip.
- `[Benchmark]` no assertion requirement.
- `[Artifact]` no assertion requirement.

Architecture fit:
- Feasible with per-test execution context in interpreter runner hook (assert counter + skipped terminal state).
- Minimal blast radius if implemented in tester/interpret test lane only, leaving benchmark/artifact lanes unchanged.

---

## 4) Skip semantics (current + design)

### 4.1 Current state

- No Oct-level `SkipTest(...)` builtin found.
- No explicit skip outcome in oct test result model (`PASS`/`FAIL` only).
- Existing “skip” usages found are Go `t.Skip(...)` in Go tests, not octest runtime semantics.

### 4.2 Recommended M3 shape

`SkipTest(reason: String) -> Void`
- test-only builtin (available only in `.octest` execution lane).
- mandatory non-empty reason.
- immediate terminal outcome for current fact/theory row.
- reported as `SKIP` with reason in test output.
- counts as valid terminal outcome for assertion-required policy.
- rejected in `.oct` non-test contexts.

Implementation direction:
- Use sentinel runtime outcome (distinct from panic-like error) surfaced through tester result struct.

---

## 5) Timeout semantics (current + design)

### 5.1 Current state

- No per-test timeout in `tester.Execute` loop.
- No benchmark/artifact per-case timeout in lane executors.
- A hung interpreted fact/theory can block indefinitely.
- `go test ./...` has Go test framework timeout when invoked externally, but oct CLI command itself has no explicit per-case timeout policy.

### 5.2 Recommended rollout

- MVP (M4): default per-fact/per-theory-row timeout (e.g., 30s) enforced in `oct test` lane.
- Later optional attribute override (`[TimeoutSeconds(n)]`) only if attribute-argument surface is accepted cleanly by parser/reference.
- Keep benchmark lane timeout policy separate (benchmarks are measurement-oriented and compiled subprocess based).

---

## 6) Benchmark semantics (current)

- `[Benchmark]` exists and is parser/typechecker validated as `.octest`-only, `fn() -> Void`, no params, no mixed attributes.
- Executed by `oct bench`.
- Reported separately with `RUN/PASS/FAIL` and benchmark summary.
- Compiled mode is already used by benchmark execution path.
- Benchmarks can coexist with facts/theories/artifacts in same file; commands run only their own lane.
- Optional benchmark `.octagon` outputs are supported via `--octagon-out` and profile outputs via `--profile`.

Policy alignment: current architecture already strongly supports a distinct performance lane.

---

## 7) Artifact semantics (current)

### 7.1 What `[Artifact]` means today

- Marker for functions executed by `oct artifact` lane only.
- Must be in `.octest`, have no params, return `Void` or `Void ! Error`, and cannot combine with fact/theory/benchmark.
- Direct calls are rejected; discovery and invocation belong to the explicit artifact phase.

### 7.2 Output behavior

- `Artifact.Write*` declares output relative to `--output-root`, stages it, and publishes only after all selected entries succeed.
- Absolute, escaping, and duplicate paths are rejected; unchanged content is not rewritten.
- The global `WriteOctagon(...)` spelling delegates to the same capability for legacy consumers.

### 7.3 Failure behavior

- First artifact runtime failure causes command failure (`1 artifact(s) failed`).

### 7.4 Coexistence and run mode

- Artifacts coexist in mixed `.octest` files; `oct artifact` partitions by attribute.
- Artifact execution is a build-time phase backed by the typed interpreter.

### 7.5 Determinism/schema guidance

- Discovery and publication order are deterministic; ambient effects are rejected and seeded Oct randomness remains deterministic.
- `.octagon` emission/load contracts remain the structured-output convention; phase and path rules are normative in `31-octest.md`.

---

## 8) Mixed `.octest` behavior (current)

Current behavior is clean lane partitioning by command:
- `test` => facts/theories (+ `.octfail`)
- `bench` => benchmarks only
- `artifact` => artifacts only

Mixed-file suites are explicitly covered by command tests and align with policy direction.

---

## 9) Gaps and risks

1. No assertion counting permits vacuous passing tests.
2. No first-class skip semantics; no way to declare intentional non-run contract outcome.
3. No per-test timeout allows indefinite hangs.
4. Artifact command output contract is operational but under-documented (directory/layout/determinism/schema expectations).
5. Reference docs mention lane partitioning, but M1 policy details (assertion-required, skip terminal state, timeout defaults) are not yet specified.

Inconsistency surfaced per policy:
- Repository behavior already supports benchmark/artifact lanes and mixed-file partitioning, but Language/reference lacks explicit zero-assert/skip/timeout semantics (documentation gap, not code contradiction).

---

## 10) Proposed staged implementation plan

## M2 — Assertion counting + zero-assert failure

- Add per-test-case accounting in `oct test` lane for `Assert.*` invocations.
- Define pass condition for `[Fact]/[Theory]`: `assert_count > 0` OR `skipped=true` (future M3 hook).
- Fail with clear message for zero-assert completion.
- Add positive/negative coverage tests:
  - fact zero assert fails
  - theory row zero assert fails
  - existing assert helpers increment count.

## M3 — `SkipTest(reason)`

- Add test-only builtin: `SkipTest(reason: String) -> Void`.
- Enforce non-empty reason.
- Runtime emits terminal skip sentinel; tester reports `SKIP Package.Test: reason`.
- Skip treated as valid terminal outcome exempt from zero-assert failure.
- Add docs + parser/typechecker guardrails for test-only usage.

## M4 — Timeout protection

- Add default per-case timeout for `oct test` lane (fact and each theory row).
- Recommended default: 30s, configurable later via CLI flag.
- Failure message must identify timed-out case and elapsed threshold.
- Optional follow-up: `[TimeoutSeconds(n)]` attribute once attribute-argument policy is finalized.

## M5 — Benchmark/artifact lane policy hardening

- Codify mixed-file partitioning as normative in reference docs.
- Explicitly document execution mode defaults:
  - test/artifact interpreted
  - bench compiled.
- Clarify artifact success semantics (artifact function completed; explicit `WriteOctagon` failures fail run).
- Add `.octagon` schema/version guidance for artifact evidence files (convention + examples).

Rationale for ordering:
- M2/M3/M4 tighten correctness contracts in test lane first.
- M5 locks policy/documentation for non-correctness lanes without forcing runtime upheaval.

---

## 11) Required validation executed for M1 audit

- `go test ./...`
- `go run ./cmd/oct test Libraries/Random`
- `go run ./cmd/oct test Experiments/RandomApiBakeoff`
- `go run ./cmd/oct artifact testdata/m24h/valid`

(Results captured in this change task summary.)
