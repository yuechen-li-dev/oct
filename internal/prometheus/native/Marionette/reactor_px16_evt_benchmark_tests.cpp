#include "../reactor_api.h"
#include "../reactor_dominatus_predictor.h"
#include "../reactor_judgment_engine.h"
#include "../reactor_policy_memory.h"
#include "test_harness.h"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstdlib>
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
        std::string status = "skip";
        std::string reference_mode = "none";
        float max_abs_error = 0.0f;
        float max_rel_error = 0.0f;
        float tolerance = kAbsTolerance;
    };

    struct TimingResult
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
        TimingResult timing;
        PrometheusSgemmPolicyDiagnostics diag{};
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

    struct ReportData
    {
        std::string schema = "prometheus.sgemm.px16.evt.v1";
        std::string timestamp_utc;
        std::string device_name = "unknown";
        std::uint32_t vendor_id = 0u;
        std::uint32_t device_id = 0u;
        std::string driver = "unknown";
        std::string benchmark_name;
        Summary summary;
        std::vector<CaseResult> cases;
        std::string global_skip_reason;
    };

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

    bool should_run_optional_large_case()
    {
#if defined(_WIN32)
        char* value = nullptr;
        std::size_t value_length = 0u;
        if (_dupenv_s(&value, &value_length, "OCT_PROMETHEUS_PX16_EVT_ENABLE_2048") != 0 || value == nullptr) {
            return false;
        }

        const bool enabled = std::string_view(value, value_length > 0u ? value_length - 1u : 0u) == "1";
        free(value);
        return enabled;
#else
        const char* value = std::getenv("OCT_PROMETHEUS_PX16_EVT_ENABLE_2048");
        return value != nullptr && std::string_view(value) == "1";
#endif
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

    void finalize_timing(const ShapeCase& shape,
                         const std::vector<double>& samples_ns,
                         std::string_view timing_source,
                         TimingResult& timing)
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

        const double flops = 2.0 * static_cast<double>(shape.m) * static_cast<double>(shape.n) * static_cast<double>(shape.k);
        const double median_seconds = sorted[sorted.size() / 2u] / 1.0e9;
        timing.gflops_median = median_seconds > 0.0 ? (flops / median_seconds) / 1.0e9 : 0.0;
    }

    bool is_large_shape(const ShapeCase& shape)
    {
        return shape.m >= 512u && shape.n >= 512u && shape.k >= 512u;
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
    }

    void detect_cross_case_anomalies(ReportData& report)
    {
        std::vector<CaseResult*> successful;
        for (CaseResult& result : report.cases) {
            if (!result.skipped && result.correctness.status == "pass" && result.timing.gflops_median > 0.0) {
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
            if (result.correctness.status == "pass" && result.runtime_status == PROM_OK) {
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

    CaseResult run_evt_case(void* handle, const ShapeCase& shape)
    {
        CaseResult result;
        result.name = shape.name;
        result.m = shape.m;
        result.n = shape.n;
        result.k = shape.k;
        result.optional = shape.optional;
        result.timing.warmup_iterations = warmup_iterations_for(shape);
        result.timing.iterations = measured_iterations_for(shape);

        if (shape.optional && !should_run_optional_large_case()) {
            result.skipped = true;
            result.skip_reason = "optional_large_case_disabled_for_local_repeatability";
            return result;
        }

        const bool use_dense_oracle = static_cast<std::uint64_t>(shape.m) * shape.n * shape.k <= 16ull * 1024ull * 1024ull;
        std::vector<float> a = use_dense_oracle
            ? dense_deterministic_matrix(shape.m, shape.k, shape.m ^ (shape.k << 4u))
            : separable_matrix_a(shape.m, shape.k);
        std::vector<float> b = use_dense_oracle
            ? dense_deterministic_matrix(shape.k, shape.n, shape.k ^ (shape.n << 5u) ^ 17u)
            : separable_matrix_b(shape.k, shape.n);
        std::vector<float> c(static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n), 0.0f);
        const std::vector<float> expected = use_dense_oracle
            ? cpu_oracle_dense(shape.m, shape.n, shape.k, a, b)
            : cpu_oracle_separable(shape.m, shape.n, shape.k);
        result.correctness.reference_mode = use_dense_oracle ? "dense_cpu_oracle" : "separable_rank1_oracle";

        std::uint32_t stage = PROM_STAGE_NONE;
        int detail_code = 0;

        for (std::uint32_t i = 0u; i < result.timing.warmup_iterations; ++i) {
            const int status = prometheus_reactor_runtime_sgemm(
                handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail_code);
            if (status != PROM_OK) {
                result.runtime_status = status;
                result.correctness.status = "fail";
                result.anomalies.push_back("warmup_call_failed");
                result.final_stage = stage;
                result.final_detail_code = detail_code;
                return result;
            }
        }

        std::vector<double> cpu_samples_ns;
        std::vector<double> gpu_samples_ns;
        cpu_samples_ns.reserve(result.timing.iterations);
        gpu_samples_ns.reserve(result.timing.iterations);

        for (std::uint32_t i = 0u; i < result.timing.iterations; ++i) {
            const auto begin = std::chrono::steady_clock::now();
            const int status = prometheus_reactor_runtime_sgemm(
                handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, &stage, &detail_code);
            const auto end = std::chrono::steady_clock::now();
            cpu_samples_ns.push_back(static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(end - begin).count()));

            if (status != PROM_OK) {
                result.runtime_status = status;
                result.correctness.status = "fail";
                result.anomalies.push_back("measured_call_failed");
                result.final_stage = stage;
                result.final_detail_code = detail_code;
                return result;
            }

            if (prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &result.diag) != PROM_OK) {
                result.runtime_status = PROM_ERROR;
                result.correctness.status = "fail";
                result.anomalies.push_back("policy_diagnostics_query_failed");
                result.final_stage = stage;
                result.final_detail_code = detail_code;
                return result;
            }

            if (result.diag.p13_m5_last_gpu_timing_valid != 0u && result.diag.p13_m5_last_gpu_duration_ns > 0u) {
                gpu_samples_ns.push_back(static_cast<double>(result.diag.p13_m5_last_gpu_duration_ns));
            }
        }

        result.final_stage = stage;
        result.final_detail_code = detail_code;
        result.correctness = compare_with_tolerance(expected, c);
        result.correctness.reference_mode = use_dense_oracle ? "dense_cpu_oracle" : "separable_rank1_oracle";

        if (gpu_samples_ns.size() == cpu_samples_ns.size() && !gpu_samples_ns.empty()) {
            finalize_timing(shape, gpu_samples_ns, "vulkan_timestamp_query", result.timing);
        } else {
            finalize_timing(shape, cpu_samples_ns, "cpu_wall_clock", result.timing);
        }

        detect_case_anomalies(result);
        if (result.diag.p13_m5_timestamp_available == 0u) {
            result.anomalies.push_back("gpu_timing_unavailable_using_cpu_wall_clock");
        }

        return result;
    }

    std::string render_json(const ReportData& report)
    {
        std::ostringstream out;
        out << "{\n";
        out << "  \"schema\": \"" << json_escape(report.schema) << "\",\n";
        out << "  \"device\": {\n";
        out << "    \"name\": \"" << json_escape(report.device_name) << "\",\n";
        out << "    \"vendor_id\": " << report.vendor_id << ",\n";
        out << "    \"device_id\": " << report.device_id << ",\n";
        out << "    \"driver\": \"" << json_escape(report.driver) << "\",\n";
        out << "    \"timestamp\": \"" << json_escape(report.timestamp_utc) << "\"\n";
        out << "  },\n";
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
            out << "      \"path\": {\n";
            out << "        \"requested\": \"" << path_name(result.diag.px16_m6_requested_path) << "\",\n";
            out << "        \"selected\": \"" << path_name(result.diag.px16_m6_selected_path) << "\",\n";
            out << "        \"executed\": \"" << path_name(result.diag.px16_m6_executed_path) << "\"\n";
            out << "      },\n";
            out << "      \"compute_mode\": {\n";
            out << "        \"requested\": \"" << compute_mode_name(result.diag.px16_m6_requested_compute_mode) << "\",\n";
            out << "        \"selected\": \"" << compute_mode_name(result.diag.px16_m6_selected_compute_mode) << "\",\n";
            out << "        \"executed\": \"" << compute_mode_name(result.diag.px16_m6_executed_compute_mode) << "\"\n";
            out << "      },\n";
            out << "      \"variant\": {\n";
            out << "        \"recommended\": \"" << occupancy_variant_name(result.diag.px16_m6_selector_recommended_variant) << "\",\n";
            out << "        \"selected\": \"" << occupancy_variant_name(result.diag.px16_m6_selector_selected_variant) << "\",\n";
            out << "        \"requested\": \"" << occupancy_variant_name(result.diag.px16_m6_requested_dispatch_variant) << "\",\n";
            out << "        \"executed\": \"" << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant) << "\",\n";
            out << "        \"path_status\": \"" << variant_path_status_name(result.diag.px16_m6_variant_path_status) << "\",\n";
            out << "        \"production_eligible\": " << bool_json(result.diag.px16_m6_variant_production_eligible != 0u) << ",\n";
            out << "        \"dispatch_enabled\": " << bool_json(result.diag.px16_m6_variant_dispatch_enabled != 0u) << ",\n";
            out << "        \"dvt_validated\": " << bool_json(result.diag.px16_m6_variant_dvt_validated != 0u) << ",\n";
            out << "        \"pvt_validated\": " << bool_json(result.diag.px16_m6_variant_pvt_validated != 0u) << ",\n";
            out << "        \"lifecycle_telemetry_only\": " << bool_json(result.diag.px16_m6_variant_lifecycle_telemetry_only != 0u) << "\n";
            out << "      },\n";
            out << "      \"force_direct\": {\n";
            out << "        \"requested\": " << bool_json(result.diag.px16_m6_force_direct_requested != 0u) << ",\n";
            out << "        \"applied\": " << bool_json(result.diag.px16_m6_force_direct_applied != 0u) << ",\n";
            out << "        \"reason\": \"" << force_direct_reason_name(result.diag.px16_m6_force_direct_reason) << "\"\n";
            out << "      },\n";
            out << "      \"p15\": {\n";
            out << "        \"reservation_present\": " << bool_json(result.diag.px16_m6_p15_reservation_present != 0u) << ",\n";
            out << "        \"reservation_matured\": " << bool_json(result.diag.px16_m6_p15_reservation_matured != 0u) << ",\n";
            out << "        \"reservation_consumed\": " << bool_json(result.diag.px16_m6_p15_reservation_consumed != 0u) << ",\n";
            out << "        \"reserved_variant\": \"" << occupancy_variant_name(result.diag.px16_m6_p15_reserved_variant_id) << "\",\n";
            out << "        \"live_selected_variant\": \"" << occupancy_variant_name(result.diag.px16_m6_p15_live_selected_variant_id) << "\",\n";
            out << "        \"match\": " << bool_json(result.diag.px16_m6_p15_reconciliation_match != 0u) << ",\n";
            out << "        \"block_reason\": \"" << p15_block_reason_name(result.diag.px16_m6_p15_block_reason) << "\",\n";
            out << "        \"correction_action\": \"" << p15_correction_action_name(result.diag.px16_m6_p15_correction_action) << "\",\n";
            out << "        \"reservation_stale_or_expired\": " << bool_json(result.diag.px16_m6_p15_reservation_stale_or_expired != 0u) << ",\n";
            out << "        \"confidence_before\": " << result.diag.px16_m6_p15_confidence_before << ",\n";
            out << "        \"confidence_after\": " << result.diag.px16_m6_p15_confidence_after << "\n";
            out << "      },\n";
            out << "      \"correctness\": {\n";
            out << "        \"status\": \"" << json_escape(result.correctness.status) << "\",\n";
            out << "        \"reference_mode\": \"" << json_escape(result.correctness.reference_mode) << "\",\n";
            out << "        \"max_abs_error\": " << result.correctness.max_abs_error << ",\n";
            out << "        \"max_rel_error\": " << result.correctness.max_rel_error << ",\n";
            out << "        \"tolerance\": " << result.correctness.tolerance << "\n";
            out << "      },\n";
            out << "      \"timing\": {\n";
            out << "        \"warmup_iterations\": " << result.timing.warmup_iterations << ",\n";
            out << "        \"iterations\": " << result.timing.iterations << ",\n";
            out << "        \"min_ms\": " << result.timing.min_ms << ",\n";
            out << "        \"median_ms\": " << result.timing.median_ms << ",\n";
            out << "        \"p95_ms\": " << result.timing.p95_ms << ",\n";
            out << "        \"gflops_median\": " << result.timing.gflops_median << ",\n";
            out << "        \"timing_source\": \"" << json_escape(result.timing.timing_source) << "\",\n";
            out << "        \"gpu_timing_failure_reason\": \"" << timing_failure_reason_name(result.diag.p13_m5_last_gpu_timing_failure_reason) << "\"\n";
            out << "      },\n";
            out << "      \"runtime\": {\n";
            out << "        \"status\": " << result.runtime_status << ",\n";
            out << "        \"final_stage\": " << result.final_stage << ",\n";
            out << "        \"final_detail_code\": " << result.final_detail_code << "\n";
            out << "      },\n";
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

        out << "  ]\n";
        out << "}\n";
        return out.str();
    }

    std::string render_markdown(const ReportData& report)
    {
        std::ostringstream out;
        out << "# Prometheus SGEMM Px16 EVT Report\n\n";
        out << "## Device / Runtime\n\n";
        out << "- Device: " << report.device_name << "\n";
        out << "- Vendor ID: " << report.vendor_id << "\n";
        out << "- Device ID: " << report.device_id << "\n";
        out << "- Driver: " << report.driver << "\n";
        out << "- Timestamp: " << report.timestamp_utc << "\n";
        out << "- Benchmark: " << report.benchmark_name << "\n\n";

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

        out << "## Production SGEMM Results\n\n";
        out << "| shape | policy | path | selected variant | executed variant | median ms | GFLOP/s | correctness | anomalies |\n";
        out << "| --- | --- | --- | --- | --- | ---: | ---: | --- | --- |\n";
        for (const CaseResult& result : report.cases) {
            const std::string path_summary =
                path_name(result.diag.px16_m6_requested_path) + " -> " +
                path_name(result.diag.px16_m6_selected_path) + " -> " +
                path_name(result.diag.px16_m6_executed_path);
            const std::string correctness_summary = result.skipped
                ? "skip"
                : result.correctness.status + " (" + result.correctness.reference_mode + ")";
            std::string anomaly_summary = result.anomalies.empty() ? "none" : result.anomalies.front();
            for (std::size_t i = 1u; i < result.anomalies.size(); ++i) {
                anomaly_summary += ", " + result.anomalies[i];
            }
            if (result.skipped && !result.skip_reason.empty()) {
                anomaly_summary = "skip: " + result.skip_reason;
            }
            out << "| " << result.name
                << " | " << policy_mode_name(result.diag.px16_m6_policy_mode)
                << " | " << path_summary
                << " | " << occupancy_variant_name(result.diag.px16_m6_selector_selected_variant)
                << " | " << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant)
                << " | " << result.timing.median_ms
                << " | " << result.timing.gflops_median
                << " | " << correctness_summary
                << " | " << anomaly_summary << " |\n";
        }
        out << "\n";

        out << "## Variant Distribution\n\n";
        for (const CaseResult& result : report.cases) {
            out << "- " << result.name
                << ": selected=" << occupancy_variant_name(result.diag.px16_m6_selector_selected_variant)
                << ", requested=" << occupancy_variant_name(result.diag.px16_m6_requested_dispatch_variant)
                << ", executed=" << occupancy_variant_name(result.diag.px16_m6_executed_dispatch_variant)
                << ", path_status=" << variant_path_status_name(result.diag.px16_m6_variant_path_status)
                << "\n";
        }
        out << "\n";

        out << "## P15 Reconciliation Summary\n\n";
        for (const CaseResult& result : report.cases) {
            out << "- " << result.name
                << ": reservation_present=" << (result.diag.px16_m6_p15_reservation_present != 0u ? "true" : "false")
                << ", reserved=" << occupancy_variant_name(result.diag.px16_m6_p15_reserved_variant_id)
                << ", live=" << occupancy_variant_name(result.diag.px16_m6_p15_live_selected_variant_id)
                << ", match=" << (result.diag.px16_m6_p15_reconciliation_match != 0u ? "true" : "false")
                << ", block_reason=" << p15_block_reason_name(result.diag.px16_m6_p15_block_reason)
                << ", correction_action=" << p15_correction_action_name(result.diag.px16_m6_p15_correction_action)
                << "\n";
        }
        out << "\n";

        out << "## Anomalies / Suspected Performance Blockers\n\n";
        bool wrote_anomaly = false;
        for (const CaseResult& result : report.cases) {
            for (const std::string& anomaly : result.anomalies) {
                wrote_anomaly = true;
                out << "- " << result.name << ": " << anomaly << "\n";
            }
        }
        if (!wrote_anomaly) {
            out << "- none\n";
        }
        out << "\n";

        out << "## Optional Explicit Variant Comparison\n\n";
        out << "Not run in this lane. Production selector-controlled dispatch is the primary EVT measurement path for Px16 M7.\n\n";

        out << "## Notes\n\n";
        out << "- Main results use `prometheus_reactor_runtime_sgemm(...)` rather than explicit benchmark-variant dispatch.\n";
        out << "- Timing source is reported per case. GPU timestamps win when valid for every measured iteration; otherwise CPU wall-clock is reported.\n";
        out << "- Large-shape correctness uses a deterministic separable reference so the EVT lane stays locally repeatable without changing tolerance policy.\n";
        out << "- Generated artifacts belong under `out/test-artifacts/` and should not be committed by default.\n";
        return out.str();
    }

    ReportData build_synthetic_report()
    {
        ReportData report;
        report.timestamp_utc = "2026-07-08T00:00:00Z";
        report.benchmark_name = "synthetic";

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
        result.timing.timing_source = "cpu_wall_clock";
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
        finalize_summary(report);
        return report;
    }
}

FACT(PrometheusSgemmPx16Evt_ArtifactWritersEmitSchemaAndCaseRows)
{
    const ReportData report = build_synthetic_report();
    const std::string json = render_json(report);
    const std::string markdown = render_markdown(report);

    ASSERT_TRUE(json.find("\"schema\": \"prometheus.sgemm.px16.evt.v1\"") != std::string::npos, "JSON artifact should include the EVT schema");
    ASSERT_TRUE(json.find("\"anomalies\": [") != std::string::npos, "JSON artifact should include anomalies arrays");
    ASSERT_TRUE(json.find("\"name\": \"square_512x512x512\"") != std::string::npos, "JSON artifact should include at least one case row");
    ASSERT_TRUE(markdown.find("# Prometheus SGEMM Px16 EVT Report") != std::string::npos, "Markdown artifact should include the report heading");
    ASSERT_TRUE(markdown.find("| square_512x512x512 |") != std::string::npos, "Markdown artifact should include at least one case row");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_artifact_writer_smoke.json"), json),
                "artifact writer smoke JSON should be created");
    ASSERT_TRUE(context.WriteArtifactFile(std::filesystem::path("prometheus_sgemm_px16_evt_artifact_writer_smoke.md"), markdown),
                "artifact writer smoke Markdown should be created");
}

VALIDATED_BENCHMARK_WITH_ITERATIONS(PrometheusSgemmPx16Evt_ProductionArtifactLane, 1)
{
    ReportData report;
    report.timestamp_utc = timestamp_now_utc();
    report.benchmark_name = context.BenchmarkName();

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

    for (const ShapeCase& shape : evt_shapes()) {
        report.cases.push_back(run_evt_case(handle, shape));
    }

    detect_cross_case_anomalies(report);
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
        if (result.correctness.status != "pass") {
            context.RecordFailure(
                __FILE__,
                __LINE__,
                "PX16_EVT_CORRECTNESS",
                "EVT case correctness failed",
                result.name,
                result.correctness.status);
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
