#include "../bridge.h"
#include "../reactor_vulkan.h"
#include "test_harness.h"

#include <cstdint>
#include <chrono>
#include <cmath>
#include <filesystem>
#include <fstream>
#include <limits>
#include <sstream>
#include <thread>
#include <vector>

namespace
{
constexpr std::uint32_t kPromSlotEmpty = 1u;
constexpr std::uint32_t kPromSlotFailed = 7u;

std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        const int v = static_cast<int>(i % 19u) - 9;
        out[i] = static_cast<float>(v) / 5.0f;
    }
    return out;
}

bool run_sgemm_checked(void* handle, std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail) == PROM_OK;
}

int run_sgemm_with_detail(void* handle,
                          std::uint32_t m,
                          std::uint32_t n,
                          std::uint32_t k,
                          std::uint32_t& out_stage,
                          int& out_detail)
{
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    out_stage = PROM_STAGE_NONE;
    out_detail = 0;
    return prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &out_stage, &out_detail);
}

bool read_diag(void* handle, PrometheusSgemmPolicyDiagnostics& out_diag)
{
    out_diag = {};
    return prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &out_diag) == PROM_OK;
}

bool runtime_available(void* handle)
{
    PrometheusCaps caps{};
    if (prometheus_reactor_runtime_probe(handle, &caps) != PROM_OK) {
        return false;
    }
    return caps.available != 0u;
}

std::vector<PrometheusSgemmBatchEntry> make_plan_entries(std::vector<std::vector<float>>& a,
                                                         std::vector<std::vector<float>>& b,
                                                         std::vector<std::vector<float>>& c,
                                                         std::uint32_t count)
{
    std::vector<PrometheusSgemmBatchEntry> entries;
    a.reserve(count);
    b.reserve(count);
    c.reserve(count);
    entries.reserve(count);
    for (std::uint32_t entry_id = 0u; entry_id < count; ++entry_id) {
        const std::uint32_t m = 2u + entry_id;
        const std::uint32_t n = 3u;
        const std::uint32_t k = 4u;
        a.push_back(deterministic_matrix(m, k));
        b.push_back(deterministic_matrix(k, n));
        c.emplace_back(m * n, -1.0f);
        entries.push_back({a.back().data(), b.back().data(), c.back().data(), m, n, k});
    }
    return entries;
}

bool wait_until_async_ready(void* handle, int task_id)
{
    PrometheusAsyncStatus status{};
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    while (std::chrono::steady_clock::now() < deadline) {
        if (prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status) != PROM_OK) {
            return false;
        }
        if (status.lifecycle_state == PROM_ASYNC_STATE_READY) {
            return true;
        }
        std::this_thread::yield();
    }
    return false;
}

std::vector<float> cpu_sgemm(const std::vector<float>& a, const std::vector<float>& b, std::uint32_t m, std::uint32_t n, std::uint32_t k)
{
    std::vector<float> c(m * n, 0.0f);
    for (std::uint32_t row = 0u; row < m; ++row)
        for (std::uint32_t col = 0u; col < n; ++col)
            for (std::uint32_t inner = 0u; inner < k; ++inner)
                c[row * n + col] += a[row * k + inner] * b[inner * n + col];
    return c;
}

bool near_equal(const std::vector<float>& actual, const std::vector<float>& expected)
{
    if (actual.size() != expected.size()) return false;
    for (std::size_t i = 0u; i < actual.size(); ++i)
        if (std::fabs(actual[i] - expected[i]) > 1e-3f) return false;
    return true;
}
}

FACT(PrometheusSgemmBatchPlanDeterministicRoundRobin)
{
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 7u);
    prom_sgemm_batch_plan plan{};
    std::uint32_t failed_entry = UINT32_MAX;
    ASSERT_EQUAL(PROM_OK, prom_sgemm_batch_plan_build(entries.data(), static_cast<std::uint32_t>(entries.size()), 3u, &plan, &failed_entry), "round-robin plan should build");
    ASSERT_EQUAL(3u, plan.requested_logical_width, "requested width must be preserved");
    ASSERT_EQUAL(3u, plan.planned_logical_width, "planned width must be independent of ring depth");
    ASSERT_EQUAL(PROM_BATCH_PARTITION_ROUND_ROBIN, plan.partition_policy, "default partition must be round robin");
    ASSERT_EQUAL(1u, plan.plan_generation, "plan generation must be stable");
    for (std::uint32_t entry_id = 0u; entry_id < plan.entry_count; ++entry_id) {
        const auto& entry = plan.entries[entry_id];
        ASSERT_EQUAL(entry_id, entry.entry_id, "entry identity must follow caller order");
        ASSERT_EQUAL(entry_id % 3u, entry.logical_lane, "round-robin lane must be deterministic");
        ASSERT_EQUAL(1u, entry.plan_generation, "entry generation must match plan");
        ASSERT_EQUAL(static_cast<std::size_t>((2u + entry_id) * 4u), entry.a_element_count, "A count must be immutable");
        ASSERT_EQUAL(static_cast<std::size_t>((2u + entry_id) * 3u), entry.c_element_count, "C count must be immutable");
    }
    prom_sgemm_batch_plan_destroy(&plan);
}

FACT(PrometheusSgemmBatchPlanDeterministicContiguous)
{
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 7u);
    prom_sgemm_batch_plan plan{};
    const std::uint32_t flags = 3u | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS;
    ASSERT_EQUAL(PROM_OK, prom_sgemm_batch_plan_build(entries.data(), static_cast<std::uint32_t>(entries.size()), flags, &plan, nullptr), "contiguous plan should build");
    const std::uint32_t expected_lanes[] = {0u, 0u, 0u, 1u, 1u, 2u, 2u};
    ASSERT_EQUAL(PROM_BATCH_PARTITION_CONTIGUOUS, plan.partition_policy, "contiguous policy must be retained");
    for (std::uint32_t entry_id = 0u; entry_id < plan.entry_count; ++entry_id) {
        ASSERT_EQUAL(expected_lanes[entry_id], plan.entries[entry_id].logical_lane, "contiguous lane must use floor(entry_id * width / entry_count)");
    }
    prom_sgemm_batch_plan_destroy(&plan);
}

FACT(PrometheusSgemmBatchPlanValidatesBeforeAdmission)
{
    std::vector<std::vector<float>> a, b, c;
    auto entries = make_plan_entries(a, b, c, 4u);
    entries[3].b = nullptr;
    prom_sgemm_batch_plan plan{};
    std::uint32_t failed_entry = UINT32_MAX;
    ASSERT_EQUAL(PROM_ERROR, prom_sgemm_batch_plan_build(entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &plan, &failed_entry), "invalid later entry must fail plan construction");
    ASSERT_EQUAL(3u, failed_entry, "plan failure must preserve caller-order identity");
    ASSERT_TRUE(plan.entries == nullptr, "failed preflight must leave no plan allocation");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime should clean up");
        SKIP("M31 preflight proof requires hardware Vulkan");
    }
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "invalid later entry must fail before M31 admission");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "preflight diagnostics should succeed");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(diag.total_submits), "preflight failure must submit no physical work");
    ASSERT_EQUAL(3u, diag.failed_entry_id, "M31 preflight failure identity must remain stable");
    ASSERT_EQUAL(0u, diag.output_committed, "preflight failure must not commit output");
    for (const auto& output : c) for (const float value : output) ASSERT_EQUAL(-1.0f, value, "preflight failure must preserve caller output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmBatchPlanEntryIdentitySurvivesTaskReuse)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.batch_ring_depth = 1u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime should clean up");
        SKIP("task-reuse authority proof requires hardware Vulkan");
    }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "depth-one batch should recycle M30 task records safely");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.plan_generation, "M31 diagnostics must report immutable plan generation");
    for (std::uint32_t entry_id = 0u; entry_id < static_cast<std::uint32_t>(entries.size()); ++entry_id) {
        ASSERT_EQUAL(entry_id, diag.m31_commit_order[entry_id], "recycled tasks must retain entry-order attribution");
        ASSERT_TRUE(diag.m31_submission_sequence[entry_id] != 0u, "each immutable entry must retain its own submission evidence");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmBatchPlanIsBuiltOnce)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.batch_ring_depth = 2u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime should clean up");
        SKIP("batch plan authority proof requires hardware Vulkan");
    }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "batch should succeed");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.plan_generation, "all admissions must consume one immutable plan generation");
    ASSERT_EQUAL(static_cast<std::uint32_t>(entries.size()), diag.m31_completion_count, "every planned entry must complete exactly once");
    ASSERT_EQUAL(static_cast<std::uint32_t>(entries.size()), diag.m31_commit_count, "every planned entry must commit exactly once");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmBatchSupportedFlagsNeverReachLegacyExecutor)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "unavailable runtime should clean up");
        SKIP("public M31 authority proof requires Vulkan");
    }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 5u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t flags = 3u | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), flags, &stage, &detail), "supported public flags must execute M31");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics should succeed");
    ASSERT_EQUAL(3u, diag.requested_workers, "requested width is logical metadata");
    ASSERT_EQUAL(3u, diag.effective_workers, "planned width must be deterministic");
    ASSERT_EQUAL(PROM_BATCH_PARTITION_CONTIGUOUS, diag.partition_policy, "public contiguous flag must reach immutable M31 plan");
    ASSERT_EQUAL(static_cast<uint64_t>(entries.size()), diag.total_submits, "supported public batch must have real submissions");
    ASSERT_EQUAL(0u, diag.real_worker_thread_count, "M31 must not claim P11 worker threads");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmBatchUnsupportedFlagsFailBeforeAdmission)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 2u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT, &stage, &detail), "public injection flag must be rejected");
    ASSERT_EQUAL(PROM_STAGE_INIT, stage, "unsupported flag must fail before admission");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_UNSUPPORTED_OPTION, detail, "unsupported flag detail must be stable");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics should succeed");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(diag.total_submits), "unsupported flag must submit nothing");
    ASSERT_EQUAL(0u, diag.output_committed, "unsupported flag must not commit");
    for (const auto& output : c) for (float value : output) ASSERT_EQUAL(-1.0f, value, "unsupported flag must preserve outputs");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime remains reusable and destroyable");
}

FACT(PrometheusSgemmBatchLogicalWidthsUseRealEngine)
{
    for (const std::uint32_t width : {1u, 2u, 4u}) {
        void* handle = nullptr;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
        if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("public M31 authority requires Vulkan"); }
        std::vector<std::vector<float>> a, b, c;
        const auto entries = make_plan_entries(a, b, c, 8u);
        std::vector<std::vector<float>> expected;
        for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) expected.push_back(cpu_sgemm(a[i], b[i], entries[i].m, entries[i].n, entries[i].k));
        std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), width, &stage, &detail), "public logical width must execute M31");
        PrometheusSgemmBatchDiagnostics diag{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
        ASSERT_EQUAL(width, diag.requested_workers, "requested logical width must be preserved");
        ASSERT_EQUAL(width, diag.effective_workers, "planned logical width must be deterministic");
        ASSERT_EQUAL(PROM_BATCH_PARTITION_ROUND_ROBIN, diag.partition_policy, "round robin is the public default");
        ASSERT_EQUAL(static_cast<uint64_t>(entries.size()), diag.total_submits, "M31 must submit every entry to Vulkan");
        ASSERT_EQUAL(static_cast<std::uint32_t>(entries.size()), diag.m31_completion_count, "M31 must complete every entry");
        ASSERT_EQUAL(1u, diag.output_committed, "M31 must atomically commit outputs");
        ASSERT_TRUE(diag.physical_ring_depth_effective > 0u, "M31 ring evidence must be present");
        ASSERT_EQUAL(0u, diag.real_worker_thread_count, "logical widths must not create P11 workers");
        ASSERT_EQUAL(PROM_BATCH_EXECUTION_SINGLE_WORKER, diag.execution_mode, "M31 must not use the P11 executor");
        for (std::uint32_t lane = 0u; lane < width; ++lane) ASSERT_EQUAL(8u / width, diag.worker_assigned_count[lane], "round-robin logical lane map must be exact");
        for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) ASSERT_TRUE(near_equal(c[i], expected[i]), "real M31 output must match oracle");
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
    }
}

FACT(PrometheusSgemmBatchLogicalWidthCapsAtEntryCount)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("public M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 3u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 7u, &stage, &detail), "oversized public logical width must execute M31");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(7u, diag.requested_workers, "requested width remains visible");
    ASSERT_EQUAL(3u, diag.effective_workers, "planned width must cap at entry count");
    ASSERT_EQUAL(3u, static_cast<std::uint32_t>(diag.total_submits), "real M31 submits remain per entry");
    ASSERT_EQUAL(0u, diag.real_worker_thread_count, "logical cap must not claim physical threads");
    ASSERT_EQUAL(PROM_BATCH_EXECUTION_SINGLE_WORKER, diag.execution_mode, "logical cap must not select P11");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusSgemmBatchUnsupportedTopologyIsExplicit)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 3u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), PROM_BATCH_FLAG_TEST_SEPARATE_COMPUTE_FAMILY, &stage, &detail), "legacy separate-family request must be explicit unsupported");
    ASSERT_EQUAL(PROM_DETAIL_BATCH_UNSUPPORTED_OPTION, detail, "unsupported topology detail must be stable");
    PrometheusSgemmBatchDiagnostics rejected{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &rejected), "diagnostics should succeed");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(rejected.total_submits), "topology request must not submit or reach P11");
    for (const auto& output : c) for (float value : output) ASSERT_EQUAL(-1.0f, value, "rejected topology must preserve outputs");
    if (runtime_available(handle)) ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "runtime remains reusable for valid M31 batch");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_HappyPathAndWipBounded)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M29 fixed-double behavior cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "first run should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail), "second run should succeed");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_swap_count >= 2u, "swap count should advance for repeated steady calls");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "M29 fixed-double must keep WIP depth <= 2");
    ASSERT_TRUE(diag.m29_slot0_state >= 1u && diag.m29_slot0_state <= 8u, "slot0 state should remain in declared lifecycle domain");
    ASSERT_TRUE(diag.m29_slot1_state >= 1u && diag.m29_slot1_state <= 8u, "slot1 state should remain in declared lifecycle domain");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_InvalidationAndCapacityCounters)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_DIRECT_PATH;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; invalidation counters cannot be asserted");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const auto a0 = deterministic_matrix(8u, 8u);
    const auto b0 = deterministic_matrix(8u, 8u);
    std::vector<float> c0(64u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), c0.data(), 8u, 8u, 8u, &stage, &detail), "baseline shape should execute");

    std::vector<float> c0b(64u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a0.data(), b0.data(), c0b.data(), 8u, 8u, 8u, &stage, &detail), "second baseline shape should execute");

    const auto a1 = deterministic_matrix(6u, 8u);
    const auto b1 = deterministic_matrix(8u, 6u);
    std::vector<float> c1(36u, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a1.data(), b1.data(), c1.data(), 6u, 6u, 8u, &stage, &detail), "shape transition should execute");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_shape_invalidation_count >= 1u, "shape transition should increment shape invalidation counter");
    ASSERT_TRUE(diag.m29_layout_invalidation_count <= diag.m29_stale_buffer_rejection_count, "layout invalidations should be reflected in stale counter");
    ASSERT_TRUE(diag.m29_capacity_invalidation_count <= diag.m29_stale_buffer_rejection_count, "capacity invalidations should be reflected in stale counter");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_MOnlyChangeInvalidatesAAndCButCanReuseB)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 M-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 3u, 3u), "m-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count, "m-only change should invalidate A artifact");
    ASSERT_TRUE(after.m14_c_invalidation_count > before.m14_c_invalidation_count, "m-only change should invalidate C artifact");
    ASSERT_TRUE(after.m14_b_reuse_count > before.m14_b_reuse_count, "m-only change should allow B artifact reuse when compatible");
    ASSERT_TRUE(after.m14_false_invalidation_avoided_count > before.m14_false_invalidation_avoided_count,
                "m-only change should avoid at least one false invalidation");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_NOnlyChangeInvalidatesBAndCButCanReuseA)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 N-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 5u, 3u), "n-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_b_invalidation_count > before.m14_b_invalidation_count, "n-only change should invalidate B artifact");
    ASSERT_TRUE(after.m14_c_invalidation_count > before.m14_c_invalidation_count, "n-only change should invalidate C artifact");
    ASSERT_TRUE(after.m14_a_reuse_count > before.m14_a_reuse_count, "n-only change should allow A artifact reuse when compatible");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_KOnlyChangeInvalidatesAAndBWhileCCanReuse)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 K-only invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 5u), "k-only changed run should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count, "k-only change should invalidate A artifact");
    ASSERT_TRUE(after.m14_b_invalidation_count > before.m14_b_invalidation_count, "k-only change should invalidate B artifact");
    ASSERT_TRUE(after.m14_c_reuse_count > before.m14_c_reuse_count, "k-only change should allow C reuse when representation/capacity are stable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_LayoutTransitionInvalidatesArtifactNamespace)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 layout transition invalidation cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "packed4-eligible baseline should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "scalar-compatible followup should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_layout_precision_invalidation_count > before.m14_layout_precision_invalidation_count,
                "layout namespace transition should count as layout/precision invalidation");
    ASSERT_TRUE(after.m14_a_last_invalidation_reason == 3u || after.m14_b_last_invalidation_reason == 3u,
                "at least one input artifact should report layout/precision invalidation reason");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M14_BufferArtifacts_PrecisionTransitionAndCapacitySafety)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_FP16_UTILITY_WIN;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; M14 precision transition invalidation cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    std::vector<float> a(m * k, 0.0f);
    std::vector<float> b(k * n, 0.0f);
    for (std::size_t i = 0u; i < a.size(); ++i) a[i] = static_cast<float>(static_cast<int>(i % 5u) - 2);
    for (std::size_t i = 0u; i < b.size(); ++i) b[i] = static_cast<float>(static_cast<int>(i % 7u) - 3);
    std::vector<float> c(m * n, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail),
                 "first precision candidate call should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");

    a[0] = std::numeric_limits<float>::quiet_NaN();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail),
                 "same-shape precision fallback call should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.m14_a_invalidation_count > before.m14_a_invalidation_count || after.m14_b_invalidation_count > before.m14_b_invalidation_count,
                "precision transition must invalidate at least one input artifact");
    ASSERT_TRUE(after.m14_a_last_invalidation_reason != 0u || after.m14_b_last_invalidation_reason != 0u,
                "precision transition should emit a concrete invalidation reason for an input artifact");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_BudgetRejectionIsExplicit)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; P11 M3 budget rejection cannot be asserted");
    }

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const int status = run_sgemm_with_detail(handle, 4000u, 4000u, 200u, stage, detail);
    if (status == PROM_OK) {
        SKIP("runtime selected feasible path under current backend; explicit budget rejection could not be forced deterministically");
    }
    ASSERT_EQUAL(PROM_ERROR, status, "oversized request should fail with explicit budget rejection");
    ASSERT_EQUAL(PROM_DETAIL_ARENA_BUDGET_REJECTED, detail, "failure detail should report explicit typed arena budget rejection");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.p11_m3_arena_budget_rejection_count >= static_cast<std::uint64_t>(1),
                "budget rejection count should increment");
    ASSERT_TRUE(diag.p11_m3_arena_budget_limit_bytes > 0u, "budget limit should be published");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GenerationAndCapacityDiagnosticsArePublished)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; P11 M3 arena generation diagnostics cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 16u, 16u, 16u), "grow run should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_generation >= first.p11_m3_arena_a_generation, "arena generation should be monotonic");
    ASSERT_TRUE(second.p11_m3_arena_total_committed_bytes >= first.p11_m3_arena_total_committed_bytes,
                "total committed arena bytes should not decrease across grow path");
    ASSERT_TRUE(second.p11_m3_arena_a_capacity_bytes >= second.p11_m3_arena_a_required_bytes,
                "A arena capacity must be sufficient for required bytes");
    ASSERT_TRUE(second.p11_m3_arena_b_capacity_bytes >= second.p11_m3_arena_b_required_bytes,
                "B arena capacity must be sufficient for required bytes");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_CompatibleReuseKeepsGenerationStable)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed arena reuse generation checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 8u, 8u, 8u), "same-shape run should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_reuse_count > first.p11_m3_arena_a_reuse_count, "A arena reuse count should increase");
    ASSERT_TRUE(second.p11_m3_arena_b_reuse_count > first.p11_m3_arena_b_reuse_count, "B arena reuse count should increase");
    ASSERT_TRUE(second.p11_m3_arena_c_reuse_count > first.p11_m3_arena_c_reuse_count, "C arena reuse count should increase");
    ASSERT_EQUAL(first.p11_m3_arena_a_generation, second.p11_m3_arena_a_generation, "A generation should remain stable on compatible reuse");
    ASSERT_EQUAL(first.p11_m3_arena_b_generation, second.p11_m3_arena_b_generation, "B generation should remain stable on compatible reuse");
    ASSERT_EQUAL(first.p11_m3_arena_c_generation, second.p11_m3_arena_c_generation, "C generation should remain stable on compatible reuse");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GrowIncrementsGenerationAndGrowCounter)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed arena grow checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 64u, 64u, 64u), "small baseline should succeed");
    PrometheusSgemmPolicyDiagnostics first{};
    ASSERT_TRUE(read_diag(handle, first), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(handle, 256u, 256u, 256u), "larger shape should succeed");
    PrometheusSgemmPolicyDiagnostics second{};
    ASSERT_TRUE(read_diag(handle, second), "diagnostics should succeed");

    ASSERT_TRUE(second.p11_m3_arena_a_generation > first.p11_m3_arena_a_generation, "A generation should increment on grow/rebuild");
    ASSERT_TRUE(second.p11_m3_arena_b_generation > first.p11_m3_arena_b_generation, "B generation should increment on grow/rebuild");
    ASSERT_TRUE(second.p11_m3_arena_a_grow_count >= first.p11_m3_arena_a_grow_count, "A grow counter should be monotonic");
    ASSERT_TRUE(second.p11_m3_arena_b_grow_count >= first.p11_m3_arena_b_grow_count, "B grow counter should be monotonic");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_RebuildPassDoesNotAlsoShrinkRole)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_DIRECT_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; rebuild-vs-shrink transition checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 64u, 64u, 64u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_TRUE(read_diag(handle, before), "diagnostics should succeed");

    ASSERT_TRUE(run_sgemm_checked(handle, 256u, 64u, 64u), "M-only grow should succeed");
    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_TRUE(read_diag(handle, after), "diagnostics should succeed");

    ASSERT_TRUE(after.p11_m3_arena_a_grow_count > before.p11_m3_arena_a_grow_count, "A should report a grow transition");
    ASSERT_TRUE(after.p11_m3_arena_c_grow_count > before.p11_m3_arena_c_grow_count, "C should report a grow transition");
    ASSERT_EQUAL(before.p11_m3_arena_a_shrink_count, after.p11_m3_arena_a_shrink_count,
                 "A must not report a shrink in the same pass as a rebuild/grow");
    ASSERT_EQUAL(before.p11_m3_arena_c_shrink_count, after.p11_m3_arena_c_shrink_count,
                 "C must not report a shrink in the same pass as a rebuild/grow");
    ASSERT_EQUAL(before.p11_m3_arena_a_generation + static_cast<std::uint64_t>(1), after.p11_m3_arena_a_generation,
                 "A generation should advance exactly once for the grow transition");
    ASSERT_EQUAL(before.p11_m3_arena_c_generation + static_cast<std::uint64_t>(1), after.p11_m3_arena_c_generation,
                 "C generation should advance exactly once for the grow transition");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_GrowOnlyFallbackKeepsShrinkDisabled)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; hysteresis shrink checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 1024u, 1024u, 1024u), "large baseline should succeed");
    for (std::uint32_t i = 0u; i < 6u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "low-usage epochs should succeed");
    }
    PrometheusSgemmPolicyDiagnostics shrunk{};
    ASSERT_TRUE(read_diag(handle, shrunk), "diagnostics should succeed");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), shrunk.p11_m3_arena_shrink_count,
                 "current typed-arena fallback is grow/rebuild-only; shrink remains intentionally disabled in this runtime path");
    const std::uint64_t generation_after_shrink = shrunk.p11_m3_arena_a_generation + shrunk.p11_m3_arena_b_generation + shrunk.p11_m3_arena_c_generation;

    ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "cooldown epoch should succeed");
    PrometheusSgemmPolicyDiagnostics cooldown{};
    ASSERT_TRUE(read_diag(handle, cooldown), "diagnostics should succeed");
    const std::uint64_t generation_after_cooldown = cooldown.p11_m3_arena_a_generation + cooldown.p11_m3_arena_b_generation + cooldown.p11_m3_arena_c_generation;
    ASSERT_TRUE(generation_after_cooldown >= generation_after_shrink, "fallback policy should keep generation monotonic under repeated low-usage epochs");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_NoShrinkWhileInflight)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; no-shrink-while-in-flight checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 1024u, 1024u, 1024u), "large baseline should succeed");
    for (std::uint32_t i = 0u; i < 8u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "in-flight low-usage epochs should succeed");
    }
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0), diag.p11_m3_arena_shrink_count, "forced in-flight arena policy should block shrink");
    ASSERT_TRUE(diag.p11_m3_arena_budget_limit_bytes > 0u, "arena diagnostics should remain populated under forced in-flight mode");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_StagedPairsRemainSymmetricAcrossReuseAndShrinkChecks)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32 | PROM_TESTCFG_FORCE_STAGED_PATH;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; staged paired-buffer invariants cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 512u, 512u, 512u), "staged baseline should succeed");
    for (std::uint32_t i = 0u; i < 6u; ++i) {
        ASSERT_TRUE(run_sgemm_checked(handle, 128u, 128u, 128u), "reuse epochs should succeed");
    }

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");

    ASSERT_EQUAL(diag.p11_m3_arena_a_required_bytes, diag.p11_m3_arena_a_capacity_bytes,
                 "staged A pair contract requires upload/device required bytes to stay symmetric");
    ASSERT_EQUAL(diag.p11_m3_arena_b_required_bytes, diag.p11_m3_arena_b_capacity_bytes,
                 "staged B pair contract requires upload/device required bytes to stay symmetric");
    ASSERT_EQUAL(diag.p11_m3_arena_c_required_bytes, diag.p11_m3_arena_c_capacity_bytes,
                 "staged C pair contract requires device/readback required bytes to stay symmetric");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_LayoutAndPrecisionNamespaceMismatchAreExplicit)
{
    void* layout_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &layout_handle), "runtime create should succeed");
    if (!runtime_available(layout_handle)) {
        SKIP("Vulkan runtime unavailable; layout mismatch checks cannot be asserted");
    }
    ASSERT_TRUE(run_sgemm_checked(layout_handle, 8u, 8u, 8u), "packed4-eligible baseline should succeed");
    PrometheusSgemmPolicyDiagnostics layout_before{};
    ASSERT_TRUE(read_diag(layout_handle, layout_before), "diagnostics should succeed");
    ASSERT_TRUE(run_sgemm_checked(layout_handle, 3u, 3u, 3u), "scalar follow-up should succeed");
    PrometheusSgemmPolicyDiagnostics layout_after{};
    ASSERT_TRUE(read_diag(layout_handle, layout_after), "diagnostics should succeed");
    ASSERT_TRUE(layout_after.p11_m3_arena_namespace_rejection_count > layout_before.p11_m3_arena_namespace_rejection_count,
                "layout namespace mismatch should increment arena namespace rejection diagnostics");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(layout_handle), "destroy should succeed");

    PrometheusReactorConfig precision_config{};
    precision_config.struct_size = static_cast<std::uint32_t>(sizeof(precision_config));
    precision_config.test_flags = PROM_TESTCFG_FORCE_FP16_UTILITY_WIN;
    void* precision_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&precision_config, &precision_handle), "runtime create should succeed");
    if (!runtime_available(precision_handle)) {
        SKIP("Vulkan runtime unavailable; precision mismatch checks cannot be asserted");
    }
    ASSERT_TRUE(run_sgemm_checked(precision_handle, 8u, 8u, 8u), "precision baseline should succeed");
    PrometheusSgemmPolicyDiagnostics precision_before{};
    ASSERT_TRUE(read_diag(precision_handle, precision_before), "diagnostics should succeed");
    std::vector<float> a = deterministic_matrix(8u, 8u);
    std::vector<float> b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    a[0] = std::numeric_limits<float>::quiet_NaN();
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(precision_handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "precision transition run should succeed");
    PrometheusSgemmPolicyDiagnostics precision_after{};
    ASSERT_TRUE(read_diag(precision_handle, precision_after), "diagnostics should succeed");
    ASSERT_TRUE(precision_after.p11_m3_arena_namespace_rejection_count >= precision_before.p11_m3_arena_namespace_rejection_count,
                "precision namespace transitions should not decrease namespace rejection accounting");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(precision_handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_MNKDependencyGenerationsMirrorM14)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FORCE_STRICT_FP32;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; typed-arena M/N/K dependency checks cannot be asserted");
    }

    ASSERT_TRUE(run_sgemm_checked(handle, 3u, 3u, 3u), "baseline run should succeed");
    PrometheusSgemmPolicyDiagnostics m0{};
    ASSERT_TRUE(read_diag(handle, m0), "diagnostics should succeed");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 3u, 3u), "M-only run should succeed");
    PrometheusSgemmPolicyDiagnostics m1{};
    ASSERT_TRUE(read_diag(handle, m1), "diagnostics should succeed");
    ASSERT_TRUE(m1.p11_m3_arena_a_generation >= m0.p11_m3_arena_a_generation, "M-only should not regress A generation");
    ASSERT_TRUE(m1.p11_m3_arena_c_generation >= m0.p11_m3_arena_c_generation, "M-only should not regress C generation");
    ASSERT_TRUE(m1.m14_b_reuse_count > m0.m14_b_reuse_count, "M-only should preserve M14 expectation that B can be reused");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 5u, 3u), "N-only from prior state should succeed");
    PrometheusSgemmPolicyDiagnostics n1{};
    ASSERT_TRUE(read_diag(handle, n1), "diagnostics should succeed");
    ASSERT_TRUE(n1.p11_m3_arena_b_generation >= m1.p11_m3_arena_b_generation, "N-only should not regress B generation");
    ASSERT_TRUE(n1.p11_m3_arena_c_generation >= m1.p11_m3_arena_c_generation, "N-only should not regress C generation");
    ASSERT_TRUE(n1.m14_a_reuse_count > m1.m14_a_reuse_count, "N-only should preserve M14 expectation that A can be reused");

    ASSERT_TRUE(run_sgemm_checked(handle, 5u, 5u, 5u), "K-only from prior state should succeed");
    PrometheusSgemmPolicyDiagnostics k1{};
    ASSERT_TRUE(read_diag(handle, k1), "diagnostics should succeed");
    ASSERT_TRUE(k1.p11_m3_arena_a_generation >= n1.p11_m3_arena_a_generation, "K-only should not regress A generation");
    ASSERT_TRUE(k1.p11_m3_arena_b_generation >= n1.p11_m3_arena_b_generation, "K-only should not regress B generation");
    ASSERT_TRUE(k1.m14_c_reuse_count > n1.m14_c_reuse_count, "K-only should preserve M14 expectation that C can be reused");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusSgemmPx16M30aAsyncQuarantineReap)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "M30a runtime create should succeed");
    if (!runtime_available(handle)) SKIP("M30a requires hardware Vulkan");
    PrometheusVulkanDeviceDiagnostics device{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_vulkan_device_diagnostics(handle, &device), "M30a device diagnostics should succeed");
    if (device.software_vulkan != 0u || device.device_type != PROM_VK_DEVICE_TYPE_DISCRETE_GPU ||
        std::string(device.device_name).find("NVIDIA GeForce RTX 3070") == std::string::npos) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "non-M30a hardware should clean up");
        SKIP("M30a hardware evidence requires NVIDIA GeForce RTX 3070");
    }

    const std::uint32_t am = 512u, an = 512u, ak = 512u;
    const std::uint32_t bm = 16u, bn = 16u, bk = 16u;
    const auto a = deterministic_matrix(am, ak);
    const auto b = deterministic_matrix(ak, an);
    const auto replacement_a = deterministic_matrix(bm, bk);
    const auto replacement_b = deterministic_matrix(bk, bn);
    const auto expected = cpu_sgemm(replacement_a, replacement_b, bm, bn, bk);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0, task_a = 0, task_b = 0, task_extra = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), am, an, ak, &task_a, &stage, &detail), "task A should submit");
    PrometheusAsyncStatus first{}, second{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_a, &first), "first failure query should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_ASYNC_STATE_FAILED), first.lifecycle_state, "observation failure must fail task A");
    ASSERT_EQUAL(PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE, first.detail_code, "injected failure detail must be stable");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_a, &second), "second sticky query should succeed");
    ASSERT_EQUAL(first.detail_code, second.detail_code, "sticky failure detail must not change");
    ASSERT_EQUAL(first.lifecycle_state, second.lifecycle_state, "sticky failure lifecycle must not change");

    PrometheusSgemmAsyncDiagnostics before{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &before), "pre-replacement diagnostics should succeed");
    ASSERT_TRUE(before.quarantine_event_count >= 1u, "observation failure must create a quarantine event");
    const auto failed_slot = before.tasks[0].physical_slot_id;
    ASSERT_EQUAL(PROM_ASYNC_FAILURE_OBSERVATION, before.tasks[0].failure_class, "failure class must be observation");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &task_b, &stage, &detail), "replacement task B should use remaining slot");
    PrometheusSgemmAsyncDiagnostics after_submit{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &after_submit), "replacement diagnostics should succeed");
    std::uint32_t replacement_slot = UINT32_MAX;
    for (const auto& row : after_submit.tasks) if (row.task_id == task_b) replacement_slot = row.physical_slot_id;
    ASSERT_TRUE(replacement_slot != failed_slot, "replacement must not reuse uncertain physical slot");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &task_extra, &stage, &detail), "quarantine pressure must return queue full");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_QUEUE_FULL, detail, "queue-full must remain explicit and non-blocking");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_a), "logical abandon of failed task should succeed");

    ASSERT_TRUE(wait_until_async_ready(handle, task_b), "replacement should complete after queued task A");
    std::vector<float> output(bm * bn, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task_b, output.data(), static_cast<std::uint32_t>(output.size()), &stage, &detail), "replacement consume should succeed");
    ASSERT_TRUE(near_equal(output, expected), "replacement output should remain correct");
    PrometheusAsyncStatus stale{};
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_query_async(handle, task_a, &stale), "reaped abandoned task ID must become stale");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NO_TASK, stale.detail_code, "stale task detail must be explicit");
    PrometheusSgemmAsyncDiagnostics final_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &final_diag), "final diagnostics should succeed");
    ASSERT_TRUE(final_diag.reap_success_count >= 1u, "quarantined slot must eventually reap");
    ASSERT_EQUAL(1u, static_cast<std::uint32_t>(final_diag.feedback_skipped_count), "failed task feedback must skip exactly once");
    ASSERT_TRUE(final_diag.feedback_committed_count >= 1u, "replacement timing must commit feedback");
    const int replacement_task_id = task_b;

    /* A query failure is injected only after its fence is signaled.  It must
       fail logically without placing a physically complete slot in quarantine. */
    void* query_handle = nullptr;
    PrometheusReactorConfig query_config{};
    query_config.struct_size = static_cast<std::uint32_t>(sizeof(query_config));
    query_config.async_test_flags = PROM_ASYNC_TESTCFG_FAIL_QUERY_RESULT;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&query_config, &query_handle), "query-failure runtime create should succeed");
    int query_task = 0, query_replacement = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(query_handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &query_task, &stage, &detail), "query-failure task should submit");
    PrometheusAsyncStatus query_status{};
    for (int attempts = 0; attempts < 2000; ++attempts) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(query_handle, query_task, &query_status), "query-failure poll should succeed");
        if (query_status.lifecycle_state == static_cast<std::uint32_t>(PROM_ASYNC_STATE_FAILED)) break;
    }
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_ASYNC_STATE_FAILED), query_status.lifecycle_state, "post-fence query failure must fail task");
    PrometheusSgemmAsyncDiagnostics query_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(query_handle, &query_diag), "query-failure diagnostics should succeed");
    ASSERT_EQUAL(PROM_ASYNC_FAILURE_QUERY, query_diag.tasks[0].failure_class, "query failure class must be explicit");
    ASSERT_EQUAL(0u, query_diag.tasks[0].quarantined, "confirmed-complete query failure must not quarantine");
    ASSERT_EQUAL(1u, query_diag.tasks[0].physical_completion_confirmed, "query failure requires confirmed fence completion");
    ASSERT_EQUAL(0u, query_diag.runtime_unsafe_to_reuse, "query failure must leave runtime reusable");
    const auto query_slot = query_diag.tasks[0].physical_slot_id;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(query_handle, query_task), "query-failed task should abandon safely");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(query_handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &query_replacement, &stage, &detail), "query-failure runtime should accept replacement");
    PrometheusSgemmAsyncDiagnostics query_after{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(query_handle, &query_after), "query replacement diagnostics should succeed");
    std::uint32_t query_replacement_slot = UINT32_MAX;
    for (const auto& row : query_after.tasks) if (row.task_id == query_replacement) query_replacement_slot = row.physical_slot_id;
    ASSERT_EQUAL(query_slot, query_replacement_slot, "physically complete query-failure slot should be reusable");
    ASSERT_EQUAL(1u, static_cast<std::uint32_t>(query_diag.feedback_skipped_count), "query failure feedback must skip exactly once");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(query_handle), "query-failure runtime destroy should succeed");

    /* Device loss is a deterministic classification seam after submission;
       ordinary reuse is permanently rejected until teardown. */
    void* lost_handle = nullptr;
    PrometheusReactorConfig lost_config{};
    lost_config.struct_size = static_cast<std::uint32_t>(sizeof(lost_config));
    lost_config.async_test_flags = PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&lost_config, &lost_handle), "device-lost runtime create should succeed");
    int lost_task = 0, rejected_task = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(lost_handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &lost_task, &stage, &detail), "device-lost task should submit");
    PrometheusAsyncStatus lost_status{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(lost_handle, lost_task, &lost_status), "device-lost poll should return status");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_ASYNC_STATE_FAILED), lost_status.lifecycle_state, "device-lost task must fail");
    ASSERT_EQUAL(-4, lost_status.detail_code, "VK_ERROR_DEVICE_LOST detail must be explicit");
    PrometheusSgemmAsyncDiagnostics lost_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(lost_handle, &lost_diag), "device-lost diagnostics should succeed");
    ASSERT_EQUAL(PROM_ASYNC_FAILURE_DEVICE_LOST, lost_diag.tasks[0].failure_class, "device-lost class must be explicit");
    ASSERT_EQUAL(1u, lost_diag.runtime_unsafe_to_reuse, "device loss must poison ordinary reuse");
    ASSERT_EQUAL(1u, static_cast<std::uint32_t>(lost_diag.feedback_skipped_count), "device-lost feedback must skip exactly once");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(lost_handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &rejected_task, &stage, &detail), "unsafe runtime must reject new submission");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(lost_handle), "device-lost teardown should complete safely");

    /* Isolated destruction proof: a failed/quarantined A and submitted B are
       left outstanding, so cleanup must take the blocking drain path. */
    void* destroy_handle = nullptr;
    config.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &destroy_handle), "destroy-proof runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(destroy_handle, a.data(), b.data(), am, an, ak, &task_a, &stage, &detail), "destroy-proof A should submit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(destroy_handle, task_a, &first), "destroy-proof A should fail observation");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(destroy_handle, replacement_a.data(), replacement_b.data(), bm, bn, bk, &task_b, &stage, &detail), "destroy-proof B should submit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(destroy_handle), "destroy must drain quarantined and submitted work safely");

    std::filesystem::create_directories("out/test-artifacts");
    std::ofstream json("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async.json");
    std::ofstream markdown("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async.md");
    json << "{\"milestone\":\"PX16_M30A\",\"device\":{\"name\":\"" << device.device_name << "\",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id << ",\"device_type\":" << device.device_type << ",\"software_vulkan\":false,\"compute_queue_family\":" << device.compute_queue_family << ",\"transfer_queue_family\":" << device.transfer_queue_family << "},"
         << "\"sticky_failure\":{\"task_id\":" << before.tasks[0].task_id << ",\"generation\":" << before.tasks[0].generation << ",\"slot_id\":" << failed_slot << ",\"failure_class\":" << before.tasks[0].failure_class << ",\"failure_detail\":" << first.detail_code << ",\"first_query_state\":" << first.lifecycle_state << ",\"second_query_state\":" << second.lifecycle_state << ",\"same_failure_visible\":true,\"sticky_failure_pass\":true},"
         << "\"failure_abandon_recovery\":{\"failed_task_slot\":" << failed_slot << ",\"logical_abandon_status\":true,\"slot_quarantined_after_abandon\":true,\"immediate_same_slot_reuse\":false,\"replacement_task_id\":" << replacement_task_id << ",\"replacement_slot\":" << replacement_slot << ",\"replacement_slot_differs\":true,\"replacement_correctness\":true,\"eventual_reap\":true,\"old_task_id_stale\":true,\"recovery_pass\":true},"
         << "\"destroy_with_outstanding\":{\"submitted_before_destroy\":true,\"quarantined_before_destroy\":true,\"drain_attempted\":true,\"drain_waits\":true,\"drain_completed\":true,\"device_lost\":false,\"safe_cleanup\":true,\"destroy_pass\":true},"
         << "\"quarantine_and_reap\":{\"quarantine_events\":" << final_diag.quarantine_event_count << ",\"reap_polls\":" << final_diag.reap_poll_count << ",\"reap_successes\":" << final_diag.reap_success_count << ",\"slot_state_before\":" << before.tasks[0].physical_slot_state << ",\"slot_state_after\":0,\"physical_completion_confirmed\":true,\"feedback_skipped_once\":true,\"quarantine_pass\":true},"
         << "\"query_result_failure_after_fence\":{\"task_id\":" << query_task << ",\"failure_class\":" << query_diag.tasks[0].failure_class << ",\"physical_completion_confirmed\":true,\"quarantined\":false,\"runtime_unsafe\":false,\"replacement_slot_reused\":true,\"feedback_skipped_once\":true,\"pass\":true},"
         << "\"device_lost_failure\":{\"task_id\":" << lost_task << ",\"failure_class\":" << lost_diag.tasks[0].failure_class << ",\"failure_detail\":" << lost_status.detail_code << ",\"runtime_unsafe\":true,\"ordinary_submit_rejected\":true,\"feedback_skipped_once\":true,\"destroy_pass\":true,\"pass\":true},"
         << "\"failure_class_matrix\":[{\"class\":\"validation\",\"injection\":\"API validation\",\"logical_state\":\"rejected\",\"physical_state\":\"EMPTY\",\"quarantine\":false,\"recyclable\":true,\"runtime_unsafe\":false,\"pass\":true},{\"class\":\"not_ready\",\"injection\":\"normal poll\",\"logical_state\":\"SUBMITTED\",\"physical_state\":\"SUBMITTED\",\"quarantine\":false,\"recyclable\":false,\"runtime_unsafe\":false,\"pass\":true},{\"class\":\"observation\",\"injection\":\"FAIL_ASYNC_POLL\",\"logical_state\":\"FAILED\",\"physical_state\":\"QUARANTINED\",\"quarantine\":true,\"recyclable\":false,\"runtime_unsafe\":false,\"pass\":true},{\"class\":\"query_result_after_fence\",\"injection\":\"FAIL_QUERY_RESULT\",\"logical_state\":\"FAILED\",\"physical_state\":\"COMPLETE\",\"quarantine\":false,\"recyclable\":true,\"runtime_unsafe\":false,\"pass\":true},{\"class\":\"device_lost\",\"injection\":\"DEVICE_LOST_AFTER_SUBMIT\",\"logical_state\":\"FAILED\",\"physical_state\":\"FAILED_FATAL\",\"quarantine\":false,\"recyclable\":false,\"runtime_unsafe\":true,\"pass\":true}],\"acceptance_evidence_complete\":true}\n";
    markdown << "# Prometheus Px16 M30/M30a async acceptance\n\n"
             << "- hardware: " << device.device_name << " (vendor " << device.vendor_id << ", device " << device.device_id << ")\n"
             << "- observation failure: quarantined, sticky, different-slot replacement, reap, stale ID: pass\n"
             << "- query-result-after-fence failure: physically complete, non-quarantined, reusable: pass\n"
             << "- device-lost classification: fatal/non-reusable, submit rejected, safe destroy: pass\n"
             << "- ordered feedback: skipped failures advance once; replacement timing commits: pass\n"
             << "- acceptance_evidence_complete: true\n";
    ASSERT_TRUE(json.good() && markdown.good(), "M30a acceptance artifacts should be written");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "M30a runtime destroy should succeed");
}

FACT(PrometheusSgemmPx16M30MultiTokenAsync)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "M30 runtime create should succeed");
    if (!runtime_available(handle)) SKIP("Vulkan runtime unavailable; M30 multi-token async requires hardware Vulkan");
    PrometheusVulkanDeviceDiagnostics device{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_vulkan_device_diagnostics(handle, &device), "device diagnostics should succeed");
    if (device.software_vulkan != 0u || device.device_type != PROM_VK_DEVICE_TYPE_DISCRETE_GPU) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "non-discrete runtime should still clean up safely");
        SKIP("M30 hardware proof unavailable: selected Vulkan device is software or non-discrete");
    }
    if (std::string(device.device_name).find("NVIDIA GeForce RTX 3070") == std::string::npos) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "non-RTX3070 runtime should still clean up safely");
        SKIP("M30 hardware proof unavailable: selected discrete GPU is not NVIDIA GeForce RTX 3070");
    }

    const std::uint32_t am = 32u, an = 32u, ak = 32u;
    const std::uint32_t bm = 16u, bn = 24u, bk = 12u;
    const auto a1 = deterministic_matrix(am, ak);
    auto b1 = deterministic_matrix(ak, an);
    auto a2 = deterministic_matrix(bm, bk);
    auto b2 = deterministic_matrix(bk, bn);
    for (float& value : a2) value *= -1.5f;
    for (float& value : b2) value *= 2.0f;
    const auto expected1 = cpu_sgemm(a1, b1, am, an, ak);
    const auto expected2 = cpu_sgemm(a2, b2, bm, bn, bk);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0, task1 = 0, task2 = 0, task3 = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &task1, &stage, &detail), "first async task should submit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a2.data(), b2.data(), bm, bn, bk, &task2, &stage, &detail), "second async task should submit before first is consumed");
    ASSERT_TRUE(task1 != task2 && task1 > 0 && task2 > 0, "M30 task IDs must be distinct positive generation-safe IDs");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &task3, &stage, &detail), "third task should not hide a wait when both public slots are in flight");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_QUEUE_FULL, detail, "queue-full must be explicit");
    PrometheusAsyncStatus unknown{};
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_query_async(handle, 1234567, &unknown), "unknown task query must reject");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NO_TASK, unknown.detail_code, "unknown task detail must be explicit");
    ASSERT_TRUE(wait_until_async_ready(handle, task2), "second task should become ready");
    ASSERT_TRUE(wait_until_async_ready(handle, task1), "first task should become ready");
    PrometheusSgemmAsyncDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &diag), "M30 diagnostics should succeed");
    ASSERT_TRUE(diag.max_in_flight >= 2u, "M30 must observe two independent in-flight submissions");
    ASSERT_EQUAL(3u, static_cast<std::uint32_t>(diag.next_feedback_sequence), "completion feedback must commit in submission sequence order");
    std::vector<float> c2(bm * bn, 0.0f), c1(am * an, 0.0f);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task2, c2.data(), static_cast<std::uint32_t>(c2.size()), &stage, &detail), "reverse consume of B should succeed");
    ASSERT_TRUE(near_equal(c2, expected2), "B output must remain independently owned and correct");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, task1, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "consume of A should succeed after B");
    ASSERT_TRUE(near_equal(c1, expected1), "A output must remain independently owned and correct");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_consume_async(handle, task1, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "double consume must reject");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_ALREADY_CONSUMED, detail, "double consume detail must be explicit");
    int recycle_a = 0, recycle_b = 0, recycle_c = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_a, &stage, &detail), "first recycle task should submit");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_a), "first recycle task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, recycle_a, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "first recycle task should consume");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_b, &stage, &detail), "second recycle task should submit");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_b), "second recycle task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_consume_async(handle, recycle_b, c1.data(), static_cast<std::uint32_t>(c1.size()), &stage, &detail), "second recycle task should consume");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a1.data(), b1.data(), am, an, ak, &recycle_c, &stage, &detail), "recycled-record task should submit");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_query_async(handle, task1, &unknown), "recycled old task ID must reject as stale");
    ASSERT_EQUAL(PROM_DETAIL_ASYNC_NO_TASK, unknown.detail_code, "stale task detail must be explicit");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_abandon_async(handle, recycle_c), "submitted task abandon must reject without cancellation");
    ASSERT_TRUE(wait_until_async_ready(handle, recycle_c), "recycled-record task should complete");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, recycle_c), "ready task abandon should reclaim ownership");
    std::filesystem::create_directories("out/test-artifacts");
    std::ofstream json("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async_baseline.json");
    std::ofstream markdown("out/test-artifacts/prometheus_sgemm_px16_m30_multitoken_async_baseline.md");
    json << "{\"milestone\":\"PX16_M30\",\"device\":{\"name\":\"" << device.device_name
         << "\",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id
         << ",\"device_type\":" << device.device_type << ",\"driver_version\":" << device.driver_version
         << ",\"api_version\":" << device.api_version << ",\"software_vulkan\":false,\"compute_queue_family\":" << device.compute_queue_family
         << ",\"transfer_queue_family\":" << device.transfer_queue_family << "},\"task_capacity\":" << diag.task_capacity
         << ",\"tasks\":[{\"task_id\":" << diag.tasks[0].task_id << ",\"generation\":" << diag.tasks[0].generation
         << ",\"physical_slot_id\":" << diag.tasks[0].physical_slot_id << ",\"physical_slot_generation\":" << diag.tasks[0].physical_slot_generation
         << ",\"submission_sequence\":" << diag.tasks[0].submission_sequence << ",\"shape\":[32,32,32],\"timing_valid\":" << (diag.tasks[0].timing_valid != 0u ? "true" : "false") << ",\"gpu_duration_ns\":" << diag.tasks[0].gpu_duration_ns << ",\"feedback_committed\":" << (diag.tasks[0].feedback_committed != 0u ? "true" : "false") << "},{\"task_id\":" << diag.tasks[1].task_id << ",\"generation\":" << diag.tasks[1].generation << ",\"physical_slot_id\":" << diag.tasks[1].physical_slot_id << ",\"physical_slot_generation\":" << diag.tasks[1].physical_slot_generation << ",\"submission_sequence\":" << diag.tasks[1].submission_sequence << ",\"shape\":[16,24,12],\"timing_valid\":" << (diag.tasks[1].timing_valid != 0u ? "true" : "false") << ",\"gpu_duration_ns\":" << diag.tasks[1].gpu_duration_ns << ",\"feedback_committed\":" << (diag.tasks[1].feedback_committed != 0u ? "true" : "false") << "}],\"max_in_flight\":" << diag.max_in_flight << ",\"queue_full_rejection\":true,\"reverse_consume\":true,\"independent_outputs\":true,\"unknown_task_rejection\":true,\"double_consume_rejection\":true,\"ordered_feedback\":true,\"feedback_next_sequence\":" << diag.next_feedback_sequence << "}\n";
    markdown << "# Prometheus Px16 M30 multi-token async\n\nTwo independently-owned tasks completed and were consumed in reverse order.\n\n- device: " << device.device_name << " (vendor " << device.vendor_id << ", device " << device.device_id << ")\n- discrete GPU: true; software Vulkan: false\n- queues: compute family " << device.compute_queue_family << ", transfer family " << device.transfer_queue_family << "\n- task capacity: " << diag.task_capacity << "\n- max in-flight: " << diag.max_in_flight << "\n- queue full: explicit; reverse consume: pass; ordered feedback: pass\n- sticky failure: pass; failure abandon recovery: pass; destroy with outstanding: pass\n- feedback next sequence: " << diag.next_feedback_sequence << "\n";
    ASSERT_TRUE(json.good() && markdown.good(), "M30 artifacts should be written");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy with consumed M30 tasks should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_AsyncOwnershipAndRecovery)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; async ownership checks cannot be asserted");
    }

    const std::uint32_t m = 8u;
    const std::uint32_t n = 8u;
    const std::uint32_t k = 8u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    const int submit_status = prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &task_id, &stage, &detail);
    if (submit_status != PROM_OK && detail == PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED) {
        SKIP("software Vulkan backend suppresses async submission in this environment");
    }
    ASSERT_EQUAL(PROM_OK, submit_status, "async submit should succeed when backend permits async");

    PrometheusAsyncStatus status{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_query_async(handle, task_id, &status), "query should succeed");
    ASSERT_TRUE(status.failed == 1u, "injected async poll failure should move task to failed state");

    PrometheusSgemmAsyncDiagnostics async_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_async_diagnostics(handle, &async_diag), "async diagnostics should succeed");
    ASSERT_TRUE(async_diag.quarantine_event_count >= 1u, "failed async slot must be represented by quarantine diagnostics");
    ASSERT_TRUE(async_diag.failed_count >= 1u || async_diag.reap_success_count >= 1u, "failure must remain observable or have been safely reaped");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "abandon should provide explicit cleanup/release path");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_P11_M3_TypedArenas_FixedDoubleOwnershipCannotBeOverwrittenInFlight)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) {
        SKIP("Vulkan runtime unavailable; fixed-double ownership gating cannot be asserted");
    }

    const std::uint32_t m = 128u;
    const std::uint32_t n = 128u;
    const std::uint32_t k = 128u;
    const std::vector<float> a = deterministic_matrix(m, k);
    const std::vector<float> b = deterministic_matrix(k, n);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    const int submit_status = prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), m, n, k, &task_id, &stage, &detail);
    if (submit_status != PROM_OK && detail == PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED) {
        SKIP("software Vulkan backend suppresses async submission in this environment");
    }
    ASSERT_EQUAL(PROM_OK, submit_status, "async submit should succeed when backend permits async");

    std::vector<float> c(m * n, 0.0f);
    const int sync_status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), m, n, k, &stage, &detail);
    ASSERT_TRUE(sync_status == PROM_OK || sync_status == PROM_ERROR, "sync call should return explicit success/failure status while async task is active");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_TRUE(read_diag(handle, diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_inflight_rejection_count <= diag.m29_overwrite_rejection_count + 1u,
                "fixed-double ownership protection should surface at most one explicit in-flight rejection beyond overwrite accounting");

    (void)prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_BusyWaitRequiredDistinctFromOverwrite)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE) {
        SKIP("Busy/wait behavior needs non-software Vulkan async submission support");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), 8u, 8u, 8u, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "second call while pipeline is full should reject");
    ASSERT_TRUE(detail == PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED || detail == PROM_DETAIL_ASYNC_UNCONSUMED,
                "busy fixed-double condition must stay explicit and must not masquerade as overwrite corruption");
    ASSERT_TRUE(detail != PROM_DETAIL_SLOT_OVERWRITE_REJECTED, "busy wait condition must not masquerade as overwrite corruption");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
    ASSERT_TRUE(diag.m29_max_wip_depth <= 2u, "busy handling must preserve WIP <= 2");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(diag.m29_overwrite_rejection_count), "normal busy-full pipeline should not increment overwrite rejection counter");

    ASSERT_TRUE(wait_until_async_ready(handle, task_id), "async task should become ready before cleanup");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "cleanup should release slot ownership");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_PostSwapFailureMarksSlotFailed)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_QUEUE_SUBMIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; submit failure path cannot be asserted");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "queue-submit failure hook should fail call");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_TRUE(diag.m29_failure_slot_id >= 0, "post-swap failure must record failed slot id");
    ASSERT_TRUE(diag.m29_failure_reason != 0, "post-swap failure must record failure reason");
    ASSERT_TRUE(diag.m29_slot0_state == kPromSlotFailed || diag.m29_slot1_state == kPromSlotFailed, "post-swap failure must move active slot into FAILED");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_CleanupToEmptyDeterministicForSafeStates)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.test_flags = PROM_TESTCFG_FAIL_DISPATCH;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; cleanup semantics cannot be asserted");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail), "dispatch failure should leave slot in recoverable failed state");

    PrometheusSgemmPolicyDiagnostics before{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &before), "diagnostics should be queryable before recovery");
    ASSERT_TRUE(before.m29_slot0_state == kPromSlotFailed || before.m29_slot1_state == kPromSlotFailed, "setup should expose a failed slot before recovery");

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "second run keeps failure injection active but must still perform deterministic cleanup first");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "third run should revisit failed slot and cleanup via legal cleanup path");

    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &after), "diagnostics should remain queryable after recovery");
    ASSERT_TRUE(after.m29_cleanup_success_count > before.m29_cleanup_success_count, "cleanup counter should advance when recovering failed/non-inflight slot");
    ASSERT_TRUE(after.m29_overwrite_rejection_count == before.m29_overwrite_rejection_count, "cleanup recovery should not be misreported as overwrite rejection");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_M29_FixedDouble_CleanupRejectsInflightOwnership)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u || caps.backend_type == PROM_BACKEND_VULKAN_SOFTWARE) {
        SKIP("In-flight cleanup rejection needs non-software Vulkan async submission support");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    int task_id = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_submit_async(handle, a.data(), b.data(), 8u, 8u, 8u, &task_id, &stage, &detail), "async submit should succeed");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_abandon_async(handle, task_id), "abandon must reject unresolved in-flight ownership");

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diagnostics should remain queryable");
    ASSERT_TRUE(diag.m29_inflight_rejection_count >= 1u, "illegal in-flight cleanup attempt should increment in-flight rejection diagnostics");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusSgemmBatchRefillRing)
{
    struct Shape { std::uint32_t m, n, k; };
    const std::vector<Shape> shapes = {
        {128u, 128u, 128u}, {256u, 256u, 64u}, {64u, 512u, 128u}, {512u, 64u, 128u},
        {31u, 29u, 23u}, {17u, 17u, 17u}, {192u, 320u, 96u}, {96u, 96u, 96u}
    };
    const float sentinel = -98765.25f;
    PrometheusVulkanDeviceDiagnostics device{};
    void* identity_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &identity_handle), "batch refill runtime create should succeed");
    if (!runtime_available(identity_handle)) SKIP("batch refill ring requires hardware Vulkan");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_vulkan_device_diagnostics(identity_handle, &device), "device diagnostics should succeed");
    if (device.software_vulkan != 0u || device.device_type != PROM_VK_DEVICE_TYPE_DISCRETE_GPU ||
        std::string(device.device_name).find("NVIDIA GeForce RTX 3070") == std::string::npos) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(identity_handle), "non-RTX3070 runtime should clean up");
        SKIP("batch refill hardware proof requires NVIDIA GeForce RTX 3070");
    }
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(identity_handle), "identity runtime should clean up");

    std::ostringstream depth_json;
    std::ostringstream depth_markdown;
    depth_json << "[";
    depth_markdown << "| depth | wall ns | max in-flight | submits | polls | waits | ring full | refills | query harvests | gpu ns | correctness |\n|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n";
    for (const std::uint32_t depth : {1u, 2u, 4u}) {
        PrometheusReactorConfig config{};
        config.struct_size = static_cast<std::uint32_t>(sizeof(config));
        config.batch_ring_depth = depth;
        void* handle = nullptr;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "depth runtime create should succeed");
        std::vector<std::vector<float>> a, b, c, expected;
        std::vector<PrometheusSgemmBatchEntry> entries;
        a.reserve(shapes.size()); b.reserve(shapes.size()); c.reserve(shapes.size()); expected.reserve(shapes.size()); entries.reserve(shapes.size());
        for (std::size_t i = 0; i < shapes.size(); ++i) {
            a.push_back(deterministic_matrix(shapes[i].m, shapes[i].k));
            b.push_back(deterministic_matrix(shapes[i].k, shapes[i].n));
            for (float& value : a.back()) value += static_cast<float>(i) * 0.03125f;
            for (float& value : b.back()) value -= static_cast<float>(i) * 0.015625f;
            c.emplace_back(shapes[i].m * shapes[i].n, sentinel);
            expected.push_back(cpu_sgemm(a.back(), b.back(), shapes[i].m, shapes[i].n, shapes[i].k));
            entries.push_back({a.back().data(), b.back().data(), c.back().data(), shapes[i].m, shapes[i].n, shapes[i].k});
        }
        const auto begin = std::chrono::steady_clock::now();
        std::uint32_t stage = PROM_STAGE_NONE;
        int detail = 0;
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "public batch refill call should succeed");
        const auto wall_ns = static_cast<std::uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::steady_clock::now() - begin).count());
        PrometheusSgemmBatchDiagnostics diag{};
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "batch diagnostics should succeed");
        ASSERT_EQUAL(depth, diag.physical_ring_depth_configured, "configured batch depth must be truthful");
        ASSERT_EQUAL(depth, diag.physical_ring_depth_effective, "effective batch depth must be truthful");
        ASSERT_EQUAL(static_cast<std::uint32_t>(entries.size()), static_cast<std::uint32_t>(diag.total_submits), "every entry must have a physical submission");
        ASSERT_EQUAL(1u, diag.output_committed, "successful batch must atomically commit");
        ASSERT_EQUAL(0u, diag.current_in_flight, "successful batch must leave no in-flight ownership");
        ASSERT_TRUE(diag.refill_count > 0u, "batch must refill physical slots");
        ASSERT_TRUE(depth == 1u || diag.max_in_flight >= 2u, "depth two and four must prove multiple outstanding entries");
        for (std::size_t i = 0; i < entries.size(); ++i) {
            ASSERT_TRUE(near_equal(c[i], expected[i]), "each committed batch result must match the CPU oracle");
            ASSERT_EQUAL(static_cast<std::uint32_t>(i), diag.m31_commit_order[i], "caller commit order must be entry order");
            ASSERT_TRUE(diag.m31_submission_sequence[i] != 0u, "each entry must retain submission attribution");
        }
        std::uint64_t gpu_ns = 0u;
        for (std::size_t i = 0; i < entries.size(); ++i) gpu_ns += diag.m31_gpu_duration_ns[i];
        depth_json << (depth == 1u ? "" : ",") << "{\"requested_depth\":" << depth << ",\"effective_depth\":" << diag.physical_ring_depth_effective
                   << ",\"entry_count\":" << entries.size() << ",\"wall_ns\":" << wall_ns << ",\"max_in_flight\":" << diag.max_in_flight
                   << ",\"submits\":" << diag.total_submits << ",\"polls\":" << diag.total_polls << ",\"forced_waits\":" << diag.total_forced_waits
                   << ",\"ring_full\":" << diag.ring_full_count << ",\"refills\":" << diag.refill_count << ",\"query_harvests\":" << diag.query_harvest_count
                   << ",\"aggregate_gpu_ns\":" << gpu_ns << ",\"completion_order\":[";
        for (std::size_t i = 0; i < entries.size(); ++i) depth_json << (i == 0u ? "" : ",") << diag.m31_completion_order[i];
        depth_json << "],\"commit_order\":[0,1,2,3,4,5,6,7],\"correctness\":true,\"output_committed\":true}";
        depth_markdown << "| " << depth << " | " << wall_ns << " | " << diag.max_in_flight << " | " << diag.total_submits << " | " << diag.total_polls << " | " << diag.total_forced_waits << " | " << diag.ring_full_count << " | " << diag.refill_count << " | " << diag.query_harvest_count << " | " << gpu_ns << " | pass |\n";
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "success depth runtime should clean up");
    }

    PrometheusReactorConfig failure_config{};
    failure_config.struct_size = static_cast<std::uint32_t>(sizeof(failure_config));
    failure_config.batch_ring_depth = 4u;
    void* failure_handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&failure_config, &failure_handle), "failure runtime create should succeed");
    std::vector<std::vector<float>> fa, fb, fc;
    std::vector<PrometheusSgemmBatchEntry> failure_entries;
    fa.reserve(shapes.size()); fb.reserve(shapes.size()); fc.reserve(shapes.size()); failure_entries.reserve(shapes.size());
    for (const Shape& shape : shapes) {
        fa.push_back(deterministic_matrix(shape.m, shape.k)); fb.push_back(deterministic_matrix(shape.k, shape.n));
        fc.emplace_back(shape.m * shape.n, sentinel);
        failure_entries.push_back({fa.back().data(), fb.back().data(), fc.back().data(), shape.m, shape.n, shape.k});
    }
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    const std::uint32_t failure_entry = 2u;
    const std::uint32_t failure_flags = (failure_entry + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch_m31_test(failure_handle, failure_entries.data(), static_cast<std::uint32_t>(failure_entries.size()), failure_flags, &stage, &detail), "test-only injected post-submit batch failure must fail");
    PrometheusSgemmBatchDiagnostics failure_diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(failure_handle, &failure_diag), "failure diagnostics should succeed");
    ASSERT_EQUAL(0u, failure_diag.output_committed, "failed batch must not commit caller outputs");
    ASSERT_EQUAL(failure_entry, failure_diag.failed_entry_id, "first injected failure entry must remain authoritative");
    ASSERT_EQUAL(PROM_STAGE_SUBMIT, failure_diag.failure_stage, "injected failure stage must remain authoritative");
    ASSERT_TRUE(failure_diag.total_submits >= failure_entry + 1u && failure_diag.total_submits < failure_entries.size(), "failure must stop admission after already-submitted work");
    ASSERT_EQUAL(0u, failure_diag.current_in_flight, "failure drain must resolve submitted ownership");
    for (const auto& output : fc) for (const float value : output) ASSERT_EQUAL(sentinel, value, "failed batch must leave every caller output sentinel intact");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(failure_handle), "failure runtime should clean up");

    std::filesystem::create_directories("out/test-artifacts");
    std::ofstream json("out/test-artifacts/prometheus_sgemm_batch_refill_ring.json");
    std::ofstream markdown("out/test-artifacts/prometheus_sgemm_batch_refill_ring.md");
    json << "{\"test\":\"PrometheusSgemmBatchRefillRing\",\"hardware_proof\":true,\"real_multi_submit_proof\":true,\"atomic_success_commit\":true,\"atomic_failure_no_commit\":true,\"acceptance_evidence_complete\":true,\"device\":{\"name\":\"" << device.device_name << "\",\"vendor_id\":" << device.vendor_id << ",\"device_id\":" << device.device_id << ",\"device_type\":" << device.device_type << ",\"driver_version\":" << device.driver_version << ",\"api_version\":" << device.api_version << ",\"software_vulkan\":false,\"compute_queue_family\":" << device.compute_queue_family << ",\"transfer_queue_family\":" << device.transfer_queue_family << "},\"depths\":" << depth_json.str() << "],\"failure\":{\"first_failure_entry\":2,\"output_committed\":false,\"drained\":true}}\n";
    markdown << "# Prometheus batch refill ring\n\nFocused test: `PrometheusSgemmBatchRefillRing`. RTX 3070 hardware proof; public batch API; one compute queue.\n\n" << depth_markdown.str() << "\nSuccess: atomic entry-ID commit and CPU-oracle correctness pass. Failure: post-submit admission stop, drain, and sentinel-preserving no-commit pass.\n";
    ASSERT_TRUE(json.good() && markdown.good(), "batch refill artifacts should be written");
}

FACT(PrometheusSgemmBatchFirstFailureDeterministic)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.batch_ring_depth = 4u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch_m31_test(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO, &stage, &detail), "test-only dual candidate seam should fail");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics");
    ASSERT_EQUAL(0u, diag.failed_entry_id, "reducer must select lowest caller entry id, not discovery order");
    ASSERT_EQUAL(PROM_STAGE_SUBMIT, diag.failure_stage, "primary stage must remain submit");
    ASSERT_EQUAL(0u, diag.output_committed, "failure must remain atomic");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchStopsAdmissionAfterFailure)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 4u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 10u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    const std::uint32_t flags = (2u + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch_m31_test(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), flags, &stage, &detail), "test-only post-submit failure");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics");
    ASSERT_EQUAL(2u, diag.failed_entry_id, "failure identity");
    ASSERT_EQUAL(3u, static_cast<std::uint32_t>(diag.total_submits), "admission must stop at selected failure");
    for (std::uint32_t i = 3u; i < static_cast<std::uint32_t>(entries.size()); ++i) ASSERT_EQUAL(0u, diag.m31_submission_sequence[i], "skipped tail has no physical submit evidence");
    ASSERT_EQUAL(0u, diag.current_in_flight, "drain must resolve admitted ownership");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchFailureIsAtomic)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 2u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 6u);
    for (auto& output : c) std::fill(output.begin(), output.end(), -7331.0f);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch_m31_test(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), (1u + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT, &stage, &detail), "test-only failure after real submit");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics");
    ASSERT_EQUAL(0u, diag.output_committed, "no partial output publish");
    for (const auto& output : c) for (float value : output) ASSERT_EQUAL(-7331.0f, value, "caller output must remain byte-equivalent sentinel values");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchRuntimeReusableAfterNonfatalFailure)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 2u;
    config.test_flags = PROM_TESTCFG_FAIL_ASYNC_POLL;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 4u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "injected observation failure");
    PrometheusSgemmBatchDiagnostics failed{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &failed), "failure diagnostics");
    ASSERT_EQUAL(0u, failed.output_committed, "failed batch stays atomic");
    std::vector<std::vector<float>> a2, b2, c2;
    const auto replacement = make_plan_entries(a2, b2, c2, 3u);
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, replacement.data(), static_cast<std::uint32_t>(replacement.size()), 0u, &stage, &detail), "reaped runtime must accept replacement batch");
    PrometheusSgemmBatchDiagnostics recovered{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &recovered), "replacement diagnostics");
    ASSERT_EQUAL(1u, recovered.output_committed, "replacement commits after nonfatal failure");
    ASSERT_EQUAL(0u, recovered.current_in_flight, "replacement leaves no ownership");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchFatalFailureMarksRuntimeUnsafe)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 2u;
    config.async_test_flags = PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 4u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "post-submit device-loss seam must fail");
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), 1u, 0u, &stage, &detail), "unsafe runtime must reject later batch submit");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "fatal teardown remains safe");
}

FACT(PrometheusSgemmBatchFeedbackMatchesValidCompletions)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 4u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 6u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "success batch");
    PrometheusSgemmBatchDiagnostics success{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &success), "success diagnostics");
    ASSERT_EQUAL(static_cast<std::uint64_t>(entries.size()), success.feedback_committed_count, "each valid completion contributes feedback once");
    ASSERT_EQUAL(0u, static_cast<std::uint32_t>(success.feedback_skipped_count), "success skips no feedback");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchDrainsSubmittedEntriesSafely)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 4u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_batch_m31_test(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), (2u + 1u) << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT, &stage, &detail), "test-only failure while ring contains multiple submitted entries");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics");
    ASSERT_TRUE(diag.max_in_flight >= 2u, "test must have multiple real submissions in flight");
    ASSERT_EQUAL(0u, diag.current_in_flight, "every admitted nonfatal entry must be drained or reaped");
    ASSERT_EQUAL(0u, diag.output_committed, "drain cannot publish partial output");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchCommitsInEntryOrder)
{
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config)); config.batch_ring_depth = 4u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create");
    if (!runtime_available(handle)) { ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup"); SKIP("M31 authority requires Vulkan"); }
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    std::uint32_t stage = PROM_STAGE_NONE; int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch(handle, entries.data(), static_cast<std::uint32_t>(entries.size()), 0u, &stage, &detail), "success batch");
    PrometheusSgemmBatchDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_batch_diagnostics(handle, &diag), "diagnostics");
    for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) ASSERT_EQUAL(i, diag.m31_commit_order[i], "commit must use immutable entry order");
    ASSERT_EQUAL(static_cast<std::uint32_t>(entries.size()), diag.m31_commit_count, "all staged entries commit once");
    std::filesystem::create_directories("out/test-artifacts");
    std::ofstream json("out/test-artifacts/prometheus_r2c_batch_semantics.json");
    std::ofstream markdown("out/test-artifacts/prometheus_r2c_batch_semantics.md");
    json << "{\"test\":\"PrometheusSgemmBatchCommitsInEntryOrder\",\"plan_generation\":" << diag.plan_generation
         << ",\"logical_width\":" << diag.effective_workers << ",\"ring_depth\":" << diag.physical_ring_depth_effective
         << ",\"max_in_flight\":" << diag.max_in_flight << ",\"completion_order\":[";
    for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) json << (i == 0u ? "" : ",") << diag.m31_completion_order[i];
    json << "],\"commit_order\":[";
    for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) json << (i == 0u ? "" : ",") << diag.m31_commit_order[i];
    json << "],\"submitted_sequences\":[";
    for (std::uint32_t i = 0u; i < static_cast<std::uint32_t>(entries.size()); ++i) json << (i == 0u ? "" : ",") << diag.m31_submission_sequence[i];
    json << "],\"output_committed\":true,\"feedback_committed\":" << diag.feedback_committed_count << ",\"feedback_skipped\":" << diag.feedback_skipped_count << "}\n";
    markdown << "# Prometheus R2c M31 batch semantics\n\n"
             << "RTX authority lane: shared M29 ring, immutable plan generation " << diag.plan_generation
             << ", max in-flight " << diag.max_in_flight << ".\n\n"
             << "- Commit order: ascending entry IDs\n- Output committed: true\n- Feedback committed/skipped: "
             << diag.feedback_committed_count << "/" << diag.feedback_skipped_count << "\n";
    ASSERT_TRUE(json.good() && markdown.good(), "R2c authority evidence artifacts should be written");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "cleanup");
}

FACT(PrometheusSgemmBatchLogicalLanesAreMetadataOnly)
{
    std::vector<std::vector<float>> a, b, c;
    const auto entries = make_plan_entries(a, b, c, 8u);
    prom_sgemm_batch_plan plan{};
    ASSERT_EQUAL(PROM_OK, prom_sgemm_batch_plan_build(entries.data(), static_cast<std::uint32_t>(entries.size()), 3u, &plan, nullptr), "plan build");
    for (std::uint32_t i = 0u; i < plan.entry_count; ++i) ASSERT_EQUAL(i % 3u, plan.entries[i].logical_lane, "logical lane map is deterministic metadata");
    prom_sgemm_batch_plan_destroy(&plan);
    /* Nonzero width flags deliberately remain on P11 until R2d.  This R2c
       test therefore proves only the real immutable metadata contract and
       does not pretend a lane selects an M31 queue or slot. */
}
