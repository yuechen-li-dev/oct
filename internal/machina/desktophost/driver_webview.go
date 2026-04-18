//go:build machina_desktop_webview && cgo && (linux || darwin)

package desktophost

import webview "github.com/webview/webview_go"

type webviewDriverFactory struct{}

// NewWebviewDriverFactory enables real native shell launch using webview_go.
func NewWebviewDriverFactory() DriverFactory {
	return webviewDriverFactory{}
}

func (webviewDriverFactory) New() (WebviewDriver, error) {
	instance := webview.New(false)
	return &webviewDriver{instance: instance}, nil
}

type webviewDriver struct {
	instance webview.WebView
}

func (d *webviewDriver) SetTitle(title string) {
	d.instance.SetTitle(title)
}

func (d *webviewDriver) SetSize(width int, height int) {
	d.instance.SetSize(width, height, webview.HintNone)
}

func (d *webviewDriver) Init(js string) {
	d.instance.Init(js)
}

func (d *webviewDriver) Navigate(targetURL string) {
	d.instance.Navigate(targetURL)
}

func (d *webviewDriver) Run() {
	d.instance.Run()
}

func (d *webviewDriver) Destroy() {
	d.instance.Destroy()
}
