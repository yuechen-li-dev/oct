#include "test_doom.h"
#include "test_harness.h"

#include <cstdlib>
#include <filesystem>
#include <string_view>

namespace
{
    [[nodiscard]] bool ContainsText(std::string_view text, std::string_view token)
    {
        return text.find(token) != std::string_view::npos;
    }
}

DOOM_FACT(MarionetteDoom_AbortInChild_EnvelopeCase)
{
    FORETELL_DOOM("Intentional abort to validate subprocess doom containment and artifact breadcrumbs.");
    std::abort();
}

FACT(DoomCaseRegistrationFindsNamedCase)
{
    ASSERT_TRUE(
        ::marionette::tests::IsDoomCaseRegistered("MarionetteDoom_AbortInChild_EnvelopeCase"),
        "doom case should be discoverable by name");
}

FACT(AssertDoomContainsCatastropheAndRecoversForetelling)
{
    ASSERT_DOOM(MarionetteDoom_AbortInChild_EnvelopeCase);

    bool foundSummary = false;
    bool foundBreadcrumb = false;
    for (const std::filesystem::path& artifactPath : context.ArtifactPaths()) {
        const std::string normalizedPath = artifactPath.lexically_normal().string();
        if (ContainsText(normalizedPath, "doom_MarionetteDoom_AbortInChild_EnvelopeCase_summary")) {
            foundSummary = true;
        }

        if (ContainsText(normalizedPath, "doom_MarionetteDoom_AbortInChild_EnvelopeCase_breadcrumb")) {
            foundBreadcrumb = true;
        }
    }

    ASSERT_TRUE(foundSummary, "summary artifact should be attached to parent test context");
    ASSERT_TRUE(foundBreadcrumb, "breadcrumb artifact should be attached to parent test context");
}
