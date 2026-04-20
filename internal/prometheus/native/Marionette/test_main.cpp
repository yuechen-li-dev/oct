#include "test_harness.h"
#include "test_doom.h"

#include <filesystem>
#include <fstream>
#include <iostream>

namespace marionette::tests
{
    TestContext::TestContext(std::string_view testName)
        : testName_(testName)
    {
    }

    [[nodiscard]] std::string_view TestContext::TestName() const { return testName_; }

    [[nodiscard]] std::string TestContext::DisplayName() const { return testName_; }

    [[nodiscard]] const std::vector<Failure>& TestContext::Failures() const { return failures_; }

    [[nodiscard]] const std::vector<std::filesystem::path>& TestContext::ArtifactPaths() const { return artifactPaths_; }

    [[nodiscard]] const Skip* TestContext::SkipState() const
    {
        if (!skipped_) {
            return nullptr;
        }
        return &skip_;
    }

    [[nodiscard]] bool TestContext::HasFailures() const { return !failures_.empty(); }

    [[nodiscard]] bool TestContext::IsSkipped() const { return skipped_; }

    void TestContext::RecordFailure(
        const char* file,
        int line,
        std::string_view assertion,
        std::string_view message,
        std::string_view expected,
        std::string_view actual)
    {
        failures_.push_back(Failure{
            .testName = std::string(testName_),
            .file = file,
            .line = line,
            .assertion = std::string(assertion),
            .message = std::string(message),
            .expected = std::string(expected),
            .actual = std::string(actual),
        });
    }

    void TestContext::SkipTest(const char* file, int line, std::string_view reason)
    {
        skipped_ = true;
        skip_ = Skip{
            .testName = std::string(testName_),
            .file = file,
            .line = line,
            .reason = std::string(reason),
        };
    }

    [[nodiscard]] bool TestContext::WriteTextArtifact(std::string_view artifactName, std::string_view content)
    {
        std::filesystem::path path = std::filesystem::path("out") / "test-artifacts" / (std::string(testName_) + "_" + std::string(artifactName) + ".txt");
        std::error_code error;
        std::filesystem::create_directories(path.parent_path(), error);
        if (error) {
            RecordFailure(__FILE__, __LINE__, "WRITE_TEXT_ARTIFACT", "failed to create artifact directory", path.parent_path().string(), error.message());
            return false;
        }

        std::ofstream stream(path, std::ios::binary | std::ios::trunc);
        if (!stream.is_open()) {
            RecordFailure(__FILE__, __LINE__, "WRITE_TEXT_ARTIFACT", "failed to create artifact file", path.string(), "open failed");
            return false;
        }

        stream << content;
        stream.close();
        artifactPaths_.push_back(path);
        return static_cast<bool>(stream) || !content.empty();
    }

    TestRegistrar::TestRegistrar(const char* testName, TestFunction function)
    {
        Registry().push_back(TestCase{.name = testName, .function = function});
    }

    [[nodiscard]] std::vector<TestCase>& Registry()
    {
        static std::vector<TestCase> registry;
        return registry;
    }

    [[nodiscard]] int RunAllTests(std::string_view filter)
    {
        int failed = 0;
        int skipped = 0;
        int ran = 0;

        for (const TestCase& test : Registry()) {
            if (!filter.empty() && test.name.find(filter) == std::string::npos) {
                continue;
            }
            ++ran;
            TestContext context(test.name);
            test.function(context);

            if (context.IsSkipped()) {
                ++skipped;
                std::cout << "[SKIP] " << test.name << "\n";
                continue;
            }

            if (context.HasFailures()) {
                ++failed;
                std::cout << "[FAIL] " << test.name << "\n";
                for (const Failure& failure : context.Failures()) {
                    std::cout << "  " << failure.assertion << " at " << failure.file << ":" << failure.line << " :: " << failure.message;
                    if (!failure.expected.empty() || !failure.actual.empty()) {
                        std::cout << " (expected=" << failure.expected << ", actual=" << failure.actual << ")";
                    }
                    std::cout << "\n";
                }
            } else {
                std::cout << "[PASS] " << test.name << "\n";
            }
        }

        std::cout << "\nSummary: ran=" << ran << " failed=" << failed << " skipped=" << skipped << "\n";
        return failed == 0 ? 0 : 1;
    }

    [[nodiscard]] int RunBenchmarks(std::string_view)
    {
        std::cout << "Benchmarks are not configured in this stub harness build.\n";
        return 0;
    }

    [[nodiscard]] std::string FormatValue(bool value) { return value ? "true" : "false"; }
    [[nodiscard]] std::string FormatValue(int value) { return std::to_string(value); }
    [[nodiscard]] std::string FormatValue(std::uint32_t value) { return std::to_string(value); }
    [[nodiscard]] std::string FormatValue(std::size_t value) { return std::to_string(value); }
    [[nodiscard]] std::string FormatValue(const char* value) { return value == nullptr ? "null" : std::string(value); }
    [[nodiscard]] std::string FormatValue(const std::string& value) { return value; }
    [[nodiscard]] std::string FormatValue(std::string_view value) { return std::string(value); }
}

int main(int argc, char** argv)
{
    if (argc >= 1 && argv[0] != nullptr) {
        ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
    }

    std::string_view filter;
    if (argc >= 2 && argv[1] != nullptr) {
        filter = argv[1];
    }

    return ::marionette::tests::RunAllTests(filter);
}
