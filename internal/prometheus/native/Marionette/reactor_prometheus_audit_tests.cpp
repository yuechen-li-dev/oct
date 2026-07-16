#include "test_harness.h"

#include "../reactor_prometheus_audit.h"
#include "../reactor_vulkan.h"
#include "../reactor_vulkan_srt_2accum_k_spirv.h"
#include "../reactor_vulkan_b2x2_row_major_biased_spirv.h"
#include "../reactor_vulkan_a2x4_row_biased_accum8_spirv.h"
#include "../reactor_vulkan_tiled_spirv.h"
#include "../reactor_vulkan_memory_conservative_spirv.h"
#include "../reactor_vulkan_sgemm_scalar_plus_spirv.h"
#include "../reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h"
#include "../reactor_vulkan_packed4_spirv.h"
#include "../reactor_vulkan_fp16_spirv.h"
#define k_prom_sgemm_srt_2accum_k_spirv k_prom_m37b_srt_2accum_k_spirv
#include "../reactor_vulkan_sgemm_srt_2accum_k_spirv.h"
#undef k_prom_sgemm_srt_2accum_k_spirv
#define k_prom_sgemm_b2x2_row_major_biased_spirv k_prom_m37b_b2x2_spirv
#include "../reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h"
#undef k_prom_sgemm_b2x2_row_major_biased_spirv
#define k_prom_sgemm_a2x4_row_biased_accum8_spirv k_prom_m37b_a2x4_spirv
#include "../reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h"
#undef k_prom_sgemm_a2x4_row_biased_accum8_spirv
#include "../reactor_judgment_engine.h"
#include "../reactor_vulkan_sgemm_internal.h"

#include <algorithm>
#include <filesystem>
#include <cstring>
#include <array>
#include <cmath>
#include <fstream>
#include <iomanip>
#include <limits>
#include <numeric>
#include <sstream>
#include <string>
#include <vector>

extern "C" {
extern const uint32_t k_prom_sgemm_spirv[];
extern const size_t k_prom_sgemm_spirv_size_bytes;
}

namespace
{
constexpr std::uint64_t kAuditSeed = 99u;
constexpr std::uint32_t kTimingWarmupIterations = 1u;
constexpr std::uint32_t kTimingMeasuredIterations = 5u;

struct ValidationAccounting {
    bool requested = false;
    bool available = false;
    bool enabled = false;
    bool debug_utils_active = false;
    bool device_loss = false;
    std::vector<std::string> enabled_layers;
    std::uint32_t warning_count = 0u;
    std::uint32_t error_count = 0u;
};

struct TimingStats {
    std::vector<std::uint64_t> samples;
    std::uint64_t minimum_ns = 0u;
    std::uint64_t median_ns = 0u;
    std::uint64_t maximum_ns = 0u;
    std::uint64_t p10_ns = 0u;
    std::uint64_t p90_ns = 0u;
    double coefficient_of_variation = 0.0;
};

struct MismatchDetail {
    bool has_mismatch = false;
    std::uint32_t row = 0u;
    std::uint32_t column = 0u;
    float expected = 0.0f;
    float original = 0.0f;
    float candidate = 0.0f;
    float absolute_error = 0.0f;
    float relative_error = 0.0f;
};

struct PairwiseRunRecord {
    std::string pair_name;
    std::string original_name;
    std::string candidate_name;
    std::string original_entry;
    std::string candidate_entry;
    std::uint64_t original_hash = 0u;
    std::uint64_t candidate_hash = 0u;
    std::uint32_t m = 0u;
    std::uint32_t n = 0u;
    std::uint32_t k = 0u;
    prom_sgemm_dispatch_geometry original_dispatch{};
    prom_sgemm_dispatch_geometry candidate_dispatch{};
    prom_sgemm_kernel_dispatch_metadata original_footprint{};
    prom_sgemm_kernel_dispatch_metadata candidate_footprint{};
    std::uint32_t compute_mode = 0u;
    float tolerance = 0.0f;
    bool original_vs_cpu = false;
    bool candidate_vs_cpu = false;
    bool original_vs_candidate = false;
    MismatchDetail first_mismatch{};
    TimingStats original_timing{};
    TimingStats candidate_timing{};
    std::string replay_id;
};

struct PairTimingSummary {
    std::string pair_name;
    std::uint32_t m = 0u;
    std::uint32_t n = 0u;
    std::uint32_t k = 0u;
    TimingStats original{};
    TimingStats candidate{};
};

std::vector<std::uint32_t> MinimalComputeModule(const char* entry, std::uint32_t x, std::uint32_t y, std::uint32_t z)
{
    std::vector<std::uint32_t> words = {0x07230203u, 0x00010000u, 0u, 2u, 0u};
    std::vector<std::uint32_t> nameWords((std::strlen(entry) + 4u) / 4u, 0u);
    std::memcpy(nameWords.data(), entry, std::strlen(entry));
    words.push_back(((3u + static_cast<std::uint32_t>(nameWords.size())) << 16u) | 15u);
    words.push_back(5u);
    words.push_back(1u);
    words.insert(words.end(), nameWords.begin(), nameWords.end());
    words.push_back((6u << 16u) | 16u);
    words.push_back(1u);
    words.push_back(17u);
    words.push_back(x);
    words.push_back(y);
    words.push_back(z);
    return words;
}

PrometheusAuditShaderDescriptor Descriptor(const std::uint32_t* words, std::size_t size, const char* entry = "main")
{
    PrometheusAuditShaderDescriptor descriptor{};
    descriptor.name = "audit-test";
    descriptor.spirv_words = words;
    descriptor.spirv_size_bytes = size;
    descriptor.entry_point = entry;
    descriptor.dispatch = {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u};
    descriptor.k_packing_factor = 1u;
    descriptor.provenance = "Marionette";
    descriptor.comparison_group = "unit";
    return descriptor;
}

void FillInputs(std::vector<float>* a, std::vector<float>* b, std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    a->resize(static_cast<std::size_t>(m) * k);
    b->resize(static_cast<std::size_t>(k) * n);
    for (std::uint32_t index = 0; index < a->size(); ++index) (*a)[index] = static_cast<float>(static_cast<int>((index * 17u) % 23u) - 11) * 0.0625f;
    for (std::uint32_t index = 0; index < b->size(); ++index) (*b)[index] = static_cast<float>(static_cast<int>((index * 11u) % 19u) - 9) * 0.0625f;
}

void ReferenceSgemm(const std::vector<float>& a, const std::vector<float>& b, std::vector<float>* c,
                    std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    c->assign(static_cast<std::size_t>(m) * n, 0.0f);
    for (std::uint32_t row = 0; row < m; ++row) for (std::uint32_t column = 0; column < n; ++column)
        for (std::uint32_t inner = 0; inner < k; ++inner) (*c)[static_cast<std::size_t>(row) * n + column] += a[static_cast<std::size_t>(row) * k + inner] * b[static_cast<std::size_t>(inner) * n + column];
}

struct AuditPairDefinition {
    const char* name;
    const char* original_file;
    const char* candidate_file;
    const char* candidate_entry;
    prom_sgemm_kernel_dispatch_metadata dispatch;
    std::uint32_t compute_mode;
    float tolerance;
};

bool RunAuditModule(void* runtime, const prom_sgemm_audit_execution_descriptor& descriptor,
                    const std::vector<float>& a, const std::vector<float>& b, std::vector<float>* output,
                    std::uint32_t m, std::uint32_t n, std::uint32_t k, prom_sgemm_audit_execution_result* result)
{
    output->assign(static_cast<std::size_t>(m) * n, 0.0f);
    return prom_reactor_runtime_sgemm_audit_impl(runtime, a.data(), b.data(), output->data(), m, n, k, &descriptor, result) == PROM_OK;
}

void AppendJsonString(std::ostringstream& output, const std::string& value)
{
    output << '"';
    for (char ch : value) {
        if (ch == '"' || ch == '\\') output << '\\';
        output << ch;
    }
    output << '"';
}

std::string ComputeModeName(std::uint32_t compute_mode)
{
    switch (compute_mode) {
    case PROM_VK_COMPUTE_PACKED4_FP32: return "packed4_fp32";
    case PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM: return "fp16_storage_fp32_accum";
    default: return "scalar_row_major_fp32";
    }
}

ValidationAccounting CollectValidationAccounting()
{
    ValidationAccounting accounting;
    std::uint32_t layer_count = 0u;
    if (vkEnumerateInstanceLayerProperties(&layer_count, nullptr) != VK_SUCCESS || layer_count == 0u) {
        return accounting;
    }
    std::vector<VkLayerProperties> layers(layer_count);
    if (vkEnumerateInstanceLayerProperties(&layer_count, layers.data()) != VK_SUCCESS) {
        return accounting;
    }
    for (const VkLayerProperties& layer : layers) {
        if (std::strcmp(layer.layerName, "VK_LAYER_KHRONOS_validation") == 0) {
            accounting.available = true;
            break;
        }
    }
    return accounting;
}

void CaptureRuntimeValidationAccounting(const prometheus_runtime* runtime, ValidationAccounting* accounting)
{
    if (runtime == nullptr || accounting == nullptr) return;
    accounting->requested = runtime->validation_requested != 0u;
    accounting->available = runtime->validation_available != 0u;
    accounting->enabled = runtime->validation_enabled != 0u;
    accounting->debug_utils_active = runtime->validation_debug_utils_active != 0u;
    accounting->warning_count = runtime->validation_warning_count;
    accounting->error_count = runtime->validation_error_count;
    if (accounting->enabled && accounting->enabled_layers.empty()) {
        accounting->enabled_layers.push_back("VK_LAYER_KHRONOS_validation");
    }
}

TimingStats ComputeTimingStats(const std::vector<std::uint64_t>& samples)
{
    TimingStats stats;
    stats.samples = samples;
    if (samples.empty()) return stats;
    std::vector<std::uint64_t> sorted = samples;
    std::sort(sorted.begin(), sorted.end());
    stats.minimum_ns = sorted.front();
    stats.maximum_ns = sorted.back();
    stats.median_ns = sorted[sorted.size() / 2u];
    stats.p10_ns = sorted[(sorted.size() - 1u) / 10u];
    stats.p90_ns = sorted[((sorted.size() - 1u) * 9u) / 10u];
    const double mean = std::accumulate(sorted.begin(), sorted.end(), 0.0) / static_cast<double>(sorted.size());
    double sumSquared = 0.0;
    for (const std::uint64_t sample : sorted) {
        const double delta = static_cast<double>(sample) - mean;
        sumSquared += delta * delta;
    }
    stats.coefficient_of_variation = mean > 0.0 ? std::sqrt(sumSquared / static_cast<double>(sorted.size())) / mean : 0.0;
    return stats;
}

TimingStats MeasureTiming(void* runtime, const prom_sgemm_audit_execution_descriptor& descriptor,
                          const std::vector<float>& a, const std::vector<float>& b,
                          std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    std::vector<float> output(static_cast<std::size_t>(m) * n, 0.0f);
    prom_sgemm_audit_execution_result result{};
    std::vector<std::uint64_t> samples(kTimingMeasuredIterations, 0u);
    if (prom_reactor_runtime_sgemm_audit_benchmark_impl(runtime, a.data(), b.data(), output.data(), m, n, k, &descriptor,
                                                         kTimingWarmupIterations, kTimingMeasuredIterations,
                                                         samples.data(), static_cast<std::uint32_t>(samples.size()), &result) != PROM_OK ||
        result.gpu_timing_valid == 0u || result.pipeline_create_count != 1u ||
        result.warmup_dispatch_count != kTimingWarmupIterations || result.measured_dispatch_count != kTimingMeasuredIterations ||
        result.dispatches_per_sample != 1u || result.timestamp_interval_command_mask != PROM_SGEMM_AUDIT_TIMESTAMP_DISPATCH ||
        result.query_reset_before_start_timestamp != 1u || result.fence_wait_before_query_results != 1u) return {};
    return ComputeTimingStats(samples);
}

MismatchDetail FindFirstMismatch(const std::vector<float>& expected,
                                 const std::vector<float>& original,
                                 const std::vector<float>& candidate,
                                 std::uint32_t n,
                                 float tolerance)
{
    for (std::size_t index = 0; index < expected.size(); ++index) {
        const float diff_original = std::fabs(expected[index] - original[index]);
        const float diff_candidate = std::fabs(expected[index] - candidate[index]);
        const float diff_pair = std::fabs(original[index] - candidate[index]);
        if (diff_original > tolerance || diff_candidate > tolerance || diff_pair > tolerance) {
            MismatchDetail mismatch;
            mismatch.has_mismatch = true;
            mismatch.row = static_cast<std::uint32_t>(index / n);
            mismatch.column = static_cast<std::uint32_t>(index % n);
            mismatch.expected = expected[index];
            mismatch.original = original[index];
            mismatch.candidate = candidate[index];
            mismatch.absolute_error = std::max(diff_original, diff_candidate);
            const float denominator = std::max(std::fabs(expected[index]), 1.0e-6f);
            mismatch.relative_error = mismatch.absolute_error / denominator;
            return mismatch;
        }
    }
    return {};
}

std::string RenderTimingJson(const TimingStats& stats)
{
    std::ostringstream output;
    output << "{\"min_ns\":" << stats.minimum_ns
           << ",\"median_ns\":" << stats.median_ns
           << ",\"max_ns\":" << stats.maximum_ns
           << ",\"p10_ns\":" << stats.p10_ns
           << ",\"p90_ns\":" << stats.p90_ns
           << ",\"coefficient_of_variation\":" << std::setprecision(8) << stats.coefficient_of_variation
           << '}';
    return output.str();
}

std::string RenderValidationJson(const ValidationAccounting& validation)
{
    std::ostringstream output;
    output << "{\"requested\":" << (validation.requested ? "true" : "false")
           << ",\"available\":" << (validation.available ? "true" : "false")
           << ",\"enabled\":" << (validation.enabled ? "true" : "false")
           << ",\"debug_utils_active\":" << (validation.debug_utils_active ? "true" : "false")
           << ",\"enabled_layers\":[";
    for (std::size_t index = 0; index < validation.enabled_layers.size(); ++index) {
        if (index != 0u) output << ',';
        AppendJsonString(output, validation.enabled_layers[index]);
    }
    output << "],\"warning_count\":" << validation.warning_count
           << ",\"error_count\":" << validation.error_count
           << ",\"device_loss\":" << (validation.device_loss ? "true" : "false")
           << '}';
    return output.str();
}

std::string RenderMismatchJson(const MismatchDetail& mismatch)
{
    if (!mismatch.has_mismatch) return "null";
    std::ostringstream output;
    output << "{\"row\":" << mismatch.row
           << ",\"column\":" << mismatch.column
           << ",\"expected\":" << std::setprecision(9) << mismatch.expected
           << ",\"original\":" << mismatch.original
           << ",\"candidate\":" << mismatch.candidate
           << ",\"absolute_error\":" << mismatch.absolute_error
           << ",\"relative_error\":" << mismatch.relative_error
           << '}';
    return output.str();
}

std::string RenderRunJson(const PairwiseRunRecord& run, const ValidationAccounting& validation)
{
    std::ostringstream output;
    output << "{\"schemaVersion\":1"
           << ",\"pair\":{\"name\":";
    AppendJsonString(output, run.pair_name);
    output << ",\"original\":{\"name\":";
    AppendJsonString(output, run.original_name);
    output << ",\"hash\":\"" << std::hex << run.original_hash << std::dec << "\",\"entryPoint\":";
    AppendJsonString(output, run.original_entry);
    output << "},\"candidate\":{\"name\":";
    AppendJsonString(output, run.candidate_name);
    output << ",\"hash\":\"" << std::hex << run.candidate_hash << std::dec << "\",\"entryPoint\":";
    AppendJsonString(output, run.candidate_entry);
    output << "}}"
           << ",\"workload\":{\"m\":" << run.m << ",\"n\":" << run.n << ",\"k\":" << run.k << ",\"seed\":" << kAuditSeed << '}'
           << ",\"dispatch\":{\"original\":{\"groups\":[" << run.original_dispatch.groups_x << ',' << run.original_dispatch.groups_y << ',' << run.original_dispatch.groups_z
           << "],\"footprint\":[" << run.original_footprint.outputs_per_invocation_m << ',' << run.original_footprint.outputs_per_invocation_n << "]}"
           << ",\"candidate\":{\"groups\":[" << run.candidate_dispatch.groups_x << ',' << run.candidate_dispatch.groups_y << ',' << run.candidate_dispatch.groups_z
           << "],\"footprint\":[" << run.candidate_footprint.outputs_per_invocation_m << ',' << run.candidate_footprint.outputs_per_invocation_n << "]}}"
           << ",\"mode\":";
    AppendJsonString(output, ComputeModeName(run.compute_mode));
    output << ",\"tolerance\":" << std::setprecision(6) << run.tolerance
           << ",\"correctness\":{\"originalVsCpu\":" << (run.original_vs_cpu ? "true" : "false")
           << ",\"candidateVsCpu\":" << (run.candidate_vs_cpu ? "true" : "false")
           << ",\"originalVsCandidate\":" << (run.original_vs_candidate ? "true" : "false")
           << ",\"firstMismatch\":" << RenderMismatchJson(run.first_mismatch) << '}'
           << ",\"timing\":{\"original\":" << RenderTimingJson(run.original_timing)
           << ",\"candidate\":" << RenderTimingJson(run.candidate_timing) << '}'
           << ",\"validation\":" << RenderValidationJson(validation)
           << ",\"replayId\":";
    AppendJsonString(output, run.replay_id);
    output << '}';
    return output.str();
}

std::string RenderPairTimingSummaryJson(const PairTimingSummary& summary)
{
    std::ostringstream output;
    output << "{\"pair\":";
    AppendJsonString(output, summary.pair_name);
    output << ",\"workload\":{\"m\":" << summary.m << ",\"n\":" << summary.n << ",\"k\":" << summary.k << '}'
           << ",\"original\":" << RenderTimingJson(summary.original)
           << ",\"candidate\":" << RenderTimingJson(summary.candidate)
           << '}';
    return output.str();
}
}

FACT(PrometheusAuditRegistryIsolatedFromProductionRegistry)
{
    PrometheusAuditShaderRegistry registry;
    std::string error;
    PrometheusAuditShaderDescriptor descriptor = Descriptor(k_prom_sgemm_srt_2accum_k_spirv,
                                                            sizeof(k_prom_sgemm_srt_2accum_k_spirv));
    descriptor.name = "embedded-original-srt";
    descriptor.dispatch = {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u};
    ASSERT_TRUE(registry.RegisterEmbedded(descriptor, &error), error);
    ASSERT_EQUAL(1u, static_cast<std::uint32_t>(registry.Enumerate().size()), "audit registration is private to audit registry");
    ASSERT_TRUE(registry.Find("embedded-original-srt") != nullptr, "explicit audit name lookup succeeds");
}

FACT(PrometheusAuditRejectsMalformedCandidates)
{
    const std::vector<std::uint32_t> valid = MinimalComputeModule("main", 8u, 8u, 1u);
    PrometheusAuditShaderDescriptor descriptor = Descriptor(valid.data(), valid.size() * sizeof(std::uint32_t));
    ASSERT_TRUE(prometheus_audit_validate_shader(descriptor).valid, "well-formed compute candidate validates");

    descriptor.spirv_size_bytes -= 1u;
    ASSERT_FALSE(prometheus_audit_validate_shader(descriptor).valid, "bad byte length is rejected");
    descriptor.spirv_size_bytes = valid.size() * sizeof(std::uint32_t);
    std::vector<std::uint32_t> badMagic = valid;
    badMagic[0] = 0u;
    descriptor.spirv_words = badMagic.data();
    ASSERT_FALSE(prometheus_audit_validate_shader(descriptor).valid, "bad magic is rejected");
    descriptor.spirv_words = valid.data();
    descriptor.entry_point = "missing";
    ASSERT_FALSE(prometheus_audit_validate_shader(descriptor).valid, "missing entry point is rejected");
    descriptor.entry_point = "main";
    descriptor.dispatch.threads_y = 4u;
    ASSERT_FALSE(prometheus_audit_validate_shader(descriptor).valid, "workgroup mismatch is rejected");
}

FACT(PrometheusAuditFileRegistrationAndCanonicalA2x4Dispatch)
{
    const std::vector<std::uint32_t> module = MinimalComputeModule("main", 8u, 8u, 1u);
    const std::filesystem::path path = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) / "out" / "test-artifacts" / "prometheus_audit_candidate.spv";
    std::filesystem::create_directories(path.parent_path());
    {
        std::ofstream output(path, std::ios::binary);
        output.write(reinterpret_cast<const char*>(module.data()), static_cast<std::streamsize>(module.size() * sizeof(std::uint32_t)));
    }
    PrometheusAuditShaderDescriptor descriptor = Descriptor(nullptr, 0u);
    const std::string pathString = path.string();
    descriptor.name = "file-a2x4";
    descriptor.file_path = pathString.c_str();
    descriptor.dispatch = {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u};
    PrometheusAuditShaderRegistry registry;
    std::string error;
    ASSERT_TRUE(registry.RegisterFile(descriptor, &error), error);
    const PrometheusAuditDispatch dispatch = prometheus_audit_dispatch_for(*registry.Find("file-a2x4"), 3u, 5u);
    ASSERT_TRUE(dispatch.valid, dispatch.error);
    ASSERT_EQUAL(1u, dispatch.geometry.groups_x, "A2x4 rows use two outputs per invocation");
    ASSERT_EQUAL(1u, dispatch.geometry.groups_y, "A2x4 columns use four outputs per invocation");
    const PrometheusAuditValidation validation = prometheus_audit_validate_shader(*registry.Find("file-a2x4"));
    const std::string json = prometheus_audit_json_summary(*registry.Find("file-a2x4"), validation, dispatch, 3u, 5u, 7u, 99u);
    ASSERT_TRUE(json.find("\"output_footprint\":[2,4]") != std::string::npos, "JSON reports canonical A2x4 footprint");
}

FACT(PrometheusAuditReplayIdentityIsDeterministic)
{
    const std::vector<std::uint32_t> module = MinimalComputeModule("main", 8u, 8u, 1u);
    PrometheusAuditShaderDescriptor original = Descriptor(module.data(), module.size() * sizeof(std::uint32_t));
    original.name = "original";
    PrometheusAuditShaderDescriptor candidate = original;
    candidate.name = "candidate";
    const std::string first = prometheus_audit_replay_identity(original, candidate, 3u, 5u, 7u, 99u);
    const std::string second = prometheus_audit_replay_identity(original, candidate, 3u, 5u, 7u, 99u);
    ASSERT_EQUAL(first, second, "replay identity is stable");
}

FACT(PrometheusAuditPipelineOverrideUsesRealSgemmPath)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "runtime creation succeeds");
    if (runtime == nullptr) SKIP("Vulkan runtime unavailable");
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan backend unavailable");
    }
    constexpr std::uint32_t m = 3u;
    constexpr std::uint32_t n = 5u;
    constexpr std::uint32_t k = 7u;
    std::vector<float> a;
    std::vector<float> b;
    std::vector<float> expected;
    std::vector<float> output(static_cast<std::size_t>(m) * n, 0.0f);
    FillInputs(&a, &b, m, n, k);
    ReferenceSgemm(a, b, &expected, m, n, k);
    prom_sgemm_audit_execution_descriptor descriptor{};
    descriptor.spirv_words = k_prom_sgemm_srt_2accum_k_spirv;
    descriptor.spirv_size_bytes = sizeof(k_prom_sgemm_srt_2accum_k_spirv);
    descriptor.entry_point = "main";
    descriptor.dispatch = {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u};
    descriptor.compute_mode = static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED);
    descriptor.provenance = "production registry asset 10";
    prom_sgemm_audit_execution_result result{};
    const int status = prom_reactor_runtime_sgemm_audit_impl(runtime, a.data(), b.data(), output.data(), m, n, k, &descriptor, &result);
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_EQUAL(PROM_OK, status, "audit override executes through the synchronous SGEMM path");
    ASSERT_EQUAL(1u, result.dispatch_geometry.groups_x, "audit footprint drives groups X");
    ASSERT_EQUAL(1u, result.dispatch_geometry.groups_y, "audit footprint drives groups Y");
    for (std::size_t index = 0; index < expected.size(); ++index) ASSERT_NEAR(expected[index], output[index], 0.001f, "audit output matches CPU reference");
}

FACT(PrometheusAuditOriginalFivePairwiseHardware)
{
    const std::array<AuditPairDefinition, 5> pairs = {{
        {"SRT-2accum-K", "sgemm_srt_2accum_k.spv", "sgemm_srt_2accum_k.spv", "SgemmSrt2AccumK_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"B2x2-row-major-biased", "sgemm_b2x2_row_major_biased.spv", "sgemm_b2x2_row_major_biased.spv", "SgemmB2x2_CS", {8u, 8u, 1u, 2u, 2u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"A2x4-row-biased-accum8", "sgemm_a2x4_row_biased_accum8.spv", "sgemm_a2x4_row_biased_accum8.spv", "SgemmA2x4_CS", {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"Packed4FP32", "sgemm_packed4_fp32.spv", "sgemm_packed4_fp32.spv", "SgemmPacked4_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_PACKED4_FP32), 0.002f},
        {"FP16-storage/FP32-accum", "sgemm_fp16_storage_fp32_accum.spv", "sgemm_fp16_storage_fp32_accum.spv", "SgemmFp16StorageFp32Accum_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM), 0.03f},
    }};
    const std::array<std::array<std::uint32_t, 3>, 9> shapes = {{{2u, 2u, 2u}, {8u, 8u, 8u}, {5u, 7u, 9u}, {3u, 5u, 7u}, {5u, 8u, 8u}, {8u, 5u, 8u}, {5u, 7u, 11u}, {1u, 9u, 3u}, {3u, 5u, 7u}}};
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "runtime creation succeeds");
    if (runtime == nullptr) SKIP("Vulkan runtime unavailable");
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan backend unavailable");
    }
    const std::filesystem::path artifactRoot = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "SDSL_V_ORIGINAL_SPIRV_REWRITE";
    const std::filesystem::path generated = artifactRoot / "generated";
    const std::filesystem::path originals = artifactRoot / "original";
    ValidationAccounting validation = CollectValidationAccounting();
    CaptureRuntimeValidationAccounting(static_cast<const prometheus_runtime*>(runtime), &validation);
    std::vector<PairwiseRunRecord> runs;
    std::vector<PairTimingSummary> pair_timings;
    for (const AuditPairDefinition& pair : pairs) {
        const std::string candidatePath = (generated / pair.candidate_file).string();
        PrometheusAuditShaderDescriptor fileDescriptor{};
        fileDescriptor.name = pair.name;
        fileDescriptor.file_path = candidatePath.c_str();
        fileDescriptor.entry_point = pair.candidate_entry;
        fileDescriptor.dispatch = pair.dispatch;
        fileDescriptor.k_packing_factor = pair.compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_PACKED4_FP32) ? 4u : 1u;
        PrometheusAuditShaderRegistry registry;
        std::string error;
        ASSERT_TRUE(registry.RegisterFile(fileDescriptor, &error), error);
        const PrometheusAuditShaderDescriptor* candidate = registry.Find(pair.name);
        ASSERT_TRUE(candidate != nullptr, "candidate registration succeeds");
        const std::string originalPath = (originals / pair.original_file).string();
        PrometheusAuditShaderDescriptor originalFile{};
        originalFile.name = pair.name;
        originalFile.file_path = originalPath.c_str();
        originalFile.entry_point = "main";
        originalFile.dispatch = pair.dispatch;
        originalFile.k_packing_factor = fileDescriptor.k_packing_factor;
        PrometheusAuditShaderRegistry originalsRegistry;
        ASSERT_TRUE(originalsRegistry.RegisterFile(originalFile, &error), error);
        const PrometheusAuditShaderDescriptor* archived = originalsRegistry.Find(pair.name);
        ASSERT_TRUE(archived != nullptr, "archived original registration succeeds");
        PrometheusAuditShaderDescriptor originalDescriptor = *archived;
        const PrometheusAuditValidation originalValidation = prometheus_audit_validate_shader(originalDescriptor);
        const PrometheusAuditValidation candidateValidation = prometheus_audit_validate_shader(*candidate);
        ASSERT_TRUE(originalValidation.valid, originalValidation.error);
        ASSERT_TRUE(candidateValidation.valid, candidateValidation.error);
        prom_sgemm_audit_execution_descriptor original{};
        original.spirv_words = originalDescriptor.spirv_words;
        original.spirv_size_bytes = originalDescriptor.spirv_size_bytes;
        original.entry_point = "main";
        original.dispatch = pair.dispatch;
        original.compute_mode = pair.compute_mode;
        prom_sgemm_audit_execution_descriptor generatedDescriptor = original;
        generatedDescriptor.spirv_words = candidate->spirv_words;
        generatedDescriptor.spirv_size_bytes = candidate->spirv_size_bytes;
        generatedDescriptor.entry_point = candidate->entry_point;
        for (const auto& shape : shapes) {
            std::vector<float> a;
            std::vector<float> b;
            std::vector<float> expected;
            std::vector<float> originalOutput;
            std::vector<float> candidateOutput;
            FillInputs(&a, &b, shape[0], shape[1], shape[2]);
            ReferenceSgemm(a, b, &expected, shape[0], shape[1], shape[2]);
            prom_sgemm_audit_execution_result originalResult{};
            prom_sgemm_audit_execution_result candidateResult{};
            ASSERT_TRUE(RunAuditModule(runtime, original, a, b, &originalOutput, shape[0], shape[1], shape[2], &originalResult), "original audit module executes");
            ASSERT_TRUE(RunAuditModule(runtime, generatedDescriptor, a, b, &candidateOutput, shape[0], shape[1], shape[2], &candidateResult), "generated audit module executes");
            bool originalVsCpu = true;
            bool candidateVsCpu = true;
            bool originalVsCandidate = true;
            for (std::size_t index = 0; index < expected.size(); ++index) {
                if (std::fabs(expected[index] - originalOutput[index]) > pair.tolerance) originalVsCpu = false;
                if (std::fabs(expected[index] - candidateOutput[index]) > pair.tolerance) candidateVsCpu = false;
                if (std::fabs(originalOutput[index] - candidateOutput[index]) > pair.tolerance) originalVsCandidate = false;
            }
            const MismatchDetail mismatch = FindFirstMismatch(expected, originalOutput, candidateOutput, shape[1], pair.tolerance);
            ASSERT_TRUE(!mismatch.has_mismatch, "pairwise run remains correct");
            const TimingStats originalTiming = MeasureTiming(runtime, original, a, b, shape[0], shape[1], shape[2]);
            const TimingStats candidateTiming = MeasureTiming(runtime, generatedDescriptor, a, b, shape[0], shape[1], shape[2]);
            ASSERT_TRUE(!originalTiming.samples.empty(), "original timing samples are captured");
            ASSERT_TRUE(!candidateTiming.samples.empty(), "candidate timing samples are captured");
            PairwiseRunRecord run{};
            run.pair_name = pair.name;
            run.original_name = pair.name;
            run.candidate_name = pair.name;
            run.original_entry = original.entry_point;
            run.candidate_entry = generatedDescriptor.entry_point;
            run.original_hash = originalValidation.spirv_hash;
            run.candidate_hash = candidateValidation.spirv_hash;
            run.m = shape[0];
            run.n = shape[1];
            run.k = shape[2];
            run.original_dispatch = originalResult.dispatch_geometry;
            run.candidate_dispatch = candidateResult.dispatch_geometry;
            run.original_footprint = pair.dispatch;
            run.candidate_footprint = candidate->dispatch;
            run.compute_mode = pair.compute_mode;
            run.tolerance = pair.tolerance;
            run.original_vs_cpu = originalVsCpu;
            run.candidate_vs_cpu = candidateVsCpu;
            run.original_vs_candidate = originalVsCandidate;
            run.first_mismatch = mismatch;
            run.original_timing = originalTiming;
            run.candidate_timing = candidateTiming;
            run.replay_id = prometheus_audit_replay_identity(originalDescriptor, *candidate, shape[0], shape[1], shape[2], kAuditSeed);
            runs.push_back(run);
        }
        std::vector<float> timingA;
        std::vector<float> timingB;
        FillInputs(&timingA, &timingB, 127u, 131u, 129u);
        PairTimingSummary summary{};
        summary.pair_name = pair.name;
        summary.m = 127u;
        summary.n = 131u;
        summary.k = 129u;
        summary.original = MeasureTiming(runtime, original, timingA, timingB, summary.m, summary.n, summary.k);
        summary.candidate = MeasureTiming(runtime, generatedDescriptor, timingA, timingB, summary.m, summary.n, summary.k);
        ASSERT_TRUE(!summary.original.samples.empty(), "pair timing original samples are captured");
        ASSERT_TRUE(!summary.candidate.samples.empty(), "pair timing candidate samples are captured");
        pair_timings.push_back(summary);
    }
    std::ostringstream report;
    report << "{\"schemaVersion\":1"
           << ",\"validation\":" << RenderValidationJson(validation)
           << ",\"pairTimingSummaries\":[";
    for (std::size_t index = 0; index < pair_timings.size(); ++index) {
        if (index != 0u) report << ',';
        report << RenderPairTimingSummaryJson(pair_timings[index]);
    }
    report << "],\"runs\":[";
    for (std::size_t index = 0; index < runs.size(); ++index) {
        if (index != 0u) report << ',';
        report << RenderRunJson(runs[index], validation);
    }
    report << "]}";
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m34a_pairwise_summary.json", report.str()), "pairwise audit JSON is written");
}

// M37b is intentionally a narrow, test-only adapter: production assets are
// passed to the established audit seam without changing selector ownership.
FACT(PrometheusM37bProductionTimingRows)
{
    struct M37bKernel {
        const char* name;
        const uint32_t* words;
        size_t bytes;
        const char* entry;
        prom_sgemm_kernel_dispatch_metadata dispatch;
        uint32_t mode;
        float tolerance;
    };
    const std::array<M37bKernel, 10> kernels = {{
        {"scalar", k_prom_sgemm_spirv, k_prom_sgemm_spirv_size_bytes, "main", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_BASELINE), 0.002f},
        {"tiled", k_prom_sgemm_tiled_spirv, sizeof(k_prom_sgemm_tiled_spirv), "main", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"memory-conservative", k_prom_sgemm_memory_conservative_spirv, sizeof(k_prom_sgemm_memory_conservative_spirv), "main", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"scalar-plus", k_prom_sgemm_scalar_plus_spirv, sizeof(k_prom_sgemm_scalar_plus_spirv), "SgemmScalarBaselinePlus8x8_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"tile16", k_prom_sgemm_tile16x16_shared_fp32_spirv, sizeof(k_prom_sgemm_tile16x16_shared_fp32_spirv), "SgemmTile16x16SharedFp32_CS", {16u, 16u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"SRT", k_prom_m37b_srt_2accum_k_spirv, sizeof(k_prom_m37b_srt_2accum_k_spirv), "SgemmSrt2AccumK_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"B2x2", k_prom_m37b_b2x2_spirv, sizeof(k_prom_m37b_b2x2_spirv), "SgemmB2x2_CS", {8u, 8u, 1u, 2u, 2u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"A2x4", k_prom_m37b_a2x4_spirv, sizeof(k_prom_m37b_a2x4_spirv), "SgemmA2x4_CS", {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"Packed4", k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "SgemmPacked4_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_PACKED4_FP32), 0.002f},
        {"FP16", k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv), "SgemmFp16StorageFp32Accum_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<uint32_t>(PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM), 0.03f},
    }};
    const std::array<std::array<uint32_t, 3>, 2> workloads = {{{512u, 512u, 512u}, {127u, 131u, 129u}}};
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "M37b runtime creation succeeds");
    if (runtime == nullptr) {
        SKIP("Vulkan runtime unavailable");
    }
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan backend unavailable");
    }
    std::ostringstream report;
    report << "{\"backend\":\"prometheus\",\"warmup\":" << kTimingWarmupIterations << ",\"iterations\":" << kTimingMeasuredIterations << ",\"rows\":[";
    bool first = true;
    for (const M37bKernel& kernel : kernels) {
        for (const auto& shape : workloads) {
            std::vector<float> a;
            std::vector<float> b;
            std::vector<float> expected;
            std::vector<float> output;
            FillInputs(&a, &b, shape[0], shape[1], shape[2]);
            ReferenceSgemm(a, b, &expected, shape[0], shape[1], shape[2]);
            prom_sgemm_audit_execution_descriptor descriptor{};
            descriptor.spirv_words = kernel.words;
            descriptor.spirv_size_bytes = kernel.bytes;
            descriptor.entry_point = kernel.entry;
            descriptor.dispatch = kernel.dispatch;
            descriptor.compute_mode = kernel.mode;
            descriptor.provenance = "M37b exact production asset";
            prom_sgemm_audit_execution_result execution{};
            const bool ran = RunAuditModule(runtime, descriptor, a, b, &output, shape[0], shape[1], shape[2], &execution);
            bool correct = ran;
            if (correct) {
                for (size_t index = 0; index < expected.size(); ++index) {
                    if (std::fabs(expected[index] - output[index]) > kernel.tolerance) {
                        correct = false;
                        break;
                    }
                }
            }
            const TimingStats timing = ran ? MeasureTiming(runtime, descriptor, a, b, shape[0], shape[1], shape[2]) : TimingStats{};
            ASSERT_TRUE(ran && correct && !timing.samples.empty(), "M37b production artifact must execute, match CPU, and timestamp");
            if (!first) {
                report << ',';
            }
            first = false;
            report << "{\"kernel\":";
            AppendJsonString(report, kernel.name);
            report << ",\"m\":" << shape[0]
                   << ",\"n\":" << shape[1]
                   << ",\"k\":" << shape[2]
                   << ",\"spirv_bytes\":" << kernel.bytes
                   << ",\"entry_point\":";
            AppendJsonString(report, kernel.entry);
            report << ",\"local_size\":[" << kernel.dispatch.threads_x << ',' << kernel.dispatch.threads_y << ',' << kernel.dispatch.threads_z << ']'
                   << ",\"footprint\":[" << kernel.dispatch.outputs_per_invocation_m << ',' << kernel.dispatch.outputs_per_invocation_n << ']'
                   << ",\"groups\":[" << execution.dispatch_geometry.groups_x << ',' << execution.dispatch_geometry.groups_y << ',' << execution.dispatch_geometry.groups_z << ']'
                   << ",\"push_constants\":[" << execution.push_constant_m << ',' << execution.push_constant_n << ',' << execution.push_constant_k << ']'
                   << ",\"buffer_bytes\":[" << execution.a_buffer_bytes << ',' << execution.b_buffer_bytes << ',' << execution.c_buffer_bytes << ']'
                   << ",\"memory_type_indices\":[" << execution.a_memory_type_index << ',' << execution.b_memory_type_index << ',' << execution.c_memory_type_index << ']'
                   << ",\"memory_property_flags\":[" << execution.a_memory_property_flags << ',' << execution.b_memory_property_flags << ',' << execution.c_memory_property_flags << ']'
                   << ",\"buffer_usage_flags\":[" << execution.a_usage_flags << ',' << execution.b_usage_flags << ',' << execution.c_usage_flags << ']'
                   << ",\"memory_alignments\":[" << execution.a_memory_alignment << ',' << execution.b_memory_alignment << ',' << execution.c_memory_alignment << ']'
                   << ",\"memory_offsets\":[" << execution.a_memory_offset << ',' << execution.b_memory_offset << ',' << execution.c_memory_offset << ']'
                   << ",\"compute_queue_family_index\":" << execution.compute_queue_family_index
                   << ",\"selected_path\":" << execution.selected_path
                   << ",\"dispatches_per_sample\":1"
                   << ",\"timestamp_interval_command_mask\":" << PROM_SGEMM_AUDIT_TIMESTAMP_DISPATCH
                   << ",\"correct\":" << (correct ? "true" : "false")
                   << ",\"min\":" << timing.minimum_ns
                   << ",\"median\":" << timing.median_ns
                   << ",\"max\":" << timing.maximum_ns << '}';
        }
    }
    report << "]}";
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m37b_rows.json", report.str()), "M37b Prometheus rows are written");
}

FACT(PrometheusM38aDirectMemoryDiagnostic)
{
    struct Kernel {
        const char* name;
        const std::uint32_t* words;
        std::size_t bytes;
        const char* entry;
        std::uint32_t mode;
        std::uint32_t footprint_m;
        std::uint32_t footprint_n;
    };
    const std::array<Kernel, 4> kernels = {{
        {"B2x2", k_prom_m37b_b2x2_spirv, sizeof(k_prom_m37b_b2x2_spirv), "SgemmB2x2_CS", static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 2u, 2u},
        {"A2x4", k_prom_m37b_a2x4_spirv, sizeof(k_prom_m37b_a2x4_spirv), "SgemmA2x4_CS", static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 2u, 4u},
        {"Packed4", k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "SgemmPacked4_CS", static_cast<std::uint32_t>(PROM_VK_COMPUTE_PACKED4_FP32), 1u, 1u},
        {"FP16", k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv), "SgemmFp16StorageFp32Accum_CS", static_cast<std::uint32_t>(PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM), 1u, 1u},
    }};
    PrometheusReactorConfig config{};
    config.struct_size = sizeof(config);
    config.test_flags = PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(&config, &runtime), "M38a direct diagnostic runtime creation succeeds");
    PrometheusCaps caps{};
    if (runtime == nullptr || prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan runtime unavailable");
    }
    std::ostringstream report;
    report << "{\"rows\":[";
    bool first = true;
    for (const Kernel& kernel : kernels) {
        for (const auto& shape : std::array<std::array<std::uint32_t, 3>, 2>{{{{512u, 512u, 512u}}, {{127u, 131u, 129u}}}}) {
            std::vector<float> a;
            std::vector<float> b;
            std::vector<float> output;
            FillInputs(&a, &b, shape[0], shape[1], shape[2]);
            prom_sgemm_audit_execution_descriptor descriptor{};
            descriptor.spirv_words = kernel.words;
            descriptor.spirv_size_bytes = kernel.bytes;
            descriptor.entry_point = kernel.entry;
            descriptor.dispatch = {8u, 8u, 1u, kernel.footprint_m, kernel.footprint_n, 0u, 0u, 0u, 0u};
            descriptor.compute_mode = kernel.mode;
            descriptor.provenance = "M38a direct-memory diagnostic";
            prom_sgemm_audit_execution_result execution{};
            ASSERT_TRUE(RunAuditModule(runtime, descriptor, a, b, &output, shape[0], shape[1], shape[2], &execution), "direct diagnostic dispatch succeeds");
            const TimingStats timing = MeasureTiming(runtime, descriptor, a, b, shape[0], shape[1], shape[2]);
            ASSERT_FALSE(timing.samples.empty(), "direct diagnostic timestamp samples exist");
            if (!first) report << ',';
            first = false;
            report << "{\"kernel\":";
            AppendJsonString(report, kernel.name);
            report << ",\"m\":" << shape[0] << ",\"n\":" << shape[1] << ",\"k\":" << shape[2]
                   << ",\"median_ns\":" << timing.median_ns
                   << ",\"selected_path\":" << execution.selected_path
                   << ",\"memory_type_indices\":[" << execution.a_memory_type_index << ',' << execution.b_memory_type_index << ',' << execution.c_memory_type_index << ']'
                   << ",\"memory_property_flags\":[" << execution.a_memory_property_flags << ',' << execution.b_memory_property_flags << ',' << execution.c_memory_property_flags << "]}";
        }
    }
    report << "]}";
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m38a_direct_memory.json", report.str()), "M38a direct-memory artifact is written");
}

FACT(PrometheusAuditA2x4HistoricalFootprintExperiment)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "runtime creation succeeds");
    if (runtime == nullptr) SKIP("Vulkan runtime unavailable");
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan backend unavailable");
    }
    constexpr std::uint32_t m = 3u;
    constexpr std::uint32_t n = 17u;
    constexpr std::uint32_t k = 7u;
    std::vector<float> a;
    std::vector<float> b;
    std::vector<float> expected;
    std::vector<float> canonicalOutput;
    std::vector<float> historicalOutput;
    FillInputs(&a, &b, m, n, k);
    ReferenceSgemm(a, b, &expected, m, n, k);
    prom_sgemm_audit_execution_descriptor canonical{};
    canonical.spirv_words = k_prom_sgemm_a2x4_row_biased_accum8_spirv;
    canonical.spirv_size_bytes = sizeof(k_prom_sgemm_a2x4_row_biased_accum8_spirv);
    canonical.entry_point = "main";
    canonical.dispatch = {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u};
    canonical.compute_mode = static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED);
    prom_sgemm_audit_execution_descriptor historical = canonical;
    historical.dispatch.outputs_per_invocation_n = 2u;
    prom_sgemm_audit_execution_result canonicalResult{};
    prom_sgemm_audit_execution_result historicalResult{};
    ASSERT_TRUE(RunAuditModule(runtime, canonical, a, b, &canonicalOutput, m, n, k, &canonicalResult), "canonical A2x4 executes");
    ASSERT_TRUE(RunAuditModule(runtime, historical, a, b, &historicalOutput, m, n, k, &historicalResult), "historical A2x4 executes");
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_EQUAL(1u, canonicalResult.dispatch_geometry.groups_y, "canonical 2x4 needs one group in N");
    ASSERT_EQUAL(2u, historicalResult.dispatch_geometry.groups_y, "historical 2x2 over-dispatches N");
    for (std::size_t index = 0; index < expected.size(); ++index) {
        ASSERT_NEAR(expected[index], canonicalOutput[index], 0.002f, "canonical A2x4 matches reference");
        ASSERT_NEAR(expected[index], historicalOutput[index], 0.002f, "historical A2x4 remains guarded");
        ASSERT_NEAR(canonicalOutput[index], historicalOutput[index], 0.002f, "extra historical invocations do not corrupt output");
    }
}

FACT(PrometheusAuditReplayProofPassAndSyntheticFailure)
{
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "runtime creation succeeds");
    if (runtime == nullptr) SKIP("Vulkan runtime unavailable");
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK || caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan backend unavailable");
    }
    constexpr std::uint32_t m = 3u;
    constexpr std::uint32_t n = 5u;
    constexpr std::uint32_t k = 7u;
    std::vector<float> a;
    std::vector<float> b;
    std::vector<float> expected;
    std::vector<float> originalOutput;
    std::vector<float> candidateOutput;
    FillInputs(&a, &b, m, n, k);
    ReferenceSgemm(a, b, &expected, m, n, k);

    PrometheusAuditShaderDescriptor originalDescriptor{};
    originalDescriptor.name = "SRT-2accum-K";
    originalDescriptor.spirv_words = k_prom_sgemm_srt_2accum_k_spirv;
    originalDescriptor.spirv_size_bytes = sizeof(k_prom_sgemm_srt_2accum_k_spirv);
    originalDescriptor.entry_point = "main";
    originalDescriptor.dispatch = {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u};
    originalDescriptor.k_packing_factor = 1u;

    const std::filesystem::path generated = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "SDSL_V_ORIGINAL_SPIRV_REWRITE" / "generated" / "sgemm_srt_2accum_k.spv";
    PrometheusAuditShaderDescriptor candidateFile{};
    candidateFile.name = "SRT-2accum-K";
    const std::string candidatePath = generated.string();
    candidateFile.file_path = candidatePath.c_str();
    candidateFile.entry_point = "SgemmSrt2AccumK_CS";
    candidateFile.dispatch = originalDescriptor.dispatch;
    candidateFile.k_packing_factor = 1u;
    PrometheusAuditShaderRegistry registry;
    std::string error;
    ASSERT_TRUE(registry.RegisterFile(candidateFile, &error), error);
    const PrometheusAuditShaderDescriptor* candidateDescriptor = registry.Find("SRT-2accum-K");
    ASSERT_TRUE(candidateDescriptor != nullptr, "candidate descriptor resolves");

    prom_sgemm_audit_execution_descriptor original{};
    original.spirv_words = originalDescriptor.spirv_words;
    original.spirv_size_bytes = originalDescriptor.spirv_size_bytes;
    original.entry_point = originalDescriptor.entry_point;
    original.dispatch = originalDescriptor.dispatch;
    original.compute_mode = static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED);
    prom_sgemm_audit_execution_descriptor candidate = original;
    candidate.spirv_words = candidateDescriptor->spirv_words;
    candidate.spirv_size_bytes = candidateDescriptor->spirv_size_bytes;
    candidate.entry_point = candidateDescriptor->entry_point;

    prom_sgemm_audit_execution_result originalResult{};
    prom_sgemm_audit_execution_result candidateResult{};
    ASSERT_TRUE(RunAuditModule(runtime, original, a, b, &originalOutput, m, n, k, &originalResult), "passing replay original executes");
    ASSERT_TRUE(RunAuditModule(runtime, candidate, a, b, &candidateOutput, m, n, k, &candidateResult), "passing replay candidate executes");
    const std::string replayId = prometheus_audit_replay_identity(originalDescriptor, *candidateDescriptor, m, n, k, kAuditSeed);
    const std::string replayIdAgain = prometheus_audit_replay_identity(originalDescriptor, *candidateDescriptor, m, n, k, kAuditSeed);
    ASSERT_EQUAL(replayId, replayIdAgain, "passing replay identity is stable");
    ASSERT_TRUE(!FindFirstMismatch(expected, originalOutput, candidateOutput, n, 0.002f).has_mismatch, "passing replay remains correct");

    std::vector<float> failedCandidate = candidateOutput;
    failedCandidate[0] += 0.5f;
    const MismatchDetail firstFailure = FindFirstMismatch(expected, originalOutput, failedCandidate, n, 0.002f);
    ASSERT_TRUE(firstFailure.has_mismatch, "synthetic failure produces a mismatch");
    ASSERT_EQUAL(0u, firstFailure.row, "synthetic failure mismatch row is stable");
    ASSERT_EQUAL(0u, firstFailure.column, "synthetic failure mismatch column is stable");
    ASSERT_NEAR(expected[0], firstFailure.expected, 0.000001f, "synthetic failure records expected value");
    ASSERT_NEAR(originalOutput[0], firstFailure.original, 0.000001f, "synthetic failure records original value");
    ASSERT_NEAR(failedCandidate[0], firstFailure.candidate, 0.000001f, "synthetic failure records candidate value");

    std::vector<float> originalOutputAgain;
    std::vector<float> candidateOutputAgain;
    ASSERT_TRUE(RunAuditModule(runtime, original, a, b, &originalOutputAgain, m, n, k, &originalResult), "failing replay original executes");
    ASSERT_TRUE(RunAuditModule(runtime, candidate, a, b, &candidateOutputAgain, m, n, k, &candidateResult), "failing replay candidate executes");
    candidateOutputAgain[0] += 0.5f;
    const MismatchDetail secondFailure = FindFirstMismatch(expected, originalOutputAgain, candidateOutputAgain, n, 0.002f);
    ASSERT_TRUE(secondFailure.has_mismatch, "replayed synthetic failure still mismatches");
    ASSERT_EQUAL(firstFailure.row, secondFailure.row, "failing replay row is stable");
    ASSERT_EQUAL(firstFailure.column, secondFailure.column, "failing replay column is stable");
    ASSERT_NEAR(firstFailure.expected, secondFailure.expected, 0.000001f, "failing replay expected value is stable");
    ASSERT_NEAR(firstFailure.original, secondFailure.original, 0.000001f, "failing replay original value is stable");
    ASSERT_NEAR(firstFailure.candidate, secondFailure.candidate, 0.000001f, "failing replay candidate value is stable");

    const ValidationAccounting validation = CollectValidationAccounting();
    const TimingStats passOriginalTiming = MeasureTiming(runtime, original, a, b, m, n, k);
    const TimingStats passCandidateTiming = MeasureTiming(runtime, candidate, a, b, m, n, k);
    ASSERT_TRUE(!passOriginalTiming.samples.empty(), "passing replay original timing samples are captured");
    ASSERT_TRUE(!passCandidateTiming.samples.empty(), "passing replay candidate timing samples are captured");
    PairwiseRunRecord passing{};
    passing.pair_name = "SRT-2accum-K";
    passing.original_name = "SRT-2accum-K";
    passing.candidate_name = "SRT-2accum-K";
    passing.original_entry = original.entry_point;
    passing.candidate_entry = candidate.entry_point;
    passing.original_hash = prometheus_audit_validate_shader(originalDescriptor).spirv_hash;
    passing.candidate_hash = prometheus_audit_validate_shader(*candidateDescriptor).spirv_hash;
    passing.m = m;
    passing.n = n;
    passing.k = k;
    passing.original_dispatch = originalResult.dispatch_geometry;
    passing.candidate_dispatch = candidateResult.dispatch_geometry;
    passing.original_footprint = originalDescriptor.dispatch;
    passing.candidate_footprint = candidateDescriptor->dispatch;
    passing.compute_mode = static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED);
    passing.tolerance = 0.002f;
    passing.original_vs_cpu = true;
    passing.candidate_vs_cpu = true;
    passing.original_vs_candidate = true;
    passing.original_timing = passOriginalTiming;
    passing.candidate_timing = passCandidateTiming;
    passing.replay_id = replayId;

    PairwiseRunRecord failing = passing;
    failing.first_mismatch = firstFailure;
    failing.original_vs_candidate = false;
    failing.candidate_vs_cpu = false;

    const std::string passingJson = RenderRunJson(passing, validation);
    const std::string failingJson = RenderRunJson(failing, validation);
    ASSERT_EQUAL(passingJson, RenderRunJson(passing, validation), "passing replay JSON is deterministic");
    ASSERT_EQUAL(failingJson, RenderRunJson(failing, validation), "failing replay JSON is deterministic");
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m34a_passing_replay.json", passingJson), "passing replay JSON is written");
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m34a_failing_replay.json", failingJson), "failing replay JSON is written");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM38bMemorySelectionAndProfilePolicy)
{
    VkPhysicalDeviceMemoryProperties properties{};
    properties.memoryHeapCount = 2u;
    properties.memoryHeaps[0].size = 8ull * 1024ull * 1024ull * 1024ull;
    properties.memoryHeaps[0].flags = VK_MEMORY_HEAP_DEVICE_LOCAL_BIT;
    properties.memoryHeaps[1].size = 16ull * 1024ull * 1024ull * 1024ull;
    properties.memoryTypeCount = 4u;
    properties.memoryTypes[0] = {VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0u};
    properties.memoryTypes[1] = {VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1u};
    properties.memoryTypes[2] = {VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT | VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                 VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 0u};
    properties.memoryTypes[3] = {VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT |
                                 VK_MEMORY_PROPERTY_HOST_CACHED_BIT, 1u};
    constexpr std::uint32_t allTypes = 0x0fu;
    ASSERT_EQUAL(0u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL),
                 "pure device-local selection excludes host-visible types");
    ASSERT_EQUAL(1u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM),
                 "coherent system selection excludes device-local types");
    ASSERT_EQUAL(2u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL),
                 "mapped device-local selection requires all three property flags");
    ASSERT_EQUAL(std::numeric_limits<std::uint32_t>::max(),
                 prom_vk_find_memory_type_for_placement(&properties, 0x03u,
                     PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL),
                 "mapped device-local absence is explicit");
    ASSERT_EQUAL(2u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL),
                 "mixed placement A may select mapped device-local");
    ASSERT_EQUAL(0u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL),
                 "mixed placement B may independently remain pure local");
    ASSERT_EQUAL(0u, prom_vk_find_memory_type_for_placement(&properties, allTypes,
                                                             PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL),
                 "mixed placement C may independently remain pure local");

    prom_sgemm_memory_profile profile{};
    profile.enabled = 1u;
    profile.kernel_compute_mode = PROM_VK_COMPUTE_PACKED4_FP32;
    profile.vendor_id = 0x10deu;
    profile.device_id = 0x2488u;
    profile.driver_version_min = 100u;
    profile.driver_version_max = 200u;
    profile.input_placement = PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL;
    profile.output_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
    profile.minimum_m = 64u; profile.minimum_n = 64u; profile.minimum_k = 64u;
    profile.maximum_total_bytes = 4ull * 1024ull * 1024ull;
    profile.minimum_budget_headroom_bytes = 128ull * 1024ull * 1024ull;
    prom_sgemm_memory_profile_facts facts{};
    facts.experiment_enabled = 1u;
    facts.kernel_compute_mode = PROM_VK_COMPUTE_PACKED4_FP32;
    facts.vendor_id = 0x10deu; facts.device_id = 0x2488u; facts.driver_version = 150u;
    facts.mapped_device_local_type_exists = 1u;
    facts.m = 512u; facts.n = 512u; facts.k = 512u;
    facts.total_bytes = 3ull * 1024ull * 1024ull;
    facts.heap_budget_bytes = 7ull * 1024ull * 1024ull * 1024ull;
    facts.heap_usage_bytes = 1ull * 1024ull * 1024ull * 1024ull;
    prom_sgemm_memory_profile_decision decision{};
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(1u, decision.matched, "known enabled profile matches inside the proven envelope");
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL, decision.fallback_placement,
                 "fallback remains pure device-local");
    profile.kernel_compute_mode = PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM;
    facts.kernel_compute_mode = PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(1u, decision.matched, "FP16 is the other explicitly eligible profile kernel");
    profile.kernel_compute_mode = PROM_VK_COMPUTE_BASELINE;
    facts.kernel_compute_mode = PROM_VK_COMPUTE_BASELINE;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_KERNEL, decision.reason,
                 "unapproved kernels cannot opt into alternate placement");
    profile.kernel_compute_mode = PROM_VK_COMPUTE_PACKED4_FP32;
    facts.kernel_compute_mode = PROM_VK_COMPUTE_PACKED4_FP32;
    facts.driver_version = profile.driver_version_min - 1u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_DRIVER, decision.reason, "unknown driver rejects the profile");
    facts.driver_version = 150u;
    facts.m = profile.minimum_m - 1u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_SHAPE, decision.reason, "out-of-envelope shape rejects the profile");
    facts.m = 512u;
    facts.device_id = 0x9999u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_DEVICE, decision.reason, "unknown device rejects the profile");
    facts.device_id = profile.device_id; facts.mapped_device_local_type_exists = 0u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_MEMORY_TYPE, decision.reason, "missing mapped type falls back");
    facts.mapped_device_local_type_exists = 1u; facts.heap_budget_bytes = facts.heap_usage_bytes + facts.total_bytes;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_BUDGET, decision.reason, "budget headroom rejects the preference");
    facts.heap_budget_bytes = 8ull * 1024ull * 1024ull * 1024ull; facts.total_bytes = profile.maximum_total_bytes + 1u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_CAPACITY, decision.reason, "profile maximum bytes are bounded");
    facts.total_bytes = 3ull * 1024ull * 1024ull; facts.experiment_enabled = 0u;
    prom_sgemm_memory_profile_select(&profile, &facts, &decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_DISABLED, decision.reason, "disabled experiment cannot change placement");
    prom_sgemm_memory_profile_allocation_failed(&decision);
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PROFILE_REASON_ALLOCATION_FAILURE, decision.reason,
                 "allocation failure is recorded explicitly");
    ASSERT_EQUAL(PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL, decision.fallback_placement,
                 "allocation failure preserves deterministic pure-local fallback");
}

FACT(PrometheusM38bPackedSizingFiniteComparatorAndAggregation)
{
    const auto packed4Bytes = [](std::uint32_t rows, std::uint32_t k) {
        const std::uint64_t paddedK = (static_cast<std::uint64_t>(k) + 3u) & ~3ull;
        return static_cast<std::uint64_t>(rows) * paddedK * sizeof(float);
    };
    const auto fp16Bytes = [](std::uint32_t rows, std::uint32_t columns) {
        const std::uint64_t elements = static_cast<std::uint64_t>(rows) * columns;
        return ((elements + 1u) / 2u) * sizeof(std::uint32_t);
    };
    ASSERT_EQUAL(67'056ull, packed4Bytes(127u, 129u), "Packed4 pads hostile K to four lanes");
    ASSERT_EQUAL(32'768ull, fp16Bytes(127u, 129u), "FP16 flat-half layout packs the odd final lane");
    const auto finiteEqual = [](float expected, float actual, float tolerance) {
        return std::isfinite(expected) && std::isfinite(actual) && std::fabs(expected - actual) <= tolerance;
    };
    ASSERT_TRUE(finiteEqual(1.0f, 1.001f, 0.002f), "finite comparator accepts in-tolerance values");
    ASSERT_FALSE(finiteEqual(1.0f, std::numeric_limits<float>::infinity(), 1.0f), "finite comparator rejects infinity");
    ASSERT_FALSE(finiteEqual(std::numeric_limits<float>::quiet_NaN(), 0.0f, 1.0f), "finite comparator rejects NaN");
    const TimingStats first = ComputeTimingStats({50u, 10u, 40u, 20u, 30u});
    const TimingStats second = ComputeTimingStats({50u, 10u, 40u, 20u, 30u});
    ASSERT_EQUAL(first.minimum_ns, second.minimum_ns, "aggregation minimum is deterministic");
    ASSERT_EQUAL(first.median_ns, second.median_ns, "aggregation median is deterministic");
    ASSERT_EQUAL(first.p90_ns, second.p90_ns, "aggregation percentile is deterministic");
    ASSERT_TRUE(first.coefficient_of_variation > 0.0, "aggregation reports variability");
}

namespace
{
struct M38bKernel {
    const char* name;
    const std::uint32_t* words;
    std::size_t bytes;
    const char* entry;
    prom_sgemm_kernel_dispatch_metadata dispatch;
    std::uint32_t mode;
    float tolerance;
};

struct M38bPlacementRun {
    bool ran = false;
    bool correct = false;
    TimingStats gpu;
    TimingStats preparation;
    TimingStats end_to_end;
    prom_sgemm_placement_benchmark_result execution{};
};

M38bPlacementRun RunM38bPlacement(void* runtime,
                                  const M38bKernel& kernel,
                                  const std::vector<float>& a,
                                  const std::vector<float>& b,
                                  const std::vector<float>& expected,
                                  std::uint32_t m,
                                  std::uint32_t n,
                                  std::uint32_t k,
                                  const prom_sgemm_placement_benchmark_options& options)
{
    M38bPlacementRun run;
    std::vector<float> output(static_cast<std::size_t>(m) * n, 0.0f);
    std::vector<std::uint64_t> gpu(options.iterations, 0u);
    std::vector<std::uint64_t> preparation(options.iterations, 0u);
    std::vector<std::uint64_t> endToEnd(options.iterations, 0u);
    prom_sgemm_audit_execution_descriptor descriptor{};
    descriptor.spirv_words = kernel.words;
    descriptor.spirv_size_bytes = kernel.bytes;
    descriptor.entry_point = kernel.entry;
    descriptor.dispatch = kernel.dispatch;
    descriptor.compute_mode = kernel.mode;
    descriptor.provenance = "M38b exact production asset";
    run.ran = prom_reactor_runtime_sgemm_placement_benchmark_impl(
                  runtime, a.data(), b.data(), output.data(), m, n, k, &descriptor, &options,
                  gpu.data(), preparation.data(), endToEnd.data(), options.iterations, &run.execution) == PROM_OK;
    run.correct = run.ran && output.size() == expected.size();
    if (run.correct) {
        for (std::size_t index = 0u; index < output.size(); ++index) {
            if (!std::isfinite(output[index]) || !std::isfinite(expected[index]) ||
                std::fabs(output[index] - expected[index]) > kernel.tolerance) {
                run.correct = false;
                break;
            }
        }
    }
    if (run.ran) {
        run.gpu = ComputeTimingStats(gpu);
        run.preparation = ComputeTimingStats(preparation);
        run.end_to_end = ComputeTimingStats(endToEnd);
    }
    return run;
}

void AppendM38bPlacementRun(std::ostringstream& report,
                            const char* section,
                            const char* kernel,
                            const char* configuration,
                            std::uint32_t m,
                            std::uint32_t n,
                            std::uint32_t k,
                            std::uint32_t reuseMode,
                            const M38bPlacementRun& run,
                            bool* first)
{
    if (!*first) report << ',';
    *first = false;
    report << "{\"section\":"; AppendJsonString(report, section);
    report << ",\"kernel\":"; AppendJsonString(report, kernel);
    report << ",\"configuration\":"; AppendJsonString(report, configuration);
    report << ",\"m\":" << m << ",\"n\":" << n << ",\"k\":" << k
           << ",\"reuse_mode\":" << reuseMode
           << ",\"ran\":" << (run.ran ? "true" : "false")
           << ",\"correct\":" << (run.correct ? "true" : "false")
           << ",\"supported\":" << run.execution.supported
           << ",\"placements\":[" << run.execution.a_memory_property_flags << ','
           << run.execution.b_memory_property_flags << ',' << run.execution.c_memory_property_flags << ']'
           << ",\"memory_type_indices\":[" << run.execution.a_memory_type_index << ','
           << run.execution.b_memory_type_index << ',' << run.execution.c_memory_type_index << ']'
           << ",\"heap_indices\":[" << run.execution.a_heap_index << ','
           << run.execution.b_heap_index << ',' << run.execution.c_heap_index << ']'
           << ",\"buffer_bytes\":[" << run.execution.a_buffer_bytes << ','
           << run.execution.b_buffer_bytes << ',' << run.execution.c_buffer_bytes << ']'
           << ",\"initial_preparation_ns\":" << run.execution.initial_preparation_ns
           << ",\"gpu\":" << RenderTimingJson(run.gpu)
           << ",\"preparation\":" << RenderTimingJson(run.preparation)
           << ",\"end_to_end\":" << RenderTimingJson(run.end_to_end) << '}';
}
}

FACT(PrometheusM38bPackedMemoryPlacementExperiment)
{
    const std::array<M38bKernel, 2> packedKernels = {{
        {"Packed4", k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "SgemmPacked4_CS",
         {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_PACKED4_FP32, 0.002f},
        {"FP16", k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv),
         "SgemmFp16StorageFp32Accum_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u},
         PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM, 0.03f},
    }};
    const std::array<M38bKernel, 5> competitors = {{
        {"scalar", k_prom_sgemm_spirv, k_prom_sgemm_spirv_size_bytes, "main",
         {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_BASELINE, 0.002f},
        {"tiled", k_prom_sgemm_tiled_spirv, sizeof(k_prom_sgemm_tiled_spirv), "main",
         {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_TILED, 0.002f},
        {"memory-conservative", k_prom_sgemm_memory_conservative_spirv, sizeof(k_prom_sgemm_memory_conservative_spirv), "main",
         {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_TILED, 0.002f},
        {"B2x2", k_prom_m37b_b2x2_spirv, sizeof(k_prom_m37b_b2x2_spirv), "SgemmB2x2_CS",
         {8u, 8u, 1u, 2u, 2u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_TILED, 0.002f},
        {"A2x4", k_prom_m37b_a2x4_spirv, sizeof(k_prom_m37b_a2x4_spirv), "SgemmA2x4_CS",
         {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u}, PROM_VK_COMPUTE_TILED, 0.002f},
    }};
    struct PlacementCase { const char* name; std::uint32_t a; std::uint32_t b; std::uint32_t c; };
    constexpr std::uint32_t local = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
    constexpr std::uint32_t mapped = PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_DEVICE_LOCAL;
    constexpr std::uint32_t system = PROM_SGEMM_MEMORY_PLACEMENT_HOST_VISIBLE_COHERENT_SYSTEM;
    const std::array<PlacementCase, 8> placementMatrix = {{
        {"all-pure-device-local", local, local, local},
        {"all-mapped-device-local", mapped, mapped, mapped},
        {"inputs-mapped-output-local", mapped, mapped, local},
        {"inputs-local-output-mapped", local, local, mapped},
        {"A-mapped-only", mapped, local, local},
        {"B-mapped-only", local, mapped, local},
        {"C-mapped-only", local, local, mapped},
        {"all-coherent-system-control", system, system, system},
    }};
    const std::array<std::array<std::uint32_t, 3>, 7> workloads = {{
        {{64u, 64u, 64u}}, {{256u, 256u, 256u}}, {{512u, 512u, 512u}},
        {{1024u, 256u, 512u}}, {{256u, 1024u, 512u}}, {{127u, 131u, 129u}}, {{511u, 509u, 513u}},
    }};
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "M38b runtime creation succeeds");
    PrometheusCaps caps{};
    if (runtime == nullptr || prom_reactor_runtime_probe_impl(runtime, &caps) != PROM_OK ||
        caps.available == 0u || caps.backend_type == PROM_BACKEND_STUB) {
        if (runtime != nullptr) prom_reactor_runtime_destroy_impl(runtime);
        SKIP("real Vulkan runtime unavailable");
    }
    std::ostringstream report;
    report << "{\"schema_version\":1,\"rows\":[";
    bool first = true;

    std::vector<float> representativeA, representativeB, representativeExpected;
    FillInputs(&representativeA, &representativeB, 512u, 512u, 512u);
    ReferenceSgemm(representativeA, representativeB, &representativeExpected, 512u, 512u, 512u);
    for (const M38bKernel& kernel : packedKernels) {
        for (const PlacementCase& placement : placementMatrix) {
            prom_sgemm_placement_benchmark_options options{};
            options.a_placement = placement.a; options.b_placement = placement.b; options.c_placement = placement.c;
            options.reuse_mode = PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD;
            options.warmup = 1u; options.iterations = 9u;
            const M38bPlacementRun run = RunM38bPlacement(runtime, kernel, representativeA, representativeB,
                                                           representativeExpected, 512u, 512u, 512u, options);
            ASSERT_TRUE(run.ran && run.correct, "M38b full placement matrix row executes correctly");
            AppendM38bPlacementRun(report, "placement-matrix", kernel.name, placement.name,
                                   512u, 512u, 512u, options.reuse_mode, run, &first);
        }
    }

    const std::array<PlacementCase, 3> workloadPlacements = {{
        {"all-pure-device-local", local, local, local},
        {"all-mapped-device-local", mapped, mapped, mapped},
        {"inputs-mapped-output-local", mapped, mapped, local},
    }};
    for (const auto& shape : workloads) {
        std::vector<float> a, b, expected;
        FillInputs(&a, &b, shape[0], shape[1], shape[2]);
        ReferenceSgemm(a, b, &expected, shape[0], shape[1], shape[2]);
        for (const M38bKernel& kernel : packedKernels) {
            for (const PlacementCase& placement : workloadPlacements) {
                prom_sgemm_placement_benchmark_options options{};
                options.a_placement = placement.a; options.b_placement = placement.b; options.c_placement = placement.c;
                options.reuse_mode = PROM_SGEMM_PLACEMENT_REUSE_WARM;
                options.warmup = 1u; options.iterations = 7u;
                const M38bPlacementRun run = RunM38bPlacement(runtime, kernel, a, b, expected,
                                                               shape[0], shape[1], shape[2], options);
                ASSERT_TRUE(run.ran && run.correct, "M38b workload placement row executes correctly");
                AppendM38bPlacementRun(report, "workload-matrix", kernel.name, placement.name,
                                       shape[0], shape[1], shape[2], options.reuse_mode, run, &first);
            }
        }
        for (const M38bKernel& kernel : competitors) {
            prom_sgemm_audit_execution_descriptor descriptor{};
            descriptor.spirv_words = kernel.words; descriptor.spirv_size_bytes = kernel.bytes;
            descriptor.entry_point = kernel.entry; descriptor.dispatch = kernel.dispatch;
            descriptor.compute_mode = kernel.mode; descriptor.provenance = "M38b production competitor";
            std::vector<float> output;
            prom_sgemm_audit_execution_result execution{};
            const bool ran = RunAuditModule(runtime, descriptor, a, b, &output, shape[0], shape[1], shape[2], &execution);
            bool correct = ran;
            for (std::size_t index = 0u; correct && index < output.size(); ++index) {
                correct = std::isfinite(output[index]) && std::fabs(output[index] - expected[index]) <= kernel.tolerance;
            }
            const TimingStats timing = ran ? MeasureTiming(runtime, descriptor, a, b, shape[0], shape[1], shape[2]) : TimingStats{};
            ASSERT_TRUE(ran && correct && !timing.samples.empty(), "M38b production competitor executes correctly");
            if (!first) report << ','; first = false;
            report << "{\"section\":\"competitor\",\"kernel\":"; AppendJsonString(report, kernel.name);
            report << ",\"configuration\":\"production-pure-device-local\",\"m\":" << shape[0]
                   << ",\"n\":" << shape[1] << ",\"k\":" << shape[2]
                   << ",\"ran\":" << (ran ? "true" : "false") << ",\"correct\":" << (correct ? "true" : "false")
                   << ",\"gpu\":" << RenderTimingJson(timing) << '}';
        }
        {
            std::vector<float> output(static_cast<std::size_t>(shape[0]) * shape[1], 0.0f);
            std::vector<std::uint64_t> samples;
            std::uint32_t stage = 0u;
            int detail = 0;
            bool correct = true;
            std::uint32_t executedVariant = 0u;
            for (std::uint32_t iteration = 0u; iteration < 6u; ++iteration) {
                const bool ran = prom_reactor_runtime_sgemm_impl(runtime, a.data(), b.data(), output.data(),
                                                                  shape[0], shape[1], shape[2], &stage, &detail) == PROM_OK;
                if (!ran) { correct = false; break; }
                if (iteration != 0u) samples.push_back(static_cast<prometheus_runtime*>(runtime)->last_gpu_duration_ns);
                executedVariant = static_cast<prometheus_runtime*>(runtime)->slot_diag.px16_m6_executed_dispatch_variant;
            }
            for (std::size_t index = 0u; correct && index < output.size(); ++index) {
                correct = std::isfinite(output[index]) && std::fabs(output[index] - expected[index]) <= 0.03f;
            }
            const TimingStats timing = ComputeTimingStats(samples);
            ASSERT_TRUE(correct && !timing.samples.empty(), "M38b production-selected path executes correctly");
            if (!first) report << ','; first = false;
            report << "{\"section\":\"production-selected\",\"kernel\":\"production-selected\""
                   << ",\"configuration\":\"unchanged-policy\",\"m\":" << shape[0]
                   << ",\"n\":" << shape[1] << ",\"k\":" << shape[2]
                   << ",\"executed_variant\":" << executedVariant
                   << ",\"ran\":true,\"correct\":" << (correct ? "true" : "false")
                   << ",\"gpu\":" << RenderTimingJson(timing) << '}';
        }
    }

    for (const M38bKernel& kernel : packedKernels) {
        for (const std::uint32_t reuseMode : std::array<std::uint32_t, 4>{{
                 PROM_SGEMM_PLACEMENT_REUSE_COLD_ALLOCATION, PROM_SGEMM_PLACEMENT_REUSE_WARM,
                 PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD, PROM_SGEMM_PLACEMENT_REUSE_OUTPUT_TURNOVER}}) {
            prom_sgemm_placement_benchmark_options options{};
            options.a_placement = mapped; options.b_placement = mapped; options.c_placement = mapped;
            options.reuse_mode = reuseMode; options.warmup = 0u; options.iterations = 7u;
            const M38bPlacementRun run = RunM38bPlacement(runtime, kernel, representativeA, representativeB,
                                                           representativeExpected, 512u, 512u, 512u, options);
            ASSERT_TRUE(run.ran && run.correct, "M38b reuse mode executes correctly");
            AppendM38bPlacementRun(report, "reuse-mode", kernel.name, "all-mapped-device-local",
                                   512u, 512u, 512u, reuseMode, run, &first);
        }
        for (const std::uint32_t dispatches : std::array<std::uint32_t, 3>{{1u, 10u, 100u}}) {
            prom_sgemm_placement_benchmark_options options{};
            options.a_placement = mapped; options.b_placement = mapped; options.c_placement = mapped;
            options.reuse_mode = PROM_SGEMM_PLACEMENT_REUSE_WARM; options.iterations = dispatches;
            const M38bPlacementRun run = RunM38bPlacement(runtime, kernel, representativeA, representativeB,
                                                           representativeExpected, 512u, 512u, 512u, options);
            ASSERT_TRUE(run.ran && run.correct, "M38b amortized warm run executes correctly");
            std::string name = "warm-amortized-" + std::to_string(dispatches);
            AppendM38bPlacementRun(report, "amortized", kernel.name, name.c_str(),
                                   512u, 512u, 512u, options.reuse_mode, run, &first);
        }
    }

    for (std::uint32_t round = 0u; round < 5u; ++round) {
        std::vector<float> rotatedA = representativeA;
        std::vector<float> rotatedExpected = representativeExpected;
        const float scale = 1.0f + static_cast<float>(round) * 0.03125f;
        for (float& value : rotatedA) value *= scale;
        for (float& value : rotatedExpected) value *= scale;
        for (const M38bKernel& kernel : packedKernels) {
            const std::array<PlacementCase, 2> order = round % 2u == 0u
                ? std::array<PlacementCase, 2>{{{"all-pure-device-local", local, local, local}, {"all-mapped-device-local", mapped, mapped, mapped}}}
                : std::array<PlacementCase, 2>{{{"all-mapped-device-local", mapped, mapped, mapped}, {"all-pure-device-local", local, local, local}}};
            for (const PlacementCase& placement : order) {
                prom_sgemm_placement_benchmark_options options{};
                options.a_placement = placement.a; options.b_placement = placement.b; options.c_placement = placement.c;
                options.reuse_mode = PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD; options.warmup = 1u; options.iterations = 9u;
                options.perturb_cache = 1u; options.cache_perturbation_bytes = 32ull * 1024ull * 1024ull;
                const M38bPlacementRun run = RunM38bPlacement(runtime, kernel, rotatedA, representativeB,
                                                               rotatedExpected, 512u, 512u, 512u, options);
                ASSERT_TRUE(run.ran && run.correct, "M38b repeatability/cache perturbation row executes correctly");
                std::string name = std::string(placement.name) + "-round-" + std::to_string(round);
                AppendM38bPlacementRun(report, "repeatability", kernel.name, name.c_str(),
                                       512u, 512u, 512u, options.reuse_mode, run, &first);
            }
        }
    }

    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services), "M38b obtains Vulkan services for capacity audit");
    VkPhysicalDeviceMemoryProperties2 memoryProperties2{};
    VkPhysicalDeviceMemoryBudgetPropertiesEXT budget{};
    memoryProperties2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_MEMORY_PROPERTIES_2;
    budget.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_MEMORY_BUDGET_PROPERTIES_EXT;
    bool budgetAvailable = false;
    std::uint32_t extensionCount = 0u;
    if (vkEnumerateDeviceExtensionProperties(services.physical_device, nullptr, &extensionCount, nullptr) == VK_SUCCESS) {
        std::vector<VkExtensionProperties> extensions(extensionCount);
        if (vkEnumerateDeviceExtensionProperties(services.physical_device, nullptr, &extensionCount, extensions.data()) == VK_SUCCESS) {
            budgetAvailable = std::any_of(extensions.begin(), extensions.end(), [](const VkExtensionProperties& extension) {
                return std::strcmp(extension.extensionName, VK_EXT_MEMORY_BUDGET_EXTENSION_NAME) == 0;
            });
        }
    }
    if (budgetAvailable) memoryProperties2.pNext = &budget;
    vkGetPhysicalDeviceMemoryProperties2(services.physical_device, &memoryProperties2);
    VkPhysicalDeviceProperties deviceProperties{};
    vkGetPhysicalDeviceProperties(services.physical_device, &deviceProperties);
    PrometheusVulkanDeviceDiagnostics device{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_vulkan_device_diagnostics_impl(runtime, &device), "M38b device identity is available");
    report << "],\"capacity\":{\"device_name\":"; AppendJsonString(report, device.device_name);
    report << ",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id
           << ",\"driver_version\":" << device.driver_version << ",\"budget_available\":" << (budgetAvailable ? "true" : "false")
           << ",\"max_memory_allocation_count\":" << deviceProperties.limits.maxMemoryAllocationCount
           << ",\"max_storage_buffer_range\":" << deviceProperties.limits.maxStorageBufferRange
           << ",\"memory_types\":[";
    for (std::uint32_t index = 0u; index < memoryProperties2.memoryProperties.memoryTypeCount; ++index) {
        if (index != 0u) report << ',';
        const VkMemoryType& type = memoryProperties2.memoryProperties.memoryTypes[index];
        report << "{\"index\":" << index << ",\"flags\":" << type.propertyFlags << ",\"heap\":" << type.heapIndex << '}';
    }
    report << "],\"heaps\":[";
    for (std::uint32_t index = 0u; index < memoryProperties2.memoryProperties.memoryHeapCount; ++index) {
        if (index != 0u) report << ',';
        report << "{\"index\":" << index << ",\"size\":" << memoryProperties2.memoryProperties.memoryHeaps[index].size
               << ",\"flags\":" << memoryProperties2.memoryProperties.memoryHeaps[index].flags
               << ",\"budget\":" << (budgetAvailable ? budget.heapBudget[index] : 0u)
               << ",\"usage\":" << (budgetAvailable ? budget.heapUsage[index] : 0u) << '}';
    }
    ValidationAccounting validation = CollectValidationAccounting();
    CaptureRuntimeValidationAccounting(static_cast<const prometheus_runtime*>(runtime), &validation);
    report << "],\"validation\":" << RenderValidationJson(validation) << "}}";
    ASSERT_EQUAL(0u, validation.warning_count, "M38b validation warning count remains zero");
    ASSERT_EQUAL(0u, validation.error_count, "M38b validation error count remains zero");
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m38b_memory_placement.json", report.str()),
                "M38b machine-readable experiment artifact is written");
}
