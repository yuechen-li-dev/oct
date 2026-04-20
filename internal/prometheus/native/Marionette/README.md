# Marionette harness quick reference

Marionette is a powerful yet ultra-lightweight test harness for C++. 

## Building

Marionette has no external dependencies. Compile all `.cpp` files in `Marionette/` together into a single binary with any C++20 compiler. The only required define is `MARIONETTE_TEST_REPO_ROOT`, which tells the harness where to write artifact output.

**g++ / clang++**

```sh
g++ -std=c++20 -O2 \
  -DMARIONETTE_TEST_REPO_ROOT="\"$(pwd)\"" \
  Marionette/test_main.cpp \
  Marionette/test_harness.cpp \
  Marionette/test_doom.cpp \
  Marionette/smoke_tests.cpp \
  Marionette/test_harness_phase1_tests.cpp \
  Marionette/test_harness_doom_tests.cpp \
  -o marionette_tests
```

Run from the repository root so `$(pwd)` resolves correctly and artifacts land under `out/test-artifacts/`.

When adding your own tests, append your `.cpp` files to the same compile command — no registration step required beyond the `FACT`/`THEORY`/`BENCHMARK` macros in source.

**Run modes**

```sh
./marionette_tests                  # run all tests
./marionette_tests <filter>         # run tests whose name contains <filter>
./marionette_tests --bench          # run all benchmarks
./marionette_tests --bench <filter> # run matching benchmarks
```

Artifacts from passing and failing tests are written to `out/test-artifacts/<TestName>/`.

## Core test declaration

### `FACT`

```cpp
FACT(MyTestName)
{
    ASSERT_TRUE(true, "example");
}
```

Registers one test case.

### `THEORY` + `RunTheoryCases`

`THEORY` is currently an alias for `FACT`, and `RunTheoryCases` lives on `TestContext`.

```cpp
struct CaseData { const char* name; int value; };

THEORY(ExampleTheory)
{
    const std::vector<CaseData> cases = {
        {"zero", 0},
        {"one", 1}
    };

    context.RunTheoryCases(cases, [](::marionette::tests::TestContext& theoryContext, const CaseData& testCase) {
        ASSERT_TRUE(testCase.value >= 0, "case value should be non-negative");
    });
}
```

## Assertions

### `ASSERT_TRUE` / `ASSERT_FALSE`

```cpp
ASSERT_TRUE(condition, "must be true");
ASSERT_FALSE(condition, "must be false");
```

### `ASSERT_EQUAL` / `ASSERT_NOT_EQUAL`

```cpp
ASSERT_EQUAL(expected, actual, "values should match");
ASSERT_NOT_EQUAL(expected, actual, "values should differ");
```

### `ASSERT_SEQUENCE_EQUAL`

```cpp
ASSERT_SEQUENCE_EQUAL(expectedVector, actualVector, "ordered sequence should match");
```

Element equality uses `operator==` on each element pair at the same index.

- custom structs in expected/actual sequences must define `operator==`
- this is why many runtime structs used in tests for other projects define equality directly

### `ASSERT_NEAR`

```cpp
ASSERT_NEAR(10.0f, measured, 0.1f, "difference should stay in tolerance");
```

Current template constraint: expected, actual, and tolerance must resolve to the same numeric type.

- mixed-type calls (for example `float` value with `double` literal tolerance) can fail to compile
- use explicit literal suffixes where needed (for example `10.0f` and `0.1f` for float comparisons)

### `FAIL`

```cpp
if (fatalCondition) {
    FAIL("fatal condition encountered");
}
```

### `SKIP`

```cpp
if (!preconditionAvailable) {
    SKIP("precondition unavailable in this environment");
}
```

## Artifacts

Write test diagnostics through the context:

```cpp
ASSERT_TRUE(context.WriteTextArtifact("trace_summary", "...details..."), "artifact should be written");
```

Artifacts are materialized under repository `out/test-artifacts/...` via the harness.

## Benchmarks (separate from correctness tests)

Benchmarks register in a separate benchmark registry and are executed via bench mode.

### `BENCHMARK`

```cpp
BENCHMARK(MyBenchmark)
{
    volatile std::uint64_t value = context.iteration;
    (void)value;
}
```

`BENCHMARK(...)` currently defaults to **10000 iterations** per benchmark case.

### `BENCHMARK_WITH_ITERATIONS`

```cpp
BENCHMARK_WITH_ITERATIONS(MyBenchmarkFixed, 128)
{
    volatile std::uint64_t value = context.iteration + 1;
    (void)value;
}
```

Use when you need explicit fixed iteration count.

### `ExecuteBenchmarks` vs `RunBenchmarks`

- `ExecuteBenchmarks(filter)` runs matching benchmark functions and returns a structured `std::vector<BenchmarkResult>` (`name`, `iterations`, `elapsedNanoseconds`) for assertions, custom formatting, or artifact emission.
- `RunBenchmarks(filter)` is the CLI-oriented wrapper: it calls `ExecuteBenchmarks`, prints `[BENCH] ...` summary lines, and returns process exit code `0`.

Use `ExecuteBenchmarks` from tests/library code when you need programmatic inspection; use `RunBenchmarks` for `main()` bench mode (`--bench`).

> Guidance: benchmarks measure performance behavior; they are not substitutes for `FACT`/`THEORY` correctness checks.

## Doom module (quarantined)

Use this only for intentional subprocess-abnormal-termination envelope tests.

### `DOOM_FACT`

```cpp
DOOM_FACT(MyDoomCase)
{
    FORETELL_DOOM("intentional abort path");
    std::abort();
}
```

### `FORETELL_DOOM`

Attaches expected catastrophe context for envelope diagnostics.

### `ASSERT_DOOM`

```cpp
FACT(DoomEnvelopeRecovered)
{
    ASSERT_DOOM(MyDoomCase);
}
```

Asserts the doom subprocess terminated abnormally and produced expected diagnostic envelope artifacts.

### Operational wiring required in a real test binary

Doom tests need explicit process wiring (as in `tests/Marionette/test_main.cpp`):

1. **Set executable path early** so the parent can spawn itself:

```cpp
if (argc >= 1 && argv[0] != nullptr) {
    ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
}
```

2. **Handle doom child mode in `main()`**:

```cpp
if (mode == "--doom-case") {
    // argv[2] = doom case name
    // argv[3] must be "--doom-artifact-dir"
    // argv[4] = artifact directory path
    return ::marionette::tests::RunDoomCaseInChild(doomCaseName, artifactDirectory);
}
```

3. The parent-side doom launcher (`RunDoomCaseSubprocess`) invokes the executable with:
   - `--doom-case <CaseName>`
   - `--doom-artifact-dir <ArtifactDirectory>`

Without this `main()` contract plus `SetMarionetteExecutablePath(...)`, doom subprocess assertions cannot launch correctly.

> Guidance: doom tests are intentionally quarantined behavior. Do not use doom patterns casually in greenfield runtime or ordinary unit tests.

## Anti-patterns

- Do **not** use benchmarks as pass/fail correctness tests.
- Do **not** use doom macros for ordinary negative tests.
- Do **not** bypass harness assertions with silent logging-only checks.
- Do **not** skip writing artifacts when debugging replay/trace mismatches; capture bounded evidence.
