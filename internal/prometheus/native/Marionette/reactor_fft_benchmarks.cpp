#include "test_harness.h"
#include "../reactor_api.h"

#include <algorithm>
#include <array>
#include <chrono>
#include <cstdint>
#include <iostream>
#include <vector>

static void run_fft_benchmark_case(::marionette::tests::BenchmarkContext& context,
                                   const char* name,
                                   std::uint32_t elementCount,
                                   std::uint32_t batchCount)
{
    constexpr std::uint32_t sampleCount = 7u;
    constexpr std::uint32_t warmIterations = 8u;
    constexpr std::uint32_t measuredIterations = 64u;
    const std::uint32_t stride = elementCount;
    std::vector<PrometheusComplex32> input(elementCount * batchCount, {0.0f, 0.0f});
    std::vector<PrometheusComplex32> output(elementCount * batchCount, {0.0f, 0.0f});
    for (std::uint32_t batch = 0u; batch < batchCount; ++batch) input[batch * stride] = {1.0f, 0.0f};

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "benchmark runtime create must succeed");
    if (handle == nullptr) return;
    PrometheusFftRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.input = input.data(); request.output = output.data(); request.element_count = elementCount;
    request.batch_count = batchCount; request.stride_elements = 0u; request.flags = 0u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    const auto coldStart = std::chrono::steady_clock::now();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &request, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail),
                 "cold benchmark call must execute Vulkan FFT");
    const auto coldEnd = std::chrono::steady_clock::now();
    ASSERT_EQUAL(0, detail, "cold benchmark cannot count unavailable work");

    for (std::uint32_t i = 0u; i < warmIterations; ++i) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &request, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail),
                     "warm-up must execute Vulkan FFT");
    }

    std::array<double, sampleCount> nanosecondsPerTransform{};
    for (std::uint32_t sample = 0u; sample < sampleCount; ++sample) {
        const auto start = std::chrono::steady_clock::now();
        for (std::uint32_t iteration = 0u; iteration < measuredIterations; ++iteration) {
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &request, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail),
                         "timed benchmark iteration must execute Vulkan FFT");
        }
        const auto end = std::chrono::steady_clock::now();
        nanosecondsPerTransform[sample] = static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(end - start).count()) /
                                           static_cast<double>(measuredIterations);
        for (std::uint32_t batch = 0u; batch < batchCount; ++batch) {
            ASSERT_NEAR(1.0f, output[batch * stride].real, 2.0e-4f, "impulse checksum is verified outside the timed loop");
            ASSERT_NEAR(1.0f, output[batch * stride + elementCount - 1u].real, 2.0e-4f, "impulse final-bin checksum is verified outside the timed loop");
        }
    }
    std::sort(nanosecondsPerTransform.begin(), nanosecondsPerTransform.end());
    const double coldNanoseconds = static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(coldEnd - coldStart).count());
    const double medianNanoseconds = nanosecondsPerTransform[sampleCount / 2u];
    const double p90Nanoseconds = nanosecondsPerTransform[(sampleCount * 9u) / 10u - 1u];
    const double samplesPerSecond = static_cast<double>(elementCount) * static_cast<double>(batchCount) * 1.0e9 / medianNanoseconds;
    std::cout << "[FFT-BENCH] name=" << name
              << " N=" << elementCount
              << " direction=forward batch=" << batchCount
              << " stride=" << stride
              << " cold_e2e_ns=" << static_cast<std::uint64_t>(coldNanoseconds)
              << " warm_median_e2e_ns=" << static_cast<std::uint64_t>(medianNanoseconds)
              << " warm_p90_e2e_ns=" << static_cast<std::uint64_t>(p90Nanoseconds)
              << " samples_per_second=" << static_cast<std::uint64_t>(samplesPerSecond)
              << " includes=upload,readback,pipeline_setup"
              << " numerical=pass\n";
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "benchmark runtime destroy must succeed");
    (void)context;
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusReactor_Fft_Radix2Benchmark_N16, 1u)
{
    run_fft_benchmark_case(context, "radix2_n16", 16u, 1u);
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusReactor_Fft_Radix2Benchmark_N256_B4, 1u)
{
    run_fft_benchmark_case(context, "radix2_n256_b4", 256u, 4u);
}
