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
