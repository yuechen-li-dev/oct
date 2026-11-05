// All Prometheus operation families share this host synchronization boundary.
// It protects the brief queue API call, never the GPU completion wait. Device
// teardown uses the same boundary and requires callers to stop admitting work.
#include "reactor_vulkan.h"
#include <mutex>
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
