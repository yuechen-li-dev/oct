// compiled_model_lock resolves the deliberately closed EVT-2 Z-Image Turbo
// compiled-model manifest. It is a linker PoC, not a runtime graph loader.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/octagon"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const (
	noiseRefiner0 = "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e"
	noiseRefiner1 = "80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8"
	block1Oracle  = "9b133c9ed3772f782e1bd77ff5b89732dc28406eec2078f1692d1899e2eb39e7"
)

func main() {
	manifest := flag.String("manifest", "internal/prometheus/models/zimage-turbo/manifest.oct", "compiled-model manifest.oct")
	out := flag.String("out", "", "lock-tagon.octagon output path (defaults beside manifest)")
	check := flag.Bool("check", false, "require an existing byte-identical lock")
	nativeOut := flag.String("native-out", "", "generated native descriptor header (defaults beside lock)")
	auditNativeOut := flag.String("audit-native-out", "", "generated native audit schedule header (defaults beside lock)")
	auditLayoutOut := flag.String("audit-layout-out", "", "generated audit arena layout JSON (defaults beside lock)")
	flag.Parse()
	lockPath := *out
	if lockPath == "" {
		lockPath = filepath.Join(filepath.Dir(*manifest), "lock-tagon.octagon")
	}
	nativePath := *nativeOut
	if nativePath == "" {
		nativePath = filepath.Join(filepath.Dir(lockPath), "resolved_descriptor.h")
	}
	auditNativePath := *auditNativeOut
	if auditNativePath == "" {
		auditNativePath = filepath.Join(filepath.Dir(lockPath), "resolved_audit_schedule.h")
	}
	auditLayoutPath := *auditLayoutOut
	if auditLayoutPath == "" {
		auditLayoutPath = filepath.Join(filepath.Dir(lockPath), "resolved_audit_arena.json")
	}
	data, err := os.ReadFile(*manifest)
	if err != nil {
		fail("read manifest: %v", err)
	}
	file, err := source.Load(*manifest)
	if err != nil {
		fail("load manifest: %v", err)
	}
	lexed, err := lex.Analyze(file)
	if err != nil {
		fail("lex manifest: %v", err)
	}
	parsed, err := parse.BuildFile(lexed)
	if err != nil {
		fail("parse manifest: %v", err)
	}
	if parsed.Package != "Manifest" || !hasFunction(parsed.Functions, "Manifest") || !hasFunction(parsed.Functions, "CompiledModel") {
		fail("manifest must be a package Manifest with Manifest() and CompiledModel() declarations")
	}
	for _, required := range []string{
		"Name: \"ZImageTurbo\"", "Assembly: \"NoiseRefiner\"", "Parameters: \"noise_refiner.0\"",
		"Parameters: \"noise_refiner.1\"", "sha256:" + noiseRefiner0, "sha256:" + noiseRefiner1,
		"sha256:" + block1Oracle, "InputAbi: \"ModelEmbedding.FP32\"", "OutputAbi: \"ModelEmbedding.FP32\"",
		"Flow: [\"NoiseRefiner0\", \"NoiseRefiner1\"]",
		"AuditProfile: \"NoiseRefinerPersistentProjectionSummary.v1; static; bounded; no prefix replay\"",
	} {
		if !strings.Contains(string(data), required) {
			fail("compiled-model declaration is missing required authority %q", required)
		}
	}
	manifestID := resolvedManifestIdentity(data)
	lock := render(manifestID)
	if *check {
		existing, readErr := os.ReadFile(lockPath)
		if readErr != nil {
			fail("stale lock: %s is required; run compiled_model_lock", lockPath)
		}
		if string(existing) != lock {
			fail("stale lock: %s differs from manifest/artifact resolution; run compiled_model_lock", lockPath)
		}
		if _, parseErr := octagon.Load(lockPath); parseErr != nil {
			fail("lock-tagon is not valid Octagon: %v", parseErr)
		}
		if generated, generateErr := nativeProjection(existing); generateErr != nil {
			fail("generate native descriptor: %v", generateErr)
		} else if actual, actualErr := os.ReadFile(nativePath); actualErr != nil || string(actual) != generated {
			fail("stale native descriptor: %s differs from lock-tagon; run compiled_model_lock", nativePath)
		}
		if generated, layout, generateErr := auditScheduleProjection(existing); generateErr != nil {
			fail("generate native audit schedule: %v", generateErr)
		} else if actual, actualErr := os.ReadFile(auditNativePath); actualErr != nil || string(actual) != generated {
			fail("stale native audit schedule: %s differs from lock-tagon; run compiled_model_lock", auditNativePath)
		} else if actual, actualErr := os.ReadFile(auditLayoutPath); actualErr != nil || string(actual) != layout {
			fail("stale audit arena layout: %s differs from lock-tagon", auditLayoutPath)
		}
		fmt.Printf("compiled model lock valid: %s\n", digest([]byte(lock)))
		return
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		fail("create lock directory: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte(lock), 0o644); err != nil {
		fail("write lock: %v", err)
	}
	if _, parseErr := octagon.Load(lockPath); parseErr != nil {
		fail("generated lock-tagon is not valid Octagon: %v", parseErr)
	}
	projection, projectionErr := nativeProjection([]byte(lock))
	if projectionErr != nil {
		fail("generate native descriptor: %v", projectionErr)
	}
	if err := os.WriteFile(nativePath, []byte(projection), 0o644); err != nil {
		fail("write native descriptor: %v", err)
	}
	auditSchedule, auditLayout, auditErr := auditScheduleProjection([]byte(lock))
	if auditErr != nil {
		fail("generate native audit schedule: %v", auditErr)
	}
	if err := os.WriteFile(auditNativePath, []byte(auditSchedule), 0o644); err != nil {
		fail("write native audit schedule: %v", err)
	}
	if err := os.WriteFile(auditLayoutPath, []byte(auditLayout), 0o644); err != nil {
		fail("write audit arena layout: %v", err)
	}
	fmt.Printf("wrote %s\ncompiled model lock identity: %s\n", lockPath, digest([]byte(lock)))
}

func hasFunction(functions []ast.FunctionDecl, name string) bool {
	for _, fn := range functions {
		if fn.Name == name {
			return true
		}
	}
	return false
}

func digest(data []byte) string       { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func fail(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }

// The authored file is parsed and its closed facts are validated above.  The
// manifest identity normalizes only platform line endings, so the same authored
// declaration resolves identically on Windows and Unix while every semantic
// byte remains part of the accepted lock.
func resolvedManifestIdentity(data []byte) string {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	return digest([]byte(normalized))
}

func render(manifestID string) string {
	semanticID := digest([]byte("ZImageTurbo|f332072aa78be7aecdf3ee76d5c247082da564a6|NoiseRefiner|ModelEmbedding.FP32|ZImageReferenceFp32"))
	productionID := digest([]byte("13 production SDSL-V pipelines; ids 24-36; reused unchanged|AdaLN -> attention -> FFN; resident FP32 boundary|654891776"))
	auditID := digest([]byte("NoiseRefinerPersistentProjectionSummary.v1|static|bounded=47186176|full=adaln-vectors|projection-summary=large-stages|no-prefix-replay"))
	return fmt.Sprintf(`CompiledModelLock {
    Schema: "oct.sdslv.compiled-model-lock-tagon.v1"
    ManifestIdentity: "sha256:%s"
    Model: "ZImageTurbo"
    Revision: "f332072aa78be7aecdf3ee76d5c247082da564a6"
    Checkpoint: "sha256:2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
    Runtime: "Prometheus.Vulkan"
    AssemblyFamily: "ZImageTurbo.NoiseRefiner"
    AssemblyIdentity: "sha256:6517444848718386192"
    InternalAbi: "ModelEmbedding.FP32 -> ModelEmbedding.FP32"
    Precision: "FP16 immutable weights; FP32 arithmetic/activations/reductions; no activation FP16"
    ShaderPortfolio: "13 production SDSL-V pipelines; ids 24-36; reused unchanged"
    MemoryPlan: "654891776 bytes steady-state; 361820672 byte parameter arena"
    ExecutionPlan: "AdaLN -> attention -> FFN; resident FP32 boundary"
	    ModelSemanticIdentity: "sha256:%s"
	    ProductionExecutionIdentity: "sha256:%s"
	    AuditProfile: "NoiseRefinerPersistentProjectionSummary.v1"
	    AuditProfileIdentity: "sha256:%s"
	    AuditBudgetBytes: 47186176
	    AuditPolicy: "Full small vectors; ProjectionAndSummary large persistent stages; static capture at last legal lifetime; no repeated prefix replay"
    Blocks: [
        ResolvedBlock { Id: 0 Name: "NoiseRefiner0" ParameterSet: "NoiseRefiner0" Parameters: "noise_refiner.0" Cache: "sha256:%s" Oracle: "O19.NoiseRefiner0" Predecessor: "" Successor: "NoiseRefiner1" },
        ResolvedBlock { Id: 1 Name: "NoiseRefiner1" ParameterSet: "NoiseRefiner1" Parameters: "noise_refiner.1" Cache: "sha256:%s" Oracle: "sha256:%s" Predecessor: "NoiseRefiner0" Successor: "" }
    ]
    Flow: "NoiseRefiner0 -> NoiseRefiner1; FP32 resident; atomic rebind; no BF16 cast; no host bounce"
    Replay: "assembly-family + parameter-set + binding-generation + input + execution-contract"
}
`, manifestID, semanticID, productionID, auditID, noiseRefiner0, noiseRefiner1, block1Oracle)
}

// nativeProjection is intentionally derived from the immutable lock document,
// not the authored manifest. Native code gets a closed const descriptor table;
// it never reparses a model name or independently reconstructs topology.
func nativeProjection(lock []byte) (string, error) {
	for _, required := range []string{"AssemblyFamily: \"ZImageTurbo.NoiseRefiner\"", "Id: 0", "Id: 1", "sha256:" + noiseRefiner0, "sha256:" + noiseRefiner1, "FP32 resident"} {
		if !strings.Contains(string(lock), required) {
			return "", fmt.Errorf("lock missing %q", required)
		}
	}
	identity := digest(lock)
	return fmt.Sprintf(`/* Generated by tools/compiled_model_lock from lock-tagon.octagon. Do not edit. */
#ifndef OCT_ZIMAGE_TURBO_RESOLVED_DESCRIPTOR_H
#define OCT_ZIMAGE_TURBO_RESOLVED_DESCRIPTOR_H

#define PROM_ZIMAGE_TURBO_LOCK_ID 0x%sull
#define PROM_ZIMAGE_TURBO_NO_BLOCK UINT32_MAX
static const PrometheusNoiseRefinerResolvedDescriptor k_prom_zimage_turbo_noise_refiner_blocks[] = {
  {PROM_ZIMAGE_TURBO_LOCK_ID, 0u, PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO, PROM_NOISE_REFINER_PARAMETER_SET_0, 0x%sull, 0u, 0u, 0u, 654891776ull, 6517444848718386192ull, 0u, PROM_ZIMAGE_TURBO_NO_BLOCK, 1u},
  {PROM_ZIMAGE_TURBO_LOCK_ID, 1u, PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO, PROM_NOISE_REFINER_PARAMETER_SET_1, 0x%sull, 0u, 0u, 0u, 654891776ull, 6517444848718386192ull, 0x%sull, 0u, PROM_ZIMAGE_TURBO_NO_BLOCK},
};
static const PrometheusNoiseRefinerResolvedDescriptor* prom_zimage_turbo_resolve_noise_refiner_descriptor(uint64_t lock_identity, uint32_t model_local_block_id) {
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || model_local_block_id > 1u) return NULL;
  return &k_prom_zimage_turbo_noise_refiner_blocks[model_local_block_id];
}
#endif
`, identity[:16], noiseRefiner0[:16], noiseRefiner1[:16], block1Oracle[:16]), nil
}
