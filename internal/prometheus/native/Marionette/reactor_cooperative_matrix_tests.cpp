#include "test_harness.h"

#include "../reactor_judgment_engine.h"
#include "../reactor_prometheus_audit.h"
#include "../reactor_vulkan.h"
#include "../reactor_vulkan_tiled_spirv.h"
#include "../reactor_vulkan_memory_conservative_spirv.h"
#include "../reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h"
#include "../reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h"
#include "../reactor_vulkan_packed4_spirv.h"
#include "../reactor_vulkan_fp16_spirv.h"
#include "../../shaders/sdslv/experimental/sgemm/cooperative/sgemm_cooperative_f16_f32_m16n16k16_spirv.h"

#include <algorithm>
#include <array>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstdlib>
#include <iomanip>
#include <limits>
#include <numeric>
#include <sstream>
#include <string>
#include <vector>

namespace
{
constexpr const char* kCooperativeEntry = "CooperativeSgemmF16F32M16N16K16_CS";

struct TimingStats {
    std::uint64_t minimum = 0u;
    std::uint64_t median = 0u;
    std::uint64_t maximum = 0u;
};

TimingStats Stats(std::vector<std::uint64_t> values)
{
    if (values.empty()) return {};
    std::sort(values.begin(), values.end());
    return {values.front(), values[values.size() / 2u], values.back()};
}

std::uint64_t HashWords(const std::uint32_t* words, std::size_t bytes)
{
    std::uint64_t hash = 1469598103934665603ull;
    const auto* data = reinterpret_cast<const unsigned char*>(words);
    for (std::size_t i = 0u; i < bytes; ++i) {
        hash ^= data[i];
        hash *= 1099511628211ull;
    }
    return hash;
}

bool HasOpcode(const std::uint32_t* words, std::size_t bytes, std::uint16_t opcode)
{
    const std::size_t count = bytes / sizeof(std::uint32_t);
    for (std::size_t i = 5u; i < count;) {
        const std::uint32_t wordCount = words[i] >> 16u;
        if (wordCount == 0u || i + wordCount > count) return false;
        if ((words[i] & 0xffffu) == opcode) return true;
        i += wordCount;
    }
    return false;
}

prom_sgemm_audit_execution_descriptor CooperativeDescriptor()
{
    prom_sgemm_audit_execution_descriptor descriptor{};
    descriptor.spirv_words = k_prom_m40a_cooperative_sgemm_spirv;
    descriptor.spirv_size_bytes = sizeof(k_prom_m40a_cooperative_sgemm_spirv);
    descriptor.entry_point = kCooperativeEntry;
    descriptor.dispatch = {32u, 1u, 1u, 1u, 1u, 16u, 16u, 16u, 1u, 16u, 16u};
    descriptor.compute_mode = PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM;
    descriptor.provenance = "M40a SDSL-V experimental cooperative proof";
    descriptor.spirv_hash = HashWords(descriptor.spirv_words, descriptor.spirv_size_bytes);
    descriptor.require_full_subgroups = 1u;
    return descriptor;
}

void FillIdentityWorkload(std::vector<float>* a, std::vector<float>* b, std::uint32_t size)
{
    a->resize(static_cast<std::size_t>(size) * size);
    b->assign(static_cast<std::size_t>(size) * size, 0.0f);
    for (std::uint32_t row = 0u; row < size; ++row) {
        for (std::uint32_t column = 0u; column < size; ++column) {
            const int value = static_cast<int>((row * 3u + column * 5u) % 17u) - 8;
            (*a)[static_cast<std::size_t>(row) * size + column] = static_cast<float>(value) / 16.0f;
        }
        (*b)[static_cast<std::size_t>(row) * size + row] = 1.0f;
    }
}

bool Matches(const std::vector<float>& expected, const std::vector<float>& actual, float tolerance = 0.002f)
{
    if (expected.size() != actual.size()) return false;
    for (std::size_t i = 0u; i < expected.size(); ++i) {
        if (!std::isfinite(actual[i]) || std::fabs(expected[i] - actual[i]) > tolerance) return false;
    }
    return true;
}

bool Run(void* runtime, const prom_sgemm_audit_execution_descriptor& descriptor,
         const std::vector<float>& a, const std::vector<float>& b, std::vector<float>* c,
         std::uint32_t size, std::uint32_t warmup, std::uint32_t iterations,
         std::vector<std::uint64_t>* samples)
{
    c->assign(static_cast<std::size_t>(size) * size, 0.0f);
    samples->assign(iterations, 0u);
    prom_sgemm_audit_execution_result result{};
    const bool executed = prom_reactor_runtime_sgemm_audit_benchmark_impl(
               runtime, a.data(), b.data(), c->data(), size, size, size, &descriptor,
               warmup, iterations, samples->data(), iterations, &result) == PROM_OK &&
           result.gpu_timing_valid != 0u && result.pipeline_create_count == 1u;
    if (!executed) return false;
    if (descriptor.dispatch.workgroup_output_m != 0u) {
        return result.dispatch_geometry.groups_x == size / descriptor.dispatch.workgroup_output_m &&
               result.dispatch_geometry.groups_y == size / descriptor.dispatch.workgroup_output_n;
    }
    return true;
}

class EnvironmentValue final {
public:
    EnvironmentValue(const char* name, const char* value) : name_(name)
    {
        const char* previous = std::getenv(name);
        if (previous != nullptr) { hadPrevious_ = true; previous_ = previous; }
        Set(value);
    }
    ~EnvironmentValue() { Set(hadPrevious_ ? previous_.c_str() : nullptr); }
private:
    void Set(const char* value)
    {
#ifdef _WIN32
        _putenv_s(name_.c_str(), value == nullptr ? "" : value);
#else
        if (value == nullptr) unsetenv(name_.c_str()); else setenv(name_.c_str(), value, 1);
#endif
    }
    std::string name_;
    std::string previous_;
    bool hadPrevious_ = false;
};

struct Kernel {
    const char* name;
    const std::uint32_t* words;
    std::size_t bytes;
    const char* entry;
    prom_sgemm_kernel_dispatch_metadata dispatch;
    std::uint32_t mode;
    const char* precision;
};

prom_sgemm_audit_execution_descriptor Descriptor(const Kernel& kernel)
{
    prom_sgemm_audit_execution_descriptor descriptor{};
    descriptor.spirv_words = kernel.words;
    descriptor.spirv_size_bytes = kernel.bytes;
    descriptor.entry_point = kernel.entry;
    descriptor.dispatch = kernel.dispatch;
    descriptor.compute_mode = kernel.mode;
    descriptor.provenance = "M40a production comparison";
    descriptor.spirv_hash = HashWords(kernel.words, kernel.bytes);
    return descriptor;
}

std::string TimingJson(const TimingStats& stats)
{
    std::ostringstream out;
    out << "{\"min_ns\":" << stats.minimum << ",\"median_ns\":" << stats.median
        << ",\"max_ns\":" << stats.maximum << '}';
    return out.str();
}

std::string CommandTraceJson(const prom_m40b_command_trace& trace)
{
    std::ostringstream out;
    out << "{\"entry_count\":" << trace.entry_count
        << ",\"submit_count\":" << trace.submit_count
        << ",\"intermediate_buffer_count\":" << trace.intermediate_buffer_count
        << ",\"intermediate_host_copy_count\":" << trace.intermediate_host_copy_count
        << ",\"final_readback_copy_count\":" << trace.final_readback_copy_count
        << ",\"replay_id\":" << trace.replay_id << ",\"entries\":[";
    for (std::uint32_t index = 0u; index < trace.entry_count; ++index) {
        const prom_m40b_command_trace_entry& entry = trace.entries[index];
        if (index != 0u) out << ',';
        out << "{\"operation\":" << entry.operation
            << ",\"submit_index\":" << entry.submit_index
            << ",\"reduction_stage_index\":" << entry.reduction_stage_index
            << ",\"source_stage_mask\":" << entry.source_stage_mask
            << ",\"destination_stage_mask\":" << entry.destination_stage_mask
            << ",\"source_access_mask\":" << entry.source_access_mask
            << ",\"destination_access_mask\":" << entry.destination_access_mask
            << ",\"source_queue_family\":" << entry.source_queue_family
            << ",\"destination_queue_family\":" << entry.destination_queue_family << '}';
    }
    out << "]}";
    return out.str();
}

std::string ReplayIdentity(const char* kernel, std::uint32_t size, std::uint64_t shaderHash,
                           const char* mode = "kernel-only")
{
    std::ostringstream out;
    out << "m40a-identity-workload-v1:" << kernel << ':' << size << 'x' << size << 'x' << size
        << ':' << mode << ':' << std::hex << shaderHash;
    return out.str();
}

void FillComposedIdentityWorkload(std::vector<float>* a, std::vector<float>* b,
                                  std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    a->resize(static_cast<std::size_t>(m) * k);
    b->assign(static_cast<std::size_t>(k) * n, 0.0f);
    for (std::uint32_t row = 0u; row < m; ++row) {
        for (std::uint32_t column = 0u; column < k; ++column) {
            const int value = static_cast<int>((row * 11u + column * 7u) % 29u) - 14;
            (*a)[static_cast<std::size_t>(row) * k + column] = static_cast<float>(value) / 32.0f;
        }
    }
    for (std::uint32_t diagonal = 0u; diagonal < std::min(n, k); ++diagonal) {
        (*b)[static_cast<std::size_t>(diagonal) * n + diagonal] = 1.0f;
    }
}

std::vector<float> ComposedSoftmaxOracle(const std::vector<float>& a,
                                         std::uint32_t m, std::uint32_t n, std::uint32_t k,
                                         bool rounded)
{
    std::vector<float> output(static_cast<std::size_t>(m) * n);
    for (std::uint32_t row = 0u; row < m; ++row) {
        float maximum = -std::numeric_limits<float>::infinity();
        for (std::uint32_t column = 0u; column < n; ++column) {
            float value = column < k ? a[static_cast<std::size_t>(row) * k + column] : 0.0f;
            if (rounded) value = prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(value));
            maximum = std::max(maximum, value);
        }
        double denominator = 0.0;
        for (std::uint32_t column = 0u; column < n; ++column) {
            float value = column < k ? a[static_cast<std::size_t>(row) * k + column] : 0.0f;
            if (rounded) value = prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(value));
            denominator += std::exp(static_cast<double>(value - maximum));
        }
        for (std::uint32_t column = 0u; column < n; ++column) {
            float value = column < k ? a[static_cast<std::size_t>(row) * k + column] : 0.0f;
            if (rounded) value = prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(value));
            output[static_cast<std::size_t>(row) * n + column] =
                static_cast<float>(std::exp(static_cast<double>(value - maximum)) / denominator);
        }
    }
    return output;
}

std::string SoftmaxMismatch(const std::vector<float>& expected, const std::vector<float>& actual,
                            std::uint32_t m, std::uint32_t n,
                            const prom_m40b_padding_plan& padding,
                            std::uint64_t shaderHash, std::uint64_t reductionReplayID)
{
    if (expected.size() != actual.size()) return "softmax result size mismatch";
    for (std::uint32_t row = 0u; row < m; ++row) {
        double rowSum = 0.0;
        for (std::uint32_t column = 0u; column < n; ++column) {
            const std::size_t index = static_cast<std::size_t>(row) * n + column;
            const float absolute = std::fabs(expected[index] - actual[index]);
            const float relative = absolute / std::max(std::fabs(expected[index]), 1.0e-20f);
            rowSum += actual[index];
            if (!std::isfinite(actual[index]) || actual[index] < -2.0e-7f ||
                (absolute > 2.0e-5f && relative > 2.0e-4f)) {
                std::ostringstream message;
                message << "softmax mismatch row=" << row << " column=" << column
                        << " expected=" << expected[index] << " actual=" << actual[index]
                        << " absolute_error=" << absolute << " relative_error=" << relative
                        << " logical=" << m << 'x' << n
                        << " padded=" << padding.padded_m << 'x' << padding.padded_n << 'x' << padding.padded_k
                        << " cooperative_shader_hash64=0x" << std::hex << shaderHash
                        << " reduction_replay_id=0x" << reductionReplayID;
                return message.str();
            }
        }
        if (!std::isfinite(rowSum) || std::fabs(rowSum - 1.0) > 2.0e-4) {
            std::ostringstream message;
            message << "softmax row-sum mismatch row=" << row << " expected=1 actual=" << rowSum
                    << " logical=" << m << 'x' << n
                    << " padded=" << padding.padded_m << 'x' << padding.padded_n << 'x' << padding.padded_k
                    << " cooperative_shader_hash64=0x" << std::hex << shaderHash
                    << " reduction_replay_id=0x" << reductionReplayID;
            return message.str();
        }
    }
    return {};
}
} // namespace

FACT(PrometheusM40bBoundedContracts)
{
    prom_m40b_padding_plan aligned{};
    prom_m40b_padding_plan awkward{};
    ASSERT_EQUAL(PROM_OK, prom_m40b_calculate_padding_plan(128u, 1024u, 1024u, &aligned), "aligned padding plan is valid");
    ASSERT_EQUAL(128u, aligned.padded_m, "aligned M is unchanged");
    ASSERT_EQUAL(1024u, aligned.padded_n, "aligned N is unchanged");
    ASSERT_EQUAL(PROM_OK, prom_m40b_calculate_padding_plan(127u, 1001u, 1023u, &awkward), "awkward padding plan is valid");
    ASSERT_EQUAL(128u, awkward.padded_m, "awkward M pads to 16");
    ASSERT_EQUAL(1008u, awkward.padded_n, "awkward N pads to 16");
    ASSERT_EQUAL(1024u, awkward.padded_k, "awkward K pads to 16");
    ASSERT_TRUE(awkward.replay_id != aligned.replay_id, "padding identity includes logical and padded shape");
    ASSERT_TRUE(prom_m40b_calculate_padding_plan(0u, 1u, 1u, &awkward) != PROM_OK, "zero logical regions reject");
    ASSERT_TRUE(prom_m40b_calculate_padding_plan(UINT32_MAX, 1u, 1u, &awkward) != PROM_OK, "overflowing round-up rejects");

    prom_device_buffer_view view{};
    view.buffer = reinterpret_cast<VkBuffer>(static_cast<std::uintptr_t>(1u));
    view.byte_length = aligned.intermediate_c_bytes;
    view.element_type = PROM_DEVICE_ELEMENT_F32;
    view.logical_rows = 128u; view.logical_columns = 1024u; view.row_stride_elements = 1024u;
    view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    view.owning_device = reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(2u));
    view.owning_lifetime_id = 7u; view.owning_slot_generation = 3u;
    int32_t detail = 0;
    ASSERT_EQUAL(PROM_OK, prom_m40b_validate_device_buffer_view(&view, view.owning_device,
                 PROM_DEVICE_ELEMENT_F32, 128u, 1024u, PROM_DEVICE_ACCESS_COMPUTE_READ, &detail),
                 "device C view validates without a host pointer");
    ASSERT_TRUE(prom_m40b_validate_device_buffer_view(&view,
                reinterpret_cast<VkDevice>(static_cast<std::uintptr_t>(3u)), PROM_DEVICE_ELEMENT_F32,
                128u, 1024u, PROM_DEVICE_ACCESS_COMPUTE_READ, &detail) != PROM_OK,
                "cross-device handoff rejects");
    ASSERT_EQUAL(PROM_M40B_DETAIL_CROSS_DEVICE, detail, "cross-device rejection is explicit");

    prom_m40b_command_trace one{};
    prom_m40b_command_trace two{};
    prom_m40b_plan_command_trace(PROM_M40B_INPUT_HOST_A_PERSISTENT_B,
                                 PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER, 1u, &one);
    prom_m40b_plan_command_trace(PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B,
                                 PROM_M40B_SUBMIT_TWO_BOUNDED, 5u, &two);
    ASSERT_EQUAL(1u, one.submit_count, "one-command plan has one submit");
    ASSERT_EQUAL(2u, two.submit_count, "bounded split plan has two submits");
    ASSERT_EQUAL(1u, one.intermediate_buffer_count, "exactly one logical C intermediate exists");
    ASSERT_EQUAL(0u, one.intermediate_host_copy_count, "no host copy occurs between reactors");
    ASSERT_EQUAL(1u, one.final_readback_copy_count, "only final softmax readback is planned");
    const auto barrier = std::find_if(one.entries, one.entries + one.entry_count, [](const auto& entry) {
        return entry.operation == PROM_M40B_TRACE_COMPUTE_WRITE_TO_READ_BARRIER;
    });
    ASSERT_TRUE(barrier != one.entries + one.entry_count, "handoff barrier is explicit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT), barrier->source_stage_mask,
                 "handoff source stage is compute");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_WRITE_BIT), barrier->source_access_mask,
                 "handoff source access is shader write");
    ASSERT_EQUAL(static_cast<std::uint32_t>(VK_ACCESS_SHADER_READ_BIT), barrier->destination_access_mask,
                 "handoff destination access is shader read");
    ASSERT_EQUAL(UINT32_MAX, barrier->source_queue_family, "no queue ownership transfer is planned");
    ASSERT_TRUE(one.replay_id != two.replay_id, "submit and stage topology participate in replay identity");
}

FACT(PrometheusM40bExperimentalSelectorPredicate)
{
    prom_m40b_selector_facts facts{};
    facts.capability_state = PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE;
    facts.tuple_m = facts.tuple_n = facts.tuple_k = 16u;
    facts.shader_float16 = facts.vulkan_memory_model = 1u;
    facts.precision_allows_f16_rounded = 1u;
    facts.m = 128u; facts.n = 1024u; facts.k = 1024u;
    facts.padding_supported = facts.persistent_b_available = facts.device_resident_composition = 1u;
    prom_m40b_selector_decision decision{};
    prom_m40b_selector_evaluate(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M40B_SELECTOR_DISABLED), decision.reason,
                 "experimental candidate remains disabled by default");
    facts.experimental_enabled = 1u;
    prom_m40b_selector_evaluate(&facts, &decision);
    ASSERT_EQUAL(1u, decision.eligible, "proven bounded facts are eligible experimentally");
    ASSERT_EQUAL(0u, decision.selected, "eligibility does not change production selection authority");
    facts.precision_allows_f16_rounded = 0u;
    prom_m40b_selector_evaluate(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M40B_SELECTOR_PRECISION), decision.reason,
                 "precision policy rejection is explicit");
    facts.precision_allows_f16_rounded = 1u; facts.capability_state = PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE;
    prom_m40b_selector_evaluate(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M40B_SELECTOR_CAPABILITY), decision.reason,
                 "extension absence falls back explicitly");
}

FACT(PrometheusM40aCooperativeMatrixStaticContract)
{
    PrometheusAuditShaderDescriptor descriptor{};
    descriptor.name = "m40a-cooperative";
    descriptor.spirv_words = k_prom_m40a_cooperative_sgemm_spirv;
    descriptor.spirv_size_bytes = sizeof(k_prom_m40a_cooperative_sgemm_spirv);
    descriptor.entry_point = kCooperativeEntry;
    descriptor.dispatch = {32u, 1u, 1u, 1u, 1u, 16u, 16u, 16u, 1u, 16u, 16u};
    descriptor.input_layout = PrometheusAuditInputLayout::PackedFp16U32;
    descriptor.precision = PrometheusAuditPrecision::Fp16StorageFp32Accum;
    descriptor.k_packing_factor = 2u;
    descriptor.require_full_subgroups = true;
    const PrometheusAuditValidation validation = prometheus_audit_validate_shader(descriptor);
    const PrometheusAuditDispatch dispatch = prometheus_audit_dispatch_for(descriptor, 512u, 512u);
    ASSERT_TRUE(validation.valid, "cooperative artifact has the declared entry and LocalSize");
    ASSERT_TRUE(dispatch.valid, "cooperative whole-workgroup footprint is valid");
    ASSERT_EQUAL(32u, dispatch.geometry.groups_x, "512 rows use 32 cooperative tiles");
    ASSERT_EQUAL(32u, dispatch.geometry.groups_y, "512 columns use 32 cooperative tiles");
    ASSERT_TRUE(HasOpcode(descriptor.spirv_words, descriptor.spirv_size_bytes, 4457u), "SPIR-V contains OpCooperativeMatrixLoadKHR");
    ASSERT_TRUE(HasOpcode(descriptor.spirv_words, descriptor.spirv_size_bytes, 4458u), "SPIR-V contains OpCooperativeMatrixStoreKHR");
    ASSERT_TRUE(HasOpcode(descriptor.spirv_words, descriptor.spirv_size_bytes, 4459u), "SPIR-V contains OpCooperativeMatrixMulAddKHR");
}

FACT(PrometheusM40aExtensionAbsentFallback)
{
    EnvironmentValue disabled("PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX", "1");
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "ordinary runtime starts with cooperative extension disabled");
    if (runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(runtime, &services), "runtime services remain available");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE), services.cooperative_matrix_state,
                 "simulated extension absence stays optional");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM40aCooperativeMatrixHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creation succeeds");
    if (runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(runtime, &services);
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("useful KHR cooperative tuple unavailable");
    }
    ASSERT_EQUAL(32u, services.subgroup_size, "selected proof contract uses one full 32-lane subgroup");
    ASSERT_TRUE(services.validation_enabled != 0u, "validation is enabled for hardware proof");
    const auto descriptor = CooperativeDescriptor();
    for (const std::uint32_t size : {16u, 256u, 512u, 1024u}) {
        std::vector<float> a, b, c;
        std::vector<std::uint64_t> samples;
        FillIdentityWorkload(&a, &b, size);
        ASSERT_TRUE(Run(runtime, descriptor, a, b, &c, size, 1u, 3u, &samples), "cooperative aligned dispatch succeeds and timestamps");
        ASSERT_TRUE(Matches(a, c, 0.002f), "cooperative output matches exact identity-matrix CPU oracle");
    }
    {
        const std::uint32_t awkward = 257u;
        std::vector<float> a, b, c(static_cast<std::size_t>(awkward) * awkward);
        FillIdentityWorkload(&a, &b, awkward);
        prom_sgemm_audit_execution_result result{};
        ASSERT_TRUE(prom_reactor_runtime_sgemm_audit_impl(runtime, a.data(), b.data(), c.data(), awkward, awkward, awkward,
                                                          &descriptor, &result) != PROM_OK,
                    "M/N/K tails are explicitly rejected");
        ASSERT_EQUAL(0u, result.pipeline_create_count, "tail rejection happens before pipeline creation");
    }
    prom_reactor_runtime_get_vk_services(runtime, &services);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE), services.cooperative_matrix_state,
                 "successful dispatch promotes the audit state to executable");
    ASSERT_EQUAL(0u, services.validation_error_count, "hardware proof is validation clean");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM40bDeviceResidentComposedHardwareProof)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(nullptr, &runtime), "Vulkan runtime creation succeeds");
    if (runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(runtime, &services);
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("useful KHR cooperative tuple unavailable");
    }
    std::uint64_t generation = 1u;
    for (const auto shape : {std::array<std::uint32_t, 3>{128u, 1024u, 1024u},
                             std::array<std::uint32_t, 3>{127u, 1001u, 1023u}}) {
        const std::uint32_t m = shape[0];
        const std::uint32_t n = shape[1];
        const std::uint32_t k = shape[2];
        std::vector<float> a, b;
        FillComposedIdentityWorkload(&a, &b, m, n, k);
        const std::vector<float> expected = ComposedSoftmaxOracle(a, m, n, k, true);
        std::vector<float> output(static_cast<std::size_t>(m) * n);
        prom_m40b_prepare_request prepareB{};
        prepareB.values = b.data(); prepareB.m = m; prepareB.n = n; prepareB.k = k;
        prepareB.kernel = PROM_M40B_KERNEL_COOPERATIVE; prepareB.generation = generation;
        prom_m40b_prepare_result preparedB{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_persistent_b(runtime, &prepareB, &preparedB),
                     "persistent packed B converts and uploads once");
        ASSERT_EQUAL(generation, preparedB.generation, "persistent B generation is visible");

        prom_m40b_execution_request hostRequest{};
        hostRequest.host_a = a.data(); hostRequest.output = output.data();
        hostRequest.m = m; hostRequest.n = n; hostRequest.k = k;
        hostRequest.kernel = PROM_M40B_KERNEL_COOPERATIVE;
        hostRequest.input_mode = PROM_M40B_INPUT_HOST_A_PERSISTENT_B;
        hostRequest.submit_plan = PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER;
        hostRequest.required_b_generation = generation;
        prom_m40b_execution_result hostResult{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &hostRequest, &hostResult),
                     "host A plus persistent B composes cooperative SGEMM and softmax");
        const std::string hostMismatch = SoftmaxMismatch(expected, output, m, n, hostResult.padding,
                                                         hostResult.cooperative_shader_hash,
                                                         hostResult.reduction_replay_id);
        ASSERT_TRUE(hostMismatch.empty(), hostMismatch.empty() ?
                    "one final readback matches stable f16-rounded CPU SGEMM-softmax oracle" : hostMismatch.c_str());
        ASSERT_EQUAL(1u, hostResult.no_intermediate_host_copy, "C stays device resident through softmax");
        ASSERT_EQUAL(1u, hostResult.command_trace.intermediate_buffer_count, "one logical C exists");
        ASSERT_EQUAL(n, hostResult.intermediate_c.logical_columns, "device view carries logical shape explicitly");
        ASSERT_EQUAL(((n + 15u) & ~15u), hostResult.intermediate_c.row_stride_elements,
                     "padded C stride remains separate from logical softmax width");
        ASSERT_TRUE(hostResult.sgemm_gpu_ns > 0u && hostResult.softmax_gpu_ns > 0u && hostResult.combined_gpu_ns > 0u,
                    "dispatch, softmax, and combined GPU intervals are separate");

        prom_m40b_prepare_request prepareA{};
        prepareA.values = a.data(); prepareA.m = m; prepareA.n = n; prepareA.k = k;
        prepareA.kernel = PROM_M40B_KERNEL_COOPERATIVE; prepareA.generation = generation;
        prom_m40b_prepare_result preparedA{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_resident_a(runtime, &prepareA, &preparedA),
                     "packed A residency experiment uploads once");
        std::fill(output.begin(), output.end(), 0.0f);
        prom_m40b_execution_request deviceRequest = hostRequest;
        deviceRequest.host_a = nullptr;
        deviceRequest.input_mode = PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B;
        deviceRequest.submit_plan = PROM_M40B_SUBMIT_TWO_BOUNDED;
        deviceRequest.required_a_generation = generation;
        prom_m40b_execution_result deviceResult{};
        ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &deviceRequest, &deviceResult),
                     "resident A+B use explicit bounded two-submit dependency");
        const std::string deviceMismatch = SoftmaxMismatch(expected, output, m, n, deviceResult.padding,
                                                           deviceResult.cooperative_shader_hash,
                                                           deviceResult.reduction_replay_id);
        ASSERT_TRUE(deviceMismatch.empty(), deviceMismatch.empty() ?
                    "resident A+B result matches oracle" : deviceMismatch.c_str());
        ASSERT_EQUAL(0u, deviceResult.a_conversion_ns, "resident A skips per-operation conversion");
        ASSERT_EQUAL(2u, deviceResult.submit_count, "two-submit plan is observed");
        ASSERT_TRUE(deviceResult.buffer_reuse_count > 0u, "steady execution reuses ring buffers");

        prom_m40b_execution_request stale = deviceRequest;
        stale.required_b_generation = generation - 1u;
        ASSERT_TRUE(prom_reactor_runtime_m40b_execute(runtime, &stale, &deviceResult) != PROM_OK,
                    "stale persistent B generation rejects before dispatch");
        ASSERT_EQUAL(PROM_M40B_DETAIL_STALE_GENERATION, deviceResult.detail_code, "stale use is diagnosed");
        generation += 1u;
    }
    prom_reactor_runtime_get_vk_services(runtime, &services);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE),
                 services.cooperative_matrix_state,
                 "successful composed cooperative execution promotes executable capability state");
    ASSERT_EQUAL(0u, services.validation_warning_count, "composed proof has zero validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "composed proof has zero validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
}

FACT(PrometheusM40bQuarantineReapProtectsPersistentReplacement)
{
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.reduction_ring_depth = 2u;
    config.reduction_test_flags = PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION;
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_create_impl(&config, &runtime), "fault-injection runtime creation succeeds");
    if (runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(runtime, &services);
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("useful KHR cooperative tuple unavailable");
    }

    constexpr std::uint32_t m = 128u;
    constexpr std::uint32_t n = 320u;
    constexpr std::uint32_t k = 1024u;
    std::vector<float> a, b;
    FillComposedIdentityWorkload(&a, &b, m, n, k);
    const std::vector<float> expected = ComposedSoftmaxOracle(a, m, n, k, true);
    std::vector<float> output(static_cast<std::size_t>(m) * n);
    prom_m40b_prepare_request prepareB{};
    prepareB.values = b.data(); prepareB.m = m; prepareB.n = n; prepareB.k = k;
    prepareB.kernel = PROM_M40B_KERNEL_COOPERATIVE; prepareB.generation = 1u;
    prom_m40b_prepare_result preparedB{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_persistent_b(runtime, &prepareB, &preparedB),
                 "initial persistent B preparation succeeds");
    prom_m40b_execution_request request{};
    request.host_a = a.data(); request.output = output.data();
    request.m = m; request.n = n; request.k = k;
    request.kernel = PROM_M40B_KERNEL_COOPERATIVE;
    request.input_mode = PROM_M40B_INPUT_HOST_A_PERSISTENT_B;
    request.submit_plan = PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER;
    request.required_b_generation = 1u;
    prom_m40b_execution_result failed{};
    ASSERT_EQUAL(PROM_ERROR, prom_reactor_runtime_m40b_execute(runtime, &request, &failed),
                 "injected post-submit completion uncertainty is surfaced");
    ASSERT_EQUAL(PROM_M40B_DETAIL_COMPLETION_UNCERTAIN, failed.detail_code,
                 "uncertain completion has a distinct logical failure");
    ASSERT_EQUAL(0u, failed.physical_slot_recyclable,
                 "submitted producer-owned C remains quarantined until physical completion");

    prepareB.generation = 2u;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_persistent_b(runtime, &prepareB, &preparedB),
                 "replacement B waits for and reaps all consumers before replacing storage");
    ASSERT_EQUAL(1u, preparedB.replaced, "persistent B replacement is explicit");
    request.required_b_generation = 2u;
    prom_m40b_execution_result recovered{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &recovered),
                 "ring recovers after the one-shot uncertain completion");
    const std::string recoveryMismatch = SoftmaxMismatch(expected, output, m, n, recovered.padding,
                                                         recovered.cooperative_shader_hash,
                                                         recovered.reduction_replay_id);
    ASSERT_TRUE(recoveryMismatch.empty(), recoveryMismatch.empty() ?
                "recovery uses the fresh B generation without stale data" : recoveryMismatch.c_str());
    PrometheusReductionDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_reduction_diagnostics(runtime, &diagnostics),
                 "composed lifecycle diagnostics are available");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), diagnostics.quarantine_count,
                 "only uncertain submitted work enters quarantine");
    ASSERT_TRUE(diagnostics.reap_count >= 1u, "persistent replacement physically reaps the quarantined slot");
    ASSERT_EQUAL(0u, diagnostics.quarantined_slots, "replacement and recovery leave no slot quarantined");
    prom_reactor_runtime_get_vk_services(runtime, &services);
    ASSERT_EQUAL(0u, services.validation_warning_count, "lifecycle injection remains validation-warning clean");
    ASSERT_EQUAL(0u, services.validation_error_count, "lifecycle injection remains validation-error clean");
    prom_reactor_runtime_destroy_impl(runtime);
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM40bDeviceResidentInferenceCorpus, 1u)
{
    struct Workload { std::uint32_t m; std::uint32_t n; std::uint32_t k; const char* group; };
    struct ComposedKernel { const char* name; std::uint32_t id; bool rounded; };
    const std::array<Workload, 14> workloads = {{
        {128u,1024u,1024u,"aligned"}, {256u,1024u,1024u,"aligned"},
        {512u,1024u,1024u,"aligned"}, {1024u,1024u,1024u,"aligned"},
        {256u,4096u,1024u,"aligned"}, {1024u,4096u,1024u,"aligned"},
        {128u,320u,1024u,"diffusion-width"}, {128u,640u,1024u,"diffusion-width"},
        {128u,768u,1024u,"diffusion-width"}, {128u,1280u,1024u,"diffusion-width"},
        {128u,2048u,1024u,"diffusion-width"},
        {127u,1001u,1023u,"awkward"}, {257u,769u,1025u,"awkward"},
        {511u,1281u,2049u,"awkward"},
    }};
    const std::array<ComposedKernel, 3> kernels = {{
        {"cooperative", PROM_M40B_KERNEL_COOPERATIVE, true},
        {"A2x4", PROM_M40B_KERNEL_A2X4, false},
        {"conventional-fp16", PROM_M40B_KERNEL_CONVENTIONAL_FP16, true},
    }};
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(runtime, &services);
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("useful KHR cooperative tuple unavailable");
    }
    std::uint64_t bGeneration = 1u;
    std::uint64_t aGeneration = 1u;
    std::ostringstream json;
    json << "{\"schema\":\"prometheus.m40b.device-resident-inference.v1\",\"notation\":\"M x N x K\","
         << "\"warm_repetitions\":5,\"rows\":[";
    bool firstRow = true;
    for (const Workload& workload : workloads) {
        std::vector<float> a, b;
        FillComposedIdentityWorkload(&a, &b, workload.m, workload.n, workload.k);
        for (const ComposedKernel& kernel : kernels) {
            std::vector<float> output(static_cast<std::size_t>(workload.m) * workload.n);
            const std::vector<float> expected = ComposedSoftmaxOracle(a, workload.m, workload.n, workload.k, kernel.rounded);
            prom_m40b_prepare_request prepareB{};
            prepareB.values = b.data(); prepareB.m = workload.m; prepareB.n = workload.n; prepareB.k = workload.k;
            prepareB.kernel = kernel.id; prepareB.generation = bGeneration++;
            prom_m40b_prepare_result preparedB{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_persistent_b(runtime, &prepareB, &preparedB),
                         "benchmark persistent B preparation succeeds");
            prom_m40b_execution_request request{};
            request.host_a = a.data(); request.output = output.data();
            request.m = workload.m; request.n = workload.n; request.k = workload.k;
            request.kernel = kernel.id; request.input_mode = PROM_M40B_INPUT_HOST_A_PERSISTENT_B;
            request.submit_plan = PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER;
            request.required_b_generation = preparedB.generation;
            prom_m40b_execution_result first{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &first),
                         "first persistent-B/new-A invocation succeeds");
            const std::string firstMismatch = SoftmaxMismatch(expected, output, workload.m, workload.n,
                                                              first.padding, first.cooperative_shader_hash,
                                                              first.reduction_replay_id);
            ASSERT_TRUE(firstMismatch.empty(), firstMismatch.empty() ?
                        "first composed invocation is correct" : firstMismatch.c_str());
            const prom_m40b_command_trace hostOneTrace = first.command_trace;
            std::vector<std::uint64_t> hostSgemm, hostSoftmax, hostCombined, hostReadback, hostEndToEnd;
            for (std::uint32_t repetition = 0u; repetition < 5u; ++repetition) {
                prom_m40b_execution_result measured{};
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &measured),
                             "warm persistent-B/new-A invocation succeeds");
                hostSgemm.push_back(measured.sgemm_gpu_ns); hostSoftmax.push_back(measured.softmax_gpu_ns);
                hostCombined.push_back(measured.combined_gpu_ns); hostReadback.push_back(measured.final_readback_ns);
                hostEndToEnd.push_back(measured.end_to_end_ns);
            }
            prom_m40b_prepare_request prepareA{};
            prepareA.values = a.data(); prepareA.m = workload.m; prepareA.n = workload.n; prepareA.k = workload.k;
            prepareA.kernel = kernel.id; prepareA.generation = aGeneration++;
            prom_m40b_prepare_result preparedA{};
            ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_prepare_resident_a(runtime, &prepareA, &preparedA),
                         "benchmark resident A preparation succeeds");
            request.host_a = nullptr; request.input_mode = PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B;
            request.required_a_generation = preparedA.generation;
            std::vector<std::uint64_t> deviceCombined, deviceEndToEnd, oneSubmitCpu, twoSubmitCombined, twoSubmitCpu;
            prom_m40b_execution_result last{};
            for (std::uint32_t repetition = 0u; repetition < 5u; ++repetition) {
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &last),
                             "resident A+B invocation succeeds");
                deviceCombined.push_back(last.combined_gpu_ns); deviceEndToEnd.push_back(last.end_to_end_ns);
                oneSubmitCpu.push_back(last.cpu_submission_ns);
            }
            const std::string residentMismatch = SoftmaxMismatch(expected, output, workload.m, workload.n,
                                                                 last.padding, last.cooperative_shader_hash,
                                                                 last.reduction_replay_id);
            ASSERT_TRUE(residentMismatch.empty(), residentMismatch.empty() ?
                        "resident A+B composed output is correct" : residentMismatch.c_str());
            const prom_m40b_command_trace residentOneTrace = last.command_trace;
            request.submit_plan = PROM_M40B_SUBMIT_TWO_BOUNDED;
            for (std::uint32_t repetition = 0u; repetition < 3u; ++repetition) {
                ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &last),
                             "bounded two-submit comparison succeeds");
                twoSubmitCombined.push_back(last.combined_gpu_ns); twoSubmitCpu.push_back(last.cpu_submission_ns);
            }
            const prom_m40b_command_trace residentTwoTrace = last.command_trace;
            std::uint64_t repeat10Median = 0u;
            std::uint64_t repeat100Median = 0u;
            if (kernel.id == PROM_M40B_KERNEL_COOPERATIVE && workload.m == 128u && workload.n == 1024u && workload.k == 1024u) {
                std::vector<std::uint64_t> repeat10;
                std::vector<std::uint64_t> repeat100;
                request.submit_plan = PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER;
                for (std::uint32_t repetition = 0u; repetition < 10u; ++repetition) {
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &last), "10-operation replay succeeds");
                    repeat10.push_back(last.end_to_end_ns);
                }
                for (std::uint32_t repetition = 0u; repetition < 100u; ++repetition) {
                    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_m40b_execute(runtime, &request, &last), "100-operation replay succeeds");
                    repeat100.push_back(last.end_to_end_ns);
                }
                repeat10Median = Stats(repeat10).median;
                repeat100Median = Stats(repeat100).median;
            }
            if (!firstRow) json << ',';
            firstRow = false;
            json << "{\"kernel\":\"" << kernel.name << "\",\"precision\":\""
                 << (kernel.rounded ? "f16-rounded-input-f32-accum-output" : "fp32-input-accum-output")
                 << "\",\"group\":\"" << workload.group << "\",\"m\":" << workload.m
                 << ",\"n\":" << workload.n << ",\"k\":" << workload.k
                 << ",\"padded_m\":" << first.padding.padded_m << ",\"padded_n\":" << first.padding.padded_n
                 << ",\"padded_k\":" << first.padding.padded_k << ",\"correct\":true"
                 << ",\"b_prepare\":{\"conversion_ns\":" << preparedB.conversion_ns
                 << ",\"upload_ns\":" << preparedB.upload_ns << ",\"generation\":" << preparedB.generation << '}'
                 << ",\"a_resident_prepare\":{\"conversion_ns\":" << preparedA.conversion_ns
                 << ",\"upload_ns\":" << preparedA.upload_ns << ",\"generation\":" << preparedA.generation << '}'
                 << ",\"first\":{\"sgemm_gpu_ns\":" << first.sgemm_gpu_ns
                 << ",\"softmax_gpu_ns\":" << first.softmax_gpu_ns << ",\"combined_gpu_ns\":" << first.combined_gpu_ns
                 << ",\"readback_ns\":" << first.final_readback_ns << ",\"end_to_end_ns\":" << first.end_to_end_ns << '}'
                 << ",\"persistent_b_new_a\":{\"sgemm\":" << TimingJson(Stats(hostSgemm))
                 << ",\"softmax\":" << TimingJson(Stats(hostSoftmax)) << ",\"combined\":" << TimingJson(Stats(hostCombined))
                 << ",\"readback\":" << TimingJson(Stats(hostReadback)) << ",\"end_to_end\":" << TimingJson(Stats(hostEndToEnd)) << '}'
                 << ",\"device_a_b\":{\"combined\":" << TimingJson(Stats(deviceCombined))
                 << ",\"end_to_end\":" << TimingJson(Stats(deviceEndToEnd)) << '}'
                 << ",\"submit_comparison\":{\"one_cpu\":" << TimingJson(Stats(oneSubmitCpu))
                 << ",\"two_cpu\":" << TimingJson(Stats(twoSubmitCpu))
                 << ",\"two_combined\":" << TimingJson(Stats(twoSubmitCombined)) << '}'
                 << ",\"repeat_10_median_end_to_end_ns\":" << repeat10Median
                 << ",\"repeat_100_median_end_to_end_ns\":" << repeat100Median
                 << ",\"command_plan_replay_id\":" << last.command_plan_replay_id
                 << ",\"command_traces\":{\"host_one\":" << CommandTraceJson(hostOneTrace)
                 << ",\"resident_one\":" << CommandTraceJson(residentOneTrace)
                 << ",\"resident_two\":" << CommandTraceJson(residentTwoTrace) << '}'
                 << ",\"reduction_replay_id\":" << last.reduction_replay_id
                 << ",\"shader_hash\":" << last.cooperative_shader_hash
                 << ",\"retained_bytes\":" << last.retained_bytes
                 << ",\"buffer_grows\":" << last.buffer_allocation_count
                 << ",\"buffer_reuses\":" << last.buffer_reuse_count
                 << ",\"descriptor_updates\":" << last.descriptor_update_count
                 << ",\"pipeline_count\":" << last.pipeline_create_count
                 << ",\"command_buffer_reuses\":" << last.command_buffer_reuse_count << '}';
        }
    }
    prom_reactor_runtime_get_vk_services(runtime, &services);
    json << "],\"capability\":{\"state\":" << services.cooperative_matrix_state
         << ",\"tuple\":\"subgroup-m16-n16-k16-f16-f16-f32-f32\",\"subgroup_size\":" << services.subgroup_size
         << "},\"validation\":{\"warnings\":" << services.validation_warning_count
         << ",\"errors\":" << services.validation_error_count << "}}";
    ASSERT_EQUAL(0u, services.validation_warning_count, "corpus has zero validation warnings");
    ASSERT_EQUAL(0u, services.validation_error_count, "corpus has zero validation errors");
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m40b_device_resident_inference.json", json.str()),
                "M40b benchmark artifact is written");
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusM40aCooperativeMatrixComparison, 1u)
{
    constexpr std::uint32_t benchmarkWarmup = 1u;
    constexpr std::uint32_t benchmarkIterations = 5u;
    constexpr std::uint32_t preparationWarmup = 1u;
    constexpr std::uint32_t preparationIterations = 5u;
    EnvironmentValue validationEnvironment("PROMETHEUS_VK_VALIDATION", "1");
    const std::array<Kernel, 7> kernels = {{
        {"tiled", k_prom_sgemm_tiled_spirv, sizeof(k_prom_sgemm_tiled_spirv), "main", {8u,8u,1u,1u,1u,0u,0u,0u,0u}, PROM_VK_COMPUTE_TILED, "fp32"},
        {"memory-conservative", k_prom_sgemm_memory_conservative_spirv, sizeof(k_prom_sgemm_memory_conservative_spirv), "main", {8u,8u,1u,1u,1u,0u,0u,0u,0u}, PROM_VK_COMPUTE_TILED, "fp32"},
        {"B2x2", k_prom_sgemm_b2x2_row_major_biased_spirv, sizeof(k_prom_sgemm_b2x2_row_major_biased_spirv), "SgemmB2x2_CS", {8u,8u,1u,2u,2u,0u,0u,0u,0u}, PROM_VK_COMPUTE_TILED, "fp32"},
        {"A2x4", k_prom_sgemm_a2x4_row_biased_accum8_spirv, sizeof(k_prom_sgemm_a2x4_row_biased_accum8_spirv), "SgemmA2x4_CS", {8u,8u,1u,2u,4u,0u,0u,0u,0u}, PROM_VK_COMPUTE_TILED, "fp32"},
        {"Packed4", k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "SgemmPacked4_CS", {8u,8u,1u,1u,1u,0u,0u,0u,0u}, PROM_VK_COMPUTE_PACKED4_FP32, "fp32"},
        {"FP16-storage", k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv), "SgemmFp16StorageFp32Accum_CS", {8u,8u,1u,1u,1u,0u,0u,0u,0u}, PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM, "fp16-input-fp32-accum"},
        {"cooperative", k_prom_m40a_cooperative_sgemm_spirv, sizeof(k_prom_m40a_cooperative_sgemm_spirv), kCooperativeEntry, {32u,1u,1u,1u,1u,16u,16u,16u,1u,16u,16u}, PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM, "fp16-input-fp32-accum"},
    }};
    void* runtime = nullptr;
    if (prom_reactor_runtime_create_impl(nullptr, &runtime) != PROM_OK || runtime == nullptr) { SKIP("Vulkan runtime unavailable"); }
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(runtime, &services);
    if (services.cooperative_matrix_feature_enabled == 0u) {
        prom_reactor_runtime_destroy_impl(runtime);
        SKIP("useful KHR cooperative tuple unavailable");
    }
    std::ostringstream json;
    json << "{\"schema\":\"prometheus.m40a.cooperative-benchmark.v1\",\"warmup\":" << benchmarkWarmup
         << ",\"iterations\":" << benchmarkIterations << ",\"rows\":[";
    bool first = true;
    for (const std::uint32_t size : {256u, 512u, 1024u}) {
        std::vector<float> a, b;
        FillIdentityWorkload(&a, &b, size);
        for (const Kernel& kernel : kernels) {
            auto descriptor = kernel.name == std::string("cooperative") ? CooperativeDescriptor() : Descriptor(kernel);
            std::vector<float> c;
            std::vector<std::uint64_t> samples;
            const bool ran = Run(runtime, descriptor, a, b, &c, size, benchmarkWarmup, benchmarkIterations, &samples);
            ASSERT_TRUE(ran && Matches(a, c, 0.002f), "comparison kernel executes correctly");
            const TimingStats timing = Stats(samples);
            if (!first) json << ',';
            first = false;
            const double gflops = timing.median == 0u ? 0.0 :
                (2.0 * static_cast<double>(size) * size * size) / static_cast<double>(timing.median);
            json << "{\"kernel\":\"" << kernel.name << "\",\"precision\":\"" << kernel.precision
                 << "\",\"m\":" << size << ",\"n\":" << size << ",\"k\":" << size
                 << ",\"correct\":" << (ran && Matches(a, c, 0.002f) ? "true" : "false")
                 << ",\"shader_hash\":\"" << std::hex << descriptor.spirv_hash << std::dec
                 << "\",\"replay_identity\":\"" << ReplayIdentity(kernel.name, size, descriptor.spirv_hash)
                 << "\",\"timing\":" << TimingJson(timing) << ",\"effective_gflops\":" << std::setprecision(10) << gflops << '}';
        }
        std::vector<float> selectedOutput(static_cast<std::size_t>(size) * size, 0.0f);
        PrometheusSgemmResidentBenchmarkRequest selectedRequest{};
        selectedRequest.struct_size = sizeof(selectedRequest);
        selectedRequest.a = a.data(); selectedRequest.b = b.data(); selectedRequest.c = selectedOutput.data();
        selectedRequest.m = size; selectedRequest.n = size; selectedRequest.k = size;
        selectedRequest.mode = PROM_SGEMM_RESIDENT_MODE_PRODUCTION_SELECTOR;
        selectedRequest.warmup_iterations = benchmarkWarmup; selectedRequest.iterations = benchmarkIterations;
        PrometheusSgemmResidentBenchmarkResult selectedResult{};
        selectedResult.struct_size = sizeof(selectedResult);
        const bool selectedTimed = prom_reactor_runtime_sgemm_resident_benchmark_impl(runtime, &selectedRequest, &selectedResult) == PROM_OK &&
            selectedResult.gpu_timestamp_valid != 0u;
        uint32_t selectedStage = 0u;
        int selectedDetail = 0;
        const bool selectedCorrect = prom_reactor_runtime_sgemm_impl(runtime, a.data(), b.data(), selectedOutput.data(),
            size, size, size, &selectedStage, &selectedDetail) == PROM_OK && Matches(a, selectedOutput, 0.002f);
        const bool selectedRan = selectedTimed && selectedCorrect;
        ASSERT_TRUE(selectedRan, "current production-selected path executes correctly");
        if (!first) json << ',';
        first = false;
        json << "{\"kernel\":\"production-selected\",\"precision\":\"fp32-policy-result\",\"m\":" << size
             << ",\"n\":" << size << ",\"k\":" << size << ",\"correct\":" << (selectedRan ? "true" : "false")
             << ",\"selected_variant\":" << selectedResult.production_selected_variant
             << ",\"executed_variant\":" << selectedResult.executed_variant
             << ",\"replay_identity\":\"" << ReplayIdentity("production-selected", size, selectedResult.executed_variant)
             << "\""
             << ",\"timing\":{\"min_ns\":" << selectedResult.kernel_min_ns
             << ",\"median_ns\":" << selectedResult.kernel_median_ns
             << ",\"p95_ns\":" << selectedResult.kernel_p95_ns << "}}";
    }
    json << "],\"preparation_modes\":[";
    std::vector<float> a, b, c;
    constexpr std::uint32_t preparationSize = 512u;
    FillIdentityWorkload(&a, &b, preparationSize);
    const auto cooperativeDescriptor = CooperativeDescriptor();
    struct PreparationKernel { const char* name; prom_sgemm_audit_execution_descriptor descriptor; };
    const std::array<PreparationKernel, 3> preparationKernels = {{
        {"A2x4", Descriptor(kernels[3])},
        {"FP16-storage", Descriptor(kernels[5])},
        {"cooperative", cooperativeDescriptor},
    }};
    bool firstMode = true;
    for (const auto& preparationKernel : preparationKernels) {
    for (const auto mode : {PROM_SGEMM_PLACEMENT_REUSE_REUPLOAD,
         PROM_SGEMM_PLACEMENT_REUSE_PERSISTENT_B_REUPLOAD_A, PROM_SGEMM_PLACEMENT_REUSE_WARM}) {
        const char* modeName = mode == PROM_SGEMM_PLACEMENT_REUSE_WARM ? "persistent-input" :
            (mode == PROM_SGEMM_PLACEMENT_REUSE_PERSISTENT_B_REUPLOAD_A ? "persistent-packed-weight" :
                                                                            "reupload-every-iteration");
        prom_sgemm_placement_benchmark_options options{};
        options.a_placement = options.b_placement = options.c_placement = PROM_SGEMM_MEMORY_PLACEMENT_PURE_DEVICE_LOCAL;
        options.reuse_mode = mode; options.warmup = preparationWarmup; options.iterations = preparationIterations;
        std::vector<std::uint64_t> kernel(preparationIterations), preparation(preparationIterations);
        std::vector<std::uint64_t> endToEnd(preparationIterations), conversion(preparationIterations);
        std::vector<std::uint64_t> upload(preparationIterations), readback(preparationIterations);
        c.assign(static_cast<std::size_t>(preparationSize) * preparationSize, 0.0f);
        prom_sgemm_placement_benchmark_result result{};
        const bool ran = prom_reactor_runtime_sgemm_placement_benchmark_detailed_impl(
            runtime, a.data(), b.data(), c.data(), preparationSize, preparationSize, preparationSize,
            &preparationKernel.descriptor, &options,
            kernel.data(), preparation.data(), endToEnd.data(), conversion.data(), upload.data(), readback.data(),
            preparationIterations, &result) == PROM_OK;
        ASSERT_TRUE(ran && Matches(a, c, 0.002f), "cooperative preparation mode executes correctly");
        if (!firstMode) json << ',';
        firstMode = false;
        json << "{\"kernel\":\"" << preparationKernel.name << "\",\"m\":" << preparationSize
             << ",\"n\":" << preparationSize << ",\"k\":" << preparationSize << ",\"mode\":\"" << modeName
             << "\",\"replay_identity\":\""
             << ReplayIdentity(preparationKernel.name, preparationSize, preparationKernel.descriptor.spirv_hash, modeName)
             << "\",\"kernel_timing\":" << TimingJson(Stats(kernel))
             << ",\"conversion_pack\":" << TimingJson(Stats(conversion))
             << ",\"upload_staging\":" << TimingJson(Stats(upload))
             << ",\"readback\":" << TimingJson(Stats(readback))
             << ",\"preparation_total\":" << TimingJson(Stats(preparation))
             << ",\"end_to_end\":" << TimingJson(Stats(endToEnd)) << '}';
    }
    }
    constexpr std::uint32_t awkwardSize = 257u;
    std::vector<float> awkwardA, awkwardB, awkwardC(static_cast<std::size_t>(awkwardSize) * awkwardSize);
    std::vector<std::uint64_t> tailRejectionSamples(preparationIterations);
    FillIdentityWorkload(&awkwardA, &awkwardB, awkwardSize);
    for (std::uint32_t iteration = 0u; iteration < preparationIterations; ++iteration) {
        prom_sgemm_audit_execution_result tailResult{};
        const auto begin = std::chrono::steady_clock::now();
        const bool rejected = prom_reactor_runtime_sgemm_audit_impl(
            runtime, awkwardA.data(), awkwardB.data(), awkwardC.data(), awkwardSize, awkwardSize, awkwardSize,
            &cooperativeDescriptor, &tailResult) != PROM_OK;
        const auto end = std::chrono::steady_clock::now();
        tailRejectionSamples[iteration] = static_cast<std::uint64_t>(
            std::chrono::duration_cast<std::chrono::nanoseconds>(end - begin).count());
        ASSERT_TRUE(rejected, "awkward cooperative shape is rejected");
        ASSERT_EQUAL(0u, tailResult.pipeline_create_count, "tail rejection creates no pipeline");
    }
    prom_reactor_runtime_get_vk_services(runtime, &services);
    json << "],\"preparation_warmup\":" << preparationWarmup
         << ",\"preparation_iterations\":" << preparationIterations
         << ",\"tail\":{\"strategy\":\"reject\",\"m\":" << awkwardSize
         << ",\"n\":" << awkwardSize << ",\"k\":" << awkwardSize
         << ",\"pipeline_create_count\":0,\"host_rejection_timing\":" << TimingJson(Stats(tailRejectionSamples)) << '}'
         << ",\"capability\":{\"extension\":\"VK_KHR_cooperative_matrix\",\"spec_version\":"
         << services.cooperative_matrix_extension_spec_version << ",\"subgroup_size\":" << services.subgroup_size
         << ",\"tuple\":\"subgroup-m16-n16-k16-f16-f16-f32-f32\"},\"validation\":{\"enabled\":"
         << (services.validation_enabled ? "true" : "false") << ",\"warnings\":" << services.validation_warning_count
         << ",\"errors\":" << services.validation_error_count << "}}";
    ASSERT_EQUAL(0u, services.validation_error_count, "comparison remains validation clean");
    prom_reactor_runtime_destroy_impl(runtime);
    ASSERT_TRUE(context.WriteTextArtifact("prometheus_m40a_cooperative_benchmark.json", json.str()), "benchmark JSON is written");
}
