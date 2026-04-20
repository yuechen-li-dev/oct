#include "test_harness.h"
#include "test_doom.h"

#include <filesystem>
#include <string_view>

int main(int argc, char** argv)
{
    if (argc >= 1 && argv[0] != nullptr) {
        ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
    }

    if (argc >= 2 && argv[1] != nullptr) {
        const std::string_view mode = argv[1];
        if (mode == "--doom-case") {
            if (argc < 5 || argv[2] == nullptr || argv[3] == nullptr || argv[4] == nullptr) {
                return 20;
            }

            const std::string_view doomCaseName = argv[2];
            const std::string_view artifactFlag = argv[3];
            if (artifactFlag != "--doom-artifact-dir") {
                return 21;
            }

            const std::filesystem::path artifactDirectory(argv[4]);
            return ::marionette::tests::RunDoomCaseInChild(doomCaseName, artifactDirectory);
        }

        if (mode == "--bench") {
            std::string_view filter;
            if (argc >= 3 && argv[2] != nullptr) {
                filter = argv[2];
            }

            return ::marionette::tests::RunBenchmarks(filter);
        }
    }

    std::string_view filter;
    if (argc >= 2 && argv[1] != nullptr) {
        filter = argv[1];
    }

    return ::marionette::tests::RunAllTests(filter);
}
