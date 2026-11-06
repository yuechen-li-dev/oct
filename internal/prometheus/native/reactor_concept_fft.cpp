#include "reactor_concept_fft.h"
#include "reactor_shader_package.h"
#include "concept_vulkan.h"
#include <cmath>
#include <cstdio>
#include <memory>
#include <mutex>
#include <new>
#include <numbers>
#include <system_error>
#include <vector>

namespace {
struct Attachment {
    void* executor{};
    std::mutex mutex;
    uint32_t count{};
    std::vector<double> twiddles;
};
std::mutex registry;
// Runtime destruction requires quiescent callers, as for SGEMM. The registry
// serializes attachment; one lock protects this owner's plans and GPU work.
Attachment* attach(void* host, const prom_vk_runtime_services& services) {
    const std::lock_guard lock{registry};
    if (auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_fft_state(host))) return state;
    auto state = std::make_unique<Attachment>();
    state->executor = prom_concept_fft_create(host, &services);
    if (!state->executor) return nullptr;
    if (prom_reactor_runtime_set_concept_fft_state(host, state.get()) != PROM_OK) {
        if (prom_concept_fft_destroy(state->executor) != PROM_OK) std::terminate();
        return nullptr;
    }
    return state.release();
}
int execute(void* host, const void* input, void* output, uint64_t bytes,
    uint32_t count, uint32_t batches, uint32_t stride, uint32_t inverse, uint32_t normalize,
    bool fp64, uint32_t* stage, int* detail) {
    uint32_t localStage{}; int localDetail{};
    if (!stage) stage = &localStage;
    if (!detail) detail = &localDetail;
    *stage = PROM_STAGE_INIT; *detail = PROM_FFT_DETAIL_VULKAN_UNAVAILABLE;
    prom_vk_runtime_services services{};
    if (prom_reactor_runtime_get_vk_services(host, &services) != PROM_OK || !services.backend_available) return PROM_ERROR;
    PrometheusCaps caps{};
    if (prom_reactor_runtime_probe_impl(host, &caps) != PROM_OK ||
        caps.backend_type != PROM_BACKEND_VULKAN) return PROM_ERROR;
    if (fp64 && !services.shader_float64_enabled) { *detail = PROM_FFT_DETAIL_FLOAT64_UNAVAILABLE; return PROM_ERROR; }
    try {
        auto* state = attach(host, services);
        if (!state) { *detail = PROM_FFT_DETAIL_ALLOCATION_FAILED; return PROM_ERROR; }
        const std::lock_guard lock{state->mutex};
        if (fp64 && state->count != count) {
            std::vector<double> table(count);
            for (uint32_t i = 0; i < count / 2; ++i) {
                const double angle = -2.0 * std::numbers::pi * i / count;
                table[2 * i] = std::cos(angle); table[2 * i + 1] = std::sin(angle);
            }
            state->twiddles = std::move(table); state->count = count;
        }
        if (ConceptVkSetQueueSubmit(services.device, host, prom_vk_concept_queue_submit) != 0) return PROM_ERROR;
        const int code = prom_concept_fft_run(state->executor, input, output, state->twiddles.data(),
            bytes, count, batches, stride, inverse, normalize, fp64, services.test_flags);
        switch (code) {
        case 0: *detail = 0; *stage = PROM_STAGE_TRANSFER_OUT; return PROM_OK;
        case -801: *detail = PROM_FFT_DETAIL_ALLOCATION_FAILED; break;
        case -802: *detail = PROM_FFT_DETAIL_UPLOAD_FAILED; *stage = PROM_STAGE_TRANSFER_IN; break;
        case -804: *detail = PROM_FFT_DETAIL_READBACK_FAILED; *stage = PROM_STAGE_TRANSFER_OUT; break;
        case -805: *detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; break;
        default: *detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *stage = PROM_STAGE_SUBMIT; break;
        }
    } catch (const std::bad_alloc&) { *detail = PROM_FFT_DETAIL_ALLOCATION_FAILED; }
      catch (const std::system_error&) { *detail = PROM_FFT_DETAIL_VULKAN_UNAVAILABLE; }
    return PROM_ERROR;
}
}
extern "C" PipelineCreation PromFftLoadPipeline(void* host, VkDevice device, VkPipelineCache cache, uint32_t role) {
    PipelineCreation failed{}; failed.code = VK_ERROR_INITIALIZATION_FAILED;
    if (role > 2) return failed;
    const uint32_t kernel = role == 2 ? 70u : 52u + role;
    char variant[32]{};
    std::snprintf(variant, sizeof variant, "kernel-%u-default", kernel);
    prom_shader_package* package{};
    VkShaderModule shader{}; const char* entry{}; prom_shader_package_diagnostic diagnostic{};
    if (prom_reactor_runtime_get_shader_package(host, &package) != PROM_OK || !package ||
        !prom_shader_package_create_module(package, device, variant, &shader, &entry, &diagnostic)) return failed;
    const BindingSlot bindings[] = {{0, 0}, {1, 0}, {2, 0}};
    return ConceptVkCreatePipelineFromModule(device, cache, shader, entry,
        ConceptVkBindingSlots{bindings, role == 2 ? 3u : 2u}, role == 1 ? 32u : 16u);
}
extern "C" int prom_concept_fft_execute(void* host, const PrometheusFftRequest* r,
    uint64_t bytes, uint32_t* stage, int* detail) {
    return execute(host, r->input, r->output, bytes, r->element_count, r->batch_count,
        r->stride_elements ? r->stride_elements : r->element_count,
        (r->flags & PROM_FFT_FLAG_INVERSE) != 0, (r->flags & PROM_FFT_FLAG_INVERSE_NORMALIZE) != 0,
        false, stage, detail);
}
extern "C" int prom_reactor_runtime_fft64_impl(void* host, const double* input, double* output,
    uint32_t count, uint32_t* stage, int* detail) {
    if (!prom_reactor_runtime_validate_handle(host)) {
        prom_vk_set_status(stage, detail, PROM_STAGE_INIT, PROM_INVALID_HANDLE); return PROM_INVALID_HANDLE;
    }
    if (!input || !output || count == 0 || (count & (count - 1)) != 0 || count > (1u << 20)) {
        prom_vk_set_status(stage, detail, PROM_STAGE_INIT, PROM_FFT_DETAIL_INVALID_REQUEST); return PROM_ERROR;
    }
    return execute(host, input, output, uint64_t{count} * 16u, count, 1u, count, 0u, 0u, true, stage, detail);
}
extern "C" int prom_reactor_runtime_concept_fft_detach(void* host) {
    try {
        const std::lock_guard lock{registry};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_fft_state(host));
        if (!state) return PROM_OK;
        if (prom_concept_fft_destroy(state->executor) != PROM_OK) return PROM_ERROR;
        prom_reactor_runtime_set_concept_fft_state(host, nullptr);
        delete state;
        return PROM_OK;
    } catch (const std::system_error&) { return PROM_ERROR; }
}
extern "C" int prom_reactor_runtime_concept_fft_stats(void* host, uint64_t* allocations, uint64_t* reuses) {
    if (!allocations || !reuses || !prom_reactor_runtime_validate_handle(host)) return PROM_ERROR;
    try {
        const std::lock_guard lock{registry};
        auto* state = static_cast<Attachment*>(prom_reactor_runtime_concept_fft_state(host));
        if (!state) { *allocations = 0; *reuses = 0; return PROM_OK; }
        const std::lock_guard operation{state->mutex};
        prom_concept_fft_stats(state->executor, allocations, reuses);
        return PROM_OK;
    } catch (const std::system_error&) { return PROM_ERROR; }
}
