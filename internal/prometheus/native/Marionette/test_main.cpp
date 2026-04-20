#include "test_doom.h"
#include "test_harness.h"

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

    const std::string_view mode = ArgOrEmpty(argc, argv, 1);

    if (mode == "--doom-case") {
        const std::string_view doomCaseName = ArgOrEmpty(argc, argv, 2);
        const std::string_view artifactFlag = ArgOrEmpty(argc, argv, 3);
        const std::string_view artifactPath = ArgOrEmpty(argc, argv, 4);
        if (doomCaseName.empty() || artifactFlag != "--doom-artifact-dir" || artifactPath.empty()) {
            return 2;
        }

        return ::marionette::tests::RunDoomCaseInChild(doomCaseName, std::filesystem::path(artifactPath));
    }

    if (mode == "--bench") {
        return ::marionette::tests::RunBenchmarks(ArgOrEmpty(argc, argv, 2));
    }

    return ::marionette::tests::RunAllTests(mode);
}
