#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_TRANSFORMER_CONTROL_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_TRANSFORMER_CONTROL_H

#include "reactor_vulkan.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Pure M49b-to-fixed-stack adaptation.  Vulkan recording and slot-owned
   buffers remain in the stack owner; this unit owns no Vulkan resources. */
typedef struct prom_m49b_paired_estimate {
  float sample_delta[PROM_NUM_M49B_MAX_SAMPLES];
  double sampled_l2_error;
  double sampled_linf_error;
  double signed_projection_a_delta;
  double signed_projection_b_delta;
  double absolute_projection_delta;
  double estimated_gain;
  double confidence;
  uint64_t paired_identity;
  uint32_t finite_agreement;
  uint32_t coordinate_disagreement_count;
  uint32_t reference_suspect;
} prom_m49b_paired_estimate;

uint32_t prom_m49b_num_path(uint32_t projection_path);
uint64_t prom_m49b_shape_identity(const prom_m48_stack_request* request);
uint32_t prom_m49b_canary_due(const prom_num_m49b_controller* controller,
                              uint64_t execution_index);
int prom_m49b_estimate_paired_discrepancy(
    const float* selected_samples, const float* witness_samples,
    uint32_t sample_count, uint64_t shape_identity,
    uint64_t selected_replay_identity, uint64_t witness_replay_identity,
    uint64_t parameter_generation, prom_m49b_paired_estimate* out_estimate);
void prom_m49b_apply_fixed_stack_policy(prom_num_m49b_controller* controller,
                                        prom_m48_stack_request* request,
                                        uint64_t execution_identity);
void prom_m49b_quarantine_execution(prom_num_m49b_controller* controller,
                                    const prom_m48_stack_request* request,
                                    uint64_t execution_identity);

#ifdef __cplusplus
}
#endif

#endif
