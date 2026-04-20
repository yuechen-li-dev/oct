#include "test_harness.h"

FACT(MarionetteHarness_CanWriteArtifact)
{
    const bool wrote = context.WriteTextArtifact("phase1_smoke", "phase1 harness smoke artifact");
    ASSERT_TRUE(wrote, "artifact write should succeed");
    ASSERT_FALSE(context.ArtifactPaths().empty(), "artifact path should be recorded");
}
