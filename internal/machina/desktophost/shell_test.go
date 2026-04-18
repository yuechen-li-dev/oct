package desktophost

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHostPageDataURLIncludesSharedRuntimes(t *testing.T) {
	assetsRoot := makeAssetsRootFixture(t)

	dataURL, err := BuildHostPageDataURL(assetsRoot)
	if err != nil {
		t.Fatalf("BuildHostPageDataURL failed: %v", err)
	}

	if !strings.HasPrefix(dataURL, "data:text/html,") {
		t.Fatalf("expected html data url prefix, got %q", dataURL)
	}
	if !strings.Contains(dataURL, "MachinaUIHost") {
		t.Fatalf("expected embedded shared host runtime script")
	}
	if !strings.Contains(dataURL, "MachinaUIDesktopHost") {
		t.Fatalf("expected embedded desktop adapter runtime script")
	}
}

func TestLaunchWiresNativeShellAroundSharedRuntime(t *testing.T) {
	assetsRoot := makeAssetsRootFixture(t)
	wasmPath := filepath.Join(t.TempDir(), "counter.ui.wasm")
	wasmBytes := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	if err := os.WriteFile(wasmPath, wasmBytes, 0o644); err != nil {
		t.Fatalf("write wasm fixture: %v", err)
	}

	factory := &fakeFactory{driver: &fakeDriver{}}
	if err := Launch(factory, LauncherConfig{
		WasmArtifactPath: wasmPath,
		AssetsRoot:       assetsRoot,
	}); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	driver := factory.driver
	if !driver.runCalled {
		t.Fatalf("expected native shell run to be invoked")
	}
	if !driver.destroyCalled {
		t.Fatalf("expected native shell destroy to be invoked")
	}
	if driver.title == "" {
		t.Fatalf("expected native shell title to be set")
	}
	if driver.width <= 0 || driver.height <= 0 {
		t.Fatalf("expected native shell size to be set, got %dx%d", driver.width, driver.height)
	}
	if !strings.Contains(driver.initJS, "MachinaDesktopBridge") {
		t.Fatalf("expected shell init script to install desktop bridge")
	}
	encoded := base64.StdEncoding.EncodeToString(wasmBytes)
	if !strings.Contains(driver.initJS, encoded) {
		t.Fatalf("expected shell init script to contain wasm bytes bridge payload")
	}
	if !strings.Contains(driver.navigateURL, "data:text/html,") {
		t.Fatalf("expected shell to navigate to embedded host page")
	}
	if !strings.Contains(driver.navigateURL, "MachinaUIDesktopHost") {
		t.Fatalf("expected embedded host page to boot shared desktop host runtime")
	}
}

func makeAssetsRootFixture(t *testing.T) string {
	t.Helper()
	assetsRoot := t.TempDir()
	hostDir := filepath.Join(assetsRoot, "machina-ui-host")
	desktopDir := filepath.Join(assetsRoot, "machina-ui-desktop")
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		t.Fatalf("mkdir host fixture: %v", err)
	}
	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		t.Fatalf("mkdir desktop fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "host.js"), []byte("window.MachinaUIHost = {ready:true};"), 0o644); err != nil {
		t.Fatalf("write host.js fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(desktopDir, "desktop_host.js"), []byte("window.MachinaUIDesktopHost = {bootDesktopHost: async ()=>({})};"), 0o644); err != nil {
		t.Fatalf("write desktop_host.js fixture: %v", err)
	}
	return assetsRoot
}

type fakeFactory struct {
	driver *fakeDriver
}

func (f *fakeFactory) New() (WebviewDriver, error) {
	return f.driver, nil
}

type fakeDriver struct {
	title         string
	width         int
	height        int
	initJS        string
	navigateURL   string
	runCalled     bool
	destroyCalled bool
}

func (f *fakeDriver) SetTitle(title string) {
	f.title = title
}

func (f *fakeDriver) SetSize(width int, height int) {
	f.width = width
	f.height = height
}

func (f *fakeDriver) Init(js string) {
	f.initJS = js
}

func (f *fakeDriver) Navigate(targetURL string) {
	f.navigateURL = targetURL
}

func (f *fakeDriver) Run() {
	f.runCalled = true
}

func (f *fakeDriver) Destroy() {
	f.destroyCalled = true
}
