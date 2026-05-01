#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_FILTER_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_FILTER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define PROM_DOMINATUS_FILTER_MAX_WINDOW 9u

typedef enum prom_dominatus_filter_kind {
  PROM_DOM_FILTER_KIND_NONE = 0,
  PROM_DOM_FILTER_KIND_EMA = 1,
  PROM_DOM_FILTER_KIND_MEDIAN = 2,
  PROM_DOM_FILTER_KIND_HYSTERESIS = 3,
  PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA = 4,
} prom_dominatus_filter_kind;

typedef struct prom_dominatus_filter_params {
  prom_dominatus_filter_kind kind;
  double alpha;
  double band;
  uint32_t window;
} prom_dominatus_filter_params;

typedef struct prom_dominatus_filter_output {
  double estimate;
  double residual;
  double stability_proxy;
  uint32_t sample_count;
  uint32_t warmup;
  uint32_t updated;
  uint32_t held;
  uint32_t valid;
} prom_dominatus_filter_output;

typedef struct prom_dominatus_filter_state {
  prom_dominatus_filter_kind kind;
  prom_dominatus_filter_params params;
  uint32_t initialized;
  uint32_t sample_count;
  uint64_t last_update_tick;
  double estimate;
  double last_residual;
  double stability_proxy;
  uint32_t update_count;
  uint32_t hold_count;
  double window_buffer[PROM_DOMINATUS_FILTER_MAX_WINDOW];
  uint32_t window_count;
  uint32_t window_index;
} prom_dominatus_filter_state;

void prom_dominatus_filter_init(prom_dominatus_filter_state* state, const prom_dominatus_filter_params* params);
void prom_dominatus_filter_warm_start(prom_dominatus_filter_state* state,
                                      const prom_dominatus_filter_params* params,
                                      double prior_estimate);
void prom_dominatus_filter_reset(prom_dominatus_filter_state* state);
prom_dominatus_filter_output prom_dominatus_filter_update(prom_dominatus_filter_state* state,
                                                           double measurement,
                                                           uint64_t tick);

prom_dominatus_filter_params prom_dominatus_filter_params_ema(double alpha);
prom_dominatus_filter_params prom_dominatus_filter_params_median(uint32_t window);
prom_dominatus_filter_params prom_dominatus_filter_params_hysteresis(double band);
prom_dominatus_filter_params prom_dominatus_filter_params_hybrid_median_ema(uint32_t window, double alpha);

#ifdef __cplusplus
}
#endif

#endif
