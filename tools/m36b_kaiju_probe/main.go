// m36b_kaiju_probe is a non-production evidence tool for the pre-M36b JSON
// spike. The production sidecar must use OCTWRAP/Octagon, not this envelope.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type resource struct {
	Set           uint32 `json:"set"`
	Binding       uint32 `json:"binding"`
	Access        string `json:"access"`
	Kind          string `json:"kind"`
	ElementType   string `json:"elementType"`
	ByteLength    int    `json:"byteLength"`
	PayloadBase64 string `json:"payloadBase64"`
	Readback      bool   `json:"readback"`
}
type request struct {
	SchemaVersion  int        `json:"schemaVersion"`
	Operation      string     `json:"operation"`
	SPIRVPath      string     `json:"spirvPath"`
	SPIRVHash      string     `json:"spirvHash"`
	EntryPoint     string     `json:"entryPoint"`
	WorkgroupSize  [3]uint32  `json:"workgroupSize"`
	DispatchGroups [3]uint32  `json:"dispatchGroups"`
	Resources      []resource `json:"resources"`
	Warmup         int        `json:"warmup"`
	Iterations     int        `json:"iterations"`
}

func main() {
	spv := flag.String("spv", "", "canonical SPIR-V path")
	hash := flag.String("sha256", "", "canonical SHA-256")
	spike := flag.String("spike", "out/kaiju-spike/octxiliary-kaiju-vulkan.exe", "Kaiju JSON spike executable")
	flag.Parse()
	if *spv == "" || *hash == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/m36b_kaiju_probe --spv file.spv --sha256 hash")
		os.Exit(2)
	}
	payload := base64.StdEncoding.EncodeToString(make([]byte, 16))
	r := request{1, "compute.dispatch", *spv, *hash, "main", [3]uint32{1, 1, 1}, [3]uint32{4, 1, 1}, []resource{{0, 0, "readonly", "storage_buffer", "f32", 16, payload, false}, {0, 1, "readwrite", "storage_buffer", "f32", 16, payload, true}}, 2, 8}
	dir, err := os.MkdirTemp("", "m36b-kaiju-probe-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "request.json")
	b, _ := json.Marshal(r)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		panic(err)
	}
	cmd := exec.Command(*spike, "--request", p)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
