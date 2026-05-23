package main

import (
	"os"
	"testing"
)

func skipUnlessSlow(t *testing.T) {
	t.Helper()
	if os.Getenv("OCT_RUN_SLOW_TESTS") != "1" {
		t.Skip("set OCT_RUN_SLOW_TESTS=1 to run slow integration/science tests")
	}
}
