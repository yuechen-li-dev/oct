#include "test_harness.h"

#include "../reactor_prometheus_audit.h"
#include "../reactor_vulkan.h"
#include "../reactor_vulkan_srt_2accum_k_spirv.h"
#include "../reactor_vulkan_b2x2_row_major_biased_spirv.h"
#include "../reactor_vulkan_a2x4_row_biased_accum8_spirv.h"
#include "../reactor_vulkan_packed4_spirv.h"
#include "../reactor_vulkan_fp16_spirv.h"
#include "../reactor_judgment_engine.h"

#include <filesystem>
#include <cstring>
#include <array>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>

namespace
{
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
    const std::uint32_t* original_words;
    std::size_t original_size;
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
        {"SRT-2accum-K", k_prom_sgemm_srt_2accum_k_spirv, sizeof(k_prom_sgemm_srt_2accum_k_spirv), "sgemm_srt_2accum_k.spv", "SgemmSrt2AccumK_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"B2x2-row-major-biased", k_prom_sgemm_b2x2_row_major_biased_spirv, sizeof(k_prom_sgemm_b2x2_row_major_biased_spirv), "sgemm_b2x2_row_major_biased.spv", "SgemmB2x2_CS", {8u, 8u, 1u, 2u, 2u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"A2x4-row-biased-accum8", k_prom_sgemm_a2x4_row_biased_accum8_spirv, sizeof(k_prom_sgemm_a2x4_row_biased_accum8_spirv), "sgemm_a2x4_row_biased_accum8.spv", "SgemmA2x4_CS", {8u, 8u, 1u, 2u, 4u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED), 0.002f},
        {"Packed4FP32", k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "sgemm_packed4_fp32.spv", "SgemmPacked4_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_PACKED4_FP32), 0.002f},
        {"FP16-storage/FP32-accum", k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv), "sgemm_fp16_storage_fp32_accum.spv", "SgemmFp16StorageFp32Accum_CS", {8u, 8u, 1u, 1u, 1u, 0u, 0u, 0u, 0u}, static_cast<std::uint32_t>(PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM), 0.03f},
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
    const std::filesystem::path generated = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) / "internal" / "prometheus" / "DevelopmentReport" / "artifacts" / "SDSL_V_ORIGINAL_SPIRV_REWRITE" / "generated";
    std::ostringstream report;
    report << "{\"pairs\":[";
    bool firstReport = true;
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
        prom_sgemm_audit_execution_descriptor original{};
        original.spirv_words = pair.original_words;
        original.spirv_size_bytes = pair.original_size;
        original.entry_point = "main";
        original.dispatch = pair.dispatch;
        original.compute_mode = pair.compute_mode;
        prom_sgemm_audit_execution_descriptor generatedDescriptor = original;
        generatedDescriptor.spirv_words = candidate->spirv_words;
        generatedDescriptor.spirv_size_bytes = candidate->spirv_size_bytes;
        generatedDescriptor.entry_point = candidate->entry_point;
        std::uint64_t originalTime = 0u;
        std::uint64_t candidateTime = 0u;
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
            originalTime += originalResult.gpu_duration_ns;
            candidateTime += candidateResult.gpu_duration_ns;
            for (std::size_t index = 0; index < expected.size(); ++index) {
                ASSERT_NEAR(expected[index], originalOutput[index], pair.tolerance, "original matches CPU reference");
                ASSERT_NEAR(expected[index], candidateOutput[index], pair.tolerance, "candidate matches CPU reference");
                ASSERT_NEAR(originalOutput[index], candidateOutput[index], pair.tolerance, "original and candidate match");
            }
        }
        if (!firstReport) report << ',';
        firstReport = false;
        report << "{\"name\":\"" << pair.name << "\",\"original_gpu_ns_sum\":" << originalTime
               << ",\"candidate_gpu_ns_sum\":" << candidateTime << ",\"cases\":" << shapes.size() << '}';
    }
    report << "]}";
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m34a_pairwise_summary.json", report.str()), "pairwise audit JSON is written");
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
