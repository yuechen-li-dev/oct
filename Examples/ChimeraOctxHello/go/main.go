package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

const (
	requestID = 1
	goValue   = 7
)

func main() {
	sidecar, err := sidecarPath()
	if err != nil {
		fatal(err)
	}
	cmd := exec.Command(sidecar)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatal(err)
	}

	if err := octxiliary.WriteHandshake(stdin); err != nil {
		fatal(err)
	}
	if err := octxiliary.ReadHandshake(stdout); err != nil {
		fatal(err)
	}
	req := octxiliary.Request{
		ID:       requestID,
		Family:   "ChimeraOctx",
		Function: "ChimeraHello",
		HasArgs:  true,
		Args: []octxiliary.Value{{
			Kind:       octxiliary.ValueRecord,
			RecordType: "ChimeraRequest",
			Fields: []octxiliary.FieldValue{{
				Name:  "GoValue",
				Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: goValue},
			}},
		}},
	}
	if err := octxiliary.WriteFrame(stdin, octxiliary.EncodeRequest(req)); err != nil {
		fatal(err)
	}
	body, err := octxiliary.ReadFrame(stdout)
	if err != nil {
		fatal(err)
	}
	resp, err := octxiliary.ParseResponse(body)
	if err != nil {
		fatal(err)
	}
	if !resp.OK {
		fatal(fmt.Errorf("sidecar error: %s", resp.Error))
	}
	goOut, rustOut, total, err := parseChimeraResponse(resp)
	if err != nil {
		fatal(err)
	}
	if err := stdin.Close(); err != nil {
		fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		fatal(err)
	}
	fmt.Printf("chimera octx hello: go=%d rust=%d total=%d\n", goOut, rustOut, total)
}

func sidecarPath() (string, error) {
	if len(os.Args) > 1 && os.Args[1] != "" {
		return os.Args[1], nil
	}
	if env := os.Getenv("OCT_CHIMERA_OCTX_SIDECAR"); env != "" {
		return env, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "chimera-octx-sidecar"), nil
}

func parseChimeraResponse(resp octxiliary.Response) (int, int, int, error) {
	if resp.ID != requestID || !resp.HasValue || resp.Value.Kind != octxiliary.ValueRecord || resp.Value.RecordType != "ChimeraResponse" {
		return 0, 0, 0, fmt.Errorf("unexpected ChimeraResponse envelope")
	}
	values := map[string]int{}
	for _, field := range resp.Value.Fields {
		if field.Value.Kind != octxiliary.ValueInt {
			return 0, 0, 0, fmt.Errorf("field %s is not Int", field.Name)
		}
		values[field.Name] = field.Value.Int
	}
	return values["GoValue"], values["RustValue"], values["Total"], nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "chimera octx hello: %v\n", err)
	os.Exit(1)
}
