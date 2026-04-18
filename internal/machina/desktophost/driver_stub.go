//go:build !machina_desktop_webview || !cgo || (!linux && !darwin)

package desktophost

import "fmt"

type unsupportedFactory struct{}

// NewWebviewDriverFactory returns a factory placeholder when the real shell build tag is disabled.
func NewWebviewDriverFactory() DriverFactory {
	return unsupportedFactory{}
}

func (unsupportedFactory) New() (WebviewDriver, error) {
	return nil, fmt.Errorf("native webview shell is unavailable; rebuild with -tags machina_desktop_webview and cgo on linux/darwin")
}
