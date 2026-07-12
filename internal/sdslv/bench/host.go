package bench

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const hostSchemaVersion = 1

type hostRequest struct {
	SchemaVersion  int        `json:"schemaVersion"`
	SpirvPath      string     `json:"spirvPath"`
	SpirvHash      string     `json:"spirvHash"`
	EntryPoint     string     `json:"entryPoint"`
	BenchmarkID    string     `json:"benchmarkId"`
	ReplayID       string     `json:"replayId"`
	WorkgroupSize  [3]uint32  `json:"workgroupSize"`
	DispatchGroups [3]uint32  `json:"dispatchGroups"`
	Warmup         uint32     `json:"warmup"`
	Iterations     uint32     `json:"iterations"`
	Resources      []Resource `json:"resources"`
}
type hostDevice struct {
	Name         string `json:"Name"`
	Vendor       string `json:"Vendor"`
	GodotVersion string `json:"GodotVersion"`
	Backend      string `json:"Backend"`
}
type hostResponse struct {
	SchemaVersion  int         `json:"SchemaVersion"`
	Success        bool        `json:"Success"`
	Device         *hostDevice `json:"Device"`
	SamplesNS      []uint64    `json:"SamplesNs"`
	TimingSource   string      `json:"TimingSource"`
	TimingIncludes []string    `json:"TimingIncludes"`
	Warnings       []string    `json:"Warnings"`
	Error          *string     `json:"Error"`
}
type Result struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	EntryPoint     string     `json:"entryPoint"`
	WorkgroupSize  [3]uint32  `json:"workgroupSize"`
	DispatchGroups [3]uint32  `json:"dispatchGroups"`
	Warmup         uint32     `json:"warmup"`
	Iterations     uint32     `json:"iterations"`
	SPIRVHash      string     `json:"spirvHash"`
	ReplayID       string     `json:"replayId"`
	TimingSource   string     `json:"timingSource"`
	TimingIncludes []string   `json:"timingIncludes"`
	Samples        Statistics `json:"samplesNs"`
}
type RunReport struct {
	SchemaVersion int         `json:"schemaVersion"`
	Source        string      `json:"source"`
	Device        *hostDevice `json:"device"`
	Validation    struct {
		Backend  string   `json:"backend"`
		Warnings []string `json:"warnings"`
		Errors   []string `json:"errors"`
	} `json:"validation"`
	Benchmarks          []Result `json:"benchmarks"`
	BackendCapabilities struct {
		ResourceFree            bool   `json:"resourceFree"`
		ScalarStorageBuffers    bool   `json:"scalarStorageBuffers"`
		VectorStorageBuffers    bool   `json:"vectorStorageBuffers"`
		NdarrayGeneratedModules bool   `json:"ndarrayGeneratedModules"`
		TensorGeneratedModules  bool   `json:"tensorGeneratedModules"`
		TimingSource            string `json:"timingSource"`
	} `json:"backendCapabilities"`
}

func run(path string, manifest Manifest, selected []Case) (RunReport, error) {
	module, err := loadBenchmarkModule(path)
	if err != nil {
		return RunReport{}, err
	}
	root, err := os.MkdirTemp("", "sdslvbench-")
	if err != nil {
		return RunReport{}, err
	}
	defer os.RemoveAll(root)
	report := RunReport{SchemaVersion: SchemaVersion, Source: manifest.Source}
	report.Validation.Backend = "godot"
	report.BackendCapabilities.ResourceFree = true
	report.BackendCapabilities.ScalarStorageBuffers = true
	report.BackendCapabilities.VectorStorageBuffers = true
	report.BackendCapabilities.TimingSource = "synchronized_host_elapsed"
	for _, c := range selected {
		mir, err := lower.Module(isolatedModule(module, c))
		if err != nil {
			return RunReport{}, err
		}
		hlslText, err := hlsl.EmitBenchmark(mir, c.EntryPoint, c.WorkgroupSize)
		if err != nil {
			return RunReport{}, err
		}
		base := filepath.Join(root, c.ID)
		hlslPath := base + ".hlsl"
		spvPath := base + ".spv"
		if err = os.WriteFile(hlslPath, []byte(hlslText), 0o644); err != nil {
			return RunReport{}, err
		}
		if err = compileSPIRV(hlslPath, spvPath); err != nil {
			return RunReport{}, err
		}
		bytes, err := os.ReadFile(spvPath)
		if err != nil {
			return RunReport{}, err
		}
		sum := sha256.Sum256(bytes)
		hash := hex.EncodeToString(sum[:])
		req := hostRequest{SchemaVersion: hostSchemaVersion, SpirvPath: spvPath, SpirvHash: hash, EntryPoint: "main", BenchmarkID: c.ID, ReplayID: c.ReplayID, WorkgroupSize: c.WorkgroupSize, DispatchGroups: c.DispatchGroups, Warmup: c.Warmup, Iterations: c.Iterations, Resources: c.Resources}
		reqPath := base + ".request.json"
		responsePath := base + ".response.json"
		data, _ := json.Marshal(req)
		if err = os.WriteFile(reqPath, data, 0o644); err != nil {
			return RunReport{}, err
		}
		response, err := invokeGodot(reqPath, responsePath)
		if err != nil {
			return RunReport{}, err
		}
		if response.SchemaVersion != hostSchemaVersion || !response.Success || len(response.SamplesNS) != int(c.Iterations) {
			if response.Error != nil {
				return RunReport{}, fmt.Errorf("Godot benchmark host: %s", *response.Error)
			}
			return RunReport{}, fmt.Errorf("Godot benchmark host returned invalid response")
		}
		if report.Device == nil {
			report.Device = response.Device
		}
		report.Validation.Warnings = append(report.Validation.Warnings, response.Warnings...)
		result := Result{c.ID, c.Name, c.EntryPoint, c.WorkgroupSize, c.DispatchGroups, c.Warmup, c.Iterations, hash, c.ReplayID, response.TimingSource, response.TimingIncludes, StatisticsFor(response.SamplesNS)}
		report.Benchmarks = append(report.Benchmarks, result)
	}
	return report, nil
}

// isolatedModule is the M36a compilation boundary: independently declared
// benchmark shaders never share resource declarations or descriptor bindings.
func isolatedModule(module ast.Module, c Case) ast.Module {
	out := module
	out.Decls = make([]ast.Decl, 0, len(module.Decls))
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.ShaderDecl:
			if c.Shader != "" && d.Name == c.Shader {
				out.Decls = append(out.Decls, d)
			}
		case ast.FunctionDecl:
			if c.Shader == "" {
				if d.Name == c.EntryPoint {
					out.Decls = append(out.Decls, d)
				}
			} else {
				out.Decls = append(out.Decls, d)
			}
		default:
			out.Decls = append(out.Decls, decl)
		}
	}
	return out
}
func loadBenchmarkModule(path string) (ast.Module, error) {
	file, err := source.Load(path)
	if err != nil {
		return ast.Module{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return ast.Module{}, err
	}
	m, err := parse.BuildModule(tokens)
	if err != nil {
		return ast.Module{}, err
	}
	if err = validate.Module(m); err != nil {
		return ast.Module{}, err
	}
	return m, nil
}
func compileSPIRV(hlslPath, spvPath string) error {
	dxc, err := exec.LookPath("dxc")
	if err != nil {
		return fmt.Errorf("DXC not found: %w", err)
	}
	env := os.Getenv("SDSLV_BENCH_DXC_ENV")
	if env == "" {
		env = "vulkan1.0"
	}
	out, err := exec.Command(dxc, "-T", "cs_6_0", "-E", "main", "-spirv", "-fspv-target-env="+env, "-Fo", spvPath, hlslPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("DXC compile benchmark: %w: %s", err, out)
	}
	return nil
}
func invokeGodot(request, response string) (hostResponse, error) {
	exe, err := resolveGodot()
	if err != nil {
		return hostResponse{}, err
	}
	project := filepath.Join("tools", "sdslv_benchmark_host")
	cmd := exec.Command(exe, "--path", project, "--", "--request", request, "--response", response)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return hostResponse{}, fmt.Errorf("start Godot benchmark host: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(60 * time.Second):
		killProcessTree(cmd)
		<-done
		return hostResponse{}, fmt.Errorf("Godot benchmark host timed out after 60s")
	}
	if waitErr != nil {
		output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return hostResponse{}, fmt.Errorf("Godot benchmark host failed: %w: %s", waitErr, output)
	}
	data, err := os.ReadFile(response)
	if err != nil {
		return hostResponse{}, fmt.Errorf("Godot benchmark host did not write response: %w", err)
	}
	var result hostResponse
	if err = json.Unmarshal(data, &result); err != nil {
		return hostResponse{}, fmt.Errorf("malformed Godot benchmark response: %w", err)
	}
	return result, nil
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
		return
	}
	_ = cmd.Process.Kill()
}
func resolveGodot() (string, error) {
	if p := os.Getenv("GODOT4"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("GODOT4 does not exist: %s", p)
	}
	for _, n := range []string{"godot4", "godot"} {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Godot 4 Mono executable not found; set GODOT4 to its path")
}
