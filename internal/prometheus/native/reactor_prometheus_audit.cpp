#include "reactor_prometheus_audit.h"

#include <cstring>
#include <fstream>
#include <iomanip>
#include <limits>
#include <sstream>

namespace
{
constexpr std::uint32_t kSpirvMagic = 0x07230203u;
constexpr std::uint16_t kOpEntryPoint = 15u;
constexpr std::uint16_t kOpExecutionMode = 16u;
constexpr std::uint32_t kExecutionModelGlCompute = 5u;
constexpr std::uint32_t kExecutionModeLocalSize = 17u;

[[nodiscard]] std::uint64_t HashWords(const std::uint32_t* words, std::size_t sizeBytes)
{
    constexpr std::uint64_t kOffset = 14695981039346656037ull;
    constexpr std::uint64_t kPrime = 1099511628211ull;
    std::uint64_t hash = kOffset;
    const auto* bytes = reinterpret_cast<const unsigned char*>(words);
    for (std::size_t index = 0; index < sizeBytes; ++index) {
        hash ^= bytes[index];
        hash *= kPrime;
    }
    return hash;
}

[[nodiscard]] std::string SpirvString(const std::uint32_t* words, std::size_t wordCount, std::size_t firstWord)
{
    const char* data = reinterpret_cast<const char*>(words + firstWord);
    const std::size_t bytes = (wordCount - firstWord) * sizeof(std::uint32_t);
    std::size_t length = 0u;
    while (length < bytes && data[length] != '\0') {
        ++length;
    }
    return std::string(data, length);
}

[[nodiscard]] bool DescriptorShapeIsValid(const PrometheusAuditShaderDescriptor& descriptor, std::string* error)
{
    if (descriptor.name == nullptr || descriptor.name[0] == '\0' || descriptor.entry_point == nullptr ||
        descriptor.entry_point[0] == '\0') {
        *error = "audit shader name and entry point are required";
        return false;
    }
    if (descriptor.dispatch.threads_x == 0u || descriptor.dispatch.threads_y == 0u || descriptor.dispatch.threads_z == 0u ||
        descriptor.dispatch.outputs_per_invocation_m == 0u || descriptor.dispatch.outputs_per_invocation_n == 0u ||
        descriptor.k_packing_factor == 0u) {
        *error = "workgroup, output footprint, and K packing factor must be nonzero";
        return false;
    }
    return true;
}

void AppendJsonString(std::ostringstream& output, const char* value)
{
    output << '"';
    if (value != nullptr) {
        for (const char* cursor = value; *cursor != '\0'; ++cursor) {
            if (*cursor == '"' || *cursor == '\\') {
                output << '\\';
            }
            output << *cursor;
        }
    }
    output << '"';
}
}

PrometheusAuditValidation prometheus_audit_validate_shader(const PrometheusAuditShaderDescriptor& descriptor)
{
    PrometheusAuditValidation result;
    std::string shapeError;
    if (!DescriptorShapeIsValid(descriptor, &shapeError)) {
        result.error = shapeError;
        return result;
    }
    if (descriptor.spirv_words == nullptr || descriptor.spirv_size_bytes == 0u) {
        result.error = "SPIR-V module is empty";
        return result;
    }
    if ((descriptor.spirv_size_bytes % sizeof(std::uint32_t)) != 0u) {
        result.error = "SPIR-V byte length is not a multiple of four";
        return result;
    }
    const std::size_t wordCount = descriptor.spirv_size_bytes / sizeof(std::uint32_t);
    if (wordCount < 5u || descriptor.spirv_words[0] != kSpirvMagic) {
        result.error = "SPIR-V magic is invalid";
        return result;
    }

    bool computeEntryFound = false;
    std::uint32_t entryId = 0u;
    bool localSizeFound = false;
    for (std::size_t offset = 5u; offset < wordCount;) {
        const std::uint32_t instruction = descriptor.spirv_words[offset];
        const std::uint16_t length = static_cast<std::uint16_t>(instruction >> 16u);
        const std::uint16_t opcode = static_cast<std::uint16_t>(instruction & 0xffffu);
        if (length == 0u || offset + length > wordCount) {
            result.error = "SPIR-V instruction extends past module";
            return result;
        }
        if (opcode == kOpEntryPoint && length >= 4u && descriptor.spirv_words[offset + 1u] == kExecutionModelGlCompute &&
            SpirvString(descriptor.spirv_words, wordCount, offset + 3u) == descriptor.entry_point) {
            computeEntryFound = true;
            entryId = descriptor.spirv_words[offset + 2u];
        }
        if (opcode == kOpExecutionMode && length == 6u && descriptor.spirv_words[offset + 2u] == kExecutionModeLocalSize) {
            if (computeEntryFound && descriptor.spirv_words[offset + 1u] == entryId) {
                result.local_size_x = descriptor.spirv_words[offset + 3u];
                result.local_size_y = descriptor.spirv_words[offset + 4u];
                result.local_size_z = descriptor.spirv_words[offset + 5u];
                localSizeFound = true;
            }
        }
        offset += length;
    }
    if (!computeEntryFound) {
        result.error = "requested compute entry point is absent";
        return result;
    }
    if (!localSizeFound) {
        result.error = "requested compute entry point has no LocalSize execution mode";
        return result;
    }
    if (result.local_size_x != descriptor.dispatch.threads_x || result.local_size_y != descriptor.dispatch.threads_y ||
        result.local_size_z != descriptor.dispatch.threads_z) {
        result.error = "descriptor workgroup size does not match SPIR-V LocalSize";
        return result;
    }
    result.valid = true;
    result.spirv_hash = HashWords(descriptor.spirv_words, descriptor.spirv_size_bytes);
    return result;
}

PrometheusAuditDispatch prometheus_audit_dispatch_for(const PrometheusAuditShaderDescriptor& descriptor,
                                                       std::uint32_t m,
                                                       std::uint32_t n)
{
    PrometheusAuditDispatch result;
    const std::uint64_t rows = descriptor.dispatch.workgroup_output_m != 0u
                                   ? descriptor.dispatch.workgroup_output_m
                                   : static_cast<std::uint64_t>(descriptor.dispatch.threads_x) * descriptor.dispatch.outputs_per_invocation_m;
    const std::uint64_t columns = descriptor.dispatch.workgroup_output_n != 0u
                                      ? descriptor.dispatch.workgroup_output_n
                                      : static_cast<std::uint64_t>(descriptor.dispatch.threads_y) * descriptor.dispatch.outputs_per_invocation_n;
    if (rows == 0u || columns == 0u || rows > std::numeric_limits<std::uint32_t>::max() ||
        columns > std::numeric_limits<std::uint32_t>::max()) {
        result.error = "invalid output footprint";
        return result;
    }
    result.geometry = prom_sgemm_dispatch_geometry_for_metadata(m, n, &descriptor.dispatch);
    if (result.geometry.groups_x == 0u || result.geometry.groups_y == 0u ||
        static_cast<std::uint64_t>(result.geometry.groups_x) * rows < m ||
        static_cast<std::uint64_t>(result.geometry.groups_y) * columns < n) {
        result.error = "dispatch does not cover output domain";
        return result;
    }
    result.valid = true;
    return result;
}

bool PrometheusAuditShaderRegistry::RegisterEmbedded(const PrometheusAuditShaderDescriptor& descriptor, std::string* error)
{
    if (error == nullptr) {
        return false;
    }
    if (descriptor.file_path != nullptr || Find(descriptor.name == nullptr ? "" : descriptor.name) != nullptr) {
        *error = "embedded audit descriptor has a file path or duplicates a name";
        return false;
    }
    const PrometheusAuditValidation validation = prometheus_audit_validate_shader(descriptor);
    if (!validation.valid) {
        *error = validation.error;
        return false;
    }
    entries_.push_back(descriptor);
    return true;
}

bool PrometheusAuditShaderRegistry::RegisterFile(const PrometheusAuditShaderDescriptor& descriptor, std::string* error)
{
    if (error == nullptr) {
        return false;
    }
    if (descriptor.file_path == nullptr || descriptor.file_path[0] == '\0' || descriptor.spirv_words != nullptr ||
        Find(descriptor.name == nullptr ? "" : descriptor.name) != nullptr) {
        *error = "file audit descriptor is malformed or duplicates a name";
        return false;
    }
    std::ifstream input(descriptor.file_path, std::ios::binary | std::ios::ate);
    if (!input) {
        *error = "candidate SPIR-V file does not exist";
        return false;
    }
    const std::streamsize size = input.tellg();
    if (size <= 0 || (size % static_cast<std::streamsize>(sizeof(std::uint32_t))) != 0) {
        *error = "candidate SPIR-V file is empty or has invalid byte length";
        return false;
    }
    input.seekg(0, std::ios::beg);
    file_storage_.emplace_back(static_cast<std::size_t>(size) / sizeof(std::uint32_t));
    if (!input.read(reinterpret_cast<char*>(file_storage_.back().data()), size)) {
        file_storage_.pop_back();
        *error = "candidate SPIR-V file could not be read";
        return false;
    }
    PrometheusAuditShaderDescriptor loaded = descriptor;
    loaded.spirv_words = file_storage_.back().data();
    loaded.spirv_size_bytes = static_cast<std::size_t>(size);
    const PrometheusAuditValidation validation = prometheus_audit_validate_shader(loaded);
    if (!validation.valid) {
        file_storage_.pop_back();
        *error = validation.error;
        return false;
    }
    entries_.push_back(loaded);
    return true;
}

const PrometheusAuditShaderDescriptor* PrometheusAuditShaderRegistry::Find(const std::string& name) const
{
    for (const auto& entry : entries_) {
        if (entry.name != nullptr && name == entry.name) {
            return &entry;
        }
    }
    return nullptr;
}

const std::vector<PrometheusAuditShaderDescriptor>& PrometheusAuditShaderRegistry::Enumerate() const
{
    return entries_;
}

std::string prometheus_audit_replay_identity(const PrometheusAuditShaderDescriptor& original,
                                             const PrometheusAuditShaderDescriptor& candidate,
                                             std::uint32_t m,
                                             std::uint32_t n,
                                             std::uint32_t k,
                                             std::uint64_t seed)
{
    const PrometheusAuditValidation originalValidation = prometheus_audit_validate_shader(original);
    const PrometheusAuditValidation candidateValidation = prometheus_audit_validate_shader(candidate);
    std::ostringstream output;
    output << (original.name == nullptr ? "unknown" : original.name) << ':' << std::hex << originalValidation.spirv_hash << ':'
           << (candidate.name == nullptr ? "unknown" : candidate.name) << ':' << candidateValidation.spirv_hash << std::dec << ':'
           << m << 'x' << n << 'x' << k << ':' << seed << ':'
           << (original.entry_point == nullptr ? "" : original.entry_point) << ':'
           << (candidate.entry_point == nullptr ? "" : candidate.entry_point) << ':'
           << candidate.dispatch.outputs_per_invocation_m << 'x' << candidate.dispatch.outputs_per_invocation_n;
    return output.str();
}

std::string prometheus_audit_json_summary(const PrometheusAuditShaderDescriptor& descriptor,
                                          const PrometheusAuditValidation& validation,
                                          const PrometheusAuditDispatch& dispatch,
                                          std::uint32_t m,
                                          std::uint32_t n,
                                          std::uint32_t k,
                                          std::uint64_t seed)
{
    std::ostringstream output;
    output << "{\"shader\":";
    AppendJsonString(output, descriptor.name);
    output << ",\"provenance\":";
    AppendJsonString(output, descriptor.provenance);
    output << ",\"entry_point\":";
    AppendJsonString(output, descriptor.entry_point);
    output << ",\"spirv_hash\":\"" << std::hex << validation.spirv_hash << std::dec << "\""
           << ",\"workgroup\":[" << descriptor.dispatch.threads_x << ',' << descriptor.dispatch.threads_y << ',' << descriptor.dispatch.threads_z << ']'
           << ",\"output_footprint\":[" << descriptor.dispatch.outputs_per_invocation_m << ',' << descriptor.dispatch.outputs_per_invocation_n << ']'
           << ",\"workload\":[" << m << ',' << n << ',' << k << ']'
           << ",\"dispatch\":[" << dispatch.geometry.groups_x << ',' << dispatch.geometry.groups_y << ',' << dispatch.geometry.groups_z << ']'
           << ",\"seed\":" << seed << ",\"validation\":" << (validation.valid ? "true" : "false") << '}';
    return output.str();
}
