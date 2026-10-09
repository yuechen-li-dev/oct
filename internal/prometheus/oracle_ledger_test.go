package prometheus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Validate archived artifact integrity, not scientific language semantics.
func TestCompletedOracleLedgerPinsArchivedWitnessBytes(t *testing.T) {
	root := filepath.Join("DevelopmentReport", "artifacts", "Evt2OctOracle")
	data, err := os.ReadFile(filepath.Join(root, "experiment_ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ledger struct {
		Experiments []struct {
			Identities []string `json:"artifact_identities"`
		} `json:"experiments"`
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, experiment := range ledger.Experiments {
		for _, identity := range experiment.Identities {
			split := strings.LastIndex(identity, ":")
			if split < 0 {
				continue
			}
			name, want := identity[:split], identity[split+1:]
			// Other entries describe external tensor ranges rather than files.
			if len(want) != 64 || !strings.Contains(name, ".") {
				continue
			}
			if filepath.Base(name) != name {
				t.Fatalf("non-local archive identity %q", identity)
			}
			bytes, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			hash := sha256.Sum256(bytes)
			if got := hex.EncodeToString(hash[:]); got != want {
				t.Errorf("historical witness %s changed: got %s, ledger pins %s; publish a new run separately", name, got, want)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("ledger pins no archived witness files")
	}
}
