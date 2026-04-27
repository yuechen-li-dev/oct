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
