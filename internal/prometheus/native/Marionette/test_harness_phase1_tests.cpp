#include "test_harness.h"

FACT(MarionetteHarness_CanWriteArtifact)
{
    const bool wrote = context.WriteTextArtifact("phase1_smoke", "phase1 harness smoke artifact");
    ASSERT_TRUE(wrote, "artifact write should succeed");
    ASSERT_FALSE(context.ArtifactPaths().empty(), "artifact path should be recorded");
}

FACT(MarionetteHarness_BenchmarkContextSupportsValidationState)
{
    ::marionette::tests::BenchmarkContext benchmarkContext("benchmark_validation_smoke");
    benchmarkContext.RecordFailure(__FILE__, __LINE__, "ASSERT_TRUE", "benchmark failure smoke", "true", "false");
    ASSERT_TRUE(benchmarkContext.HasFailures(), "benchmark context should retain recorded failures");
    ASSERT_FALSE(benchmarkContext.IsSkipped(), "benchmark context should not begin skipped");

    const bool wrote = benchmarkContext.WriteTextArtifact("benchmark_validation_summary", "validated benchmark artifact");
    ASSERT_TRUE(wrote, "benchmark context should support artifact writes");
    ASSERT_FALSE(benchmarkContext.ArtifactPaths().empty(), "benchmark artifact path should be recorded");

    benchmarkContext.SkipTest(__FILE__, __LINE__, "benchmark skip smoke");
    ASSERT_TRUE(benchmarkContext.IsSkipped(), "benchmark context should support skip state");
    ASSERT_TRUE(benchmarkContext.SkipState() != nullptr, "benchmark skip state should be queryable");
}

FACT(MarionetteHarness_CanWriteRootArtifactFiles)
{
    ::marionette::tests::BenchmarkContext benchmarkContext("benchmark_root_artifact_smoke");
    const std::filesystem::path jsonPath("phase1-root/benchmark_validation_summary.json");
    const bool wrote = benchmarkContext.WriteArtifactFile(jsonPath, "{\"schema\":\"marionette.phase1\"}\n");

    ASSERT_TRUE(wrote, "benchmark context should support explicit artifact file paths");
    ASSERT_FALSE(benchmarkContext.ArtifactPaths().empty(), "explicit artifact file path should be recorded");
    ASSERT_TRUE(std::filesystem::exists(benchmarkContext.ArtifactPaths().front()), "explicit artifact file should be created on disk");
}

FACT(MarionetteHarness_StandardBenchmarksRemainFilterable)
{
    const std::vector<::marionette::tests::BenchmarkResult> standardResults =
        ::marionette::tests::ExecuteBenchmarks("SmokeBenchmarkCountsIterations");
    ASSERT_EQUAL(static_cast<std::size_t>(1u), standardResults.size(), "standard benchmark should remain discoverable");
    ASSERT_EQUAL(static_cast<std::uint64_t>(8u), standardResults.front().configuredIterations, "configured iteration count should round-trip");
    ASSERT_EQUAL(static_cast<std::uint64_t>(8u), standardResults.front().iterations, "standard benchmark should execute every iteration");
    ASSERT_FALSE(standardResults.front().failed, "standard benchmark should not fail");
    ASSERT_FALSE(standardResults.front().skipped, "standard benchmark should not skip");

    const std::vector<::marionette::tests::BenchmarkResult> validatedResults =
        ::marionette::tests::ExecuteBenchmarks(
            "SmokeBenchmarkCountsIterations",
            ::marionette::tests::MARIONETTE_BENCHMARK_CATEGORY_VALIDATED);
    ASSERT_TRUE(validatedResults.empty(), "standard benchmark should not leak into validated benchmark selection");
}
