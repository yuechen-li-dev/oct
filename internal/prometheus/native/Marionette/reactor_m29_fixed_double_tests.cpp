#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <vector>

namespace
{
std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        const int v = static_cast<int>(i % 19u) - 9;
        out[i] = static_cast<float>(v) / 5.0f;
    }
    return out;
}
}

FACT(PrometheusReactor_M29_FixedDouble_HappyPathAndWipBounded)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M29 fixed-double behavior cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "first run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "second run should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_swap_count >= 2u, "swap count should advance for repeated steady calls");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "M29 fixed-double must keep WIP depth <= 2");
    ASSERT_TRUE(diag.m29_slot0_state >= 1u && diag.m29_slot0_state <= 8u, "slot0 state should remain in declared lifecycle domain");
    ASSERT_TRUE(diag.m29_slot1_state >= 1u && diag.m29_slot1_state <= 8u, "slot1 state should remain in declared lifecycle domain");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_InvalidationAndCapacityCounters)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; invalidation counters cannot be asserted");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const auto a0 = deterministic_matrix(8u, 8u);
    const auto b0 = deterministic_matrix(8u, 8u);
    std::vector<float> c0(64u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), c0.data(), 8u, 8u, 8u, &stage, &detail), "baseline shape should execute");

    std::vector<float> c0b(64u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), c0b.data(), 8u, 8u, 8u, &stage, &detail), "second baseline shape should execute");

    const auto a1 = deterministic_matrix(6u, 8u);
    const auto b1 = deterministic_matrix(8u, 6u);
    std::vector<float> c1(36u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a1.data(), b1.data(), c1.data(), 6u, 6u, 8u, &stage, &detail), "shape transition should execute");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_shape_invalidation_count >= 1u, "shape transition should increment shape invalidation counter");
    ASSERT_TRUE(diag.m29_layout_invalidation_count <= diag.m29_stale_buffer_rejection_count, "layout invalidations should be reflected in stale counter");
    ASSERT_TRUE(diag.m29_capacity_invalidation_count <= diag.m29_stale_buffer_rejection_count, "capacity invalidations should be reflected in stale counter");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_AsyncOwnershipAndRecovery)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; async ownership checks cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    const int submit_status = prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &task_id, &stage, &detail);
    if (submit_status != PROM_OK && detail == PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED) {
        SKIP("software Vulkan backend suppresses async submission in this environment");
    }
    ASSERT_EQUAL(PROM_OK, submit_status, "async submit should succeed when backend permits async");

    PrometheusAsyncStatus status{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status), "query should succeed");
    ASSERT_TRUE(status.failed == 1u, "injected async poll failure should move task to failed state");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_failure_slot_id >= 0, "failed async slot id should be surfaced");
    ASSERT_TRUE(diag.m29_failure_reason != 0, "failed async reason should be surfaced");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "abandon should provide explicit cleanup/release path");

    std::vector<float> c(m * n, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "sync run should recover after abandon cleanup");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable after recovery");
    ASSERT_TRUE(diag.m29_inflight_rejection_count <= diag.m29_overwrite_rejection_count + 1u, "inflight rejection accounting should remain bounded and truthful");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "recovery path must preserve WIP <= 2");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
