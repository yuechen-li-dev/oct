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
        facts.policy_mode = PROM_POLICY_MODE_AGGRESSIVE;
        facts.packed4_available = 1u;
        facts.packed4_small_shape = 0u;
        facts.packed4_padding_waste_permille = 0u;
        facts.packed4_mode_budget_permille = 380u;
        facts.packed4_row_major_valid = 1u;
        facts.packed4_tail_valid = 1u;
        return facts;
    }

    prom_policy_thresholds base_policy_thresholds()
    {
        prom_policy_thresholds thresholds{};
        thresholds.retreat_enter_permille = 240u;
        thresholds.retreat_exit_permille = 160u;
        thresholds.recovery_enter_permille = 420u;
        thresholds.recovery_exit_permille = 260u;
        thresholds.min_commit_decisions = 2u;
        thresholds.retreat_cooldown_decisions = 2u;
        thresholds.recovery_hold_decisions = 3u;
        return thresholds;
    }

    prom_policy_facts base_policy_facts()
    {
        prom_policy_facts facts{};
        facts.waste_ratio_permille = 120u;
        facts.pending_waste_ratio_permille = 120u;
        facts.hard_retreat_override = 0u;
        facts.hard_recovery_override = 0u;
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
        ASSERT_EQUAL(PROM_VK_COMPUTE_PACKED4_FP32, decision.compute_mode, "small direct-friendly shape should pick packed4 compute when all gates pass");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, decision.final_detail, "packed4 selection should be explicitly observable");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 64u * 64u * 4u;
        facts.tiled_shape = 0u;
        facts.readback_required = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "large staged-capable shape should select a mode");
        ASSERT_EQUAL(PROM_VK_PATH_DIRECT, decision.selected_path, "packed4 should keep direct path when selected");
        ASSERT_EQUAL(PROM_VK_COMPUTE_PACKED4_FP32, decision.compute_mode, "non-tiled eligible shape should pick packed4");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, decision.final_detail, "packed4 selection should remain directly observable");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.work_units = 64u * 64u * 4u;
        facts.tiled_shape = 0u;
        facts.readback_required = 0u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "upload-only staged shape should select a mode");
        ASSERT_EQUAL(PROM_VK_PATH_DIRECT, decision.selected_path, "packed4 path should keep canonical direct row-major output path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_PACKED4_FP32, decision.compute_mode, "upload-only staged shape should still pick packed4 when gates pass");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, decision.final_detail, "upload-only packed4 selection should be observable");
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
        ASSERT_EQUAL(PROM_VK_COMPUTE_PACKED4_FP32, decision.compute_mode, "constrained capability shape should still allow packed4 when direct exists");
        ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_PACKED4_FP32, decision.final_detail, "packed4 detail should remain explicit in constrained capability scenario");
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

    {
        prom_judgment_facts facts = base_facts();
        facts.packed4_small_shape = 1u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "small-shape packed4 rejection should still choose fallback scalar mode");
        ASSERT_EQUAL(PROM_VK_COMPUTE_BASELINE, decision.compute_mode, "small-shape packed4 rejection should fallback to baseline compute");
        ASSERT_EQUAL(PROM_PACKED4_REJECT_SMALL_SHAPE, decision.packed4_reject_reason, "small-shape rejection reason should be explicit");
    }

    {
        prom_judgment_facts facts = base_facts();
        facts.policy_mode = PROM_POLICY_MODE_SAFE;
        facts.packed4_padding_waste_permille = 260u;
        facts.packed4_mode_budget_permille = 220u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "mode-budget packed4 rejection should still choose fallback scalar mode");
        ASSERT_EQUAL(PROM_VK_COMPUTE_BASELINE, decision.compute_mode, "mode-budget packed4 rejection should fallback to baseline compute");
        ASSERT_EQUAL(PROM_PACKED4_REJECT_MODE_BUDGET_DENIED, decision.packed4_reject_reason, "safe-mode over-budget rejection should be explicit");
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

FACT(PrometheusJudgmentEngine_HasNoCrossCallHysteresisOrCommitmentMemory)
{
    prom_judgment_facts near_threshold = base_facts();
    near_threshold.packed4_available = 0u;
    near_threshold.readback_required = 1u;
    near_threshold.work_units = static_cast<std::uint64_t>(PROM_JUDGMENT_STAGING_WORK_THRESHOLD) - 1u;
    near_threshold.tiled_shape = 0u;

    prom_judgment_decision below_threshold{};
    prom_judgment_engine_select_sgemm_mode(&near_threshold, &below_threshold);
    ASSERT_EQUAL(1u, below_threshold.success, "below-threshold evaluation should select a mode");
    ASSERT_EQUAL(PROM_VK_PATH_DIRECT, below_threshold.selected_path, "below-threshold shape should select direct path");
    ASSERT_EQUAL(PROM_VK_COMPUTE_BASELINE, below_threshold.compute_mode, "below-threshold shape should stay baseline");

    near_threshold.work_units = static_cast<std::uint64_t>(PROM_JUDGMENT_STAGING_WORK_THRESHOLD);
    prom_judgment_decision at_threshold{};
    prom_judgment_engine_select_sgemm_mode(&near_threshold, &at_threshold);
    ASSERT_EQUAL(1u, at_threshold.success, "at-threshold evaluation should select a mode");
    ASSERT_EQUAL(PROM_VK_PATH_STAGED_UPLOAD_READBACK, at_threshold.selected_path, "at-threshold shape should immediately switch to staged path");

    near_threshold.work_units = static_cast<std::uint64_t>(PROM_JUDGMENT_STAGING_WORK_THRESHOLD) - 1u;
    prom_judgment_decision return_below{};
    prom_judgment_engine_select_sgemm_mode(&near_threshold, &return_below);
    ASSERT_EQUAL(1u, return_below.success, "return-below-threshold evaluation should select a mode");
    ASSERT_EQUAL(PROM_VK_PATH_DIRECT, return_below.selected_path, "return-below-threshold shape should immediately switch back to direct path");
}

FACT(PrometheusJudgmentEngine_PolicyMemoryHysteresisAvoidsThresholdChatter)
{
    prom_policy_memory memory{};
    prom_policy_thresholds thresholds = base_policy_thresholds();
    prom_policy_facts facts = base_policy_facts();
    prom_policy_mode mode = PROM_POLICY_MODE_AGGRESSIVE;

    prom_policy_memory_init(&memory, PROM_POLICY_MODE_AGGRESSIVE);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, mode, "initial low waste should keep aggressive mode");

    facts.waste_ratio_permille = 260u;
    facts.pending_waste_ratio_permille = 260u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, mode, "min-commit should block immediate retreat on first threshold crossing");

    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_SAFE, mode, "after min-commit dwell, threshold crossing should retreat to safe");

    facts.waste_ratio_permille = 200u;
    facts.pending_waste_ratio_permille = 200u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_SAFE, mode, "safe mode should be retained while signal stays within hysteresis band");
}

FACT(PrometheusJudgmentEngine_PolicyMemoryCooldownAndRecoveryHoldAreEnforced)
{
    prom_policy_memory memory{};
    prom_policy_thresholds thresholds = base_policy_thresholds();
    prom_policy_facts facts = base_policy_facts();
    prom_policy_mode mode = PROM_POLICY_MODE_AGGRESSIVE;

    prom_policy_memory_init(&memory, PROM_POLICY_MODE_AGGRESSIVE);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, mode, "first decision should remain aggressive at low waste");

    facts.waste_ratio_permille = 280u;
    facts.pending_waste_ratio_permille = 280u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_SAFE, mode, "aggressive should retreat to safe after dwell requirement is met");
    ASSERT_TRUE(memory.cooldown_remaining > 0u, "retreat should set safe cooldown window");

    facts.waste_ratio_permille = 80u;
    facts.pending_waste_ratio_permille = 80u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_SAFE, mode, "cooldown should block immediate return to aggressive");

    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, mode, "after cooldown expires, low waste should allow aggressive re-entry");

    facts.hard_recovery_override = 1u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_RECOVERY, mode, "hard recovery override should force recovery mode");
    ASSERT_TRUE(memory.recovery_cooldown_remaining > 0u, "recovery entry should set hold timer");

    facts.hard_recovery_override = 0u;
    facts.waste_ratio_permille = 0u;
    facts.pending_waste_ratio_permille = 0u;
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_RECOVERY, mode, "recovery hold should block immediate exit");

    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    mode = prom_judgment_engine_update_policy_mode(&memory, &facts, &thresholds);
    ASSERT_EQUAL(PROM_POLICY_MODE_SAFE, mode, "recovery should exit to safe after hold timer expires");
}

FACT(PrometheusJudgmentEngine_PolicyMemoryDeterministicAndBoundSafe)
{
    prom_policy_memory first_memory{};
    prom_policy_memory second_memory{};
    prom_policy_thresholds thresholds = base_policy_thresholds();
    prom_policy_facts facts = base_policy_facts();
    prom_policy_mode first_mode = PROM_POLICY_MODE_AGGRESSIVE;
    prom_policy_mode second_mode = PROM_POLICY_MODE_AGGRESSIVE;

    prom_policy_memory_init(&first_memory, static_cast<prom_policy_mode>(999));
    prom_policy_memory_init(&second_memory, static_cast<prom_policy_mode>(999));
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, first_memory.current_mode, "invalid initial mode should clamp to aggressive");
    ASSERT_EQUAL(PROM_POLICY_MODE_AGGRESSIVE, second_memory.current_mode, "invalid initial mode should clamp deterministically");

    first_memory.decisions_in_mode = UINT32_MAX;
    second_memory.decisions_in_mode = UINT32_MAX;
    first_mode = prom_judgment_engine_update_policy_mode(&first_memory, &facts, &thresholds);
    second_mode = prom_judgment_engine_update_policy_mode(&second_memory, &facts, &thresholds);
    ASSERT_EQUAL(first_mode, second_mode, "same memory and facts should produce same mode");
    ASSERT_EQUAL(first_memory.current_mode, second_memory.current_mode, "mode memory should match across deterministic runs");
    ASSERT_EQUAL(first_memory.decisions_in_mode, second_memory.decisions_in_mode, "dwell counter should match across deterministic runs");
    ASSERT_EQUAL(UINT32_MAX, first_memory.decisions_in_mode, "dwell counter should saturate without overflow");

    first_memory.cooldown_remaining = 0u;
    second_memory.cooldown_remaining = 0u;
    first_mode = prom_judgment_engine_update_policy_mode(&first_memory, &facts, &thresholds);
    second_mode = prom_judgment_engine_update_policy_mode(&second_memory, &facts, &thresholds);
    ASSERT_EQUAL(first_mode, second_mode, "determinism should hold for repeated updates");
    ASSERT_TRUE(first_memory.cooldown_remaining <= thresholds.retreat_cooldown_decisions,
                "cooldown counter should remain within valid non-negative bounds");
}
