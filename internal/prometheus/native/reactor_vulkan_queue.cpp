// All Prometheus operation families share this host synchronization boundary.
// It protects the brief queue API call, never the GPU completion wait. Device
// teardown uses the same boundary and requires callers to stop admitting work.
#include "reactor_vulkan.h"
#include <mutex>

// One callback identity per foreign device owner, shared by all Concept
// attachments. The Vulkan library rejects conflicting submit authorities.
extern "C" int32_t prom_vk_concept_queue_submit(void*, VkQueue queue, VkCommandBuffer commands, VkFence fence) {
    VkSubmitInfo info{VK_STRUCTURE_TYPE_SUBMIT_INFO};
    info.commandBufferCount = 1u; info.pCommandBuffers = &commands;
    return prom_vk_queue_submit(queue, 1u, &info, fence);
}
extern "C" int32_t prom_vk_concept_queue_submit_sync(void *, VkQueue queue,
                                                     VkCommandBuffer commands, VkFence fence,
                                                     VkSemaphore wait, VkSemaphore signal)
{
    VkSubmitInfo info{VK_STRUCTURE_TYPE_SUBMIT_INFO};
    VkPipelineStageFlags stage = VK_PIPELINE_STAGE_ALL_COMMANDS_BIT;
    info.commandBufferCount = 1;
    info.pCommandBuffers = &commands;
    if (wait)
    {
        info.waitSemaphoreCount = 1;
        info.pWaitSemaphores = &wait;
        info.pWaitDstStageMask = &stage;
    }
    if (signal)
    {
        info.signalSemaphoreCount = 1;
        info.pSignalSemaphores = &signal;
    }
    return prom_vk_queue_submit(queue, 1, &info, fence);
}
#include <system_error>

namespace { std::mutex queue_api; }

extern "C" VkResult prom_vk_queue_submit(VkQueue queue, uint32_t count,
    const VkSubmitInfo* submits, VkFence fence) {
    try {
        const std::lock_guard lock{queue_api};
        return vkQueueSubmit(queue, count, submits, fence);
    } catch (const std::system_error&) { return VK_ERROR_UNKNOWN; }
}

extern "C" VkResult prom_vk_device_wait_idle(VkDevice device) {
    try {
        const std::lock_guard lock{queue_api};
        return vkDeviceWaitIdle(device);
    } catch (const std::system_error&) { return VK_ERROR_UNKNOWN; }
}
