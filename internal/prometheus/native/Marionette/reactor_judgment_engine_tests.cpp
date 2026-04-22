#include "../reactor_judgment_engine.h"
#include "test_harness.h"

#include <cstdint>

namespace
{
    prom_judgment_facts base_facts()
    {
        prom_judgment_facts facts{};
        facts.m = 16u;
        facts.n = 16u;
        facts.k = 16u;
        facts.work_units = static_cast<std::uint64_t>(facts.m) * static_cast<std::uint64_t>(facts.n) * static_cast<std::uint64_t>(facts.k);
        facts.can_stage = 1u;
        facts.can_direct = 1u;
        facts.allow_fallback = 1u;
        facts.readback_required = 1u;
        facts.force_direct = 0u;
        facts.force_staged = 0u;
        facts.force_tiled = 0u;
        facts.tiled_shape = 0u;
        facts.software_vulkan = 0u;
        return facts;
    }
}

FACT(PrometheusJudgmentEngine_DeterministicForSameFacts)
{
    const prom_judgment_facts facts = base_facts();
    prom_judgment_decision first{};
    prom_judgment_decision second{};

    prom_judgment_engine_select_sgemm_mode(&facts, &first);
    prom_judgment_engine_select_sgemm_mode(&facts, &second);

    ASSERT_EQUAL(1u, first.success, "first evaluation should select a candidate");
    ASSERT_EQUAL(1u, second.success, "second evaluation should select a candidate");
    ASSERT_EQUAL(first.selected_path, second.selected_path, "same facts should produce the same selected path");
    ASSERT_EQUAL(first.compute_mode, second.compute_mode, "same facts should produce the same compute mode");
    ASSERT_EQUAL(first.final_detail, second.final_detail, "same facts should produce the same observability detail");
    ASSERT_EQUAL(first.winning_candidate_index, second.winning_candidate_index, "same facts should produce the same winner index");
    ASSERT_EQUAL(first.winning_score, second.winning_score, "same facts should produce the same winner score");
}

FACT(PrometheusJudgmentEngine_CandidateDiscriminationCoversCurrentModeSpace)
{
    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 4u * 4u * 4u;
        facts.readback_required = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "small direct-friendly shape should select a mode");
        ASSERT_EQUAL(PROM_VK_PATH_DIRECT, decision.selected_path, "small direct-friendly shape should pick direct path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_BASELINE, decision.compute_mode, "small direct-friendly shape should keep baseline compute");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT, decision.final_detail, "small direct-friendly shape should expose direct detail");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 128u * 128u * 16u;
        facts.tiled_shape = 1u;
        facts.readback_required = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "large staged-capable shape should select a mode");
        ASSERT_EQUAL(PROM_VK_PATH_STAGED_UPLOAD_READBACK, decision.selected_path, "large staged-capable readback shape should pick staged readback path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_TILED, decision.compute_mode, "large tiled-eligible shape should pick tiled compute");
        ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, decision.final_detail, "large staged+tiled selection should be observable");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 128u * 128u * 16u;
        facts.tiled_shape = 1u;
        facts.readback_required = 0u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "upload-only staged shape should select a mode");
        ASSERT_EQUAL(PROM_VK_PATH_STAGED_UPLOAD, decision.selected_path, "upload-only staged shape should pick staged-upload path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_TILED, decision.compute_mode, "upload-only staged large shape should still pick tiled compute");
        ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_TILED, decision.final_detail, "upload-only staged+tiled selection should be observable");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 128u * 128u * 16u;
        facts.can_stage = 0u;
        facts.software_vulkan = 1u;
        facts.tiled_shape = 0u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "constrained capability shape should still select a mode when direct is available");
        ASSERT_EQUAL(PROM_VK_PATH_DIRECT, decision.selected_path, "constrained capability shape should fall back to direct path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_BASELINE, decision.compute_mode, "constrained capability fallback should remain baseline compute");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT, decision.final_detail, "when staging is unavailable up front the policy should remain direct without staged fallback detail");
    }
}

FACT(PrometheusJudgmentEngine_IntegrationParityWithReactorPolicyScenarios)
{
    {
        prom_judgment_facts facts = base_facts();
        facts.force_staged = 1u;
        facts.can_stage = 0u;
        facts.allow_fallback = 0u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(0u, decision.success, "forced staged path without capability and without fallback should fail");
        ASSERT_EQUAL(PROM_DETAIL_CAPABILITY_MISMATCH, decision.error_detail, "forced staged mismatch should report capability mismatch detail");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.force_direct = 1u;
        facts.can_direct = 0u;
        facts.can_stage = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "forced direct path should degrade to staged when direct is unavailable and staging is available");
        ASSERT_EQUAL(PROM_VK_PATH_STAGED_UPLOAD_READBACK, decision.selected_path, "forced direct degradation should preserve readback-required staged mode");
        ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK, decision.final_detail, "forced direct degradation should keep staged-readback observability");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.force_tiled = 1u;
        facts.force_staged = 1u;
        facts.can_stage = 0u;
        facts.allow_fallback = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "forced staged+tiled with no staging should still recover when direct fallback is allowed");
        ASSERT_EQUAL(PROM_VK_PATH_DIRECT, decision.selected_path, "fallback from staged+tiled should land on direct path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_TILED, decision.compute_mode, "fallback from staged+tiled should preserve forced tiled selection semantics");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_TILED, decision.final_detail, "fallback from staged+tiled should surface direct+tiled detail after tiled selection");
        ASSERT_TRUE(decision.winning_score > -100000, "winning score should be populated for diagnostics");
    }
}

FACT(PrometheusJudgmentEngine_AsyncSubmissionPolicyIsExplicit)
{
    {
        prom_judgment_async_facts facts{};
        facts.request_async = 1u;
        facts.in_flight = 0u;
        facts.software_vulkan = 0u;

        prom_judgment_async_decision decision{};
        prom_judgment_engine_select_async_submission(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "async policy should accept eligible async submissions");
        ASSERT_EQUAL(1u, decision.execute_async, "eligible async request should be explicitly selected as async");
    }

    {
        prom_judgment_async_facts facts{};
        facts.request_async = 1u;
        facts.in_flight = 1u;
        facts.software_vulkan = 0u;

        prom_judgment_async_decision decision{};
        prom_judgment_engine_select_async_submission(&facts, &decision);
        ASSERT_EQUAL(0u, decision.success, "in-flight overlap should reject async selection");
        ASSERT_EQUAL(PROM_DETAIL_REUSE_IN_FLIGHT, decision.reject_detail, "in-flight overlap should expose explicit ownership hazard detail");
    }

    {
        prom_judgment_async_facts facts{};
        facts.request_async = 1u;
        facts.in_flight = 0u;
        facts.software_vulkan = 1u;

        prom_judgment_async_decision decision{};
        prom_judgment_engine_select_async_submission(&facts, &decision);
        ASSERT_EQUAL(0u, decision.success, "software Vulkan policy should suppress async mode");
        ASSERT_EQUAL(PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED, decision.reject_detail, "software suppression should remain explicitly observable");
    }
}
