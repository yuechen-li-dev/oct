#include "reactor_concept_rmsnorm.h"
#include "concept_vulkan.h"
#include "reactor_shader_package.h"
#include <chrono>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <expected>
#include <memory>
#include <mutex>
#include <new>
#include <system_error>

static_assert(__cpp_lib_expected >= 202202L);
namespace {
// Calls on one model owner, including resident consumers and destruction, are
// externally serialized. This lock also protects the Concept executor itself.
// Allocation/mutex errors are translated; no exception crosses the C ABI.
struct Attachment { void* executor{}; std::mutex mutex; };
std::mutex attachments;
using Clock = std::chrono::steady_clock;
uint64_t elapsed(Clock::time_point begin) {
    return static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(Clock::now() - begin).count());
}
[[nodiscard]] std::expected<Attachment*, int> attach(void* host, const prom_vk_runtime_services& services) {
    const std::lock_guard lock{attachments};
    if (auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_norm_state(host))) return state;
    auto state = std::make_unique<Attachment>();
    int32_t detail{};
    state->executor = prom_concept_norm_create(host, &services, &detail);
    if (!state->executor) return std::unexpected(detail);
    if (prom_reactor_runtime_set_concept_norm_state(host, state.get()) != PROM_OK) {
        if (prom_concept_norm_destroy(state->executor) != PROM_OK) std::terminate();
        return std::unexpected(-3);
    }
    return state.release();
}
bool finite(const float* values, uint64_t count) {
    for (uint64_t i = 0; i < count; ++i) if (!std::isfinite(values[i])) return false;
    return true;
}
uint64_t weight_hash(const float* values, uint64_t count) {
    uint64_t hash = 1469598103934665603ull;
    for (uint64_t i = 0; i < count; ++i) {
        uint32_t bits{}; std::memcpy(&bits, values + i, sizeof bits);
        for (unsigned byte = 0; byte < 4; ++byte) { hash ^= static_cast<uint8_t>(bits >> (byte * 8)); hash *= 1099511628211ull; }
    }
    return hash;
}
bool valid(const prom_m49a_m46_request& r) {
    const uint64_t storage = uint64_t{r.tokens} * r.z_row_stride;
    const uint64_t logical = uint64_t{r.tokens} * r.model_width;
    if (!r.matched_z || !r.tokens || r.tokens > PROM_M42_MAX_TOKENS || !r.model_width || r.model_width > PROM_M42_MAX_MODEL_WIDTH ||
        r.z_row_stride < r.model_width || r.strategy < PROM_M46_STRATEGY_SEPARATE_OUTPUT || r.strategy > PROM_M46_STRATEGY_IN_PLACE_Z ||
        r.requested_reduction_plan > PROM_M46_REDUCTION_FORCE_STAGED ||
        (r.requested_reduction_plan == PROM_M46_REDUCTION_FORCE_FUSED && r.model_width > 1024u) ||
        !std::isfinite(r.epsilon) || r.epsilon <= 0 || !r.input_generation || !r.reference_input_hash ||
        !r.required_weight_generation || !r.required_weight_hash || !r.exact_source_hash ||
        storage != r.matched_storage_element_count || storage > SIZE_MAX / sizeof(float) ||
        storage * sizeof(float) > PROM_M46_CAPACITY_LIMIT_BYTES || r.direct_rope > 1u ||
        r.bf16_roundtrip_input > 1u || r.bf16_roundtrip_output > 1u) return false;
    if (!r.direct_rope) return r.output && r.inv_rms_output && r.output_element_count == logical && r.inv_rms_output_element_count == r.tokens;
    return r.model_width == 256u && r.z_row_stride == 256u && r.rope_tokens == 15u &&
        (r.rope_heads == 1u || r.rope_heads == 8u) && r.tokens == r.rope_tokens * r.rope_heads &&
        r.rope_table_generation && r.rope_cosine && r.rope_sine &&
        r.rope_cosine_element_count == uint64_t{r.rope_tokens} * r.model_width &&
        r.rope_sine_element_count == r.rope_cosine_element_count &&
        (r.rope_output ? r.rope_output_element_count == logical : r.rope_output_element_count == 0u);
}
}

extern "C" PipelineCreation PromNormLoadPipeline(void* host, VkDevice device, VkPipelineCache cache, uint32_t role) {
    PipelineCreation failed{}; failed.code = VK_ERROR_INITIALIZATION_FAILED;
    constexpr uint32_t kernels[] = {63, 64, 67, 68};
    if (role >= 4) return failed;
    char variant[32]{}; std::snprintf(variant, sizeof variant, "kernel-%u-default", kernels[role]);
    VkShaderModule shader{}; const char* entry{}; prom_shader_package_diagnostic diagnostic{};
    prom_shader_package* package{};
    if (prom_reactor_runtime_get_shader_package(host, &package) != PROM_OK || !package ||
        !prom_shader_package_create_module(package, device, variant, &shader, &entry, &diagnostic)) return failed;
    const BindingSlot slots[] = {{0, 0}, {1, 0}, {2, 0}, {3, 0}};
    return ConceptVkCreatePipelineFromModule(device, cache, shader, entry, ConceptVkBindingSlots{slots, 4}, 32);
}

extern "C" int prom_reactor_runtime_m46_prepare_weight(void* host, const prom_m46_weight_prepare_request* r,
    prom_m46_weight_prepare_result* out) {
    if (!out) return PROM_ERROR;
    *out = {}; out->stage = PROM_STAGE_INIT; out->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    if (!prom_reactor_runtime_validate_handle(host)) return PROM_INVALID_HANDLE;
    if (!r || !r->values || !r->model_width || r->model_width > PROM_M42_MAX_MODEL_WIDTH ||
        !r->generation || r->element_count != r->model_width) return PROM_ERROR;
    if (!finite(r->values, r->element_count)) { out->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT; return PROM_ERROR; }
    out->hash = weight_hash(r->values, r->element_count);
    const auto begin = Clock::now();
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(host, &services) != PROM_OK || !services.backend_available) return PROM_ERROR;
    try {
        auto state = attach(host, services);
        if (!state) { out->detail_code = PROM_M46_DETAIL_RESOURCE; return PROM_ERROR; }
        const std::lock_guard lock{(*state)->mutex};
        const int status = prom_concept_norm_weight((*state)->executor, r, out);
        out->preparation_ns = elapsed(begin);
        if (status == PROM_OK) { out->stage = 0; out->detail_code = 0; }
        return status;
    } catch (const std::bad_alloc&) { out->detail_code = PROM_M46_DETAIL_RESOURCE; }
      catch (const std::system_error&) { out->detail_code = PROM_M46_DETAIL_RESOURCE; }
    return PROM_ERROR;
}

extern "C" int prom_reactor_runtime_m49a_execute_m46(void* host, const prom_m49a_m46_request* r,
    prom_m49a_m46_result* out) {
    if (!out) return PROM_ERROR;
    *out = {}; out->audit_only = 1; out->no_product_intermediate_readback_change = 1;
    out->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    if (!prom_reactor_runtime_validate_handle(host)) return PROM_INVALID_HANDLE;
    if (!r || !valid(*r)) return PROM_ERROR;
    out->input_hash = prom_num_hash_float_bits(r->matched_z, r->matched_storage_element_count);
    if (!finite(r->matched_z, r->matched_storage_element_count) || out->input_hash != r->reference_input_hash ||
        (r->direct_rope && (!finite(r->rope_cosine, r->rope_cosine_element_count) || !finite(r->rope_sine, r->rope_sine_element_count)))) {
        out->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT; return PROM_ERROR;
    }
    const auto begin = Clock::now();
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(host, &services) != PROM_OK || !services.backend_available) return PROM_ERROR;
    try {
        auto state = attach(host, services);
        if (!state) { out->detail_code = PROM_M46_DETAIL_RESOURCE; return PROM_ERROR; }
        const std::lock_guard lock{(*state)->mutex};
        // A legacy consumer can retain these borrows after an uncertain wait.
        // Before replacing a published resident buffer, establish its completion.
        prom_device_buffer_view prior{};
        if (r->direct_rope && prom_concept_norm_resident((*state)->executor, r->rope_heads, &prior) == PROM_OK &&
            prom_vk_device_wait_idle(services.device) != VK_SUCCESS) {
            out->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN; return PROM_ERROR;
        }
        if (ConceptVkSetQueueSubmit(services.device, host, prom_vk_concept_queue_submit) != 0) { out->detail_code = PROM_M46_DETAIL_RESOURCE; return PROM_ERROR; }
        out->detail_code = 0;
        const int status = prom_concept_norm_execute((*state)->executor, r, out);
        out->rmsnorm.end_to_end_ns = elapsed(begin);
        if (status != PROM_OK) return status;
        const float* values = r->direct_rope ? r->rope_output : r->output;
        const uint64_t count = r->direct_rope ? r->rope_output_element_count : r->output_element_count;
        if (values && !finite(values, count)) { out->detail_code = PROM_M46_DETAIL_READBACK; return PROM_ERROR; }
        out->output_hash = values ? prom_num_hash_float_bits(values, count) : 1u;
        if (!r->direct_rope) {
            if (!finite(r->inv_rms_output, r->inv_rms_output_element_count)) { out->detail_code = PROM_M46_DETAIL_READBACK; return PROM_ERROR; }
            out->inv_rms_hash = prom_num_hash_float_bits(r->inv_rms_output, r->inv_rms_output_element_count);
        }
        return PROM_OK;
    } catch (const std::bad_alloc&) { out->detail_code = PROM_M46_DETAIL_RESOURCE; }
      catch (const std::system_error&) { out->detail_code = PROM_M46_DETAIL_RESOURCE; }
    return PROM_ERROR;
}

extern "C" int prom_reactor_runtime_concept_norm_resident(void* host, uint32_t heads, prom_device_buffer_view* view) {
    try {
        const std::lock_guard registry{attachments};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_norm_state(host));
        if (!state) return PROM_ERROR;
        const std::lock_guard lock{state->mutex};
        return prom_concept_norm_resident(state->executor, heads, view);
    } catch (const std::system_error&) { return PROM_ERROR; }
}
extern "C" void prom_reactor_runtime_concept_norm_consume(void* host) {
    try {
        const std::lock_guard registry{attachments};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_norm_state(host));
        if (!state) return;
        const std::lock_guard lock{state->mutex};
        prom_concept_norm_consume(state->executor);
    } catch (const std::system_error&) { /* Retaining borrows is conservative. */ }
}
extern "C" int prom_reactor_runtime_concept_norm_detach(void* host) {
    try {
        const std::lock_guard registry{attachments};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_norm_state(host));
        if (!state) return PROM_OK;
        {
            const std::lock_guard lock{state->mutex};
            prom_vk_runtime_services services{};
            if (prom_reactor_runtime_get_vk_services(host, &services) != PROM_OK) return PROM_ERROR;
            const auto idle = prom_vk_device_wait_idle(services.device);
            if (idle != VK_SUCCESS && idle != VK_ERROR_DEVICE_LOST) return PROM_ERROR;
            if (prom_concept_norm_destroy(state->executor) != PROM_OK) return PROM_ERROR;
        }
        prom_reactor_runtime_set_concept_norm_state(host, nullptr);
        delete state; return PROM_OK;
    } catch (const std::system_error&) { return PROM_ERROR; }
}
