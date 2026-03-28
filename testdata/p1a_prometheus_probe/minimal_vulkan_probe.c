#include <stdio.h>
#include <stdlib.h>
#include <vulkan/vulkan.h>

int main(void) {
    VkApplicationInfo appInfo = {
        .sType = VK_STRUCTURE_TYPE_APPLICATION_INFO,
        .pApplicationName = "p1a-probe",
        .applicationVersion = VK_MAKE_VERSION(1, 0, 0),
        .pEngineName = "none",
        .engineVersion = VK_MAKE_VERSION(1, 0, 0),
        .apiVersion = VK_API_VERSION_1_0,
    };

    VkInstanceCreateInfo createInfo = {
        .sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO,
        .pApplicationInfo = &appInfo,
    };

    VkInstance instance = VK_NULL_HANDLE;
    VkResult r = vkCreateInstance(&createInfo, NULL, &instance);
    if (r != VK_SUCCESS) {
        fprintf(stderr, "vkCreateInstance failed: %d\n", r);
        return 2;
    }

    uint32_t deviceCount = 0;
    r = vkEnumeratePhysicalDevices(instance, &deviceCount, NULL);
    if (r != VK_SUCCESS) {
        fprintf(stderr, "vkEnumeratePhysicalDevices(count) failed: %d\n", r);
        vkDestroyInstance(instance, NULL);
        return 3;
    }

    printf("vkCreateInstance: success\n");
    printf("physical_device_count: %u\n", deviceCount);

    if (deviceCount > 0) {
        VkPhysicalDevice* devices = (VkPhysicalDevice*)malloc(sizeof(VkPhysicalDevice) * deviceCount);
        if (!devices) {
            fprintf(stderr, "malloc failed\n");
            vkDestroyInstance(instance, NULL);
            return 4;
        }

        r = vkEnumeratePhysicalDevices(instance, &deviceCount, devices);
        if (r != VK_SUCCESS) {
            fprintf(stderr, "vkEnumeratePhysicalDevices(list) failed: %d\n", r);
            free(devices);
            vkDestroyInstance(instance, NULL);
            return 5;
        }

        for (uint32_t i = 0; i < deviceCount; i++) {
            VkPhysicalDeviceProperties props;
            vkGetPhysicalDeviceProperties(devices[i], &props);
            printf("device[%u]: %s (api %u.%u.%u)\n",
                   i,
                   props.deviceName,
                   VK_VERSION_MAJOR(props.apiVersion),
                   VK_VERSION_MINOR(props.apiVersion),
                   VK_VERSION_PATCH(props.apiVersion));
        }

        free(devices);
    }

    vkDestroyInstance(instance, NULL);
    return 0;
}
