#ifndef PROMETHEUS_CONCEPT_MODEL_BLOCK_H
#define PROMETHEUS_CONCEPT_MODEL_BLOCK_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
struct prom_model_block_state;
struct prom_model_block_weight_resource;
struct prom_reduction_runtime_state;
struct prom_compiled_model_session_state;
int prom_concept_session_create(void* host, const prom_vk_runtime_services* services,
    struct prom_compiled_model_session_state* session, uint64_t image, uint64_t context, uint64_t joint, uint32_t fault);
int prom_concept_session_destroy(struct prom_compiled_model_session_state* session);
void prom_concept_session_sync(struct prom_compiled_model_session_state* session);
int prom_concept_session_drain(struct prom_compiled_model_session_state* session);
int prom_concept_session_capture(struct prom_compiled_model_session_state* session,
    uint32_t role, const prom_vk_buffer* source, uint64_t producer, uint64_t output);
int prom_concept_session_compose(struct prom_compiled_model_session_state* session, uint64_t image, uint64_t context);
int prom_concept_session_joint_begin(struct prom_compiled_model_session_state* session, uint64_t image, uint64_t context, uint64_t joint);
int prom_concept_session_joint_end(struct prom_compiled_model_session_state* session, int commit, int uncertain, uint64_t producer, uint64_t output);
int prom_concept_session_joint_borrowed(struct prom_compiled_model_session_state* session);
int prom_concept_session_reset(struct prom_compiled_model_session_state* session);
void prom_concept_session_inject(struct prom_compiled_model_session_state* session, uint32_t flags);
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
int prom_concept_block_context_pipelines(struct prom_model_block_state* block);
int prom_concept_block_main_pipelines(struct prom_model_block_state* block, uint32_t attention, uint32_t groups);
int prom_concept_block_main_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, const prom_vk_buffer* joint, uint32_t chain, int32_t* detail);
/* Borrowed from completed Concept work until the next drain/run. */
VkQueryPool prom_concept_block_queries(struct prom_model_block_state* block);
int prom_concept_block_context_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t resident, int32_t* detail);
/* Record one stage into a legacy-owned command buffer. The caller owns the
   surrounding synchronization, submission, completion, and recording lifetime. */
void prom_concept_block_record_stage(struct prom_model_block_state* block, VkCommandBuffer commands, uint32_t role);
int prom_concept_block_run(struct prom_reduction_runtime_state* state, struct prom_model_block_state* block,
    uint32_t mode, uint32_t elements, uint32_t stage, uint32_t resident, int32_t* detail);
int prom_concept_block_attention_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t stage, int capture, int32_t* detail);
int prom_concept_block_ffn_run(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t stage, int capture, int32_t* detail);
/* Synchronous weight lifecycle. Native packets and staging bytes borrow the
   Concept owner; only a completed binding commit changes its generation. */
int prom_concept_weights_begin(struct prom_model_block_state* block, uint32_t mode,
    const uint32_t* physical_slots, const PrometheusModelBlockWeightUpload* uploads, uint32_t count);
int prom_concept_weights_upload(struct prom_reduction_runtime_state* state,
    struct prom_model_block_state* block, uint32_t index);
void prom_concept_weights_abort(struct prom_model_block_state* block);
void prom_concept_weights_views(struct prom_model_block_state* block, int candidate,
    struct prom_model_block_weight_resource* views);
void prom_concept_weights_commit(struct prom_model_block_state* block);
void prom_concept_weights_external(struct prom_model_block_state* block);
int prom_concept_block_configure(void* host, VkDevice device);
#ifdef __cplusplus
}
#endif
#endif
