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

std::string ReplayIdentity(const char* kernel, std::uint32_t size, std::uint64_t shaderHash,
                           const char* mode = "kernel-only")
{
    std::ostringstream out;
    out << "m40a-identity-workload-v1:" << kernel << ':' << size << 'x' << size << 'x' << size
        << ':' << mode << ':' << std::hex << shaderHash;
    return out.str();
}
} // namespace

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
