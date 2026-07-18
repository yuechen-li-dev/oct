#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <cstdint>
#include <numeric>
#include <sstream>
#include <vector>

namespace
{
constexpr std::uint32_t kElementCount = 1024u;
constexpr std::uint32_t kAuditElements = 64u;
constexpr std::uint32_t kWeightBytes = 64u;

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

bool runtime_available(void* runtime)
{
    PrometheusCaps caps{};
    return prometheus_reactor_runtime_probe(runtime, &caps) == PROM_OK && caps.available != 0u;
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
