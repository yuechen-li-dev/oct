package main

import (
	"fmt"
	"os"

	"github.com/yuechen-li-dev/oct/internal/build"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./Experiments/PerfM4 <source.oct> <generated.go>")
		os.Exit(2)
	}
	source, err := build.EmitGoSource(os.Args[1], build.GoSourceOptions{PackageName: "specialized"})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], source, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
