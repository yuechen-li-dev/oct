#pragma once

#include <filesystem>
#include <string>
#include <string_view>
#include <vector>

namespace marionette::tests
{
    using DoomFunction = void (*)();

    struct DoomCase
    {
        std::string name;
        DoomFunction function = nullptr;
    };

    class DoomRegistrar
    {
    public:
        DoomRegistrar(const char* doomCaseName, DoomFunction function);
    };

    struct DoomRunResult
    {
        bool launched = false;
        bool terminatedAbnormally = false;
        int exitCode = 0;
        int signalNumber = 0;
        std::filesystem::path artifactDirectory;
        std::filesystem::path breadcrumbPath;
        std::filesystem::path stdoutPath;
        std::filesystem::path stderrPath;
        std::string foretelling;
    };

    [[nodiscard]] std::vector<DoomCase>& DoomRegistry();
    [[nodiscard]] bool IsDoomCaseRegistered(std::string_view doomCaseName);
    [[nodiscard]] bool RecordDoomForetelling(std::string_view foretelling);
    [[nodiscard]] const std::filesystem::path& CurrentDoomArtifactDirectory();

    void SetMarionetteExecutablePath(std::filesystem::path executablePath);
    [[nodiscard]] int RunDoomCaseInChild(std::string_view doomCaseName, std::filesystem::path artifactDirectory);
    [[nodiscard]] DoomRunResult RunDoomCaseSubprocess(std::string_view doomCaseName, std::filesystem::path artifactDirectory);
}

#define DOOM_FACT(DOOM_CASE_NAME) \
    static void DOOM_CASE_NAME(); \
    static const ::marionette::tests::DoomRegistrar DOOM_CASE_NAME##_doom_registrar(#DOOM_CASE_NAME, &DOOM_CASE_NAME); \
    static void DOOM_CASE_NAME()

#define FORETELL_DOOM(MESSAGE) \
    do { \
        (void)::marionette::tests::RecordDoomForetelling((MESSAGE)); \
    } while (false)
