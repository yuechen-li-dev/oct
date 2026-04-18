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
	t.Cleanup(driver.Destroy)

	driver.SetTitle("Machina UI native smoke")
	driver.SetSize(320, 240)
	driver.Init(`window.__machina_webview_smoke = { ready: true };`)
	driver.Navigate("data:text/html,%3C!doctype%20html%3E%3Ctitle%3Emachina%20smoke%3C/title%3E%3Cp%3Eok%3C/p%3E")
}
