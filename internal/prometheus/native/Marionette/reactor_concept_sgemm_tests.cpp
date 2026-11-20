#include "test_harness.h"
#include "../reactor_concept_fft.h"
#include "../reactor_vulkan_sgemm_internal.h"
#include <array>
#include <chrono>
#include <cmath>
#include <cstdlib>
#include <thread>
#include <string_view>
#include <vector>

namespace {
struct Runtime {
    void* handle{};
    ~Runtime() { if (handle) prometheus_reactor_runtime_destroy(handle); }
};
std::vector<float> Matrix(uint32_t count, uint32_t seed) {
    std::vector<float> result(count);
    for (uint32_t i = 0; i < count; ++i) result[i] = (static_cast<int>((i * 7 + seed) % 23) - 11) / 8.0f;
    return result;
}
void CheckProduct(marionette::tests::TestContext& context, const std::vector<float>& a,
    const std::vector<float>& b, const std::vector<float>& c, uint32_t m, uint32_t n, uint32_t k) {
    for (uint32_t row = 0; row < m; ++row) for (uint32_t column = 0; column < n; ++column) {
        double expected{};
        for (uint32_t inner = 0; inner < k; ++inner) expected += double(a[row*k+inner]) * b[inner*n+column];
        ASSERT_NEAR(expected, double(c[row*n+column]), 1e-4, "GPU product agrees with independent binary64 accumulation");
    }
}
}

FACT(PrometheusReactor_ConceptSgemmSharedDeviceRetainsIndependentAsyncOutputs) {
    Runtime runtime;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &runtime.handle), "create shared runtime");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(runtime.handle, &caps), "probe runtime");
    if (!caps.available) SKIP("Vulkan unavailable");
    constexpr uint32_t m = 37, n = 29, k = 13;
    auto a = Matrix(m*k, 3), b = Matrix(k*n, 5), other = Matrix(m*k, 19);
    const auto originalA = a, originalB = b;
    std::array<int, 2> tasks{};
    uint32_t stage{}; int detail{};
    const int first = prometheus_reactor_runtime_sgemm_submit_async(runtime.handle,
        a.data(), b.data(), m, n, k, &tasks[0], &stage, &detail);
    ASSERT_EQUAL(PROM_OK, first, "submit first Concept-owned work");
    if (first != PROM_OK) { ASSERT_EQUAL(0, detail, "first submission detail"); return; }
    const int second = prometheus_reactor_runtime_sgemm_submit_async(runtime.handle,
        other.data(), b.data(), m, n, k, &tasks[1], &stage, &detail);
    ASSERT_EQUAL(PROM_OK, second, "submit independent work");
    if (second != PROM_OK) { ASSERT_EQUAL(0, detail, "second submission detail"); return; }
    // A different Concept reactor borrows this exact device while both products
    // retain their own descriptors, command buffers, submissions and output.
    const double impulse[] = {1, 0, 0, 0, 0, 0, 0, 0}; double spectrum[8]{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft64(runtime.handle, impulse, spectrum, 4,
        &stage, &detail), "FFT coexists with pending SGEMM on the shared device");
    for (uint32_t i = 0; i < 4; ++i) {
        ASSERT_NEAR(1.0, spectrum[2*i], 1e-12, "FFT real impulse response");
        ASSERT_NEAR(0.0, spectrum[2*i+1], 1e-12, "FFT imaginary impulse response");
    }
    for (uint32_t index = 0; index < 2; ++index) {
        PrometheusAsyncStatus status{};
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
        do {
            const int observed = prometheus_reactor_runtime_sgemm_query_async(runtime.handle, tasks[index], &status);
            ASSERT_EQUAL(PROM_OK, observed, "observe product");
            if (observed != PROM_OK) return;
            if (status.ready || status.failed) break;
            std::this_thread::yield();
        } while (std::chrono::steady_clock::now() < deadline);
        ASSERT_TRUE(status.ready && !status.failed, "real submission completes");
        std::vector<float> output(m*n, -999);
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(runtime.handle, tasks[index],
            output.data(), static_cast<uint32_t>(output.size()), &stage, &detail), "consume owned output");
        CheckProduct(context, index ? other : a, b, output, m, n, k);
    }
    std::vector<float> synchronous(m*n);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(runtime.handle, a.data(), b.data(), synchronous.data(),
        m, n, k, &stage, &detail), "sync work follows consumed async work without stale singleton admission");
    CheckProduct(context, a, b, synchronous, m, n, k);
    ASSERT_TRUE(a == originalA && b == originalB, "input storage is preserved");
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime.handle, &services), "shared device services");
    ASSERT_EQUAL(0u, services.validation_error_count, "shared ownership produces no Vulkan validation errors");
}

FACT(PrometheusReactor_ConceptSgemmRepeatedTransferHandoffIsValidationClean) {
    Runtime runtime;
    PrometheusReactorConfig config{};
    config.struct_size = sizeof config;
    config.test_flags = PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &runtime.handle), "create transfer runtime");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(runtime.handle, &caps), "probe transfer runtime");
    if (!caps.available) SKIP("Vulkan unavailable");
    auto a = Matrix(96*96, 3), b = Matrix(96*96, 7);
    std::vector<float> output(96*96);
    for (uint32_t iteration = 0; iteration < 3; ++iteration) {
        a = Matrix(96*96, 3 + iteration*5);
        // Upload-only calls intentionally leave host output untouched. Use the
        // resident API's explicit readback to check the transferred GPU data.
        PrometheusSgemmResidentBenchmarkRequest request{};
        request.struct_size = sizeof request;
        request.a = a.data(); request.b = b.data(); request.c = output.data();
        request.m = 96; request.n = 96; request.k = 96;
        request.iterations = 1;
        request.flags = PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK;
        PrometheusSgemmResidentBenchmarkResult result{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_resident_benchmark(runtime.handle,
            &request, &result), "reused staged buffers traverse transfer and compute with real readback");
        CheckProduct(context, a, b, output, 96, 96, 96);
    }
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime.handle, &services), "transfer services");
    if (const char* validation = std::getenv("PROMETHEUS_VK_VALIDATION"); validation && std::string_view(validation) == "1")
        ASSERT_EQUAL(1u, services.validation_enabled, "requested Vulkan validation is actually active");
    if (services.transfer_queue_available && services.compute_queue_family_index != services.transfer_queue_family_index) {
        const auto* host = static_cast<const prometheus_runtime*>(runtime.handle);
        for (const auto* input : {&host->staged_upload_a, &host->staged_upload_b, &host->staged_device_a, &host->staged_device_b})
            ASSERT_EQUAL(VK_SHARING_MODE_CONCURRENT, input->sharing_mode, "reused input pairs admit both real queue families");
    }
    ASSERT_EQUAL(0u, services.validation_error_count, "repeated queue handoffs preserve buffer ownership");
}
