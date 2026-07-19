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
	noiseRefiner0    = "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e"
	noiseRefiner1    = "80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8"
	block1Oracle     = "9b133c9ed3772f782e1bd77ff5b89732dc28406eec2078f1692d1899e2eb39e7"
	contextRefiner0  = "c08b908a921a80e16995abc3f3eefcadd1a94a78cd76b56e58d6b21e6ce412ae"
	contextRefiner1  = "30268c3b0d7a6fafc411119c929326854e73d394ec95ebeba21f89aa43dfc95f"
	context0Oracle   = "d2b8167de614da25211eb69991e1b7700992bf5f4ae527bff8a55366ea1ae6df"
	context1Oracle   = "08377e8a46b65cff998b740a3fd0ba3c1565b471dad71fcca3f4310617c220b0"
	mainTransformer0 = "48e987811885741ae5f1bf16b28db33ca7f23e09f1e99c1c2fe3d81bdd1caeb6"
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
		"Name: \"ZImageTurbo\"", "Assembly: \"NoiseRefiner\"", "Assembly: \"ContextRefiner\"", "Parameters: \"noise_refiner.0\"",
		"Parameters: \"noise_refiner.1\"", "sha256:" + noiseRefiner0, "sha256:" + noiseRefiner1,
		"Parameters: \"context_refiner.0\"", "Parameters: \"context_refiner.1\"", "sha256:" + contextRefiner0, "sha256:" + contextRefiner1,
		"sha256:" + block1Oracle, "sha256:" + context0Oracle, "sha256:" + context1Oracle,
		"InputAbi: \"ModelEmbedding.FP32\"", "OutputAbi: \"ModelEmbedding.FP32\"",
		"ContextEmbedding -> ContextRefiner0 -> ContextRefiner1", "Parameters: \"layers.0\"", "Assembly: \"MainTransformer\"",
		"sha256:" + mainTransformer0, "Role: \"PreparedImage\"", "Role: \"PreparedContext\"", "Role: \"JointWorking\"",
		"TokenChannel.ImageThenContext", "serialized execution with concurrent session residency",
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
	semanticID := digest([]byte("ZImageTurbo|f332072aa78be7aecdf3ee76d5c247082da564a6|NoiseRefiner+ContextRefiner+MainTransformer0|PreparedImage+PreparedContext+JointWorking|ImageThenContext|ZImageReferenceFp32"))
	productionID := digest([]byte("NoiseRefiner ids 24-36; ContextRefiner ids 25,26,31-37,38,39; MainTransformer ids 40-43|closed session physical ImageThenContext composition|serialized representative execution"))
	auditID := digest([]byte("NoiseRefinerPersistentProjectionSummary.v1+ContextRefinerPersistentProjectionSummary.v1+MainTransformerRepresentativeStatic.v1|static|bounded=47186176|no-prefix-replay"))
	return fmt.Sprintf(`CompiledModelLock {
    Schema: "oct.sdslv.compiled-model-lock-tagon.v1"
    ManifestIdentity: "sha256:%s"
    Model: "ZImageTurbo"
    Revision: "f332072aa78be7aecdf3ee76d5c247082da564a6"
    Checkpoint: "sha256:2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
    Runtime: "Prometheus.Vulkan"
    AssemblyFamily: "ZImageTurbo.ClosedFamilies"
    InternalAbi: "ModelEmbedding.FP32 -> ModelEmbedding.FP32; ContextEmbedding.FP32 -> ContextEmbedding.FP32; PreparedImage | PreparedContext -> JointWorking[ImageThenContext].FP32"
    Precision: "FP16 immutable weights; FP32 arithmetic/activations/reductions; no activation FP16"
    ShaderPortfolio: "NoiseRefiner ids 24-36; ContextRefiner reuses ids 25,26,31-37 and adds ids 38-39; MainTransformer joint ids 40-43"
    MemoryPlan: "NoiseRefiner: 654891776 bytes steady-state; ContextRefiner: 536870912 bytes steady-state; session streams: PreparedImage=15728640, PreparedContext=491520, JointWorking=16220160 bytes"
    ExecutionPlan: "NoiseRefiner: AdaLN -> attention -> FFN; ContextRefiner: RMSNorm -> attention -> FFN; session captures both final FP32 streams; physical ImageThenContext composition; serialized MainTransformer0 representative"
	    ModelSemanticIdentity: "sha256:%s"
	    ProductionExecutionIdentity: "sha256:%s"
    AuditProfile: "NoiseRefinerPersistentProjectionSummary.v1; ContextRefinerPersistentProjectionSummary.v1; MainTransformerRepresentativeStatic.v1"
	    AuditProfileIdentity: "sha256:%s"
	    AuditBudgetBytes: 47186176
	    AuditPolicy: "Full small vectors; ProjectionAndSummary large persistent stages; static capture at last legal lifetime; no repeated prefix replay"
    Blocks: [
        ResolvedBlock { Family: "ZImageTurbo.NoiseRefiner" Id: 0 Name: "NoiseRefiner0" ParameterSet: "NoiseRefiner0" Parameters: "noise_refiner.0" Cache: "sha256:%s" Oracle: "O19.NoiseRefiner0" Predecessor: "" Successor: "NoiseRefiner1" Transport: "ModelEmbedding.FP32 resident" },
        ResolvedBlock { Family: "ZImageTurbo.NoiseRefiner" Id: 1 Name: "NoiseRefiner1" ParameterSet: "NoiseRefiner1" Parameters: "noise_refiner.1" Cache: "sha256:%s" Oracle: "sha256:%s" Predecessor: "NoiseRefiner0" Successor: "" Transport: "ModelEmbedding.FP32 resident" },
        ResolvedBlock { Family: "ZImageTurbo.ContextRefiner" Id: 0 Name: "ContextRefiner0" ParameterSet: "ContextRefiner0" Parameters: "context_refiner.0" Cache: "sha256:%s" Oracle: "sha256:%s" Predecessor: "ContextEmbedding" Successor: "ContextRefiner1" Transport: "ContextEmbedding.FP32 resident" },
        ResolvedBlock { Family: "ZImageTurbo.ContextRefiner" Id: 1 Name: "ContextRefiner1" ParameterSet: "ContextRefiner1" Parameters: "context_refiner.1" Cache: "sha256:%s" Oracle: "sha256:%s" Predecessor: "ContextRefiner0" Successor: "PreparedContext" Transport: "ModelEmbedding.FP32 resident" },
        ResolvedBlock { Family: "ZImageTurbo.MainTransformer" Id: 0 Name: "MainTransformer0" ParameterSet: "MainTransformer0" Parameters: "layers.0" Cache: "sha256:%s" Oracle: "M2C.JointFp32.layers.0" Predecessor: "PreparedImage|PreparedContext" Successor: "MainImageOutput" Transport: "JointWorking.FP32 ImageThenContext resident" }
    ]
    ResidentStreams: [
        ResidentStream { Role: "PreparedImage" Producer: "NoiseRefiner1" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 1024 Width: 3840 Layout: "TokenChannel" Bytes: 15728640 Generation: "replace-on-capture" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" },
        ResidentStream { Role: "PreparedContext" Producer: "ContextRefiner1" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 32 Width: 3840 Layout: "TokenChannel" Bytes: 491520 Generation: "replace-on-capture" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" },
        ResidentStream { Role: "JointWorking" Producer: "M2CPhysicalComposition" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 1056 Width: 3840 Layout: "TokenChannel.ImageThenContext" Bytes: 16220160 Generation: "replace-on-compose" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" }
    ]
    Flow: "NoiseRefiner0 -> NoiseRefiner1 -> PreparedImage and ContextEmbedding -> ContextRefiner0 -> ContextRefiner1 -> PreparedContext; Joint = Image | Context; serialized session execution; atomic rebind; no BF16 cast; no host bounce"
    Replay: "lock + assembly-family + parameter-set + binding-generation + PreparedImage-generation + PreparedContext-generation + joint-composition"
}
`, manifestID, semanticID, productionID, auditID, noiseRefiner0, noiseRefiner1, block1Oracle, contextRefiner0, context0Oracle, contextRefiner1, context1Oracle, mainTransformer0)
}

// nativeProjection is intentionally derived from the immutable lock document,
// not the authored manifest. Native code gets a closed const descriptor table;
// it never reparses a model name or independently reconstructs topology.
func nativeProjection(lock []byte) (string, error) {
	for _, required := range []string{"AssemblyFamily: \"ZImageTurbo.ClosedFamilies\"", "Family: \"ZImageTurbo.NoiseRefiner\"", "Family: \"ZImageTurbo.ContextRefiner\"", "Family: \"ZImageTurbo.MainTransformer\"", "sha256:" + noiseRefiner0, "sha256:" + noiseRefiner1, "sha256:" + contextRefiner0, "sha256:" + contextRefiner1, "sha256:" + mainTransformer0, "Role: \"PreparedImage\"", "Role: \"PreparedContext\"", "Role: \"JointWorking\"", "TokenChannel.ImageThenContext"} {
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
static const PrometheusContextRefinerResolvedDescriptor k_prom_zimage_turbo_context_refiner_blocks[] = {
  {PROM_ZIMAGE_TURBO_LOCK_ID, 0u, PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO, PROM_CONTEXT_REFINER_PARAMETER_SET_0, 0x%sull, 0u, 0u, 0u, 536870912ull, 0x%sull, 0x%sull, PROM_ZIMAGE_TURBO_NO_BLOCK, 1u},
  {PROM_ZIMAGE_TURBO_LOCK_ID, 1u, PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO, PROM_CONTEXT_REFINER_PARAMETER_SET_1, 0x%sull, 0u, 0u, 0u, 536870912ull, 0x%sull, 0x%sull, 0u, PROM_ZIMAGE_TURBO_NO_BLOCK},
};
static const PrometheusCompiledModelResidentStreamDescriptor k_prom_zimage_turbo_resident_streams[] = {
  {PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_STREAM_PREPARED_IMAGE, PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO, 1u, PROM_ZIMAGE_STREAM_SEMANTIC_ABI_MODEL_EMBEDDING_FP32, PROM_ZIMAGE_STREAM_DTYPE_FP32, 1024u, 3840u, PROM_ZIMAGE_STREAM_LAYOUT_TOKEN_CHANNEL, 15728640ull, PROM_ZIMAGE_STREAM_GENERATION_REPLACE_ON_CAPTURE, PROM_ZIMAGE_STREAM_LIFETIME_SESSION, PROM_ZIMAGE_STREAM_CONSUMER_MAIN_TRANSFORMER, PROM_ZIMAGE_STREAM_MUTABILITY_IMMUTABLE, PROM_ZIMAGE_STREAM_ALIAS_FORBIDDEN, PROM_ZIMAGE_STREAM_TRANSPORT_DEVICE_LOCAL},
  {PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT, PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO, 1u, PROM_ZIMAGE_STREAM_SEMANTIC_ABI_MODEL_EMBEDDING_FP32, PROM_ZIMAGE_STREAM_DTYPE_FP32, 32u, 3840u, PROM_ZIMAGE_STREAM_LAYOUT_TOKEN_CHANNEL, 491520ull, PROM_ZIMAGE_STREAM_GENERATION_REPLACE_ON_CAPTURE, PROM_ZIMAGE_STREAM_LIFETIME_SESSION, PROM_ZIMAGE_STREAM_CONSUMER_MAIN_TRANSFORMER, PROM_ZIMAGE_STREAM_MUTABILITY_IMMUTABLE, PROM_ZIMAGE_STREAM_ALIAS_FORBIDDEN, PROM_ZIMAGE_STREAM_TRANSPORT_DEVICE_LOCAL},
  {PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_STREAM_JOINT_WORKING, PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO, 0u, PROM_ZIMAGE_STREAM_SEMANTIC_ABI_MODEL_EMBEDDING_FP32, PROM_ZIMAGE_STREAM_DTYPE_FP32, 1056u, 3840u, PROM_ZIMAGE_STREAM_LAYOUT_TOKEN_CHANNEL, 16220160ull, PROM_ZIMAGE_STREAM_GENERATION_REPLACE_ON_COMPOSE, PROM_ZIMAGE_STREAM_LIFETIME_SESSION, PROM_ZIMAGE_STREAM_CONSUMER_MAIN_TRANSFORMER, PROM_ZIMAGE_STREAM_MUTABILITY_IMMUTABLE, PROM_ZIMAGE_STREAM_ALIAS_FORBIDDEN, PROM_ZIMAGE_STREAM_TRANSPORT_DEVICE_LOCAL},
};
static const PrometheusMainTransformerResolvedDescriptor k_prom_zimage_turbo_main_transformer_blocks[] = {
  {PROM_ZIMAGE_TURBO_LOCK_ID, 0u, PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO, PROM_MAIN_TRANSFORMER_PARAMETER_SET_0, 0x%sull, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_STREAM_PREPARED_IMAGE, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT, PROM_ZIMAGE_STREAM_JOINT_WORKING, 1024u, 32u, 1056u, 3840u},
};
static const PrometheusNoiseRefinerResolvedDescriptor* prom_zimage_turbo_resolve_noise_refiner_descriptor(uint64_t lock_identity, uint32_t model_local_block_id) {
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || model_local_block_id > 1u) return NULL;
  return &k_prom_zimage_turbo_noise_refiner_blocks[model_local_block_id];
}
static const PrometheusContextRefinerResolvedDescriptor* prom_zimage_turbo_resolve_context_refiner_descriptor(uint64_t lock_identity, uint32_t model_local_block_id) {
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || model_local_block_id > 1u) return NULL;
  return &k_prom_zimage_turbo_context_refiner_blocks[model_local_block_id];
}
static const PrometheusCompiledModelResidentStreamDescriptor* prom_zimage_turbo_resolve_resident_stream_descriptor(uint64_t lock_identity, uint32_t role) {
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || role == 0u || role > 3u) return NULL;
  return &k_prom_zimage_turbo_resident_streams[role - 1u];
}
static const PrometheusMainTransformerResolvedDescriptor* prom_zimage_turbo_resolve_main_transformer_descriptor(uint64_t lock_identity, uint32_t model_local_block_id) {
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || model_local_block_id != 0u) return NULL;
  return &k_prom_zimage_turbo_main_transformer_blocks[0u];
}
#endif
`, identity[:16], noiseRefiner0[:16], noiseRefiner1[:16], block1Oracle[:16], contextRefiner0[:16], "a30b2ac6ff947b21", context0Oracle[:16], contextRefiner1[:16], "a30b2ac6ff947b21", context1Oracle[:16], mainTransformer0[:16]), nil
}
