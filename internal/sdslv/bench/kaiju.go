package bench

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	internaloctx "github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/internal/sdslv/emit/hlsl"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

const kaijuSidecarCommand = "octxiliary-kaiju-vulkan"

func runKaiju(path string, manifest Manifest, selected []Case) (RunReport, error) {
	sidecar, err := resolveKaijuSidecar()
	if err != nil {
		return RunReport{}, err
	}
	capabilities, err := invokeKaijuCapabilities(sidecar)
	if err != nil {
		return RunReport{}, err
	}
	if !capabilities.DispatchSupported || !capabilities.BenchmarkSupported {
		return RunReport{}, fmt.Errorf("Kaiju sidecar is installed but does not advertise dispatch+benchmark support")
	}
	module, err := loadBenchmarkModule(path)
	if err != nil {
		return RunReport{}, err
	}
	root, err := os.MkdirTemp("", "sdslvbench-kaiju-")
	if err != nil {
		return RunReport{}, err
	}
	defer os.RemoveAll(root)
	report := RunReport{SchemaVersion: SchemaVersion, Source: manifest.Source}
	report.Validation.Backend = BackendKaiju
	report.BackendCapabilities.ResourceFree = true
	report.BackendCapabilities.ScalarStorageBuffers = true
	report.BackendCapabilities.VectorStorageBuffers = true
	report.BackendCapabilities.NdarrayGeneratedModules = true
	report.BackendCapabilities.TensorGeneratedModules = true
	report.BackendCapabilities.TimingSource = kaijuvulkan.TimingSourceVulkanQueryPoolGPU
	report.Device = &hostDevice{
		Name: capabilities.Device.DeviceName,
		Vendor: fmt.Sprintf("0x%04x", capabilities.Device.VendorID),
		Backend: "kaiju-vulkan",
		GodotVersion: "",
	}
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
		spirv, err := os.ReadFile(spvPath)
		if err != nil {
			return RunReport{}, err
		}
		sum := sha256.Sum256(spirv)
		hash := hex.EncodeToString(sum[:])
		request, err := benchmarkRequestForCase(c, spirv, hash)
		if err != nil {
			return RunReport{}, err
		}
		response, err := invokeKaijuBenchmark(sidecar, request)
		if err != nil {
			return RunReport{}, err
		}
		if !response.Success {
			return RunReport{}, fmt.Errorf("Kaiju benchmark %s failed: %s", c.ID, formatKaijuErrors(response.Errors))
		}
		if response.SpirvSHA256 != hash {
			return RunReport{}, fmt.Errorf("Kaiju benchmark %s returned hash %s, expected %s", c.ID, response.SpirvSHA256, hash)
		}
		if len(response.Timing.SamplesNS) != int(c.Iterations) {
			return RunReport{}, fmt.Errorf("Kaiju benchmark %s returned %d timing samples, expected %d", c.ID, len(response.Timing.SamplesNS), c.Iterations)
		}
		for _, warning := range response.Warnings {
			report.Validation.Warnings = append(report.Validation.Warnings, warning.Message)
		}
		for _, diagnostic := range response.Errors {
			report.Validation.Errors = append(report.Validation.Errors, diagnostic.Message)
		}
		report.Benchmarks = append(report.Benchmarks, Result{
			ID: c.ID,
			Name: c.Name,
			EntryPoint: c.EntryPoint,
			WorkgroupSize: c.WorkgroupSize,
			DispatchGroups: c.DispatchGroups,
			Warmup: c.Warmup,
			Iterations: c.Iterations,
			SPIRVHash: hash,
			ReplayID: c.ReplayID,
			TimingSource: response.Timing.Source,
			TimingIncludes: []string{response.Timing.StageSpan},
			Samples: StatisticsFor(response.Timing.SamplesNS),
		})
	}
	return report, nil
}

func benchmarkRequestForCase(c Case, spirv []byte, hash string) (kaijuvulkan.BenchmarkRequest, error) {
	resources := make([]kaijuvulkan.Resource, 0, len(c.Resources))
	for _, resource := range c.Resources {
		payload, err := base64.StdEncoding.DecodeString(resource.PayloadBase64)
		if err != nil {
			return kaijuvulkan.BenchmarkRequest{}, fmt.Errorf("decode resource payload binding %d: %w", resource.Binding, err)
		}
		resources = append(resources, kaijuvulkan.Resource{
			Set: resource.Set,
			Binding: resource.Binding,
			Access: resource.Access,
			Kind: kaijuvulkan.ResourceKindStorageBuffer,
			ElementType: resource.ElementType,
			ByteLength: resource.ByteLength,
			Payload: payload,
			Readback: resource.Readback,
		})
	}
	return kaijuvulkan.BenchmarkRequest{
		DispatchRequest: kaijuvulkan.DispatchRequest{
			BenchmarkID: c.ID,
			ReplayID: c.ReplayID,
			Spirv: spirv,
			SpirvSHA256: hash,
			EntryPoint: "main",
			WorkgroupSize: kaijuvulkan.UInt3{X: c.WorkgroupSize[0], Y: c.WorkgroupSize[1], Z: c.WorkgroupSize[2]},
			DispatchGroups: kaijuvulkan.UInt3{X: c.DispatchGroups[0], Y: c.DispatchGroups[1], Z: c.DispatchGroups[2]},
			PushConstants: nil,
			Resources: resources,
		},
		Warmup: c.Warmup,
		Iterations: c.Iterations,
	}, nil
}

func invokeKaijuCapabilities(sidecar string) (kaijuvulkan.Capabilities, error) {
	value, err := invokeKaiju(sidecar, kaijuvulkan.OperationCapabilities, nil)
	if err != nil {
		return kaijuvulkan.Capabilities{}, err
	}
	return kaijuvulkan.ParseCapabilitiesValue(value)
}

func invokeKaijuBenchmark(sidecar string, request kaijuvulkan.BenchmarkRequest) (kaijuvulkan.DispatchResponse, error) {
	value, err := invokeKaiju(sidecar, kaijuvulkan.OperationBenchmark, &request)
	if err != nil {
		return kaijuvulkan.DispatchResponse{}, err
	}
	return kaijuvulkan.ParseDispatchResponseValue(value)
}

func invokeKaiju(sidecar string, function string, benchmark *kaijuvulkan.BenchmarkRequest) (internaloctx.Value, error) {
	cmd := exec.Command(sidecar)
	in, err := cmd.StdinPipe()
	if err != nil {
		return internaloctx.Value{}, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return internaloctx.Value{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return internaloctx.Value{}, err
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	defer func() {
		select {
		case <-waitDone:
		case <-time.After(100 * time.Millisecond):
			killProcessTree(cmd)
			select {
			case <-waitDone:
			case <-time.After(2 * time.Second):
			}
		}
	}()
	if err := internaloctx.WriteHandshake(in); err != nil {
		return internaloctx.Value{}, err
	}
	handshakeResult := make(chan error, 1)
	go func() { handshakeResult <- internaloctx.ReadHandshake(out) }()
	select {
	case err := <-handshakeResult:
		if err != nil {
			return internaloctx.Value{}, err
		}
	case <-time.After(5 * time.Second):
		killProcessTree(cmd)
		<-waitDone
		return internaloctx.Value{}, fmt.Errorf("Kaiju sidecar timed out during handshake: %s", strings.TrimSpace(stderr.String()))
	}
	args := []internaloctx.Value{}
	if benchmark != nil {
		args = append(args, kaijuvulkan.BenchmarkRequestValue(*benchmark))
	}
	req := internaloctx.Request{ID: 1, Family: kaijuvulkan.Family, Function: function, HasArgs: true, Args: args}
	if benchmark == nil {
		req.HasArgs = false
	}
	if err := internaloctx.WriteFrame(in, internaloctx.EncodeRequest(req)); err != nil {
		return internaloctx.Value{}, err
	}
	_ = in.Close()
	frameResult := make(chan struct {
		frame string
		err   error
	}, 1)
	go func() {
		frame, err := internaloctx.ReadFrame(out)
		frameResult <- struct {
			frame string
			err   error
		}{frame: frame, err: err}
	}()
	var frame string
	select {
	case result := <-frameResult:
		if result.err != nil {
			return internaloctx.Value{}, result.err
		}
		frame = result.frame
	case <-time.After(35 * time.Second):
		killProcessTree(cmd)
		<-waitDone
		return internaloctx.Value{}, fmt.Errorf("Kaiju sidecar timed out waiting for %s response: %s", function, strings.TrimSpace(stderr.String()))
	}
	response, err := internaloctx.ParseResponse(frame)
	if err != nil {
		return internaloctx.Value{}, err
	}
	if !response.OK {
		return internaloctx.Value{}, errors.New(strings.TrimSpace(response.Error))
	}
	if !response.HasValue {
		return internaloctx.Value{}, fmt.Errorf("Kaiju sidecar returned no typed value")
	}
	select {
	case waitErr := <-waitDone:
		if waitErr != nil {
			return internaloctx.Value{}, fmt.Errorf("Kaiju sidecar failed: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
		}
	case <-time.After(2 * time.Second):
		killProcessTree(cmd)
		return internaloctx.Value{}, fmt.Errorf("Kaiju sidecar timed out")
	}
	return response.Value, nil
}

func formatKaijuErrors(values []kaijuvulkan.Diagnostic) string {
	if len(values) == 0 {
		return "unknown typed sidecar failure"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, value.MessageID+": "+value.Message)
	}
	return strings.Join(parts, "; ")
}

func resolveKaijuSidecar() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if path, ok := resolveSidecarInDir(filepath.Dir(exe), runtime.GOOS); ok {
			return path, nil
		}
	}
	if wrapperPath := os.Getenv("OCT_WRAPPER_PATH"); wrapperPath != "" {
		if path, ok := resolveSidecarFromWrapperPath(wrapperPath, runtime.GOOS); ok {
			return path, nil
		}
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 8; i++ {
			if path, ok := resolveSidecarInDir(filepath.Join(dir, "dist", "sidecars"), runtime.GOOS); ok {
				return path, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", fmt.Errorf("Kaiju sidecar %q not found; run go run ./tools/build_sidecars --kaiju --out dist/sidecars or set OCT_WRAPPER_PATH", kaijuSidecarCommand)
}

func resolveSidecarFromWrapperPath(wrapperPath string, goos string) (string, bool) {
	info, err := os.Stat(wrapperPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return resolveSidecarInDir(wrapperPath, goos)
	}
	for _, candidate := range sidecarCommandCandidates(goos) {
		if filepath.Base(wrapperPath) == candidate {
			return wrapperPath, true
		}
	}
	return "", false
}

func resolveSidecarInDir(dir string, goos string) (string, bool) {
	for _, candidate := range sidecarCommandCandidates(goos) {
		path := filepath.Join(dir, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func sidecarCommandCandidates(goos string) []string {
	if goos == "windows" {
		return []string{kaijuSidecarCommand + ".exe", kaijuSidecarCommand}
	}
	return []string{kaijuSidecarCommand}
}
