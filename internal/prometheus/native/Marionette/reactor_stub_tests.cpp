#include "../bridge.h"
#include "test_harness.h"

#include <cmath>
#include <cstdint>
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
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_BACKEND_VULKAN), caps.backend_type, "available runtime should identify Vulkan backend");
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
        ASSERT_EQUAL(0, detail, "sgemm success should produce zero detail code");

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
