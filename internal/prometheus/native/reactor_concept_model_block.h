#ifndef PROMETHEUS_CONCEPT_MODEL_BLOCK_H
#define PROMETHEUS_CONCEPT_MODEL_BLOCK_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
struct prom_model_block_state;
struct prom_reduction_runtime_state;
/* C11 transport for Concept-owned resources; every prom_vk_buffer returned is
   a borrow. Host calls on a model owner are externally serialized. */
void* prom_concept_block_create(void* host, const prom_vk_runtime_services* services);
int prom_concept_block_allocate(void* owner, uint64_t bytes, uint32_t placement, prom_vk_buffer* view);
int prom_concept_block_release(void* owner, VkBuffer buffer);
int prom_concept_block_destroy(void* owner);
int prom_concept_block_drain(struct prom_model_block_state* block);
int prom_concept_block_pipelines(struct prom_model_block_state* block);
int prom_concept_block_identity_pipeline(struct prom_model_block_state* block);
int prom_concept_block_attention_pipelines(struct prom_model_block_state* block);
int prom_concept_block_ffn_pipelines(struct prom_model_block_state* block);
/* Record one stage into a legacy-owned command buffer. The caller owns the
   surrounding synchronization, submission, completion, and recording lifetime. */
void prom_concept_block_record_stage(struct prom_model_block_state* block, VkCommandBuffer commands, uint32_t role);
int prom_concept_block_run(struct prom_reduction_runtime_state* state, struct prom_model_block_state* block,
    uint32_t mode, uint32_t elements, uint32_t stage, uint32_t resident, int32_t* detail);
int prom_concept_block_attention_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t stage, int capture, int32_t* detail);
int prom_concept_block_ffn_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t stage, int capture, int32_t* detail);
int prom_concept_block_configure(void* host, VkDevice device);
#ifdef __cplusplus
}
#endif
#endif
