#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <limits>
#include <string>
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
                                   : static_cast<std::uint32_t>(PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP),
                     softmax_plan.strategy,
                     "softmax selector must use the documented bounded threshold");
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
