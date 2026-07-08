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

    {
        prom_judgment_facts facts = base_facts();
        facts.policy_mode = PROM_POLICY_MODE_SAFE;
        facts.m = 256u;
        facts.n = 128u;
        facts.k = 512u;
        facts.work_units = static_cast<std::uint64_t>(facts.m) * static_cast<std::uint64_t>(facts.n) * static_cast<std::uint64_t>(facts.k);
        facts.readback_required = 1u;
        facts.tiled_shape = 1u;
        facts.packed4_available = 0u;
        facts.capability_fp16_storage = 0u;

        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(1u, decision.success, "safe eligible tiled shape should still select a path");
        ASSERT_EQUAL(PROM_VK_PATH_STAGED_UPLOAD_READBACK, decision.selected_path, "safe eligible tiled shape should keep staged-readback path");
        ASSERT_EQUAL(PROM_VK_COMPUTE_TILED, decision.compute_mode, "safe policy alone must not suppress tiled dispatch");
        ASSERT_EQUAL(PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED, decision.final_detail, "safe tiled dispatch should stay explicitly observable");
    }
}

FACT(PrometheusJudgmentEngine_FP16PolicyGatesAreExplicitAndDeterministic)
{
    {
        prom_judgment_facts facts = base_facts();
        facts.strict_fp32 = 1u;
        facts.tolerance_known = 1u;
        facts.tolerance_pass = 1u;
        facts.capability_fp16_storage = 1u;
        facts.fallback_available = 1u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_STRICT_FP32, decision.fp16_reject_reason, "strict-fp32 gate must reject fp16 explicitly");
    }
    {
        prom_judgment_facts facts = base_facts();
        facts.tolerance_known = 0u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_TOLERANCE_UNKNOWN, decision.fp16_reject_reason, "unknown tolerance must reject fp16 explicitly");
    }
    {
        prom_judgment_facts facts = base_facts();
        facts.tolerance_known = 1u;
        facts.tolerance_pass = 0u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_TOLERANCE_EXCEEDED, decision.fp16_reject_reason, "tolerance failure must reject fp16 explicitly");
    }
    {
        prom_judgment_facts facts = base_facts();
        facts.tolerance_known = 1u;
        facts.tolerance_pass = 1u;
        facts.has_special_values = 1u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_SPECIAL_VALUE, decision.fp16_reject_reason, "special values must reject fp16 explicitly");
    }
    {
        prom_judgment_facts facts = base_facts();
        facts.tolerance_known = 1u;
        facts.tolerance_pass = 1u;
        facts.capability_fp16_storage = 0u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_CAPABILITY_MISSING, decision.fp16_reject_reason, "capability-missing gate must reject fp16 explicitly");
    }
    {
        prom_judgment_facts facts = base_facts();
        facts.tolerance_known = 1u;
        facts.tolerance_pass = 1u;
        facts.capability_fp16_storage = 1u;
        facts.fallback_available = 0u;
        facts.fp16_utility_score = 1200;
        prom_judgment_decision decision{};
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        ASSERT_EQUAL(PROM_FP16_REJECT_FALLBACK_REQUIRED, decision.fp16_reject_reason, "fallback-available gate must reject fp16 explicitly");
    }
}

FACT(PrometheusJudgmentEngine_FP16CanWinOnlyWhenTopUtilityAndAllGatesPass)
{
    prom_judgment_facts facts = base_facts();
    facts.tolerance_known = 1u;
    facts.tolerance_pass = 1u;
    facts.capability_fp16_storage = 1u;
    facts.fallback_available = 1u;
    facts.fp16_utility_score = 1201;

    prom_judgment_decision decision{};
    prom_judgment_engine_select_sgemm_mode(&facts, &decision);
    ASSERT_EQUAL(1u, decision.success, "eligible fp16 mode should still yield successful candidate selection");
    ASSERT_EQUAL(PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM, decision.compute_mode, "fp16 should be selected only when all gates pass and utility wins");
    ASSERT_EQUAL(PROM_DETAIL_PATH_DIRECT_FP16_STORAGE_FP32_ACCUM, decision.final_detail, "fp16 selection detail must stay explicit");
    ASSERT_EQUAL(1u, decision.fp16_selected, "fp16 selected flag must be explicit");

    facts.fp16_utility_score = 800;
    prom_judgment_engine_select_sgemm_mode(&facts, &decision);
    ASSERT_TRUE(decision.compute_mode != PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM, "lower utility should force deterministic fallback candidate");
    ASSERT_EQUAL(PROM_FP16_REJECT_NOT_TOP_UTILITY, decision.fp16_reject_reason, "non-winning fp16 should still expose explicit reason code");
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

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorFixedDoubleDefaultWhenFeasible)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 2400u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1500u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = 400;
    facts.pull_lag_headroom_slots_permille = 300;
    facts.serial_jit_headroom_slots_permille = 1400;
    facts.transfer_variance_class = PROM_VARIANCE_LOW;
    facts.compute_predictability_class = PROM_PREDICTABILITY_STABLE;
    facts.fallback_available = 1u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(1u, decision.success, "selector should succeed when fixed-double is feasible");
    ASSERT_EQUAL(PROM_BUFFERING_MODE_FIXED_DOUBLE_DEFAULT, decision.selected_mode, "fixed-double should win as default when feasible");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_FIXED_DOUBLE_SELECTED, decision.reason_code, "fixed-double selection reason should be explicit");
    ASSERT_TRUE(decision.fixed_score > decision.pull_lag_score, "fixed-double score should dominate in normal feasible regime");
}

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorPullLagAndSerialFallbackPaths)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 1600u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1500u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = -400;
    facts.pull_lag_headroom_slots_permille = 100;
    facts.serial_jit_headroom_slots_permille = 600;
    facts.transfer_variance_class = PROM_VARIANCE_MODERATE;
    facts.compute_predictability_class = PROM_PREDICTABILITY_TRACKED;
    facts.fallback_available = 1u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(1u, decision.success, "selector should succeed when pull-lag pressure path is feasible");
    ASSERT_EQUAL(PROM_BUFFERING_MODE_PULL_LAG_PRESSURE, decision.selected_mode, "pull-lag should win when fixed-double is infeasible and guarded gates pass");

    facts.transfer_variance_class = PROM_VARIANCE_HIGH;
    facts.memory_budget_slots_permille = 1200u;
    facts.fixed_double_headroom_slots_permille = -800;
    facts.pull_lag_headroom_slots_permille = -300;
    facts.serial_jit_headroom_slots_permille = 200;
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(1u, decision.success, "selector should still succeed with serial fallback when pull-lag is rejected");
    ASSERT_EQUAL(PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL, decision.selected_mode, "serial survival should win when fixed-double and pull-lag are blocked");
    ASSERT_TRUE(decision.pull_lag_feasible == 0u, "high-variance gate should reject pull-lag feasibility");
}

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorHardFailureWhenNoModeFeasible)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 900u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1500u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = -1100;
    facts.pull_lag_headroom_slots_permille = -600;
    facts.serial_jit_headroom_slots_permille = -100;
    facts.transfer_variance_class = PROM_VARIANCE_HIGH;
    facts.compute_predictability_class = PROM_PREDICTABILITY_UNSTABLE;
    facts.fallback_available = 0u;
    facts.starvation_risk_high = 1u;
    facts.pull_lag_wip_waste_exceeded = 1u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(0u, decision.success, "selector must fail explicitly when all buffering modes are infeasible");
    ASSERT_EQUAL(PROM_BUFFERING_MODE_NONE, decision.selected_mode, "no-feasible result should not select a partial mode");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE, decision.reason_code, "hard failure reason should be explicit");
}

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorHeadroomIsCandidateSpecific)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 1600u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1500u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = -400;
    facts.pull_lag_headroom_slots_permille = 100;
    facts.serial_jit_headroom_slots_permille = 600;
    facts.transfer_variance_class = PROM_VARIANCE_LOW;
    facts.compute_predictability_class = PROM_PREDICTABILITY_STABLE;
    facts.fallback_available = 1u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(PROM_BUFFERING_MODE_PULL_LAG_PRESSURE, decision.selected_mode, "pull-lag should win in medium budget case");
    ASSERT_EQUAL(-100000, decision.fixed_score, "infeasible fixed mode should keep sentinel score");
    ASSERT_EQUAL(700 + 100, decision.pull_lag_score, "pull-lag score must use pull-lag headroom");
    ASSERT_EQUAL(300 + 600, decision.serial_score, "serial score must use serial headroom");
}

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorPreservesPerModeRejectionReasons)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 1200u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1000u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = -800;
    facts.pull_lag_headroom_slots_permille = 200;
    facts.serial_jit_headroom_slots_permille = 200;
    facts.transfer_variance_class = PROM_VARIANCE_HIGH;
    facts.compute_predictability_class = PROM_PREDICTABILITY_STABLE;
    facts.fallback_available = 1u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(1u, decision.success, "serial fallback should succeed");
    ASSERT_EQUAL(PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL, decision.selected_mode, "serial should be selected after pull-lag rejection");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_FIXED_DOUBLE_MEMORY_INSUFFICIENT, decision.fixed_double_rejection_reason,
                 "fixed rejection reason must remain explicit");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_PULL_LAG_VARIANCE_MISS, decision.pull_lag_rejection_reason,
                 "pull-lag variance rejection reason must be preserved");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED, decision.final_reason_code,
                 "final reason should represent the selected fallback mode");
}

FACT(PrometheusJudgmentEngine_M35_BufferingSelectorNoFeasibleIncludesPerModeReasons)
{
    prom_buffering_selector_facts facts{};
    facts.memory_budget_slots_permille = 900u;
    facts.required_fixed_slots_permille = 2000u;
    facts.required_pull_lag_peak_slots_permille = 1500u;
    facts.required_serial_slots_permille = 1000u;
    facts.fixed_double_headroom_slots_permille = -1100;
    facts.pull_lag_headroom_slots_permille = -600;
    facts.serial_jit_headroom_slots_permille = -100;
    facts.transfer_variance_class = PROM_VARIANCE_LOW;
    facts.compute_predictability_class = PROM_PREDICTABILITY_STABLE;
    facts.fallback_available = 0u;

    prom_buffering_selector_decision decision{};
    prom_judgment_engine_select_buffering_mode(&facts, &decision);
    ASSERT_EQUAL(0u, decision.success, "all infeasible modes must fail");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE, decision.final_reason_code, "final reason must be explicit no-feasible");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_FIXED_DOUBLE_MEMORY_INSUFFICIENT, decision.fixed_double_rejection_reason,
                 "fixed rejection reason should be populated");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_PULL_LAG_MEMORY_INSUFFICIENT, decision.pull_lag_rejection_reason,
                 "pull-lag rejection reason should be populated");
    ASSERT_EQUAL(PROM_BUFFERING_REASON_SERIAL_JIT_MEMORY_INSUFFICIENT, decision.serial_jit_rejection_reason,
                 "serial rejection reason should be populated");
}

FACT(PrometheusJudgmentEngine_P13M2_OccupancyBandClassificationDeterministic)
{
    prom_occupancy_selector_facts facts{};
    facts.register_file_class = 3u;
    facts.shared_memory_class = 3u;
    facts.memory_bandwidth_class = 3u;
    facts.fp32_throughput_class = 3u;
    facts.max_workgroup_class = 3u;
    facts.queue_capability_class = 3u;
    facts.m = 512u;
    facts.n = 512u;
    facts.k = 512u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;

    prom_occupancy_selector_decision a{};
    prom_occupancy_selector_decision b{};
    prom_judgment_engine_select_occupancy_variant(&facts, &a);
    prom_judgment_engine_select_occupancy_variant(&facts, &b);
    ASSERT_EQUAL(a.device_band, b.device_band, "same facts should classify to same occupancy device band");
    ASSERT_EQUAL(a.selected_variant, b.selected_variant, "same facts should pick same variant");
}

FACT(PrometheusJudgmentEngine_P13M2_OccupancySelectorClampAndOverrideRules)
{
    prom_occupancy_selector_facts facts{};
    facts.register_file_class = 1u;
    facts.shared_memory_class = 2u;
    facts.memory_bandwidth_class = 2u;
    facts.fp32_throughput_class = 2u;
    facts.max_workgroup_class = 1u;
    facts.queue_capability_class = 2u;
    facts.m = 2048u;
    facts.n = 2048u;
    facts.k = 2048u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;

    prom_occupancy_selector_decision decision{};
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_TRUE(decision.selected_variant != static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8),
                "register-constrained occupancy band should not use aggressive variant");

    facts.register_file_class = 5u;
    facts.shared_memory_class = 4u;
    facts.memory_bandwidth_class = 2u;
    facts.fp32_throughput_class = 5u;
    facts.max_workgroup_class = 4u;
    facts.queue_capability_class = 4u;
    facts.m = 1024u;
    facts.n = 1536u;
    facts.k = 4096u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8), decision.selected_variant,
                 "compute-rich FFN-like shape should allow aggressive variant");

    facts.m = 128u;
    facts.n = 128u;
    facts.k = 128u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;
    prom_occupancy_selector_decision small{};
    prom_judgment_engine_select_occupancy_variant(&facts, &small);
    ASSERT_TRUE(small.selected_variant != decision.selected_variant, "shape class should affect occupancy variant");

    facts.register_file_class = 1u;
    facts.shared_memory_class = 2u;
    facts.memory_bandwidth_class = 2u;
    facts.fp32_throughput_class = 2u;
    facts.max_workgroup_class = 1u;
    facts.queue_capability_class = 2u;
    facts.m = 128u;
    facts.n = 128u;
    facts.k = 128u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE), decision.selected_variant,
                 "register-constrained small shapes should now reach memory-conservative");

    facts.register_file_class = 0u;
    facts.shared_memory_class = 0u;
    facts.memory_bandwidth_class = 0u;
    facts.fp32_throughput_class = 0u;
    facts.max_workgroup_class = 0u;
    facts.queue_capability_class = 0u;
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_REASON_UNKNOWN_DEVICE_FALLBACK), decision.clamp_reason,
                 "unknown occupancy device should emit explicit fallback reason");
    ASSERT_EQUAL(1u, decision.fallback_used, "unknown occupancy device should mark fallback");

    facts.register_file_class = 3u;
    facts.shared_memory_class = 3u;
    facts.memory_bandwidth_class = 3u;
    facts.fp32_throughput_class = 3u;
    facts.max_workgroup_class = 3u;
    facts.queue_capability_class = 3u;
    facts.m = 512u;
    facts.n = 512u;
    facts.k = 512u;
    facts.work_units = static_cast<std::uint64_t>(facts.m) * facts.n * facts.k;
    facts.manual_override_enabled = 1u;
    facts.manual_override_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE);
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_EQUAL(1u, decision.override_used, "safe occupancy override should be accepted");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_REASON_MANUAL_OVERRIDE_USED), decision.clamp_reason,
                 "accepted override should have explicit reason");

    facts.register_file_class = 1u;
    facts.max_workgroup_class = 1u;
    facts.manual_override_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8);
    prom_judgment_engine_select_occupancy_variant(&facts, &decision);
    ASSERT_EQUAL(0u, decision.override_used, "unsafe occupancy override should be rejected");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_OCCUPANCY_REASON_OVERRIDE_REJECTED), decision.clamp_reason,
                 "rejected override should have explicit reason");
}

FACT(PrometheusJudgmentEngine_P13M9_ResourceLeaseDecisionReasonsAndLookaheadBound)
{
    prom_resource_lease_facts facts{};
    facts.worker_id = 2u;
    facts.slot_id = 1u;
    facts.entry_id = 77u;
    facts.selected_recipe_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4);
    facts.requested_resource_class = static_cast<std::uint32_t>(PROM_LEASE_RESOURCE_CLASS_COMPUTE);
    facts.current_outstanding_depth = 1u;
    facts.max_outstanding_depth = 3u;
    facts.lookahead_requested = 1u;
    facts.lookahead_limit = 3u;
    facts.transfer_overlap_available = 1u;
    facts.true_multi_queue_selected = 1u;

    prom_resource_lease_decision decision{};
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(1u, decision.grant, "safe lease facts should grant");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_STATE_GRANTED), decision.lease_state, "safe lease should be granted");
    ASSERT_EQUAL(1u, decision.lookahead_allowed, "lookahead should be allowed under cap");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_GRANTED), decision.deny_reason, "grant reason should be explicit");

    facts.failed_slot_mask = (1u << facts.slot_id);
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "failed slot should deny lease");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_SLOT_FAILED), decision.deny_reason, "failed-slot deny reason should be explicit");

    facts.failed_slot_mask = 0u;
    facts.invalidated_slot_mask = (1u << facts.slot_id);
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "invalidated slot should deny lease");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED), decision.deny_reason,
                 "invalidated-slot deny reason should be explicit");

    facts.invalidated_slot_mask = 0u;
    facts.unsafe_to_reuse = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "unsafe runtime should deny lease");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME), decision.deny_reason,
                 "unsafe runtime deny reason should be explicit");

    facts.unsafe_to_reuse = 0u;
    facts.current_outstanding_depth = facts.max_outstanding_depth;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "outstanding depth at cap should deny lease");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT), decision.deny_reason,
                 "outstanding limit deny reason should be explicit");
    ASSERT_EQUAL(0u, decision.lookahead_allowed, "lookahead should be blocked at cap");

    facts.current_outstanding_depth = 1u;
    facts.yield_requested = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_STATE_YIELDED), decision.lease_state, "yield request should emit yielded state");
}

FACT(PrometheusJudgmentEngine_P13_M13_LeaseUtilityPolicy_HardUtilityFairnessAndLookahead)
{
    prom_resource_lease_facts facts{};
    facts.worker_id = 1u;
    facts.slot_id = 0u;
    facts.requested_resource_class = static_cast<std::uint32_t>(PROM_LEASE_RESOURCE_CLASS_COMPUTE);
    facts.current_outstanding_depth = 1u;
    facts.max_outstanding_depth = 4u;
    facts.lookahead_requested = 1u;
    facts.lookahead_limit = 3u;
    facts.ready_slot_mask = 1u;
    facts.slot_attention_mask = 1u;
    facts.pipeline_latency_pressure_class = 3u;
    facts.transfer_overlap_available = 1u;
    facts.true_multi_queue_selected = 1u;

    prom_resource_lease_decision decision{};
    facts.failed_slot_mask = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP), decision.detail, "hard gates must override utility");

    facts.failed_slot_mask = 0u;
    facts.register_pressure_class = 1u;
    facts.shared_memory_pressure_class = 1u;
    facts.memory_bandwidth_pressure_class = 1u;
    facts.compute_pressure_class = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(1u, decision.grant, "safe ready slot should grant");
    ASSERT_EQUAL(1u, decision.lookahead_allowed, "latency-dominant lookahead should be allowed");

    facts.register_pressure_class = 5u;
    facts.shared_memory_pressure_class = 5u;
    facts.compute_pressure_class = 5u;
    facts.lookahead_requested = 0u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "high pressure should backpressure");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_UTILITY_BACKPRESSURE_PRESSURE_OR_CONTENTION), decision.detail,
                 "pressure backpressure reason should be explicit");

    facts.register_pressure_class = 1u;
    facts.shared_memory_pressure_class = 1u;
    facts.compute_pressure_class = 1u;
    facts.lookahead_requested = 1u;
    facts.lookahead_limit = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.lookahead_allowed, "lookahead should block at lookahead limit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_HARD_BLOCK_LOOKAHEAD_LIMIT_OR_TRANSFER), decision.detail,
                 "lookahead block reason should be explicit");

    facts.lookahead_limit = 3u;
    facts.yield_requested = 1u;
    facts.lease_held = 0u;
    facts.current_outstanding_depth = 0u;
    facts.max_outstanding_depth = 0u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_NO_YIELD_WITHOUT_HELD_LEASE), decision.detail,
                 "yield without held lease should be denied");
    facts.lease_held = 1u;
    facts.max_outstanding_depth = 4u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_STATE_YIELDED), decision.lease_state, "yield with held lease should pass");

    facts.yield_requested = 0u;
    facts.worker_id = 3u;
    facts.slot_id = 1u;
    facts.ready_slot_mask = 0x3u;
    facts.slot_attention_mask = 0x3u;
    for (std::uint32_t i = 0; i < 8u; ++i) {
        prom_judgment_engine_decide_resource_lease(&facts, &decision);
    }
    facts.worker_id = 4u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(1u, decision.grant, "under-served worker should still receive grant");
}

FACT(PrometheusJudgmentEngine_P13_M14_ResourceLeaseDecisionIsPureFromFacts)
{
    prom_resource_lease_facts facts{};
    facts.worker_id = 3u;
    facts.slot_id = 1u;
    facts.entry_id = 5u;
    facts.shape_class = static_cast<std::uint32_t>(PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE);
    facts.device_band = static_cast<std::uint32_t>(PROM_OCCUPANCY_DEVICE_BAND_BALANCED);
    facts.selected_recipe_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE);
    facts.requested_resource_class = static_cast<std::uint32_t>(PROM_LEASE_RESOURCE_CLASS_COMPUTE);
    facts.register_pressure_class = 2u;
    facts.shared_memory_pressure_class = 3u;
    facts.memory_bandwidth_pressure_class = 3u;
    facts.compute_pressure_class = 2u;
    facts.pipeline_latency_pressure_class = 3u;
    facts.current_outstanding_depth = 0u;
    facts.max_outstanding_depth = 2u;
    facts.lookahead_requested = 1u;
    facts.lookahead_limit = 2u;
    facts.ready_slot_mask = (1u << facts.slot_id);
    facts.transfer_overlap_available = 1u;
    facts.true_multi_queue_selected = 1u;

    prom_resource_lease_decision baseline{};
    prom_judgment_engine_decide_resource_lease(&facts, &baseline);
    for (std::uint32_t i = 0u; i < 32u; ++i) {
        prom_resource_lease_decision again{};
        prom_judgment_engine_decide_resource_lease(&facts, &again);
        ASSERT_EQUAL(baseline.lease_state, again.lease_state, "pure lease decision should be deterministic across repeated calls");
        ASSERT_EQUAL(baseline.grant, again.grant, "grant bit must remain stable for identical facts");
        ASSERT_EQUAL(baseline.detail, again.detail, "detail reason must remain stable for identical facts");
    }
}

FACT(PrometheusJudgmentEngine_P13_M14_SingleCallModeSkipsContentionBackpressureButKeepsHardGates)
{
    prom_resource_lease_facts facts{};
    facts.worker_id = 0u;
    facts.slot_id = 0u;
    facts.requested_resource_class = static_cast<std::uint32_t>(PROM_LEASE_RESOURCE_CLASS_COMPUTE);
    facts.single_call_mode = 1u;
    facts.current_outstanding_depth = 0u;
    facts.max_outstanding_depth = 1u;
    facts.lookahead_limit = 1u;
    facts.ready_slot_mask = 1u;
    facts.slot_attention_mask = 1u;
    facts.register_pressure_class = 4u;
    facts.shared_memory_pressure_class = 4u;
    facts.memory_bandwidth_pressure_class = 4u;
    facts.compute_pressure_class = 4u;

    prom_resource_lease_decision decision{};
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(1u, decision.grant, "single-call mode should grant after hard gates pass");

    facts.unsafe_to_reuse = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "unsafe hard gate should still deny single-call mode");
    facts.unsafe_to_reuse = 0u;

    facts.failed_slot_mask = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(1u, decision.grant, "single-call mode should ignore stale failed/invalidated slot masks");

    facts.single_call_mode = 0u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "batch mode must still deny failed-slot mask");

    facts.failed_slot_mask = 0u;
    facts.single_call_mode = 1u;
    facts.current_outstanding_depth = 1u;
    facts.max_outstanding_depth = 1u;
    prom_judgment_engine_decide_resource_lease(&facts, &decision);
    ASSERT_EQUAL(0u, decision.grant, "single-call mode must still deny at outstanding-depth cap");
}
