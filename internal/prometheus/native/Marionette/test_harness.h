#pragma once

#include <cstddef>
#include <cstdint>
#include <filesystem>
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

        void RecordFailure(
            const char* file,
            int line,
            std::string_view assertion,
            std::string_view message,
            std::string_view expected = {},
            std::string_view actual = {});
        void SkipTest(const char* file, int line, std::string_view reason);
        [[nodiscard]] bool WriteTextArtifact(std::string_view artifactName, std::string_view content);

    private:
        std::string testName_;
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

    struct BenchmarkContext
    {
        std::uint64_t iteration = 0;
    };

    [[nodiscard]] std::vector<TestCase>& Registry();
    [[nodiscard]] int RunAllTests(std::string_view filter);
    [[nodiscard]] int RunBenchmarks(std::string_view filter);

    [[nodiscard]] std::string FormatValue(bool value);
    [[nodiscard]] std::string FormatValue(int value);
    [[nodiscard]] std::string FormatValue(std::uint32_t value);
    [[nodiscard]] std::string FormatValue(std::size_t value);
    [[nodiscard]] std::string FormatValue(const char* value);
    [[nodiscard]] std::string FormatValue(const std::string& value);
    [[nodiscard]] std::string FormatValue(std::string_view value);
}

#define FACT(TEST_NAME) \
    static void TEST_NAME(::marionette::tests::TestContext& context); \
    static const ::marionette::tests::TestRegistrar TEST_NAME##_registrar(#TEST_NAME, &TEST_NAME); \
    static void TEST_NAME(::marionette::tests::TestContext& context)

#define THEORY(TEST_NAME) FACT(TEST_NAME)

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

#define FAIL(MESSAGE) \
    do { \
        context.RecordFailure(__FILE__, __LINE__, "FAIL", MESSAGE); \
    } while (false)

#define SKIP(MESSAGE) \
    do { \
        context.SkipTest(__FILE__, __LINE__, MESSAGE); \
        return; \
    } while (false)
