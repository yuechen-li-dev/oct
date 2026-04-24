#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <vector>

namespace
{
std::vector<float> matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        out[i] = static_cast<float>((i % 13u) + 1u) / 7.0f;
    }
    return out;
}
}

FACT(PrometheusReactor_M31_TransferQueueFallback_NoDedicatedQueue)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY | PROM_TESTCFG_FORCE_NO_DEDICATED_TRANSFER;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; dedicated transfer fallback cannot be asserted");
    }

    const auto a = matrix(64u, 64u);
    const auto b = matrix(64u, 64u);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail), "sgemm should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.m31_transfer_queue_used, "no dedicated queue must fallback to single-queue upload path");
    ASSERT_EQUAL(PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE, diag.m31_transfer_fallback_reason, "fallback reason must expose no-dedicated-queue");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M31_TransferQueueFallback_PseudoSharedQueue)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY | PROM_TESTCFG_FORCE_SHARED_TRANSFER;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; pseudo-transfer fallback cannot be asserted");
    }

    const auto a = matrix(64u, 64u);
    const auto b = matrix(64u, 64u);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail), "sgemm should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m31_dedicated_transfer_available == 1u, "pseudo-transfer simulation should preserve transfer-capable queue visibility");
    ASSERT_EQUAL(0u, diag.m31_queue_families_differ, "pseudo transfer must share queue family with compute");
    ASSERT_EQUAL(PROM_TRANSFER_FALLBACK_PSEUDO_SHARED_QUEUE, diag.m31_transfer_fallback_reason, "pseudo/shared transfer must not claim overlap enablement");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M31_TransferQueuePath_EnabledWhenDedicatedAndLarge)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; dedicated transfer selection cannot be asserted");
    }

    const auto a = matrix(96u, 96u);
    const auto b = matrix(96u, 96u);
    std::vector<float> c(96u * 96u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 96u, 96u, 96u, &stage, &detail), "sgemm should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    if (diag.m31_dedicated_transfer_available == 0u || diag.m31_queue_families_differ == 0u) {
        SKIP("runtime does not expose a real dedicated transfer queue");
    }
    ASSERT_EQUAL(1u, diag.m31_transfer_queue_used, "dedicated transfer queue should be used for qualifying upload-only staged work");
    ASSERT_EQUAL(PROM_TRANSFER_FALLBACK_NONE, diag.m31_transfer_fallback_reason, "enabled transfer path should not report fallback reason");
    ASSERT_TRUE(diag.m31_transfer_compute_wait_count >= 1u, "compute should wait on transfer completion");
    ASSERT_TRUE(diag.m31_queue_family_handoff_count >= 2u, "queue-family ownership release/acquire handoff must be observed");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M31_TransferQueueAsyncReadinessWaitsForTransferAndCompute)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY | PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE) {
        SKIP("Async readiness test needs non-software Vulkan runtime");
    }

    const auto a = matrix(96u, 96u);
    const auto b = matrix(96u, 96u);
    int task_id = 0;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int submit_status = prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), 96u, 96u, 96u, &task_id, &stage, &detail);
    if (submit_status != PROM_OK) {
        SKIP("async submission rejected by runtime policy");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    if (diag.m31_transfer_queue_used == 0u) {
        SKIP("runtime fell back to single-queue path; transfer readiness not applicable");
    }

    PrometheusAsyncStatus status{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status), "query should succeed");
    if (status.ready == 1u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
        ASSERT_EQUAL(1u, diag.m31_async_transfer_complete, "ready async status must include completed transfer work");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "abandon should clean up task");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M31_TransferSubmitFailureMarksSlotFailure)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY | PROM_TESTCFG_FAIL_TRANSFER_SUBMIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; transfer failure path cannot be asserted");
    }

    const auto a = matrix(96u, 96u);
    const auto b = matrix(96u, 96u);
    std::vector<float> c(96u * 96u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 96u, 96u, 96u, &stage, &detail);

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    if (diag.m31_transfer_queue_used == 0u) {
        SKIP("runtime fell back to single-queue path; transfer-submit failure test not applicable");
    }
    ASSERT_EQUAL(PROM_ERROR, status, "transfer submit failure injection must fail the call");
    ASSERT_TRUE(diag.m31_transfer_failure_slot_id >= 0, "transfer failure should surface failed slot id");
    ASSERT_TRUE(diag.m31_transfer_failure_reason != 0, "transfer failure should surface explicit reason");
    ASSERT_TRUE(diag.m29_failure_slot_id >= 0, "slot failure should remain connected to fixed-double ownership tracking");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
