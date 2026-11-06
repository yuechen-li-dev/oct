#ifndef OCT_REACTOR_CONCEPT_SGEMM_H
#define OCT_REACTOR_CONCEPT_SGEMM_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
typedef struct prom_sgemm_range { VkBuffer buffer; uint64_t bytes; } prom_sgemm_range;
typedef struct prom_sgemm_shader {
    VkPipeline pipeline;
    VkPipelineLayout layout;
    VkDescriptorSet descriptors;
    VkQueryPool queries;
    uint32_t queryBase, timed;
} prom_sgemm_shader;
typedef struct prom_sgemm_job {
    prom_sgemm_range buffers[7];
    prom_sgemm_shader shader;
    uint32_t m, n, k, x, y, z, dispatches, path, dedicated, computeFamily, transferFamily, uploadMask, readback, perturbWord;
} prom_sgemm_job;
void* prom_concept_sgemm_create(const prom_vk_runtime_services* services);
int prom_concept_sgemm_destroy(void* owner);
int prom_concept_sgemm_record(void* owner, uint32_t slot, const prom_sgemm_job* job);
int prom_concept_sgemm_submit(void* owner, uint32_t slot);
int prom_concept_sgemm_wait(void* owner, uint32_t slot);
int prom_concept_sgemm_poll(void* owner, uint32_t slot);
int prom_concept_sgemm_timings(void* owner, uint32_t slot, uint64_t* record, uint64_t* submit);
#ifndef PROM_SGEMM_LAYOUT_TRANSPORT
uint64_t PromSgemmNow(void);
void PromSgemmBind(VkCommandBuffer commands, prom_sgemm_shader shader, uint32_t m, uint32_t n, uint32_t k);
void PromSgemmDispatch(VkCommandBuffer commands, uint32_t x, uint32_t y, uint32_t z);
void PromSgemmTimestamp(VkCommandBuffer commands, prom_sgemm_shader shader, uint32_t end);
void PromSgemmOwnership(VkCommandBuffer commands, prom_sgemm_job job, uint32_t acquire);
#endif
#ifdef __cplusplus
}
#endif
#endif
