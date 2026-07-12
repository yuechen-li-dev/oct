#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_PROMETHEUS_AUDIT_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_PROMETHEUS_AUDIT_H

/*
 * Test-only SPIR-V audit vocabulary.  This header is intentionally absent from
 * reactor_vulkan_sgemm.c and reactor_shader_registry.c: production selection
 * has no route to these descriptors.
 */

#include <cstddef>
#include <cstdint>
#include <deque>
#include <string>
#include <vector>

#include "reactor_sgemm_dispatch_metadata.h"

enum class PrometheusAuditInputLayout : std::uint32_t {
    ScalarRowMajor = 1u,
    PackedFloat4 = 2u,
    PackedFp16U32 = 3u,
};

enum class PrometheusAuditPrecision : std::uint32_t {
    Fp32 = 1u,
    Fp16StorageFp32Accum = 2u,
};

struct PrometheusAuditShaderDescriptor {
    const char* name = nullptr;
    const std::uint32_t* spirv_words = nullptr;
    std::size_t spirv_size_bytes = 0u;
    const char* file_path = nullptr;
    const char* entry_point = nullptr;
    prom_sgemm_kernel_dispatch_metadata dispatch{};
    PrometheusAuditInputLayout input_layout = PrometheusAuditInputLayout::ScalarRowMajor;
    PrometheusAuditPrecision precision = PrometheusAuditPrecision::Fp32;
    std::uint32_t k_packing_factor = 1u;
    const char* provenance = nullptr;
    const char* comparison_group = nullptr;
};

struct PrometheusAuditValidation {
    bool valid = false;
    std::string error;
    std::uint64_t spirv_hash = 0u;
    std::uint32_t local_size_x = 0u;
    std::uint32_t local_size_y = 0u;
    std::uint32_t local_size_z = 0u;
};

struct PrometheusAuditDispatch {
    bool valid = false;
    std::string error;
    prom_sgemm_dispatch_geometry geometry{};
};

class PrometheusAuditShaderRegistry final {
public:
    bool RegisterEmbedded(const PrometheusAuditShaderDescriptor& descriptor, std::string* error);
    bool RegisterFile(const PrometheusAuditShaderDescriptor& descriptor, std::string* error);
    const PrometheusAuditShaderDescriptor* Find(const std::string& name) const;
    const std::vector<PrometheusAuditShaderDescriptor>& Enumerate() const;

private:
    std::vector<PrometheusAuditShaderDescriptor> entries_;
    std::deque<std::vector<std::uint32_t>> file_storage_;
};

PrometheusAuditValidation prometheus_audit_validate_shader(const PrometheusAuditShaderDescriptor& descriptor);
PrometheusAuditDispatch prometheus_audit_dispatch_for(const PrometheusAuditShaderDescriptor& descriptor,
                                                       std::uint32_t m,
                                                       std::uint32_t n);
std::string prometheus_audit_replay_identity(const PrometheusAuditShaderDescriptor& original,
                                             const PrometheusAuditShaderDescriptor& candidate,
                                             std::uint32_t m,
                                             std::uint32_t n,
                                             std::uint32_t k,
                                             std::uint64_t seed);
std::string prometheus_audit_json_summary(const PrometheusAuditShaderDescriptor& descriptor,
                                          const PrometheusAuditValidation& validation,
                                          const PrometheusAuditDispatch& dispatch,
                                          std::uint32_t m,
                                          std::uint32_t n,
                                          std::uint32_t k,
                                          std::uint64_t seed);

#endif
