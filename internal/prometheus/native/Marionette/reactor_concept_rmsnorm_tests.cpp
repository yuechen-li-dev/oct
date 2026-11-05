#include "test_harness.h"
#include "../reactor_concept_rmsnorm.h"
#include "concept_vulkan.h"
#include <algorithm>
#include <bit>
#include <cmath>
#include <cstdio>
#include <limits>
#include <vector>

namespace {
float bf16(float value) {
    auto bits = std::bit_cast<uint32_t>(value);
    bits += 0x7fffu + ((bits >> 16u) & 1u);
    return std::bit_cast<float>(bits & 0xffff0000u);
}
std::vector<float> normalized(const std::vector<float>& input, const std::vector<float>& weight,
    unsigned rows, unsigned stride, bool roundInput, bool roundOutput, float epsilon, std::vector<float>* inverse = nullptr) {
    const auto width = weight.size();
    std::vector<float> out(rows * width);
    for (unsigned row = 0; row < rows; ++row) {
        double sum{};
        for (unsigned col = 0; col < width; ++col) {
            const double x = roundInput ? bf16(input[row * stride + col]) : input[row * stride + col];
            sum += x * x;
        }
        const float inv = static_cast<float>(1.0 / std::sqrt(sum / width + epsilon));
        if (inverse) (*inverse)[row] = inv;
        for (unsigned col = 0; col < width; ++col) {
            const float x = roundInput ? bf16(input[row * stride + col]) : input[row * stride + col];
            const float value = x * inv * weight[col];
            out[row * width + col] = roundOutput ? bf16(value) : value;
        }
    }
    return out;
}
double max_error(const std::vector<float>& expected, const std::vector<float>& actual) {
    double error{};
    for (size_t i = 0; i < actual.size(); ++i) {
        if (!std::isfinite(actual[i])) return std::numeric_limits<double>::infinity();
        error = std::max(error, std::abs(double{actual[i]} - expected[i]));
    }
    return error;
}
prom_m49a_m46_request matched(const std::vector<float>& input, std::vector<float>& output,
    std::vector<float>& inverse, unsigned rows, unsigned width, unsigned stride,
    const prom_m46_weight_prepare_result& weight) {
    prom_m49a_m46_request r{};
    r.matched_z = input.data(); r.matched_storage_element_count = input.size();
    r.output = output.data(); r.output_element_count = output.size();
    r.inv_rms_output = inverse.data(); r.inv_rms_output_element_count = inverse.size();
    r.tokens = rows; r.model_width = width; r.z_row_stride = stride;
    r.strategy = PROM_M46_STRATEGY_SEPARATE_OUTPUT; r.epsilon = 1e-6f;
    r.input_generation = 1; r.reference_input_hash = prom_num_hash_float_bits(input.data(), input.size());
    r.required_weight_generation = weight.generation; r.required_weight_hash = weight.hash;
    r.exact_source_hash = 0x52a1; return r;
}
std::vector<float> positional(const std::vector<float>& input, const std::vector<float>& weight, unsigned heads) {
    const auto norm = normalized(input, weight, 15 * heads, 256, true, true, 1e-6f);
    std::vector<float> out(norm.size());
    for (size_t i = 0; i < out.size(); ++i) {
        const unsigned component = static_cast<unsigned>(i % 256);
        const auto pair = component < 128 ? i + 128 : i - 128;
        const float a = bf16(norm[i] * 0.75f), b = bf16(norm[pair] * 0.5f);
        out[i] = bf16(component < 128 ? a - b : a + b);
    }
    return out;
}
}

FACT(PrometheusConceptRmsNorm_SharedDeviceWeightPublicationAndReuse)
{
    const int buffers = ConceptVkTestLiveBuffers(), contexts = ConceptVkTestLiveContexts();
    void* owner{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &owner), "owner creates");
    if (!owner) return;
    prom_vk_runtime_services before{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(owner, &before), "shared services available");
    ASSERT_TRUE(before.backend_available != 0, "qualification requires real Vulkan");
    uint64_t generation{};
    double worst{};
    for (const unsigned width : {31u, 1536u, 4096u}) {
        constexpr unsigned rows = 3;
        const unsigned stride = width + 7;
        std::vector<float> input(rows * stride, 0.125f), weight(width), output(rows * width), inverse(rows), expectedInverse(rows);
        for (size_t i = 0; i < input.size(); ++i) input[i] = (int((i * 7) % 31) - 15) / 63.0f;
        for (size_t i = 0; i < weight.size(); ++i) weight[i] = 0.75f + float(i % 7) / 64;
        prom_m46_weight_prepare_request upload{weight.data(), weight.size(), width, ++generation};
        prom_m46_weight_prepare_result prepared{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(owner, &upload, &prepared), "weight is committed through Concept");
        auto r = matched(input, output, inverse, rows, width, stride, prepared);
        for (const unsigned precision : {0u, 1u, 2u, 3u}) {
            for (const unsigned strategy : {PROM_M46_STRATEGY_SEPARATE_OUTPUT, PROM_M46_STRATEGY_IN_PLACE_Z}) {
                r.strategy = strategy; r.bf16_roundtrip_input = precision & 1u; r.bf16_roundtrip_output = precision >> 1u;
                r.requested_reduction_plan = width == 31 ? PROM_M46_REDUCTION_FORCE_STAGED : PROM_M46_REDUCTION_AUTO;
                const auto expected = normalized(input, weight, rows, stride, r.bf16_roundtrip_input != 0, r.bf16_roundtrip_output != 0, r.epsilon, &expectedInverse);
                prom_m49a_m46_result result{};
                const int status = prom_reactor_runtime_m49a_execute_m46(owner, &r, &result);
                if (status != PROM_OK) std::fprintf(stderr, "RMSNorm width=%u precision=%u strategy=%u detail=%d\n", width, precision, strategy, result.detail_code);
                ASSERT_EQUAL(PROM_OK, status, "strided normalization executes on shared device");
                worst = std::max(worst, max_error(expected, output));
                ASSERT_TRUE(max_error(expected, output) < 1e-4, "FP32/BF16 output matches independent double-sum oracle");
                ASSERT_TRUE(max_error(expectedInverse, inverse) < 1e-5, "inverse RMS agrees with independent oracle");
                ASSERT_EQUAL(1u, result.rmsnorm.submit_count, "the chain submits once");
                ASSERT_EQUAL(1u, result.rmsnorm.no_intermediate_host_copy, "no intermediate readback");
                ASSERT_TRUE(result.rmsnorm.retained_bytes > 0, "retained capacity is reported");
                prom_m49a_m46_result repeated{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m49a_execute_m46(owner, &r, &repeated), "warm repeat executes");
                ASSERT_EQUAL(result.output_hash, repeated.output_hash, "warm output is bitwise stable");
                ASSERT_EQUAL(result.replay_identity, repeated.replay_identity, "logical replay is independent of physical generation");
                ASSERT_EQUAL(result.rmsnorm.buffer_allocation_count, repeated.rmsnorm.buffer_allocation_count, "warm request allocates no buffers");
            }
        }
        prom_m46_weight_prepare_result rejected{};
        ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_m46_prepare_weight(owner, &upload, &rejected), "same generation is stale");
        ASSERT_EQUAL(PROM_M46_DETAIL_STALE_WEIGHT_GENERATION, rejected.detail_code, "stale reason survives the ABI");
        const float saved = weight[0]; weight[0] = std::numeric_limits<float>::quiet_NaN(); upload.generation = generation + 1;
        ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_m46_prepare_weight(owner, &upload, &rejected), "nonfinite candidate is rejected");
        weight[0] = saved;
        prom_m49a_m46_result preserved{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m49a_execute_m46(owner, &r, &preserved), "rejected candidate preserves prior committed weight");
        ASSERT_TRUE(prom_reactor_runtime_reduction_state(owner) == nullptr, "RMSNorm never initializes the legacy model executor");
    }
    float a = 2, b = 3, c = -1; uint32_t stage{}; int detail{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(owner, &a, &b, &c, 1, 1, 1, &stage, &detail), "legacy SGEMM coexists");
    ASSERT_EQUAL(6.0f, c, "SGEMM remains correct");
    std::vector<float> values(2048, 1), sum(1);
    PrometheusReductionRequest reduction{}; reduction.struct_size = sizeof reduction;
    reduction.input = values.data(); reduction.output = sum.data(); reduction.input_element_count = values.size(); reduction.output_element_count = sum.size();
    reduction.row_count = 1; reduction.elements_per_row = 2048; reduction.operation = PROM_REDUCTION_OPERATION_SUM;
    PrometheusReductionExecutionResult reduced{};
    const int reducedStatus = prometheus_reactor_runtime_reduction(owner, &reduction, &reduced);
    if (reducedStatus != PROM_OK) std::fprintf(stderr, "shared reduction detail=%d stage=%u\n", reduced.detail_code, reduced.stage);
    ASSERT_EQUAL(PROM_OK, reducedStatus, "Concept reductions coexist with normalization");
    ASSERT_EQUAL(2048.0f, sum[0], "reduction remains correct");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_concept_norm_detach(owner), "normalization borrower detaches");
    prom_vk_runtime_services after{}; prom_reactor_runtime_get_vk_services(owner, &after);
    ASSERT_TRUE(before.device == after.device && before.compute_queue == after.compute_queue, "device identity survives borrower detach");
    ASSERT_EQUAL(0u, after.validation_error_count, "shared execution is validation clean");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(owner), "owner destroys");
    ASSERT_EQUAL(buffers, ConceptVkTestLiveBuffers(), "all Concept buffers retired");
    ASSERT_EQUAL(contexts, ConceptVkTestLiveContexts(), "all borrowed contexts retired");
    std::fprintf(stderr, "Concept RMSNorm worst absolute error=%g\n", worst);
}

FACT(PrometheusConceptRmsNorm_ResidentRopeFeedsLegacyAttention)
{
    const int buffers = ConceptVkTestLiveBuffers();
    void* owner{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &owner), "resident owner creates");
    if (!owner) return;
    std::vector<float> q(15 * 8 * 256), k(15 * 256), weight(256), cosine(15 * 256, 0.75f), sine(15 * 256, 0.5f);
    for (size_t i = 0; i < q.size(); ++i) q[i] = (int(i * 13 % 61) - 30) / 71.0f;
    for (size_t i = 0; i < k.size(); ++i) k[i] = (int(i * 7 % 41) - 20) / 53.0f;
    for (size_t i = 0; i < weight.size(); ++i) weight[i] = 0.75f + float(i % 5) / 32;
    const auto expectedQ = positional(q, weight, 8), expectedK = positional(k, weight, 1);
    std::vector<float> scores(8 * 15 * 15), expected(scores.size());
    for (unsigned head = 0; head < 8; ++head) for (unsigned row = 0; row < 15; ++row) for (unsigned col = 0; col < 15; ++col) {
        double dot{};
        for (unsigned d = 0; d < 256; ++d) dot += double{expectedQ[(row * 8 + head) * 256 + d]} * expectedK[col * 256 + d];
        expected[(head * 15 + row) * 15 + col] = static_cast<float>(dot * 0.0625);
    }
    // Optional final positional readback still skips the normalized boundary.
    std::vector<float> qOutput(q.size());
    PrometheusGemma4E2BM1HeadRmsNormRopeRequest head{}; head.struct_size = sizeof head;
    head.input = q.data(); head.weight = weight.data(); head.cosine = cosine.data(); head.sine = sine.data(); head.output = qOutput.data();
    head.input_element_count = q.size(); head.weight_element_count = weight.size(); head.cosine_element_count = cosine.size(); head.sine_element_count = sine.size(); head.output_element_count = qOutput.size();
    head.tokens = 15; head.heads = 8; head.head_width = 256; head.epsilon = 1e-6f; head.input_generation = 1; head.weight_generation = 1; head.table_generation = 1; head.exact_source_hash = 11;
    PrometheusGemma4E2BM1HeadRmsNormRopeResult headResult{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope(owner, &head, &headResult), "optional final readback executes");
    ASSERT_TRUE(max_error(expectedQ, qOutput) < 1e-4, "positional output matches BF16 oracle");
    ASSERT_EQUAL(0u, headResult.normalized_readback_count, "head normalization stays resident");
    ASSERT_TRUE(prom_reactor_runtime_reduction_state(owner) == nullptr, "head normalization and RoPE have only Concept ownership");
    PrometheusGemma4E2BM1AttentionScoresRequest request{}; request.struct_size = sizeof request;
    request.query_input = q.data(); request.key_input = k.data(); request.query_weight = weight.data(); request.key_weight = weight.data(); request.cosine = cosine.data(); request.sine = sine.data(); request.scores = scores.data();
    request.query_input_element_count = q.size(); request.key_input_element_count = k.size(); request.query_weight_element_count = weight.size(); request.key_weight_element_count = weight.size(); request.cosine_element_count = cosine.size(); request.sine_element_count = sine.size(); request.score_element_count = scores.size();
    request.tokens = 15; request.query_heads = 8; request.key_heads = 1; request.head_width = 256; request.epsilon = 1e-6f; request.scale = 0.0625f;
    request.query_input_generation = 1; request.key_input_generation = 2; request.table_generation = 1; request.query_exact_source_hash = 11; request.key_exact_source_hash = 12;
    for (unsigned order = 0; order < 2; ++order) {
        request.preparation_order = order;
        request.query_weight_generation = order == 0 ? 2 : 5;
        request.key_weight_generation = order == 0 ? 3 : 4;
        PrometheusGemma4E2BM1AttentionScoresResult result{};
        const int status = prometheus_reactor_runtime_gemma4e2b_m1_attention_scores(owner, &request, &result);
        if (status != PROM_OK) std::fprintf(stderr, "resident chain order=%u detail=%d stage=%u\n", order, result.detail_code, result.stage);
        ASSERT_EQUAL(PROM_OK, status, "resident Concept Q/K feed legacy attention in either preparation order");
        ASSERT_TRUE(max_error(expected, scores) < 1e-3, "scores match independent CPU chain");
        ASSERT_EQUAL(0u, result.host_detour_count, "Q/K have no host detour");
        ASSERT_EQUAL(1u, result.score_readback_count, "only scores are read back");
        ASSERT_TRUE(result.query_slot_id != result.key_slot_id && result.query_slot_id != result.score_slot_id && result.key_slot_id != result.score_slot_id, "independent physical owners have distinct diagnostic IDs");
        prom_device_buffer_view consumed{};
        ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_concept_norm_resident(owner, 8, &consumed), "successful attention consumes resident authority");
    }
    prom_vk_runtime_services services{}; prom_reactor_runtime_get_vk_services(owner, &services);
    ASSERT_EQUAL(0u, services.validation_error_count, "resident producer/consumer chain is validation clean");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(owner), "resident owner drains both allocators");
    ASSERT_EQUAL(buffers, ConceptVkTestLiveBuffers(), "resident outputs do not outlive owner");
    std::fprintf(stderr, "Concept RMSNorm resident attention max error=%g\n", max_error(expected, scores));
}

FACT(PrometheusConceptRmsNorm_UncertainCompletionRetainsAndDrains)
{
    const int buffers = ConceptVkTestLiveBuffers();
    PrometheusReactorConfig config{}; config.struct_size = sizeof config;
    config.reduction_test_flags = PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION;
    void* owner{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &owner), "uncertainty owner creates");
    if (!owner) return;
    std::vector<float> input(4096, 1), weight(4096, 1), output(4096, -99), inverse(1, -99);
    prom_m46_weight_prepare_request upload{weight.data(), weight.size(), 4096, 1};
    prom_m46_weight_prepare_result prepared{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(owner, &upload, &prepared), "weight prepares before uncertainty");
    auto request = matched(input, output, inverse, 1, 4096, 4096, prepared);
    prom_m49a_m46_result result{};
    ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_m49a_execute_m46(owner, &request, &result), "uncertain completion is surfaced");
    ASSERT_EQUAL(PROM_M46_DETAIL_COMPLETION_UNCERTAIN, result.detail_code, "uncertainty detail survives transport");
    ASSERT_EQUAL(-99.0f, output[0], "uncertain work cannot publish output");
    ASSERT_EQUAL(-99.0f, inverse[0], "uncertain work cannot publish inverse RMS");
    ASSERT_TRUE(ConceptVkTestPendingSubmissions() > 0, "submission remains owned after uncertain observation");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m49a_execute_m46(owner, &request, &result), "retry drains prior work before reuse");
    ASSERT_NEAR(1.0f / std::sqrt(1.000001f), output[0], 1e-6f, "retry publishes correct output");
    ASSERT_EQUAL(0, ConceptVkTestPendingSubmissions(), "retry retires prior and current fences");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_concept_norm_detach(owner), "drained borrower detaches");
    ASSERT_EQUAL(buffers, ConceptVkTestLiveBuffers(), "quarantined allocation is released after completion");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(owner, &upload, &prepared), "owner admits a fresh borrower after detach");
    ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_m49a_execute_m46(owner, &request, &result), "fresh borrower owns a fresh fault state");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_concept_norm_detach(owner), "detach directly drains quarantined work");
    float a = 2, b = 3, c{}; uint32_t stage{}; int detail{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(owner, &a, &b, &c, 1, 1, 1, &stage, &detail), "legacy owner survives quarantine detach");
    ASSERT_EQUAL(6.0f, c, "legacy output is correct");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(owner), "owner destroys independently");
    ASSERT_EQUAL(buffers, ConceptVkTestLiveBuffers(), "no retained allocations after destruction");
}
