#include "test_harness.h"
#include "test_doom.h"

FACT(MarionetteDoom_NoDefaultCasesRegistered)
{
    ASSERT_FALSE(::marionette::tests::IsDoomCaseRegistered("nonexistent_doom_case"), "unknown doom case should not be registered");
}
