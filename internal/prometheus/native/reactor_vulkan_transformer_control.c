#include "reactor_vulkan_transformer_control.h"

#include <math.h>
#include <string.h>

static uint64_t prom_m49b_hash_u64(uint64_t hash, uint64_t value) {
  uint32_t byte_index;
  for (byte_index = 0u; byte_index < 8u; ++byte_index) {
    hash ^= (value >> (byte_index * 8u)) & 0xffu;
    hash *= 1099511628211ull;
  }
  return hash;
}

uint32_t prom_m49b_num_path(uint32_t projection_path) {
  if (projection_path == PROM_M47_PROJECTION_A2X4_FP32)
    return PROM_NUM_PATH_GPU_A2X4_FP32;
  if (projection_path == PROM_M47_PROJECTION_COOPERATIVE)
    return PROM_NUM_PATH_GPU_COOPERATIVE_FP16;
  return PROM_NUM_PATH_GPU_CONVENTIONAL_FP16;
}

uint64_t prom_m49b_shape_identity(const prom_m48_stack_request* request) {
  uint64_t hash = 1469598103934665603ull;
  if (request == NULL) return 0u;
  hash = prom_m49b_hash_u64(hash, request->tokens);
  hash = prom_m49b_hash_u64(hash, request->model_width);
  hash = prom_m49b_hash_u64(hash, request->head_count);
  hash = prom_m49b_hash_u64(hash, request->head_dim);
  return prom_m49b_hash_u64(hash, request->ffn_width);
}

uint32_t prom_m49b_canary_due(const prom_num_m49b_controller* controller,
                              uint64_t execution_index) {
  if (controller == NULL || controller->parameters.canary_interval == 0u) return 0u;
  return controller->state == PROM_NUM_M49B_UNIDENTIFIED ||
                 controller->state == PROM_NUM_M49B_REFERENCE_SUSPECT ||
                 controller->state == PROM_NUM_M49B_QUARANTINED ||
                 execution_index % controller->parameters.canary_interval == 0u
             ? 1u : 0u;
}

int prom_m49b_estimate_paired_discrepancy(
    const float* selected_samples, const float* witness_samples,
    uint32_t sample_count, uint64_t shape_identity,
    uint64_t selected_replay_identity, uint64_t witness_replay_identity,
    uint64_t parameter_generation, prom_m49b_paired_estimate* out_estimate) {
  uint64_t hash = 1469598103934665603ull;
  double squared_sum = 0.0;
  uint32_t index;
  if (selected_samples == NULL || witness_samples == NULL || out_estimate == NULL ||
      sample_count < 4u || sample_count > PROM_NUM_M49B_MAX_SAMPLES) return 0;
  memset(out_estimate, 0, sizeof(*out_estimate));
  /* A paired witness has to name both executions and the pinned parameter
     generation.  Different replay identities are expected: their paths are
     deliberately different, while shape/weights/initial activation are shared
     by the fixed-stack invocation. */
  if (shape_identity == 0u || selected_replay_identity == 0u ||
      witness_replay_identity == 0u || parameter_generation == 0u) {
    out_estimate->reference_suspect = 1u;
    return 1;
  }
  out_estimate->finite_agreement = 1u;
  for (index = 0u; index < sample_count; ++index) {
    const float selected = selected_samples[index];
    const float witness = witness_samples[index];
    const float delta = selected - witness;
    const double absolute_delta = fabs((double)delta);
    if (!isfinite(selected) || !isfinite(witness) || !isfinite(delta)) {
      out_estimate->finite_agreement = 0u;
      out_estimate->reference_suspect = 1u;
    }
    out_estimate->sample_delta[index] = delta;
    squared_sum += (double)delta * (double)delta;
    if (absolute_delta > out_estimate->sampled_linf_error)
      out_estimate->sampled_linf_error = absolute_delta;
    out_estimate->signed_projection_a_delta +=
        (index & 1u) == 0u ? (double)delta : -(double)delta;
    out_estimate->signed_projection_b_delta +=
        ((index * 7u + 3u) & 1u) == 0u ? (double)delta : -(double)delta;
    out_estimate->absolute_projection_delta += absolute_delta;
    if (delta != 0.0f) out_estimate->coordinate_disagreement_count += 1u;
    hash = prom_m49b_hash_u64(hash, (uint64_t)(uint32_t)index);
    {
      uint32_t bits = 0u;
      memcpy(&bits, &delta, sizeof(bits));
      hash = prom_m49b_hash_u64(hash, bits);
    }
  }
  out_estimate->sampled_l2_error = sqrt(squared_sum);
  /* Paired output evidence does not estimate an input-disturbance gain.  A
     neutral, explicit gain is safer than inventing a learned amplification. */
  out_estimate->estimated_gain = 1.0;
  hash = prom_m49b_hash_u64(hash, shape_identity);
  hash = prom_m49b_hash_u64(hash, selected_replay_identity);
  hash = prom_m49b_hash_u64(hash, witness_replay_identity);
  hash = prom_m49b_hash_u64(hash, parameter_generation);
  out_estimate->paired_identity = hash;
  if (out_estimate->finite_agreement != 0u && out_estimate->reference_suspect == 0u) {
    /* Authored conservative confidence: complete 16-coordinate, same-request
       paired evidence clears the 0.75 default floor but leaves room for the
       later DVT calibration campaign.  Reduced coverage is proportionally
       discounted; stale/missing identities above never receive confidence. */
    out_estimate->confidence = 0.80 * (double)sample_count /
                               (double)PROM_NUM_M49B_MAX_SAMPLES;
  }
  return 1;
}

void prom_m49b_apply_fixed_stack_policy(
    prom_num_m49b_controller* controller, prom_m48_stack_request* request,
    uint64_t execution_identity) {
  uint32_t layer;
  uint32_t apply_checkpoint = 0u;
  uint32_t apply_fallback = 0u;
  if (controller == NULL || request == NULL || controller->parameters.rollout_stage < 2u)
    return;
  if (controller->state == PROM_NUM_M49B_FALLBACK_RECOMMENDED &&
      controller->parameters.rollout_stage == 3u && controller->cooldown_remaining != 0u)
    apply_fallback = 1u;
  else if (controller->state == PROM_NUM_M49B_CHECKPOINT_RECOMMENDED ||
           (controller->state == PROM_NUM_M49B_FALLBACK_RECOMMENDED &&
            controller->parameters.rollout_stage == 3u &&
            controller->cooldown_remaining == 0u))
    apply_checkpoint = 1u;
  if (apply_checkpoint == 0u && apply_fallback == 0u) return;
  request->numerical_control_mode = PROM_M48_NUMERICAL_CONTROL_M49B;
  request->controller_parameter_generation = controller->parameter_generation;
  request->controller_execution_identity = execution_identity;
  for (layer = 0u; layer < request->layer_count; ++layer) {
    if (apply_fallback != 0u ||
        (layer + 1u) % controller->parameters.checkpoint_interval == 0u)
      request->controller_layer_projection_path[layer] = PROM_M47_PROJECTION_A2X4_FP32;
  }
}

void prom_m49b_quarantine_execution(prom_num_m49b_controller* controller,
                                    const prom_m48_stack_request* request,
                                    uint64_t execution_identity) {
  const float zeros[4] = {0.0f, 0.0f, 0.0f, 0.0f};
  prom_num_m49b_observation observation;
  prom_num_m49b_evidence evidence;
  prom_num_m49b_decision decision;
  if (controller == NULL || request == NULL) return;
  memset(&observation, 0, sizeof(observation));
  observation.completion_known = 0u;
  observation.lifecycle_fault = 1u;
  observation.current_path = prom_m49b_num_path(request->projection_path);
  observation.execution_index = execution_identity;
  observation.shape_identity = prom_m49b_shape_identity(request);
  observation.sampled_values = zeros;
  observation.sampled_value_count = 4u;
  observation.confidence = 0.0;
  (void)prom_num_m49b_observe(controller, &observation, &evidence, &decision);
}
