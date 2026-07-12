// Package bench owns .sdslvbench discovery and presentation. It does not use
// the .sdslvtest result ABI or Prometheus' production shader registry.
package bench

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const SchemaVersion = 1

type Case struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	EntryPoint     string      `json:"entryPoint"`
	WorkgroupSize  [3]uint32   `json:"workgroupSize"`
	DispatchGroups [3]uint32   `json:"dispatchGroups"`
	Warmup         uint32      `json:"warmup"`
	Iterations     uint32      `json:"iterations"`
	SourceSpan     source.Span `json:"sourceSpan"`
	ReplayID       string      `json:"replayId"`
	Resources      []Resource  `json:"resources"`
	Shader         string      `json:"shader,omitempty"`
}
type Resource struct {
	Set            uint32 `json:"set"`
	Binding        uint32 `json:"binding"`
	Access         string `json:"access"`
	ElementType    string `json:"elementType"`
	ByteLength     uint32 `json:"byteLength"`
	PayloadBase64  string `json:"payloadBase64"`
	Readback       bool   `json:"readback"`
	SentinelBase64 string `json:"sentinelBase64,omitempty"`
}
type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Source        string `json:"source"`
	Benchmarks    []Case `json:"benchmarks"`
}
type Options struct {
	List   bool
	CaseID string
	JSON   bool
}

func Discover(path string) (Manifest, error) {
	if filepath.Ext(path) != ".sdslvbench" {
		return Manifest{}, fmt.Errorf("sdslv bench expects a .sdslvbench file")
	}
	file, err := source.Load(path)
	if err != nil {
		return Manifest{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return Manifest{}, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return Manifest{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}
	identity := sourceIdentity(abs)
	values, issues := validate.ValidatedBenchmarks(module, identity)
	if len(issues) != 0 {
		return Manifest{}, diagnostic.Error(issues)
	}
	m := Manifest{SchemaVersion: SchemaVersion, Source: identity}
	for _, v := range values {
		c := Case{ID: v.StableID, Name: v.Name, Shader: v.Shader, EntryPoint: v.Name, WorkgroupSize: v.Launch.WorkgroupSize, DispatchGroups: v.Launch.DispatchGroups, Warmup: v.WarmupCount, Iterations: v.IterationCount, SourceSpan: v.SourceSpan, Resources: projectResources(v.Resources, v.Launch.WorkgroupSize, v.Launch.DispatchGroups)}
		sum := sha256.Sum256([]byte(c.ID + "\x00" + c.EntryPoint))
		c.ReplayID = "sdslvbench-replay-" + hex.EncodeToString(sum[:12])
		m.Benchmarks = append(m.Benchmarks, c)
	}
	sort.Slice(m.Benchmarks, func(i, j int) bool { return m.Benchmarks[i].ID < m.Benchmarks[j].ID })
	return m, nil
}
func projectResources(values []ast.ResourceDecl, workgroup, groups [3]uint32) []Resource {
	out := make([]Resource, 0, len(values))
	n := uint64(workgroup[0]) * uint64(workgroup[1]) * uint64(workgroup[2]) * uint64(groups[0]) * uint64(groups[1]) * uint64(groups[2])
	for i, r := range values {
		if r.Type.Name != "array" || len(r.Type.Args) != 1 {
			continue
		}
		kind := r.Type.Args[0].Name
		width := uint64(4)
		if kind == "float2" {
			width = 8
		}
		if kind == "float4" {
			width = 16
		}
		size := n * width
		if size == 0 || size > 1<<24 {
			continue
		}
		payload := make([]byte, size)
		out = append(out, Resource{Set: 0, Binding: resourceBinding(r, uint32(i)), Access: r.Access, ElementType: kind, ByteLength: uint32(size), PayloadBase64: base64.StdEncoding.EncodeToString(payload), Readback: r.Access == "readwrite"})
	}
	return out
}
func resourceBinding(r ast.ResourceDecl, fallback uint32) uint32 {
	for _, a := range r.Attributes {
		if a.Name == "binding" && len(a.Arguments) == 1 {
			if n, err := strconv.ParseUint(strings.TrimRight(a.Arguments[0].(ast.IntegerLiteral).Value, "uU"), 10, 32); err == nil {
				return uint32(n)
			}
		}
	}
	return fallback
}
func Execute(path string, out io.Writer, o Options) error {
	m, err := Discover(path)
	if err != nil {
		return err
	}
	selected := make([]Case, 0, len(m.Benchmarks))
	for _, c := range m.Benchmarks {
		if o.CaseID == "" || o.CaseID == c.ID {
			selected = append(selected, c)
		}
	}
	if o.CaseID != "" && len(selected) == 0 {
		return fmt.Errorf("no .sdslvbench case with stable id %q", o.CaseID)
	}
	if o.List {
		for _, c := range selected {
			fmt.Fprintf(out, "%s %s dispatch=%dx%dx%d workgroup=%dx%dx%d warmup=%d iterations=%d\n", c.ID, c.Name, c.DispatchGroups[0], c.DispatchGroups[1], c.DispatchGroups[2], c.WorkgroupSize[0], c.WorkgroupSize[1], c.WorkgroupSize[2], c.Warmup, c.Iterations)
		}
		return nil
	}
	report, err := run(path, m, selected)
	if err != nil {
		return err
	}
	if o.JSON {
		b, _ := json.MarshalIndent(report, "", "  ")
		_, err = fmt.Fprintln(out, string(b))
		return err
	}
	for _, r := range report.Benchmarks {
		fmt.Fprintf(out, "%s\nID: %s\nGPU: %s\nDispatch: %dx%dx%d groups\nWarmup: %d\nSamples: %d\nMin: %d ns\nMedian: %d ns\nMax: %d ns\nTiming: %s\nReplay: %s\n", r.Name, r.ID, report.Device.Name, r.DispatchGroups[0], r.DispatchGroups[1], r.DispatchGroups[2], r.Warmup, r.Samples.Count, r.Samples.Min, r.Samples.Median, r.Samples.Max, r.TimingSource, r.ReplayID)
	}
	return nil
}
func sourceIdentity(abs string) string {
	wd, err := os.Getwd()
	if err == nil {
		if rel, e := filepath.Rel(wd, abs); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}
