// evt2_payload_check validates the intentionally local EVT-2 authorities
// without copying large tensors or writing artifacts.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

type payload struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	DType        string `json:"dtype"`
	Shape        []int  `json:"shape"`
	Projection   struct {
		Finite bool `json:"finite"`
	} `json:"projection"`
}

type stageManifest struct {
	Schema      string             `json:"schema"`
	RopeFrame   int                `json:"rope_frame"`
	Stages      map[string]payload `json:"stages"`
	FinalOutput payload            `json:"final_output"`
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"; %s\n", append(args, zimage.NoiseRefiner0PayloadGuide)...)
	os.Exit(1)
}

func readManifest(path string) (stageManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return stageManifest{}, err
	}
	var manifest stageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return stageManifest{}, err
	}
	return manifest, nil
}

func validateCanonical(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("O19 canonical stage directory absent: %s", root)
		}
		return fmt.Errorf("O19 canonical stage directory check: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("O19 canonical stage root is not a directory: %s", root)
	}
	localManifestPath := filepath.Join(root, "o19_stage_manifest.json")
	localManifestHash, err := fileHash(localManifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("O19 canonical stage manifest absent: %s", localManifestPath)
		}
		return fmt.Errorf("O19 canonical stage manifest read: %w", err)
	}
	if localManifestHash != zimage.NoiseRefiner0StageManifestSHA256 {
		return fmt.Errorf("O19 canonical stage manifest identity mismatch: got %s want %s", localManifestHash, zimage.NoiseRefiner0StageManifestSHA256)
	}
	projectionPath := filepath.Join(root, "o19_stage_projections.json")
	projectionHash, err := fileHash(projectionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("O19 canonical stage projections absent: %s", projectionPath)
		}
		return fmt.Errorf("O19 canonical stage projections read: %w", err)
	}
	if projectionHash != zimage.NoiseRefiner0StageProjectionsSHA256 {
		return fmt.Errorf("O19 canonical stage projections identity mismatch: got %s want %s", projectionHash, zimage.NoiseRefiner0StageProjectionsSHA256)
	}
	want, err := readManifest("internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/canonical_stage_manifest.json")
	if err != nil {
		return fmt.Errorf("committed O19 stage manifest absent or invalid: %w", err)
	}
	got, err := readManifest(localManifestPath)
	if err != nil {
		return fmt.Errorf("O19 canonical stage manifest invalid: %w", err)
	}
	if want.Schema != "oct.prometheus.evt2.o19.canonical-stage-manifest.v2" || got.Schema != want.Schema || got.RopeFrame != 33 {
		return fmt.Errorf("O19 canonical stage manifest contract mismatch")
	}
	if len(want.Stages) != 34 || len(got.Stages) != len(want.Stages) {
		return fmt.Errorf("O19 canonical full-stage payload count mismatch: got %d want %d", len(got.Stages), len(want.Stages))
	}
	for name, expected := range want.Stages {
		actual, ok := got.Stages[name]
		if !ok {
			return fmt.Errorf("O19 canonical stage payload missing from manifest: %s", name)
		}
		if actual.RelativePath != expected.RelativePath || actual.SHA256 != expected.SHA256 || actual.Bytes != expected.Bytes || actual.DType != expected.DType || !actual.Projection.Finite {
			return fmt.Errorf("O19 canonical stage identity mismatch: %s", name)
		}
		path := filepath.Join(root, filepath.FromSlash(actual.RelativePath))
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("O19 canonical stage payload missing: %s", actual.RelativePath)
			}
			return fmt.Errorf("O19 canonical stage payload check %s: %w", name, err)
		}
		if info.Size() != actual.Bytes {
			return fmt.Errorf("O19 canonical stage byte count mismatch: %s got %d want %d", name, info.Size(), actual.Bytes)
		}
		hash, err := fileHash(path)
		if err != nil {
			return fmt.Errorf("O19 canonical stage hash read %s: %w", name, err)
		}
		if hash != actual.SHA256 {
			return fmt.Errorf("O19 canonical stage payload identity mismatch: %s got %s want %s", name, hash, actual.SHA256)
		}
	}
	finalPath := filepath.Join(root, "final_output.f32.bin")
	finalHash, err := fileHash(finalPath)
	if err != nil {
		return fmt.Errorf("O19 final diagnostic payload missing: %w", err)
	}
	if finalHash != zimage.NoiseRefiner0FinalDiagnosticSHA256 || got.FinalOutput.SHA256 != zimage.NoiseRefiner0FinalDiagnosticSHA256 {
		return fmt.Errorf("O19 final diagnostic identity mismatch: got %s want %s", finalHash, zimage.NoiseRefiner0FinalDiagnosticSHA256)
	}
	return nil
}

func validateNoiseRefiner1Canonical(root string) error {
	manifestPath := filepath.Join(root, "manifest.json")
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("noise_refiner.1 canonical manifest: %w", err)
	}
	if manifest.Schema != "oct.prometheus.evt2.o19.fp32-reference-bundle.v1" || manifest.RopeFrame != 33 || len(manifest.Stages) != 34 {
		return fmt.Errorf("noise_refiner.1 canonical manifest contract mismatch")
	}
	for name, stage := range manifest.Stages {
		if !stage.Projection.Finite || stage.RelativePath == "" || stage.SHA256 == "" || stage.Bytes == 0 {
			return fmt.Errorf("noise_refiner.1 canonical stage metadata mismatch: %s", name)
		}
		path := filepath.Join(root, filepath.FromSlash(stage.RelativePath))
		info, statErr := os.Stat(path)
		if statErr != nil || info.Size() != stage.Bytes {
			return fmt.Errorf("noise_refiner.1 canonical stage missing or truncated: %s", name)
		}
		digest, hashErr := fileHash(path)
		if hashErr != nil || digest != stage.SHA256 {
			return fmt.Errorf("noise_refiner.1 canonical stage identity mismatch: %s", name)
		}
	}
	finalPath := filepath.Join(root, "final_output.f32.bin")
	digest, err := fileHash(finalPath)
	if err != nil || digest != zimage.NoiseRefiner1CanonicalFinalSHA256 || manifest.FinalOutput.SHA256 != zimage.NoiseRefiner1CanonicalFinalSHA256 {
		return fmt.Errorf("noise_refiner.1 canonical final identity mismatch")
	}
	return nil
}

func main() {
	paths, err := zimage.NoiseRefiner0PayloadPathsFromEnvironment()
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("OCT_EVT2_CACHE=%s\n", paths.CacheRoot)
	fmt.Printf("OCT_EVT2_ORACLE=%s\n", paths.OracleRoot)
	bundle, err := zimage.LoadNoiseRefiner0PayloadBundle(paths)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("cache aggregate=%s\n", bundle.CacheManifest.AggregateSHA256)
	fmt.Printf("block input=%s\n", bundle.Input.SHA256)
	fmt.Printf("timestep=%s\n", bundle.Timestep.SHA256)
	fmt.Printf("M1B replay authority: valid\n")

	historical := zimage.NoiseRefiner0HistoricalCaptureRoot(paths.CacheRoot)
	if configured := os.Getenv("OCT_EVT2_M1B_CANONICAL"); configured != "" && strings.EqualFold(filepath.Clean(configured), filepath.Clean(historical)) {
		fail("historical capture_04 was selected through OCT_EVT2_M1B_CANONICAL; it is non-normative and incompatible with O19 M1C-M1E acceptance")
	}
	if err := validateCanonical(zimage.NoiseRefiner0O19CanonicalRoot(paths.CacheRoot)); err != nil {
		fail("%v", err)
	}
	fmt.Printf("M1C-M1E canonical stage authority: valid\n")
	if _, err := zimage.LoadNoiseRefiner1CacheManifest(paths.CacheRoot); err != nil {
		fail("%v", err)
	}
	if err := validateNoiseRefiner1Canonical(zimage.NoiseRefiner1CanonicalRoot(paths.CacheRoot)); err != nil {
		fail("%v", err)
	}
	fmt.Printf("noise_refiner.0 authority: valid\n")
	fmt.Printf("noise_refiner.1 authority: valid\n")
	fmt.Printf("two-block chain authority: valid\n")
}
