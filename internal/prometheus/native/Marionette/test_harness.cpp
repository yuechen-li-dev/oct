#include "test_harness.h"
#include "test_doom.h"

#include <algorithm>
#include <chrono>
#include <filesystem>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <sstream>
#include <string_view>
#include <system_error>

namespace marionette::tests
{
    namespace
    {
        [[nodiscard]] std::filesystem::path GetArtifactRoot()
        {
            return std::filesystem::path{ MARIONETTE_TEST_REPO_ROOT } / "out" / "test-artifacts";
        }

        [[nodiscard]] std::string SanitizePathComponent(std::string_view value)
        {
            std::string sanitized;
            sanitized.reserve(value.size());

            for (const char character : value) {
                const bool isLetter =
                    (character >= 'a' && character <= 'z') ||
                    (character >= 'A' && character <= 'Z');
                const bool isDigit = character >= '0' && character <= '9';
                const bool isAllowedPunctuation = character == '-' || character == '_';

                if (isLetter || isDigit || isAllowedPunctuation) {
                    sanitized.push_back(character);
                    continue;
                }

                sanitized.push_back('_');
            }

            if (sanitized.empty()) {
                return "unnamed";
            }

            return sanitized;
        }

        [[nodiscard]] bool MatchesFilter(std::string_view testName, std::string_view filter)
        {
            if (filter.empty()) {
                return true;
            }

            std::size_t start = 0;
            while (start <= filter.size()) {
                const std::size_t end = filter.find(',', start);
                const std::string_view token = end == std::string_view::npos
                    ? filter.substr(start)
                    : filter.substr(start, end - start);
                if (!token.empty() && testName.find(token) != std::string_view::npos) {
                    return true;
                }
                if (end == std::string_view::npos) {
                    break;
                }
                start = end + 1;
            }

            return false;
        }

        [[nodiscard]] bool MatchesBenchmarkCategory(std::uint32_t category, std::uint32_t categoryMask)
        {
            return (category & categoryMask) != 0u;
        }

        template <typename ContextType>
        void PrintFailure(const ContextType&, const Failure& failure)
        {
            std::cout
                << "  FAIL " << failure.testName
                << " at " << failure.file << ":" << failure.line
                << " [" << failure.assertion << "]\n"
                << "    message: " << failure.message << "\n";

            if (!failure.expected.empty() || !failure.actual.empty()) {
                std::cout << "    expected: " << failure.expected << "\n";
                std::cout << "    actual:   " << failure.actual << "\n";
            }
        }

        template <typename ContextType>
        void PrintSkip(const ContextType&, const Skip& skip)
        {
            std::cout
                << "  SKIP " << skip.testName
                << " at " << skip.file << ":" << skip.line << "\n"
                << "    reason: " << skip.reason << "\n";
        }

        template <typename ContextType>
        void PrintArtifacts(const ContextType& context)
        {
            for (const std::filesystem::path& artifactPath : context.ArtifactPaths()) {
                std::cout << "    artifact: " << artifactPath.lexically_normal().string() << "\n";
            }
        }

        [[nodiscard]] std::string FormatFloatingValue(double value)
        {
            std::ostringstream stream;
            stream << std::setprecision(17) << value;
            return stream.str();
        }

        [[nodiscard]] std::string ReadTextFile(const std::filesystem::path& path)
        {
            std::ifstream file(path, std::ios::binary);
            if (!file.is_open()) {
                return {};
            }

            std::ostringstream stream;
            stream << file.rdbuf();
            return stream.str();
        }
    }

    TestContext::TestContext(std::string_view testName)
        : testName_(testName)
    {
    }

    [[nodiscard]] std::string_view TestContext::TestName() const
    {
        return testName_;
    }

    [[nodiscard]] std::string TestContext::DisplayName() const
    {
        if (theoryCaseName_.empty()) {
            return testName_;
        }

        return testName_ + "[" + theoryCaseName_ + "]";
    }

    [[nodiscard]] const std::vector<Failure>& TestContext::Failures() const
    {
        return failures_;
    }

    [[nodiscard]] const std::vector<std::filesystem::path>& TestContext::ArtifactPaths() const
    {
        return artifactPaths_;
    }

    [[nodiscard]] const Skip* TestContext::SkipState() const
    {
        if (!skipped_) {
            return nullptr;
        }

        return &skip_;
    }

    [[nodiscard]] bool TestContext::HasFailures() const
    {
        return !failures_.empty();
    }

    [[nodiscard]] bool TestContext::IsSkipped() const
    {
        return skipped_;
    }

    [[nodiscard]] std::filesystem::path TestContext::ArtifactDirectory() const
    {
        return GetArtifactRoot() / SanitizePathComponent(DisplayName());
    }

    void TestContext::RecordFailure(
        const char* file,
        int line,
        std::string_view assertion,
        std::string_view message,
        std::string_view expected,
        std::string_view actual)
    {
        failures_.push_back(Failure{
            .testName = DisplayName(),
            .file = file,
            .line = line,
            .assertion = std::string(assertion),
            .message = std::string(message),
            .expected = std::string(expected),
            .actual = std::string(actual)
        });
    }

    void TestContext::SkipTest(const char* file, int line, std::string_view reason)
    {
        skipped_ = true;
        failures_.clear();
        skip_ = Skip{
            .testName = DisplayName(),
            .file = file,
            .line = line,
            .reason = std::string(reason)
        };
    }

    void TestContext::EnterTheoryCase(std::string_view caseName)
    {
        theoryCaseName_ = caseName;
    }

    void TestContext::LeaveTheoryCase()
    {
        theoryCaseName_.clear();
    }

    [[nodiscard]] bool TestContext::WriteTextArtifact(std::string_view artifactName, std::string_view content)
    {
        const std::filesystem::path directory = ArtifactDirectory();
        std::error_code error;
        std::filesystem::create_directories(directory, error);
        if (error) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to create artifact directory",
                directory.string(),
                error.message());
            return false;
        }

        const std::filesystem::path artifactPath =
            directory / (SanitizePathComponent(artifactName) + ".txt");
        std::ofstream artifactFile(artifactPath, std::ios::binary | std::ios::trunc);
        if (!artifactFile.is_open()) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to open artifact file",
                artifactPath.string(),
                "could not open file");
            return false;
        }

        artifactFile << std::string(content);
        artifactFile.close();
        if (!artifactFile) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to write artifact file",
                artifactPath.string(),
                "write failed");
            return false;
        }

        artifactPaths_.push_back(artifactPath);
        return true;
    }

    BenchmarkContext::BenchmarkContext(std::string_view benchmarkName)
        : benchmarkName_(benchmarkName)
    {
    }

    [[nodiscard]] std::string_view BenchmarkContext::BenchmarkName() const
    {
        return benchmarkName_;
    }

    [[nodiscard]] std::string BenchmarkContext::DisplayName() const
    {
        return benchmarkName_;
    }

    [[nodiscard]] const std::vector<Failure>& BenchmarkContext::Failures() const
    {
        return failures_;
    }

    [[nodiscard]] const std::vector<std::filesystem::path>& BenchmarkContext::ArtifactPaths() const
    {
        return artifactPaths_;
    }

    [[nodiscard]] const Skip* BenchmarkContext::SkipState() const
    {
        if (!skipped_) {
            return nullptr;
        }

        return &skip_;
    }

    [[nodiscard]] bool BenchmarkContext::HasFailures() const
    {
        return !failures_.empty();
    }

    [[nodiscard]] bool BenchmarkContext::IsSkipped() const
    {
        return skipped_;
    }

    [[nodiscard]] std::filesystem::path BenchmarkContext::ArtifactDirectory() const
    {
        return GetArtifactRoot() / SanitizePathComponent(DisplayName());
    }

    void BenchmarkContext::RecordFailure(
        const char* file,
        int line,
        std::string_view assertion,
        std::string_view message,
        std::string_view expected,
        std::string_view actual)
    {
        failures_.push_back(Failure{
            .testName = DisplayName(),
            .file = file,
            .line = line,
            .assertion = std::string(assertion),
            .message = std::string(message),
            .expected = std::string(expected),
            .actual = std::string(actual)
        });
    }

    void BenchmarkContext::SkipTest(const char* file, int line, std::string_view reason)
    {
        skipped_ = true;
        failures_.clear();
        skip_ = Skip{
            .testName = DisplayName(),
            .file = file,
            .line = line,
            .reason = std::string(reason)
        };
    }

    [[nodiscard]] bool BenchmarkContext::WriteTextArtifact(std::string_view artifactName, std::string_view content)
    {
        const std::filesystem::path directory = ArtifactDirectory();
        std::error_code error;
        std::filesystem::create_directories(directory, error);
        if (error) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to create artifact directory",
                directory.string(),
                error.message());
            return false;
        }

        const std::filesystem::path artifactPath =
            directory / (SanitizePathComponent(artifactName) + ".txt");
        std::ofstream artifactFile(artifactPath, std::ios::binary | std::ios::trunc);
        if (!artifactFile.is_open()) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to open artifact file",
                artifactPath.string(),
                "could not open file");
            return false;
        }

        artifactFile << std::string(content);
        artifactFile.close();
        if (!artifactFile) {
            RecordFailure(
                __FILE__,
                __LINE__,
                "WRITE_TEXT_ARTIFACT",
                "failed to write artifact file",
                artifactPath.string(),
                "write failed");
            return false;
        }

        artifactPaths_.push_back(artifactPath);
        return true;
    }

    TestRegistrar::TestRegistrar(const char* testName, TestFunction function)
    {
        Registry().push_back(TestCase{
            .name = testName,
            .function = function
        });
    }

    void AssertDoomByName(
        TestContext& context,
        std::string_view doomCaseName,
        const char* file,
        int line)
    {
        if (!IsDoomCaseRegistered(doomCaseName)) {
            context.RecordFailure(
                file,
                line,
                "ASSERT_DOOM",
                "doom case is not registered",
                std::string(doomCaseName),
                "missing");
            return;
        }

        const std::filesystem::path artifactDirectory =
            context.ArtifactDirectory() / "doom" / std::string(doomCaseName);
        const DoomRunResult result = RunDoomCaseSubprocess(doomCaseName, artifactDirectory);
        if (!result.launched) {
            context.RecordFailure(
                file,
                line,
                "ASSERT_DOOM",
                "doom subprocess did not launch",
                "launched",
                "not launched");
            return;
        }

        const bool hasForetelling = !result.foretelling.empty();
        const bool hasBreadcrumb = std::filesystem::exists(result.breadcrumbPath);
        const bool hasStdout = std::filesystem::exists(result.stdoutPath);
        const bool hasStderr = std::filesystem::exists(result.stderrPath);

        const bool hasDiagnosticEnvelope = hasForetelling && hasBreadcrumb && hasStdout && hasStderr;
        if (!result.terminatedAbnormally || !hasDiagnosticEnvelope) {
            std::string actual = "terminatedAbnormally=" + FormatValue(result.terminatedAbnormally)
                + ", hasForetelling=" + FormatValue(hasForetelling)
                + ", hasBreadcrumb=" + FormatValue(hasBreadcrumb)
                + ", hasStdout=" + FormatValue(hasStdout)
                + ", hasStderr=" + FormatValue(hasStderr);
            context.RecordFailure(
                file,
                line,
                "ASSERT_DOOM",
                "doom envelope was not recovered",
                "terminated abnormally with diagnostic envelope",
                actual);
        }

        const std::string breadcrumbText = ReadTextFile(result.breadcrumbPath);
        const std::string stdoutText = ReadTextFile(result.stdoutPath);
        const std::string stderrText = ReadTextFile(result.stderrPath);

        std::string summary = "doom-case=" + std::string(doomCaseName) + "\n";
        summary += "terminated-abnormally=" + FormatValue(result.terminatedAbnormally) + "\n";
        summary += "exit-code=" + std::to_string(result.exitCode) + "\n";
        summary += "signal=" + std::to_string(result.signalNumber) + "\n";
        summary += "foretelling=" + result.foretelling + "\n";
        summary += "artifact-directory=" + result.artifactDirectory.string() + "\n";

        const bool wroteSummary =
            context.WriteTextArtifact(std::string("doom_") + std::string(doomCaseName) + "_summary", summary);
        const bool wroteBreadcrumb =
            context.WriteTextArtifact(std::string("doom_") + std::string(doomCaseName) + "_breadcrumb", breadcrumbText);
        const bool wroteStdout =
            context.WriteTextArtifact(std::string("doom_") + std::string(doomCaseName) + "_stdout", stdoutText);
        const bool wroteStderr =
            context.WriteTextArtifact(std::string("doom_") + std::string(doomCaseName) + "_stderr", stderrText);

        const bool wroteArtifacts = wroteSummary && wroteBreadcrumb && wroteStdout && wroteStderr;
        if (!wroteArtifacts) {
            context.RecordFailure(
                file,
                line,
                "ASSERT_DOOM",
                "failed to persist one or more doom artifacts",
                "all artifacts written",
                "artifact write failure");
        }
    }

    BenchmarkRegistrar::BenchmarkRegistrar(
        const char* benchmarkName,
        BenchmarkFunction function,
        std::uint64_t iterations,
        std::uint32_t category)
    {
        BenchmarkRegistry().push_back(BenchmarkCase{
            .name = benchmarkName,
            .function = function,
            .iterations = iterations,
            .category = category
        });
    }

    [[nodiscard]] std::vector<TestCase>& Registry()
    {
        static std::vector<TestCase> registry;
        return registry;
    }

    [[nodiscard]] std::vector<BenchmarkCase>& BenchmarkRegistry()
    {
        static std::vector<BenchmarkCase> registry;
        return registry;
    }

    [[nodiscard]] int RunAllTests(std::string_view filter)
    {
        std::vector<TestCase>& tests = Registry();
        std::sort(
            tests.begin(),
            tests.end(),
            [](const TestCase& left, const TestCase& right)
            {
                return left.name < right.name;
            });

        int executedCount = 0;
        int passedCount = 0;
        int failedCount = 0;
        int skippedCount = 0;
        int totalFailureCount = 0;

        for (const TestCase& test : tests) {
            if (!MatchesFilter(test.name, filter)) {
                continue;
            }

            ++executedCount;
            std::cout << "[RUN] " << test.name << "\n" << std::flush;

            TestContext context(test.name);
            test.function(context);

            if (context.IsSkipped()) {
                ++skippedCount;
                std::cout << "[SKIP] " << test.name << "\n";
                PrintSkip(context, *context.SkipState());
                PrintArtifacts(context);
                continue;
            }

            if (context.HasFailures()) {
                ++failedCount;
                totalFailureCount += static_cast<int>(context.Failures().size());
                std::cout << "[FAIL] " << test.name << "\n";
                for (const Failure& failure : context.Failures()) {
                    PrintFailure(context, failure);
                }
                PrintArtifacts(context);
                continue;
            }

            ++passedCount;
            std::cout << "[PASS] " << test.name << "\n";
            PrintArtifacts(context);
        }

        std::cout
            << "\nSummary: "
            << executedCount << " test(s), "
            << passedCount << " passed, "
            << skippedCount << " skipped, "
            << failedCount << " failed, "
            << totalFailureCount << " assertion failure(s)\n";

        return failedCount == 0 ? 0 : 1;
    }

    [[nodiscard]] std::vector<BenchmarkResult> ExecuteBenchmarks(std::string_view filter, std::uint32_t categoryMask)
    {
        std::vector<BenchmarkCase>& benchmarks = BenchmarkRegistry();
        std::sort(
            benchmarks.begin(),
            benchmarks.end(),
            [](const BenchmarkCase& left, const BenchmarkCase& right)
            {
                return left.name < right.name;
            });

        std::vector<BenchmarkResult> results;
        for (const BenchmarkCase& benchmark : benchmarks) {
            if (!MatchesFilter(benchmark.name, filter)) {
                continue;
            }
            if (!MatchesBenchmarkCategory(benchmark.category, categoryMask)) {
                continue;
            }

            BenchmarkContext context(benchmark.name);
            const auto start = std::chrono::steady_clock::now();
            std::uint64_t executedIterations = 0u;
            for (std::uint64_t iteration = 0; iteration < benchmark.iterations; ++iteration) {
                context.iteration = iteration;
                benchmark.function(context);
                executedIterations += 1u;
                if (context.IsSkipped() || context.HasFailures()) {
                    break;
                }
            }
            const auto stop = std::chrono::steady_clock::now();
            const auto elapsed = std::chrono::duration_cast<std::chrono::nanoseconds>(stop - start);

            results.push_back(BenchmarkResult{
                .name = benchmark.name,
                .configuredIterations = benchmark.iterations,
                .iterations = executedIterations,
                .elapsedNanoseconds = static_cast<std::uint64_t>(elapsed.count()),
                .category = benchmark.category,
                .failures = context.Failures(),
                .artifactPaths = context.ArtifactPaths(),
                .skip = context.SkipState() != nullptr ? *context.SkipState() : Skip{},
                .skipped = context.IsSkipped(),
                .failed = context.HasFailures()
            });
        }

        return results;
    }

    [[nodiscard]] int RunBenchmarks(std::string_view filter, std::uint32_t categoryMask)
    {
        const std::vector<BenchmarkResult> results = ExecuteBenchmarks(filter, categoryMask);
        int failedCount = 0;
        int skippedCount = 0;
        int totalFailureCount = 0;

        for (const BenchmarkResult& result : results) {
            BenchmarkContext context(result.name);
            const std::uint64_t averageNanoseconds =
                result.iterations == 0 ? 0 : result.elapsedNanoseconds / result.iterations;

            if (result.skipped) {
                ++skippedCount;
                std::cout << "[SKIP] " << result.name << "\n";
                PrintSkip(context, result.skip);
                for (const std::filesystem::path& artifactPath : result.artifactPaths) {
                    std::cout << "    artifact: " << artifactPath.lexically_normal().string() << "\n";
                }
                continue;
            }
            if (result.failed) {
                ++failedCount;
                totalFailureCount += static_cast<int>(result.failures.size());
                std::cout << "[FAIL] " << result.name << "\n";
                for (const Failure& failure : result.failures) {
                    PrintFailure(context, failure);
                }
                for (const std::filesystem::path& artifactPath : result.artifactPaths) {
                    std::cout << "    artifact: " << artifactPath.lexically_normal().string() << "\n";
                }
                continue;
            }

            std::cout
                << "[BENCH] " << result.name
                << " iterations=" << result.iterations;
            if (result.iterations != result.configuredIterations) {
                std::cout << "/" << result.configuredIterations;
            }
            std::cout
                << " elapsed_ns=" << result.elapsedNanoseconds
                << " avg_ns=" << averageNanoseconds
                << "\n";
            for (const std::filesystem::path& artifactPath : result.artifactPaths) {
                std::cout << "    artifact: " << artifactPath.lexically_normal().string() << "\n";
            }
        }

        std::cout
            << "\nBenchmark Summary: "
            << results.size() << " benchmark(s), "
            << skippedCount << " skipped, "
            << failedCount << " failed, "
            << totalFailureCount << " assertion failure(s)\n";
        return failedCount == 0 ? 0 : 1;
    }

    [[nodiscard]] std::string FormatValue(bool value)
    {
        return value ? "true" : "false";
    }

    [[nodiscard]] std::string FormatValue(double value)
    {
        return FormatFloatingValue(value);
    }

    [[nodiscard]] std::string FormatValue(float value)
    {
        return FormatFloatingValue(static_cast<double>(value));
    }

    [[nodiscard]] std::string FormatValue(int value)
    {
        return std::to_string(value);
    }

    [[nodiscard]] std::string FormatValue(int64_t value)
    {
        return std::to_string(value);
    }

    [[nodiscard]] std::string FormatValue(std::size_t value)
    {
        return std::to_string(value);
    }

    [[nodiscard]] std::string FormatValue(unsigned int value)
    {
        return std::to_string(value);
    }

    [[nodiscard]] std::string FormatValue(const char* value)
    {
        if (value == nullptr) {
            return "null";
        }

        return std::string(value);
    }

    [[nodiscard]] std::string FormatValue(const std::string& value)
    {
        return value;
    }

    [[nodiscard]] std::string FormatValue(std::string_view value)
    {
        return std::string(value);
    }
}
