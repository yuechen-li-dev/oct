#include "test_harness.h"

#include "../reactor_prometheus_audit.h"
#include "../reactor_vulkan_srt_2accum_k_spirv.h"

#include <filesystem>
#include <cstring>
#include <fstream>
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
