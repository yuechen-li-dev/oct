#include "test_harness.h"
#include "test_doom.h"

#include <filesystem>

int main(int argc, char** argv)
{
    if (argc >= 1 && argv[0] != nullptr) {
        ::marionette::tests::SetMarionetteExecutablePath(std::filesystem::path(argv[0]));
    }
    return ::marionette::tests::RunAllTests("DrainTimeout,OutOfOrder,FailureMatrix_DrainTimeoutSlowCase");
}
