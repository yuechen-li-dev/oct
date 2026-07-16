#include "test_harness.h"

#include "../reactor_vulkan.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <cstdint>
#include <cstdlib>
#include <iomanip>
#include <limits>
#include <sstream>
#include <string>
#include <vector>

namespace
{
class EnvironmentValue final
{
public:
    EnvironmentValue(const char* name, const char* value) : name_(name)
    {
        const char* previous = std::getenv(name);
        if (previous != nullptr) {
            hadPrevious_ = true;
            previous_ = previous;
        }
        Set(value);
    }

    ~EnvironmentValue()
    {
        Set(hadPrevious_ ? previous_.c_str() : nullptr);
    }

private:
    void Set(const char* value)
    {
#ifdef _WIN32
        _putenv_s(name_.c_str(), value == nullptr ? "" : value);
#else
        if (value == nullptr) {
            unsetenv(name_.c_str());
        } else {
            setenv(name_.c_str(), value, 1);
        }
#endif
    }

    std::string name_;
    std::string previous_;
    bool hadPrevious_ = false;
};

void FillAttentionInputs(
    std::vector<float>* x,
    std::vector<float>* wq,
    std::vector<float>* wk,
    std::vector<float>* wv,
    std::uint32_t tokens,
    std::uint32_t modelWidth,
    std::uint32_t headDim)
{
    x->resize(static_cast<std::size_t>(tokens) * modelWidth);
    wq->resize(static_cast<std::size_t>(modelWidth) * headDim);
    wk->resize(static_cast<std::size_t>(modelWidth) * headDim);
    wv->resize(static_cast<std::size_t>(modelWidth) * headDim);
    for (std::size_t index = 0u; index < x->size(); ++index) {
        const int value = static_cast<int>((index * 17u + 3u) % 31u) - 15;
        (*x)[index] = static_cast<float>(value) / 128.0f;
    }
    for (std::size_t index = 0u; index < wq->size(); ++index) {
        (*wq)[index] = static_cast<float>(static_cast<int>((index * 7u + 1u) % 23u) - 11) / 128.0f;
        (*wk)[index] = static_cast<float>(static_cast<int>((index * 11u + 5u) % 29u) - 14) / 128.0f;
        (*wv)[index] = static_cast<float>(static_cast<int>((index * 13u + 9u) % 19u) - 9) / 128.0f;
    }
}

prom_m42_reference_result Reference(
    const std::vector<float>& x,
    const std::vector<float>& wq,
    const std::vector<float>& wk,
    const std::vector<float>& wv,
    std::uint32_t tokens,
    std::uint32_t modelWidth,
    std::uint32_t headDim,
    std::uint32_t precision,
    std::vector<float>* output,
    std::vector<float>* q = nullptr,
    std::vector<float>* k = nullptr,
    std::vector<float>* v = nullptr,
    std::vector<float>* scores = nullptr,
    std::vector<float>* probabilities = nullptr)
{
    const std::size_t qCount = static_cast<std::size_t>(tokens) * headDim;
    const std::size_t scoreCount = static_cast<std::size_t>(tokens) * tokens;
    output->assign(qCount, 0.0f);
    if (q != nullptr) q->assign(qCount, 0.0f);
    if (k != nullptr) k->assign(qCount, 0.0f);
    if (v != nullptr) v->assign(qCount, 0.0f);
    if (scores != nullptr) scores->assign(scoreCount, 0.0f);
    if (probabilities != nullptr) probabilities->assign(scoreCount, 0.0f);
    prom_m42_reference_request request{};
    request.x = x.data();
    request.wq = wq.data();
    request.wk = wk.data();
    request.wv = wv.data();
    request.output = output->data();
    request.q = q == nullptr ? nullptr : q->data();
    request.k = k == nullptr ? nullptr : k->data();
    request.v = v == nullptr ? nullptr : v->data();
    request.scores = scores == nullptr ? nullptr : scores->data();
    request.probabilities = probabilities == nullptr ? nullptr : probabilities->data();
    request.tokens = tokens;
    request.model_width = modelWidth;
    request.head_dim = headDim;
    request.value_dim = headDim;
    request.precision_policy = precision;
    prom_m42_reference_result result{};
    if (prom_m42_attention_cpu_reference(&request, &result) != PROM_OK) {
        result.all_finite = 0u;
    }
    return result;
}

bool ContainsStage(const prom_m42_attention_plan& plan, std::uint32_t operation)
{
    for (std::uint32_t index = 0u; index < plan.stage_count; ++index) {
        if (plan.stages[index].operation == operation) return true;
    }
    return false;
}

std::uint64_t MedianMetric(
    const std::vector<prom_m42_attention_result>& results,
    std::uint64_t prom_m42_attention_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m42_attention_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

struct AttentionBenchmarkRecord
{
    const char* workload = nullptr;
    const char* path = nullptr;
    std::uint32_t tokens = 0u;
    std::uint32_t modelWidth = 0u;
    std::uint32_t headDim = 0u;
    std::uint32_t selectedPath = 0u;
    std::uint64_t replayId = 0u;
    std::uint64_t reductionReplayId = 0u;
    std::uint64_t q = 0u;
    std::uint64_t k = 0u;
    std::uint64_t v = 0u;
    std::uint64_t qPack = 0u;
    std::uint64_t kLayout = 0u;
    std::uint64_t vPack = 0u;
    std::uint64_t qk = 0u;
    std::uint64_t scale = 0u;
    std::uint64_t softmax = 0u;
    std::uint64_t pPack = 0u;
    std::uint64_t pv = 0u;
    std::uint64_t totalGpu = 0u;
    std::uint64_t hostEndToEnd = 0u;
    std::uint64_t residentEndToEnd = 0u;
    std::uint64_t finalReadback = 0u;
    std::uint64_t retainedBytes = 0u;
    bool correct = false;
};
}

FACT(PrometheusM42BoundedAttentionPlanContracts)
{
    prom_m42_plan_request request{};
    request.tokens = 128u;
    request.model_width = 1024u;
    request.head_dim = 128u;
    request.value_dim = 128u;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.preferred_path = PROM_M42_PATH_COOPERATIVE;
    request.allow_fallback = 1u;
    request.input_mode = PROM_M42_INPUT_HOST_X;
    request.cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED;
    request.wq_generation = 7u;
    request.wk_generation = 7u;
    request.wv_generation = 7u;
    request.wq_hash = 11u;
    request.wk_hash = 13u;
    request.wv_hash = 17u;

    prom_m42_attention_plan plan{};
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &plan), "primary attention plan builds");
    ASSERT_EQUAL(128u, plan.tokens, "X and Q/K/V use Tokens=128");
    ASSERT_EQUAL(1024u, plan.model_width, "X and projection weights use ModelWidth=1024");
    ASSERT_EQUAL(128u, plan.head_dim, "one-head Q/K/V width is HeadDim=128");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_COOPERATIVE), plan.selected_path,
                 "eligible reduced-precision attention selects the cooperative path");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_K_LAYOUT_PACK_TRANSPOSE_F16), plan.k_layout_strategy,
                 "K layout is an explicit packed transpose");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PROBABILITY_PACK_F16), plan.probability_strategy,
                 "P is explicitly converted before cooperative P x V");
    ASSERT_EQUAL(0u, plan.intermediate_host_copy_count, "the plan has no intermediate host copy");
    ASSERT_EQUAL(1u, plan.final_readback_copy_count, "only one final application readback is planned");
    ASSERT_TRUE(ContainsStage(plan, PROM_M42_STAGE_PROJECT_Q) &&
                ContainsStage(plan, PROM_M42_STAGE_PROJECT_K) &&
                ContainsStage(plan, PROM_M42_STAGE_PROJECT_V),
                "Q, K, and V each have a real SGEMM producer stage");
    ASSERT_TRUE(ContainsStage(plan, PROM_M42_STAGE_QK_TRANSPOSE) &&
                ContainsStage(plan, PROM_M42_STAGE_SCALE) &&
                ContainsStage(plan, PROM_M42_STAGE_SOFTMAX) &&
                ContainsStage(plan, PROM_M42_STAGE_PV),
                "the complete forward operator is present");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_STAGE_FINAL_READBACK),
                 plan.stages[plan.stage_count - 1u].operation,
                 "final readback is ordered after P x V");
    for (std::uint32_t index = 0u; index < plan.stage_count; ++index) {
        const prom_m42_stage_plan& stage = plan.stages[index];
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED), stage.source_queue_family,
                     "attention stages never transfer queue-family ownership");
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED), stage.destination_queue_family,
                     "attention stages retain one queue owner");
        if (stage.barrier_before != 0u || stage.barrier_after != 0u) {
            ASSERT_TRUE(stage.source_stage_mask != 0u && stage.destination_stage_mask != 0u &&
                        stage.source_access_mask != 0u && stage.destination_access_mask != 0u,
                        "machine-readable barrier stages carry exact masks");
        }
    }
    ASSERT_NEAR(1.0f / std::sqrt(128.0f), plan.scale, 1.0e-7f,
                "default scale is exactly 1/sqrt(HeadDim) in FP32");

    prom_m42_attention_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &replay), "identical request replans");
    ASSERT_EQUAL(plan.replay_id, replay.replay_id, "operator replay identity is deterministic");
    request.wq_generation += 1u;
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &replay), "new generation replans");
    ASSERT_TRUE(plan.replay_id != replay.replay_id, "weight generations participate in replay identity");

    request.tokens = 127u;
    request.model_width = 1001u;
    request.head_dim = 127u;
    request.value_dim = 127u;
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &plan), "awkward padded plan builds");
    ASSERT_EQUAL(128u, plan.padded_tokens, "Tokens pad to 16");
    ASSERT_EQUAL(1008u, plan.padded_model_width, "ModelWidth pads to 16");
    ASSERT_EQUAL(128u, plan.padded_head_dim, "HeadDim pads to 16");
    ASSERT_EQUAL(127u, plan.buffers[plan.buffer_count - 1u].logical_columns,
                 "final logical output excludes padded HeadDim");

    request.cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE;
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &plan), "extension absence has a bounded fallback");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16), plan.selected_path,
                 "capability fallback preserves the reduced precision contract");
    ASSERT_EQUAL(1u, plan.fallback_used, "capability fallback is telemetered");
    request.cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED;
    request.rollback_active = 1u;
    ASSERT_EQUAL(PROM_OK, prom_m42_attention_plan_build(&request, &plan), "rollback plan builds");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_A2X4), plan.selected_path,
                 "explicit rollback selects the proven FP32 product path");

    request.value_dim = 126u;
    ASSERT_TRUE(prom_m42_attention_plan_build(&request, &plan) != PROM_OK,
                "the first bounded operator rejects ValueDim != HeadDim");
    request.value_dim = 127u;
    request.tokens = 0u;
    ASSERT_TRUE(prom_m42_attention_plan_build(&request, &plan) != PROM_OK, "zero dimensions reject");
}

FACT(PrometheusM42KTransposeAndReferenceContracts)
{
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    ASSERT_EQUAL(static_cast<std::uint64_t>(2u * 32u + 5u),
                 prom_m42_k_transpose_index(5u, 2u, tokens, 32u),
                 "K[token,head] maps to KTranspose[head,token] with padded token stride");
    ASSERT_EQUAL(std::numeric_limits<std::uint64_t>::max(),
                 prom_m42_k_transpose_index(tokens, 0u, tokens, 32u),
                 "out-of-range token mapping rejects");

    std::vector<float> x;
    std::vector<float> wq;
    std::vector<float> wk;
    std::vector<float> wv;
    FillAttentionInputs(&x, &wq, &wk, &wv, tokens, modelWidth, headDim);
    std::vector<float> reducedOutput;
    std::vector<float> fp32Output;
    std::vector<float> q;
    std::vector<float> k;
    std::vector<float> v;
    std::vector<float> scores;
    std::vector<float> probabilities;
    const prom_m42_reference_result reduced = Reference(
        x, wq, wk, wv, tokens, modelWidth, headDim, PROM_M42_PRECISION_F16_ROUNDED,
        &reducedOutput, &q, &k, &v, &scores, &probabilities);
    ASSERT_EQUAL(1u, reduced.all_finite, "reduced precision reference is finite");
    ASSERT_NEAR(1.0f, reduced.minimum_probability_row_sum, 2.0e-5f,
                "every stable-softmax row sums to one");
    ASSERT_NEAR(1.0f, reduced.maximum_probability_row_sum, 2.0e-5f,
                "row-sum maximum remains one");
    const prom_m42_reference_result fp32 = Reference(
        x, wq, wk, wv, tokens, modelWidth, headDim, PROM_M42_PRECISION_FP32, &fp32Output);
    ASSERT_EQUAL(1u, fp32.all_finite, "FP32 product reference is finite");
    ASSERT_TRUE(reducedOutput != fp32Output, "reduced and FP32 precision contracts are not claimed bit-equivalent");

    prom_m42_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m42_attention_compare(PROM_M42_STAGE_PV, reducedOutput.data(), reducedOutput.data(),
                                             tokens, headDim, tokens, headDim,
                                             1.0e-5f, 1.0e-4f, 101u, 103u, &mismatch),
                 "identical final outputs compare");
    ASSERT_EQUAL(1u, mismatch.matched, "successful comparison is explicit");
    fp32Output = reducedOutput;
    fp32Output[3u * headDim + 4u] += 1.0f;
    ASSERT_TRUE(prom_m42_attention_compare(PROM_M42_STAGE_PV, reducedOutput.data(), fp32Output.data(),
                                           tokens, headDim, tokens, headDim,
                                           1.0e-5f, 1.0e-4f, 101u, 103u, &mismatch) != PROM_OK,
                "first mismatch is rejected");
    ASSERT_EQUAL(3u, mismatch.row, "mismatch row is localized");
    ASSERT_EQUAL(4u, mismatch.column, "mismatch column is localized");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_STAGE_PV), mismatch.stage,
                 "mismatch identifies the operator stage");
    ASSERT_EQUAL(static_cast<std::uint64_t>(101u), mismatch.operator_replay_id,
                 "mismatch carries operator replay identity");

    x[7] = std::numeric_limits<float>::infinity();
    std::vector<float> invalidOutput(static_cast<std::size_t>(tokens) * headDim);
    prom_m42_reference_request invalid{};
    invalid.x = x.data(); invalid.wq = wq.data(); invalid.wk = wk.data(); invalid.wv = wv.data();
    invalid.output = invalidOutput.data(); invalid.tokens = tokens; invalid.model_width = modelWidth;
    invalid.head_dim = headDim; invalid.value_dim = headDim;
    invalid.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    prom_m42_reference_result invalidResult{};
    ASSERT_TRUE(prom_m42_attention_cpu_reference(&invalid, &invalidResult) != PROM_OK,
                "nonfinite caller input rejects");
    ASSERT_EQUAL(PROM_M42_DETAIL_NONFINITE_INPUT, invalidResult.detail_code,
                 "nonfinite rejection is distinct");
}

FACT(PrometheusM42DeviceResidentAttentionHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan services unavailable");
    }

    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    std::vector<float> wq;
    std::vector<float> wk;
    std::vector<float> wv;
    FillAttentionInputs(&x, &wq, &wk, &wv, tokens, modelWidth, headDim);
    std::vector<float> expected;
    std::vector<float> expectedQ;
    std::vector<float> expectedK;
    std::vector<float> expectedV;
    std::vector<float> expectedScores;
    std::vector<float> expectedP;
    const prom_m42_reference_result reference = Reference(
        x, wq, wk, wv, tokens, modelWidth, headDim, PROM_M42_PRECISION_F16_ROUNDED,
        &expected, &expectedQ, &expectedK, &expectedV, &expectedScores, &expectedP);
    ASSERT_EQUAL(1u, reference.all_finite, "tiny CPU attention reference succeeds");

    prom_m42_weight_prepare_request prepareWeights{};
    prepareWeights.wq = wq.data(); prepareWeights.wk = wk.data(); prepareWeights.wv = wv.data();
    prepareWeights.model_width = modelWidth; prepareWeights.head_dim = headDim;
    prepareWeights.value_dim = headDim; prepareWeights.generation = 1u;
    prom_m42_weight_prepare_result preparedWeights{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_prepare_weights(runtime, &prepareWeights, &preparedWeights),
                 "Wq/Wk/Wv upload once and GPU-pack once");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), preparedWeights.wq_generation,
                 "weight generation is stable and visible");
    ASSERT_TRUE(preparedWeights.wq_hash != 0u && preparedWeights.wk_hash != 0u && preparedWeights.wv_hash != 0u,
                "all persistent weights have content identities");

    std::vector<float> output(static_cast<std::size_t>(tokens) * headDim);
    std::vector<float> auditQ(output.size());
    std::vector<float> auditK(output.size());
    std::vector<float> auditV(output.size());
    std::vector<float> auditScores(static_cast<std::size_t>(tokens) * tokens);
    std::vector<float> auditP(auditScores.size());
    prom_m42_attention_request request{};
    request.host_x = x.data(); request.output = output.data();
    request.audit_q = auditQ.data(); request.audit_k = auditK.data(); request.audit_v = auditV.data();
    request.audit_scores = auditScores.data(); request.audit_probabilities = auditP.data();
    request.tokens = tokens; request.model_width = modelWidth; request.head_dim = headDim;
    request.value_dim = headDim; request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.preferred_path = PROM_M42_PATH_COOPERATIVE; request.allow_fallback = 1u;
    request.input_mode = PROM_M42_INPUT_HOST_X; request.audit_intermediates = 1u;
    request.required_wq_generation = 1u; request.required_wk_generation = 1u; request.required_wv_generation = 1u;
    prom_m42_attention_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_execute(runtime, &request, &result),
                 "real GPU Q/K/V producers compose through P x V");
    ASSERT_EQUAL(3u, result.qkv_gpu_producer_dispatch_count, "Q, K, and V each came from an SGEMM dispatch");
    ASSERT_EQUAL(1u, result.final_readback_count, "application path reads back only final Output");
    ASSERT_EQUAL(5u, result.audit_readback_count, "isolated audit localizes all intermediate stages outside timing");
    ASSERT_EQUAL(1u, result.no_intermediate_host_copy, "timed command plan has no intermediate host copy");
    ASSERT_TRUE(result.q_projection_gpu_ns > 0u && result.k_projection_gpu_ns > 0u &&
                result.v_projection_gpu_ns > 0u && result.qk_gpu_ns > 0u &&
                result.softmax_gpu_ns > 0u && result.pv_gpu_ns > 0u &&
                result.total_attention_gpu_ns > 0u,
                "stage and central attention GPU timestamps are populated");
    prom_m42_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m42_attention_compare(PROM_M42_STAGE_PV, expected.data(), output.data(),
                                             tokens, headDim, tokens, headDim,
                                             2.0e-3f, 2.0e-2f, result.plan.replay_id,
                                             result.plan.reduction_replay_id, &mismatch),
                 "final output matches the exact reduced-precision CPU route");
    const int qStatus = prom_m42_attention_compare(
        PROM_M42_STAGE_PROJECT_Q, expectedQ.data(), auditQ.data(), tokens, headDim, tokens, headDim,
        3.0e-3f, 2.0e-2f, result.plan.replay_id, result.plan.reduction_replay_id, &mismatch);
    std::ostringstream qMessage;
    qMessage << "audit Q matches the projection oracle; first mismatch row=" << mismatch.row
             << " column=" << mismatch.column << " expected=" << mismatch.expected
             << " actual=" << mismatch.actual << " abs=" << mismatch.absolute_error
             << " rel=" << mismatch.relative_error;
    ASSERT_EQUAL(PROM_OK, qStatus, qMessage.str());
    const int kStatus = prom_m42_attention_compare(
        PROM_M42_STAGE_PROJECT_K, expectedK.data(), auditK.data(), tokens, headDim, tokens, headDim,
        3.0e-3f, 2.0e-2f, result.plan.replay_id, result.plan.reduction_replay_id, &mismatch);
    std::ostringstream kMessage;
    kMessage << "audit K matches the projection oracle; first mismatch row=" << mismatch.row
             << " column=" << mismatch.column << " expected=" << mismatch.expected
             << " actual=" << mismatch.actual << " abs=" << mismatch.absolute_error
             << " rel=" << mismatch.relative_error;
    ASSERT_EQUAL(PROM_OK, kStatus, kMessage.str());
    const int vStatus = prom_m42_attention_compare(
        PROM_M42_STAGE_PROJECT_V, expectedV.data(), auditV.data(), tokens, headDim, tokens, headDim,
        3.0e-3f, 2.0e-2f, result.plan.replay_id, result.plan.reduction_replay_id, &mismatch);
    std::ostringstream vMessage;
    vMessage << "audit V matches the projection oracle; first mismatch row=" << mismatch.row
             << " column=" << mismatch.column << " expected=" << mismatch.expected
             << " actual=" << mismatch.actual << " abs=" << mismatch.absolute_error
             << " rel=" << mismatch.relative_error;
    ASSERT_EQUAL(PROM_OK, vStatus, vMessage.str());
    ASSERT_EQUAL(PROM_OK,
                 prom_m42_attention_compare(PROM_M42_STAGE_SCALE, expectedScores.data(), auditScores.data(),
                                             tokens, tokens, tokens, tokens,
                                             2.0e-3f, 2.0e-2f, result.plan.replay_id,
                                             result.plan.reduction_replay_id, &mismatch),
                 "audit Scores include the exact GPU scale");
    ASSERT_EQUAL(PROM_OK,
                 prom_m42_attention_compare(PROM_M42_STAGE_SOFTMAX, expectedP.data(), auditP.data(),
                                             tokens, tokens, tokens, tokens,
                                             2.0e-4f, 2.0e-3f, result.plan.replay_id,
                                             result.plan.reduction_replay_id, &mismatch),
                 "M39b consumes scaled Scores directly");

    prom_m42_resident_x_prepare_request prepareX{};
    prepareX.x = x.data(); prepareX.tokens = tokens; prepareX.model_width = modelWidth; prepareX.generation = 1u;
    prom_m42_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "device-X preparation retains FP32 and packed representations");
    std::fill(output.begin(), output.end(), 0.0f);
    request.host_x = nullptr;
    request.input_mode = PROM_M42_INPUT_RESIDENT_X;
    request.required_x_generation = 1u;
    request.audit_intermediates = 0u;
    prom_m42_attention_result residentResult{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_execute(runtime, &request, &residentResult),
                 "resident X executes the same real projection chain");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0u), residentResult.x_conversion_ns,
                 "resident X has no per-operation host conversion");
    ASSERT_EQUAL(PROM_OK,
                 prom_m42_attention_compare(PROM_M42_STAGE_PV, expected.data(), output.data(),
                                             tokens, headDim, tokens, headDim,
                                             2.0e-3f, 2.0e-2f, residentResult.plan.replay_id,
                                             residentResult.plan.reduction_replay_id, &mismatch),
                 "resident-X output matches the oracle");

    request.required_x_generation = 0u;
    ASSERT_TRUE(prom_reactor_runtime_m42_execute(runtime, &request, &residentResult) != PROM_OK,
                "stale X generation rejects before dispatch");
    ASSERT_EQUAL(PROM_M42_DETAIL_STALE_X_GENERATION, residentResult.detail_code,
                 "stale X rejection is explicit");

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services), "runtime services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "attention proof is validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count, "attention proof is validation-error clean");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM42FaultInjectionPreservesLifecycle)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan services unavailable");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    std::vector<float> wq;
    std::vector<float> wk;
    std::vector<float> wv;
    FillAttentionInputs(&x, &wq, &wk, &wv, tokens, modelWidth, headDim);
    prom_m42_weight_prepare_request prepare{};
    prepare.wq = wq.data(); prepare.wk = wk.data(); prepare.wv = wv.data();
    prepare.model_width = modelWidth; prepare.head_dim = headDim; prepare.value_dim = headDim; prepare.generation = 1u;
    prom_m42_weight_prepare_result prepared{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_prepare_weights(runtime, &prepare, &prepared),
                 "fault test prepares persistent weights");
    std::vector<float> output(static_cast<std::size_t>(tokens) * headDim);
    prom_m42_attention_request request{};
    request.host_x = x.data(); request.output = output.data(); request.tokens = tokens;
    request.model_width = modelWidth; request.head_dim = headDim; request.value_dim = headDim;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.preferred_path = PROM_M42_PATH_COOPERATIVE; request.allow_fallback = 1u;
    request.input_mode = PROM_M42_INPUT_HOST_X;
    request.required_wq_generation = 1u; request.required_wk_generation = 1u; request.required_wv_generation = 1u;
    for (const std::uint32_t fault : {PROM_M42_FAULT_AFTER_Q_PROJECTION,
                                      PROM_M42_FAULT_AFTER_QK,
                                      PROM_M42_FAULT_AFTER_SOFTMAX}) {
        request.fault_point = fault;
        prom_m42_attention_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m42_execute(runtime, &request, &failed) != PROM_OK,
                    "pre-final injected logical failure is surfaced");
        ASSERT_EQUAL(PROM_M42_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "partial-stage fault has a distinct detail");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "completed partial work remains physically recyclable");
    }
    request.fault_point = PROM_M42_FAULT_AFTER_PV_SUBMIT;
    prom_m42_attention_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m42_execute(runtime, &request, &uncertain) != PROM_OK,
                "post-PxV completion uncertainty is surfaced");
    ASSERT_EQUAL(PROM_M42_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "post-submit uncertainty is distinct");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "uncertain submitted work is quarantined");

    prepare.generation = 2u;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_prepare_weights(runtime, &prepare, &prepared),
                 "weight replacement reaps the quarantined attention slot");
    ASSERT_EQUAL(1u, prepared.replaced, "persistent replacement is explicit");
    request.fault_point = PROM_M42_FAULT_NONE;
    request.required_wq_generation = 2u; request.required_wk_generation = 2u; request.required_wv_generation = 2u;
    prom_m42_attention_result recovered{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_execute(runtime, &request, &recovered),
                 "fresh generations recover after reap");
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(runtime, &diagnostics),
                 "shared-owner lifecycle diagnostics remain available");
    ASSERT_TRUE(diagnostics.quarantine_count >= 1u, "uncertain attention work entered quarantine");
    ASSERT_TRUE(diagnostics.reap_count >= 1u, "persistent replacement physically reaped it");
    ASSERT_EQUAL(0u, diagnostics.quarantined_slots, "recovery leaves no slot quarantined");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM42ExtensionAbsentUsesConventionalFallback)
{
    EnvironmentValue disabled("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX", "1");
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan services unavailable");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    std::vector<float> wq;
    std::vector<float> wk;
    std::vector<float> wv;
    FillAttentionInputs(&x, &wq, &wk, &wv, tokens, modelWidth, headDim);
    prom_m42_weight_prepare_request prepare{};
    prepare.wq = wq.data(); prepare.wk = wk.data(); prepare.wv = wv.data();
    prepare.model_width = modelWidth; prepare.head_dim = headDim; prepare.value_dim = headDim; prepare.generation = 1u;
    prom_m42_weight_prepare_result prepared{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_prepare_weights(runtime, &prepare, &prepared),
                 "persistent weights prepare without the extension");
    std::vector<float> output(static_cast<std::size_t>(tokens) * headDim);
    prom_m42_attention_request request{};
    request.host_x = x.data(); request.output = output.data(); request.tokens = tokens;
    request.model_width = modelWidth; request.head_dim = headDim; request.value_dim = headDim;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.preferred_path = PROM_M42_PATH_COOPERATIVE; request.allow_fallback = 1u;
    request.input_mode = PROM_M42_INPUT_HOST_X;
    request.required_wq_generation = 1u; request.required_wk_generation = 1u; request.required_wv_generation = 1u;
    prom_m42_attention_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m42_execute(runtime, &request, &result),
                 "complete attention executes through the conventional FP16 fallback");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16), result.selected_path,
                 "fallback path selection is explicit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_SELECTOR_CAPABILITY_FALLBACK), result.selector_reason,
                 "capability rejection reason is telemetered");
    prom_reactor_runtime_destroy_impl(runtime);
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM42AttentionCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan services unavailable");
    }
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M42 cooperative comparison requires the proven KHR tuple");
    }

    struct Workload
    {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t modelWidth;
        std::uint32_t headDim;
    };
    const std::array<Workload, 6> workloads = {{
        {"tiny", 16u, 64u, 16u},
        {"primary", 128u, 1024u, 128u},
        {"larger-head", 128u, 1024u, 256u},
        {"more-tokens", 256u, 1024u, 128u},
        {"awkward-padded", 127u, 1001u, 127u},
        {"softmax-boundary", 1024u, 128u, 64u},
    }};
    struct Path
    {
        const char* name;
        std::uint32_t path;
        std::uint32_t precision;
    };
    const std::array<Path, 3> paths = {{
        {"cooperative-f16", PROM_M42_PATH_COOPERATIVE, PROM_M42_PRECISION_F16_ROUNDED},
        {"a2x4-fp32", PROM_M42_PATH_A2X4, PROM_M42_PRECISION_FP32},
        {"conventional-fp16", PROM_M42_PATH_CONVENTIONAL_FP16, PROM_M42_PRECISION_F16_ROUNDED},
    }};

    std::vector<AttentionBenchmarkRecord> records;
    std::uint64_t generation = 1u;
    std::uint64_t primaryWarm10Median = 0u;
    std::uint64_t primaryWarm100Median = 0u;
    std::ostringstream preparationJson;
    preparationJson << '[';
    bool firstPreparation = true;
    for (const Workload& workload : workloads) {
        std::vector<float> x;
        std::vector<float> wq;
        std::vector<float> wk;
        std::vector<float> wv;
        FillAttentionInputs(&x, &wq, &wk, &wv, workload.tokens, workload.modelWidth, workload.headDim);
        std::vector<float> reducedExpected;
        std::vector<float> fp32Expected;
        const prom_m42_reference_result reducedReference = Reference(
            x, wq, wk, wv, workload.tokens, workload.modelWidth, workload.headDim,
            PROM_M42_PRECISION_F16_ROUNDED, &reducedExpected);
        const prom_m42_reference_result fp32Reference = Reference(
            x, wq, wk, wv, workload.tokens, workload.modelWidth, workload.headDim,
            PROM_M42_PRECISION_FP32, &fp32Expected);
        ASSERT_EQUAL(1u, reducedReference.all_finite, "reduced corpus reference is finite");
        ASSERT_EQUAL(1u, fp32Reference.all_finite, "FP32 corpus reference is finite");
        if (reducedReference.all_finite == 0u || fp32Reference.all_finite == 0u) {
            prom_reactor_runtime_destroy_impl(runtime);
            return;
        }

        prom_m42_weight_prepare_request prepareWeights{};
        prepareWeights.wq = wq.data(); prepareWeights.wk = wk.data(); prepareWeights.wv = wv.data();
        prepareWeights.model_width = workload.modelWidth; prepareWeights.head_dim = workload.headDim;
        prepareWeights.value_dim = workload.headDim; prepareWeights.generation = generation;
        prom_m42_weight_prepare_result preparedWeights{};
        const int weightStatus = prom_reactor_runtime_m42_prepare_weights(runtime, &prepareWeights, &preparedWeights);
        ASSERT_EQUAL(PROM_OK, weightStatus, "corpus persistent weights prepare once");
        if (weightStatus != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
        prom_m42_resident_x_prepare_request prepareX{};
        prepareX.x = x.data(); prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth; prepareX.generation = generation;
        prom_m42_resident_x_prepare_result preparedX{};
        const int xStatus = prom_reactor_runtime_m42_prepare_resident_x(runtime, &prepareX, &preparedX);
        ASSERT_EQUAL(PROM_OK, xStatus, "corpus resident X prepares once");
        if (xStatus != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
        if (!firstPreparation) preparationJson << ',';
        firstPreparation = false;
        preparationJson << "{\"workload\":\"" << workload.name
                        << "\",\"generation\":" << generation
                        << ",\"weight_validation_hash_ns\":" << preparedWeights.validation_hash_ns
                        << ",\"weight_upload_and_pack_ns\":" << preparedWeights.upload_and_pack_ns
                        << ",\"weight_gpu_upload_and_pack_ns\":" << preparedWeights.gpu_upload_and_pack_ns
                        << ",\"weight_retained_bytes\":" << preparedWeights.retained_bytes
                        << ",\"resident_x_validation_hash_ns\":" << preparedX.validation_hash_ns
                        << ",\"resident_x_upload_and_pack_ns\":" << preparedX.upload_and_pack_ns
                        << ",\"resident_x_gpu_upload_and_pack_ns\":" << preparedX.gpu_upload_and_pack_ns
                        << '}';

        for (const Path& path : paths) {
            std::vector<float> output(static_cast<std::size_t>(workload.tokens) * workload.headDim);
            prom_m42_attention_request request{};
            request.host_x = x.data(); request.output = output.data();
            request.tokens = workload.tokens; request.model_width = workload.modelWidth;
            request.head_dim = workload.headDim; request.value_dim = workload.headDim;
            request.precision_policy = path.precision; request.preferred_path = path.path;
            request.allow_fallback = 0u; request.input_mode = PROM_M42_INPUT_HOST_X;
            request.required_wq_generation = generation; request.required_wk_generation = generation;
            request.required_wv_generation = generation;
            prom_m42_attention_result warmup{};
            const int warmupStatus = prom_reactor_runtime_m42_execute(runtime, &request, &warmup);
            std::ostringstream warmupMessage;
            warmupMessage << "host-fed attention warmup succeeds for " << workload.name << '/' << path.name
                          << "; detail=" << warmup.detail_code << " stage=" << warmup.stage;
            ASSERT_EQUAL(PROM_OK, warmupStatus, warmupMessage.str());
            if (warmupStatus != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
            std::vector<prom_m42_attention_result> hostResults;
            for (std::uint32_t repetition = 0u; repetition < 3u; ++repetition) {
                prom_m42_attention_result measured{};
                const int status = prom_reactor_runtime_m42_execute(runtime, &request, &measured);
                ASSERT_EQUAL(PROM_OK, status, "host-fed attention measurement succeeds");
                if (status != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
                hostResults.push_back(measured);
            }
            request.host_x = nullptr;
            request.input_mode = PROM_M42_INPUT_RESIDENT_X;
            request.required_x_generation = generation;
            prom_m42_attention_result residentWarmup{};
            const int residentWarmupStatus = prom_reactor_runtime_m42_execute(runtime, &request, &residentWarmup);
            ASSERT_EQUAL(PROM_OK, residentWarmupStatus, "resident-X attention warmup succeeds");
            if (residentWarmupStatus != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
            std::vector<prom_m42_attention_result> residentResults;
            for (std::uint32_t repetition = 0u; repetition < 3u; ++repetition) {
                prom_m42_attention_result measured{};
                const int status = prom_reactor_runtime_m42_execute(runtime, &request, &measured);
                ASSERT_EQUAL(PROM_OK, status, "resident-X attention measurement succeeds");
                if (status != PROM_OK) { prom_reactor_runtime_destroy_impl(runtime); return; }
                residentResults.push_back(measured);
            }
            const std::vector<float>& expected = path.precision == PROM_M42_PRECISION_FP32
                                                     ? fp32Expected
                                                     : reducedExpected;
            prom_m42_mismatch mismatch{};
            const int compareStatus = prom_m42_attention_compare(
                PROM_M42_STAGE_PV, expected.data(), output.data(), workload.tokens, workload.headDim,
                (workload.tokens + 15u) & ~15u, (workload.headDim + 15u) & ~15u,
                path.precision == PROM_M42_PRECISION_FP32 ? 2.0e-3f : 1.0e-2f,
                path.precision == PROM_M42_PRECISION_FP32 ? 2.0e-2f : 5.0e-2f,
                residentResults.back().plan.replay_id,
                residentResults.back().plan.reduction_replay_id, &mismatch);
            ASSERT_EQUAL(PROM_OK, compareStatus, "corpus final output matches its precision oracle");
            AttentionBenchmarkRecord record{};
            record.workload = workload.name;
            record.path = path.name;
            record.tokens = workload.tokens;
            record.modelWidth = workload.modelWidth;
            record.headDim = workload.headDim;
            record.selectedPath = residentResults.back().selected_path;
            record.replayId = residentResults.back().plan.replay_id;
            record.reductionReplayId = residentResults.back().plan.reduction_replay_id;
            record.q = MedianMetric(residentResults, &prom_m42_attention_result::q_projection_gpu_ns);
            record.k = MedianMetric(residentResults, &prom_m42_attention_result::k_projection_gpu_ns);
            record.v = MedianMetric(residentResults, &prom_m42_attention_result::v_projection_gpu_ns);
            record.qPack = MedianMetric(residentResults, &prom_m42_attention_result::q_pack_gpu_ns);
            record.kLayout = MedianMetric(residentResults, &prom_m42_attention_result::k_layout_gpu_ns);
            record.vPack = MedianMetric(residentResults, &prom_m42_attention_result::v_pack_gpu_ns);
            record.qk = MedianMetric(residentResults, &prom_m42_attention_result::qk_gpu_ns);
            record.scale = MedianMetric(residentResults, &prom_m42_attention_result::scale_gpu_ns);
            record.softmax = MedianMetric(residentResults, &prom_m42_attention_result::softmax_gpu_ns);
            record.pPack = MedianMetric(residentResults, &prom_m42_attention_result::p_pack_gpu_ns);
            record.pv = MedianMetric(residentResults, &prom_m42_attention_result::pv_gpu_ns);
            record.totalGpu = MedianMetric(residentResults, &prom_m42_attention_result::total_attention_gpu_ns);
            record.hostEndToEnd = MedianMetric(hostResults, &prom_m42_attention_result::end_to_end_ns);
            record.residentEndToEnd = MedianMetric(residentResults, &prom_m42_attention_result::end_to_end_ns);
            record.finalReadback = MedianMetric(residentResults, &prom_m42_attention_result::final_readback_ns);
            record.retainedBytes = residentResults.back().retained_bytes;
            record.correct = compareStatus == PROM_OK;
            records.push_back(record);

            if (path.path == PROM_M42_PATH_COOPERATIVE && std::string(workload.name) == "primary") {
                std::vector<prom_m42_attention_result> repeated10;
                std::vector<prom_m42_attention_result> repeated100;
                for (std::uint32_t repetition = 0u; repetition < 10u; ++repetition) {
                    prom_m42_attention_result repeated{};
                    if (prom_reactor_runtime_m42_execute(runtime, &request, &repeated) != PROM_OK) {
                        FAIL("10-operation cooperative replay failed");
                        prom_reactor_runtime_destroy_impl(runtime);
                        return;
                    }
                    repeated10.push_back(repeated);
                }
                for (std::uint32_t repetition = 0u; repetition < 100u; ++repetition) {
                    prom_m42_attention_result repeated{};
                    if (prom_reactor_runtime_m42_execute(runtime, &request, &repeated) != PROM_OK) {
                        FAIL("100-operation cooperative replay failed");
                        prom_reactor_runtime_destroy_impl(runtime);
                        return;
                    }
                    repeated100.push_back(repeated);
                }
                primaryWarm10Median = MedianMetric(repeated10, &prom_m42_attention_result::end_to_end_ns);
                primaryWarm100Median = MedianMetric(repeated100, &prom_m42_attention_result::end_to_end_ns);
            }
        }
        generation += 1u;
    }
    preparationJson << ']';

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services), "final benchmark services are available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "corpus has zero validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "corpus has zero validation errors");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m42.device-resident-attention.v1\",\n"
         << "  \"tensor_convention\": \"one-head X[T,M] Wq/Wk/Wv[M,H] Q/K/V[T,H] Scores/P[T,T] Output[T,H]\",\n"
         << "  \"precision_contract\": \"f16-rounded inputs and inter-SGEMM boundaries, f32 accumulation/output; A2x4 is FP32 product comparison\",\n"
         << "  \"warm_repetitions\": 3,\n"
         << "  \"primary_warm_10_median_end_to_end_ns\": " << primaryWarm10Median << ",\n"
         << "  \"primary_warm_100_median_end_to_end_ns\": " << primaryWarm100Median << ",\n"
         << "  \"validation\": {\"warnings\": " << services.validation_warning_count
         << ", \"errors\": " << services.validation_error_count << "},\n"
         << "  \"device\": {\"cooperative_state\": " << services.cooperative_matrix_state
         << ", \"subgroup_size\": " << services.subgroup_size << "},\n"
         << "  \"preparations\": " << preparationJson.str() << ",\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const AttentionBenchmarkRecord& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"path\":\"" << record.path
             << "\",\"tokens\":" << record.tokens
             << ",\"model_width\":" << record.modelWidth
             << ",\"head_dim\":" << record.headDim
             << ",\"selected_path\":" << record.selectedPath
             << ",\"replay_id\":" << record.replayId
             << ",\"reduction_replay_id\":" << record.reductionReplayId
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"q_projection_gpu_ns\":" << record.q
             << ",\"k_projection_gpu_ns\":" << record.k
             << ",\"v_projection_gpu_ns\":" << record.v
             << ",\"q_pack_gpu_ns\":" << record.qPack
             << ",\"k_layout_gpu_ns\":" << record.kLayout
             << ",\"v_pack_gpu_ns\":" << record.vPack
             << ",\"qk_gpu_ns\":" << record.qk
             << ",\"scale_gpu_ns\":" << record.scale
             << ",\"softmax_gpu_ns\":" << record.softmax
             << ",\"p_pack_gpu_ns\":" << record.pPack
             << ",\"pv_gpu_ns\":" << record.pv
             << ",\"total_attention_gpu_ns\":" << record.totalGpu
             << ",\"host_fed_end_to_end_ns\":" << record.hostEndToEnd
             << ",\"resident_x_end_to_end_ns\":" << record.residentEndToEnd
             << ",\"final_readback_ns\":" << record.finalReadback
             << ",\"retained_bytes\":" << record.retainedBytes << '}';
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m42_device_resident_attention.json", json.str()),
                "M42 benchmark artifact is written");
    prom_reactor_runtime_destroy_impl(runtime);
}
