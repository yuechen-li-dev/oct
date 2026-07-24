package prometheus

import (
	"encoding/json"
	"os"
	"testing"
)

func gemma4e2bIntegrationRoot(t *testing.T) string {
	t.Helper()
	if os.Getenv("OCT_RUN_PROMETHEUS_INTEGRATION") != "1" {
		t.Skip("set OCT_RUN_PROMETHEUS_INTEGRATION=1 to run the real Gemma4 E2B M1 RTX slice")
	}
	checkpointRoot := os.Getenv("G4E2B_CHECKPOINT_ROOT")
	if checkpointRoot == "" {
		t.Skip("set G4E2B_CHECKPOINT_ROOT to the validated external checkpoint root")
	}
	return checkpointRoot
}

func assertFreshGemmaRawScoreAuthority(t *testing.T, result gemma4e2bCanonicalSliceResult, preparationOrder uint32) {
	t.Helper()
	if result.PreparationOrder != preparationOrder {
		t.Fatalf("preparation order was not reported explicitly: got %d want %d", result.PreparationOrder, preparationOrder)
	}
	if result.ScoreNative.PositionalDispatchCount != 2 || result.ScoreNative.ScoreDispatchCount != 1 ||
		result.ScoreNative.ScoreReadbackCount != 1 || result.ScoreNative.HostDetourCount != 0 ||
		!result.ScoreNative.ScoreWritten {
		t.Fatalf("fresh-session raw-score lifecycle was incomplete: %+v", result.ScoreNative)
	}
	if len(result.Scores) != 1800 {
		t.Fatalf("fresh-session raw-score authority checked %d values, want 1800", len(result.Scores))
	}
	if !result.ScoreStageLocalExact {
		t.Fatalf("fresh-session raw-score tensor differs from the sequential stage-local authority: %+v", result.ScoreStageLocal)
	}
	if result.ScorePristineExact {
		t.Log("pristine and resident positional fixtures are currently bit-identical")
	}
	t.Logf("preparation_order=%d scores=%d/%d stage_local=%+v pristine=%+v native=%+v", preparationOrder, len(result.Scores), 1800, result.ScoreStageLocal, result.ScorePristine, result.ScoreNative)
}

// TestGemma4E2BM1FreshSessionQFirstAuthority is the independent Q-first
// numerical authority. It is intentionally not a same-session reuse test.
func TestGemma4E2BM1FreshSessionQFirstAuthority(t *testing.T) {
	checkpointRoot := gemma4e2bIntegrationRoot(t)
	result, err := runGemma4e2bFreshSessionRawScoreAuthority(checkpointRoot, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshGemmaRawScoreAuthority(t, result, 1)
}

// TestGemma4E2BM1FreshSessionKFirstAuthority is the independent K-first
// numerical authority. A separate test invocation creates fresh runtime state.
func TestGemma4E2BM1FreshSessionKFirstAuthority(t *testing.T) {
	checkpointRoot := gemma4e2bIntegrationRoot(t)
	result, err := runGemma4e2bFreshSessionRawScoreAuthority(checkpointRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshGemmaRawScoreAuthority(t, result, 0)
}

// TestGemma4E2BM1SameSession7406Characterization is a known-defect
// characterization. It passes only when the current reusable-session
// sequence reaches M49 required-weight validation with -7406 after the
// second chain's M46 preparation boundary and before positional dispatch.
func TestGemma4E2BM1SameSession7406Characterization(t *testing.T) {
	checkpointRoot := gemma4e2bIntegrationRoot(t)
	result, err := runGemma4e2bSameSessionLifecycleCharacterization(checkpointRoot)
	if err != nil {
		t.Fatal(err)
	}
	trace := result.SameSession
	if trace == nil {
		t.Fatal("same-session characterization produced no lifecycle trace")
	}
	if !trace.FirstScore.ScoreWritten || trace.FirstScore.PositionalDispatchCount != 2 ||
		trace.FirstScore.ScoreDispatchCount != 1 || trace.FirstScore.ScoreReadbackCount != 1 {
		t.Fatalf("first chain did not complete before the boundary: %+v", trace.FirstScore)
	}
	if !trace.SecondM46PreparationBoundaryObserved {
		t.Fatalf("second-chain M46 preparation boundary was not observed through the current ABI: %+v", trace)
	}
	if !trace.M49RequiredWeightValidationRejected || trace.SecondBoundary.DetailCode != -7406 {
		t.Fatalf("second chain did not reject at the required M49 -7406 boundary: %+v", trace)
	}
	if trace.PositionalDispatchBegun || trace.ScoreDispatchBegun || trace.ScoreDestinationWritten {
		t.Fatalf("dispatch or score output began after the M49 rejection: %+v", trace)
	}
	if trace.SecondBoundary.ObservedWeightGeneration != 0 || trace.SecondBoundary.RequestedWeightGeneration != 0 {
		t.Fatalf("unexpected propagated M46 generation fields at the M49 early return: %+v", trace.SecondBoundary)
	}
	traceJSON, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("marshal same-session lifecycle trace: %v", err)
	}
	t.Logf("known-defect same-session trace JSON: %s", traceJSON)
}
