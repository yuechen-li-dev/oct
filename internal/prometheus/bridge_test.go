package prometheus

import (
	"errors"
	"testing"
)

func TestBridgeSymbolMissingIsStructured(t *testing.T) {
	_, err := runtimeFromLibrary("/tmp/reactor.so", &fakeLibrary{symbols: map[string]any{
		reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
	}})
	var issue *ReactorIssue
	if !errors.As(err, &issue) {
		t.Fatalf("expected ReactorIssue, got %v", err)
	}
	if issue.Code != ReactorIssueSymbolMissing {
		t.Fatalf("expected symbol missing, got %s", issue.Code)
	}
}

func TestBridgeABIMismatchIsStructured(t *testing.T) {
	_, err := runtimeFromLibrary("/tmp/reactor.so", &fakeLibrary{symbols: map[string]any{
		reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion + 1 }),
		reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
		reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
		reactorSymbolProbe:      reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) { return reactorCaps{Available: true}, nil }),
		reactorSymbolSGEMM: reactorSGEMM(func(reactorRuntimeHandle, int, int, int, []float32, []float32) ([]float32, reactorCallStatus, error) {
			return nil, reactorCallStatus{}, nil
		}),
	}})
	var issue *ReactorIssue
	if !errors.As(err, &issue) {
		t.Fatalf("expected ReactorIssue, got %v", err)
	}
	if issue.Code != ReactorIssueABIMismatch {
		t.Fatalf("expected abi mismatch, got %s", issue.Code)
	}
}

func TestBridgeSuccessfulLoadProbeViaStubLibrary(t *testing.T) {
	rt, err := runtimeFromLibrary("/tmp/reactor.so", &fakeLibrary{symbols: map[string]any{
		reactorSymbolABIVersion: reactorABI(func() uint32 { return reactorExpectedABIVersion }),
		reactorSymbolCreate:     reactorCreate(func(reactorCreateConfig) (reactorRuntimeHandle, error) { return reactorRuntimeHandle{}, nil }),
		reactorSymbolDestroy:    reactorDestroy(func(reactorRuntimeHandle) {}),
		reactorSymbolProbe:      reactorProbe(func(reactorRuntimeHandle) (reactorCaps, error) { return reactorCaps{Available: true}, nil }),
		reactorSymbolSGEMM: reactorSGEMM(func(_ reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
			return cpuSGEMM(m, n, k, a, b), reactorCallStatus{}, nil
		}),
	}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if rt == nil {
		t.Fatalf("expected runtime handle")
	}
	rt.Close()
}
