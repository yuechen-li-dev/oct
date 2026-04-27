#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <limits>
#include <vector>

namespace
{
constexpr std::uint32_t kPromSlotEmpty = 1u;
constexpr std::uint32_t kPromSlotFailed = 7u;

std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        const int v = static_cast<int>(i % 19u) - 9;
        out[i] = static_cast<float>(v) / 5.0f;
    }
    return out;
}

bool run_sgemm_checked(void* handle, std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail) == PROM_OK;
}

bool read_diag(void* handle, PrometheusSgemmPolicyDiagnostics& out_diag)
{
    out_diag = {};
    return prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &out_diag) == PROM_OK;
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

FACT(PrometheusReactor_M14_BufferArtifacts_MOnlyChangeInvalidatesAAndCButCanReuseB)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 M-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 3u, 3u), "m-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count, "m-only change should invalidate A artifact");
    ASSERT_TRUE(after.m14_c_invalidation_count > before.m14_c_invalidation_count, "m-only change should invalidate C artifact");
    ASSERT_TRUE(after.m14_b_reuse_count > before.m14_b_reuse_count, "m-only change should allow B artifact reuse when compatible");
    ASSERT_TRUE(after.m14_false_invalidation_avoided_count > before.m14_false_invalidation_avoided_count,
                "m-only change should avoid at least one false invalidation");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_NOnlyChangeInvalidatesBAndCButCanReuseA)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 N-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 5u, 3u), "n-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_b_invalidation_count > before.m14_b_invalidation_count, "n-only change should invalidate B artifact");
    ASSERT_TRUE(after.m14_c_invalidation_count > before.m14_c_invalidation_count, "n-only change should invalidate C artifact");
    ASSERT_TRUE(after.m14_a_reuse_count > before.m14_a_reuse_count, "n-only change should allow A artifact reuse when compatible");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_KOnlyChangeInvalidatesAAndBWhileCCanReuse)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 K-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 5u), "k-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count, "k-only change should invalidate A artifact");
    ASSERT_TRUE(after.m14_b_invalidation_count > before.m14_b_invalidation_count, "k-only change should invalidate B artifact");
    ASSERT_TRUE(after.m14_c_reuse_count > before.m14_c_reuse_count, "k-only change should allow C reuse when representation/capacity are stable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_LayoutTransitionInvalidatesArtifactNamespace)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 layout transition invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "packed4-eligible baseline should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "scalar-compatible followup should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_layout_precision_invalidation_count > before.m14_layout_precision_invalidation_count,
                "layout namespace transition should count as layout/precision invalidation");
    ASSERT_TRUE(after.m14_a_last_invalidation_reason == 3u || after.m14_b_last_invalidation_reason == 3u,
                "at least one input artifact should report layout/precision invalidation reason");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_PrecisionTransitionAndCapacitySafety)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_FP16_UTILITY_WIN;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 precision transition invalidation cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    std::vector<float> a = deterministic_matrix(m, k);
    std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail),
                 "first precision candidate call should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");

    a[0] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail),
                 "same-shape precision fallback call should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count || after.m14_b_invalidation_count > before.m14_b_invalidation_count,
                "precision transition must invalidate at least one input artifact");
    ASSERT_TRUE(after.m14_a_last_invalidation_reason != 0u || after.m14_b_last_invalidation_reason != 0u,
                "precision transition should emit a concrete invalidation reason for an input artifact");

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

FACT(PrometheusReactor_M29_FixedDouble_BusyWaitRequiredDistinctFromOverwrite)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE) {
        SKIP("Busy/wait behavior needs non-software Vulkan async submission support");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), 8u, 8u, 8u, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "second call while pipeline is full should reject");
    ASSERT_EQUAL(PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED, detail, "busy fixed-double condition must be explicit wait/retry semantics");
    ASSERT_TRUE(detail != PROM_DETAIL_SLOT_OVERWRITE_REJECTED, "busy wait condition must not masquerade as overwrite corruption");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "busy handling must preserve WIP <= 2");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(diag.m29_overwrite_rejection_count), "normal busy-full pipeline should not increment overwrite rejection counter");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "cleanup should release slot ownership");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_PostSwapFailureMarksSlotFailed)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_QUEUE_SUBMIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; submit failure path cannot be asserted");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "queue-submit failure hook should fail call");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_failure_slot_id >= 0, "post-swap failure must record failed slot id");
    ASSERT_TRUE(diag.m29_failure_reason != 0, "post-swap failure must record failure reason");
    ASSERT_TRUE(diag.m29_slot0_state == kPromSlotFailed || diag.m29_slot1_state == kPromSlotFailed, "post-swap failure must move active slot into FAILED");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_CleanupToEmptyDeterministicForSafeStates)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_DISPATCH;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; cleanup semantics cannot be asserted");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "dispatch failure should leave slot in recoverable failed state");

    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &before), "diagnostics should be queryable before recovery");
    ASSERT_TRUE(before.m29_slot0_state == kPromSlotFailed || before.m29_slot1_state == kPromSlotFailed, "setup should expose a failed slot before recovery");

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "second run keeps failure injection active but must still perform deterministic cleanup first");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "third run should revisit failed slot and cleanup via legal cleanup path");

    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &after), "diagnostics should remain queryable after recovery");
    ASSERT_TRUE(after.m29_cleanup_success_count > before.m29_cleanup_success_count, "cleanup counter should advance when recovering failed/non-inflight slot");
    ASSERT_TRUE(after.m29_overwrite_rejection_count == before.m29_overwrite_rejection_count, "cleanup recovery should not be misreported as overwrite rejection");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_CleanupRejectsInflightOwnership)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE) {
        SKIP("In-flight cleanup rejection needs non-software Vulkan async submission support");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), 8u, 8u, 8u, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "abandon must reject unresolved in-flight ownership");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
    ASSERT_TRUE(diag.m29_inflight_rejection_count >= 1u, "illegal in-flight cleanup attempt should increment in-flight rejection diagnostics");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
