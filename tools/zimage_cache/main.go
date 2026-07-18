package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
	"os"
)

func main() {
	source := flag.String("source", "", "checkpoint")
	root := flag.String("cache-root", "", "cache root")
	sha := flag.String("sha256", "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6", "source hash")
	flag.Parse()
	if *source == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "-source and -cache-root are required")
		os.Exit(2)
	}
	m, e := zimage.BuildNoiseRefiner0Cache(*source, *root, *sha)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	b, _ := json.Marshal(m)
	fmt.Println(string(b))
}
