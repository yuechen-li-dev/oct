package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuechen-li-dev/oct/internal/sdslv/bench"
)

func main() {
	source := flag.String("source", "examples/SDSL-V/M36a/BasicBenchmarks.sdslvbench", "M36a benchmark source")
	out := flag.String("out", "examples/SDSL-V/M36a/artifacts", "canonical artifact directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/generate_m36b_canonical [--source path] [--out path]")
		os.Exit(2)
	}
	m, err := bench.GenerateCanonicalArtifacts(*source, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate M36b canonical artifacts:", err)
		os.Exit(1)
	}
	for _, a := range m.Artifacts {
		fmt.Printf("%s %s %s\n", a.Name, a.BenchmarkID, a.SPIRVSHA256)
	}
}
