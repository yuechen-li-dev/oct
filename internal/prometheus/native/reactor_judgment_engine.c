#include "reactor_judgment_engine.h"

#include <stddef.h>

typedef struct prom_judgment_candidate {
  prom_vk_path_mode path;
  prom_vk_compute_mode compute;
} prom_judgment_candidate;

static int candidate_detail_code(prom_vk_path_mode path, prom_vk_compute_mode compute, uint32_t fallback_used) {
  if (compute == PROM_VK_COMPUTE_TILED) {
    if (path == PROM_VK_PATH_DIRECT) {
      return PROM_DETAIL_PATH_DIRECT_TILED;
    }
    if (path == PROM_VK_PATH_STAGED_UPLOAD) {
      return PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
    }
    return PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
  }

  if (path == PROM_VK_PATH_DIRECT) {
    return fallback_used != 0u ? PROM_DETAIL_PATH_FALLBACK_TO_DIRECT : PROM_DETAIL_PATH_DIRECT;
  }
  if (path == PROM_VK_PATH_STAGED_UPLOAD) {
    return PROM_DETAIL_PATH_STAGED_UPLOAD;
  }
  return PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK;
}

static uint32_t candidate_path_feasible(prom_vk_path_mode path, const prom_judgment_facts* facts) {
  if (path == PROM_VK_PATH_DIRECT) {
    return facts->can_direct;
  }
  return facts->can_stage;
}

static uint32_t path_matches_request(prom_vk_path_mode path, prom_vk_path_mode requested) {
  return path == requested ? 1u : 0u;
}

void prom_judgment_engine_select_sgemm_mode(const prom_judgment_facts* facts, prom_judgment_decision* out_decision) {
  static const prom_judgment_candidate k_candidates[] = {
      {PROM_VK_PATH_DIRECT, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_DIRECT, PROM_VK_COMPUTE_TILED},
      {PROM_VK_PATH_STAGED_UPLOAD, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_STAGED_UPLOAD, PROM_VK_COMPUTE_TILED},
      {PROM_VK_PATH_STAGED_UPLOAD_READBACK, PROM_VK_COMPUTE_BASELINE},
      {PROM_VK_PATH_STAGED_UPLOAD_READBACK, PROM_VK_COMPUTE_TILED},
  };
  const uint32_t candidate_count = (uint32_t)(sizeof(k_candidates) / sizeof(k_candidates[0]));
  uint32_t desired_tiled;
  prom_vk_path_mode requested_path;
  int best_score;
  uint32_t best_index;
  uint32_t best_fallback;
  uint32_t candidate_index;

  if (out_decision == NULL) {
    return;
  }

  out_decision->success = 0u;
  out_decision->error_detail = PROM_DETAIL_CAPABILITY_MISMATCH;
  out_decision->requested_path = PROM_VK_PATH_DIRECT;
  out_decision->selected_path = PROM_VK_PATH_DIRECT;
  out_decision->compute_mode = PROM_VK_COMPUTE_BASELINE;
  out_decision->final_detail = PROM_DETAIL_CAPABILITY_MISMATCH;
  out_decision->used_fallback_to_direct = 0u;
  out_decision->winning_candidate_index = 0u;
  out_decision->winning_score = -100000;

  if (facts == NULL) {
    return;
  }

  if (facts->force_direct != 0u) {
    requested_path = PROM_VK_PATH_DIRECT;
  } else if (facts->force_staged != 0u) {
    requested_path = facts->readback_required != 0u ? PROM_VK_PATH_STAGED_UPLOAD_READBACK : PROM_VK_PATH_STAGED_UPLOAD;
  } else if (facts->can_stage != 0u && facts->work_units >= (uint64_t)PROM_JUDGMENT_STAGING_WORK_THRESHOLD) {
    requested_path = facts->readback_required != 0u ? PROM_VK_PATH_STAGED_UPLOAD_READBACK : PROM_VK_PATH_STAGED_UPLOAD;
  } else {
    requested_path = PROM_VK_PATH_DIRECT;
  }
  out_decision->requested_path = requested_path;

  desired_tiled = (facts->force_tiled != 0u || facts->tiled_shape != 0u) ? 1u : 0u;

  best_score = -100000;
  best_index = 0u;
  best_fallback = 0u;

  for (candidate_index = 0u; candidate_index < candidate_count; ++candidate_index) {
    const prom_judgment_candidate candidate = k_candidates[candidate_index];
    int score = 0;
    uint32_t fallback_used = 0u;

    if (candidate.path == PROM_VK_PATH_STAGED_UPLOAD && facts->readback_required != 0u) {
      continue;
    }
    if (candidate.path == PROM_VK_PATH_STAGED_UPLOAD_READBACK && facts->readback_required == 0u) {
      continue;
    }
    if ((requested_path == PROM_VK_PATH_STAGED_UPLOAD || requested_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) &&
        facts->can_stage == 0u && candidate.path == PROM_VK_PATH_DIRECT) {
      continue;
    }

    if (candidate_path_feasible(candidate.path, facts) == 0u) {
      if (candidate.path == PROM_VK_PATH_DIRECT) {
        continue;
      }
      if (facts->allow_fallback == 0u || facts->can_direct == 0u) {
        continue;
      }
      fallback_used = 1u;
    }

    if (fallback_used != 0u) {
      score += 200;
    } else {
      score += path_matches_request(candidate.path, requested_path) != 0u ? 500 : 0;
    }

    if (candidate.compute == PROM_VK_COMPUTE_TILED) {
      if (desired_tiled == 0u) {
        continue;
      }
      score += 20;
    } else if (desired_tiled == 0u) {
      score += 20;
    }

    if (candidate.path == PROM_VK_PATH_DIRECT) {
      score += 5;
    } else {
      score += 10;
      if (facts->software_vulkan != 0u) {
        score -= 2;
      }
    }

    if (score > best_score) {
      best_score = score;
      best_index = candidate_index;
      best_fallback = fallback_used;
    }
  }

  if (best_score <= -100000) {
    return;
  }

  out_decision->success = 1u;
  out_decision->error_detail = 0;
  out_decision->winning_candidate_index = best_index;
  out_decision->winning_score = best_score;
  out_decision->used_fallback_to_direct = best_fallback;
  out_decision->compute_mode = k_candidates[best_index].compute;
  out_decision->selected_path = best_fallback != 0u ? PROM_VK_PATH_DIRECT : k_candidates[best_index].path;
  out_decision->final_detail =
      candidate_detail_code(out_decision->selected_path, out_decision->compute_mode, out_decision->used_fallback_to_direct);
}
