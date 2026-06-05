package interpret

import (
	"io"
	"os/exec"
	"testing"
)

func TestInterpretedWrapperClientCloseReapsStartedProcess(t *testing.T) {
	cmd := exec.Command("sh", "-c", "cat >/dev/null")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sidecar stand-in: %v", err)
	}
	client := &interpretedWrapperClient{cmd: cmd, in: in, out: out}

	client.close()
	client.close()

	if cmd.ProcessState == nil {
		t.Fatalf("expected close to wait for sidecar process")
	}
}

func TestInterpretedWrapperClientCacheCloseIsIdempotent(t *testing.T) {
	cache := newInterpretedWrapperClientCache()
	cache.clients["already-failed"] = &interpretedWrapperClient{err: io.ErrClosedPipe}

	cache.close()
	cache.close()

	if len(cache.clients) != 0 {
		t.Fatalf("expected cache close to clear clients, got %d", len(cache.clients))
	}
}
