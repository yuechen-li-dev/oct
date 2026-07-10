#include "../reactor_api.h"
#include "../reactor_dominatus_predictor.h"
#include "../reactor_sgemm_dispatch_metadata.h"
#include "../reactor_judgment_engine.h"
#include "../reactor_policy_memory.h"
#include "../reactor_vulkan.h"
#include "../reactor_vulkan_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv.h"
#include "../reactor_vulkan_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv.h"
#include "../reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.h"
#include "../reactor_vulkan_sgemm_scalar_plus_spirv.h"
#include "../reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h"
#include "test_harness.h"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstdlib>
#include <ctime>
#include <filesystem>
#include <limits>
#include <numeric>
#include <sstream>
#include <string>
#include <string_view>
#include <vector>

namespace
{
    constexpr float kAbsTolerance = 1.0e-4f;
    constexpr float kRelTolerance = 1.0e-4f;

    struct ShapeCase
    {
        const char* name;
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
        bool optional;
    };

    struct CorrectnessResult
    {
        std::string status = "not_run_in_benchmark_mode";
        std::string reference_mode = "none";
        std::string source = "see_correctness_lane";
        float max_abs_error = 0.0f;
        float max_rel_error = 0.0f;
        float tolerance = kAbsTolerance;
    };

    struct TimingDecomposition
    {
        double upload_ms = 0.0;
        double pre_dispatch_ms = 0.0;
        double command_record_ms = 0.0;
        double dispatch_submit_ms = 0.0;
        double kernel_gpu_ms = 0.0;
        double readback_ms = 0.0;
        double sync_wait_ms = 0.0;
        double post_sync_ms = 0.0;
        double post_readback_ms = 0.0;
        double total_wall_ms = 0.0;
        double benchmark_total_ms = 0.0;
        double oracle_ms = 0.0;
        double validation_readback_ms = 0.0;
        double validation_ms = 0.0;
        double unaccounted_host_ms = 0.0;
        double tolerance_eval_ms = 0.0;
        double kernel_only_gflops = 0.0;
        double end_to_end_gflops = 0.0;
        bool gpu_timestamp_valid = false;
        std::string timing_source = "unknown";
    };

    struct TimingStats
    {
        std::uint32_t warmup_iterations = 0u;
        std::uint32_t iterations = 0u;
        double min_ms = 0.0;
        double median_ms = 0.0;
        double p95_ms = 0.0;
        double gflops_median = 0.0;
        std::string timing_source = "cpu_wall_clock";
    };

    struct CaseResult
    {
        std::string name;
        std::uint32_t m = 0u;
        std::uint32_t n = 0u;
        std::uint32_t k = 0u;
        bool optional = false;
        bool skipped = false;
        std::string skip_reason;
        int runtime_status = PROM_OK;
        std::uint32_t final_stage = PROM_STAGE_NONE;
        int final_detail_code = 0;
        CorrectnessResult correctness;
        TimingStats timing;
        TimingDecomposition timing_decomposition;
        PrometheusSgemmPolicyDiagnostics diag{};
        std::vector<std::string> anomalies;
    };

    struct VariantComparisonRow
    {
        std::string shape;
        std::uint32_t m = 0u;
        std::uint32_t n = 0u;
        std::uint32_t k = 0u;
        std::string variant;
        std::string requested_variant;
        std::string executed_variant;
        std::string path;
        std::string correctness = "not_run_in_benchmark_mode";
        std::string reference_mode = "none";
        bool skipped = false;
        std::string skip_reason;
        int runtime_status = PROM_OK;
        std::uint32_t final_stage = PROM_STAGE_NONE;
        int final_detail_code = 0;
        double median_total_ms = 0.0;
        double median_kernel_ms = 0.0;
        double end_to_end_gflops = 0.0;
        double kernel_only_gflops = 0.0;
        bool gpu_timestamp_valid = false;
        std::string timing_source = "unknown";
        TimingDecomposition timing_decomposition;
        PrometheusSgemmPolicyDiagnostics diag{};
        std::vector<std::string> anomalies;
    };

    struct ResidentBenchmarkRow
    {
        std::string shape;
        std::uint32_t m = 0u;
        std::uint32_t n = 0u;
        std::uint32_t k = 0u;
        std::string mode;
        std::string variant;
        std::string requested_variant;
        std::string executed_variant;
        std::string correctness = "not_run_in_benchmark_mode";
        std::string reference_mode = "none";
        bool skipped = false;
        std::string skip_reason;
        int runtime_status = PROM_OK;
        std::uint32_t setup_stage = PROM_STAGE_NONE;
        int setup_detail_code = 0;
        std::uint32_t final_stage = PROM_STAGE_NONE;
        int final_detail_code = 0;
        bool resident_mode_available = false;
        bool resident_mode_used = false;
        double upload_once_ms = 0.0;
        double setup_ms = 0.0;
        double kernel_median_ms = 0.0;
        double kernel_min_ms = 0.0;
        double kernel_p95_ms = 0.0;
        double total_loop_ms = 0.0;
        std::uint32_t iterations = 0u;
        double readback_once_ms = 0.0;
        double validation_ms = 0.0;
        double kernel_only_gflops = 0.0;
        double loop_gflops = 0.0;
        bool gpu_timestamp_valid = false;
        std::string gpu_timing_failure_reason = "UNKNOWN";
        PrometheusSgemmPolicyDiagnostics diag{};
        std::vector<std::string> anomalies;
    };

    struct ResidentFailureMatrixRow
    {
        std::string shape;
        std::uint32_t m = 0u;
        std::uint32_t n = 0u;
        std::uint32_t k = 0u;
        std::string requested_variant;
        std::string staged_requested_variant;
        std::string staged_executed_variant;
        std::string resident_executed_variant;
        std::string production_resident_executed_variant;
        std::string staged_path;
        std::string resident_failure_stage = "none";
        std::string staged_warmup_result = "not_run";
        std::string staged_measured_result = "not_run";
        std::string resident_setup_result = "not_run";
        std::string resident_warmup_result = "not_run";
        std::string resident_measured_result = "not_run";
        std::string vk_result = "none";
        int staged_runtime_status = PROM_OK;
        std::uint32_t staged_out_stage = PROM_STAGE_NONE;
        int staged_out_detail_code = 0;
        int resident_runtime_status = PROM_OK;
        std::uint32_t resident_setup_stage = PROM_STAGE_NONE;
        int resident_setup_detail_code = 0;
        std::uint32_t resident_out_stage = PROM_STAGE_NONE;
        int resident_out_detail_code = 0;
        bool production_resident_succeeds = false;
        bool resident_mode_available = false;
        bool selected_pipeline_present = false;
        bool descriptor_update_status = false;
        bool timestamp_query_available = false;
        bool timestamp_query_valid = false;
        std::uint32_t dispatch_groups_x = 0u;
        std::uint32_t dispatch_groups_y = 0u;
        std::uint32_t dispatch_groups_z = 0u;
        std::uint32_t logical_m_per_group = 0u;
        std::uint32_t logical_n_per_group = 0u;
        std::uint32_t metadata_numthreads_x = 0u;
        std::uint32_t metadata_numthreads_y = 0u;
        std::uint32_t metadata_numthreads_z = 0u;
        std::uint32_t metadata_outputs_per_invocation_m = 0u;
        std::uint32_t metadata_outputs_per_invocation_n = 0u;
        std::uint32_t metadata_tile_m = 0u;
        std::uint32_t metadata_tile_n = 0u;
        std::uint32_t metadata_tile_k = 0u;
        std::uint32_t metadata_unroll_k = 0u;
        std::uint64_t a_elements = 0u;
        std::uint64_t b_elements = 0u;
        std::uint64_t c_elements = 0u;
        std::uint64_t a_bytes = 0u;
        std::uint64_t b_bytes = 0u;
        std::uint64_t c_bytes = 0u;
        std::uint32_t descriptor_binding_count = 3u;
        std::uint32_t descriptor_binding_a = 0u;
        std::uint32_t descriptor_binding_b = 1u;
        std::uint32_t descriptor_binding_c = 2u;
        bool shader_module_present = false;
        std::uint32_t push_constant_bytes = 12u;
        std::uint32_t push_constant_offset_m = 0u;
        std::uint32_t push_constant_offset_n = 4u;
        std::uint32_t push_constant_offset_k = 8u;
        std::uint32_t timestamp_query_slot_begin = 0u;
        std::uint32_t timestamp_query_slot_end = 1u;
        std::uint32_t resident_iteration_count = 0u;
        std::vector<std::string> notes;
    };

    struct RuntimeHandleScope
    {
        void* handle = nullptr;

        RuntimeHandleScope() = default;
        RuntimeHandleScope(const RuntimeHandleScope&) = delete;
        RuntimeHandleScope& operator=(const RuntimeHandleScope&) = delete;

        ~RuntimeHandleScope()
        {
            if (handle != nullptr) {
                (void)prometheus_reactor_runtime_destroy(handle);
                handle = nullptr;
            }
        }
    };

    struct SelectorVsFastestRow
    {
        std::string shape;
        std::string production_variant;
        std::string executed_production_variant;
        std::string fastest_variant;
        std::string comparison_basis;
        double production_median_ms = 0.0;
        double fastest_median_ms = 0.0;
        double slowdown_ratio = 0.0;
        bool picked_same_variant_as_fastest_explicit = false;
        bool production_slower_than_fastest_explicit = false;
        std::vector<std::string> anomalies;
    };

    struct Summary
    {
        std::size_t cases_total = 0u;
        std::size_t cases_passed = 0u;
        std::size_t cases_failed = 0u;
        std::size_t cases_skipped = 0u;
        double best_gflops = 0.0;
        double median_gflops = 0.0;
        std::size_t anomaly_count = 0u;
    };

    struct DeviceMetadata
    {
        std::string name = "unknown";
        std::string backend = "unknown";
        std::string device_type = "unknown";
        std::uint32_t vendor_id = 0u;
        std::uint32_t device_id = 0u;
        std::string driver_version = "unknown";
        std::string api_version = "unknown";
        std::uint32_t max_compute_workgroup_invocations = 0u;
        std::uint32_t max_compute_shared_memory_size = 0u;
        std::uint32_t subgroup_size = 0u;
    };

    struct ReportData
    {
        std::string schema = "prometheus.sgemm.px16.evt.v3";
        std::string timestamp_utc;
        std::string benchmark_name;
        DeviceMetadata device;
        Summary summary;
        std::vector<CaseResult> cases;
        std::vector<VariantComparisonRow> variant_comparison;
        std::vector<ResidentBenchmarkRow> resident_production;
        std::vector<ResidentBenchmarkRow> resident_variant_comparison;
        std::vector<SelectorVsFastestRow> selector_vs_fastest;
        std::vector<SelectorVsFastestRow> selector_vs_fastest_resident;
        std::vector<std::string> performance_diagnosis;
        std::string global_skip_reason;
        std::string run_mode = "performance_benchmark";
        std::string validation_status_source = "not_run_in_benchmark_mode";
        bool resident_device_mode_available = false;
        bool explicit_cube_1024_enabled = false;
    };

    VariantComparisonRow run_explicit_variant_case(void* handle, const ShapeCase& shape, std::uint32_t variant, bool validate_output);
    ResidentBenchmarkRow run_resident_case(void* handle,
                                           const ShapeCase& shape,
                                           bool explicit_variant,
                                           std::uint32_t requested_variant,
                                           bool validate_output);

    std::string json_escape(std::string_view value)
    {
        std::ostringstream out;
        for (const char ch : value) {
            switch (ch) {
                case '\\':
                    out << "\\\\";
                    break;
                case '"':
                    out << "\\\"";
                    break;
                case '\n':
                    out << "\\n";
                    break;
                case '\r':
                    out << "\\r";
                    break;
                case '\t':
                    out << "\\t";
                    break;
                default:
                    if (static_cast<unsigned char>(ch) < 0x20u) {
                        out << "?";
                    } else {
                        out << ch;
                    }
                    break;
            }
        }
        return out.str();
    }

    std::string bool_json(bool value)
    {
        return value ? "true" : "false";
    }

    std::string timestamp_now_utc()
    {
        using namespace std::chrono;
        const auto now = system_clock::now();
        const std::time_t as_time_t = system_clock::to_time_t(now);
        std::tm utc_tm{};
#if defined(_WIN32)
        gmtime_s(&utc_tm, &as_time_t);
#else
        gmtime_r(&as_time_t, &utc_tm);
#endif
        char buffer[32] = {};
        std::strftime(buffer, sizeof(buffer), "%Y-%m-%dT%H:%M:%SZ", &utc_tm);
        return std::string(buffer);
    }

    std::string vulkan_version_name(std::uint32_t version)
    {
        if (version == 0u) {
            return "unknown";
        }
        std::ostringstream out;
        out << VK_VERSION_MAJOR(version) << "." << VK_VERSION_MINOR(version) << "." << VK_VERSION_PATCH(version);
        return out.str();
    }

    std::string backend_name(std::uint32_t backend_type)
    {
        switch (backend_type) {
            case PROM_BACKEND_STUB:
                return "STUB";
            case PROM_BACKEND_VULKAN:
                return "VULKAN";
            case PROM_BACKEND_VULKAN_SOFTWARE:
                return "VULKAN_SOFTWARE";
            default:
                return "UNKNOWN";
        }
    }

    std::string vulkan_device_type_name(VkPhysicalDeviceType device_type)
    {
        switch (device_type) {
            case VK_PHYSICAL_DEVICE_TYPE_INTEGRATED_GPU:
                return "INTEGRATED_GPU";
            case VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU:
                return "DISCRETE_GPU";
            case VK_PHYSICAL_DEVICE_TYPE_VIRTUAL_GPU:
                return "VIRTUAL_GPU";
            case VK_PHYSICAL_DEVICE_TYPE_CPU:
                return "CPU";
            default:
                return "OTHER";
        }
    }

    std::string policy_mode_name(std::uint32_t mode)
    {
        switch (mode) {
            case PROM_POLICY_MODE_AGGRESSIVE:
                return "AGGRESSIVE";
            case PROM_POLICY_MODE_SAFE:
                return "SAFE";
            case PROM_POLICY_MODE_RECOVERY:
                return "RECOVERY";
            default:
                return "UNKNOWN";
        }
    }

    std::string occupancy_variant_name(std::uint32_t variant)
    {
        switch (variant) {
            case PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR:
                return "BASELINE_SCALAR";
            case PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE:
                return "MEMORY_CONSERVATIVE";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS:
                return "SDSL_SCALAR_PLUS";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32:
                return "SDSL_REG2X2_TILE16X16_FP32";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32:
                return "SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32:
                return "SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32:
                return "SDSL_TILE16X16_SHARED_FP32";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE:
                return "SMALL_REGISTER_TILE";
            case PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4:
                return "BALANCED_2X2_ACCUM4";
            case PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8:
                return "AGGRESSIVE_4X4_ACCUM8";
            default:
                return "UNKNOWN";
        }
    }

    std::string path_name(std::uint32_t path)
    {
        switch (path) {
            case PROM_VK_PATH_DIRECT:
                return "DIRECT";
            case PROM_VK_PATH_STAGED_UPLOAD:
                return "STAGED_UPLOAD";
            case PROM_VK_PATH_STAGED_UPLOAD_READBACK:
                return "STAGED_UPLOAD_READBACK";
            default:
                return "UNKNOWN";
        }
    }

    std::string compute_mode_name(std::uint32_t mode)
    {
        switch (mode) {
            case PROM_VK_COMPUTE_BASELINE:
                return "BASELINE";
            case PROM_VK_COMPUTE_TILED:
                return "TILED";
            case PROM_VK_COMPUTE_PACKED4_FP32:
                return "PACKED4_FP32";
            case PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM:
                return "FP16_STORAGE_FP32_ACCUM";
            default:
                return "UNKNOWN";
        }
    }

    std::string force_direct_reason_name(std::uint32_t reason)
    {
        switch (reason) {
            case PROM_SGEMM_FORCE_DIRECT_REASON_NONE:
                return "NONE";
            case PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE:
                return "EXPLICIT_OVERRIDE";
            case PROM_SGEMM_FORCE_DIRECT_REASON_SAFE_CONCRETE_HAZARD:
                return "SAFE_CONCRETE_HAZARD";
            default:
                return "UNKNOWN";
        }
    }

    std::string variant_path_status_name(std::uint32_t status)
    {
        switch (status) {
            case PROM_OCCUPANCY_VARIANT_PATH_STATUS_BASELINE:
                return "BASELINE";
            case PROM_OCCUPANCY_VARIANT_PATH_STATUS_ALIAS_OR_NOT_WIRED:
                return "ALIAS_OR_NOT_WIRED";
            case PROM_OCCUPANCY_VARIANT_PATH_STATUS_NOT_WIRED:
                return "NOT_WIRED";
            case PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED:
                return "WIRED";
            default:
                return "UNKNOWN";
        }
    }

    std::string p15_block_reason_name(std::uint32_t reason)
    {
        switch (reason) {
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NONE:
                return "NONE";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_DISABLED:
                return "DISABLED";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NOT_HEALTHY:
                return "NOT_HEALTHY";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_MARGIN_FAILED:
                return "MARGIN_FAILED";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_REASON_BINDING:
                return "REASON_BINDING";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NO_MATURED_RESERVATION:
                return "NO_MATURED_RESERVATION";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_SHAPE_MISMATCH:
                return "SHAPE_MISMATCH";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH:
                return "VARIANT_MISMATCH";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CAPABILITY_MISMATCH:
                return "CAPABILITY_MISMATCH";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_STALE_RESERVATION:
                return "STALE_RESERVATION";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CANCELLED_RESERVATION:
                return "CANCELLED_RESERVATION";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_ALREADY_CONSUMED:
                return "ALREADY_CONSUMED";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_FALLBACK_REQUIRED:
                return "FALLBACK_REQUIRED";
            case PROM_P15_SHADOW_FEEDFORWARD_BLOCK_RESERVATION_NOT_READY:
                return "RESERVATION_NOT_READY";
            default:
                return "UNKNOWN";
        }
    }

    std::string p15_correction_action_name(std::uint32_t action)
    {
        switch (action) {
            case PROM_DOM_CORRECTION_ACTION_NONE:
                return "NONE";
            case PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE:
                return "LOWER_CONFIDENCE";
            case PROM_DOM_CORRECTION_ACTION_REDUCE_DEPTH:
                return "REDUCE_DEPTH";
            case PROM_DOM_CORRECTION_ACTION_CANCEL_FUTURE_LEASE:
                return "CANCEL_FUTURE_LEASE";
            case PROM_DOM_CORRECTION_ACTION_FALLBACK:
                return "FALLBACK";
            case PROM_DOM_CORRECTION_ACTION_MARK_STALE:
                return "MARK_STALE";
            default:
                return "UNKNOWN";
        }
    }

    std::string timing_failure_reason_name(std::uint32_t reason)
    {
        switch (reason) {
            case PROM_SGEMM_GPU_TIMING_FAILURE_NONE:
                return "NONE";
            case PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED:
                return "UNSUPPORTED";
            case PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE:
                return "QUERY_POOL_UNAVAILABLE";
            case PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD:
                return "INVALID_PERIOD";
            case PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE:
                return "QUERY_UNAVAILABLE";
            case PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER:
                return "INVALID_ORDER";
            case PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED:
                return "COMMAND_FAILED";
            default:
                return "UNKNOWN";
        }
    }

    const std::vector<ShapeCase>& evt_shapes()
    {
        static const std::vector<ShapeCase> cases = {
            {"small_16x16x16", 16u, 16u, 16u, false},
            {"small_32x32x32", 32u, 32u, 32u, false},
            {"small_64x64x64", 64u, 64u, 64u, false},
            {"square_127x127x127", 127u, 127u, 127u, false},
            {"square_128x128x128", 128u, 128u, 128u, false},
            {"rect_255x129x65", 255u, 129u, 65u, false},
            {"oddk_64x64x65", 64u, 64u, 65u, false},
            {"square_256x256x256", 256u, 256u, 256u, false},
            {"square_512x512x512", 512u, 512u, 512u, false},
            {"skinny_1024x64x1024", 1024u, 64u, 1024u, false},
            {"wide_64x1024x1024", 64u, 1024u, 1024u, false},
            {"lowk_1024x1024x64", 1024u, 1024u, 64u, false},
            {"square_1024x1024x1024", 1024u, 1024u, 1024u, false},
            {"square_2048x2048x2048", 2048u, 2048u, 2048u, true},
        };
        return cases;
    }

    const std::vector<ShapeCase>& explicit_variant_shapes(bool enable_1024_cube)
    {
        static const std::vector<ShapeCase> base_cases = {
            {"small_64x64x64", 64u, 64u, 64u, false},
            {"square_128x128x128", 128u, 128u, 128u, false},
            {"square_256x256x256", 256u, 256u, 256u, false},
            {"square_512x512x512", 512u, 512u, 512u, false},
            {"skinny_1024x64x1024", 1024u, 64u, 1024u, false},
            {"wide_64x1024x1024", 64u, 1024u, 1024u, false},
            {"lowk_1024x1024x64", 1024u, 1024u, 64u, false},
            {"rect_255x129x65", 255u, 129u, 65u, false},
        };
        static const std::vector<ShapeCase> with_cube = {
            {"small_64x64x64", 64u, 64u, 64u, false},
            {"square_128x128x128", 128u, 128u, 128u, false},
            {"square_256x256x256", 256u, 256u, 256u, false},
            {"square_512x512x512", 512u, 512u, 512u, false},
            {"square_1024x1024x1024", 1024u, 1024u, 1024u, true},
            {"skinny_1024x64x1024", 1024u, 64u, 1024u, false},
            {"wide_64x1024x1024", 64u, 1024u, 1024u, false},
            {"lowk_1024x1024x64", 1024u, 1024u, 64u, false},
            {"rect_255x129x65", 255u, 129u, 65u, false},
        };
        return enable_1024_cube ? with_cube : base_cases;
    }

    const std::vector<ShapeCase>& correctness_shapes()
    {
        static const std::vector<ShapeCase> cases = {
            {"exact_16x16x16", 16u, 16u, 16u, false},
            {"small_8x8x8", 8u, 8u, 8u, false},
            {"odd_17x17x17", 17u, 17u, 17u, false},
            {"odd_31x29x23", 31u, 29u, 23u, false},
            {"skinny_64x16x64", 64u, 16u, 64u, false},
            {"wide_16x64x64", 16u, 64u, 64u, false},
            {"lowk_64x64x8", 64u, 64u, 8u, false},
            {"medium_128x128x128", 128u, 128u, 128u, false},
        };
        return cases;
    }

    std::vector<std::uint32_t> wired_variants()
    {
        return {
            PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
            PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS,
            PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32,
            PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32,
            PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32,
            PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32,
            PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE,
            PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4,
            PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8,
            PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE,
        };
    }

    bool env_flag_enabled(const char* name)
    {
#if defined(_WIN32)
        char* value = nullptr;
        std::size_t value_length = 0u;
        if (_dupenv_s(&value, &value_length, name) != 0 || value == nullptr) {
            return false;
        }
        const bool enabled = std::string_view(value, value_length > 0u ? value_length - 1u : 0u) == "1";
        free(value);
        return enabled;
#else
        const char* value = std::getenv(name);
        return value != nullptr && std::string_view(value) == "1";
#endif
    }

    bool should_run_optional_large_case()
    {
        return env_flag_enabled("OCT_PROMETHEUS_PX16_EVT_ENABLE_2048");
    }

    bool should_run_explicit_1024_cube()
    {
        return env_flag_enabled("OCT_PROMETHEUS_PX16_EVT_ENABLE_EXPLICIT_1024_CUBE");
    }

    std::uint32_t warmup_iterations_for(const ShapeCase& shape)
    {
        const std::uint64_t flops = 2ull * shape.m * shape.n * shape.k;
        if (flops <= 2ull * 64u * 64u * 64u) {
            return 2u;
        }
        return 1u;
    }

    std::uint32_t measured_iterations_for(const ShapeCase& shape)
    {
        const std::uint64_t flops = 2ull * shape.m * shape.n * shape.k;
        if (flops <= 2ull * 64u * 64u * 64u) {
            return 9u;
        }
        if (flops <= 2ull * 256u * 256u * 256u) {
            return 7u;
        }
        if (flops <= 2ull * 512u * 512u * 512u) {
            return 5u;
        }
        return 3u;
    }

    bool create_runtime(RuntimeHandleScope& runtime, PrometheusCaps& caps, std::string& failure_reason)
    {
        runtime.handle = nullptr;
        if (prometheus_reactor_runtime_create(nullptr, &runtime.handle) != PROM_OK || runtime.handle == nullptr) {
            failure_reason = "runtime_create_failed";
            return false;
        }
        if (prometheus_reactor_runtime_probe(runtime.handle, &caps) != PROM_OK) {
            failure_reason = "runtime_probe_failed";
            return false;
        }
        if (caps.available == 0u) {
            failure_reason = "vulkan_runtime_unavailable";
            return false;
        }
        return true;
    }

    prom_sgemm_dispatch_geometry dispatch_geometry_for_variant(std::uint32_t variant, std::uint32_t m, std::uint32_t n)
    {
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS) {
            const prom_sgemm_kernel_dispatch_metadata metadata{
                k_prom_sgemm_scalar_plus_spirv_numthreads_x,
                k_prom_sgemm_scalar_plus_spirv_numthreads_y,
                k_prom_sgemm_scalar_plus_spirv_numthreads_z,
                k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_m,
                k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_n,
                k_prom_sgemm_scalar_plus_spirv_tile_m,
                k_prom_sgemm_scalar_plus_spirv_tile_n,
                0u,
                k_prom_sgemm_scalar_plus_spirv_unroll_k
            };
            return prom_sgemm_dispatch_geometry_for_metadata(m, n, &metadata);
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32) {
            const prom_sgemm_kernel_dispatch_metadata metadata{
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_x,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_y,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_z,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_m,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_n,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_m,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_n,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_k,
                k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_unroll_k
            };
            return prom_sgemm_dispatch_geometry_for_metadata(m, n, &metadata);
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32) {
            const prom_sgemm_kernel_dispatch_metadata metadata{
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_x,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_y,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_z,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_m,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_n,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_m,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_n,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_k,
                k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_unroll_k
            };
            return prom_sgemm_dispatch_geometry_for_metadata(m, n, &metadata);
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32) {
            const prom_sgemm_kernel_dispatch_metadata metadata{
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_x,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_y,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_z,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_m,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_n,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_m,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_n,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_k,
                k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_unroll_k
            };
            return prom_sgemm_dispatch_geometry_for_metadata(m, n, &metadata);
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32) {
            const prom_sgemm_kernel_dispatch_metadata metadata{
                k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_x,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_y,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_z,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_m,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_n,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_m,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_n,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_k,
                k_prom_sgemm_tile16x16_shared_fp32_spirv_unroll_k
            };
            return prom_sgemm_dispatch_geometry_for_metadata(m, n, &metadata);
        }

        prom_sgemm_dispatch_geometry geometry{};
        geometry.groups_x = prom_sgemm_ceil_div_u32(m, 8u);
        geometry.groups_y = prom_sgemm_ceil_div_u32(n, 8u);
        geometry.groups_z = 1u;
        geometry.logical_m_per_group = 8u;
        geometry.logical_n_per_group = 8u;
        return geometry;
    }

    void fill_variant_metadata(std::uint32_t variant, ResidentFailureMatrixRow& row)
    {
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS) {
            row.metadata_numthreads_x = k_prom_sgemm_scalar_plus_spirv_numthreads_x;
            row.metadata_numthreads_y = k_prom_sgemm_scalar_plus_spirv_numthreads_y;
            row.metadata_numthreads_z = k_prom_sgemm_scalar_plus_spirv_numthreads_z;
            row.metadata_outputs_per_invocation_m = k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_m;
            row.metadata_outputs_per_invocation_n = k_prom_sgemm_scalar_plus_spirv_outputs_per_invocation_n;
            row.metadata_tile_m = k_prom_sgemm_scalar_plus_spirv_tile_m;
            row.metadata_tile_n = k_prom_sgemm_scalar_plus_spirv_tile_n;
            row.metadata_tile_k = 0u;
            row.metadata_unroll_k = k_prom_sgemm_scalar_plus_spirv_unroll_k;
            row.shader_module_present = true;
            return;
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32) {
            row.metadata_numthreads_x = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_x;
            row.metadata_numthreads_y = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_y;
            row.metadata_numthreads_z = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_numthreads_z;
            row.metadata_outputs_per_invocation_m = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_m;
            row.metadata_outputs_per_invocation_n = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_outputs_per_invocation_n;
            row.metadata_tile_m = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_m;
            row.metadata_tile_n = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_n;
            row.metadata_tile_k = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_tile_k;
            row.metadata_unroll_k = k_prom_sgemm_reg2x2_tile16x16_fp32_spirv_unroll_k;
            row.shader_module_present = true;
            return;
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32) {
            row.metadata_numthreads_x = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_x;
            row.metadata_numthreads_y = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_y;
            row.metadata_numthreads_z = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_z;
            row.metadata_outputs_per_invocation_m = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_m;
            row.metadata_outputs_per_invocation_n = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_n;
            row.metadata_tile_m = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_m;
            row.metadata_tile_n = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_n;
            row.metadata_tile_k = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_k;
            row.metadata_unroll_k = k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_unroll_k;
            row.shader_module_present = true;
            return;
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32) {
            row.metadata_numthreads_x = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_x;
            row.metadata_numthreads_y = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_y;
            row.metadata_numthreads_z = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_z;
            row.metadata_outputs_per_invocation_m = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_m;
            row.metadata_outputs_per_invocation_n = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_n;
            row.metadata_tile_m = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_m;
            row.metadata_tile_n = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_n;
            row.metadata_tile_k = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_k;
            row.metadata_unroll_k = k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_unroll_k;
            row.shader_module_present = true;
            return;
        }
        if (variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32) {
            row.metadata_numthreads_x = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_x;
            row.metadata_numthreads_y = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_y;
            row.metadata_numthreads_z = k_prom_sgemm_tile16x16_shared_fp32_spirv_numthreads_z;
            row.metadata_outputs_per_invocation_m = k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_m;
            row.metadata_outputs_per_invocation_n = k_prom_sgemm_tile16x16_shared_fp32_spirv_outputs_per_invocation_n;
            row.metadata_tile_m = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_m;
            row.metadata_tile_n = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_n;
            row.metadata_tile_k = k_prom_sgemm_tile16x16_shared_fp32_spirv_tile_k;
            row.metadata_unroll_k = k_prom_sgemm_tile16x16_shared_fp32_spirv_unroll_k;
            row.shader_module_present = true;
            return;
        }

        row.metadata_numthreads_x = 8u;
        row.metadata_numthreads_y = 8u;
        row.metadata_numthreads_z = 1u;
        row.metadata_outputs_per_invocation_m = 1u;
        row.metadata_outputs_per_invocation_n = 1u;
        row.metadata_tile_m = 1u;
        row.metadata_tile_n = 1u;
        row.metadata_tile_k = 0u;
        row.metadata_unroll_k = 0u;
        row.shader_module_present =
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8;
    }

    std::string vk_result_name(int detail_code)
    {
        switch (detail_code) {
            case VK_SUCCESS:
                return "VK_SUCCESS";
            case VK_ERROR_DEVICE_LOST:
                return "VK_ERROR_DEVICE_LOST";
            case VK_ERROR_OUT_OF_HOST_MEMORY:
                return "VK_ERROR_OUT_OF_HOST_MEMORY";
            case VK_ERROR_OUT_OF_DEVICE_MEMORY:
                return "VK_ERROR_OUT_OF_DEVICE_MEMORY";
            case VK_ERROR_INITIALIZATION_FAILED:
                return "VK_ERROR_INITIALIZATION_FAILED";
            default:
                return detail_code < 0 ? ("VK_RESULT_" + std::to_string(detail_code)) : "none";
        }
    }

    std::string diagnostic_failure_stage(const VariantComparisonRow& row)
    {
        if (row.runtime_status == PROM_OK) {
            return "none";
        }
        if (std::find(row.anomalies.begin(), row.anomalies.end(), "warmup_call_failed") != row.anomalies.end()) {
            return "warmup_dispatch";
        }
        if (std::find(row.anomalies.begin(), row.anomalies.end(), "measured_call_failed") != row.anomalies.end()) {
            return "measured_dispatch";
        }
        if (row.final_stage == PROM_STAGE_TRANSFER_IN) {
            return "allocate_buffers";
        }
        return "unknown";
    }

    std::string diagnostic_failure_stage(const ResidentBenchmarkRow& row)
    {
        if (row.runtime_status == PROM_OK) {
            return "none";
        }
        if (row.resident_mode_used && row.iterations == 0u) {
            return "warmup_dispatch";
        }
        if (row.setup_stage != PROM_STAGE_NONE && row.resident_mode_used == false) {
            return "setup_dispatch";
        }
        if (row.setup_stage == PROM_STAGE_TRANSFER_IN) {
            return "allocate_buffers";
        }
        return "unknown";
    }

    std::vector<float> dense_deterministic_matrix(std::uint32_t rows, std::uint32_t cols, std::uint32_t salt)
    {
        std::vector<float> out(static_cast<std::size_t>(rows) * static_cast<std::size_t>(cols), 0.0f);
        for (std::size_t i = 0; i < out.size(); ++i) {
            const std::uint32_t mixed = static_cast<std::uint32_t>((i * 1103515245u) + 12345u + (salt * 2654435761u));
            const int bounded = static_cast<int>(mixed % 29u) - 14;
            out[i] = static_cast<float>(bounded) / 16.0f;
        }
        return out;
    }

    std::vector<float> separable_matrix_a(std::uint32_t m, std::uint32_t k)
    {
        std::vector<float> out(static_cast<std::size_t>(m) * static_cast<std::size_t>(k), 0.0f);
        for (std::uint32_t row = 0u; row < m; ++row) {
            const float row_factor = static_cast<float>((row % 19u) + 1u) / 19.0f;
            for (std::uint32_t col = 0u; col < k; ++col) {
                const float col_factor = static_cast<float>((col % 17u) + 1u) / 17.0f;
                out[static_cast<std::size_t>(row) * k + col] = row_factor * col_factor;
            }
        }
        return out;
    }

    std::vector<float> separable_matrix_b(std::uint32_t k, std::uint32_t n)
    {
        std::vector<float> out(static_cast<std::size_t>(k) * static_cast<std::size_t>(n), 0.0f);
        for (std::uint32_t row = 0u; row < k; ++row) {
            const float row_factor = static_cast<float>((row % 23u) + 1u) / 23.0f;
            for (std::uint32_t col = 0u; col < n; ++col) {
                const float col_factor = static_cast<float>((col % 29u) + 1u) / 29.0f;
                out[static_cast<std::size_t>(row) * n + col] = row_factor * col_factor;
            }
        }
        return out;
    }

    std::vector<float> cpu_oracle_dense(std::uint32_t m,
                                        std::uint32_t n,
                                        std::uint32_t k,
                                        const std::vector<float>& a,
                                        const std::vector<float>& b)
    {
        std::vector<float> c(static_cast<std::size_t>(m) * static_cast<std::size_t>(n), 0.0f);
        for (std::uint32_t row = 0u; row < m; ++row) {
            for (std::uint32_t col = 0u; col < n; ++col) {
                float sum = 0.0f;
                for (std::uint32_t kk = 0u; kk < k; ++kk) {
                    sum += a[static_cast<std::size_t>(row) * k + kk] * b[static_cast<std::size_t>(kk) * n + col];
                }
                c[static_cast<std::size_t>(row) * n + col] = sum;
            }
        }
        return c;
    }

    std::vector<float> cpu_oracle_separable(std::uint32_t m, std::uint32_t n, std::uint32_t k)
    {
        std::vector<float> row_factor(m, 0.0f);
        std::vector<float> col_factor(n, 0.0f);
        for (std::uint32_t row = 0u; row < m; ++row) {
            row_factor[row] = static_cast<float>((row % 19u) + 1u) / 19.0f;
        }
        for (std::uint32_t col = 0u; col < n; ++col) {
            col_factor[col] = static_cast<float>((col % 29u) + 1u) / 29.0f;
        }

        double inner_sum = 0.0;
        for (std::uint32_t kk = 0u; kk < k; ++kk) {
            const double a_col = static_cast<double>((kk % 17u) + 1u) / 17.0;
            const double b_row = static_cast<double>((kk % 23u) + 1u) / 23.0;
            inner_sum += a_col * b_row;
        }

        std::vector<float> c(static_cast<std::size_t>(m) * static_cast<std::size_t>(n), 0.0f);
        for (std::uint32_t row = 0u; row < m; ++row) {
            for (std::uint32_t col = 0u; col < n; ++col) {
                c[static_cast<std::size_t>(row) * n + col] =
                    static_cast<float>(static_cast<double>(row_factor[row]) * static_cast<double>(col_factor[col]) * inner_sum);
            }
        }
        return c;
    }

    CorrectnessResult compare_with_tolerance(const std::vector<float>& expected, const std::vector<float>& actual)
    {
        CorrectnessResult result;
        result.status = "pass";
        if (expected.size() != actual.size()) {
            result.status = "fail";
            result.max_abs_error = std::numeric_limits<float>::infinity();
            result.max_rel_error = std::numeric_limits<float>::infinity();
            return result;
        }

        for (std::size_t i = 0; i < expected.size(); ++i) {
            const float abs_error = std::fabs(expected[i] - actual[i]);
            const float denom = std::max(std::fabs(expected[i]), 1.0e-8f);
            const float rel_error = abs_error / denom;
            result.max_abs_error = std::max(result.max_abs_error, abs_error);
            result.max_rel_error = std::max(result.max_rel_error, rel_error);
            if (!std::isfinite(actual[i]) || (abs_error > kAbsTolerance && rel_error > kRelTolerance)) {
                result.status = "fail";
            }
        }
        return result;
    }

    std::size_t percentile_index(std::size_t size, double percentile)
    {
        if (size == 0u) {
            return 0u;
        }
        const double scaled = percentile * static_cast<double>(size - 1u);
        return static_cast<std::size_t>(std::ceil(scaled));
    }

    double median_ms_from_ns(std::vector<double> samples_ns)
    {
        if (samples_ns.empty()) {
            return 0.0;
        }
        std::sort(samples_ns.begin(), samples_ns.end());
        return samples_ns[samples_ns.size() / 2u] / 1.0e6;
    }

    double gflops_from_ns(const ShapeCase& shape, double duration_ns)
    {
        if (duration_ns <= 0.0) {
            return 0.0;
        }
        const double flops =
            2.0 * static_cast<double>(shape.m) * static_cast<double>(shape.n) * static_cast<double>(shape.k);
        return (flops / (duration_ns / 1.0e9)) / 1.0e9;
    }

    double ns_to_ms(std::uint64_t ns)
    {
        return static_cast<double>(ns) / 1.0e6;
    }

    double accounted_host_ms(const TimingDecomposition& timing_decomposition)
    {
        return timing_decomposition.pre_dispatch_ms +
            timing_decomposition.command_record_ms +
            timing_decomposition.dispatch_submit_ms +
            timing_decomposition.sync_wait_ms +
            timing_decomposition.post_sync_ms +
            timing_decomposition.readback_ms +
            timing_decomposition.post_readback_ms +
            (timing_decomposition.gpu_timestamp_valid ? timing_decomposition.kernel_gpu_ms : 0.0);
    }

    void finalize_timing_stats(const ShapeCase& shape,
                               const std::vector<double>& samples_ns,
                               std::string_view timing_source,
                               TimingStats& timing)
    {
        timing.timing_source = std::string(timing_source);
        if (samples_ns.empty()) {
            return;
        }

        std::vector<double> sorted = samples_ns;
        std::sort(sorted.begin(), sorted.end());
        timing.min_ms = sorted.front() / 1.0e6;
        timing.median_ms = sorted[sorted.size() / 2u] / 1.0e6;
        timing.p95_ms = sorted[percentile_index(sorted.size(), 0.95)] / 1.0e6;
        timing.gflops_median = gflops_from_ns(shape, sorted[sorted.size() / 2u]);
    }

    bool is_large_shape(const ShapeCase& shape)
    {
        return shape.m >= 512u && shape.n >= 512u && shape.k >= 512u;
    }

    struct MeasurementBundle
    {
        std::vector<double> total_wall_ns;
        std::vector<double> kernel_gpu_ns;
        std::vector<double> upload_wall_ns;
        std::vector<double> pre_dispatch_wall_ns;
        std::vector<double> command_record_wall_ns;
        std::vector<double> dispatch_submit_wall_ns;
        std::vector<double> sync_wait_wall_ns;
        std::vector<double> post_sync_wall_ns;
        std::vector<double> readback_wall_ns;
        std::vector<double> post_readback_wall_ns;
        std::vector<double> tolerance_eval_wall_ns;
    };

    struct PreparedCaseData
    {
        bool use_dense_reference = true;
        std::vector<float> a;
        std::vector<float> b;
        std::string reference_mode;
    };

    PreparedCaseData prepare_case_data(const ShapeCase& shape)
    {
        PreparedCaseData data;
        data.use_dense_reference = static_cast<std::uint64_t>(shape.m) * shape.n * shape.k <= 16ull * 1024ull * 1024ull;
        data.a = data.use_dense_reference
            ? dense_deterministic_matrix(shape.m, shape.k, shape.m ^ (shape.k << 4u))
            : separable_matrix_a(shape.m, shape.k);
        data.b = data.use_dense_reference
            ? dense_deterministic_matrix(shape.k, shape.n, shape.k ^ (shape.n << 5u) ^ 17u)
            : separable_matrix_b(shape.k, shape.n);
        data.reference_mode = data.use_dense_reference ? "dense_cpu_oracle" : "separable_rank1_oracle";
        return data;
    }

    CorrectnessResult validate_case_output(const ShapeCase& shape,
                                           const PreparedCaseData& data,
                                           const std::vector<float>& actual,
                                           TimingDecomposition& timing_decomposition)
    {
        const auto validation_begin = std::chrono::steady_clock::now();
        const auto oracle_begin = std::chrono::steady_clock::now();
        const std::vector<float> expected = data.use_dense_reference
            ? cpu_oracle_dense(shape.m, shape.n, shape.k, data.a, data.b)
            : cpu_oracle_separable(shape.m, shape.n, shape.k);
        const auto oracle_end = std::chrono::steady_clock::now();
        CorrectnessResult result = compare_with_tolerance(expected, actual);
        const auto validation_end = std::chrono::steady_clock::now();

        result.reference_mode = data.reference_mode;
        result.source = "explicit_validation_lane";
        timing_decomposition.oracle_ms =
            static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(oracle_end - oracle_begin).count()) / 1.0e6;
        timing_decomposition.validation_ms =
            static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(validation_end - validation_begin).count()) / 1.0e6;
        timing_decomposition.validation_readback_ms = 0.0;
        return result;
    }

    void detect_case_anomalies(CaseResult& result)
    {
        if (result.correctness.status == "fail") {
            result.anomalies.push_back("correctness_failure");
        }
        if (result.diag.px16_m6_selector_selected_variant != result.diag.px16_m6_requested_dispatch_variant) {
            result.anomalies.push_back("selector_selected_variant_differs_from_requested_dispatch_variant");
        }
        if (result.diag.px16_m6_selected_compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED) &&
            result.diag.px16_m6_requested_dispatch_variant != result.diag.px16_m6_executed_dispatch_variant) {
            result.anomalies.push_back("requested_dispatch_variant_differs_from_executed_variant_on_tiled_path");
        }
        if (result.diag.px16_m6_selected_path != result.diag.px16_m6_executed_path &&
            result.diag.px16_m6_executed_path == static_cast<std::uint32_t>(PROM_VK_PATH_DIRECT)) {
            result.anomalies.push_back("executed_path_fell_back_to_direct");
        }
        if (result.diag.px16_m6_policy_mode == static_cast<std::uint32_t>(PROM_POLICY_MODE_SAFE) &&
            result.diag.px16_m6_force_direct_applied != 0u &&
            result.diag.px16_m6_force_direct_reason == static_cast<std::uint32_t>(PROM_SGEMM_FORCE_DIRECT_REASON_NONE)) {
            result.anomalies.push_back("safe_policy_force_direct_without_reason");
        }
        if (result.diag.px16_m6_requested_dispatch_variant != static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) &&
            result.diag.px16_m6_variant_path_status != static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED)) {
            result.anomalies.push_back("wired_variant_not_reported_as_wired");
        }
        if (result.diag.px16_m6_requested_dispatch_variant != static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) &&
            result.diag.px16_m6_variant_production_eligible == 0u) {
            result.anomalies.push_back("wired_variant_not_reported_production_eligible");
        }
        if (result.diag.px16_m6_requested_dispatch_variant != static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) &&
            result.diag.px16_m6_variant_dispatch_enabled == 0u) {
            result.anomalies.push_back("wired_variant_not_reported_dispatch_enabled");
        }
        if (result.diag.px16_m6_p15_reservation_present != 0u &&
            result.diag.px16_m6_p15_reconciliation_match == 0u &&
            result.diag.px16_m6_p15_correction_action == static_cast<std::uint32_t>(PROM_DOM_CORRECTION_ACTION_NONE)) {
            result.anomalies.push_back("p15_mismatch_without_correction_action");
        }
        if (is_large_shape({ "", result.m, result.n, result.k, false }) &&
            result.diag.px16_m6_variant_production_eligible != 0u &&
            result.diag.px16_m6_executed_compute_mode != static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED)) {
            result.anomalies.push_back("large_eligible_shape_did_not_execute_tiled");
        }
        if (!result.timing_decomposition.gpu_timestamp_valid) {
            result.anomalies.push_back("kernel_timing_unavailable");
        }
        if (result.diag.px16_m6_executed_path == static_cast<std::uint32_t>(PROM_VK_PATH_STAGED_UPLOAD_READBACK) &&
            result.timing_decomposition.upload_ms + result.timing_decomposition.readback_ms + result.timing_decomposition.sync_wait_ms >
                std::max(result.timing_decomposition.kernel_gpu_ms, 0.0) * 1.2) {
            result.anomalies.push_back("staging_transfer_overhead_dominates");
        }
    }

    void detect_cross_case_anomalies(ReportData& report)
    {
        std::vector<CaseResult*> successful;
        for (CaseResult& result : report.cases) {
            if (!result.skipped && result.runtime_status == PROM_OK && result.timing.gflops_median > 0.0) {
                successful.push_back(&result);
            }
        }

        std::sort(successful.begin(), successful.end(), [](const CaseResult* left, const CaseResult* right) {
            const std::uint64_t left_work = static_cast<std::uint64_t>(left->m) * left->n * left->k;
            const std::uint64_t right_work = static_cast<std::uint64_t>(right->m) * right->n * right->k;
            return left_work < right_work;
        });

        for (std::size_t i = 1u; i < successful.size(); ++i) {
            const double previous = successful[i - 1u]->timing.gflops_median;
            const double current = successful[i]->timing.gflops_median;
            if (previous > 0.0 && current < previous * 0.5) {
                successful[i]->anomalies.push_back("neighbor_gflops_drop");
            }
        }
    }

    void finalize_summary(ReportData& report)
    {
        report.summary.cases_total = report.cases.size();
        std::vector<double> passing_gflops;
        for (const CaseResult& result : report.cases) {
            if (result.skipped) {
                ++report.summary.cases_skipped;
                continue;
            }
            report.summary.anomaly_count += result.anomalies.size();
            if (result.runtime_status == PROM_OK &&
                (result.correctness.status == "pass" || result.correctness.status == "not_run_in_benchmark_mode")) {
                ++report.summary.cases_passed;
                if (result.timing.gflops_median > 0.0) {
                    passing_gflops.push_back(result.timing.gflops_median);
                    report.summary.best_gflops = std::max(report.summary.best_gflops, result.timing.gflops_median);
                }
            } else {
                ++report.summary.cases_failed;
            }
        }
        if (!passing_gflops.empty()) {
            std::sort(passing_gflops.begin(), passing_gflops.end());
            report.summary.median_gflops = passing_gflops[passing_gflops.size() / 2u];
        }
    }

    bool fetch_device_metadata(void* handle, const PrometheusCaps& caps, DeviceMetadata& device)
    {
        prom_vk_runtime_services services{};
        if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK ||
            services.physical_device == VK_NULL_HANDLE) {
            return false;
        }

        VkPhysicalDeviceProperties props{};
        vkGetPhysicalDeviceProperties(services.physical_device, &props);
        device.name = props.deviceName;
        device.backend = backend_name(caps.backend_type);
        device.device_type = vulkan_device_type_name(props.deviceType);
        device.vendor_id = props.vendorID;
        device.device_id = props.deviceID;
        device.driver_version = vulkan_version_name(props.driverVersion);
        device.api_version = vulkan_version_name(props.apiVersion);
        device.max_compute_workgroup_invocations = props.limits.maxComputeWorkGroupInvocations;
        device.max_compute_shared_memory_size = props.limits.maxComputeSharedMemorySize;
        device.subgroup_size = 0u;
        return true;
    }

    bool collect_measurement(void* handle,
                             const ShapeCase& shape,
                             const PreparedCaseData& data,
                             bool explicit_variant,
                             std::uint32_t requested_variant,
                             bool validate_output,
                             TimingStats& timing,
                             TimingDecomposition& timing_decomposition,
                             CorrectnessResult& correctness,
                             PrometheusSgemmPolicyDiagnostics& diag,
                             std::uint32_t& final_stage,
                             int& final_detail_code,
                             int& runtime_status,
                             std::vector<std::string>& anomalies)
    {
        std::vector<float> c(static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n), 0.0f);
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail_code = 0;
        timing.warmup_iterations = warmup_iterations_for(shape);
        timing.iterations = measured_iterations_for(shape);

        for (std::uint32_t i = 0u; i < timing.warmup_iterations; ++i) {
            const int status = explicit_variant
                ? prometheus_reactor_runtime_sgemm_benchmark_variant(
                      handle, data.a.data(), data.b.data(), c.data(), shape.m, shape.n, shape.k, requested_variant, &stage, &detail_code)
                : prometheus_reactor_runtime_sgemm(handle, data.a.data(), data.b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail_code);
            if (status != PROM_OK) {
                runtime_status = status;
                final_stage = stage;
                final_detail_code = detail_code;
                anomalies.push_back("warmup_call_failed");
                correctness.status = "fail";
                return false;
            }
        }

        MeasurementBundle samples;
        for (std::uint32_t i = 0u; i < timing.iterations; ++i) {
            const auto begin = std::chrono::steady_clock::now();
            const int status = explicit_variant
                ? prometheus_reactor_runtime_sgemm_benchmark_variant(
                      handle, data.a.data(), data.b.data(), c.data(), shape.m, shape.n, shape.k, requested_variant, &stage, &detail_code)
                : prometheus_reactor_runtime_sgemm(handle, data.a.data(), data.b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail_code);
            const auto end = std::chrono::steady_clock::now();
            samples.total_wall_ns.push_back(static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(end - begin).count()));

            if (status != PROM_OK) {
                runtime_status = status;
                final_stage = stage;
                final_detail_code = detail_code;
                anomalies.push_back("measured_call_failed");
                correctness.status = "fail";
                return false;
            }

            if (prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag) != PROM_OK) {
                runtime_status = PROM_ERROR;
                final_stage = stage;
                final_detail_code = detail_code;
                anomalies.push_back("policy_diagnostics_query_failed");
                correctness.status = "fail";
                return false;
            }

            samples.upload_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_upload_wall_ns));
            samples.pre_dispatch_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_pre_dispatch_wall_ns));
            samples.command_record_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_command_record_wall_ns));
            samples.dispatch_submit_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_dispatch_submit_wall_ns));
            samples.sync_wait_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_sync_wait_wall_ns));
            samples.post_sync_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_post_sync_wall_ns));
            samples.readback_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_readback_wall_ns));
            samples.post_readback_wall_ns.push_back(static_cast<double>(diag.px16_m8_last_post_readback_wall_ns));
            samples.tolerance_eval_wall_ns.push_back(static_cast<double>(diag.px16_m17_last_tolerance_eval_wall_ns));
            if (diag.p13_m5_last_gpu_timing_valid != 0u && diag.p13_m5_last_gpu_duration_ns > 0u) {
                samples.kernel_gpu_ns.push_back(static_cast<double>(diag.p13_m5_last_gpu_duration_ns));
            }
        }

        final_stage = stage;
        final_detail_code = detail_code;
        timing_decomposition.upload_ms = median_ms_from_ns(samples.upload_wall_ns);
        timing_decomposition.pre_dispatch_ms = median_ms_from_ns(samples.pre_dispatch_wall_ns);
        timing_decomposition.command_record_ms = median_ms_from_ns(samples.command_record_wall_ns);
        timing_decomposition.dispatch_submit_ms = median_ms_from_ns(samples.dispatch_submit_wall_ns);
        timing_decomposition.sync_wait_ms = median_ms_from_ns(samples.sync_wait_wall_ns);
        timing_decomposition.post_sync_ms = median_ms_from_ns(samples.post_sync_wall_ns);
        timing_decomposition.readback_ms = median_ms_from_ns(samples.readback_wall_ns);
        timing_decomposition.post_readback_ms = median_ms_from_ns(samples.post_readback_wall_ns);
        timing_decomposition.tolerance_eval_ms = median_ms_from_ns(samples.tolerance_eval_wall_ns);
        timing_decomposition.total_wall_ms = median_ms_from_ns(samples.total_wall_ns);
        timing_decomposition.benchmark_total_ms = timing_decomposition.total_wall_ms;
        timing_decomposition.gpu_timestamp_valid = samples.kernel_gpu_ns.size() == samples.total_wall_ns.size() && !samples.kernel_gpu_ns.empty();
        timing_decomposition.kernel_gpu_ms = timing_decomposition.gpu_timestamp_valid ? median_ms_from_ns(samples.kernel_gpu_ns) : 0.0;
        timing_decomposition.unaccounted_host_ms = timing_decomposition.total_wall_ms - accounted_host_ms(timing_decomposition);
        timing_decomposition.end_to_end_gflops =
            gflops_from_ns(shape, timing_decomposition.total_wall_ms * 1.0e6);
        timing_decomposition.kernel_only_gflops =
            timing_decomposition.gpu_timestamp_valid ? gflops_from_ns(shape, timing_decomposition.kernel_gpu_ms * 1.0e6) : 0.0;
        timing_decomposition.timing_source = timing_decomposition.gpu_timestamp_valid ? "mixed" : "cpu_wall";

        if (timing_decomposition.gpu_timestamp_valid) {
            finalize_timing_stats(shape, samples.kernel_gpu_ns, "vulkan_timestamp_query", timing);
        } else {
            finalize_timing_stats(shape, samples.total_wall_ns, "cpu_wall_clock", timing);
        }
        if (validate_output) {
            correctness = validate_case_output(shape, data, c, timing_decomposition);
        } else {
            correctness.status = "not_run_in_benchmark_mode";
            correctness.reference_mode = "none";
            correctness.source = "see_correctness_lane";
        }
        return true;
    }

    void apply_resident_result(const ShapeCase& shape,
                               const PrometheusSgemmResidentBenchmarkResult& native,
                               ResidentBenchmarkRow& row)
    {
        row.resident_mode_available = native.resident_mode_available != 0u;
        row.resident_mode_used = native.resident_mode_used != 0u;
        row.executed_variant = occupancy_variant_name(native.executed_variant);
        row.setup_stage = native.setup_stage;
        row.setup_detail_code = native.setup_detail_code;
        row.final_stage = native.final_stage;
        row.final_detail_code = native.final_detail_code;
        row.upload_once_ms = ns_to_ms(native.upload_once_wall_ns);
        row.setup_ms = ns_to_ms(native.setup_wall_ns);
        row.kernel_median_ms = ns_to_ms(native.kernel_median_ns);
        row.kernel_min_ms = ns_to_ms(native.kernel_min_ns);
        row.kernel_p95_ms = ns_to_ms(native.kernel_p95_ns);
        row.total_loop_ms = ns_to_ms(native.total_loop_wall_ns);
        row.iterations = native.iterations;
        row.readback_once_ms = ns_to_ms(native.readback_once_wall_ns);
        row.validation_ms = ns_to_ms(native.validation_wall_ns);
        row.gpu_timestamp_valid = native.gpu_timestamp_valid != 0u;
        row.gpu_timing_failure_reason = timing_failure_reason_name(native.gpu_timing_failure_reason);
        row.kernel_only_gflops = row.gpu_timestamp_valid ? gflops_from_ns(shape, static_cast<double>(native.kernel_median_ns)) : 0.0;
        row.loop_gflops = native.total_loop_wall_ns > 0u && native.iterations > 0u
            ? gflops_from_ns(shape, static_cast<double>(native.total_loop_wall_ns) / static_cast<double>(native.iterations))
            : 0.0;
    }

    ResidentBenchmarkRow run_resident_case(void* handle,
                                           const ShapeCase& shape,
                                           bool explicit_variant,
                                           std::uint32_t requested_variant,
                                           bool validate_output)
    {
        ResidentBenchmarkRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;
        row.mode = explicit_variant ? "explicit_variant" : "production_selector";
        row.variant = explicit_variant ? occupancy_variant_name(requested_variant) : "PRODUCTION_SELECTOR";
        row.requested_variant = explicit_variant ? occupancy_variant_name(requested_variant) : "selector";

        if (shape.optional && explicit_variant && !should_run_explicit_1024_cube()) {
            row.skipped = true;
            row.skip_reason = "explicit_1024_cube_disabled";
            return row;
        }
        if (shape.optional && !explicit_variant && !should_run_optional_large_case()) {
            row.skipped = true;
            row.skip_reason = "optional_large_case_disabled_for_local_repeatability";
            return row;
        }

        const PreparedCaseData data = prepare_case_data(shape);
        std::vector<float> c(static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n), 0.0f);
        PrometheusSgemmResidentBenchmarkRequest request{};
        request.struct_size = sizeof(request);
        request.a = data.a.data();
        request.b = data.b.data();
        request.c = validate_output ? c.data() : nullptr;
        request.m = shape.m;
        request.n = shape.n;
        request.k = shape.k;
        request.mode = explicit_variant ? PROM_SGEMM_RESIDENT_MODE_EXPLICIT_VARIANT : PROM_SGEMM_RESIDENT_MODE_PRODUCTION_SELECTOR;
        request.requested_variant = requested_variant;
        request.warmup_iterations = warmup_iterations_for(shape);
        request.iterations = measured_iterations_for(shape);
        request.flags = validate_output ? PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK : 0u;

        PrometheusSgemmResidentBenchmarkResult native{};
        row.runtime_status = prometheus_reactor_runtime_sgemm_resident_benchmark(handle, &request, &native);
        apply_resident_result(shape, native, row);
        row.final_stage = native.final_stage != PROM_STAGE_NONE ? native.final_stage : native.setup_stage;
        row.final_detail_code = native.final_detail_code != 0 ? native.final_detail_code : native.setup_detail_code;

        if (prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &row.diag) != PROM_OK) {
            row.anomalies.push_back("policy_diagnostics_query_failed");
        }

        if (row.runtime_status != PROM_OK) {
            if (row.resident_mode_available && row.resident_mode_used && row.iterations == 0u) {
                row.skipped = true;
                row.skip_reason = "resident_setup_did_not_produce_device_local_staged_buffers";
                row.anomalies.push_back(row.skip_reason);
                row.correctness = "not_run_in_benchmark_mode";
            } else {
                row.anomalies.push_back(row.resident_mode_available ? "resident_runtime_failed" : "resident_mode_unavailable");
                row.correctness = "fail";
            }
            return row;
        }

        if (!row.resident_mode_available || !row.resident_mode_used) {
            row.anomalies.push_back("resident_mode_unavailable");
        }
        if (explicit_variant && native.executed_variant != requested_variant &&
            native.executed_compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED)) {
            row.anomalies.push_back("resident_explicit_requested_variant_did_not_execute");
        }
        if (!explicit_variant && native.production_selected_variant != native.executed_variant &&
            native.executed_compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED)) {
            row.anomalies.push_back("resident_production_selected_differs_from_executed");
        }
        if (!row.gpu_timestamp_valid) {
            row.anomalies.push_back("gpu_timestamp_unavailable");
        }

        if (validate_output) {
            TimingDecomposition validation_timing;
            row.correctness = validate_case_output(shape, data, c, validation_timing).status;
            row.reference_mode = data.reference_mode;
            row.validation_ms += validation_timing.validation_ms;
            if (row.correctness != "pass") {
                row.anomalies.push_back("resident_correctness_failure");
            }
        } else {
            row.correctness = "not_run_in_benchmark_mode";
            row.reference_mode = "none";
        }
        return row;
    }

    VariantComparisonRow run_explicit_variant_case_fresh(const ShapeCase& shape, std::uint32_t variant, bool validate_output)
    {
        VariantComparisonRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;
        row.variant = occupancy_variant_name(variant);
        row.requested_variant = occupancy_variant_name(variant);

        RuntimeHandleScope runtime;
        PrometheusCaps caps{};
        std::string failure_reason;
        if (!create_runtime(runtime, caps, failure_reason)) {
            row.runtime_status = PROM_ERROR;
            row.final_stage = PROM_STAGE_INIT;
            row.final_detail_code = PROM_ERROR;
            row.anomalies.push_back(failure_reason);
            return row;
        }
        return run_explicit_variant_case(runtime.handle, shape, variant, validate_output);
    }

    ResidentBenchmarkRow run_resident_case_fresh(const ShapeCase& shape,
                                                 bool explicit_variant,
                                                 std::uint32_t requested_variant,
                                                 bool validate_output)
    {
        ResidentBenchmarkRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;
        row.mode = explicit_variant ? "explicit_variant" : "production_selector";
        row.variant = explicit_variant ? occupancy_variant_name(requested_variant) : "PRODUCTION_SELECTOR";
        row.requested_variant = explicit_variant ? occupancy_variant_name(requested_variant) : "selector";

        RuntimeHandleScope runtime;
        PrometheusCaps caps{};
        std::string failure_reason;
        if (!create_runtime(runtime, caps, failure_reason)) {
            row.runtime_status = PROM_ERROR;
            row.final_stage = PROM_STAGE_INIT;
            row.final_detail_code = PROM_ERROR;
            row.anomalies.push_back(failure_reason);
            return row;
        }
        return run_resident_case(runtime.handle, shape, explicit_variant, requested_variant, validate_output);
    }

    ResidentFailureMatrixRow build_failure_matrix_row(const ShapeCase& shape, std::uint32_t variant)
    {
        ResidentFailureMatrixRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;
        row.requested_variant = occupancy_variant_name(variant);
        row.a_elements = static_cast<std::uint64_t>(shape.m) * static_cast<std::uint64_t>(shape.k);
        row.b_elements = static_cast<std::uint64_t>(shape.k) * static_cast<std::uint64_t>(shape.n);
        row.c_elements = static_cast<std::uint64_t>(shape.m) * static_cast<std::uint64_t>(shape.n);
        row.a_bytes = row.a_elements * sizeof(float);
        row.b_bytes = row.b_elements * sizeof(float);
        row.c_bytes = row.c_elements * sizeof(float);
        const prom_sgemm_dispatch_geometry geometry = dispatch_geometry_for_variant(variant, shape.m, shape.n);
        row.dispatch_groups_x = geometry.groups_x;
        row.dispatch_groups_y = geometry.groups_y;
        row.dispatch_groups_z = geometry.groups_z;
        row.logical_m_per_group = geometry.logical_m_per_group;
        row.logical_n_per_group = geometry.logical_n_per_group;
        fill_variant_metadata(variant, row);
        row.selected_pipeline_present =
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 ||
            variant == PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8;
        row.descriptor_update_status = row.selected_pipeline_present;

        const ResidentBenchmarkRow production_resident = run_resident_case_fresh(
            shape,
            false,
            PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
            false);
        row.production_resident_succeeds = production_resident.runtime_status == PROM_OK;
        row.production_resident_executed_variant = production_resident.executed_variant;
        if (!row.production_resident_succeeds) {
            row.notes.push_back("production_resident_failed_on_fresh_runtime");
        }

        const VariantComparisonRow staged = run_explicit_variant_case_fresh(shape, variant, false);
        row.staged_requested_variant = staged.requested_variant;
        row.staged_executed_variant = staged.executed_variant;
        row.staged_path = staged.path;
        row.staged_runtime_status = staged.runtime_status;
        row.staged_out_stage = staged.final_stage;
        row.staged_out_detail_code = staged.final_detail_code;
        row.staged_warmup_result = std::find(staged.anomalies.begin(), staged.anomalies.end(), "warmup_call_failed") != staged.anomalies.end()
            ? "failed"
            : "passed";
        row.staged_measured_result = staged.runtime_status == PROM_OK
            ? "passed"
            : (std::find(staged.anomalies.begin(), staged.anomalies.end(), "measured_call_failed") != staged.anomalies.end()
                ? "failed"
                : "not_run");

        const ResidentBenchmarkRow resident = run_resident_case_fresh(shape, true, variant, false);
        row.resident_executed_variant = resident.executed_variant;
        row.resident_runtime_status = resident.runtime_status;
        row.resident_setup_stage = resident.setup_stage;
        row.resident_setup_detail_code = resident.setup_detail_code;
        row.resident_out_stage = resident.final_stage;
        row.resident_out_detail_code = resident.final_detail_code;
        row.resident_mode_available = resident.resident_mode_available;
        row.resident_iteration_count = resident.iterations;
        row.timestamp_query_available = resident.diag.p13_m5_timestamp_available != 0u;
        row.timestamp_query_valid = resident.diag.p13_m5_last_gpu_timing_valid != 0u;
        row.resident_setup_result = (resident.runtime_status == PROM_OK || resident.resident_mode_used) ? "passed" : "failed";
        row.resident_warmup_result = resident.runtime_status == PROM_OK
            ? "passed"
            : (resident.resident_mode_used && resident.iterations == 0u ? "failed" : "not_run");
        row.resident_measured_result = resident.runtime_status == PROM_OK && resident.iterations > 0u ? "passed" : "not_run";
        row.resident_failure_stage = staged.runtime_status != PROM_OK
            ? diagnostic_failure_stage(staged)
            : diagnostic_failure_stage(resident);
        row.vk_result = vk_result_name(
            staged.runtime_status != PROM_OK ? staged.final_detail_code : resident.final_detail_code);
        if (staged.runtime_status != PROM_OK) {
            row.notes.push_back("fresh_runtime_staged_explicit_failed");
        }
        if (resident.runtime_status != PROM_OK) {
            row.notes.push_back("fresh_runtime_resident_explicit_failed");
        }
        if (resident.runtime_status != PROM_OK &&
            resident.final_stage == PROM_STAGE_INIT &&
            resident.final_detail_code == PROM_ERROR) {
            row.notes.push_back("runtime_unavailable_after_prior_device_loss");
        }
        if (resident.runtime_status != PROM_OK && resident.final_detail_code == VK_ERROR_DEVICE_LOST) {
            row.notes.push_back("resident_explicit_hit_device_lost");
        }
        if (staged.runtime_status != PROM_OK && staged.final_detail_code == VK_ERROR_DEVICE_LOST) {
            row.notes.push_back("staged_explicit_hit_device_lost");
        }
        return row;
    }

    std::string render_failure_matrix_json(const std::vector<ResidentFailureMatrixRow>& rows)
    {
        std::ostringstream out;
        out << "{\n";
        out << "  \"schema\": \"prometheus.sgemm.px16.resident_failure_matrix.v1\",\n";
        out << "  \"timestamp_utc\": \"" << json_escape(timestamp_now_utc()) << "\",\n";
        out << "  \"rows\": [\n";
        for (std::size_t index = 0; index < rows.size(); ++index) {
            const ResidentFailureMatrixRow& row = rows[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"m\": " << row.m
                << ", \"n\": " << row.n
                << ", \"k\": " << row.k
                << ", \"requested_variant\": \"" << json_escape(row.requested_variant)
                << "\", \"staged_requested_variant\": \"" << json_escape(row.staged_requested_variant)
                << "\", \"staged_executed_variant\": \"" << json_escape(row.staged_executed_variant)
                << "\", \"resident_executed_variant\": \"" << json_escape(row.resident_executed_variant)
                << "\", \"production_resident_executed_variant\": \"" << json_escape(row.production_resident_executed_variant)
                << "\", \"staged_path\": \"" << json_escape(row.staged_path)
                << "\", \"resident_failure_stage\": \"" << json_escape(row.resident_failure_stage)
                << "\", \"staged_warmup_result\": \"" << json_escape(row.staged_warmup_result)
                << "\", \"staged_measured_result\": \"" << json_escape(row.staged_measured_result)
                << "\", \"resident_setup_result\": \"" << json_escape(row.resident_setup_result)
                << "\", \"resident_warmup_result\": \"" << json_escape(row.resident_warmup_result)
                << "\", \"resident_measured_result\": \"" << json_escape(row.resident_measured_result)
                << "\", \"vk_result\": \"" << json_escape(row.vk_result)
                << "\", \"staged_runtime_status\": " << row.staged_runtime_status
                << ", \"staged_out_stage\": " << row.staged_out_stage
                << ", \"staged_out_detail_code\": " << row.staged_out_detail_code
                << ", \"resident_runtime_status\": " << row.resident_runtime_status
                << ", \"resident_setup_stage\": " << row.resident_setup_stage
                << ", \"resident_setup_detail_code\": " << row.resident_setup_detail_code
                << ", \"resident_out_stage\": " << row.resident_out_stage
                << ", \"resident_out_detail_code\": " << row.resident_out_detail_code
                << ", \"production_resident_succeeds\": " << bool_json(row.production_resident_succeeds)
                << ", \"resident_mode_available\": " << bool_json(row.resident_mode_available)
                << ", \"selected_pipeline_present\": " << bool_json(row.selected_pipeline_present)
                << ", \"descriptor_update_status\": " << bool_json(row.descriptor_update_status)
                << ", \"timestamp_query_available\": " << bool_json(row.timestamp_query_available)
                << ", \"timestamp_query_valid\": " << bool_json(row.timestamp_query_valid)
                << ", \"dispatch_groups\": {\"x\": " << row.dispatch_groups_x
                << ", \"y\": " << row.dispatch_groups_y
                << ", \"z\": " << row.dispatch_groups_z << "}"
                << ", \"logical_group_coverage\": {\"m\": " << row.logical_m_per_group
                << ", \"n\": " << row.logical_n_per_group << "}"
                << ", \"metadata\": {\"numthreads_x\": " << row.metadata_numthreads_x
                << ", \"numthreads_y\": " << row.metadata_numthreads_y
                << ", \"numthreads_z\": " << row.metadata_numthreads_z
                << ", \"outputs_per_invocation_m\": " << row.metadata_outputs_per_invocation_m
                << ", \"outputs_per_invocation_n\": " << row.metadata_outputs_per_invocation_n
                << ", \"tile_m\": " << row.metadata_tile_m
                << ", \"tile_n\": " << row.metadata_tile_n
                << ", \"tile_k\": " << row.metadata_tile_k
                << ", \"unroll_k\": " << row.metadata_unroll_k << "}"
                << ", \"buffer_elements\": {\"a\": " << row.a_elements
                << ", \"b\": " << row.b_elements
                << ", \"c\": " << row.c_elements << "}"
                << ", \"buffer_bytes\": {\"a\": " << row.a_bytes
                << ", \"b\": " << row.b_bytes
                << ", \"c\": " << row.c_bytes << "}"
                << ", \"descriptor_binding_count\": " << row.descriptor_binding_count
                << ", \"descriptor_bindings\": {\"a\": " << row.descriptor_binding_a
                << ", \"b\": " << row.descriptor_binding_b
                << ", \"c\": " << row.descriptor_binding_c << "}"
                << ", \"shader_module_present\": " << bool_json(row.shader_module_present)
                << ", \"push_constants\": {\"bytes\": " << row.push_constant_bytes
                << ", \"m_offset\": " << row.push_constant_offset_m
                << ", \"n_offset\": " << row.push_constant_offset_n
                << ", \"k_offset\": " << row.push_constant_offset_k << "}"
                << ", \"timestamp_query_slots\": {\"begin\": " << row.timestamp_query_slot_begin
                << ", \"end\": " << row.timestamp_query_slot_end << "}"
                << ", \"resident_iteration_count\": " << row.resident_iteration_count
                << ", \"notes\": [";
            for (std::size_t note_index = 0; note_index < row.notes.size(); ++note_index) {
                if (note_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.notes[note_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < rows.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ]\n";
        out << "}\n";
        return out.str();
    }

    std::string render_failure_matrix_markdown(const std::vector<ResidentFailureMatrixRow>& rows)
    {
        std::ostringstream out;
        out << "# Prometheus SGEMM Px16 Resident Explicit Failure Matrix\n\n";
        out << "## Path Differences\n\n";
        out << "- Production resident setup uses `prometheus_reactor_runtime_sgemm(...)` and keeps selector authority intact.\n";
        out << "- Explicit resident setup uses `prometheus_reactor_runtime_sgemm_benchmark_variant(...)` with `PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_UPLOAD_ONLY`, then re-dispatches through `prom_sgemm_resident_dispatch_once(...)`.\n";
        out << "- The focused matrix uses a fresh runtime per row so one `VK_ERROR_DEVICE_LOST` row does not poison later rows.\n\n";
        out << "## Matrix\n\n";
        out << "| shape | variant | production resident | staged warmup | staged measured | resident setup | resident warmup | resident measured | failure stage | vk_result | staged stage/detail | resident stage/detail | groups | bytes A/B/C |\n";
        out << "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n";
        for (const ResidentFailureMatrixRow& row : rows) {
            out << "| " << row.shape
                << " | " << row.requested_variant
                << " | " << (row.production_resident_succeeds ? "pass" : "fail")
                << " (" << row.production_resident_executed_variant << ")"
                << " | " << row.staged_warmup_result
                << " | " << row.staged_measured_result
                << " | " << row.resident_setup_result
                << " | " << row.resident_warmup_result
                << " | " << row.resident_measured_result
                << " | " << row.resident_failure_stage
                << " | " << row.vk_result
                << " | " << row.staged_out_stage << "/" << row.staged_out_detail_code
                << " | " << row.resident_out_stage << "/" << row.resident_out_detail_code
                << " | " << row.dispatch_groups_x << "x" << row.dispatch_groups_y << "x" << row.dispatch_groups_z
                << " | " << row.a_bytes << "/" << row.b_bytes << "/" << row.c_bytes
                << " |\n";
        }
        out << "\n## ABI and Dispatch Notes\n\n";
        for (const ResidentFailureMatrixRow& row : rows) {
            out << "### " << row.shape << " :: " << row.requested_variant << "\n\n";
            out << "- dispatch groups: `(" << row.dispatch_groups_x << ", " << row.dispatch_groups_y << ", " << row.dispatch_groups_z << ")`\n";
            out << "- logical group coverage: `M=" << row.logical_m_per_group << "`, `N=" << row.logical_n_per_group << "`\n";
            out << "- metadata: `numthreads=(" << row.metadata_numthreads_x << ", " << row.metadata_numthreads_y << ", " << row.metadata_numthreads_z
                << ")`, `outputs=(" << row.metadata_outputs_per_invocation_m << ", " << row.metadata_outputs_per_invocation_n
                << ")`, `tile=(" << row.metadata_tile_m << ", " << row.metadata_tile_n << ", " << row.metadata_tile_k
                << ")`, `unroll_k=" << row.metadata_unroll_k << "`\n";
            out << "- descriptor bindings: `A=" << row.descriptor_binding_a << "`, `B=" << row.descriptor_binding_b
                << "`, `C=" << row.descriptor_binding_c << "`, `count=" << row.descriptor_binding_count << "`\n";
            out << "- push constants: `bytes=" << row.push_constant_bytes << "`, offsets `m/n/k = "
                << row.push_constant_offset_m << "/" << row.push_constant_offset_n << "/" << row.push_constant_offset_k << "`\n";
            out << "- shader module present: `" << (row.shader_module_present ? "yes" : "no")
                << "`, pipeline present: `" << (row.selected_pipeline_present ? "yes" : "no") << "`\n";
            out << "- buffers (bytes): `A=" << row.a_bytes << "`, `B=" << row.b_bytes << "`, `C=" << row.c_bytes << "`\n\n";
        }
        return out.str();
    }

    CaseResult run_evt_case(void* handle, const ShapeCase& shape, bool validate_output)
    {
        CaseResult result;
        result.name = shape.name;
        result.m = shape.m;
        result.n = shape.n;
        result.k = shape.k;
        result.optional = shape.optional;

        if (shape.optional && !should_run_optional_large_case()) {
            result.skipped = true;
            result.skip_reason = "optional_large_case_disabled_for_local_repeatability";
            return result;
        }

        const PreparedCaseData data = prepare_case_data(shape);
        const bool ok = collect_measurement(handle,
                                            shape,
                                            data,
                                            false,
                                            PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
                                            validate_output,
                                            result.timing,
                                            result.timing_decomposition,
                                            result.correctness,
                                            result.diag,
                                            result.final_stage,
                                            result.final_detail_code,
                                            result.runtime_status,
                                            result.anomalies);
        if (ok) {
            detect_case_anomalies(result);
        }
        return result;
    }

    VariantComparisonRow run_explicit_variant_case(void* handle, const ShapeCase& shape, std::uint32_t variant, bool validate_output)
    {
        VariantComparisonRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;
        row.variant = occupancy_variant_name(variant);
        row.requested_variant = occupancy_variant_name(variant);

        if (shape.optional && !should_run_explicit_1024_cube()) {
            row.skipped = true;
            row.skip_reason = "explicit_1024_cube_disabled";
            return row;
        }

        const PreparedCaseData data = prepare_case_data(shape);
        TimingStats timing;
        TimingDecomposition decomposition;
        CorrectnessResult correctness;
        const bool ok = collect_measurement(handle,
                                            shape,
                                            data,
                                            true,
                                            variant,
                                            validate_output,
                                            timing,
                                            decomposition,
                                            correctness,
                                            row.diag,
                                            row.final_stage,
                                            row.final_detail_code,
                                            row.runtime_status,
                                            row.anomalies);
        row.executed_variant = occupancy_variant_name(row.diag.px16_m6_executed_dispatch_variant);
        row.path = path_name(row.diag.px16_m6_executed_path);
        row.correctness = correctness.status;
        row.reference_mode = correctness.reference_mode;
        row.timing_decomposition = decomposition;
        row.median_total_ms = decomposition.total_wall_ms;
        row.median_kernel_ms = decomposition.kernel_gpu_ms;
        row.end_to_end_gflops = decomposition.end_to_end_gflops;
        row.kernel_only_gflops = decomposition.kernel_only_gflops;
        row.gpu_timestamp_valid = decomposition.gpu_timestamp_valid;
        row.timing_source = decomposition.timing_source;

        if (!ok) {
            return row;
        }

        if (validate_output && row.correctness != "pass") {
            row.anomalies.push_back("correctness_failure");
        }
        if (row.diag.px16_m8_last_executed_explicit_variant_request != variant) {
            row.anomalies.push_back("explicit_variant_request_not_recorded");
        }
        if (row.diag.px16_m6_executed_dispatch_variant != variant &&
            row.diag.px16_m6_selected_compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED)) {
            row.anomalies.push_back("explicit_requested_variant_did_not_execute");
        }
        if (!row.gpu_timestamp_valid) {
            row.anomalies.push_back("kernel_timing_unavailable");
        }
        if (row.diag.px16_m6_variant_path_status != static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED) &&
            variant != PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) {
            row.skipped = true;
            row.skip_reason = "variant_not_wired_for_runtime_path";
        }
        return row;
    }

    void postprocess_variant_comparison(ReportData& report)
    {
        for (const ShapeCase& shape : explicit_variant_shapes(report.explicit_cube_1024_enabled)) {
            std::vector<VariantComparisonRow*> rows;
            for (VariantComparisonRow& row : report.variant_comparison) {
                if (row.shape == shape.name) {
                    rows.push_back(&row);
                }
            }
            auto find_variant = [&](std::uint32_t variant) -> VariantComparisonRow* {
                for (VariantComparisonRow* row : rows) {
                    if (row->requested_variant == occupancy_variant_name(variant)) {
                        return row;
                    }
                }
                return nullptr;
            };
            VariantComparisonRow* aggressive = find_variant(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8);
            VariantComparisonRow* balanced = find_variant(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4);
            VariantComparisonRow* srt = find_variant(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE);
            VariantComparisonRow* mc = find_variant(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE);

            if (aggressive != nullptr && balanced != nullptr && aggressive->runtime_status == PROM_OK && balanced->runtime_status == PROM_OK &&
                aggressive->median_total_ms > balanced->median_total_ms * 1.2) {
                aggressive->anomalies.push_back("aggressive_variant_much_slower_than_balanced");
            }
            if (aggressive != nullptr && srt != nullptr && aggressive->runtime_status == PROM_OK && srt->runtime_status == PROM_OK &&
                aggressive->median_total_ms > srt->median_total_ms * 1.2) {
                aggressive->anomalies.push_back("aggressive_variant_much_slower_than_srt");
            }
            if (mc != nullptr && mc->runtime_status == PROM_OK) {
                const double peer_best = std::min(
                    balanced != nullptr && balanced->runtime_status == PROM_OK ? balanced->median_total_ms : std::numeric_limits<double>::infinity(),
                    srt != nullptr && srt->runtime_status == PROM_OK ? srt->median_total_ms : std::numeric_limits<double>::infinity());
                if (std::isfinite(peer_best) && mc->median_total_ms < peer_best * 0.8) {
                    mc->anomalies.push_back("memory_conservative_unexpectedly_wins");
                }
                if (std::isfinite(peer_best) && mc->median_total_ms > peer_best * 1.5) {
                    mc->anomalies.push_back("memory_conservative_unexpectedly_loses");
                }
            }
        }
    }

    void build_selector_vs_fastest(ReportData& report)
    {
        for (const ShapeCase& shape : explicit_variant_shapes(report.explicit_cube_1024_enabled)) {
            const CaseResult* production = nullptr;
            for (const CaseResult& result : report.cases) {
                if (result.name == shape.name) {
                    production = &result;
                    break;
                }
            }
            if (production == nullptr || production->skipped || production->runtime_status != PROM_OK) {
                continue;
            }

            std::vector<const VariantComparisonRow*> valid_rows;
            for (const VariantComparisonRow& row : report.variant_comparison) {
                if (row.shape == shape.name && !row.skipped && row.runtime_status == PROM_OK) {
                    valid_rows.push_back(&row);
                }
            }
            if (valid_rows.empty()) {
                continue;
            }

            const bool can_use_kernel_basis = std::all_of(valid_rows.begin(), valid_rows.end(), [](const VariantComparisonRow* row) {
                return row->gpu_timestamp_valid && row->median_kernel_ms > 0.0;
            }) && production->timing_decomposition.gpu_timestamp_valid && production->timing_decomposition.kernel_gpu_ms > 0.0;

            const VariantComparisonRow* fastest = *std::min_element(
                valid_rows.begin(),
                valid_rows.end(),
                [can_use_kernel_basis](const VariantComparisonRow* left, const VariantComparisonRow* right) {
                    const double left_ms = can_use_kernel_basis ? left->median_kernel_ms : left->median_total_ms;
                    const double right_ms = can_use_kernel_basis ? right->median_kernel_ms : right->median_total_ms;
                    return left_ms < right_ms;
                });

            SelectorVsFastestRow row;
            row.shape = shape.name;
            row.production_variant = occupancy_variant_name(production->diag.px16_m6_selector_selected_variant);
            row.executed_production_variant = occupancy_variant_name(production->diag.px16_m6_executed_dispatch_variant);
            row.fastest_variant = fastest->executed_variant;
            row.comparison_basis = can_use_kernel_basis ? "kernel_gpu_ms" : "total_wall_ms";
            row.production_median_ms = can_use_kernel_basis ? production->timing_decomposition.kernel_gpu_ms
                                                            : production->timing_decomposition.total_wall_ms;
            row.fastest_median_ms = can_use_kernel_basis ? fastest->median_kernel_ms : fastest->median_total_ms;
            row.slowdown_ratio = row.fastest_median_ms > 0.0 ? row.production_median_ms / row.fastest_median_ms : 0.0;
            row.picked_same_variant_as_fastest_explicit = row.executed_production_variant == row.fastest_variant;
            row.production_slower_than_fastest_explicit = row.slowdown_ratio > 1.0;

            if (row.slowdown_ratio > 1.2) {
                row.anomalies.push_back("production_more_than_20_percent_slower_than_fastest");
            }
            if (!row.picked_same_variant_as_fastest_explicit) {
                row.anomalies.push_back("production_variant_differs_from_fastest_explicit_variant");
            }
            report.selector_vs_fastest.push_back(row);
        }
    }

    void postprocess_resident_variant_comparison(ReportData& report)
    {
        for (const ShapeCase& shape : explicit_variant_shapes(report.explicit_cube_1024_enabled)) {
            std::vector<ResidentBenchmarkRow*> rows;
            for (ResidentBenchmarkRow& row : report.resident_variant_comparison) {
                if (row.shape == shape.name && !row.skipped && row.runtime_status == PROM_OK) {
                    rows.push_back(&row);
                }
            }
            if (rows.empty()) {
                continue;
            }
            const bool can_use_kernel_basis = std::all_of(rows.begin(), rows.end(), [](const ResidentBenchmarkRow* row) {
                return row->gpu_timestamp_valid && row->kernel_median_ms > 0.0;
            });
            ResidentBenchmarkRow* fastest = *std::min_element(
                rows.begin(),
                rows.end(),
                [can_use_kernel_basis](const ResidentBenchmarkRow* left, const ResidentBenchmarkRow* right) {
                    const double left_ms = can_use_kernel_basis ? left->kernel_median_ms : left->total_loop_ms;
                    const double right_ms = can_use_kernel_basis ? right->kernel_median_ms : right->total_loop_ms;
                    return left_ms < right_ms;
                });
            if (fastest->executed_variant == occupancy_variant_name(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) && rows.size() > 1u) {
                fastest->anomalies.push_back("baseline_scalar_beats_all_tiled_variants_on_resident_time");
            }
        }
    }

    void build_selector_vs_fastest_resident(ReportData& report)
    {
        for (const ShapeCase& shape : explicit_variant_shapes(report.explicit_cube_1024_enabled)) {
            const ResidentBenchmarkRow* production = nullptr;
            for (const ResidentBenchmarkRow& result : report.resident_production) {
                if (result.shape == shape.name) {
                    production = &result;
                    break;
                }
            }
            if (production == nullptr || production->skipped || production->runtime_status != PROM_OK) {
                continue;
            }

            std::vector<const ResidentBenchmarkRow*> valid_rows;
            for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
                if (row.shape == shape.name && !row.skipped && row.runtime_status == PROM_OK) {
                    valid_rows.push_back(&row);
                }
            }
            if (valid_rows.empty()) {
                continue;
            }

            const bool can_use_kernel_basis = std::all_of(valid_rows.begin(), valid_rows.end(), [](const ResidentBenchmarkRow* row) {
                return row->gpu_timestamp_valid && row->kernel_median_ms > 0.0;
            }) && production->gpu_timestamp_valid && production->kernel_median_ms > 0.0;

            const ResidentBenchmarkRow* fastest = *std::min_element(
                valid_rows.begin(),
                valid_rows.end(),
                [can_use_kernel_basis](const ResidentBenchmarkRow* left, const ResidentBenchmarkRow* right) {
                    const double left_ms = can_use_kernel_basis ? left->kernel_median_ms : left->total_loop_ms;
                    const double right_ms = can_use_kernel_basis ? right->kernel_median_ms : right->total_loop_ms;
                    return left_ms < right_ms;
                });

            SelectorVsFastestRow row;
            row.shape = shape.name;
            row.production_variant = occupancy_variant_name(production->diag.px16_m6_selector_selected_variant);
            row.executed_production_variant = production->executed_variant;
            row.fastest_variant = fastest->executed_variant;
            row.comparison_basis = can_use_kernel_basis ? "resident_kernel_gpu_ms" : "resident_loop_wall_ms";
            row.production_median_ms = can_use_kernel_basis ? production->kernel_median_ms : production->total_loop_ms;
            row.fastest_median_ms = can_use_kernel_basis ? fastest->kernel_median_ms : fastest->total_loop_ms;
            row.slowdown_ratio = row.fastest_median_ms > 0.0 ? row.production_median_ms / row.fastest_median_ms : 0.0;
            row.picked_same_variant_as_fastest_explicit = row.executed_production_variant == row.fastest_variant;
            row.production_slower_than_fastest_explicit = row.slowdown_ratio > 1.0;

            if (row.slowdown_ratio > 1.2) {
                row.anomalies.push_back("resident_production_more_than_20_percent_slower_than_fastest");
            }
            if (!row.picked_same_variant_as_fastest_explicit) {
                row.anomalies.push_back("resident_production_variant_differs_from_fastest_explicit_variant");
            }
            if (fastest->executed_variant == occupancy_variant_name(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) &&
                row.executed_production_variant != fastest->executed_variant) {
                row.anomalies.push_back("memory_conservative_wins_but_selector_does_not_choose_it");
            }
            report.selector_vs_fastest_resident.push_back(row);
        }
    }

    void build_diagnosis(ReportData& report)
    {
        bool transfer_bound = false;
        bool kernel_bound = false;
        bool selector_issue = false;
        bool timing_unavailable = false;
        bool resident_overhead_gap = false;
        std::vector<std::string> large_shape_cases;

        for (const CaseResult& result : report.cases) {
            if (result.skipped || result.runtime_status != PROM_OK) {
                continue;
            }
            if (!result.timing_decomposition.gpu_timestamp_valid) {
                timing_unavailable = true;
            }
            if (result.diag.px16_m6_executed_path == static_cast<std::uint32_t>(PROM_VK_PATH_STAGED_UPLOAD_READBACK) &&
                result.timing_decomposition.upload_ms + result.timing_decomposition.readback_ms + result.timing_decomposition.sync_wait_ms >
                    std::max(result.timing_decomposition.kernel_gpu_ms, 0.0) * 1.2) {
                transfer_bound = true;
            }
            if (result.timing_decomposition.gpu_timestamp_valid &&
                result.timing_decomposition.kernel_only_gflops > 0.0 &&
                result.timing_decomposition.end_to_end_gflops > 0.0 &&
                result.timing_decomposition.kernel_only_gflops < result.timing_decomposition.end_to_end_gflops * 1.3) {
                kernel_bound = true;
            }
            if (is_large_shape({ "", result.m, result.n, result.k, false }) && result.timing.gflops_median < 100.0) {
                large_shape_cases.push_back(result.name);
            }
        }

        for (const SelectorVsFastestRow& row : report.selector_vs_fastest) {
            if (row.slowdown_ratio > 1.2 || !row.picked_same_variant_as_fastest_explicit) {
                selector_issue = true;
            }
        }
        for (const SelectorVsFastestRow& row : report.selector_vs_fastest_resident) {
            if (row.slowdown_ratio > 1.2 || !row.picked_same_variant_as_fastest_explicit) {
                selector_issue = true;
            }
        }
        for (const CaseResult& staged : report.cases) {
            if (staged.skipped || staged.runtime_status != PROM_OK) {
                continue;
            }
            for (const ResidentBenchmarkRow& resident : report.resident_production) {
                if (resident.shape == staged.name &&
                    !resident.skipped &&
                    resident.runtime_status == PROM_OK &&
                    resident.total_loop_ms > 0.0 &&
                    staged.timing_decomposition.benchmark_total_ms > resident.total_loop_ms * 1.5) {
                    resident_overhead_gap = true;
                }
            }
        }

        if (transfer_bound) {
            report.performance_diagnosis.push_back("Likely transfer-bound on staged cases: upload/readback/sync median wall time exceeds kernel time on at least one representative shape.");
        }
        if (!report.resident_device_mode_available) {
            report.performance_diagnosis.push_back("Resident device benchmark mode was unavailable; staged-only numbers remain the only performance surface.");
        }
        if (resident_overhead_gap) {
            report.performance_diagnosis.push_back("Staged production wall time is materially slower than resident loop time on at least one shape, pointing to upload/readback/sync overhead.");
        }
        if (kernel_bound) {
            report.performance_diagnosis.push_back("Kernel-side slowdown is also present on at least one measured shape: kernel-only GFLOP/s remains low even when wall-time decomposition is available.");
        }
        if (selector_issue) {
            report.performance_diagnosis.push_back("Selector issue observed: production executed variant is slower than the fastest correct explicit variant on at least one comparison shape.");
        }
        if (timing_unavailable) {
            report.performance_diagnosis.push_back("Kernel-vs-transfer diagnosis is incomplete for some cases because GPU timestamps were unavailable or invalid and the lane fell back to CPU wall timing.");
        }
        if (!large_shape_cases.empty()) {
            std::string joined;
            for (std::size_t i = 0; i < large_shape_cases.size(); ++i) {
                if (i != 0u) {
                    joined += ", ";
                }
                joined += large_shape_cases[i];
            }
            report.performance_diagnosis.push_back("Large-shape collapse cases observed: " + joined + ".");
        }
        if (report.performance_diagnosis.empty()) {
            report.performance_diagnosis.push_back("No dominant single bottleneck was isolated from the current measurements.");
        }
    }

    std::string render_json(const ReportData& report)
    {
        std::ostringstream out;
        out << "{\n";
        out << "  \"schema\": \"" << json_escape(report.schema) << "\",\n";
        out << "  \"timestamp_utc\": \"" << json_escape(report.timestamp_utc) << "\",\n";
        out << "  \"benchmark_name\": \"" << json_escape(report.benchmark_name) << "\",\n";
        out << "  \"run_mode\": \"" << json_escape(report.run_mode) << "\",\n";
        out << "  \"validation_status_source\": \"" << json_escape(report.validation_status_source) << "\",\n";
        out << "  \"device\": {\n";
        out << "    \"name\": \"" << json_escape(report.device.name) << "\",\n";
        out << "    \"backend\": \"" << json_escape(report.device.backend) << "\",\n";
        out << "    \"device_type\": \"" << json_escape(report.device.device_type) << "\",\n";
        out << "    \"vendor_id\": " << report.device.vendor_id << ",\n";
        out << "    \"device_id\": " << report.device.device_id << ",\n";
        out << "    \"driver_version\": \"" << json_escape(report.device.driver_version) << "\",\n";
        out << "    \"api_version\": \"" << json_escape(report.device.api_version) << "\",\n";
        out << "    \"max_compute_workgroup_invocations\": " << report.device.max_compute_workgroup_invocations << ",\n";
        out << "    \"max_compute_shared_memory_size\": " << report.device.max_compute_shared_memory_size << ",\n";
        out << "    \"subgroup_size\": " << report.device.subgroup_size << "\n";
        out << "  },\n";
        out << "  \"resident_device_mode_available\": " << bool_json(report.resident_device_mode_available) << ",\n";
        out << "  \"summary\": {\n";
        out << "    \"cases_total\": " << report.summary.cases_total << ",\n";
        out << "    \"cases_passed\": " << report.summary.cases_passed << ",\n";
        out << "    \"cases_failed\": " << report.summary.cases_failed << ",\n";
        out << "    \"cases_skipped\": " << report.summary.cases_skipped << ",\n";
        out << "    \"best_gflops\": " << report.summary.best_gflops << ",\n";
        out << "    \"median_gflops\": " << report.summary.median_gflops << ",\n";
        out << "    \"anomaly_count\": " << report.summary.anomaly_count << "\n";
        out << "  },\n";
        out << "  \"global_skip_reason\": \"" << json_escape(report.global_skip_reason) << "\",\n";
        out << "  \"cases\": [\n";
        for (std::size_t index = 0u; index < report.cases.size(); ++index) {
            const CaseResult& result = report.cases[index];
            out << "    {\n";
            out << "      \"name\": \"" << json_escape(result.name) << "\",\n";
            out << "      \"m\": " << result.m << ",\n";
            out << "      \"n\": " << result.n << ",\n";
            out << "      \"k\": " << result.k << ",\n";
            out << "      \"optional\": " << bool_json(result.optional) << ",\n";
            out << "      \"skipped\": " << bool_json(result.skipped) << ",\n";
            out << "      \"skip_reason\": \"" << json_escape(result.skip_reason) << "\",\n";
            out << "      \"policy_mode\": \"" << policy_mode_name(result.diag.px16_m6_policy_mode) << "\",\n";
            out << "      \"path\": {\"requested\": \"" << path_name(result.diag.px16_m6_requested_path)
                << "\", \"selected\": \"" << path_name(result.diag.px16_m6_selected_path)
                << "\", \"executed\": \"" << path_name(result.diag.px16_m6_executed_path) << "\"},\n";
            out << "      \"compute_mode\": {\"requested\": \"" << compute_mode_name(result.diag.px16_m6_requested_compute_mode)
                << "\", \"selected\": \"" << compute_mode_name(result.diag.px16_m6_selected_compute_mode)
                << "\", \"executed\": \"" << compute_mode_name(result.diag.px16_m6_executed_compute_mode) << "\"},\n";
            out << "      \"variant\": {\"recommended\": \"" << occupancy_variant_name(result.diag.px16_m6_selector_recommended_variant)
                << "\", \"selected\": \"" << occupancy_variant_name(result.diag.px16_m6_selector_selected_variant)
                << "\", \"requested\": \"" << occupancy_variant_name(result.diag.px16_m6_requested_dispatch_variant)
                << "\", \"executed\": \"" << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant)
                << "\", \"path_status\": \"" << variant_path_status_name(result.diag.px16_m6_variant_path_status)
                << "\", \"production_eligible\": " << bool_json(result.diag.px16_m6_variant_production_eligible != 0u)
                << ", \"dispatch_enabled\": " << bool_json(result.diag.px16_m6_variant_dispatch_enabled != 0u) << "},\n";
            out << "      \"force_direct\": {\"requested\": " << bool_json(result.diag.px16_m6_force_direct_requested != 0u)
                << ", \"applied\": " << bool_json(result.diag.px16_m6_force_direct_applied != 0u)
                << ", \"reason\": \"" << force_direct_reason_name(result.diag.px16_m6_force_direct_reason) << "\"},\n";
            out << "      \"p15\": {\"reservation_present\": " << bool_json(result.diag.px16_m6_p15_reservation_present != 0u)
                << ", \"reservation_matured\": " << bool_json(result.diag.px16_m6_p15_reservation_matured != 0u)
                << ", \"reservation_consumed\": " << bool_json(result.diag.px16_m6_p15_reservation_consumed != 0u)
                << ", \"reserved_variant\": \"" << occupancy_variant_name(result.diag.px16_m6_p15_reserved_variant_id)
                << "\", \"live_selected_variant\": \"" << occupancy_variant_name(result.diag.px16_m6_p15_live_selected_variant_id)
                << "\", \"match\": " << bool_json(result.diag.px16_m6_p15_reconciliation_match != 0u)
                << ", \"block_reason\": \"" << p15_block_reason_name(result.diag.px16_m6_p15_block_reason)
                << "\", \"correction_action\": \"" << p15_correction_action_name(result.diag.px16_m6_p15_correction_action) << "\"},\n";
            out << "      \"correctness\": {\"status\": \"" << json_escape(result.correctness.status)
                << "\", \"reference_mode\": \"" << json_escape(result.correctness.reference_mode)
                << "\", \"source\": \"" << json_escape(result.correctness.source)
                << "\", \"max_abs_error\": " << result.correctness.max_abs_error
                << ", \"max_rel_error\": " << result.correctness.max_rel_error
                << ", \"tolerance\": " << result.correctness.tolerance << "},\n";
            out << "      \"timing\": {\"warmup_iterations\": " << result.timing.warmup_iterations
                << ", \"iterations\": " << result.timing.iterations
                << ", \"min_ms\": " << result.timing.min_ms
                << ", \"median_ms\": " << result.timing.median_ms
                << ", \"p95_ms\": " << result.timing.p95_ms
                << ", \"gflops_median\": " << result.timing.gflops_median
                << ", \"timing_source\": \"" << json_escape(result.timing.timing_source)
                << "\", \"gpu_timing_failure_reason\": \"" << timing_failure_reason_name(result.diag.p13_m5_last_gpu_timing_failure_reason) << "\"},\n";
            out << "      \"timing_decomposition\": {\"upload_ms\": " << result.timing_decomposition.upload_ms
                << ", \"pre_dispatch_ms\": " << result.timing_decomposition.pre_dispatch_ms
                << ", \"command_record_ms\": " << result.timing_decomposition.command_record_ms
                << ", \"dispatch_submit_ms\": " << result.timing_decomposition.dispatch_submit_ms
                << ", \"kernel_gpu_ms\": " << result.timing_decomposition.kernel_gpu_ms
                << ", \"readback_ms\": " << result.timing_decomposition.readback_ms
                << ", \"sync_wait_ms\": " << result.timing_decomposition.sync_wait_ms
                << ", \"post_sync_ms\": " << result.timing_decomposition.post_sync_ms
                << ", \"post_readback_ms\": " << result.timing_decomposition.post_readback_ms
                << ", \"total_wall_ms\": " << result.timing_decomposition.total_wall_ms
                << ", \"benchmark_total_ms\": " << result.timing_decomposition.benchmark_total_ms
                << ", \"oracle_ms\": " << result.timing_decomposition.oracle_ms
                << ", \"validation_readback_ms\": " << result.timing_decomposition.validation_readback_ms
                << ", \"validation_ms\": " << result.timing_decomposition.validation_ms
                << ", \"unaccounted_host_ms\": " << result.timing_decomposition.unaccounted_host_ms
                << ", \"tolerance_eval_ms\": " << result.timing_decomposition.tolerance_eval_ms
                << ", \"tolerance_eval_in_dispatch\": " << bool_json(result.diag.px16_m17_last_tolerance_eval_in_dispatch != 0u)
                << ", \"tolerance_eval_source\": " << result.diag.px16_m17_last_tolerance_eval_source
                << ", \"kernel_only_gflops\": " << result.timing_decomposition.kernel_only_gflops
                << ", \"end_to_end_gflops\": " << result.timing_decomposition.end_to_end_gflops
                << ", \"gpu_timestamp_valid\": " << bool_json(result.timing_decomposition.gpu_timestamp_valid)
                << ", \"timing_source\": \"" << json_escape(result.timing_decomposition.timing_source) << "\"},\n";
            out << "      \"runtime\": {\"status\": " << result.runtime_status
                << ", \"final_stage\": " << result.final_stage
                << ", \"final_detail_code\": " << result.final_detail_code << "},\n";
            out << "      \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < result.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(result.anomalies[anomaly_index]) << "\"";
            }
            out << "]\n";
            out << "    }";
            if (index + 1u < report.cases.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"resident_production\": [\n";
        for (std::size_t index = 0u; index < report.resident_production.size(); ++index) {
            const ResidentBenchmarkRow& row = report.resident_production[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"mode\": \"" << json_escape(row.mode)
                << "\", \"variant\": \"" << json_escape(row.variant)
                << "\", \"executed_variant\": \"" << json_escape(row.executed_variant)
                << "\", \"resident_mode_available\": " << bool_json(row.resident_mode_available)
                << ", \"resident_mode_used\": " << bool_json(row.resident_mode_used)
                << ", \"resident_upload_once_ms\": " << row.upload_once_ms
                << ", \"resident_setup_ms\": " << row.setup_ms
                << ", \"resident_kernel_median_ms\": " << row.kernel_median_ms
                << ", \"resident_kernel_min_ms\": " << row.kernel_min_ms
                << ", \"resident_kernel_p95_ms\": " << row.kernel_p95_ms
                << ", \"resident_total_loop_ms\": " << row.total_loop_ms
                << ", \"resident_iterations\": " << row.iterations
                << ", \"resident_readback_once_ms\": " << row.readback_once_ms
                << ", \"resident_validation_ms\": " << row.validation_ms
                << ", \"resident_kernel_only_gflops\": " << row.kernel_only_gflops
                << ", \"resident_loop_gflops\": " << row.loop_gflops
                << ", \"gpu_timestamp_valid\": " << bool_json(row.gpu_timestamp_valid)
                << ", \"gpu_timing_failure_reason\": \"" << json_escape(row.gpu_timing_failure_reason)
                << "\", \"correctness\": \"" << json_escape(row.correctness)
                << "\", \"reference_mode\": \"" << json_escape(row.reference_mode)
                << "\", \"runtime_status\": " << row.runtime_status
                << ", \"final_stage\": " << row.final_stage
                << ", \"final_detail_code\": " << row.final_detail_code
                << ", \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < row.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.anomalies[anomaly_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < report.resident_production.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"resident_variant_comparison\": [\n";
        for (std::size_t index = 0u; index < report.resident_variant_comparison.size(); ++index) {
            const ResidentBenchmarkRow& row = report.resident_variant_comparison[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"m\": " << row.m
                << ", \"n\": " << row.n
                << ", \"k\": " << row.k
                << ", \"mode\": \"" << json_escape(row.mode)
                << "\", \"variant\": \"" << json_escape(row.variant)
                << "\", \"requested_variant\": \"" << json_escape(row.requested_variant)
                << "\", \"executed_variant\": \"" << json_escape(row.executed_variant)
                << "\", \"selector_recommended_variant\": \"" << occupancy_variant_name(row.diag.px16_m6_selector_recommended_variant)
                << "\", \"selector_selected_variant\": \"" << occupancy_variant_name(row.diag.px16_m6_selector_selected_variant)
                << "\", \"skipped\": " << bool_json(row.skipped)
                << ", \"skip_reason\": \"" << json_escape(row.skip_reason)
                << "\", \"resident_mode_available\": " << bool_json(row.resident_mode_available)
                << ", \"resident_mode_used\": " << bool_json(row.resident_mode_used)
                << ", \"resident_upload_once_ms\": " << row.upload_once_ms
                << ", \"resident_setup_ms\": " << row.setup_ms
                << ", \"resident_kernel_median_ms\": " << row.kernel_median_ms
                << ", \"resident_kernel_min_ms\": " << row.kernel_min_ms
                << ", \"resident_kernel_p95_ms\": " << row.kernel_p95_ms
                << ", \"resident_total_loop_ms\": " << row.total_loop_ms
                << ", \"resident_iterations\": " << row.iterations
                << ", \"resident_readback_once_ms\": " << row.readback_once_ms
                << ", \"resident_validation_ms\": " << row.validation_ms
                << ", \"resident_kernel_only_gflops\": " << row.kernel_only_gflops
                << ", \"resident_loop_gflops\": " << row.loop_gflops
                << ", \"gpu_timestamp_valid\": " << bool_json(row.gpu_timestamp_valid)
                << ", \"gpu_timing_failure_reason\": \"" << json_escape(row.gpu_timing_failure_reason)
                << "\", \"correctness\": \"" << json_escape(row.correctness)
                << "\", \"reference_mode\": \"" << json_escape(row.reference_mode)
                << "\", \"runtime_status\": " << row.runtime_status
                << ", \"final_stage\": " << row.final_stage
                << ", \"final_detail_code\": " << row.final_detail_code
                << ", \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < row.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.anomalies[anomaly_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < report.resident_variant_comparison.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"variant_comparison\": [\n";
        for (std::size_t index = 0u; index < report.variant_comparison.size(); ++index) {
            const VariantComparisonRow& row = report.variant_comparison[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"m\": " << row.m
                << ", \"n\": " << row.n
                << ", \"k\": " << row.k
                << ", \"variant\": \"" << json_escape(row.variant)
                << "\", \"requested_variant\": \"" << json_escape(row.requested_variant)
                << "\", \"executed_variant\": \"" << json_escape(row.executed_variant)
                << "\", \"selector_recommended_variant\": \"" << occupancy_variant_name(row.diag.px16_m6_selector_recommended_variant)
                << "\", \"selector_selected_variant\": \"" << occupancy_variant_name(row.diag.px16_m6_selector_selected_variant)
                << "\", \"path\": \"" << json_escape(row.path)
                << "\", \"correctness\": \"" << json_escape(row.correctness)
                << "\", \"reference_mode\": \"" << json_escape(row.reference_mode)
                << "\", \"skipped\": " << bool_json(row.skipped)
                << ", \"skip_reason\": \"" << json_escape(row.skip_reason)
                << "\", \"median_total_ms\": " << row.median_total_ms
                << ", \"median_kernel_ms\": " << row.median_kernel_ms
                << ", \"end_to_end_gflops\": " << row.end_to_end_gflops
                << ", \"kernel_only_gflops\": " << row.kernel_only_gflops
                << ", \"gpu_timestamp_valid\": " << bool_json(row.gpu_timestamp_valid)
                << ", \"timing_source\": \"" << json_escape(row.timing_source)
                << "\", \"timing_decomposition\": {\"upload_ms\": " << row.timing_decomposition.upload_ms
                << ", \"pre_dispatch_ms\": " << row.timing_decomposition.pre_dispatch_ms
                << ", \"command_record_ms\": " << row.timing_decomposition.command_record_ms
                << ", \"dispatch_submit_ms\": " << row.timing_decomposition.dispatch_submit_ms
                << ", \"kernel_gpu_ms\": " << row.timing_decomposition.kernel_gpu_ms
                << ", \"readback_ms\": " << row.timing_decomposition.readback_ms
                << ", \"sync_wait_ms\": " << row.timing_decomposition.sync_wait_ms
                << ", \"post_sync_ms\": " << row.timing_decomposition.post_sync_ms
                << ", \"post_readback_ms\": " << row.timing_decomposition.post_readback_ms
                << ", \"total_wall_ms\": " << row.timing_decomposition.total_wall_ms
                << ", \"benchmark_total_ms\": " << row.timing_decomposition.benchmark_total_ms
                << ", \"oracle_ms\": " << row.timing_decomposition.oracle_ms
                << ", \"validation_readback_ms\": " << row.timing_decomposition.validation_readback_ms
                << ", \"validation_ms\": " << row.timing_decomposition.validation_ms
                << ", \"unaccounted_host_ms\": " << row.timing_decomposition.unaccounted_host_ms
                << ", \"tolerance_eval_ms\": " << row.timing_decomposition.tolerance_eval_ms
                << ", \"kernel_only_gflops\": " << row.timing_decomposition.kernel_only_gflops
                << ", \"end_to_end_gflops\": " << row.timing_decomposition.end_to_end_gflops
                << ", \"gpu_timestamp_valid\": " << bool_json(row.timing_decomposition.gpu_timestamp_valid)
                << ", \"timing_source\": \"" << json_escape(row.timing_decomposition.timing_source) << "\"}"
                << ", \"runtime_status\": " << row.runtime_status
                << ", \"final_stage\": " << row.final_stage
                << ", \"final_detail_code\": " << row.final_detail_code
                << ", \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < row.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.anomalies[anomaly_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < report.variant_comparison.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"selector_vs_fastest\": [\n";
        for (std::size_t index = 0u; index < report.selector_vs_fastest.size(); ++index) {
            const SelectorVsFastestRow& row = report.selector_vs_fastest[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"production_variant\": \"" << json_escape(row.production_variant)
                << "\", \"executed_production_variant\": \"" << json_escape(row.executed_production_variant)
                << "\", \"fastest_variant\": \"" << json_escape(row.fastest_variant)
                << "\", \"comparison_basis\": \"" << json_escape(row.comparison_basis)
                << "\", \"production_median_ms\": " << row.production_median_ms
                << ", \"fastest_median_ms\": " << row.fastest_median_ms
                << ", \"production_vs_fastest_ratio\": " << row.slowdown_ratio
                << ", \"picked_same_variant_as_fastest_explicit\": " << bool_json(row.picked_same_variant_as_fastest_explicit)
                << ", \"production_slower_than_fastest_explicit\": " << bool_json(row.production_slower_than_fastest_explicit)
                << ", \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < row.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.anomalies[anomaly_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < report.selector_vs_fastest.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"selector_vs_fastest_resident\": [\n";
        for (std::size_t index = 0u; index < report.selector_vs_fastest_resident.size(); ++index) {
            const SelectorVsFastestRow& row = report.selector_vs_fastest_resident[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"production_resident_variant\": \"" << json_escape(row.executed_production_variant)
                << "\", \"fastest_resident_variant\": \"" << json_escape(row.fastest_variant)
                << "\", \"comparison_basis\": \"" << json_escape(row.comparison_basis)
                << "\", \"production_resident_kernel_ms\": " << row.production_median_ms
                << ", \"fastest_resident_kernel_ms\": " << row.fastest_median_ms
                << ", \"slowdown_ratio\": " << row.slowdown_ratio
                << ", \"picked_same_variant\": " << bool_json(row.picked_same_variant_as_fastest_explicit)
                << ", \"anomalies\": [";
            for (std::size_t anomaly_index = 0u; anomaly_index < row.anomalies.size(); ++anomaly_index) {
                if (anomaly_index != 0u) {
                    out << ", ";
                }
                out << "\"" << json_escape(row.anomalies[anomaly_index]) << "\"";
            }
            out << "]}";
            if (index + 1u < report.selector_vs_fastest_resident.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"performance_diagnosis\": [";
        for (std::size_t index = 0u; index < report.performance_diagnosis.size(); ++index) {
            if (index != 0u) {
                out << ", ";
            }
            out << "\"" << json_escape(report.performance_diagnosis[index]) << "\"";
        }
        out << "]\n";
        out << "}\n";
        return out.str();
    }

    std::string anomalies_or_none(const std::vector<std::string>& anomalies)
    {
        if (anomalies.empty()) {
            return "none";
        }
        std::string out = anomalies.front();
        for (std::size_t i = 1u; i < anomalies.size(); ++i) {
            out += ", " + anomalies[i];
        }
        return out;
    }

    std::string render_markdown(const ReportData& report)
    {
        std::ostringstream out;
        out << "# Prometheus SGEMM Px16 EVT Report\n\n";
        out << "## Device / Runtime\n\n";
        out << "- Device: " << report.device.name << "\n";
        out << "- Backend: " << report.device.backend << "\n";
        out << "- Device Type: " << report.device.device_type << "\n";
        out << "- Vendor ID: " << report.device.vendor_id << "\n";
        out << "- Device ID: " << report.device.device_id << "\n";
        out << "- Driver Version: " << report.device.driver_version << "\n";
        out << "- API Version: " << report.device.api_version << "\n";
        out << "- Max Compute Workgroup Invocations: " << report.device.max_compute_workgroup_invocations << "\n";
        out << "- Max Compute Shared Memory Size: " << report.device.max_compute_shared_memory_size << "\n";
        out << "- Subgroup Size: " << report.device.subgroup_size << "\n";
        out << "- Resident Device Mode Available: " << (report.resident_device_mode_available ? "true" : "false") << "\n";
        out << "- Timestamp: " << report.timestamp_utc << "\n";
        out << "- Benchmark: " << report.benchmark_name << "\n\n";
        out << "- Run Mode: " << report.run_mode << "\n";
        out << "- Validation Status Source: " << report.validation_status_source << "\n\n";

        out << "## Summary\n\n";
        out << "- Cases total: " << report.summary.cases_total << "\n";
        out << "- Passed: " << report.summary.cases_passed << "\n";
        out << "- Failed: " << report.summary.cases_failed << "\n";
        out << "- Skipped: " << report.summary.cases_skipped << "\n";
        out << "- Best GFLOP/s: " << report.summary.best_gflops << "\n";
        out << "- Median GFLOP/s: " << report.summary.median_gflops << "\n";
        out << "- Anomaly count: " << report.summary.anomaly_count << "\n";
        if (!report.global_skip_reason.empty()) {
            out << "- Global skip reason: " << report.global_skip_reason << "\n";
        }
        out << "\n";

        out << "## Performance Benchmark\n\n";
        out << "This table measures the production GPU SGEMM operation. CPU oracle/reference work is not part of the default benchmark mode.\n\n";
        out << "| shape | policy | path | selected variant | executed variant | kernel ms | gpu operation ms | GFLOP/s | timing source | validation status source | anomalies |\n";
        out << "| --- | --- | --- | --- | --- | ---: | ---: | ---: | --- | --- | --- |\n";
        for (const CaseResult& result : report.cases) {
            const std::string path_summary =
                path_name(result.diag.px16_m6_requested_path) + " -> " +
                path_name(result.diag.px16_m6_selected_path) + " -> " +
                path_name(result.diag.px16_m6_executed_path);
            const std::string correctness_summary = result.skipped
                ? "skip"
                : result.correctness.status + " (" + result.correctness.source + ")";
            std::string anomaly_summary = anomalies_or_none(result.anomalies);
            if (result.skipped && !result.skip_reason.empty()) {
                anomaly_summary = "skip: " + result.skip_reason;
            }
            out << "| " << result.name
                << " | " << policy_mode_name(result.diag.px16_m6_policy_mode)
                << " | " << path_summary
                << " | " << occupancy_variant_name(result.diag.px16_m6_selector_selected_variant)
                << " | " << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant)
                << " | " << result.timing_decomposition.kernel_gpu_ms
                << " | " << result.timing_decomposition.benchmark_total_ms
                << " | " << result.timing.gflops_median
                << " | " << result.timing.timing_source
                << " | " << correctness_summary
                << " | " << anomaly_summary << " |\n";
        }
        out << "\n";

        out << "## Timing Decomposition\n\n";
        out << "| shape | path | variant | benchmark total ms | kernel ms | upload ms | pre-dispatch ms | command record ms | dispatch submit ms | sync wait ms | post-sync ms | readback ms | post-readback ms | unaccounted host ms | tolerance eval ms | tolerance eval in dispatch | end-to-end GFLOP/s | kernel GFLOP/s | timing source |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | ---: | ---: | --- |\n";
        for (const CaseResult& result : report.cases) {
            out << "| " << result.name
                << " | " << path_name(result.diag.px16_m6_executed_path)
                << " | " << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant)
                << " | " << result.timing_decomposition.benchmark_total_ms
                << " | " << result.timing_decomposition.kernel_gpu_ms
                << " | " << result.timing_decomposition.upload_ms
                << " | " << result.timing_decomposition.pre_dispatch_ms
                << " | " << result.timing_decomposition.command_record_ms
                << " | " << result.timing_decomposition.dispatch_submit_ms
                << " | " << result.timing_decomposition.sync_wait_ms
                << " | " << result.timing_decomposition.post_sync_ms
                << " | " << result.timing_decomposition.readback_ms
                << " | " << result.timing_decomposition.post_readback_ms
                << " | " << result.timing_decomposition.unaccounted_host_ms
                << " | " << result.timing_decomposition.tolerance_eval_ms
                << " | " << (result.diag.px16_m17_last_tolerance_eval_in_dispatch != 0u ? "true" : "false")
                << " | " << result.timing_decomposition.end_to_end_gflops
                << " | " << result.timing_decomposition.kernel_only_gflops
                << " | " << result.timing_decomposition.timing_source << " |\n";
        }
        out << "\n";

        out << "## Resident Device Benchmark\n\n";
        out << "| shape | mode | variant | iterations | resident kernel ms | resident loop ms | kernel GFLOP/s | loop GFLOP/s | staged kernel ms | staged total ms | staged/resident ratio | correctness/validation source |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n";
        for (const ResidentBenchmarkRow& row : report.resident_production) {
            const CaseResult* staged = nullptr;
            for (const CaseResult& result : report.cases) {
                if (result.name == row.shape) {
                    staged = &result;
                    break;
                }
            }
            const double staged_kernel_ms = staged != nullptr ? staged->timing_decomposition.kernel_gpu_ms : 0.0;
            const double staged_total_ms = staged != nullptr ? staged->timing_decomposition.benchmark_total_ms : 0.0;
            const double ratio = row.total_loop_ms > 0.0 ? staged_total_ms / row.total_loop_ms : 0.0;
            const std::string correctness_summary = row.skipped ? "skip" : row.correctness + " (" + row.reference_mode + ")";
            out << "| " << row.shape
                << " | " << row.mode
                << " | " << row.executed_variant
                << " | " << row.iterations
                << " | " << row.kernel_median_ms
                << " | " << row.total_loop_ms
                << " | " << row.kernel_only_gflops
                << " | " << row.loop_gflops
                << " | " << staged_kernel_ms
                << " | " << staged_total_ms
                << " | " << ratio
                << " | " << correctness_summary << " |\n";
        }
        out << "\n";

        out << "## Resident Explicit Variant Comparison\n\n";
        out << "| shape | variant | executed | resident kernel ms | resident GFLOP/s | correctness source | anomalies |\n";
        out << "| --- | --- | --- | ---: | ---: | --- | --- |\n";
        for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
            const std::string correctness_summary = row.skipped ? "skip" : row.correctness + " (" + row.reference_mode + ")";
            const std::string anomaly_summary = row.skipped && !row.skip_reason.empty()
                ? "skip: " + row.skip_reason
                : anomalies_or_none(row.anomalies);
            out << "| " << row.shape
                << " | " << row.variant
                << " | " << row.executed_variant
                << " | " << row.kernel_median_ms
                << " | " << row.kernel_only_gflops
                << " | " << correctness_summary
                << " | " << anomaly_summary << " |\n";
        }
        out << "\n";

        out << "## Oracle / Validation Cost\n\n";
        out << "| shape | validation status | reference mode | oracle ms | validation readback ms | validation ms |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: |\n";
        for (const CaseResult& result : report.cases) {
            out << "| " << result.name
                << " | " << result.correctness.status
                << " | " << result.correctness.reference_mode
                << " | " << result.timing_decomposition.oracle_ms
                << " | " << result.timing_decomposition.validation_readback_ms
                << " | " << result.timing_decomposition.validation_ms << " |\n";
        }
        out << "\n";

        out << "## Explicit Variant Comparison\n\n";
        out << "| shape | variant | executed | median total ms | median kernel ms | e2e GFLOP/s | kernel GFLOP/s | correctness | anomalies |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: | ---: | --- | --- |\n";
        for (const VariantComparisonRow& row : report.variant_comparison) {
            const std::string correctness_summary = row.skipped ? "skip" : row.correctness + " (" + row.reference_mode + ")";
            const std::string anomaly_summary = row.skipped && !row.skip_reason.empty()
                ? "skip: " + row.skip_reason
                : anomalies_or_none(row.anomalies);
            out << "| " << row.shape
                << " | " << row.variant
                << " | " << row.executed_variant
                << " | " << row.median_total_ms
                << " | " << row.median_kernel_ms
                << " | " << row.end_to_end_gflops
                << " | " << row.kernel_only_gflops
                << " | " << correctness_summary
                << " | " << anomaly_summary << " |\n";
        }
        out << "\n";

        out << "## Selector vs Fastest Resident Variant\n\n";
        out << "| shape | production resident variant | fastest resident variant | production resident kernel ms | fastest resident kernel ms | slowdown ratio | picked same variant? |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: | --- |\n";
        for (const SelectorVsFastestRow& row : report.selector_vs_fastest_resident) {
            out << "| " << row.shape
                << " | " << row.executed_production_variant
                << " | " << row.fastest_variant
                << " | " << row.production_median_ms
                << " | " << row.fastest_median_ms
                << " | " << row.slowdown_ratio
                << " | " << (row.picked_same_variant_as_fastest_explicit ? "yes" : "no") << " |\n";
        }
        out << "\n";

        out << "## Selector vs Fastest Variant\n\n";
        out << "| shape | production variant | fastest explicit variant | production ms | fastest ms | production vs fastest ratio | picked same variant as fastest explicit | production slower than fastest explicit |\n";
        out << "| --- | --- | --- | ---: | ---: | ---: | --- | --- |\n";
        for (const SelectorVsFastestRow& row : report.selector_vs_fastest) {
            out << "| " << row.shape
                << " | " << row.executed_production_variant
                << " | " << row.fastest_variant
                << " | " << row.production_median_ms
                << " | " << row.fastest_median_ms
                << " | " << row.slowdown_ratio
                << " | " << (row.picked_same_variant_as_fastest_explicit ? "yes" : "no")
                << " | " << (row.production_slower_than_fastest_explicit ? "yes" : "no") << " |\n";
        }
        out << "\n";

        out << "## Performance Diagnosis\n\n";
        for (const std::string& diagnosis : report.performance_diagnosis) {
            out << "- " << diagnosis << "\n";
        }
        out << "\n";

        out << "## Notes\n\n";
        out << "- Main production results still use `prometheus_reactor_runtime_sgemm(...)`.\n";
        out << "- Unit/FACT tests validate correctness; benchmarks measure performance.\n";
        out << "- Default performance benchmark mode reports `validation_status=not_run_in_benchmark_mode`; use the correctness lane for CPU oracle validation.\n";
        out << "- Explicit variant comparison uses `prometheus_reactor_runtime_sgemm_benchmark_variant(...)` and is labeled as comparison-only telemetry, not production selector behavior.\n";
        out << "- Explicit comparison rows now use fresh runtime handles per row so a single `VK_ERROR_DEVICE_LOST` result does not cascade into unrelated variant rows.\n";
        out << "- `kernel_only_gflops` uses Vulkan timestamp kernel time only when every measured iteration produced a valid GPU timestamp. Otherwise the report calls kernel timing unavailable and falls back to end-to-end wall timing.\n";
        out << "- `STAGED_UPLOAD_READBACK` rows should be read using the decomposition table: upload/readback/sync buckets are host-observed wall slices, while `kernel_gpu_ms` is the Vulkan timestamp duration when valid.\n";
        out << "- Resident device benchmark mode is benchmark/diagnostic-only: it uploads A/B once, keeps A/B/C device-resident across timed iterations, and reads C back only when validation is explicitly requested.\n";
        out << "- Resident timing currently uses one submit/wait per timed dispatch because the backend timestamp query pool exposes one start/end pair; it still avoids per-iteration upload/readback.\n";
        out << "- Generated artifacts belong under `out/test-artifacts/` and should not be committed by default.\n";
        return out.str();
    }

    ReportData build_synthetic_report()
    {
        ReportData report;
        report.timestamp_utc = "2026-07-08T00:00:00Z";
        report.benchmark_name = "synthetic";
        report.device.name = "Synthetic GPU";
        report.device.backend = "VULKAN";
        report.device.device_type = "DISCRETE_GPU";
        report.device.vendor_id = 4318u;
        report.device.device_id = 9152u;
        report.device.driver_version = "1.2.3";
        report.device.api_version = "1.3.0";
        report.device.max_compute_workgroup_invocations = 1024u;
        report.device.max_compute_shared_memory_size = 49152u;
        report.resident_device_mode_available = true;

        CaseResult result;
        result.name = "square_512x512x512";
        result.m = 512u;
        result.n = 512u;
        result.k = 512u;
        result.correctness.status = "pass";
        result.correctness.reference_mode = "dense_cpu_oracle";
        result.timing.warmup_iterations = 1u;
        result.timing.iterations = 3u;
        result.timing.min_ms = 0.9;
        result.timing.median_ms = 1.0;
        result.timing.p95_ms = 1.2;
        result.timing.gflops_median = 250.0;
        result.timing.timing_source = "vulkan_timestamp_query";
        result.timing_decomposition.total_wall_ms = 1.0;
        result.timing_decomposition.benchmark_total_ms = 1.0;
        result.timing_decomposition.kernel_gpu_ms = 0.8;
        result.timing_decomposition.upload_ms = 0.05;
        result.timing_decomposition.pre_dispatch_ms = 0.01;
        result.timing_decomposition.command_record_ms = 0.03;
        result.timing_decomposition.readback_ms = 0.05;
        result.timing_decomposition.sync_wait_ms = 0.08;
        result.timing_decomposition.post_sync_ms = 0.01;
        result.timing_decomposition.post_readback_ms = 0.01;
        result.timing_decomposition.dispatch_submit_ms = 0.02;
        result.timing_decomposition.unaccounted_host_ms =
            result.timing_decomposition.total_wall_ms - accounted_host_ms(result.timing_decomposition);
        result.timing_decomposition.tolerance_eval_ms = 0.0;
        result.timing_decomposition.end_to_end_gflops = 250.0;
        result.timing_decomposition.kernel_only_gflops = 312.5;
        result.timing_decomposition.gpu_timestamp_valid = true;
        result.timing_decomposition.timing_source = "mixed";
        result.diag.px16_m6_policy_mode = PROM_POLICY_MODE_SAFE;
        result.diag.px16_m6_selector_recommended_variant = PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
        result.diag.px16_m6_selector_selected_variant = PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
        result.diag.px16_m6_requested_dispatch_variant = PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
        result.diag.px16_m6_executed_dispatch_variant = PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
        result.diag.px16_m6_requested_path = PROM_VK_PATH_STAGED_UPLOAD_READBACK;
        result.diag.px16_m6_selected_path = PROM_VK_PATH_STAGED_UPLOAD_READBACK;
        result.diag.px16_m6_executed_path = PROM_VK_PATH_STAGED_UPLOAD_READBACK;
        result.diag.px16_m6_requested_compute_mode = PROM_VK_COMPUTE_TILED;
        result.diag.px16_m6_selected_compute_mode = PROM_VK_COMPUTE_TILED;
        result.diag.px16_m6_executed_compute_mode = PROM_VK_COMPUTE_TILED;
        result.diag.px16_m6_variant_path_status = PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED;
        result.diag.px16_m6_variant_production_eligible = 1u;
        result.diag.px16_m6_variant_dispatch_enabled = 1u;
        result.anomalies.push_back("synthetic_anomaly");
        report.cases.push_back(result);

        VariantComparisonRow comparison;
        comparison.shape = "square_512x512x512";
        comparison.variant = "BALANCED_2X2_ACCUM4";
        comparison.requested_variant = "BALANCED_2X2_ACCUM4";
        comparison.executed_variant = "BALANCED_2X2_ACCUM4";
        comparison.path = "STAGED_UPLOAD_READBACK";
        comparison.correctness = "pass";
        comparison.reference_mode = "dense_cpu_oracle";
        comparison.median_total_ms = 1.0;
        comparison.median_kernel_ms = 0.8;
        comparison.end_to_end_gflops = 250.0;
        comparison.kernel_only_gflops = 312.5;
        report.variant_comparison.push_back(comparison);

        ResidentBenchmarkRow resident;
        resident.shape = "square_512x512x512";
        resident.m = 512u;
        resident.n = 512u;
        resident.k = 512u;
        resident.mode = "production_selector";
        resident.variant = "PRODUCTION_SELECTOR";
        resident.executed_variant = "BALANCED_2X2_ACCUM4";
        resident.correctness = "not_run_in_benchmark_mode";
        resident.reference_mode = "none";
        resident.resident_mode_available = true;
        resident.resident_mode_used = true;
        resident.upload_once_ms = 0.1;
        resident.setup_ms = 1.1;
        resident.kernel_median_ms = 0.7;
        resident.kernel_min_ms = 0.6;
        resident.kernel_p95_ms = 0.8;
        resident.total_loop_ms = 2.2;
        resident.iterations = 3u;
        resident.kernel_only_gflops = 383.0;
        resident.loop_gflops = 365.0;
        resident.gpu_timestamp_valid = true;
        resident.diag.px16_m6_selector_selected_variant = PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4;
        report.resident_production.push_back(resident);

        ResidentBenchmarkRow resident_variant = resident;
        resident_variant.mode = "explicit_variant";
        resident_variant.variant = "BALANCED_2X2_ACCUM4";
        resident_variant.requested_variant = "BALANCED_2X2_ACCUM4";
        report.resident_variant_comparison.push_back(resident_variant);

        SelectorVsFastestRow selector;
        selector.shape = "square_512x512x512";
        selector.production_variant = "BALANCED_2X2_ACCUM4";
        selector.executed_production_variant = "BALANCED_2X2_ACCUM4";
        selector.fastest_variant = "BALANCED_2X2_ACCUM4";
        selector.comparison_basis = "kernel_gpu_ms";
        selector.production_median_ms = 0.8;
        selector.fastest_median_ms = 0.8;
        selector.slowdown_ratio = 1.0;
        selector.picked_same_variant_as_fastest_explicit = true;
        selector.production_slower_than_fastest_explicit = false;
        report.selector_vs_fastest.push_back(selector);
        report.selector_vs_fastest_resident.push_back(selector);

        report.performance_diagnosis.push_back("Synthetic diagnosis row.");
        finalize_summary(report);
        return report;
    }
}

FACT(PrometheusSgemmPx16Evt_ArtifactWritersEmitSchemaAndCaseRows)
{
    const ReportData report = build_synthetic_report();
    const std::string json = render_json(report);
    const std::string markdown = render_markdown(report);

    ASSERT_TRUE(json.find("\"schema\": \"prometheus.sgemm.px16.evt.v3\"") != std::string::npos, "JSON artifact should include the EVT schema");
    ASSERT_TRUE(json.find("\"timing_decomposition\"") != std::string::npos, "JSON artifact should include timing decomposition");
    ASSERT_TRUE(json.find("\"pre_dispatch_ms\"") != std::string::npos, "JSON artifact should include pre-dispatch timing");
    ASSERT_TRUE(json.find("\"command_record_ms\"") != std::string::npos, "JSON artifact should include command-record timing");
    ASSERT_TRUE(json.find("\"tolerance_eval_in_dispatch\"") != std::string::npos, "JSON artifact should include tolerance eval dispatch status");
    ASSERT_TRUE(json.find("\"post_sync_ms\"") != std::string::npos, "JSON artifact should include post-sync timing");
    ASSERT_TRUE(json.find("\"post_readback_ms\"") != std::string::npos, "JSON artifact should include post-readback timing");
    ASSERT_TRUE(json.find("\"unaccounted_host_ms\"") != std::string::npos, "JSON artifact should include unaccounted host timing");
    ASSERT_TRUE(json.find("\"oracle_ms\"") != std::string::npos, "JSON artifact should include oracle timing");
    ASSERT_TRUE(json.find("\"variant_comparison\"") != std::string::npos, "JSON artifact should include variant comparison");
    ASSERT_TRUE(json.find("\"resident_production\"") != std::string::npos, "JSON artifact should include resident production rows");
    ASSERT_TRUE(json.find("\"resident_variant_comparison\"") != std::string::npos, "JSON artifact should include resident variant rows");
    ASSERT_TRUE(json.find("\"selector_vs_fastest_resident\"") != std::string::npos, "JSON artifact should include resident selector comparison");
    ASSERT_TRUE(json.find("\"picked_same_variant_as_fastest_explicit\"") != std::string::npos, "JSON artifact should split selector-vs-fastest identity");
    ASSERT_TRUE(markdown.find("## Timing Decomposition") != std::string::npos, "Markdown artifact should include timing decomposition");
    ASSERT_TRUE(markdown.find("pre-dispatch ms") != std::string::npos, "Markdown artifact should include pre-dispatch timing");
    ASSERT_TRUE(markdown.find("command record ms") != std::string::npos, "Markdown artifact should include command-record timing");
    ASSERT_TRUE(markdown.find("tolerance eval in dispatch") != std::string::npos, "Markdown artifact should include tolerance eval dispatch status");
    ASSERT_TRUE(markdown.find("post-sync ms") != std::string::npos, "Markdown artifact should include post-sync timing");
    ASSERT_TRUE(markdown.find("## Resident Device Benchmark") != std::string::npos, "Markdown artifact should include resident benchmark section");
    ASSERT_TRUE(markdown.find("## Selector vs Fastest Resident Variant") != std::string::npos, "Markdown artifact should include resident selector comparison");
    ASSERT_TRUE(markdown.find("## Explicit Variant Comparison") != std::string::npos, "Markdown artifact should include explicit variant comparison");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_artifact_writer_smoke.json"), json),
                "artifact writer smoke JSON should be created");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_artifact_writer_smoke.md"), markdown),
                "artifact writer smoke Markdown should be created");
}

FACT(PrometheusSgemmPx16Evt_CommandRecordAccounting)
{
    TimingDecomposition timing;
    timing.total_wall_ms = 10.0;
    timing.upload_ms = 1.0;
    timing.pre_dispatch_ms = 0.25;
    timing.command_record_ms = 2.0;
    timing.dispatch_submit_ms = 0.5;
    timing.sync_wait_ms = 1.5;
    timing.post_sync_ms = 0.5;
    timing.readback_ms = 0.75;
    timing.post_readback_ms = 0.25;
    timing.kernel_gpu_ms = 3.0;
    timing.gpu_timestamp_valid = true;
    timing.unaccounted_host_ms = timing.total_wall_ms - accounted_host_ms(timing);

    ASSERT_NEAR(2.0, timing.command_record_ms, 1.0e-9, "command-record timing should be preserved");
    ASSERT_TRUE(timing.pre_dispatch_ms >= 0.0, "pre-dispatch timing should be non-negative");
    ASSERT_TRUE(timing.command_record_ms >= 0.0, "command-record timing should be non-negative");
    ASSERT_TRUE(timing.post_sync_ms >= 0.0, "post-sync timing should be non-negative");
    ASSERT_TRUE(timing.post_readback_ms >= 0.0, "post-readback timing should be non-negative");
    ASSERT_NEAR(1.25, timing.unaccounted_host_ms, 1.0e-9, "unaccounted host timing should subtract coarse H2 buckets without double-counting upload");
}

FACT(PrometheusSgemmPx16Resident)
{
    const ShapeCase shape{"resident_32x32x32", 32u, 32u, 32u, false};
    const PreparedCaseData data = prepare_case_data(shape);
    std::vector<float> c(static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n), 0.0f);

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (handle == nullptr) {
        return;
    }

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "runtime probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; resident SGEMM infrastructure cannot execute");
    }

    PrometheusSgemmResidentBenchmarkRequest request{};
    request.struct_size = sizeof(request);
    request.a = data.a.data();
    request.b = data.b.data();
    request.c = c.data();
    request.m = shape.m;
    request.n = shape.n;
    request.k = shape.k;
    request.mode = PROM_SGEMM_RESIDENT_MODE_PRODUCTION_SELECTOR;
    request.iterations = 2u;
    request.flags = PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK;

    PrometheusSgemmResidentBenchmarkResult result{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_resident_benchmark(handle, &request, &result),
                 "resident production selector benchmark should execute");
    ASSERT_TRUE(result.resident_mode_available != 0u, "resident mode should report available on device-local Vulkan");
    ASSERT_TRUE(result.resident_mode_used != 0u, "resident mode should be used");
    ASSERT_EQUAL(2u, result.iterations, "resident benchmark should execute requested timed iterations");
    ASSERT_TRUE(result.upload_once_wall_ns > 0u, "resident benchmark should report upload-once timing");
    ASSERT_TRUE(result.total_loop_wall_ns > 0u, "resident benchmark should report loop timing");

    TimingDecomposition validation_timing;
    const CorrectnessResult correctness = validate_case_output(shape, data, c, validation_timing);
    ASSERT_EQUAL(std::string("pass"), correctness.status, "resident production selector output should validate");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(diag.px16_m6_selector_selected_variant, result.production_selected_variant,
                 "resident production mode should report selector-selected variant");

    std::fill(c.begin(), c.end(), 0.0f);
    request.mode = PROM_SGEMM_RESIDENT_MODE_EXPLICIT_VARIANT;
    request.requested_variant = PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_resident_benchmark(handle, &request, &result),
                 "resident explicit variant benchmark should execute");
    ASSERT_EQUAL(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE, result.requested_variant,
                 "resident explicit variant request should be recorded");
    if (result.executed_compute_mode == static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED)) {
        ASSERT_EQUAL(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE, result.executed_variant,
                     "resident explicit variant should execute requested tiled variant");
    }
    const CorrectnessResult explicit_correctness = validate_case_output(shape, data, c, validation_timing);
    ASSERT_EQUAL(std::string("pass"), explicit_correctness.status, "resident explicit variant output should validate");

    ASSERT_TRUE(result.validation_wall_ns == result.readback_once_wall_ns,
                "native resident timing should include final readback only, not CPU oracle validation");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");

    PrometheusReactorConfig unavailable_config{};
    unavailable_config.struct_size = sizeof(unavailable_config);
    unavailable_config.test_flags = PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY;
    void* unavailable_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&unavailable_config, &unavailable_handle),
                 "runtime create for unavailable seam should succeed");
    if (unavailable_handle != nullptr) {
        PrometheusSgemmResidentBenchmarkResult unavailable_result{};
        const int unavailable_status =
            prometheus_reactor_runtime_sgemm_resident_benchmark(unavailable_handle, &request, &unavailable_result);
        ASSERT_NOT_EQUAL(PROM_OK, unavailable_status, "resident benchmark should fail clearly when device-local memory is forced unavailable");
        ASSERT_FALSE(unavailable_result.resident_mode_available != 0u,
                     "resident unavailable seam should report resident mode unavailable");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(unavailable_handle), "unavailable seam runtime destroy should succeed");
    }
}

FACT(PrometheusSgemmPx16ResidentExplicitFailureMatrix)
{
    RuntimeHandleScope probe_runtime;
    PrometheusCaps caps{};
    std::string failure_reason;
    if (!create_runtime(probe_runtime, caps, failure_reason)) {
        SKIP("Vulkan runtime unavailable; resident explicit failure matrix cannot execute");
    }

    const std::vector<ShapeCase> shapes = {
        {"small_64x64x64", 64u, 64u, 64u, false},
        {"square_128x128x128", 128u, 128u, 128u, false},
        {"square_256x256x256", 256u, 256u, 256u, false},
        {"skinny_1024x64x1024", 1024u, 64u, 1024u, false},
    };
    const std::vector<std::uint32_t> variants = {
        PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
        PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE,
        PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32,
    };

    std::vector<ResidentFailureMatrixRow> rows;
    for (const ShapeCase& shape : shapes) {
        for (const std::uint32_t variant : variants) {
            rows.push_back(build_failure_matrix_row(shape, variant));
        }
    }

    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_px16_resident_failure_matrix.json"),
                    render_failure_matrix_json(rows)),
                "resident explicit failure matrix JSON should be written");
    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_px16_resident_failure_matrix.md"),
                    render_failure_matrix_markdown(rows)),
                "resident explicit failure matrix markdown should be written");

    bool observed_large_failure = false;
    bool all_large_rows_passed = true;
    for (const ResidentFailureMatrixRow& row : rows) {
        if ((row.shape == "square_256x256x256" || row.shape == "skinny_1024x64x1024") &&
            (row.staged_runtime_status != PROM_OK || row.resident_runtime_status != PROM_OK)) {
            observed_large_failure = true;
            all_large_rows_passed = false;
            ASSERT_FALSE(row.resident_failure_stage.empty(), "failure matrix should populate a failure stage for large-case failures");
            ASSERT_TRUE(row.staged_out_stage != PROM_STAGE_NONE || row.resident_out_stage != PROM_STAGE_NONE,
                        "failure matrix should preserve out_stage for large-case failures");
            ASSERT_TRUE(row.staged_out_detail_code != 0 || row.resident_out_detail_code != 0,
                        "failure matrix should preserve out_detail_code for large-case failures");
        }
    }
    if (!observed_large_failure) {
        for (const ResidentFailureMatrixRow& row : rows) {
            if (row.shape == "square_256x256x256" || row.shape == "skinny_1024x64x1024") {
                ASSERT_EQUAL(std::string("passed"), row.staged_warmup_result,
                             "large-case staged warmup should pass when the fresh-runtime matrix proves the old broad failure was a reuse cascade");
                ASSERT_EQUAL(std::string("passed"), row.staged_measured_result,
                             "large-case staged measured dispatch should pass when isolated on a fresh runtime");
                ASSERT_EQUAL(std::string("passed"), row.resident_warmup_result,
                             "large-case resident warmup should pass when isolated on a fresh runtime");
            }
        }
    }
    ASSERT_TRUE(observed_large_failure || all_large_rows_passed,
                "focused matrix should either reproduce an isolated large-case failure or prove the fresh-runtime rows all pass");
}

FACT(PrometheusSgemmPx16M15aSdslScalarPlusLowKRepro)
{
    RuntimeHandleScope probe_runtime;
    PrometheusCaps caps{};
    std::string failure_reason;
    if (!create_runtime(probe_runtime, caps, failure_reason)) {
        SKIP("Vulkan runtime unavailable; M15a low-K repro cannot execute");
    }

    const std::vector<ShapeCase> shapes = {
        {"lowk_128x128x64", 128u, 128u, 64u, false},
        {"lowk_256x256x64", 256u, 256u, 64u, false},
        {"lowk_512x512x64", 512u, 512u, 64u, false},
        {"lowk_1024x1024x16", 1024u, 1024u, 16u, false},
        {"lowk_1024x1024x32", 1024u, 1024u, 32u, false},
        {"lowk_1024x1024x64", 1024u, 1024u, 64u, false},
        {"lowk_1024x1024x65", 1024u, 1024u, 65u, false},
    };
    const std::vector<std::uint32_t> variants = {
        PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR,
        PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32,
        PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32,
    };

    std::vector<ResidentFailureMatrixRow> rows;
    rows.reserve(shapes.size() * variants.size());
    for (const ShapeCase& shape : shapes) {
        for (const std::uint32_t variant : variants) {
            rows.push_back(build_failure_matrix_row(shape, variant));
        }
    }

    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_px16_m15a_sdsl_scalar_plus_lowk_repro.json"),
                    render_failure_matrix_json(rows)),
                "M15a low-K repro JSON should be written");
    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_px16_m15a_sdsl_scalar_plus_lowk_repro.md"),
                    render_failure_matrix_markdown(rows)),
                "M15a low-K repro markdown should be written");

    bool saw_scalar_plus_row = false;
    for (const ResidentFailureMatrixRow& row : rows) {
        if (row.requested_variant == "SDSL_SCALAR_PLUS") {
            saw_scalar_plus_row = true;
            ASSERT_TRUE(row.metadata_numthreads_x != 0u && row.metadata_numthreads_y != 0u,
                        "SDSL scalar-plus repro rows must record generated metadata");
            ASSERT_TRUE(row.shader_module_present, "SDSL scalar-plus repro rows must report shader module presence");
            ASSERT_TRUE(row.descriptor_binding_count == 3u, "SDSL scalar-plus repro rows must preserve descriptor binding count");
            ASSERT_TRUE(row.push_constant_bytes == 12u, "SDSL scalar-plus repro rows must preserve push-constant ABI size");
            ASSERT_NOT_EQUAL(std::string("VK_ERROR_DEVICE_LOST"), row.vk_result,
                             "scalar-plus low-K repro should not lose the device after the ABI fix");
        } else {
            ASSERT_NOT_EQUAL(std::string("VK_ERROR_DEVICE_LOST"), row.vk_result,
                             "control variants should not lose the device in the focused low-K repro");
        }
    }
    ASSERT_TRUE(saw_scalar_plus_row, "M15a low-K repro must include scalar-plus rows");
}

FACT(PrometheusSgemmM17SdslReg2x2)
{
    RuntimeHandleScope probe_runtime;
    PrometheusCaps caps{};
    std::string failure_reason;
    if (!create_runtime(probe_runtime, caps, failure_reason)) {
        SKIP("Vulkan runtime unavailable; M17 explicit reg2x2 lane cannot execute");
    }

    const std::vector<ShapeCase> shapes = {
        {"exact_16x16x16", 16u, 16u, 16u, false},
        {"small_8x8x8", 8u, 8u, 8u, false},
        {"odd_17x17x17", 17u, 17u, 17u, false},
        {"odd_31x29x23", 31u, 29u, 23u, false},
        {"skinny_64x16x64", 64u, 16u, 64u, false},
        {"wide_16x64x64", 16u, 64u, 64u, false},
        {"lowk_64x64x8", 64u, 64u, 8u, false},
        {"medium_128x128x128", 128u, 128u, 128u, false},
    };

    std::vector<VariantComparisonRow> staged_rows;
    std::vector<ResidentBenchmarkRow> resident_rows;
    staged_rows.reserve(shapes.size());
    resident_rows.reserve(shapes.size());
    for (const ShapeCase& shape : shapes) {
        staged_rows.push_back(run_explicit_variant_case_fresh(shape, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32, true));
        resident_rows.push_back(run_resident_case_fresh(shape, true, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32, true));
    }

    std::ostringstream json;
    json << "{\n";
    json << "  \"schema\": \"prometheus.sgemm.sdslv.m17.reg2x2.v1\",\n";
    json << "  \"variant\": \"SDSL_REG2X2_TILE16X16_FP32\",\n";
    json << "  \"rows\": [\n";
    for (std::size_t index = 0u; index < staged_rows.size(); ++index) {
        const VariantComparisonRow& staged = staged_rows[index];
        const ResidentBenchmarkRow& resident = resident_rows[index];
        json << "    {\"shape\": \"" << json_escape(staged.shape)
             << "\", \"m\": " << staged.m
             << ", \"n\": " << staged.n
             << ", \"k\": " << staged.k
             << ", \"correctness\": \"" << json_escape(staged.correctness)
             << "\", \"kernel_ms\": " << resident.kernel_median_ms
             << ", \"wall_ms\": " << staged.median_total_ms
             << ", \"gflops\": " << resident.kernel_only_gflops
             << ", \"resident_mode_available\": " << bool_json(resident.resident_mode_available)
             << ", \"executed_variant\": \"" << json_escape(resident.executed_variant) << "\"}";
        if (index + 1u < staged_rows.size()) {
            json << ",";
        }
        json << "\n";
    }
    json << "  ]\n";
    json << "}\n";

    std::ostringstream markdown;
    markdown << "# Prometheus SDSL-V M17 Reg2x2 SGEMM\n\n";
    markdown << "| shape | correctness | staged wall ms | resident kernel ms | resident GFLOP/s | executed variant |\n";
    markdown << "| --- | --- | ---: | ---: | ---: | --- |\n";
    for (std::size_t index = 0u; index < staged_rows.size(); ++index) {
        const VariantComparisonRow& staged = staged_rows[index];
        const ResidentBenchmarkRow& resident = resident_rows[index];
        markdown << "| " << staged.shape
                 << " | " << staged.correctness
                 << " | " << staged.median_total_ms
                 << " | " << resident.kernel_median_ms
                 << " | " << resident.kernel_only_gflops
                 << " | " << resident.executed_variant << " |\n";
    }

    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m17_reg2x2.json"),
                    json.str()),
                "M17 reg2x2 JSON artifact should be written");
    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m17_reg2x2.md"),
                    markdown.str()),
                "M17 reg2x2 markdown artifact should be written");

    for (const VariantComparisonRow& row : staged_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M17 staged explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M17 staged explicit variant should validate");
    }
    for (const ResidentBenchmarkRow& row : resident_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M17 resident explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M17 resident explicit variant should validate");
    }
}

FACT(PrometheusSgemmM20SdslReg2x2ExactTail)
{
    RuntimeHandleScope probe_runtime;
    PrometheusCaps caps{};
    std::string failure_reason;
    if (!create_runtime(probe_runtime, caps, failure_reason)) {
        SKIP("Vulkan runtime unavailable; M20 explicit exacttail lane cannot execute");
    }

    const std::vector<ShapeCase> shapes = {
        {"exact_16x16x16", 16u, 16u, 16u, false},
        {"exact_32x32x32", 32u, 32u, 32u, false},
        {"small_8x8x8", 8u, 8u, 8u, false},
        {"odd_17x17x17", 17u, 17u, 17u, false},
        {"odd_31x29x23", 31u, 29u, 23u, false},
        {"skinny_64x16x64", 64u, 16u, 64u, false},
        {"wide_16x64x64", 16u, 64u, 64u, false},
        {"lowk_64x64x8", 64u, 64u, 8u, false},
        {"medium_128x128x128", 128u, 128u, 128u, false},
    };

    std::vector<VariantComparisonRow> m17_staged_rows;
    std::vector<VariantComparisonRow> m20_staged_rows;
    std::vector<ResidentBenchmarkRow> m17_resident_rows;
    std::vector<ResidentBenchmarkRow> m20_resident_rows;
    m17_staged_rows.reserve(shapes.size());
    m20_staged_rows.reserve(shapes.size());
    m17_resident_rows.reserve(shapes.size());
    m20_resident_rows.reserve(shapes.size());
    for (const ShapeCase& shape : shapes) {
        m17_staged_rows.push_back(run_explicit_variant_case_fresh(shape, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32, true));
        m20_staged_rows.push_back(run_explicit_variant_case_fresh(shape, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32, true));
        m17_resident_rows.push_back(run_resident_case_fresh(shape, true, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32, true));
        m20_resident_rows.push_back(run_resident_case_fresh(shape, true, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32, true));
    }

    std::ostringstream json;
    json << "{\n";
    json << "  \"schema\": \"prometheus.sgemm.sdslv.m20.exacttail.v1\",\n";
    json << "  \"baseline_variant\": \"SDSL_REG2X2_TILE16X16_FP32\",\n";
    json << "  \"candidate_variant\": \"SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32\",\n";
    json << "  \"metadata\": {\"numthreads_x\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_x
         << ", \"numthreads_y\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_y
         << ", \"numthreads_z\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_numthreads_z
         << ", \"outputs_per_invocation_m\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_m
         << ", \"outputs_per_invocation_n\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_outputs_per_invocation_n
         << ", \"tile_m\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_m
         << ", \"tile_n\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_n
         << ", \"tile_k\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_tile_k
         << ", \"unroll_k\": " << k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv_unroll_k << "},\n";
    json << "  \"rows\": [\n";
    for (std::size_t index = 0u; index < shapes.size(); ++index) {
        const VariantComparisonRow& m17_staged = m17_staged_rows[index];
        const VariantComparisonRow& m20_staged = m20_staged_rows[index];
        const ResidentBenchmarkRow& m17_resident = m17_resident_rows[index];
        const ResidentBenchmarkRow& m20_resident = m20_resident_rows[index];
        json << "    {\"shape\": \"" << json_escape(m20_staged.shape)
             << "\", \"m\": " << m20_staged.m
             << ", \"n\": " << m20_staged.n
             << ", \"k\": " << m20_staged.k
             << ", \"m17_correctness\": \"" << json_escape(m17_staged.correctness)
             << "\", \"m20_correctness\": \"" << json_escape(m20_staged.correctness)
             << "\", \"m17_staged_wall_ms\": " << m17_staged.median_total_ms
             << ", \"m20_staged_wall_ms\": " << m20_staged.median_total_ms
             << ", \"m17_resident_kernel_ms\": " << m17_resident.kernel_median_ms
             << ", \"m20_resident_kernel_ms\": " << m20_resident.kernel_median_ms
             << ", \"m17_resident_gflops\": " << m17_resident.kernel_only_gflops
             << ", \"m20_resident_gflops\": " << m20_resident.kernel_only_gflops
             << ", \"m17_executed_variant\": \"" << json_escape(m17_resident.executed_variant)
             << "\", \"m20_executed_variant\": \"" << json_escape(m20_resident.executed_variant) << "\"}";
        if (index + 1u < shapes.size()) {
            json << ",";
        }
        json << "\n";
    }
    json << "  ]\n";
    json << "}\n";

    std::ostringstream markdown;
    markdown << "# Prometheus SDSL-V M20 ExactTail SGEMM\n\n";
    markdown << "| shape | M17 correctness | M20 correctness | M17 staged wall ms | M20 staged wall ms | M17 resident kernel ms | M20 resident kernel ms | M17 GFLOP/s | M20 GFLOP/s | M20 executed variant |\n";
    markdown << "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n";
    for (std::size_t index = 0u; index < shapes.size(); ++index) {
        const VariantComparisonRow& m17_staged = m17_staged_rows[index];
        const VariantComparisonRow& m20_staged = m20_staged_rows[index];
        const ResidentBenchmarkRow& m17_resident = m17_resident_rows[index];
        const ResidentBenchmarkRow& m20_resident = m20_resident_rows[index];
        markdown << "| " << m20_staged.shape
                 << " | " << m17_staged.correctness
                 << " | " << m20_staged.correctness
                 << " | " << m17_staged.median_total_ms
                 << " | " << m20_staged.median_total_ms
                 << " | " << m17_resident.kernel_median_ms
                 << " | " << m20_resident.kernel_median_ms
                 << " | " << m17_resident.kernel_only_gflops
                 << " | " << m20_resident.kernel_only_gflops
                 << " | " << m20_resident.executed_variant << " |\n";
    }

    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m20_exacttail.json"),
                    json.str()),
                "M20 exacttail JSON artifact should be written");
    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m20_exacttail.md"),
                    markdown.str()),
                "M20 exacttail markdown artifact should be written");

    for (const VariantComparisonRow& row : m20_staged_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M20 staged explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M20 staged explicit variant should validate");
        ASSERT_EQUAL(std::string("SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32"), row.executed_variant,
                     "M20 staged explicit variant should report exacttail execution");
    }
    for (const ResidentBenchmarkRow& row : m20_resident_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M20 resident explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M20 resident explicit variant should validate");
        ASSERT_EQUAL(std::string("SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32"), row.executed_variant,
                     "M20 resident explicit variant should report exacttail execution");
    }
}

FACT(PrometheusSgemmM24SdslReg2x2FlowBoard)
{
    RuntimeHandleScope probe_runtime;
    PrometheusCaps caps{};
    std::string failure_reason;
    if (!create_runtime(probe_runtime, caps, failure_reason)) {
        SKIP("Vulkan runtime unavailable; M24 explicit flowboard lane cannot execute");
    }

    const std::vector<ShapeCase> shapes = {
        {"exact_16x16x16", 16u, 16u, 16u, false},
        {"exact_32x32x32", 32u, 32u, 32u, false},
        {"small_8x8x8", 8u, 8u, 8u, false},
        {"odd_17x17x17", 17u, 17u, 17u, false},
        {"odd_31x29x23", 31u, 29u, 23u, false},
        {"skinny_64x16x64", 64u, 16u, 64u, false},
        {"wide_16x64x64", 16u, 64u, 64u, false},
        {"lowk_64x64x8", 64u, 64u, 8u, false},
        {"medium_128x128x128", 128u, 128u, 128u, false},
    };

    std::vector<VariantComparisonRow> m20_staged_rows;
    std::vector<VariantComparisonRow> m24_staged_rows;
    std::vector<ResidentBenchmarkRow> m20_resident_rows;
    std::vector<ResidentBenchmarkRow> m24_resident_rows;
    m20_staged_rows.reserve(shapes.size());
    m24_staged_rows.reserve(shapes.size());
    m20_resident_rows.reserve(shapes.size());
    m24_resident_rows.reserve(shapes.size());
    for (const ShapeCase& shape : shapes) {
        m20_staged_rows.push_back(run_explicit_variant_case_fresh(shape, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32, true));
        m24_staged_rows.push_back(run_explicit_variant_case_fresh(shape, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32, true));
        m20_resident_rows.push_back(run_resident_case_fresh(shape, true, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32, true));
        m24_resident_rows.push_back(run_resident_case_fresh(shape, true, PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32, true));
    }

    std::ostringstream json;
    json << "{\n";
    json << "  \"schema\": \"prometheus.sgemm.sdslv.m24.flowboard.v1\",\n";
    json << "  \"baseline_variant\": \"SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32\",\n";
    json << "  \"candidate_variant\": \"SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32\",\n";
    json << "  \"metadata\": {\"numthreads_x\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_x
         << ", \"numthreads_y\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_y
         << ", \"numthreads_z\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_numthreads_z
         << ", \"outputs_per_invocation_m\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_m
         << ", \"outputs_per_invocation_n\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_outputs_per_invocation_n
         << ", \"tile_m\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_m
         << ", \"tile_n\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_n
         << ", \"tile_k\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_tile_k
         << ", \"unroll_k\": " << k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv_unroll_k << "},\n";
    json << "  \"rows\": [\n";
    for (std::size_t index = 0u; index < shapes.size(); ++index) {
        const VariantComparisonRow& m20_staged = m20_staged_rows[index];
        const VariantComparisonRow& m24_staged = m24_staged_rows[index];
        const ResidentBenchmarkRow& m20_resident = m20_resident_rows[index];
        const ResidentBenchmarkRow& m24_resident = m24_resident_rows[index];
        const double ratio =
            m24_resident.kernel_median_ms > 0.0 ? (m20_resident.kernel_median_ms / m24_resident.kernel_median_ms) : 0.0;
        json << "    {\"shape\": \"" << json_escape(m24_staged.shape)
             << "\", \"m\": " << m24_staged.m
             << ", \"n\": " << m24_staged.n
             << ", \"k\": " << m24_staged.k
             << ", \"m20_correctness\": \"" << json_escape(m20_staged.correctness)
             << "\", \"m24_correctness\": \"" << json_escape(m24_staged.correctness)
             << "\", \"m20_staged_wall_ms\": " << m20_staged.median_total_ms
             << ", \"m24_staged_wall_ms\": " << m24_staged.median_total_ms
             << ", \"m20_resident_kernel_ms\": " << m20_resident.kernel_median_ms
             << ", \"m24_resident_kernel_ms\": " << m24_resident.kernel_median_ms
             << ", \"m20_resident_gflops\": " << m20_resident.kernel_only_gflops
             << ", \"m24_resident_gflops\": " << m24_resident.kernel_only_gflops
             << ", \"m20_vs_m24_kernel_ratio\": " << ratio
             << ", \"m20_executed_variant\": \"" << json_escape(m20_resident.executed_variant)
             << "\", \"m24_executed_variant\": \"" << json_escape(m24_resident.executed_variant) << "\"}";
        if (index + 1u < shapes.size()) {
            json << ",";
        }
        json << "\n";
    }
    json << "  ]\n";
    json << "}\n";

    std::ostringstream markdown;
    markdown << "# Prometheus SDSL-V M24 FlowBoard SGEMM\n\n";
    markdown << "| shape | M20 correctness | M24 correctness | M20 staged wall ms | M24 staged wall ms | M20 resident kernel ms | M24 resident kernel ms | M20 GFLOP/s | M24 GFLOP/s | M20/M24 kernel ratio | M24 executed variant |\n";
    markdown << "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n";
    for (std::size_t index = 0u; index < shapes.size(); ++index) {
        const VariantComparisonRow& m20_staged = m20_staged_rows[index];
        const VariantComparisonRow& m24_staged = m24_staged_rows[index];
        const ResidentBenchmarkRow& m20_resident = m20_resident_rows[index];
        const ResidentBenchmarkRow& m24_resident = m24_resident_rows[index];
        const double ratio =
            m24_resident.kernel_median_ms > 0.0 ? (m20_resident.kernel_median_ms / m24_resident.kernel_median_ms) : 0.0;
        markdown << "| " << m24_staged.shape
                 << " | " << m20_staged.correctness
                 << " | " << m24_staged.correctness
                 << " | " << m20_staged.median_total_ms
                 << " | " << m24_staged.median_total_ms
                 << " | " << m20_resident.kernel_median_ms
                 << " | " << m24_resident.kernel_median_ms
                 << " | " << m20_resident.kernel_only_gflops
                 << " | " << m24_resident.kernel_only_gflops
                 << " | " << ratio
                 << " | " << m24_resident.executed_variant << " |\n";
    }

    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m24_flowboard.json"),
                    json.str()),
                "M24 flowboard JSON artifact should be written");
    ASSERT_TRUE(context.WriteArtifactFile(
                    std::filesystem::path("prometheus_sgemm_sdslv_m24_flowboard.md"),
                    markdown.str()),
                "M24 flowboard markdown artifact should be written");

    for (const VariantComparisonRow& row : m24_staged_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M24 staged explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M24 staged explicit variant should validate");
        ASSERT_EQUAL(std::string("SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32"), row.executed_variant,
                     "M24 staged explicit variant should report flowboard execution");
    }
    for (const ResidentBenchmarkRow& row : m24_resident_rows) {
        ASSERT_EQUAL(PROM_OK, row.runtime_status, "M24 resident explicit variant should run");
        ASSERT_EQUAL(std::string("pass"), row.correctness, "M24 resident explicit variant should validate");
        ASSERT_EQUAL(std::string("SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32"), row.executed_variant,
                     "M24 resident explicit variant should report flowboard execution");
    }
}

BENCHMARK_WITH_ITERATIONS(PrometheusSgemmPx16Evt_ProductionPerformanceLane, 1)
{
    ReportData report;
    report.timestamp_utc = timestamp_now_utc();
    report.benchmark_name = context.BenchmarkName();
    report.run_mode = "performance_benchmark";
    report.validation_status_source = "not_run_in_benchmark_mode";
    report.explicit_cube_1024_enabled = should_run_explicit_1024_cube();

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (handle == nullptr) {
        return;
    }

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "runtime probe should succeed");
    if (caps.available == 0u) {
        report.global_skip_reason = "vulkan_runtime_unavailable";
        finalize_summary(report);
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_results.json"), render_json(report)),
                    "skip JSON artifact should be written");
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_report.md"), render_markdown(report)),
                    "skip Markdown artifact should be written");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; EVT SGEMM artifact lane cannot execute");
    }

    (void)fetch_device_metadata(handle, caps, report.device);
    const bool software_backend = caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE ||
        report.device.backend == "VULKAN_SOFTWARE" ||
        report.device.device_type == "CPU" ||
        report.device.name.find("llvmpipe") != std::string::npos ||
        report.device.name.find("LLVMPIPE") != std::string::npos;
    const bool expected_3070 = report.device.name.find("RTX 3070") != std::string::npos;
    if (software_backend || !expected_3070) {
        report.global_skip_reason = software_backend
            ? "benchmark_guard_rejected_software_vulkan_backend"
            : "benchmark_guard_rejected_non_3070_device";
        report.performance_diagnosis.push_back(
            "Benchmark guard refused to run timing characterization because the runtime was not the expected hardware Vulkan path on the local RTX 3070.");
        finalize_summary(report);
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_results.json"), render_json(report)),
                    "guard JSON artifact should be written");
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_report.md"), render_markdown(report)),
                    "guard Markdown artifact should be written");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP(software_backend ? "Software Vulkan backend detected; refusing to benchmark non-GPU path"
                              : "Expected local RTX 3070 device was not selected; refusing to benchmark wrong adapter");
    }

    for (const ShapeCase& shape : evt_shapes()) {
        report.cases.push_back(run_evt_case(handle, shape, false));
        report.resident_production.push_back(run_resident_case(handle, shape, false, PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR, false));
    }

    for (const ShapeCase& shape : explicit_variant_shapes(report.explicit_cube_1024_enabled)) {
        for (const std::uint32_t variant : wired_variants()) {
            report.variant_comparison.push_back(run_explicit_variant_case_fresh(shape, variant, false));
            report.resident_variant_comparison.push_back(run_resident_case_fresh(shape, true, variant, false));
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_production) {
        report.resident_device_mode_available = report.resident_device_mode_available || row.resident_mode_available;
    }
    for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
        report.resident_device_mode_available = report.resident_device_mode_available || row.resident_mode_available;
    }
    postprocess_variant_comparison(report);
    postprocess_resident_variant_comparison(report);
    build_selector_vs_fastest(report);
    build_selector_vs_fastest_resident(report);
    detect_cross_case_anomalies(report);
    build_diagnosis(report);
    finalize_summary(report);

    const std::string json = render_json(report);
    const std::string markdown = render_markdown(report);
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_results.json"), json),
                "EVT JSON artifact should be written");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_report.md"), markdown),
                "EVT Markdown artifact should be written");

    for (const CaseResult& result : report.cases) {
        if (result.skipped) {
            continue;
        }
        if (result.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_RUNTIME",
                "EVT case runtime call failed",
                result.name,
                std::to_string(result.runtime_status));
        }
    }

    for (const VariantComparisonRow& row : report.variant_comparison) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_EXPLICIT_RUNTIME",
                "explicit variant case runtime call failed",
                row.shape + ":" + row.variant,
                std::to_string(row.runtime_status));
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_production) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_RESIDENT_RUNTIME",
                "resident production case runtime call failed",
                row.shape,
                std::to_string(row.runtime_status));
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_RESIDENT_EXPLICIT_RUNTIME",
                "resident explicit variant case runtime call failed",
                row.shape + ":" + row.variant,
                std::to_string(row.runtime_status));
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmPx16Evt_CorrectnessValidationLane)
{
    ReportData report;
    report.timestamp_utc = timestamp_now_utc();
    report.benchmark_name = context.TestName();
    report.run_mode = "correctness_validation";
    report.validation_status_source = "explicit_fact_lane";

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (handle == nullptr) {
        return;
    }

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "runtime probe should succeed");
    if (caps.available == 0u) {
        report.global_skip_reason = "vulkan_runtime_unavailable";
        finalize_summary(report);
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_validation_results.json"), render_json(report)),
                    "skip validation JSON artifact should be written");
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_validation_report.md"), render_markdown(report)),
                    "skip validation Markdown artifact should be written");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; Px16 SGEMM validation lane cannot execute");
    }

    (void)fetch_device_metadata(handle, caps, report.device);

    for (const ShapeCase& shape : correctness_shapes()) {
        report.cases.push_back(run_evt_case(handle, shape, true));
        report.resident_production.push_back(run_resident_case(handle, shape, false, PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR, true));
    }

    for (const ShapeCase& shape : correctness_shapes()) {
        for (const std::uint32_t variant : wired_variants()) {
            report.variant_comparison.push_back(run_explicit_variant_case_fresh(shape, variant, true));
            report.resident_variant_comparison.push_back(run_resident_case_fresh(shape, true, variant, true));
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_production) {
        report.resident_device_mode_available = report.resident_device_mode_available || row.resident_mode_available;
    }
    for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
        report.resident_device_mode_available = report.resident_device_mode_available || row.resident_mode_available;
    }
    postprocess_variant_comparison(report);
    postprocess_resident_variant_comparison(report);
    build_selector_vs_fastest(report);
    build_selector_vs_fastest_resident(report);
    detect_cross_case_anomalies(report);
    build_diagnosis(report);
    finalize_summary(report);

    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_validation_results.json"), render_json(report)),
                "validation JSON artifact should be written");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_validation_report.md"), render_markdown(report)),
                "validation Markdown artifact should be written");

    for (const CaseResult& result : report.cases) {
        if (result.skipped) {
            continue;
        }
        if (result.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_RUNTIME",
                "validation case runtime call failed",
                result.name,
                std::to_string(result.runtime_status));
        }
        if (result.correctness.status != "pass") {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_CORRECTNESS",
                "validation case correctness failed",
                result.name,
                result.correctness.status);
        }
    }

    for (const VariantComparisonRow& row : report.variant_comparison) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_EXPLICIT_RUNTIME",
                "explicit variant validation case runtime call failed",
                row.shape + ":" + row.variant,
                std::to_string(row.runtime_status));
        }
        if (row.correctness != "pass") {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_EXPLICIT_CORRECTNESS",
                "explicit variant validation case correctness failed",
                row.shape + ":" + row.variant,
                row.correctness);
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_production) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_RESIDENT_RUNTIME",
                "resident production validation case runtime call failed",
                row.shape,
                std::to_string(row.runtime_status));
        }
        if (row.correctness != "pass") {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_RESIDENT_CORRECTNESS",
                "resident production validation case correctness failed",
                row.shape,
                row.correctness);
        }
    }

    for (const ResidentBenchmarkRow& row : report.resident_variant_comparison) {
        if (row.skipped) {
            continue;
        }
        if (row.runtime_status != PROM_OK) {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_RESIDENT_EXPLICIT_RUNTIME",
                "resident explicit variant validation case runtime call failed",
                row.shape + ":" + row.variant,
                std::to_string(row.runtime_status));
        }
        if (row.correctness != "pass") {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_VALIDATION_RESIDENT_EXPLICIT_CORRECTNESS",
                "resident explicit variant validation case correctness failed",
                row.shape + ":" + row.variant,
                row.correctness);
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
