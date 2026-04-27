#include "../bridge.h"
#include "test_harness.h"

#include <cmath>
#include <cstdint>
#include <limits>
#include <vector>

namespace
{
std::vector<float> matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        out[i] = static_cast<float>((static_cast<int>(i % 17u) - 8)) / 5.0f;
    }
    return out;
}

std::vector<float> matrix_with_specials(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out = matrix(rows, cols);
    if (!out.empty()) {
        out[0] = std::numeric_limits<float>::infinity();
    }
    return out;
}
}

FACT(PrometheusReactor_M15_LayoutPrecisionCache_ReusesAndInvalidatesOnPacked4AndShapeFacts)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; M15 selector cache integration cannot be asserted");
    }

    const auto a = matrix(64u, 64u);
    const auto b = matrix(64u, 64u);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "first run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &first), "first diagnostics query should succeed");
    ASSERT_EQUAL(1u, first.p10_m15_layout_precision_selector_cache_valid, "cache should become valid after first run");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1), first.p10_m15_layout_precision_selector_recompute_count,
                 "first run should recompute layout/precision subdecision");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "second run should succeed");
    PrometheusSgemmPolicyDiagnostics reused{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &reused), "second diagnostics query should succeed");
    ASSERT_TRUE(reused.p10_m15_layout_precision_selector_reuse_count >= static_cast<std::uint64_t>(1),
                "unchanged facts should reuse cached layout/precision decision");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), reused.p10_m15_layout_precision_selector_last_dirty_dependency_mask,
                 "same-value writes should not dirty layout/precision dependency keys");

    const auto a_small = matrix(3u, 64u);
    std::vector<float> c_small(3u * 64u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a_small.data(), b.data(), c_small.data(), 3u, 64u, 64u, &stage, &detail),
                 "packed4-shape dependency change run should succeed");
    PrometheusSgemmPolicyDiagnostics shape_changed{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &shape_changed),
                 "shape-change diagnostics query should succeed");
    ASSERT_TRUE(shape_changed.p10_m15_layout_precision_selector_last_dirty_dependency_mask != static_cast<std::uint64_t>(0),
                "packed4/shape dependency change should produce non-zero dirty mask");
    ASSERT_TRUE(shape_changed.p10_m15_layout_precision_selector_recompute_count >= static_cast<std::uint64_t>(2),
                "packed4/shape dependency change should force recompute");
    ASSERT_TRUE(shape_changed.p10_m15_layout_precision_selector_invalidation_count >= static_cast<std::uint64_t>(1),
                "packed4/shape dependency change should invalidate cached decision");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M15_LayoutPrecisionCache_InvalidatesOnFp16Facts_AndCanBeDisabled)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = static_cast<std::uint32_t>(sizeof(cfg));

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan unavailable; M15 fp16 dependency cache integration cannot be asserted");
    }

    const auto a = matrix(64u, 64u);
    const auto b = matrix(64u, 64u);
    const auto a_special = matrix_with_specials(64u, 64u);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "baseline run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a_special.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "fp16-fact-changed run should succeed");

    PrometheusSgemmPolicyDiagnostics fp16_changed{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &fp16_changed),
                 "fp16-change diagnostics query should succeed");
    ASSERT_TRUE(fp16_changed.p10_m15_layout_precision_selector_last_dirty_dependency_mask != static_cast<std::uint64_t>(0),
                "fp16-dependent fact changes should dirty the layout/precision dependency mask");
    ASSERT_TRUE(fp16_changed.p10_m15_layout_precision_selector_recompute_count >= static_cast<std::uint64_t>(2),
                "fp16-dependent fact changes should force recompute");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    cfg.test_flags = PROM_TESTCFG_DISABLE_SELECTOR_CACHE;
    handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "cache-disabled runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "cache-disabled probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed after skip");
        SKIP("Vulkan unavailable; cache-disable behavior cannot be asserted");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "cache-disabled first run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "cache-disabled second run should succeed");
    PrometheusSgemmPolicyDiagnostics disabled_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &disabled_diag),
                 "cache-disabled diagnostics query should succeed");
    ASSERT_EQUAL(0u, disabled_diag.p10_m15_layout_precision_selector_cache_enabled,
                 "cache-enabled flag should reflect disabled selector-cache test config");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), disabled_diag.p10_m15_layout_precision_selector_reuse_count,
                 "disabled cache should not reuse layout/precision decisions");
    ASSERT_TRUE(disabled_diag.p10_m15_layout_precision_selector_recompute_count >= static_cast<std::uint64_t>(2),
                "disabled cache should recompute layout/precision decisions each run");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
