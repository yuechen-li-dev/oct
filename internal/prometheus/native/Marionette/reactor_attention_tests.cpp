#include "test_harness.h"

#include "../reactor_vulkan.h"

#include <algorithm>
#include <array>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <cstdint>
#include <cstdlib>
#include <fstream>
#include <iomanip>
#include <limits>
#include <sstream>
#include <string>
#include <string_view>
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

using GroupWeights = std::array<std::array<std::vector<float>, PROM_M43_WEIGHT_KIND_COUNT>,
                                PROM_M43_HEAD_COUNT>;

void FillGroupInputs(
    std::vector<float>* x,
    GroupWeights* weights,
    std::uint32_t tokens,
    std::uint32_t modelWidth,
    std::uint32_t headDim)
{
    x->resize(static_cast<std::size_t>(tokens) * modelWidth);
    for (std::size_t index = 0u; index < x->size(); ++index) {
        const int value = static_cast<int>((index * 17u + 3u) % 31u) - 15;
        (*x)[index] = static_cast<float>(value) / 128.0f;
    }
    const std::size_t weightCount = static_cast<std::size_t>(modelWidth) * headDim;
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            std::vector<float>& values = (*weights)[head][kind];
            values.resize(weightCount);
            for (std::size_t index = 0u; index < weightCount; ++index) {
                const std::uint32_t seed = 5u + head * 19u + kind * 11u;
                const std::uint32_t multiplier = 7u + kind * 4u + head * 2u;
                const int value = static_cast<int>((index * multiplier + seed) % 29u) - 14;
                values[index] = static_cast<float>(value) / 128.0f;
            }
        }
    }
}

std::uint64_t GroupWeightGeneration(std::uint32_t head, std::uint32_t kind)
{
    return 100u + static_cast<std::uint64_t>(head) * PROM_M43_WEIGHT_KIND_COUNT + kind;
}

void FillGroupPlanRequest(prom_m43_plan_request* request,
                          std::uint32_t tokens,
                          std::uint32_t modelWidth,
                          std::uint32_t headDim,
                          std::uint32_t strategy,
                          std::uint32_t inputMode)
{
    *request = {};
    request->head_count = PROM_M43_HEAD_COUNT;
    request->tokens = tokens;
    request->model_width = modelWidth;
    request->head_dim = headDim;
    request->precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request->allow_fallback = 1u;
    request->input_mode = inputMode;
    request->execution_strategy = strategy;
    request->cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED;
    request->shared_x_generation = 41u;
    request->shared_x_hash = 43u;
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        request->preferred_path[head] = PROM_M42_PATH_COOPERATIVE;
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            request->weight_generation[head][kind] = GroupWeightGeneration(head, kind);
            request->weight_hash[head][kind] = 1009u + head * 101u + kind * 17u;
        }
    }
}

bool PrepareGroupWeights(void* runtime,
                         const GroupWeights& weights,
                         std::uint32_t modelWidth,
                         std::uint32_t headDim,
                         std::uint32_t replacementHead = PROM_M43_HEAD_COUNT,
                         std::uint32_t replacementKind = PROM_M43_WEIGHT_KIND_COUNT,
                         std::uint64_t replacementGeneration = 0u)
{
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            if (replacementHead < PROM_M43_HEAD_COUNT &&
                (head != replacementHead || kind != replacementKind)) continue;
            prom_m43_weight_prepare_request request{};
            request.values = weights[head][kind].data();
            request.element_count = static_cast<std::uint64_t>(modelWidth) * headDim;
            request.head_index = head;
            request.weight_kind = kind;
            request.model_width = modelWidth;
            request.head_dim = headDim;
            request.generation = replacementHead < PROM_M43_HEAD_COUNT
                                     ? replacementGeneration
                                     : GroupWeightGeneration(head, kind);
            prom_m43_weight_prepare_result result{};
            if (prom_reactor_runtime_m43_prepare_weight(runtime, &request, &result) != PROM_OK ||
                result.hash == 0u || result.generation != request.generation) return false;
        }
    }
    return true;
}

void FillGroupExecutionRequest(prom_m43_attention_group_request* request,
                               const float* hostX,
                               float* output,
                               std::uint32_t tokens,
                               std::uint32_t modelWidth,
                               std::uint32_t headDim,
                               std::uint32_t strategy,
                               std::uint32_t inputMode,
                               std::uint64_t xGeneration)
{
    *request = {};
    request->host_x = hostX;
    request->output = output;
    request->host_x_element_count = inputMode == PROM_M42_INPUT_HOST_X
                                        ? static_cast<std::uint64_t>(tokens) * modelWidth
                                        : 0u;
    request->output_element_count =
        static_cast<std::uint64_t>(PROM_M43_HEAD_COUNT) * tokens * headDim;
    request->head_count = PROM_M43_HEAD_COUNT;
    request->tokens = tokens;
    request->model_width = modelWidth;
    request->head_dim = headDim;
    request->precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request->allow_fallback = 1u;
    request->input_mode = inputMode;
    request->execution_strategy = strategy;
    request->shared_x_generation = xGeneration;
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        request->preferred_path[head] = PROM_M42_PATH_COOPERATIVE;
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            request->required_weight_generation[head][kind] = GroupWeightGeneration(head, kind);
        }
    }
}

prom_m43_reference_result GroupReference(const std::vector<float>& x,
                                         const GroupWeights& weights,
                                         std::uint32_t tokens,
                                         std::uint32_t modelWidth,
                                         std::uint32_t headDim,
                                         std::vector<float>* output,
                                         std::uint32_t precision = PROM_M42_PRECISION_F16_ROUNDED)
{
    output->assign(static_cast<std::size_t>(PROM_M43_HEAD_COUNT) * tokens * headDim, 0.0f);
    prom_m43_reference_request request{};
    request.x = x.data();
    request.output = output->data();
    request.x_element_count = static_cast<std::uint64_t>(tokens) * modelWidth;
    request.weight_element_count = static_cast<std::uint64_t>(modelWidth) * headDim;
    request.output_element_count = static_cast<std::uint64_t>(PROM_M43_HEAD_COUNT) * tokens * headDim;
    request.head_count = PROM_M43_HEAD_COUNT;
    request.tokens = tokens;
    request.model_width = modelWidth;
    request.head_dim = headDim;
    request.precision_policy = precision;
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            request.weight[head][kind] = weights[head][kind].data();
        }
    }
    prom_m43_reference_result result{};
    if (prom_m43_attention_cpu_reference(&request, &result) != PROM_OK) result.all_finite = 0u;
    return result;
}

struct GroupBenchmarkRecord
{
    std::string workload;
    std::string path;
    std::string strategy;
    std::string inputMode;
    std::uint32_t tokens = 0u;
    std::uint32_t modelWidth = 0u;
    std::uint32_t headDim = 0u;
    std::uint64_t replayId = 0u;
    std::uint64_t weightPreparationNs = 0u;
    std::uint64_t weightPreparationGpuNs = 0u;
    std::uint64_t xPreparationNs = 0u;
    std::uint64_t xPreparationGpuNs = 0u;
    std::uint64_t sharedXValidationNs = 0u;
    std::uint64_t sharedXUploadGpuNs = 0u;
    std::uint64_t sharedXPackGpuNs = 0u;
    std::uint64_t projectionGpuNs = 0u;
    std::uint64_t postProjectionGpuNs = 0u;
    std::uint64_t qPackGpuNs = 0u;
    std::uint64_t kLayoutGpuNs = 0u;
    std::uint64_t vPackGpuNs = 0u;
    std::uint64_t qkGpuNs = 0u;
    std::uint64_t scaleGpuNs = 0u;
    std::uint64_t softmaxGpuNs = 0u;
    std::uint64_t pPackGpuNs = 0u;
    std::uint64_t pvGpuNs = 0u;
    std::uint64_t totalGpuNs = 0u;
    std::uint64_t cpuRecordingNs = 0u;
    std::uint64_t cpuSubmissionNs = 0u;
    std::uint64_t finalReadbackNs = 0u;
    std::uint64_t endToEndNs = 0u;
    std::uint64_t retainedBytes = 0u;
    std::uint64_t exactBytes = 0u;
    std::array<std::uint64_t, PROM_M43_HEAD_COUNT> qProjection{};
    std::array<std::uint64_t, PROM_M43_HEAD_COUNT> kProjection{};
    std::array<std::uint64_t, PROM_M43_HEAD_COUNT> vProjection{};
    std::array<std::uint64_t, PROM_M43_HEAD_COUNT> headReplay{};
    std::uint32_t submitCount = 0u;
    std::uint32_t dispatchCount = 0u;
    std::uint32_t barrierCalls = 0u;
    std::uint32_t barrierBuffers = 0u;
    bool correct = false;
};

std::uint64_t MedianGroupMetric(
    const std::vector<prom_m43_attention_group_result>& results,
    std::uint64_t prom_m43_attention_group_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m43_attention_group_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

std::uint64_t MedianGroupHeadMetric(const std::vector<prom_m43_attention_group_result>& results,
                                    std::uint32_t head,
                                    std::uint32_t projection)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m43_attention_group_result& result : results) {
        if (projection == PROM_M43_WEIGHT_Q) values.push_back(result.q_projection_gpu_ns[head]);
        else if (projection == PROM_M43_WEIGHT_K) values.push_back(result.k_projection_gpu_ns[head]);
        else values.push_back(result.v_projection_gpu_ns[head]);
    }
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

void FillOutputProjectionWeight(std::vector<float>* wo,
                                std::uint32_t headDim,
                                std::uint32_t modelWidth)
{
    wo->resize(static_cast<std::size_t>(PROM_M44_HEAD_COUNT) * headDim * modelWidth);
    for (std::size_t index = 0u; index < wo->size(); ++index) {
        const int value = static_cast<int>((index * 23u + 11u) % 37u) - 18;
        (*wo)[index] = static_cast<float>(value) / 256.0f;
    }
}

bool PrepareOutputProjectionWeight(void* runtime,
                                   const std::vector<float>& wo,
                                   std::uint32_t headDim,
                                   std::uint32_t modelWidth,
                                   std::uint64_t generation,
                                   prom_m44_wo_prepare_result* outResult = nullptr)
{
    prom_m44_wo_prepare_request request{};
    request.values = wo.data();
    request.element_count = static_cast<std::uint64_t>(PROM_M44_HEAD_COUNT) * headDim * modelWidth;
    request.head_count = PROM_M44_HEAD_COUNT;
    request.head_dim = headDim;
    request.model_width = modelWidth;
    request.generation = generation;
    prom_m44_wo_prepare_result result{};
    const bool ok = prom_reactor_runtime_m44_prepare_wo(runtime, &request, &result) == PROM_OK &&
                    result.generation == generation && result.hash != 0u;
    if (outResult != nullptr) *outResult = result;
    return ok;
}

prom_m44_reference_result OutputProjectionReference(const std::vector<float>& headMajor,
                                                    const std::vector<float>& wo,
                                                    std::uint32_t tokens,
                                                    std::uint32_t headDim,
                                                    std::uint32_t modelWidth,
                                                    std::uint32_t precision,
                                                    std::vector<float>* output,
                                                    std::vector<float>* concatenated = nullptr)
{
    output->assign(static_cast<std::size_t>(tokens) * modelWidth, 0.0f);
    if (concatenated != nullptr) {
        concatenated->assign(static_cast<std::size_t>(tokens) * PROM_M44_HEAD_COUNT * headDim, 0.0f);
    }
    prom_m44_reference_request request{};
    request.head_major = headMajor.data();
    request.wo = wo.data();
    request.concatenated = concatenated == nullptr ? nullptr : concatenated->data();
    request.output = output->data();
    request.head_major_element_count = headMajor.size();
    request.wo_element_count = wo.size();
    request.output_element_count = output->size();
    request.head_count = PROM_M44_HEAD_COUNT;
    request.tokens = tokens;
    request.head_dim = headDim;
    request.model_width = modelWidth;
    request.precision_policy = precision;
    prom_m44_reference_result result{};
    if (prom_m44_output_projection_cpu_reference(&request, &result) != PROM_OK) result.all_finite = 0u;
    return result;
}

void FillM44ComposedRequest(prom_m44_composed_request* request,
                            const float* hostX,
                            float* output,
                            std::uint32_t tokens,
                            std::uint32_t modelWidth,
                            std::uint32_t headDim,
                            std::uint32_t aggregation,
                            std::uint32_t projection,
                            std::uint32_t submitPlan,
                            std::uint32_t inputMode,
                            std::uint64_t xGeneration,
                            std::uint64_t woGeneration)
{
    *request = {};
    FillGroupExecutionRequest(&request->attention, hostX, nullptr, tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED, inputMode, xGeneration);
    request->output = output;
    request->output_element_count = static_cast<std::uint64_t>(tokens) * modelWidth;
    request->aggregation_strategy = aggregation;
    request->projection_path = projection;
    request->submit_plan = submitPlan;
    request->required_wo_generation = woGeneration;
}

void FillM45ComposedRequest(prom_m45_composed_request* request,
                            float* output,
                            std::uint32_t tokens,
                            std::uint32_t modelWidth,
                            std::uint32_t headDim,
                            std::uint32_t strategy,
                            std::uint32_t submitPolicy,
                            std::uint64_t xGeneration,
                            std::uint64_t woGeneration)
{
    *request = {};
    FillGroupExecutionRequest(&request->attention, nullptr, nullptr, tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED,
                              PROM_M42_INPUT_RESIDENT_X, xGeneration);
    request->output = output;
    request->output_element_count = output == nullptr
                                        ? 0u
                                        : static_cast<std::uint64_t>(tokens) * modelWidth;
    request->aggregation_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request->projection_path = PROM_M44_PROJECTION_COOPERATIVE;
    request->residual_strategy = strategy;
    request->submit_policy = submitPolicy;
    request->required_wo_generation = woGeneration;
}

struct M44BenchmarkRecord
{
    std::string workload;
    std::string strategy;
    std::string path;
    std::string submitPlan;
    std::uint32_t tokens = 0u;
    std::uint32_t headDim = 0u;
    std::uint32_t modelWidth = 0u;
    std::uint64_t replayId = 0u;
    std::uint64_t m43ReplayId = 0u;
    std::uint64_t woPreparationNs = 0u;
    std::uint64_t woPreparationGpuNs = 0u;
    std::uint64_t m43GpuNs = 0u;
    std::uint64_t aggregationGpuNs = 0u;
    std::uint64_t projectionGpuNs = 0u;
    std::uint64_t accumulationGpuNs = 0u;
    std::uint64_t m44GpuNs = 0u;
    std::uint64_t totalGpuNs = 0u;
    std::uint64_t cpuRecordingNs = 0u;
    std::uint64_t cpuSubmissionNs = 0u;
    std::uint64_t finalReadbackNs = 0u;
    std::uint64_t endToEndNs = 0u;
    std::uint64_t cpuConcatenateNs = 0u;
    std::uint64_t cpuPackNs = 0u;
    std::uint64_t temporaryBytes = 0u;
    std::uint64_t retainedBytes = 0u;
    std::uint64_t exactBytes = 0u;
    std::uint64_t sourceHeadBytes = 0u;
    std::uint64_t contiguousF32Bytes = 0u;
    std::uint64_t contiguousPackedBytes = 0u;
    std::uint64_t partialOutputBytes = 0u;
    std::uint64_t accumulationBytes = 0u;
    std::uint64_t woUploadBytes = 0u;
    std::uint64_t woF32Bytes = 0u;
    std::uint64_t woPackedBytes = 0u;
    std::uint64_t finalYBytes = 0u;
    std::uint64_t finalReadbackBytes = 0u;
    std::uint32_t reusableDescriptorSets = 0u;
    std::uint32_t descriptorBindings = 0u;
    std::uint32_t submitCount = 0u;
    std::uint32_t dispatchCount = 0u;
    std::uint32_t barrierCalls = 0u;
    std::uint32_t barrierBuffers = 0u;
    std::uint32_t copyRegions = 0u;
    std::uint32_t intermediateHostCopies = 0u;
    bool correct = false;
};

std::uint64_t MedianM44Metric(const std::vector<prom_m44_composed_result>& results,
                              std::uint64_t prom_m44_composed_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m44_composed_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

std::uint64_t MedianM45Metric(const std::vector<prom_m45_composed_result>& results,
                              std::uint64_t prom_m45_composed_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m45_composed_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

std::uint64_t MedianM46Metric(const std::vector<prom_m46_composed_result>& results,
                              std::uint64_t prom_m46_composed_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m46_composed_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

std::uint64_t MedianM47Metric(const std::vector<prom_m47_composed_result>& results,
                              std::uint64_t prom_m47_composed_result::*member)
{
    std::vector<std::uint64_t> values;
    values.reserve(results.size());
    for (const prom_m47_composed_result& result : results) values.push_back(result.*member);
    std::sort(values.begin(), values.end());
    return values.empty() ? 0u : values[values.size() / 2u];
}

prom_m48_plan_request M48ResidentPlanRequest(std::uint32_t layerCount = PROM_M48_LAYER_COUNT,
                                             std::uint32_t auditMode = 0u)
{
    prom_m48_plan_request request{};
    request.initial_activation_mode = PROM_M48_INITIAL_RESIDENT;
    request.initial_activation_exclusive = 1u;
    request.layer_count = layerCount;
    request.audit_mode = auditMode;
    request.tokens = 128u;
    request.model_width = 1024u;
    request.head_count = PROM_M43_HEAD_COUNT;
    request.head_dim = 128u;
    request.ffn_width = 4096u;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.projection_path = PROM_M47_PROJECTION_COOPERATIVE;
    request.attention_strategy = PROM_M43_STRATEGY_PROJECTION_GROUPED;
    request.output_projection_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request.rmsnorm_strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    request.gating_strategy = PROM_M47_GATING_FUSED_DIRECT_PACKED;
    request.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_DOWN;
    request.activation_strategy = PROM_M48_ACTIVATION_PING_PONG;
    request.submit_topology = PROM_M48_SUBMIT_ONE_STACK;
    request.optional_final_readback = 1u;
    request.expected_initial_generation = 48001u;
    request.initial_content_hash = 48002u;
    request.resident_initial_activation.buffer =
        reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(48003u));
    request.resident_initial_activation.byte_length =
        static_cast<VkDeviceSize>(request.tokens) * request.model_width * sizeof(float);
    request.resident_initial_activation.element_type = PROM_DEVICE_ELEMENT_F32;
    request.resident_initial_activation.logical_rows = request.tokens;
    request.resident_initial_activation.logical_columns = request.model_width;
    request.resident_initial_activation.row_stride_elements = request.model_width;
    request.resident_initial_activation.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    request.resident_initial_activation.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    request.resident_initial_activation.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    request.resident_initial_activation.owning_device =
        reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(48004u));
    request.resident_initial_activation.owning_lifetime_id = request.expected_initial_generation;
    request.resident_initial_activation.owning_slot_id = 0u;
    request.resident_initial_activation.owning_slot_generation = 1u;
    for (std::uint32_t layer = 0u; layer < layerCount; ++layer) {
        for (std::uint32_t resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource) {
            request.layer[layer].generation[resource] = 50000u + layer * 100u + resource;
            request.layer[layer].content_hash[resource] = 60000u + layer * 100u + resource;
        }
    }
    return request;
}
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

FACT(PrometheusM43BoundedGroupedAttentionContracts)
{
    prom_m43_plan_request request{};
    FillGroupPlanRequest(&request, 128u, 1024u, 128u,
                         PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X);
    prom_m43_attention_plan grouped{};
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &grouped),
                 "the fixed eight-head grouped plan builds");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT, grouped.head_count, "head count is fixed at eight");
    ASSERT_EQUAL(1u, grouped.submit_count, "normal grouped execution has one submit");
    ASSERT_EQUAL(1u, grouped.shared_x_conversion_count, "host X is converted once");
    ASSERT_EQUAL(1u, grouped.shared_x_upload_count, "host X is uploaded once");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT, grouped.shared_x_consumer_count,
                 "the single shared X has eight consumers");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT,
                 grouped.persistent_weight_count, "24 persistent weights are explicit");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT,
                 grouped.qkv_projection_dispatch_count, "all 24 Q/K/V projections are SGEMMs");
    ASSERT_EQUAL(0u, grouped.intermediate_host_copy_count,
                 "the grouped plan has no intermediate host copy");
    ASSERT_EQUAL(1u, grouped.final_readback_count, "the grouped correctness path has one readback");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M43_OUTPUT_HEAD_MAJOR), grouped.output_layout,
                 "the bounded physical aggregate is head-major");
    ASSERT_TRUE(grouped.memory.shared_x_f32_bytes > 0u && grouped.memory.shared_x_packed_bytes > 0u,
                "one FP32 and one packed shared-X representation are sized");
    ASSERT_TRUE(grouped.memory.persistent_weight_packed_bytes > grouped.memory.shared_x_packed_bytes,
                "the 24 packed weights dominate the one shared packed X");
    ASSERT_TRUE(grouped.memory.exact_retained_bytes <= grouped.memory.capacity_limit_bytes,
                "the primary case fits the bounded capacity");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0u),
                 prom_m43_output_index(0u, 0u, 0u, 128u, 128u),
                 "head-major output starts at head zero");
    ASSERT_EQUAL(static_cast<std::uint64_t>(128u * 128u),
                 prom_m43_output_index(1u, 0u, 0u, 128u, 128u),
                 "head one follows one complete head range");
    ASSERT_EQUAL(UINT64_MAX, prom_m43_output_index(8u, 0u, 0u, 128u, 128u),
                 "out-of-range heads reject");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_STAGE_UPLOAD_X), grouped.stages[0].operation,
                 "shared X preparation is the first command stage");
    for (std::uint32_t index = 1u; index <= PROM_M43_HEAD_COUNT; ++index) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_STAGE_PROJECT_Q), grouped.stages[index].operation,
                     "projection-grouped execution records all Q projections first");
    }
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_STAGE_FINAL_READBACK),
                 grouped.stages[grouped.stage_count - 1u].operation,
                 "grouped readback follows every head");
    for (std::uint32_t index = 0u; index < grouped.stage_count; ++index) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED),
                     grouped.stages[index].source_queue_family,
                     "no grouped stage transfers queue ownership");
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED),
                     grouped.stages[index].destination_queue_family,
                     "one compute queue owns every grouped stage");
    }

    prom_m43_attention_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &replay),
                 "identical grouped requests replan");
    ASSERT_EQUAL(grouped.aggregate_replay_id, replay.aggregate_replay_id,
                 "aggregate replay identity is deterministic");
    ASSERT_EQUAL(grouped.command_plan_replay_id, replay.command_plan_replay_id,
                 "command trace identity is deterministic");
    const auto originalHeadReplay = grouped.head_replay_id;
    request.weight_generation[3u][PROM_M43_WEIGHT_K] += 1u;
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &replay),
                 "one head's Wk generation can change independently");
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        if (head == 3u) {
            ASSERT_TRUE(replay.head_replay_id[head] != originalHeadReplay[head],
                        "the replaced head gets a fresh replay identity");
        } else {
            ASSERT_EQUAL(originalHeadReplay[head], replay.head_replay_id[head],
                         "unrelated heads retain their replay identity");
        }
    }
    ASSERT_TRUE(replay.aggregate_replay_id != grouped.aggregate_replay_id,
                "the aggregate replay includes every independent generation");

    FillGroupPlanRequest(&request, 128u, 1024u, 128u,
                         PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_RESIDENT_X);
    request.rollback_active[2u] = 1u;
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &replay),
                 "one head can take an independent same-precision fallback");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16),
                 replay.selected_path[2u], "the rollback head selects conventional FP16");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_COOPERATIVE),
                 replay.selected_path[1u], "an unrelated head remains cooperative");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M43_INELIGIBLE_ROLLBACK), replay.eligibility.reason,
                 "the aggregate eligibility predicate records the rollback head mask");

    FillGroupPlanRequest(&request, 128u, 1024u, 128u,
                         PROM_M43_STRATEGY_COMPLETE_HEADS, PROM_M42_INPUT_RESIDENT_X);
    prom_m43_attention_plan completeHeads{};
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &completeHeads),
                 "complete-head ordering is a second bounded one-submit plan");
    ASSERT_EQUAL(1u, completeHeads.submit_count, "alternate grouped ordering still submits once");
    ASSERT_EQUAL(0u, completeHeads.shared_x_upload_count, "resident X has no warm upload");
    ASSERT_TRUE(completeHeads.barrier_call_count > grouped.barrier_call_count,
                "projection grouping collapses Q/K/V visibility into fewer barrier calls");

    request.execution_strategy = PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42;
    prom_m43_attention_plan sequential{};
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &sequential),
                 "the eight-M42 baseline has a deterministic plan");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT, sequential.submit_count,
                 "the baseline preserves eight queue submits");

    FillGroupPlanRequest(&request, 127u, 1001u, 127u,
                         PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_RESIDENT_X);
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &replay),
                 "awkward grouped shapes use bounded padding");
    ASSERT_EQUAL(128u, replay.padded_tokens, "awkward Tokens round to 128");
    ASSERT_EQUAL(1008u, replay.padded_model_width, "awkward ModelWidth rounds to 1008");
    ASSERT_EQUAL(128u, replay.padded_head_dim, "awkward HeadDim rounds to 128");

    request.head_count = 7u;
    ASSERT_TRUE(prom_m43_attention_plan_build(&request, &replay) != PROM_OK,
                "arbitrary head counts reject");
    FillGroupPlanRequest(&request, 1024u, 4096u, 1024u,
                         PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_RESIDENT_X);
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&request, &replay),
                 "oversized but arithmetically valid requests retain an audit plan");
    ASSERT_TRUE(replay.memory.exact_retained_bytes > replay.memory.capacity_limit_bytes,
                "the exact memory model identifies the capacity overflow");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M43_INELIGIBLE_CAPACITY), replay.eligibility.reason,
                 "capacity rejection has an exact pre-submission reason");

    prom_m43_eligibility_facts facts{};
    facts.head_count = PROM_M43_HEAD_COUNT;
    facts.cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE;
    facts.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    facts.tokens = 128u; facts.model_width = 1024u; facts.head_dim = 128u;
    facts.padding_supported = 1u;
    facts.persistent_weight_count = PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
    facts.shared_x_available = 1u; facts.generations_valid = 1u;
    facts.required_capacity_bytes = 1u; facts.available_capacity_bytes = 2u;
    prom_m43_eligibility_decision decision{};
    prom_m43_eligibility_evaluate(&facts, &decision);
    ASSERT_EQUAL(0u, decision.eligible, "extension absence is selector-ineligible");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M43_INELIGIBLE_CAPABILITY), decision.reason,
                 "the exact grouped fallback reason is retained");
}

FACT(PrometheusM43GroupedCpuOracleAndMismatchLocalization)
{
    constexpr std::uint32_t tokens = 4u;
    constexpr std::uint32_t modelWidth = 8u;
    constexpr std::uint32_t headDim = 4u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    std::vector<float> expected;
    const prom_m43_reference_result reference =
        GroupReference(x, weights, tokens, modelWidth, headDim, &expected);
    ASSERT_EQUAL(1u, reference.all_finite, "all eight CPU heads remain finite");
    ASSERT_TRUE(reference.minimum_probability_row_sum > 0.999f &&
                reference.maximum_probability_row_sum < 1.001f,
                "every grouped probability row sums approximately to one");

    prom_m43_plan_request planRequest{};
    FillGroupPlanRequest(&planRequest, tokens, modelWidth, headDim,
                         PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_RESIDENT_X);
    prom_m43_attention_plan plan{};
    ASSERT_EQUAL(PROM_OK, prom_m43_attention_plan_build(&planRequest, &plan),
                 "oracle comparison plan builds");
    prom_m43_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m43_attention_compare(expected.data(), expected.data(), PROM_M43_HEAD_COUNT,
                                             tokens, headDim, 1.0e-6f, 1.0e-6f, &plan, &mismatch),
                 "identical grouped outputs compare");
    std::vector<float> actual = expected;
    actual[prom_m43_output_index(5u, 2u, 1u, tokens, headDim)] += 1.0f;
    ASSERT_TRUE(prom_m43_attention_compare(expected.data(), actual.data(), PROM_M43_HEAD_COUNT,
                                           tokens, headDim, 1.0e-6f, 1.0e-6f,
                                           &plan, &mismatch) != PROM_OK,
                "a grouped mismatch is surfaced");
    ASSERT_EQUAL(5u, mismatch.head_index, "the first mismatching head is explicit");
    ASSERT_EQUAL(2u, mismatch.stage_mismatch.row, "the first mismatching row is explicit");
    ASSERT_EQUAL(1u, mismatch.stage_mismatch.column, "the first mismatching column is explicit");
    ASSERT_EQUAL(plan.head_replay_id[5u], mismatch.head_replay_id,
                 "mismatch evidence carries the per-head replay identity");
    ASSERT_EQUAL(plan.aggregate_replay_id, mismatch.aggregate_replay_id,
                 "mismatch evidence carries the aggregate replay identity");
}

FACT(PrometheusM44OutputProjectionContracts)
{
    prom_m44_plan_request request{};
    request.head_count = PROM_M44_HEAD_COUNT;
    request.tokens = 128u;
    request.head_dim = 128u;
    request.model_width = 1024u;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.aggregation_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request.projection_path = PROM_M44_PROJECTION_COOPERATIVE;
    request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
    request.cooperative_capability_state = PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED;
    request.wo_generation = 700u;
    request.wo_hash = 701u;
    request.m43_aggregate_replay_id = 702u;
    const VkDevice device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(101u));
    for (std::uint32_t head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
        prom_device_buffer_view& view = request.head_views[head];
        view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(head + 1u));
        view.byte_length = static_cast<VkDeviceSize>(128u * 128u * sizeof(float));
        view.element_type = PROM_DEVICE_ELEMENT_F32;
        view.logical_rows = 128u;
        view.logical_columns = 128u;
        view.row_stride_elements = 128u;
        view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
        view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
        view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
        view.owning_device = device;
        view.owning_lifetime_id = 900u;
        view.owning_slot_id = 1u;
        view.owning_slot_generation = 4u;
    }
    prom_m44_output_projection_plan interleave{};
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &interleave),
                 "the primary eight-view interleave plan builds");
    ASSERT_EQUAL(1024u, interleave.concatenated_width,
                 "eight HeadDim=128 views concatenate to logical width 1024");
    ASSERT_EQUAL(1024u, interleave.output_row_stride,
                 "the aligned cooperative Y stride is explicit");
    ASSERT_EQUAL(1u, interleave.eligibility.eligible,
                 "the complete primary cooperative plan is eligible");
    ASSERT_EQUAL(0u, interleave.intermediate_host_copy_count,
                 "the device interleave has no host concatenate");
    ASSERT_EQUAL(1u, interleave.final_readback_count,
                 "the device plan has one final Y readback");
    ASSERT_TRUE(interleave.memory.contiguous_packed_bytes > 0u &&
                interleave.memory.contiguous_f32_bytes == 0u,
                "packed interleave retains only the selected concatenation representation");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_STAGE_HEADS_READY),
                 interleave.stages[0].operation,
                 "all eight producer views become visible before aggregation");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_STAGE_FINAL_READBACK),
                 interleave.stages[interleave.stage_count - 1u].operation,
                 "final readback remains last");
    ASSERT_EQUAL(PROM_M44_HEAD_COUNT, interleave.stages[0].barrier_buffer_count,
                 "the producer barrier names all eight head ranges");
    for (std::uint32_t stage = 0u; stage < interleave.stage_count; ++stage) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED),
                     interleave.stages[stage].source_queue_family,
                     "M44 never transfers queue ownership");
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED),
                     interleave.stages[stage].destination_queue_family,
                     "M44 retains the shared compute queue");
    }
    prom_m44_output_projection_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &replay),
                 "the identical plan rebuilds");
    ASSERT_EQUAL(interleave.replay_id, replay.replay_id,
                 "M44 replay identity is deterministic");
    request.wo_generation += 1u;
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &replay),
                 "a newer Wo generation replans");
    ASSERT_TRUE(interleave.replay_id != replay.replay_id,
                "Wo generation participates in replay identity");
    request.wo_generation -= 1u;

    request.aggregation_strategy = PROM_M44_AGGREGATION_DIRECT_SEGMENTED;
    request.projection_path = PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16;
    prom_m44_output_projection_plan direct{};
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &direct),
                 "the bounded direct segmented plan builds");
    ASSERT_EQUAL(0u, direct.memory.contiguous_packed_bytes,
                 "direct projection materializes no full concatenate buffer");
    ASSERT_EQUAL(0u, direct.memory.partial_output_bytes,
                 "the selected direct route needs no eight-output partial tensor");
    ASSERT_EQUAL(1u, direct.dispatch_count,
                 "direct aggregation and projection are one bounded dispatch");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_STAGE_DIRECT_PROJECTION),
                 direct.stages[1].operation,
                 "the direct stage is explicit in the command trace");

    request.head_views[1].buffer = request.head_views[0].buffer;
    prom_m44_output_projection_plan overlap{};
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &overlap),
                 "overlap is represented as a deterministic eligibility result");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_INELIGIBLE_VIEW_OVERLAP),
                 overlap.eligibility.reason, "overlapping head ranges reject exactly");
    request.head_views[1].buffer =
        reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(2u));
    request.head_views[2].owning_slot_generation = 0u;
    ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_plan_build(&request, &overlap),
                 "a stale view remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_INELIGIBLE_VIEW_GENERATION),
                 overlap.eligibility.reason, "stale physical identity has an exact generation reason");
    request.head_views[2].owning_slot_generation = 4u;
    request.head_count = 7u;
    ASSERT_TRUE(prom_m44_output_projection_plan_build(&request, &overlap) != PROM_OK,
                "M44 never generalizes beyond exactly eight heads");
}

FACT(PrometheusM44ConcatenationAndCpuOracle)
{
    constexpr std::uint32_t tokens = 3u;
    constexpr std::uint32_t headDim = 2u;
    constexpr std::uint32_t modelWidth = 5u;
    std::vector<float> heads(static_cast<std::size_t>(PROM_M44_HEAD_COUNT) * tokens * headDim);
    for (std::size_t index = 0u; index < heads.size(); ++index) {
        heads[index] = static_cast<float>(static_cast<int>(index) - 17) / 17.0f;
    }
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    std::vector<float> roundedOutput;
    std::vector<float> concatenated;
    const prom_m44_reference_result rounded =
        OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                  PROM_M42_PRECISION_F16_ROUNDED,
                                  &roundedOutput, &concatenated);
    ASSERT_EQUAL(1u, rounded.all_finite, "the rounded M44 oracle remains finite");
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
            for (std::uint32_t column = 0u; column < headDim; ++column) {
                const std::uint64_t source =
                    (static_cast<std::uint64_t>(head) * tokens + token) * headDim + column;
                const std::uint64_t destination =
                    prom_m44_concat_index(token, head, column, tokens, headDim);
                const float expected = prom_sgemm_fp16_bits_to_float32(
                    prom_sgemm_float32_to_fp16_bits(heads[source]));
                ASSERT_NEAR(expected, concatenated[destination], 0.0f,
                            "token-major C[token,head*HeadDim+column] is exact after rounding");
            }
        }
    }
    ASSERT_EQUAL(UINT64_MAX, prom_m44_concat_index(tokens, 0u, 0u, tokens, headDim),
                 "out-of-range concatenation coordinates reject");
    std::vector<float> exactOutput;
    const prom_m44_reference_result exact =
        OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                  PROM_M42_PRECISION_FP32, &exactOutput);
    ASSERT_EQUAL(1u, exact.all_finite, "the FP32 comparison oracle remains finite");
    ASSERT_TRUE(roundedOutput != exactOutput,
                "the reduced and exact precision contracts are not silently conflated");
    prom_m44_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedOutput.data(), roundedOutput.data(),
                                                     tokens, modelWidth, 1.0e-6f, 1.0e-6f,
                                                     PROM_M44_AGGREGATION_INTERLEAVE,
                                                     7u, 11u, 13u, &mismatch),
                 "identical projected outputs compare");
    std::vector<float> actual = roundedOutput;
    actual[static_cast<std::size_t>(2u) * modelWidth + 3u] += 1.0f;
    ASSERT_TRUE(prom_m44_output_projection_compare(roundedOutput.data(), actual.data(),
                                                   tokens, modelWidth, 1.0e-6f, 1.0e-6f,
                                                   PROM_M44_AGGREGATION_DIRECT_SEGMENTED,
                                                   7u, 11u, 13u, &mismatch) != PROM_OK,
                "the first M44 mismatch is surfaced");
    ASSERT_EQUAL(2u, mismatch.token, "mismatch token is explicit");
    ASSERT_EQUAL(3u, mismatch.output_column, "mismatch output column is explicit");
    ASSERT_EQUAL(7u, mismatch.wo_generation, "mismatch carries the Wo generation");
    ASSERT_EQUAL(11u, mismatch.m43_aggregate_replay_id,
                 "mismatch carries the M43 aggregate replay identity");
    ASSERT_EQUAL(13u, mismatch.m44_replay_id,
                 "mismatch carries the M44 replay identity");
}

FACT(PrometheusM45ResidualOwnershipContracts)
{
    const VkDevice device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(45u));
    prom_m45_plan_request request{};
    request.tokens = 127u;
    request.model_width = 1001u;
    request.strategy = PROM_M45_STRATEGY_SEPARATE_OUTPUT;
    request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
    request.precision_policy = PROM_M45_PRECISION_FP32;
    request.y_exclusive = 1u;
    request.final_readback = 1u;
    request.expected_x_generation = 71u;
    request.expected_y_generation = 72u;
    request.m44_replay_id = 73u;
    request.x_view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(101u));
    request.x_view.byte_length = static_cast<VkDeviceSize>(127u * 1024u * sizeof(float));
    request.x_view.element_type = PROM_DEVICE_ELEMENT_F32;
    request.x_view.logical_rows = 127u;
    request.x_view.logical_columns = 1001u;
    request.x_view.row_stride_elements = 1024u;
    request.x_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    request.x_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    request.x_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    request.x_view.owning_device = device;
    request.x_view.owning_lifetime_id = 71u;
    request.x_view.owning_slot_id = UINT32_MAX;
    request.x_view.owning_slot_generation = 9u;
    request.y_view = request.x_view;
    request.y_view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(102u));
    request.y_view.byte_length = static_cast<VkDeviceSize>(127u * 1008u * sizeof(float));
    request.y_view.row_stride_elements = 1008u;
    request.y_view.owning_lifetime_id = 72u;
    request.y_view.owning_slot_id = 1u;
    request.y_view.owning_slot_generation = 10u;
    prom_m45_residual_plan separate{};
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &separate),
                 "the awkward-stride separate residual plan builds");
    ASSERT_EQUAL(1u, separate.eligibility.eligible,
                 "matching logical shapes may use different physical strides");
    ASSERT_EQUAL(1001u, separate.z_row_stride,
                 "separate Z uses one compact explicit stride");
    ASSERT_EQUAL(1u, separate.dispatch_count, "residual addition is one dispatch");
    ASSERT_EQUAL(0u, separate.intermediate_host_copy_count,
                 "the residual plan has no intermediate host copy");
    ASSERT_EQUAL(1u, separate.final_readback_count, "one optional final Z readback is explicit");
    ASSERT_EQUAL(4u, separate.barrier_count,
                 "X, Y, Z, and readback each have one normalized exact barrier");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT),
                 separate.barriers[1].destination_access_mask,
                 "separate Y is read-only at the residual boundary");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_QUEUE_FAMILY_IGNORED),
                 separate.barriers[2].source_queue_family,
                 "residual barriers never transfer queue ownership");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 1024u * sizeof(float)),
                 separate.barriers[0].byte_length, "X's padded physical range is exact");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 1008u * sizeof(float)),
                 separate.barriers[1].byte_length, "Y's independent physical range is exact");

    request.strategy = PROM_M45_STRATEGY_IN_PLACE_Y;
    request.submit_policy = PROM_M45_SUBMIT_TWO_BOUNDED;
    prom_m45_residual_plan inPlace{};
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &inPlace),
                 "the exclusive in-place-Y residual plan builds");
    ASSERT_EQUAL(1u, inPlace.eligibility.eligible, "exclusive Y ownership is mechanically accepted");
    ASSERT_EQUAL(1008u, inPlace.z_row_stride, "Y's physical stride becomes Z's stride");
    ASSERT_EQUAL(0u, inPlace.memory.z_device_bytes, "in-place Y allocates no separate Z");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 1001u * sizeof(float)),
                 inPlace.memory.in_place_y_saved_bytes, "saved separate-Z bytes are exact");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT),
                 inPlace.barriers[1].destination_access_mask,
                 "in-place Y has a read/write destination access mask");
    ASSERT_EQUAL(2u, inPlace.submit_count, "the bounded split has exactly two submits");
    ASSERT_TRUE(inPlace.z_generation != inPlace.y_generation,
                "unchanged physical storage receives a distinct post-residual content generation");
    ASSERT_TRUE(inPlace.replay_id != separate.replay_id,
                "strategy, aliasing, and submit topology participate in replay identity");
    prom_m45_residual_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &replay),
                 "the identical in-place plan rebuilds");
    ASSERT_EQUAL(inPlace.replay_id, replay.replay_id, "M45 replay identity is deterministic");

    request.final_readback = 0u;
    prom_m45_residual_plan retainedOnly{};
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &retainedOnly),
                 "a retained-only Z plan builds");
    ASSERT_EQUAL(0u, retainedOnly.final_readback_count, "final Z readback is optional");
    ASSERT_EQUAL(3u, retainedOnly.barrier_count,
                 "retained-only Z ends in an exact compute-consumer barrier");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT),
                 retainedOnly.barriers[2].destination_access_mask,
                 "retained Z is ready for the next compute consumer");
}

FACT(PrometheusM45ResidualValidationAndAliasing)
{
    const VkDevice device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(51u));
    prom_m45_plan_request request{};
    request.tokens = 4u;
    request.model_width = 5u;
    request.strategy = PROM_M45_STRATEGY_IN_PLACE_Y;
    request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
    request.precision_policy = PROM_M45_PRECISION_FP32;
    request.y_exclusive = 1u;
    request.expected_x_generation = 11u;
    request.expected_y_generation = 12u;
    request.m44_replay_id = 13u;
    auto fillView = [device](prom_device_buffer_view* view, std::uintptr_t handle,
                             std::uint32_t stride, std::uint64_t generation) {
        *view = {};
        view->buffer = reinterpret_cast<VkBuffer>(handle);
        view->byte_length = static_cast<VkDeviceSize>(4u * stride * sizeof(float));
        view->element_type = PROM_DEVICE_ELEMENT_F32;
        view->logical_rows = 4u;
        view->logical_columns = 5u;
        view->row_stride_elements = stride;
        view->layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
        view->producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
        view->required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
        view->owning_device = device;
        view->owning_lifetime_id = generation;
        view->owning_slot_id = 1u;
        view->owning_slot_generation = 2u;
    };
    fillView(&request.x_view, 201u, 7u, 11u);
    fillView(&request.y_view, 202u, 8u, 12u);
    prom_m45_residual_plan plan{};
    request.y_exclusive = 0u;
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "missing exclusivity remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_EXCLUSIVITY),
                 plan.eligibility.reason, "in-place Y rejects without exclusive ownership");
    request.y_exclusive = 1u;
    request.pre_residual_y_consumer_count = 1u;
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "a live pre-residual Y consumer remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_EXCLUSIVITY),
                 plan.eligibility.reason, "in-place Y rejects another pre-residual consumer");
    request.pre_residual_y_consumer_count = 0u;
    request.x_view.buffer = request.y_view.buffer;
    request.x_view.offset = sizeof(float);
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "partial overlap remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_ALIAS),
                 plan.eligibility.reason, "partial X/Y overlap rejects exactly");
    fillView(&request.x_view, 201u, 7u, 11u);
    request.y_view.owning_lifetime_id = 99u;
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "a stale Y generation remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_GENERATION),
                 plan.eligibility.reason, "stale Y rejects before submission");
    request.y_view.owning_lifetime_id = 12u;
    request.y_view.owning_device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(52u));
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "a cross-device view remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_DEVICE),
                 plan.eligibility.reason, "cross-device residual views reject exactly");
    request.y_view.owning_device = device;
    request.y_view.row_stride_elements = 4u;
    request.y_view.byte_length = 4u * 4u * sizeof(float);
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "an insufficient physical stride remains inspectable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_STRIDE),
                 plan.eligibility.reason, "stride smaller than logical width rejects exactly");
    fillView(&request.y_view, 202u, 8u, 12u);
    request.strategy = PROM_M45_STRATEGY_IN_PLACE_X_AUDIT;
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_plan_build(&request, &plan),
                 "the bounded in-place-X audit produces a decision");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M45_INELIGIBLE_IN_PLACE_X),
                 plan.eligibility.reason,
                 "in-place X is rejected because resident X remains shared and immutable");
    request.tokens = 1025u;
    ASSERT_TRUE(prom_m45_residual_plan_build(&request, &plan) != PROM_OK,
                "out-of-envelope sizing rejects before capacity or resource work");
}

FACT(PrometheusM45ResidualCpuOracleAndMismatch)
{
    constexpr std::uint32_t tokens = 3u;
    constexpr std::uint32_t width = 5u;
    constexpr std::uint32_t xStride = 7u;
    constexpr std::uint32_t yStride = 8u;
    constexpr std::uint32_t zStride = 6u;
    std::vector<float> x(tokens * xStride, -99.0f);
    std::vector<float> y(tokens * yStride, -88.0f);
    std::vector<float> z(tokens * zStride, 77.0f);
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            x[token * xStride + column] = static_cast<float>(token * 10u + column) / 8.0f;
            y[token * yStride + column] = -static_cast<float>(token * 3u + column) / 16.0f;
        }
    }
    prom_m45_reference_request reference{};
    reference.x = x.data();
    reference.y = y.data();
    reference.z = z.data();
    reference.x_element_count = x.size();
    reference.y_element_count = y.size();
    reference.z_element_count = z.size();
    reference.tokens = tokens;
    reference.model_width = width;
    reference.x_row_stride = xStride;
    reference.y_row_stride = yStride;
    reference.z_row_stride = zStride;
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_cpu_reference(&reference),
                 "the FP32 residual oracle supports independent strides");
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            ASSERT_NEAR(x[token * xStride + column] + y[token * yStride + column],
                        z[token * zStride + column], 0.0f,
                        "the logical residual element is exact FP32 addition");
        }
        ASSERT_NEAR(77.0f, z[token * zStride + width], 0.0f,
                    "the CPU oracle leaves padding deterministic and untouched");
    }
    std::vector<float> compact(tokens * width);
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            compact[token * width + column] = z[token * zStride + column];
        }
    }
    prom_m45_residual_plan plan{};
    plan.strategy = PROM_M45_STRATEGY_IN_PLACE_Y;
    plan.x_generation = 11u;
    plan.y_generation = 12u;
    plan.z_generation = 13u;
    plan.m44_replay_id = 14u;
    plan.replay_id = 15u;
    prom_m45_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK, prom_m45_residual_compare(compact.data(), compact.data(), tokens, width,
                                                    0.0f, 0.0f, &plan, &mismatch),
                 "identical residual output compares");
    std::vector<float> actual = compact;
    actual[2u * width + 3u] += 1.0f;
    ASSERT_TRUE(prom_m45_residual_compare(compact.data(), actual.data(), tokens, width,
                                          0.0f, 0.0f, &plan, &mismatch) != PROM_OK,
                "the first residual mismatch is surfaced");
    ASSERT_EQUAL(2u, mismatch.token, "mismatch token is explicit");
    ASSERT_EQUAL(3u, mismatch.column, "mismatch column is explicit");
    ASSERT_EQUAL(11u, mismatch.x_generation, "mismatch carries X generation");
    ASSERT_EQUAL(12u, mismatch.y_generation, "mismatch carries Y generation");
    ASSERT_EQUAL(13u, mismatch.z_generation, "mismatch carries Z generation");
    ASSERT_EQUAL(14u, mismatch.m44_replay_id, "mismatch carries M44 replay identity");
    ASSERT_EQUAL(15u, mismatch.m45_replay_id, "mismatch carries M45 replay identity");
}

FACT(PrometheusM46RmsNormPlanningAndOwnershipContracts)
{
    const VkDevice device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(61u));
    prom_m46_plan_request request{};
    request.tokens = 127u;
    request.model_width = 1001u;
    request.epsilon = 1.0e-5f;
    request.strategy = PROM_M46_STRATEGY_SEPARATE_OUTPUT;
    request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
    request.z_exclusive = 1u;
    request.final_readback = 1u;
    request.expected_z_generation = 81u;
    request.weight_generation = 82u;
    request.weight_hash = 83u;
    request.m45_replay_id = 84u;
    request.z_view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(301u));
    request.z_view.byte_length = static_cast<VkDeviceSize>(127u * 1024u * sizeof(float));
    request.z_view.element_type = PROM_DEVICE_ELEMENT_F32;
    request.z_view.logical_rows = 127u;
    request.z_view.logical_columns = 1001u;
    request.z_view.row_stride_elements = 1024u;
    request.z_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    request.z_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    request.z_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    request.z_view.owning_device = device;
    request.z_view.owning_lifetime_id = 81u;
    request.z_view.owning_slot_id = 1u;
    request.z_view.owning_slot_generation = 9u;

    prom_m46_rmsnorm_plan separate{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &separate),
                 "awkward fused RMSNorm planning succeeds");
    ASSERT_EQUAL(1u, separate.eligibility_eligible, "the valid retained Z view is eligible");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M46_REDUCTION_FUSED),
                 separate.reduction_plan, "width at most 1024 uses one row reduction");
    ASSERT_EQUAL(2u, separate.dispatch_count, "fused reduction plus apply is two dispatches");
    ASSERT_EQUAL(0u, separate.intermediate_host_copy_count,
                 "RMSNorm planning never inserts an intermediate host copy");
    ASSERT_EQUAL(1u, separate.final_readback_count, "one final N readback is explicit");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * sizeof(float)),
                 separate.memory.inv_rms_bytes, "explicit InvRms storage is exact");
    ASSERT_EQUAL(0u, separate.memory.partial_sum_bytes,
                 "the fused plan allocates no partial sum tensor");
    ASSERT_EQUAL(1001u, separate.n_row_stride, "separate N is compact");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT),
                 separate.barriers[2].destination_access_mask,
                 "separate apply keeps Z read-only");
    ASSERT_EQUAL(0u, separate.intermediate_host_copy_count,
                 "padding is excluded without a host compaction step");

    request.requested_reduction_plan = PROM_M46_REDUCTION_FORCE_STAGED;
    prom_m46_rmsnorm_plan forcedStaged{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &forcedStaged),
                 "a staged audit may be forced where fused is also legal");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M46_REDUCTION_STAGED),
                 forcedStaged.reduction_plan, "the forced staged audit is explicit");
    ASSERT_EQUAL(3u, forcedStaged.dispatch_count,
                 "forced staged execution adds one final reduction dispatch");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * sizeof(float)),
                 forcedStaged.memory.partial_sum_bytes,
                 "one partial per awkward row is exact at the comparison boundary");
    request.requested_reduction_plan = PROM_M46_REDUCTION_AUTO;

    request.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    request.submit_policy = PROM_M46_SUBMIT_TWO_BOUNDED;
    prom_m46_rmsnorm_plan inPlace{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &inPlace),
                 "exclusive in-place Z RMSNorm planning succeeds");
    ASSERT_EQUAL(1024u, inPlace.n_row_stride, "in-place N retains Z's physical stride");
    ASSERT_EQUAL(0u, inPlace.memory.n_device_bytes, "in-place normalization allocates no full N");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 1001u * sizeof(float)),
                 inPlace.memory.in_place_saved_bytes, "saved compact N bytes are exact");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT),
                 inPlace.barriers[2].destination_access_mask,
                 "all reduction reads precede the destructive in-place apply");
    ASSERT_EQUAL(2u, inPlace.submit_count, "split execution has exactly two submits");
    ASSERT_TRUE(inPlace.n_generation != inPlace.z_generation,
                "an aliased physical buffer receives a new N content generation");
    prom_m46_rmsnorm_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &replay),
                 "identical M46 planning repeats");
    ASSERT_EQUAL(inPlace.replay_id, replay.replay_id, "M46 replay identity is deterministic");

    request.model_width = 4096u;
    request.z_view.logical_columns = 4096u;
    request.z_view.row_stride_elements = 4096u;
    request.z_view.byte_length = static_cast<VkDeviceSize>(127u * 4096u * sizeof(float));
    prom_m46_rmsnorm_plan staged{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &staged),
                 "4096-wide staged RMSNorm planning succeeds");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M46_REDUCTION_STAGED),
                 staged.reduction_plan, "width above 1024 uses staged reduction");
    ASSERT_EQUAL(4u, staged.partials_per_row, "4096 width has four deterministic partials");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 4u * sizeof(float)),
                 staged.memory.partial_sum_bytes, "staged partial storage is exact");
    ASSERT_EQUAL(3u, staged.dispatch_count, "partial, final, and apply are three dispatches");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M46_BUFFER_PARTIALS),
                 staged.barriers[1].buffer_identity, "the staged dependency names partial sums");
}

FACT(PrometheusM46RmsNormValidationOracleAndMismatch)
{
    constexpr std::uint32_t tokens = 3u;
    constexpr std::uint32_t width = 5u;
    constexpr std::uint32_t zStride = 8u;
    constexpr std::uint32_t nStride = 7u;
    std::vector<float> z(tokens * zStride, 99.0f);
    std::vector<float> weight{1.0f, 0.5f, 1.5f, -1.0f, 2.0f};
    std::vector<float> n(tokens * nStride, 77.0f);
    std::vector<float> invRms(tokens, 0.0f);
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            const int value = static_cast<int>(token * width + column) - 6;
            z[token * zStride + column] = static_cast<float>(value) / 8.0f;
        }
    }
    prom_m46_reference_request reference{};
    reference.z = z.data();
    reference.weight = weight.data();
    reference.n = n.data();
    reference.inv_rms = invRms.data();
    reference.z_element_count = z.size();
    reference.weight_element_count = weight.size();
    reference.n_element_count = n.size();
    reference.tokens = tokens;
    reference.model_width = width;
    reference.z_row_stride = zStride;
    reference.n_row_stride = nStride;
    reference.epsilon = 1.0e-5f;
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&reference),
                 "the FP32 RMSNorm oracle supports padded independent strides");
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        ASSERT_TRUE(std::isfinite(invRms[token]), "each row receives one finite InvRms");
        ASSERT_NEAR(77.0f, n[token * nStride + width], 0.0f,
                    "the RMSNorm oracle leaves padding untouched");
    }
    std::vector<float> compact(tokens * width);
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t column = 0u; column < width; ++column) {
            compact[token * width + column] = n[token * nStride + column];
        }
    }
    prom_m46_rmsnorm_plan plan{};
    plan.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    plan.epsilon = reference.epsilon;
    plan.z_generation = 21u;
    plan.weight_generation = 22u;
    plan.n_generation = 23u;
    plan.m45_replay_id = 24u;
    plan.replay_id = 25u;
    prom_m46_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_compare(compact.data(), compact.data(), tokens, width,
                                                   0.0f, 0.0f, &plan, nullptr,
                                                   invRms.data(), &mismatch),
                 "identical normalized output compares");
    std::vector<float> actual = compact;
    actual[2u * width + 3u] += 1.0f;
    ASSERT_TRUE(prom_m46_rmsnorm_compare(compact.data(), actual.data(), tokens, width,
                                         0.0f, 0.0f, &plan, nullptr,
                                         invRms.data(), &mismatch) != PROM_OK,
                "the first row-local RMSNorm mismatch is surfaced");
    ASSERT_EQUAL(2u, mismatch.token, "mismatch token is explicit");
    ASSERT_EQUAL(3u, mismatch.column, "mismatch column is explicit");
    ASSERT_EQUAL(21u, mismatch.z_generation, "mismatch carries Z generation");
    ASSERT_EQUAL(22u, mismatch.weight_generation, "mismatch carries Weight generation");
    ASSERT_EQUAL(23u, mismatch.n_generation, "mismatch carries N generation");
    ASSERT_EQUAL(24u, mismatch.m45_replay_id, "mismatch carries M45 replay identity");
    ASSERT_EQUAL(25u, mismatch.m46_replay_id, "mismatch carries M46 replay identity");

    reference.epsilon = 0.0f;
    ASSERT_TRUE(prom_m46_rmsnorm_cpu_reference(&reference) != PROM_OK,
                "non-positive epsilon rejects");
    reference.epsilon = std::numeric_limits<float>::infinity();
    ASSERT_TRUE(prom_m46_rmsnorm_cpu_reference(&reference) != PROM_OK,
                "non-finite epsilon rejects");
    reference.epsilon = 1.0e-5f;
    weight[2] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_TRUE(prom_m46_rmsnorm_cpu_reference(&reference) != PROM_OK,
                "non-finite Weight rejects");
}

FACT(PrometheusM47GatedFfnPlanningOwnershipAndMemory)
{
    const VkDevice device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(71u));
    prom_m47_plan_request request{};
    request.tokens = 127u;
    request.model_width = 1001u;
    request.ffn_width = 3001u;
    request.projection_path = PROM_M47_PROJECTION_COOPERATIVE;
    request.gating_strategy = PROM_M47_GATING_FUSED_DIRECT_PACKED;
    request.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_DOWN;
    request.submit_policy = PROM_M47_SUBMIT_TWO_BOUNDED;
    request.final_readback = 1u;
    request.expected_n_generation = 91u;
    request.m46_replay_id = 92u;
    request.n_view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(401u));
    request.n_view.byte_length = static_cast<VkDeviceSize>(127u * 1024u * sizeof(float));
    request.n_view.element_type = PROM_DEVICE_ELEMENT_F32;
    request.n_view.logical_rows = 127u;
    request.n_view.logical_columns = 1001u;
    request.n_view.row_stride_elements = 1024u;
    request.n_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    request.n_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    request.n_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    request.n_view.owning_device = device;
    request.n_view.owning_lifetime_id = 91u;
    request.n_view.owning_slot_id = 1u;
    request.n_view.owning_slot_generation = 12u;
    for (std::uint32_t weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
        request.weight_generation[weight] = 101u + weight;
        request.weight_hash[weight] = 201u + weight;
    }
    prom_m47_gated_ffn_plan direct{};
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_plan_build(&request, &direct),
                 "awkward direct-packed gated FFN planning succeeds");
    ASSERT_EQUAL(1u, direct.eligibility_eligible, "the retained M46 N view is eligible");
    ASSERT_EQUAL(128u, direct.padded_tokens, "awkward tokens pad exactly");
    ASSERT_EQUAL(1008u, direct.padded_model_width, "awkward model width pads exactly");
    ASSERT_EQUAL(3008u, direct.padded_ffn_width, "awkward FFN width pads exactly");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_HIDDEN_PACKED_F16),
                 direct.hidden_storage, "direct fusion exposes an explicit packed boundary");
    ASSERT_EQUAL(0u, direct.memory.hidden_f32_bytes,
                 "direct-packed fusion never retains redundant FP32 Hidden");
    ASSERT_EQUAL(0u, direct.memory.separate_output_bytes,
                 "in-place Down allocates no second output tensor");
    ASSERT_EQUAL(0u, direct.intermediate_host_copy_count,
                 "the complete FFN plan has no intermediate readback");
    ASSERT_EQUAL(1u, direct.final_readback_count, "one final output readback is explicit");
    ASSERT_EQUAL(2u, direct.submit_count, "the bounded split has exactly two submits");
    ASSERT_TRUE(direct.output_generation != direct.down_generation,
                "in-place Down receives a distinct BlockOutput content generation");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT),
                 direct.barriers[direct.barrier_count - 3u].destination_access_mask,
                 "Down becomes residual read/write only after Wdown completes");

    prom_m47_gated_ffn_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_plan_build(&request, &replay),
                 "identical M47 planning repeats");
    ASSERT_EQUAL(direct.replay_id, replay.replay_id, "M47 replay identity is deterministic");

    request.gating_strategy = PROM_M47_GATING_SEPARATE;
    request.residual_strategy = PROM_M47_RESIDUAL_SEPARATE_OUTPUT;
    prom_m47_gated_ffn_plan separate{};
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_plan_build(&request, &separate),
                 "separate activation/multiply and output planning succeeds");
    ASSERT_TRUE(separate.memory.activated_gate_bytes > 0u,
                "the separate baseline owns one explicit ActivatedGate tensor");
    ASSERT_TRUE(separate.memory.hidden_f32_bytes > 0u && separate.memory.hidden_packed_bytes > 0u,
                "separate reduced execution exposes both required Hidden representations");
    ASSERT_EQUAL(static_cast<std::uint64_t>(127u * 1001u * sizeof(float)),
                 separate.memory.separate_output_bytes, "separate BlockOutput is compact and exact");
    ASSERT_TRUE(separate.dispatch_count > direct.dispatch_count,
                "separate gating adds the activation, multiply, and pack passes");

    request.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT;
    prom_m47_gated_ffn_plan rejected{};
    ASSERT_TRUE(prom_m47_gated_ffn_plan_build(&request, &rejected) != PROM_OK,
                "in-place N is mechanically rejected");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_INELIGIBLE_EXCLUSIVITY),
                 rejected.eligibility_reason, "the rejected N mutation has an exact reason");
}

FACT(PrometheusM47GatedFfnOracleSiluPrecisionAndMismatch)
{
    constexpr std::uint32_t tokens = 2u;
    constexpr std::uint32_t modelWidth = 3u;
    constexpr std::uint32_t ffnWidth = 4u;
    constexpr std::uint32_t nStride = 5u;
    constexpr std::uint32_t outputStride = 6u;
    std::vector<float> n(tokens * nStride, 77.0f);
    n[0] = -2.0f; n[1] = 0.5f; n[2] = 1.0f;
    n[nStride] = 3.0f; n[nStride + 1u] = -1.0f; n[nStride + 2u] = 0.25f;
    std::vector<float> wgate(modelWidth * ffnWidth);
    std::vector<float> wup(modelWidth * ffnWidth);
    std::vector<float> wdown(ffnWidth * modelWidth);
    for (std::size_t index = 0u; index < wgate.size(); ++index) {
        wgate[index] = static_cast<float>(static_cast<int>(index % 7u) - 3) / 8.0f;
        wup[index] = static_cast<float>(static_cast<int>((index * 3u) % 11u) - 5) / 16.0f;
    }
    for (std::size_t index = 0u; index < wdown.size(); ++index) {
        wdown[index] = static_cast<float>(static_cast<int>((index * 5u) % 13u) - 6) / 16.0f;
    }
    std::vector<float> gate(tokens * ffnWidth);
    std::vector<float> up(tokens * ffnWidth);
    std::vector<float> hidden(tokens * ffnWidth);
    std::vector<float> down(tokens * modelWidth);
    std::vector<float> output(tokens * outputStride, 55.0f);
    prom_m47_reference_request reference{};
    reference.n = n.data();
    reference.wgate = wgate.data();
    reference.wup = wup.data();
    reference.wdown = wdown.data();
    reference.gate = gate.data();
    reference.up = up.data();
    reference.hidden = hidden.data();
    reference.down = down.data();
    reference.output = output.data();
    reference.n_element_count = n.size();
    reference.wgate_element_count = wgate.size();
    reference.wup_element_count = wup.size();
    reference.wdown_element_count = wdown.size();
    reference.output_element_count = output.size();
    reference.tokens = tokens;
    reference.model_width = modelWidth;
    reference.ffn_width = ffnWidth;
    reference.n_row_stride = nStride;
    reference.output_row_stride = outputStride;
    reference.projection_path = PROM_M47_PROJECTION_A2X4_FP32;
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_cpu_reference(&reference),
                 "exact FP32 gated FFN oracle succeeds with independent strides");
    for (float value : gate) ASSERT_TRUE(std::isfinite(value), "Gate remains finite");
    for (float value : up) ASSERT_TRUE(std::isfinite(value), "Up remains finite");
    for (float value : hidden) ASSERT_TRUE(std::isfinite(value), "SiLU-gated Hidden remains finite");
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        ASSERT_NEAR(55.0f, output[token * outputStride + modelWidth], 0.0f,
                    "the oracle leaves output padding untouched");
    }
    std::vector<float> roundedOutput(tokens * outputStride, 55.0f);
    reference.output = roundedOutput.data();
    reference.projection_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_cpu_reference(&reference),
                 "the reduced-precision oracle applies exact F16 boundaries");
    ASSERT_TRUE(output[0] != roundedOutput[0],
                "the explicit rounded route is observably distinct from exact FP32");

    prom_m47_gated_ffn_plan plan{};
    plan.ffn_width = ffnWidth;
    plan.gating_strategy = PROM_M47_GATING_FUSED_FP32;
    plan.n_generation = 301u;
    plan.weight_generation[0] = 302u;
    plan.weight_generation[1] = 303u;
    plan.weight_generation[2] = 304u;
    plan.output_generation = 305u;
    plan.m46_replay_id = 306u;
    plan.replay_id = 307u;
    prom_m47_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_compare(roundedOutput.data(), roundedOutput.data(),
                                                     tokens, modelWidth, outputStride, outputStride,
                                                     0.0f, 0.0f, &plan, gate.data(), up.data(),
                                                     hidden.data(), down.data(), &mismatch),
                 "identical complete block output compares");
    std::vector<float> actual = roundedOutput;
    actual[outputStride + 2u] += 1.0f;
    ASSERT_TRUE(prom_m47_gated_ffn_compare(roundedOutput.data(), actual.data(), tokens, modelWidth,
                                           outputStride, outputStride, 0.0f, 0.0f, &plan,
                                           gate.data(), up.data(), hidden.data(), down.data(),
                                           &mismatch) != PROM_OK,
                "the first M47 mismatch is localized");
    ASSERT_EQUAL(1u, mismatch.token, "mismatch token is explicit");
    ASSERT_EQUAL(2u, mismatch.column, "mismatch column is explicit");
    ASSERT_EQUAL(301u, mismatch.n_generation, "mismatch carries N generation");
    ASSERT_EQUAL(306u, mismatch.m46_replay_id, "mismatch carries M46 replay identity");
    ASSERT_EQUAL(307u, mismatch.m47_replay_id, "mismatch carries M47 replay identity");

    wgate[0] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_TRUE(prom_m47_gated_ffn_cpu_reference(&reference) != PROM_OK,
                "non-finite FFN weights reject");
}

FACT(PrometheusFp16ConversionUsesRoundToNearestEven)
{
    struct ConversionCase {
        float value;
        std::uint16_t expected;
    };
    const std::array<ConversionCase, 10u> cases{{
        {0.0f, 0x0000u}, {-0.0f, 0x8000u},
        {1.00048828125f, 0x3c00u}, {1.00146484375f, 0x3c02u},
        {-1.00048828125f, 0xbc00u}, {-1.00146484375f, 0xbc02u},
        {0x1.0p-25f, 0x0000u}, {0x1.8p-24f, 0x0002u},
        {65504.0f, 0x7bffu}, {std::numeric_limits<float>::infinity(), 0x7c00u},
    }};
    for (const ConversionCase& testCase : cases)
        ASSERT_EQUAL(testCase.expected, prom_sgemm_float32_to_fp16_bits(testCase.value),
                     "the shared CPU packing authority uses IEEE binary16 RNE");
}

FACT(PrometheusM48FixedStackTopologyOwnershipAndMemory)
{
    prom_m48_plan_request request = M48ResidentPlanRequest();
    prom_m48_transformer_stack_plan plan{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&request, &plan),
                 "the primary fixed four-layer stack plan is eligible");
    ASSERT_EQUAL(1u, plan.eligibility_eligible, "M48 eligibility is explicit");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, plan.layer_count, "the product stack has exactly four layers");
    ASSERT_EQUAL(PROM_M48_TOTAL_RESOURCE_COUNT, plan.persistent_resource_count,
                 "four layers own exactly 116 independently identified resources");
    ASSERT_EQUAL(PROM_M48_MAX_BOUNDARIES, plan.boundary_count,
                 "four layers expose exactly three handoff boundaries");
    ASSERT_EQUAL(0u, plan.intermediate_host_copy_count,
                 "the fixed stack has no intermediate host copy");
    ASSERT_EQUAL(0u, plan.intermediate_readback_count,
                 "the fixed stack has no intermediate readback");
    ASSERT_EQUAL(1u, plan.final_readback_count, "only one optional final readback exists");
    ASSERT_EQUAL(1u, plan.submit_count, "the one-stack topology submits once");
    ASSERT_EQUAL(0u, plan.semaphore_count, "one submit needs no inter-submit semaphore");
    ASSERT_EQUAL(1u, plan.fence_count, "one final fence owns stack completion");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT * 134u, plan.memory.descriptor_set_count,
                 "each layer has a distinct bounded descriptor recording set");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT * PROM_M48_QUERY_COUNT_PER_LAYER,
                 plan.memory.timestamp_query_count, "timestamp ranges are disjoint and bounded");
    ASSERT_EQUAL(static_cast<std::uint64_t>(167780352u),
                 plan.memory.persistent_weight_bytes_per_layer,
                 "primary retained parameter bytes per layer are exact");
    ASSERT_EQUAL(static_cast<std::uint64_t>(671121408u),
                 plan.memory.persistent_weight_bytes,
                 "all four primary parameter layers are retained independently");
    ASSERT_EQUAL(static_cast<std::uint64_t>(10486272u),
                 plan.memory.one_block_working_set_bytes,
                 "one serial block working set is exact and not multiplied by four");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1048576u), plan.memory.activation_bytes,
                 "ping-pong owns two padded output roles in addition to immutable A0");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1048576u), plan.memory.ping_pong_saved_bytes,
                 "ping-pong saves two primary activation outputs versus per-layer retention");
    ASSERT_TRUE(plan.memory.exact_retained_bytes < PROM_M48_CAPACITY_LIMIT_BYTES,
                "the primary stack plus one quarantine reserve fits the explicit cap");

    for (std::uint32_t boundary = 0u; boundary < plan.boundary_count; ++boundary) {
        ASSERT_EQUAL(boundary, plan.boundary[boundary].producer_layer,
                     "each boundary names its producer layer");
        ASSERT_EQUAL(boundary + 1u, plan.boundary[boundary].consumer_layer,
                     "each boundary names its next consumer layer");
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_WRITE_BIT),
                     plan.boundary[boundary].source_access_mask,
                     "layer output begins as a compute write");
        ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT),
                     plan.boundary[boundary].destination_access_mask,
                     "the next layer consumes it as a compute read");
        ASSERT_EQUAL(plan.layer[boundary].output_generation,
                     plan.boundary[boundary].content_generation,
                     "the exact handoff content generation is traced");
        ASSERT_EQUAL(plan.layer[boundary].output_generation,
                     plan.layer[boundary + 1u].input_generation,
                     "the output generation feeds the next layer directly");
    }
    ASSERT_EQUAL(plan.layer[3].output_generation, plan.final_output_generation,
                 "the retained final view is exactly layer four output");
    ASSERT_TRUE(plan.layer[0].output_activation_role != plan.layer[1].output_activation_role &&
                plan.layer[0].output_activation_role == plan.layer[2].output_activation_role,
                "the two physical activation roles alternate without content-generation reuse");
}

FACT(PrometheusM48ReplayIdentityReplacementAndLayerOrder)
{
    const prom_m48_plan_request originalRequest = M48ResidentPlanRequest();
    prom_m48_transformer_stack_plan original{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&originalRequest, &original),
                 "baseline M48 identity plan succeeds");
    prom_m48_transformer_stack_plan repeated{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&originalRequest, &repeated),
                 "identical stack planning repeats");
    ASSERT_EQUAL(original.replay_id, repeated.replay_id,
                 "aggregate stack replay identity is deterministic");
    ASSERT_EQUAL(original.final_output_generation, repeated.final_output_generation,
                 "semantic output identity is deterministic across physical-slot reuse");

    const std::array<std::pair<std::uint32_t, std::uint32_t>, 4u> replacements{{
        {2u, prom_m48_attention_resource_index(5u, PROM_M43_WEIGHT_K)},
        {1u, PROM_M48_RESOURCE_WO},
        {3u, PROM_M48_RESOURCE_WDOWN},
        {0u, PROM_M48_RESOURCE_RMSNORM},
    }};
    for (const auto& replacement : replacements) {
        prom_m48_plan_request changedRequest = originalRequest;
        changedRequest.layer[replacement.first].generation[replacement.second] += 1000u;
        changedRequest.layer[replacement.first].content_hash[replacement.second] += 1000u;
        prom_m48_transformer_stack_plan changed{};
        ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&changedRequest, &changed),
                     "one independently replaced resource replans the stack");
        for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
            if (layer == replacement.first) {
                ASSERT_TRUE(changed.layer[layer].replay_id != original.layer[layer].replay_id,
                            "only the owning layer intrinsic replay identity changes");
            } else {
                ASSERT_EQUAL(original.layer[layer].replay_id, changed.layer[layer].replay_id,
                             "unrelated layer resource identities remain stable");
            }
        }
        for (std::uint32_t layer = 0u; layer < replacement.first; ++layer) {
            ASSERT_EQUAL(original.layer[layer].output_generation, changed.layer[layer].output_generation,
                         "outputs before a replacement remain unchanged");
        }
        for (std::uint32_t layer = replacement.first; layer < PROM_M48_LAYER_COUNT; ++layer) {
            ASSERT_TRUE(original.layer[layer].output_generation != changed.layer[layer].output_generation,
                        "the replacement changes its output and every downstream content identity");
        }
        ASSERT_TRUE(original.replay_id != changed.replay_id,
                    "one resource replacement changes aggregate stack identity");
    }

    prom_m48_plan_request swappedRequest = originalRequest;
    std::swap(swappedRequest.layer[1], swappedRequest.layer[2]);
    prom_m48_transformer_stack_plan swapped{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&swappedRequest, &swapped),
                 "the same resources in a different order still form a legal plan");
    ASSERT_TRUE(original.replay_id != swapped.replay_id,
                "swapping layers one and two changes ordered aggregate identity");
    ASSERT_TRUE(original.final_output_generation != swapped.final_output_generation,
                "layer order changes the semantic activation chain");
}

FACT(PrometheusM48ValidationSubmitPlansAndCapacity)
{
    prom_m48_plan_request request = M48ResidentPlanRequest();
    prom_m48_transformer_stack_plan oneSubmit{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&request, &oneSubmit),
                 "one-submit product plan succeeds");
    request.submit_topology = PROM_M48_SUBMIT_PER_LAYER;
    prom_m48_transformer_stack_plan perLayer{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&request, &perLayer),
                 "same-queue per-layer submit plan succeeds");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, perLayer.submit_count,
                 "per-layer topology has four exact submits");
    ASSERT_EQUAL(PROM_M48_MAX_BOUNDARIES, perLayer.semaphore_count,
                 "three same-queue semaphore dependencies join four submits");
    ASSERT_EQUAL(1u, perLayer.fence_count, "only final stack completion owns a fence");
    ASSERT_TRUE(oneSubmit.command_plan_replay_id != perLayer.command_plan_replay_id,
                "submit topology participates in command identity");

    for (const std::uint32_t layers : {1u, 2u, 4u}) {
        prom_m48_plan_request audit = M48ResidentPlanRequest(layers, 1u);
        prom_m48_transformer_stack_plan auditPlan{};
        ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&audit, &auditPlan),
                     "bounded one/two/four layer audit planning succeeds");
        ASSERT_EQUAL(layers, auditPlan.layer_count, "audit layer count remains exact");
    }
    prom_m48_plan_request invalid = M48ResidentPlanRequest(2u, 0u);
    prom_m48_transformer_stack_plan rejected{};
    ASSERT_TRUE(prom_m48_transformer_stack_plan_build(&invalid, &rejected) != PROM_OK,
                "two layers cannot masquerade as the product contract");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M48_INELIGIBLE_LAYER_COUNT),
                 rejected.eligibility_reason, "product layer-count rejection is exact");

    invalid = M48ResidentPlanRequest();
    invalid.layer[2].generation[PROM_M48_RESOURCE_WUP] = 0u;
    ASSERT_TRUE(prom_m48_transformer_stack_plan_build(&invalid, &rejected) != PROM_OK,
                "a missing generation in the complete matrix rejects");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M48_INELIGIBLE_WEIGHT),
                 rejected.eligibility_reason, "missing weight identity has an exact reason");
    invalid = M48ResidentPlanRequest();
    invalid.resident_initial_activation.owning_lifetime_id -= 1u;
    ASSERT_TRUE(prom_m48_transformer_stack_plan_build(&invalid, &rejected) != PROM_OK,
                "a stale initial activation rejects before execution");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M48_INELIGIBLE_INITIAL_ACTIVATION),
                 rejected.eligibility_reason, "stale A0 has an exact reason");
    invalid = M48ResidentPlanRequest();
    invalid.capacity_limit_bytes = 1u;
    ASSERT_TRUE(prom_m48_transformer_stack_plan_build(&invalid, &rejected) != PROM_OK,
                "capacity rejection happens before submission");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M48_INELIGIBLE_CAPACITY),
                 rejected.eligibility_reason, "capacity rejection is deterministic");
    invalid = M48ResidentPlanRequest();
    invalid.optional_final_readback = 0u;
    prom_m48_transformer_stack_plan noReadback{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_plan_build(&invalid, &noReadback),
                 "pure resident benchmarking omits even the final readback");
    ASSERT_EQUAL(0u, noReadback.final_readback_count, "zero-readback product mode is explicit");
    ASSERT_EQUAL(0u, noReadback.memory.final_readback_bytes,
                 "zero-readback capacity excludes compact host storage");
}

FACT(PrometheusM48FourLayerCpuOracleUsesDistinctOrderedWeights)
{
    constexpr std::uint32_t tokens = 2u;
    constexpr std::uint32_t modelWidth = 8u;
    constexpr std::uint32_t headDim = 1u;
    constexpr std::uint32_t ffnWidth = 16u;
    const std::size_t modelElements = static_cast<std::size_t>(tokens) * modelWidth;
    std::vector<float> initial(modelElements);
    std::vector<float> output(modelElements, 0.0f);
    for (std::size_t index = 0u; index < initial.size(); ++index)
        initial[index] = static_cast<float>(static_cast<int>(index) - 7) / 32.0f;
    const std::vector<float> originalInitial = initial;
    std::array<GroupWeights, PROM_M48_LAYER_COUNT> attentionWeights;
    std::array<std::vector<float>, PROM_M48_LAYER_COUNT> wo;
    std::array<std::vector<float>, PROM_M48_LAYER_COUNT> norm;
    std::array<std::array<std::vector<float>, PROM_M47_WEIGHT_COUNT>,
               PROM_M48_LAYER_COUNT> ffn;
    prom_m48_reference_request request{};
    request.initial_activation = initial.data();
    request.output = output.data();
    request.initial_element_count = initial.size();
    request.output_element_count = output.size();
    request.layer_count = PROM_M48_LAYER_COUNT;
    request.tokens = tokens;
    request.model_width = modelWidth;
    request.head_count = PROM_M43_HEAD_COUNT;
    request.head_dim = headDim;
    request.ffn_width = ffnWidth;
    request.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    request.projection_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
    request.epsilon = 1.0e-5f;
    for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                std::vector<float>& weight = attentionWeights[layer][head][kind];
                weight.resize(modelWidth * headDim);
                for (std::size_t index = 0u; index < weight.size(); ++index) {
                    const int value = static_cast<int>((index + head * 3u + kind * 5u + layer * 7u) % 11u) - 5;
                    weight[index] = static_cast<float>(value) / 128.0f;
                }
                request.layer[layer].attention_weight[head][kind] = weight.data();
            }
        }
        wo[layer].resize(modelWidth * modelWidth);
        for (std::size_t index = 0u; index < wo[layer].size(); ++index) {
            const int value = static_cast<int>((index * 3u + layer * 11u) % 17u) - 8;
            wo[layer][index] = static_cast<float>(value) / 128.0f;
        }
        norm[layer].resize(modelWidth);
        for (std::size_t index = 0u; index < norm[layer].size(); ++index)
            norm[layer][index] = 0.75f + static_cast<float>(layer + index) / 64.0f;
        ffn[layer][PROM_M47_WEIGHT_GATE].resize(modelWidth * ffnWidth);
        ffn[layer][PROM_M47_WEIGHT_UP].resize(modelWidth * ffnWidth);
        ffn[layer][PROM_M47_WEIGHT_DOWN].resize(ffnWidth * modelWidth);
        for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind) {
            for (std::size_t index = 0u; index < ffn[layer][kind].size(); ++index) {
                const int value = static_cast<int>((index * (kind + 3u) + layer * 13u) % 19u) - 9;
                ffn[layer][kind][index] = static_cast<float>(value) / 256.0f;
            }
        }
        request.layer[layer].wo = wo[layer].data();
        request.layer[layer].rmsnorm_weight = norm[layer].data();
        request.layer[layer].wgate = ffn[layer][PROM_M47_WEIGHT_GATE].data();
        request.layer[layer].wup = ffn[layer][PROM_M47_WEIGHT_UP].data();
        request.layer[layer].wdown = ffn[layer][PROM_M47_WEIGHT_DOWN].data();
    }
    prom_m48_reference_result result{};
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_cpu_reference(&request, &result),
                 "the exact reduced-precision four-layer CPU oracle succeeds");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, result.completed_layer_count,
                 "the oracle executes all four complete M43-M47 blocks");
    ASSERT_EQUAL(1u, result.all_finite, "the final activation is finite");
    ASSERT_TRUE(initial == originalInitial, "the immutable initial authority is not overwritten");
    ASSERT_TRUE(output != initial, "four distinct layers materially transform the activation");

    std::vector<float> firstOutput = output;
    std::swap(request.layer[1], request.layer[2]);
    ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_cpu_reference(&request, &result),
                 "the same distinct layer resources execute in swapped order");
    ASSERT_TRUE(output != firstOutput, "ordered layer identity has observable semantic effect");
    std::swap(request.layer[1], request.layer[2]);

    ffn[2u][PROM_M47_WEIGHT_DOWN][3u] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_TRUE(prom_m48_transformer_stack_cpu_reference(&request, &result) != PROM_OK,
                "a nonfinite layer-two parameter fails the bounded oracle");
    ASSERT_EQUAL(2u, result.failed_layer, "the first failing layer is localized");
    ASSERT_EQUAL(47u, result.failed_stage, "the failing gated-FFN stage is localized");
    ASSERT_EQUAL(2u, result.completed_layer_count, "only completed prior layers are reported");
}

FACT(PrometheusM48LiveFixedStackUsesFourLayerBundlesAndTwoSubmitTopologies)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr)
        SKIP("Vulkan runtime unavailable");
    prom_vk_runtime_services availability{};
    if (prom_reactor_runtime_get_vk_services(runtime, &availability) != PROM_OK ||
        availability.backend_available == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime services unavailable");
    }

    const char* corpusShape = std::getenv("PROMETHEUS_M48_CORPUS_SHAPE");
    const bool printCorpus = std::getenv("PROMETHEUS_M48_PRINT") != nullptr;
    const bool primaryPrecisionAudit = std::getenv("PROMETHEUS_M48_PRIMARY_PRECISION_AUDIT") != nullptr;
    const char* requestedPath = std::getenv("PROMETHEUS_M48_CORPUS_PATH");
    const std::string_view pathName = requestedPath == nullptr ? "conventional" : requestedPath;
    const std::uint32_t projectionPath = pathName == "cooperative"
                                             ? PROM_M47_PROJECTION_COOPERATIVE
                                             : pathName == "a2x4"
                                                   ? PROM_M47_PROJECTION_A2X4_FP32
                                                   : PROM_M47_PROJECTION_CONVENTIONAL_FP16;
    const std::uint32_t precisionPolicy = projectionPath == PROM_M47_PROJECTION_A2X4_FP32
                                              ? PROM_M42_PRECISION_FP32
                                              : PROM_M42_PRECISION_F16_ROUNDED;
    const std::uint32_t gatingStrategy = projectionPath == PROM_M47_PROJECTION_A2X4_FP32
                                             ? PROM_M47_GATING_FUSED_FP32
                                             : PROM_M47_GATING_FUSED_DIRECT_PACKED;
    const std::string_view shape = corpusShape == nullptr ? "tiny" : corpusShape;
    const bool fullOracle = shape == "tiny";
    const bool selectedOracleAudit = primaryPrecisionAudit && shape == "primary";
    const std::uint32_t tokens = shape == "more_tokens" ? 256u
                               : shape == "token_boundary" ? 1024u
                               : shape == "tiny" ? 16u : 128u;
    const std::uint32_t modelWidth = shape == "awkward" ? 1000u
                                   : shape == "token_boundary" ? 128u
                                   : shape == "tiny" ? 128u : 1024u;
    const std::uint32_t headDim = modelWidth / PROM_M43_HEAD_COUNT;
    const std::uint32_t ffnWidth = shape == "smaller_expansion" ? 2048u
                                 : shape == "awkward" ? 3008u
                                 : shape == "token_boundary" ? 512u
                                 : shape == "tiny" ? 256u : 4096u;
    constexpr std::uint64_t initialGeneration = 480000u;
    const std::size_t activationElements = static_cast<std::size_t>(tokens) * modelWidth;
    std::vector<float> initial(activationElements);
    std::vector<float> oneSubmitOutput(activationElements, 0.0f);
    std::vector<float> fourSubmitOutput(activationElements, 0.0f);
    std::vector<float> referenceOutput(activationElements, 0.0f);
    std::array<std::vector<float>, PROM_M48_LAYER_COUNT> referenceLayerOutputs;
    std::array<std::array<std::vector<float>, PROM_M48_AUDIT_STAGE_COUNT>,
               PROM_M48_LAYER_COUNT> referenceStageOutputs;
    std::array<std::array<std::vector<float>, PROM_M48_RESOURCE_COUNT>,
               PROM_M48_LAYER_COUNT> weights;
    std::array<std::array<std::uint64_t, PROM_M48_RESOURCE_COUNT>,
               PROM_M48_LAYER_COUNT> generations{};
    for (std::size_t index = 0u; index < initial.size(); ++index)
        initial[index] = static_cast<float>(static_cast<int>(index) - 7) / 64.0f;

    prom_m48_reference_request reference{};
    reference.initial_activation = initial.data();
    reference.output = referenceOutput.data();
    reference.initial_element_count = initial.size();
    reference.output_element_count = referenceOutput.size();
    reference.layer_count = PROM_M48_LAYER_COUNT;
    reference.tokens = tokens;
    reference.model_width = modelWidth;
    reference.head_count = PROM_M43_HEAD_COUNT;
    reference.head_dim = headDim;
    reference.ffn_width = ffnWidth;
    reference.precision_policy = precisionPolicy;
    reference.projection_path = projectionPath;
    reference.epsilon = 1.0e-5f;
    if (selectedOracleAudit) {
        for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
            referenceLayerOutputs[layer].assign(activationElements, 0.0f);
            reference.audit_layer_output[layer] = referenceLayerOutputs[layer].data();
            for (std::uint32_t stage = 0u; stage < PROM_M48_AUDIT_STAGE_COUNT; ++stage) {
                referenceStageOutputs[layer][stage].assign(activationElements, 0.0f);
                reference.audit_stage_output[layer][stage] =
                    referenceStageOutputs[layer][stage].data();
            }
        }
    }

    for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
        for (std::uint32_t resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource) {
            std::size_t count = modelWidth * headDim;
            if (resource == PROM_M48_RESOURCE_WO) count = modelWidth * modelWidth;
            else if (resource == PROM_M48_RESOURCE_RMSNORM) count = modelWidth;
            else if (resource >= PROM_M48_RESOURCE_WGATE) count = modelWidth * ffnWidth;
            weights[layer][resource].resize(count);
            for (std::size_t index = 0u; index < count; ++index) {
                const int value = static_cast<int>((index * 3u + resource * 5u + layer * 7u) % 17u) - 8;
                weights[layer][resource][index] = resource == PROM_M48_RESOURCE_RMSNORM
                    ? 0.75f + static_cast<float>((index + layer) % 7u) / 32.0f
                    : static_cast<float>(value) / 256.0f;
            }
            generations[layer][resource] = 481000u + layer * 100u + resource;
            prom_m48_layer_weight_prepare_request prepare{};
            prepare.values = weights[layer][resource].data();
            prepare.element_count = weights[layer][resource].size();
            prepare.layer_index = layer;
            prepare.resource_index = resource;
            prepare.model_width = modelWidth;
            prepare.head_dim = headDim;
            prepare.ffn_width = ffnWidth;
            prepare.generation = generations[layer][resource];
            prom_m48_layer_weight_prepare_result prepared{};
            ASSERT_EQUAL(PROM_OK,
                         prom_reactor_runtime_m48_prepare_layer_weight(runtime, &prepare, &prepared),
                         "each ordered M48 layer parameter prepares independently");
        }
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                reference.layer[layer].attention_weight[head][kind] =
                    weights[layer][prom_m48_attention_resource_index(head, kind)].data();
            }
        }
        reference.layer[layer].wo = weights[layer][PROM_M48_RESOURCE_WO].data();
        reference.layer[layer].rmsnorm_weight = weights[layer][PROM_M48_RESOURCE_RMSNORM].data();
        reference.layer[layer].wgate = weights[layer][PROM_M48_RESOURCE_WGATE].data();
        reference.layer[layer].wup = weights[layer][PROM_M48_RESOURCE_WUP].data();
        reference.layer[layer].wdown = weights[layer][PROM_M48_RESOURCE_WDOWN].data();
    }
    prom_m48_reference_result referenceResult{};
    if (fullOracle || selectedOracleAudit) {
        ASSERT_EQUAL(PROM_OK, prom_m48_transformer_stack_cpu_reference(&reference, &referenceResult),
                     "the selected stack audit has an independent four-block CPU oracle");
    }

    prom_m48_stack_request request{};
    request.host_initial_activation = initial.data();
    request.host_initial_element_count = initial.size();
    request.output = oneSubmitOutput.data();
    request.output_element_count = oneSubmitOutput.size();
    request.initial_activation_mode = PROM_M48_INITIAL_HOST;
    request.layer_count = PROM_M48_LAYER_COUNT;
    request.tokens = tokens;
    request.model_width = modelWidth;
    request.head_count = PROM_M43_HEAD_COUNT;
    request.head_dim = headDim;
    request.ffn_width = ffnWidth;
    request.precision_policy = precisionPolicy;
    request.projection_path = projectionPath;
    request.attention_strategy = PROM_M43_STRATEGY_PROJECTION_GROUPED;
    request.output_projection_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request.rmsnorm_strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    request.gating_strategy = gatingStrategy;
    request.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_DOWN;
    request.submit_topology = PROM_M48_SUBMIT_ONE_STACK;
    request.allow_fallback = 1u;
    request.epsilon = reference.epsilon;
    request.expected_initial_generation = initialGeneration;
    for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer)
        for (std::uint32_t resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource)
            request.required_generation[layer][resource] = generations[layer][resource];

    prom_vk_runtime_services before{};
    prom_vk_runtime_services after{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &before),
                 "validation baseline is readable");
    prom_m48_stack_result oneSubmit{};
    const int oneSubmitStatus = prom_reactor_runtime_m48_execute_stack(runtime, &request, &oneSubmit);
    ASSERT_EQUAL(0, oneSubmit.detail_code, "one-submit failure detail remains clear on success");
    ASSERT_EQUAL(PROM_OK, oneSubmitStatus,
                 "one caller-owned command buffer executes four real transformer blocks");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, oneSubmit.completed_layer_count,
                 "all four layers complete in one fixed stack lifecycle");
    ASSERT_EQUAL(1u, oneSubmit.submit_count, "one-stack topology performs one queue submit");
    ASSERT_EQUAL(0u, oneSubmit.semaphore_count, "one-stack topology needs no semaphore");
    ASSERT_EQUAL(0u, oneSubmit.intermediate_readback_count,
                 "the product path performs no intermediate readback");
    const std::uint32_t expectedProjectionPath =
        projectionPath == PROM_M47_PROJECTION_COOPERATIVE &&
                (before.cooperative_matrix_feature_enabled == 0u || before.subgroup_size != 32u)
            ? PROM_M47_PROJECTION_CONVENTIONAL_FP16
            : projectionPath;
    ASSERT_EQUAL(expectedProjectionPath, oneSubmit.selected_projection_path,
                 "the selected stack path records deterministic cooperative fallback");
    for (std::uint32_t layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer)
        ASSERT_TRUE(oneSubmit.layer[layer].total_gpu_ns > 0u,
                    "each recorded transformer layer has a nonzero timestamp interval");
    if (fullOracle) {
        for (std::size_t index = 0u; index < activationElements; ++index)
            ASSERT_TRUE(std::abs(oneSubmitOutput[index] - referenceOutput[index]) <= 8.0e-2f,
                        "live one-submit output agrees with the reduced-precision CPU oracle");
    } else if (selectedOracleAudit) {
        const std::array<const char*, PROM_M48_AUDIT_STAGE_COUNT> stageNames{{
            "attention", "output_projection", "first_residual", "rmsnorm", "ffn",
        }};
        const std::array<float, PROM_M48_AUDIT_STAGE_COUNT> absoluteTolerances{{
            1.0e-2f, 8.0e-3f, 1.0e-2f, 2.0e-3f, 3.0e-3f,
        }};
        const std::array<float, PROM_M48_AUDIT_STAGE_COUNT> relativeTolerances{{
            5.0e-2f, 3.0e-2f, 4.0e-2f, 2.0e-2f, 4.0e-2f,
        }};
        std::array<std::vector<float>, PROM_M48_LAYER_COUNT> capturedLayerOutputs;
        for (std::uint32_t auditLayer = 0u; auditLayer < PROM_M48_LAYER_COUNT; ++auditLayer) {
            for (std::uint32_t auditStage = 0u;
                 auditStage < PROM_M48_AUDIT_STAGE_COUNT; ++auditStage) {
                std::vector<float> stageOutput(activationElements, 0.0f);
                prom_m48_stack_request layerRequest = request;
                prom_m48_stack_result layerResult{};
                float maximumAbsoluteError = 0.0f;
                float maximumRelativeError = 0.0f;
                std::uint32_t mismatchCount = 0u;
                layerRequest.layer_count = auditLayer + 1u;
                layerRequest.audit_mode = 1u;
                layerRequest.output = nullptr;
                layerRequest.output_element_count = 0u;
                layerRequest.audit_stage_output = stageOutput.data();
                layerRequest.audit_stage_output_element_count = stageOutput.size();
                layerRequest.audit_stage = auditStage + 1u;
                ASSERT_EQUAL(PROM_OK,
                             prom_reactor_runtime_m48_execute_stack(runtime, &layerRequest,
                                                                    &layerResult),
                             "primary audit captures one real stage from each ordered layer prefix");
                for (std::size_t index = 0u; index < activationElements; ++index) {
                    const std::uint32_t head = auditStage == 0u
                        ? static_cast<std::uint32_t>(index / (tokens * headDim)) : 0u;
                    const std::size_t headIndex = auditStage == 0u
                        ? index - static_cast<std::size_t>(head) * tokens * headDim : index;
                    const std::uint32_t token = auditStage == 0u
                        ? static_cast<std::uint32_t>(headIndex / headDim)
                        : static_cast<std::uint32_t>(index / modelWidth);
                    const std::uint32_t column = auditStage == 0u
                        ? head * headDim + static_cast<std::uint32_t>(headIndex % headDim)
                        : static_cast<std::uint32_t>(index % modelWidth);
                    const float expected = referenceStageOutputs[auditLayer][auditStage][index];
                    const float actual = stageOutput[index];
                    const float absoluteError = std::abs(actual - expected);
                    const float relativeError = absoluteError /
                        std::max(std::abs(expected), 1.0e-20f);
                    maximumAbsoluteError = std::max(maximumAbsoluteError, absoluteError);
                    maximumRelativeError = std::max(maximumRelativeError, relativeError);
                    if (!std::isfinite(actual) ||
                        (absoluteError > absoluteTolerances[auditStage] &&
                         relativeError > relativeTolerances[auditStage])) {
                        ++mismatchCount;
                        if (mismatchCount <= 3u) {
                            std::fprintf(stderr,
                                         "M48 stage audit mismatch layer=%u stage=%s token=%u column=%u expected=%g actual=%g abs=%g rel=%g abs_tol=%g rel_tol=%g layer_replay=%llu stack_replay=%llu path=%u\n",
                                         auditLayer, stageNames[auditStage], token, column,
                                         expected, actual, absoluteError, relativeError,
                                         absoluteTolerances[auditStage],
                                         relativeTolerances[auditStage],
                                         static_cast<unsigned long long>(
                                             layerResult.layer[auditLayer].replay_id),
                                         static_cast<unsigned long long>(layerResult.replay_id),
                                         layerResult.selected_projection_path);
                        }
                    }
                }
                std::fprintf(stderr,
                             "M48 stage audit summary layer=%u stage=%s max_abs=%g max_rel=%g mismatches=%u elements=%zu\n",
                             auditLayer, stageNames[auditStage], maximumAbsoluteError,
                             maximumRelativeError, mismatchCount, activationElements);
                ASSERT_EQUAL(0u, mismatchCount,
                             "stage boundary agrees with its established precision tolerance");
                if (auditStage == PROM_M48_AUDIT_STAGE_ATTENTION - 1u) {
                    std::vector<float> matchedInputAttention(activationElements, 0.0f);
                    const std::vector<float>& matchedInput = auditLayer == 0u
                        ? initial : capturedLayerOutputs[auditLayer - 1u];
                    prom_m43_reference_request matchedRequest{};
                    prom_m43_reference_result matchedResult{};
                    std::uint32_t matchedMismatchCount = 0u;
                    float matchedMaximumAbsoluteError = 0.0f;
                    float matchedMaximumRelativeError = 0.0f;
                    matchedRequest.x = matchedInput.data();
                    matchedRequest.output = matchedInputAttention.data();
                    matchedRequest.x_element_count = activationElements;
                    matchedRequest.weight_element_count =
                        static_cast<std::uint64_t>(modelWidth) * headDim;
                    matchedRequest.output_element_count = activationElements;
                    matchedRequest.head_count = PROM_M43_HEAD_COUNT;
                    matchedRequest.tokens = tokens;
                    matchedRequest.model_width = modelWidth;
                    matchedRequest.head_dim = headDim;
                    matchedRequest.precision_policy = precisionPolicy;
                    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
                        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                            matchedRequest.weight[head][kind] =
                                reference.layer[auditLayer].attention_weight[head][kind];
                        }
                    }
                    ASSERT_EQUAL(PROM_OK,
                                 prom_m43_attention_cpu_reference(&matchedRequest, &matchedResult),
                                 "matched-input attention oracle succeeds");
                    for (std::size_t index = 0u; index < activationElements; ++index) {
                        const float expected = matchedInputAttention[index];
                        const float actual = stageOutput[index];
                        const float absoluteError = std::abs(actual - expected);
                        const float relativeError = absoluteError /
                            std::max(std::abs(expected), 1.0e-20f);
                        matchedMaximumAbsoluteError =
                            std::max(matchedMaximumAbsoluteError, absoluteError);
                        matchedMaximumRelativeError =
                            std::max(matchedMaximumRelativeError, relativeError);
                        if (!std::isfinite(actual) ||
                            (absoluteError > absoluteTolerances[auditStage] &&
                             relativeError > relativeTolerances[auditStage])) {
                            ++matchedMismatchCount;
                        }
                    }
                    std::fprintf(stderr,
                                 "M48 matched-input attention summary layer=%u max_abs=%g max_rel=%g mismatches=%u elements=%zu\n",
                                 auditLayer, matchedMaximumAbsoluteError,
                                 matchedMaximumRelativeError, matchedMismatchCount,
                                 activationElements);
                    ASSERT_EQUAL(0u, matchedMismatchCount,
                                 "attention agrees when CPU and GPU consume the same activation");
                }
                if (auditStage == PROM_M48_AUDIT_STAGE_FFN - 1u) {
                    capturedLayerOutputs[auditLayer] = stageOutput;
                }
            }
        }
    } else {
        for (const float value : oneSubmitOutput)
            ASSERT_TRUE(std::isfinite(value), "bounded corpus output remains finite");
    }

    request.output = fourSubmitOutput.data();
    request.submit_topology = PROM_M48_SUBMIT_PER_LAYER;
    prom_m48_stack_result fourSubmit{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &fourSubmit),
                 "four submits execute as one same-queue semaphore chain without host waits");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, fourSubmit.submit_count,
                 "bounded per-layer topology has exactly four submits");
    ASSERT_EQUAL(PROM_M48_MAX_BOUNDARIES, fourSubmit.semaphore_count,
                 "three semaphore dependencies connect the four submits");
    for (std::size_t index = 0u; index < activationElements; ++index)
        ASSERT_TRUE(std::abs(oneSubmitOutput[index] - fourSubmitOutput[index]) <= 2.0e-3f,
                    "one-submit and semaphore-chain results agree");

    prom_m48_stack_result warm{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &warm),
                 "a warm stack repeats after slot recycling");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0u), warm.buffer_allocation_count,
                 "warm fixed-stack execution performs zero Vulkan buffer allocation");

    for (const std::uint32_t fault : {
             PROM_M48_FAULT_BEFORE_LAYER_0,
             PROM_M48_FAULT_DURING_LAYER_0_ATTENTION,
             PROM_M48_FAULT_AFTER_LAYER_0_OUTPUT,
             PROM_M48_FAULT_DURING_LAYER_1_RMSNORM,
             PROM_M48_FAULT_DURING_LAYER_1_FFN,
             PROM_M48_FAULT_AFTER_LAYER_2_OUTPUT,
             PROM_M48_FAULT_DURING_LAYER_3_ATTENTION,
             PROM_M48_FAULT_DURING_LAYER_3_FFN,
             PROM_M48_FAULT_AFTER_FINAL_OUTPUT,
             PROM_M48_FAULT_BEFORE_FINAL_READBACK,
         }) {
        request.fault_point = fault;
        prom_m48_stack_result knownFault{};
        ASSERT_TRUE(prom_reactor_runtime_m48_execute_stack(runtime, &request, &knownFault) != PROM_OK,
                    "each known stack fault aborts at its logical location");
        ASSERT_EQUAL(static_cast<std::int32_t>(PROM_M48_DETAIL_FAULT_INJECTED),
                     knownFault.detail_code, "known fault remains explicit");
        ASSERT_EQUAL(1u, knownFault.physical_slot_recyclable,
                     "known fault recycles without quarantine");
        request.fault_point = PROM_M48_FAULT_NONE;
        prom_m48_stack_result afterKnownFault{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m48_execute_stack(runtime, &request, &afterKnownFault),
                     "a complete stack succeeds after each known-fault recycle");
    }

    request.fault_point = PROM_M48_FAULT_UNCERTAIN_COMPLETION;
    prom_m48_stack_result uncertainFault{};
    ASSERT_TRUE(prom_reactor_runtime_m48_execute_stack(runtime, &request, &uncertainFault) != PROM_OK,
                "uncertain submitted completion quarantines the whole stack slot");
    ASSERT_EQUAL(static_cast<std::int32_t>(PROM_M48_DETAIL_COMPLETION_UNCERTAIN),
                 uncertainFault.detail_code, "uncertain completion has its own detail code");
    ASSERT_EQUAL(0u, uncertainFault.physical_slot_recyclable,
                 "uncertain completion is not prematurely recyclable");
    request.fault_point = PROM_M48_FAULT_NONE;
    prom_m48_stack_result afterUncertainFault{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m48_execute_stack(runtime, &request, &afterUncertainFault),
                 "fence reaping permits a successful stack after quarantine");

    prom_m48_initial_activation_prepare_request prepareInitial{};
    prepareInitial.values = initial.data();
    prepareInitial.element_count = initial.size();
    prepareInitial.tokens = tokens;
    prepareInitial.model_width = modelWidth;
    prepareInitial.generation = initialGeneration + 1u;
    prom_m48_initial_activation_prepare_result preparedInitial{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m48_prepare_initial_activation(runtime, &prepareInitial,
                                                                       &preparedInitial),
                 "A0 may be prepared as a family-resident immutable activation");
    request.initial_activation_mode = PROM_M48_INITIAL_RESIDENT;
    request.host_initial_activation = nullptr;
    request.host_initial_element_count = 0u;
    request.expected_initial_generation = prepareInitial.generation;
    prom_m48_stack_result residentInitial{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &residentInitial),
                 "resident A0 feeds layer zero without host authority or bounce");

    request.audit_mode = 1u;
    std::array<std::uint64_t, 2u> auditGpu{};
    std::uint32_t auditIndex = 0u;
    for (const std::uint32_t auditLayers : {1u, 2u}) {
        request.layer_count = auditLayers;
        prom_m48_stack_result audit{};
        const int auditStatus = prom_reactor_runtime_m48_execute_stack(runtime, &request, &audit);
        ASSERT_EQUAL(0, audit.detail_code, "bounded audit detail remains clear on success");
        ASSERT_EQUAL(PROM_OK, auditStatus,
                     "bounded live one/two-layer audit depth executes through the same recorder");
        ASSERT_EQUAL(auditLayers, audit.completed_layer_count,
                     "audit executes exactly the requested bounded layer prefix");
        auditGpu[auditIndex++] = audit.total_stack_gpu_ns;
    }
    request.layer_count = PROM_M48_LAYER_COUNT;
    request.audit_mode = 0u;

    request.submit_topology = PROM_M48_SUBMIT_HOST_WAIT_PER_LAYER_AUDIT;
    request.audit_mode = 1u;
    prom_m48_stack_result hostWait{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &hostWait),
                 "audit host-wait topology retains every activation on the device");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, hostWait.submit_count,
                 "host-wait audit submits one complete block per layer");
    ASSERT_EQUAL(0u, hostWait.semaphore_count,
                 "host-wait audit isolates host synchronization rather than semaphores");
    ASSERT_EQUAL(0u, hostWait.intermediate_readback_count,
                 "host-wait audit does not add host activation movement");
    ASSERT_TRUE(hostWait.cpu_wait_ns > 0u,
                "host-wait audit reports the accumulated host fence wait interval");

    request.initial_activation_mode = PROM_M48_INITIAL_HOST;
    request.host_initial_activation = initial.data();
    request.host_initial_element_count = initial.size();
    request.expected_initial_generation = initialGeneration;
    request.submit_topology = PROM_M48_SUBMIT_HOST_BOUNCE_PER_LAYER_AUDIT;
    prom_m48_stack_result hostBounce{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &hostBounce),
                 "audit host-bounce topology completes all four layers");
    ASSERT_EQUAL(PROM_M48_LAYER_COUNT, hostBounce.submit_count,
                 "host-bounce audit submits each complete block separately");
    ASSERT_EQUAL(0u, hostBounce.semaphore_count,
                 "host-bounce audit does not disguise host movement as a semaphore chain");
    ASSERT_EQUAL(PROM_M48_MAX_BOUNDARIES, hostBounce.intermediate_readback_count,
                 "host-bounce audit reads every inter-layer activation");
    ASSERT_EQUAL(PROM_M48_MAX_BOUNDARIES, hostBounce.intermediate_host_copy_count,
                 "host-bounce audit reuploads every retained host activation");
    ASSERT_TRUE(hostBounce.cpu_wait_ns > 0u,
                "host-bounce audit reports synchronization cost");
    for (std::size_t index = 0u; index < activationElements; ++index)
        ASSERT_TRUE(std::abs(oneSubmitOutput[index] - fourSubmitOutput[index]) <= 2.0e-3f,
                    "host-bounce preserves the same complete-block arithmetic result");

    request.initial_activation_mode = PROM_M48_INITIAL_RESIDENT;
    request.host_initial_activation = nullptr;
    request.host_initial_element_count = 0u;
    request.expected_initial_generation = prepareInitial.generation;
    request.submit_topology = PROM_M48_SUBMIT_PER_LAYER;
    request.audit_mode = 0u;

    std::uint64_t warm10Median = 0u;
    std::uint64_t warm10P10 = 0u;
    std::uint64_t warm10P90 = 0u;
    std::uint64_t warm100Median = 0u;
    std::uint64_t warm100P10 = 0u;
    std::uint64_t warm100P90 = 0u;
    if (printCorpus) {
        auto measureWarm = [&](std::uint32_t count, std::uint64_t* median,
                               std::uint64_t* p10, std::uint64_t* p90) {
            std::vector<std::uint64_t> samples;
            samples.reserve(count);
            request.submit_topology = PROM_M48_SUBMIT_ONE_STACK;
            for (std::uint32_t sample = 0u; sample < count; ++sample) {
                prom_m48_stack_result value{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m48_execute_stack(runtime, &request, &value),
                             "warm corpus stack completes");
                ASSERT_EQUAL(static_cast<std::uint64_t>(0u), value.buffer_allocation_count,
                             "primed warm corpus stack allocates no Vulkan buffer");
                samples.push_back(value.total_stack_gpu_ns);
            }
            std::sort(samples.begin(), samples.end());
            *median = samples[samples.size() / 2u];
            *p10 = samples[(samples.size() - 1u) / 10u];
            *p90 = samples[((samples.size() - 1u) * 9u) / 10u];
        };
        measureWarm(10u, &warm10Median, &warm10P10, &warm10P90);
        measureWarm(100u, &warm100Median, &warm100P10, &warm100P90);
    }

    const std::array<std::pair<std::uint32_t, std::uint32_t>, 4u> liveReplacements{{
        {2u, prom_m48_attention_resource_index(5u, PROM_M43_WEIGHT_K)},
        {1u, PROM_M48_RESOURCE_WO},
        {3u, PROM_M48_RESOURCE_WDOWN},
        {0u, PROM_M48_RESOURCE_RMSNORM},
    }};
    std::uint64_t precedingReplay = residentInitial.replay_id;
    for (const auto& replacement : liveReplacements) {
        std::vector<float>& values = weights[replacement.first][replacement.second];
        values[0] += 1.0f / 512.0f;
        generations[replacement.first][replacement.second] += 10000u;
        prom_m48_layer_weight_prepare_request replace{};
        replace.values = values.data();
        replace.element_count = values.size();
        replace.layer_index = replacement.first;
        replace.resource_index = replacement.second;
        replace.model_width = modelWidth;
        replace.head_dim = headDim;
        replace.ffn_width = ffnWidth;
        replace.generation = generations[replacement.first][replacement.second];
        prom_m48_layer_weight_prepare_result replaced{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m48_prepare_layer_weight(runtime, &replace, &replaced),
                     "one exact layer parameter replaces without rebuilding other bundles");
        request.required_generation[replacement.first][replacement.second] = replace.generation;
        prom_m48_stack_result afterReplacement{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m48_execute_stack(runtime, &request, &afterReplacement),
                     "the next complete stack consumes one coherent replacement generation");
        ASSERT_TRUE(precedingReplay != afterReplacement.replay_id,
                    "each exact live replacement changes aggregate replay identity");
        precedingReplay = afterReplacement.replay_id;
    }
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &after),
                 "validation result is readable");
    ASSERT_EQUAL(before.validation_error_count, after.validation_error_count,
                 "validation-enabled one/four-submit stack execution adds no errors");
    if (!fullOracle || printCorpus) {
        std::fprintf(stderr,
                     "M48 corpus shape=%.*s path=%.*s one=%llu two=%llu four_one_submit=%llu four_submit=%llu layer0=%llu layer1=%llu layer2=%llu layer3=%llu host_wait_gpu=%llu host_wait_e2e=%llu host_wait_cpu_wait=%llu host_bounce_gpu=%llu host_bounce_e2e=%llu host_bounce_cpu_wait=%llu host_bounce_copy=%llu resident_e2e=%llu host_e2e=%llu warm=%llu warm10_med=%llu warm10_p10=%llu warm10_p90=%llu warm100_med=%llu warm100_p10=%llu warm100_p90=%llu\n",
                     static_cast<int>(shape.size()), shape.data(),
                     static_cast<int>(pathName.size()), pathName.data(),
                     static_cast<unsigned long long>(auditGpu[0]),
                     static_cast<unsigned long long>(auditGpu[1]),
                     static_cast<unsigned long long>(oneSubmit.total_stack_gpu_ns),
                     static_cast<unsigned long long>(fourSubmit.total_stack_gpu_ns),
                     static_cast<unsigned long long>(oneSubmit.layer[0].total_gpu_ns),
                     static_cast<unsigned long long>(oneSubmit.layer[1].total_gpu_ns),
                     static_cast<unsigned long long>(oneSubmit.layer[2].total_gpu_ns),
                     static_cast<unsigned long long>(oneSubmit.layer[3].total_gpu_ns),
                     static_cast<unsigned long long>(hostWait.total_stack_gpu_ns),
                     static_cast<unsigned long long>(hostWait.end_to_end_ns),
                     static_cast<unsigned long long>(hostWait.cpu_wait_ns),
                     static_cast<unsigned long long>(hostBounce.total_stack_gpu_ns),
                     static_cast<unsigned long long>(hostBounce.end_to_end_ns),
                     static_cast<unsigned long long>(hostBounce.cpu_wait_ns),
                     static_cast<unsigned long long>(hostBounce.host_bounce_copy_ns),
                     static_cast<unsigned long long>(residentInitial.end_to_end_ns),
                     static_cast<unsigned long long>(oneSubmit.end_to_end_ns),
                     static_cast<unsigned long long>(warm.total_stack_gpu_ns),
                     static_cast<unsigned long long>(warm10Median),
                     static_cast<unsigned long long>(warm10P10),
                     static_cast<unsigned long long>(warm10P90),
                     static_cast<unsigned long long>(warm100Median),
                     static_cast<unsigned long long>(warm100P10),
                     static_cast<unsigned long long>(warm100P90));
    }
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM48EvtArtifactSchemaIsTruthful)
{
    const std::string path = std::string(MARIONETTE_TEST_REPO_ROOT) +
        "/internal/prometheus/DevelopmentReport/artifacts/M48/"
        "multi_block_golden_path_evt_closeout.json";
    std::ifstream input(path, std::ios::binary);
    ASSERT_TRUE(input.good(), "the committed M48 status artifact is readable");
    const std::string artifact((std::istreambuf_iterator<char>(input)),
                               std::istreambuf_iterator<char>());
    ASSERT_TRUE(artifact.find("prometheus.m48.multi-block-golden-path.v1") != std::string::npos,
                "the M48 artifact schema is explicit");
    ASSERT_TRUE(artifact.find("\"evt_state\": \"in_progress\"") != std::string::npos,
                "the artifact keeps EVT open until the complete hardware corpus exists");
    ASSERT_TRUE(artifact.find("\"total\": 116") != std::string::npos,
                "the complete persistent resource matrix is recorded");
    ASSERT_TRUE(artifact.find("\"exact_retained_bytes\": 695763968") != std::string::npos,
                "the primary capacity result is deterministic");
    ASSERT_TRUE(artifact.find("\"executed\": true") != std::string::npos,
                "live fixed-stack execution is machine readable");
    ASSERT_TRUE(artifact.find("\"primary_conventional\"") != std::string::npos &&
                    artifact.find("\"four_one_submit_gpu_ns\": 37364288") != std::string::npos,
                "the artifact records measured final-authority primary timing");
    ASSERT_TRUE(artifact.find("\"warm_100_median_gpu_ns\":") != std::string::npos,
                "the artifact retains a real 100-stack distribution rather than one sample");
    ASSERT_TRUE(artifact.find("\"standalone_m47_thin_wrapper_migration\": true") != std::string::npos,
                "the artifact keeps the remaining compatibility authority debt explicit");
}

FACT(PrometheusM44ComposedHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M44 hardware proof requires the proven cooperative tuple");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    constexpr std::uint64_t woGeneration = 500u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "M44 proof prepares the 24 M43 weights");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    prom_m44_wo_prepare_result preparedWo{};
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth,
                                              woGeneration, &preparedWo),
                "persistent Wo prepares once in FP32 and packed forms");
    ASSERT_TRUE(preparedWo.gpu_upload_and_pack_ns > 0u && preparedWo.retained_bytes > 0u,
                "one-time Wo preparation and retention are measured");
    std::vector<float> headReference;
    const prom_m43_reference_result groupedReference =
        GroupReference(x, weights, tokens, modelWidth, headDim, &headReference);
    ASSERT_EQUAL(1u, groupedReference.all_finite, "the M43 source oracle succeeds");
    std::vector<float> roundedExpected;
    const prom_m44_reference_result roundedReference =
        OutputProjectionReference(headReference, wo, tokens, headDim, modelWidth,
                                  PROM_M42_PRECISION_F16_ROUNDED, &roundedExpected);
    ASSERT_EQUAL(1u, roundedReference.all_finite, "the rounded M44 oracle succeeds");
    std::vector<float> output(roundedExpected.size(), 0.0f);
    prom_m44_composed_request request{};
    FillM44ComposedRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                           PROM_M44_AGGREGATION_INTERLEAVE,
                           PROM_M44_PROJECTION_COOPERATIVE,
                           PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
                           PROM_M42_INPUT_HOST_X, 1u, woGeneration);
    prom_m44_composed_result interleave{};
    const int interleaveStatus = prom_reactor_runtime_m44_execute_composed(runtime, &request, &interleave);
    ASSERT_EQUAL(0, interleave.detail_code, "successful M44 detail remains zero");
    ASSERT_EQUAL(0u, interleave.stage, "successful M44 stage remains zero");
    ASSERT_EQUAL(PROM_OK, interleaveStatus,
                 "M43 and packed interleave cooperative projection execute in one submit");
    ASSERT_EQUAL(1u, interleave.submit_count, "the preferred path uses one command buffer/submit");
    ASSERT_EQUAL(1u, interleave.final_readback_count, "only final Y is read back");
    ASSERT_EQUAL(1u, interleave.no_intermediate_host_copy,
                 "the composed device plan never reads back the eight heads");
    ASSERT_EQUAL(0u, interleave.attention.final_readback_count,
                 "M43's terminal benchmark readback is removed in composition");
    ASSERT_TRUE(interleave.aggregation_gpu_ns > 0u && interleave.projection_gpu_ns > 0u &&
                interleave.m44_gpu_ns > 0u && interleave.total_m43_m44_gpu_ns > interleave.m44_gpu_ns,
                "aggregation, projection, M44, and product intervals are distinct");
    ASSERT_EQUAL(tokens, interleave.output_view.logical_rows,
                 "Y view retains its logical token count");
    ASSERT_EQUAL(modelWidth, interleave.output_view.logical_columns,
                 "Y view retains its logical model width");
    ASSERT_EQUAL(modelWidth, interleave.output_view.row_stride_elements,
                 "aligned Y is contiguous row-major");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_DEVICE_ACCESS_COMPUTE_READ),
                 interleave.output_view.required_consumer_access,
                 "Y is ready for a future bounded device consumer");
    for (std::uint32_t head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_DEVICE_ACCESS_COMPUTE_READ),
                     interleave.attention.head_output_view[head].required_consumer_access,
                     "every M43 head view names its M44 consumer access");
        ASSERT_EQUAL(interleave.logical_request_id,
                     interleave.attention.head_output_view[head].owning_lifetime_id,
                     "all heads remain under the composed logical lifetime");
    }
    prom_m44_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                     tokens, modelWidth, 8.0e-3f, 3.0e-2f,
                                                     interleave.plan.aggregation_strategy,
                                                     woGeneration,
                                                     interleave.plan.m43_aggregate_replay_id,
                                                     interleave.plan.replay_id, &mismatch),
                 "packed interleave cooperative Y matches the exact rounded oracle");

    std::fill(output.begin(), output.end(), 0.0f);
    request.aggregation_strategy = PROM_M44_AGGREGATION_DIRECT_SEGMENTED;
    request.projection_path = PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16;
    prom_m44_composed_result direct{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &direct),
                 "the direct eight-segment projection executes without a concatenate buffer");
    ASSERT_EQUAL(0u, direct.plan.memory.contiguous_packed_bytes,
                 "direct execution materializes no hidden C");
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                     tokens, modelWidth, 3.0e-3f, 2.0e-2f,
                                                     direct.plan.aggregation_strategy,
                                                     woGeneration,
                                                     direct.plan.m43_aggregate_replay_id,
                                                     direct.plan.replay_id, &mismatch),
                 "direct segmented Y matches the same rounded oracle");

    std::fill(output.begin(), output.end(), 0.0f);
    request.aggregation_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request.projection_path = PROM_M44_PROJECTION_CONVENTIONAL_FP16;
    prom_m44_composed_result conventional{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &conventional),
                 "the same-precision conventional FP16 fallback executes");
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                     tokens, modelWidth, 8.0e-3f, 3.0e-2f,
                                                     conventional.plan.aggregation_strategy,
                                                     woGeneration,
                                                     conventional.plan.m43_aggregate_replay_id,
                                                     conventional.plan.replay_id, &mismatch),
                 "conventional FP16 consumes the identical rounded input contract");

    std::vector<float> exactExpected;
    const prom_m44_reference_result exactReference =
        OutputProjectionReference(headReference, wo, tokens, headDim, modelWidth,
                                  PROM_M42_PRECISION_FP32, &exactExpected);
    ASSERT_EQUAL(1u, exactReference.all_finite, "the FP32 product oracle succeeds");
    std::fill(output.begin(), output.end(), 0.0f);
    request.projection_path = PROM_M44_PROJECTION_A2X4_FP32;
    prom_m44_composed_result a2x4{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &a2x4),
                 "device interleave plus A2x4 FP32 executes as a labeled baseline");
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(exactExpected.data(), output.data(),
                                                     tokens, modelWidth, 3.0e-3f, 2.0e-2f,
                                                     a2x4.plan.aggregation_strategy,
                                                     woGeneration,
                                                     a2x4.plan.m43_aggregate_replay_id,
                                                     a2x4.plan.replay_id, &mismatch),
                 "A2x4 Y matches the separately labeled FP32 oracle");

    std::fill(output.begin(), output.end(), 0.0f);
    request.projection_path = PROM_M44_PROJECTION_COOPERATIVE;
    request.submit_plan = PROM_M44_SUBMIT_TWO_BOUNDED;
    prom_m44_composed_result twoSubmit{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &twoSubmit),
                 "the semaphore-linked bounded second-submit comparison executes");
    ASSERT_EQUAL(2u, twoSubmit.submit_count, "the comparison reports two submissions");
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                     tokens, modelWidth, 8.0e-3f, 3.0e-2f,
                                                     twoSubmit.plan.aggregation_strategy,
                                                     woGeneration,
                                                     twoSubmit.plan.m43_aggregate_replay_id,
                                                     twoSubmit.plan.replay_id, &mismatch),
                 "two-submit composition preserves Y");
    prom_m44_composed_result twoSubmitWarm{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &twoSubmitWarm),
                 "the second physical ring slot reaches warm capacity");
    prom_m44_composed_result twoSubmitWarmAgain{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &twoSubmitWarmAgain),
                 "the warmed composed path repeats");
    ASSERT_EQUAL(twoSubmitWarm.buffer_allocation_count, twoSubmitWarmAgain.buffer_allocation_count,
                 "warm M43+M44 execution performs no Vulkan buffer allocation");

    std::vector<float> actualHeads(headReference.size(), 0.0f);
    prom_m43_attention_group_request groupedRequest{};
    FillGroupExecutionRequest(&groupedRequest, x.data(), actualHeads.data(), tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X, 1u);
    prom_m43_attention_group_result grouped{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &groupedRequest, &grouped),
                 "the audit-only bad baseline first copies M43 heads to host");
    prom_m44_host_bounce_request hostRequest{};
    hostRequest.head_major = actualHeads.data();
    hostRequest.head_major_element_count = actualHeads.size();
    hostRequest.output = output.data();
    hostRequest.output_element_count = output.size();
    hostRequest.head_count = PROM_M44_HEAD_COUNT;
    hostRequest.tokens = tokens;
    hostRequest.head_dim = headDim;
    hostRequest.model_width = modelWidth;
    hostRequest.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
    hostRequest.projection_path = PROM_M44_PROJECTION_COOPERATIVE;
    hostRequest.required_wo_generation = woGeneration;
    hostRequest.m43_aggregate_replay_id = grouped.plan.aggregate_replay_id;
    prom_m44_host_bounce_result hostBounce{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_host_bounce(runtime, &hostRequest, &hostBounce),
                 "CPU concatenate/reupload/output-projection baseline executes");
    ASSERT_EQUAL(1u, hostBounce.intermediate_host_copy_count,
                 "the audit baseline labels its device-residency violation");
    ASSERT_TRUE(hostBounce.cpu_concatenate_ns > 0u && hostBounce.projection_gpu_ns > 0u &&
                hostBounce.end_to_end_ns > 0u,
                "host bounce records concatenate, projection, and end-to-end cost");
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                     tokens, modelWidth, 8.0e-3f, 3.0e-2f,
                                                     PROM_M44_AGGREGATION_INTERLEAVE,
                                                     woGeneration,
                                                     grouped.plan.aggregate_replay_id,
                                                     hostBounce.replay_id, &mismatch),
                 "host-bounce Y matches the rounded oracle");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M44 services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count,
                 "M44 hardware proof is validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count,
                 "M44 hardware proof is validation-error clean");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM45ComposedOwnershipAndLifecycleHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M45 hardware proof requires the proven cooperative tuple");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    constexpr std::uint64_t xGeneration = 45u;
    constexpr std::uint64_t woGeneration = 545u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    const std::vector<float> originalX = x;
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "M45 prepares all M43 weights");
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data();
    prepareX.element_count = x.size();
    prepareX.tokens = tokens;
    prepareX.model_width = modelWidth;
    prepareX.generation = xGeneration;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "M45 prepares one immutable resident X generation");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, woGeneration),
                "M45 prepares persistent Wo");
    std::vector<float> heads;
    const prom_m43_reference_result headReference =
        GroupReference(x, weights, tokens, modelWidth, headDim, &heads);
    ASSERT_EQUAL(1u, headReference.all_finite, "the grouped attention oracle succeeds");
    std::vector<float> y;
    const prom_m44_reference_result yReference =
        OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                  PROM_M42_PRECISION_F16_ROUNDED, &y);
    ASSERT_EQUAL(1u, yReference.all_finite, "the pre-residual Y oracle succeeds");
    std::vector<float> expected(y.size());
    for (std::size_t index = 0u; index < expected.size(); ++index) expected[index] = x[index] + y[index];

    std::uint64_t primedAllocations = 0u;
    for (const std::uint32_t strategy : {PROM_M45_STRATEGY_SEPARATE_OUTPUT,
                                        PROM_M45_STRATEGY_IN_PLACE_Y}) {
        for (const std::uint32_t submit : {PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                                          PROM_M45_SUBMIT_TWO_BOUNDED}) {
            std::vector<float> output(expected.size(), 0.0f);
            prom_m45_composed_request composed{};
            FillM45ComposedRequest(&composed, output.data(), tokens, modelWidth, headDim,
                                   strategy, submit, xGeneration, woGeneration);
            prom_m45_composed_result result{};
            ASSERT_EQUAL(PROM_OK,
                         prom_reactor_runtime_m45_execute_composed(runtime, &composed, &result),
                         "real resident X plus real M44 Y executes through M45");
            prom_m45_mismatch mismatch{};
            ASSERT_EQUAL(PROM_OK,
                         prom_m45_residual_compare(expected.data(), output.data(), tokens,
                                                   modelWidth, 1.0e-2f, 4.0e-2f,
                                                   &result.residual_plan, &mismatch),
                         "M45 final Z matches X plus the precision-specific M44 Y oracle");
            ASSERT_EQUAL(submit == PROM_M45_SUBMIT_TWO_BOUNDED ? 2u : 1u,
                         result.submit_count, "M45 reports the exact bounded submit count");
            ASSERT_EQUAL(1u, result.final_readback_count, "only final Z is read back");
            ASSERT_EQUAL(1u, result.no_intermediate_host_copy,
                         "M43/M44/M45 composition has no intermediate host copy");
            ASSERT_EQUAL(1u, result.physical_slot_recyclable,
                         "known completion makes the complete composed slot recyclable");
            ASSERT_EQUAL(xGeneration, result.x_generation, "the resident X generation is retained");
            ASSERT_TRUE(result.y_generation != 0u && result.z_generation != 0u &&
                        result.y_generation != result.z_generation,
                        "pre- and post-residual logical generations are distinct");
            ASSERT_TRUE(result.residual_gpu_ns > 0u && result.total_m43_m44_m45_gpu_ns > 0u,
                        "residual and complete-product GPU intervals are measured");
            if (strategy == PROM_M45_STRATEGY_IN_PLACE_Y) {
                ASSERT_EQUAL(result.y_view.buffer, result.z_view.buffer,
                             "in-place Y reuses the exact physical buffer as logical Z");
                ASSERT_EQUAL(result.y_view.owning_slot_generation,
                             result.z_view.owning_slot_generation,
                             "physical slot generation stays stable across the content transition");
                ASSERT_EQUAL(0u, result.residual_plan.memory.z_device_bytes,
                             "in-place Y retains no separate Z allocation");
            } else {
                ASSERT_TRUE(result.y_view.buffer != result.z_view.buffer,
                            "separate output keeps X, Y, and Z disjoint");
                ASSERT_TRUE(result.residual_plan.memory.z_device_bytes > 0u,
                            "separate output owns one grow-only Z allocation");
            }
            primedAllocations = result.buffer_allocation_count;
        }
    }
    ASSERT_TRUE(x == originalX, "resident X remains immutable across every residual strategy");

    prom_m45_composed_request retainedRequest{};
    FillM45ComposedRequest(&retainedRequest, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y, PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           xGeneration, woGeneration);
    prom_m45_composed_result retained{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &retainedRequest, &retained),
                 "M45 may retain Z without any readback");
    ASSERT_EQUAL(0u, retained.final_readback_count, "retained-only execution performs no readback");
    ASSERT_TRUE(retained.z_view.buffer != VK_NULL_HANDLE,
                "retained-only execution exposes one clean FP32 Z device view");
    ASSERT_EQUAL(primedAllocations, retained.buffer_allocation_count,
                 "warm in-place execution performs no Vulkan buffer allocation");

    prom_m45_composed_request staleRequest = retainedRequest;
    staleRequest.attention.shared_x_generation = xGeneration - 1u;
    prom_m45_composed_result stale{};
    ASSERT_TRUE(prom_reactor_runtime_m45_execute_composed(runtime, &staleRequest, &stale) != PROM_OK,
                "stale resident X rejects before slot acquisition");
    ASSERT_EQUAL(PROM_M45_DETAIL_STALE_X_GENERATION, stale.detail_code,
                 "stale X has an exact M45 diagnostic");
    staleRequest = retainedRequest;
    staleRequest.residual_strategy = PROM_M45_STRATEGY_IN_PLACE_X_AUDIT;
    ASSERT_TRUE(prom_reactor_runtime_m45_execute_composed(runtime, &staleRequest, &stale) != PROM_OK,
                "runtime in-place X remains rejected");
    ASSERT_EQUAL(PROM_M45_DETAIL_IN_PLACE_X_REJECTED, stale.detail_code,
                 "in-place X audit rejection is explicit");

    std::vector<float> faultOutput(expected.size(), 0.0f);
    prom_m45_composed_request faultRequest{};
    FillM45ComposedRequest(&faultRequest, faultOutput.data(), tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y, PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           xGeneration, woGeneration);
    for (std::uint32_t fault = PROM_M45_FAULT_BEFORE_RESIDUAL_BARRIERS;
         fault <= PROM_M45_FAULT_BEFORE_FINAL_READBACK; ++fault) {
        faultRequest.fault_point = fault;
        prom_m45_composed_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m45_execute_composed(runtime, &faultRequest, &failed) != PROM_OK,
                    "known-completion M45 fault reports logical failure");
        ASSERT_EQUAL(PROM_M45_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "known-completion fault has the exact detail");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "known-completion fault returns the aliased slot exactly once");
    }
    faultRequest.fault_point = PROM_M45_FAULT_AFTER_RESIDUAL_SUBMISSION;
    prom_m45_composed_result afterSubmit{};
    ASSERT_TRUE(prom_reactor_runtime_m45_execute_composed(runtime, &faultRequest, &afterSubmit) != PROM_OK,
                "post-submit logical fault remains recyclable after known completion");
    ASSERT_EQUAL(1u, afterSubmit.physical_slot_recyclable,
                 "post-submit known completion preserves recyclability");
    faultRequest.fault_point = PROM_M45_FAULT_UNCERTAIN_COMPLETION;
    prom_m45_composed_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m45_execute_composed(runtime, &faultRequest, &uncertain) != PROM_OK,
                "uncertain M45 completion reports failure");
    ASSERT_EQUAL(PROM_M45_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "uncertain completion is distinct from logical failure");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "uncertain completion quarantines the entire composed slot");

    prepareX.generation = xGeneration + 1u;
    prom_m43_resident_x_prepare_result replacedX{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &replacedX),
                 "resident-X replacement waits and reaps the quarantined M45 slot");
    ASSERT_EQUAL(1u, replacedX.replaced, "resident X replacement is explicit");
    faultRequest.fault_point = PROM_M45_FAULT_NONE;
    faultRequest.attention.shared_x_generation = xGeneration + 1u;
    prom_m45_composed_result recovered{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m45_execute_composed(runtime, &faultRequest, &recovered),
                 "M45 recovers after quarantine reap with the newer X generation");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M45 validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "M45 produces no validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "M45 produces no validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM46ComposedRmsNormHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M46 hardware proof requires the proven cooperative tuple");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    constexpr std::uint64_t xGeneration = 46u;
    constexpr std::uint64_t woGeneration = 646u;
    constexpr std::uint64_t weightGeneration = 746u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "M46 prepares all grouped attention weights");
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data();
    prepareX.element_count = x.size();
    prepareX.tokens = tokens;
    prepareX.model_width = modelWidth;
    prepareX.generation = xGeneration;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "M46 prepares one immutable resident X generation");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, woGeneration),
                "M46 prepares persistent Wo");
    std::vector<float> weight(modelWidth);
    for (std::uint32_t column = 0u; column < modelWidth; ++column) {
        weight[column] = 0.75f + static_cast<float>(column % 11u) / 32.0f;
    }
    prom_m46_weight_prepare_request prepareWeight{};
    prepareWeight.values = weight.data();
    prepareWeight.element_count = weight.size();
    prepareWeight.model_width = modelWidth;
    prepareWeight.generation = weightGeneration;
    prom_m46_weight_prepare_result preparedWeight{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m46_prepare_weight(runtime, &prepareWeight, &preparedWeight),
                 "M46 uploads one persistent generation-checked scale vector");
    ASSERT_EQUAL(weightGeneration, preparedWeight.generation,
                 "the prepared Weight generation is exact");
    ASSERT_TRUE(preparedWeight.hash != 0u && preparedWeight.preparation_ns > 0u,
                "Weight preparation records identity and one-time cost");

    std::vector<float> heads;
    ASSERT_EQUAL(1u, GroupReference(x, weights, tokens, modelWidth, headDim, &heads).all_finite,
                 "M46 grouped oracle succeeds");
    std::vector<float> y;
    ASSERT_EQUAL(1u, OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                               PROM_M42_PRECISION_F16_ROUNDED, &y).all_finite,
                 "M46 output projection oracle succeeds");
    std::vector<float> z(y.size());
    for (std::size_t index = 0u; index < z.size(); ++index) z[index] = x[index] + y[index];
    std::vector<float> expected(z.size(), 0.0f);
    std::vector<float> invRms(tokens, 0.0f);
    prom_m46_reference_request reference{};
    reference.z = z.data();
    reference.weight = weight.data();
    reference.n = expected.data();
    reference.inv_rms = invRms.data();
    reference.z_element_count = z.size();
    reference.weight_element_count = weight.size();
    reference.n_element_count = expected.size();
    reference.tokens = tokens;
    reference.model_width = modelWidth;
    reference.z_row_stride = modelWidth;
    reference.n_row_stride = modelWidth;
    reference.epsilon = 1.0e-5f;
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&reference),
                 "M46 exact FP32 CPU oracle succeeds");

    for (const std::uint32_t strategy : {PROM_M46_STRATEGY_SEPARATE_OUTPUT,
                                         PROM_M46_STRATEGY_IN_PLACE_Z}) {
        for (const std::uint32_t submit : {PROM_M46_SUBMIT_ONE_COMMAND_BUFFER,
                                           PROM_M46_SUBMIT_TWO_BOUNDED}) {
            std::vector<float> output(expected.size(), 0.0f);
            prom_m46_composed_request composed{};
            FillM45ComposedRequest(&composed.upstream, nullptr, tokens, modelWidth, headDim,
                                   PROM_M45_STRATEGY_IN_PLACE_Y,
                                   PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                                   xGeneration, woGeneration);
            composed.output = output.data();
            composed.output_element_count = output.size();
            composed.epsilon = 1.0e-5f;
            composed.strategy = strategy;
            composed.submit_policy = submit;
            composed.required_weight_generation = weightGeneration;
            prom_m46_composed_result result{};
            ASSERT_EQUAL(PROM_OK,
                         prom_reactor_runtime_m46_execute_composed(runtime, &composed, &result),
                         "real retained M45 Z executes through bounded device RMSNorm");
            prom_m46_mismatch mismatch{};
            ASSERT_EQUAL(PROM_OK,
                         prom_m46_rmsnorm_compare(expected.data(), output.data(), tokens, modelWidth,
                                                  1.0e-3f, 2.0e-2f,
                                                  &result.rmsnorm_plan, nullptr,
                                                  invRms.data(), &mismatch),
                         "device RMSNorm N matches the FP32 reference");
            ASSERT_EQUAL(submit == PROM_M46_SUBMIT_TWO_BOUNDED ? 2u : 1u,
                         result.submit_count, "M46 reports the exact submit topology");
            ASSERT_EQUAL(1u, result.final_readback_count, "only final N is read back");
            ASSERT_EQUAL(1u, result.no_intermediate_host_copy,
                         "real Z remains device-resident through normalization");
            ASSERT_TRUE(result.m46_gpu_ns > 0u && result.apply_gpu_ns > 0u &&
                        result.total_m43_m44_m45_m46_gpu_ns > result.m46_gpu_ns,
                        "RMSNorm and complete-fragment GPU intervals are measured");
            ASSERT_EQUAL(result.upstream.physical_slot_generation,
                         result.n_view.owning_slot_generation,
                         "the bounded owner keeps one physical slot generation through N");
            if (strategy == PROM_M46_STRATEGY_IN_PLACE_Z) {
                ASSERT_EQUAL(result.upstream.z_view.buffer, result.n_view.buffer,
                             "in-place Z transitions the exact physical buffer to N");
                ASSERT_EQUAL(0u, result.rmsnorm_plan.memory.n_device_bytes,
                             "in-place Z allocates no duplicate full N");
            } else {
                ASSERT_TRUE(result.upstream.z_view.buffer != result.n_view.buffer,
                            "separate output keeps Z read-only and N disjoint");
            }
        }
    }

    std::vector<float> faultOutput(expected.size(), 0.0f);
    prom_m46_composed_request faultRequest{};
    FillM45ComposedRequest(&faultRequest.upstream, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y,
                           PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           xGeneration, woGeneration);
    faultRequest.output = faultOutput.data();
    faultRequest.output_element_count = faultOutput.size();
    faultRequest.epsilon = 1.0e-5f;
    faultRequest.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    faultRequest.submit_policy = PROM_M46_SUBMIT_TWO_BOUNDED;
    faultRequest.required_weight_generation = weightGeneration;
    for (std::uint32_t fault = PROM_M46_FAULT_BEFORE_REDUCTION;
         fault <= PROM_M46_FAULT_BEFORE_FINAL_READBACK; ++fault) {
        faultRequest.fault_point = fault;
        prom_m46_composed_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m46_execute_composed(runtime, &faultRequest, &failed) != PROM_OK,
                    "known-completion M46 fault reports logical failure");
        ASSERT_EQUAL(PROM_M46_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "known-completion M46 fault has the exact detail");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "known completion returns the Z/N alias exactly once");
    }
    faultRequest.fault_point = PROM_M46_FAULT_UNCERTAIN_COMPLETION;
    prom_m46_composed_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m46_execute_composed(runtime, &faultRequest, &uncertain) != PROM_OK,
                "uncertain M46 completion reports failure");
    ASSERT_EQUAL(PROM_M46_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "uncertain M46 completion is distinct from a logical fault");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "uncertain M46 completion quarantines the complete composed slot");
    prepareWeight.generation = weightGeneration + 1u;
    prom_m46_weight_prepare_result replacedWeight{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m46_prepare_weight(runtime, &prepareWeight, &replacedWeight),
                 "Weight replacement waits for and reaps the quarantined M46 slot");
    ASSERT_EQUAL(1u, replacedWeight.replaced, "Weight replacement is explicit");
    faultRequest.fault_point = PROM_M46_FAULT_NONE;
    faultRequest.required_weight_generation = weightGeneration + 1u;
    prom_m46_composed_result recovered{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m46_execute_composed(runtime, &faultRequest, &recovered),
                 "M46 recovers after quarantine reap with the new Weight generation");

    prom_m46_composed_request stale{};
    FillM45ComposedRequest(&stale.upstream, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y,
                           PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           xGeneration, woGeneration);
    stale.epsilon = 1.0e-5f;
    stale.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    stale.submit_policy = PROM_M46_SUBMIT_TWO_BOUNDED;
    stale.required_weight_generation = weightGeneration - 1u;
    prom_m46_composed_result staleResult{};
    ASSERT_TRUE(prom_reactor_runtime_m46_execute_composed(runtime, &stale, &staleResult) != PROM_OK,
                "stale RMSNorm Weight generation rejects before M45 execution");
    ASSERT_EQUAL(PROM_M46_DETAIL_STALE_WEIGHT_GENERATION, staleResult.detail_code,
                 "stale Weight rejection is exact");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M46 validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "M46 produces no validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "M46 produces no validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM47CompleteTransformerBlockHardwareProof)
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
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    constexpr std::uint32_t ffnWidth = 256u;
    constexpr std::uint64_t xGeneration = 470u;
    constexpr std::uint64_t woGeneration = 471u;
    constexpr std::uint64_t normGeneration = 472u;
    std::array<std::uint64_t, PROM_M47_WEIGHT_COUNT> ffnGeneration{473u, 474u, 475u};
    std::vector<float> x;
    GroupWeights attentionWeights;
    FillGroupInputs(&x, &attentionWeights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, attentionWeights, modelWidth, headDim),
                "M47 prepares all grouped attention weights");
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data();
    prepareX.element_count = x.size();
    prepareX.tokens = tokens;
    prepareX.model_width = modelWidth;
    prepareX.generation = xGeneration;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "M47 prepares the immutable resident X");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, woGeneration),
                "M47 prepares persistent Wo");
    std::vector<float> normWeight(modelWidth);
    for (std::uint32_t column = 0u; column < modelWidth; ++column)
        normWeight[column] = 0.75f + static_cast<float>(column % 11u) / 32.0f;
    prom_m46_weight_prepare_request prepareNorm{};
    prepareNorm.values = normWeight.data();
    prepareNorm.element_count = normWeight.size();
    prepareNorm.model_width = modelWidth;
    prepareNorm.generation = normGeneration;
    prom_m46_weight_prepare_result preparedNorm{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m46_prepare_weight(runtime, &prepareNorm, &preparedNorm),
                 "M47 prepares the persistent RMSNorm scale");

    std::array<std::vector<float>, PROM_M47_WEIGHT_COUNT> ffnWeights;
    ffnWeights[PROM_M47_WEIGHT_GATE].resize(static_cast<std::size_t>(modelWidth) * ffnWidth);
    ffnWeights[PROM_M47_WEIGHT_UP].resize(static_cast<std::size_t>(modelWidth) * ffnWidth);
    ffnWeights[PROM_M47_WEIGHT_DOWN].resize(static_cast<std::size_t>(ffnWidth) * modelWidth);
    for (std::size_t index = 0u; index < ffnWeights[PROM_M47_WEIGHT_GATE].size(); ++index) {
        ffnWeights[PROM_M47_WEIGHT_GATE][index] =
            static_cast<float>(static_cast<int>((index * 7u + 3u) % 29u) - 14) / 512.0f;
        ffnWeights[PROM_M47_WEIGHT_UP][index] =
            static_cast<float>(static_cast<int>((index * 11u + 5u) % 31u) - 15) / 512.0f;
    }
    for (std::size_t index = 0u; index < ffnWeights[PROM_M47_WEIGHT_DOWN].size(); ++index) {
        ffnWeights[PROM_M47_WEIGHT_DOWN][index] =
            static_cast<float>(static_cast<int>((index * 13u + 7u) % 37u) - 18) / 512.0f;
    }
    for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind) {
        prom_m47_weight_prepare_request prepare{};
        prepare.values = ffnWeights[kind].data();
        prepare.element_count = ffnWeights[kind].size();
        prepare.kind = kind;
        prepare.model_width = modelWidth;
        prepare.ffn_width = ffnWidth;
        prepare.generation = ffnGeneration[kind];
        prom_m47_weight_prepare_result prepared{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_prepare_weight(runtime, &prepare, &prepared),
                     "each independent FFN weight generation prepares once");
        ASSERT_EQUAL(ffnGeneration[kind], prepared.generation,
                     "the independent FFN weight generation is exact");
        ASSERT_TRUE(prepared.hash != 0u && prepared.retained_f32_bytes > 0u &&
                    prepared.retained_packed_bytes > 0u,
                    "each FFN weight retains FP32 and packed representations");
    }

    std::vector<float> heads;
    ASSERT_EQUAL(1u, GroupReference(x, attentionWeights, tokens, modelWidth, headDim, &heads).all_finite,
                 "M47 grouped attention oracle succeeds");
    std::vector<float> y;
    ASSERT_EQUAL(1u, OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                               PROM_M42_PRECISION_F16_ROUNDED, &y).all_finite,
                 "M47 output projection oracle succeeds");
    std::vector<float> z(y.size());
    for (std::size_t index = 0u; index < z.size(); ++index) z[index] = x[index] + y[index];
    std::vector<float> n(z.size());
    std::vector<float> invRms(tokens);
    prom_m46_reference_request normReference{};
    normReference.z = z.data();
    normReference.weight = normWeight.data();
    normReference.n = n.data();
    normReference.inv_rms = invRms.data();
    normReference.z_element_count = z.size();
    normReference.weight_element_count = normWeight.size();
    normReference.n_element_count = n.size();
    normReference.tokens = tokens;
    normReference.model_width = modelWidth;
    normReference.z_row_stride = modelWidth;
    normReference.n_row_stride = modelWidth;
    normReference.epsilon = 1.0e-5f;
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&normReference),
                 "M47 retained N oracle succeeds");

    struct Case { std::uint32_t path; std::uint32_t gating; std::uint32_t residual; std::uint32_t submit; };
    const std::array<Case, 5u> cases{{
        {PROM_M47_PROJECTION_A2X4_FP32, PROM_M47_GATING_SEPARATE,
         PROM_M47_RESIDUAL_SEPARATE_OUTPUT, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {PROM_M47_PROJECTION_A2X4_FP32, PROM_M47_GATING_FUSED_FP32,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_TWO_BOUNDED},
        {PROM_M47_PROJECTION_CONVENTIONAL_FP16, PROM_M47_GATING_FUSED_FP32,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {PROM_M47_PROJECTION_CONVENTIONAL_FP16, PROM_M47_GATING_FUSED_DIRECT_PACKED,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_TWO_BOUNDED},
        {PROM_M47_PROJECTION_COOPERATIVE, PROM_M47_GATING_FUSED_DIRECT_PACKED,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
    }};
    for (const Case& testCase : cases) {
        if (testCase.path == PROM_M47_PROJECTION_COOPERATIVE &&
            services.cooperative_matrix_feature_enabled == 0u) continue;
        std::vector<float> gate(static_cast<std::size_t>(tokens) * ffnWidth);
        std::vector<float> up(gate.size());
        std::vector<float> hidden(gate.size());
        std::vector<float> down(static_cast<std::size_t>(tokens) * modelWidth);
        std::vector<float> expected(down.size());
        prom_m47_reference_request reference{};
        reference.n = n.data();
        reference.wgate = ffnWeights[PROM_M47_WEIGHT_GATE].data();
        reference.wup = ffnWeights[PROM_M47_WEIGHT_UP].data();
        reference.wdown = ffnWeights[PROM_M47_WEIGHT_DOWN].data();
        reference.gate = gate.data();
        reference.up = up.data();
        reference.hidden = hidden.data();
        reference.down = down.data();
        reference.output = expected.data();
        reference.n_element_count = n.size();
        reference.wgate_element_count = ffnWeights[PROM_M47_WEIGHT_GATE].size();
        reference.wup_element_count = ffnWeights[PROM_M47_WEIGHT_UP].size();
        reference.wdown_element_count = ffnWeights[PROM_M47_WEIGHT_DOWN].size();
        reference.output_element_count = expected.size();
        reference.tokens = tokens;
        reference.model_width = modelWidth;
        reference.ffn_width = ffnWidth;
        reference.n_row_stride = modelWidth;
        reference.output_row_stride = modelWidth;
        reference.projection_path = testCase.path;
        ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_cpu_reference(&reference),
                     "precision-matched complete block oracle succeeds");

        std::vector<float> output(expected.size());
        prom_m47_composed_request composed{};
        FillM45ComposedRequest(&composed.upstream.upstream, nullptr, tokens, modelWidth, headDim,
                               PROM_M45_STRATEGY_IN_PLACE_Y,
                               PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                               xGeneration, woGeneration);
        composed.upstream.epsilon = 1.0e-5f;
        composed.upstream.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
        composed.upstream.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
        composed.upstream.required_weight_generation = normGeneration;
        composed.output = output.data();
        composed.output_element_count = output.size();
        composed.ffn_width = ffnWidth;
        composed.projection_path = testCase.path;
        composed.gating_strategy = testCase.gating;
        composed.residual_strategy = testCase.residual;
        composed.submit_policy = testCase.submit;
        for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind)
            composed.required_weight_generation[kind] = ffnGeneration[kind];
        prom_m47_composed_result result{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m47_execute_composed(runtime, &composed, &result),
                     "real retained M46 N executes through the complete device FFN tail");
        prom_m47_mismatch mismatch{};
        const int compareStatus = prom_m47_gated_ffn_compare(
            expected.data(), output.data(), tokens, modelWidth, modelWidth, modelWidth,
            2.0e-3f, 3.0e-2f, &result.ffn_plan, gate.data(), up.data(), hidden.data(),
            down.data(), &mismatch);
        if (compareStatus != PROM_OK) {
            std::fprintf(stderr,
                         "M47 mismatch path=%u gating=%u residual=%u submit=%u token=%u column=%u expected=%g actual=%g abs=%g rel=%g\n",
                         testCase.path, testCase.gating, testCase.residual, testCase.submit,
                         mismatch.token, mismatch.column, mismatch.expected, mismatch.actual,
                         mismatch.absolute_error, mismatch.relative_error);
        }
        ASSERT_EQUAL(PROM_OK, compareStatus,
                     "complete BlockOutput matches the precision-matched oracle");
        ASSERT_EQUAL(testCase.submit == PROM_M47_SUBMIT_TWO_BOUNDED ? 2u : 1u,
                     result.submit_count, "M47 reports the exact complete-block submit topology");
        ASSERT_EQUAL(1u, result.final_readback_count, "only final BlockOutput is read back");
        ASSERT_EQUAL(1u, result.no_intermediate_host_copy,
                     "N, Gate, Up, Hidden, and Down remain device-resident");
        ASSERT_TRUE(result.gate_projection_gpu_ns > 0u && result.up_projection_gpu_ns > 0u &&
                    result.down_projection_gpu_ns > 0u && result.residual_gpu_ns > 0u &&
                    result.m47_gpu_ns > result.residual_gpu_ns,
                    "all M47 GPU stages have exact nonzero timing");
        ASSERT_EQUAL(result.upstream.upstream.physical_slot_generation,
                     result.output_view.owning_slot_generation,
                     "the complete block retains one physical slot generation");
        if (testCase.residual == PROM_M47_RESIDUAL_IN_PLACE_DOWN) {
            ASSERT_EQUAL(result.ffn_plan.down_row_stride, result.output_view.row_stride_elements,
                         "in-place Down retains the projection output stride");
            ASSERT_EQUAL(0u, result.ffn_plan.memory.separate_output_bytes,
                         "in-place Down allocates no separate BlockOutput");
        }
    }

    std::vector<float> lifecycleOutput(static_cast<std::size_t>(tokens) * modelWidth);
    prom_m47_composed_request lifecycle{};
    FillM45ComposedRequest(&lifecycle.upstream.upstream, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y,
                           PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           xGeneration, woGeneration);
    lifecycle.upstream.epsilon = 1.0e-5f;
    lifecycle.upstream.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    lifecycle.upstream.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
    lifecycle.upstream.required_weight_generation = normGeneration;
    lifecycle.output = lifecycleOutput.data();
    lifecycle.output_element_count = lifecycleOutput.size();
    lifecycle.ffn_width = ffnWidth;
    lifecycle.projection_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
    lifecycle.gating_strategy = PROM_M47_GATING_FUSED_DIRECT_PACKED;
    lifecycle.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_DOWN;
    lifecycle.submit_policy = PROM_M47_SUBMIT_TWO_BOUNDED;
    for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind)
        lifecycle.required_weight_generation[kind] = ffnGeneration[kind];
    for (std::uint32_t fault = PROM_M47_FAULT_BEFORE_GATE;
         fault <= PROM_M47_FAULT_BEFORE_FINAL_READBACK; ++fault) {
        lifecycle.fault_point = fault;
        lifecycle.gating_strategy = fault == PROM_M47_FAULT_DURING_ACTIVATION
                                        ? PROM_M47_GATING_SEPARATE
                                        : PROM_M47_GATING_FUSED_DIRECT_PACKED;
        prom_m47_composed_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m47_execute_composed(runtime, &lifecycle, &failed) != PROM_OK,
                    "known-completion M47 faults report logical failure");
        ASSERT_EQUAL(PROM_M47_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "known-completion M47 fault detail is exact");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "known completion returns the complete slot exactly once");
    }
    lifecycle.gating_strategy = PROM_M47_GATING_FUSED_DIRECT_PACKED;
    lifecycle.fault_point = PROM_M47_FAULT_UNCERTAIN_COMPLETION;
    prom_m47_composed_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m47_execute_composed(runtime, &lifecycle, &uncertain) != PROM_OK,
                "uncertain M47 completion reports failure");
    ASSERT_EQUAL(PROM_M47_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "uncertain M47 completion is distinct from a logical fault");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "uncertain completion quarantines the complete M43-M47 slot");

    for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind) {
        ffnGeneration[kind] += 100u;
        prom_m47_weight_prepare_request replace{};
        replace.values = ffnWeights[kind].data();
        replace.element_count = ffnWeights[kind].size();
        replace.kind = kind;
        replace.model_width = modelWidth;
        replace.ffn_width = ffnWidth;
        replace.generation = ffnGeneration[kind];
        prom_m47_weight_prepare_result replaced{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_prepare_weight(runtime, &replace, &replaced),
                     "independent FFN weight replacement waits and reaps all users");
        ASSERT_EQUAL(1u, replaced.replaced, "each FFN weight replacement is explicit");
        ASSERT_EQUAL(ffnGeneration[kind], replaced.generation,
                     "only the selected FFN weight publishes its next generation");
        lifecycle.required_weight_generation[kind] = ffnGeneration[kind];
    }
    lifecycle.fault_point = PROM_M47_FAULT_NONE;
    prom_m47_composed_request stale = lifecycle;
    stale.required_weight_generation[PROM_M47_WEIGHT_GATE] -= 1u;
    prom_m47_composed_result staleResult{};
    ASSERT_TRUE(prom_reactor_runtime_m47_execute_composed(runtime, &stale, &staleResult) != PROM_OK,
                "a stale independent FFN generation rejects before slot acquisition");
    ASSERT_EQUAL(PROM_M47_DETAIL_STALE_WEIGHT_GENERATION, staleResult.detail_code,
                 "stale FFN weight rejection is exact");
    prom_m47_composed_result warmFirst{};
    prom_m47_composed_result warmSecond{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &lifecycle, &warmFirst),
                 "M47 recovers after quarantine reap and independent replacements");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &lifecycle, &warmSecond),
                 "a second warmed complete block execution succeeds");
    ASSERT_EQUAL(warmFirst.buffer_allocation_count, warmSecond.buffer_allocation_count,
                 "warm repeated M47 execution performs no Vulkan buffer allocation");
    ASSERT_EQUAL(warmFirst.pipeline_create_count, warmSecond.pipeline_create_count,
                 "warm repeated M47 execution creates no pipeline");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M47 validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "M47 produces no validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "M47 produces no validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM47ExtensionAbsentUsesDeviceResidentConventionalFallback)
{
    EnvironmentValue disabled("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX", "1");
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr)
        SKIP("Vulkan runtime unavailable");
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    constexpr std::uint32_t ffnWidth = 256u;
    std::vector<float> x;
    GroupWeights attentionWeights;
    FillGroupInputs(&x, &attentionWeights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, attentionWeights, modelWidth, headDim),
                "M47 fallback prepares grouped weights");
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data();
    prepareX.element_count = x.size();
    prepareX.tokens = tokens;
    prepareX.model_width = modelWidth;
    prepareX.generation = 570u;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "M47 fallback prepares resident X");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, 571u),
                "M47 fallback prepares Wo");
    std::vector<float> normWeight(modelWidth, 1.0f);
    prom_m46_weight_prepare_request prepareNorm{};
    prepareNorm.values = normWeight.data();
    prepareNorm.element_count = normWeight.size();
    prepareNorm.model_width = modelWidth;
    prepareNorm.generation = 572u;
    prom_m46_weight_prepare_result preparedNorm{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(runtime, &prepareNorm, &preparedNorm),
                 "M47 fallback prepares RMSNorm scale");
    std::array<std::vector<float>, PROM_M47_WEIGHT_COUNT> weights;
    weights[0].resize(static_cast<std::size_t>(modelWidth) * ffnWidth);
    weights[1].resize(weights[0].size());
    weights[2].resize(static_cast<std::size_t>(ffnWidth) * modelWidth);
    for (std::size_t index = 0u; index < weights[0].size(); ++index) {
        weights[0][index] = static_cast<float>(static_cast<int>(index % 13u) - 6) / 512.0f;
        weights[1][index] = static_cast<float>(static_cast<int>((index * 3u) % 17u) - 8) / 512.0f;
    }
    for (std::size_t index = 0u; index < weights[2].size(); ++index)
        weights[2][index] = static_cast<float>(static_cast<int>((index * 5u) % 19u) - 9) / 512.0f;
    for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind) {
        prom_m47_weight_prepare_request prepare{};
        prepare.values = weights[kind].data();
        prepare.element_count = weights[kind].size();
        prepare.kind = kind;
        prepare.model_width = modelWidth;
        prepare.ffn_width = ffnWidth;
        prepare.generation = 573u + kind;
        prom_m47_weight_prepare_result prepared{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_prepare_weight(runtime, &prepare, &prepared),
                     "M47 fallback prepares all FFN representations");
    }
    std::vector<float> output(static_cast<std::size_t>(tokens) * modelWidth);
    prom_m47_composed_request request{};
    FillM45ComposedRequest(&request.upstream.upstream, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y, PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           570u, 571u);
    request.upstream.epsilon = 1.0e-5f;
    request.upstream.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    request.upstream.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
    request.upstream.required_weight_generation = 572u;
    request.output = output.data();
    request.output_element_count = output.size();
    request.ffn_width = ffnWidth;
    request.projection_path = PROM_M47_PROJECTION_COOPERATIVE;
    request.gating_strategy = PROM_M47_GATING_FUSED_DIRECT_PACKED;
    request.residual_strategy = PROM_M47_RESIDUAL_IN_PLACE_DOWN;
    request.submit_policy = PROM_M47_SUBMIT_ONE_COMMAND_BUFFER;
    for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind)
        request.required_weight_generation[kind] = 573u + kind;
    prom_m47_composed_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &result),
                 "the complete block remains device-resident without cooperative capability");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_PROJECTION_CONVENTIONAL_FP16),
                 result.ffn_plan.projection_path,
                 "M47 records the conventional FP16 fallback");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_PROJECTION_CONVENTIONAL_FP16),
                 result.upstream.upstream.projection_plan.projection_path,
                 "M44 records the same-precision conventional fallback");
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16),
                     result.upstream.upstream.attention.plan.selected_path[head],
                     "every M43 head records conventional fallback");
    }
    for (float value : output) ASSERT_TRUE(std::isfinite(value), "fallback BlockOutput is finite");
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "fallback validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "fallback has no validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "fallback has no validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM46StagedComposedHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M46 staged proof requires the proven cooperative tuple");
    }
    constexpr std::uint32_t tokens = 1u;
    constexpr std::uint32_t modelWidth = 2048u;
    constexpr std::uint32_t headDim = 256u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "staged M46 prepares grouped weights");
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data();
    prepareX.element_count = x.size();
    prepareX.tokens = tokens;
    prepareX.model_width = modelWidth;
    prepareX.generation = 2048u;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "staged M46 prepares resident X");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, 3048u),
                "staged M46 prepares Wo");
    std::vector<float> weight(modelWidth, 1.0f);
    prom_m46_weight_prepare_request prepareWeight{};
    prepareWeight.values = weight.data();
    prepareWeight.element_count = weight.size();
    prepareWeight.model_width = modelWidth;
    prepareWeight.generation = 4048u;
    prom_m46_weight_prepare_result preparedWeight{};
    ASSERT_EQUAL(PROM_OK,
                 prom_reactor_runtime_m46_prepare_weight(runtime, &prepareWeight, &preparedWeight),
                 "staged M46 prepares scale Weight");
    std::vector<float> heads;
    ASSERT_EQUAL(1u, GroupReference(x, weights, tokens, modelWidth, headDim, &heads).all_finite,
                 "staged grouped oracle succeeds");
    std::vector<float> y;
    ASSERT_EQUAL(1u, OutputProjectionReference(heads, wo, tokens, headDim, modelWidth,
                                               PROM_M42_PRECISION_F16_ROUNDED, &y).all_finite,
                 "staged projection oracle succeeds");
    std::vector<float> z(y.size());
    for (std::size_t index = 0u; index < z.size(); ++index) z[index] = x[index] + y[index];
    std::vector<float> expected(z.size());
    std::vector<float> invRms(tokens);
    prom_m46_reference_request reference{};
    reference.z = z.data();
    reference.weight = weight.data();
    reference.n = expected.data();
    reference.inv_rms = invRms.data();
    reference.z_element_count = z.size();
    reference.weight_element_count = weight.size();
    reference.n_element_count = expected.size();
    reference.tokens = tokens;
    reference.model_width = modelWidth;
    reference.z_row_stride = modelWidth;
    reference.n_row_stride = modelWidth;
    reference.epsilon = 1.0e-5f;
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&reference),
                 "staged CPU oracle succeeds");
    std::vector<float> output(expected.size());
    prom_m46_composed_request request{};
    FillM45ComposedRequest(&request.upstream, nullptr, tokens, modelWidth, headDim,
                           PROM_M45_STRATEGY_IN_PLACE_Y,
                           PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                           2048u, 3048u);
    request.output = output.data();
    request.output_element_count = output.size();
    request.epsilon = 1.0e-5f;
    request.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
    request.submit_policy = PROM_M46_SUBMIT_TWO_BOUNDED;
    request.required_weight_generation = 4048u;
    prom_m46_composed_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &result),
                 "real 2048-wide M45 Z executes through staged RMSNorm");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M46_REDUCTION_STAGED),
                 result.rmsnorm_plan.reduction_plan, "device execution uses staged reduction");
    ASSERT_EQUAL(2u, result.rmsnorm_plan.partials_per_row,
                 "2048 width produces two exact partials");
    ASSERT_TRUE(result.final_reduction_gpu_ns > 0u,
                "the final reduction has an independent GPU interval");
    prom_m46_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m46_rmsnorm_compare(expected.data(), output.data(), tokens, modelWidth,
                                          2.0e-3f, 2.0e-2f, &result.rmsnorm_plan,
                                          nullptr, invRms.data(), &mismatch),
                 "staged device RMSNorm matches the FP32 oracle");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "staged validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count,
                 "staged M46 has zero validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count,
                 "staged M46 has zero validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM44LifecycleFaultsAndWoReplacement)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M44 lifecycle proof requires the proven cooperative tuple");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "fault proof prepares M43 weights");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, 800u),
                "fault proof prepares Wo generation 800");
    std::vector<float> output(static_cast<std::size_t>(tokens) * modelWidth);
    prom_m44_composed_request request{};
    FillM44ComposedRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                           PROM_M44_AGGREGATION_INTERLEAVE,
                           PROM_M44_PROJECTION_COOPERATIVE,
                           PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
                           PROM_M42_INPUT_HOST_X, 1u, 800u);
    for (const std::uint32_t fault : {PROM_M44_FAULT_BEFORE_AGGREGATION,
                                      PROM_M44_FAULT_DURING_INTERLEAVE,
                                      PROM_M44_FAULT_AFTER_INTERLEAVE,
                                      PROM_M44_FAULT_BEFORE_FINAL_READBACK,
                                      PROM_M44_FAULT_AFTER_PROJECTION_SUBMIT}) {
        request.fault_point = fault;
        prom_m44_composed_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m44_execute_composed(runtime, &request, &failed) != PROM_OK,
                    "a bounded M44 logical fault is surfaced");
        ASSERT_EQUAL(PROM_M44_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "known-complete M44 fault remains a logical failure");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "known fence completion keeps the composed slot recyclable");
    }
    request.aggregation_strategy = PROM_M44_AGGREGATION_DIRECT_SEGMENTED;
    request.projection_path = PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16;
    request.fault_point = PROM_M44_FAULT_MID_DIRECT_PROJECTION;
    prom_m44_composed_result failedDirect{};
    ASSERT_TRUE(prom_reactor_runtime_m44_execute_composed(runtime, &request, &failedDirect) != PROM_OK,
                "mid-direct fault is surfaced");
    ASSERT_EQUAL(1u, failedDirect.physical_slot_recyclable,
                 "known-complete direct work remains recyclable");
    request.aggregation_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
    request.projection_path = PROM_M44_PROJECTION_COOPERATIVE;
    request.fault_point = PROM_M44_FAULT_UNCERTAIN_COMPLETION;
    prom_m44_composed_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m44_execute_composed(runtime, &request, &uncertain) != PROM_OK,
                "uncertain projection completion is surfaced");
    ASSERT_EQUAL(PROM_M44_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "uncertain completion remains distinct from logical failure");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "uncertain M44 completion quarantines the complete M43+M44 slot");
    prom_m44_wo_prepare_result replaced{};
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, 801u, &replaced),
                "Wo replacement waits for and reaps the in-flight composed group");
    ASSERT_EQUAL(1u, replaced.replaced, "Wo replacement is explicit");
    request.fault_point = PROM_M44_FAULT_NONE;
    prom_m44_composed_result stale{};
    ASSERT_TRUE(prom_reactor_runtime_m44_execute_composed(runtime, &request, &stale) != PROM_OK,
                "the stale Wo generation rejects");
    ASSERT_EQUAL(PROM_M44_DETAIL_STALE_WO_GENERATION, stale.detail_code,
                 "stale Wo rejection is exact");
    request.required_wo_generation = 801u;
    prom_m44_composed_result recovered{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &recovered),
                 "the fresh Wo generation recovers after reap");
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(runtime, &diagnostics),
                 "M44 lifecycle diagnostics remain available");
    ASSERT_TRUE(diagnostics.quarantine_count >= 1u,
                "uncertain M44 work entered quarantine");
    ASSERT_TRUE(diagnostics.reap_count >= 1u,
                "Wo replacement physically reaped the owning slot");
    ASSERT_EQUAL(0u, diagnostics.quarantined_slots,
                 "recovery leaves no M44 slot quarantined");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM44ExtensionAbsentUsesConventionalFallback)
{
    EnvironmentValue disabled("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX", "1");
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "M43 weights prepare without cooperative capability");
    std::vector<float> wo;
    FillOutputProjectionWeight(&wo, headDim, modelWidth);
    ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, headDim, modelWidth, 900u),
                "Wo prepares without cooperative capability");
    std::vector<float> headReference;
    GroupReference(x, weights, tokens, modelWidth, headDim, &headReference);
    std::vector<float> expected;
    OutputProjectionReference(headReference, wo, tokens, headDim, modelWidth,
                              PROM_M42_PRECISION_F16_ROUNDED, &expected);
    std::vector<float> output(expected.size());
    prom_m44_composed_request request{};
    FillM44ComposedRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                           PROM_M44_AGGREGATION_INTERLEAVE,
                           PROM_M44_PROJECTION_COOPERATIVE,
                           PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
                           PROM_M42_INPUT_HOST_X, 1u, 900u);
    prom_m44_composed_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &result),
                 "complete attention through Y remains available without the extension");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M44_PROJECTION_CONVENTIONAL_FP16),
                 result.plan.projection_path,
                 "M44 records the same-precision conventional fallback");
    for (std::uint32_t head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16),
                     result.attention.plan.selected_path[head],
                     "M43 also records each per-head conventional fallback");
    }
    prom_m44_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m44_output_projection_compare(expected.data(), output.data(),
                                                     tokens, modelWidth, 8.0e-3f, 3.0e-2f,
                                                     result.plan.aggregation_strategy,
                                                     900u, result.plan.m43_aggregate_replay_id,
                                                     result.plan.replay_id, &mismatch),
                 "extension-disabled Y matches the rounded oracle");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM43GroupedAttentionHardwareProof)
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
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "24 independent persistent weights prepare");
    std::vector<float> expected;
    const prom_m43_reference_result reference =
        GroupReference(x, weights, tokens, modelWidth, headDim, &expected);
    ASSERT_EQUAL(1u, reference.all_finite, "grouped CPU reference succeeds");
    std::vector<float> output(expected.size(), 0.0f);
    prom_m43_attention_group_request request{};
    FillGroupExecutionRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X, 1u);
    prom_m43_attention_group_result grouped{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &grouped),
                 "host-X grouped attention completes through all eight P x V stages");
    ASSERT_EQUAL(1u, grouped.shared_x_conversion_count, "host X is packed once");
    ASSERT_EQUAL(1u, grouped.shared_x_upload_count, "host X is uploaded once");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT, grouped.shared_x_consumer_count,
                 "all eight heads consume the one shared X");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT,
                 grouped.qkv_projection_dispatch_count, "all 24 projections execute on GPU");
    ASSERT_EQUAL(1u, grouped.submit_count, "the normal grouped path submits one command buffer");
    ASSERT_EQUAL(1u, grouped.final_readback_count, "one grouped final readback verifies correctness");
    ASSERT_EQUAL(1u, grouped.no_intermediate_host_copy, "no Q/K/V/Scores/P readback occurs");
    ASSERT_TRUE(grouped.projection_total_gpu_ns > 0u && grouped.post_projection_total_gpu_ns > 0u &&
                grouped.grouped_attention_gpu_ns > 0u,
                "projection, remaining attention, and aggregate timestamps are populated");
    prom_m43_mismatch mismatch{};
    ASSERT_EQUAL(PROM_OK,
                 prom_m43_attention_compare(expected.data(), output.data(), PROM_M43_HEAD_COUNT,
                                             tokens, headDim, 3.0e-3f, 2.0e-2f,
                                             &grouped.plan, &mismatch),
                 "host-X grouped output matches the exact precision oracle");
    request.output_element_count -= 1u;
    prom_m43_attention_group_result wrongCount{};
    ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &wrongCount) != PROM_OK,
                "an inexact grouped output element count rejects");
    ASSERT_EQUAL(PROM_M43_DETAIL_INVALID_REQUEST, wrongCount.detail_code,
                 "element-count rejection is explicit");
    request.output_element_count += 1u;
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        ASSERT_TRUE(grouped.head_output_view[head].buffer != VK_NULL_HANDLE,
                    "each final head output remains device-resident");
        ASSERT_EQUAL(tokens, grouped.head_output_view[head].logical_rows,
                     "each output view retains Tokens rows");
        ASSERT_EQUAL(headDim, grouped.head_output_view[head].logical_columns,
                     "each output view retains HeadDim columns");
    }

    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data(); prepareX.tokens = tokens; prepareX.model_width = modelWidth;
    prepareX.element_count = static_cast<std::uint64_t>(tokens) * modelWidth;
    prepareX.generation = 2u;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "one resident shared-X generation prepares");
    std::fill(output.begin(), output.end(), 0.0f);
    FillGroupExecutionRequest(&request, nullptr, output.data(), tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_COMPLETE_HEADS, PROM_M42_INPUT_RESIDENT_X, 2u);
    prom_m43_attention_group_result completeHeads{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &completeHeads),
                 "resident X executes the alternate complete-head ordering");
    ASSERT_EQUAL(0u, completeHeads.shared_x_upload_count, "resident X removes the warm upload");
    ASSERT_EQUAL(PROM_OK,
                 prom_m43_attention_compare(expected.data(), output.data(), PROM_M43_HEAD_COUNT,
                                             tokens, headDim, 3.0e-3f, 2.0e-2f,
                                             &completeHeads.plan, &mismatch),
                 "alternate strategy remains correct");

    request.shared_x_generation = 1u;
    prom_m43_attention_group_result staleX{};
    ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &staleX) != PROM_OK,
                "a stale shared-X generation rejects before recording");
    ASSERT_EQUAL(PROM_M43_DETAIL_STALE_X_GENERATION, staleX.detail_code,
                 "shared-X staleness is explicit");
    request.shared_x_generation = 2u;

    std::fill(output.begin(), output.end(), 0.0f);
    request.execution_strategy = PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42;
    prom_m43_attention_group_result sequential{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &sequential),
                 "the resident-X eight-M42 baseline executes");
    ASSERT_EQUAL(PROM_M43_HEAD_COUNT, sequential.submit_count,
                 "the baseline records eight independent submits");
    ASSERT_EQUAL(PROM_OK,
                 prom_m43_attention_compare(expected.data(), output.data(), PROM_M43_HEAD_COUNT,
                                             tokens, headDim, 3.0e-3f, 2.0e-2f,
                                             &sequential.plan, &mismatch),
                 "the sequential baseline uses identical logical inputs");
    prom_m43_attention_group_result sequentialWarm{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &sequentialWarm),
                 "the second baseline invocation uses established ring capacity");
    ASSERT_EQUAL(sequential.buffer_allocation_count, sequentialWarm.buffer_allocation_count,
                 "warm grouped and baseline execution allocate no new Vulkan buffers");

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "grouped services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "grouped execution is validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count, "grouped execution is validation-error clean");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM43GroupedFaultQuarantineAndSingleWeightReplacement)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "fault proof prepares every independent weight");
    std::vector<float> output(static_cast<std::size_t>(PROM_M43_HEAD_COUNT) * tokens * headDim);
    prom_m43_attention_group_request request{};
    FillGroupExecutionRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X, 1u);
    for (const std::uint32_t fault : {PROM_M43_FAULT_SHARED_X_UPLOAD,
                                      PROM_M43_FAULT_MID_PROJECTIONS,
                                      PROM_M43_FAULT_HEAD_QK,
                                      PROM_M43_FAULT_HEAD_SOFTMAX,
                                      PROM_M43_FAULT_FINAL_READBACK}) {
        request.fault_point = fault;
        request.fault_head = fault == PROM_M43_FAULT_HEAD_QK ? 2u : 4u;
        prom_m43_attention_group_result failed{};
        ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &failed) != PROM_OK,
                    "bounded partial grouped fault is surfaced");
        ASSERT_EQUAL(PROM_M43_DETAIL_FAULT_INJECTED, failed.detail_code,
                     "completed partial work reports logical failure");
        ASSERT_EQUAL(1u, failed.physical_slot_recyclable,
                     "fence-confirmed partial grouped work remains recyclable");
    }
    request.fault_point = PROM_M43_FAULT_HEAD_PV_SUBMIT;
    request.fault_head = 5u;
    prom_m43_attention_group_result uncertain{};
    ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &uncertain) != PROM_OK,
                "one head's post-PxV completion uncertainty is surfaced");
    ASSERT_EQUAL(PROM_M43_DETAIL_COMPLETION_UNCERTAIN, uncertain.detail_code,
                 "uncertain grouped completion is distinct");
    ASSERT_EQUAL(0u, uncertain.physical_slot_recyclable,
                 "one uncertain head quarantines the complete physical group");

    constexpr std::uint32_t replacementHead = 3u;
    constexpr std::uint32_t replacementKind = PROM_M43_WEIGHT_K;
    const std::uint64_t replacementGeneration = 1000u;
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim,
                                    replacementHead, replacementKind, replacementGeneration),
                "replacing one Wk waits for and reaps the group that used it");
    request.fault_point = PROM_M43_FAULT_NONE;
    prom_m43_attention_group_result stale{};
    ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &stale) != PROM_OK,
                "the stale single-head Wk generation rejects");
    ASSERT_EQUAL(PROM_M43_DETAIL_STALE_WEIGHT_GENERATION, stale.detail_code,
                 "single-resource staleness is explicit");
    request.required_weight_generation[replacementHead][replacementKind] = replacementGeneration;
    prom_m43_attention_group_result recovered{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &recovered),
                 "fresh per-head generations recover after reap");
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
            const std::uint64_t expectedGeneration =
                head == replacementHead && kind == replacementKind
                    ? replacementGeneration
                    : GroupWeightGeneration(head, kind);
            ASSERT_EQUAL(expectedGeneration, recovered.weight_generation[head][kind],
                         "one Wk replacement leaves every unrelated generation unchanged");
        }
    }
    prom_m43_resident_x_prepare_request prepareX{};
    prepareX.x = x.data(); prepareX.tokens = tokens; prepareX.model_width = modelWidth;
    prepareX.element_count = static_cast<std::uint64_t>(tokens) * modelWidth;
    prepareX.generation = 1u;
    prom_m43_resident_x_prepare_result preparedX{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "fault proof prepares a resident shared X");
    request.host_x = nullptr;
    request.host_x_element_count = 0u;
    request.input_mode = PROM_M42_INPUT_RESIDENT_X;
    request.shared_x_generation = 1u;
    request.fault_point = PROM_M43_FAULT_HEAD_PV_SUBMIT;
    request.fault_head = 6u;
    ASSERT_TRUE(prom_reactor_runtime_m43_execute(runtime, &request, &uncertain) != PROM_OK,
                "resident-X grouped work can also become completion-uncertain");
    prepareX.generation = 2u;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                 "shared-X replacement waits for and reaps every grouped consumer");
    ASSERT_EQUAL(1u, preparedX.replaced, "shared-X replacement is explicit");
    request.shared_x_generation = 2u;
    request.fault_point = PROM_M43_FAULT_NONE;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &recovered),
                 "the fresh shared-X generation recovers after reap");
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(runtime, &diagnostics),
                 "group lifecycle diagnostics remain available");
    ASSERT_TRUE(diagnostics.quarantine_count >= 1u, "uncertain grouped work entered quarantine");
    ASSERT_TRUE(diagnostics.reap_count >= 1u, "weight replacement physically reaped the group");
    ASSERT_EQUAL(0u, diagnostics.quarantined_slots, "recovery leaves no group quarantined");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM43ExtensionAbsentUsesPerHeadConventionalFallback)
{
    EnvironmentValue disabled("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX", "1");
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    constexpr std::uint32_t tokens = 16u;
    constexpr std::uint32_t modelWidth = 64u;
    constexpr std::uint32_t headDim = 16u;
    std::vector<float> x;
    GroupWeights weights;
    FillGroupInputs(&x, &weights, tokens, modelWidth, headDim);
    ASSERT_TRUE(PrepareGroupWeights(runtime, weights, modelWidth, headDim),
                "group weights prepare without the cooperative extension");
    std::vector<float> output(static_cast<std::size_t>(PROM_M43_HEAD_COUNT) * tokens * headDim);
    prom_m43_attention_group_request request{};
    FillGroupExecutionRequest(&request, x.data(), output.data(), tokens, modelWidth, headDim,
                              PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X, 1u);
    prom_m43_attention_group_result result{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &result),
                 "all eight heads execute through conventional FP16 fallback");
    for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_PATH_CONVENTIONAL_FP16),
                     result.plan.selected_path[head], "each head records its selected fallback");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M42_SELECTOR_CAPABILITY_FALLBACK),
                     result.plan.selector_reason[head], "each head records the capability reason");
    }
    prom_reactor_runtime_destroy_impl(runtime);
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM43AttentionCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
        services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("M43 corpus requires the proven cooperative tuple");
    }
    struct Workload
    {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t modelWidth;
        std::uint32_t headDim;
        bool primary;
    };
    const std::array<Workload, 6> workloads = {{
        {"tiny", 16u, 128u, 16u, false},
        {"primary", 128u, 1024u, 128u, true},
        {"more_tokens", 256u, 1024u, 128u, false},
        {"larger_head", 128u, 1024u, 256u, false},
        {"awkward", 127u, 1001u, 127u, false},
        {"softmax_boundary", 1024u, 128u, 64u, false},
    }};
    struct BenchmarkCase
    {
        const char* path;
        const char* strategy;
        const char* inputMode;
        std::uint32_t preferredPath;
        std::uint32_t precision;
        std::uint32_t executionStrategy;
        std::uint32_t input;
    };
    const std::array<BenchmarkCase, 4> normalCases = {{
        {"cooperative", "projection_grouped", "host_x", PROM_M42_PATH_COOPERATIVE,
         PROM_M42_PRECISION_F16_ROUNDED, PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_HOST_X},
        {"cooperative", "projection_grouped", "resident_x", PROM_M42_PATH_COOPERATIVE,
         PROM_M42_PRECISION_F16_ROUNDED, PROM_M43_STRATEGY_PROJECTION_GROUPED, PROM_M42_INPUT_RESIDENT_X},
        {"cooperative", "complete_heads", "resident_x", PROM_M42_PATH_COOPERATIVE,
         PROM_M42_PRECISION_F16_ROUNDED, PROM_M43_STRATEGY_COMPLETE_HEADS, PROM_M42_INPUT_RESIDENT_X},
        {"cooperative", "eight_sequential_m42", "resident_x", PROM_M42_PATH_COOPERATIVE,
         PROM_M42_PRECISION_F16_ROUNDED, PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42,
         PROM_M42_INPUT_RESIDENT_X},
    }};
    const std::array<BenchmarkCase, 2> primaryBaselines = {{
        {"conventional_fp16", "eight_sequential_m42", "resident_x", PROM_M42_PATH_CONVENTIONAL_FP16,
         PROM_M42_PRECISION_F16_ROUNDED, PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42,
         PROM_M42_INPUT_RESIDENT_X},
        {"a2x4_fp32", "eight_sequential_m42", "resident_x", PROM_M42_PATH_A2X4,
         PROM_M42_PRECISION_FP32, PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42,
         PROM_M42_INPUT_RESIDENT_X},
    }};
    std::vector<GroupBenchmarkRecord> records;
    std::uint64_t warm10EndToEnd = 0u;
    std::uint64_t warm10Gpu = 0u;
    std::uint64_t warm100EndToEnd = 0u;
    std::uint64_t warm100Gpu = 0u;
    for (std::size_t workloadIndex = 0u; workloadIndex < workloads.size(); ++workloadIndex) {
        const Workload& workload = workloads[workloadIndex];
        const std::uint64_t generationBase = 10000u + workloadIndex * 1000u;
        const std::uint64_t xGeneration = 500u + workloadIndex;
        std::uint64_t generations[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT]{};
        std::uint64_t weightPreparationNs = 0u;
        std::uint64_t weightPreparationGpuNs = 0u;
        std::vector<float> x;
        GroupWeights weights;
        FillGroupInputs(&x, &weights, workload.tokens, workload.modelWidth, workload.headDim);
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                prom_m43_weight_prepare_request prepare{};
                prepare.values = weights[head][kind].data();
                prepare.element_count = static_cast<std::uint64_t>(workload.modelWidth) * workload.headDim;
                prepare.head_index = head;
                prepare.weight_kind = kind;
                prepare.model_width = workload.modelWidth;
                prepare.head_dim = workload.headDim;
                prepare.generation = generationBase + head * PROM_M43_WEIGHT_KIND_COUNT + kind;
                generations[head][kind] = prepare.generation;
                prom_m43_weight_prepare_result prepared{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_weight(runtime, &prepare, &prepared),
                             "benchmark weight preparation succeeds");
                weightPreparationNs += prepared.upload_and_pack_ns;
                weightPreparationGpuNs += prepared.gpu_upload_and_pack_ns;
            }
        }
        prom_m43_resident_x_prepare_request prepareX{};
        prepareX.x = x.data();
        prepareX.element_count = static_cast<std::uint64_t>(workload.tokens) * workload.modelWidth;
        prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth;
        prepareX.generation = xGeneration;
        prom_m43_resident_x_prepare_result preparedX{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                     "benchmark resident X preparation succeeds");
        std::vector<float> expectedF16;
        const prom_m43_reference_result f16Reference =
            GroupReference(x, weights, workload.tokens, workload.modelWidth, workload.headDim, &expectedF16);
        ASSERT_EQUAL(1u, f16Reference.all_finite, "benchmark reduced-precision oracle succeeds");
        std::vector<float> expectedF32;
        if (workload.primary) {
            const prom_m43_reference_result f32Reference =
                GroupReference(x, weights, workload.tokens, workload.modelWidth, workload.headDim,
                               &expectedF32, PROM_M42_PRECISION_FP32);
            ASSERT_EQUAL(1u, f32Reference.all_finite, "primary FP32 oracle succeeds");
        }
        std::vector<BenchmarkCase> cases(normalCases.begin(), normalCases.end());
        if (workload.primary) cases.insert(cases.end(), primaryBaselines.begin(), primaryBaselines.end());
        for (const BenchmarkCase& benchmarkCase : cases) {
            const std::vector<float>& expected = benchmarkCase.precision == PROM_M42_PRECISION_FP32
                                                     ? expectedF32
                                                     : expectedF16;
            std::vector<float> output(expected.size(), 0.0f);
            prom_m43_attention_group_request request{};
            FillGroupExecutionRequest(&request,
                                      benchmarkCase.input == PROM_M42_INPUT_HOST_X ? x.data() : nullptr,
                                      output.data(), workload.tokens, workload.modelWidth, workload.headDim,
                                      benchmarkCase.executionStrategy, benchmarkCase.input, xGeneration);
            request.precision_policy = benchmarkCase.precision;
            for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
                request.preferred_path[head] = benchmarkCase.preferredPath;
                for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                    request.required_weight_generation[head][kind] = generations[head][kind];
                }
            }
            for (std::uint32_t warmupIndex = 0u; warmupIndex < 5u; ++warmupIndex) {
                prom_m43_attention_group_result warmup{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &warmup),
                             "M43 benchmark warmup succeeds");
            }
            std::vector<prom_m43_attention_group_result> measurements;
            for (std::uint32_t repetition = 0u; repetition < 5u; ++repetition) {
                prom_m43_attention_group_result measured{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &request, &measured),
                             "M43 benchmark measurement succeeds");
                measurements.push_back(measured);
            }
            prom_m43_mismatch mismatch{};
            const bool correct =
                prom_m43_attention_compare(expected.data(), output.data(), PROM_M43_HEAD_COUNT,
                                            workload.tokens, workload.headDim, 3.0e-3f, 2.0e-2f,
                                            &measurements.back().plan, &mismatch) == PROM_OK;
            ASSERT_TRUE(correct, "M43 benchmark output matches its exact precision oracle");
            GroupBenchmarkRecord record{};
            record.workload = workload.name;
            record.path = benchmarkCase.path;
            record.strategy = benchmarkCase.strategy;
            record.inputMode = benchmarkCase.inputMode;
            record.tokens = workload.tokens;
            record.modelWidth = workload.modelWidth;
            record.headDim = workload.headDim;
            record.replayId = measurements.back().plan.aggregate_replay_id;
            record.weightPreparationNs = weightPreparationNs;
            record.weightPreparationGpuNs = weightPreparationGpuNs;
            record.xPreparationNs = preparedX.upload_and_pack_ns;
            record.xPreparationGpuNs = preparedX.gpu_upload_and_pack_ns;
            record.sharedXValidationNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::shared_x_validation_ns);
            record.sharedXUploadGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::shared_x_upload_gpu_ns);
            record.sharedXPackGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::shared_x_pack_gpu_ns);
            record.projectionGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::projection_total_gpu_ns);
            record.postProjectionGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::post_projection_total_gpu_ns);
            record.qPackGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::q_pack_total_gpu_ns);
            record.kLayoutGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::k_layout_total_gpu_ns);
            record.vPackGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::v_pack_total_gpu_ns);
            record.qkGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::qk_total_gpu_ns);
            record.scaleGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::scale_total_gpu_ns);
            record.softmaxGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::softmax_total_gpu_ns);
            record.pPackGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::p_pack_total_gpu_ns);
            record.pvGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::pv_total_gpu_ns);
            record.totalGpuNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::grouped_attention_gpu_ns);
            record.cpuRecordingNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::cpu_recording_ns);
            record.cpuSubmissionNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::cpu_submission_ns);
            record.finalReadbackNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::final_readback_ns);
            record.endToEndNs = MedianGroupMetric(measurements,
                &prom_m43_attention_group_result::end_to_end_ns);
            record.retainedBytes = measurements.back().retained_bytes;
            record.exactBytes = measurements.back().exact_request_bytes;
            record.submitCount = measurements.back().submit_count;
            record.dispatchCount = measurements.back().plan.dispatch_count;
            record.barrierCalls = measurements.back().plan.barrier_call_count;
            record.barrierBuffers = measurements.back().plan.barrier_buffer_count;
            record.correct = correct;
            for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
                record.qProjection[head] = MedianGroupHeadMetric(measurements, head, PROM_M43_WEIGHT_Q);
                record.kProjection[head] = MedianGroupHeadMetric(measurements, head, PROM_M43_WEIGHT_K);
                record.vProjection[head] = MedianGroupHeadMetric(measurements, head, PROM_M43_WEIGHT_V);
                record.headReplay[head] = measurements.back().plan.head_replay_id[head];
            }
            records.push_back(record);
        }
        if (workload.primary) {
            std::vector<float> output(expectedF16.size(), 0.0f);
            prom_m43_attention_group_request repeatRequest{};
            FillGroupExecutionRequest(&repeatRequest, nullptr, output.data(), workload.tokens,
                                      workload.modelWidth, workload.headDim,
                                      PROM_M43_STRATEGY_PROJECTION_GROUPED,
                                      PROM_M42_INPUT_RESIDENT_X, xGeneration);
            for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
                for (std::uint32_t kind = 0u; kind < PROM_M43_WEIGHT_KIND_COUNT; ++kind) {
                    repeatRequest.required_weight_generation[head][kind] = generations[head][kind];
                }
            }
            std::vector<prom_m43_attention_group_result> repeated10;
            std::vector<prom_m43_attention_group_result> repeated100;
            for (std::uint32_t repetition = 0u; repetition < 10u; ++repetition) {
                prom_m43_attention_group_result result{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &repeatRequest, &result),
                             "primary 10-group repeat succeeds");
                repeated10.push_back(result);
            }
            for (std::uint32_t repetition = 0u; repetition < 100u; ++repetition) {
                prom_m43_attention_group_result result{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &repeatRequest, &result),
                             "primary 100-group repeat succeeds");
                repeated100.push_back(result);
            }
            warm10EndToEnd = MedianGroupMetric(repeated10, &prom_m43_attention_group_result::end_to_end_ns);
            warm10Gpu = MedianGroupMetric(repeated10, &prom_m43_attention_group_result::grouped_attention_gpu_ns);
            warm100EndToEnd = MedianGroupMetric(repeated100, &prom_m43_attention_group_result::end_to_end_ns);
            warm100Gpu = MedianGroupMetric(repeated100, &prom_m43_attention_group_result::grouped_attention_gpu_ns);
        }
    }
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M43 benchmark services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count, "M43 corpus is validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count, "M43 corpus is validation-error clean");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m43.bounded-grouped-attention.v1\",\n"
         << "  \"head_count\": " << PROM_M43_HEAD_COUNT << ",\n"
         << "  \"output_layout\": \"head_major\",\n"
         << "  \"validation\": {\"warnings\": " << services.validation_warning_count
         << ", \"errors\": " << services.validation_error_count << "},\n"
         << "  \"primary_repeats\": {\"warm_10_end_to_end_ns\": " << warm10EndToEnd
         << ", \"warm_10_gpu_ns\": " << warm10Gpu
         << ", \"warm_100_end_to_end_ns\": " << warm100EndToEnd
         << ", \"warm_100_gpu_ns\": " << warm100Gpu << "},\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const GroupBenchmarkRecord& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"path\":\"" << record.path
             << "\",\"strategy\":\"" << record.strategy
             << "\",\"input_mode\":\"" << record.inputMode
             << "\",\"tokens\":" << record.tokens
             << ",\"model_width\":" << record.modelWidth
             << ",\"head_dim\":" << record.headDim
             << ",\"replay_id\":" << record.replayId
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"weight_preparation_ns\":" << record.weightPreparationNs
             << ",\"weight_preparation_gpu_ns\":" << record.weightPreparationGpuNs
             << ",\"x_preparation_ns\":" << record.xPreparationNs
             << ",\"x_preparation_gpu_ns\":" << record.xPreparationGpuNs
             << ",\"shared_x_validation_ns\":" << record.sharedXValidationNs
             << ",\"shared_x_upload_gpu_ns\":" << record.sharedXUploadGpuNs
             << ",\"shared_x_pack_gpu_ns\":" << record.sharedXPackGpuNs
             << ",\"projection_gpu_ns\":" << record.projectionGpuNs
             << ",\"post_projection_gpu_ns\":" << record.postProjectionGpuNs
             << ",\"q_pack_gpu_ns\":" << record.qPackGpuNs
             << ",\"k_layout_gpu_ns\":" << record.kLayoutGpuNs
             << ",\"v_pack_gpu_ns\":" << record.vPackGpuNs
             << ",\"qk_gpu_ns\":" << record.qkGpuNs
             << ",\"scale_gpu_ns\":" << record.scaleGpuNs
             << ",\"softmax_gpu_ns\":" << record.softmaxGpuNs
             << ",\"p_pack_gpu_ns\":" << record.pPackGpuNs
             << ",\"pv_gpu_ns\":" << record.pvGpuNs
             << ",\"total_gpu_ns\":" << record.totalGpuNs
             << ",\"cpu_recording_ns\":" << record.cpuRecordingNs
             << ",\"cpu_submission_ns\":" << record.cpuSubmissionNs
             << ",\"final_readback_ns\":" << record.finalReadbackNs
             << ",\"end_to_end_ns\":" << record.endToEndNs
             << ",\"retained_bytes\":" << record.retainedBytes
             << ",\"exact_request_bytes\":" << record.exactBytes
             << ",\"submit_count\":" << record.submitCount
             << ",\"dispatch_count\":" << record.dispatchCount
             << ",\"barrier_calls\":" << record.barrierCalls
             << ",\"barrier_buffers\":" << record.barrierBuffers
             << ",\"q_projection_per_head_ns\":[";
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            if (head != 0u) json << ',';
            json << record.qProjection[head];
        }
        json << "],\"k_projection_per_head_ns\":[";
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            if (head != 0u) json << ',';
            json << record.kProjection[head];
        }
        json << "],\"v_projection_per_head_ns\":[";
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            if (head != 0u) json << ',';
            json << record.vProjection[head];
        }
        json << "],\"head_replay_ids\":[";
        for (std::uint32_t head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
            if (head != 0u) json << ',';
            json << record.headReplay[head];
        }
        json << "]}";
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m43_bounded_grouped_attention.json", json.str()),
                "M43 benchmark artifact is written");
    prom_reactor_runtime_destroy_impl(runtime);
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM44AttentionOutputProjectionCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    struct Workload
    {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t headDim;
        std::uint32_t modelWidth;
        bool primary;
    };
    const std::array<Workload, 6> workloads = {{
        {"tiny", 16u, 16u, 128u, false},
        {"primary", 128u, 128u, 1024u, true},
        {"more_tokens", 256u, 128u, 1024u, false},
        {"larger_head", 128u, 256u, 1024u, false},
        {"awkward", 127u, 127u, 1001u, false},
        {"softmax_boundary", 1024u, 64u, 128u, false},
    }};
    struct Strategy
    {
        const char* name;
        const char* path;
        const char* submit;
        std::uint32_t aggregation;
        std::uint32_t projection;
        std::uint32_t submitPlan;
        std::uint32_t precision;
    };
    const std::array<Strategy, 5> strategies = {{
        {"interleave", "cooperative", "one", PROM_M44_AGGREGATION_INTERLEAVE,
         PROM_M44_PROJECTION_COOPERATIVE, PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
         PROM_M42_PRECISION_F16_ROUNDED},
        {"interleave", "a2x4_fp32", "one", PROM_M44_AGGREGATION_INTERLEAVE,
         PROM_M44_PROJECTION_A2X4_FP32, PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
         PROM_M42_PRECISION_FP32},
        {"interleave", "conventional_fp16", "one", PROM_M44_AGGREGATION_INTERLEAVE,
         PROM_M44_PROJECTION_CONVENTIONAL_FP16, PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
         PROM_M42_PRECISION_F16_ROUNDED},
        {"direct_segmented", "direct_fp16", "one", PROM_M44_AGGREGATION_DIRECT_SEGMENTED,
         PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16, PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
         PROM_M42_PRECISION_F16_ROUNDED},
        {"interleave", "cooperative", "two", PROM_M44_AGGREGATION_INTERLEAVE,
         PROM_M44_PROJECTION_COOPERATIVE, PROM_M44_SUBMIT_TWO_BOUNDED,
         PROM_M42_PRECISION_F16_ROUNDED},
    }};
    std::vector<M44BenchmarkRecord> records;
    std::uint64_t warm10Gpu = 0u;
    std::uint64_t warm10EndToEnd = 0u;
    std::uint64_t warm100Gpu = 0u;
    std::uint64_t warm100EndToEnd = 0u;
    std::uint32_t totalWarnings = 0u;
    std::uint32_t totalErrors = 0u;
    for (const Workload& workload : workloads) {
        void* runtime = nullptr;
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime),
                     "M44 corpus runtime creates");
        prom_vk_runtime_services services{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                     "M44 corpus services are available");
        if (services.cooperative_matrix_feature_enabled == 0u) {
            prom_reactor_runtime_destroy_impl(runtime);
            SKIP("M44 corpus requires the proven cooperative tuple");
        }
        std::vector<float> x;
        GroupWeights weights;
        FillGroupInputs(&x, &weights, workload.tokens, workload.modelWidth, workload.headDim);
        ASSERT_TRUE(PrepareGroupWeights(runtime, weights, workload.modelWidth, workload.headDim),
                    "M44 corpus prepares 24 M43 weights");
        prom_m43_resident_x_prepare_request prepareX{};
        prepareX.x = x.data();
        prepareX.element_count = x.size();
        prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth;
        prepareX.generation = 77u;
        prom_m43_resident_x_prepare_result preparedX{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                     "M44 corpus prepares resident shared X");
        std::vector<float> wo;
        FillOutputProjectionWeight(&wo, workload.headDim, workload.modelWidth);
        prom_m44_wo_prepare_result preparedWo{};
        ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, workload.headDim,
                                                  workload.modelWidth, 500u, &preparedWo),
                    "M44 corpus prepares persistent Wo");
        std::vector<float> headExpected;
        const prom_m43_reference_result headReference =
            GroupReference(x, weights, workload.tokens, workload.modelWidth,
                           workload.headDim, &headExpected);
        ASSERT_EQUAL(1u, headReference.all_finite, "M43 corpus source oracle succeeds");
        std::vector<float> roundedExpected;
        std::vector<float> exactExpected;
        ASSERT_EQUAL(1u,
                     OutputProjectionReference(headExpected, wo, workload.tokens, workload.headDim,
                                               workload.modelWidth, PROM_M42_PRECISION_F16_ROUNDED,
                                               &roundedExpected).all_finite,
                     "rounded M44 corpus oracle succeeds");
        ASSERT_EQUAL(1u,
                     OutputProjectionReference(headExpected, wo, workload.tokens, workload.headDim,
                                               workload.modelWidth, PROM_M42_PRECISION_FP32,
                                               &exactExpected).all_finite,
                     "FP32 M44 corpus oracle succeeds");
        for (const Strategy& strategy : strategies) {
            std::vector<float> primedOutput(
                static_cast<std::size_t>(workload.tokens) * workload.modelWidth);
            prom_m44_composed_request primeRequest{};
            FillM44ComposedRequest(&primeRequest, nullptr, primedOutput.data(), workload.tokens,
                                   workload.modelWidth, workload.headDim,
                                   strategy.aggregation, strategy.projection, strategy.submitPlan,
                                   PROM_M42_INPUT_RESIDENT_X, 77u, 500u);
            for (std::uint32_t slot = 0u; slot < 2u; ++slot) {
                prom_m44_composed_result primed{};
                ASSERT_EQUAL(PROM_OK,
                             prom_reactor_runtime_m44_execute_composed(runtime, &primeRequest, &primed),
                             "M44 corpus primes both grow-only slots for every plan");
            }
        }
        for (const Strategy& strategy : strategies) {
            std::vector<float> output(static_cast<std::size_t>(workload.tokens) * workload.modelWidth);
            prom_m44_composed_request request{};
            FillM44ComposedRequest(&request, nullptr, output.data(), workload.tokens,
                                   workload.modelWidth, workload.headDim,
                                   strategy.aggregation, strategy.projection, strategy.submitPlan,
                                   PROM_M42_INPUT_RESIDENT_X, 77u, 500u);
            // A conservative 32 executions keeps fresh-runtime GPU power-state
            // ramp outside the sample window for every corpus shape. With only
            // two, the first strategy could carry a repeatable ~10x penalty.
            for (std::uint32_t warmup = 0u; warmup < 32u; ++warmup) {
                prom_m44_composed_result warm{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &warm),
                             "M44 corpus warmup succeeds");
            }
            std::vector<prom_m44_composed_result> measurements;
            for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
                prom_m44_composed_result measured{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &measured),
                             "M44 corpus measurement succeeds");
                measurements.push_back(measured);
            }
            const std::vector<float>& expected =
                strategy.precision == PROM_M42_PRECISION_FP32 ? exactExpected : roundedExpected;
            prom_m44_mismatch mismatch{};
            const bool correct =
                prom_m44_output_projection_compare(expected.data(), output.data(),
                                                    workload.tokens, workload.modelWidth,
                                                    1.0e-2f, 4.0e-2f,
                                                    strategy.aggregation, 500u,
                                                    measurements.back().plan.m43_aggregate_replay_id,
                                                    measurements.back().plan.replay_id,
                                                    &mismatch) == PROM_OK;
            ASSERT_TRUE(correct, "M44 corpus Y matches its precision-specific oracle");
            const prom_m44_composed_result& last = measurements.back();
            M44BenchmarkRecord record{};
            record.workload = workload.name;
            record.strategy = strategy.name;
            record.path = strategy.path;
            record.submitPlan = strategy.submit;
            record.tokens = workload.tokens;
            record.headDim = workload.headDim;
            record.modelWidth = workload.modelWidth;
            record.replayId = last.plan.replay_id;
            record.m43ReplayId = last.plan.m43_aggregate_replay_id;
            record.woPreparationNs = preparedWo.upload_and_pack_ns + preparedWo.validation_hash_ns;
            record.woPreparationGpuNs = preparedWo.gpu_upload_and_pack_ns;
            {
                std::vector<std::uint64_t> values;
                for (const prom_m44_composed_result& value : measurements)
                    values.push_back(value.attention.grouped_attention_gpu_ns);
                std::sort(values.begin(), values.end());
                record.m43GpuNs = values[values.size() / 2u];
            }
            record.aggregationGpuNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::aggregation_gpu_ns);
            record.projectionGpuNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::projection_gpu_ns);
            record.accumulationGpuNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::accumulation_gpu_ns);
            record.m44GpuNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::m44_gpu_ns);
            record.totalGpuNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::total_m43_m44_gpu_ns);
            record.cpuRecordingNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::cpu_recording_ns);
            record.cpuSubmissionNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::cpu_submission_ns);
            record.finalReadbackNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::final_readback_ns);
            record.endToEndNs = MedianM44Metric(measurements,
                &prom_m44_composed_result::end_to_end_ns);
            record.temporaryBytes = last.plan.memory.contiguous_f32_bytes +
                                    last.plan.memory.contiguous_packed_bytes +
                                    last.plan.memory.partial_output_bytes +
                                    last.plan.memory.accumulation_bytes;
            record.retainedBytes = last.retained_bytes;
            record.exactBytes = last.exact_request_bytes;
            record.sourceHeadBytes = last.plan.memory.source_head_bytes;
            record.contiguousF32Bytes = last.plan.memory.contiguous_f32_bytes;
            record.contiguousPackedBytes = last.plan.memory.contiguous_packed_bytes;
            record.partialOutputBytes = last.plan.memory.partial_output_bytes;
            record.accumulationBytes = last.plan.memory.accumulation_bytes;
            record.woUploadBytes = last.plan.memory.wo_upload_bytes;
            record.woF32Bytes = last.plan.memory.wo_f32_bytes;
            record.woPackedBytes = last.plan.memory.wo_packed_bytes;
            record.finalYBytes = last.plan.memory.final_y_bytes;
            record.finalReadbackBytes = last.plan.memory.final_readback_bytes;
            record.reusableDescriptorSets = last.plan.memory.reusable_descriptor_set_count;
            record.descriptorBindings = last.plan.memory.descriptor_binding_count;
            record.submitCount = last.submit_count;
            record.dispatchCount = last.attention.plan.dispatch_count + last.plan.dispatch_count;
            record.barrierCalls = last.attention.plan.barrier_call_count + last.plan.barrier_call_count;
            record.barrierBuffers = last.attention.plan.barrier_buffer_count + last.plan.barrier_buffer_count;
            record.copyRegions = last.plan.copy_region_count;
            record.intermediateHostCopies = 0u;
            record.correct = correct;
            records.push_back(record);
            if (workload.primary && strategy.aggregation == PROM_M44_AGGREGATION_INTERLEAVE &&
                strategy.projection == PROM_M44_PROJECTION_COOPERATIVE &&
                strategy.submitPlan == PROM_M44_SUBMIT_ONE_COMMAND_BUFFER) {
                std::vector<prom_m44_composed_result> repeated10;
                std::vector<prom_m44_composed_result> repeated100;
                for (std::uint32_t repeat = 0u; repeat < 10u; ++repeat) {
                    prom_m44_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &value),
                                 "M44 primary 10-repeat succeeds");
                    repeated10.push_back(value);
                }
                for (std::uint32_t repeat = 0u; repeat < 100u; ++repeat) {
                    prom_m44_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &request, &value),
                                 "M44 primary 100-repeat succeeds");
                    repeated100.push_back(value);
                }
                warm10Gpu = MedianM44Metric(repeated10, &prom_m44_composed_result::total_m43_m44_gpu_ns);
                warm10EndToEnd = MedianM44Metric(repeated10, &prom_m44_composed_result::end_to_end_ns);
                warm100Gpu = MedianM44Metric(repeated100, &prom_m44_composed_result::total_m43_m44_gpu_ns);
                warm100EndToEnd = MedianM44Metric(repeated100, &prom_m44_composed_result::end_to_end_ns);
            }
        }

        std::vector<float> actualHeads(headExpected.size());
        prom_m43_attention_group_request groupedRequest{};
        FillGroupExecutionRequest(&groupedRequest, nullptr, actualHeads.data(), workload.tokens,
                                  workload.modelWidth, workload.headDim,
                                  PROM_M43_STRATEGY_PROJECTION_GROUPED,
                                  PROM_M42_INPUT_RESIDENT_X, 77u);
        std::vector<prom_m43_attention_group_result> m43Measurements;
        std::vector<prom_m44_host_bounce_result> hostMeasurements;
        std::vector<std::uint64_t> hostProductGpu;
        std::vector<std::uint64_t> hostProductEndToEnd;
        for (std::uint32_t iteration = 0u; iteration < 3u; ++iteration) {
            prom_m43_attention_group_result grouped{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_execute(runtime, &groupedRequest, &grouped),
                         "host-bounce baseline reads the M43 heads");
            prom_m44_host_bounce_request hostRequest{};
            hostRequest.head_major = actualHeads.data();
            hostRequest.head_major_element_count = actualHeads.size();
            std::vector<float> output(roundedExpected.size());
            hostRequest.output = output.data();
            hostRequest.output_element_count = output.size();
            hostRequest.head_count = PROM_M44_HEAD_COUNT;
            hostRequest.tokens = workload.tokens;
            hostRequest.head_dim = workload.headDim;
            hostRequest.model_width = workload.modelWidth;
            hostRequest.precision_policy = PROM_M42_PRECISION_F16_ROUNDED;
            hostRequest.projection_path = PROM_M44_PROJECTION_COOPERATIVE;
            hostRequest.required_wo_generation = 500u;
            hostRequest.m43_aggregate_replay_id = grouped.plan.aggregate_replay_id;
            prom_m44_host_bounce_result host{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_host_bounce(runtime, &hostRequest, &host),
                         "host-bounce output projection executes");
            prom_m44_mismatch mismatch{};
            ASSERT_EQUAL(PROM_OK,
                         prom_m44_output_projection_compare(roundedExpected.data(), output.data(),
                                                             workload.tokens, workload.modelWidth,
                                                             1.0e-2f, 4.0e-2f,
                                                             PROM_M44_AGGREGATION_INTERLEAVE,
                                                             500u, grouped.plan.aggregate_replay_id,
                                                             host.replay_id, &mismatch),
                         "host-bounce Y remains correct");
            m43Measurements.push_back(grouped);
            hostMeasurements.push_back(host);
            hostProductGpu.push_back(grouped.grouped_attention_gpu_ns + host.upload_gpu_ns +
                                     host.projection_gpu_ns);
            hostProductEndToEnd.push_back(grouped.end_to_end_ns + host.end_to_end_ns);
        }
        std::sort(hostProductGpu.begin(), hostProductGpu.end());
        std::sort(hostProductEndToEnd.begin(), hostProductEndToEnd.end());
        M44BenchmarkRecord hostRecord{};
        hostRecord.workload = workload.name;
        hostRecord.strategy = "host_bounce";
        hostRecord.path = "cooperative";
        hostRecord.submitPlan = "two_cpu_separated";
        hostRecord.tokens = workload.tokens;
        hostRecord.headDim = workload.headDim;
        hostRecord.modelWidth = workload.modelWidth;
        hostRecord.replayId = hostMeasurements.back().replay_id;
        hostRecord.m43ReplayId = m43Measurements.back().plan.aggregate_replay_id;
        hostRecord.woPreparationNs = preparedWo.upload_and_pack_ns + preparedWo.validation_hash_ns;
        hostRecord.woPreparationGpuNs = preparedWo.gpu_upload_and_pack_ns;
        hostRecord.m43GpuNs = MedianGroupMetric(m43Measurements,
            &prom_m43_attention_group_result::grouped_attention_gpu_ns);
        {
            std::vector<std::uint64_t> values;
            for (const prom_m44_host_bounce_result& value : hostMeasurements)
                values.push_back(value.upload_gpu_ns);
            std::sort(values.begin(), values.end());
            hostRecord.aggregationGpuNs = values[values.size() / 2u];
            values.clear();
            for (const prom_m44_host_bounce_result& value : hostMeasurements)
                values.push_back(value.projection_gpu_ns);
            std::sort(values.begin(), values.end());
            hostRecord.projectionGpuNs = values[values.size() / 2u];
            values.clear();
            for (const prom_m44_host_bounce_result& value : hostMeasurements)
                values.push_back(value.final_readback_ns);
            std::sort(values.begin(), values.end());
            hostRecord.finalReadbackNs = values[values.size() / 2u] +
                                         MedianGroupMetric(m43Measurements,
                                           &prom_m43_attention_group_result::final_readback_ns);
            values.clear();
            for (const prom_m44_host_bounce_result& value : hostMeasurements)
                values.push_back(value.cpu_concatenate_ns);
            std::sort(values.begin(), values.end());
            hostRecord.cpuConcatenateNs = values[values.size() / 2u];
            values.clear();
            for (const prom_m44_host_bounce_result& value : hostMeasurements)
                values.push_back(value.cpu_pack_ns);
            std::sort(values.begin(), values.end());
            hostRecord.cpuPackNs = values[values.size() / 2u];
        }
        hostRecord.m44GpuNs = hostRecord.aggregationGpuNs + hostRecord.projectionGpuNs;
        hostRecord.totalGpuNs = hostProductGpu[hostProductGpu.size() / 2u];
        hostRecord.endToEndNs = hostProductEndToEnd[hostProductEndToEnd.size() / 2u];
        hostRecord.retainedBytes = m43Measurements.back().retained_bytes +
                                   hostMeasurements.back().retained_bytes;
        hostRecord.exactBytes = m43Measurements.back().exact_request_bytes;
        hostRecord.submitCount = 2u;
        hostRecord.intermediateHostCopies = 1u;
        hostRecord.correct = true;
        records.push_back(hostRecord);

        M44BenchmarkRecord noProjection{};
        noProjection.workload = workload.name;
        noProjection.strategy = "no_output_projection";
        noProjection.path = "m43_only";
        noProjection.submitPlan = "one";
        noProjection.tokens = workload.tokens;
        noProjection.headDim = workload.headDim;
        noProjection.modelWidth = workload.modelWidth;
        noProjection.replayId = m43Measurements.back().plan.aggregate_replay_id;
        noProjection.m43ReplayId = noProjection.replayId;
        noProjection.m43GpuNs = MedianGroupMetric(m43Measurements,
            &prom_m43_attention_group_result::grouped_attention_gpu_ns);
        noProjection.totalGpuNs = noProjection.m43GpuNs;
        noProjection.endToEndNs = MedianGroupMetric(m43Measurements,
            &prom_m43_attention_group_result::end_to_end_ns);
        noProjection.finalReadbackNs = MedianGroupMetric(m43Measurements,
            &prom_m43_attention_group_result::final_readback_ns);
        noProjection.retainedBytes = m43Measurements.back().retained_bytes;
        noProjection.exactBytes = m43Measurements.back().exact_request_bytes;
        noProjection.submitCount = 1u;
        noProjection.dispatchCount = m43Measurements.back().plan.dispatch_count;
        noProjection.barrierCalls = m43Measurements.back().plan.barrier_call_count;
        noProjection.barrierBuffers = m43Measurements.back().plan.barrier_buffer_count;
        noProjection.correct = true;
        records.push_back(noProjection);
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                     "M44 workload services remain available");
        totalWarnings += services.validation_warning_count;
        totalErrors += services.validation_error_count;
        prom_reactor_runtime_destroy_impl(runtime);
    }
    ASSERT_EQUAL(0u, totalWarnings, "M44 corpus is validation-warning clean");
    ASSERT_EQUAL(0u, totalErrors, "M44 corpus is validation-error clean");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m44.multihead-output-projection.v1\",\n"
         << "  \"head_count\": " << PROM_M44_HEAD_COUNT << ",\n"
         << "  \"source_layout\": \"head_major_views\",\n"
         << "  \"logical_concatenation\": \"C[token,head*HeadDim+column]\",\n"
         << "  \"output_layout\": \"row_major_tokens_model_width\",\n"
         << "  \"precision\": {\"cooperative_input\": \"f16_rne\", "
            "\"cooperative_weight\": \"f16_rne\", \"accumulation\": \"fp32\", "
            "\"output\": \"fp32\"},\n"
         << "  \"shader_artifacts\": {\n"
         << "    \"dxc\": \"1.9.0.5347-fe2615732\",\n"
         << "    \"interleave\": {\"source_sha256\": "
            "\"67de5fac9fc51ee483b07e2c38d48d077f89e5942b8d08ca3307b3896695ad43\", "
            "\"hlsl_sha256\": \"4278f12da1b5c7368415660058dea1cbbaa6c1f75b0f651c49cdb2d2186c6033\", "
            "\"spv_sha256\": \"ef5bd1d4aac8cce92c0548f92e63f9c8be508217bc62e1c4d14c40d07ace041e\"},\n"
         << "    \"direct_segmented\": {\"source_sha256\": "
            "\"08a4106010c596ac9ac45743087e683804fb8891152f09a0080e19b009244b79\", "
            "\"hlsl_sha256\": \"a60501488878d38deb2f7fe8a03531b3399fcf818b3172b6a8fbdab18786677f\", "
            "\"spv_sha256\": \"1f2c7914051d28c23605731f95125d4d048327c1c8e0af769f38b16693032365\"}\n"
         << "  },\n"
         << "  \"warmup_operations_per_plan\": 32,\n"
         << "  \"measurement_operations_per_plan\": 5,\n"
         << "  \"capacity_prime_operations_per_plan\": 2,\n"
         << "  \"validation\": {\"warnings\": " << totalWarnings
         << ", \"errors\": " << totalErrors << "},\n"
         << "  \"primary_repeats\": {\"warm_10_gpu_ns\": " << warm10Gpu
         << ", \"warm_10_end_to_end_ns\": " << warm10EndToEnd
         << ", \"warm_100_gpu_ns\": " << warm100Gpu
         << ", \"warm_100_end_to_end_ns\": " << warm100EndToEnd << "},\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const M44BenchmarkRecord& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"strategy\":\"" << record.strategy
             << "\",\"path\":\"" << record.path
             << "\",\"submit_plan\":\"" << record.submitPlan
             << "\",\"tokens\":" << record.tokens
             << ",\"head_dim\":" << record.headDim
             << ",\"model_width\":" << record.modelWidth
             << ",\"replay_id\":" << record.replayId
             << ",\"m43_replay_id\":" << record.m43ReplayId
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"wo_preparation_ns\":" << record.woPreparationNs
             << ",\"wo_preparation_gpu_ns\":" << record.woPreparationGpuNs
             << ",\"m43_gpu_ns\":" << record.m43GpuNs
             << ",\"aggregation_gpu_ns\":" << record.aggregationGpuNs
             << ",\"projection_gpu_ns\":" << record.projectionGpuNs
             << ",\"accumulation_gpu_ns\":" << record.accumulationGpuNs
             << ",\"m44_gpu_ns\":" << record.m44GpuNs
             << ",\"total_m43_m44_gpu_ns\":" << record.totalGpuNs
             << ",\"cpu_recording_ns\":" << record.cpuRecordingNs
             << ",\"cpu_submission_ns\":" << record.cpuSubmissionNs
             << ",\"final_readback_ns\":" << record.finalReadbackNs
             << ",\"end_to_end_ns\":" << record.endToEndNs
             << ",\"cpu_concatenate_ns\":" << record.cpuConcatenateNs
             << ",\"cpu_pack_ns\":" << record.cpuPackNs
             << ",\"temporary_bytes\":" << record.temporaryBytes
             << ",\"retained_bytes\":" << record.retainedBytes
             << ",\"exact_request_bytes\":" << record.exactBytes
             << ",\"source_head_bytes\":" << record.sourceHeadBytes
             << ",\"contiguous_f32_bytes\":" << record.contiguousF32Bytes
             << ",\"contiguous_packed_bytes\":" << record.contiguousPackedBytes
             << ",\"partial_output_bytes\":" << record.partialOutputBytes
             << ",\"accumulation_bytes\":" << record.accumulationBytes
             << ",\"wo_upload_bytes\":" << record.woUploadBytes
             << ",\"wo_f32_bytes\":" << record.woF32Bytes
             << ",\"wo_packed_bytes\":" << record.woPackedBytes
             << ",\"final_y_bytes\":" << record.finalYBytes
             << ",\"final_readback_bytes\":" << record.finalReadbackBytes
             << ",\"reusable_descriptor_sets\":" << record.reusableDescriptorSets
             << ",\"descriptor_bindings\":" << record.descriptorBindings
             << ",\"submit_count\":" << record.submitCount
             << ",\"dispatch_count\":" << record.dispatchCount
             << ",\"barrier_calls\":" << record.barrierCalls
             << ",\"barrier_buffers\":" << record.barrierBuffers
             << ",\"copy_regions\":" << record.copyRegions
             << ",\"intermediate_host_copies\":" << record.intermediateHostCopies
             << "}";
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m44_multihead_output_projection.json", json.str()),
                "M44 benchmark artifact is written");
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM45ResidualOwnershipCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    struct Workload
    {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t modelWidth;
        std::uint32_t headDim;
        bool primary;
    };
    const std::array<Workload, 6u> workloads{{
        {"tiny", 16u, 128u, 16u, false},
        {"primary", 128u, 1024u, 128u, true},
        {"more_tokens", 256u, 1024u, 128u, false},
        {"wider", 128u, 2048u, 256u, false},
        {"awkward", 127u, 1001u, 127u, false},
        {"boundary", 1024u, 1024u, 64u, false},
    }};
    struct Record
    {
        std::string workload;
        std::string strategy;
        std::string submit;
        std::uint32_t tokens = 0u;
        std::uint32_t modelWidth = 0u;
        std::uint32_t headDim = 0u;
        std::uint64_t replay = 0u;
        std::uint64_t m44Replay = 0u;
        std::uint64_t zGeneration = 0u;
        std::uint64_t m43Gpu = 0u;
        std::uint64_t aggregationGpu = 0u;
        std::uint64_t projectionGpu = 0u;
        std::uint64_t m44Gpu = 0u;
        std::uint64_t residualGpu = 0u;
        std::uint64_t totalGpu = 0u;
        std::uint64_t cpuRecording = 0u;
        std::uint64_t cpuSubmission = 0u;
        std::uint64_t finalReadback = 0u;
        std::uint64_t endToEnd = 0u;
        std::uint64_t retained = 0u;
        std::uint64_t exact = 0u;
        std::uint64_t saved = 0u;
        std::uint64_t allocations = 0u;
        std::uint64_t cpuAdd = 0u;
        std::uint64_t xReadback = 0u;
        std::uint32_t submitCount = 0u;
        bool correct = false;
    };
    std::vector<Record> records;
    std::uint64_t validationWarnings = 0u;
    std::uint64_t validationErrors = 0u;
    std::uint64_t warm10Gpu = 0u;
    std::uint64_t warm100Gpu = 0u;
    std::uint64_t warm10EndToEnd = 0u;
    std::uint64_t warm100EndToEnd = 0u;
    for (const Workload& workload : workloads) {
        void* runtime = nullptr;
        if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
            SKIP("Vulkan runtime unavailable");
        }
        prom_vk_runtime_services services{};
        if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
            services.cooperative_matrix_feature_enabled == 0u) {
            prom_reactor_runtime_destroy_impl(runtime);
            SKIP("M45 corpus requires the proven cooperative tuple");
        }
        std::vector<float> x;
        GroupWeights weights;
        FillGroupInputs(&x, &weights, workload.tokens, workload.modelWidth, workload.headDim);
        ASSERT_TRUE(PrepareGroupWeights(runtime, weights, workload.modelWidth, workload.headDim),
                    "M45 corpus prepares grouped weights");
        prom_m43_resident_x_prepare_request prepareX{};
        prepareX.x = x.data();
        prepareX.element_count = x.size();
        prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth;
        prepareX.generation = 77u;
        prom_m43_resident_x_prepare_result preparedX{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                     "M45 corpus prepares resident X");
        std::vector<float> wo;
        FillOutputProjectionWeight(&wo, workload.headDim, workload.modelWidth);
        ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, workload.headDim,
                                                  workload.modelWidth, 500u),
                    "M45 corpus prepares Wo");
        std::vector<float> heads;
        ASSERT_EQUAL(1u, GroupReference(x, weights, workload.tokens, workload.modelWidth,
                                        workload.headDim, &heads).all_finite,
                     "M45 corpus grouped oracle succeeds");
        std::vector<float> y;
        ASSERT_EQUAL(1u, OutputProjectionReference(heads, wo, workload.tokens,
                                                   workload.headDim, workload.modelWidth,
                                                   PROM_M42_PRECISION_F16_ROUNDED, &y).all_finite,
                     "M45 corpus projection oracle succeeds");
        std::vector<float> expected(y.size());
        for (std::size_t index = 0u; index < expected.size(); ++index) expected[index] = x[index] + y[index];
        for (const std::uint32_t strategy : {PROM_M45_STRATEGY_SEPARATE_OUTPUT,
                                            PROM_M45_STRATEGY_IN_PLACE_Y}) {
            for (const std::uint32_t submit : {PROM_M45_SUBMIT_ONE_COMMAND_BUFFER,
                                              PROM_M45_SUBMIT_TWO_BOUNDED}) {
                std::vector<float> output(expected.size());
                prom_m45_composed_request request{};
                FillM45ComposedRequest(&request, output.data(), workload.tokens,
                                       workload.modelWidth, workload.headDim,
                                       strategy, submit, 77u, 500u);
                prom_m45_composed_result prime0{};
                prom_m45_composed_result prime1{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &prime0),
                             "M45 primes the first ring slot");
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &prime1),
                             "M45 primes the second ring slot");
                for (std::uint32_t warm = 0u; warm < 32u; ++warm) {
                    prom_m45_composed_result ignored{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &ignored),
                                 "M45 warm execution succeeds");
                }
                std::vector<prom_m45_composed_result> measured;
                for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
                    prom_m45_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &value),
                                 "M45 measured execution succeeds");
                    measured.push_back(value);
                }
                const prom_m45_composed_result& last = measured.back();
                prom_m45_mismatch mismatch{};
                const bool correct = prom_m45_residual_compare(expected.data(), output.data(),
                    workload.tokens, workload.modelWidth, 1.0e-2f, 4.0e-2f,
                    &last.residual_plan, &mismatch) == PROM_OK;
                ASSERT_TRUE(correct, "M45 corpus Z is correct");
                ASSERT_EQUAL(prime1.buffer_allocation_count, last.buffer_allocation_count,
                             "M45 performs no allocation after both slots reach warm capacity");
                Record record{};
                record.workload = workload.name;
                record.strategy = strategy == PROM_M45_STRATEGY_IN_PLACE_Y ? "in_place_y" : "separate_output";
                record.submit = submit == PROM_M45_SUBMIT_TWO_BOUNDED ? "two" : "one";
                record.tokens = workload.tokens;
                record.modelWidth = workload.modelWidth;
                record.headDim = workload.headDim;
                record.replay = last.residual_plan.replay_id;
                record.m44Replay = last.projection_plan.replay_id;
                record.zGeneration = last.z_generation;
                record.m43Gpu = MedianGroupMetric(
                    [&measured]() {
                        std::vector<prom_m43_attention_group_result> values;
                        for (const auto& item : measured) values.push_back(item.attention);
                        return values;
                    }(), &prom_m43_attention_group_result::grouped_attention_gpu_ns);
                record.aggregationGpu = MedianM45Metric(measured, &prom_m45_composed_result::aggregation_gpu_ns);
                record.projectionGpu = MedianM45Metric(measured, &prom_m45_composed_result::projection_gpu_ns);
                record.m44Gpu = MedianM45Metric(measured, &prom_m45_composed_result::m44_gpu_ns);
                record.residualGpu = MedianM45Metric(measured, &prom_m45_composed_result::residual_gpu_ns);
                record.totalGpu = MedianM45Metric(measured, &prom_m45_composed_result::total_m43_m44_m45_gpu_ns);
                record.cpuRecording = MedianM45Metric(measured, &prom_m45_composed_result::cpu_recording_ns);
                record.cpuSubmission = MedianM45Metric(measured, &prom_m45_composed_result::cpu_submission_ns);
                record.finalReadback = MedianM45Metric(measured, &prom_m45_composed_result::final_readback_ns);
                record.endToEnd = MedianM45Metric(measured, &prom_m45_composed_result::end_to_end_ns);
                record.retained = last.retained_bytes;
                record.exact = last.exact_request_bytes;
                record.saved = last.residual_plan.memory.in_place_y_saved_bytes;
                record.allocations = last.buffer_allocation_count;
                record.submitCount = last.submit_count;
                record.correct = correct;
                records.push_back(record);
                if (workload.primary && strategy == PROM_M45_STRATEGY_IN_PLACE_Y &&
                    submit == PROM_M45_SUBMIT_ONE_COMMAND_BUFFER) {
                    std::vector<prom_m45_composed_result> repeated10;
                    std::vector<prom_m45_composed_result> repeated100;
                    for (std::uint32_t repeat = 0u; repeat < 10u; ++repeat) {
                        prom_m45_composed_result value{};
                        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &value),
                                     "M45 primary 10-repeat succeeds");
                        repeated10.push_back(value);
                    }
                    for (std::uint32_t repeat = 0u; repeat < 100u; ++repeat) {
                        prom_m45_composed_result value{};
                        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &request, &value),
                                     "M45 primary 100-repeat succeeds");
                        repeated100.push_back(value);
                    }
                    warm10Gpu = MedianM45Metric(repeated10, &prom_m45_composed_result::total_m43_m44_m45_gpu_ns);
                    warm10EndToEnd = MedianM45Metric(repeated10, &prom_m45_composed_result::end_to_end_ns);
                    warm100Gpu = MedianM45Metric(repeated100, &prom_m45_composed_result::total_m43_m44_m45_gpu_ns);
                    warm100EndToEnd = MedianM45Metric(repeated100, &prom_m45_composed_result::end_to_end_ns);
                }
            }
        }
        std::vector<float> yReadback(y.size());
        prom_m44_composed_request m44Request{};
        FillM44ComposedRequest(&m44Request, nullptr, yReadback.data(), workload.tokens,
                               workload.modelWidth, workload.headDim,
                               PROM_M44_AGGREGATION_INTERLEAVE,
                               PROM_M44_PROJECTION_COOPERATIVE,
                               PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
                               PROM_M42_INPUT_RESIDENT_X, 77u, 500u);
        std::vector<prom_m44_composed_result> m44Measured;
        std::vector<std::uint64_t> hostEndToEnd;
        std::vector<std::uint64_t> cpuAdd;
        std::vector<std::uint64_t> xReadbackTimes;
        for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
            prom_m44_composed_result m44{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m44_execute_composed(runtime, &m44Request, &m44),
                         "M43+M44 no-residual baseline executes");
            std::vector<float> xReadback(x.size());
            prom_m45_resident_x_readback_request xRequest{};
            xRequest.output = xReadback.data();
            xRequest.output_element_count = xReadback.size();
            xRequest.tokens = workload.tokens;
            xRequest.model_width = workload.modelWidth;
            xRequest.expected_x_generation = 77u;
            prom_m45_resident_x_readback_result xResult{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_read_resident_x(runtime, &xRequest, &xResult),
                         "host-bounce baseline reads the real resident X buffer");
            const auto begin = std::chrono::steady_clock::now();
            std::vector<float> hostZ(yReadback.size());
            for (std::size_t index = 0u; index < hostZ.size(); ++index)
                hostZ[index] = xReadback[index] + yReadback[index];
            const auto end = std::chrono::steady_clock::now();
            const auto addNs = static_cast<std::uint64_t>(
                std::chrono::duration_cast<std::chrono::nanoseconds>(end - begin).count());
            prom_m44_mismatch mismatch{};
            ASSERT_EQUAL(PROM_OK, prom_m44_output_projection_compare(
                expected.data(), hostZ.data(), workload.tokens, workload.modelWidth,
                1.0e-2f, 4.0e-2f, PROM_M44_AGGREGATION_INTERLEAVE,
                500u, m44.plan.m43_aggregate_replay_id, m44.plan.replay_id, &mismatch),
                "CPU host-bounce residual remains correct");
            m44Measured.push_back(m44);
            cpuAdd.push_back(addNs);
            xReadbackTimes.push_back(xResult.end_to_end_ns);
            hostEndToEnd.push_back(m44.end_to_end_ns + xResult.end_to_end_ns + addNs);
        }
        std::sort(cpuAdd.begin(), cpuAdd.end());
        std::sort(xReadbackTimes.begin(), xReadbackTimes.end());
        std::sort(hostEndToEnd.begin(), hostEndToEnd.end());
        Record noResidual{};
        noResidual.workload = workload.name;
        noResidual.strategy = "m43_m44_no_residual";
        noResidual.submit = "one";
        noResidual.tokens = workload.tokens;
        noResidual.modelWidth = workload.modelWidth;
        noResidual.headDim = workload.headDim;
        noResidual.replay = m44Measured.back().plan.replay_id;
        noResidual.m44Replay = noResidual.replay;
        noResidual.m43Gpu = MedianGroupMetric(
            [&m44Measured]() {
                std::vector<prom_m43_attention_group_result> values;
                for (const auto& item : m44Measured) values.push_back(item.attention);
                return values;
            }(), &prom_m43_attention_group_result::grouped_attention_gpu_ns);
        noResidual.aggregationGpu = MedianM44Metric(m44Measured, &prom_m44_composed_result::aggregation_gpu_ns);
        noResidual.projectionGpu = MedianM44Metric(m44Measured, &prom_m44_composed_result::projection_gpu_ns);
        noResidual.m44Gpu = MedianM44Metric(m44Measured, &prom_m44_composed_result::m44_gpu_ns);
        noResidual.totalGpu = MedianM44Metric(m44Measured, &prom_m44_composed_result::total_m43_m44_gpu_ns);
        noResidual.finalReadback = MedianM44Metric(m44Measured, &prom_m44_composed_result::final_readback_ns);
        noResidual.endToEnd = MedianM44Metric(m44Measured, &prom_m44_composed_result::end_to_end_ns);
        noResidual.retained = m44Measured.back().retained_bytes;
        noResidual.exact = m44Measured.back().exact_request_bytes;
        noResidual.submitCount = 1u;
        noResidual.correct = true;
        records.push_back(noResidual);
        Record host = noResidual;
        host.strategy = "cpu_host_bounce";
        host.submit = "x_y_readback_cpu_add_no_reupload";
        host.cpuAdd = cpuAdd[cpuAdd.size() / 2u];
        host.xReadback = xReadbackTimes[xReadbackTimes.size() / 2u];
        host.endToEnd = hostEndToEnd[hostEndToEnd.size() / 2u];
        host.submitCount = 2u;
        records.push_back(host);
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                     "M45 workload validation services remain available");
        validationWarnings += services.validation_warning_count;
        validationErrors += services.validation_error_count;
        prom_reactor_runtime_destroy_impl(runtime);
    }
    ASSERT_EQUAL(0u, validationWarnings, "M45 corpus has zero validation warnings");
    ASSERT_EQUAL(0u, validationErrors, "M45 corpus has zero validation errors");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m45.device-resident-residual.v1\",\n"
         << "  \"shader\": {\"source_sha256\": \"e83b60a31f82d9af785710d54a2055814917e1d0e9ecb988ab8eaf489313777d\", "
            "\"hlsl_sha256\": \"7fedb0c77e12e2b975ae6529e1c50d5e98490a0dcc4f31ff1bfab8808e5b18e6\", "
            "\"spirv_sha256\": \"c6e177b9fb86f1e5b01e05c544091577629fb4f51d4e08b9c6b624d85b6d0acc\"},\n"
         << "  \"validation\": {\"warnings\": " << validationWarnings
         << ", \"errors\": " << validationErrors << "},\n"
         << "  \"warmups_per_plan\": 32,\n  \"measurements_per_plan\": 5,\n"
         << "  \"primary_repeats\": {\"warm_10_gpu_ns\": " << warm10Gpu
         << ", \"warm_10_end_to_end_ns\": " << warm10EndToEnd
         << ", \"warm_100_gpu_ns\": " << warm100Gpu
         << ", \"warm_100_end_to_end_ns\": " << warm100EndToEnd << "},\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const Record& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"strategy\":\"" << record.strategy
             << "\",\"submit_policy\":\"" << record.submit
             << "\",\"tokens\":" << record.tokens
             << ",\"model_width\":" << record.modelWidth
             << ",\"head_dim\":" << record.headDim
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"replay_id\":" << record.replay
             << ",\"m44_replay_id\":" << record.m44Replay
             << ",\"z_generation\":" << record.zGeneration
             << ",\"m43_gpu_ns\":" << record.m43Gpu
             << ",\"aggregation_gpu_ns\":" << record.aggregationGpu
             << ",\"projection_gpu_ns\":" << record.projectionGpu
             << ",\"m44_gpu_ns\":" << record.m44Gpu
             << ",\"residual_gpu_ns\":" << record.residualGpu
             << ",\"total_m43_m44_m45_gpu_ns\":" << record.totalGpu
             << ",\"cpu_recording_ns\":" << record.cpuRecording
             << ",\"cpu_submission_ns\":" << record.cpuSubmission
             << ",\"final_readback_ns\":" << record.finalReadback
             << ",\"end_to_end_ns\":" << record.endToEnd
             << ",\"cpu_add_ns\":" << record.cpuAdd
             << ",\"x_readback_ns\":" << record.xReadback
             << ",\"retained_bytes\":" << record.retained
             << ",\"exact_request_bytes\":" << record.exact
             << ",\"in_place_saved_bytes\":" << record.saved
             << ",\"allocation_count\":" << record.allocations
             << ",\"submit_count\":" << record.submitCount << "}";
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m45_device_resident_residual.json", json.str()),
                "M45 benchmark artifact is written");
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM46DeviceResidentRmsNormCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    struct Workload
    {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t modelWidth;
        std::uint32_t headDim;
        bool primary;
    };
    const std::array<Workload, 7u> workloads{{
        {"tiny", 16u, 128u, 16u, false},
        {"primary", 128u, 1024u, 128u, true},
        {"more_tokens", 256u, 1024u, 128u, false},
        {"wider", 128u, 2048u, 256u, false},
        {"awkward", 127u, 1001u, 127u, false},
        {"staged_4096", 128u, 4096u, 512u, false},
        {"token_boundary", 1024u, 1024u, 128u, false},
    }};
    struct Record
    {
        std::string workload;
        std::string strategy;
        std::string submit;
        std::string reduction;
        std::uint32_t tokens = 0u;
        std::uint32_t modelWidth = 0u;
        std::uint32_t headDim = 0u;
        std::uint32_t submitCount = 0u;
        std::uint32_t dispatchCount = 0u;
        std::uint32_t barrierCount = 0u;
        std::uint64_t replay = 0u;
        std::uint64_t m45Replay = 0u;
        std::uint64_t nGeneration = 0u;
        std::uint64_t weightPreparation = 0u;
        std::uint64_t reductionGpu = 0u;
        std::uint64_t finalReductionGpu = 0u;
        std::uint64_t invRmsGpu = 0u;
        std::uint64_t applyGpu = 0u;
        std::uint64_t m46Gpu = 0u;
        std::uint64_t residualGpu = 0u;
        std::uint64_t completeGpu = 0u;
        std::uint64_t cpuRecording = 0u;
        std::uint64_t cpuSubmission = 0u;
        std::uint64_t finalReadback = 0u;
        std::uint64_t endToEnd = 0u;
        std::uint64_t retained = 0u;
        std::uint64_t exact = 0u;
        std::uint64_t partialBytes = 0u;
        std::uint64_t invRmsBytes = 0u;
        std::uint64_t savedBytes = 0u;
        std::uint64_t allocationCount = 0u;
        std::uint64_t hostCpuNorm = 0u;
        bool correct = false;
    };
    std::vector<Record> records;
    std::uint64_t validationWarnings = 0u;
    std::uint64_t validationErrors = 0u;
    std::uint64_t warm10M46 = 0u;
    std::uint64_t warm10Complete = 0u;
    std::uint64_t warm10EndToEnd = 0u;
    std::uint64_t warm100M46 = 0u;
    std::uint64_t warm100Complete = 0u;
    std::uint64_t warm100EndToEnd = 0u;
    for (const Workload& workload : workloads) {
        void* runtime = nullptr;
        if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) {
            SKIP("Vulkan runtime unavailable");
        }
        prom_vk_runtime_services services{};
        if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
            services.cooperative_matrix_feature_enabled == 0u) {
            prom_reactor_runtime_destroy_impl(runtime);
            SKIP("M46 corpus requires the proven cooperative tuple");
        }
        std::vector<float> x;
        GroupWeights weights;
        FillGroupInputs(&x, &weights, workload.tokens, workload.modelWidth, workload.headDim);
        ASSERT_TRUE(PrepareGroupWeights(runtime, weights, workload.modelWidth, workload.headDim),
                    "M46 corpus prepares grouped weights");
        prom_m43_resident_x_prepare_request prepareX{};
        prepareX.x = x.data();
        prepareX.element_count = x.size();
        prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth;
        prepareX.generation = 177u;
        prom_m43_resident_x_prepare_result preparedX{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                     "M46 corpus prepares resident X");
        std::vector<float> wo;
        FillOutputProjectionWeight(&wo, workload.headDim, workload.modelWidth);
        ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, workload.headDim,
                                                  workload.modelWidth, 1500u),
                    "M46 corpus prepares Wo");
        std::vector<float> weight(workload.modelWidth);
        for (std::uint32_t column = 0u; column < workload.modelWidth; ++column)
            weight[column] = 0.75f + static_cast<float>(column % 17u) / 64.0f;
        prom_m46_weight_prepare_request prepareWeight{};
        prepareWeight.values = weight.data();
        prepareWeight.element_count = weight.size();
        prepareWeight.model_width = workload.modelWidth;
        prepareWeight.generation = 2500u;
        prom_m46_weight_prepare_result preparedWeight{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m46_prepare_weight(runtime, &prepareWeight, &preparedWeight),
                     "M46 corpus prepares persistent scale Weight");

        std::vector<float> zOracle(static_cast<std::size_t>(workload.tokens) * workload.modelWidth);
        prom_m45_composed_request oracleRequest{};
        FillM45ComposedRequest(&oracleRequest, zOracle.data(), workload.tokens,
                               workload.modelWidth, workload.headDim,
                               PROM_M45_STRATEGY_IN_PLACE_Y,
                               PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 177u, 1500u);
        prom_m45_composed_result oracleResult{};
        ASSERT_EQUAL(PROM_OK,
                     prom_reactor_runtime_m45_execute_composed(runtime, &oracleRequest, &oracleResult),
                     "M46 corpus captures one real M45 Z correctness oracle");
        std::vector<float> expected(zOracle.size());
        std::vector<float> invRms(workload.tokens);
        prom_m46_reference_request reference{};
        reference.z = zOracle.data();
        reference.weight = weight.data();
        reference.n = expected.data();
        reference.inv_rms = invRms.data();
        reference.z_element_count = zOracle.size();
        reference.weight_element_count = weight.size();
        reference.n_element_count = expected.size();
        reference.tokens = workload.tokens;
        reference.model_width = workload.modelWidth;
        reference.z_row_stride = workload.modelWidth;
        reference.n_row_stride = workload.modelWidth;
        reference.epsilon = 1.0e-5f;
        ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&reference),
                     "M46 corpus FP32 oracle succeeds");

        for (const std::uint32_t strategy : {PROM_M46_STRATEGY_SEPARATE_OUTPUT,
                                             PROM_M46_STRATEGY_IN_PLACE_Z}) {
            for (const std::uint32_t submit : {PROM_M46_SUBMIT_ONE_COMMAND_BUFFER,
                                               PROM_M46_SUBMIT_TWO_BOUNDED}) {
                std::vector<float> output(expected.size());
                prom_m46_composed_request request{};
                FillM45ComposedRequest(&request.upstream, nullptr, workload.tokens,
                                       workload.modelWidth, workload.headDim,
                                       PROM_M45_STRATEGY_IN_PLACE_Y,
                                       PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 177u, 1500u);
                request.output = output.data();
                request.output_element_count = output.size();
                request.epsilon = 1.0e-5f;
                request.strategy = strategy;
                request.submit_policy = submit;
                request.required_weight_generation = 2500u;
                prom_m46_composed_result prime0{};
                prom_m46_composed_result prime1{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &prime0),
                             "M46 primes the first physical slot");
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &prime1),
                             "M46 primes the second physical slot");
                for (std::uint32_t warm = 0u; warm < 16u; ++warm) {
                    prom_m46_composed_result ignored{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &ignored),
                                 "M46 warm execution succeeds");
                }
                std::vector<prom_m46_composed_result> measured;
                for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
                    prom_m46_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &value),
                                 "M46 measured execution succeeds");
                    measured.push_back(value);
                }
                const prom_m46_composed_result& last = measured.back();
                prom_m46_mismatch mismatch{};
                const bool correct = prom_m46_rmsnorm_compare(
                    expected.data(), output.data(), workload.tokens, workload.modelWidth,
                    2.0e-3f, 2.0e-2f, &last.rmsnorm_plan, nullptr,
                    invRms.data(), &mismatch) == PROM_OK;
                ASSERT_TRUE(correct, "M46 corpus N is correct");
                ASSERT_EQUAL(prime1.buffer_allocation_count, last.buffer_allocation_count,
                             "M46 performs no allocation after both slots reach warm capacity");
                Record record{};
                record.workload = workload.name;
                record.strategy = strategy == PROM_M46_STRATEGY_IN_PLACE_Z
                                    ? "in_place_z" : "separate_output";
                record.submit = submit == PROM_M46_SUBMIT_TWO_BOUNDED ? "two" : "one";
                record.reduction = last.rmsnorm_plan.reduction_plan == PROM_M46_REDUCTION_STAGED
                                     ? "staged" : "fused";
                record.tokens = workload.tokens;
                record.modelWidth = workload.modelWidth;
                record.headDim = workload.headDim;
                record.submitCount = last.submit_count;
                record.dispatchCount = last.rmsnorm_plan.dispatch_count;
                record.barrierCount = last.rmsnorm_plan.barrier_count;
                record.replay = last.rmsnorm_plan.replay_id;
                record.m45Replay = last.upstream.residual_plan.replay_id;
                record.nGeneration = last.n_generation;
                record.weightPreparation = preparedWeight.preparation_ns;
                record.reductionGpu = MedianM46Metric(measured, &prom_m46_composed_result::reduction_gpu_ns);
                record.finalReductionGpu = MedianM46Metric(measured, &prom_m46_composed_result::final_reduction_gpu_ns);
                record.invRmsGpu = MedianM46Metric(measured, &prom_m46_composed_result::inv_rms_gpu_ns);
                record.applyGpu = MedianM46Metric(measured, &prom_m46_composed_result::apply_gpu_ns);
                record.m46Gpu = MedianM46Metric(measured, &prom_m46_composed_result::m46_gpu_ns);
                record.residualGpu = MedianM45Metric(
                    [&measured]() {
                        std::vector<prom_m45_composed_result> values;
                        for (const auto& item : measured) values.push_back(item.upstream);
                        return values;
                    }(), &prom_m45_composed_result::residual_gpu_ns);
                record.completeGpu = MedianM46Metric(measured, &prom_m46_composed_result::total_m43_m44_m45_m46_gpu_ns);
                record.cpuRecording = MedianM46Metric(measured, &prom_m46_composed_result::cpu_recording_ns);
                record.cpuSubmission = MedianM46Metric(measured, &prom_m46_composed_result::cpu_submission_ns);
                record.finalReadback = MedianM46Metric(measured, &prom_m46_composed_result::final_readback_ns);
                record.endToEnd = MedianM46Metric(measured, &prom_m46_composed_result::end_to_end_ns);
                record.retained = last.retained_bytes;
                record.exact = last.exact_request_bytes;
                record.partialBytes = last.rmsnorm_plan.memory.partial_sum_bytes;
                record.invRmsBytes = last.rmsnorm_plan.memory.inv_rms_bytes;
                record.savedBytes = last.rmsnorm_plan.memory.in_place_saved_bytes;
                record.allocationCount = last.buffer_allocation_count;
                record.correct = correct;
                records.push_back(record);
                if (workload.primary && strategy == PROM_M46_STRATEGY_IN_PLACE_Z &&
                    submit == PROM_M46_SUBMIT_ONE_COMMAND_BUFFER) {
                    std::vector<prom_m46_composed_result> repeat10;
                    std::vector<prom_m46_composed_result> repeat100;
                    for (std::uint32_t repeat = 0u; repeat < 10u; ++repeat) {
                        prom_m46_composed_result value{};
                        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &value),
                                     "M46 primary 10-repeat succeeds");
                        repeat10.push_back(value);
                    }
                    for (std::uint32_t repeat = 0u; repeat < 100u; ++repeat) {
                        prom_m46_composed_result value{};
                        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &value),
                                     "M46 primary 100-repeat succeeds");
                        repeat100.push_back(value);
                    }
                    warm10M46 = MedianM46Metric(repeat10, &prom_m46_composed_result::m46_gpu_ns);
                    warm10Complete = MedianM46Metric(repeat10, &prom_m46_composed_result::total_m43_m44_m45_m46_gpu_ns);
                    warm10EndToEnd = MedianM46Metric(repeat10, &prom_m46_composed_result::end_to_end_ns);
                    warm100M46 = MedianM46Metric(repeat100, &prom_m46_composed_result::m46_gpu_ns);
                    warm100Complete = MedianM46Metric(repeat100, &prom_m46_composed_result::total_m43_m44_m45_m46_gpu_ns);
                    warm100EndToEnd = MedianM46Metric(repeat100, &prom_m46_composed_result::end_to_end_ns);
                }
            }
        }

        if (workload.primary) {
            std::vector<float> output(expected.size());
            prom_m46_composed_request request{};
            FillM45ComposedRequest(&request.upstream, nullptr, workload.tokens,
                                   workload.modelWidth, workload.headDim,
                                   PROM_M45_STRATEGY_IN_PLACE_Y,
                                   PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 177u, 1500u);
            request.output = output.data();
            request.output_element_count = output.size();
            request.epsilon = 1.0e-5f;
            request.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
            request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
            request.requested_reduction_plan = PROM_M46_REDUCTION_FORCE_STAGED;
            request.required_weight_generation = 2500u;
            prom_m46_composed_result prime0{};
            prom_m46_composed_result prime1{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &prime0),
                         "M46 forced-staged audit primes the first slot");
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &prime1),
                         "M46 forced-staged audit primes the second slot");
            for (std::uint32_t warm = 0u; warm < 16u; ++warm) {
                prom_m46_composed_result ignored{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &ignored),
                             "M46 forced-staged audit warms");
            }
            std::vector<prom_m46_composed_result> measured;
            for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
                prom_m46_composed_result value{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &request, &value),
                             "M46 forced-staged audit measures");
                measured.push_back(value);
            }
            const prom_m46_composed_result& last = measured.back();
            prom_m46_mismatch mismatch{};
            ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_compare(
                expected.data(), output.data(), workload.tokens, workload.modelWidth,
                2.0e-3f, 2.0e-2f, &last.rmsnorm_plan, nullptr,
                invRms.data(), &mismatch), "forced-staged primary N is correct");
            ASSERT_EQUAL(prime1.buffer_allocation_count, last.buffer_allocation_count,
                         "forced-staged primary remains allocation-free after warm capacity");
            Record record{};
            record.workload = workload.name;
            record.strategy = "in_place_z";
            record.submit = "one_forced_staged";
            record.reduction = "staged";
            record.tokens = workload.tokens;
            record.modelWidth = workload.modelWidth;
            record.headDim = workload.headDim;
            record.submitCount = last.submit_count;
            record.dispatchCount = last.rmsnorm_plan.dispatch_count;
            record.barrierCount = last.rmsnorm_plan.barrier_count;
            record.replay = last.rmsnorm_plan.replay_id;
            record.m45Replay = last.upstream.residual_plan.replay_id;
            record.nGeneration = last.n_generation;
            record.weightPreparation = preparedWeight.preparation_ns;
            record.reductionGpu = MedianM46Metric(measured, &prom_m46_composed_result::reduction_gpu_ns);
            record.finalReductionGpu = MedianM46Metric(measured, &prom_m46_composed_result::final_reduction_gpu_ns);
            record.invRmsGpu = MedianM46Metric(measured, &prom_m46_composed_result::inv_rms_gpu_ns);
            record.applyGpu = MedianM46Metric(measured, &prom_m46_composed_result::apply_gpu_ns);
            record.m46Gpu = MedianM46Metric(measured, &prom_m46_composed_result::m46_gpu_ns);
            record.residualGpu = last.upstream.residual_gpu_ns;
            record.completeGpu = MedianM46Metric(measured, &prom_m46_composed_result::total_m43_m44_m45_m46_gpu_ns);
            record.cpuRecording = MedianM46Metric(measured, &prom_m46_composed_result::cpu_recording_ns);
            record.cpuSubmission = MedianM46Metric(measured, &prom_m46_composed_result::cpu_submission_ns);
            record.finalReadback = MedianM46Metric(measured, &prom_m46_composed_result::final_readback_ns);
            record.endToEnd = MedianM46Metric(measured, &prom_m46_composed_result::end_to_end_ns);
            record.retained = last.retained_bytes;
            record.exact = last.exact_request_bytes;
            record.partialBytes = last.rmsnorm_plan.memory.partial_sum_bytes;
            record.invRmsBytes = last.rmsnorm_plan.memory.inv_rms_bytes;
            record.savedBytes = last.rmsnorm_plan.memory.in_place_saved_bytes;
            record.allocationCount = last.buffer_allocation_count;
            record.correct = true;
            records.push_back(record);
        }

        std::vector<std::uint64_t> hostEndToEnd;
        std::vector<std::uint64_t> hostCpu;
        std::vector<prom_m45_composed_result> m45Measured;
        for (std::uint32_t iteration = 0u; iteration < 5u; ++iteration) {
            std::vector<float> hostZ(zOracle.size());
            prom_m45_composed_request hostRequest{};
            FillM45ComposedRequest(&hostRequest, hostZ.data(), workload.tokens,
                                   workload.modelWidth, workload.headDim,
                                   PROM_M45_STRATEGY_IN_PLACE_Y,
                                   PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 177u, 1500u);
            prom_m45_composed_result m45{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m45_execute_composed(runtime, &hostRequest, &m45),
                         "M45 no-normalization and host-bounce source executes");
            std::vector<float> hostN(hostZ.size());
            prom_m46_reference_request hostReference = reference;
            hostReference.z = hostZ.data();
            hostReference.n = hostN.data();
            const auto cpuBegin = std::chrono::steady_clock::now();
            ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&hostReference),
                         "host-bounce CPU RMSNorm succeeds");
            const auto cpuEnd = std::chrono::steady_clock::now();
            const auto cpuNs = static_cast<std::uint64_t>(
                std::chrono::duration_cast<std::chrono::nanoseconds>(cpuEnd - cpuBegin).count());
            prom_m46_mismatch mismatch{};
            prom_m46_rmsnorm_plan hostPlan{};
            hostPlan.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
            ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_compare(
                expected.data(), hostN.data(), workload.tokens, workload.modelWidth,
                2.0e-3f, 2.0e-2f, &hostPlan, nullptr, invRms.data(), &mismatch),
                "host-bounce CPU N remains correct");
            hostCpu.push_back(cpuNs);
            hostEndToEnd.push_back(m45.end_to_end_ns + cpuNs);
            m45Measured.push_back(m45);
        }
        std::sort(hostCpu.begin(), hostCpu.end());
        std::sort(hostEndToEnd.begin(), hostEndToEnd.end());
        Record baseline{};
        baseline.workload = workload.name;
        baseline.strategy = "m43_m44_m45_no_normalization";
        baseline.submit = "one";
        baseline.reduction = "none";
        baseline.tokens = workload.tokens;
        baseline.modelWidth = workload.modelWidth;
        baseline.headDim = workload.headDim;
        baseline.submitCount = 1u;
        baseline.residualGpu = MedianM45Metric(m45Measured, &prom_m45_composed_result::residual_gpu_ns);
        baseline.completeGpu = MedianM45Metric(m45Measured, &prom_m45_composed_result::total_m43_m44_m45_gpu_ns);
        baseline.finalReadback = MedianM45Metric(m45Measured, &prom_m45_composed_result::final_readback_ns);
        baseline.endToEnd = MedianM45Metric(m45Measured, &prom_m45_composed_result::end_to_end_ns);
        baseline.correct = true;
        records.push_back(baseline);
        Record host = baseline;
        host.strategy = "cpu_host_bounce";
        host.submit = "final_z_readback_cpu_rmsnorm_no_reupload";
        host.hostCpuNorm = hostCpu[hostCpu.size() / 2u];
        host.endToEnd = hostEndToEnd[hostEndToEnd.size() / 2u];
        records.push_back(host);
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                     "M46 corpus validation services remain available");
        validationWarnings += services.validation_warning_count;
        validationErrors += services.validation_error_count;
        prom_reactor_runtime_destroy_impl(runtime);
    }
    ASSERT_EQUAL(0u, validationWarnings, "M46 corpus has zero validation warnings");
    ASSERT_EQUAL(0u, validationErrors, "M46 corpus has zero validation errors");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m46.device-resident-rmsnorm.v1\",\n"
         << "  \"epsilon\": 0.00001,\n"
         << "  \"warmups_per_plan\": 16,\n  \"measurements_per_plan\": 5,\n"
         << "  \"validation\": {\"warnings\": " << validationWarnings
         << ", \"errors\": " << validationErrors << "},\n"
         << "  \"primary_repeats\": {\"warm_10_m46_gpu_ns\": " << warm10M46
         << ", \"warm_10_complete_gpu_ns\": " << warm10Complete
         << ", \"warm_10_end_to_end_ns\": " << warm10EndToEnd
         << ", \"warm_100_m46_gpu_ns\": " << warm100M46
         << ", \"warm_100_complete_gpu_ns\": " << warm100Complete
         << ", \"warm_100_end_to_end_ns\": " << warm100EndToEnd << "},\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const Record& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"strategy\":\"" << record.strategy
             << "\",\"submit_policy\":\"" << record.submit
             << "\",\"reduction_plan\":\"" << record.reduction
             << "\",\"tokens\":" << record.tokens
             << ",\"model_width\":" << record.modelWidth
             << ",\"head_dim\":" << record.headDim
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"submit_count\":" << record.submitCount
             << ",\"dispatch_count\":" << record.dispatchCount
             << ",\"barrier_count\":" << record.barrierCount
             << ",\"replay_id\":" << record.replay
             << ",\"m45_replay_id\":" << record.m45Replay
             << ",\"n_generation\":" << record.nGeneration
             << ",\"weight_preparation_ns\":" << record.weightPreparation
             << ",\"reduction_gpu_ns\":" << record.reductionGpu
             << ",\"final_reduction_gpu_ns\":" << record.finalReductionGpu
             << ",\"inv_rms_gpu_ns\":" << record.invRmsGpu
             << ",\"apply_gpu_ns\":" << record.applyGpu
             << ",\"m46_gpu_ns\":" << record.m46Gpu
             << ",\"residual_gpu_ns\":" << record.residualGpu
             << ",\"complete_gpu_ns\":" << record.completeGpu
             << ",\"cpu_recording_ns\":" << record.cpuRecording
             << ",\"cpu_submission_ns\":" << record.cpuSubmission
             << ",\"final_readback_ns\":" << record.finalReadback
             << ",\"host_cpu_norm_ns\":" << record.hostCpuNorm
             << ",\"end_to_end_ns\":" << record.endToEnd
             << ",\"retained_bytes\":" << record.retained
             << ",\"exact_request_bytes\":" << record.exact
             << ",\"partial_bytes\":" << record.partialBytes
             << ",\"inv_rms_bytes\":" << record.invRmsBytes
             << ",\"in_place_saved_bytes\":" << record.savedBytes
             << ",\"allocation_count\":" << record.allocationCount << "}";
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m46_device_resident_rmsnorm.json", json.str()),
                "M46 benchmark artifact is written");
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM47GatedFfnCompleteBlockCorpus, 1u)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    struct Workload {
        const char* name;
        std::uint32_t tokens;
        std::uint32_t modelWidth;
        std::uint32_t headDim;
        std::uint32_t ffnWidth;
    };
    const std::array<Workload, 7u> workloads{{
        {"tiny", 16u, 128u, 16u, 256u},
        {"primary", 128u, 1024u, 128u, 4096u},
        {"more_tokens", 256u, 1024u, 128u, 4096u},
        {"smaller_expansion", 128u, 1024u, 128u, 2048u},
        {"wider_model", 128u, 2048u, 256u, 4096u},
        {"awkward", 127u, 1001u, 127u, 3001u},
        {"token_boundary", 1024u, 256u, 32u, 512u},
    }};
    struct Strategy {
        const char* name;
        std::uint32_t path;
        std::uint32_t gating;
        std::uint32_t residual;
        std::uint32_t submit;
    };
    const std::array<Strategy, 7u> strategies{{
        {"separate_cooperative", PROM_M47_PROJECTION_COOPERATIVE, PROM_M47_GATING_SEPARATE,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {"fused_fp32_cooperative", PROM_M47_PROJECTION_COOPERATIVE, PROM_M47_GATING_FUSED_FP32,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {"direct_packed_cooperative", PROM_M47_PROJECTION_COOPERATIVE,
         PROM_M47_GATING_FUSED_DIRECT_PACKED, PROM_M47_RESIDUAL_IN_PLACE_DOWN,
         PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {"direct_packed_cooperative_split", PROM_M47_PROJECTION_COOPERATIVE,
         PROM_M47_GATING_FUSED_DIRECT_PACKED, PROM_M47_RESIDUAL_IN_PLACE_DOWN,
         PROM_M47_SUBMIT_TWO_BOUNDED},
        {"direct_packed_conventional", PROM_M47_PROJECTION_CONVENTIONAL_FP16,
         PROM_M47_GATING_FUSED_DIRECT_PACKED, PROM_M47_RESIDUAL_IN_PLACE_DOWN,
         PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {"fused_fp32_a2x4", PROM_M47_PROJECTION_A2X4_FP32, PROM_M47_GATING_FUSED_FP32,
         PROM_M47_RESIDUAL_IN_PLACE_DOWN, PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
        {"direct_packed_separate_output", PROM_M47_PROJECTION_COOPERATIVE,
         PROM_M47_GATING_FUSED_DIRECT_PACKED, PROM_M47_RESIDUAL_SEPARATE_OUTPUT,
         PROM_M47_SUBMIT_ONE_COMMAND_BUFFER},
    }};
    struct Record {
        std::string workload;
        std::string strategy;
        std::uint32_t path = 0u;
        std::uint32_t gating = 0u;
        std::uint32_t residual = 0u;
        std::uint32_t submit = 0u;
        std::uint32_t tokens = 0u;
        std::uint32_t modelWidth = 0u;
        std::uint32_t ffnWidth = 0u;
        std::uint64_t replay = 0u;
        std::uint64_t outputGeneration = 0u;
        std::uint64_t nPack = 0u;
        std::uint64_t gate = 0u;
        std::uint64_t up = 0u;
        std::uint64_t activation = 0u;
        std::uint64_t multiply = 0u;
        std::uint64_t fused = 0u;
        std::uint64_t hiddenPack = 0u;
        std::uint64_t down = 0u;
        std::uint64_t residualGpu = 0u;
        std::uint64_t m47 = 0u;
        std::uint64_t complete = 0u;
        std::uint64_t recording = 0u;
        std::uint64_t submission = 0u;
        std::uint64_t readback = 0u;
        std::uint64_t endToEnd = 0u;
        std::uint64_t retained = 0u;
        std::uint64_t exact = 0u;
        std::uint64_t allocationCount = 0u;
        std::uint64_t preparation = 0u;
        std::uint64_t preparationGpu = 0u;
        std::uint64_t hostCpu = 0u;
        std::uint64_t hostEndToEnd = 0u;
        bool correct = false;
    };
    std::vector<Record> records;
    std::uint64_t validationWarnings = 0u;
    std::uint64_t validationErrors = 0u;
    std::uint64_t warm10M47 = 0u;
    std::uint64_t warm10Complete = 0u;
    std::uint64_t warm10EndToEnd = 0u;
    std::uint64_t warm100M47 = 0u;
    std::uint64_t warm100Complete = 0u;
    std::uint64_t warm100EndToEnd = 0u;
    for (const Workload& workload : workloads) {
        void* runtime = nullptr;
        if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr)
            SKIP("Vulkan runtime unavailable");
        prom_vk_runtime_services services{};
        if (prom_reactor_runtime_get_vk_services(runtime, &services) != PROM_OK ||
            services.cooperative_matrix_feature_enabled == 0u) {
            prom_reactor_runtime_destroy_impl(runtime);
            SKIP("M47 corpus requires the proven cooperative tuple");
        }
        std::vector<float> x;
        GroupWeights attentionWeights;
        FillGroupInputs(&x, &attentionWeights, workload.tokens, workload.modelWidth, workload.headDim);
        ASSERT_TRUE(PrepareGroupWeights(runtime, attentionWeights, workload.modelWidth, workload.headDim),
                    "M47 corpus prepares grouped weights");
        prom_m43_resident_x_prepare_request prepareX{};
        prepareX.x = x.data();
        prepareX.element_count = x.size();
        prepareX.tokens = workload.tokens;
        prepareX.model_width = workload.modelWidth;
        prepareX.generation = 4700u;
        prom_m43_resident_x_prepare_result preparedX{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m43_prepare_resident_x(runtime, &prepareX, &preparedX),
                     "M47 corpus prepares resident X");
        std::vector<float> wo;
        FillOutputProjectionWeight(&wo, workload.headDim, workload.modelWidth);
        ASSERT_TRUE(PrepareOutputProjectionWeight(runtime, wo, workload.headDim,
                                                  workload.modelWidth, 4701u),
                    "M47 corpus prepares Wo");
        std::vector<float> normWeight(workload.modelWidth);
        for (std::uint32_t column = 0u; column < workload.modelWidth; ++column)
            normWeight[column] = 0.75f + static_cast<float>(column % 17u) / 64.0f;
        prom_m46_weight_prepare_request prepareNorm{};
        prepareNorm.values = normWeight.data();
        prepareNorm.element_count = normWeight.size();
        prepareNorm.model_width = workload.modelWidth;
        prepareNorm.generation = 4702u;
        prom_m46_weight_prepare_result preparedNorm{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(runtime, &prepareNorm, &preparedNorm),
                     "M47 corpus prepares RMSNorm scale");
        std::array<std::vector<float>, PROM_M47_WEIGHT_COUNT> ffnWeights;
        ffnWeights[0].resize(static_cast<std::size_t>(workload.modelWidth) * workload.ffnWidth);
        ffnWeights[1].resize(ffnWeights[0].size());
        ffnWeights[2].resize(static_cast<std::size_t>(workload.ffnWidth) * workload.modelWidth);
        for (std::size_t index = 0u; index < ffnWeights[0].size(); ++index) {
            ffnWeights[0][index] = static_cast<float>(static_cast<int>((index * 7u + 3u) % 29u) - 14) / 1024.0f;
            ffnWeights[1][index] = static_cast<float>(static_cast<int>((index * 11u + 5u) % 31u) - 15) / 1024.0f;
        }
        for (std::size_t index = 0u; index < ffnWeights[2].size(); ++index)
            ffnWeights[2][index] = static_cast<float>(static_cast<int>((index * 13u + 7u) % 37u) - 18) / 1024.0f;
        std::uint64_t preparation = 0u;
        std::uint64_t preparationGpu = 0u;
        for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind) {
            prom_m47_weight_prepare_request prepare{};
            prepare.values = ffnWeights[kind].data();
            prepare.element_count = ffnWeights[kind].size();
            prepare.kind = kind;
            prepare.model_width = workload.modelWidth;
            prepare.ffn_width = workload.ffnWidth;
            prepare.generation = 4800u + kind;
            prom_m47_weight_prepare_result prepared{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_prepare_weight(runtime, &prepare, &prepared),
                         "M47 corpus prepares each FFN weight");
            preparation += prepared.preparation_ns;
            preparationGpu += prepared.gpu_upload_and_pack_ns;
        }

        std::vector<float> residentN(static_cast<std::size_t>(workload.tokens) * workload.modelWidth);
        prom_m46_composed_request nRequest{};
        FillM45ComposedRequest(&nRequest.upstream, nullptr, workload.tokens, workload.modelWidth,
                               workload.headDim, PROM_M45_STRATEGY_IN_PLACE_Y,
                               PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 4700u, 4701u);
        nRequest.output = residentN.data();
        nRequest.output_element_count = residentN.size();
        nRequest.epsilon = 1.0e-5f;
        nRequest.strategy = PROM_M46_STRATEGY_IN_PLACE_Z;
        nRequest.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
        nRequest.required_weight_generation = 4702u;
        prom_m46_composed_result nResult{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_execute_composed(runtime, &nRequest, &nResult),
                     "M47 corpus captures one audit N outside the timed product path");
        std::array<std::vector<float>, 2u> expected;
        std::array<std::uint64_t, 2u> hostCpu{};
        for (std::uint32_t precisionIndex = 0u; precisionIndex < 2u; ++precisionIndex) {
            std::vector<float> gate(static_cast<std::size_t>(workload.tokens) * workload.ffnWidth);
            std::vector<float> up(gate.size());
            std::vector<float> hidden(gate.size());
            std::vector<float> down(static_cast<std::size_t>(workload.tokens) * workload.modelWidth);
            expected[precisionIndex].resize(down.size());
            prom_m47_reference_request reference{};
            reference.n = residentN.data();
            reference.wgate = ffnWeights[0].data();
            reference.wup = ffnWeights[1].data();
            reference.wdown = ffnWeights[2].data();
            reference.gate = gate.data();
            reference.up = up.data();
            reference.hidden = hidden.data();
            reference.down = down.data();
            reference.output = expected[precisionIndex].data();
            reference.n_element_count = residentN.size();
            reference.wgate_element_count = ffnWeights[0].size();
            reference.wup_element_count = ffnWeights[1].size();
            reference.wdown_element_count = ffnWeights[2].size();
            reference.output_element_count = expected[precisionIndex].size();
            reference.tokens = workload.tokens;
            reference.model_width = workload.modelWidth;
            reference.ffn_width = workload.ffnWidth;
            reference.n_row_stride = workload.modelWidth;
            reference.output_row_stride = workload.modelWidth;
            reference.projection_path = precisionIndex == 0u
                                          ? PROM_M47_PROJECTION_A2X4_FP32
                                          : PROM_M47_PROJECTION_CONVENTIONAL_FP16;
            const auto cpuBegin = std::chrono::steady_clock::now();
            ASSERT_EQUAL(PROM_OK, prom_m47_gated_ffn_cpu_reference(&reference),
                         "M47 corpus CPU oracle succeeds");
            const auto cpuEnd = std::chrono::steady_clock::now();
            hostCpu[precisionIndex] = static_cast<std::uint64_t>(
                std::chrono::duration_cast<std::chrono::nanoseconds>(cpuEnd - cpuBegin).count());
        }
        for (const Strategy& strategy : strategies) {
            std::vector<float> output(expected[0].size());
            prom_m47_composed_request request{};
            FillM45ComposedRequest(&request.upstream.upstream, nullptr, workload.tokens,
                                   workload.modelWidth, workload.headDim,
                                   PROM_M45_STRATEGY_IN_PLACE_Y,
                                   PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, 4700u, 4701u);
            request.upstream.epsilon = 1.0e-5f;
            request.upstream.strategy = strategy.path == PROM_M47_PROJECTION_A2X4_FP32
                                          ? PROM_M46_STRATEGY_SEPARATE_OUTPUT
                                          : PROM_M46_STRATEGY_IN_PLACE_Z;
            request.upstream.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
            request.upstream.required_weight_generation = 4702u;
            request.output = output.data();
            request.output_element_count = output.size();
            request.ffn_width = workload.ffnWidth;
            request.projection_path = strategy.path;
            request.gating_strategy = strategy.gating;
            request.residual_strategy = strategy.residual;
            request.submit_policy = strategy.submit;
            for (std::uint32_t kind = 0u; kind < PROM_M47_WEIGHT_COUNT; ++kind)
                request.required_weight_generation[kind] = 4800u + kind;
            prom_m47_composed_result prime0{};
            prom_m47_composed_result prime1{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &prime0),
                         "M47 corpus primes the first slot");
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &prime1),
                         "M47 corpus primes the second slot");
            for (std::uint32_t warm = 0u; warm < 4u; ++warm) {
                prom_m47_composed_result ignored{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &ignored),
                             "M47 corpus warm execution succeeds");
            }
            std::vector<prom_m47_composed_result> measured;
            for (std::uint32_t sample = 0u; sample < 5u; ++sample) {
                prom_m47_composed_result value{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &value),
                             "M47 corpus measured execution succeeds");
                measured.push_back(value);
            }
            const prom_m47_composed_result& last = measured.back();
            prom_m47_mismatch mismatch{};
            const std::uint32_t precisionIndex = strategy.path == PROM_M47_PROJECTION_A2X4_FP32 ? 0u : 1u;
            const bool correct = prom_m47_gated_ffn_compare(
                expected[precisionIndex].data(), output.data(), workload.tokens, workload.modelWidth,
                workload.modelWidth, workload.modelWidth, 3.0e-3f, 4.0e-2f,
                &last.ffn_plan, nullptr, nullptr, nullptr, nullptr, &mismatch) == PROM_OK;
            ASSERT_TRUE(correct, "M47 corpus BlockOutput matches the precision oracle");
            ASSERT_EQUAL(prime1.buffer_allocation_count, last.buffer_allocation_count,
                         "M47 corpus performs no warm allocation");
            Record record{};
            record.workload = workload.name;
            record.strategy = strategy.name;
            record.path = strategy.path;
            record.gating = strategy.gating;
            record.residual = strategy.residual;
            record.submit = strategy.submit;
            record.tokens = workload.tokens;
            record.modelWidth = workload.modelWidth;
            record.ffnWidth = workload.ffnWidth;
            record.replay = last.ffn_plan.replay_id;
            record.outputGeneration = last.output_generation;
            record.nPack = MedianM47Metric(measured, &prom_m47_composed_result::n_pack_gpu_ns);
            record.gate = MedianM47Metric(measured, &prom_m47_composed_result::gate_projection_gpu_ns);
            record.up = MedianM47Metric(measured, &prom_m47_composed_result::up_projection_gpu_ns);
            record.activation = MedianM47Metric(measured, &prom_m47_composed_result::activation_gpu_ns);
            record.multiply = MedianM47Metric(measured, &prom_m47_composed_result::gating_multiply_gpu_ns);
            record.fused = MedianM47Metric(measured, &prom_m47_composed_result::fused_gating_gpu_ns);
            record.hiddenPack = MedianM47Metric(measured, &prom_m47_composed_result::hidden_pack_gpu_ns);
            record.down = MedianM47Metric(measured, &prom_m47_composed_result::down_projection_gpu_ns);
            record.residualGpu = MedianM47Metric(measured, &prom_m47_composed_result::residual_gpu_ns);
            record.m47 = MedianM47Metric(measured, &prom_m47_composed_result::m47_gpu_ns);
            record.complete = MedianM47Metric(measured,
                &prom_m47_composed_result::total_m43_m44_m45_m46_m47_gpu_ns);
            record.recording = MedianM47Metric(measured, &prom_m47_composed_result::cpu_recording_ns);
            record.submission = MedianM47Metric(measured, &prom_m47_composed_result::cpu_submission_ns);
            record.readback = MedianM47Metric(measured, &prom_m47_composed_result::final_readback_ns);
            record.endToEnd = MedianM47Metric(measured, &prom_m47_composed_result::end_to_end_ns);
            record.retained = last.retained_bytes;
            record.exact = last.exact_request_bytes;
            record.allocationCount = last.buffer_allocation_count;
            record.preparation = preparation;
            record.preparationGpu = preparationGpu;
            record.hostCpu = hostCpu[precisionIndex];
            record.hostEndToEnd = nResult.end_to_end_ns + hostCpu[precisionIndex];
            record.correct = correct;
            records.push_back(record);
            if (std::string_view(workload.name) == "primary" &&
                std::string_view(strategy.name) == "direct_packed_cooperative_split") {
                std::vector<prom_m47_composed_result> repeated10;
                std::vector<prom_m47_composed_result> repeated100;
                for (std::uint32_t repeat = 0u; repeat < 10u; ++repeat) {
                    prom_m47_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &value),
                                 "M47 primary 10-repeat succeeds");
                    repeated10.push_back(value);
                }
                for (std::uint32_t repeat = 0u; repeat < 100u; ++repeat) {
                    prom_m47_composed_result value{};
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m47_execute_composed(runtime, &request, &value),
                                 "M47 primary 100-repeat succeeds");
                    repeated100.push_back(value);
                }
                warm10M47 = MedianM47Metric(repeated10, &prom_m47_composed_result::m47_gpu_ns);
                warm10Complete = MedianM47Metric(
                    repeated10, &prom_m47_composed_result::total_m43_m44_m45_m46_m47_gpu_ns);
                warm10EndToEnd = MedianM47Metric(repeated10, &prom_m47_composed_result::end_to_end_ns);
                warm100M47 = MedianM47Metric(repeated100, &prom_m47_composed_result::m47_gpu_ns);
                warm100Complete = MedianM47Metric(
                    repeated100, &prom_m47_composed_result::total_m43_m44_m45_m46_m47_gpu_ns);
                warm100EndToEnd = MedianM47Metric(repeated100, &prom_m47_composed_result::end_to_end_ns);
            }
        }
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                     "M47 corpus validation services remain available");
        validationWarnings += services.validation_warning_count;
        validationErrors += services.validation_error_count;
        prom_reactor_runtime_destroy_impl(runtime);
    }
    ASSERT_EQUAL(0u, validationWarnings, "M47 corpus has zero validation warnings");
    ASSERT_EQUAL(0u, validationErrors, "M47 corpus has zero validation errors");
    std::ostringstream json;
    json << "{\n  \"schema\": \"prometheus.m47.gated-ffn-complete-block.v1\",\n"
         << "  \"validation\": {\"warnings\": " << validationWarnings
         << ", \"errors\": " << validationErrors << "},\n"
         << "  \"warmups_per_plan\": 4,\n  \"measurements_per_plan\": 5,\n"
         << "  \"primary_repeats\": {\"warm_10_m47_gpu_ns\": " << warm10M47
         << ", \"warm_10_complete_gpu_ns\": " << warm10Complete
         << ", \"warm_10_end_to_end_ns\": " << warm10EndToEnd
         << ", \"warm_100_m47_gpu_ns\": " << warm100M47
         << ", \"warm_100_complete_gpu_ns\": " << warm100Complete
         << ", \"warm_100_end_to_end_ns\": " << warm100EndToEnd << "},\n"
         << "  \"records\": [\n";
    for (std::size_t index = 0u; index < records.size(); ++index) {
        const Record& record = records[index];
        if (index != 0u) json << ",\n";
        json << "    {\"workload\":\"" << record.workload
             << "\",\"strategy\":\"" << record.strategy
             << "\",\"tokens\":" << record.tokens
             << ",\"model_width\":" << record.modelWidth
             << ",\"ffn_width\":" << record.ffnWidth
             << ",\"projection_path\":" << record.path
             << ",\"gating_strategy\":" << record.gating
             << ",\"residual_strategy\":" << record.residual
             << ",\"submit_policy\":" << record.submit
             << ",\"correct\":" << (record.correct ? "true" : "false")
             << ",\"replay_id\":" << record.replay
             << ",\"output_generation\":" << record.outputGeneration
             << ",\"n_pack_gpu_ns\":" << record.nPack
             << ",\"gate_projection_gpu_ns\":" << record.gate
             << ",\"up_projection_gpu_ns\":" << record.up
             << ",\"activation_gpu_ns\":" << record.activation
             << ",\"gating_multiply_gpu_ns\":" << record.multiply
             << ",\"fused_gating_gpu_ns\":" << record.fused
             << ",\"hidden_pack_gpu_ns\":" << record.hiddenPack
             << ",\"down_projection_gpu_ns\":" << record.down
             << ",\"residual_gpu_ns\":" << record.residualGpu
             << ",\"m47_gpu_ns\":" << record.m47
             << ",\"complete_gpu_ns\":" << record.complete
             << ",\"cpu_recording_ns\":" << record.recording
             << ",\"cpu_submission_ns\":" << record.submission
             << ",\"final_readback_ns\":" << record.readback
             << ",\"end_to_end_ns\":" << record.endToEnd
             << ",\"weight_preparation_ns\":" << record.preparation
             << ",\"weight_preparation_gpu_ns\":" << record.preparationGpu
             << ",\"host_cpu_ffn_ns\":" << record.hostCpu
             << ",\"host_bounce_lower_bound_ns\":" << record.hostEndToEnd
             << ",\"retained_bytes\":" << record.retained
             << ",\"exact_request_bytes\":" << record.exact
             << ",\"allocation_count\":" << record.allocationCount << "}";
    }
    json << "\n  ]\n}\n";
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m47_gated_ffn_complete_block.json", json.str()),
                "M47 benchmark artifact is written");
}
