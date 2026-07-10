//go:build toolchain

package interpret

import (
	"os/exec"
	"sync"
	"testing"
)

var (
	nodeDiscoveryOnce sync.Once
	nodeDiscoveryErr  error
)

func requireNodeTool(t *testing.T) {
	t.Helper()
	nodeDiscoveryOnce.Do(func() {
		_, nodeDiscoveryErr = exec.LookPath("node")
	})
	if nodeDiscoveryErr != nil {
		t.Skipf("node is required for the real wasm harness: %v", nodeDiscoveryErr)
	}
}
