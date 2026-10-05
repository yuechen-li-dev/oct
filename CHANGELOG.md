# Changelog

## Unreleased

- Productize `oct-mcp` 0.1.0 with bounded source-only tools, stdio and streamable HTTP transports, structured results, temporary workspaces, artifact IDs, security/deployment documentation, and the local Codex plugin package.
- Dogfood and simplify the agent workflow: local Codex is now skills-first around `oct test --json` and `oct artifact --json`; hosted MCP exposes `oct_workspace_info`, `oct_test`, `oct_artifact`, playground-only `oct_run`, and scoped `oct_get_artifact`. The CLI now reports stable structured test/artifact results, explicit test fallback counts, and interpreted artifact metadata.
- **Breaking:** `Random@0.2.0` replaces the generator-state API with counter-based streams. A draw is a pure function of `(stream, index, parameters)`: `Random.Seeded`, `Fork`, `Child`, `Unit`, `Between`, `IntBetween`, `Normal`, and the helpers `Chance`, `Exponential`, `Units`, `Normals`, `Spike`, coins and dice. `Rng`, `RngSeed`, `Rand*`, `Gaussian`, every `*Result` record and the `Next`/`Value` threading idiom are removed with no deprecated wrappers. The bit generator is Philox4x32-10, implemented once in Go for both execution lanes, so seeded sequences changed and recorded outputs must be regenerated. Specification and migration table: `internal/random/Random.md`.
- Add `Entropy` (`Seed`, `IntBetween`, `Unit`, `Bytes`), a compiler-owned namespace over the operating system's random source that needs no import. It replaces `Random.CryptoRand*`; the `Crypto*` coin and dice helpers are removed. Artifact evaluation and capability discovery reject it.
- Regenerate the recorded outputs of `Experiments/PrometheusMeasurementFilteringLab` M1–M4 and `Experiments/FmBrownNoiseKalman` M3–M6 under `Random@0.2.0`. Each experiment's report states what moved.
- Fix compiled execution of `if` expressions, which evaluated both branches before selecting one.
- Rewrite `oct fmt` layout to work from the lexer's tokens. It now only sets indentation and spacing, never moves or changes a token, copies Oct-XML verbatim, and verifies that its output has the same tokens on the same lines before writing. Directory runs attempt every file and report every refusal.
- `oct fmt` keeps each arrow as written. `--arrows thin` and `--arrows fat` write one spelling throughout; before, every `=>` became `->`.
- `.octfail` gains `expect runtime error: "..."`: a program that must compile and then fail when its `Main` runs, checked in both lanes. An `.octfail` may also import a library of its repository, and a failure of the Go toolchain on generated code no longer satisfies a contract.
- **Breaking:** `Assert.Equal`, `Assert.Near`, `Assert.True` and `Assert.False` reject an unhandled fallible operand; write `F()!` or use `Assert.LGTM`. The interpreted lane used to unwrap it silently.
- **Breaking:** a `[Fact]` with no assertion fails in the compiled lane, as it already did interpreted.
- Fix compiled execution of a fallible `match` whose arm contains an `if` (the `if` was skipped) or discards its binding with `_` (did not build), and of element-wise arithmetic on two arrays (did not build).
- Import resolution continues to the nearest ancestor with `Libraries/`; a nested `Packages/` directory adds packages and no longer hides the repository's libraries.
- Add `TestLanguageCorpusRunsInBothLanes` (integration lane): every `Language` fixture directory runs in both execution lanes. Ten directories that no longer loaded are repaired.
- Reference: fallible `match` is a statement with block arms. The expression form shown before was never implemented.
- Add `[Interpreted("reason")]` and `[Compiled("reason")]` for a `[Fact]` or `[Theory]` that belongs to one execution lane. The reason is required; the other lane reports the test as skipped, and a `[Compiled]` test does not fall back to the interpreter under `--execution auto`.
- `.octfail` gains `expect artifact error: "..."` for `[Artifact]` entry points that must fail and publish nothing, and may state several expectation lines that one failure must all contain. The expected failures under `Language/` that Go tests used to drive are `.octfail` contracts.
- Fix compiled execution of utility `when`. An enum-targeted `when utility` with payload candidates now compiles. Every utility `when` used to evaluate the value and score of each case and the `else` value before selecting, so a case whose condition was false could fail the program; both lanes now evaluate in the order the reference gives.
- **Breaking:** the `manifest.oct` of an imported package is an error when it exists and does not parse or validate. It used to be ignored when the importing program's root required no manifests, which silently removed the package's wrapper declarations.
- A value named `matrix` can be indexed; only `matrix[[...]]` is a literal.
- `Libraries/IfErrNotEqualNil` parses and has tests: it is the identity template.
- The compiled lane reports that an `Artifact.*` builtin is available only during `oct artifact` evaluation, in place of "does not yet support builtin".
- Test sidecars are built once into a cache keyed by their sources (`internal/sidecarcache`, `OCT_SIDECAR_CACHE_DIR`) and reused across test runs. `TestLanguageCorpusRunsInBothLanes` runs the wrapper fixture directories with them.
- **Breaking:** every utility `when` evaluates one value, the value of the arm it selects. The plain standalone form and `when policy` used to evaluate the value of every case whose condition held.
- **Breaking:** `hysteresis` and `min_commit` on a standalone `when utility` are a parse error. They had no effect there; they belong to `when policy` in a flow state.
- **Breaking:** `when policy` commits to an arm, not to the value the arm produced. An arm whose value changes keeps its commitment, two arms with equal values are different commitments, and `else` is never held. Flow checkpoints record the arm (interpreter checkpoint version 4, compiled payload version 2); earlier checkpoints are refused. The Verilog profile's site register is `UtilitySite<N>Arm`.
- **Breaking:** `hysteresis` measures the leading arm against the committed arm's score at this evaluation. It used to measure against the score recorded when the arm was committed, so a committed arm whose score had since fallen was held against rivals that now beat it. The recorded score is removed from flow checkpoints and from the Verilog profile's ports.
- A value named `vector` can be indexed. `vector[...]` indexes a parameter, local, loop variable, match binding or capture named `vector` where one is in scope, and is a literal everywhere else.
- Fix compiled execution of a `when policy` inside a larger expression (did not build), and add a Verilog fixture for a standalone `when utility` in a flow state.

## v0.1.0 — initial preview

Oct 0.1 is an early preview of a scientific programming language and toolchain for reproducible research, portable computation, and AI-assisted experimentation.

Highlights:

- scientific language core with source contracts under `Language/`;
- interpreted and compiled execution paths;
- SI units and scientific numeric/library surfaces;
- xUnit-style tests, artifacts, and benchmarks;
- Octomata flow/state machines;
- tensors and Einstein notation support in the language surface;
- package manager MVP with local/Git source sync and transitive exact dependency graph sync;
- source-controlled canonical first-party registry at `Registry/registry.oct`;
- canonical `Mathematics@0.1.0` package name, with no `Math` alias;
- optional project-root `lock.octagon` for locked sync;
- Octxiliary wrapper sidecars declared by manifests;
- explicit native sidecar builds through `oct pkg build-wrappers --allow-native`;
- Go-based native binary path for compiled programs.

Pre-1.0 notes:

- language syntax and semantics may change before 1.0;
- public Go APIs, including sidecar helper APIs, may evolve;
- package registry format and lockfile contents may evolve;
- the standard library/package APIs are not stable;
- hosted registry, publishing, auth, signing, `.octpkg` artifacts, semver ranges, `latest`, and package solver behavior are not part of v0.1;
- performance is not final and should not be treated as a release guarantee.
