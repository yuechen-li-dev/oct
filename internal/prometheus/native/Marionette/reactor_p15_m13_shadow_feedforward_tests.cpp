#include "../reactor_api.h"
#include "test_harness.h"

FACT(PrometheusP15M13ShadowFeedforward_DefaultOffDoesNotEnableAuthority)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_enabled, "default canary off");
    ASSERT_EQUAL(0u, diag.p15_shadow_authority_enabled, "authority must remain off when feature flag is off");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusP15M13ShadowFeedforward_EnabledFlagPropagatesToAuthority)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;
    cfg.p15_shadow_canary_enabled = 1u;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(1u, diag.p15_shadow_canary_enabled, "enabled canary exported");
    ASSERT_EQUAL(1u, diag.p15_shadow_authority_enabled, "authority should track canary feature flag");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
