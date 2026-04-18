package desktophost

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultWindowTitle  = "Machina UI Desktop Host (M100b)"
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
)

// WindowConfig defines bounded native shell sizing/title options.
type WindowConfig struct {
	Title  string
	Width  int
	Height int
}

// LauncherConfig configures the bounded native webview shell launch.
type LauncherConfig struct {
	WasmArtifactPath string
	AssetsRoot       string
	Window           WindowConfig
}

// WebviewDriver isolates toolkit-specific shell operations.
type WebviewDriver interface {
	SetTitle(title string)
	SetSize(width int, height int)
	Init(js string)
	Navigate(targetURL string)
	Run()
	Destroy()
}

// DriverFactory constructs the native shell driver.
type DriverFactory interface {
	New() (WebviewDriver, error)
}

// Launch boots the real native shell around shared host runtime assets.
func Launch(factory DriverFactory, cfg LauncherConfig) error {
	if factory == nil {
		return fmt.Errorf("desktop host launch requires a driver factory")
	}
	if strings.TrimSpace(cfg.WasmArtifactPath) == "" {
		return fmt.Errorf("desktop host launch requires a wasm artifact path")
	}

	assetsRoot := strings.TrimSpace(cfg.AssetsRoot)
	if assetsRoot == "" {
		var err error
		assetsRoot, err = defaultAssetsRoot()
		if err != nil {
			return err
		}
	}

	driver, err := factory.New()
	if err != nil {
		return fmt.Errorf("desktop host launch failed creating native shell driver: %w", err)
	}
	defer driver.Destroy()

	window := normalizedWindowConfig(cfg.Window)
	driver.SetTitle(window.Title)
	driver.SetSize(window.Width, window.Height)

	bridgeInitJS, err := buildBridgeInitScript(cfg.WasmArtifactPath)
	if err != nil {
		return err
	}
	hostPageURL, err := BuildHostPageDataURL(assetsRoot)
	if err != nil {
		return err
	}

	driver.Init(bridgeInitJS)
	driver.Navigate(hostPageURL)
	driver.Run()
	return nil
}

func normalizedWindowConfig(cfg WindowConfig) WindowConfig {
	normalized := cfg
	if strings.TrimSpace(normalized.Title) == "" {
		normalized.Title = defaultWindowTitle
	}
	if normalized.Width <= 0 {
		normalized.Width = defaultWindowWidth
	}
	if normalized.Height <= 0 {
		normalized.Height = defaultWindowHeight
	}
	return normalized
}

func buildBridgeInitScript(wasmArtifactPath string) (string, error) {
	payload, err := loadWasmArtifactBase64(wasmArtifactPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`window.MachinaDesktopBridge = {
  getWasmArtifactBase64: function () { return %q; },
  onEventJSON: function (eventJSON) { console.log('[machina-ui-event]', eventJSON); }
};`, payload), nil
}

func loadWasmArtifactBase64(wasmArtifactPath string) (string, error) {
	payload, err := os.ReadFile(wasmArtifactPath)
	if err != nil {
		return "", fmt.Errorf("desktop host launch failed reading wasm artifact %q: %w", wasmArtifactPath, err)
	}
	return base64.StdEncoding.EncodeToString(payload), nil
}

// BuildHostPageDataURL creates a single-page host document embedding shared runtime scripts.
func BuildHostPageDataURL(assetsRoot string) (string, error) {
	html, err := buildHostPageHTML(assetsRoot)
	if err != nil {
		return "", err
	}
	return "data:text/html," + url.PathEscape(html), nil
}

func buildHostPageHTML(assetsRoot string) (string, error) {
	hostJS, err := os.ReadFile(filepath.Join(assetsRoot, "machina-ui-host", "host.js"))
	if err != nil {
		return "", fmt.Errorf("desktop host launch failed loading shared host runtime: %w", err)
	}
	desktopJS, err := os.ReadFile(filepath.Join(assetsRoot, "machina-ui-desktop", "desktop_host.js"))
	if err != nil {
		return "", fmt.Errorf("desktop host launch failed loading desktop adapter runtime: %w", err)
	}

	return strings.TrimSpace(fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Machina UI Desktop Host (M100b)</title>
</head>
<body>
  <main>
    <h1>Machina UI Desktop Host (M100b)</h1>
    <p>Real native webview shell around shared Machina UI host runtime.</p>
  </main>
  <div id="app"></div>
  <script>%s</script>
  <script>%s</script>
  <script>
    (async function () {
      const container = document.getElementById('app');
      await window.MachinaUIDesktopHost.bootDesktopHost({
        document,
        container,
        nativeBridge: window.MachinaDesktopBridge
      });
    })().catch(function (error) {
      const pre = document.createElement('pre');
      pre.textContent = String(error && error.stack ? error.stack : error);
      document.body.appendChild(pre);
    });
  </script>
</body>
</html>`, string(hostJS), string(desktopJS))), nil
}

func defaultAssetsRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("desktop host launch failed resolving current directory: %w", err)
	}
	for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "tools", "machina-ui-host", "host.js")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return filepath.Join(dir, "tools"), nil
		}
	}
	return "", fmt.Errorf("desktop host launch could not locate tools assets root from %q", cwd)
}
