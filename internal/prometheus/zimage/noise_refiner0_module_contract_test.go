package zimage

import "testing"

func TestNoiseRefiner0ModuleContractIsPathIndependentAndStable(t *testing.T) {
	bundle := NoiseRefiner0PayloadBundle{
		CacheBytes:    361820672,
		CacheManifest: CacheManifest{AggregateSHA256: NoiseRefiner0CacheAggregateSHA256},
	}
	shaders := []NoiseRefiner0ShaderIdentity{
		{ID: "zimage-rope", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", PipelineID: "rope-v1"},
		{ID: "zimage-qkv", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", PipelineID: "qkv-v1"},
	}
	first, err := NewNoiseRefiner0ModuleContract(bundle, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "rtx-cooperative-or-a2x4", shaders)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewNoiseRefiner0ModuleContract(bundle, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "rtx-cooperative-or-a2x4", []NoiseRefiner0ShaderIdentity{shaders[1], shaders[0]})
	if err != nil {
		t.Fatal(err)
	}
	if first.ModelContractID != second.ModelContractID || first.ShaderPortfolioID != second.ShaderPortfolioID || first.ExecutionReplayID != second.ExecutionReplayID {
		t.Fatal("module identity changed with shader input order")
	}
}

func TestNoiseRefiner0ModuleContractRejectsIncompleteShaderIdentity(t *testing.T) {
	bundle := NoiseRefiner0PayloadBundle{
		CacheBytes:    361820672,
		CacheManifest: CacheManifest{AggregateSHA256: NoiseRefiner0CacheAggregateSHA256},
	}
	if _, err := NewNoiseRefiner0ModuleContract(bundle, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "rtx-route", []NoiseRefiner0ShaderIdentity{{ID: "missing-hash", PipelineID: "pipeline"}}); err == nil {
		t.Fatal("expected incomplete shader portfolio rejection")
	}
}
