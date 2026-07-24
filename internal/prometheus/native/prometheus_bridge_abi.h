#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_PROMETHEUS_BRIDGE_ABI_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_PROMETHEUS_BRIDGE_ABI_H

/*
 * The public reactor_api.h declarations are the ABI authority.  This header
 * contains only the dynamic-loader projection used by the Go bridge: function
 * pointer types and aliases to public request/result layouts.  It must not
 * redeclare an ABI struct or numeric constant.
 */
#include "reactor_api.h"

#if defined(_WIN32)
#define OCT_PROM_CALL __cdecl
#else
#define OCT_PROM_CALL
#endif

typedef uint32_t (OCT_PROM_CALL *oct_prom_abi_fn)(void);
typedef int (OCT_PROM_CALL *oct_prom_create_fn)(void*, void**);
typedef int (OCT_PROM_CALL *oct_prom_destroy_fn)(void*);
typedef int (OCT_PROM_CALL *oct_prom_probe_fn)(void*, PrometheusCaps*);
typedef int (OCT_PROM_CALL *oct_prom_sgemm_fn)(
    void*, const float*, const float*, float*, uint32_t, uint32_t, uint32_t, uint32_t*, int*);
typedef int (OCT_PROM_CALL *oct_prom_submit_async_fn)(
    void*, const float*, const float*, uint32_t, uint32_t, uint32_t, int*, uint32_t*, int*);
typedef int (OCT_PROM_CALL *oct_prom_query_async_fn)(void*, int, PrometheusAsyncStatus*);
typedef int (OCT_PROM_CALL *oct_prom_consume_async_fn)(
    void*, int, float*, uint32_t, uint32_t*, int*);
typedef int (OCT_PROM_CALL *oct_prom_abandon_async_fn)(void*, int);
typedef int (OCT_PROM_CALL *oct_prom_gemma4e2b_input_rmsnorm_fn)(
    void*, const PrometheusGemma4E2BM1InputRmsNormRequest*,
    PrometheusGemma4E2BM1InputRmsNormResult*);
typedef int (OCT_PROM_CALL *oct_prom_gemma4e2b_rope_fn)(
    void*, const PrometheusGemma4E2BM1RopeRequest*, PrometheusGemma4E2BM1RopeResult*);
typedef int (OCT_PROM_CALL *oct_prom_gemma4e2b_head_rmsnorm_rope_fn)(
    void*, const PrometheusGemma4E2BM1HeadRmsNormRopeRequest*,
    PrometheusGemma4E2BM1HeadRmsNormRopeResult*);
typedef int (OCT_PROM_CALL *oct_prom_gemma4e2b_attention_scores_fn)(
    void*, const PrometheusGemma4E2BM1AttentionScoresRequest*,
    PrometheusGemma4E2BM1AttentionScoresResult*);

/* Host bridge names are aliases, never second struct declarations. */
typedef PrometheusCaps oct_prom_caps;
typedef PrometheusReactorConfig oct_prom_cfg;
typedef PrometheusAsyncStatus oct_prom_async_status;
typedef PrometheusGemma4E2BM1InputRmsNormRequest oct_prom_gemma4e2b_input_rmsnorm_request;
typedef PrometheusGemma4E2BM1InputRmsNormResult oct_prom_gemma4e2b_input_rmsnorm_result;
typedef PrometheusGemma4E2BM1RopeRequest oct_prom_gemma4e2b_rope_request;
typedef PrometheusGemma4E2BM1RopeResult oct_prom_gemma4e2b_rope_result;
typedef PrometheusGemma4E2BM1HeadRmsNormRopeRequest oct_prom_gemma4e2b_head_rmsnorm_rope_request;
typedef PrometheusGemma4E2BM1HeadRmsNormRopeResult oct_prom_gemma4e2b_head_rmsnorm_rope_result;

#undef OCT_PROM_CALL

#endif
