#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"
#include "../../models/zimage-turbo/resolved_audit_schedule.h"
#include "../../models/zimage-turbo/resolved_descriptor.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <chrono>
#include <cstring>
#include <cstdint>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <iostream>
#include <limits>
#include <numeric>
#include <sstream>
#include <vector>

namespace
{
constexpr std::uint32_t kElementCount = 1024u;
constexpr std::uint32_t kAuditElements = 64u;
constexpr std::uint32_t kWeightBytes = 64u;
constexpr std::array<std::uint64_t, PROM_MODEL_BLOCK_MAX_WEIGHTS> kM1BWeightBytes{
    30720u, 7864320u, 256u, 29491200u, 256u, 88473600u, 7680u,
    7680u, 78643200u, 78643200u, 78643200u, 7680u, 7680u};

PrometheusModelBlockCreateRequest make_request()
{
    PrometheusModelBlockCreateRequest request{};
    request.struct_size = sizeof(request);
    request.model_contract_identity = 0x101u;
    request.weight_identity = 0x102u;
    request.shader_portfolio_identity = 0x103u;
    request.precision_policy_identity = 0x104u;
    request.capability_route_identity = 0x105u;
    request.memory_ceiling_bytes = 1024u * 1024u;
    request.external_input_bytes = kElementCount * sizeof(float);
    request.external_output_bytes = kElementCount * sizeof(float);
    request.audit_bytes = kAuditElements * sizeof(float);
    request.shader_id = 23u;
    request.weight_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    for (std::uint32_t i = 0u; i < request.weight_count; ++i) {
        request.weights[i].content_identity = 0x200u + i;
        request.weights[i].layout_identity = 0x300u + i;
        request.weights[i].byte_count = kWeightBytes;
    }
    request.step_count = PROM_MODEL_BLOCK_MAX_STEPS;
    request.steps[0] = PROM_MODEL_BLOCK_STEP_BIND_PIPELINE;
    request.steps[1] = PROM_MODEL_BLOCK_STEP_BIND_RESOURCES;
    request.steps[2] = PROM_MODEL_BLOCK_STEP_PUSH_CONSTANTS;
    request.steps[3] = PROM_MODEL_BLOCK_STEP_DISPATCH;
    request.steps[4] = PROM_MODEL_BLOCK_STEP_BARRIER;
    request.steps[5] = PROM_MODEL_BLOCK_STEP_AUDIT_COPY;
    request.steps[6] = PROM_MODEL_BLOCK_STEP_OUTPUT_COPY;
    return request;
}

PrometheusModelBlockCreateRequest make_m1b_request()
{
    PrometheusModelBlockCreateRequest request = make_request();
    request.memory_ceiling_bytes = 512ull * 1024ull * 1024ull;
    request.external_input_bytes = 1024ull * 3840ull * sizeof(std::uint16_t);
    request.external_output_bytes = 0u;
    request.audit_bytes = 65536ull * sizeof(float);
    request.shader_id = 24u;
    request.assembly_family = PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO;
    request.parameter_set = PROM_NOISE_REFINER_PARAMETER_SET_0;
    request.parameter_set_aggregate_identity = 0xa1ba526898a2a752ull;
    for (std::uint32_t i = 0u; i < request.weight_count; ++i) {
        request.weights[i].content_identity = 0x5a00u + i;
        request.weights[i].layout_identity = 0x6b00u + i;
        request.weights[i].byte_count = kM1BWeightBytes[i];
    }
    return request;
}

bool runtime_available(void* runtime)
{
    PrometheusCaps caps{};
    return prometheus_reactor_runtime_probe(runtime, &caps) == PROM_OK && caps.available != 0u;
}

std::vector<std::uint8_t> read_binary_file(const std::filesystem::path& path)
{
    std::ifstream stream(path, std::ios::binary | std::ios::ate);
    if (!stream) return {};
    const std::streamsize size = stream.tellg();
    if (size <= 0) return {};
    std::vector<std::uint8_t> bytes(static_cast<std::size_t>(size));
    stream.seekg(0, std::ios::beg);
    if (!stream.read(reinterpret_cast<char*>(bytes.data()), size)) return {};
    return bytes;
}

struct ComparisonMetrics {
    double l2 = 0.0;
    double linf = 0.0;
    double relativeL2 = 0.0;
    std::uint64_t firstMismatch = 0u;
    bool finite = true;
};

ComparisonMetrics compare_float_region(const float* actual, const std::vector<std::uint8_t>& referenceBytes,
                                       std::uint64_t startElement, std::uint64_t elementCount)
{
    ComparisonMetrics metrics{};
    double errorSquares = 0.0;
    double referenceSquares = 0.0;
    metrics.firstMismatch = elementCount;
    if (referenceBytes.size() < static_cast<std::size_t>((startElement + elementCount) * sizeof(float))) {
        metrics.finite = false;
        metrics.firstMismatch = 0u;
        return metrics;
    }
    for (std::uint64_t offset = 0u; offset < elementCount; ++offset) {
        float reference = 0.0f;
        std::memcpy(&reference, referenceBytes.data() + static_cast<std::size_t>((startElement + offset) * sizeof(float)),
                    sizeof(reference));
        const float value = actual[startElement + offset];
        const double difference = static_cast<double>(value) - static_cast<double>(reference);
        metrics.finite = metrics.finite && std::isfinite(value) && std::isfinite(reference);
        errorSquares += difference * difference;
        referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
        metrics.linf = std::max(metrics.linf, std::fabs(difference));
        if (metrics.firstMismatch == elementCount && value != reference) metrics.firstMismatch = offset;
    }
    metrics.l2 = std::sqrt(errorSquares);
    metrics.relativeL2 = referenceSquares == 0.0 ? 0.0 : metrics.l2 / std::sqrt(referenceSquares);
    return metrics;
}

double mean_ns(const std::array<std::uint64_t, 10>& values)
{
    return static_cast<double>(std::accumulate(values.begin(), values.end(), std::uint64_t{0})) /
        static_cast<double>(values.size());
}

int upload_all(void* runtime, std::uint64_t block_id, const PrometheusModelBlockCreateRequest& request,
               PrometheusModelBlockEvidence* out_evidence)
{
    std::array<std::array<std::uint8_t, kWeightBytes>, PROM_MODEL_BLOCK_MAX_WEIGHTS> bytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> uploads{};
    for (std::uint32_t i = 0u; i < request.weight_count; ++i) {
        for (std::uint32_t byte = 0u; byte < kWeightBytes; ++byte) bytes[i][byte] = static_cast<std::uint8_t>((i * 17u + byte * 3u) & 0xffu);
        uploads[i].binding_index = i;
        uploads[i].bytes = bytes[i].data();
        uploads[i].byte_count = kWeightBytes;
        uploads[i].content_identity = request.weights[i].content_identity;
        uploads[i].layout_identity = request.weights[i].layout_identity;
    }
    return prometheus_reactor_runtime_model_block_upload_weights(
        runtime, block_id, uploads.data(), static_cast<std::uint32_t>(uploads.size()), out_evidence);
}
}

FACT(PrometheusResidentModelBlockExecutesAClosedGpuProgramAndStaysWarm)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creates for resident model block");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_request();
    std::uint64_t block_id = 0u;
    PrometheusModelBlockEvidence evidence{};
    const int create_result = prometheus_reactor_runtime_model_block_create(runtime, &create, &block_id, &evidence);
    ASSERT_EQUAL(PROM_OK, create_result, "closed resident command program creates");
    if (create_result != PROM_OK) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    ASSERT_TRUE(block_id != 0u, "resident block owner identity is nonzero");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_MAX_STEPS, evidence.fixed_step_count, "the plan has exactly seven closed steps");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_MAX_WEIGHTS, evidence.weight_count, "the ABI accepts the full M1 weight count");
    ASSERT_TRUE(evidence.cold_buffer_allocation_count >= 20u, "creation preallocates resident and reusable buffers");
    ASSERT_EQUAL(1u, evidence.pipeline_create_count, "pipeline is created once at cold start");

    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> malformed{};
    malformed[0].binding_index = 0u;
    malformed[0].byte_count = kWeightBytes;
    malformed[0].layout_identity = 0x300u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_upload_weights(runtime, block_id, malformed.data(), static_cast<std::uint32_t>(malformed.size()), &evidence), "wrong weight identity is rejected");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH, evidence.detail_code, "wrong hash/layout has exact fault identity");
    ASSERT_EQUAL(PROM_OK, upload_all(runtime, block_id, create, &evidence), "all declared immutable weights upload resident");
    ASSERT_EQUAL(1u, evidence.weights_uploaded, "complete resident bundle is marked immutable only after upload");
    ASSERT_EQUAL(create.weight_count, static_cast<std::uint32_t>(evidence.weight_upload_count), "each declared weight is uploaded exactly once");
    std::vector<float> input(kElementCount), output(kElementCount, 0.0f), audit(kAuditElements, 0.0f);
    for (std::uint32_t i = 0u; i < kElementCount; ++i) input[i] = static_cast<float>(static_cast<int>((i * 19u) % 113u) - 56) / 32.0f;
    PrometheusModelBlockExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.input = input.data();
    execute.output = output.data();
    execute.element_count = input.size();
    execute.input_identity = 0x8abcu;
    execute.audit_enabled = 1u;
    execute.audit_output = audit.data();
    execute.audit_element_capacity = audit.size();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "resident shader chain executes on GPU");
    ASSERT_EQUAL(1u, evidence.output_valid, "output is accepted after completion");
    ASSERT_EQUAL(1u, evidence.audit_valid, "declared bounded audit is captured");
    const std::uint64_t replay = evidence.replay_identity;
    const std::uint64_t allocations = evidence.cold_buffer_allocation_count;
    const std::uint64_t uploads = evidence.weight_upload_count;
    for (std::uint32_t i = 0u; i < kElementCount; ++i) ASSERT_EQUAL(input[i], output[i], "resident identity shader preserves output");
    for (std::uint32_t i = 0u; i < kAuditElements; ++i) ASSERT_EQUAL(input[i], audit[i], "audit reads only declared bounded projection");
    for (std::uint32_t iteration = 0u; iteration < 10u; ++iteration) {
        std::fill(output.begin(), output.end(), 0.0f);
        std::fill(audit.begin(), audit.end(), 0.0f);
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "warm execution succeeds");
        ASSERT_EQUAL(allocations, evidence.cold_buffer_allocation_count, "warm execution allocates no Vulkan buffers");
        ASSERT_EQUAL(0u, evidence.warm_buffer_allocation_count, "warm Vulkan allocation counter remains zero");
        ASSERT_EQUAL(uploads, evidence.weight_upload_count, "warm execution never reuploads weights");
        ASSERT_EQUAL(1u, evidence.pipeline_create_count, "warm execution never creates a pipeline");
        ASSERT_EQUAL(replay, evidence.replay_identity, "identical input/audit configuration has stable replay identity");
    }

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_impl(
                              runtime, block_id, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION),
                 "resident owner routes lifecycle fault injection through the established reactor state");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "uncertain completion quarantines block resources");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN, evidence.detail_code, "uncertain completion carries exact fault identity");
    ASSERT_EQUAL(1u, evidence.quarantined, "no stale output is accepted while quarantined");
    ASSERT_EQUAL(0u, evidence.output_valid, "quarantined execution marks output invalid");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "next execution reaps completed quarantine and recovers");
    ASSERT_EQUAL(0u, evidence.quarantined, "recovery reaps the resident resource");

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_impl(
                              runtime, block_id, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT),
                 "command submission fault is injected through the centralized reactor state");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "command submission failure rejects stale output");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_QUEUE_SUBMIT_FAILED, evidence.detail_code, "queue submission fault has exact identity");
    ASSERT_EQUAL(0u, evidence.output_valid, "queue submission failure does not accept stale output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "one-shot command submission fault recovers on the next execution");

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_execution_fault_impl(
                              runtime, block_id, PROM_TESTCFG_FAIL_DOWNLOAD),
                 "audit readback fault is injected only into the resident block");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_execute(runtime, block_id, &execute, &evidence), "audit readback failure rejects output");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED, evidence.detail_code, "audit failure has exact identity");
    ASSERT_EQUAL(0u, evidence.output_valid, "audit failure does not accept stale output");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, block_id), "destroy after execution is safe");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_destroy(runtime, block_id), "repeated destroy cannot double release");
    std::uint64_t recreated = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &recreated, &evidence), "a later resident block can create after destruction");
    ASSERT_TRUE(recreated != block_id, "owner identity does not recycle after destruction");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, recreated), "destroy before execution is safe");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "runtime teardown reclaims model-block resources");
}

FACT(PrometheusM1BPreAttentionOwnerCreatesIngressAndFiveModelPipelines)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creates for M1B owner preflight");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_m1b_request();
    std::uint64_t blockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_create_fault_impl(
                              runtime, PROM_TESTCFG_FAIL_PIPELINE_CREATE),
                 "next M1B creation receives the ingress pipeline fault");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence),
                 "ingress pipeline creation failure cleans partial M1B ownership");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_INGRESS_PIPELINE_CREATE_FAILED, evidence.detail_code,
                 "ingress pipeline creation failure has exact fault identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence),
                 "resident owner creates the fixed M1B through M1D shader portfolio");
    if (blockID == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    ASSERT_EQUAL(PROM_MODEL_BLOCK_MAX_WEIGHTS, evidence.weight_count, "M1B declares all thirteen immutable weights");
    ASSERT_EQUAL(26u, evidence.pipeline_create_count,
                 "cold creation owns 13 model pipelines plus 13 immutable audit-source views");
    ASSERT_EQUAL(26u, evidence.descriptor_set_count,
                 "cold creation owns every model and audit descriptor set before execution");
    ASSERT_TRUE(evidence.persistent_bytes == 361820672u, "M1B immutable arena has the accepted cache byte count");
    ASSERT_TRUE(evidence.cold_buffer_allocation_count >= 26u, "M1B preallocates all resident and bounded-audit buffers");
    ASSERT_EQUAL(0u, evidence.weight_upload_count, "creation does not permit an implicit warm upload");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID),
                 "M1B preflight resources destroy safely before upload");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "M1B preflight runtime destroys safely");
}

FACT(PrometheusM1BBf16IngressWidensEveryBitPatternExactlyOnGpu)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creates for exhaustive BF16 ingress");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_m1b_request();
    std::uint64_t blockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence),
                 "fixed production ingress pipeline creates with the M1B owner");
    if (blockID == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    std::vector<std::uint16_t> input(65536u);
    std::vector<float> output(input.size(), 0.0f);
    for (std::uint32_t bits = 0u; bits < input.size(); ++bits) input[bits] = static_cast<std::uint16_t>(bits);
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_bf16_ingress_impl(
                              runtime, blockID, input.data(), static_cast<std::uint32_t>(input.size()),
                              output.data(), static_cast<std::uint32_t>(output.size()), &evidence),
                 "production ingress widens the exhaustive BF16 domain on GPU");
    std::uint32_t firstMismatch = static_cast<std::uint32_t>(input.size());
    for (std::uint32_t bits = 0u; bits < input.size(); ++bits) {
        std::uint32_t actual = 0u;
        std::memcpy(&actual, &output[bits], sizeof(actual));
        if (actual != (bits << 16u)) {
            firstMismatch = bits;
            break;
        }
    }
    ASSERT_EQUAL(static_cast<std::uint32_t>(input.size()), firstMismatch,
                 "all 65536 GPU results have the exact canonical FP32 bit pattern");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_execution_fault_impl(
                              runtime, blockID, PROM_TESTCFG_FAIL_DISPATCH),
                 "ingress dispatch fault is injected into the resident block");
    ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_model_block_test_bf16_ingress_impl(
                                 runtime, blockID, input.data(), static_cast<std::uint32_t>(input.size()),
                                 output.data(), static_cast<std::uint32_t>(output.size()), &evidence),
                 "ingress dispatch failure rejects the audit output");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED, evidence.detail_code,
                 "ingress dispatch failure has exact fault identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID),
                 "exhaustive ingress resources destroy safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "exhaustive ingress runtime destroys safely");
}

FACT(PrometheusM1BIngressRejectsWrongExternalByteCounts)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creates for ingress ABI faults");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_m1b_request();
    std::uint64_t blockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence),
                 "M1B owner creates for exact ingress size faults");
    std::array<std::uint16_t, 1> input{};
    std::array<std::uint16_t, 1> timestep{};
    PrometheusModelBlockM1BExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.model_input_bf16 = input.data();
    execute.timestep_bf16 = timestep.data();
    execute.model_input_bytes = 1u;
    execute.timestep_bytes = 512u;
    execute.input_identity = 1u;
    execute.timestep_identity = 2u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                 "wrong BF16 model input byte count is rejected before execution");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_INGRESS_INPUT_SIZE_MISMATCH, evidence.detail_code,
                 "wrong input byte count has exact fault identity");
    execute.model_input_bytes = 1024ull * 3840ull * sizeof(std::uint16_t);
    execute.timestep_bytes = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                 "wrong BF16 timestep byte count is rejected before execution");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_INGRESS_TIMESTEP_SIZE_MISMATCH, evidence.detail_code,
                 "wrong timestep byte count has exact fault identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID), "ingress ABI fault block destroys safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "ingress ABI fault runtime destroys safely");
}

FACT(PrometheusContextRefinerRealPayloadExecutesTheClosedResidentBlock)
{
    const char* enabled = std::getenv("OCT_EVT2_M2B_REAL");
    const char* cacheRootText = std::getenv("OCT_EVT2_CACHE");
    if (enabled == nullptr || std::string(enabled) != "1" || cacheRootText == nullptr) {
        SKIP("set OCT_EVT2_M2B_REAL=1 and OCT_EVT2_CACHE for the ContextRefiner hardware lane");
    }
    const std::filesystem::path cacheRoot(cacheRootText);
    const std::filesystem::path layer = cacheRoot / "layers" /
        "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6" / "context_refiner.0";
    const std::filesystem::path canonical = cacheRoot / "canonical" /
        "f332072aa78be7aecdf3ee76d5c247082da564a6" / "m2b-fp32-reference" / "context_refiner.0";
    constexpr std::array<const char*, 11> names{{
        "context_refiner.0.attention.k_norm.weight.fp16.bin", "context_refiner.0.attention.out.weight.fp16.bin",
        "context_refiner.0.attention.q_norm.weight.fp16.bin", "context_refiner.0.attention.qkv.weight.fp16.bin",
        "context_refiner.0.attention_norm1.weight.fp16.bin", "context_refiner.0.attention_norm2.weight.fp16.bin",
        "context_refiner.0.feed_forward.w1.weight.fp16.bin", "context_refiner.0.feed_forward.w2.weight.fp16.bin",
        "context_refiner.0.feed_forward.w3.weight.fp16.bin", "context_refiner.0.ffn_norm1.weight.fp16.bin",
        "context_refiner.0.ffn_norm2.weight.fp16.bin"}};
    constexpr std::array<std::uint64_t, 11> byteCounts{{256u,29491200u,256u,88473600u,7680u,7680u,78643200u,78643200u,78643200u,7680u,7680u}};
    std::array<std::vector<std::uint8_t>, 11> weightBytes{};
    std::array<PrometheusModelBlockWeightUpload, 11> uploads{};
    for (std::uint32_t index = 0u; index < names.size(); ++index) {
        weightBytes[index] = read_binary_file(layer / names[index]);
        ASSERT_EQUAL(byteCounts[index], static_cast<std::uint64_t>(weightBytes[index].size()), "ContextRefiner cache tensor has its declared physical size");
        uploads[index].binding_index = index;
        uploads[index].bytes = weightBytes[index].data();
        uploads[index].byte_count = weightBytes[index].size();
        uploads[index].content_identity = 0x7100u + index;
        uploads[index].layout_identity = 0x7200u + index;
    }
    const std::vector<std::uint8_t> inputBytes = read_binary_file(canonical / "input.f32.bin");
    const std::vector<std::uint8_t> referenceBytes = read_binary_file(canonical / "final_output.f32.bin");
    ASSERT_EQUAL(32u * 3840u * sizeof(float), static_cast<std::uint64_t>(inputBytes.size()), "ContextEmbedding input has the fixed FP32 ABI");
    ASSERT_EQUAL(inputBytes.size(), referenceBytes.size(), "ContextRefiner canonical final has the fixed FP32 ABI");
    if (inputBytes.size() != 32u * 3840u * sizeof(float) || referenceBytes.size() != inputBytes.size()) return;
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "ContextRefiner runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    PrometheusContextRefinerCreateRequest create{};
    create.struct_size = sizeof(create);
    create.model_local_block_id = 0u;
    create.lock_identity = PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID;
    create.upload_count = static_cast<std::uint32_t>(uploads.size());
    create.uploads = uploads.data();
    std::uint64_t blockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_create(runtime, &create, &blockID, &evidence), "ContextRefiner0 creates from its lock-resolved closed bundle");
    if (blockID == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    PrometheusContextRefiner0ExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.context_input = reinterpret_cast<const float*>(inputBytes.data());
    execute.context_input_bytes = inputBytes.size();
    execute.input_identity = 0xf6e4a2842dbbdfa7ull;
    execute.output_identity = 0xd2b8167de614da25ull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner0_execute(runtime, blockID, &execute, &evidence), "ContextRefiner0 executes as one resident FP32 command sequence");
    const std::uint64_t context0FirstExecutionNs = evidence.last_execution_ns;
    const std::uint64_t context0Allocations = evidence.warm_buffer_allocation_count;
    const std::uint64_t context0Uploads = evidence.weight_upload_count;
    const std::uint64_t context0Pipelines = evidence.pipeline_create_count;
    const std::uint64_t context0Descriptors = evidence.descriptor_set_count;
    std::array<std::uint64_t, 10> context0WarmNs{};
    for (std::uint32_t iteration = 0u; iteration < context0WarmNs.size(); ++iteration) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner0_execute(runtime, blockID, &execute, &evidence),
                     "ContextRefiner0 warm execution retains its closed FP32 ingress contract");
        context0WarmNs[iteration] = evidence.last_execution_ns;
    }
    ASSERT_EQUAL(context0Allocations, evidence.warm_buffer_allocation_count, "ContextRefiner0 warm executions allocate no buffers");
    ASSERT_EQUAL(context0Uploads, evidence.weight_upload_count, "ContextRefiner0 warm executions upload no weights");
    ASSERT_EQUAL(context0Pipelines, evidence.pipeline_create_count, "ContextRefiner0 warm executions create no pipelines");
    ASSERT_EQUAL(context0Descriptors, evidence.descriptor_set_count, "ContextRefiner0 warm executions grow no descriptor pool");
    const std::uint64_t firstGeneration = evidence.output_generation;
    std::vector<std::uint32_t> staticAudit(PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_ARENA_BYTES / sizeof(std::uint32_t));
    PrometheusContextRefinerStaticAuditRequest staticRequest{};
    staticRequest.struct_size = sizeof(staticRequest);
    staticRequest.lock_identity = PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID;
    staticRequest.input_generation = firstGeneration;
    staticRequest.output_identity = 0xd2b8167de614da25ull;
    staticRequest.audit_arena = staticAudit.data();
    staticRequest.audit_arena_capacity_bytes = staticAudit.size() * sizeof(std::uint32_t);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_execute_static_audit(runtime, blockID, &staticRequest, &evidence), "ContextRefiner static audit captures every lock-declared persistent stage in one execution");
    constexpr std::array<const char*, PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_STAGE_COUNT> stageNames{{
        "context_embedding_input", "attention_norm", "qkv", "q_norm", "k_norm", "q_rope", "k_rope",
        "attention_aggregation", "attention_projection", "attention_residual", "ffn_norm", "w1", "w3",
        "ffn_gated_hidden", "w2", "final_output"}};
    for (std::uint32_t stage = 0u; stage < stageNames.size(); ++stage) {
        const auto& entry = k_prom_zimage_turbo_context_audit_schedule[stage];
        const std::vector<std::uint8_t> stageBytes = stage == 0u
            ? read_binary_file(canonical / "input.f32.bin")
            : read_binary_file(canonical / "stages" / (std::string(stageNames[stage]) + ".f32.bin"));
        ASSERT_EQUAL(static_cast<std::uint64_t>(entry.element_count) * sizeof(float), static_cast<std::uint64_t>(stageBytes.size()), "ContextRefiner canonical stage has the scheduled extent");
        double maximumDifference = 0.0;
        const std::uint32_t words = entry.audit_destination_offset / sizeof(std::uint32_t);
        for (std::uint32_t projection = 0u; projection < entry.projection_key_count; ++projection) {
            const std::uint32_t key = staticAudit[words + 16u + projection * 2u];
            float actual = 0.0f, reference = 0.0f;
            std::memcpy(&actual, &staticAudit[words + 17u + projection * 2u], sizeof(actual));
            std::memcpy(&reference, stageBytes.data() + key * sizeof(float), sizeof(reference));
            maximumDifference = std::max(maximumDifference, std::fabs(static_cast<double>(actual) - static_cast<double>(reference)));
        }
        std::cout << "M2B stage=" << stageNames[stage] << " projection_linf=" << maximumDifference << "\n";
        ASSERT_TRUE(std::isfinite(maximumDifference) && maximumDifference <= 1.0e-4,
                    "ContextRefiner static-audit projection matches its canonical stage without a broad tolerance");
    }
    std::vector<float> output(32u * 3840u);
    PrometheusContextRefinerFinalAuditRequest audit{};
    audit.struct_size = sizeof(audit);
    audit.required_output_generation = evidence.output_generation;
    audit.output_identity = execute.output_identity;
    audit.output = output.data();
    audit.output_element_capacity = output.size();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_audit_final(runtime, blockID, &audit, &evidence), "ContextRefiner final audit is an explicit post-completion egress");
    float finiteMaximum = 0.0f;
    double errorSquares = 0.0, referenceSquares = 0.0, linf = 0.0;
    for (std::size_t index = 0u; index < output.size(); ++index) {
        float reference = 0.0f;
        std::memcpy(&reference, referenceBytes.data() + index * sizeof(float), sizeof(reference));
        const double difference = static_cast<double>(output[index]) - static_cast<double>(reference);
        finiteMaximum = std::max(finiteMaximum, std::fabs(output[index]));
        errorSquares += difference * difference;
        referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
        linf = std::max(linf, std::fabs(difference));
    }
    const double relativeL2 = std::sqrt(errorSquares / referenceSquares);
    ASSERT_TRUE(std::isfinite(finiteMaximum), "ContextRefiner0 final output is finite");
    ASSERT_TRUE(std::isfinite(relativeL2) && std::isfinite(linf), "ContextRefiner0 final comparison is finite");
    std::cout << "M2B ContextRefiner0 final_abs_max=" << finiteMaximum << " relative_l2=" << relativeL2
              << " linf=" << linf << " execution_ns=" << evidence.last_execution_ns << "\n";
    ASSERT_TRUE(relativeL2 <= 1.0e-5 && linf <= 1.0e-3, "ContextRefiner0 final stays within the accepted FP32 numerical bound");
    const std::uint64_t context0ResidentGeneration = evidence.output_generation;
    const std::filesystem::path layer1 = cacheRoot / "layers" /
        "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6" / "context_refiner.1";
    const std::filesystem::path canonical1 = cacheRoot / "canonical" /
        "f332072aa78be7aecdf3ee76d5c247082da564a6" / "m2b-fp32-reference" / "context_refiner.1";
    std::array<std::vector<std::uint8_t>, 11> weightBytes1{};
    std::array<PrometheusModelBlockWeightUpload, 11> uploads1{};
    for (std::uint32_t index = 0u; index < names.size(); ++index) {
        std::string name(names[index]);
        name.replace(0u, std::string("context_refiner.0").size(), "context_refiner.1");
        weightBytes1[index] = read_binary_file(layer1 / name);
        ASSERT_EQUAL(byteCounts[index], static_cast<std::uint64_t>(weightBytes1[index].size()), "ContextRefiner1 cache tensor has its declared physical size");
        uploads1[index].binding_index = index;
        uploads1[index].bytes = weightBytes1[index].data();
        uploads1[index].byte_count = weightBytes1[index].size();
        uploads1[index].content_identity = 0x7300u + index;
        uploads1[index].layout_identity = 0x7400u + index;
    }
    PrometheusContextRefinerRebindRequest rebind{};
    rebind.struct_size = sizeof(rebind);
    rebind.model_local_block_id = 1u;
    rebind.lock_identity = PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID;
    rebind.upload_count = static_cast<std::uint32_t>(uploads1.size());
    rebind.uploads = uploads1.data();
    PrometheusContextRefinerRebindRequest invalidRebind = rebind;
    invalidRebind.lock_identity ^= 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_context_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                 "ContextRefiner rebind rejects a stale or foreign lock before resident mutation");
    invalidRebind = rebind;
    invalidRebind.model_local_block_id = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_context_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                 "ContextRefiner rebind rejects the illegal ContextRefiner0 to ContextRefiner0 transition");
    invalidRebind = rebind;
    invalidRebind.upload_count -= 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_context_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                 "ContextRefiner rebind rejects a partial immutable ContextRefiner1 package");
    const auto contextRebindBegin = std::chrono::steady_clock::now();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_rebind(runtime, blockID, &rebind, &evidence), "ContextRefiner0 weights atomically rebind to ContextRefiner1");
    const std::uint64_t contextRebindNs = static_cast<std::uint64_t>(
        std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - contextRebindBegin).count());
    PrometheusContextRefinerResidentExecuteRequest execute1{};
    execute1.struct_size = sizeof(execute1);
    execute1.input_generation = context0ResidentGeneration;
    execute1.output_identity = 0x08377e8a46b65cffull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_execute_resident(runtime, blockID, &execute1, &evidence), "ContextRefiner1 consumes the resident FP32 ContextRefiner0 boundary");
    const std::uint64_t context1FirstExecutionNs = evidence.last_execution_ns;
    const std::uint64_t context1Allocations = evidence.warm_buffer_allocation_count;
    const std::uint64_t context1Uploads = evidence.weight_upload_count;
    const std::uint64_t context1Pipelines = evidence.pipeline_create_count;
    const std::uint64_t context1Descriptors = evidence.descriptor_set_count;
    std::array<std::uint64_t, 10> context1WarmNs{};
    for (std::uint32_t iteration = 0u; iteration < context1WarmNs.size(); ++iteration) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_execute_resident(runtime, blockID, &execute1, &evidence),
                     "ContextRefiner1 warm execution reuses the immutable resident ContextRefiner0 boundary");
        context1WarmNs[iteration] = evidence.last_execution_ns;
    }
    ASSERT_EQUAL(context1Allocations, evidence.warm_buffer_allocation_count, "ContextRefiner1 warm executions allocate no buffers");
    ASSERT_EQUAL(context1Uploads, evidence.weight_upload_count, "ContextRefiner1 warm executions upload no weights");
    ASSERT_EQUAL(context1Pipelines, evidence.pipeline_create_count, "ContextRefiner1 warm executions create no pipelines");
    ASSERT_EQUAL(context1Descriptors, evidence.descriptor_set_count, "ContextRefiner1 warm executions grow no descriptor pool");
    const std::vector<std::uint8_t> reference1Bytes = read_binary_file(canonical1 / "final_output.f32.bin");
    ASSERT_EQUAL(referenceBytes.size(), reference1Bytes.size(), "ContextRefiner1 canonical final has the fixed FP32 ABI");
    staticRequest.input_generation = context0ResidentGeneration;
    staticRequest.output_identity = execute1.output_identity;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_execute_static_audit(runtime, blockID, &staticRequest, &evidence),
                 "ContextRefiner1 static audit reuses the immutable ContextRefiner0 resident boundary");
    for (std::uint32_t stage = 0u; stage < stageNames.size(); ++stage) {
        const auto& entry = k_prom_zimage_turbo_context_audit_schedule[stage];
        const std::vector<std::uint8_t> stageBytes = stage == 0u
            ? read_binary_file(canonical / "stages" / "final_output.f32.bin")
            : read_binary_file(canonical1 / "stages" / (std::string(stageNames[stage]) + ".f32.bin"));
        double maximumDifference = 0.0;
        const std::uint32_t words = entry.audit_destination_offset / sizeof(std::uint32_t);
        for (std::uint32_t projection = 0u; projection < entry.projection_key_count; ++projection) {
            const std::uint32_t key = staticAudit[words + 16u + projection * 2u];
            float actual = 0.0f, reference = 0.0f;
            std::memcpy(&actual, &staticAudit[words + 17u + projection * 2u], sizeof(actual));
            std::memcpy(&reference, stageBytes.data() + key * sizeof(float), sizeof(reference));
            maximumDifference = std::max(maximumDifference, std::fabs(static_cast<double>(actual) - static_cast<double>(reference)));
        }
        std::cout << "M2B context1_static_stage=" << stageNames[stage] << " projection_linf=" << maximumDifference << "\n";
        ASSERT_TRUE(std::isfinite(maximumDifference) && maximumDifference <= 1.0e-4,
                    "ContextRefiner1 static-audit projection matches its canonical stage without a broad tolerance");
    }
    PrometheusContextRefinerFinalAuditRequest audit1{};
    audit1.struct_size = sizeof(audit1);
    audit1.required_output_generation = evidence.output_generation;
    audit1.output_identity = execute1.output_identity;
    audit1.output = output.data();
    audit1.output_element_capacity = output.size();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_audit_final(runtime, blockID, &audit1, &evidence), "ContextRefiner1 final audit is an explicit post-completion egress");
    double errorSquares1 = 0.0, referenceSquares1 = 0.0, linf1 = 0.0;
    for (std::size_t index = 0u; index < output.size(); ++index) {
        float reference = 0.0f;
        std::memcpy(&reference, reference1Bytes.data() + index * sizeof(float), sizeof(reference));
        const double difference = static_cast<double>(output[index]) - static_cast<double>(reference);
        errorSquares1 += difference * difference;
        referenceSquares1 += static_cast<double>(reference) * static_cast<double>(reference);
        linf1 = std::max(linf1, std::fabs(difference));
    }
    const double relativeL21 = std::sqrt(errorSquares1 / referenceSquares1);
    std::cout << "M2B ContextRefiner1 relative_l2=" << relativeL21 << " linf=" << linf1
              << " execution_ns=" << evidence.last_execution_ns << "\n";
    std::cout << "M2B timing context0_first_ns=" << context0FirstExecutionNs << " context0_warm_ns=";
    for (std::size_t index = 0u; index < context0WarmNs.size(); ++index) {
        if (index != 0u) std::cout << ',';
        std::cout << context0WarmNs[index];
    }
    std::cout << " rebind_ns=" << contextRebindNs << " context1_first_ns=" << context1FirstExecutionNs
              << " context1_warm_ns=";
    for (std::size_t index = 0u; index < context1WarmNs.size(); ++index) {
        if (index != 0u) std::cout << ',';
        std::cout << context1WarmNs[index];
    }
    std::cout << " warm_churn=zero\n";
    ASSERT_TRUE(std::isfinite(relativeL21) && relativeL21 <= 1.0e-5 && linf1 <= 1.0e-3,
                "ContextRefiner1 resident chain output stays within the accepted FP32 numerical bound");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID), "ContextRefiner block destroys safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "ContextRefiner runtime destroys safely");
}

FACT(PrometheusM1BRealPayloadReachesTheFirstCanonicalModelWitness)
{
    const char* enabled = std::getenv("OCT_EVT2_M1B_REAL");
    const char* cacheRootText = std::getenv("OCT_EVT2_CACHE");
    const char* oracleRootText = std::getenv("OCT_EVT2_ORACLE");
    if (enabled == nullptr || std::string(enabled) != "1" || cacheRootText == nullptr || oracleRootText == nullptr) {
        SKIP("set OCT_EVT2_M1B_REAL=1 and the validated payload roots for the real hardware lane");
    }
    const std::filesystem::path cacheBlock = std::filesystem::path(cacheRootText) / "layers" /
        "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6" / "noise_refiner.0";
    const std::filesystem::path oracleRun = std::filesystem::path(oracleRootText) / "run_02";
    constexpr std::array<const char*, PROM_MODEL_BLOCK_MAX_WEIGHTS> names{
        "noise_refiner.0.adaLN_modulation.0.bias.fp16.bin",
        "noise_refiner.0.adaLN_modulation.0.weight.fp16.bin",
        "noise_refiner.0.attention.k_norm.weight.fp16.bin",
        "noise_refiner.0.attention.out.weight.fp16.bin",
        "noise_refiner.0.attention.q_norm.weight.fp16.bin",
        "noise_refiner.0.attention.qkv.weight.fp16.bin",
        "noise_refiner.0.attention_norm1.weight.fp16.bin",
        "noise_refiner.0.attention_norm2.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w1.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w2.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w3.weight.fp16.bin",
        "noise_refiner.0.ffn_norm1.weight.fp16.bin",
        "noise_refiner.0.ffn_norm2.weight.fp16.bin"};
    std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> weightBytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> uploads{};
    for (std::uint32_t index = 0u; index < names.size(); ++index) {
        weightBytes[index] = read_binary_file(cacheBlock / names[index]);
        ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(weightBytes[index].size()),
                     "validated cache tensor retains its exact physical byte count");
    }
    const std::vector<std::uint8_t> input = read_binary_file(oracleRun / "noise_refiner_0_input.bin");
    const std::vector<std::uint8_t> timestep = read_binary_file(oracleRun / "noise_refiner_0_timestep.bin");
    ASSERT_EQUAL(7864320u, static_cast<std::uint64_t>(input.size()), "real captured BF16 input is complete");
    ASSERT_EQUAL(512u, static_cast<std::uint64_t>(timestep.size()), "real captured BF16 timestep is complete");
    if (input.size() != 7864320u || timestep.size() != 512u) return;

    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "clean Vulkan runtime creates for the real M1B pass");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    PrometheusModelBlockCreateRequest create = make_m1b_request();
    create.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
    std::uint64_t blockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence),
                 "real M1B resident owner creates its complete fixed portfolio");
    if (blockID == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    std::vector<float> widened(1024u * 3840u);
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_bf16_ingress_impl(
                              runtime, blockID, reinterpret_cast<const std::uint16_t*>(input.data()),
                              static_cast<std::uint32_t>(widened.size()), widened.data(),
                              static_cast<std::uint32_t>(widened.size()), &evidence),
                 "full real captured input widens on GPU before model arithmetic");
    std::uint32_t ingressMismatch = static_cast<std::uint32_t>(widened.size());
    for (std::uint32_t index = 0u; index < widened.size(); ++index) {
        const std::uint16_t bf16 = static_cast<std::uint16_t>(input[index * 2u]) |
                                   static_cast<std::uint16_t>(input[index * 2u + 1u] << 8u);
        std::uint32_t actual = 0u;
        std::memcpy(&actual, &widened[index], sizeof(actual));
        if (actual != (static_cast<std::uint32_t>(bf16) << 16u)) {
            ingressMismatch = index;
            break;
        }
    }
    ASSERT_EQUAL(static_cast<std::uint32_t>(widened.size()), ingressMismatch,
                 "full real input ingress has exact FP32 bit identity");
    for (std::uint32_t index = 0u; index < uploads.size(); ++index) {
        uploads[index].binding_index = index;
        uploads[index].bytes = weightBytes[index].data();
        uploads[index].byte_count = weightBytes[index].size();
        uploads[index].content_identity = create.weights[index].content_identity;
        uploads[index].layout_identity = create.weights[index].layout_identity;
    }
    const auto uploadBegin = std::chrono::steady_clock::now();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_upload_weights(
                              runtime, blockID, uploads.data(), static_cast<std::uint32_t>(uploads.size()), &evidence),
                 "all thirteen validated FP16 tensors upload exactly once");
    const auto uploadEnd = std::chrono::steady_clock::now();
    const std::uint64_t uploadNs = static_cast<std::uint64_t>(
        std::chrono::duration_cast<std::chrono::nanoseconds>(uploadEnd - uploadBegin).count());
    std::vector<float> audit(1024u * 11520u);
    PrometheusModelBlockM1BExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.model_input_bf16 = input.data();
    execute.timestep_bf16 = timestep.data();
    execute.model_input_bytes = input.size();
    execute.timestep_bytes = timestep.size();
    execute.input_identity = 0x857cea75e69d665cull;
    execute.timestep_identity = 0xbc0ba90e94f5ae98ull;
    execute.audit_enabled = 1u;
    execute.audit_stage = PROM_MODEL_BLOCK_M1B_AUDIT_ADALN_PROJECTION;
    execute.audit_output = audit.data();
    execute.audit_element_capacity = audit.size();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                 "real fixed M1B program reaches resident PositionedQ PositionedK and V");
    constexpr std::array<std::uint32_t, 6> coordinates{0u, 1u, 5120u, 10240u, 15358u, 15359u};
    constexpr std::array<float, 6> expected{1.3601882f, 0.47051555f, 0.03414398f, -1.0670887f, 0.117215075f, 0.20466696f};
    for (std::uint32_t index = 0u; index < coordinates.size(); ++index) {
        const float difference = std::fabs(audit[coordinates[index]] - expected[index]);
        std::cout << "M1B AdaLN coordinate " << coordinates[index] << " gpu=" << audit[coordinates[index]]
                  << " canonical=" << expected[index] << " abs=" << difference << "\n";
        ASSERT_TRUE(std::isfinite(audit[coordinates[index]]) && difference <= 1.0e-4f,
                    "AdaLN selected canonical witness matches without a broad tolerance");
    }
    struct StageWitness {
        const char* name;
        std::uint32_t stage;
        std::uint32_t elements;
        std::array<std::uint32_t, 6> coordinates;
        std::array<float, 6> expected;
    };
    const std::array<StageWitness, 20> stages{{
        {"timestep_linear", PROM_MODEL_BLOCK_M1B_AUDIT_ADALN_PROJECTION, 15360u, {0u,1u,5120u,10240u,15358u,15359u}, {1.3601882f,0.47051555f,0.03414398f,-1.0670887f,0.117215075f,0.20466696f}},
        {"attention_scale_raw", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_RAW, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {1.3601882f,0.47051555f,-0.9094451f,-0.24177729f,-1.3053262f,-2.0265357f}},
        {"attention_scale_adjusted", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_ADJUSTED, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {2.3601882f,1.4705155f,0.09055489f,0.7582227f,-0.30532622f,-1.0265357f}},
        {"attention_gate_raw", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_GATE_RAW, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {-0.092752255f,0.033863652f,0.03414398f,-0.009241605f,0.11381386f,-0.030876685f}},
        {"attention_gate_tanh", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_GATE_TANH, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {-0.092487186f,0.033850715f,0.03413072f,-0.009241343f,0.11332496f,-0.030866876f}},
        {"mlp_scale_raw", PROM_MODEL_BLOCK_M1B_AUDIT_MLP_SCALE_RAW, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {-1.2256036f,-0.44273403f,-0.5722749f,-1.0670887f,-0.63635045f,0.2840054f}},
        {"mlp_scale_adjusted", PROM_MODEL_BLOCK_M1B_AUDIT_MLP_SCALE_ADJUSTED, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {-0.22560358f,0.557266f,0.42772508f,-0.06708872f,0.36364955f,1.2840054f}},
        {"mlp_gate_raw", PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_RAW, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {0.12004629f,-0.1627551f,-0.004695369f,0.16414051f,0.117215075f,0.20466696f}},
        {"mlp_gate_tanh", PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_TANH, 3840u, {0u,1u,1280u,2560u,3838u,3839u}, {0.11947293f,-0.16133308f,-0.0046953345f,0.16268213f,0.11668119f,0.2018563f}},
        {"attention_norm", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_NORM, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {-2.9183338f,-0.2905034f,-0.47690302f,0.07386842f,0.038262118f,-1.058256f}},
        {"attention_modulated", PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_MODULATED, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {-6.887817f,-0.42718977f,-0.0431859f,0.056008715f,-0.0116824275f,1.0863377f}},
        {"qkv", PROM_MODEL_BLOCK_M1B_AUDIT_FUSED_QKV, 11796480u, {0u,1u,3932160u,7864320u,11796478u,11796479u}, {0.11094941f,-20.398643f,-8.927877f,21.416372f,-111.61618f,62.262966f}},
        {"q", PROM_MODEL_BLOCK_M1B_AUDIT_Q, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {0.11094941f,-20.398643f,81.20082f,-106.941124f,-67.58962f,-39.493553f}},
        {"k", PROM_MODEL_BLOCK_M1B_AUDIT_K, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {43.58648f,-8.07982f,-65.52536f,-5.828332f,-23.167105f,-7.610529f}},
        {"v", PROM_MODEL_BLOCK_M1B_AUDIT_V, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {54.267914f,-6.3479257f,26.322664f,61.98557f,-111.61618f,62.262966f}},
        {"q_norm", PROM_MODEL_BLOCK_M1B_AUDIT_Q_NORM, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {0.0015781499f,-0.3210185f,0.9649556f,-1.1055946f,-0.8490496f,-0.5325902f}},
        {"k_norm", PROM_MODEL_BLOCK_M1B_AUDIT_K_NORM, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {0.6562256f,-0.14462532f,-0.72043616f,-0.069245115f,-0.44722673f,-0.15771928f}},
        {"q_rope", PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_Q, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {0.32096922f,0.005840092f,-0.046113525f,0.07149897f,-0.75824535f,-0.6554399f}},
        {"k_rope", PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_K, 3932160u, {0u,1u,1310720u,2621440u,3932158u,3932159u}, {0.13590002f,0.6580879f,0.2728928f,-1.1536793f,-0.41806194f,-0.22385556f}},
        {"timestep_input", PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_TIMESTEP, 256u, {0u,1u,85u,170u,254u,255u}, {0.06298828f,0.21582031f,-0.067871094f,0.10107422f,-0.076171875f,-0.115722656f}}
    }};
    const char* canonicalRootText = std::getenv("OCT_EVT2_M1B_CANONICAL");
    const std::filesystem::path canonicalRoot = canonicalRootText == nullptr
        ? std::filesystem::path(".tmp") / "evt2-m1b-canonical"
        : std::filesystem::path(canonicalRootText);
    for (const StageWitness& stage : stages) {
        execute.audit_stage = stage.stage;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                     "real M1B stage audit execution succeeds");
        float maxDifference = 0.0f;
        for (std::uint32_t index = 0u; index < stage.coordinates.size(); ++index) {
            const float actual = audit[stage.coordinates[index]];
            const float difference = std::fabs(actual - stage.expected[index]);
            maxDifference = std::max(maxDifference, difference);
            const float bound = std::max(2.0e-5f, std::fabs(stage.expected[index]) * 2.0e-5f);
            ASSERT_TRUE(std::isfinite(actual) && difference <= bound,
                        "selected canonical stage coordinate matches the narrow measured bound");
        }
        const std::vector<std::uint8_t> canonicalBytes = read_binary_file(canonicalRoot / (std::string(stage.name) + ".f32.bin"));
        ASSERT_EQUAL(static_cast<std::uint64_t>(stage.elements) * sizeof(float),
                     static_cast<std::uint64_t>(canonicalBytes.size()),
                     "local canonical M1B replay stage has the exact physical size");
        double errorSquares = 0.0;
        double referenceSquares = 0.0;
        double linf = 0.0;
        std::uint32_t firstDifference = stage.elements;
        if (canonicalBytes.size() == static_cast<std::size_t>(stage.elements) * sizeof(float)) {
            for (std::uint32_t element = 0u; element < stage.elements; ++element) {
                float reference = 0.0f;
                std::memcpy(&reference, canonicalBytes.data() + element * sizeof(float), sizeof(reference));
                const double difference = static_cast<double>(audit[element]) - static_cast<double>(reference);
                errorSquares += difference * difference;
                referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                linf = std::max(linf, std::fabs(difference));
                if (firstDifference == stage.elements) {
                    std::uint32_t actualBits = 0u;
                    std::uint32_t referenceBits = 0u;
                    std::memcpy(&actualBits, &audit[element], sizeof(actualBits));
                    std::memcpy(&referenceBits, &reference, sizeof(referenceBits));
                    if (actualBits != referenceBits) firstDifference = element;
                }
            }
        }
        const double l2 = std::sqrt(errorSquares);
        const double rms = std::sqrt(errorSquares / static_cast<double>(stage.elements));
        const double relativeL2 = referenceSquares == 0.0 ? 0.0 : l2 / std::sqrt(referenceSquares);
        std::cout << "M1B stage " << stage.name << " selected_linf=" << maxDifference
                  << " l2=" << l2 << " linf=" << linf << " relative_l2=" << relativeL2
                  << " rms=" << rms << " first_bit_difference=" << firstDifference
                  << " elements=" << stage.elements << "\n";
    }
    execute.audit_enabled = 0u;
    execute.audit_stage = PROM_MODEL_BLOCK_M1B_AUDIT_NONE;
    const std::uint64_t replayIdentity = evidence.replay_identity;
    std::array<std::uint64_t, 10> warmNs{};
    for (std::uint32_t iteration = 0u; iteration < warmNs.size(); ++iteration) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                     "zero-audit warm resident execution succeeds");
        warmNs[iteration] = evidence.last_execution_ns;
    }
    ASSERT_EQUAL(0u, evidence.warm_buffer_allocation_count, "ten warm executions allocate no buffers");
    ASSERT_EQUAL(13u, evidence.weight_upload_count, "ten warm executions perform no additional uploads");
    ASSERT_TRUE(evidence.replay_identity != replayIdentity, "audit mode participates in replay identity");
    const std::uint64_t stableWarmReplay = evidence.replay_identity;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                 "repeat warm replay succeeds");
    ASSERT_EQUAL(stableWarmReplay, evidence.replay_identity, "identical warm runs retain deterministic replay identity");
    const char* runM1C = std::getenv("OCT_EVT2_M1C_REAL");
    if (runM1C != nullptr && std::string(runM1C) == "1") {
        std::vector<float> attentionResidual(1024u * 3840u);
        PrometheusModelBlockM1CExecuteRequest m1c{};
        m1c.struct_size = sizeof(m1c);
        m1c.m1b_prefix_replay_identity = evidence.replay_identity;
        m1c.output_identity = 0xff02d4bf4acfc0dcull;
        m1c.output = attentionResidual.data();
        m1c.output_element_capacity = attentionResidual.size();
        std::array<float, PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS> transientAudit{};
        m1c.transient_audit = transientAudit.data();
        m1c.transient_audit_element_capacity = transientAudit.size();
        const char* cacheRootText = std::getenv("OCT_EVT2_CACHE");
        ASSERT_TRUE(cacheRootText != nullptr, "real M1C requires the documented OCT_EVT2_CACHE root");
        const std::filesystem::path m1cRoot = std::filesystem::path(cacheRootText == nullptr ? "" : cacheRootText) /
            "canonical" / "f332072aa78be7aecdf3ee76d5c247082da564a6" / "o19-fp32-reference" / "noise_refiner.0" / "stages";
        constexpr std::array<std::uint32_t, 6> m1cCoordinates{0u, 1u, 1310720u, 2621440u, 3932158u, 3932159u};
        struct M1CStage { const char* name; std::uint32_t auditStage; std::array<float, 6> expected; };
        const std::array<M1CStage, 3> m1cStages{{
            {"attention_aggregation", PROM_MODEL_BLOCK_M1C_AUDIT_ATTENTION, {22.60872f,-16.405487f,-10.827898f,208.46693f,-74.963585f,110.65977f}},
            {"attention_projection", PROM_MODEL_BLOCK_M1C_AUDIT_PROJECTION, {38.777737f,392.79315f,469.82147f,-6.6808763f,89.16175f,-10.219366f}},
            {"attention_residual", PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL, {-1.0000031f,-0.1181361f,-0.19042696f,0.018676776f,0.01519823f,-0.5390627f}}
        }};
        for (const M1CStage& stage : m1cStages) {
            m1c.audit_stage = stage.auditStage;
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1c(runtime, blockID, &m1c, &evidence),
                         "resident M1C consumes the matching M1B replay without host QKV ingress");
            const std::vector<std::uint8_t> canonicalBytes = read_binary_file(m1cRoot / (std::string(stage.name) + ".f32.bin"));
            ASSERT_EQUAL(static_cast<std::uint64_t>(attentionResidual.size()) * sizeof(float),
                         static_cast<std::uint64_t>(canonicalBytes.size()), "O19 M1C boundary payload exists");
            double errorSquares = 0.0, referenceSquares = 0.0, linf = 0.0;
            std::uint32_t firstDifference = static_cast<std::uint32_t>(attentionResidual.size());
            float minimum = attentionResidual[0u], maximum = attentionResidual[0u];
            for (std::uint32_t element = 0u; element < attentionResidual.size(); ++element) {
                float reference = 0.0f;
                std::memcpy(&reference, canonicalBytes.data() + element * sizeof(float), sizeof(reference));
                const double difference = static_cast<double>(attentionResidual[element]) - static_cast<double>(reference);
                errorSquares += difference * difference;
                referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                linf = std::max(linf, std::fabs(difference));
                minimum = std::min(minimum, attentionResidual[element]);
                maximum = std::max(maximum, attentionResidual[element]);
                if (firstDifference == attentionResidual.size() && attentionResidual[element] != reference) firstDifference = element;
            }
            const double l2 = std::sqrt(errorSquares);
            const double relativeL2 = l2 / std::sqrt(referenceSquares);
            const double rms = std::sqrt(errorSquares / static_cast<double>(attentionResidual.size()));
            for (std::uint32_t index = 0u; index < m1cCoordinates.size(); ++index) {
                const float difference = std::fabs(attentionResidual[m1cCoordinates[index]] - stage.expected[index]);
                ASSERT_TRUE(std::isfinite(attentionResidual[m1cCoordinates[index]]) && difference <= 2.0e-3f,
                            "M1C selected O19 witness stays within the narrow measured bound");
            }
            ASSERT_TRUE(std::isfinite(minimum) && std::isfinite(maximum) && relativeL2 <= 2.0e-5,
                        "M1C full resident boundary is finite and remains within the measured FP32 bound");
            std::cout << "M1C stage " << stage.name << " shape=[1,1024,3840] layout=row-major finite=true min="
                      << minimum << " max=" << maximum << " l2=" << l2 << " linf=" << linf
                      << " relative_l2=" << relativeL2 << " rms=" << rms
                      << " first_mismatching_coordinate=" << firstDifference << "\n";
        }
        ASSERT_EQUAL(1.0f, transientAudit[30u], "first selected softmax denominator is finite and positive");
        ASSERT_EQUAL(1.0f, transientAudit[62u], "last selected softmax denominator is finite and positive");
        ASSERT_TRUE(std::isfinite(transientAudit[20u]) && std::isfinite(transientAudit[21u]) &&
                        std::isfinite(transientAudit[22u]) && std::fabs(transientAudit[22u] - 1.0f) <= 2.0e-5f &&
                        std::isfinite(transientAudit[52u]) && std::isfinite(transientAudit[53u]) &&
                        std::isfinite(transientAudit[54u]) && std::fabs(transientAudit[54u] - 1.0f) <= 2.0e-5f,
                    "selected first and last softmax rows are finite and normalize to one");
        const std::array<const char*, 2> transientRows{{"attention_logits_token0_head0", "attention_logits_token1023_head29"}};
        const std::array<const char*, 2> transientProbabilities{{"attention_probabilities_token0_head0", "attention_probabilities_token1023_head29"}};
        constexpr std::array<std::uint32_t, 4> transientKeys{{0u, 1u, 512u, 1023u}};
        for (std::uint32_t row = 0u; row < 2u; ++row) {
            const std::vector<std::uint8_t> logits = read_binary_file(m1cRoot / (std::string(transientRows[row]) + ".f32.bin"));
            const std::vector<std::uint8_t> probabilities = read_binary_file(m1cRoot / (std::string(transientProbabilities[row]) + ".f32.bin"));
            const std::uint32_t base = row * 32u;
            for (std::uint32_t sample = 0u; sample < transientKeys.size(); ++sample) {
                float canonicalLogit = 0.0f, canonicalProbability = 0.0f;
                std::memcpy(&canonicalLogit, logits.data() + transientKeys[sample] * sizeof(float), sizeof(float));
                std::memcpy(&canonicalProbability, probabilities.data() + transientKeys[sample] * sizeof(float), sizeof(float));
                ASSERT_TRUE(std::fabs(transientAudit[base + sample * 5u + 1u] - canonicalLogit) <= 2.0e-5f &&
                                std::fabs(transientAudit[base + sample * 5u + 4u] - canonicalProbability) <= 2.0e-6f,
                            "selected streaming logits and probabilities match the exact O19 row witnesses");
            }
        }
        std::cout << "M1C transient first raw=" << transientAudit[0u] << " scaled=" << transientAudit[1u]
                  << " max=" << transientAudit[20u] << " shifted=" << transientAudit[2u]
                  << " exp=" << transientAudit[3u] << " sum=" << transientAudit[21u]
                  << " probability=" << transientAudit[4u] << " row_sum=" << transientAudit[22u]
                  << " probability_times_v=" << transientAudit[23u] << " head_output=" << transientAudit[24u] << "\n";
        std::cout << "M1C transient last raw=" << transientAudit[32u] << " scaled=" << transientAudit[33u]
                  << " max=" << transientAudit[52u] << " shifted=" << transientAudit[34u]
                  << " exp=" << transientAudit[35u] << " sum=" << transientAudit[53u]
                  << " probability=" << transientAudit[36u] << " row_sum=" << transientAudit[54u]
                  << " probability_times_v=" << transientAudit[55u] << " head_output=" << transientAudit[56u] << "\n";
        std::array<std::uint64_t, 10> chainedWarmNs{};
        const std::uint64_t allocationsBeforeWarm = evidence.warm_buffer_allocation_count;
        const std::uint64_t uploadsBeforeWarm = evidence.weight_upload_count;
        const std::uint64_t pipelinesBeforeWarm = evidence.pipeline_create_count;
        const std::uint64_t descriptorsBeforeWarm = evidence.descriptor_set_count;
        for (std::uint32_t iteration = 0u; iteration < chainedWarmNs.size(); ++iteration) {
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                         "warm chained M1B prefix execution succeeds");
            m1c.m1b_prefix_replay_identity = evidence.replay_identity;
            m1c.audit_stage = PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL;
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1c(runtime, blockID, &m1c, &evidence),
                         "warm chained M1C residual execution succeeds");
            chainedWarmNs[iteration] = evidence.last_execution_ns;
        }
        ASSERT_EQUAL(allocationsBeforeWarm, evidence.warm_buffer_allocation_count,
                     "warm M1B to M1C chain allocates no buffers");
        ASSERT_EQUAL(uploadsBeforeWarm, evidence.weight_upload_count, "warm M1B to M1C chain uploads no weights");
        ASSERT_EQUAL(pipelinesBeforeWarm, evidence.pipeline_create_count, "warm M1B to M1C chain creates no pipelines");
        ASSERT_EQUAL(descriptorsBeforeWarm, evidence.descriptor_set_count, "warm M1B to M1C chain grows no descriptor pools");
        std::array<std::uint64_t, 10> sortedChainedWarmNs = chainedWarmNs;
        std::sort(sortedChainedWarmNs.begin(), sortedChainedWarmNs.end());
        const double chainedWarmMean = static_cast<double>(std::accumulate(chainedWarmNs.begin(), chainedWarmNs.end(), std::uint64_t{0})) /
            static_cast<double>(chainedWarmNs.size());
        double chainedWarmVariance = 0.0;
        for (const std::uint64_t elapsed : chainedWarmNs) {
            const double delta = static_cast<double>(elapsed) - chainedWarmMean;
            chainedWarmVariance += delta * delta;
        }
        std::cout << "M1C chained_warm_ns=";
        for (std::size_t index = 0u; index < chainedWarmNs.size(); ++index) {
            if (index != 0u) std::cout << ',';
            std::cout << chainedWarmNs[index];
        }
        std::cout << "\n";
        std::cout << "M1C chained_warm_stats_ns median="
                  << (sortedChainedWarmNs[4u] + sortedChainedWarmNs[5u]) / 2u
                  << " mean=" << chainedWarmMean
                  << " min=" << sortedChainedWarmNs.front()
                  << " p95=" << sortedChainedWarmNs[9u]
                  << " stddev=" << std::sqrt(chainedWarmVariance / static_cast<double>(chainedWarmNs.size())) << "\n";

        const char* runM1D = std::getenv("OCT_EVT2_M1D_REAL");
        if (runM1D != nullptr && std::string(runM1D) == "1") {
            struct M1DStage { const char* name; std::uint32_t auditStage; std::uint32_t elements; };
            const std::array<M1DStage, 7> m1dStages{{
                {"ffn_norm", PROM_MODEL_BLOCK_M1D_AUDIT_FFN_NORM, 1024u * 3840u},
                {"ffn_modulated", PROM_MODEL_BLOCK_M1D_AUDIT_FFN_MODULATED, 1024u * 3840u},
                {"w1", PROM_MODEL_BLOCK_M1D_AUDIT_W1, 1024u * 10240u},
                {"w3", PROM_MODEL_BLOCK_M1D_AUDIT_W3, 1024u * 10240u},
                {"ffn_gated_hidden", PROM_MODEL_BLOCK_M1D_AUDIT_GATED_HIDDEN, 1024u * 10240u},
                {"w2", PROM_MODEL_BLOCK_M1D_AUDIT_W2, 1024u * 3840u},
                {"final_output", PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT, 1024u * 3840u},
            }};
            std::vector<float> m1dOutput(1024u * 10240u);
            PrometheusModelBlockM1DExecuteRequest m1d{};
            m1d.struct_size = sizeof(m1d);
            m1d.output_identity = 0xa4fd07d58b1c9e23ull;
            m1d.output = m1dOutput.data();
            m1d.output_element_capacity = m1dOutput.size();
            for (const M1DStage& stage : m1dStages) {
                ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1b(runtime, blockID, &execute, &evidence),
                             "M1D audit replays the required real M1B resident prefix");
                m1c.m1b_prefix_replay_identity = evidence.replay_identity;
                m1c.audit_stage = PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL;
                ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1c(runtime, blockID, &m1c, &evidence),
                             "M1D audit replays the required real M1C resident residual");
                m1d.m1c_prefix_replay_identity = evidence.replay_identity;
                m1d.audit_stage = stage.auditStage;
                ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute_m1d(runtime, blockID, &m1d, &evidence),
                             "M1D executes a fixed resident FFN stage without host ingress");
                const std::vector<std::uint8_t> canonicalBytes = read_binary_file(m1cRoot / (std::string(stage.name) + ".f32.bin"));
                ASSERT_EQUAL(static_cast<std::uint64_t>(stage.elements) * sizeof(float), static_cast<std::uint64_t>(canonicalBytes.size()),
                             "M1D canonical O19 stage payload exists with the fixed shape");
                double errorSquares = 0.0, referenceSquares = 0.0, linf = 0.0;
                float minimum = m1dOutput[0u], maximum = m1dOutput[0u];
                std::uint32_t firstDifference = stage.elements;
                for (std::uint32_t element = 0u; element < stage.elements; ++element) {
                    float reference = 0.0f;
                    std::memcpy(&reference, canonicalBytes.data() + element * sizeof(float), sizeof(reference));
                    const double difference = static_cast<double>(m1dOutput[element]) - static_cast<double>(reference);
                    errorSquares += difference * difference;
                    referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                    linf = std::max(linf, std::fabs(difference));
                    minimum = std::min(minimum, m1dOutput[element]);
                    maximum = std::max(maximum, m1dOutput[element]);
                    if (firstDifference == stage.elements && m1dOutput[element] != reference) firstDifference = element;
                }
                const double l2 = std::sqrt(errorSquares);
                const double relativeL2 = referenceSquares == 0.0 ? 0.0 : l2 / std::sqrt(referenceSquares);
                const double rms = std::sqrt(errorSquares / static_cast<double>(stage.elements));
                ASSERT_TRUE(std::isfinite(minimum) && std::isfinite(maximum) && relativeL2 <= 5.0e-5,
                            "M1D resident stage remains FP32-finite and within the narrow measured O19 bound");
                std::cout << "M1D stage " << stage.name << " finite=true min=" << minimum << " max=" << maximum
                          << " l2=" << l2 << " linf=" << linf << " relative_l2=" << relativeL2
                          << " rms=" << rms << " first_mismatching_coordinate=" << firstDifference << "\n";
            }
            PrometheusNoiseRefiner0ExecuteRequest complete{};
            complete.struct_size = sizeof(complete);
            complete.model_input_bf16 = input.data();
            complete.timestep_bf16 = timestep.data();
            complete.model_input_bytes = input.size();
            complete.timestep_bytes = timestep.size();
            complete.input_identity = 0x9a7d6e5c4b3a2918ull;
            complete.timestep_identity = 0x1029384756abcdefull;
            complete.output_identity = 0xa4fd07d58b1c9e23ull;
            const std::uint64_t completeAllocations = evidence.warm_buffer_allocation_count;
            const std::uint64_t completeUploads = evidence.weight_upload_count;
            const std::uint64_t completePipelines = evidence.pipeline_create_count;
            const std::uint64_t completeDescriptors = evidence.descriptor_set_count;
            std::array<std::uint64_t, 10> completeWarmNs{};
            for (std::uint32_t iteration = 0u; iteration < completeWarmNs.size(); ++iteration) {
                const int completeResult = prometheus_reactor_runtime_noise_refiner0_execute(runtime, blockID, &complete, &evidence);
                std::cout << "M1F complete iteration=" << iteration << " result=" << completeResult
                          << " detail=" << evidence.detail_code << "\n";
                ASSERT_EQUAL(PROM_OK, completeResult,
                             "M1F complete resident block execution succeeds without host intermediate output");
                ASSERT_TRUE(evidence.output_valid != 0u && evidence.audit_valid == 0u,
                            "M1F retains resident output and does not turn a warm execution into an audit bounce");
                completeWarmNs[iteration] = evidence.last_execution_ns;
            }
            ASSERT_EQUAL(completeAllocations, evidence.warm_buffer_allocation_count, "M1F warm block allocates no buffers");
            ASSERT_EQUAL(completeUploads, evidence.weight_upload_count, "M1F warm block uploads no weights");
            ASSERT_EQUAL(completePipelines, evidence.pipeline_create_count, "M1F warm block creates no pipelines");
            ASSERT_EQUAL(completeDescriptors, evidence.descriptor_set_count, "M1F warm block grows no descriptors");
            std::array<std::uint64_t, 10> sortedCompleteWarmNs = completeWarmNs;
            std::sort(sortedCompleteWarmNs.begin(), sortedCompleteWarmNs.end());
            const double completeWarmMean = static_cast<double>(std::accumulate(completeWarmNs.begin(), completeWarmNs.end(), std::uint64_t{0})) /
                static_cast<double>(completeWarmNs.size());
            double completeWarmVariance = 0.0;
            for (const std::uint64_t elapsed : completeWarmNs) {
                const double delta = static_cast<double>(elapsed) - completeWarmMean;
                completeWarmVariance += delta * delta;
            }
            std::cout << "M1F complete_warm_ns=";
            for (std::size_t index = 0u; index < completeWarmNs.size(); ++index) {
                if (index != 0u) std::cout << ',';
                std::cout << completeWarmNs[index];
            }
            std::cout << "\nM1F complete_warm_stats_ns median=" << (sortedCompleteWarmNs[4u] + sortedCompleteWarmNs[5u]) / 2u
                      << " mean=" << completeWarmMean << " min=" << sortedCompleteWarmNs.front()
                      << " p95=" << sortedCompleteWarmNs[9u]
                      << " stddev=" << std::sqrt(completeWarmVariance / static_cast<double>(completeWarmNs.size())) << "\n";

            /* M2A-R: the second parameter set is never a facade alias.  The
               block-0 FP32 final remains in attention while a complete second
               immutable package is staged, descriptor-bound, and committed. */
            const std::filesystem::path block1Cache = std::filesystem::path(cacheRootText) / "layers" /
                "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6" / "noise_refiner.1";
            constexpr std::array<const char*, PROM_MODEL_BLOCK_MAX_WEIGHTS> block1Names{
                "noise_refiner.1.adaLN_modulation.0.bias.fp16.bin",
                "noise_refiner.1.adaLN_modulation.0.weight.fp16.bin",
                "noise_refiner.1.attention.k_norm.weight.fp16.bin",
                "noise_refiner.1.attention.out.weight.fp16.bin",
                "noise_refiner.1.attention.q_norm.weight.fp16.bin",
                "noise_refiner.1.attention.qkv.weight.fp16.bin",
                "noise_refiner.1.attention_norm1.weight.fp16.bin",
                "noise_refiner.1.attention_norm2.weight.fp16.bin",
                "noise_refiner.1.feed_forward.w1.weight.fp16.bin",
                "noise_refiner.1.feed_forward.w2.weight.fp16.bin",
                "noise_refiner.1.feed_forward.w3.weight.fp16.bin",
                "noise_refiner.1.ffn_norm1.weight.fp16.bin",
                "noise_refiner.1.ffn_norm2.weight.fp16.bin"};
            std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> block1Bytes{};
            std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> block1Uploads{};
            for (std::uint32_t index = 0u; index < block1Names.size(); ++index) {
                block1Bytes[index] = read_binary_file(block1Cache / block1Names[index]);
                ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(block1Bytes[index].size()),
                             "block-1 cache has the identical closed physical tensor layout");
                block1Uploads[index].binding_index = index;
                block1Uploads[index].bytes = block1Bytes[index].data();
                block1Uploads[index].byte_count = block1Bytes[index].size();
                block1Uploads[index].content_identity = 0x7c00u + index;
                block1Uploads[index].layout_identity = 0x8d00u + index;
            }
            PrometheusNoiseRefinerRebindRequest rebind{};
            rebind.struct_size = sizeof(rebind);
            rebind.lock_identity = PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID;
            rebind.model_local_block_id = 1u;
            rebind.upload_count = static_cast<std::uint32_t>(block1Uploads.size());
            rebind.uploads = block1Uploads.data();
            PrometheusNoiseRefinerRebindRequest invalidRebind = rebind;
            invalidRebind.lock_identity ^= 1u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                         "stale or foreign lock identity is rejected before resident mutation");
            invalidRebind = rebind;
            invalidRebind.model_local_block_id = 0u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                         "illegal block-0 to block-0 transition is rejected by the resolved predecessor/successor contract");
            invalidRebind = rebind;
            const std::uint64_t originalBlock1Bytes = block1Uploads[0u].byte_count;
            block1Uploads[0u].byte_count -= 2u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_rebind(runtime, blockID, &invalidRebind, &evidence),
                         "wrong block-1 payload layout is rejected before the candidate arena is allocated");
            block1Uploads[0u].byte_count = originalBlock1Bytes;
            const std::uint64_t block0OutputGeneration = evidence.output_generation;
            const auto rebindBegin = std::chrono::steady_clock::now();
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_rebind(runtime, blockID, &rebind, &evidence),
                         "complete block-1 package commits only after all staged uploads and descriptor writes succeed");
            const auto rebindEnd = std::chrono::steady_clock::now();
            ASSERT_EQUAL(PROM_NOISE_REFINER_PARAMETER_SET_1, evidence.parameter_set,
                         "committed resident handle reports the closed block-1 parameter identity");
            ASSERT_TRUE(evidence.output_valid == 0u && evidence.replay_identity == 0u,
                        "commit invalidates block-0 output and replay acceptance before block-1 dispatch");
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner0_execute(runtime, blockID, &complete, &evidence),
                         "block-0 facade cannot relabel a block-1 resident binding");
            PrometheusNoiseRefinerResidentExecuteRequest resident{};
            resident.struct_size = sizeof(resident);
            resident.input_generation = block0OutputGeneration;
            resident.output_identity = 0x9b133c9ed3772f78ull;
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_execute_resident(runtime, blockID, &resident, &evidence),
                         "block-1 consumes the resident block-0 FP32 final without BF16 ingress or host activation bounce");
            std::vector<float> block1Final(1024u * 3840u);
            PrometheusNoiseRefinerFinalAuditRequest block1Audit{};
            block1Audit.struct_size = sizeof(block1Audit);
            block1Audit.required_output_generation = evidence.output_generation;
            block1Audit.output_identity = 0x9b133c9ed3772f78ull;
            block1Audit.output = block1Final.data();
            block1Audit.output_element_capacity = block1Final.size();
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_audit_final(runtime, blockID, &block1Audit, &evidence),
                         "post-chain audit copies the completed block-1 FP32 final without changing the resident chain ABI");
            const std::filesystem::path block1Canonical = std::filesystem::path(cacheRootText) / "canonical" /
                "f332072aa78be7aecdf3ee76d5c247082da564a6" / "o19-fp32-reference" / "noise_refiner.1" / "final_output.f32.bin";
            const std::vector<std::uint8_t> block1CanonicalBytes = read_binary_file(block1Canonical);
            ASSERT_EQUAL(static_cast<std::uint64_t>(block1Final.size()) * sizeof(float),
                         static_cast<std::uint64_t>(block1CanonicalBytes.size()), "block-1 canonical final authority is present");
            double block1ErrorSquares = 0.0, block1ReferenceSquares = 0.0, block1Linf = 0.0;
            for (std::uint32_t element = 0u; element < block1Final.size() &&
                 block1CanonicalBytes.size() == block1Final.size() * sizeof(float); ++element) {
                float reference = 0.0f;
                std::memcpy(&reference, block1CanonicalBytes.data() + element * sizeof(float), sizeof(reference));
                const double difference = static_cast<double>(block1Final[element]) - static_cast<double>(reference);
                block1ErrorSquares += difference * difference;
                block1ReferenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                block1Linf = std::max(block1Linf, std::fabs(difference));
            }
            const double block1RelativeL2 = std::sqrt(block1ErrorSquares) / std::sqrt(block1ReferenceSquares);
            ASSERT_TRUE(std::isfinite(block1RelativeL2) && block1RelativeL2 <= 5.0e-5,
                        "block-1 resident final matches the canonical FP32 authority within the accepted compiled-block threshold");
            const std::array<const char*, PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT> block1PersistentStages{{
                "timestep_linear", "attention_scale_raw", "attention_scale_adjusted", "attention_gate_raw",
                "attention_gate_tanh", "mlp_scale_raw", "mlp_scale_adjusted", "mlp_gate_raw", "mlp_gate_tanh",
                "attention_norm", "attention_modulated", "qkv", "q", "k", "v", "q_norm", "k_norm", "q_rope",
                "k_rope", "attention_aggregation", "attention_projection", "attention_residual", "ffn_norm",
                "ffn_modulated", "w1", "w3", "ffn_gated_hidden", "w2", "final_output"}};
            const std::filesystem::path block1StageRoot = block1Canonical.parent_path() / "stages";
            std::vector<std::uint8_t> staticAuditArena(PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES, 0u);
            std::vector<std::uint8_t> repeatedAuditArena(PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES, 0u);
            PrometheusNoiseRefinerStaticAuditRequest staticAudit{};
            staticAudit.struct_size = sizeof(staticAudit);
            staticAudit.lock_identity = PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID;
            staticAudit.input_generation = block0OutputGeneration;
            staticAudit.output_identity = 0x9b133c9ed3772f78ull;
            staticAudit.audit_arena = staticAuditArena.data();
            staticAudit.audit_arena_capacity_bytes = staticAuditArena.size();
            PrometheusNoiseRefinerStaticAuditRequest invalidStaticAudit = staticAudit;
            invalidStaticAudit.lock_identity ^= 1u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_execute_static_audit(
                                         runtime, blockID, &invalidStaticAudit, &evidence),
                         "foreign static-audit lock is rejected before GPU submission");
            invalidStaticAudit = staticAudit;
            invalidStaticAudit.audit_arena_capacity_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES - 1u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_execute_static_audit(
                                         runtime, blockID, &invalidStaticAudit, &evidence),
                         "static audit cannot mutate or exceed the fixed arena contract");
            invalidStaticAudit = staticAudit;
            invalidStaticAudit.input_generation += 1u;
            ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_noise_refiner_execute_static_audit(
                                         runtime, blockID, &invalidStaticAudit, &evidence),
                         "mixed resident execution generation is rejected before command recording");
            const std::uint64_t staticAllocations = evidence.warm_buffer_allocation_count;
            const std::uint64_t staticPipelines = evidence.pipeline_create_count;
            const std::uint64_t staticDescriptors = evidence.descriptor_set_count;
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_execute_static_audit(
                                      runtime, blockID, &staticAudit, &evidence),
                         "block-1 executes every persistent capture in one lock-derived static batch");
            const std::uint32_t staticGeneration = static_cast<std::uint32_t>(evidence.output_generation);
            ASSERT_EQUAL(staticAllocations, evidence.warm_buffer_allocation_count,
                         "static audit allocates no warm tensor shadow arena");
            ASSERT_EQUAL(staticPipelines, evidence.pipeline_create_count,
                         "static audit recreates no shader module or pipeline");
            ASSERT_EQUAL(staticDescriptors, evidence.descriptor_set_count,
                         "static audit performs no repeated descriptor rebind");
            staticAudit.audit_arena = repeatedAuditArena.data();
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_execute_static_audit(
                                      runtime, blockID, &staticAudit, &evidence),
                         "a repeated complete static batch reuses the immutable resident predecessor");
            const std::uint32_t repeatedGeneration = static_cast<std::uint32_t>(evidence.output_generation);
            ASSERT_EQUAL(staticGeneration + 1u, repeatedGeneration,
                         "each complete static batch owns exactly one execution generation");
            ASSERT_TRUE(std::memcmp(staticAuditArena.data() + PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_OFFSET,
                                    repeatedAuditArena.data() + PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_OFFSET,
                                    PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_BYTES) == 0,
                        "bounded transient attention witnesses do not accumulate state across static batches");

            for (std::size_t stageIndex = 0u; stageIndex < block1PersistentStages.size(); ++stageIndex) {
                const char* stageName = block1PersistentStages[stageIndex];
                const auto& schedule = k_prom_zimage_turbo_audit_schedule[stageIndex];
                const std::vector<std::uint8_t> canonicalBytes = read_binary_file(
                    block1StageRoot / (std::string(stageName) + ".f32.bin"));
                ASSERT_EQUAL(static_cast<std::uint64_t>(schedule.element_count) * sizeof(float),
                             static_cast<std::uint64_t>(canonicalBytes.size()),
                             "block-1 persistent canonical stage payload has its declared physical size");
                double errorSquares = 0.0, referenceSquares = 0.0, linf = 0.0;
                double actualSquares = 0.0, canonicalSquares = 0.0;
                float minimum = std::numeric_limits<float>::infinity();
                float maximum = -std::numeric_limits<float>::infinity();
                float canonicalMinimum = std::numeric_limits<float>::infinity();
                float canonicalMaximum = -std::numeric_limits<float>::infinity();
                std::uint32_t firstDifference = schedule.element_count;
                std::uint32_t compared = 0u;
                const auto* record = reinterpret_cast<const std::uint32_t*>(
                    staticAuditArena.data() + schedule.audit_destination_offset);
                const auto* repeatedRecord = reinterpret_cast<const std::uint32_t*>(
                    repeatedAuditArena.data() + schedule.audit_destination_offset);
                if (schedule.capture_policy == PROM_ZIMAGE_AUDIT_CAPTURE_FULL) {
                    const auto* actual = reinterpret_cast<const float*>(record);
                    for (std::uint32_t element = 0u; element < schedule.element_count; ++element) {
                        float reference = 0.0f;
                        std::memcpy(&reference, canonicalBytes.data() + element * sizeof(float), sizeof(reference));
                        const double difference = static_cast<double>(actual[element]) - static_cast<double>(reference);
                        errorSquares += difference * difference;
                        referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                        linf = std::max(linf, std::fabs(difference));
                        minimum = std::min(minimum, actual[element]);
                        maximum = std::max(maximum, actual[element]);
                        actualSquares += static_cast<double>(actual[element]) * static_cast<double>(actual[element]);
                        if (firstDifference == schedule.element_count && actual[element] != reference) firstDifference = element;
                    }
                    ASSERT_TRUE(std::memcmp(record, repeatedRecord, schedule.element_count * sizeof(float)) == 0,
                                "full-copy audit evidence is deterministic across complete static batches");
                    compared = schedule.element_count;
                } else {
                    ASSERT_EQUAL(schedule.stage_id, record[0u], "summary record retains the generated stage ID");
                    ASSERT_EQUAL(staticGeneration, record[1u], "summary record retains one execution generation");
                    ASSERT_EQUAL(schedule.element_count, record[2u], "summary record retains the declared element count");
                    ASSERT_TRUE(record[3u] != 0u && record[4u] == 0u && record[5u] == 0u &&
                                    record[6u] == schedule.element_count && record[12u] == 1u && record[13u] == 0u,
                                "summary record is finite, complete, and status-clean");
                    ASSERT_EQUAL(repeatedGeneration, repeatedRecord[1u],
                                 "repeated summary record has only the next complete execution generation");
                    for (std::uint32_t word = 0u; word < 64u; ++word) {
                        if (word != 1u) ASSERT_EQUAL(record[word], repeatedRecord[word],
                                                    "summary shader output is deterministic apart from generation");
                    }
                    std::memcpy(&minimum, &record[8u], sizeof(minimum));
                    std::memcpy(&maximum, &record[9u], sizeof(maximum));
                    for (std::uint32_t projection = 0u; projection < record[7u]; ++projection) {
                        const std::uint32_t sourceIndex = record[16u + projection * 2u];
                        float actual = 0.0f;
                        float reference = 0.0f;
                        std::memcpy(&actual, &record[17u + projection * 2u], sizeof(actual));
                        std::memcpy(&reference, canonicalBytes.data() + sourceIndex * sizeof(float), sizeof(reference));
                        const double difference = static_cast<double>(actual) - static_cast<double>(reference);
                        errorSquares += difference * difference;
                        referenceSquares += static_cast<double>(reference) * static_cast<double>(reference);
                        linf = std::max(linf, std::fabs(difference));
                        if (firstDifference == schedule.element_count && actual != reference) firstDifference = sourceIndex;
                        ++compared;
                    }
                }
                for (std::uint32_t element = 0u; element < schedule.element_count; ++element) {
                    float reference = 0.0f;
                    std::memcpy(&reference, canonicalBytes.data() + element * sizeof(float), sizeof(reference));
                    canonicalSquares += static_cast<double>(reference) * static_cast<double>(reference);
                    canonicalMinimum = std::min(canonicalMinimum, reference);
                    canonicalMaximum = std::max(canonicalMaximum, reference);
                }
                const double l2 = std::sqrt(errorSquares);
                const double relativeL2 = referenceSquares == 0.0 ? 0.0 : l2 / std::sqrt(referenceSquares);
                const double errorRms = std::sqrt(errorSquares / static_cast<double>(compared));
                float outputRms = 0.0f;
                if (schedule.capture_policy != PROM_ZIMAGE_AUDIT_CAPTURE_FULL) {
                    std::memcpy(&outputRms, &record[11u], sizeof(outputRms));
                } else {
                    outputRms = static_cast<float>(std::sqrt(actualSquares / schedule.element_count));
                }
                const double canonicalRms = std::sqrt(canonicalSquares / schedule.element_count);
                const double minimumTolerance = std::max(1.52588e-4, std::fabs(canonicalMinimum) * 5.0e-5);
                const double maximumTolerance = std::max(1.52588e-4, std::fabs(canonicalMaximum) * 5.0e-5);
                const double rmsTolerance = std::max(1.52588e-4, std::fabs(canonicalRms) * 5.0e-5);
                ASSERT_TRUE(std::isfinite(minimum) && std::isfinite(maximum) && std::isfinite(outputRms) &&
                                relativeL2 <= 5.0e-5 &&
                                std::fabs(static_cast<double>(minimum) - canonicalMinimum) <= minimumTolerance &&
                                std::fabs(static_cast<double>(maximum) - canonicalMaximum) <= maximumTolerance &&
                                std::fabs(static_cast<double>(outputRms) - canonicalRms) <= rmsTolerance,
                            "block-1 persistent evidence remains finite and inside measured summary/projection tolerances");
                std::cout << "M2A static stage " << stageName << " finite=true min=" << minimum << " max=" << maximum
                          << " l2=" << l2 << " linf=" << linf << " relative_l2=" << relativeL2 << " rms=" << errorRms
                          << " summary_rms=" << outputRms << " first_mismatching_coordinate=" << firstDifference
                          << " accepted_threshold=5e-5 authority=f332072aa78be7aecdf3ee76d5c247082da564a6\n";
            }
            std::cout << "M2A chain block0_output_generation=" << block0OutputGeneration
                      << " block1_output_generation=" << evidence.output_generation
                      << " rebind_ns=" << std::chrono::duration_cast<std::chrono::nanoseconds>(rebindEnd - rebindBegin).count()
                      << " block1_execution_ns=" << evidence.last_execution_ns
                      << " binding_generation=" << evidence.binding_generation
                      << " descriptor_generation=" << evidence.descriptor_generation
                      << " block1_relative_l2=" << block1RelativeL2
                      << " block1_linf=" << block1Linf
                      << " replay_identity=" << evidence.replay_identity << "\n";
        }
    }
    std::cout << "M1B evidence uploaded_bytes=361820672 upload_ns=" << uploadNs
              << " persistent_bytes=" << evidence.persistent_bytes
              << " reusable_bytes=" << evidence.reusable_bytes
              << " audit_bytes=" << evidence.audit_bytes
              << " total_committed_bytes=" << evidence.total_committed_bytes
              << " peak_plan_bytes=" << evidence.peak_plan_bytes
              << " cold_buffer_allocations=" << evidence.cold_buffer_allocation_count
              << " warm_buffer_allocations=" << evidence.warm_buffer_allocation_count
              << " pipeline_creations=" << evidence.pipeline_create_count
              << " descriptor_sets=" << evidence.descriptor_set_count
              << " weight_uploads=" << evidence.weight_upload_count
              << " execution_plan_identity=" << evidence.execution_plan_identity
              << " replay_identity=" << evidence.replay_identity << "\n";
    std::cout << "M1B warm_ns=";
    for (std::size_t index = 0u; index < warmNs.size(); ++index) {
        if (index != 0u) std::cout << ',';
        std::cout << warmNs[index];
    }
    std::cout << "\n";
    std::cout << "M1B boundary_gpu_ns=";
    for (std::size_t index = 0u; index < PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT; ++index) {
        if (index != 0u) std::cout << ',';
        std::cout << evidence.m1b_boundary_gpu_ns[index];
    }
    std::cout << "\n";
    ASSERT_EQUAL(13u, evidence.weight_upload_count, "the one NoiseRefiner0 owner uploads its declared thirteen-tensor package exactly once");
    ASSERT_EQUAL(0u, evidence.warm_buffer_allocation_count, "first real execution performs no warm buffer allocation");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID), "real resident M1B resources destroy safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "real M1B runtime destroys safely");
}

FACT(PrometheusM2CSessionKeepsClosedSlotsAndRejectsMissingOrStaleJointInputs)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "M2C session runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    PrometheusCompiledModelSessionCreateRequest create{};
    create.struct_size = sizeof(create);
    create.execution_profile = PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY;
    create.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID ^ 1u;
    PrometheusCompiledModelSessionEvidence evidence{};
    std::uint64_t sessionID = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_compiled_model_session_create(
                                 runtime, &create, &sessionID, &evidence),
                 "M2C session rejects a foreign lock before allocating streams");
    create.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_create(
                                runtime, &create, &sessionID, &evidence),
                 "M2C session preallocates the lock-defined image context and joint slots");
    ASSERT_TRUE(sessionID != 0u && evidence.prepared_image_bytes == 1024ull * 3840ull * sizeof(float) &&
                    evidence.prepared_context_bytes == 32ull * 3840ull * sizeof(float) &&
                    evidence.joint_bytes == (1024ull + 32ull) * 3840ull * sizeof(float),
                "M2C physical joint plan freezes Image then Context FP32 extents");
    ASSERT_EQUAL(3u, evidence.cold_buffer_allocation_count, "M2C allocates all resident stream slots during cold setup");
    PrometheusCompiledModelSessionComposeRequest compose{};
    compose.struct_size = sizeof(compose);
    compose.required_image_generation = 1u;
    compose.required_context_generation = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_compiled_model_session_compose_joint(
                                 runtime, sessionID, &compose, &evidence),
                 "M2C rejects a joint when one or both closed producer slots are missing");
    ASSERT_EQUAL(PROM_MODEL_SESSION_DETAIL_STALE_STREAM, evidence.detail_code,
                 "M2C missing stream rejection has a distinct stale-generation identity");
    const std::uint64_t retainedSessionID = sessionID;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_compiled_model_session_create(
                                 runtime, &create, &sessionID, &evidence),
                 "M2C runtime admits only one closed session owner");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_destroy(runtime, retainedSessionID),
                 "M2C session releases all three resident slots safely");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_compiled_model_session_destroy(runtime, retainedSessionID),
                 "M2C repeated session destruction cannot transfer ownership twice");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "M2C session runtime destroys safely");
}

FACT(PrometheusM2CMainTransformerFacadeRejectsUnresolvedOrPartialRequests)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "M2C main runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }

    PrometheusModelBlockEvidence evidence{};
    std::uint64_t blockID = 0u;
    PrometheusMainTransformerCreateRequest create{};
    create.struct_size = sizeof(create);
    create.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID ^ 1u;
    create.model_local_block_id = 0u;
    create.upload_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_main_transformer_create(
                                 runtime, &create, &blockID, &evidence),
                 "MainTransformer facade rejects a foreign lock before owner mutation");
    ASSERT_EQUAL(0u, blockID, "foreign lock does not allocate a representative owner");

    create.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    create.model_local_block_id = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_main_transformer_create(
                                 runtime, &create, &blockID, &evidence),
                 "MainTransformer facade rejects runtime-selected topology before mutation");
    ASSERT_EQUAL(0u, blockID, "wrong representative id leaves the model owner empty");

    create.model_local_block_id = 0u;
    create.upload_count = PROM_MODEL_BLOCK_MAX_WEIGHTS - 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_main_transformer_create(
                                 runtime, &create, &blockID, &evidence),
                 "MainTransformer facade rejects a partial layers.0 payload before mutation");
    ASSERT_EQUAL(0u, blockID, "partial payload cannot install a parameter set");

    PrometheusMainTransformerExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.session_identity = 1u;
    execute.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    execute.model_local_block_id = 0u;
    execute.required_image_generation = 1u;
    execute.required_context_generation = 1u;
    execute.required_joint_generation = 1u;
    std::array<std::uint16_t, 256> timestep{};
    execute.timestep_bf16 = timestep.data();
    execute.timestep_bytes = timestep.size() * sizeof(std::uint16_t);
    execute.timestep_identity = 0x4d32435f74696d65ull;
    execute.output_identity = 0x4d32435f6d61696eull;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_main_transformer_execute(
                                 runtime, blockID, &execute, &evidence),
                 "MainTransformer execute rejects before mutation when no closed owner exists");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, evidence.detail_code,
                 "missing representative owner has an exact failure identity");

    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "M2C main runtime destroys safely");
}

FACT(PrometheusM2CRealRetainedStreamsFeedRepresentativeMainTransformer)
{
    const char* enabled = std::getenv("OCT_EVT2_M2C_REAL");
    const char* cacheRootText = std::getenv("OCT_EVT2_CACHE");
    const char* oracleRootText = std::getenv("OCT_EVT2_ORACLE");
    if (enabled == nullptr || std::string(enabled) != "1" || cacheRootText == nullptr || oracleRootText == nullptr) {
        SKIP("set OCT_EVT2_M2C_REAL=1, OCT_EVT2_CACHE, and OCT_EVT2_ORACLE for the real retained MainTransformer lane");
    }
    const std::filesystem::path cacheRoot(cacheRootText);
    const std::filesystem::path oracleRoot(oracleRootText);
    const std::filesystem::path checkpointRoot = cacheRoot / "layers" /
        "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6";
    constexpr std::array<const char*, PROM_MODEL_BLOCK_MAX_WEIGHTS> noise0Names{
        "noise_refiner.0.adaLN_modulation.0.bias.fp16.bin",
        "noise_refiner.0.adaLN_modulation.0.weight.fp16.bin",
        "noise_refiner.0.attention.k_norm.weight.fp16.bin",
        "noise_refiner.0.attention.out.weight.fp16.bin",
        "noise_refiner.0.attention.q_norm.weight.fp16.bin",
        "noise_refiner.0.attention.qkv.weight.fp16.bin",
        "noise_refiner.0.attention_norm1.weight.fp16.bin",
        "noise_refiner.0.attention_norm2.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w1.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w2.weight.fp16.bin",
        "noise_refiner.0.feed_forward.w3.weight.fp16.bin",
        "noise_refiner.0.ffn_norm1.weight.fp16.bin",
        "noise_refiner.0.ffn_norm2.weight.fp16.bin"};
    std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> noise0WeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> noise0Uploads{};
    for (std::uint32_t index = 0u; index < noise0Names.size(); ++index) {
        noise0WeightBytes[index] = read_binary_file(checkpointRoot / "noise_refiner.0" / noise0Names[index]);
        ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(noise0WeightBytes[index].size()),
                     "NoiseRefiner0 cache tensor has its declared size");
    }
    const std::vector<std::uint8_t> modelInput = read_binary_file(oracleRoot / "run_02" / "noise_refiner_0_input.bin");
    const std::vector<std::uint8_t> timestep = read_binary_file(oracleRoot / "run_02" / "noise_refiner_0_timestep.bin");
    ASSERT_EQUAL(1024u * 3840u * sizeof(std::uint16_t), static_cast<std::uint64_t>(modelInput.size()),
                 "M2C real image ingress BF16 payload exists");
    ASSERT_EQUAL(256u * sizeof(std::uint16_t), static_cast<std::uint64_t>(timestep.size()),
                 "M2C real timestep BF16 payload exists");
    if (modelInput.size() != 1024u * 3840u * sizeof(std::uint16_t) ||
        timestep.size() != 256u * sizeof(std::uint16_t)) return;

    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "M2C clean runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    PrometheusCompiledModelSessionCreateRequest sessionCreate{};
    sessionCreate.struct_size = sizeof(sessionCreate);
    sessionCreate.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    sessionCreate.execution_profile = PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY;
#if defined(PROMETHEUS_DVT2_M5B_BUILTIN_TOPOLOGY_EXPERIMENT)
    sessionCreate.main_attention_route_policy = PROM_MAIN_ATTENTION_ROUTE_BUILTIN_TOPOLOGY;
#else
    sessionCreate.main_attention_route_policy = PROM_MAIN_ATTENTION_ROUTE_AUTO;
#endif
    PrometheusCompiledModelSessionEvidence sessionEvidence{};
    std::uint64_t sessionID = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_create(
                              runtime, &sessionCreate, &sessionID, &sessionEvidence),
                 "M2C clean process creates the closed compiled-model session");
    ASSERT_TRUE(sessionID != 0u && sessionEvidence.prepared_image_bytes == 15728640ull &&
                    sessionEvidence.prepared_context_bytes == 491520ull &&
                    sessionEvidence.joint_bytes == 16220160ull,
                "M2C session exposes only the three lock-defined resident stream slots");

    PrometheusModelBlockCreateRequest noiseCreate = make_m1b_request();
    noiseCreate.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
    std::uint64_t noiseBlockID = 0u;
    PrometheusModelBlockEvidence evidence{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &noiseCreate, &noiseBlockID, &evidence),
                 "NoiseRefiner0 owner creates for M2C retained image preparation");
    for (std::uint32_t index = 0u; index < noise0Uploads.size(); ++index) {
        noise0Uploads[index].binding_index = index;
        noise0Uploads[index].bytes = noise0WeightBytes[index].data();
        noise0Uploads[index].byte_count = noise0WeightBytes[index].size();
        noise0Uploads[index].content_identity = noiseCreate.weights[index].content_identity;
        noise0Uploads[index].layout_identity = noiseCreate.weights[index].layout_identity;
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_upload_weights(
                              runtime, noiseBlockID, noise0Uploads.data(), static_cast<std::uint32_t>(noise0Uploads.size()), &evidence),
                 "NoiseRefiner0 validated payload uploads before M2C execution");
    PrometheusNoiseRefiner0ExecuteRequest noise0{};
    noise0.struct_size = sizeof(noise0);
    noise0.model_input_bf16 = modelInput.data();
    noise0.timestep_bf16 = timestep.data();
    noise0.model_input_bytes = modelInput.size();
    noise0.timestep_bytes = timestep.size();
    noise0.input_identity = 0x857cea75e69d665cull;
    noise0.timestep_identity = 0xbc0ba90e94f5ae98ull;
    noise0.output_identity = 0xa4fd07d58b1c9e23ull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner0_execute(runtime, noiseBlockID, &noise0, &evidence),
                 "NoiseRefiner0 produces the real retained image predecessor without host intermediate arithmetic");
    const std::uint64_t noise0Generation = evidence.output_generation;

    std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> noise1WeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> noise1Uploads{};
    for (std::uint32_t index = 0u; index < noise0Names.size(); ++index) {
        std::string name(noise0Names[index]);
        name.replace(0u, std::string("noise_refiner.0").size(), "noise_refiner.1");
        noise1WeightBytes[index] = read_binary_file(checkpointRoot / "noise_refiner.1" / name);
        ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(noise1WeightBytes[index].size()),
                     "NoiseRefiner1 cache tensor has its declared size");
        noise1Uploads[index].binding_index = index;
        noise1Uploads[index].bytes = noise1WeightBytes[index].data();
        noise1Uploads[index].byte_count = noise1WeightBytes[index].size();
        noise1Uploads[index].content_identity = 0x8000u + index;
        noise1Uploads[index].layout_identity = 0x8100u + index;
    }
    PrometheusNoiseRefinerRebindRequest noiseRebind{};
    noiseRebind.struct_size = sizeof(noiseRebind);
    noiseRebind.model_local_block_id = 1u;
    noiseRebind.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    noiseRebind.upload_count = static_cast<std::uint32_t>(noise1Uploads.size());
    noiseRebind.uploads = noise1Uploads.data();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_rebind(runtime, noiseBlockID, &noiseRebind, &evidence),
                 "NoiseRefiner1 parameter set binds atomically for M2C");
    PrometheusNoiseRefinerResidentExecuteRequest noise1{};
    noise1.struct_size = sizeof(noise1);
    noise1.input_generation = noise0Generation;
    noise1.output_identity = 0x9b133c9ed3772f78ull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_noise_refiner_execute_resident(runtime, noiseBlockID, &noise1, &evidence),
                 "NoiseRefiner1 consumes the retained image stream without host reconstruction");
    PrometheusCompiledModelSessionCaptureRequest capture{};
    capture.struct_size = sizeof(capture);
    capture.source_output_generation = evidence.output_generation;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_capture_completed(
                              runtime, sessionID, noiseBlockID, &capture, &sessionEvidence),
                 "NoiseRefiner1 final captures into PreparedImage");
    const std::uint64_t preparedImageGeneration = sessionEvidence.prepared_image_generation;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, noiseBlockID),
                 "NoiseRefiner owner is destroyed after image capture without destroying the session");

    constexpr std::array<const char*, 11> context0Names{{
        "context_refiner.0.attention.k_norm.weight.fp16.bin", "context_refiner.0.attention.out.weight.fp16.bin",
        "context_refiner.0.attention.q_norm.weight.fp16.bin", "context_refiner.0.attention.qkv.weight.fp16.bin",
        "context_refiner.0.attention_norm1.weight.fp16.bin", "context_refiner.0.attention_norm2.weight.fp16.bin",
        "context_refiner.0.feed_forward.w1.weight.fp16.bin", "context_refiner.0.feed_forward.w2.weight.fp16.bin",
        "context_refiner.0.feed_forward.w3.weight.fp16.bin", "context_refiner.0.ffn_norm1.weight.fp16.bin",
        "context_refiner.0.ffn_norm2.weight.fp16.bin"}};
    constexpr std::array<std::uint64_t, 11> contextBytes{{256u,29491200u,256u,88473600u,7680u,7680u,78643200u,78643200u,78643200u,7680u,7680u}};
    std::array<std::vector<std::uint8_t>, 11> context0WeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, 11> context0Uploads{};
    for (std::uint32_t index = 0u; index < context0Names.size(); ++index) {
        context0WeightBytes[index] = read_binary_file(checkpointRoot / "context_refiner.0" / context0Names[index]);
        ASSERT_EQUAL(contextBytes[index], static_cast<std::uint64_t>(context0WeightBytes[index].size()),
                     "ContextRefiner0 cache tensor has its declared size");
        context0Uploads[index].binding_index = index;
        context0Uploads[index].bytes = context0WeightBytes[index].data();
        context0Uploads[index].byte_count = context0WeightBytes[index].size();
        context0Uploads[index].content_identity = 0x7100u + index;
        context0Uploads[index].layout_identity = 0x7200u + index;
    }
    const std::filesystem::path context0Canonical = cacheRoot / "canonical" / "f332072aa78be7aecdf3ee76d5c247082da564a6" /
        "m2b-fp32-reference" / "context_refiner.0";
    const std::vector<std::uint8_t> contextInput = read_binary_file(context0Canonical / "input.f32.bin");
    ASSERT_EQUAL(32u * 3840u * sizeof(float), static_cast<std::uint64_t>(contextInput.size()),
                 "ContextRefiner0 real context ingress exists");
    PrometheusContextRefinerCreateRequest contextCreate{};
    contextCreate.struct_size = sizeof(contextCreate);
    contextCreate.model_local_block_id = 0u;
    contextCreate.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    contextCreate.upload_count = static_cast<std::uint32_t>(context0Uploads.size());
    contextCreate.uploads = context0Uploads.data();
    std::uint64_t contextBlockID = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_create(runtime, &contextCreate, &contextBlockID, &evidence),
                 "ContextRefiner0 owner creates for M2C retained context preparation");
    PrometheusContextRefiner0ExecuteRequest context0{};
    context0.struct_size = sizeof(context0);
    context0.context_input = reinterpret_cast<const float*>(contextInput.data());
    context0.context_input_bytes = contextInput.size();
    context0.input_identity = 0xf6e4a2842dbbdfa7ull;
    context0.output_identity = 0xd2b8167de614da25ull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner0_execute(runtime, contextBlockID, &context0, &evidence),
                 "ContextRefiner0 prepares the real context stream");
    const std::uint64_t context0Generation = evidence.output_generation;
    std::array<std::vector<std::uint8_t>, 11> context1WeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, 11> context1Uploads{};
    for (std::uint32_t index = 0u; index < context0Names.size(); ++index) {
        std::string name(context0Names[index]);
        name.replace(0u, std::string("context_refiner.0").size(), "context_refiner.1");
        context1WeightBytes[index] = read_binary_file(checkpointRoot / "context_refiner.1" / name);
        ASSERT_EQUAL(contextBytes[index], static_cast<std::uint64_t>(context1WeightBytes[index].size()),
                     "ContextRefiner1 cache tensor has its declared size");
        context1Uploads[index].binding_index = index;
        context1Uploads[index].bytes = context1WeightBytes[index].data();
        context1Uploads[index].byte_count = context1WeightBytes[index].size();
        context1Uploads[index].content_identity = 0x7300u + index;
        context1Uploads[index].layout_identity = 0x7400u + index;
    }
    PrometheusContextRefinerRebindRequest contextRebind{};
    contextRebind.struct_size = sizeof(contextRebind);
    contextRebind.model_local_block_id = 1u;
    contextRebind.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    contextRebind.upload_count = static_cast<std::uint32_t>(context1Uploads.size());
    contextRebind.uploads = context1Uploads.data();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_rebind(runtime, contextBlockID, &contextRebind, &evidence),
                 "ContextRefiner1 parameter set binds atomically for M2C");
    PrometheusContextRefinerResidentExecuteRequest context1{};
    context1.struct_size = sizeof(context1);
    context1.input_generation = context0Generation;
    context1.output_identity = 0x08377e8a46b65cffull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_context_refiner_execute_resident(runtime, contextBlockID, &context1, &evidence),
                 "ContextRefiner1 consumes retained context without host reconstruction");
    capture.source_output_generation = evidence.output_generation;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_capture_completed(
                              runtime, sessionID, contextBlockID, &capture, &sessionEvidence),
                 "ContextRefiner1 final captures into PreparedContext without invalidating PreparedImage");
    const std::uint64_t preparedContextGeneration = sessionEvidence.prepared_context_generation;
    ASSERT_EQUAL(preparedImageGeneration, sessionEvidence.prepared_image_generation,
                 "capturing PreparedContext does not invalidate PreparedImage");
    PrometheusCompiledModelSessionComposeRequest compose{};
    compose.struct_size = sizeof(compose);
    compose.required_image_generation = preparedImageGeneration;
    compose.required_context_generation = preparedContextGeneration;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_compose_joint(
                              runtime, sessionID, &compose, &sessionEvidence),
                 "M2C composes JointWorking on device in Image then Context order");
    ASSERT_EQUAL(1u, sessionEvidence.joint_generation, "first physical joint composition owns generation one");
    const std::uint64_t jointGeneration = sessionEvidence.joint_generation;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, contextBlockID),
                 "ContextRefiner owner is destroyed after context capture without destroying the session");

    std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> mainWeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> mainUploads{};
    for (std::uint32_t index = 0u; index < noise0Names.size(); ++index) {
        std::string name(noise0Names[index]);
        name.replace(0u, std::string("noise_refiner.0").size(), "layers.0");
        mainWeightBytes[index] = read_binary_file(checkpointRoot / "layers.0" / name);
        ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(mainWeightBytes[index].size()),
                     "MainTransformer layers.0 cache tensor has its declared size");
        mainUploads[index].binding_index = index;
        mainUploads[index].bytes = mainWeightBytes[index].data();
        mainUploads[index].byte_count = mainWeightBytes[index].size();
        mainUploads[index].content_identity = 0x9000u + index;
        mainUploads[index].layout_identity = 0x9100u + index;
    }
    PrometheusMainTransformerCreateRequest mainCreate{};
    mainCreate.struct_size = sizeof(mainCreate);
    mainCreate.model_local_block_id = 0u;
    mainCreate.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    mainCreate.upload_count = static_cast<std::uint32_t>(mainUploads.size());
    mainCreate.uploads = mainUploads.data();
    std::uint64_t mainBlockID = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_create(runtime, &mainCreate, &mainBlockID, &evidence),
                 "MainTransformer0 layers.0 owner creates from the lock-resolved descriptor");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_get_evidence(runtime, sessionID, &sessionEvidence),
                 "MainTransformer route selection is available after owner creation");
#if defined(PROMETHEUS_DVT2_M5B_BUILTIN_TOPOLOGY_EXPERIMENT)
    ASSERT_EQUAL(PROM_MAIN_ATTENTION_ROUTE_BUILTIN_TOPOLOGY, sessionEvidence.selected_main_attention_route,
                 "builtin-topology route remains selected without a topology-probe execution runner");
    ASSERT_EQUAL(49u, sessionEvidence.main_attention_shader_id,
                 "builtin-topology route binds the isolated payload identity");
#endif
    PrometheusMainTransformerExecuteRequest mainExecute{};
    mainExecute.struct_size = sizeof(mainExecute);
    mainExecute.session_identity = sessionID;
    mainExecute.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    mainExecute.model_local_block_id = 0u;
    mainExecute.required_image_generation = preparedImageGeneration;
    mainExecute.required_context_generation = preparedContextGeneration;
    mainExecute.required_joint_generation = jointGeneration;
    mainExecute.timestep_bf16 = timestep.data();
    mainExecute.timestep_bytes = timestep.size();
    mainExecute.timestep_identity = 0xbc0ba90e94f5ae98ull;
    mainExecute.output_identity = 0x4d32435f6d61696eull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(runtime, mainBlockID, &mainExecute, &evidence),
                 "MainTransformer0 consumes real retained JointWorking without host activation bounce");
    const std::uint64_t mainOutputGeneration = evidence.output_generation;
    std::vector<float> finalJoint(1056u * 3840u);
    PrometheusMainTransformerFinalAuditRequest finalAudit{};
    finalAudit.struct_size = sizeof(finalAudit);
    finalAudit.required_output_generation = mainOutputGeneration;
    finalAudit.output_identity = mainExecute.output_identity;
    finalAudit.output = finalJoint.data();
    finalAudit.output_element_capacity = finalJoint.size();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_audit_final(runtime, mainBlockID, &finalAudit, &evidence),
                 "MainTransformer final audit reads back only the completed representative joint output");
    const std::filesystem::path mainCanonical = cacheRoot / "canonical" / "f332072aa78be7aecdf3ee76d5c247082da564a6" /
        "m2c-fp32-reference" / "layers.0" / "stages" / "final_joint_output.f32.bin";
    const std::vector<std::uint8_t> referenceJoint = read_binary_file(mainCanonical);
    ASSERT_EQUAL(static_cast<std::uint64_t>(finalJoint.size()) * sizeof(float),
                 static_cast<std::uint64_t>(referenceJoint.size()),
                 "M2C final joint FP32 oracle exists with exact physical size");
    const ComparisonMetrics joint = compare_float_region(finalJoint.data(), referenceJoint, 0u, finalJoint.size());
    const ComparisonMetrics image = compare_float_region(finalJoint.data(), referenceJoint, 0u, 1024u * 3840u);
    const ComparisonMetrics contextRegion = compare_float_region(finalJoint.data(), referenceJoint, 1024u * 3840u, 32u * 3840u);
    const ComparisonMetrics lastImageToken = compare_float_region(finalJoint.data(), referenceJoint, 1023u * 3840u, 3840u);
    const ComparisonMetrics firstContextToken = compare_float_region(finalJoint.data(), referenceJoint, 1024u * 3840u, 3840u);
    std::cout << "M2C final_joint finite=" << joint.finite << " relative_l2=" << joint.relativeL2
              << " linf=" << joint.linf << " accepted_threshold=5e-5 first_mismatch=" << joint.firstMismatch << "\n";
    std::cout << "M2C image_region relative_l2=" << image.relativeL2 << " linf=" << image.linf
              << " context_region_relative_l2=" << contextRegion.relativeL2 << " context_region_linf=" << contextRegion.linf
              << " last_image_token_relative_l2=" << lastImageToken.relativeL2
              << " first_context_token_relative_l2=" << firstContextToken.relativeL2 << "\n";
    ASSERT_TRUE(joint.finite && joint.relativeL2 <= 5.0e-5 && image.relativeL2 <= 5.0e-5 &&
                    contextRegion.relativeL2 <= 5.0e-5,
                "MainTransformer0 representative output matches the deterministic FP32 joint oracle");

    std::vector<float> staticAuditJoint(1056u * 3840u);
    PrometheusMainTransformerStaticAuditRequest staticAudit{};
    staticAudit.struct_size = sizeof(staticAudit);
    staticAudit.session_identity = sessionID;
    staticAudit.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    staticAudit.model_local_block_id = 0u;
    staticAudit.required_image_generation = preparedImageGeneration;
    staticAudit.required_context_generation = preparedContextGeneration;
    staticAudit.required_joint_generation = jointGeneration;
    staticAudit.output_identity = 0x4d32435f73746174ull;
    staticAudit.audit_arena = staticAuditJoint.data();
    staticAudit.audit_arena_capacity_bytes = staticAuditJoint.size() * sizeof(float);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute_static_audit(
                              runtime, mainBlockID, &staticAudit, &evidence),
                 "MainTransformer0 static replay reads the lock-bound resident joint without activation reconstruction");
    const ComparisonMetrics staticJoint = compare_float_region(staticAuditJoint.data(), referenceJoint, 0u, staticAuditJoint.size());
    std::cout << "M2D static_final finite=" << staticJoint.finite << " relative_l2=" << staticJoint.relativeL2
              << " linf=" << staticJoint.linf << " accepted_threshold=5e-5\n";
    ASSERT_TRUE(staticJoint.finite && staticJoint.relativeL2 <= 5.0e-5,
                "MainTransformer0 static replay preserves the accepted final gated-residual boundary");

    const std::uint64_t warmAllocations = evidence.warm_buffer_allocation_count;
    const std::uint64_t warmUploads = evidence.weight_upload_count;
    const std::uint64_t warmPipelines = evidence.pipeline_create_count;
    const std::uint64_t warmDescriptors = evidence.descriptor_set_count;
    std::array<std::uint64_t, 10> warmNs{};
    std::array<std::uint64_t, 10> warmGpuNs{};
    for (std::uint32_t iteration = 0u; iteration < warmNs.size(); ++iteration) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(runtime, mainBlockID, &mainExecute, &evidence),
                     "warm MainTransformer execution reuses retained streams and bound parameters");
        warmNs[iteration] = evidence.last_execution_ns;
        warmGpuNs[iteration] = evidence.gpu_compute_ns;
    }
    ASSERT_EQUAL(warmAllocations, evidence.warm_buffer_allocation_count, "warm M2C execution allocates no buffers");
    ASSERT_EQUAL(warmUploads, evidence.weight_upload_count, "warm M2C execution uploads no weights");
    ASSERT_EQUAL(warmPipelines, evidence.pipeline_create_count, "warm M2C execution creates no pipelines");
    ASSERT_EQUAL(warmDescriptors, evidence.descriptor_set_count, "warm M2C execution grows no descriptor pools");
    std::array<std::uint64_t, 10> sortedWarm = warmNs;
    std::array<std::uint64_t, 10> sortedWarmGpu = warmGpuNs;
    std::sort(sortedWarm.begin(), sortedWarm.end());
    std::sort(sortedWarmGpu.begin(), sortedWarmGpu.end());
    std::cout << "M2C warm_ns=";
    for (std::size_t index = 0u; index < warmNs.size(); ++index) {
        if (index != 0u) std::cout << ',';
        std::cout << warmNs[index];
    }
    std::cout << "\nM2C warm_stats_ns median=" << (sortedWarm[4u] + sortedWarm[5u]) / 2u
              << " mean=" << mean_ns(warmNs) << " min=" << sortedWarm.front()
              << " p95=" << sortedWarm[9u] << " gpu_median="
              << (sortedWarmGpu[4u] + sortedWarmGpu[5u]) / 2u
              << " gpu_mean=" << mean_ns(warmGpuNs) << " churn=zero\n";
    if (const char* alternating = std::getenv("OCT_EVT2_M5B_ALTERNATING");
        alternating != nullptr && std::string(alternating) == "1") {
        const auto run_route = [&](std::uint32_t route, bool audit, std::uint32_t sample,
                                   std::uint64_t* out_ns) {
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, mainBlockID),
                         "alternating M5b releases the previous MainTransformer owner");
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_set_main_attention_route(
                                      runtime, sessionID, route, &sessionEvidence),
                         "alternating M5b changes only the unmaterialized route on the retained session");
            mainBlockID = 0u;
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_create(
                                      runtime, &mainCreate, &mainBlockID, &evidence),
                         "alternating M5b recreates MainTransformer from identical retained inputs and weights");
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_get_evidence(
                                      runtime, sessionID, &sessionEvidence),
                         "alternating M5b records selected payload identity");
            ASSERT_EQUAL(route, sessionEvidence.selected_main_attention_route,
                         "alternating M5b selected the requested admitted route");
            ASSERT_EQUAL(route == PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL ? 41u : 49u,
                         sessionEvidence.main_attention_shader_id,
                         "alternating M5b selected the expected isolated payload identity");
            ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(
                                      runtime, mainBlockID, &mainExecute, &evidence),
                         "alternating M5b executes the same layer-0 retained boundary");
            *out_ns = evidence.last_execution_ns;
            if (audit) {
                std::vector<float> alternateAuditJoint(1056u * 3840u);
                finalAudit.required_output_generation = evidence.output_generation;
                finalAudit.output_identity = 0x4d32435f616c7400ull + sample;
                finalAudit.output = alternateAuditJoint.data();
                ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_audit_final(
                                          runtime, mainBlockID, &finalAudit, &evidence),
                             "alternating M5b representative output audit succeeds");
                const ComparisonMetrics alternateJoint = compare_float_region(
                    alternateAuditJoint.data(), referenceJoint, 0u, alternateAuditJoint.size());
                ASSERT_TRUE(alternateJoint.finite && alternateJoint.relativeL2 <= 5.0e-5,
                            "alternating M5b representative output remains numerically authoritative");
            }
        };
        std::array<std::uint64_t, 20> serialAlternatingNs{};
        std::array<std::uint64_t, 20> builtinAlternatingNs{};
        for (std::uint32_t warm = 0u; warm < 4u; ++warm) {
            std::uint64_t discarded = 0u;
            run_route(PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL, false, warm, &discarded);
            run_route(PROM_MAIN_ATTENTION_ROUTE_BUILTIN_TOPOLOGY, false, warm, &discarded);
        }
        for (std::uint32_t sample = 0u; sample < serialAlternatingNs.size(); ++sample) {
            run_route(PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL, sample == 0u, sample, &serialAlternatingNs[sample]);
            run_route(PROM_MAIN_ATTENTION_ROUTE_BUILTIN_TOPOLOGY, sample == 0u, sample, &builtinAlternatingNs[sample]);
        }
        std::cout << "M2C alternating_boundary=MainTransformer layer-0 execute last_execution_ns; "
                     "one retained compiled session, prepared streams, joint generation, Vulkan runtime, weights, "
                     "and 4 alternating warm-up pairs; owner creation/destruction and audits excluded\n";
        std::cout << "M2C alternating_serial_ns=";
        for (std::size_t index = 0u; index < serialAlternatingNs.size(); ++index) {
            if (index != 0u) std::cout << ',';
            std::cout << serialAlternatingNs[index];
        }
        std::cout << "\nM2C alternating_builtin_topology_ns=";
        for (std::size_t index = 0u; index < builtinAlternatingNs.size(); ++index) {
            if (index != 0u) std::cout << ',';
            std::cout << builtinAlternatingNs[index];
        }
        std::cout << "\nM2C alternating_selected_serial=41 alternating_selected_builtin_topology=49\n";
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, mainBlockID),
                     "alternating M5b final MainTransformer owner destroys safely");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_destroy(runtime, sessionID),
                     "alternating M5b retained session destroys safely");
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime),
                     "alternating M5b runtime destroys safely");
        return;
    }
    if (const char* bounded = std::getenv("OCT_EVT2_M5B_BOUNDED"); bounded != nullptr && std::string(bounded) == "1") {
        std::cout << "M2C bounded_m5b=1 route=" << sessionEvidence.selected_main_attention_route
                  << " shader_id=" << sessionEvidence.main_attention_shader_id << "\n";
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, mainBlockID),
                     "bounded M5b MainTransformer owner destroys safely");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_destroy(runtime, sessionID),
                     "bounded M5b compiled-model session destroys safely");
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime),
                     "bounded M5b runtime destroys safely");
        return;
    }
    std::array<std::uint64_t, 30> chainExecutionNs{};
    std::array<std::uint64_t, 30> chainGpuNs{};
    std::array<std::uint64_t, 29> chainRebindNs{};
    const auto chainStart = std::chrono::steady_clock::now();
    mainExecute.resident_chain_mode = 1u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(runtime, mainBlockID, &mainExecute, &evidence),
                 "chain mode advances JointWorking from layers.0 without host reconstruction");
    chainExecutionNs[0u] = evidence.last_execution_ns;
    chainGpuNs[0u] = evidence.gpu_compute_ns;
    const std::uint64_t chainedJointGeneration = evidence.output_generation == 0u ? 0u : jointGeneration + 1u;
    std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> main1WeightBytes{};
    std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> main1Uploads{};
    for (std::uint32_t index = 0u; index < noise0Names.size(); ++index) {
        std::string name(noise0Names[index]);
        name.replace(0u, std::string("noise_refiner.0").size(), "layers.1");
        main1WeightBytes[index] = read_binary_file(checkpointRoot / "layers.1" / name);
        ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(main1WeightBytes[index].size()),
                     "MainTransformer layers.1 cache tensor has its declared size");
        main1Uploads[index].binding_index = index;
        main1Uploads[index].bytes = main1WeightBytes[index].data();
        main1Uploads[index].byte_count = main1WeightBytes[index].size();
        main1Uploads[index].content_identity = 0x9200u + index;
        main1Uploads[index].layout_identity = 0x9300u + index;
    }
    PrometheusMainTransformerRebindRequest mainRebind{};
    mainRebind.struct_size = sizeof(mainRebind);
    mainRebind.model_local_block_id = 1u;
    mainRebind.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
    mainRebind.upload_count = static_cast<std::uint32_t>(main1Uploads.size());
    mainRebind.uploads = main1Uploads.data();
    const auto firstRebindStart = std::chrono::steady_clock::now();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_rebind(runtime, mainBlockID, &mainRebind, &evidence),
                 "MainTransformer rebind accepts only the immediate layers.1 successor");
    chainRebindNs[0u] = static_cast<std::uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - firstRebindStart).count());
    mainExecute.model_local_block_id = 1u;
    mainExecute.required_joint_generation = chainedJointGeneration;
    mainExecute.output_identity = 0x4d32435f6d616e31ull;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(runtime, mainBlockID, &mainExecute, &evidence),
                 "layers.1 consumes the layers.0 device-resident JointWorking generation");
    chainExecutionNs[1u] = evidence.last_execution_ns;
    chainGpuNs[1u] = evidence.gpu_compute_ns;
    std::uint64_t chainJointGeneration = chainedJointGeneration + 1u;
    for (std::uint32_t layer = 2u; layer < 30u; ++layer) {
        std::array<std::vector<std::uint8_t>, PROM_MODEL_BLOCK_MAX_WEIGHTS> layerWeightBytes{};
        std::array<PrometheusModelBlockWeightUpload, PROM_MODEL_BLOCK_MAX_WEIGHTS> layerUploads{};
        for (std::uint32_t index = 0u; index < noise0Names.size(); ++index) {
            std::string name(noise0Names[index]);
            name.replace(0u, std::string("noise_refiner.0").size(), "layers." + std::to_string(layer));
            layerWeightBytes[index] = read_binary_file(checkpointRoot / ("layers." + std::to_string(layer)) / name);
            ASSERT_EQUAL(kM1BWeightBytes[index], static_cast<std::uint64_t>(layerWeightBytes[index].size()),
                         "MainTransformer successor cache tensor has its declared size");
            layerUploads[index].binding_index = index;
            layerUploads[index].bytes = layerWeightBytes[index].data();
            layerUploads[index].byte_count = layerWeightBytes[index].size();
            layerUploads[index].content_identity = 0x9400u + layer * 32u + index;
            layerUploads[index].layout_identity = 0x9800u + layer * 32u + index;
        }
        PrometheusMainTransformerRebindRequest rebind{};
        rebind.struct_size = sizeof(rebind);
        rebind.model_local_block_id = layer;
        rebind.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
        rebind.upload_count = static_cast<std::uint32_t>(layerUploads.size());
        rebind.uploads = layerUploads.data();
        const auto rebindStart = std::chrono::steady_clock::now();
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_rebind(runtime, mainBlockID, &rebind, &evidence),
                     "MainTransformer transition follows the lock-resolved immediate successor");
        chainRebindNs[layer - 1u] = static_cast<std::uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - rebindStart).count());
        mainExecute.model_local_block_id = layer;
        mainExecute.required_joint_generation = chainJointGeneration;
        mainExecute.output_identity = 0x4d32435f6d616e30ull + layer;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_execute(runtime, mainBlockID, &mainExecute, &evidence),
                     "MainTransformer successor consumes the preceding resident joint generation");
        chainExecutionNs[layer] = evidence.last_execution_ns;
        chainGpuNs[layer] = evidence.gpu_compute_ns;
        chainJointGeneration += 1u;
    }
    finalAudit.required_output_generation = evidence.output_generation;
    finalAudit.output_identity = mainExecute.output_identity;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_main_transformer_audit_final(runtime, mainBlockID, &finalAudit, &evidence),
                 "layer-29 final audit reads the completed resident chain output");
    const std::filesystem::path chainCanonical = cacheRoot / "canonical" / "f332072aa78be7aecdf3ee76d5c247082da564a6" /
        "m2d-fp32-reference" / "layers.29" / "stages" / "final_joint_output.f32.bin";
    const std::vector<std::uint8_t> chainReference = read_binary_file(chainCanonical);
    ASSERT_EQUAL(static_cast<std::uint64_t>(finalJoint.size()) * sizeof(float),
                 static_cast<std::uint64_t>(chainReference.size()),
                 "M2D layer-29 FP32 authority has exact physical joint size");
    const ComparisonMetrics chainJoint = compare_float_region(finalJoint.data(), chainReference, 0u, finalJoint.size());
    const ComparisonMetrics chainImage = compare_float_region(finalJoint.data(), chainReference, 0u, 1024u * 3840u);
    const ComparisonMetrics chainContext = compare_float_region(finalJoint.data(), chainReference, 1024u * 3840u, 32u * 3840u);
    const ComparisonMetrics chainLastImage = compare_float_region(finalJoint.data(), chainReference, 1023u * 3840u, 3840u);
    const ComparisonMetrics chainFirstContext = compare_float_region(finalJoint.data(), chainReference, 1024u * 3840u, 3840u);
    std::cout << "M2D final_joint finite=" << chainJoint.finite << " relative_l2=" << chainJoint.relativeL2
              << " linf=" << chainJoint.linf << " image_relative_l2=" << chainImage.relativeL2
              << " context_relative_l2=" << chainContext.relativeL2 << " last_image_relative_l2="
              << chainLastImage.relativeL2 << " first_context_relative_l2=" << chainFirstContext.relativeL2
              << " first_mismatch=" << chainJoint.firstMismatch << " accepted_threshold=5e-5\n";
    ASSERT_TRUE(chainJoint.finite && chainJoint.relativeL2 <= 5.0e-5 && chainImage.relativeL2 <= 5.0e-5 &&
                    chainContext.relativeL2 <= 5.0e-5 && chainLastImage.relativeL2 <= 5.0e-5 &&
                    chainFirstContext.relativeL2 <= 5.0e-5,
                "the complete resident MainTransformer chain matches the layer-29 FP32 authority");
    const std::uint64_t chainElapsedNs = static_cast<std::uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - chainStart).count());
    const std::uint64_t chainExecutionTotalNs = std::accumulate(chainExecutionNs.begin(), chainExecutionNs.end(), std::uint64_t{0u});
    const std::uint64_t chainGpuTotalNs = std::accumulate(chainGpuNs.begin(), chainGpuNs.end(), std::uint64_t{0u});
    const std::uint64_t chainRebindTotalNs = std::accumulate(chainRebindNs.begin(), chainRebindNs.end(), std::uint64_t{0u});
    const std::uint64_t layerUploadBytes = 361820672ull;
    std::cout << "M2D chain_timing total_ns=" << chainElapsedNs << " execution_ns=" << chainExecutionTotalNs
              << " gpu_ns=" << chainGpuTotalNs
              << " rebind_ns=" << chainRebindTotalNs << " upload_bytes=" << layerUploadBytes * 29u
              << " rebind_upload_bandwidth_bytes_per_second="
              << (chainRebindTotalNs == 0u ? 0u : (layerUploadBytes * 29u * 1000000000ull) / chainRebindTotalNs)
              << " persistent_bytes=" << evidence.persistent_bytes
              << " reusable_bytes=" << evidence.reusable_bytes
              << " audit_bytes=" << evidence.audit_bytes
              << " external_bytes=" << evidence.external_bytes
              << " committed_bytes=" << evidence.total_committed_bytes << " peak_plan_bytes=" << evidence.peak_plan_bytes << "\n";
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, mainBlockID),
                 "MainTransformer owner destroys safely after retained-stream reuse");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_compiled_model_session_destroy(runtime, sessionID),
                 "M2C compiled-model session destroys all three resident slots safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "M2C runtime destroys safely");
}

FACT(PrometheusResidentModelBlockRejectsMalformedProgramAndPipelineFault)
{
    PrometheusModelBlockCreateRequest malformed = make_request();
    malformed.steps[3] = PROM_MODEL_BLOCK_STEP_OUTPUT_COPY;
    PrometheusModelBlockEvidence evidence{};
    std::uint64_t block_id = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_create(nullptr, &malformed, &block_id, &evidence), "malformed fixed program does not create a graph runtime");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, evidence.detail_code, "malformed plan has exact fault identity");

    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "pipeline fault runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_request();
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_create_fault_impl(
                              runtime, PROM_TESTCFG_FAIL_PIPELINE_CREATE),
                 "pipeline fault is injected only into the next resident block creation");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_model_block_create(runtime, &create, &block_id, &evidence), "injected pipeline creation failure cleans partial initialization");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_PIPELINE_CREATE_FAILED, evidence.detail_code, "pipeline fault has exact identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &block_id, &evidence), "module creation recovers after partial pipeline initialization");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, block_id), "recovered module destroys safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_model_block_test_inject_create_fault_impl(
                              runtime, PROM_TESTCFG_FAIL_UPLOAD),
                 "weight upload fault is injected only into the next resident block creation");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &block_id, &evidence), "upload fault module creates before immutable payload transfer");
    ASSERT_EQUAL(PROM_ERROR, upload_all(runtime, block_id, create, &evidence), "injected weight upload failure rejects the bundle");
    ASSERT_EQUAL(PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED, evidence.detail_code, "weight upload failure has exact identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, block_id), "failed upload module destroys safely");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "partial initialization teardown is safe");
}

FACT(PrometheusResidentModelBlockEmitsWarmTimingEvidence)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "timing runtime creates");
    if (runtime == nullptr || !runtime_available(runtime)) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("Vulkan runtime unavailable");
    }
    const PrometheusModelBlockCreateRequest create = make_request();
    PrometheusModelBlockEvidence evidence{};
    std::uint64_t blockID = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_create(runtime, &create, &blockID, &evidence), "timing resident block creates");
    if (blockID == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        return;
    }
    ASSERT_EQUAL(PROM_OK, upload_all(runtime, blockID, create, &evidence), "timing weights upload");
    std::vector<float> input(kElementCount), output(kElementCount);
    for (std::uint32_t i = 0u; i < kElementCount; ++i) input[i] = static_cast<float>(i % 31u) / 31.0f;
    PrometheusModelBlockExecuteRequest execute{};
    execute.struct_size = sizeof(execute);
    execute.input = input.data();
    execute.output = output.data();
    execute.element_count = input.size();
    execute.input_identity = 0x9911u;
    for (std::uint32_t warmup = 0u; warmup < 5u; ++warmup) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, blockID, &execute, &evidence), "timing warmup execution succeeds");
    }
    std::array<std::uint64_t, 10> samples{};
    for (std::uint32_t iteration = 0u; iteration < samples.size(); ++iteration) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_execute(runtime, blockID, &execute, &evidence), "timing measured warm execution succeeds");
        samples[iteration] = evidence.last_execution_ns;
    }
    std::array<std::uint64_t, 10> sorted = samples;
    std::sort(sorted.begin(), sorted.end());
    const double mean = static_cast<double>(std::accumulate(samples.begin(), samples.end(), std::uint64_t{0u})) / static_cast<double>(samples.size());
    double variance = 0.0;
    for (const std::uint64_t sample : samples) {
        const double delta = static_cast<double>(sample) - mean;
        variance += delta * delta;
    }
    variance /= static_cast<double>(samples.size());
    std::ostringstream artifact;
    artifact << "{\n"
             << "  \"metric\": \"host_observed_execute_ns\",\n"
             << "  \"warmup_count\": 5,\n"
             << "  \"sample_count\": 10,\n"
             << "  \"samples_ns\": [";
    for (std::size_t index = 0u; index < samples.size(); ++index) {
        if (index != 0u) artifact << ", ";
        artifact << samples[index];
    }
    artifact << "],\n"
             << "  \"min_ns\": " << sorted.front() << ",\n"
             << "  \"median_ns\": " << (sorted[4] + sorted[5]) / 2u << ",\n"
             << "  \"mean_ns\": " << mean << ",\n"
             << "  \"p95_ns\": " << sorted[9] << ",\n"
             << "  \"stddev_ns\": " << std::sqrt(variance) << ",\n"
             << "  \"warm_buffer_allocations\": " << evidence.warm_buffer_allocation_count << ",\n"
             << "  \"pipeline_creations\": " << evidence.pipeline_create_count << ",\n"
             << "  \"weight_uploads\": " << evidence.weight_upload_count << "\n"
             << "}\n";
    ASSERT_TRUE(context.WriteTextArtifact("resident_model_block_timing.json", artifact.str()), "timing artifact writes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_model_block_destroy(runtime, blockID), "timing block destroys");
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(runtime), "timing runtime destroys");
}
