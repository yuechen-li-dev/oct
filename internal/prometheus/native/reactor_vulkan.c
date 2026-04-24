#include "reactor_vulkan.h"

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN 1
#endif
#ifndef NOMINMAX
#define NOMINMAX 1
#endif
#include <windows.h>
#else
#include <pthread.h>
#endif

#include <vulkan/vulkan.h>
#include "reactor_judgment_engine.h"
#include "reactor_slot_hfsm.h"
#include "reactor_vulkan_fp16_spirv.h"
#include "reactor_vulkan_packed4_spirv.h"
#include "reactor_vulkan_tiled_spirv.h"

#define PROMETHEUS_RUNTIME_MAGIC 0x50524f4du
#define PROMETHEUS_MAX_TRACKED_HANDLES 256

#define PROM_VK_LOCAL_SIZE_X 8u
#define PROM_VK_LOCAL_SIZE_Y 8u
#define PROM_VK_TILE_K 8u

typedef struct prom_vk_buffer {
  VkBuffer buffer;
  VkDeviceMemory memory;
  void* mapped;
  VkDeviceSize size;
} prom_vk_buffer;

typedef struct prom_sgemm_controller_defaults {
  uint32_t lookahead_default;
  uint32_t lookahead_min;
  uint32_t lookahead_max;
  uint32_t outstanding_default;
  uint32_t outstanding_min;
  uint32_t outstanding_max;
  uint32_t chunk_default;
  uint32_t chunk_min;
  uint32_t chunk_max;
  uint32_t waste_budget_units;
  uint32_t retreat_permille;
  uint32_t recover_permille;
  uint32_t recovery_window;
} prom_sgemm_controller_defaults;

typedef struct prom_sgemm_controller_state {
  prom_policy_memory policy_memory;
  prom_policy_thresholds policy_thresholds;
  prom_policy_facts policy_facts;
  uint32_t lookahead;
  uint32_t outstanding_depth;
  uint32_t chunk_size;
  uint32_t pending_waste_units;
  uint32_t last_shape_signature;
  uint32_t last_shape_m;
  uint32_t last_shape_n;
  uint32_t last_shape_k;
  uint32_t last_mode;
  uint32_t decision_count;
  uint32_t retreat_count;
  uint32_t recovery_count;
  uint32_t transition_count;
  uint32_t instability_count;
  uint32_t budget_depletion_count;
  uint32_t safe_mode_decisions;
  uint32_t aggressive_mode_decisions;
  uint32_t recovery_mode_decisions;
  uint32_t lag_early_warning_count;
  uint32_t burst_dampening_count;
  uint32_t bound_violation_count;
  uint64_t wasted_work_units_total;
  uint32_t wasted_work_units_last;
  uint32_t packed4_selected_layout_format;
  uint32_t packed4_tail_count_last;
  uint64_t packed4_tail_count_total;
  uint32_t packed4_padded_lane_count_last;
  uint64_t packed4_padded_lane_count_total;
  uint32_t packed4_padding_waste_permille_last;
  uint64_t packed4_mode_budget_denials;
  uint64_t packed4_row_major_check_failures;
  uint64_t packed4_selection_count;
  uint64_t packed4_fallback_reason_padding_waste;
  uint64_t packed4_fallback_reason_small_shape;
  uint64_t packed4_fallback_reason_capability_missing;
  uint64_t packed4_fallback_reason_fallback_required;
  uint64_t packed4_fallback_reason_mode_budget_denied;
  float fp16_max_absolute_error;
  float fp16_max_relative_error;
  float fp16_aggregate_error;
  uint32_t fp16_worst_case_element_index;
  float fp16_k_error_growth;
  float fp16_cancellation_risk;
  uint32_t fp16_tolerance_known;
  uint32_t fp16_tolerance_pass;
  int fp16_fallback_reason_detail;
  uint32_t fp16_selected_candidate;
} prom_sgemm_controller_state;

typedef struct prom_slot_runtime_diag {
  uint32_t current_slot_id;
  uint32_t next_slot_id;
  uint64_t swap_count;
  uint64_t max_wip_depth;
  uint64_t overwrite_rejection_count;
  uint64_t stale_buffer_rejection_count;
  uint64_t shape_invalidation_count;
  uint64_t layout_invalidation_count;
  uint64_t capacity_invalidation_count;
  uint64_t inflight_rejection_count;
  int failure_slot_id;
  int failure_reason;
  int async_slot_id;
} prom_slot_runtime_diag;

typedef struct prometheus_runtime {
  uint32_t magic;
  uint32_t available;
  uint32_t reason_code;
  int init_detail_code;
  uint32_t test_flags;

  VkInstance instance;
  VkPhysicalDevice physical_device;
  VkDevice device;
  uint32_t queue_family_index;
  VkQueue compute_queue;
  VkCommandPool command_pool;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkDescriptorSet descriptor_set;
  VkCommandBuffer command_buffer;
  VkFence submit_fence;
  VkPipelineLayout pipeline_layout;
  VkPipeline pipeline;
  VkPipeline tiled_pipeline;
  VkPipeline packed4_pipeline;
  VkPipeline fp16_pipeline;
  prom_vk_buffer direct_a;
  prom_vk_buffer direct_b;
  prom_vk_buffer direct_c;
  prom_vk_buffer staged_device_a;
  prom_vk_buffer staged_device_b;
  prom_vk_buffer staged_device_c;
  prom_vk_buffer staged_upload_a;
  prom_vk_buffer staged_upload_b;
  prom_vk_buffer staged_readback_c;
  uint32_t buffer_shape_m;
  uint32_t buffer_shape_n;
  uint32_t buffer_shape_k;
  uint32_t has_direct_buffers;
  uint32_t has_staged_buffers;
  uint32_t has_device_local_memory;
  uint32_t has_host_visible_memory;
  uint32_t in_flight_submit;
  uint32_t software_vulkan;
  uint32_t capability_fp16_storage;
  uint32_t async_state;
  int async_task_id;
  uint32_t async_m;
  uint32_t async_n;
  uint32_t async_k;
  size_t async_c_copy_size;
  prom_vk_path_mode async_selected_path;
  int async_final_detail;
  uint32_t async_stage;
  int async_failure_detail;
  prom_sgemm_controller_state sgemm_controller;
  prom_slot_hfsm slots[2];
  prom_slot_runtime_diag slot_diag;
} prometheus_runtime;

typedef struct prom_vk_push {
  uint32_t m;
  uint32_t n;
  uint32_t k;
} prom_vk_push;

enum {
  PROM_VK_PUSH_FIELD_OFFSET_M = 0,
  PROM_VK_PUSH_FIELD_OFFSET_N = 4,
  PROM_VK_PUSH_FIELD_OFFSET_K = 8,
  PROM_VK_SHADER_PUSH_BYTES = 12,
};

enum {
  PROM_SGEMM_LOOKAHEAD_DEFAULT = 2u,
  PROM_SGEMM_LOOKAHEAD_MIN = 0u,
  PROM_SGEMM_LOOKAHEAD_MAX = 2u,
  PROM_SGEMM_OUTSTANDING_DEFAULT = 2u,
  PROM_SGEMM_OUTSTANDING_MIN = 1u,
  PROM_SGEMM_OUTSTANDING_MAX = 2u,
  PROM_SGEMM_CHUNK_DEFAULT = 16u,
  PROM_SGEMM_CHUNK_MIN = 8u,
  PROM_SGEMM_CHUNK_MAX = 32u,
  PROM_SGEMM_WASTE_BUDGET_UNITS = 64u,
  PROM_SGEMM_RETREAT_PERMILLE = 250u,
  PROM_SGEMM_RECOVER_PERMILLE = 120u,
  PROM_SGEMM_RECOVERY_WINDOW = 3u,
  PROM_SGEMM_HYSTERESIS_MARGIN = 40u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE = 380u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE = 220u,
  PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY = 140u,
};

/*
 * Push-constant layout contract (M11 hygiene port):
 * - host and shader use the same field list and order: m, n, k
 * - no mixed-width fields
 * - no reliance on implicit host padding
 * - append-only evolution only: add new fields at the end and update both
 *   this host contract and the shader module together
 */
#if defined(__cplusplus)
static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#else
_Static_assert(offsetof(prom_vk_push, m) == PROM_VK_PUSH_FIELD_OFFSET_M, "push.m offset drift");
_Static_assert(offsetof(prom_vk_push, n) == PROM_VK_PUSH_FIELD_OFFSET_N, "push.n offset drift");
_Static_assert(offsetof(prom_vk_push, k) == PROM_VK_PUSH_FIELD_OFFSET_K, "push.k offset drift");
_Static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES, "push struct size drift");
#endif

static void* g_active_handles[PROMETHEUS_MAX_TRACKED_HANDLES];

#if defined(_WIN32)
static SRWLOCK g_registry_lock = SRWLOCK_INIT;

static void registry_lock(void) {
  AcquireSRWLockExclusive(&g_registry_lock);
}

static void registry_unlock(void) {
  ReleaseSRWLockExclusive(&g_registry_lock);
}
#else
static pthread_mutex_t g_registry_mutex = PTHREAD_MUTEX_INITIALIZER;

static void registry_lock(void) {
  pthread_mutex_lock(&g_registry_mutex);
}

static void registry_unlock(void) {
  pthread_mutex_unlock(&g_registry_mutex);
}
#endif

/* SPIR-V for:
 * #version 450
 * layout(local_size_x=8, local_size_y=8) in;
 * layout(set=0,binding=0) readonly buffer ABuffer{float a[];};
 * layout(set=0,binding=1) readonly buffer BBuffer{float b[];};
 * layout(set=0,binding=2) writeonly buffer CBuffer{float c[];};
 * layout(push_constant) uniform Push{uint m; uint n; uint k;} pc;
 * SPIR-V confirms offsets m=0, n=4, k=8.
 * ... naive row-major SGEMM C=A*B
 */
static const uint32_t k_prom_sgemm_spirv[] = {
    0x07230203u, 0x00010000u, 0x0008000bu, 0x00000066u, 0x00000000u, 0x00020011u, 0x00000001u,
    0x0006000bu, 0x00000001u, 0x4c534c47u, 0x6474732eu, 0x3035342eu, 0x00000000u, 0x0003000eu,
    0x00000000u, 0x00000001u, 0x0006000fu, 0x00000005u, 0x00000004u, 0x6e69616du, 0x00000000u,
    0x0000000bu, 0x00060010u, 0x00000004u, 0x00000011u, 0x00000008u, 0x00000008u, 0x00000001u,
    0x00030003u, 0x00000002u, 0x000001c2u, 0x00040005u, 0x00000004u, 0x6e69616du, 0x00000000u,
    0x00030005u, 0x00000008u, 0x00776f72u, 0x00080005u, 0x0000000bu, 0x475f6c67u, 0x61626f6cu,
    0x766e496cu, 0x7461636fu, 0x496e6f69u, 0x00000044u, 0x00030005u, 0x00000010u, 0x006c6f63u,
    0x00040005u, 0x00000016u, 0x68737550u, 0x00000000u, 0x00040006u, 0x00000016u, 0x00000000u,
    0x0000006du, 0x00040006u, 0x00000016u, 0x00000001u, 0x0000006eu, 0x00040006u, 0x00000016u,
    0x00000002u, 0x0000006bu, 0x00030005u, 0x00000018u, 0x00006370u, 0x00030005u, 0x0000002du,
    0x006d7573u, 0x00030005u, 0x0000002fu, 0x00006b6bu, 0x00040005u, 0x0000003bu, 0x66754241u,
    0x00726566u, 0x00040006u, 0x0000003bu, 0x00000000u, 0x00000061u, 0x00030005u, 0x0000003du,
    0x00000000u, 0x00040005u, 0x00000048u, 0x66754242u, 0x00726566u, 0x00040006u, 0x00000048u,
    0x00000000u, 0x00000062u, 0x00030005u, 0x0000004au, 0x00000000u, 0x00040005u, 0x00000059u,
    0x66754243u, 0x00726566u, 0x00040006u, 0x00000059u, 0x00000000u, 0x00000063u, 0x00030005u,
    0x0000005bu, 0x00000000u, 0x00040047u, 0x0000000bu, 0x0000000bu, 0x0000001cu, 0x00030047u,
    0x00000016u, 0x00000002u, 0x00050048u, 0x00000016u, 0x00000000u, 0x00000023u, 0x00000000u,
    0x00050048u, 0x00000016u, 0x00000001u, 0x00000023u, 0x00000004u, 0x00050048u, 0x00000016u,
    0x00000002u, 0x00000023u, 0x00000008u, 0x00040047u, 0x0000003au, 0x00000006u, 0x00000004u,
    0x00030047u, 0x0000003bu, 0x00000003u, 0x00040048u, 0x0000003bu, 0x00000000u, 0x00000018u,
    0x00050048u, 0x0000003bu, 0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000003du,
    0x00000018u, 0x00040047u, 0x0000003du, 0x00000021u, 0x00000000u, 0x00040047u, 0x0000003du,
    0x00000022u, 0x00000000u, 0x00040047u, 0x00000047u, 0x00000006u, 0x00000004u, 0x00030047u,
    0x00000048u, 0x00000003u, 0x00040048u, 0x00000048u, 0x00000000u, 0x00000018u, 0x00050048u,
    0x00000048u, 0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000004au, 0x00000018u,
    0x00040047u, 0x0000004au, 0x00000021u, 0x00000001u, 0x00040047u, 0x0000004au, 0x00000022u,
    0x00000000u, 0x00040047u, 0x00000058u, 0x00000006u, 0x00000004u, 0x00030047u, 0x00000059u,
    0x00000003u, 0x00040048u, 0x00000059u, 0x00000000u, 0x00000019u, 0x00050048u, 0x00000059u,
    0x00000000u, 0x00000023u, 0x00000000u, 0x00030047u, 0x0000005bu, 0x00000019u, 0x00040047u,
    0x0000005bu, 0x00000021u, 0x00000002u, 0x00040047u, 0x0000005bu, 0x00000022u, 0x00000000u,
    0x00040047u, 0x00000065u, 0x0000000bu, 0x00000019u, 0x00020013u, 0x00000002u, 0x00030021u,
    0x00000003u, 0x00000002u, 0x00040015u, 0x00000006u, 0x00000020u, 0x00000000u, 0x00040020u,
    0x00000007u, 0x00000007u, 0x00000006u, 0x00040017u, 0x00000009u, 0x00000006u, 0x00000003u,
    0x00040020u, 0x0000000au, 0x00000001u, 0x00000009u, 0x0004003bu, 0x0000000au, 0x0000000bu,
    0x00000001u, 0x0004002bu, 0x00000006u, 0x0000000cu, 0x00000000u, 0x00040020u, 0x0000000du,
    0x00000001u, 0x00000006u, 0x0004002bu, 0x00000006u, 0x00000011u, 0x00000001u, 0x00020014u,
    0x00000014u, 0x0005001eu, 0x00000016u, 0x00000006u, 0x00000006u, 0x00000006u, 0x00040020u,
    0x00000017u, 0x00000009u, 0x00000016u, 0x0004003bu, 0x00000017u, 0x00000018u, 0x00000009u,
    0x00040015u, 0x00000019u, 0x00000020u, 0x00000001u, 0x0004002bu, 0x00000019u, 0x0000001au,
    0x00000000u, 0x00040020u, 0x0000001bu, 0x00000009u, 0x00000006u, 0x0004002bu, 0x00000019u,
    0x00000023u, 0x00000001u, 0x00030016u, 0x0000002bu, 0x00000020u, 0x00040020u, 0x0000002cu,
    0x00000007u, 0x0000002bu, 0x0004002bu, 0x0000002bu, 0x0000002eu, 0x00000000u, 0x0004002bu,
    0x00000019u, 0x00000036u, 0x00000002u, 0x0003001du, 0x0000003au, 0x0000002bu, 0x0003001eu,
    0x0000003bu, 0x0000003au, 0x00040020u, 0x0000003cu, 0x00000002u, 0x0000003bu, 0x0004003bu,
    0x0000003cu, 0x0000003du, 0x00000002u, 0x00040020u, 0x00000044u, 0x00000002u, 0x0000002bu,
    0x0003001du, 0x00000047u, 0x0000002bu, 0x0003001eu, 0x00000048u, 0x00000047u, 0x00040020u,
    0x00000049u, 0x00000002u, 0x00000048u, 0x0004003bu, 0x00000049u, 0x0000004au, 0x00000002u,
    0x0003001du, 0x00000058u, 0x0000002bu, 0x0003001eu, 0x00000059u, 0x00000058u, 0x00040020u,
    0x0000005au, 0x00000002u, 0x00000059u, 0x0004003bu, 0x0000005au, 0x0000005bu, 0x00000002u,
    0x0004002bu, 0x00000006u, 0x00000064u, 0x00000008u, 0x0006002cu, 0x00000009u, 0x00000065u,
    0x00000064u, 0x00000064u, 0x00000011u, 0x00050036u, 0x00000002u, 0x00000004u, 0x00000000u,
    0x00000003u, 0x000200f8u, 0x00000005u, 0x0004003bu, 0x00000007u, 0x00000008u, 0x00000007u,
    0x0004003bu, 0x00000007u, 0x00000010u, 0x00000007u, 0x0004003bu, 0x0000002cu, 0x0000002du,
    0x00000007u, 0x0004003bu, 0x00000007u, 0x0000002fu, 0x00000007u, 0x00050041u, 0x0000000du,
    0x0000000eu, 0x0000000bu, 0x0000000cu, 0x0004003du, 0x00000006u, 0x0000000fu, 0x0000000eu,
    0x0003003eu, 0x00000008u, 0x0000000fu, 0x00050041u, 0x0000000du, 0x00000012u, 0x0000000bu,
    0x00000011u, 0x0004003du, 0x00000006u, 0x00000013u, 0x00000012u, 0x0003003eu, 0x00000010u,
    0x00000013u, 0x0004003du, 0x00000006u, 0x00000015u, 0x00000008u, 0x00050041u, 0x0000001bu,
    0x0000001cu, 0x00000018u, 0x0000001au, 0x0004003du, 0x00000006u, 0x0000001du, 0x0000001cu,
    0x000500aeu, 0x00000014u, 0x0000001eu, 0x00000015u, 0x0000001du, 0x000400a8u, 0x00000014u,
    0x0000001fu, 0x0000001eu, 0x000300f7u, 0x00000021u, 0x00000000u, 0x000400fau, 0x0000001fu,
    0x00000020u, 0x00000021u, 0x000200f8u, 0x00000020u, 0x0004003du, 0x00000006u, 0x00000022u,
    0x00000010u, 0x00050041u, 0x0000001bu, 0x00000024u, 0x00000018u, 0x00000023u, 0x0004003du,
    0x00000006u, 0x00000025u, 0x00000024u, 0x000500aeu, 0x00000014u, 0x00000026u, 0x00000022u,
    0x00000025u, 0x000200f9u, 0x00000021u, 0x000200f8u, 0x00000021u, 0x000700f5u, 0x00000014u,
    0x00000027u, 0x0000001eu, 0x00000005u, 0x00000026u, 0x00000020u, 0x000300f7u, 0x00000029u,
    0x00000000u, 0x000400fau, 0x00000027u, 0x00000028u, 0x00000029u, 0x000200f8u, 0x00000028u,
    0x000100fdu, 0x000200f8u, 0x00000029u, 0x0003003eu, 0x0000002du, 0x0000002eu, 0x0003003eu,
    0x0000002fu, 0x0000000cu, 0x000200f9u, 0x00000030u, 0x000200f8u, 0x00000030u, 0x000400f6u,
    0x00000032u, 0x00000033u, 0x00000000u, 0x000200f9u, 0x00000034u, 0x000200f8u, 0x00000034u,
    0x0004003du, 0x00000006u, 0x00000035u, 0x0000002fu, 0x00050041u, 0x0000001bu, 0x00000037u,
    0x00000018u, 0x00000036u, 0x0004003du, 0x00000006u, 0x00000038u, 0x00000037u, 0x000500b0u,
    0x00000014u, 0x00000039u, 0x00000035u, 0x00000038u, 0x000400fau, 0x00000039u, 0x00000031u,
    0x00000032u, 0x000200f8u, 0x00000031u, 0x0004003du, 0x00000006u, 0x0000003eu, 0x00000008u,
    0x00050041u, 0x0000001bu, 0x0000003fu, 0x00000018u, 0x00000036u, 0x0004003du, 0x00000006u,
    0x00000040u, 0x0000003fu, 0x00050084u, 0x00000006u, 0x00000041u, 0x0000003eu, 0x00000040u,
    0x0004003du, 0x00000006u, 0x00000042u, 0x0000002fu, 0x00050080u, 0x00000006u, 0x00000043u,
    0x00000041u, 0x00000042u, 0x00060041u, 0x00000044u, 0x00000045u, 0x0000003du, 0x0000001au,
    0x00000043u, 0x0004003du, 0x0000002bu, 0x00000046u, 0x00000045u, 0x0004003du, 0x00000006u,
    0x0000004bu, 0x0000002fu, 0x00050041u, 0x0000001bu, 0x0000004cu, 0x00000018u, 0x00000023u,
    0x0004003du, 0x00000006u, 0x0000004du, 0x0000004cu, 0x00050084u, 0x00000006u, 0x0000004eu,
    0x0000004bu, 0x0000004du, 0x0004003du, 0x00000006u, 0x0000004fu, 0x00000010u, 0x00050080u,
    0x00000006u, 0x00000050u, 0x0000004eu, 0x0000004fu, 0x00060041u, 0x00000044u, 0x00000051u,
    0x0000004au, 0x0000001au, 0x00000050u, 0x0004003du, 0x0000002bu, 0x00000052u, 0x00000051u,
    0x00050085u, 0x0000002bu, 0x00000053u, 0x00000046u, 0x00000052u, 0x0004003du, 0x0000002bu,
    0x00000054u, 0x0000002du, 0x00050081u, 0x0000002bu, 0x00000055u, 0x00000054u, 0x00000053u,
    0x0003003eu, 0x0000002du, 0x00000055u, 0x000200f9u, 0x00000033u, 0x000200f8u, 0x00000033u,
    0x0004003du, 0x00000006u, 0x00000056u, 0x0000002fu, 0x00050080u, 0x00000006u, 0x00000057u,
    0x00000056u, 0x00000023u, 0x0003003eu, 0x0000002fu, 0x00000057u, 0x000200f9u, 0x00000030u,
    0x000200f8u, 0x00000032u, 0x0004003du, 0x00000006u, 0x0000005cu, 0x00000008u, 0x00050041u,
    0x0000001bu, 0x0000005du, 0x00000018u, 0x00000023u, 0x0004003du, 0x00000006u, 0x0000005eu,
    0x0000005du, 0x00050084u, 0x00000006u, 0x0000005fu, 0x0000005cu, 0x0000005eu, 0x0004003du,
    0x00000006u, 0x00000060u, 0x00000010u, 0x00050080u, 0x00000006u, 0x00000061u, 0x0000005fu,
    0x00000060u, 0x0004003du, 0x0000002bu, 0x00000062u, 0x0000002du, 0x00060041u, 0x00000044u,
    0x00000063u, 0x0000005bu, 0x0000001au, 0x00000061u, 0x0003003eu, 0x00000063u, 0x00000062u,
    0x000100fdu, 0x00010038u,
};

static void set_status(uint32_t* out_stage, int* out_detail_code, uint32_t stage, int detail) {
  if (out_stage != NULL) {
    *out_stage = stage;
  }
  if (out_detail_code != NULL) {
    *out_detail_code = detail;
  }
}

static void set_async_state(prometheus_runtime* rt, uint32_t state, uint32_t stage, int detail) {
  if (rt == NULL) {
    return;
  }
  rt->async_state = state;
  rt->async_stage = stage;
  if (state == PROM_ASYNC_STATE_FAILED) {
    rt->async_failure_detail = detail;
  } else {
    rt->async_failure_detail = 0;
  }
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason);
static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id);

static int update_async_progress(prometheus_runtime* rt) {
  VkResult vk_result;

  if (rt == NULL) {
    return PROM_ERROR;
  }
  if (rt->async_state != PROM_ASYNC_STATE_SUBMITTED) {
    return PROM_OK;
  }
  if ((rt->test_flags & PROM_TESTCFG_FAIL_ASYNC_POLL) != 0u) {
    rt->in_flight_submit = 0u;
    set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    if (rt->slot_diag.async_slot_id >= 0) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE);
    }
    return PROM_ERROR;
  }
  vk_result = vkGetFenceStatus(rt->device, rt->submit_fence);
  if (vk_result == VK_SUCCESS) {
    rt->in_flight_submit = 0u;
    if (rt->slot_diag.async_slot_id >= 0 && !prom_slot_mark_complete(rt, (uint32_t)rt->slot_diag.async_slot_id)) {
      prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
    set_async_state(rt, PROM_ASYNC_STATE_READY, PROM_STAGE_SUBMIT, rt->async_final_detail);
    return PROM_OK;
  }
  if (vk_result == VK_NOT_READY) {
    return PROM_OK;
  }
  rt->in_flight_submit = 0u;
  if (rt->slot_diag.async_slot_id >= 0) {
    prom_slot_mark_failure(rt, (uint32_t)rt->slot_diag.async_slot_id, (int)vk_result);
  }
  set_async_state(rt, PROM_ASYNC_STATE_FAILED, PROM_STAGE_SUBMIT, (int)vk_result);
  return PROM_ERROR;
}

static int checked_mul_u32(uint32_t left, uint32_t right, uint32_t* out_value) {
  if (out_value == NULL) {
    return 0;
  }
  if (left != 0u && right > UINT32_MAX / left) {
    return 0;
  }
  *out_value = left * right;
  return 1;
}

static int checked_float_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t bytes;

  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  bytes = (uint64_t)elements * (uint64_t)sizeof(float);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }

  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static int checked_packed_fp16_buffer_size(uint32_t rows, uint32_t cols, VkDeviceSize* out_vk_size, size_t* out_copy_size) {
  uint32_t elements;
  uint64_t words;
  uint64_t bytes;
  if (out_vk_size == NULL || out_copy_size == NULL) {
    return 0;
  }
  if (!checked_mul_u32(rows, cols, &elements)) {
    return 0;
  }
  words = ((uint64_t)elements + 1u) / 2u;
  bytes = words * (uint64_t)sizeof(uint32_t);
  if (bytes > (uint64_t)SIZE_MAX) {
    return 0;
  }
  *out_copy_size = (size_t)bytes;
  *out_vk_size = (VkDeviceSize)bytes;
  return 1;
}

static uint32_t prom_slot_other_id(uint32_t slot_id) {
  return slot_id == 0u ? 1u : 0u;
}

static uint32_t prom_slot_compute_layout_code(prom_vk_path_mode path, prom_vk_compute_mode compute_mode) {
  return ((uint32_t)path << 16u) | (uint32_t)compute_mode;
}

static uint32_t prom_slot_wip_depth(const prometheus_runtime* rt) {
  uint32_t depth = 0u;
  uint32_t i;
  for (i = 0u; i < 2u; ++i) {
    const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[i]);
    if (state == PROM_SLOT_PREPARING || state == PROM_SLOT_READY || state == PROM_SLOT_CURRENT || state == PROM_SLOT_IN_FLIGHT) {
      depth += 1u;
    }
  }
  return depth;
}

static void prom_slot_track_wip(prometheus_runtime* rt) {
  const uint64_t depth = (uint64_t)prom_slot_wip_depth(rt);
  if (depth > rt->slot_diag.max_wip_depth) {
    rt->slot_diag.max_wip_depth = depth;
  }
}

static int prom_slot_cleanup_to_empty(prom_slot_hfsm* slot) {
  if (slot == NULL) {
    return 0;
  }
  if (prom_slot_hfsm_current_state(slot) == PROM_SLOT_FAILED) {
    return prom_slot_hfsm_cleanup(slot) != 0u ? 1 : 0;
  }
  if (prom_slot_hfsm_current_state(slot) == PROM_SLOT_EMPTY) {
    return 1;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_EMPTY) == 0u) {
    return 0;
  }
  return 1;
}

static int prom_slot_prepare_for_call(prometheus_runtime* rt,
                                      uint32_t slot_id,
                                      uint32_t m,
                                      uint32_t n,
                                      uint32_t k,
                                      uint32_t layout_code,
                                      uint32_t precision_code,
                                      uint64_t required_capacity_bytes) {
  prom_slot_hfsm* slot;
  prom_slot_metadata metadata;
  const prom_slot_metadata* existing;
  const prom_slot_state state = prom_slot_hfsm_current_state(&rt->slots[slot_id]);
  int invalidation_reason = 0;

  if (state == PROM_SLOT_IN_FLIGHT || state == PROM_SLOT_CURRENT) {
    rt->slot_diag.overwrite_rejection_count += 1u;
    rt->slot_diag.inflight_rejection_count += 1u;
    return 0;
  }

  slot = &rt->slots[slot_id];
  existing = prom_slot_hfsm_metadata(slot);
  if (existing->valid != 0u) {
    if (existing->shape.m != m || existing->shape.n != n || existing->shape.k != k) {
      rt->slot_diag.shape_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (existing->layout.layout != layout_code) {
      rt->slot_diag.layout_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_INVALID_LAYOUT;
    }
    if (existing->required_capacity_bytes < required_capacity_bytes) {
      rt->slot_diag.capacity_invalidation_count += 1u;
      invalidation_reason = PROM_DETAIL_SLOT_STALE_REJECTED;
    }
    if (invalidation_reason != 0) {
      prom_slot_hfsm_mark_invalidated(slot, invalidation_reason);
      rt->slot_diag.stale_buffer_rejection_count += 1u;
    }
  }

  if (!prom_slot_cleanup_to_empty(slot)) {
    return 0;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_PREPARING) == 0u) {
    return 0;
  }

  metadata = *prom_slot_hfsm_metadata(slot);
  metadata.slot_id = slot_id;
  metadata.generation += 1u;
  metadata.valid = 1u;
  metadata.shape.m = m;
  metadata.shape.n = n;
  metadata.shape.k = k;
  metadata.layout.layout = layout_code;
  metadata.layout.precision = precision_code;
  metadata.required_capacity_bytes = required_capacity_bytes;
  metadata.failure_reason = 0;
  prom_slot_hfsm_set_metadata(slot, &metadata);

  if (prom_slot_hfsm_transition(slot, PROM_SLOT_READY) == 0u) {
    return 0;
  }
  rt->slot_diag.next_slot_id = slot_id;
  prom_slot_track_wip(rt);
  return 1;
}

static int prom_slot_swap_ready_to_current(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_current_state(slot) != PROM_SLOT_READY) {
    rt->slot_diag.stale_buffer_rejection_count += 1u;
    return 0;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CURRENT) == 0u) {
    return 0;
  }
  rt->slot_diag.current_slot_id = slot_id;
  rt->slot_diag.next_slot_id = prom_slot_other_id(slot_id);
  rt->slot_diag.swap_count += 1u;
  prom_slot_track_wip(rt);
  return 1;
}

static void prom_slot_mark_failure(prometheus_runtime* rt, uint32_t slot_id, int reason) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  (void)prom_slot_hfsm_fail(slot, reason);
  rt->slot_diag.failure_slot_id = (int)slot_id;
  rt->slot_diag.failure_reason = reason;
}

static int prom_slot_mark_submitted(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_IN_FLIGHT) == 0u) {
    return 0;
  }
  rt->slot_diag.async_slot_id = (int)slot_id;
  prom_slot_track_wip(rt);
  return 1;
}

static int prom_slot_mark_complete(prometheus_runtime* rt, uint32_t slot_id) {
  prom_slot_hfsm* slot = &rt->slots[slot_id];
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_CONSUMED) == 0u) {
    return 0;
  }
  if (prom_slot_hfsm_transition(slot, PROM_SLOT_EMPTY) == 0u) {
    return 0;
  }
  if (rt->slot_diag.async_slot_id == (int)slot_id) {
    rt->slot_diag.async_slot_id = -1;
  }
  prom_slot_track_wip(rt);
  return 1;
}

static uint16_t prom_float32_to_fp16_bits(float value) {
  union { float f; uint32_t u; } in;
  uint32_t sign;
  uint32_t exponent;
  uint32_t mantissa;
  in.f = value;
  sign = (in.u >> 16u) & 0x8000u;
  exponent = (in.u >> 23u) & 0xffu;
  mantissa = in.u & 0x7fffffu;
  if (exponent == 0xffu) {
    return (uint16_t)(sign | (mantissa == 0u ? 0x7c00u : 0x7e00u));
  }
  if (exponent > 142u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  if (exponent < 113u) {
    uint32_t shifted;
    if (exponent < 103u) {
      return (uint16_t)sign;
    }
    mantissa |= 0x800000u;
    shifted = 125u - exponent;
    mantissa = (mantissa + (1u << (shifted - 1u))) >> shifted;
    return (uint16_t)(sign | mantissa);
  }
  exponent = exponent - 112u;
  mantissa = mantissa + 0x1000u;
  if ((mantissa & 0x00800000u) != 0u) {
    mantissa = 0u;
    exponent += 1u;
  }
  if (exponent >= 31u) {
    return (uint16_t)(sign | 0x7c00u);
  }
  return (uint16_t)(sign | (exponent << 10u) | (mantissa >> 13u));
}

static float prom_fp16_bits_to_float32(uint16_t value) {
  uint32_t sign = ((uint32_t)value & 0x8000u) << 16u;
  uint32_t exponent = ((uint32_t)value >> 10u) & 0x1fu;
  uint32_t mantissa = (uint32_t)value & 0x3ffu;
  union { uint32_t u; float f; } out;
  if (exponent == 0u) {
    if (mantissa == 0u) {
      out.u = sign;
      return out.f;
    }
    exponent = 127u - 15u + 1u;
    while ((mantissa & 0x400u) == 0u) {
      mantissa <<= 1u;
      exponent -= 1u;
    }
    mantissa &= 0x3ffu;
    out.u = sign | (exponent << 23u) | (mantissa << 13u);
    return out.f;
  }
  if (exponent == 31u) {
    out.u = sign | 0x7f800000u | (mantissa << 13u);
    return out.f;
  }
  exponent = exponent + (127u - 15u);
  out.u = sign | (exponent << 23u) | (mantissa << 13u);
  return out.f;
}

static void prom_pack_fp16_pairs(const float* src, uint32_t element_count, uint32_t* dst_words) {
  uint32_t i;
  for (i = 0u; i < element_count; i += 2u) {
    uint16_t lo = prom_float32_to_fp16_bits(src[i]);
    uint16_t hi = (i + 1u < element_count) ? prom_float32_to_fp16_bits(src[i + 1u]) : (uint16_t)0u;
    dst_words[i / 2u] = (uint32_t)lo | ((uint32_t)hi << 16u);
  }
}

static uint32_t prom_round_up4_u32(uint32_t value) {
  return (value + 3u) & ~3u;
}

static uint32_t prom_packed4_tail_count(uint32_t m, uint32_t n, uint32_t k) {
  uint32_t tails = 0u;
  if ((m & 3u) != 0u) {
    tails += 1u;
  }
  if ((n & 3u) != 0u) {
    tails += 1u;
  }
  if ((k & 3u) != 0u) {
    tails += 1u;
  }
  return tails;
}

static uint32_t prom_packed4_padding_waste_permille(uint32_t m, uint32_t n, uint32_t k) {
  const uint32_t pad_k = (4u - (k & 3u)) & 3u;
  const uint64_t padded_lanes = (uint64_t)pad_k * (uint64_t)(m + n);
  const uint64_t denom = (uint64_t)m * (uint64_t)n;
  if (denom == 0u) {
    return 0u;
  }
  return (uint32_t)((padded_lanes * 1000u) / denom);
}

static uint32_t prom_packed4_mode_budget_permille(prom_policy_mode mode) {
  if (mode == PROM_POLICY_MODE_SAFE) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_SAFE;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY) {
    return PROM_SGEMM_PACKED4_MODE_BUDGET_RECOVERY;
  }
  return PROM_SGEMM_PACKED4_MODE_BUDGET_AGGRESSIVE;
}

static void prom_packed4_record_fallback(prom_sgemm_controller_state* state, prom_packed4_reject_reason reason) {
  if (state == NULL) {
    return;
  }
  if (reason == PROM_PACKED4_REJECT_PADDING_WASTE) {
    state->packed4_fallback_reason_padding_waste += 1u;
  } else if (reason == PROM_PACKED4_REJECT_SMALL_SHAPE) {
    state->packed4_fallback_reason_small_shape += 1u;
  } else if (reason == PROM_PACKED4_REJECT_CAPABILITY_MISSING) {
    state->packed4_fallback_reason_capability_missing += 1u;
  } else if (reason == PROM_PACKED4_REJECT_FALLBACK_REQUIRED) {
    state->packed4_fallback_reason_fallback_required += 1u;
  } else if (reason == PROM_PACKED4_REJECT_MODE_BUDGET_DENIED) {
    state->packed4_fallback_reason_mode_budget_denied += 1u;
    state->packed4_mode_budget_denials += 1u;
  }
}

static int prom_fp16_reject_reason_to_detail(prom_fp16_reject_reason reason) {
  if (reason == PROM_FP16_REJECT_STRICT_FP32) return PROM_DETAIL_FP16_STRICT_FP32;
  if (reason == PROM_FP16_REJECT_TOLERANCE_UNKNOWN) return PROM_DETAIL_FP16_TOLERANCE_UNKNOWN;
  if (reason == PROM_FP16_REJECT_TOLERANCE_EXCEEDED) return PROM_DETAIL_FP16_TOLERANCE_EXCEEDED;
  if (reason == PROM_FP16_REJECT_SPECIAL_VALUE) return PROM_DETAIL_FP16_SPECIAL_VALUE;
  if (reason == PROM_FP16_REJECT_CAPABILITY_MISSING) return PROM_DETAIL_FP16_CAPABILITY_MISSING;
  if (reason == PROM_FP16_REJECT_FALLBACK_REQUIRED) return PROM_DETAIL_FP16_FALLBACK_REQUIRED;
  if (reason == PROM_FP16_REJECT_NOT_TOP_UTILITY) return PROM_DETAIL_FP16_NOT_TOP_UTILITY;
  return 0;
}

static void prom_fp16_evaluate_tolerance(const float* a,
                                         const float* b,
                                         uint32_t m,
                                         uint32_t n,
                                         uint32_t k,
                                         prom_sgemm_controller_state* state,
                                         uint32_t* has_special_values,
                                         int* utility_score) {
  uint32_t row;
  uint32_t col;
  const float abs_tolerance = 0.02f;
  const float rel_tolerance = 0.02f;
  const float aggregate_tolerance = 0.01f;
  float max_abs = 0.0f;
  float max_rel = 0.0f;
  float aggregate = 0.0f;
  uint32_t worst_index = 0u;
  float sign_flip_products = 0.0f;
  float total_products = 0.0f;

  if (state == NULL || has_special_values == NULL || utility_score == NULL) {
    return;
  }
  *has_special_values = 0u;
  state->fp16_tolerance_known = 1u;
  state->fp16_tolerance_pass = 1u;
  state->fp16_max_absolute_error = 0.0f;
  state->fp16_max_relative_error = 0.0f;
  state->fp16_aggregate_error = 0.0f;
  state->fp16_worst_case_element_index = 0u;
  state->fp16_k_error_growth = 0.0f;
  state->fp16_cancellation_risk = 0.0f;

  for (row = 0u; row < m; ++row) {
    for (col = 0u; col < n; ++col) {
      uint32_t kk;
      float reference = 0.0f;
      float fp16_value = 0.0f;
      float prev = 0.0f;
      for (kk = 0u; kk < k; ++kk) {
        float av = a[row * k + kk];
        float bv = b[kk * n + col];
        float qav;
        float qbv;
        float product;
        if (!isfinite(av) || !isfinite(bv)) {
          *has_special_values = 1u;
        }
        qav = prom_fp16_bits_to_float32(prom_float32_to_fp16_bits(av));
        qbv = prom_fp16_bits_to_float32(prom_float32_to_fp16_bits(bv));
        reference += av * bv;
        product = qav * qbv;
        fp16_value += product;
        if ((prev > 0.0f && product < 0.0f) || (prev < 0.0f && product > 0.0f)) {
          sign_flip_products += 1.0f;
        }
        prev = product;
        total_products += 1.0f;
      }
      {
        float abs_err = fabsf(reference - fp16_value);
        float denom = fmaxf(fabsf(reference), 1e-6f);
        float rel_err = abs_err / denom;
        aggregate += abs_err;
        if (abs_err > max_abs) {
          max_abs = abs_err;
          worst_index = row * n + col;
        }
        if (rel_err > max_rel) {
          max_rel = rel_err;
        }
      }
    }
  }
  state->fp16_max_absolute_error = max_abs;
  state->fp16_max_relative_error = max_rel;
  state->fp16_aggregate_error = aggregate;
  state->fp16_worst_case_element_index = worst_index;
  state->fp16_k_error_growth = k > 0u ? max_abs / (float)k : max_abs;
  state->fp16_cancellation_risk = total_products > 0.0f ? sign_flip_products / total_products : 0.0f;
  if (max_abs > abs_tolerance || max_rel > rel_tolerance || (aggregate / (float)(m * n)) > aggregate_tolerance) {
    state->fp16_tolerance_pass = 0u;
  }

  *utility_score = 900 - (int)(state->fp16_max_absolute_error * 1000.0f) - (int)(state->fp16_cancellation_risk * 200.0f);
}

static void prom_compute_scalar_row_major(const float* a, const float* b, float* c, uint32_t m, uint32_t n, uint32_t k) {
  uint32_t row;
  for (row = 0u; row < m; ++row) {
    uint32_t col;
    for (col = 0u; col < n; ++col) {
      float sum = 0.0f;
      uint32_t kk;
      for (kk = 0u; kk < k; ++kk) {
        sum += a[row * k + kk] * b[kk * n + col];
      }
      c[row * n + col] = sum;
    }
  }
}

static void prom_pack_a_packed4_rowmajor(const float* src, float* dst, uint32_t m, uint32_t k, uint32_t k4) {
  uint32_t row;
  memset(dst, 0, (size_t)m * (size_t)k4 * sizeof(float));
  for (row = 0u; row < m; ++row) {
    memcpy(dst + (size_t)row * (size_t)k4, src + (size_t)row * (size_t)k, (size_t)k * sizeof(float));
  }
}

static void prom_pack_b_packed4_colmajor(const float* src, float* dst, uint32_t n, uint32_t k, uint32_t k4) {
  uint32_t col;
  memset(dst, 0, (size_t)n * (size_t)k4 * sizeof(float));
  for (col = 0u; col < n; ++col) {
    uint32_t kk;
    float* dst_col = dst + (size_t)col * (size_t)k4;
    for (kk = 0u; kk < k; ++kk) {
      dst_col[kk] = src[(size_t)kk * (size_t)n + (size_t)col];
    }
  }
}

static void prom_apply_debug_row_major_oracle(prometheus_runtime* rt,
                                              const float* a,
                                              const float* b,
                                              float* c,
                                              uint32_t m,
                                              uint32_t n,
                                              uint32_t k) {
  size_t compare_index;
  size_t compare_len = (size_t)m * (size_t)n;
  float* row_major_oracle;
  if (rt == NULL || (rt->test_flags & PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK) == 0u) {
    return;
  }
  row_major_oracle = (float*)malloc(compare_len * sizeof(float));
  if (row_major_oracle == NULL) {
    return;
  }
  prom_compute_scalar_row_major(a, b, row_major_oracle, m, n, k);
  for (compare_index = 0u; compare_index < compare_len; ++compare_index) {
    if (c[compare_index] != row_major_oracle[compare_index]) {
      rt->sgemm_controller.packed4_row_major_check_failures += 1u;
      c[compare_index] = row_major_oracle[compare_index];
    }
  }
  free(row_major_oracle);
}

static prom_sgemm_controller_defaults prom_sgemm_default_config(void) {
  prom_sgemm_controller_defaults defaults;
  defaults.lookahead_default = PROM_SGEMM_LOOKAHEAD_DEFAULT;
  defaults.lookahead_min = PROM_SGEMM_LOOKAHEAD_MIN;
  defaults.lookahead_max = PROM_SGEMM_LOOKAHEAD_MAX;
  defaults.outstanding_default = PROM_SGEMM_OUTSTANDING_DEFAULT;
  defaults.outstanding_min = PROM_SGEMM_OUTSTANDING_MIN;
  defaults.outstanding_max = PROM_SGEMM_OUTSTANDING_MAX;
  defaults.chunk_default = PROM_SGEMM_CHUNK_DEFAULT;
  defaults.chunk_min = PROM_SGEMM_CHUNK_MIN;
  defaults.chunk_max = PROM_SGEMM_CHUNK_MAX;
  defaults.waste_budget_units = PROM_SGEMM_WASTE_BUDGET_UNITS;
  defaults.retreat_permille = PROM_SGEMM_RETREAT_PERMILLE;
  defaults.recover_permille = PROM_SGEMM_RECOVER_PERMILLE;
  defaults.recovery_window = PROM_SGEMM_RECOVERY_WINDOW;
  return defaults;
}

static uint32_t prom_subtract_saturating_u32(uint32_t left, uint32_t right) {
  return left > right ? left - right : 0u;
}

static uint32_t prom_sgemm_shape_signature(uint32_t m, uint32_t n, uint32_t k) {
  return (m * 31u) ^ (n * 131u) ^ (k * 521u);
}

static uint32_t prom_sgemm_clamp_u32(uint32_t value, uint32_t min_value, uint32_t max_value) {
  if (value < min_value) {
    return min_value;
  }
  if (value > max_value) {
    return max_value;
  }
  return value;
}

static uint32_t prom_sgemm_waste_proxy_units(uint64_t work_units, uint32_t shape_changed, uint32_t software_vulkan) {
  uint32_t base_units = (uint32_t)(work_units / 65536u);
  if (base_units > 24u) {
    base_units = 24u;
  }
  if (shape_changed != 0u) {
    base_units += 10u;
  }
  if (software_vulkan != 0u) {
    base_units += 4u;
  }
  return base_units;
}

static void prom_sgemm_controller_init(prom_sgemm_controller_state* state) {
  prom_sgemm_controller_defaults defaults;
  if (state == NULL) {
    return;
  }
  memset(state, 0, sizeof(*state));
  defaults = prom_sgemm_default_config();
  prom_policy_memory_init(&state->policy_memory, PROM_POLICY_MODE_AGGRESSIVE);
  state->policy_thresholds.retreat_enter_permille = defaults.retreat_permille;
  state->policy_thresholds.retreat_exit_permille =
      defaults.recover_permille > PROM_SGEMM_HYSTERESIS_MARGIN ? defaults.recover_permille : defaults.recover_permille / 2u;
  state->policy_thresholds.recovery_enter_permille = defaults.retreat_permille + PROM_SGEMM_HYSTERESIS_MARGIN;
  state->policy_thresholds.recovery_exit_permille = defaults.recover_permille;
  state->policy_thresholds.min_commit_decisions = 2u;
  state->policy_thresholds.retreat_cooldown_decisions = defaults.recovery_window;
  state->policy_thresholds.recovery_hold_decisions = defaults.recovery_window;
  state->lookahead = defaults.lookahead_default;
  state->outstanding_depth = defaults.outstanding_default;
  state->chunk_size = defaults.chunk_default;
  state->last_mode = PROM_POLICY_MODE_AGGRESSIVE;
}

static prom_policy_mode prom_sgemm_controller_step(prom_sgemm_controller_state* state,
                                                   uint32_t m,
                                                   uint32_t n,
                                                   uint32_t k,
                                                   uint64_t work_units,
                                                   uint32_t software_vulkan) {
  prom_sgemm_controller_defaults defaults;
  uint32_t signature;
  uint32_t shape_changed;
  uint32_t waste_units;
  uint32_t waste_budget;
  prom_policy_mode mode;
  if (state == NULL) {
    return PROM_POLICY_MODE_AGGRESSIVE;
  }

  defaults = prom_sgemm_default_config();
  signature = prom_sgemm_shape_signature(m, n, k);
  shape_changed = state->last_shape_signature == 0u || state->last_shape_signature != signature ? 1u : 0u;

  waste_units = prom_sgemm_waste_proxy_units(work_units, shape_changed, software_vulkan);
  waste_budget = defaults.waste_budget_units;
  state->wasted_work_units_last = waste_units;
  state->wasted_work_units_total += (uint64_t)waste_units;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if (shape_changed != 0u) {
    state->pending_waste_units += 8u;
    if (state->pending_waste_units > waste_budget) {
      state->pending_waste_units = waste_budget;
    }
    if (state->decision_count != 0u) {
      state->burst_dampening_count += 1u;
    }
  }
  state->pending_waste_units += waste_units / 2u;
  if (state->pending_waste_units > waste_budget) {
    state->pending_waste_units = waste_budget;
  }
  if ((state->pending_waste_units * 1000u) / waste_budget >= defaults.retreat_permille) {
    state->lag_early_warning_count += 1u;
  }

  state->policy_facts.waste_ratio_permille = (waste_units * 1000u) / waste_budget;
  state->policy_facts.pending_waste_ratio_permille = (state->pending_waste_units * 1000u) / waste_budget;
  state->policy_facts.hard_retreat_override = state->pending_waste_units >= waste_budget ? 1u : 0u;
  state->policy_facts.hard_recovery_override = 0u;

  mode = prom_judgment_engine_update_policy_mode(&state->policy_memory, &state->policy_facts, &state->policy_thresholds);
  state->decision_count += 1u;
  if (mode != state->last_mode) {
    state->transition_count += 1u;
    if (state->transition_count > 1u) {
      state->instability_count += 1u;
    }
  }
  if (mode == PROM_POLICY_MODE_AGGRESSIVE) {
    state->aggressive_mode_decisions += 1u;
    state->lookahead = defaults.lookahead_default;
    state->outstanding_depth = defaults.outstanding_default;
    state->chunk_size = defaults.chunk_default;
  } else if (mode == PROM_POLICY_MODE_SAFE) {
    state->safe_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = shape_changed != 0u ? defaults.chunk_min : 12u;
  } else {
    state->recovery_mode_decisions += 1u;
    state->lookahead = 1u;
    state->outstanding_depth = 1u;
    state->chunk_size = 12u;
    if (state->policy_memory.recovery_cooldown_remaining <= 1u) {
      state->lookahead = defaults.lookahead_default;
      state->outstanding_depth = defaults.outstanding_default;
    }
  }
  if (shape_changed != 0u && state->chunk_size > defaults.chunk_min) {
    state->chunk_size -= 2u;
    if (state->chunk_size < defaults.chunk_min) {
      state->chunk_size = defaults.chunk_min;
    }
  }
  state->lookahead = prom_sgemm_clamp_u32(state->lookahead, defaults.lookahead_min, defaults.lookahead_max);
  state->outstanding_depth =
      prom_sgemm_clamp_u32(state->outstanding_depth, defaults.outstanding_min, defaults.outstanding_max);
  state->chunk_size = prom_sgemm_clamp_u32(state->chunk_size, defaults.chunk_min, defaults.chunk_max);
  if (state->lookahead < defaults.lookahead_min || state->lookahead > defaults.lookahead_max ||
      state->outstanding_depth < defaults.outstanding_min || state->outstanding_depth > defaults.outstanding_max ||
      state->chunk_size < defaults.chunk_min || state->chunk_size > defaults.chunk_max) {
    state->bound_violation_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_SAFE && state->last_mode != PROM_POLICY_MODE_SAFE) {
    state->retreat_count += 1u;
  }
  if (mode == PROM_POLICY_MODE_RECOVERY && state->last_mode != PROM_POLICY_MODE_RECOVERY) {
    state->recovery_count += 1u;
  }
  if (state->pending_waste_units >= waste_budget) {
    state->budget_depletion_count += 1u;
  }
  state->pending_waste_units = prom_subtract_saturating_u32(state->pending_waste_units, waste_units);
  state->last_shape_signature = signature;
  state->last_shape_m = m;
  state->last_shape_n = n;
  state->last_shape_k = k;
  state->last_mode = (uint32_t)mode;
  return mode;
}

static int registry_contains(void* handle) {
  size_t i;
  int found = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      found = 1;
      break;
    }
  }
  registry_unlock();
  return found;
}

static int registry_add(void* handle) {
  size_t i;
  int added = 0;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == NULL) {
      g_active_handles[i] = handle;
      added = 1;
      break;
    }
  }
  registry_unlock();
  return added;
}

static void registry_remove(void* handle) {
  size_t i;
  registry_lock();
  for (i = 0; i < PROMETHEUS_MAX_TRACKED_HANDLES; ++i) {
    if (g_active_handles[i] == handle) {
      g_active_handles[i] = NULL;
      break;
    }
  }
  registry_unlock();
}

static int text_contains_llvmpipe(const char* value) {
  size_t i;
  const char* needle = "llvmpipe";
  if (value == NULL) {
    return 0;
  }
  for (i = 0u; value[i] != '\0'; ++i) {
    size_t j = 0u;
    while (needle[j] != '\0') {
      char left = value[i + j];
      char right = needle[j];
      if (left == '\0') {
        break;
      }
      if (left >= 'A' && left <= 'Z') {
        left = (char)(left - 'A' + 'a');
      }
      if (left != right) {
        break;
      }
      ++j;
    }
    if (needle[j] == '\0') {
      return 1;
    }
  }
  return 0;
}

static uint32_t find_memory_type(VkPhysicalDevice physical_device, uint32_t type_filter, VkMemoryPropertyFlags properties) {
  uint32_t i;
  VkPhysicalDeviceMemoryProperties memory_properties;
  vkGetPhysicalDeviceMemoryProperties(physical_device, &memory_properties);
  for (i = 0u; i < memory_properties.memoryTypeCount; ++i) {
    const uint32_t bit = (1u << i);
    if ((type_filter & bit) != 0u &&
        (memory_properties.memoryTypes[i].propertyFlags & properties) == properties) {
      return i;
    }
  }
  return UINT32_MAX;
}

static VkResult create_buffer(prometheus_runtime* rt,
                              VkDeviceSize size,
                              VkBufferUsageFlags usage,
                              VkMemoryPropertyFlags memory_properties,
                              int map_memory,
                              prom_vk_buffer* out_buffer) {
  VkResult result;
  VkBufferCreateInfo buffer_info;
  VkMemoryRequirements requirements;
  VkMemoryAllocateInfo alloc_info;
  uint32_t memory_type_index;

  if (rt == NULL || out_buffer == NULL) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_BUFFER_ALLOC) != 0u) {
    return VK_ERROR_OUT_OF_DEVICE_MEMORY;
  }

  memset(out_buffer, 0, sizeof(*out_buffer));
  out_buffer->size = size;

  memset(&buffer_info, 0, sizeof(buffer_info));
  buffer_info.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
  buffer_info.size = size;
  buffer_info.usage = usage;
  buffer_info.sharingMode = VK_SHARING_MODE_EXCLUSIVE;

  result = vkCreateBuffer(rt->device, &buffer_info, NULL, &out_buffer->buffer);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetBufferMemoryRequirements(rt->device, out_buffer->buffer, &requirements);
  memory_type_index = find_memory_type(rt->physical_device, requirements.memoryTypeBits, memory_properties);
  if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_MEMORY_TYPE) != 0u ||
      (((rt->test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) &&
       (memory_properties & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u)) {
    memory_type_index = UINT32_MAX;
  }
  if (memory_type_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }

  memset(&alloc_info, 0, sizeof(alloc_info));
  alloc_info.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
  alloc_info.allocationSize = requirements.size;
  alloc_info.memoryTypeIndex = memory_type_index;

  result = vkAllocateMemory(rt->device, &alloc_info, NULL, &out_buffer->memory);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkBindBufferMemory(rt->device, out_buffer->buffer, out_buffer->memory, 0);
  if (result != VK_SUCCESS) {
    return result;
  }

  if (map_memory != 0) {
    result = vkMapMemory(rt->device, out_buffer->memory, 0, size, 0, &out_buffer->mapped);
    if (result != VK_SUCCESS) {
      return result;
    }
  }
  return VK_SUCCESS;
}

static void destroy_buffer(prometheus_runtime* rt, prom_vk_buffer* buffer) {
  if (rt == NULL || buffer == NULL || rt->device == VK_NULL_HANDLE) {
    return;
  }
  if (buffer->mapped != NULL) {
    vkUnmapMemory(rt->device, buffer->memory);
    buffer->mapped = NULL;
  }
  if (buffer->buffer != VK_NULL_HANDLE) {
    vkDestroyBuffer(rt->device, buffer->buffer, NULL);
    buffer->buffer = VK_NULL_HANDLE;
  }
  if (buffer->memory != VK_NULL_HANDLE) {
    vkFreeMemory(rt->device, buffer->memory, NULL);
    buffer->memory = VK_NULL_HANDLE;
  }
}

static void destroy_all_execution_buffers(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  destroy_buffer(rt, &rt->direct_c);
  destroy_buffer(rt, &rt->direct_b);
  destroy_buffer(rt, &rt->direct_a);
  destroy_buffer(rt, &rt->staged_readback_c);
  destroy_buffer(rt, &rt->staged_upload_b);
  destroy_buffer(rt, &rt->staged_upload_a);
  destroy_buffer(rt, &rt->staged_device_c);
  destroy_buffer(rt, &rt->staged_device_b);
  destroy_buffer(rt, &rt->staged_device_a);
  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 0u;
  rt->buffer_shape_m = 0u;
  rt->buffer_shape_n = 0u;
  rt->buffer_shape_k = 0u;
}

static int ensure_buffer_capacity(const prom_vk_buffer* buffer, VkDeviceSize required_size) {
  if (buffer == NULL) {
    return 0;
  }
  return buffer->buffer != VK_NULL_HANDLE && buffer->memory != VK_NULL_HANDLE && buffer->size >= required_size;
}

static int ensure_buffer_shape(prometheus_runtime* rt, uint32_t m, uint32_t n, uint32_t k) {
  if (rt == NULL) {
    return 0;
  }
  return rt->buffer_shape_m == m && rt->buffer_shape_n == n && rt->buffer_shape_k == k;
}

static int ensure_direct_execution_buffers(prometheus_runtime* rt,
                                           uint32_t m,
                                           uint32_t n,
                                           uint32_t k,
                                           VkDeviceSize a_buffer_size,
                                           VkDeviceSize b_buffer_size,
                                           VkDeviceSize c_buffer_size,
                                           VkResult* out_result) {
  VkResult result;
  int must_rebuild;

  if (out_result == NULL || rt == NULL) {
    return 0;
  }

  *out_result = VK_SUCCESS;
  /* Shape-only reuse is unsafe: the same logical (m,n,k) can map to different
   * backing byte sizes across compute/layout modes (baseline FP32, packed4,
   * FP16-storage). Reuse is valid only when existing buffer capacity satisfies
   * the current call's required byte sizes. */
  must_rebuild = rt->has_direct_buffers == 0u || !ensure_buffer_shape(rt, m, n, k) ||
                 !ensure_buffer_capacity(&rt->direct_a, a_buffer_size) ||
                 !ensure_buffer_capacity(&rt->direct_b, b_buffer_size) ||
                 !ensure_buffer_capacity(&rt->direct_c, c_buffer_size);
  if (!must_rebuild) {
    return 1;
  }

  destroy_all_execution_buffers(rt);

  result = create_buffer(rt,
                         a_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->direct_a);
  if (result != VK_SUCCESS) {
    *out_result = result;
    return 0;
  }
  result = create_buffer(rt,
                         b_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->direct_b);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }
  result = create_buffer(rt,
                         c_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->direct_c);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }

  rt->buffer_shape_m = m;
  rt->buffer_shape_n = n;
  rt->buffer_shape_k = k;
  rt->has_direct_buffers = 1u;
  rt->has_staged_buffers = 0u;
  return 1;
}

static int ensure_staged_execution_buffers(prometheus_runtime* rt,
                                           uint32_t m,
                                           uint32_t n,
                                           uint32_t k,
                                           VkDeviceSize a_buffer_size,
                                           VkDeviceSize b_buffer_size,
                                           VkDeviceSize c_buffer_size,
                                           VkResult* out_result) {
  VkResult result;
  int must_rebuild;

  if (out_result == NULL || rt == NULL) {
    return 0;
  }

  *out_result = VK_SUCCESS;
  /* Shape-only reuse is unsafe: layout/precision mode transitions can change
   * required upload/device/readback byte sizes without changing logical shape.
   * Reuse staged buffers only when each existing capacity is sufficient. */
  must_rebuild = rt->has_staged_buffers == 0u || !ensure_buffer_shape(rt, m, n, k) ||
                 !ensure_buffer_capacity(&rt->staged_upload_a, a_buffer_size) ||
                 !ensure_buffer_capacity(&rt->staged_upload_b, b_buffer_size) ||
                 !ensure_buffer_capacity(&rt->staged_device_a, a_buffer_size) ||
                 !ensure_buffer_capacity(&rt->staged_device_b, b_buffer_size) ||
                 !ensure_buffer_capacity(&rt->staged_device_c, c_buffer_size) ||
                 !ensure_buffer_capacity(&rt->staged_readback_c, c_buffer_size);
  if (!must_rebuild) {
    return 1;
  }

  destroy_all_execution_buffers(rt);

  result = create_buffer(rt,
                         a_buffer_size,
                         VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->staged_upload_a);
  if (result != VK_SUCCESS) {
    *out_result = result;
    return 0;
  }
  result = create_buffer(rt,
                         b_buffer_size,
                         VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->staged_upload_b);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }
  result = create_buffer(rt,
                         a_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                         VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                         0,
                         &rt->staged_device_a);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }
  result = create_buffer(rt,
                         b_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                         VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                         0,
                         &rt->staged_device_b);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }
  result = create_buffer(rt,
                         c_buffer_size,
                         VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                         VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT,
                         0,
                         &rt->staged_device_c);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }
  result = create_buffer(rt,
                         c_buffer_size,
                         VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                         VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                         1,
                         &rt->staged_readback_c);
  if (result != VK_SUCCESS) {
    *out_result = result;
    destroy_all_execution_buffers(rt);
    return 0;
  }

  rt->buffer_shape_m = m;
  rt->buffer_shape_n = n;
  rt->buffer_shape_k = k;
  rt->has_direct_buffers = 0u;
  rt->has_staged_buffers = 1u;
  return 1;
}

static void vk_runtime_cleanup(prometheus_runtime* rt) {
  if (rt == NULL) {
    return;
  }
  if (rt->device != VK_NULL_HANDLE) {
    vkDeviceWaitIdle(rt->device);
  }
  destroy_all_execution_buffers(rt);
  if (rt->tiled_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->tiled_pipeline, NULL);
    rt->tiled_pipeline = VK_NULL_HANDLE;
  }
  if (rt->packed4_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->packed4_pipeline, NULL);
    rt->packed4_pipeline = VK_NULL_HANDLE;
  }
  if (rt->fp16_pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->fp16_pipeline, NULL);
    rt->fp16_pipeline = VK_NULL_HANDLE;
  }
  if (rt->pipeline != VK_NULL_HANDLE) {
    vkDestroyPipeline(rt->device, rt->pipeline, NULL);
    rt->pipeline = VK_NULL_HANDLE;
  }
  if (rt->pipeline_layout != VK_NULL_HANDLE) {
    vkDestroyPipelineLayout(rt->device, rt->pipeline_layout, NULL);
    rt->pipeline_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_set_layout != VK_NULL_HANDLE) {
    vkDestroyDescriptorSetLayout(rt->device, rt->descriptor_set_layout, NULL);
    rt->descriptor_set_layout = VK_NULL_HANDLE;
  }
  if (rt->descriptor_pool != VK_NULL_HANDLE) {
    vkDestroyDescriptorPool(rt->device, rt->descriptor_pool, NULL);
    rt->descriptor_pool = VK_NULL_HANDLE;
  }
  if (rt->submit_fence != VK_NULL_HANDLE) {
    vkDestroyFence(rt->device, rt->submit_fence, NULL);
    rt->submit_fence = VK_NULL_HANDLE;
  }
  if (rt->command_pool != VK_NULL_HANDLE) {
    vkDestroyCommandPool(rt->device, rt->command_pool, NULL);
    rt->command_pool = VK_NULL_HANDLE;
  }
  if (rt->device != VK_NULL_HANDLE) {
    vkDestroyDevice(rt->device, NULL);
    rt->device = VK_NULL_HANDLE;
  }
  if (rt->instance != VK_NULL_HANDLE) {
    vkDestroyInstance(rt->instance, NULL);
    rt->instance = VK_NULL_HANDLE;
  }
}

static VkResult vk_runtime_init(prometheus_runtime* rt) {
  VkResult result;
  VkInstanceCreateInfo instance_info;
  uint32_t device_count = 0u;
  VkPhysicalDevice devices[16];
  uint32_t i;
  VkDeviceQueueCreateInfo queue_info;
  VkDeviceCreateInfo device_info;
  float queue_priority = 1.0f;
  VkCommandPoolCreateInfo pool_info;
  VkDescriptorSetLayoutBinding bindings[3];
  VkDescriptorSetLayoutCreateInfo set_layout_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkShaderModuleCreateInfo shader_info;
  VkShaderModule shader_module = VK_NULL_HANDLE;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo descriptor_pool_info;
  VkDescriptorSetAllocateInfo set_alloc_info;
  VkCommandBufferAllocateInfo cmd_alloc_info;
  VkFenceCreateInfo fence_info;

  memset(&instance_info, 0, sizeof(instance_info));
  instance_info.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
  result = vkCreateInstance(&instance_info, NULL, &rt->instance);
  if (result != VK_SUCCESS) {
    return result;
  }

  result = vkEnumeratePhysicalDevices(rt->instance, &device_count, NULL);
  if (result != VK_SUCCESS || device_count == 0u) {
    return result == VK_SUCCESS ? VK_ERROR_INITIALIZATION_FAILED : result;
  }

  if (device_count > 16u) {
    device_count = 16u;
  }
  result = vkEnumeratePhysicalDevices(rt->instance, &device_count, devices);
  if (result != VK_SUCCESS) {
    return result;
  }

  rt->physical_device = VK_NULL_HANDLE;
  rt->queue_family_index = UINT32_MAX;
  for (i = 0u; i < device_count; ++i) {
    uint32_t family_count = 0u;
    uint32_t family_index;
    VkQueueFamilyProperties families[32];

    vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &family_count, NULL);
    if (family_count == 0u) {
      continue;
    }
    if (family_count > 32u) {
      family_count = 32u;
    }
    vkGetPhysicalDeviceQueueFamilyProperties(devices[i], &family_count, families);
    for (family_index = 0u; family_index < family_count; ++family_index) {
      if ((families[family_index].queueFlags & VK_QUEUE_COMPUTE_BIT) != 0u) {
        rt->physical_device = devices[i];
        rt->queue_family_index = family_index;
        break;
      }
    }
    if (rt->physical_device != VK_NULL_HANDLE) {
      break;
    }
  }

  if (rt->physical_device == VK_NULL_HANDLE || rt->queue_family_index == UINT32_MAX) {
    return VK_ERROR_FEATURE_NOT_PRESENT;
  }
  {
    VkPhysicalDeviceProperties props;
    VkPhysicalDeviceMemoryProperties memory_props;
    uint32_t memory_index;
    vkGetPhysicalDeviceProperties(rt->physical_device, &props);
    if (props.deviceType == VK_PHYSICAL_DEVICE_TYPE_CPU || text_contains_llvmpipe(props.deviceName)) {
      rt->software_vulkan = 1u;
    } else {
      rt->software_vulkan = 0u;
    }

    rt->has_device_local_memory = 0u;
    rt->has_host_visible_memory = 0u;
    vkGetPhysicalDeviceMemoryProperties(rt->physical_device, &memory_props);
    for (memory_index = 0u; memory_index < memory_props.memoryTypeCount; ++memory_index) {
      VkMemoryPropertyFlags flags = memory_props.memoryTypes[memory_index].propertyFlags;
      if ((flags & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT) != 0u) {
        rt->has_device_local_memory = 1u;
      }
      if ((flags & (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) ==
          (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)) {
        rt->has_host_visible_memory = 1u;
      }
    }
  }
  rt->capability_fp16_storage = ((rt->test_flags & PROM_TESTCFG_FORCE_NO_FP16_STORAGE) == 0u) ? 1u : 0u;

  memset(&queue_info, 0, sizeof(queue_info));
  queue_info.sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
  queue_info.queueFamilyIndex = rt->queue_family_index;
  queue_info.queueCount = 1u;
  queue_info.pQueuePriorities = &queue_priority;

  memset(&device_info, 0, sizeof(device_info));
  device_info.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;
  device_info.queueCreateInfoCount = 1u;
  device_info.pQueueCreateInfos = &queue_info;

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DEVICE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  result = vkCreateDevice(rt->physical_device, &device_info, NULL, &rt->device);
  if (result != VK_SUCCESS) {
    return result;
  }

  vkGetDeviceQueue(rt->device, rt->queue_family_index, 0u, &rt->compute_queue);

  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
  pool_info.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
  pool_info.queueFamilyIndex = rt->queue_family_index;
  result = vkCreateCommandPool(rt->device, &pool_info, NULL, &rt->command_pool);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(bindings, 0, sizeof(bindings));
  bindings[0].binding = 0u;
  bindings[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[0].descriptorCount = 1u;
  bindings[0].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[1].binding = 1u;
  bindings[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[1].descriptorCount = 1u;
  bindings[1].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  bindings[2].binding = 2u;
  bindings[2].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  bindings[2].descriptorCount = 1u;
  bindings[2].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;

  memset(&set_layout_info, 0, sizeof(set_layout_info));
  set_layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  set_layout_info.bindingCount = 3u;
  set_layout_info.pBindings = bindings;
  result = vkCreateDescriptorSetLayout(rt->device, &set_layout_info, NULL, &rt->descriptor_set_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 3u;
  memset(&descriptor_pool_info, 0, sizeof(descriptor_pool_info));
  descriptor_pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  descriptor_pool_info.poolSizeCount = 1u;
  descriptor_pool_info.pPoolSizes = &pool_size;
  descriptor_pool_info.maxSets = 1u;
  result = vkCreateDescriptorPool(rt->device, &descriptor_pool_info, NULL, &rt->descriptor_pool);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&set_alloc_info, 0, sizeof(set_alloc_info));
  set_alloc_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_alloc_info.descriptorPool = rt->descriptor_pool;
  set_alloc_info.descriptorSetCount = 1u;
  set_alloc_info.pSetLayouts = &rt->descriptor_set_layout;
  result = vkAllocateDescriptorSets(rt->device, &set_alloc_info, &rt->descriptor_set);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.offset = 0u;
  push_range.size = PROM_VK_SHADER_PUSH_BYTES;

  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &rt->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  result = vkCreatePipelineLayout(rt->device, &pipeline_layout_info, NULL, &rt->pipeline_layout);
  if (result != VK_SUCCESS) {
    return result;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) {
    return VK_ERROR_INITIALIZATION_FAILED;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_spirv);
  shader_info.pCode = k_prom_sgemm_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_tiled_spirv);
  shader_info.pCode = k_prom_sgemm_tiled_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->tiled_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_packed4_spirv);
  shader_info.pCode = k_prom_sgemm_packed4_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->packed4_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&shader_info, 0, sizeof(shader_info));
  shader_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  shader_info.codeSize = sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv);
  shader_info.pCode = k_prom_sgemm_fp16_storage_fp32accum_spirv;
  result = vkCreateShaderModule(rt->device, &shader_info, NULL, &shader_module);
  if (result != VK_SUCCESS) {
    return result;
  }
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = shader_module;
  stage_info.pName = "main";

  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = rt->pipeline_layout;
  result = vkCreateComputePipelines(rt->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &rt->fp16_pipeline);
  vkDestroyShaderModule(rt->device, shader_module, NULL);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&cmd_alloc_info, 0, sizeof(cmd_alloc_info));
  cmd_alloc_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  cmd_alloc_info.commandPool = rt->command_pool;
  cmd_alloc_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  cmd_alloc_info.commandBufferCount = 1u;
  result = vkAllocateCommandBuffers(rt->device, &cmd_alloc_info, &rt->command_buffer);
  if (result != VK_SUCCESS) {
    return result;
  }

  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  result = vkCreateFence(rt->device, &fence_info, NULL, &rt->submit_fence);
  return result;
}

int prom_reactor_runtime_create_impl(void* config, void** out_handle) {
  VkResult result;
  prometheus_runtime* runtime;
  (void)config;

  if (out_handle == NULL) {
    return PROM_ERROR;
  }

  *out_handle = NULL;
  runtime = (prometheus_runtime*)malloc(sizeof(prometheus_runtime));
  if (runtime == NULL) {
    return PROM_INTERNAL_ERROR;
  }
  memset(runtime, 0, sizeof(*runtime));
  runtime->magic = PROMETHEUS_RUNTIME_MAGIC;
  runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
  prom_sgemm_controller_init(&runtime->sgemm_controller);
  prom_slot_hfsm_init(&runtime->slots[0], 0u);
  prom_slot_hfsm_init(&runtime->slots[1], 1u);
  runtime->slot_diag.current_slot_id = UINT32_MAX;
  runtime->slot_diag.next_slot_id = 0u;
  runtime->slot_diag.failure_slot_id = -1;
  runtime->slot_diag.async_slot_id = -1;

  if (config != NULL) {
    const PrometheusReactorConfig* cfg = (const PrometheusReactorConfig*)config;
    if (cfg->struct_size >= sizeof(PrometheusReactorConfig)) {
      runtime->test_flags = cfg->test_flags;
    }
  }

  if ((runtime->test_flags & PROM_TESTCFG_SKIP_VULKAN_INIT) != 0u) {
    runtime->available = 0u;
    runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
    runtime->init_detail_code = (int)VK_ERROR_INITIALIZATION_FAILED;
  } else {
    result = vk_runtime_init(runtime);
    if (result == VK_SUCCESS) {
      runtime->available = 1u;
      runtime->reason_code = PROM_REASON_NONE;
      runtime->init_detail_code = 0;
    } else {
      runtime->available = 0u;
      runtime->reason_code = PROM_REASON_VULKAN_UNAVAILABLE;
      runtime->init_detail_code = (int)result;
      vk_runtime_cleanup(runtime);
    }
  }

  if (!registry_add(runtime)) {
    vk_runtime_cleanup(runtime);
    free(runtime);
    return PROM_INTERNAL_ERROR;
  }

  *out_handle = runtime;
  return PROM_OK;
}

int prom_reactor_runtime_destroy_impl(void* handle) {
  prometheus_runtime* runtime;
  if (handle == NULL) {
    return PROM_OK;
  }
  if (!registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  if (runtime->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  registry_remove(handle);
  vk_runtime_cleanup(runtime);
  free(runtime);
  return PROM_OK;
}

int prom_reactor_runtime_probe_impl(void* handle, PrometheusCaps* out_caps) {
  prometheus_runtime* runtime;
  if (out_caps == NULL) {
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }

  runtime = (prometheus_runtime*)handle;
  out_caps->available = runtime->available;
  if (runtime->available == 0u) {
    out_caps->backend_type = PROM_BACKEND_UNKNOWN;
  } else if (runtime->software_vulkan != 0u) {
    out_caps->backend_type = PROM_BACKEND_VULKAN_SOFTWARE;
  } else {
    out_caps->backend_type = PROM_BACKEND_VULKAN;
  }
  out_caps->reason_code = runtime->reason_code;
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_impl(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  prometheus_runtime* rt;
  VkResult vk_result;
  VkWriteDescriptorSet writes[3];
  VkDescriptorBufferInfo buffer_infos[3];
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submit_info;
  VkBufferMemoryBarrier barriers[4];
  VkBufferCopy copies[3];
  prom_vk_push push;
  prom_vk_buffer* shader_a;
  prom_vk_buffer* shader_b;
  prom_vk_buffer* shader_c;
  VkDeviceSize a_buffer_size;
  VkDeviceSize b_buffer_size;
  VkDeviceSize c_buffer_size;
  size_t a_copy_size;
  size_t b_copy_size;
  size_t c_copy_size;
  uint32_t compute_k;
  uint64_t work_units;
  uint32_t can_stage;
  uint32_t can_direct;
  uint32_t tiled_shape;
  uint32_t readback_required;
  uint32_t packed4_waste_permille;
  uint32_t packed4_budget_permille;
  uint32_t packed4_small_shape;
  uint32_t packed4_tail_count;
  uint32_t packed4_padded_lane_count;
  uint32_t fp16_has_special_values;
  int fp16_utility_score;
  prom_policy_mode policy_mode;
  prom_vk_path_mode selected_path;
  prom_vk_compute_mode compute_mode;
  prom_judgment_facts judgment_facts;
  prom_judgment_decision judgment_decision;
  prom_judgment_async_facts async_facts;
  prom_judgment_async_decision async_decision;
  VkPipeline selected_pipeline;
  float* packed_a_upload = NULL;
  float* packed_b_upload = NULL;
  uint32_t* fp16_a_upload = NULL;
  uint32_t* fp16_b_upload = NULL;
  int final_detail = 0;
  uint32_t request_async = 0u;
  uint32_t work_slot_id = 0u;
  uint64_t required_capacity_bytes = 0u;

  set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);

  if (handle == NULL || !registry_contains(handle)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  request_async = (((rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u) && c == NULL) ? 1u : 0u;
  if (a == NULL || b == NULL || (request_async == 0u && c == NULL)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
    return PROM_ERROR;
  }
  if (m == 0u || n == 0u || k == 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (rt->available == 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, rt->init_detail_code);
    return PROM_ERROR;
  }
  set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, 0);
  if (rt->async_state == PROM_ASYNC_STATE_CONSUMED) {
    set_async_state(rt, PROM_ASYNC_STATE_IDLE, PROM_STAGE_NONE, 0);
  }
  if (rt->async_state == PROM_ASYNC_STATE_FAILED) {
    set_status(out_stage,
               out_detail_code,
               PROM_STAGE_SUBMIT,
               rt->async_failure_detail != 0 ? rt->async_failure_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED || rt->async_state == PROM_ASYNC_STATE_READY) {
    rt->slot_diag.inflight_rejection_count += 1u;
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_UNCONSUMED);
    return PROM_ERROR;
  }
  if (rt->in_flight_submit != 0u) {
    vk_result = vkGetFenceStatus(rt->device, rt->submit_fence);
    if (vk_result == VK_SUCCESS) {
      rt->in_flight_submit = 0u;
    } else {
      rt->slot_diag.inflight_rejection_count += 1u;
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
      return PROM_ERROR;
    }
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_INJECTED_UPLOAD_FAILURE);
    return PROM_ERROR;
  }

  can_stage = rt->has_device_local_memory;
  can_direct = rt->has_host_visible_memory;
  if ((rt->test_flags & PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY) != 0u) {
    can_stage = 0u;
  }
  readback_required = ((rt->test_flags & PROM_TESTCFG_FORCE_UPLOAD_ONLY) == 0u) ? 1u : 0u;
  work_units = (uint64_t)m * (uint64_t)n * (uint64_t)k;
  policy_mode = prom_sgemm_controller_step(&rt->sgemm_controller, m, n, k, work_units, rt->software_vulkan);
  packed4_waste_permille = prom_packed4_padding_waste_permille(m, n, k);
  packed4_budget_permille = prom_packed4_mode_budget_permille(policy_mode);
  packed4_small_shape = (m < 4u || n < 4u || k < 4u) ? 1u : 0u;
  packed4_tail_count = prom_packed4_tail_count(m, n, k);
  packed4_padded_lane_count = (uint32_t)((prom_round_up4_u32(k) - k) * (m + n));
  fp16_has_special_values = 0u;
  fp16_utility_score = -1000;
  prom_fp16_evaluate_tolerance(a, b, m, n, k, &rt->sgemm_controller, &fp16_has_special_values, &fp16_utility_score);
  if ((rt->test_flags & PROM_TESTCFG_FORCE_FP16_UTILITY_WIN) != 0u) {
    fp16_utility_score = 1201;
  }
  tiled_shape = (work_units >= (uint64_t)PROM_JUDGMENT_TILED_WORK_THRESHOLD && m >= PROM_VK_LOCAL_SIZE_X &&
                 n >= PROM_VK_LOCAL_SIZE_Y && k >= PROM_VK_TILE_K)
                    ? 1u
                    : 0u;
  memset(&judgment_facts, 0, sizeof(judgment_facts));
  judgment_facts.m = m;
  judgment_facts.n = n;
  judgment_facts.k = k;
  judgment_facts.work_units = work_units;
  judgment_facts.can_stage = can_stage;
  judgment_facts.can_direct = can_direct;
  judgment_facts.allow_fallback = ((rt->test_flags & PROM_TESTCFG_DISABLE_STAGING_FALLBACK) == 0u) ? 1u : 0u;
  judgment_facts.readback_required = readback_required;
  judgment_facts.force_direct = ((rt->test_flags & PROM_TESTCFG_FORCE_DIRECT_PATH) != 0u) ? 1u : 0u;
  if (judgment_facts.force_direct == 0u && policy_mode == PROM_POLICY_MODE_SAFE &&
      (rt->test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) == 0u && (rt->test_flags & PROM_TESTCFG_FORCE_TILED_PATH) == 0u) {
    /* SAFE mode currently biases to direct+baseline for conservative behavior.
     * This can suppress direct+tiled on large shapes; keep unchanged in this pass
     * and revisit with real GPU validation data before any policy relaxation. */
    judgment_facts.force_direct = 1u;
  }
  judgment_facts.force_staged = ((rt->test_flags & PROM_TESTCFG_FORCE_STAGED_PATH) != 0u) ? 1u : 0u;
  judgment_facts.force_tiled = ((rt->test_flags & PROM_TESTCFG_FORCE_TILED_PATH) != 0u) ? 1u : 0u;
  judgment_facts.tiled_shape = tiled_shape;
  judgment_facts.software_vulkan = rt->software_vulkan;
  judgment_facts.policy_mode = policy_mode;
  judgment_facts.packed4_available = 1u;
  judgment_facts.packed4_small_shape = packed4_small_shape;
  judgment_facts.packed4_padding_waste_permille = packed4_waste_permille;
  judgment_facts.packed4_mode_budget_permille = packed4_budget_permille;
  judgment_facts.packed4_row_major_valid = 1u;
  judgment_facts.packed4_tail_valid = 1u;
  judgment_facts.strict_fp32 = ((rt->test_flags & PROM_TESTCFG_FORCE_STRICT_FP32) != 0u) ? 1u : 0u;
  judgment_facts.tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  judgment_facts.tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  judgment_facts.has_special_values = fp16_has_special_values;
  judgment_facts.capability_fp16_storage = rt->capability_fp16_storage;
  judgment_facts.fallback_available = (judgment_facts.allow_fallback != 0u && judgment_facts.can_direct != 0u) ? 1u : 0u;
  judgment_facts.fp16_utility_score = fp16_utility_score;
  prom_judgment_engine_select_sgemm_mode(&judgment_facts, &judgment_decision);
  if (judgment_decision.success == 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, judgment_decision.error_detail);
    return PROM_ERROR;
  }
  selected_path = judgment_decision.selected_path;
  compute_mode = judgment_decision.compute_mode;
  final_detail = judgment_decision.final_detail;
  rt->sgemm_controller.packed4_tail_count_last = packed4_tail_count;
  rt->sgemm_controller.packed4_padded_lane_count_last = packed4_padded_lane_count;
  rt->sgemm_controller.packed4_padding_waste_permille_last = packed4_waste_permille;
  if (judgment_decision.packed4_reject_reason != PROM_PACKED4_REJECT_NONE) {
    prom_packed4_record_fallback(&rt->sgemm_controller, judgment_decision.packed4_reject_reason);
  }
  rt->sgemm_controller.fp16_fallback_reason_detail = prom_fp16_reject_reason_to_detail(judgment_decision.fp16_reject_reason);
  rt->sgemm_controller.fp16_selected_candidate = judgment_decision.fp16_selected != 0u ? 3u : 1u;

  compute_k = compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? prom_round_up4_u32(k) : k;
  if ((compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_packed_fp16_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_packed_fp16_buffer_size(k, n, &b_buffer_size, &b_copy_size))) ||
      (compute_mode != PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM &&
       (!checked_float_buffer_size(m, compute_k, &a_buffer_size, &a_copy_size) ||
        !checked_float_buffer_size(compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? n : k,
                                   compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? compute_k : n,
                                   &b_buffer_size,
                                   &b_copy_size))) ||
      !checked_float_buffer_size(m, n, &c_buffer_size, &c_copy_size)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SIZE_OVERFLOW);
    return PROM_ERROR;
  }
  required_capacity_bytes = (uint64_t)a_buffer_size + (uint64_t)b_buffer_size + (uint64_t)c_buffer_size;
  work_slot_id = rt->slot_diag.next_slot_id < 2u ? rt->slot_diag.next_slot_id : 0u;
  if (!prom_slot_prepare_for_call(rt,
                                  work_slot_id,
                                  m,
                                  n,
                                  compute_k,
                                  prom_slot_compute_layout_code(selected_path, compute_mode),
                                  (uint32_t)compute_mode,
                                  required_capacity_bytes)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_DETAIL_SLOT_OVERWRITE_REJECTED);
    return PROM_ERROR;
  }
  if (!prom_slot_swap_ready_to_current(rt, work_slot_id)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_SWAP_REJECTED);
    return PROM_ERROR;
  }

  if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    packed_a_upload = (float*)malloc(a_copy_size);
    packed_b_upload = (float*)malloc(b_copy_size);
    if (packed_a_upload == NULL || packed_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_a_packed4_rowmajor(a, packed_a_upload, m, k, compute_k);
    prom_pack_b_packed4_colmajor(b, packed_b_upload, n, k, compute_k);
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    fp16_a_upload = (uint32_t*)malloc(a_copy_size);
    fp16_b_upload = (uint32_t*)malloc(b_copy_size);
    if (fp16_a_upload == NULL || fp16_b_upload == NULL) {
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, PROM_ERROR);
      return PROM_ERROR;
    }
    prom_pack_fp16_pairs(a, m * compute_k, fp16_a_upload);
    prom_pack_fp16_pairs(b, compute_k * n, fp16_b_upload);
  }
  memset(&async_facts, 0, sizeof(async_facts));
  async_facts.request_async = request_async;
  async_facts.in_flight = rt->in_flight_submit;
  async_facts.software_vulkan = rt->software_vulkan;
  prom_judgment_engine_select_async_submission(&async_facts, &async_decision);
  if (async_decision.success == 0u) {
    free(packed_a_upload);
    free(packed_b_upload);
    free(fp16_a_upload);
    free(fp16_b_upload);
    prom_slot_mark_failure(rt, work_slot_id, async_decision.reject_detail);
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, async_decision.reject_detail);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    if (!ensure_direct_execution_buffers(rt, m, n, compute_k, a_buffer_size, b_buffer_size, c_buffer_size, &vk_result)) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, (int)vk_result);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->direct_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->direct_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    memset(rt->direct_c.mapped, 0, c_copy_size);
    shader_a = &rt->direct_a;
    shader_b = &rt->direct_b;
    shader_c = &rt->direct_c;
  } else {
    if (!ensure_staged_execution_buffers(rt, m, n, compute_k, a_buffer_size, b_buffer_size, c_buffer_size, &vk_result)) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_IN, (int)vk_result);
      free(packed_a_upload);
      free(packed_b_upload);
      free(fp16_a_upload);
      free(fp16_b_upload);
      return PROM_ERROR;
    }
    memcpy(rt->staged_upload_a.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_a_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_a_upload : (const void*)a),
           a_copy_size);
    memcpy(rt->staged_upload_b.mapped,
           compute_mode == PROM_VK_COMPUTE_PACKED4_FP32 ? (const void*)packed_b_upload
           : (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM ? (const void*)fp16_b_upload : (const void*)b),
           b_copy_size);
    shader_a = &rt->staged_device_a;
    shader_b = &rt->staged_device_b;
    shader_c = &rt->staged_device_c;
  }
  free(packed_a_upload);
  free(packed_b_upload);
  free(fp16_a_upload);
  free(fp16_b_upload);

  if (compute_mode == PROM_VK_COMPUTE_TILED) {
    selected_pipeline = rt->tiled_pipeline;
    if (selected_path == PROM_VK_PATH_DIRECT) {
      final_detail = PROM_DETAIL_PATH_DIRECT_TILED;
    } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD) {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_TILED;
    } else {
      final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
    }
  } else if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
    selected_pipeline = rt->packed4_pipeline;
  } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
    selected_pipeline = rt->fp16_pipeline;
  } else {
    selected_pipeline = rt->pipeline;
  }

  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = shader_a->buffer;
  buffer_infos[0].offset = 0;
  buffer_infos[0].range = shader_a->size;
  buffer_infos[1].buffer = shader_b->buffer;
  buffer_infos[1].offset = 0;
  buffer_infos[1].range = shader_b->size;
  buffer_infos[2].buffer = shader_c->buffer;
  buffer_infos[2].offset = 0;
  buffer_infos[2].range = shader_c->size;

  memset(writes, 0, sizeof(writes));
  writes[0].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
  writes[0].dstSet = rt->descriptor_set;
  writes[0].dstBinding = 0u;
  writes[0].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  writes[0].descriptorCount = 1u;
  writes[0].pBufferInfo = &buffer_infos[0];
  writes[1] = writes[0];
  writes[1].dstBinding = 1u;
  writes[1].pBufferInfo = &buffer_infos[1];
  writes[2] = writes[0];
  writes[2].dstBinding = 2u;
  writes[2].pBufferInfo = &buffer_infos[2];
  vkUpdateDescriptorSets(rt->device, 3u, writes, 0u, NULL);

  vk_result = vkResetCommandBuffer(rt->command_buffer, 0u);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  vk_result = vkBeginCommandBuffer(rt->command_buffer, &begin_info);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(barriers, 0, sizeof(barriers));
  if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->direct_b.buffer;
    barriers[1].size = rt->direct_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->direct_c.buffer;
    barriers[2].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         3u,
                         barriers,
                         0,
                         NULL);
  } else {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_upload_a.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_upload_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_upload_b.buffer;
    barriers[1].size = rt->staged_upload_b.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         2u,
                         barriers,
                         0,
                         NULL);

    memset(copies, 0, sizeof(copies));
    copies[0].size = rt->staged_upload_a.size;
    copies[1].size = rt->staged_upload_b.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_a.buffer, rt->staged_device_a.buffer, 1u, &copies[0]);
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_upload_b.buffer, rt->staged_device_b.buffer, 1u, &copies[1]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[0].buffer = rt->staged_device_a.buffer;
    barriers[0].size = rt->staged_device_a.size;
    barriers[1] = barriers[0];
    barriers[1].buffer = rt->staged_device_b.buffer;
    barriers[1].size = rt->staged_device_b.size;
    barriers[2] = barriers[0];
    barriers[2].dstAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[2].buffer = rt->staged_device_c.buffer;
    barriers[2].size = rt->staged_device_c.size;
    /* Staged device-local C is not pre-zeroed: current SGEMM kernels overwrite every final C element. */
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0,
                         0,
                         NULL,
                         3u,
                         barriers,
                         0,
                         NULL);
  }

  vkCmdBindPipeline(rt->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, selected_pipeline);
  vkCmdBindDescriptorSets(rt->command_buffer,
                          VK_PIPELINE_BIND_POINT_COMPUTE,
                          rt->pipeline_layout,
                          0u,
                          1u,
                          &rt->descriptor_set,
                          0u,
                          NULL);

  push.m = m;
  push.n = n;
  push.k = compute_k;
  vkCmdPushConstants(rt->command_buffer,
                     rt->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT,
                     0u,
                     PROM_VK_SHADER_PUSH_BYTES,
                     &push);

  set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, 0);
  if ((rt->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_INJECTED_DISPATCH_FAILURE);
    return PROM_ERROR;
  }

  /* Dispatch/indexing contract: x maps rows (m), y maps columns (n); host and shader must match this. */
  vkCmdDispatch(rt->command_buffer,
                (m + (PROM_VK_LOCAL_SIZE_X - 1u)) / PROM_VK_LOCAL_SIZE_X,
                (n + (PROM_VK_LOCAL_SIZE_Y - 1u)) / PROM_VK_LOCAL_SIZE_Y,
                1u);

  if (selected_path == PROM_VK_PATH_DIRECT) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->direct_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->direct_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[0].buffer = rt->staged_device_c.buffer;
    barriers[0].offset = 0;
    barriers[0].size = rt->staged_device_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);

    copies[2].size = rt->staged_readback_c.size;
    vkCmdCopyBuffer(rt->command_buffer, rt->staged_device_c.buffer, rt->staged_readback_c.buffer, 1u, &copies[2]);

    barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[0].dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    barriers[0].buffer = rt->staged_readback_c.buffer;
    barriers[0].size = rt->staged_readback_c.size;
    vkCmdPipelineBarrier(rt->command_buffer,
                         VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT,
                         0,
                         0,
                         NULL,
                         1u,
                         barriers,
                         0,
                         NULL);
  }

  vk_result = vkEndCommandBuffer(rt->command_buffer);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  vk_result = vkResetFences(rt->device, 1u, &rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }

  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &rt->command_buffer;
  vk_result = vkQueueSubmit(rt->compute_queue, 1u, &submit_info, rt->submit_fence);
  if (vk_result != VK_SUCCESS) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
    return PROM_ERROR;
  }
  rt->in_flight_submit = 1u;
  if (!prom_slot_mark_submitted(rt, work_slot_id)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
    return PROM_ERROR;
  }

  if (async_decision.execute_async != 0u) {
    rt->async_task_id += 1;
    rt->async_m = m;
    rt->async_n = n;
    rt->async_k = k;
    rt->async_c_copy_size = c_copy_size;
    rt->async_selected_path = selected_path;
    rt->async_final_detail = final_detail;
    set_async_state(rt, PROM_ASYNC_STATE_SUBMITTED, PROM_STAGE_SUBMIT, 0);
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
    return PROM_OK;
  }

  if ((rt->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) == 0u) {
    vk_result = vkWaitForFences(rt->device, 1u, &rt->submit_fence, VK_TRUE, UINT64_MAX);
    if (vk_result != VK_SUCCESS) {
      prom_slot_mark_failure(rt, work_slot_id, (int)vk_result);
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, (int)vk_result);
      return PROM_ERROR;
    }
    rt->in_flight_submit = 0u;
    if (!prom_slot_mark_complete(rt, work_slot_id)) {
      prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
  } else {
    prom_slot_mark_failure(rt, work_slot_id, PROM_DETAIL_REUSE_IN_FLIGHT);
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_REUSE_IN_FLIGHT);
    return PROM_ERROR;
  }

  if ((rt->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE);
    return PROM_ERROR;
  }

  if (selected_path == PROM_VK_PATH_DIRECT) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->direct_c.mapped, m, n, k);
    }
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->direct_c.mapped, c_copy_size);
  } else if (selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      prom_apply_debug_row_major_oracle(rt, a, b, (float*)rt->staged_readback_c.mapped, m, n, k);
    }
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, final_detail);
    memcpy(c, rt->staged_readback_c.mapped, c_copy_size);
  } else {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, final_detail);
  }

  if (out_stage != NULL &&
      out_detail_code != NULL &&
      ((*out_stage == PROM_STAGE_TRANSFER_OUT) || (selected_path == PROM_VK_PATH_STAGED_UPLOAD && *out_stage == PROM_STAGE_SUBMIT))) {
    if (compute_mode == PROM_VK_COMPUTE_PACKED4_FP32) {
      rt->sgemm_controller.packed4_selected_layout_format = 2u;
      rt->sgemm_controller.packed4_tail_count_total += (uint64_t)packed4_tail_count;
      rt->sgemm_controller.packed4_padded_lane_count_total += (uint64_t)packed4_padded_lane_count;
      rt->sgemm_controller.packed4_selection_count += 1u;
      rt->sgemm_controller.fp16_selected_candidate = 2u;
    } else if (compute_mode == PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM) {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 3u;
      rt->sgemm_controller.fp16_fallback_reason_detail = 0;
    } else {
      rt->sgemm_controller.packed4_selected_layout_format = 1u;
      rt->sgemm_controller.fp16_selected_candidate = 1u;
    }
    return PROM_OK;
  }
  return PROM_ERROR;
}

int prom_reactor_runtime_sgemm_submit_async_impl(void* handle,
                                                 const float* a,
                                                 const float* b,
                                                 uint32_t m,
                                                 uint32_t n,
                                                 uint32_t k,
                                                 int* out_task_id,
                                                 uint32_t* out_stage,
                                                 int* out_detail_code) {
  prometheus_runtime* rt;
  uint32_t saved_flags;
  int status;

  if (out_task_id == NULL) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_ERROR);
    return PROM_ERROR;
  }
  if (handle == NULL || !registry_contains(handle)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }

  rt = (prometheus_runtime*)handle;
  saved_flags = rt->test_flags;
  rt->test_flags = saved_flags | PROM_TESTCFG_SKIP_SUBMIT_WAIT;
  status = prom_reactor_runtime_sgemm_impl(handle, a, b, NULL, m, n, k, out_stage, out_detail_code);
  rt->test_flags = saved_flags;
  if (status != PROM_OK) {
    return status;
  }

  *out_task_id = rt->async_task_id;
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_query_async_impl(void* handle, int task_id, PrometheusAsyncStatus* out_status) {
  prometheus_runtime* rt;

  if (out_status == NULL) {
    return PROM_ERROR;
  }
  memset(out_status, 0, sizeof(*out_status));

  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    out_status->lifecycle_state = PROM_ASYNC_STATE_IDLE;
    out_status->detail_code = PROM_DETAIL_ASYNC_NO_TASK;
    return PROM_ERROR;
  }

  update_async_progress(rt);
  out_status->lifecycle_state = rt->async_state;
  out_status->stage = rt->async_stage;
  out_status->detail_code = rt->async_state == PROM_ASYNC_STATE_FAILED ? rt->async_failure_detail : rt->async_final_detail;
  out_status->ready = rt->async_state == PROM_ASYNC_STATE_READY ? 1u : 0u;
  out_status->failed = rt->async_state == PROM_ASYNC_STATE_FAILED ? 1u : 0u;
  out_status->consumed = rt->async_state == PROM_ASYNC_STATE_CONSUMED ? 1u : 0u;
  out_status->outstanding_tasks = rt->async_state == PROM_ASYNC_STATE_SUBMITTED ? 1u : 0u;
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_consume_async_impl(void* handle,
                                                  int task_id,
                                                  float* c,
                                                  uint32_t c_len,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code) {
  prometheus_runtime* rt;
  uint32_t required_len;

  set_status(out_stage, out_detail_code, PROM_STAGE_NONE, 0);
  if (handle == NULL || !registry_contains(handle)) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE);
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_INVALID_TASK);
    return PROM_ERROR;
  }
  update_async_progress(rt);
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, PROM_DETAIL_ASYNC_NOT_READY);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_FAILED) {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, rt->async_failure_detail != 0 ? rt->async_failure_detail : PROM_DETAIL_ASYNC_FAILED);
    return PROM_ERROR;
  }
  if (rt->async_state == PROM_ASYNC_STATE_CONSUMED) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_DETAIL_ASYNC_ALREADY_CONSUMED);
    return PROM_ERROR;
  }
  if (c == NULL) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }
  required_len = rt->async_m * rt->async_n;
  if (c_len < required_len) {
    set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, PROM_ERROR);
    return PROM_ERROR;
  }

  if (rt->async_selected_path == PROM_VK_PATH_DIRECT) {
    memcpy(c, rt->direct_c.mapped, rt->async_c_copy_size);
  } else if (rt->async_selected_path == PROM_VK_PATH_STAGED_UPLOAD_READBACK) {
    memcpy(c, rt->staged_readback_c.mapped, rt->async_c_copy_size);
  } else {
    set_status(out_stage, out_detail_code, PROM_STAGE_SUBMIT, rt->async_final_detail);
    set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, PROM_STAGE_SUBMIT, rt->async_final_detail);
    return PROM_OK;
  }

  set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, PROM_STAGE_TRANSFER_OUT, rt->async_final_detail);
  set_status(out_stage, out_detail_code, PROM_STAGE_TRANSFER_OUT, rt->async_final_detail);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_abandon_async_impl(void* handle, int task_id) {
  prometheus_runtime* rt;
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }
  if (task_id != rt->async_task_id || rt->async_state == PROM_ASYNC_STATE_IDLE) {
    return PROM_ERROR;
  }
  update_async_progress(rt);
  if (rt->async_state == PROM_ASYNC_STATE_SUBMITTED) {
    rt->slot_diag.inflight_rejection_count += 1u;
    return PROM_ERROR;
  }
  if (rt->slot_diag.async_slot_id >= 0) {
    const uint32_t slot_id = (uint32_t)rt->slot_diag.async_slot_id;
    if (!prom_slot_cleanup_to_empty(&rt->slots[slot_id])) {
      prom_slot_mark_failure(rt, slot_id, PROM_DETAIL_SLOT_ASYNC_OWNERSHIP);
      return PROM_ERROR;
    }
    rt->slot_diag.async_slot_id = -1;
  }
  set_async_state(rt, PROM_ASYNC_STATE_CONSUMED, rt->async_stage, rt->async_final_detail);
  return PROM_OK;
}

int prom_reactor_runtime_sgemm_policy_diagnostics_impl(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  const prom_sgemm_controller_defaults defaults = prom_sgemm_default_config();
  prometheus_runtime* rt;
  if (out_diag == NULL) {
    return PROM_ERROR;
  }
  memset(out_diag, 0, sizeof(*out_diag));
  if (handle == NULL || !registry_contains(handle)) {
    return PROM_INVALID_HANDLE;
  }
  rt = (prometheus_runtime*)handle;
  if (rt->magic != PROMETHEUS_RUNTIME_MAGIC) {
    return PROM_INVALID_HANDLE;
  }

  out_diag->current_mode = (uint32_t)rt->sgemm_controller.policy_memory.current_mode;
  out_diag->lookahead = rt->sgemm_controller.lookahead;
  out_diag->outstanding_depth = rt->sgemm_controller.outstanding_depth;
  out_diag->chunk_size = rt->sgemm_controller.chunk_size;
  out_diag->chunk_min = defaults.chunk_min;
  out_diag->chunk_max = defaults.chunk_max;
  out_diag->waste_budget_units = defaults.waste_budget_units;
  out_diag->pending_waste_units = rt->sgemm_controller.pending_waste_units;
  out_diag->wasted_work_units_last = rt->sgemm_controller.wasted_work_units_last;
  out_diag->wasted_work_units_total = rt->sgemm_controller.wasted_work_units_total;
  out_diag->decision_count = rt->sgemm_controller.decision_count;
  out_diag->retreat_count = rt->sgemm_controller.retreat_count;
  out_diag->recovery_count = rt->sgemm_controller.recovery_count;
  out_diag->transition_count = rt->sgemm_controller.transition_count;
  out_diag->instability_count = rt->sgemm_controller.instability_count;
  out_diag->budget_depletion_count = rt->sgemm_controller.budget_depletion_count;
  out_diag->safe_mode_decisions = rt->sgemm_controller.safe_mode_decisions;
  out_diag->aggressive_mode_decisions = rt->sgemm_controller.aggressive_mode_decisions;
  out_diag->recovery_mode_decisions = rt->sgemm_controller.recovery_mode_decisions;
  out_diag->lag_early_warning_count = rt->sgemm_controller.lag_early_warning_count;
  out_diag->burst_dampening_count = rt->sgemm_controller.burst_dampening_count;
  out_diag->bound_violation_count = rt->sgemm_controller.bound_violation_count;
  out_diag->packed4_selected_layout_format = rt->sgemm_controller.packed4_selected_layout_format;
  out_diag->packed4_tail_count_last = rt->sgemm_controller.packed4_tail_count_last;
  out_diag->packed4_tail_count_total = rt->sgemm_controller.packed4_tail_count_total;
  out_diag->packed4_padded_lane_count_last = rt->sgemm_controller.packed4_padded_lane_count_last;
  out_diag->packed4_padded_lane_count_total = rt->sgemm_controller.packed4_padded_lane_count_total;
  out_diag->packed4_padding_waste_permille_last = rt->sgemm_controller.packed4_padding_waste_permille_last;
  out_diag->packed4_mode_budget_denials = rt->sgemm_controller.packed4_mode_budget_denials;
  out_diag->packed4_row_major_check_failures = rt->sgemm_controller.packed4_row_major_check_failures;
  out_diag->packed4_selection_count = rt->sgemm_controller.packed4_selection_count;
  out_diag->packed4_fallback_reason_padding_waste = rt->sgemm_controller.packed4_fallback_reason_padding_waste;
  out_diag->packed4_fallback_reason_small_shape = rt->sgemm_controller.packed4_fallback_reason_small_shape;
  out_diag->packed4_fallback_reason_capability_missing = rt->sgemm_controller.packed4_fallback_reason_capability_missing;
  out_diag->packed4_fallback_reason_fallback_required = rt->sgemm_controller.packed4_fallback_reason_fallback_required;
  out_diag->packed4_fallback_reason_mode_budget_denied = rt->sgemm_controller.packed4_fallback_reason_mode_budget_denied;
  out_diag->fp16_max_absolute_error = rt->sgemm_controller.fp16_max_absolute_error;
  out_diag->fp16_max_relative_error = rt->sgemm_controller.fp16_max_relative_error;
  out_diag->fp16_aggregate_error = rt->sgemm_controller.fp16_aggregate_error;
  out_diag->fp16_worst_case_element_index = rt->sgemm_controller.fp16_worst_case_element_index;
  out_diag->fp16_k_error_growth = rt->sgemm_controller.fp16_k_error_growth;
  out_diag->fp16_cancellation_risk = rt->sgemm_controller.fp16_cancellation_risk;
  out_diag->fp16_tolerance_known = rt->sgemm_controller.fp16_tolerance_known;
  out_diag->fp16_tolerance_pass = rt->sgemm_controller.fp16_tolerance_pass;
  out_diag->fp16_fallback_reason_detail = rt->sgemm_controller.fp16_fallback_reason_detail;
  out_diag->fp16_selected_candidate = rt->sgemm_controller.fp16_selected_candidate;
  out_diag->m29_current_slot_id = rt->slot_diag.current_slot_id;
  out_diag->m29_next_slot_id = rt->slot_diag.next_slot_id;
  out_diag->m29_slot0_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[0]);
  out_diag->m29_slot1_state = (uint32_t)prom_slot_hfsm_current_state(&rt->slots[1]);
  out_diag->m29_slot0_generation = prom_slot_hfsm_metadata(&rt->slots[0])->generation;
  out_diag->m29_slot1_generation = prom_slot_hfsm_metadata(&rt->slots[1])->generation;
  out_diag->m29_slot0_valid = prom_slot_hfsm_metadata(&rt->slots[0])->valid;
  out_diag->m29_slot1_valid = prom_slot_hfsm_metadata(&rt->slots[1])->valid;
  out_diag->m29_swap_count = rt->slot_diag.swap_count;
  out_diag->m29_max_wip_depth = rt->slot_diag.max_wip_depth;
  out_diag->m29_overwrite_rejection_count = rt->slot_diag.overwrite_rejection_count;
  out_diag->m29_stale_buffer_rejection_count = rt->slot_diag.stale_buffer_rejection_count;
  out_diag->m29_shape_invalidation_count = rt->slot_diag.shape_invalidation_count;
  out_diag->m29_layout_invalidation_count = rt->slot_diag.layout_invalidation_count;
  out_diag->m29_capacity_invalidation_count = rt->slot_diag.capacity_invalidation_count;
  out_diag->m29_inflight_rejection_count = rt->slot_diag.inflight_rejection_count;
  out_diag->m29_failure_slot_id = rt->slot_diag.failure_slot_id;
  out_diag->m29_failure_reason = rt->slot_diag.failure_reason;
  return PROM_OK;
}
