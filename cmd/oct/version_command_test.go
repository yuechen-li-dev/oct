package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestVersionCommand(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"version"}, &out, &errOut); err != nil {
		t.Fatalf("expected version command to succeed, got %v stderr=%q", err, errOut.String())
	}
	if !strings.Contains(out.String(), "oct") {
		t.Fatalf("expected version output to contain oct, got %q", out.String())
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"--version"}, &out, &errOut); err != nil {
		t.Fatalf("expected version flag to succeed, got %v stderr=%q", err, errOut.String())
	}
	if !strings.Contains(out.String(), "oct") {
		t.Fatalf("expected version output to contain oct, got %q", out.String())
	}
}
