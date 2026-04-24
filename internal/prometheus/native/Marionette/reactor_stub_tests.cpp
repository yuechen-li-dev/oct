#include "../bridge.h"
#include "test_harness.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <thread>
#include <vector>

#include <vulkan/vulkan.h>

namespace
{
    constexpr std::uint32_t kExpectedAbiVersion = 1;

    struct ShapeCase {
        const char* name;
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
    };

    std::vector<float> cpu_oracle(std::uint32_t m, std::uint32_t n, std::uint32_t k, const std::vector<float>& a, const std::vector<float>& b)
    {
        std::vector<float> c(m * n, 0.0f);
        for (std::uint32_t row = 0; row < m; ++row) {
            for (std::uint32_t col = 0; col < n; ++col) {
                float sum = 0.0f;
                for (std::uint32_t kk = 0; kk < k; ++kk) {
                    sum += a[row * k + kk] * b[kk * n + col];
                }
                c[row * n + col] = sum;
            }
        }
        return c;
    }

    std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols)
    {
        std::vector<float> out(rows * cols, 0.0f);
        for (std::size_t i = 0; i < out.size(); ++i) {
            const int v = static_cast<int>(i % 23u) - 11;
            out[i] = static_cast<float>(v) / 7.0f;
        }
        return out;
    }

    bool is_success_detail_code(int detail)
    {
        return detail == PROM_DETAIL_PATH_DIRECT || detail == PROM_DETAIL_PATH_STAGED_UPLOAD ||
            detail == PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK || detail == PROM_DETAIL_PATH_FALLBACK_TO_DIRECT ||
            detail == PROM_DETAIL_PATH_DIRECT_TILED || detail == PROM_DETAIL_PATH_STAGED_UPLOAD_TILED ||
            detail == PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED || detail == PROM_DETAIL_PATH_DIRECT_PACKED4_FP32;
    }

}

FACT(PrometheusReactor_ABIVersionIsStable)
{
    ASSERT_EQUAL(kExpectedAbiVersion, prometheus_reactor_abi_version(), "reactor ABI version must remain stable");
}

FACT(PrometheusReactor_CreateDestroyLifecycle)
{
    void* handle = nullptr;
    const int create_status = prometheus_reactor_runtime_create(nullptr, &handle);
    ASSERT_EQUAL(PROM_OK, create_status, "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime create should produce an opaque handle");

    const int destroy_status = prometheus_reactor_runtime_destroy(handle);
    ASSERT_EQUAL(PROM_OK, destroy_status, "destroy should succeed for valid handle");

    const int second_destroy_status = prometheus_reactor_runtime_destroy(handle);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, second_destroy_status, "second destroy should be defensive and non-crashing");
}

FACT(PrometheusReactor_ProbeIsDeterministic)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    const int probe_status = prometheus_reactor_runtime_probe(handle, &caps);
    ASSERT_EQUAL(PROM_OK, probe_status, "probe should succeed for valid handle");
    if (caps.available != 0u) {
        ASSERT_TRUE(
            caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN) ||
                caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN_SOFTWARE),
            "available runtime should identify hardware or software Vulkan backend");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REASON_NONE), caps.reason_code, "available runtime should have no reason code");
    } else {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_BACKEND_UNKNOWN), caps.backend_type, "unavailable runtime should identify unknown backend");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REASON_VULKAN_UNAVAILABLE), caps.reason_code, "unavailable runtime should carry Vulkan unavailable reason");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_InvalidUsage)
{
    void* handle = nullptr;
    const int create_null_out = prometheus_reactor_runtime_create(nullptr, nullptr);
    ASSERT_EQUAL(PROM_ERROR, create_null_out, "create should reject null out-handle");

    PrometheusCaps caps{};
    const int probe_null_handle = prometheus_reactor_runtime_probe(nullptr, &caps);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, probe_null_handle, "probe should reject null handle");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    const int probe_null_caps = prometheus_reactor_runtime_probe(handle, nullptr);
    ASSERT_EQUAL(PROM_ERROR, probe_null_caps, "probe should reject null caps output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmPathMatchesCpuOracleWhenAvailable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        std::vector<float> one(1u, 1.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = -1;
        const int status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
        ASSERT_EQUAL(PROM_ERROR, status, "sgemm should fail explicitly when Vulkan runtime is unavailable");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm unavailable runtime should fail in init stage");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        return;
    }

    const ShapeCase shapes[] = {
        {"unit", 1u, 1u, 1u},
        {"square_small", 4u, 4u, 4u},
        {"non_square", 3u, 7u, 5u},
        {"degenerate_row", 1u, 9u, 4u},
        {"degenerate_col", 11u, 1u, 3u},
        {"awkward", 2u, 13u, 3u},
    };

    for (const ShapeCase& shape : shapes) {
        const std::vector<float> a = deterministic_matrix(shape.m, shape.k);
        const std::vector<float> b = deterministic_matrix(shape.k, shape.n);
        std::vector<float> c(shape.m * shape.n, 0.0f);
        const std::vector<float> expected = cpu_oracle(shape.m, shape.n, shape.k, a, b);

        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = -1;
        const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail);
        ASSERT_EQUAL(PROM_OK, status, "sgemm should succeed when Vulkan runtime is available");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "sgemm success should report transfer-out stage");
        ASSERT_TRUE(is_success_detail_code(detail), "sgemm success should surface selected execution path detail code");

        ASSERT_EQUAL(expected.size(), c.size(), "output size should match expected");
        for (std::size_t i = 0; i < expected.size(); ++i) {
            ASSERT_TRUE(std::isfinite(c[i]), "sgemm output must be finite");
            ASSERT_NEAR(expected[i], c[i], 1e-4f, "sgemm output should match CPU oracle within tolerance");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmDeterministicAcrossRepeatedRuns)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; deterministic GPU run cannot be asserted");
    }

    constexpr std::uint32_t m = 3u;
    constexpr std::uint32_t n = 5u;
    constexpr std::uint32_t k = 7u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);

    std::vector<float> first(m * n, 0.0f);
    std::vector<float> second(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = -1;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), first.data(), m, n, k, &stage, &detail), "first SGEMM run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), second.data(), m, n, k, &stage, &detail), "second SGEMM run should succeed");

    ASSERT_SEQUENCE_EQUAL(first, second, "repeated SGEMM runs should remain deterministic");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmSupportsManyConsecutiveCalls)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; consecutive-call Vulkan path cannot be asserted");
    }

    constexpr std::uint32_t m = 4u;
    constexpr std::uint32_t n = 4u;
    constexpr std::uint32_t k = 4u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    const std::vector<float> expected = cpu_oracle(m, n, k, a, b);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = -1;
    std::vector<float> previous;
    for (int iter = 0; iter < 8; ++iter) {
        std::vector<float> out(m * n, 0.0f);
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), out.data(), m, n, k, &stage, &detail), "consecutive SGEMM run should succeed");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "successful SGEMM should end in transfer-out stage");
        ASSERT_TRUE(is_success_detail_code(detail), "successful SGEMM should report selected path detail");
        for (std::size_t i = 0; i < expected.size(); ++i) {
            ASSERT_NEAR(expected[i], out[i], 1e-4f, "consecutive SGEMM output should remain numerically stable");
        }
        if (!previous.empty()) {
            ASSERT_SEQUENCE_EQUAL(previous, out, "consecutive SGEMM output should remain deterministic across runs");
        }
        previous = out;
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Packed4EligibleShapeSelectsPackedDetail)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; packed4 selection cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    const std::vector<float> expected = cpu_oracle(m, n, k, a, b);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "packed4-eligible shape should execute");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "packed4-eligible shape should complete transfer-out stage");
    ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, detail, "packed4-eligible shape should emit packed4 detail code");
    ASSERT_SEQUENCE_EQUAL(expected, c, "packed4 path output should match scalar oracle exactly");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed");
    ASSERT_TRUE(diag.packed4_selection_count >= 1u, "packed4 selection counter should increase");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Packed4TinyOrWasteFallbackReasonsAreObservable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; packed4 fallback reason observability cannot be asserted");
    }

    {
        const std::uint32_t m = 3u;
        const std::uint32_t n = 3u;
        const std::uint32_t k = 3u;
        const std::vector<float> a = deterministic_matrix(m, k);
        const std::vector<float> b = deterministic_matrix(k, n);
        std::vector<float> c(m * n, 0.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "tiny shape should still execute via scalar fallback");
        ASSERT_TRUE(detail != PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, "tiny shape should not select packed4 detail");
    }

    {
        const std::uint32_t m = 4u;
        const std::uint32_t n = 7u;
        const std::uint32_t k = 7u;
        const std::vector<float> a = deterministic_matrix(m, k);
        const std::vector<float> b = deterministic_matrix(k, n);
        std::vector<float> c(m * n, 0.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "padding-waste shape should still execute via scalar fallback");
        ASSERT_TRUE(detail != PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, "padding-waste shape should not select packed4 detail");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed");
    ASSERT_TRUE(diag.packed4_fallback_reason_small_shape >= 1u, "small-shape fallback counter should increment");
    ASSERT_TRUE((diag.packed4_fallback_reason_padding_waste + diag.packed4_fallback_reason_mode_budget_denied) >= 1u,
        "padding-budget fallback counters should increment");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Packed4TailAndRectangularCasesMatchScalarOracle)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; packed4 tail correctness cannot be asserted");
    }

    const ShapeCase shapes[] = {
        {"m_tail", 9u, 8u, 8u},
        {"n_tail", 8u, 10u, 8u},
        {"k_tail", 8u, 8u, 11u},
        {"combined_tails", 9u, 10u, 11u},
        {"tall_rect", 29u, 7u, 15u},
        {"wide_rect", 7u, 29u, 15u},
    };

    for (const ShapeCase& shape : shapes) {
        const std::vector<float> a = deterministic_matrix(shape.m, shape.k);
        const std::vector<float> b = deterministic_matrix(shape.k, shape.n);
        const std::vector<float> expected = cpu_oracle(shape.m, shape.n, shape.k, a, b);
        std::vector<float> c(shape.m * shape.n, 0.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail),
            "tail/rectangular case should execute");
        ASSERT_SEQUENCE_EQUAL(expected, c, "tail/rectangular case must match scalar oracle exactly");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed");
    ASSERT_TRUE(diag.packed4_tail_count_total >= 1u, "tail counter should reflect exercised non-multiple-of-4 shapes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_Packed4AllMod4TailCombinationsMatchScalarOracle)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; packed4 mod-4 tail matrix cannot be asserted");
    }

    for (std::uint32_t m_mod = 0u; m_mod < 4u; ++m_mod) {
        for (std::uint32_t n_mod = 0u; n_mod < 4u; ++n_mod) {
            for (std::uint32_t k_mod = 0u; k_mod < 4u; ++k_mod) {
                const std::uint32_t m = 8u + m_mod;
                const std::uint32_t n = 8u + n_mod;
                const std::uint32_t k = 8u + k_mod;
                const std::vector<float> a = deterministic_matrix(m, k);
                const std::vector<float> b = deterministic_matrix(k, n);
                const std::vector<float> expected = cpu_oracle(m, n, k, a, b);
                std::vector<float> c(m * n, 0.0f);
                std::uint32_t stage = PROM_STAGE_NONE;
                int detail = 0;
                ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail),
                    "mod-4 tail combination should execute");
                ASSERT_SEQUENCE_EQUAL(expected, c, "mod-4 tail combination must match scalar oracle exactly");
            }
        }
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed");
    ASSERT_TRUE(diag.packed4_tail_count_total >= 1u, "tail counters should remain observable after mod-4 matrix sweep");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmRejectsInvalidArguments)
{
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    std::vector<float> one(1u, 1.0f);
    const int invalid_handle_status = prometheus_reactor_runtime_sgemm(nullptr, one.data(), one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, invalid_handle_status, "sgemm should reject null handle");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm null handle should report init stage");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const int null_a_status = prometheus_reactor_runtime_sgemm(handle, nullptr, one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, null_a_status, "sgemm should reject null A pointer");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "sgemm null pointers should report transfer-in stage");

    const int null_b_status = prometheus_reactor_runtime_sgemm(handle, one.data(), nullptr, one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, null_b_status, "sgemm should reject null B pointer");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "sgemm null pointers should report transfer-in stage");

    const int null_c_status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), nullptr, 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, null_c_status, "sgemm should reject null C pointer");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "sgemm null pointers should report transfer-in stage");

    const int zero_m_status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 0u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, zero_m_status, "sgemm should reject zero M");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm invalid shape should report init stage");

    const int zero_n_status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 1u, 0u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, zero_n_status, "sgemm should reject zero N");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm invalid shape should report init stage");

    const int zero_k_status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 1u, 1u, 0u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, zero_k_status, "sgemm should reject zero K");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm invalid shape should report init stage");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_InitFailurePathsReportExplicitStages)
{
    void* handle = nullptr;
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);

    cfg.test_flags = PROM_TESTCFG_FAIL_DEVICE_CREATE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should return handle even on unavailable runtime");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(0u), caps.available, "device create failure should mark runtime unavailable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");

    handle = nullptr;
    cfg.test_flags = PROM_TESTCFG_FAIL_PIPELINE_CREATE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should return handle even on unavailable runtime");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(0u), caps.available, "pipeline create failure should mark runtime unavailable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_RuntimeFailurePathsReportExplicitStages)
{
    const struct FailureCase {
        const char* name;
        std::uint32_t flag;
        std::uint32_t expected_stage;
        int expected_detail;
    } cases[] = {
        {"buffer_alloc", PROM_TESTCFG_FAIL_BUFFER_ALLOC, PROM_STAGE_TRANSFER_IN, VK_ERROR_OUT_OF_DEVICE_MEMORY},
        {"upload", PROM_TESTCFG_FAIL_UPLOAD, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_INJECTED_UPLOAD_FAILURE},
        {"dispatch", PROM_TESTCFG_FAIL_DISPATCH, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_DISPATCH_FAILURE},
        {"download", PROM_TESTCFG_FAIL_DOWNLOAD, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE},
        {"skip_wait_in_flight", PROM_TESTCFG_SKIP_SUBMIT_WAIT, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT},
    };

    for (const FailureCase& failure : cases) {
        PrometheusReactorConfig cfg{};
        cfg.struct_size = sizeof(cfg);
        cfg.test_flags = failure.flag;

        void* handle = nullptr;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed for injected runtime failure");

        PrometheusCaps caps{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
        if (caps.available == 0u) {
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
            SKIP("Vulkan runtime unavailable; runtime failure path cannot execute");
        }

        const std::uint32_t m = 2u;
        const std::uint32_t n = 3u;
        const std::uint32_t k = 4u;
        const std::vector<float> a = deterministic_matrix(m, k);
        const std::vector<float> b = deterministic_matrix(k, n);
        std::vector<float> c(m * n, 0.0f);

        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;
        const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
        ASSERT_EQUAL(PROM_ERROR, status, "sgemm should fail for injected runtime failure");
        ASSERT_EQUAL(failure.expected_stage, stage, "failure should report expected stage code");
        ASSERT_EQUAL(failure.expected_detail, detail, "failure should report expected detail code");

        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
    }
}

FACT(PrometheusReactor_SgemmReuseHandlesShapeAndBufferChangesCorrectly)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; SGEMM reuse protocol path cannot be asserted");
    }

    const std::uint32_t same_m = 4u;
    const std::uint32_t same_n = 4u;
    const std::uint32_t same_k = 4u;
    const std::vector<float> a0 = deterministic_matrix(same_m, same_k);
    const std::vector<float> b0 = deterministic_matrix(same_k, same_n);
    const std::vector<float> expected0 = cpu_oracle(same_m, same_n, same_k, a0, b0);
    std::vector<float> out0(same_m * same_n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), out0.data(), same_m, same_n, same_k, &stage, &detail), "first SGEMM call should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "first SGEMM call should complete transfer-out stage");
    ASSERT_TRUE(is_success_detail_code(detail), "first SGEMM call should report selected path detail");
    for (std::size_t i = 0; i < expected0.size(); ++i) {
        ASSERT_NEAR(expected0[i], out0[i], 1e-4f, "first SGEMM call should match CPU oracle");
    }

    const std::vector<float> a1 = deterministic_matrix(same_m, same_k);
    const std::vector<float> b1 = deterministic_matrix(same_k, same_n);
    std::vector<float> a1_shifted = a1;
    std::vector<float> b1_shifted = b1;
    for (float& v : a1_shifted) {
        v += 0.25f;
    }
    for (float& v : b1_shifted) {
        v -= 0.5f;
    }

    const std::vector<float> expected1 = cpu_oracle(same_m, same_n, same_k, a1_shifted, b1_shifted);
    std::vector<float> out1(same_m * same_n, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a1_shifted.data(), b1_shifted.data(), out1.data(), same_m, same_n, same_k, &stage, &detail), "same-shape different host buffers should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "same-shape reuse should complete transfer-out stage");
    ASSERT_TRUE(is_success_detail_code(detail), "same-shape reuse should report selected path detail");
    for (std::size_t i = 0; i < expected1.size(); ++i) {
        ASSERT_NEAR(expected1[i], out1[i], 1e-4f, "same-shape different host buffers should not retain stale binding assumptions");
    }

    const std::uint32_t changed_m = 3u;
    const std::uint32_t changed_n = 6u;
    const std::uint32_t changed_k = 5u;
    const std::vector<float> a2 = deterministic_matrix(changed_m, changed_k);
    const std::vector<float> b2 = deterministic_matrix(changed_k, changed_n);
    const std::vector<float> expected2 = cpu_oracle(changed_m, changed_n, changed_k, a2, b2);
    std::vector<float> out2(changed_m * changed_n, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a2.data(), b2.data(), out2.data(), changed_m, changed_n, changed_k, &stage, &detail), "shape-change SGEMM call should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "shape-change SGEMM call should complete transfer-out stage");
    ASSERT_TRUE(is_success_detail_code(detail), "shape-change SGEMM call should report selected path detail");
    for (std::size_t i = 0; i < expected2.size(); ++i) {
        ASSERT_NEAR(expected2[i], out2[i], 1e-4f, "shape-change SGEMM call should invalidate stale shape assumptions");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_PathSelectionAvoidsBlindStagingForTinyShapes)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; tiny-shape path selection cannot be asserted");
    }

    const std::uint32_t m = 2u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 2u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "tiny-shape SGEMM should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "tiny-shape path should still complete with host-visible output");
    ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT, detail, "tiny-shape policy should prefer direct path over blunt staging");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_ForcedStagedUploadReadbackPathIsObservable)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; staged-readback path cannot be asserted");
    }

    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    const std::vector<float> expected = cpu_oracle(m, n, k, a, b);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "forced staged path should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "staged-readback path should complete transfer-out stage");
    ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK, detail, "staged-readback path should be explicitly observable");
    for (std::size_t i = 0; i < expected.size(); ++i) {
        ASSERT_NEAR(expected[i], c[i], 1e-4f, "staged-readback path should preserve SGEMM correctness");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_ForcedTiledPathCoversExactAndNonMultipleShapes)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_DIRECT_PATH | PROM_TESTCFG_FORCE_TILED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; forced tiled path cannot be asserted");
    }

    const ShapeCase shapes[] = {
        {"exact_multiple", 32u, 32u, 32u},
        {"non_multiple", 35u, 29u, 19u},
        {"rectangular", 64u, 8u, 13u},
    };

    for (const ShapeCase& shape : shapes) {
        const std::vector<float> a = deterministic_matrix(shape.m, shape.k);
        const std::vector<float> b = deterministic_matrix(shape.k, shape.n);
        const std::vector<float> expected = cpu_oracle(shape.m, shape.n, shape.k, a, b);
        std::vector<float> c(shape.m * shape.n, 0.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;

        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail), "forced tiled SGEMM should succeed");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "forced tiled SGEMM should complete transfer-out stage");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_TILED, detail, "forced tiled SGEMM should expose direct+tiled path selection");
        for (std::size_t i = 0; i < expected.size(); ++i) {
            ASSERT_NEAR(expected[i], c[i], 1e-4f, "forced tiled SGEMM should match CPU oracle");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_AutoPolicyKeepsSmallShapesOnNonTiledPath)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; small-shape fallback cannot be asserted");
    }

    const std::vector<float> a = deterministic_matrix(2u, 2u);
    const std::vector<float> b = deterministic_matrix(2u, 2u);
    std::vector<float> c(4u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 2u, 2u, 2u, &stage, &detail), "small-shape SGEMM should succeed");
    ASSERT_TRUE(detail != PROM_DETAIL_PATH_DIRECT_TILED, "small shapes should not auto-select direct+tiled path");
    ASSERT_TRUE(detail != PROM_DETAIL_PATH_STAGED_UPLOAD_TILED, "small shapes should not auto-select staged+tiled upload path");
    ASSERT_TRUE(detail != PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, "small shapes should not auto-select staged+tiled readback path");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_StagedCapableLargeShapeAutoSelectsStagedTiledPath)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_DISABLE_STAGING_FALLBACK;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; staged+tiled auto-selection cannot be asserted");
    }

    const std::uint32_t m = 64u;
    const std::uint32_t n = 64u;
    const std::uint32_t k = 64u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    const std::vector<float> expected = cpu_oracle(m, n, k, a, b);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    if (status == PROM_ERROR && detail == PROM_DETAIL_CAPABILITY_MISMATCH) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("device-local staging unavailable in this Vulkan runtime; staged-capable auto path cannot be asserted");
    }

    ASSERT_EQUAL(PROM_OK, status, "large SGEMM should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "staged+tiled auto path should complete transfer-out stage");
    ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, detail, "large staged-capable shape should auto-select staged+tiled readback path");
    for (std::size_t i = 0; i < expected.size(); ++i) {
        ASSERT_NEAR(expected[i], c[i], 1e-4f, "staged+tiled auto path should match CPU oracle");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_ForcedStagedTiledPathCoversExactNonMultipleAndRectangularShapes)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_TILED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; staged+tiled correctness cannot be asserted");
    }

    const ShapeCase shapes[] = {
        {"exact_multiple", 32u, 32u, 32u},
        {"non_multiple", 35u, 29u, 19u},
        {"rectangular", 64u, 8u, 13u},
    };

    for (const ShapeCase& shape : shapes) {
        const std::vector<float> a = deterministic_matrix(shape.m, shape.k);
        const std::vector<float> b = deterministic_matrix(shape.k, shape.n);
        const std::vector<float> expected = cpu_oracle(shape.m, shape.n, shape.k, a, b);
        std::vector<float> c(shape.m * shape.n, 13.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;

        const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail);
        if (status == PROM_ERROR && detail == PROM_DETAIL_CAPABILITY_MISMATCH) {
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
            SKIP("device-local staging unavailable in this Vulkan runtime; forced staged+tiled path cannot be asserted");
        }
        ASSERT_EQUAL(PROM_OK, status, "forced staged+tiled SGEMM should succeed");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "forced staged+tiled SGEMM should complete transfer-out stage");
        ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, detail, "forced staged+tiled SGEMM should expose staged+tiled path selection");
        for (std::size_t i = 0; i < expected.size(); ++i) {
            ASSERT_NEAR(expected[i], c[i], 1e-4f, "forced staged+tiled SGEMM should match CPU oracle");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_StagedTiledOutputOverwritesPriorHostBufferContents)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_TILED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; staged+tiled output overwrite behavior cannot be asserted");
    }

    constexpr std::uint32_t m = 32u;
    constexpr std::uint32_t n = 32u;
    constexpr std::uint32_t k = 32u;
    std::vector<float> a0 = deterministic_matrix(m, k);
    std::vector<float> b0 = deterministic_matrix(k, n);
    std::vector<float> a1 = deterministic_matrix(m, k);
    std::vector<float> b1 = deterministic_matrix(k, n);
    for (std::size_t i = 0; i < a1.size(); ++i) {
        a1[i] = -a1[i] + 0.125f;
    }
    for (std::size_t i = 0; i < b1.size(); ++i) {
        b1[i] = b1[i] * 0.5f - 0.25f;
    }

    const std::vector<float> expected0 = cpu_oracle(m, n, k, a0, b0);
    const std::vector<float> expected1 = cpu_oracle(m, n, k, a1, b1);
    std::vector<float> out(m * n, 77.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    int status = prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), out.data(), m, n, k, &stage, &detail);
    if (status == PROM_ERROR && detail == PROM_DETAIL_CAPABILITY_MISMATCH) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("device-local staging unavailable in this Vulkan runtime; staged+tiled output overwrite cannot be asserted");
    }
    ASSERT_EQUAL(PROM_OK, status, "first staged+tiled call should succeed");
    ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, detail, "first call should use staged+tiled path");
    for (std::size_t i = 0; i < expected0.size(); ++i) {
        ASSERT_NEAR(expected0[i], out[i], 1e-4f, "first call should match CPU oracle");
    }

    std::fill(out.begin(), out.end(), -123.0f);
    status = prometheus_reactor_runtime_sgemm(handle, a1.data(), b1.data(), out.data(), m, n, k, &stage, &detail);
    ASSERT_EQUAL(PROM_OK, status, "second staged+tiled call should succeed");
    ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, detail, "second call should use staged+tiled path");
    for (std::size_t i = 0; i < expected1.size(); ++i) {
        ASSERT_NEAR(expected1[i], out[i], 1e-4f, "second call should overwrite output with fresh SGEMM result");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_StagedUploadOnlyPathSkipsReadbackWhenNotRequired)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; staged upload-only path cannot be asserted");
    }

    const std::uint32_t m = 24u;
    const std::uint32_t n = 24u;
    const std::uint32_t k = 24u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "upload-only staged path should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_SUBMIT), stage, "upload-only path should stop before transfer-out");
    ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD, detail, "upload-only path should be explicitly observable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_StagedPathCapabilityFallbackAndMismatchAreExplicit)
{
    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    PrometheusReactorConfig cfg_fallback{};
    cfg_fallback.struct_size = sizeof(cfg_fallback);
    cfg_fallback.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY;
    void* fallback_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg_fallback, &fallback_handle), "runtime create should succeed for fallback case");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(fallback_handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(fallback_handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; fallback and mismatch path cannot be asserted");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(fallback_handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "forced staged path should fallback to direct when device-local memory is unavailable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "fallback path should still produce host-visible output");
    ASSERT_EQUAL(PROM_DETAIL_PATH_FALLBACK_TO_DIRECT, detail, "fallback-to-direct should be explicitly observable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(fallback_handle), "runtime destroy should succeed");

    PrometheusReactorConfig cfg_mismatch{};
    cfg_mismatch.struct_size = sizeof(cfg_mismatch);
    cfg_mismatch.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY | PROM_TESTCFG_DISABLE_STAGING_FALLBACK;
    void* mismatch_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg_mismatch, &mismatch_handle), "runtime create should succeed for mismatch case");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(mismatch_handle, &caps), "probe should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(mismatch_handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "staged request without device-local memory and fallback should fail explicitly");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "capability mismatch should be reported in transfer-in stage");
    ASSERT_EQUAL(PROM_DETAIL_CAPABILITY_MISMATCH, detail, "capability mismatch should use explicit detail code");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(mismatch_handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmRejectsShapeSizeOverflowExplicitly)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; overflow guard path cannot be asserted through runtime");
    }

    std::vector<float> one(1u, 1.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 65536u, 65536u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "overflow-prone SGEMM shape must fail explicitly");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "overflow rejection should be reported as transfer-in failure");
    ASSERT_EQUAL(PROM_DETAIL_SIZE_OVERFLOW, detail, "overflow rejection should use explicit overflow detail code");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_CleanupPreservesFailureStatusFromFailingStage)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FAIL_DISPATCH;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; cleanup status behavior cannot be asserted");
    }

    const std::uint32_t m = 2u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 2u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "dispatch injection should fail");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_SUBMIT), stage, "cleanup must preserve failing stage instead of rewriting status");
    ASSERT_EQUAL(PROM_DETAIL_INJECTED_DISPATCH_FAILURE, detail, "cleanup must preserve failing detail code");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_MemoryTypeFailureUsesDiagnosticCode)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FORCE_NO_MEMORY_TYPE;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; memory-type failure mapping cannot be asserted");
    }

    const std::uint32_t m = 1u;
    const std::uint32_t n = 1u;
    const std::uint32_t k = 1u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "injected memory type miss should fail");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "memory type miss should report transfer-in stage");
    ASSERT_EQUAL(static_cast<int>(VK_ERROR_FEATURE_NOT_PRESENT), detail, "memory type miss should map to feature-not-present code");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_RegistrySupportsConcurrentLifecycleOperations)
{
    constexpr int thread_count = 8;
    constexpr int iterations = 16;
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_VULKAN_INIT;
    std::vector<std::thread> threads;
    std::vector<int> failures(static_cast<std::size_t>(thread_count), 0);
    threads.reserve(thread_count);

    for (int t = 0; t < thread_count; ++t) {
        const int thread_index = t;
        threads.emplace_back(
            [&cfg, &failures, thread_index]()
            {
                for (int i = 0; i < iterations; ++i) {
                    void* handle = nullptr;
                    if (prometheus_reactor_runtime_create(&cfg, &handle) != PROM_OK || handle == nullptr) {
                        failures[static_cast<std::size_t>(thread_index)] = 1;
                        return;
                    }
                    if (prometheus_reactor_runtime_destroy(handle) != PROM_OK) {
                        failures[static_cast<std::size_t>(thread_index)] = 1;
                        return;
                    }
                }
            });
    }

    for (std::thread& thread : threads) {
        thread.join();
    }
    for (int failed : failures) {
        ASSERT_EQUAL(0, failed, "concurrent lifecycle operations should remain race-free");
    }
}

FACT(PrometheusReactor_AsyncDeferredCompletionIsExplicitlyObservable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN_SOFTWARE)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("async path requires non-software Vulkan runtime");
    }

    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    const std::vector<float> expected = cpu_oracle(m, n, k, a, b);
    std::vector<float> out(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = -1;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_TRUE(task_id > 0, "async submit should return a non-zero task id");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_SUBMIT), stage, "async submit should surface submit stage");

    PrometheusAsyncStatus status{};
    for (int attempts = 0; attempts < 2000; ++attempts) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status), "query should succeed for active async task");
        if (status.lifecycle_state == static_cast<std::uint32_t>(PROM_ASYNC_STATE_READY)) {
            break;
        }
    }
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_ASYNC_STATE_READY), status.lifecycle_state, "async status should eventually reach ready");
    ASSERT_EQUAL(1u, status.ready, "ready state should be explicit");
    ASSERT_EQUAL(0u, status.failed, "ready path should not mark failure");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task_id, out.data(), static_cast<std::uint32_t>(out.size()), &stage, &detail), "consume should succeed after readiness");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "consume should report transfer-out for readable result paths");
    for (std::size_t i = 0; i < out.size(); ++i) {
        ASSERT_NEAR(expected[i], out[i], 1e-4f, "async consumed output should match CPU oracle");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_AsyncUseBeforeCompleteAndDoubleConsumeAreRejected)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN_SOFTWARE)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("async path requires non-software Vulkan runtime");
    }

    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> out(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = -1;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_consume_async(handle, task_id, out.data(), static_cast<std::uint32_t>(out.size()), &stage, &detail), "consume before ready should fail explicitly");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NOT_READY, detail, "consume before ready should report distinct not-ready detail");

    PrometheusAsyncStatus status{};
    for (int attempts = 0; attempts < 2000; ++attempts) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status), "query should succeed");
        if (status.lifecycle_state == static_cast<std::uint32_t>(PROM_ASYNC_STATE_READY)) {
            break;
        }
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task_id, out.data(), static_cast<std::uint32_t>(out.size()), &stage, &detail), "first consume after ready should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_consume_async(handle, task_id, out.data(), static_cast<std::uint32_t>(out.size()), &stage, &detail), "second consume should fail");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_ALREADY_CONSUMED, detail, "double consume should be explicitly rejected");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_AsyncInFlightOwnershipAndAbandonmentAreSafe)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN_SOFTWARE)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("async path requires non-software Vulkan runtime");
    }

    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int first_task = -1;
    int second_task = -1;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &first_task, &stage, &detail), "first async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &second_task, &stage, &detail), "second async submit during in-flight work should fail");
    ASSERT_TRUE(detail == PROM_DETAIL_REUSE_IN_FLIGHT || detail == PROM_DETAIL_ASYNC_UNCONSUMED, "in-flight ownership hazard should be surfaced explicitly");

    PrometheusAsyncStatus status{};
    for (int attempts = 0; attempts < 2000; ++attempts) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, first_task, &status), "query should succeed");
        if (status.lifecycle_state == static_cast<std::uint32_t>(PROM_ASYNC_STATE_READY)) {
            break;
        }
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, first_task), "abandon after ready should be structurally safe");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &second_task, &stage, &detail), "submit after abandonment should succeed");
    ASSERT_TRUE(second_task > first_task, "new submission should allocate a fresh task id");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_AsyncFailureRemainsVisibleUntilExplicitAbandon)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == static_cast<std::uint32_t>(PROM_BACKEND_VULKAN_SOFTWARE)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("async path requires non-software Vulkan runtime");
    }

    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> out(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int first_task = -1;
    int second_task = -1;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &first_task, &stage, &detail), "first async submit should succeed");

    PrometheusAsyncStatus status{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, first_task, &status), "query should succeed for an active task id");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_ASYNC_STATE_FAILED), status.lifecycle_state, "injected async poll failure should surface failed lifecycle state");
    ASSERT_EQUAL(1u, status.failed, "failed bit should remain explicitly observable");
    ASSERT_EQUAL(PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE, status.detail_code, "failure detail should expose async poll failure");

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_consume_async(handle, first_task, out.data(), static_cast<std::uint32_t>(out.size()), &stage, &detail), "consume in failed state should fail explicitly");
    ASSERT_EQUAL(PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE, detail, "failed state should not collapse into not-ready semantics");
    ASSERT_TRUE(detail != PROM_DETAIL_ASYNC_NOT_READY, "failed and not-ready detail channels must remain distinct");

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &second_task, &stage, &detail), "resubmit over failed slot should be rejected");
    ASSERT_EQUAL(PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE, detail, "resubmit rejection should preserve explicit failed detail");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, first_task), "explicit abandon should acknowledge failed slot");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &second_task, &stage, &detail), "submit should succeed after explicit abandon");
    ASSERT_TRUE(second_task > first_task, "post-abandon submission should allocate a fresh task id");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmCandidateCPolicyDiagnosticsExposeBoundedState)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; SGEMM policy diagnostics need runnable runtime");
    }

    const std::uint32_t m = 16u;
    const std::uint32_t n = 16u;
    const std::uint32_t k = 16u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "sgemm run should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "policy diagnostics query should succeed");
    ASSERT_TRUE(diag.current_mode == 1u || diag.current_mode == 2u || diag.current_mode == 3u,
                "controller mode should remain in explicit bounded mode set");
    ASSERT_TRUE(diag.lookahead <= 2u, "lookahead must remain bounded at or below 2");
    ASSERT_TRUE(diag.outstanding_depth <= 2u, "outstanding depth must remain bounded at or below 2");
    ASSERT_TRUE(diag.chunk_size >= 8u && diag.chunk_size <= 32u, "chunk size must remain within bounded range");
    ASSERT_TRUE(diag.decision_count >= 1u, "controller should record decisions");
    ASSERT_EQUAL(0u, diag.bound_violation_count, "bounded controller should report zero bound violations");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmCandidateCPolicyRetreatsAndTracksGuardrails)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; SGEMM policy guardrails need runnable runtime");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t m = 128u;
    const std::uint32_t n = 128u;
    const std::uint32_t k = 64u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);

    for (int iter = 0; iter < 4; ++iter) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "high-pressure shape should execute");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "policy diagnostics query should succeed");
    ASSERT_TRUE(diag.retreat_count >= 1u || diag.safe_mode_decisions >= 1u, "controller should retreat or spend explicit time in safe mode under pressure");
    ASSERT_TRUE(diag.wasted_work_units_total >= diag.wasted_work_units_last, "waste counters should be monotonic and coherent");
    ASSERT_TRUE(diag.lag_early_warning_count >= 1u, "lag-aware pending waste warnings should be observable");
    ASSERT_TRUE(diag.budget_depletion_count <= diag.decision_count, "budget depletion count should remain bounded by decision count");
    ASSERT_EQUAL(0u, diag.bound_violation_count, "guardrailed controller should keep bound violation count at zero");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_SgemmCandidateCPendingWasteDrainsAndSafeModeCanExit)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; pending-waste regression requires runnable runtime");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t pressure_m = 128u;
    const std::uint32_t pressure_n = 128u;
    const std::uint32_t pressure_k = 64u;
    const std::vector<float> pressure_a = deterministic_matrix(pressure_m, pressure_k);
    const std::vector<float> pressure_b = deterministic_matrix(pressure_k, pressure_n);
    std::vector<float> pressure_c(pressure_m * pressure_n, 0.0f);

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, pressure_a.data(), pressure_b.data(), pressure_c.data(), pressure_m, pressure_n, pressure_k, &stage, &detail), "pressure run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, pressure_a.data(), pressure_b.data(), pressure_c.data(), pressure_m, pressure_n, pressure_k, &stage, &detail), "second pressure run should push controller into safe");

    const std::uint32_t drain_m = 64u;
    const std::uint32_t drain_n = 32u;
    const std::uint32_t drain_k = 32u;
    const std::vector<float> drain_a = deterministic_matrix(drain_m, drain_k);
    const std::vector<float> drain_b = deterministic_matrix(drain_k, drain_n);
    std::vector<float> drain_c(drain_m * drain_n, 0.0f);

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, drain_a.data(), drain_b.data(), drain_c.data(), drain_m, drain_n, drain_k, &stage, &detail), "first drain run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed after first drain run");
    const std::uint32_t pending_after_first_drain = diag.pending_waste_units;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, drain_a.data(), drain_b.data(), drain_c.data(), drain_m, drain_n, drain_k, &stage, &detail), "second drain run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed after second drain run");
    const std::uint32_t pending_after_second_drain = diag.pending_waste_units;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, drain_a.data(), drain_b.data(), drain_c.data(), drain_m, drain_n, drain_k, &stage, &detail), "third drain run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed after third drain run");
    const std::uint32_t pending_after_third_drain = diag.pending_waste_units;

    ASSERT_TRUE(pending_after_second_drain <= pending_after_first_drain, "pending waste should not rebound during drain sequence");
    ASSERT_TRUE(pending_after_third_drain <= pending_after_second_drain, "pending waste should continue monotonic decay under sustained low pressure");
    ASSERT_EQUAL(0u, pending_after_third_drain, "pending waste should saturate to zero once decay overshoots");

    const std::uint64_t non_safe_decisions_before_recovery_loop = diag.aggressive_mode_decisions + diag.recovery_mode_decisions;
    for (int iter = 0; iter < 20; ++iter) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, drain_a.data(), drain_b.data(), drain_c.data(), drain_m, drain_n, drain_k, &stage, &detail), "additional drain run should succeed");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics query should succeed during recovery loop");
    }

    const std::uint64_t non_safe_decisions_after_recovery_loop = diag.aggressive_mode_decisions + diag.recovery_mode_decisions;
    ASSERT_TRUE(non_safe_decisions_after_recovery_loop > non_safe_decisions_before_recovery_loop,
                "controller should be able to leave safe mode once pending waste has genuinely drained");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
