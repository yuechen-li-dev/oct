#include "reactor_concept_reduction.h"
#include "concept_vulkan.h"
#include "reactor_shader_package.h"
#include <chrono>
#include <cstdio>
#include <expected>
#include <memory>
#include <mutex>
#include <new>
#include <system_error>

static_assert(__cpp_lib_expected >= 202202L);
namespace {
// Allocation and mutex failures are translated here. No C++ exception crosses
// the public C ABI. Runtime destruction requires its callers to be quiescent.
struct Attachment {
    void* executor{};
    std::mutex mutex;
    PrometheusReductionExecutionResult last{};
};
std::mutex attachments;
using Clock = std::chrono::steady_clock;

[[nodiscard]] std::expected<Attachment*, int> attach(void* host, const prom_vk_runtime_services& services) {
    const std::lock_guard lock{attachments};
    if (auto* existing = static_cast<Attachment*>(prom_reactor_runtime_concept_reduction_state(host))) return existing;
    auto state = std::make_unique<Attachment>();
    int32_t detail{};
    state->executor = prom_concept_reduction_create(host, &services, &detail);
    if (!state->executor) return std::unexpected(detail);
    auto* result = state.get();
    if (prom_reactor_runtime_set_concept_reduction_state(host, result) != PROM_OK) {
        if (prom_concept_reduction_destroy(state->executor) != PROM_OK) std::terminate();
        return std::unexpected(-3);
    }
    state.release();
    return result;
}
}

extern "C" PipelineCreation PromReductionLoadPipeline(void* host, VkDevice device, VkPipelineCache cache, uint32_t role) {
    PipelineCreation failed{}; failed.code = VK_ERROR_INITIALIZATION_FAILED;
    if (role < 1u || role > 7u) return failed;
    char variant[32]{};
    std::snprintf(variant, sizeof variant, "kernel-%u-default", 15u + role);
    VkShaderModule shader{}; const char* entry{}; prom_shader_package_diagnostic diagnostic{};
    prom_shader_package* package{};
    if (prom_reactor_runtime_get_shader_package(host, &package) != PROM_OK || !package) return failed;
    if (!prom_shader_package_create_module(package, device,
            variant, &shader, &entry, &diagnostic)) return failed;
    const BindingSlot slots[] = {{0, 0}, {1, 0}, {2, 0}, {3, 0}};
    return ConceptVkCreatePipelineFromModule(device, cache, shader, entry, ConceptVkBindingSlots{slots, 4u}, 32u);
}

extern "C" int prom_reactor_runtime_reduction_impl(void* host, const PrometheusReductionRequest* request,
    PrometheusReductionExecutionResult* destination) {
    PrometheusReductionExecutionResult local{};
    auto& out = destination ? *destination : local;
    out = {}; out.struct_size = sizeof out; out.physical_slot_id = UINT32_MAX;
    out.first_nonfinite_index = UINT64_MAX;
    const auto begin = Clock::now();
    if (!prom_reactor_runtime_validate_handle(host)) {
        out.stage = PROM_STAGE_INIT; out.detail_code = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE; return PROM_INVALID_HANDLE;
    }
    out.stage = PROM_STAGE_INIT; out.detail_code = PROM_REDUCTION_DETAIL_INVALID_REQUEST;
    if (!request || request->struct_size < sizeof(*request)) return PROM_ERROR;
    if (!request->input) { out.detail_code = PROM_REDUCTION_DETAIL_NULL_INPUT; return PROM_ERROR; }
    if (!request->output) { out.detail_code = PROM_REDUCTION_DETAIL_NULL_OUTPUT; return PROM_ERROR; }
    if (prom_concept_reduction_validate(request, &out) != PROM_OK) return PROM_ERROR;
    if (prom_reactor_reduction_plan_impl(request, &out.plan) != PROM_OK) {
        out.detail_code = PROM_REDUCTION_DETAIL_MALFORMED_PLAN; return PROM_ERROR;
    }
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(host, &services) != PROM_OK || !services.backend_available) {
        out.stage = PROM_STAGE_INIT; out.detail_code = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE; return PROM_ERROR;
    }
    out.validation_error_count_before = services.validation_error_count;
    try {
        auto attachment = attach(host, services);
        if (!attachment) { out.stage = PROM_STAGE_INIT; out.detail_code = PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED; return PROM_ERROR; }
        const std::lock_guard lock{(*attachment)->mutex};
        if (ConceptVkSetQueueSubmit(services.device, host, prom_vk_concept_queue_submit) != 0) {
            out.stage = PROM_STAGE_INIT; out.detail_code = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE; return PROM_ERROR;
        }
        const int result = prom_concept_reduction_execute((*attachment)->executor, request, &out);
        out.end_to_end_ns = static_cast<uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(Clock::now() - begin).count());
        if (prom_reactor_runtime_get_vk_services(host, &services) == PROM_OK) out.validation_error_count_after = services.validation_error_count;
        (*attachment)->last = out;
        return result;
    } catch (const std::bad_alloc&) { out.detail_code = PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED; }
      catch (const std::system_error&) { out.detail_code = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE; }
    out.stage = PROM_STAGE_INIT;
    return PROM_ERROR;
}

extern "C" int prom_reactor_runtime_reduction_diagnostics_impl(void* host, PrometheusReductionDiagnostics* out) {
    if (!out) return PROM_ERROR;
    *out = {}; out->struct_size = sizeof(*out);
    if (!prom_reactor_runtime_validate_handle(host)) return PROM_INVALID_HANDLE;
    prom_vk_runtime_services services{};
    prom_reactor_runtime_get_vk_services(host, &services);
    out->configured_ring_depth = services.reduction_ring_depth; out->physical_slot_count = services.reduction_ring_depth;
    out->validation_enabled = services.validation_enabled; out->validation_error_count = services.validation_error_count;
    try {
        const std::lock_guard registry{attachments};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_reduction_state(host));
        if (!state) return PROM_OK;
        const std::lock_guard lock{state->mutex};
        prom_concept_reduction_stats(state->executor, out);
        out->last_replay_id = state->last.plan.replay_id; out->last_gpu_duration_ns = state->last.gpu_duration_ns;
        out->last_end_to_end_ns = state->last.end_to_end_ns; out->last_stage_count = state->last.plan.stage_count;
        out->last_physical_slot_id = state->last.physical_slot_id; out->last_physical_slot_generation = state->last.physical_slot_generation;
        return PROM_OK;
    } catch (const std::system_error&) { return PROM_ERROR; }
}

extern "C" int prom_reactor_runtime_concept_reduction_detach(void* host) {
    try {
        const std::lock_guard registry{attachments};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_reduction_state(host));
        if (!state) return PROM_OK;
        {
            const std::lock_guard lock{state->mutex};
            if (prom_concept_reduction_destroy(state->executor) != PROM_OK) return PROM_ERROR;
        }
        prom_reactor_runtime_set_concept_reduction_state(host, nullptr);
        delete state;
        return PROM_OK;
    } catch (const std::system_error&) { return PROM_ERROR; }
}
