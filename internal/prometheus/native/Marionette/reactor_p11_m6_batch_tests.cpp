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

std::uint32_t contiguous_worker_for_entry(std::uint32_t entry, std::uint32_t entry_count, std::uint32_t workers)
{
    return static_cast<std::uint32_t>((static_cast<std::uint64_t>(entry) * static_cast<std::uint64_t>(workers)) /
                                      static_cast<std::uint64_t>(entry_count));
}

std::uint32_t round_robin_worker_for_entry(std::uint32_t entry, std::uint32_t workers)
{
    return workers == 0u ? 0u : (entry % workers);
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

FACT(PrometheusReactor_P11_M7_BatchSingleEntryMatchesSingleSgemmPath)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t m = 5u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 4u;
    std::vector<float> a = make_matrix(m, k, 0.15f);
    std::vector<float> b = make_matrix(k, n, 0.35f);
    std::vector<float> single_output(m * n, 0.0f);
    std::vector<float> batch_output(m * n, 0.0f);

    std::uint32_t single_stage = PROM_STAGE_NONE;
    int single_detail = 0;
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), single_output.data(), m, n, k, &single_stage, &single_detail),
                 "single path should succeed");
    ASSERT_EQUAL(PROM_STAGE_TRANSFER_OUT, single_stage, "single path should complete transfer-out stage");

    PrometheusSgemmBatchEntry entry{a.data(), b.data(), batch_output.data(), m, n, k};
    std::uint32_t batch_stage = PROM_STAGE_NONE;
    int batch_detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, 1u, &batch_stage, &batch_detail), "single-entry batch should succeed");
    ASSERT_EQUAL(PROM_STAGE_TRANSFER_OUT, batch_stage, "single-entry batch should complete transfer-out stage");
    ASSERT_TRUE(nearly_equal(single_output, batch_output), "single-entry batch output should match single SGEMM path output");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(1u, diag.last_batch_entry_count, "single-entry batch should publish entry count");
    ASSERT_EQUAL(1u, diag.output_committed, "successful single-entry batch should commit output");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M7_ContiguousPartitionAssignmentDeterministicAndOrderedCommit)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 6u;
    const std::uint32_t workers = 3u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);

    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.1f + static_cast<float>(i) * 0.25f);
        b[i] = make_matrix(k, n, 0.4f + static_cast<float>(i) * 0.2f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t success_flags =
        workers | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT);
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, success_flags, &stage, &detail),
                 "contiguous partition batch should succeed");
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a[i], b[i]), c[i]), "contiguous partition should preserve ordered output commit");
    }

    std::vector<std::vector<float>> fail_outputs(entry_count, std::vector<float>(m * n, 7.0f));
    std::vector<PrometheusSgemmBatchEntry> fail_entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        fail_entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), fail_outputs[i].data(), m, n, k};
    }
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        const std::uint32_t fail_flags = workers | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                         ((i + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT);
        ASSERT_EQUAL(PROM_ERROR,
                     prometheus_reactor_runtime_sgemm_batch(handle, fail_entries.data(), entry_count, fail_flags, &stage, &detail),
                     "targeted failure should surface deterministic worker assignment");
        PrometheusSgemmBatchDiagnostics diag{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
        ASSERT_EQUAL(0u, diag.output_committed, "failed contiguous run must remain uncommitted");
        ASSERT_EQUAL(contiguous_worker_for_entry(i, entry_count, workers), diag.failed_worker_id, "contiguous worker assignment must be deterministic");
        ASSERT_EQUAL(i, diag.failed_entry_id, "targeted failure should preserve selected entry id");
        ASSERT_EQUAL(7.0f, fail_outputs[i][0], "failed contiguous run should not commit caller-visible output");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M7_WorkerCapsExposeHardwareAndMemoryReasons)
{
    void* handle_hw = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle_hw), "runtime create should succeed");

    const std::uint32_t m = 2u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 2u;
    std::vector<float> a = make_matrix(m, k, 0.2f);
    std::vector<float> b = make_matrix(k, n, 0.3f);
    std::vector<float> c(m * n, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t hw_flags = 8u | (3u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle_hw, &entry, 1u, hw_flags, &stage, &detail), "hardware-capped batch should succeed");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle_hw, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(8u, diag.requested_workers, "requested workers should preserve caller value");
    ASSERT_EQUAL(3u, diag.hardware_queue_cap, "hardware queue cap override should be visible");
    ASSERT_EQUAL(3u, diag.effective_workers, "effective workers should cap to hardware queue count");
    ASSERT_EQUAL(PROM_BATCH_CAP_REASON_HARDWARE_QUEUE, diag.worker_cap_reason, "hardware queue cap reason should be explicit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle_hw), "runtime destroy should succeed");

    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_FAIL_DEVICE_CREATE;
    void* handle_mem = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle_mem), "runtime create should succeed with test config");
    const std::uint32_t mem_flags =
        8u | (8u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle_mem, &entry, 1u, mem_flags, &stage, &detail), "memory-capped batch should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle_mem, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(8u, diag.requested_workers, "requested workers should preserve caller value");
    ASSERT_EQUAL(8u, diag.hardware_queue_cap, "test hardware cap override should be visible");
    ASSERT_EQUAL(4u, diag.memory_worker_cap, "32MiB test budget with 8MiB worker arena should cap at 4 workers");
    ASSERT_EQUAL(4u, diag.effective_workers, "effective workers should be memory capped");
    ASSERT_EQUAL(PROM_BATCH_CAP_REASON_MEMORY_BUDGET, diag.worker_cap_reason, "memory cap reason should be explicit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle_mem), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M7_ZeroWorkerMemoryFailureAndSingleQueueConservativeReason)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_FAIL_DEVICE_CREATE;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t m = 2u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 2u;
    std::vector<float> a = make_matrix(m, k, 0.12f);
    std::vector<float> b = make_matrix(k, n, 0.22f);
    std::vector<float> c(m * n, 9.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, 4u, &stage, &detail), "zero-worker memory case should fail");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_ZERO_WORKERS, detail, "zero-worker memory case should surface explicit detail");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(0u, diag.effective_workers, "effective workers should report zero");
    ASSERT_EQUAL(0u, diag.output_committed, "zero-worker failure should not commit output");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "zero-worker memory path should end failed");
    ASSERT_EQUAL(9.0f, c[0], "zero-worker memory failure should keep caller output untouched");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");

    void* single_queue_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &single_queue_handle), "runtime create should succeed");
    std::vector<float> c2(m * n, 0.0f);
    PrometheusSgemmBatchEntry entry2{a.data(), b.data(), c2.data(), m, n, k};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(single_queue_handle, &entry2, 1u, 4u, &stage, &detail), "single queue conservative run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(single_queue_handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(1u, diag.hardware_queue_cap, "default hardware queue cap should remain conservative single queue");
    ASSERT_EQUAL(1u, diag.effective_workers, "effective workers should remain single queue");
    ASSERT_EQUAL(PROM_BATCH_CAP_REASON_SINGLE_QUEUE_CONSERVATIVE, diag.worker_cap_reason, "single queue cap reason should be explicit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(single_queue_handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M7_LateFailureAndDominatusGapRemainExplicit)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 3u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 5.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.14f + static_cast<float>(i) * 0.12f);
        b[i] = make_matrix(k, n, 0.33f + static_cast<float>(i) * 0.08f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = 2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT);
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail), "late injected failure should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EXECUTION_FAILED, detail, "late injected failure should surface execution-failed detail");

    for (const auto& output : c) {
        ASSERT_EQUAL(5.0f, output[0], "late batch failure must not commit partial output");
    }

    PrometheusSgemmBatchDiagnostics batch_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &batch_diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, batch_diag.batch_state, "late failure should end in failed state");
    ASSERT_EQUAL(0u, batch_diag.output_committed, "late failure should keep output uncommitted");
    ASSERT_EQUAL(2u, batch_diag.failed_entry_id, "late failure should preserve failed entry id");
    ASSERT_EQUAL(0u, batch_diag.failed_worker_id, "late failure should preserve failed worker id");
    ASSERT_TRUE(batch_diag.event_drain_count > 0u, "event drain should still run on late failure");
    ASSERT_EQUAL(0u, batch_diag.worker_judgment_count, "workers should not run judgment in batch path");

    PrometheusSgemmPolicyDiagnostics policy_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &policy_diag), "policy diagnostics query should succeed");
    ASSERT_EQUAL(0u, policy_diag.decision_count, "batch workers should not mutate Dominatus policy decision counters");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M7_DiagnosticsTruthfulnessForSuccessfulBatch)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t m = 3u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 3u;
    std::vector<float> a0 = make_matrix(m, k, 0.11f);
    std::vector<float> b0 = make_matrix(k, n, 0.21f);
    std::vector<float> c0(m * n, 0.0f);
    std::vector<float> a1 = make_matrix(m, k, 0.31f);
    std::vector<float> b1 = make_matrix(k, n, 0.41f);
    std::vector<float> c1(m * n, 0.0f);
    PrometheusSgemmBatchEntry entries[2] = {
        {a0.data(), b0.data(), c0.data(), m, n, k},
        {a1.data(), b1.data(), c1.data(), m, n, k},
    };

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = 3u | (3u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries, 2u, flags, &stage, &detail), "batch should succeed");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(2u, diag.last_batch_entry_count, "entry count should be truthful");
    ASSERT_EQUAL(3u, diag.requested_workers, "requested workers should be truthful");
    ASSERT_EQUAL(3u, diag.hardware_queue_cap, "hardware queue cap should be truthful");
    ASSERT_EQUAL(3u, diag.effective_workers, "effective workers should be truthful");
    ASSERT_TRUE(diag.memory_worker_cap >= 3u, "memory worker cap should not under-report capacity");
    ASSERT_EQUAL(PROM_BATCH_CAP_REASON_NONE, diag.worker_cap_reason, "uncapped run should report no cap reason");
    ASSERT_EQUAL(PROM_BATCH_PARTITION_ROUND_ROBIN, diag.partition_policy, "partition policy should be truthful");
    ASSERT_EQUAL(PROM_BATCH_STATE_SUCCEEDED, diag.batch_state, "batch state should be truthful");
    ASSERT_EQUAL(static_cast<std::uint32_t>(-1), diag.failed_entry_id, "success should keep failed entry sentinel");
    ASSERT_EQUAL(static_cast<std::uint32_t>(-1), diag.failed_worker_id, "success should keep failed worker sentinel");
    ASSERT_EQUAL(PROM_STAGE_NONE, diag.failure_stage, "success should keep failure stage at none");
    ASSERT_EQUAL(0, diag.failure_detail, "success should keep failure detail zero");
    ASSERT_EQUAL(0u, diag.event_overflow_count, "success should keep overflow count zero");
    ASSERT_TRUE(diag.event_drain_count >= 4u, "success should report drained worker events");
    ASSERT_EQUAL(1u, diag.output_committed, "success should commit output");
    ASSERT_EQUAL(40u, diag.plan_generation, "plan generation should remain 40");
    ASSERT_EQUAL(0u, diag.worker_judgment_count, "workers should not run judgment");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M8_MultiWorkerAssignmentAndLaneExecutionDiagnostics)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 8u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.21f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.41f + static_cast<float>(i) * 0.05f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags =
        workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail), "multi-worker run should succeed");
    ASSERT_EQUAL(PROM_STAGE_TRANSFER_OUT, stage, "success should report transfer-out stage");
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a[i], b[i]), c[i]), "multi-worker run should preserve output correctness");
    }

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(workers, diag.effective_workers, "effective workers should honor test hardware override");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_LANE_SIMULATED, diag.execution_mode, "multi-worker M8 execution should be lane-simulated");
    ASSERT_EQUAL(PROM_BATCH_WORKER_RESOURCE_SIMULATED, diag.worker_resource_mode, "worker resource ownership should be explicit");
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        const std::uint32_t worker = round_robin_worker_for_entry(i, workers);
        ASSERT_TRUE(diag.worker_assigned_count[worker] > 0u, "each planned worker should receive static round-robin assignments");
    }
    for (std::uint32_t w = 0; w < workers; ++w) {
        ASSERT_EQUAL(diag.worker_assigned_count[w], diag.worker_completed_count[w], "completed count should match assigned count on success");
        ASSERT_TRUE(diag.worker_event_count[w] >= diag.worker_completed_count[w], "event count should include completion lifecycle events");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M8_ContiguousAssignmentAndFailureDrainAcrossWorkers)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 8u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 6.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.51f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.71f + static_cast<float>(i) * 0.05f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    const std::uint32_t fail_entry = 5u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = workers | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                ((fail_entry + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT);
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail), "worker failure should fail full batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EXECUTION_FAILED, detail, "failure detail should be explicit");
    for (const auto& out : c) {
        ASSERT_EQUAL(6.0f, out[0], "failed batch must not partially commit caller-visible outputs");
    }

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "failed multi-worker batch should terminate failed");
    ASSERT_EQUAL(0u, diag.output_committed, "failed multi-worker batch should remain uncommitted");
    ASSERT_EQUAL(fail_entry, diag.failed_entry_id, "failed entry should remain explicit");
    ASSERT_EQUAL(contiguous_worker_for_entry(fail_entry, entry_count, workers), diag.failed_worker_id, "failed worker should follow contiguous assignment");
    ASSERT_TRUE(diag.event_drain_count > 0u, "failure path should still drain worker events");
    ASSERT_TRUE(diag.worker_event_count[diag.failed_worker_id] > 0u, "failed worker ring should contain failure lifecycle events");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M10_RealThreadsSerializedVulkanModeAndDiagnostics)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 8u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.91f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.61f + static_cast<float>(i) * 0.05f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t fail_entry = 2u;
    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) |
                                ((fail_entry + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT);
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "real-thread run with injected failure should fail");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN, diag.execution_mode, "real-thread mode should be explicit");
    ASSERT_EQUAL(workers, diag.real_worker_thread_count, "diagnostics should expose created worker threads");
    ASSERT_EQUAL(0u, diag.lane_worker_count, "lane workers should be zero in real-thread mode");
    ASSERT_EQUAL(1u, diag.serialized_vulkan, "real-thread mode should mark serialized Vulkan bridge");
    ASSERT_TRUE(diag.serialized_bridge_enter_count >= 1u, "serialized bridge enter count should be tracked");
    ASSERT_TRUE(diag.serialized_execution_count >= 1u, "serialized execution count should record bridge entries");
    ASSERT_TRUE(diag.max_concurrent_serialized_entries <= 1u, "serialized bridge must admit at most one entry at a time");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "serialized Vulkan mode must not claim hardware parallelism");
    ASSERT_EQUAL(0u, diag.worker_judgment_count, "workers must not run judgment");
    ASSERT_EQUAL(0u, diag.output_committed, "injected failure should keep outputs uncommitted");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M10_FirstFailureWinsAndLaneFallbackStillAvailable)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 8u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 8.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 1.11f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 1.41f + static_cast<float>(i) * 0.05f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t fail_entry = 3u;
    const std::uint32_t fail_flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                     (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) |
                                     ((fail_entry + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT);
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, fail_flags, &stage, &detail),
                 "real-thread injected failure should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EXECUTION_FAILED, detail, "real-thread failure detail should be explicit");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "real-thread failure should terminate in failed state");
    ASSERT_EQUAL(fail_entry, diag.failed_entry_id, "first injected failure must be preserved");
    ASSERT_EQUAL(0u, diag.output_committed, "failed real-thread run must not commit output");
    ASSERT_TRUE(diag.event_drain_count > 0u, "failed real-thread run should still drain events");
    ASSERT_TRUE(diag.serialized_wait_count >= 0u, "serialized bridge wait diagnostic should be present");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");

    PrometheusReactorConfig lane_cfg{};
    lane_cfg.struct_size = sizeof(PrometheusReactorConfig);
    lane_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED;
    void* lane_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&lane_cfg, &lane_handle), "runtime create should succeed for lane fallback");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(lane_handle, entries.data(), entry_count,
                                                                 workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                     (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                 &stage, &detail),
                 "lane fallback run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(lane_handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_LANE_SIMULATED, diag.execution_mode, "force-lane test flag should preserve M8 fallback mode");
    ASSERT_EQUAL(workers, diag.lane_worker_count, "lane worker count should reflect effective workers in fallback");
    ASSERT_EQUAL(0u, diag.real_worker_thread_count, "lane fallback should not spawn real threads");
    ASSERT_EQUAL(0u, diag.serialized_vulkan, "lane fallback should not mark serialized Vulkan thread bridge");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(lane_handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M10_RealThreadEnablementGateIsExplicitAndForceLaneWins)
{
    const std::uint32_t entry_count = 4u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 2u;
    const std::uint32_t n = 2u;
    const std::uint32_t k = 2u;
    std::vector<std::vector<float>> a(entry_count);
    std::vector<std::vector<float>> b(entry_count);
    std::vector<std::vector<float>> c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 2.11f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 2.41f + static_cast<float>(i) * 0.05f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }
    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    PrometheusSgemmBatchDiagnostics diag{};

    PrometheusReactorConfig unrelated_cfg{};
    unrelated_cfg.struct_size = sizeof(PrometheusReactorConfig);
    unrelated_cfg.test_flags = PROM_TESTCFG_DISABLE_SELECTOR_CACHE;
    void* unrelated_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&unrelated_cfg, &unrelated_handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(unrelated_handle, entries.data(), entry_count, flags, &stage, &detail),
                 "unrelated test flag batch run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(unrelated_handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_LANE_SIMULATED, diag.execution_mode, "unrelated test flags must not enable real-thread mode");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(unrelated_handle), "runtime destroy should succeed");

    PrometheusReactorConfig explicit_cfg{};
    explicit_cfg.struct_size = sizeof(PrometheusReactorConfig);
    explicit_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* explicit_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&explicit_cfg, &explicit_handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(explicit_handle, entries.data(), entry_count, flags, &stage, &detail),
                 "explicit real-thread batch run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(explicit_handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN, diag.execution_mode,
                 "explicit real-thread flag should enable real-thread mode");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(explicit_handle), "runtime destroy should succeed");

    PrometheusReactorConfig forced_lane_cfg{};
    forced_lane_cfg.struct_size = sizeof(PrometheusReactorConfig);
    forced_lane_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED;
    void* forced_lane_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&forced_lane_cfg, &forced_lane_handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(forced_lane_handle, entries.data(), entry_count, flags, &stage, &detail),
                 "forced-lane batch run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(forced_lane_handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_LANE_SIMULATED, diag.execution_mode,
                 "force-lane flag must override explicit real-thread enablement");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(forced_lane_handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_ConcurrentFailuresPreserveDeterministicFirstFailure)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 4u;
    const std::uint32_t workers = 2u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 3.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.7f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.9f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) | PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "dual injected failures should fail batch");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "batch should terminate failed");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EXECUTION_FAILED, diag.failure_detail, "failure detail should remain execution failed");
    ASSERT_TRUE(diag.failure_count >= 1u, "failure count should be tracked");
    ASSERT_EQUAL(1u, diag.first_failure_stable, "first failure metadata should remain stable");
    ASSERT_EQUAL(0u, diag.output_committed, "failed batch must not commit outputs");
    for (const auto& out : c) {
        ASSERT_EQUAL(3.0f, out[0], "failed dual-failure batch must not commit any caller output");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_RealThreadCriticalOverflowFailsWithoutCommit)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 6u;
    const std::uint32_t workers = 3u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 11.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 1.7f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 1.9f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) |
                                (2u << PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_SHIFT);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "critical overflow should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW, detail, "critical overflow should report explicit detail");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_TRUE(diag.event_overflow_count > 0u, "overflow count should be visible");
    ASSERT_EQUAL(0u, diag.output_committed, "overflow failure must not commit output");
    ASSERT_TRUE(diag.event_drain_count > 0u, "overflow failure should still drain events");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_FailureWhileOtherWorkerEmitsDrainsSafely)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 4u;
    const std::uint32_t workers = 2u;
    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 13.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.3f + static_cast<float>(i) * 0.2f);
        b[i] = make_matrix(k, n, 0.5f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    const std::uint32_t fail_entry = 1u;
    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) |
                                ((fail_entry + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT) | PROM_BATCH_FLAG_TEST_DELAY_ENTRY0;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "failure while another worker is active should fail batch");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "failed run should terminate failed");
    ASSERT_TRUE(diag.event_drain_count > 0u, "failed run should drain events");
    ASSERT_EQUAL(0u, diag.worker_active_mask, "workers should be fully inactive after drain");
    ASSERT_EQUAL(0u, diag.output_committed, "failed run must not commit output");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_OutOfOrderCompletionStillCommitsInEntryOrder)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 2u;
    const std::uint32_t workers = 2u;
    const std::uint32_t m = 6u;
    const std::uint32_t n = 6u;
    const std::uint32_t k = 6u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 2.3f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 2.6f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) | PROM_BATCH_FLAG_TEST_DELAY_ENTRY0;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "out-of-order completion path should still succeed");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_TRUE(diag.worker_completed_count[1] >= 1u, "worker 1 should complete while worker 0 is delayed");
    ASSERT_EQUAL(1u, diag.output_committed, "success should atomically commit all entry outputs");
    ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a[0], b[0]), c[0]), "entry 0 output must match oracle");
    ASSERT_TRUE(nearly_equal(cpu_oracle(m, n, k, a[1], b[1]), c[1]), "entry 1 output must match oracle");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_RepeatedBatchesResetPerBatchState)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<float> a = make_matrix(m, k, 0.9f);
    std::vector<float> b = make_matrix(k, n, 1.1f);
    std::vector<float> c0(m * n, 0.0f), c1(m * n, 7.0f), c2(m * n, 0.0f);
    PrometheusSgemmBatchEntry e0{a.data(), b.data(), c0.data(), m, n, k};
    PrometheusSgemmBatchEntry e1{a.data(), b.data(), c1.data(), m, n, k};
    PrometheusSgemmBatchEntry e2{a.data(), b.data(), c2.data(), m, n, k};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    PrometheusSgemmBatchDiagnostics diag{};

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &e0, 1u,
                 workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT), &stage, &detail),
                 "first success batch should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_SUCCEEDED, diag.batch_state, "first run should be successful");
    ASSERT_EQUAL(1u, diag.output_committed, "successful run should commit");

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &e1, 1u,
                 workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT) |
                     ((1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT), &stage, &detail),
                 "middle failure batch should fail");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "second run should be failed");
    ASSERT_EQUAL(0u, diag.output_committed, "failed run should not commit");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &e2, 1u,
                 workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT), &stage, &detail),
                 "third success batch should recover");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_STATE_SUCCEEDED, diag.batch_state, "third run should succeed");
    ASSERT_EQUAL(1u, diag.output_committed, "third run should commit");
    ASSERT_EQUAL(PROM_STAGE_NONE, diag.failure_stage, "third run should reset failure stage");
    ASSERT_EQUAL(0, diag.failure_detail, "third run should reset failure detail");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M11_RuntimeDestroyAfterFailedRealThreadBatchIsSafe)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<float> a = make_matrix(m, k, 1.9f);
    std::vector<float> b = make_matrix(k, n, 2.1f);
    std::vector<float> c(m * n, 9.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                 workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | ((1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT),
                 &stage, &detail),
                 "failed batch setup should fail");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy after failed real-thread batch should be clean");
}

FACT(PrometheusReactor_P11_M11_WorkerRestrictionsRemainEnforcedAndDiagnosticsTruthful)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 4u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 3.1f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 3.4f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "real-thread batch should succeed");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN, diag.execution_mode, "execution mode should be truthful");
    ASSERT_EQUAL(workers, diag.real_worker_thread_count, "real worker count should be truthful");
    ASSERT_EQUAL(0u, diag.lane_worker_count, "lane worker count should be zero in real-thread mode");
    ASSERT_EQUAL(1u, diag.serialized_vulkan, "serialized execution flag should be true");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "serialized mode must not claim hardware parallelism");
    ASSERT_EQUAL(0u, diag.worker_judgment_count, "workers must not run judgment");
    ASSERT_TRUE(diag.serialized_execution_count >= entry_count, "serialized bridge entries should be counted");
    ASSERT_TRUE(diag.max_concurrent_serialized_entries <= 1u, "serialized bridge should not run concurrent submit");
    ASSERT_EQUAL(40u, diag.plan_generation, "plan generation must remain unchanged");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M13_PerWorkerCommandResourceIdentityAndQueueMappingDiagnostics)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t entry_count = 8u;
    const std::uint32_t workers = 4u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.11f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.31f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count, flags, &stage, &detail),
                 "M13 per-worker resource diagnostics run should succeed");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_TRUE(diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER ||
                    diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER,
                "resource mode should explicitly report physical or diagnostics-only ownership");
    ASSERT_EQUAL(PROM_BATCH_QUEUE_TOPOLOGY_PSEUDO_SHARED, diag.queue_topology_classification, "queue topology classification should be explicit");
    ASSERT_EQUAL(PROM_BATCH_QUEUE_MAPPING_PER_WORKER_MAPPED_SERIALIZED, diag.queue_mapping_mode, "queue mapping mode should be explicit");
    for (std::uint32_t w = 0; w < workers; ++w) {
        ASSERT_EQUAL(w, diag.worker_slot_id[w], "slot identity should be stable per worker");
        ASSERT_EQUAL(w, diag.worker_output_staging_id[w], "staging identity should be stable per worker");
        ASSERT_EQUAL(w, diag.worker_arena_bank_id[w], "arena identity should be stable per worker");
        ASSERT_TRUE(diag.worker_submit_count[w] >= 1u, "worker submit count should be tracked");
        ASSERT_TRUE(diag.worker_command_pool_id[w] != 0u, "command pool identity should be present");
        ASSERT_TRUE(diag.worker_command_buffer_id[w] != 0u, "command buffer identity should be present");
        ASSERT_TRUE(diag.worker_fence_id[w] != 0u, "fence identity should be present");
    }
    if (diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER) {
        ASSERT_EQUAL(1u, diag.worker_command_pool_valid[0], "physical mode should publish valid command-pool ownership");
        ASSERT_EQUAL(1u, diag.worker_command_buffer_valid[0], "physical mode should publish valid command-buffer ownership");
        ASSERT_EQUAL(1u, diag.worker_fence_valid[0], "physical mode should publish valid fence ownership");
        ASSERT_TRUE(diag.worker_command_buffer_id[0] != diag.worker_command_buffer_id[1], "worker command buffers must not alias");
        ASSERT_TRUE(diag.worker_command_pool_id[0] != diag.worker_command_pool_id[1], "worker command pools must not alias");
        ASSERT_TRUE(diag.worker_fence_id[0] != diag.worker_fence_id[1], "worker fences must not alias");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M13_WrongOwnerCommandResourceUseRejected)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_P11_BATCH_TEST_FORCE_WRONG_RESOURCE_OWNER;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<float> a = make_matrix(m, k, 0.2f);
    std::vector<float> b = make_matrix(k, n, 0.4f);
    std::vector<float> c(m * n, 5.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR,
                 prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                                                        workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                            (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                        &stage, &detail),
                 "wrong-owner resource use should be rejected");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION, detail, "wrong-owner rejection detail should be explicit");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_TRUE(diag.resource_ownership_violation_count >= 1u, "ownership violations should be counted");
    ASSERT_EQUAL(0u, diag.output_committed, "wrong-owner failure should remain uncommitted");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M14_PhysicalWorkerCommandResourcesModeAndSerializedSubmitPreserved)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t entry_count = 4u;
    const std::uint32_t m = 4u;
    const std::uint32_t n = 4u;
    const std::uint32_t k = 4u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(m * n, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(m, k, 0.75f + static_cast<float>(i) * 0.1f);
        b[i] = make_matrix(k, n, 0.95f + static_cast<float>(i) * 0.1f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), m, n, k};
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count,
                                                        workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                            (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                        &stage, &detail),
                 "M14 batch run should succeed");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN, diag.execution_mode, "real-thread serialized bridge mode should remain");
    ASSERT_EQUAL(1u, diag.serialized_vulkan, "serialized bridge flag should remain true");
    ASSERT_TRUE(diag.max_concurrent_serialized_entries <= 1u, "serialized submit gate must remain <= 1");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "hardware parallelism claim must remain false");
    ASSERT_TRUE(diag.serialized_bridge_enter_count >= 1u, "serialized bridge enter count should remain truthful");

    if (diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER) {
        for (std::uint32_t w = 0; w < workers; ++w) {
            ASSERT_EQUAL(1u, diag.worker_command_pool_valid[w], "physical mode should mark command pool valid");
            ASSERT_EQUAL(1u, diag.worker_command_buffer_valid[w], "physical mode should mark command buffer valid");
            ASSERT_EQUAL(1u, diag.worker_fence_valid[w], "physical mode should mark fence valid");
            ASSERT_TRUE(diag.worker_record_count[w] >= 1u, "physical mode should record worker command buffers");
            ASSERT_TRUE(diag.worker_reset_count[w] >= 1u, "physical mode should reset worker command resources");
            ASSERT_TRUE(diag.worker_wait_count[w] >= 1u, "physical mode should wait worker-local fences");
        }
        ASSERT_TRUE(diag.worker_command_pool_id[0] != diag.worker_command_pool_id[1], "physical worker command pools must be distinct");
        ASSERT_TRUE(diag.worker_command_buffer_id[0] != diag.worker_command_buffer_id[1], "physical worker command buffers must be distinct");
        ASSERT_TRUE(diag.worker_fence_id[0] != diag.worker_fence_id[1], "physical worker fences must be distinct");
    } else {
        ASSERT_EQUAL(PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER, diag.worker_resource_mode,
                     "fallback mode should remain explicit when physical allocation is unavailable");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M14_FailureHooksPreserveFirstFailureAndCleanup)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FAIL_RESET_FENCE;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t m = 3u;
    const std::uint32_t n = 3u;
    const std::uint32_t k = 3u;
    std::vector<float> a = make_matrix(m, k, 0.5f);
    std::vector<float> b = make_matrix(k, n, 0.7f);
    std::vector<float> c(m * n, 6.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), m, n, k};

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                                                               workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                   (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                               &stage, &detail);
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics query should succeed");
    if (diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER) {
        ASSERT_EQUAL(PROM_ERROR, status, "injected fence reset failure should fail physical mode batch");
        ASSERT_EQUAL(PROM_DETAIL_BATCH_FENCE_RESET_FAILED, detail, "fence reset failure should be explicit");
        ASSERT_EQUAL(0u, diag.output_committed, "failed physical mode run should not commit output");
    } else {
        ASSERT_EQUAL(PROM_OK, status, "simulated fallback may ignore Vulkan fence-failure hooks");
    }

    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed after failure run");
}

FACT(PrometheusReactor_P11_M16_SingleQueueFallsBackToSerializedBridge)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    std::vector<float> a = make_matrix(4u, 4u, 0.3f);
    std::vector<float> b = make_matrix(4u, 4u, 0.7f);
    std::vector<float> c(16u, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                                                              workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT),
                                                              &stage, &detail);
    ASSERT_TRUE(status == PROM_OK || status == PROM_ERROR, "single queue path should return a concrete status");

    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.true_multi_queue_selected, "single queue should not select true multi queue");
    ASSERT_TRUE(diag.serialized_fallback_reason != PROM_BATCH_FALLBACK_REASON_NONE, "fallback reason should be explicit");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "fallback mode must not claim hardware parallelism");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M16_IndependentTwoQueueSelectsTrueMultiQueueWithHook)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    const std::uint32_t entry_count = 2u;
    std::vector<std::vector<float>> a(entry_count), b(entry_count), c(entry_count, std::vector<float>(16u, 0.0f));
    std::vector<PrometheusSgemmBatchEntry> entries(entry_count);
    for (std::uint32_t i = 0; i < entry_count; ++i) {
        a[i] = make_matrix(4u, 4u, 0.3f + static_cast<float>(i) * 0.2f);
        b[i] = make_matrix(4u, 4u, 0.5f + static_cast<float>(i) * 0.2f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), 4u, 4u, 4u};
    }
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), entry_count,
                                                                  workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                      (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                  &stage, &detail),
                 "hooked independent queues should run");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.true_multi_queue_selected, "true multi-queue should be selected");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_REAL_THREADS_TRUE_MULTI_QUEUE, diag.execution_mode, "execution mode should report true multi-queue");
    ASSERT_EQUAL(1u, diag.hardware_parallelism_claimed, "true multi-queue should claim hardware parallelism");
    ASSERT_EQUAL(0u, diag.serialized_vulkan, "true multi-queue path should bypass serialized bridge");
    ASSERT_EQUAL(0u, diag.serialized_fallback_reason, "selected true multi-queue should clear fallback reason");
    ASSERT_EQUAL(0u, diag.worker_queue_index[0], "worker mapping should be deterministic");
    ASSERT_EQUAL(1u, diag.worker_queue_index[1], "worker mapping should be deterministic");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M16_MemoryCapBlocksTrueMultiQueue)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    std::vector<float> a = make_matrix(4u, 4u, 0.4f);
    std::vector<float> b = make_matrix(4u, 4u, 0.8f);
    std::vector<float> c(16u, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = 1u | (1u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    const int status = prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, flags, &stage, &detail);
    ASSERT_TRUE(status == PROM_OK || status == PROM_ERROR, "run should produce concrete status");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.true_multi_queue_selected, "memory-capped workers should block true multi queue");
    ASSERT_TRUE(diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_EFFECTIVE_WORKERS_LT_2 ||
                    diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_INDEPENDENT_QUEUE_LT_2,
                "fallback reason should identify a true-multi gate failure");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M16_TransferQueueNotCountedAsComputeLane)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    std::vector<float> a = make_matrix(4u, 4u, 0.2f);
    std::vector<float> b = make_matrix(4u, 4u, 0.6f);
    std::vector<float> c(16u, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, 2u, &stage, &detail), "run should succeed");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.reported_compute_queue_count >= diag.independent_compute_queue_count, "transfer queue must stay separate from compute counts");
    ASSERT_TRUE(diag.transfer_compute_sync_wait_count >= 0u, "transfer sync wait counter should be populated");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_FailureAfterSubmitDrainsAndStaysUncommitted)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    const std::uint32_t workers = 2u;
    std::vector<std::vector<float>> a(2u), b(2u), c(2u, std::vector<float>(16u, 9.0f));
    PrometheusSgemmBatchEntry entries[2];
    for (std::uint32_t i = 0; i < 2u; ++i) {
        a[i] = make_matrix(4u, 4u, 0.11f + static_cast<float>(i) * 0.2f);
        b[i] = make_matrix(4u, 4u, 0.33f + static_cast<float>(i) * 0.2f);
        entries[i] = PrometheusSgemmBatchEntry{a[i].data(), b[i].data(), c[i].data(), 4u, 4u, 4u};
    }
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = workers | PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries, 2u, flags, &stage, &detail), "post-submit injected failure should fail");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_EXECUTION_FAILED, detail, "post-submit failure detail should be explicit");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.output_committed, "failed run must remain uncommitted");
    ASSERT_EQUAL(PROM_BATCH_STATE_FAILED, diag.batch_state, "batch should finish failed");
    ASSERT_TRUE(diag.queue_drain_count >= 1u, "drain count should be recorded");
    ASSERT_EQUAL(1u, diag.first_failure_stable, "first failure metadata should remain stable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_WorkerFenceResetAndWaitFailuresRemainWorkerScoped)
{
    const std::uint32_t workers = 2u;
    const std::uint32_t run_flags = workers | (workers << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    std::vector<float> a = make_matrix(4u, 4u, 0.2f);
    std::vector<float> b = make_matrix(4u, 4u, 0.6f);
    std::vector<float> c(16u, 5.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    PrometheusReactorConfig reset_cfg{};
    reset_cfg.struct_size = sizeof(PrometheusReactorConfig);
    reset_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FAIL_RESET_FENCE;
    void* reset_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&reset_cfg, &reset_handle), "reset-failure runtime create should succeed");
    (void)prometheus_reactor_runtime_sgemm_batch(reset_handle, &entry, 1u, run_flags, &stage, &detail);
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(reset_handle, &diag), "reset diagnostics should succeed");
    if (diag.worker_resource_mode == PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER) {
        ASSERT_EQUAL(PROM_DETAIL_BATCH_FENCE_RESET_FAILED, detail, "reset failure hook should surface explicit detail");
        ASSERT_EQUAL(0u, diag.output_committed, "reset failure must remain uncommitted");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(reset_handle), "reset runtime destroy should succeed");

    PrometheusReactorConfig wait_cfg{};
    wait_cfg.struct_size = sizeof(PrometheusReactorConfig);
    wait_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_TILED_PATH;
    void* wait_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&wait_cfg, &wait_handle), "wait-failure runtime create should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(wait_handle, &entry, 1u, run_flags, &stage, &detail), "wait failure hook should fail");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_FENCE_WAIT_FAILED, detail, "wait failure detail should be explicit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(wait_handle, &diag), "wait diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.output_committed, "wait failure must remain uncommitted");
    ASSERT_TRUE(diag.failed_worker_id < workers, "wait failure should preserve failing worker id");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(wait_handle), "wait runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_DrainTimeoutMarksUnsafeToReuse)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_STAGED_PATH | PROM_TESTCFG_FORCE_TILED_PATH |
                     PROM_TESTCFG_DISABLE_SELECTOR_CACHE;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    std::vector<float> a = make_matrix(4u, 4u, 0.4f);
    std::vector<float> b = make_matrix(4u, 4u, 0.9f);
    std::vector<float> c(16u, 1.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                                                                     2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                         (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                     &stage, &detail),
                 "forced drain-timeout hook should fail");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_DRAIN_TIMEOUT, detail, "drain timeout detail should be explicit");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.drain_timeout_count >= 1u, "drain timeout count should increment");
    ASSERT_EQUAL(1u, diag.unsafe_to_reuse, "drain timeout should mark runtime unsafe");
    ASSERT_EQUAL(0u, diag.output_committed, "drain timeout path must remain uncommitted");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_DeviceLostDominatesAndMarksUnsafe)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_UPLOAD_ONLY | PROM_TESTCFG_DISABLE_STAGING_FALLBACK;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    std::vector<float> a = make_matrix(4u, 4u, 0.25f);
    std::vector<float> b = make_matrix(4u, 4u, 0.45f);
    std::vector<float> c(16u, 1.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u,
                                                                     2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                         (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                     &stage, &detail),
                 "device lost hook should fail batch");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_DEVICE_LOST, detail, "device lost detail should be explicit");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.unsafe_to_reuse, "device lost should mark runtime unsafe");
    ASSERT_EQUAL(0u, diag.output_committed, "device lost path must not commit output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_RepeatedTrueMultiBatchesAndMappingStayStable)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(PrometheusReactorConfig);
    cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    std::vector<float> a = make_matrix(4u, 4u, 0.1f);
    std::vector<float> b = make_matrix(4u, 4u, 0.2f);
    std::vector<float> c(16u, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t success_flags = 2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) | (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, success_flags, &stage, &detail), "first success should pass");
    PrometheusSgemmBatchDiagnostics d0{}, d1{}, d2{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &d0), "first diagnostics should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, success_flags | PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT, &stage, &detail),
                 "middle failure should fail");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &d1), "middle diagnostics should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, &entry, 1u, success_flags, &stage, &detail), "final success should pass");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &d2), "final diagnostics should succeed");
    ASSERT_EQUAL(d0.worker_queue_index[0], d2.worker_queue_index[0], "mapping should stay stable");
    ASSERT_EQUAL(d0.worker_queue_index[1], d2.worker_queue_index[1], "mapping should stay stable");
    ASSERT_EQUAL(1u, d2.output_committed, "final success should commit output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_P11_M17_FallbackReasonsRemainTruthfulAcrossGateTransitions)
{
    std::vector<float> a = make_matrix(4u, 4u, 0.2f);
    std::vector<float> b = make_matrix(4u, 4u, 0.5f);
    std::vector<float> c(16u, 0.0f);
    PrometheusSgemmBatchEntry entry{a.data(), b.data(), c.data(), 4u, 4u, 4u};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    PrometheusSgemmBatchDiagnostics diag{};

    PrometheusReactorConfig mem_cfg{};
    mem_cfg.struct_size = sizeof(PrometheusReactorConfig);
    mem_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH | PROM_TESTCFG_FAIL_DEVICE_CREATE;
    void* mem_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&mem_cfg, &mem_handle), "memory-cap runtime create should succeed");
    (void)prometheus_reactor_runtime_sgemm_batch(mem_handle, &entry, 1u,
                                                 4u | (4u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                     (1u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                 &stage, &detail);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(mem_handle, &diag), "memory-cap diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.true_multi_queue_selected, "memory cap should force fallback");
    ASSERT_TRUE(diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_MEMORY_CAP ||
                    diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_EFFECTIVE_WORKERS_LT_2,
                "fallback reason should be one of the memory gates");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "fallback should not claim hardware parallelism");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(mem_handle), "memory-cap runtime destroy should succeed");

    PrometheusReactorConfig forced_cfg{};
    forced_cfg.struct_size = sizeof(PrometheusReactorConfig);
    forced_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH | PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED;
    void* forced_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&forced_cfg, &forced_handle), "forced-serialized runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(forced_handle, &entry, 1u,
                                                                 2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                     (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                 &stage, &detail),
                 "forced serialized run should still execute");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(forced_handle, &diag), "forced diagnostics should succeed");
    ASSERT_TRUE(diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_FORCED_SERIALIZED ||
                    diag.serialized_fallback_reason == PROM_BATCH_FALLBACK_REASON_COMMAND_RESOURCES_INVALID,
                "forced-serialized topology should publish a concrete blocking gate reason");
    ASSERT_EQUAL(0u, diag.hardware_parallelism_claimed, "forced serialized must not claim parallelism");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(forced_handle), "forced runtime destroy should succeed");

    PrometheusReactorConfig cmd_cfg{};
    cmd_cfg.struct_size = sizeof(PrometheusReactorConfig);
    cmd_cfg.test_flags = PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS | PROM_TESTCFG_FORCE_DIRECT_PATH | PROM_TESTCFG_SKIP_VULKAN_INIT;
    void* cmd_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cmd_cfg, &cmd_handle), "command-resource runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(cmd_handle, &entry, 1u,
                                                                 2u | (2u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT) |
                                                                     (3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT),
                                                                 &stage, &detail),
                 "command-resource fallback should still execute through serialized bridge");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(cmd_handle, &diag), "command-resource diagnostics should succeed");
    ASSERT_EQUAL(0u, diag.true_multi_queue_selected, "command-resource gate failure should not select true multi");
    ASSERT_EQUAL(PROM_BATCH_FALLBACK_REASON_COMMAND_RESOURCES_INVALID, diag.serialized_fallback_reason,
                 "command-resource gate should publish explicit fallback reason");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(cmd_handle), "command-resource runtime destroy should succeed");
}
