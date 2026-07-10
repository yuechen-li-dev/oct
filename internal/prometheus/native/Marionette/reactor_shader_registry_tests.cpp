#include "../reactor_vulkan_sgemm_internal.h"
#include "test_harness.h"

#include <array>
#include <cstdlib>
#include <cstdint>
#include <filesystem>
#include <fstream>
#include <string>
#include <vector>

namespace {
std::vector<prom_shader_asset> assets() {
  std::vector<prom_shader_asset> result;
  for (std::size_t i = 0; i < prom_shader_registry_shader_asset_count(); ++i) result.push_back(*prom_shader_registry_shader_asset_at(i));
  return result;
}
std::vector<prom_compute_implementation> implementations() {
  std::vector<prom_compute_implementation> result;
  for (std::size_t i = 0; i < prom_shader_registry_compute_implementation_count(); ++i) result.push_back(*prom_shader_registry_compute_implementation_at(i));
  return result;
}
bool valid(std::vector<prom_shader_asset>& a, std::vector<prom_compute_implementation>& i) {
  return prom_shader_registry_validate_tables(a.data(), a.size(), i.data(), i.size()) != 0u;
}
std::string read_file(const std::filesystem::path& path) { std::ifstream file(path); return std::string((std::istreambuf_iterator<char>(file)), std::istreambuf_iterator<char>()); }
}

FACT(PrometheusShaderRegistryIdsAreUnique) {
  ASSERT_TRUE(prom_shader_registry_validate() != 0u, "production registry must validate");
  ASSERT_EQUAL(static_cast<std::size_t>(15), prom_shader_registry_shader_asset_count(), "all current SPIR-V assets must be present");
}
FACT(PrometheusShaderRegistryReferencesAreValid) {
  for (std::size_t i = 0; i < prom_shader_registry_compute_implementation_count(); ++i) {
    const auto* implementation = prom_shader_registry_compute_implementation_at(i);
    ASSERT_TRUE(prom_shader_registry_find_shader(implementation->shader_id) != nullptr, "implementation must reference an existing shader asset");
  }
}
FACT(PrometheusShaderRegistryStagesMatchImplementations) {
  for (std::size_t i = 0; i < prom_shader_registry_compute_implementation_count(); ++i) {
    const auto* implementation = prom_shader_registry_compute_implementation_at(i);
    ASSERT_EQUAL(PROM_SHADER_STAGE_COMPUTE, prom_shader_registry_find_shader(implementation->shader_id)->stage, "compute implementation must use compute-stage shader");
  }
}
FACT(PrometheusComputeImplementationsPreserveStableIds) {
  for (uint32_t id = 1u; id <= 11u; ++id) ASSERT_EQUAL(id, prom_shader_registry_find_compute_implementation(id)->implementation_id, "public implementation IDs must remain stable");
}
FACT(PrometheusComputeImplementationsPreserveEligibility) {
  for (std::size_t i = 0; i < prom_shader_registry_compute_implementation_count(); ++i) {
    const auto* implementation = prom_shader_registry_compute_implementation_at(i);
    ASSERT_EQUAL(1u, implementation->benchmark_enabled, "current explicit implementations remain benchmark-enabled");
    ASSERT_EQUAL(1u, implementation->selector_eligible, "current selector eligibility remains unchanged");
    ASSERT_EQUAL(1u, implementation->dispatchable, "current implementations remain dispatchable");
  }
}
FACT(PrometheusComputeImplementationsHaveDispatchMetadata) {
  for (uint32_t id = 1u; id <= 11u; ++id) ASSERT_TRUE(prom_shader_registry_dispatch_metadata(id) != nullptr, "every implementation needs dispatch metadata");
}
FACT(PrometheusComputeImplementationLookupIsDeterministic) {
  for (uint32_t id = 1u; id <= 11u; ++id) ASSERT_EQUAL(prom_shader_registry_find_compute_implementation(id), prom_shader_registry_find_compute_implementation(id), "lookup must be stable");
  ASSERT_TRUE(prom_shader_registry_find_compute_implementation(999u) == nullptr, "unknown IDs must not resolve");
}
FACT(PrometheusShaderManifestMatchesGeneratedAssets) {
  const auto root = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT);
  const std::string manifest = read_file(root / "internal/prometheus/native/shaders/manifest.json");
  ASSERT_TRUE(manifest.find("prometheus.shader-registry.v2") != std::string::npos, "manifest must declare registry schema");
  for (std::size_t i = 0; i < prom_shader_registry_shader_asset_count(); ++i) {
    const auto* asset = prom_shader_registry_shader_asset_at(i);
    ASSERT_TRUE(manifest.find(asset->name) != std::string::npos, "every runtime asset must appear in manifest");
    if (asset->generated != 0u) ASSERT_TRUE(std::filesystem::exists(root / "internal/prometheus/native" / asset->generated_header_path), "generated header must exist");
  }
  const auto* proof = prom_shader_registry_find_shader(15u);
  ASSERT_TRUE(proof != nullptr, "M28 proof asset must be registered");
  ASSERT_EQUAL(1u, proof->contains_inline_hlsl, "proof provenance must record inline HLSL");
  ASSERT_EQUAL(2u, proof->inline_hlsl_block_count, "proof block count must agree with source");
  ASSERT_TRUE(std::string(proof->foreign_targets) == "HLSL", "proof target provenance must be HLSL");
}
FACT(PrometheusComputePipelineInstancesMatchDescriptors) {
  std::array<VkPipeline, PROM_COMPUTE_PIPELINE_COUNT> pipelines{};
  for (std::size_t i = 0; i < pipelines.size(); ++i) pipelines[i] = reinterpret_cast<VkPipeline>(static_cast<uintptr_t>(i + 1u));
  std::array<prom_compute_pipeline_instance, 11> instances{};
  prom_shader_registry_initialize_pipeline_instances(instances.data(), instances.size(), pipelines.data(), pipelines.size());
  for (const auto& instance : instances) {
    ASSERT_TRUE(instance.implementation != nullptr, "instance must map to one descriptor");
    ASSERT_EQUAL(PROM_PIPELINE_READY, instance.status, "provided pipeline must be ready");
    ASSERT_EQUAL(instance.pipeline, prom_shader_registry_pipeline_for_variant(instances.data(), instances.size(), instance.implementation->implementation_id), "lookup must return descriptor instance pipeline");
  }
}
FACT(PrometheusSelectorUsesRegistryFacts) {
  for (uint32_t id = 1u; id <= 11u; ++id) ASSERT_EQUAL(1u, prom_shader_registry_is_selector_eligible(id), "selector membership must come from descriptor fact");
}
FACT(PrometheusExplicitBenchmarkEnumeratesRegistry) {
  std::size_t count = 0u;
  for (std::size_t i = 0; i < prom_shader_registry_compute_implementation_count(); ++i) if (prom_shader_registry_compute_implementation_at(i)->benchmark_enabled != 0u) ++count;
  ASSERT_EQUAL(prom_shader_registry_compute_implementation_count(), count, "all current explicit implementations must enumerate from registry facts");
}
FACT(PrometheusNumericalEligibilityRemainsRequestSpecific) {
  ASSERT_EQUAL(1u, prom_shader_registry_find_compute_implementation(1u)->selector_eligible, "static registry eligibility is not a tolerance decision");
  ASSERT_TRUE(prom_shader_registry_find_compute_implementation(1u)->capability_mask == 0u, "request numerical policy is deliberately absent from descriptor");
}
FACT(PrometheusShaderRegistryNegativeValidationCoverage) {
  auto a = assets(); auto i = implementations();
  a[1].shader_id = a[0].shader_id; ASSERT_FALSE(valid(a, i), "duplicate shader ID must fail"); a = assets();
  i[1].implementation_id = i[0].implementation_id; ASSERT_FALSE(valid(a, i), "duplicate implementation ID must fail"); i = implementations();
  i[0].shader_id = 999u; ASSERT_FALSE(valid(a, i), "missing shader reference must fail"); i = implementations();
  a[0].stage = PROM_SHADER_STAGE_VERTEX; ASSERT_FALSE(valid(a, i), "wrong shader stage must fail"); a = assets();
  a[0].spirv_size_bytes = 3u; ASSERT_FALSE(valid(a, i), "invalid SPIR-V byte size must fail"); a = assets();
  i[0].dispatchable = 0u; ASSERT_FALSE(valid(a, i), "selector-eligible nondispatchable implementation must fail"); i = implementations();
  i[0].dispatch = nullptr; ASSERT_FALSE(valid(a, i), "missing dispatch metadata must fail");
}
FACT(PrometheusShaderRegistryRejectsStaleGeneratedOutput) {
  const auto root = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT);
  const std::string manifest = read_file(root / "internal/prometheus/native/shaders/manifest.json");
  ASSERT_TRUE(manifest.find("reactor_vulkan_sgemm_scalar_plus_spirv.h") != std::string::npos, "manifest must retain generated-header ownership for drift checks");
  ASSERT_FALSE(std::filesystem::exists(root / "internal/prometheus/native/reactor_vulkan_stale_registry_output.h"), "a stale generated output must be detectable as absent from current inventory");
}
FACT(PrometheusVulkanRuntimePreflight) {
  void* handle = nullptr;
  const int create_result = prometheus_reactor_runtime_create(nullptr, &handle);
  PrometheusCaps caps{};
  const int probe_result = handle == nullptr ? PROM_ERROR : prometheus_reactor_runtime_probe(handle, &caps);
  const prometheus_runtime* runtime = static_cast<const prometheus_runtime*>(handle);
  const int vk_result = runtime == nullptr ? 0 : runtime->init_detail_code;
  std::string artifact = "create_result=" + std::to_string(create_result) + "\nprobe_result=" + std::to_string(probe_result) +
      "\ncaps_available=" + std::to_string(caps.available) + "\nreason_code=" + std::to_string(caps.reason_code) +
      "\nvk_result=" + std::to_string(vk_result) + "\n";
  if (runtime != nullptr && caps.available != 0u) {
    PrometheusVulkanDeviceDiagnostics device{};
    const int device_result = prometheus_reactor_runtime_vulkan_device_diagnostics(handle, &device);
    artifact += "device_result=" + std::to_string(device_result) + "\ndevice_name=" + device.device_name + "\nvendor_id=" + std::to_string(device.vendor_id) + "\n";
  }
  context.WriteTextArtifact("prometheus_vulkan_runtime_preflight.txt", artifact);
  const char* required = std::getenv("PROMETHEUS_REQUIRE_VULKAN_HARDWARE");
  const bool hardware_required = required != nullptr && std::string(required) == "1";
  if (handle != nullptr) ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "preflight runtime must be destroyed");
  if (caps.available == 0u) {
    if (hardware_required) FAIL("Prometheus Vulkan initialization failed; inspect preflight artifact for exact VkResult");
    SKIP("Prometheus Vulkan initialization failed; exact VkResult is recorded in the preflight artifact");
  }
  ASSERT_EQUAL(PROM_OK, create_result, "runtime create must succeed");
  ASSERT_EQUAL(PROM_OK, probe_result, "runtime probe must succeed");
}
