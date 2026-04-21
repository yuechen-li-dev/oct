//go:build machina_desktop_webview && cgo && (linux || darwin)

package desktophost

import (
	"os"
	"testing"
)

func TestWebviewDriverFactoryConstructsAndInitializesNativeBinding(t *testing.T) {
	if os.Getenv("MACHINA_WEBVIEW_SMOKE") != "1" {
		t.Skip("set MACHINA_WEBVIEW_SMOKE=1 to run native webview smoke test")
	}

	driver, err := NewWebviewDriverFactory().New()
	if err != nil {
		t.Fatalf("construct native webview driver: %v", err)
	}
	if driver == nil {
		t.Fatal("construct native webview driver: received nil driver")
	}
	t.Cleanup(driver.Destroy)

	driver.SetTitle("Machina UI native smoke")
	driver.SetSize(320, 240)
	driver.Init(`window.__machina_webview_smoke = { ready: true };`)
}
