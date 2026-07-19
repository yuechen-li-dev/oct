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
  ASSERT_EQUAL(static_cast<std::size_t>(30), prom_shader_registry_shader_asset_count(), "production assets must include the audit-only persistent summary shader");
  ASSERT_EQUAL(static_cast<std::size_t>(7), prom_shader_registry_reduction_shader_asset_count(), "M39b production reduction assets, including packed-short variants, must be present");
  ASSERT_EQUAL(static_cast<std::size_t>(0), prom_shader_registry_experimental_shader_asset_count(), "promoted reduction assets must not remain experimental");
}
FACT(PrometheusM39bReductionRegistryIsProductionOwnedAndIsolated) {
  const std::array<uint32_t, 7> expected_shader_ids = {
      PROM_REDUCTION_SHADER_ROW_SUM,
      PROM_REDUCTION_SHADER_ROW_MAX,
      PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM,
      PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE,
      PROM_REDUCTION_SHADER_SOFTMAX_FUSED,
      PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT,
      PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT,
  };
  const std::array<uint32_t, 7> expected_implementation_ids = {
      PROM_REDUCTION_IMPLEMENTATION_ROW_SUM,
      PROM_REDUCTION_IMPLEMENTATION_ROW_MAX,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED,
      PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT,
  };
  ASSERT_EQUAL(expected_shader_ids.size(), prom_shader_registry_reduction_compute_implementation_count(), "M39b must register every fixed production reduction implementation");
  ASSERT_EQUAL(static_cast<std::size_t>(0), prom_shader_registry_experimental_compute_implementation_count(), "promoted reduction implementations must not remain experimental");
  for (std::size_t i = 0; i < prom_shader_registry_reduction_shader_asset_count(); ++i) {
    const auto* asset = prom_shader_registry_reduction_shader_asset_at(i);
    const auto* implementation = prom_shader_registry_reduction_compute_implementation_at(i);
    ASSERT_TRUE(asset != nullptr && implementation != nullptr, "reduction registry rows must be complete");
    ASSERT_EQUAL(expected_shader_ids[i], asset->shader_id, "reduction shader IDs must remain stable, including packed-short variants");
    ASSERT_EQUAL(expected_implementation_ids[i], implementation->implementation_id, "reduction implementation IDs must remain stable, including packed-short variants");
    ASSERT_EQUAL(PROM_SHADER_AUTHORITY_PRODUCTION, asset->authority, "reduction source authority must be production");
    ASSERT_EQUAL(PROM_SHADER_AUTHORITY_PRODUCTION, implementation->authority, "reduction implementation authority must be production");
    ASSERT_EQUAL(PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION, implementation->pipeline_family, "reduction implementation must not enter the SGEMM pipeline family");
    ASSERT_EQUAL(0u, implementation->selector_eligible, "fixed reduction plans must not enter the SGEMM selector");
    ASSERT_EQUAL(1u, implementation->dispatchable, "promoted reduction implementation must remain dispatchable");
    ASSERT_TRUE(implementation->reduction_dispatch != nullptr, "reduction implementation needs typed dispatch metadata");
    ASSERT_TRUE(std::string(asset->source_path).find("/production/reduction/") != std::string::npos, "promoted reduction source must be confined to its production root");
  }
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
    ASSERT_EQUAL(PROM_SHADER_AUTHORITY_PRODUCTION, implementation->authority, "SGEMM implementation authority must remain explicit production");
    ASSERT_EQUAL(PROM_COMPUTE_PIPELINE_FAMILY_SGEMM, implementation->pipeline_family, "SGEMM implementations must remain in the SGEMM family");
    ASSERT_EQUAL(PROM_SHADER_AUTHORITY_PRODUCTION, prom_shader_registry_find_shader(implementation->shader_id)->authority, "SGEMM shader authority must remain explicit production");
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
  for (std::size_t i = 0; i < prom_shader_registry_reduction_shader_asset_count(); ++i) {
    const auto* asset = prom_shader_registry_reduction_shader_asset_at(i);
    ASSERT_TRUE(manifest.find(asset->name) != std::string::npos, "every production reduction asset must appear in manifest");
    ASSERT_TRUE(std::filesystem::exists(root / "internal/prometheus/native" / asset->generated_header_path), "generated reduction header must exist");
  }
  const auto* proof = prom_shader_registry_find_shader(15u);
  ASSERT_TRUE(proof != nullptr, "M28 proof asset must be registered");
  ASSERT_EQUAL(1u, proof->contains_inline_hlsl, "proof provenance must record inline HLSL");
  ASSERT_EQUAL(2u, proof->inline_hlsl_block_count, "proof block count must agree with source");
  ASSERT_TRUE(std::string(proof->foreign_targets) == "HLSL", "proof target provenance must be HLSL");
  const auto* resident = prom_shader_registry_find_shader(23u);
  ASSERT_TRUE(resident != nullptr, "M1a resident model-block proof must be registered");
  ASSERT_EQUAL(PROM_SHADER_SOURCE_SDSLV, resident->source_language, "resident proof source must be SDSL-V");
  ASSERT_EQUAL(3u, resident->descriptor_binding_count, "resident proof owns its exact binding contract");
  ASSERT_EQUAL(8u, resident->push_constant_bytes, "resident proof owns its exact push constants");
  ASSERT_TRUE(std::string(resident->source_path).find("/production/model_block/") != std::string::npos,
              "resident proof source must live in production ownership");
  const auto* audit = prom_shader_registry_find_shader(37u);
  ASSERT_TRUE(audit != nullptr, "the static persistent audit shader must be registered");
  ASSERT_EQUAL(4u, audit->descriptor_binding_count, "audit shader owns three closed source views and the arena binding");
  ASSERT_EQUAL(96u, audit->push_constant_bytes, "audit shader receives the bounded lock-derived projection keys");
  ASSERT_TRUE(std::string(audit->source_path).find("persistent_audit_summary.sdslv") != std::string::npos,
              "audit shader stays isolated from the model arithmetic sources");
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
  i[0].authority = PROM_SHADER_AUTHORITY_EXPERIMENTAL; ASSERT_FALSE(valid(a, i), "implementation and shader authority mismatch must fail"); i = implementations();
  i[0].pipeline_family = PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION; ASSERT_FALSE(valid(a, i), "SGEMM implementation in reduction family must fail"); i = implementations();
  i[0].dispatch = nullptr; ASSERT_FALSE(valid(a, i), "missing dispatch metadata must fail");
}
FACT(PrometheusShaderRegistryRejectsStaleGeneratedOutput) {
  const auto root = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT);
  const std::string manifest = read_file(root / "internal/prometheus/native/shaders/manifest.json");
  ASSERT_TRUE(manifest.find("reactor_vulkan_sgemm_scalar_plus_spirv.h") != std::string::npos, "manifest must retain generated-header ownership for drift checks");
  ASSERT_FALSE(std::filesystem::exists(root / "internal/prometheus/native/reactor_vulkan_stale_registry_output.h"), "a stale generated output must be detectable as absent from current inventory");
}
FACT(PrometheusM35bPromotedShadersUseProductionSdslvAssets) {
  const auto root = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT);
  const std::string manifest = read_file(root / "internal/prometheus/native/shaders/manifest.json");
  struct expected_asset { uint32_t id; const char* entry; const char* source; const char* header; };
  const std::array<expected_asset, 5> expected{{
      {10u, "SgemmSrt2AccumK_CS", "sgemm_srt_2accum_k.sdslv", "reactor_vulkan_sgemm_srt_2accum_k_spirv.h"},
      {11u, "SgemmB2x2_CS", "sgemm_b2x2_row_major_biased.sdslv", "reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h"},
      {12u, "SgemmA2x4_CS", "sgemm_a2x4_row_biased_accum8.sdslv", "reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h"},
      {13u, "SgemmPacked4_CS", "sgemm_packed4_fp32.sdslv", "reactor_vulkan_packed4_spirv.h"},
      {14u, "SgemmFp16StorageFp32Accum_CS", "sgemm_fp16_storage_fp32_accum.sdslv", "reactor_vulkan_fp16_spirv.h"},
  }};
  for (const auto& item : expected) {
    const auto* asset = prom_shader_registry_find_shader(item.id);
    ASSERT_TRUE(asset != nullptr, "promoted shader ID must remain registered");
    ASSERT_EQUAL(PROM_SHADER_SOURCE_SDSLV, asset->source_language, "promoted shader provenance must be SDSL-V");
    ASSERT_TRUE(std::string(asset->entry_point) == item.entry, "registry must use the generated entry point");
    ASSERT_TRUE(std::string(asset->source_path).find(item.source) != std::string::npos, "registry source must be production-owned");
    ASSERT_TRUE(std::string(asset->generated_header_path) == item.header, "registry must not reference an historical header");
    ASSERT_TRUE(manifest.find(item.header) != std::string::npos, "manifest must own the generated header");
  }
  ASSERT_TRUE(manifest.find("\"id\":13,\"name\":\"sgemm-packed4\",\"stage\":\"compute\",\"source_language\":\"sdslv\"") != std::string::npos, "Packed4 must be SDSL-V-owned");
  ASSERT_TRUE(manifest.find("\"id\":14,\"name\":\"sgemm-fp16-storage-fp32-accum\",\"stage\":\"compute\",\"source_language\":\"sdslv\"") != std::string::npos, "FP16 must be SDSL-V-owned");
}
FACT(PrometheusM34bA2x4UsesCanonicalDispatchFootprint) {
  const auto* implementation = prom_shader_registry_find_compute_implementation(5u);
  const auto* metadata = prom_shader_registry_dispatch_metadata(5u);
  ASSERT_TRUE(implementation != nullptr && metadata != nullptr, "A2x4 implementation must remain registered");
  ASSERT_TRUE(std::string(implementation->name) == "aggressive-4x4-accum8", "policy-visible A2x4 identity must remain unchanged");
  ASSERT_EQUAL(8u, metadata->threads_x, "A2x4 workgroup X must remain 8");
  ASSERT_EQUAL(8u, metadata->threads_y, "A2x4 workgroup Y must remain 8");
  ASSERT_EQUAL(1u, metadata->threads_z, "A2x4 workgroup Z must remain 1");
  ASSERT_EQUAL(2u, metadata->outputs_per_invocation_m, "A2x4 must produce two rows per invocation");
  ASSERT_EQUAL(4u, metadata->outputs_per_invocation_n, "A2x4 must produce four columns per invocation");
  const prom_sgemm_dispatch_geometry geometry = prom_sgemm_dispatch_geometry_for_metadata(3u, 17u, metadata);
  ASSERT_EQUAL(1u, geometry.groups_x, "M=3 must fit in one A2x4 X group");
  ASSERT_EQUAL(1u, geometry.groups_y, "N=17 must fit in one canonical A2x4 Y group");
  ASSERT_EQUAL(16u, geometry.logical_m_per_group, "A2x4 group coverage M must be canonical");
  ASSERT_EQUAL(32u, geometry.logical_n_per_group, "A2x4 group coverage N must be canonical");
}
FACT(PrometheusM34bValidationEnabledProductionVariants) {
#ifdef _WIN32
  ASSERT_EQUAL(0, _putenv_s("PROMETHEUS_VK_VALIDATION", "1"), "validation environment must be set");
#endif
  void* handle = nullptr;
  const int create_status = prom_reactor_runtime_create_impl(nullptr, &handle);
#ifdef _WIN32
  _putenv_s("PROMETHEUS_VK_VALIDATION", "");
#endif
  ASSERT_EQUAL(PROM_OK, create_status, "validation-enabled runtime creation must succeed");
  ASSERT_TRUE(handle != nullptr, "validation-enabled runtime must be returned");
  auto* runtime = static_cast<prometheus_runtime*>(handle);
  ASSERT_EQUAL(1u, runtime->validation_requested, "validation must be explicitly requested");
  ASSERT_EQUAL(1u, runtime->validation_available, "validation layer must be available");
  ASSERT_EQUAL(1u, runtime->validation_enabled, "validation layer must be enabled");
  ASSERT_EQUAL(1u, runtime->validation_debug_utils_active, "debug-utils capture must be active");
  constexpr uint32_t m = 3u, n = 17u, k = 7u;
  std::vector<float> a(m * k), b(k * n), expected(m * n, 0.0f), output(m * n, 0.0f);
  for (std::size_t i = 0; i < a.size(); ++i) a[i] = static_cast<float>((i % 7u) + 1u) / 8.0f;
  for (std::size_t i = 0; i < b.size(); ++i) b[i] = static_cast<float>((i % 11u) + 1u) / 12.0f;
  for (uint32_t row = 0; row < m; ++row) for (uint32_t col = 0; col < n; ++col) for (uint32_t kk = 0; kk < k; ++kk) expected[row * n + col] += a[row * k + kk] * b[kk * n + col];
  for (uint32_t variant : {3u, 4u, 5u}) {
    std::fill(output.begin(), output.end(), 0.0f);
    uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_sgemm_benchmark_variant_impl(handle, a.data(), b.data(), output.data(), m, n, k, variant, &stage, &detail), "production registry variant must execute");
    for (std::size_t i = 0; i < output.size(); ++i) ASSERT_NEAR(expected[i], output[i], 0.002f, "production variant must match CPU oracle");
  }
  ASSERT_EQUAL(0u, runtime->validation_warning_count, "validation warnings must be zero");
  ASSERT_EQUAL(0u, runtime->validation_error_count, "validation errors must be zero");
  ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_destroy_impl(handle), "validation runtime must be destroyed");
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
