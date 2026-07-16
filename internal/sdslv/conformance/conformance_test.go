package conformance

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepositoryManifest(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate conformance package")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", ".."))
	if err := Verify(root); err != nil {
		t.Fatal(err)
	}
}
