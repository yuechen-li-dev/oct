#include "test_harness.h"

#include "../reactor_vulkan.h"
#include "reactor_test_numerics.h"

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

    prom_rmsnorm_plan separate{};
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
    prom_rmsnorm_plan forcedStaged{};
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
    prom_rmsnorm_plan inPlace{};
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
    prom_rmsnorm_plan replay{};
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_plan_build(&request, &replay),
                 "identical M46 planning repeats");
    ASSERT_EQUAL(inPlace.replay_id, replay.replay_id, "M46 replay identity is deterministic");

    request.model_width = 4096u;
    request.z_view.logical_columns = 4096u;
    request.z_view.row_stride_elements = 4096u;
    request.z_view.byte_length = static_cast<VkDeviceSize>(127u * 4096u * sizeof(float));
    prom_rmsnorm_plan staged{};
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
    prom_rmsnorm_plan plan{};
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

FACT(PrometheusM49aMatchedInputM46HardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr)
        SKIP("Vulkan runtime unavailable");
    constexpr std::uint32_t tokens = 9u;
    constexpr std::uint32_t modelWidth = 128u;
    constexpr std::uint32_t rowStride = 133u;
    constexpr std::uint64_t inputGeneration = 494601u;
    constexpr std::uint64_t weightGeneration = 494602u;
    std::vector<float> z(static_cast<std::size_t>(tokens) * rowStride, 0.0f);
    for (std::uint32_t token = 0u; token < tokens; ++token) {
        for (std::uint32_t channel = 0u; channel < modelWidth; ++channel) {
            const std::size_t index = static_cast<std::size_t>(token) * rowStride + channel;
            const float magnitude = channel % 17u == 0u ? 31.75f :
                static_cast<float>((channel * 29u + token * 11u) % 101u + 1u) / 257.0f;
            z[index] = ((channel + token) & 1u) == 0u ? magnitude : -magnitude;
        }
    }
    std::vector<float> weight(modelWidth);
    for (std::uint32_t channel = 0u; channel < modelWidth; ++channel)
        weight[channel] = 0.75f + static_cast<float>((channel * 7u) % 19u) / 32.0f;
    prom_m46_weight_prepare_request prepare{};
    prepare.values = weight.data();
    prepare.element_count = weight.size();
    prepare.model_width = modelWidth;
    prepare.generation = weightGeneration;
    prom_m46_weight_prepare_result prepared{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m46_prepare_weight(runtime, &prepare, &prepared),
                 "M49a M46 exact weight generation prepares");
    std::vector<float> expected(static_cast<std::size_t>(tokens) * modelWidth);
    std::vector<float> expectedInv(tokens);
    prom_m46_reference_request reference{};
    reference.z = z.data();
    reference.weight = weight.data();
    reference.n = expected.data();
    reference.inv_rms = expectedInv.data();
    reference.z_element_count = z.size();
    reference.weight_element_count = weight.size();
    reference.n_element_count = expected.size();
    reference.tokens = tokens;
    reference.model_width = modelWidth;
    reference.z_row_stride = rowStride;
    reference.n_row_stride = modelWidth;
    reference.epsilon = 1.0e-5f;
    ASSERT_EQUAL(PROM_OK, prom_m46_rmsnorm_cpu_reference(&reference),
                 "M49a M46 CPU FP32 authority evaluates");
    std::vector<float> actual(expected.size());
    std::vector<float> actualInv(tokens);
    prom_m49a_m46_request request{};
    request.matched_z = z.data();
    request.matched_storage_element_count = z.size();
    request.output = actual.data();
    request.output_element_count = actual.size();
    request.inv_rms_output = actualInv.data();
    request.inv_rms_output_element_count = actualInv.size();
    request.tokens = tokens;
    request.model_width = modelWidth;
    request.z_row_stride = rowStride;
    request.strategy = PROM_M46_STRATEGY_SEPARATE_OUTPUT;
    request.requested_reduction_plan = PROM_M46_REDUCTION_AUTO;
    request.epsilon = 1.0e-5f;
    request.input_generation = inputGeneration;
    request.reference_input_hash = prom_num_hash_float_bits(z.data(), z.size());
    request.required_weight_generation = weightGeneration;
    request.required_weight_hash = prepared.hash;
    request.exact_source_hash = 0x49a46001u;
    prom_m49a_m46_result result{};
    const int status = prom_reactor_runtime_m49a_execute_m46(runtime, &request, &result);
    if (status != PROM_OK)
        std::fprintf(stderr, "M49a M46 failure detail=%d stage=%u\n",
                     result.detail_code, result.stage);
    ASSERT_EQUAL(PROM_OK, status, "M49a exact matched-input M46 executes");
    ASSERT_EQUAL(1u, result.matched_input, "M46 exact input identity is enforced");
    ASSERT_EQUAL(1u, result.audit_only, "M46 matched-input owner is audit-only");
    ASSERT_EQUAL(0u, result.product_authority_changed,
                 "M46 matched-input owner cannot change product authority");
    std::vector<double> scratch(actual.size());
    prom_num_error_summary disturbance{};
    ASSERT_TRUE(prom_num_summarize_error(expected.data(), actual.data(), tokens,
                                         modelWidth, 1.0e-6, 1.0, 1.0,
                                         scratch.data(), scratch.size(),
                                         &disturbance) != 0,
                "M46 matched-input local disturbance summarizes");
    double maxInvRmsError = 0.0;
    for (std::size_t token = 0u; token < actualInv.size(); ++token)
        maxInvRmsError = std::max(maxInvRmsError,
                                  std::abs(static_cast<double>(actualInv[token]) -
                                           expectedInv[token]));
    std::fprintf(stderr,
                 "M49a M46 D_l1=%g D_l2=%g D_linf=%g mae=%g rms=%g bias=%g p95=%g p99=%g max_inv_rms_error=%g gpu_ns=%llu input_hash=%llu output_hash=%llu inv_rms_hash=%llu replay=%llu\n",
                 disturbance.l1_norm, disturbance.l2_norm,
                 disturbance.linfinity_norm, disturbance.mean_absolute_error,
                 disturbance.rms_error, disturbance.signed_mean_bias,
                 disturbance.p95_absolute_error, disturbance.p99_absolute_error,
                 maxInvRmsError,
                 static_cast<unsigned long long>(result.rmsnorm.m46_gpu_ns),
                 static_cast<unsigned long long>(result.input_hash),
                 static_cast<unsigned long long>(result.output_hash),
                 static_cast<unsigned long long>(result.inv_rms_hash),
                 static_cast<unsigned long long>(result.replay_identity));
    ASSERT_TRUE(disturbance.linfinity_norm < 1.0e-4 && maxInvRmsError < 1.0e-5,
                "M46 local disturbance remains bounded");
    std::vector<float> repeatedOutput(actual.size());
    std::vector<float> repeatedInv(tokens);
    request.output = repeatedOutput.data();
    request.inv_rms_output = repeatedInv.data();
    prom_m49a_m46_result repeated{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m49a_execute_m46(runtime, &request, &repeated),
                 "M49a M46 repeats");
    ASSERT_EQUAL(result.output_hash, repeated.output_hash,
                 "M49a M46 output hash repeats bitwise");
    ASSERT_EQUAL(result.inv_rms_hash, repeated.inv_rms_hash,
                 "M49a M46 inverse RMS hash repeats bitwise");
    ASSERT_EQUAL(result.replay_identity, repeated.replay_identity,
                 "M49a M46 replay identity repeats");
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services),
                 "M49a M46 validation services remain available");
    ASSERT_EQUAL(0u, services.validation_warning_count,
                 "M49a M46 is validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count,
                 "M49a M46 is validation-error clean");
    prom_reactor_runtime_destroy_impl(runtime);
}
