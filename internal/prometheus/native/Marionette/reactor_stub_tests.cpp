#include "../bridge.h"
#include "test_harness.h"

#include <cstdint>

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
