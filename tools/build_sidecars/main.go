package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var sidecars = []string{
	"octxiliary-io",
	"octxiliary-csv",
	"octxiliary-json",
	"octxiliary-image",
	"octxiliary-pdf",
	"octxiliary-plot",
	"octxiliary-xlsx",
	"octxiliary-hash",
	"octxiliary-text",
	"octxiliary-time",
	"octxiliary-archive",
	"octxiliary-compression",
	"octxiliary-makehost",
}

func main() {
	outDir := flag.String("out", filepath.Join("dist", "sidecars"), "directory for built octxiliary sidecars")
	kaiju := flag.Bool("kaiju", false, "build optional Kaiju Vulkan sidecar from its pinned checkout")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "usage: go run ./tools/build_sidecars [--out dist/sidecars]\n")
		os.Exit(2)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create sidecar output directory %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	for _, sidecar := range sidecars {
		outPath := filepath.Join(*outDir, binaryName(sidecar))
		cmd := exec.Command("go", "build", "-o", outPath, "./cmd/"+sidecar)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("building %s -> %s\n", sidecar, outPath)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n", sidecar, err)
			os.Exit(1)
		}
	}
	if *kaiju {
		if err := buildKaiju(*outDir); err != nil {
			fmt.Fprintf(os.Stderr, "build octxiliary-kaiju-vulkan: %v\n", err)
			os.Exit(1)
		}
	}

	absOut, err := filepath.Abs(*outDir)
	if err != nil {
		absOut = *outDir
	}
	fmt.Printf("built %d sidecars\n", len(sidecars))
	fmt.Printf("set OCT_WRAPPER_PATH=%s for compiled/auto wrapper tests\n", absOut)
}

func buildKaiju(outDir string) error {
	root := filepath.Join("out", "kaiju-audit")
	check := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	b, err := check.Output()
	if err != nil {
		return fmt.Errorf("pinned Kaiju checkout required at %s: %w", root, err)
	}
	if strings.TrimSpace(string(b)) != "ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2" {
		return fmt.Errorf("Kaiju checkout has unexpected commit %s", strings.TrimSpace(string(b)))
	}
	cmd := exec.Command("go", "build", "-C", "Sidecars/KaijuVulkan", "-o", filepath.Join("..", "..", outDir, binaryName("octxiliary-kaiju-vulkan")), ".")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func binaryName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}
