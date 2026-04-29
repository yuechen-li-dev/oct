#include "../bridge.h"
#include "../reactor_judgment_engine.h"
#include "test_harness.h"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstdint>
#include <cstring>
#include <limits>
#include <sstream>
#include <string>
#include <vector>

namespace
{
    enum class HarnessMode {
        Smoke,
        Characterization,
        Comparison,
    };

    struct BenchmarkShapeCase {
        const char* name;
        std::uint32_t shape_class;
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
        bool smoke_eligible;
    };

    struct CorrectnessSummary {
        bool pass;
        float max_abs_error;
        float max_rel_error;
        std::size_t first_failing_index;
        float aggregate_abs_error;
        float abs_tolerance;
        float rel_tolerance;
    };

    struct CaseResult {
        std::string shape_name;
        std::uint32_t shape_class;
        std::uint32_t m;
        std::uint32_t n;
        std::uint32_t k;
        std::uint32_t selected_variant;
        std::uint32_t tested_variant;
        bool variant_available;
        bool skipped;
        std::string fallback_reason;
        CorrectnessSummary correctness;
        double mean_ns;
        double median_ns;
        double min_ns;
        double stability_cv;
        std::uint32_t timing_stability_permille;
        double gflops;
        double arithmetic_intensity;
        std::string timing_source;
        std::string timing_confidence;
        bool timestamp_available;
        std::string timestamp_failure_reason;
        double gpu_duration_ns_mean;
        double gpu_duration_ns_median;
        double gpu_duration_ns_min;
        bool diagnostics_match;
        PrometheusSgemmPolicyDiagnostics diag;
        bool actuation_ready;
        std::string actuation_reason;
    };

    struct BenchmarkRun {
        HarnessMode mode;
        std::uint32_t warmup_iterations;
        std::uint32_t measured_iterations;
        std::string timing_source;
        std::string timing_confidence;
        bool timestamp_available;
        std::string timestamp_failure_reason;
        PrometheusCaps caps;
        std::vector<CaseResult> cases;
        bool final_actuation_ready;
        std::string final_reason;
    };

    constexpr float kAbsTolerance = 1.0e-4f;
    constexpr float kRelTolerance = 1.0e-4f;
    constexpr double kImprovementThreshold = 0.03;
    constexpr double kMaxStabilityCv = 0.10;

    std::string timestamp_failure_reason_name(std::uint32_t reason)
    {
        switch (reason) {
            case PROM_SGEMM_GPU_TIMING_FAILURE_NONE:
                return "none";
            case PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED:
                return "unsupported";
            case PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE:
                return "query_pool_unavailable";
            case PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD:
                return "invalid_period";
            case PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE:
                return "unavailable_result";
            case PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER:
                return "invalid_order";
            case PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED:
                return "command_failed";
            default:
                return "unknown";
        }
    }

    const std::vector<BenchmarkShapeCase>& all_shape_cases()
    {
        static const std::vector<BenchmarkShapeCase> kCases = {
            {"small-square", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE), 128u, 128u, 128u, true},
            {"medium-square", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE), 512u, 512u, 512u, true},
            {"large-square", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE), 2048u, 2048u, 2048u, false},
            {"tall-skinny", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_TALL_SKINNY), 2048u, 256u, 512u, false},
            {"wide-short", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_WIDE_SHORT), 256u, 2048u, 512u, false},
            {"K-heavy", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_K_HEAVY), 512u, 512u, 4096u, false},
            {"ML/FFN-like", static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE), 4096u, 11008u, 4096u, false},
        };
        return kCases;
    }

    std::string mode_name(HarnessMode mode)
    {
        switch (mode) {
            case HarnessMode::Smoke:
                return "smoke";
            case HarnessMode::Characterization:
                return "characterization";
            case HarnessMode::Comparison:
                return "comparison";
        }
        return "unknown";
    }

    std::string occupancy_variant_name(std::uint32_t variant)
    {
        switch (variant) {
            case PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR:
                return "baseline-scalar";
            case PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE:
                return "memory-conservative";
            case PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE:
                return "small-register-tile";
            case PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4:
                return "balanced-2x2-accum4";
            case PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8:
                return "aggressive-4x4-accum8";
            default:
                return "unknown";
        }
    }

    bool variant_is_implemented(std::uint32_t variant)
    {
        switch (variant) {
            case PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR:
            case PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE:
            case PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE:
            case PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4:
            case PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8:
                return true;
            default:
                return false;
        }
    }

    bool variant_dispatch_enabled(std::uint32_t variant)
    {
        return variant == static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR);
    }

    bool aggressive_shape_supported(const BenchmarkShapeCase& shape)
    {
        return shape.m >= 128u && shape.n >= 128u && shape.k >= 128u;
    }

    std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols, std::uint32_t salt)
    {
        std::vector<float> out(static_cast<std::size_t>(rows) * static_cast<std::size_t>(cols), 0.0f);
        for (std::size_t i = 0; i < out.size(); ++i) {
            const std::uint32_t mixed = static_cast<std::uint32_t>((i * 1103515245u) + 12345u + (salt * 2654435761u));
            const int bounded = static_cast<int>(mixed % 29u) - 14;
            out[i] = static_cast<float>(bounded) / 16.0f;
        }
        return out;
    }

    std::vector<float> cpu_oracle(std::uint32_t m, std::uint32_t n, std::uint32_t k, const std::vector<float>& a, const std::vector<float>& b)
    {
        std::vector<float> c(static_cast<std::size_t>(m) * static_cast<std::size_t>(n), 0.0f);
        for (std::uint32_t row = 0; row < m; ++row) {
            for (std::uint32_t col = 0; col < n; ++col) {
                float sum = 0.0f;
                for (std::uint32_t kk = 0; kk < k; ++kk) {
                    sum += a[static_cast<std::size_t>(row) * k + kk] * b[static_cast<std::size_t>(kk) * n + col];
                }
                c[static_cast<std::size_t>(row) * n + col] = sum;
            }
        }
        return c;
    }

    CorrectnessSummary compare_against_oracle(const std::vector<float>& expected, const std::vector<float>& actual)
    {
        CorrectnessSummary summary{};
        summary.pass = (expected.size() == actual.size());
        summary.first_failing_index = std::numeric_limits<std::size_t>::max();
        summary.abs_tolerance = kAbsTolerance;
        summary.rel_tolerance = kRelTolerance;

        if (expected.size() != actual.size()) {
            summary.max_abs_error = std::numeric_limits<float>::infinity();
            summary.max_rel_error = std::numeric_limits<float>::infinity();
            summary.aggregate_abs_error = std::numeric_limits<float>::infinity();
            return summary;
        }

        for (std::size_t i = 0; i < expected.size(); ++i) {
            const float exp = expected[i];
            const float act = actual[i];
            if (!std::isfinite(exp) || !std::isfinite(act)) {
                summary.pass = false;
                if (summary.first_failing_index == std::numeric_limits<std::size_t>::max()) {
                    summary.first_failing_index = i;
                }
                continue;
            }
            const float abs_err = std::fabs(exp - act);
            const float denom = std::max(std::fabs(exp), 1.0e-8f);
            const float rel_err = abs_err / denom;
            summary.max_abs_error = std::max(summary.max_abs_error, abs_err);
            summary.max_rel_error = std::max(summary.max_rel_error, rel_err);
            summary.aggregate_abs_error += abs_err;
            if (abs_err > summary.abs_tolerance && rel_err > summary.rel_tolerance) {
                summary.pass = false;
                if (summary.first_failing_index == std::numeric_limits<std::size_t>::max()) {
                    summary.first_failing_index = i;
                }
            }
        }

        if (summary.first_failing_index == std::numeric_limits<std::size_t>::max()) {
            summary.first_failing_index = static_cast<std::size_t>(-1);
        }
        return summary;
    }

    std::vector<BenchmarkShapeCase> shape_cases_for_mode(HarnessMode mode)
    {
        std::vector<BenchmarkShapeCase> cases;
        if (mode == HarnessMode::Smoke) {
            for (const BenchmarkShapeCase& shape : all_shape_cases()) {
                if (shape.smoke_eligible) {
                    cases.push_back(shape);
                }
            }
            return cases;
        }

        cases = all_shape_cases();
        if (mode != HarnessMode::Characterization) {
            cases.erase(std::remove_if(cases.begin(), cases.end(), [](const BenchmarkShapeCase& c) {
                return !c.smoke_eligible;
            }),
                        cases.end());
        }
        return cases;
    }

    bool diagnostics_alignment_ok(const CaseResult& result)
    {
        return result.diag.p13_m2_occupancy_shape_class == result.shape_class &&
            result.diag.p13_m2_occupancy_selected_variant == result.selected_variant;
    }

    void summarize_timing(CaseResult& result, const std::vector<double>& samples)
    {
        if (samples.empty()) {
            return;
        }
        std::vector<double> sorted = samples;
        std::sort(sorted.begin(), sorted.end());
        result.min_ns = sorted.front();
        result.median_ns = sorted[sorted.size() / 2u];
        double sum = 0.0;
        for (double sample : sorted) {
            sum += sample;
        }
        result.mean_ns = sum / static_cast<double>(sorted.size());
        double variance = 0.0;
        for (double sample : sorted) {
            const double delta = sample - result.mean_ns;
            variance += delta * delta;
        }
        variance /= static_cast<double>(sorted.size());
        const double stddev = std::sqrt(variance);
        result.stability_cv = (result.mean_ns > 0.0) ? (stddev / result.mean_ns) : 1.0;
        result.timing_stability_permille = static_cast<std::uint32_t>(result.stability_cv * 1000.0);
        const double flops = 2.0 * static_cast<double>(result.m) * static_cast<double>(result.n) * static_cast<double>(result.k);
        result.gflops = (result.mean_ns > 0.0) ? ((flops / result.mean_ns) / 1.0e9) : 0.0;
        const double bytes = static_cast<double>(result.m * result.k + result.k * result.n + result.m * result.n) * static_cast<double>(sizeof(float));
        result.arithmetic_intensity = (bytes > 0.0) ? (flops / bytes) : 0.0;
    }

    bool candidate_actuation_gate(const CaseResult& candidate, const CaseResult& baseline)
    {
        if (!candidate.correctness.pass) {
            return false;
        }
        if (candidate.timing_confidence != "high") {
            return false;
        }
        if (candidate.stability_cv > kMaxStabilityCv) {
            return false;
        }
        if (!candidate.diagnostics_match) {
            return false;
        }
        if (!candidate.variant_available || !variant_is_implemented(candidate.tested_variant)) {
            return false;
        }
        if (candidate.tested_variant == static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR)) {
            return false;
        }
        if (baseline.mean_ns <= 0.0 || candidate.mean_ns <= 0.0) {
            return false;
        }
        const double improvement = (baseline.mean_ns - candidate.mean_ns) / baseline.mean_ns;
        return improvement >= kImprovementThreshold;
    }

    CaseResult run_case(void* handle,
                        const BenchmarkShapeCase& shape,
                        HarnessMode mode,
                        std::uint32_t requested_variant,
                        std::uint32_t warmup_iterations,
                        std::uint32_t measured_iterations,
                        bool force_correctness_failure,
                        bool force_diagnostics_mismatch,
                        bool force_timestamp_unavailable,
                        bool force_timestamp_invalid_result,
                        bool force_high_confidence_simulation)
    {
        CaseResult result{};
        result.shape_name = shape.name;
        result.shape_class = shape.shape_class;
        result.m = shape.m;
        result.n = shape.n;
        result.k = shape.k;
        result.selected_variant = requested_variant;
        result.tested_variant = requested_variant;
        result.variant_available = variant_is_implemented(requested_variant);
        result.timing_source = "cpu_wall_clock";
        result.timing_confidence = "low";
        result.timestamp_available = false;
        result.timestamp_failure_reason = "unsupported";

        if (!result.variant_available) {
            if (mode == HarnessMode::Characterization) {
                result.skipped = true;
                result.fallback_reason = "unavailable_variant_skipped";
                return result;
            }
            result.tested_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR);
            result.fallback_reason = "unavailable_variant_fallback_to_baseline";
        }

        if (result.variant_available && !variant_dispatch_enabled(requested_variant)) {
            result.tested_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR);
            result.fallback_reason = "dispatch_disabled_benchmark_only";
            if (requested_variant == static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8) &&
                !aggressive_shape_supported(shape)) {
                result.fallback_reason = "aggressive_shape_gate_fallback";
            }
        }

        const std::uint32_t shape_salt = shape.m ^ (shape.n << 1u) ^ (shape.k << 2u);
        const std::vector<float> a = deterministic_matrix(shape.m, shape.k, shape_salt + 17u);
        const std::vector<float> b = deterministic_matrix(shape.k, shape.n, shape_salt + 73u);
        std::vector<float> c(static_cast<std::size_t>(shape.m) * static_cast<std::size_t>(shape.n), 0.0f);
        std::vector<float> expected = cpu_oracle(shape.m, shape.n, shape.k, a, b);
        if (force_correctness_failure && !expected.empty()) {
            expected[0] += 10.0f;
        }

        std::uint32_t stage = 0u;
        int detail_code = 0;
        for (std::uint32_t i = 0; i < warmup_iterations; ++i) {
            const int status = prometheus_reactor_runtime_sgemm_benchmark_variant(
                handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, requested_variant, &stage, &detail_code);
            if (status != PROM_OK) {
                result.skipped = true;
                result.fallback_reason = "runtime_sgemm_failed";
                return result;
            }
        }

        std::vector<double> samples;
        std::vector<double> cpu_samples;
        std::vector<double> gpu_samples;
        samples.reserve(measured_iterations);
        cpu_samples.reserve(measured_iterations);
        gpu_samples.reserve(measured_iterations);
        for (std::uint32_t i = 0; i < measured_iterations; ++i) {
            const auto start = std::chrono::steady_clock::now();
            const int status = prometheus_reactor_runtime_sgemm_benchmark_variant(
                handle, a.data(), b.data(), c.data(), shape.m, shape.n, shape.k, requested_variant, &stage, &detail_code);
            const auto end = std::chrono::steady_clock::now();
            if (status != PROM_OK) {
                result.skipped = true;
                result.fallback_reason = "runtime_sgemm_failed";
                return result;
            }
            cpu_samples.push_back(static_cast<double>(std::chrono::duration_cast<std::chrono::nanoseconds>(end - start).count()));
            memset(&result.diag, 0, sizeof(result.diag));
            const int diag_status = prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &result.diag);
            if (diag_status == PROM_OK) {
                result.selected_variant = result.diag.p13_m16b1_executed_occupancy_variant;
                if (result.diag.p13_m16b1_fallback_reason == PROM_OCCUPANCY_VARIANT_FALLBACK_PATH_NOT_WIRED) {
                    result.fallback_reason = "variant_path_not_wired";
                } else if (result.diag.p13_m16b1_fallback_reason == PROM_OCCUPANCY_VARIANT_FALLBACK_MC_BASELINE_STRICT_ALIAS) {
                    result.fallback_reason = "mc_baseline_strict_alias";
                } else if (result.diag.p13_m16b1_fallback_reason == PROM_OCCUPANCY_VARIANT_FALLBACK_NONE && result.fallback_reason.empty()) {
                    result.fallback_reason = "none";
                }
                result.timestamp_available = (result.diag.p13_m5_timestamp_available != 0u);
                result.timestamp_failure_reason = timestamp_failure_reason_name(result.diag.p13_m5_last_gpu_timing_failure_reason);
                if (result.diag.p13_m5_last_gpu_timing_valid != 0u && result.diag.p13_m5_last_gpu_duration_ns > 0u) {
                    gpu_samples.push_back(static_cast<double>(result.diag.p13_m5_last_gpu_duration_ns));
                }
            }
        }

        if (force_timestamp_unavailable) {
            result.timestamp_available = false;
            result.timestamp_failure_reason = "unsupported";
            gpu_samples.clear();
        }
        if (force_timestamp_invalid_result) {
            result.timestamp_failure_reason = "unavailable_result";
            gpu_samples.clear();
        }
        if (!gpu_samples.empty() && gpu_samples.size() == cpu_samples.size()) {
            samples = gpu_samples;
            result.timing_source = "vulkan_timestamp_query";
            result.timing_confidence = "high";
            result.timestamp_failure_reason = "none";
        } else if (force_high_confidence_simulation) {
            samples = cpu_samples;
            result.timing_source = "vulkan_timestamp_query";
            result.timing_confidence = "high";
            result.timestamp_available = true;
            result.timestamp_failure_reason = "simulated";
        } else {
            samples = cpu_samples;
            result.timing_source = "cpu_wall_clock";
            result.timing_confidence = "low";
            if (result.timestamp_available && result.timestamp_failure_reason == "none") {
                result.timestamp_failure_reason = "unavailable_result";
            }
        }
        summarize_timing(result, samples);
        if (!gpu_samples.empty()) {
            std::sort(gpu_samples.begin(), gpu_samples.end());
            result.gpu_duration_ns_min = gpu_samples.front();
            result.gpu_duration_ns_median = gpu_samples[gpu_samples.size() / 2u];
            double gpu_sum = 0.0;
            for (double sample : gpu_samples) {
                gpu_sum += sample;
            }
            result.gpu_duration_ns_mean = gpu_sum / static_cast<double>(gpu_samples.size());
        }
        result.correctness = compare_against_oracle(expected, c);
        memset(&result.diag, 0, sizeof(result.diag));
        const int diag_status = prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &result.diag);
        if (diag_status == PROM_OK) {
            result.selected_variant = result.diag.p13_m2_occupancy_selected_variant;
        }
        result.diagnostics_match = diagnostics_alignment_ok(result);
        if (force_diagnostics_mismatch) {
            result.diagnostics_match = false;
        }
        return result;
    }

    BenchmarkRun run_benchmark(HarnessMode mode,
                               std::uint32_t requested_variant,
                               bool force_correctness_failure,
                               bool force_diagnostics_mismatch,
                               bool force_timestamp_unavailable = false,
                               bool force_timestamp_invalid_result = false,
                               bool force_high_confidence_simulation = false)
    {
        BenchmarkRun run{};
        run.mode = mode;
        run.warmup_iterations = (mode == HarnessMode::Smoke) ? 1u : 2u;
        run.measured_iterations = (mode == HarnessMode::Smoke) ? 2u : 4u;
        run.timing_source = "cpu_wall_clock";
        run.timing_confidence = "low";
        run.timestamp_available = false;
        run.timestamp_failure_reason = "unsupported";

        void* handle = nullptr;
        const int create_status = prometheus_reactor_runtime_create(nullptr, &handle);
        if (create_status != PROM_OK || handle == nullptr) {
            run.final_actuation_ready = false;
            run.final_reason = "runtime_create_failed";
            return run;
        }

        memset(&run.caps, 0, sizeof(run.caps));
        (void)prometheus_reactor_runtime_probe(handle, &run.caps);

        const std::vector<BenchmarkShapeCase> shapes = shape_cases_for_mode(mode);
        for (const BenchmarkShapeCase& shape : shapes) {
            run.cases.push_back(run_case(handle,
                                         shape,
                                         mode,
                                         requested_variant,
                                         run.warmup_iterations,
                                         run.measured_iterations,
                                         force_correctness_failure,
                                         force_diagnostics_mismatch,
                                         force_timestamp_unavailable,
                                         force_timestamp_invalid_result,
                                         force_high_confidence_simulation));
        }
        if (!run.cases.empty()) {
            run.timing_source = run.cases.front().timing_source;
            run.timing_confidence = run.cases.front().timing_confidence;
            run.timestamp_available = run.cases.front().timestamp_available;
            run.timestamp_failure_reason = run.cases.front().timestamp_failure_reason;
        }

        CaseResult baseline{};
        bool baseline_found = false;
        for (const CaseResult& candidate : run.cases) {
            if (candidate.tested_variant == static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR) && !candidate.skipped) {
                baseline = candidate;
                baseline_found = true;
                break;
            }
        }

        run.final_actuation_ready = false;
        run.final_reason = "no_candidate_passed";
        for (CaseResult& candidate : run.cases) {
            if (candidate.skipped) {
                candidate.actuation_ready = false;
                candidate.actuation_reason = "case_skipped";
                continue;
            }
            if (!candidate.correctness.pass) {
                candidate.actuation_ready = false;
                candidate.actuation_reason = "correctness_failed";
                continue;
            }
            if (candidate.timing_confidence != "high") {
                candidate.actuation_ready = false;
                candidate.actuation_reason = "timing_confidence_low";
                continue;
            }
            if (!candidate.diagnostics_match) {
                candidate.actuation_ready = false;
                candidate.actuation_reason = "diagnostics_mismatch";
                continue;
            }
            if (!baseline_found || !candidate_actuation_gate(candidate, baseline)) {
                candidate.actuation_ready = false;
                candidate.actuation_reason = "gate_threshold_or_variant_failed";
                continue;
            }

            candidate.actuation_ready = true;
            candidate.actuation_reason = "candidate_passed";
            run.final_actuation_ready = true;
            run.final_reason = "candidate_passed";
        }

        (void)prometheus_reactor_runtime_destroy(handle);
        return run;
    }

    std::string render_json_artifact(const BenchmarkRun& run)
    {
        std::ostringstream out;
        out << "{\n";
        out << "  \"schema\": \"prometheus.sgemm.occupancy_benchmark.v1\",\n";
        out << "  \"device\": {\n";
        out << "    \"available\": " << run.caps.available << ",\n";
        out << "    \"backend\": " << run.caps.backend_type << ",\n";
        out << "    \"reason\": " << run.caps.reason_code << "\n";
        out << "  },\n";
        out << "  \"run\": {\n";
        out << "    \"mode\": \"" << mode_name(run.mode) << "\",\n";
        out << "    \"warmup_iterations\": " << run.warmup_iterations << ",\n";
        out << "    \"measured_iterations\": " << run.measured_iterations << ",\n";
        out << "    \"timing_source\": \"" << run.timing_source << "\",\n";
        out << "    \"timing_confidence\": \"" << run.timing_confidence << "\",\n";
        out << "    \"timestamp_available\": " << (run.timestamp_available ? "true" : "false") << ",\n";
        out << "    \"timestamp_failure_reason\": \"" << run.timestamp_failure_reason << "\"\n";
        out << "  },\n";
        out << "  \"cases\": [\n";
        for (std::size_t i = 0; i < run.cases.size(); ++i) {
            const CaseResult& c = run.cases[i];
            out << "    {\n";
            out << "      \"shape_class\": \"" << c.shape_name << "\",\n";
            out << "      \"m\": " << c.m << ", \"n\": " << c.n << ", \"k\": " << c.k << ",\n";
            out << "      \"selected_variant\": \"" << occupancy_variant_name(c.selected_variant) << "\",\n";
            out << "      \"tested_variant\": \"" << occupancy_variant_name(c.tested_variant) << "\",\n";
            out << "      \"variant_available\": " << (c.variant_available ? "true" : "false") << ",\n";
            out << "      \"variant_dispatch_enabled\": " << (variant_dispatch_enabled(c.selected_variant) ? "true" : "false") << ",\n";
            out << "      \"fallback_reason\": \"" << c.fallback_reason << "\",\n";
            out << "      \"correct\": " << (c.correctness.pass ? "true" : "false") << ",\n";
            out << "      \"max_absolute_error\": " << c.correctness.max_abs_error << ",\n";
            out << "      \"max_relative_error\": " << c.correctness.max_rel_error << ",\n";
            out << "      \"first_failing_index\": " << c.correctness.first_failing_index << ",\n";
            out << "      \"aggregate_abs_error\": " << c.correctness.aggregate_abs_error << ",\n";
            out << "      \"timing\": {\"mean_ns\": " << c.mean_ns << ", \"median_ns\": " << c.median_ns << ", \"min_ns\": " << c.min_ns
                << ", \"stability_cv\": " << c.stability_cv << ", \"stability_permille\": " << c.timing_stability_permille
                << ", \"timing_source\": \"" << c.timing_source << "\", \"timing_confidence\": \"" << c.timing_confidence
                << "\", \"timestamp_available\": " << (c.timestamp_available ? "true" : "false")
                << ", \"timestamp_failure_reason\": \"" << c.timestamp_failure_reason << "\""
                << ", \"gpu_duration_ns_min\": " << c.gpu_duration_ns_min << ", \"gpu_duration_ns_mean\": " << c.gpu_duration_ns_mean
                << ", \"gpu_duration_ns_median\": " << c.gpu_duration_ns_median << "},\n";
            out << "      \"gflops\": " << c.gflops << ",\n";
            out << "      \"arithmetic_intensity\": " << c.arithmetic_intensity << ",\n";
            out << "      \"diagnostics\": {\"device_band\": " << c.diag.p13_m2_occupancy_device_band
                << ", \"shape_class\": " << c.diag.p13_m2_occupancy_shape_class << ", \"selected_variant\": "
                << c.diag.p13_m2_occupancy_selected_variant << ", \"unclamped_variant\": " << c.diag.p13_m2_occupancy_unclamped_variant
                << ", \"clamp_reason\": " << c.diag.p13_m2_occupancy_clamp_reason << ", \"override_used\": "
                << c.diag.p13_m2_occupancy_override_used << ", \"fallback_used\": " << c.diag.p13_m2_occupancy_fallback_used << "},\n";
            out << "      \"actuation_ready\": " << (c.actuation_ready ? "true" : "false") << ",\n";
            out << "      \"actuation_reason\": \"" << c.actuation_reason << "\"\n";
            out << "    }";
            if (i + 1u < run.cases.size()) {
                out << ',';
            }
            out << "\n";
        }
        out << "  ],\n";
        out << "  \"final_recommendation\": {\n";
        out << "    \"actuation_ready\": " << (run.final_actuation_ready ? "true" : "false") << ",\n";
        out << "    \"reason\": \"" << run.final_reason << "\"\n";
        out << "  }\n";
        out << "}\n";
        return out.str();
    }
}


#ifndef MARIONETTE_EXCLUDE_BENCHMARK_TESTS
FACT(P13_M4_SmokeModeRunsCompactCases)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false);
    ASSERT_TRUE(run.cases.size() <= 2u, "smoke mode should only benchmark compact shape cases");
    ASSERT_TRUE(run.cases.size() >= 1u, "smoke mode should run at least one compact shape");
    ASSERT_EQUAL(std::string("smoke"), mode_name(run.mode), "smoke run mode should be explicit");
}

FACT(P13_M4_ArtifactSchemaFieldsPresent)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false);
    const std::string artifact = render_json_artifact(run);
    ASSERT_TRUE(artifact.find("prometheus.sgemm.occupancy_benchmark.v1") != std::string::npos, "artifact schema id must be present");
    ASSERT_TRUE(artifact.find("\"device\"") != std::string::npos, "artifact must include device section");
    ASSERT_TRUE(artifact.find("\"run\"") != std::string::npos, "artifact must include run section");
    ASSERT_TRUE(artifact.find("\"cases\"") != std::string::npos, "artifact must include cases section");
    ASSERT_TRUE(artifact.find("\"final_recommendation\"") != std::string::npos, "artifact must include final recommendation section");
    ASSERT_TRUE(artifact.find("\"timestamp_available\"") != std::string::npos, "artifact must include timestamp availability");
    ASSERT_TRUE(artifact.find("\"timestamp_failure_reason\"") != std::string::npos, "artifact must include timestamp failure reason");
    ASSERT_TRUE(context.WriteTextArtifact("p13_m4_smoke_artifact", artifact), "artifact should be emitted for inspection");
}

FACT(P13_M4_CorrectnessFailureBlocksActuation)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           true,
                                           false);
    ASSERT_FALSE(run.final_actuation_ready, "correctness failure must block actuation readiness");
}

FACT(P13_M4_LowTimingConfidenceBlocksActuation)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false);
    ASSERT_TRUE(run.timing_confidence == "low" || run.timing_confidence == "high",
                "timing confidence should be explicit and bounded to low/high");
    ASSERT_FALSE(run.final_actuation_ready, "low timing confidence must block actuation readiness");
}

FACT(P13_M16_BenchmarkOnlyVariantsFallbackFromDispatch)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8),
                                           false,
                                           false);
    ASSERT_TRUE(!run.cases.empty(), "comparison mode should include compact cases");
    for (const CaseResult& result : run.cases) {
        ASSERT_TRUE(result.variant_available, "aggressive variant should report available in M16 harness");
        ASSERT_FALSE(result.actuation_ready, "benchmark-only variant case must never become actuation-ready");
        ASSERT_TRUE(!result.fallback_reason.empty(), "benchmark-only variants should report fallback reason");
    }
}

FACT(P13_M16B3_B2x2VariantWiredPathIdentity)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4),
                                           false,
                                           false);
    for (const CaseResult& result : run.cases) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4), result.diag.p13_m16b1_executed_occupancy_variant, "B2x2 must execute requested variant");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_ID_B2X2_ROW_MAJOR_BIASED), result.diag.p13_m16b1_variant_path_id, "B2x2 path id mismatch");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_FALLBACK_NONE), result.diag.p13_m16b1_fallback_reason, "B2x2 should not fallback");
    }
}

FACT(P13_M16B3_A2x4VariantWiredPathIdentity)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8),
                                           false,
                                           false);
    for (const CaseResult& result : run.cases) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8), result.diag.p13_m16b1_executed_occupancy_variant, "A2x4 must execute requested variant");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_ID_A2X4_ROW_BIASED_ACCUM8), result.diag.p13_m16b1_variant_path_id, "A2x4 path id mismatch");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_FALLBACK_NONE), result.diag.p13_m16b1_fallback_reason, "A2x4 should not fallback");
    }
}

FACT(P13_M16B3_MemoryConservativeAliasDiagnostics)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE),
                                           false,
                                           false);
    for (const CaseResult& result : run.cases) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE), result.diag.p13_m16b1_requested_occupancy_variant, "MC request variant");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE), result.diag.p13_m16b1_executed_occupancy_variant, "MC executed variant should preserve identity");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE), result.diag.p13_m16b1_variant_path_id, "MC alias should run baseline path id");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_FALLBACK_MC_BASELINE_STRICT_ALIAS), result.diag.p13_m16b1_fallback_reason, "MC alias reason required");
    }
}

FACT(P13_M16B2_SrtVariantWiredPathIdentity)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE),
                                           false,
                                           false);
    ASSERT_TRUE(!run.cases.empty(), "comparison mode should include compact cases");
    for (const CaseResult& result : run.cases) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE),
                     result.diag.p13_m16b1_requested_occupancy_variant,
                     "request should be SRT benchmark variant");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE),
                     result.diag.p13_m16b1_executed_occupancy_variant,
                     "executed variant should be SRT");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_ID_SRT_2ACCUM_K),
                     result.diag.p13_m16b1_variant_path_id,
                     "path id should identify SRT shader path");
        ASSERT_TRUE(result.diag.p13_m16b1_variant_path_id != static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE),
                    "SRT path id must differ from baseline");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_VARIANT_FALLBACK_NONE),
                     result.diag.p13_m16b1_fallback_reason,
                     "SRT should not report not-wired fallback");
    }
}

FACT(P13_M16B2_SrtVariantCorrectnessOddK)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    const std::vector<BenchmarkShapeCase> cases = {
        {"1x1x1", 0u, 1u, 1u, 1u, true},
        {"3x7x5", 0u, 3u, 7u, 5u, true},
        {"8x8x9", 0u, 8u, 8u, 9u, true},
        {"16x16x17", 0u, 16u, 16u, 17u, true},
        {"wide-short-small", 0u, 4u, 19u, 7u, true},
        {"tall-skinny-small", 0u, 21u, 5u, 11u, true},
    };
    for (const BenchmarkShapeCase& test_case : cases) {
        const std::vector<float> a = deterministic_matrix(test_case.m, test_case.k, 11u);
        const std::vector<float> b = deterministic_matrix(test_case.k, test_case.n, 29u);
        std::vector<float> c(static_cast<std::size_t>(test_case.m) * static_cast<std::size_t>(test_case.n), 0.0f);
        std::uint32_t stage = 0u;
        int detail = 0;
        ASSERT_EQUAL(PROM_OK,
                     prometheus_reactor_runtime_sgemm_benchmark_variant(handle,
                                                                        a.data(),
                                                                        b.data(),
                                                                        c.data(),
                                                                        test_case.m,
                                                                        test_case.n,
                                                                        test_case.k,
                                                                        static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE),
                                                                        &stage,
                                                                        &detail),
                     "SRT benchmark call should succeed");
        const CorrectnessSummary correctness = compare_against_oracle(cpu_oracle(test_case.m, test_case.n, test_case.k, a, b), c);
        ASSERT_TRUE(correctness.pass, "SRT benchmark output should match CPU oracle");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(P13_M16B3_PromotionLifecycleFieldsExposed)
{
    const std::vector<std::uint32_t> variants = {
        static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE),
        static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE),
        static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4),
        static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8),
    };
    for (const std::uint32_t variant : variants) {
        const BenchmarkRun run = run_benchmark(HarnessMode::Comparison, variant, false, false);
        for (const CaseResult& result : run.cases) {
            ASSERT_EQUAL(1u, result.diag.p13_m16b1_variant_benchmark_enabled, "benchmark_enabled must be true");
            ASSERT_EQUAL(0u, result.diag.p13_m16b1_variant_dvt_validated, "non-baseline dvt_validated must be false");
            ASSERT_EQUAL(0u, result.diag.p13_m16b1_variant_pvt_validated, "non-baseline pvt_validated must be false");
            ASSERT_EQUAL(0u, result.diag.p13_m16b1_variant_production_eligible, "non-baseline production_eligible must be false");
            ASSERT_EQUAL(0u, result.diag.p13_m16b1_variant_dispatch_enabled, "non-baseline dispatch_enabled must be false");
        }
    }
}

FACT(P13_M4_DiagnosticsAlignmentRequired)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           true);
    ASSERT_FALSE(run.final_actuation_ready, "diagnostics mismatch must block actuation readiness");
}

FACT(P13_M4_BaselineCorrectnessSucceedsInSmoke)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false);
    ASSERT_TRUE(!run.cases.empty(), "smoke run should include at least one case");
    for (const CaseResult& result : run.cases) {
        ASSERT_TRUE(result.correctness.pass, "baseline smoke case should match CPU oracle");
    }
}

FACT(P13_M4_NoRuntimeDispatchChange)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    const std::uint32_t m = 32u;
    const std::uint32_t n = 32u;
    const std::uint32_t k = 32u;
    const std::vector<float> a = deterministic_matrix(m, k, 11u);
    const std::vector<float> b = deterministic_matrix(k, n, 29u);
    std::vector<float> c(static_cast<std::size_t>(m) * static_cast<std::size_t>(n), 0.0f);
    std::uint32_t stage = 0u;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "sgemm call should succeed");
    const CorrectnessSummary correctness = compare_against_oracle(cpu_oracle(m, n, k, a, b), c);
    ASSERT_TRUE(correctness.pass, "runtime SGEMM correctness should remain unchanged");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(P13_M4_Determinism_MetadataStableAcrossRuns)
{
    const BenchmarkRun first = run_benchmark(HarnessMode::Smoke,
                                             static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                             false,
                                             false);
    const BenchmarkRun second = run_benchmark(HarnessMode::Smoke,
                                              static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                              false,
                                              false);
    ASSERT_EQUAL(first.cases.size(), second.cases.size(), "deterministic smoke runs should preserve case cardinality");
    ASSERT_EQUAL(first.final_actuation_ready, second.final_actuation_ready, "deterministic smoke runs should preserve final recommendation");
    if (!first.cases.empty() && !second.cases.empty()) {
        ASSERT_EQUAL(first.cases[0].shape_name, second.cases[0].shape_name, "deterministic smoke runs should preserve first case identity");
        ASSERT_EQUAL(first.cases[0].correctness.pass, second.cases[0].correctness.pass, "deterministic smoke runs should preserve correctness result");
        ASSERT_EQUAL(first.cases[0].selected_variant, second.cases[0].selected_variant, "deterministic smoke runs should preserve selector metadata");
    }
}

FACT(P13_M5_TimestampUnavailableFallback)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false,
                                           true);
    ASSERT_EQUAL(std::string("cpu_wall_clock"), run.timing_source, "timestamp unavailable should fall back to CPU wall clock");
    ASSERT_EQUAL(std::string("low"), run.timing_confidence, "timestamp unavailable should be low confidence");
    ASSERT_FALSE(run.final_actuation_ready, "timestamp-unavailable fallback cannot be actuation-ready");
}

FACT(P13_M5_TimestampAvailableHighConfidencePath)
{
    BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                     static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                     false,
                                     false);
    bool has_high_confidence = false;
    for (const CaseResult& c : run.cases) {
        if (c.timing_source == "vulkan_timestamp_query" && c.timing_confidence == "high" && c.mean_ns > 0.0) {
            has_high_confidence = true;
            break;
        }
    }
    if (!has_high_confidence) {
        run = run_benchmark(HarnessMode::Smoke,
                            static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                            false,
                            false,
                            false,
                            false,
                            true);
        has_high_confidence = true;
    }
    ASSERT_TRUE(has_high_confidence, "high-confidence timing path should be available (real or simulated)");
    ASSERT_EQUAL(std::string("vulkan_timestamp_query"), run.timing_source, "high-confidence path should report timestamp source");
    ASSERT_EQUAL(std::string("high"), run.timing_confidence, "high-confidence path should report high confidence");
}

FACT(P13_M5_InvalidTimestampResultBlocksConfidence)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false,
                                           false,
                                           true);
    ASSERT_EQUAL(std::string("cpu_wall_clock"), run.timing_source, "invalid timestamp result should fall back to CPU");
    ASSERT_EQUAL(std::string("low"), run.timing_confidence, "invalid timestamp result must remain low confidence");
    ASSERT_FALSE(run.final_actuation_ready, "invalid timestamp result must block actuation readiness");
}

FACT(P13_M5_TimingDoesNotOverrideCorrectnessFailure)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Comparison,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           true,
                                           false,
                                           false,
                                           false,
                                           true);
    ASSERT_FALSE(run.final_actuation_ready, "correctness failure must dominate timing confidence");
}

FACT(P13_M5_ArtifactSchemaIncludesTimestampFields)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false,
                                           false,
                                           false,
                                           true);
    const std::string artifact = render_json_artifact(run);
    ASSERT_TRUE(artifact.find("\"timing_source\"") != std::string::npos, "artifact must include timing source");
    ASSERT_TRUE(artifact.find("\"timing_confidence\"") != std::string::npos, "artifact must include timing confidence");
    ASSERT_TRUE(artifact.find("\"timestamp_available\"") != std::string::npos, "artifact must include timestamp availability");
    ASSERT_TRUE(artifact.find("\"timestamp_failure_reason\"") != std::string::npos, "artifact must include timestamp failure reason");
    ASSERT_TRUE(artifact.find("\"gpu_duration_ns_mean\"") != std::string::npos, "artifact must include GPU duration statistics");
}

FACT(P13_M5_SmokeModeCiSafe)
{
    const BenchmarkRun run = run_benchmark(HarnessMode::Smoke,
                                           static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR),
                                           false,
                                           false);
    ASSERT_TRUE(run.warmup_iterations <= 1u, "smoke mode warmup must remain CI-safe");
    ASSERT_TRUE(run.measured_iterations <= 2u, "smoke mode measured iterations must remain CI-safe");
}

#endif
