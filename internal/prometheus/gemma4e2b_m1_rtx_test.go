package prometheus

import (
	"encoding/json"
	"os"
	"testing"
)

func TestGemma4E2BM1CanonicalQKVRTX(t *testing.T) {
	if os.Getenv("OCT_RUN_PROMETHEUS_INTEGRATION") != "1" {
		t.Skip("set OCT_RUN_PROMETHEUS_INTEGRATION=1 to run the real Gemma4 E2B M1 RTX slice")
	}
	checkpointRoot := os.Getenv("G4E2B_CHECKPOINT_ROOT")
	if checkpointRoot == "" {
		t.Skip("set G4E2B_CHECKPOINT_ROOT to the validated external checkpoint root")
	}
	result, err := runGemma4e2bCanonicalQKVRTX(checkpointRoot)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(encoded))
}
