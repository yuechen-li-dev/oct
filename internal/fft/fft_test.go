package fft

import (
	"errors"
	"strings"
	"testing"
)

type countedBackend struct {
	calls, closed int
	err           error
}

func (b *countedBackend) Transform(input []complex128) ([]complex128, error) {
	b.calls++
	if b.err != nil {
		return nil, b.err
	}
	return CPU(input), nil
}
func (b *countedBackend) Close() { b.closed++ }

func TestForcedExecutionBypassesCalibrationAndPropagatesFailure(t *testing.T) {
	backend := &countedBackend{}
	engine := &Engine{backend: backend, attempted: true}
	input := []complex128{1, 2}
	if _, err := engine.Transform(input, true); err != nil {
		t.Fatal(err)
	}
	if backend.calls != 1 || engine.Measurement().CPUWarmNS != 0 {
		t.Fatal("forced execution calibrated or did not dispatch exactly once")
	}
	backend.err = errors.New("device lost")
	output, err := engine.Transform(input, true)
	if output != nil || err == nil || !strings.Contains(err.Error(), "device lost") {
		t.Fatalf("forced failure substituted output: %v, %v", output, err)
	}
	engine.Close()
	if backend.closed != 1 {
		t.Fatal("backend owner was not closed exactly once")
	}
}

func TestAutomaticSelectionRecordsMeasuredCallsAndCachesDecision(t *testing.T) {
	backend := &countedBackend{}
	engine := &Engine{backend: backend, attempted: true}
	defer engine.Close()
	input := []complex128{1, 2, 3, 4}
	if _, err := engine.Transform(input, false); err != nil {
		t.Fatal(err)
	}
	measurement := engine.Measurement()
	if measurement.CPUWarmNS <= 0 || measurement.GPUWarmNS <= 0 || measurement.Reason == "" {
		t.Fatalf("missing full-call evidence: %+v", measurement)
	}
	calls := backend.calls
	if _, err := engine.Transform(input, false); err != nil {
		t.Fatal(err)
	}
	if backend.calls > calls+1 {
		t.Fatal("same shape recalibrated")
	}
	// Changing the native plan through forced execution invalidates the old
	// warm decision, even though the CPU cache can retain the previous roots.
	if _, err := engine.Transform([]complex128{1, 2}, true); err != nil {
		t.Fatal(err)
	}
	if engine.Measurement().Elements != 0 {
		t.Fatal("warm decision survived a native shape change")
	}
}

func TestAutomaticFailureIsInspectableAndForcedFailureRemainsAnError(t *testing.T) {
	backend := &countedBackend{err: errors.New("shaderFloat64 unavailable")}
	engine := &Engine{backend: backend, attempted: true}
	defer engine.Close()
	if output, err := engine.Transform([]complex128{1, 2}, false); err != nil || len(output) != 2 {
		t.Fatalf("automatic CPU path failed: %v", err)
	}
	if !strings.Contains(engine.Measurement().Reason, "shaderFloat64 unavailable") {
		t.Fatal("missing rejection reason")
	}
	if output, err := engine.Transform([]complex128{1, 2}, true); err == nil || output != nil {
		t.Fatal("forced call hid GPU unavailability")
	}
}

func TestInvalidInputDoesNotDispatch(t *testing.T) {
	backend := &countedBackend{}
	engine := &Engine{backend: backend, attempted: true}
	defer engine.Close()
	for _, input := range [][]complex128{nil, {1, 2, 3}} {
		if _, err := engine.Transform(input, true); err == nil {
			t.Fatal("invalid length accepted")
		}
	}
	if backend.calls != 0 {
		t.Fatal("invalid request reached native backend")
	}
}
