#include "test_harness.h"
#include "../reactor_api.h"
#include <cstdint>

BENCHMARK_WITH_ITERATIONS(PrometheusReactor_Fft_Radix2Benchmark_N16, 2000)
{
    static void* handle = nullptr;
    static bool init = false;
    static PrometheusComplex32 in[16]{};
    static PrometheusComplex32 out[16]{};
    static PrometheusFftRequest req{};
    if (!init) {
        if (prometheus_reactor_runtime_create(nullptr, &handle) != PROM_OK) return;
        req.struct_size = static_cast<std::uint32_t>(sizeof(req));
        req.input = in; req.output = out; req.element_count = 16u; req.batch_count = 1u; req.stride_elements = 0u; req.flags = 0u;
        init = true;
    }
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    (void)context;
    (void)prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, 2u, &stage, &detail);
}
