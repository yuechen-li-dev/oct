#include "test_harness.h"
#include "../reactor_concept_fft.h"
#include <array>
#include <cmath>
#include <complex>
#include <numbers>
#include <vector>

namespace {
struct Runtime {
    void* handle{};
    ~Runtime() { if (handle) prometheus_reactor_runtime_destroy(handle); }
};
}

FACT(PrometheusReactor_Fft64IndependentDftPrecisionAndReuse) {
    Runtime runtime;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &runtime.handle), "create shared device");
    for (const uint32_t count : {1u, 2u, 8u, 32u, 128u}) {
        std::vector<double> input(2u * count), output(2u * count, -999.0);
        for (uint32_t i = 0; i < count; ++i) {
            // The low bits are deliberately lost by an FP32 input conversion.
            input[2 * i] = 1.0 + std::ldexp(static_cast<double>(i + 1), -35);
            input[2 * i + 1] = std::sin(static_cast<double>(i)) + std::ldexp(1.0, -40);
        }
        const auto original = input;
        uint32_t stage{}; int detail{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft64(runtime.handle, input.data(), output.data(), count, &stage, &detail), "binary64 FFT executes on Vulkan");
        ASSERT_EQUAL(0, detail, "GPU completion succeeds");
        ASSERT_TRUE(input == original, "input remains unchanged");
        for (uint32_t k = 0; k < count; ++k) {
            std::complex<long double> expected{};
            for (uint32_t sample = 0; sample < count; ++sample) {
                const long double angle = -2.0L * std::numbers::pi_v<long double> * k * sample / count;
                expected += std::complex<long double>{input[2 * sample], input[2 * sample + 1]} *
                    std::complex<long double>{std::cos(angle), std::sin(angle)};
            }
            const double tolerance = 2e-12 * count;
            ASSERT_NEAR(static_cast<double>(expected.real()), output[2 * k], tolerance, "independent DFT real bin");
            ASSERT_NEAR(static_cast<double>(expected.imag()), output[2 * k + 1], tolerance, "independent DFT imaginary bin");
        }
        uint64_t before{}, reused{}, after{}, reusedAfter{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_concept_fft_stats(runtime.handle, &before, &reused), "owner allocation stats");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft64(runtime.handle, input.data(), output.data(), count, &stage, &detail), "warm FFT reuses storage");
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_concept_fft_stats(runtime.handle, &after, &reusedAfter), "owner reuse stats");
        ASSERT_EQUAL(before, after, "warm call allocates no buffers");
        ASSERT_TRUE(reusedAfter > reused, "Concept reused the current storage");
    }
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime.handle, &services), "validation services");
    ASSERT_EQUAL(0u, services.validation_error_count, "FFT has no Vulkan validation errors");
    // Coexistence with the legacy SGEMM consumer on this same VkDevice/queue.
    const float a[4] = {1, 2, 3, 4}, b[4] = {5, 6, 7, 8}; float c[4]{};
    uint32_t stage{}; int detail{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(runtime.handle, a, b, c, 2, 2, 2, &stage, &detail), "legacy SGEMM still executes");
    ASSERT_NEAR(19.0f, c[0], 1e-5f, "legacy SGEMM result");
}

FACT(PrometheusReactor_Fft64UnavailableAndFailureDoNotWriteOutput) {
    const std::array<uint32_t, 7> flags = {PROM_TESTCFG_SKIP_VULKAN_INIT, PROM_TESTCFG_FAIL_BUFFER_ALLOC,
        PROM_TESTCFG_FAIL_UPLOAD, PROM_TESTCFG_FAIL_DISPATCH, PROM_TESTCFG_FAIL_DOWNLOAD,
        PROM_TESTCFG_FAIL_COMMAND_END, PROM_TESTCFG_FAIL_QUEUE_SUBMIT};
    for (uint32_t flag : flags) {
        PrometheusReactorConfig config{}; config.struct_size = sizeof config; config.test_flags = flag;
        Runtime runtime;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &runtime.handle), "create failure-configured runtime");
        const double input[4] = {1, 0, 2, 0}; double output[4] = {91, 92, 93, 94};
        uint32_t stage{}; int detail{};
        ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft64(runtime.handle, input, output, 2, &stage, &detail), "failed GPU call stays failed");
        ASSERT_TRUE(detail != 0, "failure retains a reason");
        ASSERT_EQUAL(91.0, output[0], "no CPU fallback or partial output");
        ASSERT_EQUAL(94.0, output[3], "destination tail remains untouched");
    }
}

FACT(PrometheusReactor_Fft64PreservesBinary64DynamicRange) {
    Runtime runtime;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &runtime.handle), "create binary64 device");
    for (double scale : {1e-200, 1e200}) {
        const double input[8] = {scale, -scale, 2*scale, scale, -scale, 0, 0, 2*scale};
        double output[8]{};
        uint32_t stage{}; int detail{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft64(runtime.handle, input, output, 4, &stage, &detail), "wide-range FFT executes");
        for (uint32_t k = 0; k < 4; ++k) {
            std::complex<double> expected{};
            for (uint32_t i = 0; i < 4; ++i) {
                const double angle = -2.0 * std::numbers::pi * k * i / 4;
                expected += std::complex<double>{input[2*i]/scale,input[2*i+1]/scale} * std::complex<double>{std::cos(angle),std::sin(angle)};
            }
            ASSERT_NEAR(expected.real(), output[2*k]/scale, 1e-12, "real result retains binary64 dynamic range");
            ASSERT_NEAR(expected.imag(), output[2*k+1]/scale, 1e-12, "imaginary result retains binary64 dynamic range");
        }
    }
}
