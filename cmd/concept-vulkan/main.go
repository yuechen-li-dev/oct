// concept-vulkan generates or checks the bounded M1 kernel-54 conformance artifacts.
package main

import (
	"flag"
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/conceptvulkan"
	"os"
	"path/filepath"
)

func main() {
	var source, out string
	flag.StringVar(&source, "source", "Examples/Concept-Vulkan/kernel54_probe.concept", "canonical .concept source")
	flag.StringVar(&out, "out", "internal/prometheus/native", "output directory")
	flag.Parse()
	if flag.NArg() != 1 || (flag.Arg(0) != "generate" && flag.Arg(0) != "check") {
		fmt.Fprintln(os.Stderr, "usage: concept-vulkan [flags] generate|check")
		os.Exit(2)
	}
	b, err := os.ReadFile(source)
	if err != nil {
		fail(err)
	}
	p, err := conceptvulkan.Parse(filepath.ToSlash(source), string(b))
	if err != nil {
		fail(err)
	}
	outputs, err := conceptvulkan.Generate(p, b)
	if err != nil {
		fail(err)
	}
	if flag.Arg(0) == "generate" {
		err = conceptvulkan.Write(out, outputs)
	} else {
		err = conceptvulkan.Check(out, outputs)
	}
	if err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
