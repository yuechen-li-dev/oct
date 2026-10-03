//go:build integration

package main

import (
	"os"
	"strings"
	"testing"
)

// The compiled lane's half of the source-failure contract. A compiled program
// runs in its own process, so its random source cannot be replaced from here.
// The generated program is instead built with the octentropyfail tag, under
// which internal/octrandom reads from a source that always fails.
func TestEntropySourceFailureIsAnOrdinaryErrorCompiled(t *testing.T) {
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -tags=octentropyfail"))
	checkEntropySourceFailure(t, "compiled")
}
