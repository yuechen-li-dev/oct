#include "test_harness.h"

#include "../reactor_api.h"

#include <cstdint>

FACT(PrometheusReactor_FftDiagnosticsDefaultState)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.api_declared, "fft api should be declared");
    ASSERT_EQUAL(0u, diag.capability_reported, "fft capability must remain unclaimed");
    ASSERT_EQUAL(0u, diag.production_enabled, "fft production path must remain disabled");
    ASSERT_EQUAL(0u, diag.benchmark_enabled, "fft benchmark path defaults off");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id, "executed path should default unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

static PrometheusFftRequest make_valid_req(PrometheusComplex32* in, PrometheusComplex32* out)
{
    PrometheusFftRequest req{};
    req.struct_size = static_cast<std::uint32_t>(sizeof(req));
    req.input = in;
    req.output = out;
    req.element_count = 2u;
    req.batch_count = 1u;
    req.stride_elements = 0u;
    req.flags = 0u;
    return req;
}

FACT(PrometheusReactor_FftExecutionUnavailable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft execution should be unavailable");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_SUBMIT), stage, "unavailable fft should fail at submit stage");
    ASSERT_EQUAL(PROM_FFT_DETAIL_UNAVAILABLE, detail, "unavailable fft should report explicit detail");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.last_element_count, "diag should snapshot element count");
    ASSERT_EQUAL(1u, diag.last_batch_count, "diag should snapshot batch count");
    ASSERT_EQUAL(1u, diag.plan_valid, "valid request should build deterministic plan metadata");
    ASSERT_EQUAL(1u, diag.plan_pass_count, "n=2 should require one radix-2 pass");
    ASSERT_EQUAL(2u, diag.plan_first_span, "first span should be 2");
    ASSERT_EQUAL(2u, diag.plan_last_span, "last span should be 2 for n=2");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id, "diag path should remain unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftValidationFailures)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[4]{};
    PrometheusComplex32 out[4]{};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, nullptr, &stage, &detail), "null request should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_REQUEST, detail, "null request detail");

    PrometheusFftRequest req = make_valid_req(in, out);
    req.struct_size = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "bad struct size should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_REQUEST, detail, "bad struct size detail");

    req = make_valid_req(in, out); req.input = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "null input should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, detail, "null input detail");

    req = make_valid_req(in, out); req.output = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "null output should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_OUTPUT, detail, "null output detail");

    req = make_valid_req(in, out); req.element_count = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "zero element should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT, detail, "zero element detail");

    req = make_valid_req(in, out); req.element_count = 3u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "non power of two should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NON_POWER_OF_TWO, detail, "non power detail");

    req = make_valid_req(in, out); req.flags = PROM_FFT_FLAG_FORWARD | PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "dual direction should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS, detail, "direction detail");

    req = make_valid_req(in, out); req.stride_elements = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "short stride should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_STRIDE, detail, "stride detail");

    req = make_valid_req(in, out); req.flags = PROM_FFT_FLAG_INVERSE_NORMALIZE;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "inverse normalize without inverse should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE, detail, "inverse normalize direction rule");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftBenchmarkVariantUnavailable)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, 2u, &stage, &detail),
                 "benchmark variant should be unavailable in M2");
    ASSERT_EQUAL(PROM_FFT_DETAIL_UNAVAILABLE, detail, "benchmark detail should remain unavailable");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.requested_radix, "diag should record requested benchmark variant");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.requested_path_id,
                 "requested path should reflect benchmark request");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id,
                 "executed path remains unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftInvalidAndDestroyedHandle)
{
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prometheus_reactor_runtime_fft(nullptr, &req, &stage, &detail),
                 "null handle should be rejected");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prometheus_reactor_runtime_fft(handle, &req, &stage, &detail),
                 "destroyed handle should be rejected");
}

FACT(PrometheusReactor_FftDiagnosticsSizedAndCallFreshness)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_fft_diagnostics_sized(handle, &diag, 1u),
                 "undersized diagnostics struct should return partial snapshot like SGEMM sized diagnostics");

    PrometheusComplex32 in[8]{};
    PrometheusComplex32 out[8]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.element_count = 8u;
    req.stride_elements = 0u;

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft execution should be unavailable");
    ASSERT_EQUAL(PROM_FFT_DETAIL_UNAVAILABLE, detail, "first call detail should be unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(8u, diag.last_effective_stride_elements, "stride_elements=0 should record contiguous effective stride");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_CPU_ORACLE_RESERVED), diag.requested_path_id,
                 "first call should set requested path to cpu oracle reserved");

    req = make_valid_req(in, out);
    req.input = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "second call should fail validation");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, detail, "second call should report current validation failure");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, diag.last_failure_detail,
                 "diagnostics should report latest failure and not stale unavailable detail");
    ASSERT_EQUAL(0u, diag.plan_valid, "invalid request should clear plan validity");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_NONE), diag.requested_path_id,
                 "failed validation should not keep stale requested path from prior call");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id,
                 "executed path should remain unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftPlanDeterministicN1N8N16AndFlags)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[16]{};
    PrometheusComplex32 out[16]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    PrometheusFftDiagnostics diag{};

    req.element_count = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=1 remains unavailable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.plan_valid, "n=1 should still produce deterministic plan metadata");
    ASSERT_EQUAL(0u, diag.plan_pass_count, "n=1 pass count");
    ASSERT_EQUAL(0u, diag.plan_log2_element_count, "n=1 log2");

    req = make_valid_req(in, out);
    req.element_count = 8u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=8 remains unavailable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(3u, diag.plan_pass_count, "n=8 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=8 first span");
    ASSERT_EQUAL(8u, diag.plan_last_span, "n=8 last span");
    ASSERT_EQUAL(1u, diag.plan_bit_reversal_required, "n>1 requires bit reversal step");

    req = make_valid_req(in, out);
    req.element_count = 16u;
    req.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=16 inverse remains unavailable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(4u, diag.plan_pass_count, "n=16 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=16 first span");
    ASSERT_EQUAL(16u, diag.plan_last_span, "n=16 last span");
    ASSERT_EQUAL(3u, diag.ping_pong_swap_count, "n=16 swap count");
    ASSERT_EQUAL(2u, diag.plan_direction, "inverse flag should set inverse direction");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
