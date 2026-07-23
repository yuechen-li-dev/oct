#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <filesystem>
#include <iomanip>
#include <limits>
#include <sstream>
#include <string>
#include <string_view>
#include <vector>

namespace
{
bool runtime_available(void* handle)
{
    PrometheusCaps caps{};
    return prometheus_reactor_runtime_probe(handle, &caps) == PROM_OK && caps.available != 0u;
}

std::uint64_t output_count(std::uint32_t rows, std::uint32_t width, std::uint32_t operation)
{
    return operation == PROM_REDUCTION_OPERATION_SOFTMAX
        ? static_cast<std::uint64_t>(rows) * width
        : rows;
}

PrometheusReductionRequest make_request(const std::vector<float>& input,
                                        std::vector<float>& output,
                                        std::uint32_t rows,
                                        std::uint32_t width,
                                        std::uint32_t operation,
                                        std::uint32_t flags = 0u)
{
    PrometheusReductionRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.input = input.data();
    request.output = output.data();
    request.row_count = rows;
    request.elements_per_row = width;
    request.input_element_count = static_cast<std::uint64_t>(rows) * width;
    request.output_element_count = output_count(rows, width, operation);
    request.operation = operation;
    request.finalization = operation == PROM_REDUCTION_OPERATION_SOFTMAX
        ? PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX
        : PROM_REDUCTION_FINALIZATION_NONE;
    request.flags = flags;
    return request;
}

PrometheusRowWiseSoftmaxRequest make_row_wise_softmax_request(const std::vector<float>& input,
                                                               std::vector<float>& output,
                                                               std::uint32_t rows,
                                                               std::uint32_t width)
{
    PrometheusRowWiseSoftmaxRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.input = input.data();
    request.output = output.data();
    request.row_count = rows;
    request.elements_per_row = width;
    request.input_element_count = static_cast<std::uint64_t>(rows) * width;
    request.output_element_count = static_cast<std::uint64_t>(rows) * width;
    return request;
}

std::string staged_fr_m0_package_root()
{
    return (std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) / "out" / "prometheus" / "native" /
            "SerialCanonical" / "shaders").string();
}

std::vector<float> deterministic_input(std::uint32_t rows, std::uint32_t width, float bias = 0.0f)
{
    std::vector<float> values(static_cast<std::size_t>(rows) * width);
    for (std::uint32_t row = 0u; row < rows; ++row) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            const int centered = static_cast<int>((row * 29u + column * 17u) % 101u) - 50;
            values[static_cast<std::size_t>(row) * width + column] = bias + static_cast<float>(centered) / 16.0f;
        }
    }
    return values;
}

std::vector<float> fr_m0_input(std::uint32_t rows,
                                std::uint32_t width,
                                std::string_view distribution,
                                float bias = 0.0f)
{
    std::vector<float> values = deterministic_input(rows, width, bias);
    for (std::uint32_t row = 0u; row < rows; ++row) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            const std::size_t index = static_cast<std::size_t>(row) * width + column;
            if (distribution == "constant") {
                values[index] = 3.25f;
            } else if (distribution == "positive-extreme") {
                values[index] = 10000.0f + static_cast<float>(static_cast<int>(column % 17u) - 8);
            } else if (distribution == "negative-extreme") {
                values[index] = -10000.0f + static_cast<float>(static_cast<int>(column % 17u) - 8);
            } else if (distribution == "heterogeneous") {
                const std::uint32_t mode = row % 4u;
                if (mode == 0u) values[index] = 2.0f;
                else if (mode == 1u) values[index] = static_cast<float>(static_cast<int>(column % 101u) - 50) / 8.0f;
                else if (mode == 2u) values[index] = 10000.0f + static_cast<float>(static_cast<int>(column % 17u) - 8);
                else values[index] = -10000.0f + static_cast<float>(static_cast<int>(column % 17u) - 8);
            }
        }
    }
    return values;
}

struct FrM0NumericalMetrics
{
    double maximum_absolute_error = 0.0;
    double maximum_relative_error = 0.0;
    double maximum_row_sum_error = 0.0;
    double maximum_shift_invariance_error = 0.0;
    double minimum_output = std::numeric_limits<double>::infinity();
    std::uint64_t ordering_violations = 0u;
};

void observe_fr_m0_metrics(const std::vector<float>& input,
                            const std::vector<float>& expected,
                            const std::vector<float>& actual,
                            std::uint32_t rows,
                            std::uint32_t width,
                            FrM0NumericalMetrics& metrics)
{
    for (std::uint32_t row = 0u; row < rows; ++row) {
        double row_sum = 0.0;
        const std::size_t base = static_cast<std::size_t>(row) * width;
        for (std::uint32_t column = 0u; column < width; ++column) {
            const std::size_t index = base + column;
            const double difference = std::fabs(static_cast<double>(actual[index]) - expected[index]);
            const double relative_scale = std::max(std::fabs(static_cast<double>(expected[index])), 1.0e-6);
            metrics.maximum_absolute_error = std::max(metrics.maximum_absolute_error, difference);
            metrics.maximum_relative_error = std::max(metrics.maximum_relative_error, difference / relative_scale);
            metrics.minimum_output = std::min(metrics.minimum_output, static_cast<double>(actual[index]));
            row_sum += actual[index];
        }
        metrics.maximum_row_sum_error = std::max(metrics.maximum_row_sum_error, std::fabs(row_sum - 1.0));
        for (std::uint32_t left = 0u; left < width; ++left) {
            for (std::uint32_t right = left + 1u; right < width; ++right) {
                const std::size_t left_index = base + left;
                const std::size_t right_index = base + right;
                if (input[left_index] < input[right_index] && actual[left_index] > actual[right_index] + 2.0e-6f) {
                    ++metrics.ordering_violations;
                }
                if (input[right_index] < input[left_index] && actual[right_index] > actual[left_index] + 2.0e-6f) {
                    ++metrics.ordering_violations;
                }
            }
        }
    }
}

bool execute_fr_m0_and_compare(void* handle,
                                const std::vector<float>& input,
                                std::vector<float>& output,
                                std::uint32_t rows,
                                std::uint32_t width,
                                FrM0NumericalMetrics& metrics,
                                PrometheusRowWiseSoftmaxResult& result)
{
    std::vector<float> expected(input.size(), 0.0f);
    PrometheusRowWiseSoftmaxRequest request = make_row_wise_softmax_request(input, output, rows, width);
    PrometheusReductionRequest authority = make_request(input, expected, rows, width, PROM_REDUCTION_OPERATION_SOFTMAX);
    PrometheusReductionBenchmarkResult comparison{};
    int detail = 0;
    comparison.struct_size = static_cast<std::uint32_t>(sizeof(comparison));
    if (prom_reduction_cpu_reference(&authority, expected.data(), &detail) != PROM_OK) return false;
    if (prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result) != PROM_OK) return false;
    if (prom_reduction_compare(&authority, expected.data(), output.data(), &comparison) != PROM_OK) return false;
    observe_fr_m0_metrics(input, expected, output, rows, width, metrics);
    return true;
}

bool execute_and_compare(void* handle,
                         const std::vector<float>& input,
                         std::vector<float>& output,
                         std::uint32_t rows,
                         std::uint32_t width,
                         std::uint32_t operation,
                         std::uint32_t flags,
                         PrometheusReductionExecutionResult& execution,
                         PrometheusReductionBenchmarkResult& comparison)
{
    PrometheusReductionRequest request = make_request(input, output, rows, width, operation, flags);
    std::vector<float> expected(output.size(), 0.0f);
    int detail = 0;
    if (prom_reduction_cpu_reference(&request, expected.data(), &detail) != PROM_OK) return false;
    if (prometheus_reactor_runtime_reduction(handle, &request, &execution) != PROM_OK) return false;
    comparison = {};
    comparison.struct_size = static_cast<std::uint32_t>(sizeof(comparison));
    comparison.first_mismatch_row = std::numeric_limits<std::uint32_t>::max();
    comparison.first_mismatch_column = std::numeric_limits<std::uint32_t>::max();
    return prom_reduction_compare(&request, expected.data(), output.data(), &comparison) == PROM_OK;
}
}

FACT(PrometheusReduction_PlannerCoversRequiredBoundariesDeterministically)
{
    const std::uint32_t widths[] = {
        1u, 2u, 3u, 31u, 32u, 33u, 63u, 64u, 65u, 127u, 128u, 129u,
        255u, 256u, 257u, 511u, 512u, 513u, 1024u, 4096u,
    };
    float input_value = 1.0f;
    float output_value = 0.0f;
    for (const std::uint32_t width : widths) {
        PrometheusReductionRequest sum{};
        sum.struct_size = static_cast<std::uint32_t>(sizeof(sum));
        sum.input = &input_value;
        sum.output = &output_value;
        sum.row_count = 1u;
        sum.elements_per_row = width;
        sum.input_element_count = width;
        sum.output_element_count = 1u;
        sum.operation = PROM_REDUCTION_OPERATION_SUM;
        sum.finalization = PROM_REDUCTION_FINALIZATION_NONE;
        PrometheusReductionPlan first{};
        PrometheusReductionPlan replay{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&sum, &first), "sum boundary width must plan");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&sum, &replay), "identical sum request must replan");
        ASSERT_EQUAL(first.replay_id, replay.replay_id, "identical plans must have stable replay identity");
        ASSERT_EQUAL(width > 1024u ? 2u : 1u, first.stage_count, "sum stage count must cross only at width 1024");
        ASSERT_EQUAL((width + 1023u) / 1024u, first.partial_count, "partial count must use complete ceil division");
        ASSERT_EQUAL(4u, first.temporary_alignment_bytes, "temporary values must have FP32 alignment");
        ASSERT_EQUAL(PROM_OK,
                     prom_reduction_validate_plan_for_test(&first, first.temporary_bytes, nullptr),
                     "emitted sum plan must pass structural and capacity validation");

        PrometheusReductionRequest softmax = sum;
        softmax.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
        softmax.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
        softmax.output_element_count = width;
        PrometheusReductionPlan softmax_plan{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&softmax, &softmax_plan), "softmax boundary width must plan");
        ASSERT_EQUAL(width > 1024u ? 5u : 1u, softmax_plan.stage_count, "softmax must use fused or explicit five-stage plan");
        ASSERT_EQUAL(width > 1024u ? static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_COMPOSED)
                                   : (width <= PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX
                                          ? static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS)
                                          : static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP)),
                     softmax_plan.strategy,
                     "softmax selector must preserve packed-short, fused, and staged boundaries");
    }

    const std::uint32_t row_counts[] = {1u, 2u, 16u, 128u, 1024u};
    for (const std::uint32_t rows : row_counts) {
        PrometheusReductionRequest request{};
        request.struct_size = static_cast<std::uint32_t>(sizeof(request));
        request.input = &input_value;
        request.output = &output_value;
        request.row_count = rows;
        request.elements_per_row = 513u;
        request.input_element_count = static_cast<std::uint64_t>(rows) * 513u;
        request.output_element_count = rows;
        request.operation = PROM_REDUCTION_OPERATION_MAX;
        request.finalization = PROM_REDUCTION_FINALIZATION_NONE;
        PrometheusReductionPlan plan{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&request, &plan), "required row count must plan");
        ASSERT_EQUAL(rows, plan.stages[0].groups_x, "single-workgroup max must dispatch one group per row");
    }

    for (const std::uint32_t operation : {PROM_REDUCTION_OPERATION_SUM, PROM_REDUCTION_OPERATION_SOFTMAX}) {
        PrometheusReductionRequest packed{};
        packed.struct_size = static_cast<std::uint32_t>(sizeof(packed));
        packed.input = &input_value;
        packed.output = &output_value;
        packed.row_count = 512u;
        packed.elements_per_row = 64u;
        packed.input_element_count = 512u * 64u;
        packed.output_element_count = operation == PROM_REDUCTION_OPERATION_SOFTMAX ? 512u * 64u : 512u;
        packed.operation = operation;
        packed.finalization = operation == PROM_REDUCTION_OPERATION_SOFTMAX
            ? PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX
            : PROM_REDUCTION_FINALIZATION_NONE;
        PrometheusReductionPlan plan{};
        PrometheusReductionPlan replay{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&packed, &plan), "packed short-row request must plan");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&packed, &replay), "packed short-row request must replay");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS), plan.strategy,
                     "short high-row case must select packed plan");
        ASSERT_EQUAL(64u, plan.stages[0].groups_x, "eight short rows must share one workgroup");
        ASSERT_EQUAL(plan.replay_id, replay.replay_id, "packed plan replay must be stable");
        ASSERT_EQUAL(PROM_OK, prom_reduction_validate_plan_for_test(&plan, plan.temporary_bytes, nullptr),
                     "packed plan must retain normal structural validation");

        packed.row_count = 1u;
        packed.input_element_count = 64u;
        packed.output_element_count = operation == PROM_REDUCTION_OPERATION_SOFTMAX ? 64u : 1u;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&packed, &plan), "decode request must plan");
        if (operation == PROM_REDUCTION_OPERATION_SUM) {
            ASSERT_TRUE(plan.strategy != PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS,
                        "few-row sum must retain the wide-row plan");
        } else {
            ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS), plan.strategy,
                         "short-width stable softmax must select the packed plan");
        }
    }
}

FACT(PrometheusReduction_PlannerRejectsMalformedSizesAndTemporaryCapacity)
{
    std::vector<float> input(4096u, 1.0f);
    std::vector<float> output(1u, 0.0f);
    PrometheusReductionRequest request = make_request(input, output, 1u, 4096u, PROM_REDUCTION_OPERATION_SUM);
    PrometheusReductionPlan plan{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_reduction_plan(&request, &plan), "large sum must produce a staged plan");
    ASSERT_TRUE(plan.temporary_bytes > 0u, "staged sum must declare temporary bytes");
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR,
                 prom_reduction_validate_plan_for_test(&plan, plan.temporary_bytes - 1u, &detail),
                 "temporary capacity one byte short must be rejected");
    ASSERT_EQUAL(PROM_REDUCTION_DETAIL_TEMPORARY_UNDERSIZED, detail, "undersized temporary detail must be stable");
    plan.stages[0].groups_x = 0u;
    detail = 0;
    ASSERT_EQUAL(PROM_ERROR,
                 prom_reduction_validate_plan_for_test(&plan, plan.temporary_bytes, &detail),
                 "malformed stage geometry must be rejected");
    ASSERT_EQUAL(PROM_REDUCTION_DETAIL_MALFORMED_PLAN, detail, "malformed plan detail must be stable");

    request.elements_per_row = 0u;
    request.input_element_count = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_reduction_plan(&request, &plan), "zero row width must be invalid");
    request = make_request(input, output, 1u, 4096u, PROM_REDUCTION_OPERATION_SUM);
    request.input_element_count -= 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_reduction_plan(&request, &plan), "input resource size mismatch must be invalid");
    request = make_request(input, output, 1u, 4096u, PROM_REDUCTION_OPERATION_SUM);
    request.output_element_count = 2u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_reduction_plan(&request, &plan), "output resource size mismatch must be invalid");
}

FACT(PrometheusReduction_NonfinitePolicyIsExplicitAndStable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create must succeed for validation tests");
    const float nonfinite_values[] = {
        std::numeric_limits<float>::quiet_NaN(),
        std::numeric_limits<float>::infinity(),
        -std::numeric_limits<float>::infinity(),
    };
    for (const float nonfinite : nonfinite_values) {
        std::vector<float> input = {1.0f, nonfinite, 2.0f};
        std::vector<float> output(3u, 0.0f);
        PrometheusReductionRequest request = make_request(input, output, 1u, 3u, PROM_REDUCTION_OPERATION_SOFTMAX);
        PrometheusReductionExecutionResult execution{};
        ASSERT_EQUAL(PROM_ERROR,
                     prometheus_reactor_runtime_reduction(handle, &request, &execution),
                     "NaN and infinity must be rejected before GPU dispatch");
        ASSERT_EQUAL(PROM_REDUCTION_DETAIL_NONFINITE_INPUT, execution.detail_code, "nonfinite detail must be stable");
        ASSERT_EQUAL(static_cast<uint64_t>(1u), execution.first_nonfinite_index, "first nonfinite input index must be reported");
        int detail = 0;
        ASSERT_EQUAL(PROM_ERROR,
                     prom_reduction_cpu_reference(&request, output.data(), &detail),
                     "CPU oracle must enforce the same finite-input contract");
        ASSERT_EQUAL(PROM_REDUCTION_DETAIL_NONFINITE_INPUT, detail, "CPU nonfinite detail must match runtime policy");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "validation runtime destroy must succeed");
}

FACT(PrometheusFrM0_RowWiseSoftmaxBoundaryRejectsNonfiniteAndPreservesOutput)
{
    void* handle = nullptr;
    std::vector<float> input = {1.0f, std::numeric_limits<float>::infinity(), 2.0f};
    std::vector<float> output = {-7.0f, -7.0f, -7.0f};
    PrometheusRowWiseSoftmaxRequest request = make_row_wise_softmax_request(input, output, 1u, 3u);
    PrometheusRowWiseSoftmaxResult result{};
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "positive infinity must be rejected before dispatch");
    ASSERT_EQUAL(PROM_SOFTMAX_DETAIL_NONFINITE_INPUT, result.detail_code, "nonfinite diagnostic must be stable");
    ASSERT_EQUAL(static_cast<uint64_t>(1u), result.first_nonfinite_index, "first nonfinite index must be reported");
    ASSERT_EQUAL(0u, result.output_written, "failed execution must not claim output freshness");
    for (const float value : output) ASSERT_EQUAL(-7.0f, value, "failed execution must preserve caller output");

    input[1] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "NaN must be rejected before dispatch");
    ASSERT_EQUAL(PROM_SOFTMAX_DETAIL_NONFINITE_INPUT, result.detail_code, "NaN detail must be stable");
    input[1] = -std::numeric_limits<float>::infinity();
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "negative infinity must be rejected before dispatch");
    ASSERT_EQUAL(PROM_SOFTMAX_DETAIL_NONFINITE_INPUT, result.detail_code, "negative infinity detail must be stable");

    request.row_count = 0u;
    request.elements_per_row = 3u;
    request.input = nullptr;
    request.output = nullptr;
    request.input_element_count = 0u;
    request.output_element_count = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "zero-row operation must be a synchronous no-dispatch no-op");
    ASSERT_EQUAL(0u, result.physical_dispatch_count, "zero-row operation must record no dispatch");
    ASSERT_EQUAL(0u, result.physical_submission_count, "zero-row operation must submit nothing");

    std::vector<float> overlap = {1.0f, 2.0f, 3.0f, 4.0f};
    request = {};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.input = overlap.data();
    request.output = overlap.data() + 1u;
    request.row_count = 1u;
    request.elements_per_row = 3u;
    request.input_element_count = 3u;
    request.output_element_count = 3u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "partial alias must be rejected");
    ASSERT_EQUAL(PROM_SOFTMAX_DETAIL_PARTIAL_ALIAS, result.detail_code, "partial alias detail must be stable");
    std::vector<float> valid_output(3u, 0.0f);
    request = make_row_wise_softmax_request(overlap, valid_output, 1u, 3u);
    ASSERT_EQUAL(PROM_INVALID_HANDLE, prometheus_reactor_runtime_row_wise_softmax(handle, &request, &result),
                 "invalid handles must be rejected after semantic validation");
}

FACT(PrometheusFrM0_RowWiseSoftmaxUsesOneWorkgroupOwnedDispatchPerBatchedCall)
{
    void* handle = nullptr;
    const std::string package_root = staged_fr_m0_package_root();
    ASSERT_TRUE(std::filesystem::exists(std::filesystem::path(package_root) / "manifest.json"),
                "FR-M0 GPU authority must use the staged external package");
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.shader_package_root = package_root.c_str();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle),
                 "FR-M0 GPU authority must create a package-backed runtime");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime cleanup must succeed");
        SKIP("Vulkan unavailable; FR-M0 GPU route cannot execute");
    }
    FrM0NumericalMetrics metrics{};
    PrometheusReductionDiagnostics after_small{};
    PrometheusReductionDiagnostics after_growth{};
    PrometheusReductionDiagnostics settled{};
    PrometheusReductionDiagnostics reused{};
    const auto run_case = [&](std::uint32_t rows, std::uint32_t width, std::string_view distribution, float bias = 0.0f) {
        std::vector<float> input = fr_m0_input(rows, width, distribution, bias);
        std::vector<float> output(input.size(), -99.0f);
        PrometheusRowWiseSoftmaxResult result{};
        ASSERT_TRUE(execute_fr_m0_and_compare(handle, input, output, rows, width, metrics, result),
                    "FR-M0 must agree elementwise with the independent double-precision CPU authority");
        ASSERT_EQUAL(1u, result.output_written, "successful FR-M0 execution must report fresh output");
        ASSERT_EQUAL(1u, result.physical_dispatch_count, "all rows must be batched into one physical dispatch");
        ASSERT_EQUAL(1u, result.physical_submission_count, "one FR-M0 call must use one synchronous submission");
        return output;
    };

    (void)run_case(1u, 1u, "mixed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &after_small),
                 "small FR-M0 diagnostics must be available");
    (void)run_case(3u, 31u, "mixed");
    (void)run_case(3u, 32u, "mixed");
    (void)run_case(4u, 33u, "constant");
    (void)run_case(3u, 64u, "mixed");
    (void)run_case(3u, 129u, "mixed");
    (void)run_case(3u, 256u, "mixed");
    (void)run_case(3u, 257u, "mixed");
    (void)run_case(3u, 1023u, "mixed");
    (void)run_case(3u, 1024u, "positive-extreme");
    (void)run_case(3u, 1025u, "negative-extreme");
    (void)run_case(3u, 1055u, "mixed");
    (void)run_case(5u, 1056u, "heterogeneous");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &after_growth),
                 "growth diagnostics must be available");
    ASSERT_TRUE(after_growth.buffer_allocation_count > after_small.buffer_allocation_count,
                "larger admitted rows must grow reusable Vulkan buffer capacity");
    ASSERT_TRUE(after_growth.descriptor_update_count > after_small.descriptor_update_count,
                "capacity growth must rebind the production descriptors");

    // Grow both persistent slots to the maximum admitted FR-M0 shape before proving reuse.
    (void)run_case(5u, 1056u, "heterogeneous", 0.25f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &settled),
                 "settled capacity diagnostics must be available");
    (void)run_case(1u, 1u, "constant");
    (void)run_case(5u, 1056u, "heterogeneous", -0.25f);
    (void)run_case(1u, 1u, "mixed", 0.5f);
    (void)run_case(5u, 1056u, "heterogeneous", -0.5f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &reused),
                 "reuse diagnostics must be available");
    ASSERT_EQUAL(settled.buffer_allocation_count, reused.buffer_allocation_count,
                 "alternating small and large calls must not allocate after capacity settles");
    ASSERT_TRUE(reused.buffer_reuse_count > settled.buffer_reuse_count,
                "alternating calls must reuse the persistent Vulkan buffers");
    ASSERT_EQUAL(0u, reused.validation_error_count, "FR-M0 RTX corpus must remain Vulkan-validation clean");

    std::vector<float> shift_input = fr_m0_input(3u, 257u, "mixed");
    std::vector<float> shift_output(shift_input.size(), -99.0f);
    PrometheusRowWiseSoftmaxResult shift_result{};
    ASSERT_TRUE(execute_fr_m0_and_compare(handle, shift_input, shift_output, 3u, 257u, metrics, shift_result),
                "unshifted shift-invariance authority input must execute");
    std::vector<float> shifted_input = shift_input;
    for (float& value : shifted_input) value += 17.25f;
    std::vector<float> shifted_output(shifted_input.size(), -99.0f);
    PrometheusRowWiseSoftmaxResult shifted_result{};
    ASSERT_TRUE(execute_fr_m0_and_compare(handle, shifted_input, shifted_output, 3u, 257u, metrics, shifted_result),
                "shifted shift-invariance authority input must execute");
    for (std::size_t index = 0u; index < shift_output.size(); ++index) {
        metrics.maximum_shift_invariance_error = std::max(metrics.maximum_shift_invariance_error,
                                                          std::fabs(static_cast<double>(shift_output[index]) - shifted_output[index]));
    }

    std::vector<float> in_place = fr_m0_input(3u, 1056u, "heterogeneous");
    const std::vector<float> in_place_snapshot = in_place;
    std::vector<float> in_place_expected(in_place.size(), 0.0f);
    PrometheusReductionRequest in_place_authority = make_request(in_place_snapshot, in_place_expected, 3u, 1056u,
                                                                  PROM_REDUCTION_OPERATION_SOFTMAX);
    int in_place_detail = 0;
    ASSERT_EQUAL(PROM_OK, prom_reduction_cpu_reference(&in_place_authority, in_place_expected.data(), &in_place_detail),
                 "in-place input must be accepted by the independent CPU authority");
    PrometheusRowWiseSoftmaxRequest in_place_request = make_row_wise_softmax_request(in_place, in_place, 3u, 1056u);
    PrometheusRowWiseSoftmaxResult in_place_result{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_row_wise_softmax(handle, &in_place_request, &in_place_result),
                 "exact in-place FR-M0 execution must be admitted");
    ASSERT_EQUAL(1u, in_place_result.output_written, "in-place execution must report fresh output");
    ASSERT_EQUAL(1u, in_place_result.physical_dispatch_count, "in-place rows remain one batched dispatch");
    ASSERT_EQUAL(1u, in_place_result.physical_submission_count, "in-place rows remain one submission");
    PrometheusReductionBenchmarkResult in_place_comparison{};
    in_place_comparison.struct_size = static_cast<std::uint32_t>(sizeof(in_place_comparison));
    ASSERT_EQUAL(PROM_OK, prom_reduction_compare(&in_place_authority, in_place_expected.data(), in_place.data(), &in_place_comparison),
                 "exact in-place output must agree with the independent CPU authority");
    observe_fr_m0_metrics(in_place_snapshot, in_place_expected, in_place, 3u, 1056u, metrics);

    std::vector<float> rejected_input = {1.0f, std::numeric_limits<float>::quiet_NaN(), 2.0f};
    std::vector<float> rejected_output = {-7.0f, -7.0f, -7.0f};
    PrometheusRowWiseSoftmaxRequest rejected_request = make_row_wise_softmax_request(rejected_input, rejected_output, 1u, 3u);
    PrometheusRowWiseSoftmaxResult rejected_result{};
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &rejected_request, &rejected_result),
                 "nonfinite input must be rejected before an admitted runtime executes");
    ASSERT_EQUAL(0u, rejected_result.output_written, "failed pre-transfer validation must not claim fresh output");
    for (const float value : rejected_output) ASSERT_EQUAL(-7.0f, value, "failed validation must preserve caller output");
    std::vector<float> partial_alias = {1.0f, 2.0f, 3.0f, 4.0f};
    PrometheusRowWiseSoftmaxRequest partial_request{};
    partial_request.struct_size = static_cast<std::uint32_t>(sizeof(partial_request));
    partial_request.input = partial_alias.data();
    partial_request.output = partial_alias.data() + 1u;
    partial_request.row_count = 1u;
    partial_request.elements_per_row = 3u;
    partial_request.input_element_count = 3u;
    partial_request.output_element_count = 3u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_row_wise_softmax(handle, &partial_request, &rejected_result),
                 "partial aliasing must remain rejected on the admitted runtime");
    (void)run_case(3u, 257u, "mixed", 1.0f);

    ASSERT_TRUE(metrics.minimum_output >= -1.0e-7, "softmax output must remain non-negative within FP32 roundoff");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0u), metrics.ordering_violations,
                 "softmax output must preserve strict input ordering");
    ASSERT_TRUE(metrics.maximum_shift_invariance_error <= 3.0e-5,
                "stable softmax must preserve shift invariance within the accepted FP32 tolerance");
    std::ostringstream artifact;
    artifact << std::setprecision(12)
             << "{\n"
             << "  \"gpu\": \"NVIDIA GeForce RTX 3070\",\n"
             << "  \"package_kernel\": \"kernel-20-default\",\n"
             << "  \"tested_widths\": [1,31,32,33,64,129,256,257,1023,1024,1025,1055,1056],\n"
             << "  \"maximum_absolute_error\": " << metrics.maximum_absolute_error << ",\n"
             << "  \"maximum_relative_error_near_zero_floor_1e-6\": " << metrics.maximum_relative_error << ",\n"
             << "  \"maximum_row_sum_error\": " << metrics.maximum_row_sum_error << ",\n"
             << "  \"maximum_shift_invariance_error\": " << metrics.maximum_shift_invariance_error << ",\n"
             << "  \"minimum_output\": " << metrics.minimum_output << ",\n"
             << "  \"ordering_violations\": " << metrics.ordering_violations << ",\n"
             << "  \"dispatches_per_nonempty_call\": 1,\n"
             << "  \"submissions_per_nonempty_call\": 1,\n"
             << "  \"validation_error_count\": " << reused.validation_error_count << ",\n"
             << "  \"capacity_allocations_after_growth\": " << settled.buffer_allocation_count << ",\n"
             << "  \"capacity_reuses_after_alternation\": " << reused.buffer_reuse_count << "\n"
             << "}\n";
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_fr_m0_rtx_authority.json"), artifact.str()),
                "FR-M0 RTX authority artifact must be written");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy must succeed");
}

FACT(PrometheusReduction_VulkanSumMaxAndStableSoftmaxCorrectness)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create must succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime cleanup must succeed");
        SKIP("Vulkan unavailable; fused reduction GPU correctness cannot execute");
    }

    struct Case {
        const char* name;
        std::uint32_t rows;
        std::uint32_t width;
        std::uint32_t operation;
        float bias;
    };
    const Case cases[] = {
        {"one_element_sum", 2u, 1u, PROM_REDUCTION_OPERATION_SUM, 0.0f},
        {"odd_mixed_sum", 16u, 513u, PROM_REDUCTION_OPERATION_SUM, 0.0f},
        {"packed_short_sum", 512u, 64u, PROM_REDUCTION_OPERATION_SUM, 0.0f},
        {"large_staged_sum", 16u, 4096u, PROM_REDUCTION_OPERATION_SUM, 0.0f},
        {"negative_only_max", 2u, 31u, PROM_REDUCTION_OPERATION_MAX, -20.0f},
        {"odd_mixed_max", 16u, 513u, PROM_REDUCTION_OPERATION_MAX, 0.0f},
        {"large_staged_max", 16u, 4096u, PROM_REDUCTION_OPERATION_MAX, -4.0f},
        {"stable_positive_softmax", 16u, 129u, PROM_REDUCTION_OPERATION_SOFTMAX, 10000.0f},
        {"packed_short_softmax", 512u, 64u, PROM_REDUCTION_OPERATION_SOFTMAX, 10000.0f},
        {"stable_negative_softmax", 16u, 513u, PROM_REDUCTION_OPERATION_SOFTMAX, -10000.0f},
        {"large_staged_softmax", 16u, 4096u, PROM_REDUCTION_OPERATION_SOFTMAX, 5000.0f},
    };
    for (const Case& test_case : cases) {
        std::vector<float> input = deterministic_input(test_case.rows, test_case.width, test_case.bias);
        std::vector<float> output(static_cast<std::size_t>(output_count(test_case.rows, test_case.width, test_case.operation)), -99.0f);
        PrometheusReductionExecutionResult execution{};
        PrometheusReductionBenchmarkResult comparison{};
        ASSERT_TRUE(execute_and_compare(handle, input, output, test_case.rows, test_case.width,
                                        test_case.operation, 0u, execution, comparison),
                    test_case.name);
        ASSERT_EQUAL(1u, execution.physical_slot_recyclable, "successful execution must leave its physical slot recyclable");
        ASSERT_EQUAL(1u, execution.gpu_timestamp_valid, "real reduction execution must carry query-pool GPU timing");
        ASSERT_TRUE(execution.gpu_duration_ns > 0u, "GPU timestamp duration must be positive");
        ASSERT_EQUAL(0u, execution.validation_error_count_after, "validation error count must remain zero");
        if (test_case.width > 1024u) {
            ASSERT_TRUE(execution.plan.stage_count > 1u, "large rows must use explicit staged execution");
            ASSERT_TRUE(execution.plan.temporary_bytes > 0u, "large rows must declare bounded temporary storage");
        }
    }

    const struct SoftmaxPattern {
        const char* name;
        float ordinary;
        float distinguished;
    } patterns[] = {
        {"equal_values", 7.0f, 7.0f},
        {"one_dominant_value", -80.0f, 80.0f},
    };
    for (const SoftmaxPattern& pattern : patterns) {
        std::vector<float> input(4u * 257u, pattern.ordinary);
        for (std::uint32_t row = 0u; row < 4u; ++row) input[static_cast<std::size_t>(row) * 257u + row] = pattern.distinguished;
        std::vector<float> output(input.size(), 0.0f);
        PrometheusReductionExecutionResult execution{};
        PrometheusReductionBenchmarkResult comparison{};
        ASSERT_TRUE(execute_and_compare(handle, input, output, 4u, 257u, PROM_REDUCTION_OPERATION_SOFTMAX,
                                        0u, execution, comparison),
                    pattern.name);
        for (const float value : output) {
            ASSERT_TRUE(std::isfinite(value), "valid finite softmax input must produce finite output");
            ASSERT_TRUE(value >= -2.0e-7f, "softmax output must be nonnegative within tolerance");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "correctness runtime destroy must succeed");
}

FACT(PrometheusReduction_FusedAndComposedSoftmaxAgree)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create must succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime cleanup must succeed");
        SKIP("Vulkan unavailable; softmax strategy comparison cannot execute");
    }
    const std::uint32_t widths[] = {64u, 128u, 256u, 512u, 1024u};
    for (const std::uint32_t width : widths) {
        std::vector<float> input = deterministic_input(16u, width, 9000.0f);
        std::vector<float> fused(input.size(), 0.0f);
        std::vector<float> composed(input.size(), 0.0f);
        PrometheusReductionExecutionResult fused_execution{};
        PrometheusReductionExecutionResult composed_execution{};
        PrometheusReductionBenchmarkResult comparison{};
        ASSERT_TRUE(execute_and_compare(handle, input, fused, 16u, width, PROM_REDUCTION_OPERATION_SOFTMAX,
                                        PROM_REDUCTION_FLAG_FORCE_FUSED, fused_execution, comparison),
                    "forced fused strategy must match the CPU oracle");
        ASSERT_TRUE(execute_and_compare(handle, input, composed, 16u, width, PROM_REDUCTION_OPERATION_SOFTMAX,
                                        PROM_REDUCTION_FLAG_FORCE_COMPOSED, composed_execution, comparison),
                    "forced composed strategy must match the CPU oracle");
        ASSERT_EQUAL(1u, fused_execution.plan.stage_count, "forced fused strategy must use one dispatch");
        ASSERT_EQUAL(3u, composed_execution.plan.stage_count, "small forced composed strategy must use max, sum, normalize dispatches");
        for (std::size_t index = 0u; index < fused.size(); ++index) {
            ASSERT_NEAR(fused[index], composed[index], 3.0e-5f, "fused and composed softmax outputs must agree");
        }
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "strategy runtime destroy must succeed");
}

FACT(PrometheusReduction_PersistentRingReusesResourcesAndReplayIdentity)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.reduction_ring_depth = 2u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create must succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime cleanup must succeed");
        SKIP("Vulkan unavailable; persistent reduction ring cannot execute");
    }
    std::vector<float> input = deterministic_input(16u, 513u, 1000.0f);
    std::vector<float> output(input.size(), 0.0f);
    std::uint64_t replay_id = 0u;
    std::uint64_t prior_logical_id = 0u;
    std::uint32_t observed_slot_mask = 0u;
    for (std::uint32_t iteration = 0u; iteration < 2u; ++iteration) {
        PrometheusReductionExecutionResult execution{};
        PrometheusReductionBenchmarkResult comparison{};
        ASSERT_TRUE(execute_and_compare(handle, input, output, 16u, 513u, PROM_REDUCTION_OPERATION_SOFTMAX,
                                        0u, execution, comparison),
                    "ring warmup softmax must execute correctly");
        if (iteration == 0u) replay_id = execution.plan.replay_id;
        ASSERT_EQUAL(replay_id, execution.plan.replay_id, "replayed shape must retain shader-plan identity");
        ASSERT_TRUE(execution.logical_request_id > prior_logical_id, "logical request IDs must advance independently of slots");
        prior_logical_id = execution.logical_request_id;
        observed_slot_mask |= 1u << execution.physical_slot_id;
    }
    PrometheusReductionDiagnostics warm{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &warm), "warm ring diagnostics must succeed");
    for (std::uint32_t iteration = 0u; iteration < 12u; ++iteration) {
        PrometheusReductionExecutionResult execution{};
        PrometheusReductionBenchmarkResult comparison{};
        ASSERT_TRUE(execute_and_compare(handle, input, output, 16u, 513u, PROM_REDUCTION_OPERATION_SOFTMAX,
                                        0u, execution, comparison),
                    "repeated softmax must execute correctly");
        ASSERT_EQUAL(replay_id, execution.plan.replay_id, "repeated execution must preserve replay identity");
        ASSERT_TRUE(execution.logical_request_id > prior_logical_id, "logical request ID must remain fresh");
        prior_logical_id = execution.logical_request_id;
        observed_slot_mask |= 1u << execution.physical_slot_id;
    }
    PrometheusReductionDiagnostics after{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &after), "post-reuse diagnostics must succeed");
    ASSERT_EQUAL(2u, after.configured_ring_depth, "configured physical ring depth must remain two");
    ASSERT_EQUAL(2u, after.physical_slot_count, "diagnostics must report actual slot count");
    ASSERT_EQUAL(3u, observed_slot_mask, "both persistent physical slots must participate");
    ASSERT_EQUAL(static_cast<uint64_t>(7u), after.pipeline_create_count, "seven reduction pipelines must be created once per runtime");
    ASSERT_EQUAL(warm.buffer_allocation_count, after.buffer_allocation_count, "steady-state replay must not allocate Vulkan buffers");
    ASSERT_TRUE(after.buffer_reuse_count > warm.buffer_reuse_count, "steady-state replay must reuse slot buffers");
    ASSERT_EQUAL(0u, after.quarantined_slots, "successful replay must leave no quarantined slots");
    ASSERT_EQUAL(0u, after.validation_error_count, "ring reuse must remain validation clean");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "ring runtime destroy must succeed");
}

FACT(PrometheusReduction_LogicalFailuresPreservePhysicalRecyclability)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.reduction_ring_depth = 2u;
    config.reduction_test_flags = PROM_REDUCTION_TESTCFG_MALFORMED_STAGE_METADATA |
                                  PROM_REDUCTION_TESTCFG_TEMPORARY_UNDERSIZED |
                                  PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD |
                                  PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT |
                                  PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "fault-injection runtime create must succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime cleanup must succeed");
        SKIP("Vulkan unavailable; staged failure cleanup cannot execute");
    }
    std::vector<float> input = deterministic_input(16u, 4096u, 2000.0f);
    std::vector<float> output(input.size(), -1.0f);
    PrometheusReductionRequest request = make_request(input, output, 16u, 4096u, PROM_REDUCTION_OPERATION_SOFTMAX);
    const int expected_details[] = {
        PROM_REDUCTION_DETAIL_MALFORMED_PLAN,
        PROM_REDUCTION_DETAIL_TEMPORARY_UNDERSIZED,
        PROM_REDUCTION_DETAIL_COMMAND_RECORD_FAILED,
        PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED,
        PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN,
    };
    std::uint64_t prior_logical_id = 0u;
    for (const int expected_detail : expected_details) {
        PrometheusReductionExecutionResult execution{};
        ASSERT_EQUAL(PROM_ERROR,
                     prometheus_reactor_runtime_reduction(handle, &request, &execution),
                     "each injected logical failure must be surfaced");
        ASSERT_EQUAL(expected_detail, execution.detail_code, "fault injections must fire in deterministic stage order");
        ASSERT_TRUE(execution.logical_request_id > prior_logical_id, "failed logical operations must still receive fresh identity");
        prior_logical_id = execution.logical_request_id;
        if (expected_detail == PROM_REDUCTION_DETAIL_COMMAND_RECORD_FAILED ||
            expected_detail == PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED) {
            ASSERT_EQUAL(1u, execution.physical_slot_recyclable, "pre-submit physical failures must be immediately recyclable");
        }
        if (expected_detail == PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN) {
            ASSERT_EQUAL(0u, execution.physical_slot_recyclable, "uncertain submitted work must be quarantined, not declared recyclable");
        }
    }

    PrometheusReductionExecutionResult recovery{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_reduction(handle, &request, &recovery),
                 "one-shot failures must permit a clean execution on another or reaped slot");
    ASSERT_TRUE(recovery.logical_request_id > prior_logical_id, "recovery must use a fresh logical request ID");
    PrometheusReductionExecutionResult reap_drive{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_reduction(handle, &request, &reap_drive),
                 "subsequent execution must drive quarantine reap without leaking the ring");
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(handle, &diagnostics), "failure diagnostics must succeed");
    ASSERT_EQUAL(static_cast<uint64_t>(5u), diagnostics.logical_failure_count, "all five logical failures must be counted");
    ASSERT_EQUAL(static_cast<uint64_t>(1u), diagnostics.quarantine_count, "only completion uncertainty must quarantine a physical slot");
    ASSERT_TRUE(diagnostics.reap_count >= 1u, "submitted quarantined work must be physically reaped after completion");
    ASSERT_EQUAL(0u, diagnostics.quarantined_slots, "recovery must leave no physical slot quarantined");
    ASSERT_EQUAL(0u, diagnostics.validation_error_count, "failure and recovery path must remain validation clean");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "fault-injection runtime destroy must succeed");
}
