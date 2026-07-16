#ifndef PROMETHEUS_REACTOR_VULKAN_TRANSFORMER_INTERNAL_H
#define PROMETHEUS_REACTOR_VULKAN_TRANSFORMER_INTERNAL_H

/* Internal transformer ownership vocabulary.  Pipelines and layouts remain
   family-owned; parameter resources are immutable per layer; descriptor banks
   are bounded per recorded layer; command lifetime remains with the caller. */

typedef struct prom_transformer_parameter_resource {
  prom_vk_buffer upload;
  prom_vk_buffer f32;
  prom_vk_buffer f16;
  uint64_t generation;
  uint64_t hash;
  uint32_t rows;
  uint32_t columns;
} prom_transformer_parameter_resource;

typedef struct prom_transformer_layer_resources {
  uint32_t layer_index;
  uint32_t model_width;
  uint32_t head_dim;
  uint32_t ffn_width;
  prom_transformer_parameter_resource
      attention[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  prom_transformer_parameter_resource wo;
  prom_transformer_parameter_resource rmsnorm;
  prom_transformer_parameter_resource ffn[PROM_M47_WEIGHT_COUNT];
} prom_transformer_layer_resources;

typedef struct prom_transformer_descriptor_bank {
  VkDescriptorSet m43[PROM_M43_DESCRIPTOR_SET_COUNT];
  VkDescriptorSet m44_sgemm;
  VkDescriptorSet m45;
  VkDescriptorSet m46[3u];
  VkDescriptorSet m47[PROM_M47_DESCRIPTOR_SET_COUNT];
  VkDescriptorSet m44_wide;
} prom_transformer_descriptor_bank;

typedef struct prom_transformer_record_context {
  VkCommandBuffer command_buffer;
  uint32_t query_base;
  uint32_t query_count;
  uint32_t layer_index;
  prom_transformer_descriptor_bank* descriptors;
} prom_transformer_record_context;

#endif
