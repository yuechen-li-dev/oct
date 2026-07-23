package prometheus

import (
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
	if !result.RMSNorm.HashMatch {
		t.Fatalf("RMSNorm hash drifted: got %s want %s", result.RMSNorm.ActualQuantized, result.RMSNorm.ReferenceHash)
	}
	if !result.ProjectionActivationBF16.Pass {
		t.Fatalf("projection activation BF16 operand witness diverges: %+v", result.ProjectionActivationBF16)
	}
	t.Logf("oneDNN-enabled diagnostic captures against strict portable reference: Q=%+v K=%+v V=%+v", result.QActivationBF16CPU, result.KActivationBF16CPU, result.VActivationBF16CPU)
	t.Logf("portable Vulkan projections: Q=%+v K=%+v V=%+v", result.QPortableProjection, result.KPortableProjection, result.VPortableProjection)
	t.Logf("portable Vulkan Q/K normalization: Q=%+v K=%+v", result.QNormalizedPortable, result.KNormalizedPortable)
	for _, projection := range []struct {
		name   string
		policy gemma4e2bPortableProjectionComparison
	}{
		{"Q", result.QPortableProjection},
		{"K", result.KPortableProjection},
		{"V", result.VPortableProjection},
	} {
		if !projection.policy.WithinPortablePolicy {
			t.Fatalf("%s projection exceeds the portable BF16 contract: %+v", projection.name, projection.policy)
		}
	}
	for _, projection := range []struct {
		name     string
		policy   CorrectnessResult
		repeated bool
	}{
		{"Q", result.QCPUContractPolicy, result.QRepeatedStable},
		{"K", result.KCPUContractPolicy, result.KRepeatedStable},
		{"V", result.VCPUContractPolicy, result.VRepeatedStable},
	} {
		if !projection.policy.Pass {
			t.Fatalf("%s projection violates the established numerical policy: %+v", projection.name, projection.policy)
		}
		if !projection.repeated {
			t.Fatalf("%s projection changed across identical repeated dispatches", projection.name)
		}
	}
	for _, normalization := range []struct {
		name   string
		policy gemma4e2bPortableProjectionComparison
		stable bool
		native reactorGemma4E2BM1InputRMSNormResult
	}{
		{"Q", result.QNormalizedPortable, result.QNormalizedStable, result.QNormalizedNative},
		{"K", result.KNormalizedPortable, result.KNormalizedStable, result.KNormalizedNative},
	} {
		if !normalization.policy.WithinBF16StagePolicy {
			t.Fatalf("%s head normalization violates the established numerical policy: %+v", normalization.name, normalization.policy)
		}
		if !normalization.stable || !normalization.native.OutputWritten || !normalization.native.MatchedInput {
			t.Fatalf("%s head normalization was not fully written and stable: %+v", normalization.name, normalization.native)
		}
	}
}
