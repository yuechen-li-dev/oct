#pragma once

#include <cstddef>
#include <cstdint>
#include <filesystem>
#include <iterator>
#include <string>
#include <string_view>
#include <vector>

namespace marionette::tests
{
    struct Failure
    {
        std::string testName;
        std::string file;
        int line = 0;
        std::string assertion;
        std::string message;
        std::string expected;
        std::string actual;
    };

    struct Skip
    {
        std::string testName;
        std::string file;
        int line = 0;
        std::string reason;
    };

    class TestContext
    {
    public:
        explicit TestContext(std::string_view testName);

        [[nodiscard]] std::string_view TestName() const;
        [[nodiscard]] std::string DisplayName() const;
        [[nodiscard]] const std::vector<Failure>& Failures() const;
        [[nodiscard]] const std::vector<std::filesystem::path>& ArtifactPaths() const;
        [[nodiscard]] const Skip* SkipState() const;
        [[nodiscard]] bool HasFailures() const;
        [[nodiscard]] bool IsSkipped() const;
        [[nodiscard]] std::filesystem::path ArtifactDirectory() const;

        void RecordFailure(
            const char* file,
            int line,
            std::string_view assertion,
            std::string_view message,
            std::string_view expected = {},
            std::string_view actual = {});
        void SkipTest(const char* file, int line, std::string_view reason);
        void EnterTheoryCase(std::string_view caseName);
        void LeaveTheoryCase();
        [[nodiscard]] bool WriteTextArtifact(std::string_view artifactName, std::string_view content);

        template <typename CaseCollection, typename CaseFunction>
        void RunTheoryCases(const CaseCollection& cases, CaseFunction caseFunction)
        {
            for (const auto& testCase : cases) {
                EnterTheoryCase(testCase.name);
                caseFunction(*this, testCase);
                LeaveTheoryCase();

                if (IsSkipped()) {
                    return;
                }
            }
        }

    private:
        std::string testName_;
        std::string theoryCaseName_;
        std::vector<Failure> failures_;
        std::vector<std::filesystem::path> artifactPaths_;
        Skip skip_;
        bool skipped_ = false;
    };

    using TestFunction = void (*)(TestContext& context);

    struct TestCase
    {
        std::string name;
        TestFunction function = nullptr;
    };

    class TestRegistrar
    {
    public:
        TestRegistrar(const char* testName, TestFunction function);
    };

    void AssertDoomByName(
        TestContext& context,
        std::string_view doomCaseName,
        const char* file,
        int line);

    struct BenchmarkContext
    {
        std::uint64_t iteration = 0;
    };

    using BenchmarkFunction = void (*)(BenchmarkContext& context);

    struct BenchmarkCase
    {
        std::string name;
        BenchmarkFunction function = nullptr;
        std::uint64_t iterations = 0;
    };

    class BenchmarkRegistrar
    {
    public:
        BenchmarkRegistrar(const char* benchmarkName, BenchmarkFunction function, std::uint64_t iterations);
    };

    struct BenchmarkResult
    {
        std::string name;
        std::uint64_t iterations = 0;
        std::uint64_t elapsedNanoseconds = 0;
    };

    [[nodiscard]] std::vector<TestCase>& Registry();
    [[nodiscard]] std::vector<BenchmarkCase>& BenchmarkRegistry();
    [[nodiscard]] int RunAllTests(std::string_view filter);
    [[nodiscard]] int RunBenchmarks(std::string_view filter);
    [[nodiscard]] std::vector<BenchmarkResult> ExecuteBenchmarks(std::string_view filter);

    [[nodiscard]] std::string FormatValue(bool value);
    [[nodiscard]] std::string FormatValue(double value);
    [[nodiscard]] std::string FormatValue(float value);
    [[nodiscard]] std::string FormatValue(int value);
    [[nodiscard]] std::string FormatValue(std::size_t value);
    [[nodiscard]] std::string FormatValue(const char* value);
    [[nodiscard]] std::string FormatValue(const std::string& value);
    [[nodiscard]] std::string FormatValue(std::string_view value);
    [[nodiscard]] std::string FormatValue(unsigned int value);

    template <typename NumericType>
    void AssertNear(
        TestContext& context,
        NumericType expected,
        NumericType actual,
        NumericType tolerance,
        const char* file,
        int line,
        std::string_view message)
    {
        const NumericType difference = expected >= actual ? expected - actual : actual - expected;
        if (difference <= tolerance) {
            return;
        }

        context.RecordFailure(
            file,
            line,
            "ASSERT_NEAR",
            std::string(message) + ", tolerance=" + FormatValue(tolerance) + ", difference=" + FormatValue(difference),
            FormatValue(expected),
            FormatValue(actual));
    }

    template <typename ExpectedRange, typename ActualRange>
    void AssertSequenceEqual(
        TestContext& context,
        const ExpectedRange& expected,
        const ActualRange& actual,
        const char* file,
        int line,
        std::string_view message)
    {
        auto expectedIterator = std::begin(expected);
        auto actualIterator = std::begin(actual);
        std::size_t index = 0;

        while (expectedIterator != std::end(expected) && actualIterator != std::end(actual)) {
            if (!(*expectedIterator == *actualIterator)) {
                context.RecordFailure(
                    file,
                    line,
                    "ASSERT_SEQUENCE_EQUAL",
                    std::string(message) + " (mismatch at index " + std::to_string(index) + ")",
                    FormatValue(*expectedIterator),
                    FormatValue(*actualIterator));
                return;
            }

            ++expectedIterator;
            ++actualIterator;
            ++index;
        }

        if (expectedIterator != std::end(expected) || actualIterator != std::end(actual)) {
            const std::size_t expectedSize = static_cast<std::size_t>(std::distance(std::begin(expected), std::end(expected)));
            const std::size_t actualSize = static_cast<std::size_t>(std::distance(std::begin(actual), std::end(actual)));

            context.RecordFailure(
                file,
                line,
                "ASSERT_SEQUENCE_EQUAL",
                std::string(message) + " (sequence length mismatch)",
                "size=" + std::to_string(expectedSize),
                "size=" + std::to_string(actualSize));
        }
    }
}

#define FACT(TEST_NAME) \
    static void TEST_NAME(::marionette::tests::TestContext& context); \
    static const ::marionette::tests::TestRegistrar TEST_NAME##_registrar(#TEST_NAME, &TEST_NAME); \
    static void TEST_NAME(::marionette::tests::TestContext& context)

#define THEORY(TEST_NAME) FACT(TEST_NAME)

#define BENCHMARK(BENCHMARK_NAME) \
    static void BENCHMARK_NAME(::marionette::tests::BenchmarkContext& context); \
    static const ::marionette::tests::BenchmarkRegistrar BENCHMARK_NAME##_benchmark_registrar(#BENCHMARK_NAME, &BENCHMARK_NAME, 10000); \
    static void BENCHMARK_NAME(::marionette::tests::BenchmarkContext& context)

#define BENCHMARK_WITH_ITERATIONS(BENCHMARK_NAME, ITERATIONS) \
    static void BENCHMARK_NAME(::marionette::tests::BenchmarkContext& context); \
    static const ::marionette::tests::BenchmarkRegistrar BENCHMARK_NAME##_benchmark_registrar(#BENCHMARK_NAME, &BENCHMARK_NAME, ITERATIONS); \
    static void BENCHMARK_NAME(::marionette::tests::BenchmarkContext& context)

#define ASSERT_TRUE(CONDITION, MESSAGE) \
    do { \
        if (!(CONDITION)) { \
            context.RecordFailure(__FILE__, __LINE__, "ASSERT_TRUE", MESSAGE, "true", "false"); \
        } \
    } while (false)

#define ASSERT_FALSE(CONDITION, MESSAGE) \
    do { \
        if (CONDITION) { \
            context.RecordFailure(__FILE__, __LINE__, "ASSERT_FALSE", MESSAGE, "false", "true"); \
        } \
    } while (false)

#define ASSERT_EQUAL(EXPECTED, ACTUAL, MESSAGE) \
    do { \
        const auto expectedValue = (EXPECTED); \
        const auto actualValue = (ACTUAL); \
        if (!(expectedValue == actualValue)) { \
            context.RecordFailure( \
                __FILE__, \
                __LINE__, \
                "ASSERT_EQUAL", \
                MESSAGE, \
                ::marionette::tests::FormatValue(expectedValue), \
                ::marionette::tests::FormatValue(actualValue)); \
        } \
    } while (false)

#define ASSERT_NOT_EQUAL(EXPECTED, ACTUAL, MESSAGE) \
    do { \
        const auto expectedValue = (EXPECTED); \
        const auto actualValue = (ACTUAL); \
        if (expectedValue == actualValue) { \
            context.RecordFailure( \
                __FILE__, \
                __LINE__, \
                "ASSERT_NOT_EQUAL", \
                MESSAGE, \
                ::marionette::tests::FormatValue(expectedValue), \
                ::marionette::tests::FormatValue(actualValue)); \
        } \
    } while (false)

#define ASSERT_NEAR(EXPECTED, ACTUAL, TOLERANCE, MESSAGE) \
    do { \
        const auto expectedValue = (EXPECTED); \
        const auto actualValue = (ACTUAL); \
        const auto toleranceValue = (TOLERANCE); \
        ::marionette::tests::AssertNear(context, expectedValue, actualValue, toleranceValue, __FILE__, __LINE__, MESSAGE); \
    } while (false)

#define FAIL(MESSAGE) \
    do { \
        context.RecordFailure(__FILE__, __LINE__, "FAIL", MESSAGE); \
    } while (false)

#define SKIP(MESSAGE) \
    do { \
        context.SkipTest(__FILE__, __LINE__, MESSAGE); \
        return; \
    } while (false)

#define ASSERT_SEQUENCE_EQUAL(EXPECTED, ACTUAL, MESSAGE) \
    do { \
        ::marionette::tests::AssertSequenceEqual(context, (EXPECTED), (ACTUAL), __FILE__, __LINE__, MESSAGE); \
    } while (false)

#define ASSERT_DOOM(DOOM_CASE_NAME) \
    do { \
        ::marionette::tests::AssertDoomByName(context, #DOOM_CASE_NAME, __FILE__, __LINE__); \
    } while (false)
