// zimage_context_refiner_cache derives one closed ContextRefiner package.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

func main() {
	source := flag.String("source", "", "pinned Z-Image-Turbo safetensors checkpoint")
	root := flag.String("cache-root", "", "EVT-2 cache root")
	block := flag.String("block", "", "context_refiner.0 or context_refiner.1")
	sha := flag.String("sha256", zimage.NoiseRefiner0SourceCheckpointSHA256, "pinned checkpoint SHA-256")
	flag.Parse()
	if *source == "" || *root == "" || *block == "" {
		flag.Usage()
		os.Exit(2)
	}
	result, err := zimage.BuildContextRefinerCache(*source, *root, *sha, *block)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.Marshal(result.Manifest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
