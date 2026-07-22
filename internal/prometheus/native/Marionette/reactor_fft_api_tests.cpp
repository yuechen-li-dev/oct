#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <string>
#include <vector>

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


FACT(PrometheusReactor_FftVkServiceSeamRejectsInvalidAndDestroyedHandle)
{
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prom_reactor_runtime_get_vk_services(nullptr, &services),
                 "null handle should be rejected");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prom_reactor_runtime_get_vk_services(handle, &services),
                 "destroyed handle should be rejected");
}

FACT(PrometheusReactor_FftVkServiceSeamReportsAvailabilityTruthfully)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    prom_vk_runtime_services services{};
    int status = prom_reactor_runtime_get_vk_services(handle, &services);
    if (status == PROM_OK)
    {
        ASSERT_EQUAL(1u, services.backend_available, "available runtime should report available backend");
        ASSERT_TRUE(services.device != VK_NULL_HANDLE, "service seam should expose device");
        ASSERT_TRUE(services.compute_queue != VK_NULL_HANDLE, "service seam should expose compute queue");
        ASSERT_TRUE(services.compute_command_pool != VK_NULL_HANDLE, "service seam should expose command pool");
    }
    else
    {
        ASSERT_EQUAL(PROM_ERROR, status, "unavailable backend should return PROM_ERROR");
        ASSERT_EQUAL(0u, services.backend_available, "unavailable runtime should report unavailable backend");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REASON_VULKAN_UNAVAILABLE), services.backend_reason_code,
                     "unavailable runtime should report vulkan unavailable reason");
    }

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

FACT(PrometheusReactor_FftExecutionDispatchesRadix2)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{{1.0f, 0.0f}, {2.0f, 0.0f}};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft should dispatch through Vulkan");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "fft should finish with a readback stage");
    ASSERT_EQUAL(0, detail, "successful fft should not retain a failure detail");
    ASSERT_NEAR(3.0f, out[0].real, 1.0e-5f, "radix-2 GPU dc bin");
    ASSERT_NEAR(-1.0f, out[1].real, 1.0e-5f, "radix-2 GPU Nyquist bin");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.last_element_count, "diag should snapshot element count");
    ASSERT_EQUAL(1u, diag.last_batch_count, "diag should snapshot batch count");
    ASSERT_EQUAL(1u, diag.plan_valid, "valid request should build deterministic plan metadata");
    ASSERT_EQUAL(1u, diag.plan_pass_count, "n=2 should require one radix-2 pass");
    ASSERT_EQUAL(2u, diag.plan_first_span, "first span should be 2");
    ASSERT_EQUAL(2u, diag.plan_last_span, "last span should be 2 for n=2");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.executed_path_id, "diag must identify the dispatched Vulkan route");

    PrometheusComplex32 multiIn[4]{{1.0f, 0.0f}, {2.0f, 0.0f}, {3.0f, 0.0f}, {4.0f, 0.0f}};
    PrometheusComplex32 multiOut[4]{};
    req.input = multiIn; req.output = multiOut; req.element_count = 4u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "two-pass N=4 FFT should execute");
    ASSERT_NEAR(10.0f, multiOut[0].real, 2.0e-5f, "N=4 DC");
    ASSERT_NEAR(-2.0f, multiOut[1].real, 2.0e-5f, "N=4 bin one real");
    ASSERT_NEAR(2.0f, multiOut[1].imag, 2.0e-5f, "N=4 bin one imag uses forward negative sign");
    ASSERT_NEAR(-2.0f, multiOut[2].real, 2.0e-5f, "N=4 Nyquist");
    ASSERT_NEAR(-2.0f, multiOut[3].real, 2.0e-5f, "N=4 bin three real");
    ASSERT_NEAR(-2.0f, multiOut[3].imag, 2.0e-5f, "N=4 bin three imag uses forward negative sign");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_RayQueryAdmissionIsOptionalAndCompleteWhenEnabled)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    prom_vk_runtime_services services{};
    int status = prom_reactor_runtime_get_vk_services(handle, &services);
    if (status == PROM_OK)
    {
        if (services.ray_query_state == PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED)
        {
            ASSERT_EQUAL(1u, services.ray_query_acceleration_structure_extension_supported, "enabled ray query requires acceleration-structure extension");
            ASSERT_EQUAL(1u, services.ray_query_extension_supported, "enabled ray query requires ray-query extension");
            ASSERT_EQUAL(1u, services.ray_query_deferred_host_operations_extension_supported, "enabled ray query requires deferred-host-operations extension");
            ASSERT_EQUAL(1u, services.ray_query_buffer_device_address_supported, "enabled ray query requires buffer device address");
            ASSERT_EQUAL(1u, services.ray_query_acceleration_structure_supported, "enabled ray query requires acceleration structures");
            ASSERT_EQUAL(1u, services.ray_query_supported, "enabled ray query requires rayQuery feature");
            ASSERT_TRUE(services.create_acceleration_structure != nullptr, "enabled ray query loads create entry point");
            ASSERT_TRUE(services.destroy_acceleration_structure != nullptr, "enabled ray query loads destroy entry point");
            ASSERT_TRUE(services.get_acceleration_structure_build_sizes != nullptr, "enabled ray query loads build-size entry point");
            ASSERT_TRUE(services.cmd_build_acceleration_structures != nullptr, "enabled ray query loads build entry point");
            ASSERT_TRUE(services.get_acceleration_structure_device_address != nullptr, "enabled ray query loads address entry point");
        }
        else
        {
            ASSERT_TRUE(services.ray_query_state == PROM_VK_RAY_QUERY_UNSUPPORTED ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_EXTENSION_MISSING ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_FEATURE_MISSING ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_ENTRY_POINT_MISSING,
                        "unsupported ray-query admission must retain a precise optional state");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

static std::vector<PrometheusComplex32> fft_dft_oracle(const std::vector<PrometheusComplex32>& input,
                                                        std::uint32_t elementCount,
                                                        std::uint32_t batchCount,
                                                        std::uint32_t stride,
                                                        bool inverse)
{
    constexpr double pi = 3.14159265358979323846264338327950288;
    std::vector<PrometheusComplex32> expected = input;
    for (std::uint32_t batch = 0u; batch < batchCount; ++batch) {
        const std::uint32_t base = batch * stride;
        for (std::uint32_t k = 0u; k < elementCount; ++k) {
            double real = 0.0;
            double imag = 0.0;
            for (std::uint32_t sample = 0u; sample < elementCount; ++sample) {
                const double angle = (inverse ? 2.0 : -2.0) * pi * static_cast<double>(k * sample) / static_cast<double>(elementCount);
                const double cosine = std::cos(angle);
                const double sine = std::sin(angle);
                const PrometheusComplex32 value = input[base + sample];
                real += static_cast<double>(value.real) * cosine - static_cast<double>(value.imag) * sine;
                imag += static_cast<double>(value.real) * sine + static_cast<double>(value.imag) * cosine;
            }
            if (inverse) {
                real /= static_cast<double>(elementCount);
                imag /= static_cast<double>(elementCount);
            }
            expected[base + k] = {static_cast<float>(real), static_cast<float>(imag)};
        }
    }
    return expected;
}

static void assert_fft_matches_dft(::marionette::tests::TestContext& context,
                                   void* handle,
                                   const char* name,
                                   const std::vector<PrometheusComplex32>& input,
                                   std::uint32_t elementCount,
                                   std::uint32_t batchCount,
                                   std::uint32_t stride,
                                   bool inverse)
{
    std::vector<PrometheusComplex32> output(input.size(), {77.0f, -77.0f});
    const std::vector<PrometheusComplex32> expected = fft_dft_oracle(input, elementCount, batchCount, stride, inverse);
    PrometheusFftRequest request = make_valid_req(const_cast<PrometheusComplex32*>(input.data()), output.data());
    request.element_count = elementCount;
    request.batch_count = batchCount;
    request.stride_elements = stride == elementCount ? 0u : stride;
    request.flags = inverse ? PROM_FFT_FLAG_INVERSE : 0u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), std::string(name) + " must execute");
    if (detail != 0) return;

    float maxAbsoluteError = 0.0f;
    for (std::uint32_t batch = 0u; batch < batchCount; ++batch) {
        const std::uint32_t base = batch * stride;
        for (std::uint32_t k = 0u; k < elementCount; ++k) {
            maxAbsoluteError = std::max(maxAbsoluteError, std::fabs(output[base + k].real - expected[base + k].real));
            maxAbsoluteError = std::max(maxAbsoluteError, std::fabs(output[base + k].imag - expected[base + k].imag));
        }
        for (std::uint32_t padding = elementCount; padding < stride; ++padding) {
            ASSERT_NEAR(77.0f, output[base + padding].real, 0.0f, std::string(name) + " must preserve output padding");
            ASSERT_NEAR(-77.0f, output[base + padding].imag, 0.0f, std::string(name) + " must preserve output padding");
        }
    }
    ASSERT_TRUE(maxAbsoluteError <= 3.0e-3f, std::string(name) + " independent DFT maximum absolute error is within Complex32 acceptance");
}

FACT(PrometheusReactor_FftBatchedStrideAndInverseNormalization)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    // Two N=2 transforms, with two Complex32 padding values between batches.
    PrometheusComplex32 in[6]{{1.0f, 0.0f}, {2.0f, 0.0f}, {91.0f, -91.0f}, {92.0f, -92.0f}, {3.0f, 0.0f}, {5.0f, 0.0f}};
    PrometheusComplex32 out[6]{{-1.0f, -1.0f}, {-1.0f, -1.0f}, {77.0f, -77.0f}, {78.0f, -78.0f}, {-1.0f, -1.0f}, {-1.0f, -1.0f}};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.batch_count = 2u;
    req.stride_elements = 4u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "batched strided forward FFT should execute");
    ASSERT_NEAR(3.0f, out[0].real, 1.0e-5f, "batch zero DC");
    ASSERT_NEAR(-1.0f, out[1].real, 1.0e-5f, "batch zero Nyquist");
    ASSERT_NEAR(8.0f, out[4].real, 1.0e-5f, "batch one DC");
    ASSERT_NEAR(-2.0f, out[5].real, 1.0e-5f, "batch one Nyquist");
    ASSERT_NEAR(77.0f, out[2].real, 0.0f, "first padding sentinel must survive");
    ASSERT_NEAR(78.0f, out[3].real, 0.0f, "second padding sentinel must survive");

    req.input = out;
    req.output = in;
    req.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "inverse-normalized FFT should execute");
    ASSERT_NEAR(1.0f, in[0].real, 1.0e-5f, "inverse restores batch zero sample zero");
    ASSERT_NEAR(2.0f, in[1].real, 1.0e-5f, "inverse restores batch zero sample one");
    ASSERT_NEAR(3.0f, in[4].real, 1.0e-5f, "inverse restores batch one sample zero");
    ASSERT_NEAR(5.0f, in[5].real, 1.0e-5f, "inverse restores batch one sample one");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftNumericalCorpusUsesIndependentDftAuthority)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    for (const std::uint32_t elementCount : {1u, 2u, 8u, 16u, 64u, 256u, 1024u}) {
        std::vector<PrometheusComplex32> random(elementCount);
        std::uint32_t state = 0x9e3779b9u ^ elementCount;
        for (PrometheusComplex32& value : random) {
            state = state * 1664525u + 1013904223u;
            value.real = static_cast<float>(static_cast<int>((state >> 8u) & 0xffffu) - 32768) / 32768.0f;
            state = state * 1664525u + 1013904223u;
            value.imag = static_cast<float>(static_cast<int>((state >> 8u) & 0xffffu) - 32768) / 32768.0f;
        }
        assert_fft_matches_dft(context, handle, "deterministic pseudo-random forward", random, elementCount, 1u, elementCount, false);
        assert_fft_matches_dft(context, handle, "deterministic pseudo-random inverse", random, elementCount, 1u, elementCount, true);
    }

    std::vector<PrometheusComplex32> tone(32u);
    for (std::uint32_t i = 0u; i < 32u; ++i) {
        const float angle = 2.0f * 3.14159265358979323846f * 5.0f * static_cast<float>(i) / 32.0f;
        tone[i] = {std::cos(angle), std::sin(angle)};
    }
    assert_fft_matches_dft(context, handle, "single-frequency complex tone", tone, 32u, 1u, 32u, false);

    std::vector<PrometheusComplex32> realNonSymmetric(16u);
    for (std::uint32_t i = 0u; i < 16u; ++i) realNonSymmetric[i] = {static_cast<float>((i * i + 3u * i + 1u) % 11u) - 5.0f, 0.0f};
    assert_fft_matches_dft(context, handle, "real-only non-symmetric", realNonSymmetric, 16u, 1u, 16u, false);

    std::vector<PrometheusComplex32> constant(8u, {2.0f, -0.5f});
    assert_fft_matches_dft(context, handle, "complex constant", constant, 8u, 1u, 8u, false);
    std::vector<PrometheusComplex32> alternating(8u);
    for (std::uint32_t i = 0u; i < 8u; ++i) alternating[i] = {(i & 1u) == 0u ? 1.0f : -1.0f, 0.0f};
    assert_fft_matches_dft(context, handle, "alternating signs", alternating, 8u, 1u, 8u, false);

    std::vector<PrometheusComplex32> batched(57u, {31.0f, -31.0f});
    for (std::uint32_t batch = 0u; batch < 3u; ++batch) {
        for (std::uint32_t i = 0u; i < 16u; ++i) {
            batched[batch * 19u + i] = {static_cast<float>(batch * 7u + i) / 9.0f, static_cast<float>(static_cast<int>(batch * 5u) - static_cast<int>(i)) / 11.0f};
        }
    }
    assert_fft_matches_dft(context, handle, "three distinct padded batches", batched, 16u, 3u, 19u, false);

    std::vector<PrometheusComplex32> roundTrip(64u);
    for (std::uint32_t i = 0u; i < 64u; ++i) roundTrip[i] = {static_cast<float>(i % 9u) - 4.0f, static_cast<float>((3u * i) % 13u) - 6.0f};
    const std::vector<PrometheusComplex32> original = roundTrip;
    std::vector<PrometheusComplex32> transformed(64u);
    PrometheusFftRequest request = make_valid_req(roundTrip.data(), transformed.data());
    request.element_count = 64u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), "round-trip forward must execute");
    request.input = transformed.data();
    request.output = roundTrip.data();
    request.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), "round-trip inverse must execute");
    float roundTripMaximumAbsoluteError = 0.0f;
    for (std::uint32_t i = 0u; i < 64u; ++i) {
        roundTripMaximumAbsoluteError = std::max(roundTripMaximumAbsoluteError, std::fabs(roundTrip[i].real - original[i].real));
        roundTripMaximumAbsoluteError = std::max(roundTripMaximumAbsoluteError, std::fabs(roundTrip[i].imag - original[i].imag));
    }
    ASSERT_TRUE(roundTripMaximumAbsoluteError <= 3.0e-3f, "forward then inverse Complex32 round trip must stay within acceptance");
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

FACT(PrometheusReactor_FftBenchmarkVariantExecutesSmallRadix2)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail),
                 "benchmark variant must invoke the production Vulkan radix-2 route");
    ASSERT_EQUAL(0, detail, "benchmark execution must report success only after real execution");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.requested_radix, "diag should record radix-2 planning");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.requested_path_id,
                 "requested path should reflect benchmark request");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.executed_path_id,
                 "benchmark execution must identify the Vulkan route");

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
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft execution should complete");
    ASSERT_EQUAL(0, detail, "successful fft should have no failure detail");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(8u, diag.last_effective_stride_elements, "stride_elements=0 should record contiguous effective stride");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.requested_path_id,
                 "first call should select the Vulkan radix-2 production route");

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
                 "invalid request must clear the previous executed route");

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
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=1 must execute as a bit-reversal identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.plan_valid, "n=1 should still produce deterministic plan metadata");
    ASSERT_EQUAL(0u, diag.plan_pass_count, "n=1 pass count");
    ASSERT_EQUAL(0u, diag.plan_log2_element_count, "n=1 log2");
    ASSERT_EQUAL(0u, diag.ping_pong_swap_count, "n=1 swap count");

    req = make_valid_req(in, out);
    req.element_count = 8u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=8 must execute its three planned passes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(3u, diag.plan_pass_count, "n=8 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=8 first span");
    ASSERT_EQUAL(8u, diag.plan_last_span, "n=8 last span");
    ASSERT_EQUAL(1u, diag.plan_bit_reversal_required, "n>1 requires bit reversal step");
    ASSERT_EQUAL(3u, diag.ping_pong_swap_count, "n=8 swap count equals pass count");

    req = make_valid_req(in, out);
    req.element_count = 16u;
    req.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=16 inverse must execute its four planned passes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(4u, diag.plan_pass_count, "n=16 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=16 first span");
    ASSERT_EQUAL(16u, diag.plan_last_span, "n=16 last span");
    ASSERT_EQUAL(4u, diag.ping_pong_swap_count, "n=16 swap count equals pass count");
    ASSERT_EQUAL(2u, diag.plan_direction, "inverse flag should set inverse direction");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftExecutableAndProductionQualified)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[32]{};
    PrometheusComplex32 out[32]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.element_count = 32u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail), "bounded N=32 benchmark route should execute");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "bounded production fft should execute");
    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diag");
    ASSERT_EQUAL(1u, diag.production_enabled, "production admission follows the qualified executable route");
    ASSERT_EQUAL(1u, diag.capability_reported, "FFT capability should report the registered production route");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
