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
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/octagon"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const (
	noiseRefiner0   = "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e"
	noiseRefiner1   = "80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8"
	block1Oracle    = "9b133c9ed3772f782e1bd77ff5b89732dc28406eec2078f1692d1899e2eb39e7"
	contextRefiner0 = "c08b908a921a80e16995abc3f3eefcadd1a94a78cd76b56e58d6b21e6ce412ae"
	contextRefiner1 = "30268c3b0d7a6fafc411119c929326854e73d394ec95ebeba21f89aa43dfc95f"
	context0Oracle  = "d2b8167de614da25211eb69991e1b7700992bf5f4ae527bff8a55366ea1ae6df"
	context1Oracle  = "08377e8a46b65cff998b740a3fd0ba3c1565b471dad71fcca3f4310617c220b0"
)

var mainTransformerAggregates = []string{
	"48e987811885741ae5f1bf16b28db33ca7f23e09f1e99c1c2fe3d81bdd1caeb6", "7ec04d7363bb1ac00f2aa491c2b463bc065efe380f3a1a3116c6eb61b0eec827", "41b9b7985035fd206dfdbfc3f1bf416b8ff1e8e25cc38b69bb249a2209d09c05", "3ba944c7dd3bc7873fa67cb541586ed2ff08aa2d2bd9a69a14ab8bef47b2156f", "c83fa21d1740885924c221c91233faa19a43c5460dc4a25ef6b1aa48ad99ff8c", "a643f7836a056d34574526ce0bc8aa8b58508cee6e4ef08ed4432a0b896c7715", "6ee8d2ce1947fb66ef5e712ddfebae4a60ff1cfe8df9be21dc123d3fd6720bb2", "c00357c21617e910363e6d12b11a1ed962fbdab64e4f558f587318490baea8b0", "fc1f557f758953683d64dfb4f5d923eca1d313242279d477f2df301b1b125ba3", "f3d59ef2a2f18d24a31b8b572bce0cd33a7f239a622d5651cc2337070af7984d",
	"7e79e59c29ee081b698f9c0b31b66ebcc4ab2039acd1f0aea6761ff1e3eab55c", "3fef07f8f482453ed5a3296f085ceaafebf14bfe6ab5f64ea2b5293c231676da", "a2067c05a569f2d5f201024974192ebea9d0b6162790700e05c267eefaf56344", "d4bcc84602cd9e77c40b2eb1d3882035ad5fe5a46cbebd91ff2d8a2403f56001", "ebc24a91474ac4d55fc797cd12c828da755c28bf367116562c41d7ee8c6d1051", "a42866e4b7a055f8c435c6116705b26481a0e38469c07c3e526d07bdbf7554d1", "db0272f70aa50aa9d012bbdc33cfef518d5e6ea2c8245b1602f24c837db27e40", "f6aa44dcc9f599458a9272d6a2ee666030f642f63f732ef43c8637a3f164a6d0", "f921c7aa36817102fbdbf83f379e3d1272098544a9711ea713a4f220d2b56ea8", "96a9d4e83e0468807d2bb305440eda6ed52c241d4c0f3928787afb72c2b7ad02",
	"c161dd253cfe99d75c8cf91eddfff2d92016eb3a7631772acef1711680043448", "9638d4a676d163aca60e7b63cb60fc36a7d3c8c1a05bb98191c24e641d491f8e", "0e4070f7fda57719ef4996fe4694b7a4c595985a07947eb7b38ec0fbc6d0de4d", "76813a3cc248610801489f1ee74782e0b6086447b677b55beb7b044eedc81f0b", "d64f21d7e6b801009f0de5e1868be3b9bdf23be8ebfb75a2a124d6f0adacff09", "e616bea34a39ed99a8afd576cf3ef3d75f3e7ed06c47e7cc1b60d7ae4acabfc8", "f177c2e5179c53c43122d6f349f21ad57247633053d358be9e9d9f1eb3cada39", "75700608176f43409da6e469a886ba6f0efdb62d49a6cddb0f3020968e21a9d5", "7750f6f142eeb066847ff454aa657fe7be18ebdbd2db7ec347e8c2d8802199bc", "db7de286ef1f507a181ef760c39953d17943e845361b5afe5510f79202eb87a1",
}

func mainTransformerLockBlocks() string {
	var b strings.Builder
	for index, aggregate := range mainTransformerAggregates {
		predecessor, successor := "PreparedImage|PreparedContext", "MainTransformer1"
		if index > 0 {
			predecessor = "MainTransformer" + strconv.Itoa(index-1)
		}
		if index == len(mainTransformerAggregates)-1 {
			successor = "MainTransformerOutput"
		}
		separator := ""
		if index+1 < len(mainTransformerAggregates) {
			separator = ","
		}
		fmt.Fprintf(&b, "        ResolvedBlock { Family: \"ZImageTurbo.MainTransformer\" Id: %d Name: \"MainTransformer%d\" ParameterSet: \"MainTransformer%d\" Parameters: \"layers.%d\" Cache: \"sha256:%s\" Oracle: \"M2D.JointFp32.layers.%d\" Predecessor: \"%s\" Successor: \"%s\" Transport: \"JointWorking.FP32 ImageThenContext resident\" }%s\n", index, index, index, index, aggregate, index, predecessor, successor, separator)
	}
	return b.String()
}

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
		"ContextEmbedding -> ContextRefiner0 -> ContextRefiner1", "Parameters: \"layers.0\"", "Parameters: \"layers.29\"", "Assembly: \"MainTransformer\"",
		"sha256:" + mainTransformerAggregates[0], "sha256:" + mainTransformerAggregates[29], "Role: \"PreparedImage\"", "Role: \"PreparedContext\"", "Role: \"JointWorking\"",
		"TokenChannel.ImageThenContext", "serialized resident FP32 execution",
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
	semanticID := digest([]byte("ZImageTurbo|f332072aa78be7aecdf3ee76d5c247082da564a6|NoiseRefiner+ContextRefiner+MainTransformer0-29|PreparedImage+PreparedContext+JointWorking|ImageThenContext|ZImageReferenceFp32"))
	productionID := digest([]byte("NoiseRefiner ids 24-36; ContextRefiner ids 25,26,31-37,38,39; MainTransformer ids 40-43|closed session physical ImageThenContext composition|resident 30-layer execution"))
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
    ExecutionPlan: "NoiseRefiner: AdaLN -> attention -> FFN; ContextRefiner: RMSNorm -> attention -> FFN; session captures both final FP32 streams; physical ImageThenContext composition; serialized resident MainTransformer0-29 chain"
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
%s
    ]
    ResidentStreams: [
        ResidentStream { Role: "PreparedImage" Producer: "NoiseRefiner1" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 1024 Width: 3840 Layout: "TokenChannel" Bytes: 15728640 Generation: "replace-on-capture" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" },
        ResidentStream { Role: "PreparedContext" Producer: "ContextRefiner1" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 32 Width: 3840 Layout: "TokenChannel" Bytes: 491520 Generation: "replace-on-capture" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" },
        ResidentStream { Role: "JointWorking" Producer: "M2CPhysicalComposition" SemanticAbi: "ModelEmbedding.FP32" Dtype: "FP32" Tokens: 1056 Width: 3840 Layout: "TokenChannel.ImageThenContext" Bytes: 16220160 Generation: "replace-on-compose" Lifetime: "session" Consumers: "MainTransformer" Mutability: "immutable" Alias: "forbidden" Transport: "device-local" }
    ]
    Flow: "NoiseRefiner0 -> NoiseRefiner1 -> PreparedImage and ContextEmbedding -> ContextRefiner0 -> ContextRefiner1 -> PreparedContext; Joint = Image | Context; serialized session execution; atomic rebind; no BF16 cast; no host bounce"
    Replay: "lock + assembly-family + parameter-set + binding-generation + PreparedImage-generation + PreparedContext-generation + joint-composition"
}
`, manifestID, semanticID, productionID, auditID, noiseRefiner0, noiseRefiner1, block1Oracle, contextRefiner0, context0Oracle, contextRefiner1, context1Oracle, mainTransformerLockBlocks())
}

// nativeProjection is intentionally derived from the immutable lock document,
// not the authored manifest. Native code gets a closed const descriptor table;
// it never reparses a model name or independently reconstructs topology.
func nativeProjection(lock []byte) (string, error) {
	for _, required := range []string{"AssemblyFamily: \"ZImageTurbo.ClosedFamilies\"", "Family: \"ZImageTurbo.NoiseRefiner\"", "Family: \"ZImageTurbo.ContextRefiner\"", "Family: \"ZImageTurbo.MainTransformer\"", "sha256:" + noiseRefiner0, "sha256:" + noiseRefiner1, "sha256:" + contextRefiner0, "sha256:" + contextRefiner1, "sha256:" + mainTransformerAggregates[0], "sha256:" + mainTransformerAggregates[29], "Role: \"PreparedImage\"", "Role: \"PreparedContext\"", "Role: \"JointWorking\"", "TokenChannel.ImageThenContext"} {
		if !strings.Contains(string(lock), required) {
			return "", fmt.Errorf("lock missing %q", required)
		}
	}
	identity := digest(lock)
	var mainTable strings.Builder
	for index, aggregate := range mainTransformerAggregates {
		fmt.Fprintf(&mainTable, "  {PROM_ZIMAGE_TURBO_LOCK_ID, %du, PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO, %du, 0x%sull, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_TURBO_LOCK_ID, PROM_ZIMAGE_STREAM_PREPARED_IMAGE, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT, PROM_ZIMAGE_STREAM_JOINT_WORKING, 1024u, 32u, 1056u, 3840u},\n", index, index+1, aggregate[:16])
	}
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
%s
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
  if (lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID || model_local_block_id >= 30u) return NULL;
  return &k_prom_zimage_turbo_main_transformer_blocks[model_local_block_id];
}
#endif
`, identity[:16], noiseRefiner0[:16], noiseRefiner1[:16], block1Oracle[:16], contextRefiner0[:16], "a30b2ac6ff947b21", context0Oracle[:16], contextRefiner1[:16], "a30b2ac6ff947b21", context1Oracle[:16], mainTable.String()), nil
}
