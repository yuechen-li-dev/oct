// zimage_main_transformer_cache derives the one closed M2C representative
// package from the pinned Z-Image Turbo checkpoint.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	manifest, inventory, err := zimage.BuildMainTransformerCache(*source, *root, *sha)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result := struct {
		Manifest  zimage.CacheManifest                  `json:"manifest"`
		Inventory []zimage.NoiseRefiner1TensorInventory `json:"inventory"`
	}{Manifest: manifest, Inventory: inventory}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	// Keep the finite/conversion inventory beside the immutable cache package so
	// later audit/report tools consume the same result that created the payload.
	if err := os.WriteFile(filepath.Join(zimage.MainTransformerCacheRoot(*root), "tensor_inventory.json"), encoded, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
