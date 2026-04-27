#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <vector>

namespace
{
std::vector<float> stable_matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        out[i] = static_cast<float>((static_cast<int>(i % 13u) - 6)) / 4.0f;
    }
    return out;
}
}

FACT(PrometheusReactor_M35_FixedDoubleRemainsDefaultWhenFeasible)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M35 fixed default integration cannot be asserted");
    }

    const auto a = stable_matrix(8u, 8u);
    const auto b = stable_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "sgemm run should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.m35_selected_buffering_mode, "fixed-double mode should remain default on feasible memory");
    ASSERT_TRUE(diag.m35_fixed_double_headroom_slots_permille >= 0, "fixed-double headroom should be non-negative when fixed mode is feasible");
    ASSERT_TRUE(diag.m35_pull_lag_predicted_demand_proxy_units == 0u,
                "pull-lag proxy-unit diagnostics should remain zero when pull-lag mode is not selected");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "fixed-double integration must preserve WIP <= 2");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M35_SerialSurvivalAndNoFeasibleFailureAreExplicit)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M35 fallback integration cannot be asserted");
    }

    const auto a = stable_matrix(8u, 8u);
    const auto b = stable_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "serial-survival fallback should still execute when feasible");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(3u, diag.m35_selected_buffering_mode, "serial-survival mode should be selected when fixed and pull-lag are blocked");
    ASSERT_TRUE(diag.m35_fixed_double_rejection_reason != 0u, "per-mode fixed rejection reason should be exported");
    ASSERT_TRUE(diag.m35_pull_lag_rejection_reason != 0u, "per-mode pull-lag rejection reason should be exported");
    ASSERT_EQUAL(diag.m35_reason_code, diag.m35_final_reason_code, "legacy and explicit final reason code fields should stay aligned");
    ASSERT_TRUE(diag.m35_serial_wip_depth <= 1u, "serial mode must keep WIP depth <= 1");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    config.test_flags = PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY | PROM_TESTCFG_DISABLE_STAGING_FALLBACK;
    handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "second runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "second probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed after skip");
        SKIP("Vulkan runtime unavailable; explicit no-feasible-mode failure cannot be asserted");
    }

    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "all-modes-blocked configuration should fail explicitly");
    ASSERT_EQUAL(PROM_DETAIL_BUFFERING_NO_MODE_FEASIBLE, detail, "no-feasible-mode failure reason should be explicit");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M35_DirtyKeySelectorCache_ReusesAndCanBeDisabled)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; selector cache integration cannot be asserted");
    }

    const auto a = stable_matrix(64u, 64u);
    const auto b = stable_matrix(64u, 64u);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "first run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &first), "first diagnostics query should succeed");
    ASSERT_EQUAL(1u, first.p10_m13_m35_selector_cache_valid, "cache should become valid after first run");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1), first.p10_m13_m35_selector_recompute_count, "first run should recompute");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), first.p10_m13_m35_selector_reuse_count, "first run should not reuse");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "second run should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &second), "second diagnostics query should succeed");
    ASSERT_TRUE(second.p10_m13_m35_selector_reuse_count >= static_cast<std::uint64_t>(1), "second run should reuse when dependencies are unchanged");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), second.p10_m13_m35_selector_last_dirty_dependency_mask, "unchanged dependency keys should report zero dirty mask");
    ASSERT_TRUE(second.p10_m13_m35_selector_last_visible_generation >= first.p10_m13_m35_selector_last_visible_generation,
                "visible generation may advance without forcing recompute");
    ASSERT_EQUAL(first.m35_selected_buffering_mode, second.m35_selected_buffering_mode, "reused decision must match baseline result");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    config.test_flags = PROM_TESTCFG_DISABLE_SELECTOR_CACHE;
    handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "cache-disabled runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "cache-disabled probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed after skip");
        SKIP("Vulkan runtime unavailable; cache-disable parity cannot be asserted");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "cache-disabled first run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail),
                 "cache-disabled second run should succeed");
    PrometheusSgemmPolicyDiagnostics disabled_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &disabled_diag),
                 "cache-disabled diagnostics query should succeed");
    ASSERT_EQUAL(0u, disabled_diag.p10_m13_m35_selector_cache_enabled, "cache-enabled flag should reflect disabled test config");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), disabled_diag.p10_m13_m35_selector_reuse_count, "disabled cache should never reuse decisions");
    ASSERT_TRUE(disabled_diag.p10_m13_m35_selector_recompute_count >= static_cast<std::uint64_t>(2), "disabled cache should recompute each run");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
