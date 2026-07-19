// zimage_noise_refiner1_cache builds only the pinned second noise-refiner
// package. It is intentionally not a checkpoint-wide conversion command.
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
	sha := flag.String("sha256", zimage.NoiseRefiner0SourceCheckpointSHA256, "pinned checkpoint SHA-256")
	flag.Parse()
	if *source == "" || *root == "" {
		flag.Usage()
		os.Exit(2)
	}
	result, err := zimage.BuildNoiseRefiner1Cache(*source, *root, *sha)
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
