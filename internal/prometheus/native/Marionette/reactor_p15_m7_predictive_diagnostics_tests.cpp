#include "../reactor_api.h"
#include "../reactor_dominatus_prestage.h"
#include "test_harness.h"

#include <vector>

namespace {

bool run_sgemm_once(void* handle) {
    std::vector<float> a(64u * 64u, 1.0f);
    std::vector<float> b(64u * 64u, 1.0f);
    std::vector<float> c(64u * 64u, 0.0f);
    std::uint32_t stage = 0;
    int detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 64u, 64u, 64u, &stage, &detail) == PROM_OK;
}

}

FACT(PrometheusReactor_Sgemm_P15_PredictiveDiagnostics_FieldsPresentAndDefaultOff)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (!run_sgemm_once(handle)) {
        prometheus_reactor_runtime_destroy(handle);
        SKIP("SGEMM execution unavailable in environment");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    if (diag.p13_m5_last_gpu_timing_valid != 0u) {
        ASSERT_EQUAL(1u, diag.p15_predictor_valid, "predictor valid should follow valid timing");
        ASSERT_TRUE(diag.p15_prediction_confidence >= 0.0, "confidence present");
    }
    ASSERT_EQUAL(0u, diag.p15_prestage_submitted, "prestage action must remain default-off");
    ASSERT_TRUE((diag.p15_prestage_block_reasons & PROM_DOM_PRESTAGE_BLOCK_FEATURE_DISABLED) != 0u || diag.p15_prestage_valid == 0u,
                "prestage should show disabled block when evaluated");
    ASSERT_TRUE(diag.p15_shadow_calibration_confidence >= 0.0 && diag.p15_shadow_calibration_confidence <= 1.0,
                "calibration confidence must remain clamped");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Sgemm_P15_PredictiveDiagnostics_InvalidTimingNoAdvance)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    run_sgemm_once(handle);

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.p15_predictor_valid, "invalid timing should keep predictor invalid");
    ASSERT_EQUAL(0u, diag.p15_prediction_issued, "invalid timing should not issue prediction");
    ASSERT_EQUAL(0u, diag.p15_shadow_calibration_sample_count, "invalid timing should not advance calibration samples");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
