#include "../bridge.h"
#include "test_harness.h"

#include <cmath>
#include <cstdint>
#include <vector>

namespace
{
    constexpr std::uint32_t kExpectedAbiVersion = 1;
}

FACT(PrometheusReactorStub_ABIVersionIsStable)
{
    ASSERT_EQUAL(kExpectedAbiVersion, prometheus_reactor_abi_version(), "reactor ABI version must remain stable");
}

FACT(PrometheusReactorStub_CreateDestroyLifecycle)
{
    void* handle = nullptr;
    const int createStatus = prometheus_reactor_runtime_create(nullptr, &handle);
    ASSERT_EQUAL(PROM_OK, createStatus, "runtime create should succeed");
    ASSERT_TRUE(handle != nullptr, "runtime create should produce an opaque handle");

    const int destroyStatus = prometheus_reactor_runtime_destroy(handle);
    ASSERT_EQUAL(PROM_OK, destroyStatus, "destroy should succeed for valid handle");

    const int secondDestroyStatus = prometheus_reactor_runtime_destroy(handle);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, secondDestroyStatus, "second destroy should be defensive and non-crashing");
}

FACT(PrometheusReactorStub_ProbeIsDeterministic)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    const int probeStatus = prometheus_reactor_runtime_probe(handle, &caps);
    ASSERT_EQUAL(PROM_OK, probeStatus, "probe should succeed for valid handle");
    ASSERT_EQUAL(0u, caps.available, "stub probe should report unavailable deterministically");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_BACKEND_STUB), caps.backend_type, "backend type should identify stub backend");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REASON_STUB_UNAVAILABLE), caps.reason_code, "reason code should indicate stub-unavailable state");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactorStub_InvalidUsage)
{
    void* handle = nullptr;
    const int createNullOut = prometheus_reactor_runtime_create(nullptr, nullptr);
    ASSERT_EQUAL(PROM_ERROR, createNullOut, "create should reject null out-handle");

    PrometheusCaps caps{};
    const int probeNullHandle = prometheus_reactor_runtime_probe(nullptr, &caps);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, probeNullHandle, "probe should reject null handle");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    const int probeNullCaps = prometheus_reactor_runtime_probe(handle, nullptr);
    ASSERT_EQUAL(PROM_ERROR, probeNullCaps, "probe should reject null caps output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactorStub_SgemmComputesDeterministicRowMajorProduct)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    constexpr std::uint32_t m = 2;
    constexpr std::uint32_t n = 3;
    constexpr std::uint32_t k = 4;
    const std::vector<float> a = {
        1.0f, 2.0f, 3.0f, 4.0f,
        5.0f, 6.0f, 7.0f, 8.0f};
    const std::vector<float> b = {
        1.0f, 0.0f, 2.0f,
        0.0f, 1.0f, 3.0f,
        1.0f, 0.0f, 4.0f,
        0.0f, 1.0f, 5.0f};
    std::vector<float> c(m * n, 0.0f);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = -1;
    const int status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    ASSERT_EQUAL(PROM_OK, status, "sgemm should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "sgemm should report transfer-out stage on success");
    ASSERT_EQUAL(0, detail, "sgemm success should provide zero detail code");

    const std::vector<float> expected = {
        4.0f, 6.0f, 40.0f,
        12.0f, 14.0f, 96.0f};
    ASSERT_EQUAL(expected.size(), c.size(), "output sizes should match");
    for (std::size_t i = 0; i < expected.size(); ++i) {
        ASSERT_TRUE(std::fabs(expected[i] - c[i]) < 1e-6f, "sgemm output should match expected");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactorStub_SgemmRejectsInvalidArguments)
{
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    std::vector<float> one(1u, 1.0f);
    const int invalidHandleStatus = prometheus_reactor_runtime_sgemm(nullptr, one.data(), one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, invalidHandleStatus, "sgemm should reject null handle");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm null handle should report init stage");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    const int nullBufferStatus = prometheus_reactor_runtime_sgemm(handle, nullptr, one.data(), one.data(), 1u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, nullBufferStatus, "sgemm should reject null matrix pointers");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_IN), stage, "sgemm null pointers should report transfer-in stage");

    const int zeroDimStatus = prometheus_reactor_runtime_sgemm(handle, one.data(), one.data(), one.data(), 0u, 1u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, zeroDimStatus, "sgemm should reject zero dimensions");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_INIT), stage, "sgemm invalid shape should report init stage");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
