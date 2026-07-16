#include "test_harness.h"

#include "../reactor_api.h"

#include <cstdint>
#include <sstream>
#include <string>
#include <vector>

namespace
{
struct ReductionBenchCase {
    const char* name;
    std::uint32_t operation;
    std::uint32_t rows;
    std::uint32_t width;
    std::uint32_t flags;
};

bool runtime_available(void* handle)
{
    PrometheusCaps caps{};
    return prometheus_reactor_runtime_probe(handle, &caps) == PROM_OK && caps.available != 0u;
}

const char* operation_name(std::uint32_t operation)
{
    if (operation == PROM_REDUCTION_OPERATION_SUM) return "sum";
    if (operation == PROM_REDUCTION_OPERATION_MAX) return "max";
    return "softmax";
}

std::vector<float> benchmark_input(const ReductionBenchCase& bench_case)
{
    std::vector<float> input(static_cast<std::size_t>(bench_case.rows) * bench_case.width);
    for (std::uint32_t row = 0u; row < bench_case.rows; ++row) {
        for (std::uint32_t column = 0u; column < bench_case.width; ++column) {
            const int centered = static_cast<int>((row * 13u + column * 37u) % 127u) - 63;
            float value = static_cast<float>(centered) / 32.0f;
            if (bench_case.operation == PROM_REDUCTION_OPERATION_SOFTMAX) value += 8000.0f;
            input[static_cast<std::size_t>(row) * bench_case.width + column] = value;
        }
    }
    return input;
}

bool run_benchmark_case(void* handle,
                        const ReductionBenchCase& bench_case,
                        PrometheusReductionBenchmarkResult& result)
{
    std::vector<float> input = benchmark_input(bench_case);
    const std::uint64_t output_count = bench_case.operation == PROM_REDUCTION_OPERATION_SOFTMAX
        ? static_cast<std::uint64_t>(bench_case.rows) * bench_case.width
        : bench_case.rows;
    std::vector<float> output(static_cast<std::size_t>(output_count), 0.0f);
    PrometheusReductionBenchmarkRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.reduction.struct_size = static_cast<std::uint32_t>(sizeof(request.reduction));
    request.reduction.input = input.data();
    request.reduction.output = output.data();
    request.reduction.row_count = bench_case.rows;
    request.reduction.elements_per_row = bench_case.width;
    request.reduction.input_element_count = static_cast<std::uint64_t>(bench_case.rows) * bench_case.width;
    request.reduction.output_element_count = output_count;
    request.reduction.operation = bench_case.operation;
    request.reduction.finalization = bench_case.operation == PROM_REDUCTION_OPERATION_SOFTMAX
        ? PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX
        : PROM_REDUCTION_FINALIZATION_NONE;
    request.reduction.flags = bench_case.flags;
    request.warmup_iterations = 2u;
    request.measured_iterations = 7u;
    return prometheus_reactor_runtime_reduction_benchmark(handle, &request, &result) == PROM_OK &&
           result.correctness_passed != 0u && result.validation_passed != 0u &&
           result.completed_iterations == request.measured_iterations && result.gpu_median_ns > 0u;
}

void append_result(std::ostringstream& json,
                   const ReductionBenchCase& bench_case,
                   const PrometheusReductionBenchmarkResult& result,
                   bool& first)
{
    if (!first) json << ',';
    first = false;
    json << "{\"name\":\"" << bench_case.name << "\",\"operation\":\"" << operation_name(bench_case.operation)
         << "\",\"rows\":" << bench_case.rows << ",\"width\":" << bench_case.width
         << ",\"flags\":" << bench_case.flags << ",\"stage_count\":" << result.stage_count
         << ",\"temporary_bytes\":" << result.temporary_bytes << ",\"replay_id\":" << result.replay_id
         << ",\"gpu_ns\":{\"min\":" << result.gpu_min_ns << ",\"median\":" << result.gpu_median_ns
         << ",\"max\":" << result.gpu_max_ns << "},\"end_to_end_ns\":{\"min\":" << result.end_to_end_min_ns
         << ",\"median\":" << result.end_to_end_median_ns << ",\"max\":" << result.end_to_end_max_ns
         << "},\"correctness\":true,\"validation\":true,\"device_lost\":" << result.device_lost << '}';
}
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusReduction_M39bRtx3070Corpus, 1)
{
    (void)context.iteration;
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.reduction_ring_depth = 2u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "benchmark runtime create must succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable benchmark runtime cleanup must succeed");
        SKIP("Vulkan unavailable; M39b benchmark corpus cannot execute");
    }
    PrometheusVulkanDeviceDiagnostics device{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_vulkan_device_diagnostics(handle, &device),
                 "benchmark must capture Vulkan device identity");
    if (device.software_vulkan != 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "software runtime cleanup must succeed");
        SKIP("Software Vulkan is not accepted as M39b GPU benchmark evidence");
    }

    const ReductionBenchCase corpus[] = {
        {"sum_r1_w32", PROM_REDUCTION_OPERATION_SUM, 1u, 32u, 0u},
        {"sum_r16_w127", PROM_REDUCTION_OPERATION_SUM, 16u, 127u, 0u},
        {"sum_r128_w512", PROM_REDUCTION_OPERATION_SUM, 128u, 512u, 0u},
        {"sum_r1024_w128", PROM_REDUCTION_OPERATION_SUM, 1024u, 128u, 0u},
        {"sum_r16_w513", PROM_REDUCTION_OPERATION_SUM, 16u, 513u, 0u},
        {"sum_r128_w4096", PROM_REDUCTION_OPERATION_SUM, 128u, 4096u, 0u},
        {"max_r1_w32", PROM_REDUCTION_OPERATION_MAX, 1u, 32u, 0u},
        {"max_r16_w127", PROM_REDUCTION_OPERATION_MAX, 16u, 127u, 0u},
        {"max_r128_w512", PROM_REDUCTION_OPERATION_MAX, 128u, 512u, 0u},
        {"max_r1024_w128", PROM_REDUCTION_OPERATION_MAX, 1024u, 128u, 0u},
        {"max_r16_w513", PROM_REDUCTION_OPERATION_MAX, 16u, 513u, 0u},
        {"max_r128_w4096", PROM_REDUCTION_OPERATION_MAX, 128u, 4096u, 0u},
        {"softmax_r1_w64", PROM_REDUCTION_OPERATION_SOFTMAX, 1u, 64u, 0u},
        {"softmax_r16_w128", PROM_REDUCTION_OPERATION_SOFTMAX, 16u, 128u, 0u},
        {"softmax_r128_w256", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 256u, 0u},
        {"softmax_r1024_w512", PROM_REDUCTION_OPERATION_SOFTMAX, 1024u, 512u, 0u},
        {"softmax_r128_w1024", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 1024u, 0u},
        {"softmax_r16_w4096", PROM_REDUCTION_OPERATION_SOFTMAX, 16u, 4096u, 0u},
    };
    const ReductionBenchCase strategy_cases[] = {
        {"softmax_fused_r128_w64", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 64u, PROM_REDUCTION_FLAG_FORCE_FUSED},
        {"softmax_composed_r128_w64", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 64u, PROM_REDUCTION_FLAG_FORCE_COMPOSED},
        {"softmax_fused_r128_w128", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 128u, PROM_REDUCTION_FLAG_FORCE_FUSED},
        {"softmax_composed_r128_w128", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 128u, PROM_REDUCTION_FLAG_FORCE_COMPOSED},
        {"softmax_fused_r128_w256", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 256u, PROM_REDUCTION_FLAG_FORCE_FUSED},
        {"softmax_composed_r128_w256", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 256u, PROM_REDUCTION_FLAG_FORCE_COMPOSED},
        {"softmax_fused_r128_w512", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 512u, PROM_REDUCTION_FLAG_FORCE_FUSED},
        {"softmax_composed_r128_w512", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 512u, PROM_REDUCTION_FLAG_FORCE_COMPOSED},
        {"softmax_fused_r128_w1024", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 1024u, PROM_REDUCTION_FLAG_FORCE_FUSED},
        {"softmax_composed_r128_w1024", PROM_REDUCTION_OPERATION_SOFTMAX, 128u, 1024u, PROM_REDUCTION_FLAG_FORCE_COMPOSED},
    };

    std::ostringstream json;
    json << "{\"milestone\":\"M39b\",\"device\":{\"name\":\"" << device.device_name
         << "\",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id
         << ",\"driver_version\":" << device.driver_version << ",\"api_version\":" << device.api_version
         << ",\"software_vulkan\":" << (device.software_vulkan != 0u ? "true" : "false")
         << ",\"compute_queue_family\":" << device.compute_queue_family << "},\"results\":[";
    bool first = true;
    for (const ReductionBenchCase& bench_case : corpus) {
        PrometheusReductionBenchmarkResult result{};
        ASSERT_TRUE(run_benchmark_case(handle, bench_case, result), "every bounded corpus case must be correct, timed, and validation clean");
        append_result(json, bench_case, result, first);
    }
    for (const ReductionBenchCase& bench_case : strategy_cases) {
        PrometheusReductionBenchmarkResult result{};
        ASSERT_TRUE(run_benchmark_case(handle, bench_case, result), "every strategy comparison must be correct, timed, and validation clean");
        append_result(json, bench_case, result, first);
    }
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_reduction_diagnostics(handle, &diagnostics),
                 "benchmark diagnostics must be readable");
    json << "],\"ring\":{\"depth\":" << diagnostics.configured_ring_depth
         << ",\"pipelines_created\":" << diagnostics.pipeline_create_count
         << ",\"buffer_allocations\":" << diagnostics.buffer_allocation_count
         << ",\"buffer_reuses\":" << diagnostics.buffer_reuse_count
         << ",\"temporary_capacity_bytes\":" << diagnostics.temporary_capacity_bytes
         << ",\"validation_errors\":" << diagnostics.validation_error_count
         << "},\"device_loss\":false}";
    ASSERT_EQUAL(0u, diagnostics.validation_error_count, "benchmark corpus must end with zero validation errors");
    ASSERT_EQUAL(static_cast<uint64_t>(5u), diagnostics.pipeline_create_count, "benchmark corpus must reuse five pre-created pipelines");
    ASSERT_TRUE(context.WriteTextArtifact("m39b_reduction_benchmark.json", json.str()),
                "benchmark evidence JSON must be persisted");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "benchmark runtime destroy must succeed");
}
