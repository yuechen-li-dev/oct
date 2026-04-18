package interpret

import (
	"fmt"
	"os"
	"path/filepath"
)

const machinaUIWasmArtifactName = "machina-ui-m98-counter.wasm"

// EmitMachinaUIWasmArtifact writes the M98 Machina UI Wasm module to outputPath.
//
// Current scope is intentionally narrow: this module implements the locked M96/M97
// JSON/boundary contract for the canonical counter+routing Machina UI slice used
// by runtime harness tests via direct Wasm emission (no generated-C production backend).
func EmitMachinaUIWasmArtifact(outputPath string) error {
	if outputPath == "" {
		return fmt.Errorf("machina ui wasm emission requires non-empty output path")
	}
	artifactBytes, err := emitMachinaUIWasmFromLowering()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("machina ui wasm emission failed creating output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, artifactBytes, 0o644); err != nil {
		return fmt.Errorf("machina ui wasm emission failed writing artifact: %w", err)
	}
	return nil
}
