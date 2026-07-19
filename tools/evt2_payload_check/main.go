// evt2_payload_check resolves and validates the intentionally local EVT-2
// authority without copying large tensors or writing artifacts.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

import "github.com/yuechen-li-dev/oct/internal/prometheus/zimage"

func main() {
	paths, err := zimage.NoiseRefiner0PayloadPathsFromEnvironment()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("OCT_EVT2_CACHE=%s\n", paths.CacheRoot)
	fmt.Printf("OCT_EVT2_ORACLE=%s\n", paths.OracleRoot)
	bundle, err := zimage.LoadNoiseRefiner0PayloadBundle(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("cache aggregate=%s\n", bundle.CacheManifest.AggregateSHA256)
	fmt.Printf("block input=%s\n", bundle.Input.SHA256)
	fmt.Printf("timestep=%s\n", bundle.Timestep.SHA256)
	fmt.Printf("stage witness count=%d\n", len(bundle.StageNames))
	projectionPath := "internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/canonical_stage_projections.json"
	projectionBytes, err := os.ReadFile(projectionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical stage projections absent: %s; %s\n", projectionPath, zimage.NoiseRefiner0PayloadGuide)
		os.Exit(1)
	}
	projectionHash := sha256.Sum256(projectionBytes)
	projectionIdentity := hex.EncodeToString(projectionHash[:])
	if projectionIdentity != zimage.NoiseRefiner0StageProjectionsSHA256 {
		fmt.Fprintf(os.Stderr, "canonical stage projections identity mismatch: got %s want %s; %s\n", projectionIdentity, zimage.NoiseRefiner0StageProjectionsSHA256, zimage.NoiseRefiner0PayloadGuide)
		os.Exit(1)
	}
	fmt.Printf("canonical stage projections=%s\n", projectionIdentity)
}
