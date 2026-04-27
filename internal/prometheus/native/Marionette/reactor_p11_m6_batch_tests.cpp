#include "../bridge.h"
#include "test_harness.h"

#include <cmath>
#include <cstdint>
#include <vector>

namespace
{
std::vector<float> make_matrix(std::uint32_t rows, std::uint32_t cols, float seed)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        out[i] = seed + static_cast<float>((i * 7u) % 19u) * 0.125f;
    }
    return out;
}

std::vector<float> cpu_oracle(std::uint32_t m,
                              std::uint32_t n,
                              std::uint32_t k,
                              const std::vector<float>& a,
                              const std::vector<float>& b)
{
    std::vector<float> c(m * n, 0.0f);
    for (std::uint32_t row = 0; row < m; ++row) {
        for (std::uint32_t col = 0; col < n; ++col) {
            float sum = 0.0f;
            for (std::uint32_t kk = 0; kk < k; ++kk) {
                sum += a[row * k + kk] * b[kk * n + col];
            }
            c[row * n + col] = sum;
        }
    }
    return c;
}

bool nearly_equal(const std::vector<float>& a, const std::vector<float>& b)
{
    if (a.size() != b.size()) {
        return false;
    }
    for (std::size_t i = 0; i < a.size(); ++i) {
        if (std::fabs(a[i] - b[i]) > 1e-4f) {
            return false;
        }
    }
    return true;
}
}

FACT(PrometheusReactor_P11_M6_BatchRoundRobinAndDiagnostics)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<float> a0 = make_matrix(m, k, 0.2f);
    std::vector<float> b0 = make_matrix(k, n, 0.6f);
    std::vector<float> c0(m * n, 0.0f);
    std::vector<float> a1 = make_matrix(m, k, 0.4f);
    std::vector<float> b1 = make_matrix(k, n, 0.8f);
    std::vector<float> c1(m * n, 0.0f);

    PrometheusSgemmBatchEntry entries[2] = {
        {a0.data(), b0.data(), c0.data(), m, n, k},
        {a1.data(), b1.data(), c1.data(), m, n, k},
    };

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, entries, 2u, 2u, &stage, &detail);
    ASSERT_EQUAL(PROM_OK, status, "batch should succeed");
    ASSERT_EQUAL(PROM_STAGE_TRANSFER_OUT, stage, "successful batch should report transfer-out completion stage");

    ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a0, b0), c0), "entry 0 output should match reference oracle");
    ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a1, b1), c1), "entry 1 output should match reference oracle");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(2u, diag.last_batch_entry_count, "diagnostics should expose batch entry count");
    ASSERT_EQUAL(2u, diag.requested_workers, "requested workers should decode from flags");
    ASSERT_EQUAL(1u, diag.effective_workers, "single-queue conservative cap should force one effective worker");
    ASSERT_EQUAL(PROM_BATCH_PARTITION_ROUND_ROBIN, diag.partition_policy, "default partition policy should be round robin");
    ASSERT_EQUAL(PROM_BATCH_STATE_SUCCEEDED, diag.batch_state, "batch should finish in succeeded state");
    ASSERT_EQUAL(1u, diag.output_committed, "outputs should commit only on success");
    ASSERT_EQUAL(0u, diag.worker_judgment_count, "workers must not run judgment in execution phase");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M6_BatchFailureIsAtomicAndUncommitted)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<float> a0 = make_matrix(m, k, 0.2f);
    std::vector<float> b0 = make_matrix(k, n, 0.6f);
    std::vector<float> c0(m * n, 1.0f);
    std::vector<float> c1(m * n, 1.0f);

    PrometheusSgemmBatchEntry entries[2] = {
        {a0.data(), b0.data(), c0.data(), m, n, k},
        {nullptr, b0.data(), c1.data(), m, n, k},
    };

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, entries, 2u, 1u, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "invalid plan input should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_PLAN_INVALID, detail, "invalid entry should report explicit batch plan detail");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "failure should terminate in failed state");
    ASSERT_EQUAL(0u, diag.output_committed, "failed batch must not commit outputs");
    ASSERT_EQUAL(1u, diag.failed_entry_id, "failed entry id should be captured");

    ASSERT_EQUAL(1.0f, c0[0], "entry 0 output should remain untouched when batch fails atomically");
    ASSERT_EQUAL(1.0f, c1[0], "entry 1 output should remain untouched when batch fails atomically");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M6_BatchCriticalEventOverflowFailsExplicitly)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<float> a = make_matrix(m, k, 0.25f);
    std::vector<float> b = make_matrix(k, n, 0.75f);
    std::vector<float> c(m * n, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = 1u | (2u << 16u);
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, flags, &stage, &detail);
    ASSERT_EQUAL(PROM_ERROR, status, "critical event ring overflow should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW, detail, "critical overflow should use explicit detail code");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "overflow should drive failed final state");
    ASSERT_TRUE(diag.event_overflow_count > 0u, "overflow count should be surfaced");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
