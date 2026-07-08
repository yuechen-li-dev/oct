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
}

int main(int argc, char** argv)
{
    if (argc >= 1 && argv[0] != nullptr) {
        ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
    }
    return ::marionette::tests::RunBenchmarks(
        ArgOrEmpty(argc, argv, 1),
        ::marionette::tests::MARIONETTE_BENCHMARK_CATEGORY_VALIDATED);
}
