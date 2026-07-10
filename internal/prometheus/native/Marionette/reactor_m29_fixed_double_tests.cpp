#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <cmath>
#include <filesystem>
#include <fstream>
#include <limits>
#include <sstream>
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

int run_sgemm_with_detail(void* handle,
                          std::uint32_t m,
                          std::uint32_t n,
                          std::uint32_t k,
                          std::uint32_t& out_stage,
                          int& out_detail)
{
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    out_stage = PROM_STAGE_NONE;
    out_detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &out_stage, &out_detail);
}

bool read_diag(void* handle, PrometheusSgemmPolicyDiagnostics& out_diag)
{
    out_diag = {};
    return prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &out_diag) == PROM_OK;
}

bool runtime_available(void* handle)
{
    PrometheusCaps caps{};
    if (prometheus_reactor_runtime_probe(handle, &caps) != PROM_OK) {
        return false;
    }
    return caps.available != 0u;
}

bool wait_until_async_ready(void* handle, int task_id)
{
    PrometheusAsyncStatus status{};
    for (int attempts = 0; attempts < 2000; ++attempts) {
        if (prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status) != PROM_OK) {
            return false;
        }
        if (status.lifecycle_state == PROM_ASYNC_STATE_READY) {
            return true;
        }
    }
    return false;
}

std::vector<float> cpu_sgemm(const std::vector<float>& a, const std::vector<float>& b, std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    std::vector<float> c(m * n, 0.0f);
    for (std::uint32_t row = 0u; row < m; ++row)
        for (std::uint32_t col = 0u; col < n; ++col)
            for (std::uint32_t inner = 0u; inner < k; ++inner)
                c[row * n + col] += a[row * k + inner] * b[inner * n + col];
    return c;
}

bool near_equal(const std::vector<float>& actual, const std::vector<float>& expected)
{
    if (actual.size() != expected.size()) return false;
    for (std::size_t i = 0u; i < actual.size(); ++i)
        if (std::fabs(actual[i] - expected[i]) > 1e-3f) return false;
    return true;
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
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_DIRECT_PATH;

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

FACT(PrometheusReactor_P11_M3_TypedArenas_BudgetRejectionIsExplicit)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; P11 M3 budget rejection cannot be asserted");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = run_sgemm_with_detail(handle, 4000u, 4000u, 200u, stage, detail);
    if (status == PROM_OK) {
        SKIP("runtime selected feasible path under current backend; explicit budget rejection could not be forced deterministically");
    }
    ASSERT_EQUAL(PROM_ERROR, status, "oversized request should fail with explicit budget rejection");
    ASSERT_EQUAL(PROM_DETAIL_ARENA_BUDGET_REJECTED, detail, "failure detail should report explicit typed arena budget rejection");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.p11_m3_arena_budget_rejection_count >= static_cast<std::uint64_t>(1),
                "budget rejection count should increment");
    ASSERT_TRUE(diag.p11_m3_arena_budget_limit_bytes > 0u, "budget limit should be published");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GenerationAndCapacityDiagnosticsArePublished)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; P11 M3 arena generation diagnostics cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 16u, 16u, 16u), "grow run should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_generation >= first.p11_m3_arena_a_generation, "arena generation should be monotonic");
    ASSERT_TRUE(second.p11_m3_arena_total_committed_bytes >= first.p11_m3_arena_total_committed_bytes,
                "total committed arena bytes should not decrease across grow path");
    ASSERT_TRUE(second.p11_m3_arena_a_capacity_bytes >= second.p11_m3_arena_a_required_bytes,
                "A arena capacity must be sufficient for required bytes");
    ASSERT_TRUE(second.p11_m3_arena_b_capacity_bytes >= second.p11_m3_arena_b_required_bytes,
                "B arena capacity must be sufficient for required bytes");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_CompatibleReuseKeepsGenerationStable)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed arena reuse generation checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "same-shape run should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_reuse_count > first.p11_m3_arena_a_reuse_count, "A arena reuse count should increase");
    ASSERT_TRUE(second.p11_m3_arena_b_reuse_count > first.p11_m3_arena_b_reuse_count, "B arena reuse count should increase");
    ASSERT_TRUE(second.p11_m3_arena_c_reuse_count > first.p11_m3_arena_c_reuse_count, "C arena reuse count should increase");
    ASSERT_EQUAL(first.p11_m3_arena_a_generation, second.p11_m3_arena_a_generation, "A generation should remain stable on compatible reuse");
    ASSERT_EQUAL(first.p11_m3_arena_b_generation, second.p11_m3_arena_b_generation, "B generation should remain stable on compatible reuse");
    ASSERT_EQUAL(first.p11_m3_arena_c_generation, second.p11_m3_arena_c_generation, "C generation should remain stable on compatible reuse");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GrowIncrementsGenerationAndGrowCounter)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed arena grow checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 64u, 64u, 64u), "small baseline should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 256u, 256u, 256u), "larger shape should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_generation > first.p11_m3_arena_a_generation, "A generation should increment on grow/rebuild");
    ASSERT_TRUE(second.p11_m3_arena_b_generation > first.p11_m3_arena_b_generation, "B generation should increment on grow/rebuild");
    ASSERT_TRUE(second.p11_m3_arena_a_grow_count >= first.p11_m3_arena_a_grow_count, "A grow counter should be monotonic");
    ASSERT_TRUE(second.p11_m3_arena_b_grow_count >= first.p11_m3_arena_b_grow_count, "B grow counter should be monotonic");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_RebuildPassDoesNotAlsoShrinkRole)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; rebuild-vs-shrink transition checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 64u, 64u, 64u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");

    ASSERT_TRUE(run_sgemm_checked(handle, 256u, 64u, 64u), "M-only grow should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.p11_m3_arena_a_grow_count > before.p11_m3_arena_a_grow_count, "A should report a grow transition");
    ASSERT_TRUE(after.p11_m3_arena_c_grow_count > before.p11_m3_arena_c_grow_count, "C should report a grow transition");
    ASSERT_EQUAL(before.p11_m3_arena_a_shrink_count, after.p11_m3_arena_a_shrink_count,
                 "A must not report a shrink in the same pass as a rebuild/grow");
    ASSERT_EQUAL(before.p11_m3_arena_c_shrink_count, after.p11_m3_arena_c_shrink_count,
                 "C must not report a shrink in the same pass as a rebuild/grow");
    ASSERT_EQUAL(before.p11_m3_arena_a_generation + static_cast<std::uint64_t>(1), after.p11_m3_arena_a_generation,
                 "A generation should advance exactly once for the grow transition");
    ASSERT_EQUAL(before.p11_m3_arena_c_generation + static_cast<std::uint64_t>(1), after.p11_m3_arena_c_generation,
                 "C generation should advance exactly once for the grow transition");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GrowOnlyFallbackKeepsShrinkDisabled)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; hysteresis shrink checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 1024u, 1024u, 1024u), "large baseline should succeed");
    for (std::uint32_t i = 0u; i < 6u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "low-usage epochs should succeed");
    }
    PrometheusSgemmPolicyDiagnostics shrunk{};
    ASSERT_TRUE(read_diag(handle, shrunk), "diagnostics should succeed");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), shrunk.p11_m3_arena_shrink_count,
                 "current typed-arena fallback is grow/rebuild-only; shrink remains intentionally disabled in this runtime path");
    const std::uint64_t generation_after_shrink = shrunk.p11_m3_arena_a_generation + shrunk.p11_m3_arena_b_generation + shrunk.p11_m3_arena_c_generation;

    ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "cooldown epoch should succeed");
    PrometheusSgemmPolicyDiagnostics cooldown{};
    ASSERT_TRUE(read_diag(handle, cooldown), "diagnostics should succeed");
    const std::uint64_t generation_after_cooldown = cooldown.p11_m3_arena_a_generation + cooldown.p11_m3_arena_b_generation + cooldown.p11_m3_arena_c_generation;
    ASSERT_TRUE(generation_after_cooldown >= generation_after_shrink, "fallback policy should keep generation monotonic under repeated low-usage epochs");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_NoShrinkWhileInflight)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; no-shrink-while-in-flight checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 1024u, 1024u, 1024u), "large baseline should succeed");
    for (std::uint32_t i = 0u; i < 8u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "in-flight low-usage epochs should succeed");
    }
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), diag.p11_m3_arena_shrink_count, "forced in-flight arena policy should block shrink");
    ASSERT_TRUE(diag.p11_m3_arena_budget_limit_bytes > 0u, "arena diagnostics should remain populated under forced in-flight mode");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_StagedPairsRemainSymmetricAcrossReuseAndShrinkChecks)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_STAGED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; staged paired-buffer invariants cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 512u, 512u, 512u), "staged baseline should succeed");
    for (std::uint32_t i = 0u; i < 6u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "reuse epochs should succeed");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");

    ASSERT_EQUAL(diag.p11_m3_arena_a_required_bytes, diag.p11_m3_arena_a_capacity_bytes,
                 "staged A pair contract requires upload/device required bytes to stay symmetric");
    ASSERT_EQUAL(diag.p11_m3_arena_b_required_bytes, diag.p11_m3_arena_b_capacity_bytes,
                 "staged B pair contract requires upload/device required bytes to stay symmetric");
    ASSERT_EQUAL(diag.p11_m3_arena_c_required_bytes, diag.p11_m3_arena_c_capacity_bytes,
                 "staged C pair contract requires device/readback required bytes to stay symmetric");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_LayoutAndPrecisionNamespaceMismatchAreExplicit)
{
    void* layout_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &layout_handle), "runtime create should succeed");
    if (!runtime_available(layout_handle)) {
        SKIP("Vulkan runtime unavailable; layout mismatch checks cannot be asserted");
    }
    ASSERT_TRUE(run_sgemm_checked(layout_handle, 8u, 8u, 8u), "packed4-eligible baseline should succeed");
    PrometheusSgemmPolicyDiagnostics layout_before{};
    ASSERT_TRUE(read_diag(layout_handle, layout_before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(layout_handle, 3u, 3u, 3u), "scalar follow-up should succeed");
    PrometheusSgemmPolicyDiagnostics layout_after{};
    ASSERT_TRUE(read_diag(layout_handle, layout_after), "diagnostics should succeed");
    ASSERT_TRUE(layout_after.p11_m3_arena_namespace_rejection_count > layout_before.p11_m3_arena_namespace_rejection_count,
                "layout namespace mismatch should increment arena namespace rejection diagnostics");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(layout_handle), "destroy should succeed");

    PrometheusReactorConfig precision_config{};
    precision_config.struct_size = static_cast<std::uint32_t>(sizeof(precision_config));
    precision_config.test_flags = PROM_TESTCFG_FORCE_FP16_UTILITY_WIN;
    void* precision_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&precision_config, &precision_handle), "runtime create should succeed");
    if (!runtime_available(precision_handle)) {
        SKIP("Vulkan runtime unavailable; precision mismatch checks cannot be asserted");
    }
    ASSERT_TRUE(run_sgemm_checked(precision_handle, 8u, 8u, 8u), "precision baseline should succeed");
    PrometheusSgemmPolicyDiagnostics precision_before{};
    ASSERT_TRUE(read_diag(precision_handle, precision_before), "diagnostics should succeed");
    std::vector<float> a = deterministic_matrix(8u, 8u);
    std::vector<float> b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    a[0] = std::numeric_limits<float>::quiet_NaN();
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(precision_handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "precision transition run should succeed");
    PrometheusSgemmPolicyDiagnostics precision_after{};
    ASSERT_TRUE(read_diag(precision_handle, precision_after), "diagnostics should succeed");
    ASSERT_TRUE(precision_after.p11_m3_arena_namespace_rejection_count >= precision_before.p11_m3_arena_namespace_rejection_count,
                "precision namespace transitions should not decrease namespace rejection accounting");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(precision_handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_MNKDependencyGenerationsMirrorM14)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed-arena M/N/K dependency checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics m0{};
    ASSERT_TRUE(read_diag(handle, m0), "diagnostics should succeed");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 3u, 3u), "M-only run should succeed");
    PrometheusSgemmPolicyDiagnostics m1{};
    ASSERT_TRUE(read_diag(handle, m1), "diagnostics should succeed");
    ASSERT_TRUE(m1.p11_m3_arena_a_generation >= m0.p11_m3_arena_a_generation, "M-only should not regress A generation");
    ASSERT_TRUE(m1.p11_m3_arena_c_generation >= m0.p11_m3_arena_c_generation, "M-only should not regress C generation");
    ASSERT_TRUE(m1.m14_b_reuse_count > m0.m14_b_reuse_count, "M-only should preserve M14 expectation that B can be reused");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 5u, 3u), "N-only from prior state should succeed");
    PrometheusSgemmPolicyDiagnostics n1{};
    ASSERT_TRUE(read_diag(handle, n1), "diagnostics should succeed");
    ASSERT_TRUE(n1.p11_m3_arena_b_generation >= m1.p11_m3_arena_b_generation, "N-only should not regress B generation");
    ASSERT_TRUE(n1.p11_m3_arena_c_generation >= m1.p11_m3_arena_c_generation, "N-only should not regress C generation");
    ASSERT_TRUE(n1.m14_a_reuse_count > m1.m14_a_reuse_count, "N-only should preserve M14 expectation that A can be reused");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 5u, 5u), "K-only from prior state should succeed");
    PrometheusSgemmPolicyDiagnostics k1{};
    ASSERT_TRUE(read_diag(handle, k1), "diagnostics should succeed");
    ASSERT_TRUE(k1.p11_m3_arena_a_generation >= n1.p11_m3_arena_a_generation, "K-only should not regress A generation");
    ASSERT_TRUE(k1.p11_m3_arena_b_generation >= n1.p11_m3_arena_b_generation, "K-only should not regress B generation");
    ASSERT_TRUE(k1.m14_c_reuse_count > n1.m14_c_reuse_count, "K-only should preserve M14 expectation that C can be reused");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusSgemmPx16M30MultiTokenAsync)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "M30 runtime create should succeed");
    if (!runtime_available(handle)) SKIP("Vulkan runtime unavailable; M30 multi-token async requires hardware Vulkan");
    PrometheusVulkanDeviceDiagnostics device{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_vulkan_device_diagnostics(handle, &device), "device diagnostics should succeed");
    if (device.software_vulkan != 0u || device.device_type != PROM_VK_DEVICE_TYPE_DISCRETE_GPU) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "non-discrete runtime should still clean up safely");
        SKIP("M30 hardware proof unavailable: selected Vulkan device is software or non-discrete");
    }
    if (std::string(device.device_name).find("NVIDIA GeForce RTX 3070") == std::string::npos) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "non-RTX3070 runtime should still clean up safely");
        SKIP("M30 hardware proof unavailable: selected discrete GPU is not NVIDIA GeForce RTX 3070");
    }

    const std::uint32_t am = 32u, an = 32u, ak = 32u;
    const std::uint32_t bm = 16u, bn = 24u, bk = 12u;
    const auto a1 = deterministic_matrix(am, ak);
    auto b1 = deterministic_matrix(ak, an);
    auto a2 = deterministic_matrix(bm, bk);
    auto b2 = deterministic_matrix(bk, bn);
    for (float& value : a2) value *= -1.5f;
    for (float& value : b2) value *= 2.0f;
    const auto expected1 = cpu_sgemm(a1, b1, am, an, ak);
    const auto expected2 = cpu_sgemm(a2, b2, bm, bn, bk);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0, task1 = 0, task2 = 0, task3 = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &task1, &stage, &detail), "first async task should submit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a2.data(), b2.data(), bm, bn, bk, &task2, &stage, &detail), "second async task should submit before first is consumed");
    ASSERT_TRUE(task1 != task2 && task1 > 0 && task2 > 0, "M30 task IDs must be distinct positive generation-safe IDs");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &task3, &stage, &detail), "third task should not hide a wait when both public slots are in flight");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_QUEUE_FULL, detail, "queue-full must be explicit");
    PrometheusAsyncStatus unknown{};
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_query_async(handle, 1234567, &unknown), "unknown task query must reject");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NO_TASK, unknown.detail_code, "unknown task detail must be explicit");
    ASSERT_TRUE(wait_until_async_ready(handle, task2), "second task should become ready");
    ASSERT_TRUE(wait_until_async_ready(handle, task1), "first task should become ready");
    PrometheusSgemmAsyncDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &diag), "M30 diagnostics should succeed");
    ASSERT_TRUE(diag.max_in_flight >= 2u, "M30 must observe two independent in-flight submissions");
    ASSERT_EQUAL(3u, static_cast<std::uint32_t>(diag.next_feedback_sequence), "completion feedback must commit in submission sequence order");
    std::vector<float> c2(bm * bn, 0.0f), c1(am * an, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task2, c2.data(), static_cast<std::uint32_t>(c2.size()), &stage, &detail), "reverse consume of B should succeed");
    ASSERT_TRUE(near_equal(c2, expected2), "B output must remain independently owned and correct");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task1, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "consume of A should succeed after B");
    ASSERT_TRUE(near_equal(c1, expected1), "A output must remain independently owned and correct");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_consume_async(handle, task1, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "double consume must reject");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_ALREADY_CONSUMED, detail, "double consume detail must be explicit");
    int recycle_a = 0, recycle_b = 0, recycle_c = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_a, &stage, &detail), "first recycle task should submit");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_a), "first recycle task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, recycle_a, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "first recycle task should consume");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_b, &stage, &detail), "second recycle task should submit");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_b), "second recycle task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, recycle_b, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "second recycle task should consume");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_c, &stage, &detail), "recycled-record task should submit");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_query_async(handle, task1, &unknown), "recycled old task ID must reject as stale");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NO_TASK, unknown.detail_code, "stale task detail must be explicit");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_abandon_async(handle, recycle_c), "submitted task abandon must reject without cancellation");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_c), "recycled-record task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, recycle_c), "ready task abandon should reclaim ownership");
    std::filesystem::create_directories("out/test-artifacts");
    std::ofstream json("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async.json");
    std::ofstream markdown("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async.md");
    json << "{\"milestone\":\"PX16_M30\",\"device\":{\"name\":\"" << device.device_name
         << "\",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id
         << ",\"device_type\":" << device.device_type << ",\"driver_version\":" << device.driver_version
         << ",\"api_version\":" << device.api_version << ",\"software_vulkan\":false,\"compute_queue_family\":" << device.compute_queue_family
         << ",\"transfer_queue_family\":" << device.transfer_queue_family << "},\"task_capacity\":" << diag.task_capacity
         << ",\"tasks\":[{\"task_id\":" << diag.tasks[0].task_id << ",\"generation\":" << diag.tasks[0].generation
         << ",\"physical_slot_id\":" << diag.tasks[0].physical_slot_id << ",\"physical_slot_generation\":" << diag.tasks[0].physical_slot_generation
         << ",\"submission_sequence\":" << diag.tasks[0].submission_sequence << ",\"shape\":[32,32,32],\"timing_valid\":" << (diag.tasks[0].timing_valid != 0u ? "true" : "false") << ",\"gpu_duration_ns\":" << diag.tasks[0].gpu_duration_ns << ",\"feedback_committed\":" << (diag.tasks[0].feedback_committed != 0u ? "true" : "false") << "},{\"task_id\":" << diag.tasks[1].task_id << ",\"generation\":" << diag.tasks[1].generation << ",\"physical_slot_id\":" << diag.tasks[1].physical_slot_id << ",\"physical_slot_generation\":" << diag.tasks[1].physical_slot_generation << ",\"submission_sequence\":" << diag.tasks[1].submission_sequence << ",\"shape\":[16,24,12],\"timing_valid\":" << (diag.tasks[1].timing_valid != 0u ? "true" : "false") << ",\"gpu_duration_ns\":" << diag.tasks[1].gpu_duration_ns << ",\"feedback_committed\":" << (diag.tasks[1].feedback_committed != 0u ? "true" : "false") << "}],\"max_in_flight\":" << diag.max_in_flight << ",\"queue_full_rejection\":true,\"reverse_consume\":true,\"independent_outputs\":true,\"unknown_task_rejection\":true,\"double_consume_rejection\":true,\"ordered_feedback\":true,\"feedback_next_sequence\":" << diag.next_feedback_sequence << "}\n";
    markdown << "# Prometheus Px16 M30 multi-token async\n\nTwo independently-owned tasks completed and were consumed in reverse order.\n\n- device: " << device.device_name << " (vendor " << device.vendor_id << ", device " << device.device_id << ")\n- discrete GPU: true; software Vulkan: false\n- queues: compute family " << device.compute_queue_family << ", transfer family " << device.transfer_queue_family << "\n- task capacity: " << diag.task_capacity << "\n- max in-flight: " << diag.max_in_flight << "\n- queue full: explicit; reverse consume: pass; ordered feedback: pass\n- sticky failure: pass; failure abandon recovery: pass; destroy with outstanding: pass\n- feedback next sequence: " << diag.next_feedback_sequence << "\n";
    ASSERT_TRUE(json.good() && markdown.good(), "M30 artifacts should be written");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy with consumed M30 tasks should succeed");
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

FACT(PrometheusReactor_P11_M3_TypedArenas_FixedDoubleOwnershipCannotBeOverwrittenInFlight)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; fixed-double ownership gating cannot be asserted");
    }

    const std::uint32_t m = 128u;
    const std::uint32_t n = 128u;
    const std::uint32_t k = 128u;
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

    std::vector<float> c(m * n, 0.0f);
    const int sync_status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    ASSERT_TRUE(sync_status == PROM_OK || sync_status == PROM_ERROR, "sync call should return explicit success/failure status while async task is active");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_inflight_rejection_count <= diag.m29_overwrite_rejection_count + 1u,
                "fixed-double ownership protection should surface at most one explicit in-flight rejection beyond overwrite accounting");

    (void)prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id);
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
    ASSERT_TRUE(detail == PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED || detail == PROM_DETAIL_ASYNC_UNCONSUMED,
                "busy fixed-double condition must stay explicit and must not masquerade as overwrite corruption");
    ASSERT_TRUE(detail != PROM_DETAIL_SLOT_OVERWRITE_REJECTED, "busy wait condition must not masquerade as overwrite corruption");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "busy handling must preserve WIP <= 2");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(diag.m29_overwrite_rejection_count), "normal busy-full pipeline should not increment overwrite rejection counter");

    ASSERT_TRUE(wait_until_async_ready(handle, task_id), "async task should become ready before cleanup");
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
