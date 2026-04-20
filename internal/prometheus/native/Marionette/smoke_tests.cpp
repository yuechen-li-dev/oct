#include "test_harness.h"

#include <array>
#include <filesystem>
#include <fstream>
#include <string>
#include <string_view>
#include <vector>

FACT(SmokeFactPasses)
{
    ASSERT_TRUE(true, "basic true assertion should pass");
}

FACT(SmokeFactSupportsRichAssertions)
{
    const bool isStable = true;
    const int expectedCount = 3;
    const int actualCount = 3;
    const std::string left = "host";
    const std::string right = "candidate";

    ASSERT_TRUE(isStable, "true assertions can carry an explicit message");
    ASSERT_FALSE(false, "false assertions can carry an explicit message");
    ASSERT_EQUAL(expectedCount, actualCount, "equal assertions show expected and actual values when they diverge");
    ASSERT_NOT_EQUAL(left, right, "not-equal assertions confirm distinct values");
}

FACT(SmokeFactCanBeFailedDeliberately)
{
    const bool enableIntentionalFailure = false;

    if (enableIntentionalFailure) {
        FAIL("flip enableIntentionalFailure to true when you want to inspect failure output manually");
    }

    ASSERT_TRUE(true, "default smoke run stays green");
}

THEORY(SmokeTheorySupportsNamedCases)
{
    struct AdditionCase
    {
        std::string_view name;
        int left = 0;
        int right = 0;
        int expected = 0;
    };

    const std::array<AdditionCase, 3> cases{{
        {"zeros", 0, 0, 0},
        {"small-positive", 2, 3, 5},
        {"mixed-sign", 5, -2, 3}
    }};

    context.RunTheoryCases(
        cases,
        [](auto& context, const AdditionCase& testCase)
        {
            ASSERT_EQUAL(
                testCase.expected,
                testCase.left + testCase.right,
                "theory cases should reuse the same assertion logic across multiple named rows");
        });
}

FACT(SmokeFactCanBeSkipped)
{
    SKIP("example skipped tests stay visible without failing the default run");
}

FACT(SmokeFactSupportsSequenceAssertions)
{
    const std::vector<int> expected{1, 2, 3, 5, 8};
    const std::array<int, 5> actual{{1, 2, 3, 5, 8}};

    ASSERT_SEQUENCE_EQUAL(expected, actual, "sequence equality should compare size and element order");
}

FACT(SmokeFactWritesDeterministicArtifacts)
{
    const std::string artifactContents =
        "{\n"
        "  \"status\": \"ok\",\n"
        "  \"test\": \"SmokeFactWritesDeterministicArtifacts\"\n"
        "}\n";

    ASSERT_TRUE(
        context.WriteTextArtifact("summary", artifactContents),
        "artifact helper should write a stable text file for inspection");

    const std::filesystem::path artifactPath = context.ArtifactDirectory() / "summary.txt";
    ASSERT_TRUE(std::filesystem::exists(artifactPath), "artifact file should exist at the deterministic path");

    std::ifstream artifactFile(artifactPath, std::ios::binary);
    ASSERT_TRUE(artifactFile.is_open(), "artifact file should be readable after it is written");

    const std::string actualContents{
        std::istreambuf_iterator<char>(artifactFile),
        std::istreambuf_iterator<char>{}};
    ASSERT_EQUAL(artifactContents, actualContents, "artifact contents should be deterministic and overwritten in place");
}
