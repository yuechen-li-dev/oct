#include "../reactor_api.h"
#include "../reactor_judgment_engine.h"
#include "../reactor_policy_memory.h"
#include "../reactor_vulkan.h"
#include "test_harness.h"

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <ctime>
#include <filesystem>
#include <sstream>
#include <string>
#include <string_view>
#include <vector>

namespace
{
    struct ShapeCase
    {
        const char* name;
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
        bool optional;
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
    };

    struct DeepDiagnosticRow
    {
        std::string shape;
        std::uint32_t m = 0u;
        std::uint32_t n = 0u;
        std::uint32_t k = 0u;
        std::string policy_mode = "UNKNOWN";
        std::string requested_variant = "UNKNOWN";
        std::string selected_variant = "UNKNOWN";
        std::string executed_variant = "UNKNOWN";
        std::string path = "UNKNOWN";
        double total_wall_ms = 0.0;
        double kernel_ms = 0.0;
        double upload_ms = 0.0;
        double readback_ms = 0.0;
        double sync_wait_ms = 0.0;
        double dispatch_submit_ms = 0.0;
        double command_record_ms = 0.0;
        double unaccounted_host_ms = 0.0;
        bool gpu_timestamp_valid = false;
        bool resident_available = false;
        double resident_kernel_ms = 0.0;
        double resident_loop_avg_ms = 0.0;
        double resident_vs_production_gap_ms = 0.0;
        int runtime_status = PROM_OK;
        std::uint32_t final_stage = PROM_STAGE_NONE;
        int final_detail_code = 0;
        std::string note;
    };

    struct DeepDiagnosticReport
    {
        std::string timestamp_utc;
        DeviceMetadata device;
        std::string vk_instance_layers = "(unset)";
        std::string vk_loader_layers_enable = "(unset)";
        std::vector<DeepDiagnosticRow> rows;
        std::string global_note;
    };

    constexpr ShapeCase kDeepShapes[] = {
        {"square_128x128x128", 128u, 128u, 128u, false},
        {"square_256x256x256", 256u, 256u, 256u, false},
        {"square_512x512x512", 512u, 512u, 512u, false},
        {"lowk_1024x1024x64", 1024u, 1024u, 64u, false},
        {"rect_255x129x65", 255u, 129u, 65u, false},
    };

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
                    out << ch;
                    break;
            }
        }
        return out.str();
    }

    std::string bool_json(bool value)
    {
        return value ? "true" : "false";
    }

    double ns_to_ms(std::uint64_t ns)
    {
        return static_cast<double>(ns) / 1.0e6;
    }

    double median_ms_from_ns(std::vector<double> samples_ns)
    {
        if (samples_ns.empty()) {
            return 0.0;
        }
        std::sort(samples_ns.begin(), samples_ns.end());
        return samples_ns[samples_ns.size() / 2u] / 1.0e6;
    }

    std::string env_value_or_unset(const char* name)
    {
#if defined(_WIN32)
        char* value = nullptr;
        std::size_t value_length = 0u;
        if (_dupenv_s(&value, &value_length, name) != 0 || value == nullptr) {
            return "(unset)";
        }
        const std::string result(value, value_length > 0u ? value_length - 1u : 0u);
        free(value);
        return result.empty() ? "(unset)" : result;
#else
        const char* value = std::getenv(name);
        return value == nullptr || value[0] == '\0' ? "(unset)" : std::string(value);
#endif
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

    std::string vulkan_version_name(std::uint32_t version)
    {
        if (version == 0u) {
            return "unknown";
        }
        std::ostringstream out;
        out << VK_VERSION_MAJOR(version) << "." << VK_VERSION_MINOR(version) << "." << VK_VERSION_PATCH(version);
        return out.str();
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
        return true;
    }

    DeepDiagnosticRow run_shape_case(void* handle, const ShapeCase& shape)
    {
        DeepDiagnosticRow row;
        row.shape = shape.name;
        row.m = shape.m;
        row.n = shape.n;
        row.k = shape.k;

        const std::size_t a_elems = static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.k);
        const std::size_t b_elems = static_cast<std::size_t>(shape.k) * static_cast<std::size_t>(shape.n);
        const std::size_t c_elems = static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n);
        std::vector<float> a(a_elems, 1.0f);
        std::vector<float> b(b_elems, 1.0f);
        std::vector<float> c(c_elems, 0.0f);

        for (int i = 0; i < 2; ++i) {
            std::uint32_t stage = PROM_STAGE_NONE;
            int detail = 0;
            (void)prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail);
        }

        std::vector<double> total_wall_ns;
        std::vector<double> kernel_gpu_ns;
        std::vector<double> upload_wall_ns;
        std::vector<double> command_record_wall_ns;
        std::vector<double> dispatch_submit_wall_ns;
        std::vector<double> sync_wait_wall_ns;
        std::vector<double> readback_wall_ns;
        PrometheusSgemmPolicyDiagnostics final_diag{};

        for (int i = 0; i < 5; ++i) {
            std::uint32_t stage = PROM_STAGE_NONE;
            int detail = 0;
            const auto begin = std::chrono::steady_clock::now();
            row.runtime_status =
                prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail);
            const auto end = std::chrono::steady_clock::now();
            row.final_stage = stage;
            row.final_detail_code = detail;
            if (row.runtime_status != PROM_OK) {
                row.note = "production_runtime_failed";
                return row;
            }

            total_wall_ns.push_back(static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(end - begin).count()));
            if (prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &final_diag) != PROM_OK) {
                row.runtime_status = PROM_ERROR;
                row.final_stage = stage;
                row.final_detail_code = detail;
                row.note = "policy_diagnostics_query_failed";
                return row;
            }
            upload_wall_ns.push_back(static_cast<double>(final_diag.px16_m8_last_upload_wall_ns));
            command_record_wall_ns.push_back(static_cast<double>(final_diag.px16_m8_last_command_record_wall_ns));
            dispatch_submit_wall_ns.push_back(static_cast<double>(final_diag.px16_m8_last_dispatch_submit_wall_ns));
            sync_wait_wall_ns.push_back(static_cast<double>(final_diag.px16_m8_last_sync_wait_wall_ns));
            readback_wall_ns.push_back(static_cast<double>(final_diag.px16_m8_last_readback_wall_ns));
            if (final_diag.p13_m5_last_gpu_timing_valid != 0u && final_diag.p13_m5_last_gpu_duration_ns > 0u) {
                kernel_gpu_ns.push_back(static_cast<double>(final_diag.p13_m5_last_gpu_duration_ns));
            }
        }

        row.policy_mode = policy_mode_name(final_diag.px16_m6_policy_mode);
        row.requested_variant = occupancy_variant_name(final_diag.px16_m6_requested_dispatch_variant);
        row.selected_variant = occupancy_variant_name(final_diag.px16_m6_selector_selected_variant);
        row.executed_variant = occupancy_variant_name(final_diag.px16_m6_executed_dispatch_variant);
        row.path = path_name(final_diag.px16_m6_executed_path);
        row.total_wall_ms = median_ms_from_ns(total_wall_ns);
        row.kernel_ms = median_ms_from_ns(kernel_gpu_ns);
        row.upload_ms = median_ms_from_ns(upload_wall_ns);
        row.command_record_ms = median_ms_from_ns(command_record_wall_ns);
        row.dispatch_submit_ms = median_ms_from_ns(dispatch_submit_wall_ns);
        row.sync_wait_ms = median_ms_from_ns(sync_wait_wall_ns);
        row.readback_ms = median_ms_from_ns(readback_wall_ns);
        row.gpu_timestamp_valid = kernel_gpu_ns.size() == total_wall_ns.size() && !kernel_gpu_ns.empty();
        const double accounted_ms =
            row.upload_ms +
            row.command_record_ms +
            row.dispatch_submit_ms +
            row.sync_wait_ms +
            row.readback_ms +
            (row.gpu_timestamp_valid ? row.kernel_ms : 0.0);
        row.unaccounted_host_ms = row.total_wall_ms - accounted_ms;

        PrometheusSgemmResidentBenchmarkRequest resident_request{};
        resident_request.struct_size = sizeof(resident_request);
        resident_request.a = a.data();
        resident_request.b = b.data();
        resident_request.c = nullptr;
        resident_request.m = shape.m;
        resident_request.n = shape.n;
        resident_request.k = shape.k;
        resident_request.mode = PROM_SGEMM_RESIDENT_MODE_PRODUCTION_SELECTOR;
        resident_request.warmup_iterations = 2u;
        resident_request.iterations = 5u;
        resident_request.flags = 0u;

        PrometheusSgemmResidentBenchmarkResult resident_result{};
        resident_result.struct_size = sizeof(resident_result);
        const int resident_status =
            prometheus_reactor_runtime_sgemm_resident_benchmark(handle, &resident_request, &resident_result);
        row.resident_available =
            resident_status == PROM_OK &&
            resident_result.resident_mode_available != 0u &&
            resident_result.resident_mode_used != 0u &&
            resident_result.iterations > 0u;
        if (row.resident_available) {
            row.resident_kernel_ms = ns_to_ms(resident_result.kernel_median_ns);
            row.resident_loop_avg_ms =
                ns_to_ms(resident_result.total_loop_wall_ns) / static_cast<double>(resident_result.iterations);
            row.resident_vs_production_gap_ms = row.total_wall_ms - row.resident_loop_avg_ms;
        } else {
            row.note = row.note.empty() ? "resident_mode_unavailable_or_failed" : row.note + ",resident_mode_unavailable_or_failed";
        }

        return row;
    }

    std::string render_json(const DeepDiagnosticReport& report)
    {
        std::ostringstream out;
        out << "{\n";
        out << "  \"schema\": \"prometheus.sgemm.px16.deep_diagnostics.v1\",\n";
        out << "  \"timestamp_utc\": \"" << json_escape(report.timestamp_utc) << "\",\n";
        out << "  \"device\": {\n";
        out << "    \"name\": \"" << json_escape(report.device.name) << "\",\n";
        out << "    \"backend\": \"" << json_escape(report.device.backend) << "\",\n";
        out << "    \"device_type\": \"" << json_escape(report.device.device_type) << "\",\n";
        out << "    \"vendor_id\": " << report.device.vendor_id << ",\n";
        out << "    \"device_id\": " << report.device.device_id << ",\n";
        out << "    \"driver_version\": \"" << json_escape(report.device.driver_version) << "\",\n";
        out << "    \"api_version\": \"" << json_escape(report.device.api_version) << "\"\n";
        out << "  },\n";
        out << "  \"environment\": {\n";
        out << "    \"VK_INSTANCE_LAYERS\": \"" << json_escape(report.vk_instance_layers) << "\",\n";
        out << "    \"VK_LOADER_LAYERS_ENABLE\": \"" << json_escape(report.vk_loader_layers_enable) << "\"\n";
        out << "  },\n";
        out << "  \"global_note\": \"" << json_escape(report.global_note) << "\",\n";
        out << "  \"rows\": [\n";
        for (std::size_t index = 0u; index < report.rows.size(); ++index) {
            const DeepDiagnosticRow& row = report.rows[index];
            out << "    {\"shape\": \"" << json_escape(row.shape)
                << "\", \"m\": " << row.m
                << ", \"n\": " << row.n
                << ", \"k\": " << row.k
                << ", \"policy_mode\": \"" << json_escape(row.policy_mode)
                << "\", \"requested_variant\": \"" << json_escape(row.requested_variant)
                << "\", \"selected_variant\": \"" << json_escape(row.selected_variant)
                << "\", \"executed_variant\": \"" << json_escape(row.executed_variant)
                << "\", \"path\": \"" << json_escape(row.path)
                << "\", \"kernel_ms\": " << row.kernel_ms
                << ", \"upload_ms\": " << row.upload_ms
                << ", \"readback_ms\": " << row.readback_ms
                << ", \"sync_wait_ms\": " << row.sync_wait_ms
                << ", \"dispatch_submit_ms\": " << row.dispatch_submit_ms
                << ", \"command_record_ms\": " << row.command_record_ms
                << ", \"unaccounted_host_ms\": " << row.unaccounted_host_ms
                << ", \"gpu_timestamp_valid\": " << bool_json(row.gpu_timestamp_valid)
                << ", \"resident_available\": " << bool_json(row.resident_available)
                << ", \"resident_kernel_ms\": " << row.resident_kernel_ms
                << ", \"resident_loop_avg_ms\": " << row.resident_loop_avg_ms
                << ", \"resident_vs_production_gap_ms\": " << row.resident_vs_production_gap_ms
                << ", \"runtime_status\": " << row.runtime_status
                << ", \"final_stage\": " << row.final_stage
                << ", \"final_detail_code\": " << row.final_detail_code
                << ", \"note\": \"" << json_escape(row.note) << "\"}";
            if (index + 1u < report.rows.size()) {
                out << ",";
            }
            out << "\n";
        }
        out << "  ]\n";
        out << "}\n";
        return out.str();
    }

    std::string render_markdown(const DeepDiagnosticReport& report)
    {
        std::ostringstream out;
        out << "# Prometheus SGEMM Px16 Deep Diagnostics\n\n";
        out << "## Device\n\n";
        out << "- Device: " << report.device.name << "\n";
        out << "- Backend: " << report.device.backend << "\n";
        out << "- Device Type: " << report.device.device_type << "\n";
        out << "- Vendor ID: " << report.device.vendor_id << "\n";
        out << "- Device ID: " << report.device.device_id << "\n";
        out << "- Driver Version: " << report.device.driver_version << "\n";
        out << "- API Version: " << report.device.api_version << "\n";
        out << "- Timestamp: " << report.timestamp_utc << "\n\n";
        out << "## Environment\n\n";
        out << "- VK_INSTANCE_LAYERS: " << report.vk_instance_layers << "\n";
        out << "- VK_LOADER_LAYERS_ENABLE: " << report.vk_loader_layers_enable << "\n";
        out << "- Note: explicit env vars catch only the common layer-injection path. If `command_record_ms` does not explain the host gap, run `vulkaninfo --summary` and inspect unexpected Layers entries.\n\n";
        out << "## Timing Breakdown\n\n";
        out << "| shape | policy | requested | selected | executed | path | kernel ms | upload ms | readback ms | sync wait ms | dispatch submit ms | command record ms | unaccounted host ms | resident loop avg ms | resident-production gap ms | runtime |\n";
        out << "| --- | --- | --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n";
        for (const DeepDiagnosticRow& row : report.rows) {
            std::ostringstream runtime;
            runtime << row.runtime_status << " (" << row.final_stage << "/" << row.final_detail_code << ")";
            out << "| " << row.shape
                << " | " << row.policy_mode
                << " | " << row.requested_variant
                << " | " << row.selected_variant
                << " | " << row.executed_variant
                << " | " << row.path
                << " | " << row.kernel_ms
                << " | " << row.upload_ms
                << " | " << row.readback_ms
                << " | " << row.sync_wait_ms
                << " | " << row.dispatch_submit_ms
                << " | " << row.command_record_ms
                << " | " << row.unaccounted_host_ms
                << " | " << row.resident_loop_avg_ms
                << " | " << row.resident_vs_production_gap_ms
                << " | " << runtime.str() << " |\n";
        }
        out << "\n";
        if (!report.global_note.empty()) {
            out << "## Notes\n\n";
            out << report.global_note << "\n\n";
        }
        out << "Artifacts to upload for follow-up analysis:\n";
        out << "- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.json`\n";
        out << "- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.md`\n";
        return out.str();
    }
}

FACT(PrometheusSgemmPx16DeepDiagnostics)
{
    DeepDiagnosticReport report;
    report.timestamp_utc = timestamp_now_utc();
    report.vk_instance_layers = env_value_or_unset("VK_INSTANCE_LAYERS");
    report.vk_loader_layers_enable = env_value_or_unset("VK_LOADER_LAYERS_ENABLE");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (handle == nullptr) {
        return;
    }

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "runtime probe should succeed");
    if (caps.available == 0u) {
        report.global_note = "Vulkan runtime unavailable; deep diagnostics skipped.";
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_deep_diagnostics.json"), render_json(report)),
                    "deep diagnostic skip JSON should be written");
        ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_deep_diagnostics.md"), render_markdown(report)),
                    "deep diagnostic skip markdown should be written");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; deep diagnostics cannot execute");
    }

    (void)fetch_device_metadata(handle, caps, report.device);
    for (const ShapeCase& shape : kDeepShapes) {
        report.rows.push_back(run_shape_case(handle, shape));
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");

    report.global_note =
        "H1 instrumentation measures command buffer recording plus descriptor-set updates inside `command_record_ms`. "
        "`unaccounted_host_ms` is the production wall-clock remainder after subtracting upload, command-record, submit, wait, readback, and valid kernel GPU timing.";

    const std::string json = render_json(report);
    const std::string markdown = render_markdown(report);
    ASSERT_TRUE(json.find("\"command_record_ms\"") != std::string::npos, "deep diagnostic JSON should include command-record timing");
    ASSERT_TRUE(markdown.find("command record ms") != std::string::npos, "deep diagnostic markdown should include command-record timing");
    ASSERT_TRUE(json.find("\"VK_INSTANCE_LAYERS\"") != std::string::npos, "deep diagnostic JSON should include layer environment");
    ASSERT_TRUE(json.find("\"VK_LOADER_LAYERS_ENABLE\"") != std::string::npos, "deep diagnostic JSON should include loader layer environment");
    for (const DeepDiagnosticRow& row : report.rows) {
        if (row.runtime_status == PROM_OK) {
            ASSERT_TRUE(row.command_record_ms >= 0.0, "command-record timing should be non-negative");
        }
    }
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_deep_diagnostics.json"), json),
                "deep diagnostic JSON should be written");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_deep_diagnostics.md"), markdown),
                "deep diagnostic markdown should be written");
}
