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
	var source, out, mode string
	flag.StringVar(&source, "source", "Examples/Concept-Vulkan/kernel54_probe.concept", "canonical .concept source")
	flag.StringVar(&out, "out", "internal/prometheus/native", "output directory")
	flag.StringVar(&mode, "mode", "m1", "compiler mode: m1 or evt1-m1a")
	flag.Parse()
	if flag.NArg() != 1 || (flag.Arg(0) != "generate" && flag.Arg(0) != "check") {
		fmt.Fprintln(os.Stderr, "usage: concept-vulkan [flags] generate|check")
		os.Exit(2)
	}
	b, err := os.ReadFile(source)
	if err != nil {
		fail(err)
	}
	outputs, err := compile(mode, filepath.ToSlash(source), b)
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

func compile(mode, source string, body []byte) (conceptvulkan.Outputs, error) {
	switch mode {
	case "m1":
		p, err := conceptvulkan.Parse(source, string(body))
		if err != nil {
			return nil, err
		}
		return conceptvulkan.Generate(p, body)
	case "evt1-m1a":
		module, err := conceptvulkan.ParseEVT1(source, string(body))
		if err != nil {
			return nil, err
		}
		return conceptvulkan.GenerateEVT1(module, body)
	default:
		return nil, fmt.Errorf("unknown concept-vulkan mode %q", mode)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
