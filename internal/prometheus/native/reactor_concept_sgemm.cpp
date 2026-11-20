#include "reactor_concept_sgemm.h"

// Driver details for borrowed package instances. The Concept reactor determines
// command order, hazards, repetitions and the lifetime of every submission.
extern "C" void PromSgemmBind(VkCommandBuffer commands, prom_sgemm_shader shader,
    uint32_t m, uint32_t n, uint32_t k) {
    vkCmdBindPipeline(commands, VK_PIPELINE_BIND_POINT_COMPUTE, shader.pipeline);
    vkCmdBindDescriptorSets(commands, VK_PIPELINE_BIND_POINT_COMPUTE, shader.layout,
        0, 1, &shader.descriptors, 0, nullptr);
    const uint32_t dimensions[] = {m, n, k};
    vkCmdPushConstants(commands, shader.layout, VK_SHADER_STAGE_COMPUTE_BIT, 0,
        sizeof dimensions, dimensions);
}
extern "C" void PromSgemmDispatch(VkCommandBuffer commands, uint32_t x, uint32_t y, uint32_t z) {
    vkCmdDispatch(commands, x, y, z);
}
extern "C" void PromSgemmTimestamp(VkCommandBuffer commands, prom_sgemm_shader shader, uint32_t end) {
    if (!shader.timed) return;
    if (!end) vkCmdResetQueryPool(commands, shader.queries, shader.queryBase, shader.timed == 2 ? 4 : 2);
    const auto stage = shader.timed == 2 && end == 0 ? VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT
        : shader.timed == 2 && end == 3 ? VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    vkCmdWriteTimestamp(commands, stage, shader.queries, shader.queryBase + end);
}
