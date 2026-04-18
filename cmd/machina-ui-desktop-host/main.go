package main

import (
	"flag"
	"fmt"
	"os"

	"oct/internal/machina/desktophost"
)

func main() {
	var cfg desktophost.LauncherConfig
	flag.StringVar(&cfg.WasmArtifactPath, "wasm", "", "path to emitted .ui.wasm artifact")
	flag.StringVar(&cfg.AssetsRoot, "assets-root", "", "path to tools asset root containing machina-ui-host and machina-ui-desktop")
	flag.StringVar(&cfg.Window.Title, "title", "", "window title")
	flag.IntVar(&cfg.Window.Width, "width", 0, "window width")
	flag.IntVar(&cfg.Window.Height, "height", 0, "window height")
	flag.Parse()

	if err := desktophost.Launch(desktophost.NewWebviewDriverFactory(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "machina-ui-desktop-host: %v\n", err)
		os.Exit(1)
	}
}
