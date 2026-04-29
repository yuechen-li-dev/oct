#include "test_harness.h"
#include "test_doom.h"

#include <filesystem>
#include <string_view>

namespace
{
    [[nodiscard]] std::string_view ArgOrEmpty(int argc, char** argv, int index)
    {
        if (index >= argc || argv[index] == nullptr) {
            return {};
        }

        return std::string_view(argv[index]);
    }

    constexpr std::string_view kDefaultSlowFilter =
        "PrometheusReactor_P11_M11_OutOfOrderCompletionStillCommitsInEntryOrder,"
        "PrometheusReactor_P11_M17_DrainTimeoutMarksUnsafeToReuse,"
        "PrometheusReactor_P11_M20_FailureMatrix_DrainTimeoutSlowCase";
}

int main(int argc, char** argv)
{
    if (argc >= 1 && argv[0] != nullptr) {
        ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
    }

    const std::string_view filter = ArgOrEmpty(argc, argv, 1);
    return ::marionette::tests::RunAllTests(filter.empty() ? kDefaultSlowFilter : filter);
}
