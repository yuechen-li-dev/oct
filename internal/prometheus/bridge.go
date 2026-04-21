package prometheus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	reactorEnvVar             = "OCT_PROMETHEUS_REACTOR"
	reactorExpectedABIVersion = uint32(1)

	reactorSymbolABIVersion = "prometheus_reactor_abi_version"
	reactorSymbolCreate     = "prometheus_reactor_runtime_create"
	reactorSymbolDestroy    = "prometheus_reactor_runtime_destroy"
	reactorSymbolProbe      = "prometheus_reactor_runtime_probe"
	reactorSymbolSGEMM      = "prometheus_reactor_runtime_sgemm"
)

type ReactorIssueCode string

const (
	ReactorIssueNotFound      ReactorIssueCode = "reactor_not_found"
	ReactorIssueLoadFailed    ReactorIssueCode = "reactor_load_failed"
	ReactorIssueSymbolMissing ReactorIssueCode = "symbol_missing"
	ReactorIssueABIMismatch   ReactorIssueCode = "abi_mismatch"
	ReactorIssueCreateFailed  ReactorIssueCode = "runtime_create_failed"
	ReactorIssueProbeFailed   ReactorIssueCode = "runtime_probe_unavailable"
	ReactorIssueSGEMMFailed   ReactorIssueCode = "runtime_sgemm_failed"
)

type ReactorIssue struct {
	Code        ReactorIssueCode
	Path        string
	Symbol      string
	ExpectedABI uint32
	ActualABI   uint32
	Err         error
}

func (e *ReactorIssue) Error() string {
	switch e.Code {
	case ReactorIssueNotFound:
		return "prometheus reactor not found"
	case ReactorIssueSymbolMissing:
		return fmt.Sprintf("prometheus reactor missing symbol %q", e.Symbol)
	case ReactorIssueABIMismatch:
		return fmt.Sprintf("prometheus reactor ABI mismatch expected=%d actual=%d", e.ExpectedABI, e.ActualABI)
	case ReactorIssueCreateFailed:
		return "prometheus reactor runtime creation failed"
	case ReactorIssueProbeFailed:
		return "prometheus reactor runtime probe failed"
	case ReactorIssueSGEMMFailed:
		return "prometheus reactor runtime sgemm failed"
	default:
		return "prometheus reactor load failed"
	}
}

func (e *ReactorIssue) Unwrap() error { return e.Err }

type reactorRuntimeHandle struct{}
type reactorCaps struct {
	Available   bool
	BackendType uint32
	ReasonCode  uint32
}
type reactorCallStatus struct {
	StageCode  uint32
	DetailCode int
}

type reactorABI func() uint32
type reactorCreate func() (reactorRuntimeHandle, error)
type reactorDestroy func(reactorRuntimeHandle)
type reactorProbe func(reactorRuntimeHandle) (reactorCaps, error)
type reactorSGEMM func(reactorRuntimeHandle, int, int, int, []float32, []float32) ([]float32, reactorCallStatus, error)

type dynamicLibrary interface {
	Resolve(symbol string) (any, error)
	Close() error
}

type reactorLoader interface {
	Open(path string) (dynamicLibrary, error)
}

type unavailableLoader struct{}

func (unavailableLoader) Open(path string) (dynamicLibrary, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return nil, errors.New("dynamic loading backend not enabled in this build")
}

type prometheusBridge struct {
	loader reactorLoader
}

var newPrometheusBridge = func() *prometheusBridge {
	return &prometheusBridge{loader: unavailableLoader{}}
}

type nativeRuntime struct {
	destroy reactorDestroy
	sgemm   reactorSGEMM
	handle  reactorRuntimeHandle
	caps    reactorCaps
}

func newNativeRuntime() (*nativeRuntime, error) {
	return newPrometheusBridge().openRuntime()
}

func (b *prometheusBridge) openRuntime() (*nativeRuntime, error) {
	if b == nil || b.loader == nil {
		return nil, &ReactorIssue{Code: ReactorIssueLoadFailed, Err: errors.New("bridge loader unavailable")}
	}

	candidatePaths := discoverReactorCandidates()
	if len(candidatePaths) == 0 {
		return nil, &ReactorIssue{Code: ReactorIssueNotFound}
	}

	var lastErr error
	for _, path := range candidatePaths {
		lib, err := b.loader.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			lastErr = &ReactorIssue{Code: ReactorIssueLoadFailed, Path: path, Err: err}
			continue
		}
		rt, rtErr := runtimeFromLibrary(path, lib)
		if rtErr != nil {
			_ = lib.Close()
			return nil, rtErr
		}
		return rt, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &ReactorIssue{Code: ReactorIssueNotFound}
}

func runtimeFromLibrary(path string, lib dynamicLibrary) (*nativeRuntime, error) {
	abiAny, err := lib.Resolve(reactorSymbolABIVersion)
	if err != nil {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolABIVersion, Err: err}
	}
	abiFn, ok := abiAny.(reactorABI)
	if !ok {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolABIVersion, Err: fmt.Errorf("symbol has unexpected type")}
	}
	actual := abiFn()
	if actual != reactorExpectedABIVersion {
		return nil, &ReactorIssue{Code: ReactorIssueABIMismatch, Path: path, ExpectedABI: reactorExpectedABIVersion, ActualABI: actual}
	}

	createAny, err := lib.Resolve(reactorSymbolCreate)
	if err != nil {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolCreate, Err: err}
	}
	createFn, ok := createAny.(reactorCreate)
	if !ok {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolCreate, Err: fmt.Errorf("symbol has unexpected type")}
	}
	destroyAny, err := lib.Resolve(reactorSymbolDestroy)
	if err != nil {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolDestroy, Err: err}
	}
	destroyFn, ok := destroyAny.(reactorDestroy)
	if !ok {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolDestroy, Err: fmt.Errorf("symbol has unexpected type")}
	}
	probeAny, err := lib.Resolve(reactorSymbolProbe)
	if err != nil {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolProbe, Err: err}
	}
	probeFn, ok := probeAny.(reactorProbe)
	if !ok {
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolProbe, Err: fmt.Errorf("symbol has unexpected type")}
	}

	handle, err := createFn()
	if err != nil {
		return nil, &ReactorIssue{Code: ReactorIssueCreateFailed, Path: path, Err: err}
	}
	caps, err := probeFn(handle)
	if err != nil || !caps.Available {
		destroyFn(handle)
		return nil, &ReactorIssue{Code: ReactorIssueProbeFailed, Path: path, Err: err}
	}

	sgemmAny, err := lib.Resolve(reactorSymbolSGEMM)
	if err != nil {
		destroyFn(handle)
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolSGEMM, Err: err}
	}
	sgemmFn, ok := sgemmAny.(reactorSGEMM)
	if !ok {
		destroyFn(handle)
		return nil, &ReactorIssue{Code: ReactorIssueSymbolMissing, Path: path, Symbol: reactorSymbolSGEMM, Err: fmt.Errorf("symbol has unexpected type")}
	}

	return &nativeRuntime{destroy: destroyFn, sgemm: sgemmFn, handle: handle, caps: caps}, nil
}

func (r *nativeRuntime) Close() {
	if r == nil || r.destroy == nil {
		return
	}
	r.destroy(r.handle)
	r.destroy = nil
}

func (r *nativeRuntime) SGEMM(m, n, k int, a, b []float32) ([]float32, RunStatus, error) {
	if len(a) != m*k || len(b) != k*n {
		return nil, ErrorStatus(StageInit, -2), fmt.Errorf("invalid matrix lengths")
	}
	if r == nil || r.sgemm == nil {
		return nil, ErrorStatus(StageInit, -3), &ReactorIssue{Code: ReactorIssueSGEMMFailed, Err: errors.New("runtime sgemm entrypoint unavailable")}
	}
	out, nativeStatus, err := r.sgemm(r.handle, m, n, k, a, b)
	if err != nil {
		return nil, ErrorStatus(stageFromNativeCode(nativeStatus.StageCode), nativeStatus.DetailCode), &ReactorIssue{Code: ReactorIssueSGEMMFailed, Err: err}
	}
	return out, OkStatus(), nil
}

func (r *nativeRuntime) Environment() string {
	if r == nil {
		return "unknown"
	}
	switch r.caps.BackendType {
	case 2:
		return "hardware_vulkan"
	case 3:
		return "software_vulkan_llvmpipe_or_cpu"
	default:
		return "unknown"
	}
}

func stageFromNativeCode(stage uint32) ErrorStage {
	switch stage {
	case 1:
		return StageInit
	case 2:
		return StageTransferIn
	case 3:
		return StageSubmit
	case 4:
		return StageTransferOut
	case 5:
		return StageCleanup
	default:
		return StageUnknown
	}
}

func discoverReactorCandidates() []string {
	candidates := []string{}
	seen := map[string]struct{}{}
	add := func(path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		candidates = append(candidates, clean)
	}

	if explicit := os.Getenv(reactorEnvVar); explicit != "" {
		add(explicit)
	}
	add(filepath.Join("internal", "prometheus", "reactor", reactorLibraryBasename()))
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), reactorLibraryBasename()))
	}
	return candidates
}

func reactorLibraryBasename() string {
	// P4a currently targets Unix-like development hosts; platform expansion remains future work.
	return "libprometheus_reactor.so"
}
