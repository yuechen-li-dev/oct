// evt2_m2d_cache_manifest projects the locally validated closed
// MainTransformer packages into a deterministic, payload-free inventory.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

type entry struct {
	Block     string `json:"block"`
	Aggregate string `json:"aggregate_sha256"`
	Bytes     uint64 `json:"package_bytes"`
	Tensors   int    `json:"tensor_count"`
}

func main() {
	cacheRoot := flag.String("cache-root", "", "EVT-2 cache root")
	out := flag.String("out", "internal/prometheus/DevelopmentReport/artifacts/Evt2M2d/m2d_cache_manifest.json", "payload-free manifest path")
	flag.Parse()
	if *cacheRoot == "" {
		flag.Usage()
		os.Exit(2)
	}
	entries := make([]entry, 0, 30)
	for layer := uint32(0); layer < 30; layer++ {
		path := filepath.Join(zimage.MainTransformerLayerCacheRoot(*cacheRoot, layer), "manifest.json")
		data, err := os.ReadFile(path)
		if err != nil {
			panic(fmt.Errorf("read layers.%d manifest: %w", layer, err))
		}
		var manifest zimage.CacheManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			panic(fmt.Errorf("parse layers.%d manifest: %w", layer, err))
		}
		block, err := zimage.MainTransformerLayerBlock(layer)
		if err != nil || manifest.Block != block || len(manifest.Tensors) != 13 || manifest.AggregateSHA256 == "" {
			panic(fmt.Errorf("invalid closed package %q", block))
		}
		var bytes uint64
		for _, tensor := range manifest.Tensors {
			bytes += tensor.Bytes
		}
		entries = append(entries, entry{block, manifest.AggregateSHA256, bytes, len(manifest.Tensors)})
	}
	payload := struct {
		Schema   string  `json:"schema"`
		Packages []entry `json:"packages"`
	}{"oct.prometheus.evt2.m2d.cache-manifest.v1", entries}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		panic(err)
	}
}
