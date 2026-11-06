#include "reactor_concept_model_block.h"
#include "reactor_shader_package.h"
#include "concept_vulkan.h"
#include <array>
#include <cstdio>

// Package validation and Vk ABI adaptation only. Allocation and execution
// lifetime belong to Concept. This boundary performs no throwing operations.
extern "C" int prom_concept_block_configure(void* host, VkDevice device) {
    return ConceptVkSetQueueSubmit(device, host, prom_vk_concept_queue_submit);
}
extern "C" PipelineCreation PromBlockLoadPipeline(void* host, VkDevice device,
    VkPipelineCache cache, uint32_t kernel, uint32_t bindingCount, uint32_t pushBytes) {
    PipelineCreation failed{}; failed.code = VK_ERROR_INITIALIZATION_FAILED;
    const auto* asset = prom_shader_registry_find_shader(kernel);
    if (!asset || asset->authority != PROM_SHADER_AUTHORITY_PRODUCTION ||
        asset->source_language != PROM_SHADER_SOURCE_SDSLV || asset->stage != PROM_SHADER_STAGE_COMPUTE ||
        !bindingCount || bindingCount > 8) return failed;
    prom_shader_package* package{};
    if (prom_reactor_runtime_get_shader_package(host, &package) != PROM_OK || !package) return failed;
    char variant[32]{}; std::snprintf(variant, sizeof variant, "kernel-%u-default", kernel);
    VkShaderModule shader{}; const char* entry{}; prom_shader_package_diagnostic diagnostic{};
    if (!prom_shader_package_validate_interface(package, variant, bindingCount, pushBytes, &diagnostic)) return failed;
    if (!prom_shader_package_create_module(package, device, variant, &shader, &entry, &diagnostic)) return failed;
    std::array<BindingSlot, 8> bindings{};
    for (uint32_t index = 0; index < bindingCount; ++index) bindings[index] = {index, 0};
    return ConceptVkCreatePipelineFromModule(device, cache, shader, entry,
        ConceptVkBindingSlots{bindings.data(), bindingCount}, pushBytes);
}
