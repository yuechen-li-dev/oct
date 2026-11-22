package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateUIGolden = flag.Bool("update-ui-golden", false, "rewrite testdata/ui golden dumps")

func TestUIDumpOptionIsCheckOnly(t *testing.T) {
	opts, err := parseUIOptions([]string{"check", "app.oct", "--dump"})
	if err != nil || !opts.dump {
		t.Fatalf("opts = %+v, err = %v", opts, err)
	}
	if _, err := parseUIOptions([]string{"run", "app.oct", "--dump"}); err == nil || !strings.Contains(err.Error(), "--dump applies to `oct ui check` only") {
		t.Fatalf("run --dump err = %v", err)
	}
}

// The MachinaShell example is the UI-M2 reference: a layout table placed at
// 1024x768. The golden dump pins the span vocabulary and paint order.
func TestUICheckDumpOfTheShellExample(t *testing.T) {
	app := filepath.Join("..", "..", "Examples", "MachinaShell", "main.oct")
	var out bytes.Buffer
	opts := uiOptions{sub: "check", path: app, width: 1024, height: 768, dump: true, clicks: []string{"Add to cart"}}
	if err := runUICommand(opts, &out); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "ui", "machina_shell.dump")
	if *updateUIGolden {
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("dump changed; rerun with -update-ui-golden if intended\n got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// The MachinaText example pins wrapping and overflow under the fixed metric:
// paragraph line breaks, the note that follows them, and ellipses.
func TestUICheckFrameOfTheTextExample(t *testing.T) {
	app := filepath.Join("..", "..", "Examples", "MachinaText", "main.oct")
	var out bytes.Buffer
	opts := uiOptions{sub: "check", path: app, width: 900, height: 600, clicks: []string{"More"}}
	if err := runUICommand(opts, &out); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "ui", "machina_text.frame")
	if *updateUIGolden {
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("frame changed; rerun with -update-ui-golden if intended\n got:\n%s\nwant:\n%s", out.String(), want)
	}
}
