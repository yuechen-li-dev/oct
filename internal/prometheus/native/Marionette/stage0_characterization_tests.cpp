#include "test_harness.h"

#include "../reactor_api.h"
#include "../prometheus_bridge_abi.h"
#include "../reactor_vulkan.h"

#include <cstddef>
#include <cstdint>
#include <type_traits>

namespace
{
constexpr std::size_t kInputRequestSize = 112u;
constexpr std::size_t kInputResultSize = 160u;
constexpr std::size_t kRopeRequestSize = 104u;
constexpr std::size_t kRopeResultSize = 96u;
constexpr std::size_t kHeadRmsNormRopeRequestSize = 144u;
constexpr std::size_t kHeadRmsNormRopeResultSize = 144u;
constexpr std::size_t kAttentionScoresRequestSize = 208u;
constexpr std::size_t kAttentionScoresResultSize = 136u;

static_assert(sizeof(PrometheusGemma4E2BM1InputRmsNormRequest) == kInputRequestSize);
static_assert(sizeof(PrometheusGemma4E2BM1InputRmsNormResult) == kInputResultSize);
static_assert(sizeof(PrometheusGemma4E2BM1RopeRequest) == kRopeRequestSize);
static_assert(sizeof(PrometheusGemma4E2BM1RopeResult) == kRopeResultSize);
static_assert(sizeof(PrometheusGemma4E2BM1HeadRmsNormRopeRequest) == kHeadRmsNormRopeRequestSize);
static_assert(sizeof(PrometheusGemma4E2BM1HeadRmsNormRopeResult) == kHeadRmsNormRopeResultSize);
static_assert(sizeof(PrometheusGemma4E2BM1AttentionScoresRequest) == kAttentionScoresRequestSize);
static_assert(sizeof(PrometheusGemma4E2BM1AttentionScoresResult) == kAttentionScoresResultSize);

static_assert(offsetof(PrometheusGemma4E2BM1AttentionScoresRequest, preparation_order) == 144u);
static_assert(offsetof(PrometheusGemma4E2BM1AttentionScoresResult, query_slot_id) == 32u);
static_assert(offsetof(PrometheusGemma4E2BM1AttentionScoresResult, score_byte_range) == 72u);
static_assert(offsetof(PrometheusGemma4E2BM1AttentionScoresResult, observed_weight_generation) == 120u);
static_assert(offsetof(PrometheusGemma4E2BM1AttentionScoresResult, requested_weight_generation) == 128u);

static_assert(sizeof(oct_prom_caps) == sizeof(PrometheusCaps));
static_assert(sizeof(oct_prom_cfg) == sizeof(PrometheusReactorConfig));
static_assert(sizeof(oct_prom_async_status) == sizeof(PrometheusAsyncStatus));
static_assert(offsetof(PrometheusReactorConfig, shader_package_root) == 32u);
static_assert(offsetof(PrometheusAsyncStatus, outstanding_tasks) == 24u);
static_assert(std::is_same<oct_prom_probe_fn, decltype(&prometheus_reactor_runtime_probe)>::value);
static_assert(std::is_same<oct_prom_query_async_fn,
                          decltype(&prometheus_reactor_runtime_sgemm_query_async)>::value);
static_assert(std::is_same<oct_prom_gemma4e2b_input_rmsnorm_fn,
                          decltype(&prometheus_reactor_runtime_gemma4e2b_m1_input_rmsnorm)>::value);
static_assert(std::is_same<oct_prom_gemma4e2b_rope_fn,
                          decltype(&prometheus_reactor_runtime_gemma4e2b_m1_rope)>::value);
static_assert(std::is_same<oct_prom_gemma4e2b_head_rmsnorm_rope_fn,
                          decltype(&prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope)>::value);
static_assert(std::is_same<oct_prom_gemma4e2b_attention_scores_fn,
                          decltype(&prometheus_reactor_runtime_gemma4e2b_m1_attention_scores)>::value);
static_assert(PROM_REACTOR_ABI_V1 == 1u);
static_assert(std::is_same<prom_single_head_attention_request,
                          prom_m42_attention_request>::value);
static_assert(std::is_same<prom_grouped_attention_plan,
                          prom_m43_attention_plan>::value);
static_assert(std::is_same<prom_attention_output_projection_plan,
                          prom_m44_output_projection_plan>::value);
static_assert(std::is_same<prom_attention_residual_plan,
                          prom_m45_residual_plan>::value);
static_assert(std::is_same<prom_rmsnorm_plan, prom_m46_rmsnorm_plan>::value);
static_assert(std::is_same<prom_gated_feed_forward_plan,
                          prom_m47_gated_ffn_plan>::value);
static_assert(std::is_same<prom_transformer_stack_plan,
                          prom_m48_transformer_stack_plan>::value);
}
FACT(PrometheusStage0GemmaABIAndDetailSnapshot)
{
    ASSERT_EQUAL(kInputRequestSize, sizeof(PrometheusGemma4E2BM1InputRmsNormRequest), "input RMSNorm request ABI size");
    ASSERT_EQUAL(kInputResultSize, sizeof(PrometheusGemma4E2BM1InputRmsNormResult), "input RMSNorm result ABI size");
    ASSERT_EQUAL(kRopeRequestSize, sizeof(PrometheusGemma4E2BM1RopeRequest), "RoPE request ABI size");
    ASSERT_EQUAL(kRopeResultSize, sizeof(PrometheusGemma4E2BM1RopeResult), "RoPE result ABI size");
    ASSERT_EQUAL(kHeadRmsNormRopeRequestSize, sizeof(PrometheusGemma4E2BM1HeadRmsNormRopeRequest), "resident Q/K request ABI size");
    ASSERT_EQUAL(kHeadRmsNormRopeResultSize, sizeof(PrometheusGemma4E2BM1HeadRmsNormRopeResult), "resident Q/K result ABI size");
    ASSERT_EQUAL(kAttentionScoresRequestSize, sizeof(PrometheusGemma4E2BM1AttentionScoresRequest), "raw-score request ABI size");
    ASSERT_EQUAL(kAttentionScoresResultSize, sizeof(PrometheusGemma4E2BM1AttentionScoresResult), "raw-score result ABI size");
    ASSERT_EQUAL(-7406, PROM_M46_DETAIL_STALE_WEIGHT_GENERATION, "known stale-weight detail code");
    ASSERT_EQUAL(0, PROM_OK, "success ABI value");
    ASSERT_EQUAL(1, PROM_ERROR, "error ABI value");
    ASSERT_EQUAL(1u, PROM_STAGE_INIT, "initialization stage value");
    ASSERT_EQUAL(3u, PROM_STAGE_SUBMIT, "submission stage value");
    ASSERT_EQUAL(4u, PROM_STAGE_TRANSFER_OUT, "readback stage value");
}
