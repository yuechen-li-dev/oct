package main

import (
	"bytes"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestTopLevelHelpFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"--help"}, &out, &errOut); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("usage: oct <command> [options]")) {
		t.Fatalf("expected usage in help, got %q", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("test")) {
		t.Fatalf("expected command list in help, got %q", out.String())
	}
}

func TestTopLevelHelpCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"help"}, &out, &errOut); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("usage: oct <command> [options]")) {
		t.Fatalf("expected usage in help, got %q", out.String())
	}
}

func TestUnknownCommandSuggestsHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	err := cli.Execute([]string{"tesst"}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !bytes.Contains(errOut.Bytes(), []byte("run oct --help")) {
		t.Fatalf("expected help suggestion, got %q", errOut.String())
	}
}

func TestCommandHelp(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"test", "--help"}, []string{"--suite", "--execution"}},
		{[]string{"artifact", "--help"}, []string{"artifact"}},
		{[]string{"fmt", "--help"}, []string{"--mode", "--check"}},
		{[]string{"bench", "--help"}, []string{"--profile"}},
	}
	for _, tc := range cases {
		var out, errOut bytes.Buffer
		if err := cli.Execute(tc.args, &out, &errOut); err != nil {
			t.Fatalf("%v: expected no error, got %v", tc.args, err)
		}
		for _, w := range tc.want {
			if !bytes.Contains(out.Bytes(), []byte(w)) {
				t.Fatalf("%v: expected help to mention %q, got %q", tc.args, w, out.String())
			}
		}
	}
}
