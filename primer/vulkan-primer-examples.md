document:
  name: vulkan-primer-examples
  format: toon
  version: 1

intent:
  summary:
    Provide small architectural example pairs that demonstrate the allowed Vulkan subset in practice.
    These examples are normative anchors for authors.
  audience:
    primary:
      LLM authors
    secondary:
      human contributors

examples:
  backend_boundary:
    summary:
      Vulkan should remain inside backend-facing layers. Higher-level systems should depend on renderer concepts, not raw Vulkan objects.

    good_backend_quarantine:
      why:
        High-level code requests a renderer action through backend-neutral concepts.
      code: |
        // Higher-level rendering code
        struct MeshDrawRequest
        {
            MeshHandle mesh;
            MaterialHandle material;
            Transform transform;
        };

        class IRendererBackend
        {
        public:
            virtual ~IRendererBackend() = default;
            virtual void SubmitMeshDraw(const MeshDrawRequest& request) = 0;
        };

        // Vulkan-specific code stays in the backend implementation.
        class VulkanRendererBackend final : public IRendererBackend
        {
        public:
            void SubmitMeshDraw(const MeshDrawRequest& request) override;
        };

    good_vulkan_types_confined:
      why:
        Vulkan handles and helper types are kept inside backend modules.
      code: |
        // vulkan_backend/image_resource.h
        struct VulkanImageResource
        {
            VkImage image = VK_NULL_HANDLE;
            VkImageView view = VK_NULL_HANDLE;
            VmaAllocation allocation = nullptr;
        };

        // renderer/material_system.h
        struct MaterialHandle
        {
            uint32_t index = 0;
        };

    bad_raw_handle_leakage:
      why:
        A high-level system now needs to understand Vulkan ownership and API shape.
      code: |
        class MaterialSystem
        {
        public:
            void BindMaterial(VkCommandBuffer command_buffer, VkPipelineLayout layout, uint32_t set_index);
        };

    bad_backend_as_ambient_fact:
      why:
        The renderer backend is no longer a backend. Vulkan has leaked into unrelated engine code.
      code: |
        struct SceneNode
        {
            VkBuffer vertex_buffer;
            VkDescriptorSet descriptor_set;
            VkImage shadow_map;
        };

  ownership_and_lifetime:
    summary:
      Every Vulkan object class should have a clear owner and a clear destruction rule.

    good_backend_owned_resource:
      why:
        The owning type makes lifetime responsibility explicit.
      code: |
        struct VulkanBuffer
        {
            VkBuffer handle = VK_NULL_HANDLE;
            VmaAllocation allocation = nullptr;
            size_t size_bytes = 0;
        };

        class VulkanResourceAllocator
        {
        public:
            VulkanBuffer CreateVertexBuffer(size_t size_bytes);
            void DestroyBuffer(VulkanBuffer& buffer);
        };

    good_deferred_retirement:
      why:
        GPU-visible destruction timing is centralized instead of improvised per feature.
      code: |
        struct DeferredDestroyEntry
        {
            uint64_t retire_frame = 0;
            VulkanBuffer buffer;
        };

        class VulkanDeferredDestroyQueue
        {
        public:
            void EnqueueBuffer(uint64_t retire_frame, VulkanBuffer buffer);
            void RetireCompleted(uint64_t completed_frame);
        };

    bad_scattered_destroy_calls:
      why:
        Destruction policy is now feature-local and easy to get wrong or duplicate.
      code: |
        class TextureLoader
        {
        public:
            void Shutdown()
            {
                vkDestroyImage(device_, texture_image_, nullptr);
                vkDestroyImageView(device_, texture_view_, nullptr);
                vmaFreeMemory(allocator_, texture_allocation_);
            }
        };

        class ShadowPass
        {
        public:
            void Shutdown()
            {
                vkDestroyImage(device_, shadow_image_, nullptr);
                vkDestroyImageView(device_, shadow_view_, nullptr);
                vmaFreeMemory(allocator_, shadow_allocation_);
            }
        };

    bad_unclear_owner:
      why:
        The code creates a resource, but no single owner is obvious.
      code: |
        VkBuffer CreateBuffer(...);

        void BuildMesh(...)
        {
            VkBuffer buffer = CreateBuffer(...);
            cache_.Store(buffer);
            renderer_.Track(buffer);
            scene_.buffers.push_back(buffer);
        }

  frame_resources:
    summary:
      Per-frame state should follow one stable repeated pattern.

    good_explicit_frame_context:
      why:
        Temporary per-frame objects live in an obvious place with obvious reset points.
      code: |
        struct VulkanFrameContext
        {
            VkCommandPool command_pool = VK_NULL_HANDLE;
            VkCommandBuffer primary_command_buffer = VK_NULL_HANDLE;
            VkFence frame_fence = VK_NULL_HANDLE;
            VkSemaphore acquire_semaphore = VK_NULL_HANDLE;
            VkSemaphore render_complete_semaphore = VK_NULL_HANDLE;
            VulkanUploadScratch upload_scratch;
        };

        class VulkanRendererBackend
        {
        public:
            VulkanFrameContext& CurrentFrame();
        private:
            std::array<VulkanFrameContext, kMaxFramesInFlight> frames_;
            uint32_t frame_index_ = 0;
        };

    good_clear_frame_begin_end:
      why:
        Frame lifetime and reset behavior are predictable.
      code: |
        void VulkanRendererBackend::BeginFrame()
        {
            VulkanFrameContext& frame = CurrentFrame();
            WaitForFrameFence(frame);
            ResetFrameContext(frame);
            BeginPrimaryCommandBuffer(frame.primary_command_buffer);
        }

        void VulkanRendererBackend::EndFrame()
        {
            VulkanFrameContext& frame = CurrentFrame();
            EndPrimaryCommandBuffer(frame.primary_command_buffer);
            SubmitFrame(frame);
            PresentFrame(frame);
            AdvanceFrameIndex();
        }

    bad_ad_hoc_frame_state:
      why:
        Per-frame resources are smeared across the backend with no stable pattern.
      code: |
        class VulkanRendererBackend
        {
        private:
            VkFence fence_a_;
            VkFence fence_b_;
            VkCommandBuffer upload_cmd_;
            VkCommandBuffer main_cmd_;
            VkSemaphore image_ready_;
            VkSemaphore render_ready_;
            std::vector<VkBuffer> temporary_buffers_;
            std::vector<VkImage> pending_images_;
        };

    bad_feature_local_frame_data:
      why:
        Each feature starts growing its own frame lifetime model.
      code: |
        class ShadowSystem
        {
        private:
            std::array<VkCommandBuffer, 2> shadow_frame_cmds_;
            std::array<VkFence, 2> shadow_fences_;
        };

        class UiSystem
        {
        private:
            std::array<VkCommandBuffer, 2> ui_frame_cmds_;
            std::array<VkSemaphore, 2> ui_signals_;
        };

  command_recording:
    summary:
      Command recording should be phase-oriented and predictable.

    good_phase_oriented_recording:
      why:
        Command ownership and recording order are easy to review.
      code: |
        void VulkanRendererBackend::RecordFrame(VulkanFrameContext& frame)
        {
            BeginPrimaryCommandBuffer(frame.primary_command_buffer);

            RecordShadowPass(frame.primary_command_buffer);
            RecordMainGeometryPass(frame.primary_command_buffer);
            RecordUiPass(frame.primary_command_buffer);

            EndPrimaryCommandBuffer(frame.primary_command_buffer);
        }

    good_backend_controls_recording:
      why:
        Recording entry points remain centralized.
      code: |
        void VulkanRendererBackend::RenderFrame()
        {
            VulkanFrameContext& frame = CurrentFrame();
            BeginFrame();
            RecordFrame(frame);
            EndFrame();
        }

    bad_arbitrary_subsystem_recording:
      why:
        Any subsystem can now record whenever it wants, which destroys backend coherence.
      code: |
        class MeshSystem
        {
        public:
            void Record(VkCommandBuffer command_buffer);
        };

        class LightingSystem
        {
        public:
            void Record(VkCommandBuffer command_buffer);
        };

        class ParticleSystem
        {
        public:
            void Record(VkCommandBuffer command_buffer);
        };

        // Called from many unrelated locations with no stable sequencing policy.

    bad_recording_hidden_in_helpers:
      why:
        Important command generation is buried in utility code instead of visible backend phases.
      code: |
        void DrawMesh(...)
        {
            HelperBindEverythingAndMaybeTransitionResources(command_buffer, ...);
            vkCmdDrawIndexed(command_buffer, ...);
        }

  synchronization:
    summary:
      Synchronization policy should be centralized and repeated, not improvised at call sites.

    good_central_transition_helper:
      why:
        Resource state changes are routed through a known backend policy point.
      code: |
        enum class VulkanImageUsageState
        {
            Undefined,
            TransferDst,
            ShaderReadOnly,
            ColorAttachment,
            DepthAttachment,
        };

        class VulkanSyncPlanner
        {
        public:
            void TransitionImage(
                VkCommandBuffer command_buffer,
                VulkanImageResource& image,
                VulkanImageUsageState old_state,
                VulkanImageUsageState new_state);
        };

    good_pass_declares_need_not_barrier_details:
      why:
        Pass code expresses intent while centralized policy owns the barrier details.
      code: |
        void RecordShadowPass(VkCommandBuffer command_buffer)
        {
            sync_planner_.TransitionImage(
                command_buffer,
                shadow_map_,
                VulkanImageUsageState::ShaderReadOnly,
                VulkanImageUsageState::DepthAttachment);

            // Record pass work here.
        }

    bad_barrier_folklore:
      why:
        Every subsystem invents its own synchronization dialect.
      code: |
        void ShadowSystem::Record(...)
        {
            VkImageMemoryBarrier barrier{};
            barrier.srcAccessMask = VK_ACCESS_SHADER_READ_BIT;
            barrier.dstAccessMask = VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT;
            barrier.oldLayout = VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL;
            barrier.newLayout = VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL;
            vkCmdPipelineBarrier(...);
        }

        void PostProcessSystem::Record(...)
        {
            VkImageMemoryBarrier barrier{};
            barrier.srcAccessMask = VK_ACCESS_MEMORY_READ_BIT;
            barrier.dstAccessMask = VK_ACCESS_COLOR_ATTACHMENT_WRITE_BIT;
            barrier.oldLayout = VK_IMAGE_LAYOUT_GENERAL;
            barrier.newLayout = VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL;
            vkCmdPipelineBarrier(...);
        }

    bad_sync_patch_for_now:
      why:
        A local “fix” becomes permanent backend policy by accident.
      code: |
        // TODO: this is probably wrong but it fixes flickering for now
        vkCmdPipelineBarrier(
            command_buffer,
            VK_PIPELINE_STAGE_ALL_COMMANDS_BIT,
            VK_PIPELINE_STAGE_ALL_COMMANDS_BIT,
            0,
            0, nullptr,
            0, nullptr,
            0, nullptr);

  timestamps_and_queries:
    summary:
      GPU timing should follow explicit query recipes instead of plausible-looking local improvisation.

    good_explicit_timestamp_bracketing:
      why:
        The timestamps intentionally bracket the GPU work being measured.
      code: |
        void RecordTimedComputeDispatch(VkCommandBuffer command_buffer)
        {
            vkCmdResetQueryPool(command_buffer, timestamp_query_pool_, 0, 2);
            vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, timestamp_query_pool_, 0);

            vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, compute_pipeline_);
            vkCmdDispatch(command_buffer, group_count_x_, 1, 1);

            vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, timestamp_query_pool_, 1);
        }

    good_read_after_completion_policy:
      why:
        Query results are read only after the backend has established completion.
      code: |
        void ResolveCompletedFrameQueries(const VulkanFrameContext& frame)
        {
            WaitForFrameFence(frame);
            vkGetQueryPoolResults(
                device_,
                timestamp_query_pool_,
                0,
                2,
                sizeof(results_),
                results_.data(),
                sizeof(std::uint64_t),
                VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT);
        }

    bad_timestamp_stage_folklore:
      why:
        The code records plausible timestamps without a clear statement of what GPU interval is actually being measured.
      code: |
        vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, query_pool_, 0);
        vkCmdDispatch(command_buffer, group_count_x_, 1, 1);
        vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, query_pool_, 1);

    bad_query_readback_too_early:
      why:
        Query results are read with no clear completion rule.
      code: |
        vkGetQueryPoolResults(device_, query_pool_, 0, 2, sizeof(results), results, sizeof(std::uint64_t), VK_QUERY_RESULT_64_BIT);

  shader_module_loading:
    summary:
      SPIR-V loading must respect byte representation and alignment.

    good_aligned_spirv_decode:
      why:
        The backend converts bytes into aligned 32-bit words explicitly before shader module creation.
      code: |
        std::vector<std::uint32_t> DecodeSpirvFile(const std::vector<std::byte>& bytes)
        {
            assert(bytes.size() % 4 == 0);

            std::vector<std::uint32_t> words(bytes.size() / 4);
            for (std::size_t i = 0; i < words.size(); ++i)
            {
                const std::size_t base = i * 4;
                words[i] =
                    static_cast<std::uint32_t>(std::to_integer<std::uint8_t>(bytes[base + 0])) |
                    (static_cast<std::uint32_t>(std::to_integer<std::uint8_t>(bytes[base + 1])) << 8) |
                    (static_cast<std::uint32_t>(std::to_integer<std::uint8_t>(bytes[base + 2])) << 16) |
                    (static_cast<std::uint32_t>(std::to_integer<std::uint8_t>(bytes[base + 3])) << 24);
            }

            return words;
        }

        VkShaderModuleCreateInfo create_info{};
        create_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
        create_info.codeSize = words.size() * sizeof(std::uint32_t);
        create_info.pCode = words.data();

    bad_reinterpret_cast_shader_load:
      why:
        The code assumes arbitrary file bytes are suitably aligned for uint32_t access.
      code: |
        const std::uint32_t* words =
            reinterpret_cast<const std::uint32_t*>(spirv_bytes.data());

        VkShaderModuleCreateInfo create_info{};
        create_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
        create_info.codeSize = spirv_bytes.size();
        create_info.pCode = words;

  push_constants:
    summary:
      Push constant layout should be explicit and shared between host and shader expectations.

    good_explicit_push_constant_struct:
      why:
        The host-side structure is simple, bounded, and clearly associated with a pipeline range.
      code: |
        struct DrawPushConstants
        {
            float model_matrix[16];
            std::uint32_t material_index;
            std::uint32_t padding[3];
        };

        VkPushConstantRange range{};
        range.stageFlags = VK_SHADER_STAGE_VERTEX_BIT | VK_SHADER_STAGE_FRAGMENT_BIT;
        range.offset = 0;
        range.size = sizeof(DrawPushConstants);

    bad_implicit_layout_guess:
      why:
        The code assumes a convenient host layout is automatically compatible with shader expectations.
      code: |
        struct DrawPushConstants
        {
            glm::mat4 model;
            bool selected;
        };

  uploads_and_transfers:
    summary:
      Upload paths should be standardized, with explicit staging ownership and retirement.

    good_standard_staging_flow:
      why:
        Upload behavior is consistent and easy to repeat safely.
      code: |
        struct UploadBufferRequest
        {
            const void* source_data = nullptr;
            size_t size_bytes = 0;
            VkBufferUsageFlags final_usage = 0;
        };

        class VulkanUploadSystem
        {
        public:
            VulkanBuffer CreateDeviceLocalBufferWithUpload(const UploadBufferRequest& request);
        };

    good_readiness_explicit:
      why:
        The code makes it clear when uploaded resources are safe for later use.
      code: |
        VulkanBuffer vertex_buffer = upload_system_.CreateDeviceLocalBufferWithUpload(request);
        resource_tracker_.MarkReadyAfterFrame(vertex_buffer, current_frame_id_);

    bad_one_off_feature_upload:
      why:
        Every feature now invents its own staging flow and retirement logic.
      code: |
        class TextureSystem
        {
        public:
            void UploadTextureNow(...)
            {
                // Create staging buffer here.
                // Map memory here.
                // Copy bytes here.
                // Record copy here.
                // Submit here.
                // Destroy staging buffer maybe later.
            }
        };

    bad_hidden_upload_side_effect:
      why:
        A friendly helper hides queue submission, staging lifetime, and readiness semantics.
      code: |
        void Mesh::SetVertices(const std::vector<Vertex>& vertices)
        {
            backend_->UploadToGpu(vertices.data(), vertices.size() * sizeof(Vertex));
        }

  descriptors_and_pipelines:
    summary:
      Descriptor and pipeline management should follow stable backend recipes.

    good_central_descriptor_policy:
      why:
        Descriptor allocation and update behavior are standardized.
      code: |
        class VulkanDescriptorAllocator
        {
        public:
            VkDescriptorSet AllocateMaterialSet(VkDescriptorSetLayout layout);
            void ResetFrameLocalPools(uint32_t frame_index);
        };

        class VulkanMaterialBinder
        {
        public:
            VkDescriptorSet BuildMaterialSet(const MaterialBindingData& binding_data);
        };

    good_pipeline_creation_in_backend_policy_layer:
      why:
        Pipeline setup is centralized instead of re-authored in feature code.
      code: |
        struct GraphicsPipelineRequest
        {
            ShaderProgramHandle shaders;
            RenderPassHandle render_pass;
            VertexLayoutHandle vertex_layout;
            BlendMode blend_mode;
            DepthMode depth_mode;
        };

        class VulkanPipelineCache
        {
        public:
            VkPipeline GetOrCreateGraphicsPipeline(const GraphicsPipelineRequest& request);
        };

    bad_feature_local_descriptor_religion:
      why:
        Each feature invents its own descriptor lifetime and update model.
      code: |
        class ShadowSystem
        {
            VkDescriptorPool pool_;
            VkDescriptorSetLayout layout_;
            VkDescriptorSet set_;
        };

        class UiSystem
        {
            VkDescriptorPool pool_;
            VkDescriptorSetLayout layout_;
            std::vector<VkDescriptorSet> frame_sets_;
        };

    bad_pipeline_creation_copy_paste:
      why:
        Large Vulkan setup sequences are duplicated with tiny variations and no central policy.
      code: |
        VkPipeline CreateShadowPipeline(...);
        VkPipeline CreateUiPipeline(...);
        VkPipeline CreatePostProcessPipeline(...);
        // Each repeats most of the same setup logic separately.

  wrappers:
    summary:
      Wrappers should encode ownership and repeated policy, not erase reality.

    good_thin_wrapper:
      why:
        The wrapper gives a real ownership unit while keeping important Vulkan facts visible.
      code: |
        struct VulkanImageResource
        {
            VkImage image = VK_NULL_HANDLE;
            VkImageView view = VK_NULL_HANDLE;
            VmaAllocation allocation = nullptr;
            VkFormat format = VK_FORMAT_UNDEFINED;
            VkExtent3D extent{};
        };

    good_policy_wrapper:
      why:
        The wrapper centralizes repeated backend policy instead of pretending Vulkan does not exist.
      code: |
        class VulkanCommandSubmission
        {
        public:
            void SubmitGraphics(
                VkCommandBuffer command_buffer,
                VkSemaphore wait_semaphore,
                VkSemaphore signal_semaphore,
                VkFence signal_fence);
        };

    bad_reality_hiding_wrapper_tower:
      why:
        The wrapper names sound pleasant, but they conceal synchronization, ownership, and cost.
      code: |
        class MagicalTexture
        {
        public:
            void Prepare();
            void Activate();
            void Sync();
            void Use();
            void CleanupLaterMaybe();
        };

    bad_wrap_everything_for_prestige:
      why:
        The codebase gains a second custom object model with no clear review benefit.
      code: |
        class DeviceObjectBase {};
        class BufferObjectNode : public DeviceObjectBase {};
        class ImageObjectNode : public DeviceObjectBase {};
        class DescriptorObjectNode : public DeviceObjectBase {};
        class PipelineObjectNode : public DeviceObjectBase {};

  validation_and_debugging:
    summary:
      Validation should remain part of the development workflow, not an afterthought.

    good_validation_enabled_dev_path:
      why:
        Development builds are set up to catch backend mistakes early.
      code: |
        struct VulkanInstanceCreateOptions
        {
            bool enable_validation = true;
            bool enable_debug_labels = true;
        };

        VkInstance CreateVulkanInstance(const VulkanInstanceCreateOptions& options);

    good_debug_names:
      why:
        GPU objects become easier to inspect in validation output and graphics debugging tools.
      code: |
        debug_utils_.SetObjectName(device_, vertex_buffer.handle, "main-scene-vertex-buffer");
        debug_utils_.SetObjectName(device_, shadow_map_.image, "shadow-map-image");

    bad_validation_disabled_to_reduce_noise:
      why:
        The architecture becomes less trustworthy because the feedback loop was muted.
      code: |
        VulkanInstanceCreateOptions options{};
        options.enable_validation = false; // validation is annoying right now

    bad_warning_suppression_by_obscurity:
      why:
        The code responds to backend difficulty by hiding what is happening, not by simplifying it.
      code: |
        void HelperDoGpuStuff(...)
        {
            // performs allocation, transition, submission, and destruction
            // but validation failures are now much harder to connect to the real cause
        }

review_hints:
  summary:
    Use these examples directionally, not mechanically.
  rules:
    - Prefer backend quarantine over raw Vulkan leakage.
    - Prefer one stable ownership and destruction recipe per resource class.
    - Prefer one stable frame pattern over feature-local frame inventions.
    - Prefer centralized synchronization policy over call-site barrier improvisation.
    - Prefer upload, descriptor, and pipeline recipes that repeat consistently.
    - Prefer thin wrappers that improve reasoning over thick wrappers that erase facts.
    - Prefer development flows that keep validation loud and useful.

closing:
  message:
    These examples are anchors, not decoration.
    The goal is not impressive Vulkan.
    The goal is trustworthy Vulkan backend architecture that does not turn the renderer into radioactive spaghetti.
