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

func requireSlowOctxiliary(t *testing.T) {
	t.Helper()
	if os.Getenv("OCT_SLOW_TESTS") == "1" || os.Getenv("OCT_RUN_SLOW_TESTS") == "1" {
		return
	}
	t.Skip("set OCT_SLOW_TESTS=1 (or OCT_RUN_SLOW_TESTS=1) and OCT_WRAPPER_PATH to run slow Octxiliary wrapper tests")
}
