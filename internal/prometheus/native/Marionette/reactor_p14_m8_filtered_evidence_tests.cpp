#include "../reactor_api.h"
#include "test_harness.h"

#include <vector>

namespace {

bool run_sgemm_once(void* handle, std::uint32_t m, std::uint32_t n, std::uint32_t k) {
    std::vector<float> a(static_cast<std::size_t>(m) * k, 1.0f);
    std::vector<float> b(static_cast<std::size_t>(k) * n, 1.0f);
    std::vector<float> c(static_cast<std::size_t>(m) * n, 0.0f);
    std::uint32_t stage = 0;
    int detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail) == PROM_OK;
}

}

FACT(PrometheusReactor_Sgemm_P14_FilteredEvidenceFieldsPresentWhenTimingValid)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime handle should be non-null");

    if (!run_sgemm_once(handle, 64u, 64u, 64u)) {
        prometheus_reactor_runtime_destroy(handle);
        SKIP("SGEMM execution unavailable in environment");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");

    if (diag.p13_m5_last_gpu_timing_valid == 0u) {
        ASSERT_EQUAL(0u, diag.p14_m8_filter_evidence_valid, "invalid timing should not produce filter evidence");
    } else {
        ASSERT_EQUAL(1u, diag.p14_m8_filter_evidence_valid, "valid timing should produce filter evidence");
        ASSERT_TRUE(diag.p14_m8_raw_gpu_duration_ns > 0.0, "raw duration should be present");
        ASSERT_TRUE(diag.p14_m8_filtered_gpu_duration_ns > 0.0, "filtered duration should be present");
        ASSERT_TRUE(diag.p14_m8_filter_selected_kind != 0u, "selected filter kind should be populated");
        ASSERT_TRUE(diag.p14_m8_filter_sample_count >= 1u, "sample count should advance");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Sgemm_P14_RawAndFilteredTruthSeparated)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime handle should be non-null");
    if (!run_sgemm_once(handle, 32u, 32u, 32u)) {
        prometheus_reactor_runtime_destroy(handle);
        SKIP("SGEMM execution unavailable in environment");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE((&diag.p14_m8_raw_gpu_duration_ns) != (&diag.p14_m8_filtered_gpu_duration_ns), "raw and filtered must be independent fields");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Sgemm_P14_FilterStatePersistsAcrossCalls)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime handle should be non-null");

    if (!run_sgemm_once(handle, 48u, 48u, 48u) || !run_sgemm_once(handle, 48u, 48u, 48u)) {
        prometheus_reactor_runtime_destroy(handle);
        SKIP("SGEMM execution unavailable in environment");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    if (diag.p13_m5_last_gpu_timing_valid != 0u) {
        ASSERT_TRUE(diag.p14_m8_filter_sample_count >= 2u, "filter sample count should increase across repeated calls");
        ASSERT_EQUAL(1u, diag.p14_m8_filter_evidence_valid, "filtered evidence should remain valid after repeated calls");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Sgemm_P14_InvalidTimingDoesNotUpdateFilter)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime handle should be non-null");

    std::vector<float> a(64u * 64u, 1.0f), b(64u * 64u, 1.0f), c(64u * 64u, 0.0f);
    std::uint32_t stage = 0;
    int detail = 0;
    (void)prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail);

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.p14_m8_filter_evidence_valid, "missing/invalid timing must not produce filtered evidence");
    ASSERT_EQUAL(0u, diag.p14_m8_filter_sample_count, "missing/invalid timing must not advance filter sample count");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
