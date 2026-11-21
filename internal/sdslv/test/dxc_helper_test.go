package test

import (
	"errors"
	"testing"
)

// requireCompiled fails on a compile error, except that a missing dxc skips
// the test: the shader toolchain is optional on developer machines and CI
// images without it. A dxc that is present but fails still fails the test.
func requireCompiled(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrDXCNotFound) {
		t.Skipf("dxc not installed; skipping shader compilation: %v", err)
	}
	t.Fatal(err)
}
