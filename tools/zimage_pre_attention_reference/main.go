package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

func main() {
	cacheRoot := flag.String("cache-root", "", "EVT-2 root containing layers")
	oracleRoot := flag.String("oracle-root", "", "revision directory containing run_02 and m075")
	out := flag.String("out", "", "local M1B canonical stage directory")
	flag.Parse()
	if *cacheRoot == "" || *oracleRoot == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	result, err := zimage.RunCanonicalNoiseRefiner0PreAttention(zimage.CanonicalNoiseRefiner0Paths{
		CacheRoot: *cacheRoot, OracleRoot: *oracleRoot,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	names := make([]string, 0, len(result.Stages))
	for name := range result.Stages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		values := result.Stages[name]
		data := make([]byte, len(values)*4)
		for index, value := range values {
			binary.LittleEndian.PutUint32(data[index*4:], math.Float32bits(value))
		}
		if err := os.WriteFile(filepath.Join(*out, name+".f32.bin"), data, 0o644); err != nil {
			panic(err)
		}
	}
	fmt.Printf("wrote %d canonical M1B stages to %s\n", len(names), *out)
}
