#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>
#include <vector>

namespace
{
    constexpr std::uint32_t kExpectedAbiVersion = 1;

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

    const struct {
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
    } shapes[] = {
        {1u, 1u, 1u},
        {4u, 4u, 4u},
        {3u, 5u, 7u},
    };

    for (const auto& shape : shapes) {
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
            ASSERT_NEAR(expected[i], c[i], 1e-4f, "sgemm output should match CPU oracle within tolerance");
        }
    }

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

    const int null_buffer_status = prometheus_reactor_runtime_sgemm(handle, nullptr, one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, null_buffer_status, "sgemm should reject null matrix pointers");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "sgemm null pointers should report transfer-in stage");

    const int zero_dim_status = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 0u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, zero_dim_status, "sgemm should reject zero dimensions");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm invalid shape should report init stage");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
