#include "test_doom.h"

#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <sstream>
#include <string>
#include <string_view>
#include <system_error>

#if defined(__unix__) || defined(__APPLE__)
#include <sys/wait.h>
#endif

namespace marionette::tests
{
    namespace
    {
        struct DoomExecutionContext
        {
            std::filesystem::path artifactDirectory;
            std::filesystem::path breadcrumbPath;
            std::string foretelling;
        };

        DoomExecutionContext* g_activeDoomContext = nullptr;
        std::filesystem::path g_marionetteExecutablePath;

        [[nodiscard]] std::filesystem::path MetaPathForDirectory(const std::filesystem::path& artifactDirectory)
        {
            return artifactDirectory / "doom-meta.txt";
        }

        [[nodiscard]] std::string QuotePath(const std::filesystem::path& path)
        {
            std::string quoted = "\"";
            quoted += path.string();
            quoted += "\"";
            return quoted;
        }

        [[nodiscard]] std::string ReadTextFile(const std::filesystem::path& path)
        {
            std::ifstream stream(path, std::ios::binary);
            if (!stream.is_open()) {
                return {};
            }

            std::ostringstream buffer;
            buffer << stream.rdbuf();
            return buffer.str();
        }
    }

    DoomRegistrar::DoomRegistrar(const char* doomCaseName, DoomFunction function)
    {
        DoomRegistry().push_back(DoomCase{
            .name = doomCaseName,
            .function = function
        });
    }

    [[nodiscard]] std::vector<DoomCase>& DoomRegistry()
    {
        static std::vector<DoomCase> registry;
        return registry;
    }

    [[nodiscard]] bool IsDoomCaseRegistered(std::string_view doomCaseName)
    {
        for (const DoomCase& doomCase : DoomRegistry()) {
            if (doomCase.name == doomCaseName) {
                return true;
            }
        }

        return false;
    }

    [[nodiscard]] bool RecordDoomForetelling(std::string_view foretelling)
    {
        if (g_activeDoomContext == nullptr) {
            return false;
        }

        g_activeDoomContext->foretelling = std::string(foretelling);

        std::ofstream breadcrumb(g_activeDoomContext->breadcrumbPath, std::ios::app | std::ios::binary);
        if (!breadcrumb.is_open()) {
            return false;
        }

        breadcrumb << "foretell: " << g_activeDoomContext->foretelling << "\n";
        breadcrumb.flush();
        return static_cast<bool>(breadcrumb);
    }

    [[nodiscard]] const std::filesystem::path& CurrentDoomArtifactDirectory()
    {
        static const std::filesystem::path emptyPath;
        if (g_activeDoomContext == nullptr) {
            return emptyPath;
        }

        return g_activeDoomContext->artifactDirectory;
    }

    void SetMarionetteExecutablePath(std::filesystem::path executablePath)
    {
        g_marionetteExecutablePath = std::move(executablePath);
    }

    [[nodiscard]] int RunDoomCaseInChild(std::string_view doomCaseName, std::filesystem::path artifactDirectory)
    {
        std::error_code error;
        std::filesystem::create_directories(artifactDirectory, error);
        if (error) {
            return 31;
        }

        DoomExecutionContext context{
            .artifactDirectory = artifactDirectory,
            .breadcrumbPath = artifactDirectory / "doom-breadcrumb.txt",
            .foretelling = {}
        };

        std::ofstream breadcrumb(context.breadcrumbPath, std::ios::binary | std::ios::trunc);
        if (!breadcrumb.is_open()) {
            return 32;
        }
        breadcrumb << "doom-case: " << doomCaseName << "\n";
        breadcrumb << "child-stage: entered\n";
        breadcrumb.flush();
        if (!breadcrumb) {
            return 33;
        }
        breadcrumb.close();

        std::ofstream meta(MetaPathForDirectory(artifactDirectory), std::ios::binary | std::ios::trunc);
        if (!meta.is_open()) {
            return 34;
        }
        meta << "doom-case=" << doomCaseName << "\n";
        meta << "child-entered=1\n";
        meta.flush();
        meta.close();

        DoomFunction function = nullptr;
        for (const DoomCase& doomCase : DoomRegistry()) {
            if (doomCase.name == doomCaseName) {
                function = doomCase.function;
                break;
            }
        }

        if (function == nullptr) {
            return 35;
        }

        g_activeDoomContext = &context;
        function();
        g_activeDoomContext = nullptr;

        std::ofstream completed(MetaPathForDirectory(artifactDirectory), std::ios::binary | std::ios::app);
        if (completed.is_open()) {
            completed << "child-returned=1\n";
            completed.flush();
        }

        return 0;
    }

    [[nodiscard]] DoomRunResult RunDoomCaseSubprocess(std::string_view doomCaseName, std::filesystem::path artifactDirectory)
    {
        DoomRunResult result;
        result.artifactDirectory = artifactDirectory;
        result.breadcrumbPath = artifactDirectory / "doom-breadcrumb.txt";
        result.stdoutPath = artifactDirectory / "stdout.txt";
        result.stderrPath = artifactDirectory / "stderr.txt";

        std::error_code error;
        std::filesystem::create_directories(artifactDirectory, error);
        if (error) {
            return result;
        }

        const std::filesystem::path executablePath = g_marionetteExecutablePath;
        if (executablePath.empty()) {
            return result;
        }

        const std::string command =
            QuotePath(executablePath) +
            " --doom-case " + std::string(doomCaseName) +
            " --doom-artifact-dir " + QuotePath(artifactDirectory) +
            " > " + QuotePath(result.stdoutPath) +
            " 2> " + QuotePath(result.stderrPath);

        result.launched = true;
        const int status = std::system(command.c_str());

#if defined(__unix__) || defined(__APPLE__)
        if (WIFSIGNALED(status)) {
            result.terminatedAbnormally = true;
            result.signalNumber = WTERMSIG(status);
            result.exitCode = 0;
        } else if (WIFEXITED(status)) {
            result.exitCode = WEXITSTATUS(status);
            result.terminatedAbnormally = result.exitCode != 0;
        } else {
            result.terminatedAbnormally = true;
        }
#else
        result.exitCode = status;
        result.terminatedAbnormally = status != 0;
#endif

        const std::string breadcrumbText = ReadTextFile(result.breadcrumbPath);
        const std::string token = "foretell: ";
        const std::size_t tokenStart = breadcrumbText.find(token);
        if (tokenStart != std::string::npos) {
            const std::size_t valueStart = tokenStart + token.size();
            const std::size_t valueEnd = breadcrumbText.find('\n', valueStart);
            if (valueEnd == std::string::npos) {
                result.foretelling = breadcrumbText.substr(valueStart);
            } else {
                result.foretelling = breadcrumbText.substr(valueStart, valueEnd - valueStart);
            }
        }

        return result;
    }
}
