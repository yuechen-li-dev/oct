package tester

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// conformanceEligibleCorpus is intentionally bounded. Entries must be
// deterministic, native-independent, and strictly compiled-capable. Broader
// Language corpus coverage remains in the ordinary interpreted/compiled/auto
// lanes, where intentional fallback is reported separately.
var conformanceEligibleCorpus = []string{
	"Language/Testing/CompiledBuiltinSweep/valid/core_pure_builtins.octest",
	"Language/Testing/CompiledStringBuiltins/valid/core_string_builtins.octest",
	"Language/Testing/CompiledArrayLowering/valid/core_array_lowering.octest",
	"Language/Testing/CompiledCallbacks/valid/suite.octest",
	"Language/ControlFlow/OctomataFlowExprParity/valid/flow_expr_parity_surface.octest",
	"Language/ControlFlow/OctomataCheckpointDeterminism/valid/checkpoint_resume_determinism.octest",
}

func TestConformanceEligibleCorpusHasInterpretedCompiledParity(t *testing.T) {
	for _, relativePath := range conformanceEligibleCorpus {
		relativePath := relativePath
		t.Run(filepath.ToSlash(relativePath), func(t *testing.T) {
			target := filepath.Join("..", "..", filepath.FromSlash(relativePath))
			interpreted := executeConformanceMode(t, target, "interpreted")
			compiled := executeConformanceMode(t, target, "compiled")

			interpretedSemantics := conformanceObservableOutcome(interpreted)
			compiledSemantics := conformanceObservableOutcome(compiled)
			if interpretedSemantics != compiledSemantics {
				t.Fatalf("observable outcome differs\ninterpreted:\n%s\ncompiled:\n%s", interpretedSemantics, compiledSemantics)
			}
			if strings.Contains(compiled, "interpreted fallback:") && !strings.Contains(compiled, "interpreted fallback: 0") {
				t.Fatalf("unexpected fallback in conformance-eligible corpus:\n%s", compiled)
			}
			t.Log("compiled-capable=true compiled-pass=true parity-pass=true intentional-fallback=false unexpected-fallback=false")
		})
	}
}

func executeConformanceMode(t *testing.T, target, mode string) string {
	t.Helper()
	var output bytes.Buffer
	if err := ExecuteWithOptions(target, &output, TestOptions{Execution: mode}); err != nil {
		t.Fatalf("%s execution failed: %v\n%s", mode, err, output.String())
	}
	return output.String()
}

func conformanceObservableOutcome(output string) string {
	lines := make([]string, 0)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "PASS ") || strings.HasPrefix(line, "FAIL ") || strings.HasPrefix(line, "SKIP ") || strings.HasPrefix(line, "Result: ") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
