package prometheus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	reactorEnvVar             = "OCT_PROMETHEUS_REACTOR"
	reactorExpectedABIVersion = uint32(1)

	reactorSymbolABIVersion                  = "prometheus_reactor_abi_version"
	reactorSymbolCreate                      = "prometheus_reactor_runtime_create"
	reactorSymbolDestroy                     = "prometheus_reactor_runtime_destroy"
	reactorSymbolProbe                       = "prometheus_reactor_runtime_probe"
	reactorSymbolSGEMM                       = "prometheus_reactor_runtime_sgemm"
	reactorSymbolSubmitAsync                 = "prometheus_reactor_runtime_sgemm_submit_async"
	reactorSymbolQueryAsync                  = "prometheus_reactor_runtime_sgemm_query_async"
	reactorSymbolConsumeAsync                = "prometheus_reactor_runtime_sgemm_consume_async"
	reactorSymbolAbandonAsync                = "prometheus_reactor_runtime_sgemm_abandon_async"
	reactorSymbolGemmaInputRN                = "prometheus_reactor_runtime_gemma4e2b_m1_input_rmsnorm"
	reactorSymbolGemmaProjectionActivationRN = "prometheus_reactor_runtime_gemma4e2b_m1_projection_activation_rmsnorm"
	reactorSymbolGemmaHeadRN                 = "prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm"
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

type reactorRuntimeHandle struct {
	ptr uintptr
}
type reactorCreateConfig struct {
	TestFlags         uint32
	ShaderPackageRoot string
}
type reactorCaps struct {
	Available   bool
	BackendType uint32
	ReasonCode  uint32
}
type reactorCallStatus struct {
	StageCode  uint32
	DetailCode int
}
type reactorAsyncStatus struct {
	LifecycleState   uint32
	StageCode        uint32
	DetailCode       int
	Ready            bool
	Failed           bool
	Consumed         bool
	OutstandingTasks uint32
}

type reactorABI func() uint32
type reactorCreate func(reactorCreateConfig) (reactorRuntimeHandle, error)
type reactorDestroy func(reactorRuntimeHandle)
type reactorProbe func(reactorRuntimeHandle) (reactorCaps, error)
type reactorSGEMM func(reactorRuntimeHandle, int, int, int, []float32, []float32) ([]float32, reactorCallStatus, error)
type reactorSubmitAsync func(reactorRuntimeHandle, int, int, int, []float32, []float32) (int, reactorCallStatus, error)
type reactorQueryAsync func(reactorRuntimeHandle, int) (reactorAsyncStatus, error)
type reactorConsumeAsync func(reactorRuntimeHandle, int, []float32) (reactorCallStatus, error)
type reactorAbandonAsync func(reactorRuntimeHandle, int) error
type reactorGemma4E2BM1InputRMSNorm func(reactorRuntimeHandle, []float32, []float32, uint32, uint32, uint32, float32, uint64, uint64, uint64) ([]float32, []float32, reactorGemma4E2BM1InputRMSNormResult, error)
type reactorGemma4E2BM1HeadRMSNorm = reactorGemma4E2BM1InputRMSNorm

type reactorGemma4E2BM1InputRMSNormResult struct {
	StageCode                    uint32
	DetailCode                   int
	OutputWritten                bool
	MatchedInput                 bool
	InputHash                    uint64
	WeightHash                   uint64
	OutputHash                   uint64
	InvRMSHash                   uint64
	SubmitCount                  uint32
	FinalReadbackCount           uint32
	NoIntermediateReadbackChange bool
	RetainedBytes                uint64
	BufferAllocationCount        uint64
	BufferReuseCount             uint64
	DescriptorUpdateCount        uint64
	PipelineCreateCount          uint64
	CommandBufferReuseCount      uint64
	ReductionGPUNanoseconds      uint64
	FinalReductionGPUNanoseconds uint64
	InvRMSGPUNanoseconds         uint64
	ApplyGPUNanoseconds          uint64
	EndToEndNanoseconds          uint64
}

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
	return &prometheusBridge{loader: defaultReactorLoader()}
}

type nativeRuntime struct {
	destroy                     reactorDestroy
	sgemm                       reactorSGEMM
	submitAsync                 reactorSubmitAsync
	queryAsync                  reactorQueryAsync
	consumeAsync                reactorConsumeAsync
	abandonAsync                reactorAbandonAsync
	gemmaInputRN                reactorGemma4E2BM1InputRMSNorm
	gemmaProjectionActivationRN reactorGemma4E2BM1InputRMSNorm
	gemmaHeadRN                 reactorGemma4E2BM1HeadRMSNorm
	handle                      reactorRuntimeHandle
	caps                        reactorCaps
	lib                         dynamicLibrary
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

	handle, err := createFn(reactorCreateConfig{
		TestFlags:         reactorTestFlagsFromEnv(),
		ShaderPackageRoot: discoverShaderPackageRoot(path),
	})
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

	rt := &nativeRuntime{destroy: destroyFn, sgemm: sgemmFn, handle: handle, caps: caps, lib: lib}

	if submitAny, err := lib.Resolve(reactorSymbolSubmitAsync); err == nil {
		if submitFn, ok := submitAny.(reactorSubmitAsync); ok {
			rt.submitAsync = submitFn
		}
	}
	if queryAny, err := lib.Resolve(reactorSymbolQueryAsync); err == nil {
		if queryFn, ok := queryAny.(reactorQueryAsync); ok {
			rt.queryAsync = queryFn
		}
	}
	if consumeAny, err := lib.Resolve(reactorSymbolConsumeAsync); err == nil {
		if consumeFn, ok := consumeAny.(reactorConsumeAsync); ok {
			rt.consumeAsync = consumeFn
		}
	}
	if abandonAny, err := lib.Resolve(reactorSymbolAbandonAsync); err == nil {
		if abandonFn, ok := abandonAny.(reactorAbandonAsync); ok {
			rt.abandonAsync = abandonFn
		}
	}
	if gemmaAny, err := lib.Resolve(reactorSymbolGemmaInputRN); err == nil {
		if gemmaFn, ok := gemmaAny.(reactorGemma4E2BM1InputRMSNorm); ok {
			rt.gemmaInputRN = gemmaFn
		}
	}
	if gemmaAny, err := lib.Resolve(reactorSymbolGemmaProjectionActivationRN); err == nil {
		if gemmaFn, ok := gemmaAny.(reactorGemma4E2BM1InputRMSNorm); ok {
			rt.gemmaProjectionActivationRN = gemmaFn
		}
	}
	if gemmaAny, err := lib.Resolve(reactorSymbolGemmaHeadRN); err == nil {
		if gemmaFn, ok := gemmaAny.(reactorGemma4E2BM1HeadRMSNorm); ok {
			rt.gemmaHeadRN = gemmaFn
		}
	}

	return rt, nil
}

func (r *nativeRuntime) Close() {
	if r == nil {
		return
	}
	if r.destroy != nil {
		r.destroy(r.handle)
		r.destroy = nil
	}
	if r.lib != nil {
		_ = r.lib.Close()
		r.lib = nil
	}
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

func (r *nativeRuntime) SGEMMWithStatus(m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
	if len(a) != m*k || len(b) != k*n {
		return nil, reactorCallStatus{StageCode: 1, DetailCode: -2}, fmt.Errorf("invalid matrix lengths")
	}
	if r == nil || r.sgemm == nil {
		return nil, reactorCallStatus{StageCode: 1, DetailCode: -3}, &ReactorIssue{Code: ReactorIssueSGEMMFailed, Err: errors.New("runtime sgemm entrypoint unavailable")}
	}
	out, nativeStatus, err := r.sgemm(r.handle, m, n, k, a, b)
	if err != nil {
		return nil, nativeStatus, &ReactorIssue{Code: ReactorIssueSGEMMFailed, Err: err}
	}
	return out, nativeStatus, nil
}

func (r *nativeRuntime) SubmitAsync(m, n, k int, a, b []float32) (int, reactorCallStatus, error) {
	if r == nil || r.submitAsync == nil {
		return 0, reactorCallStatus{StageCode: 1, DetailCode: -3}, fmt.Errorf("reactor async submit entrypoint unavailable")
	}
	return r.submitAsync(r.handle, m, n, k, a, b)
}

func (r *nativeRuntime) QueryAsync(taskID int) (reactorAsyncStatus, error) {
	if r == nil || r.queryAsync == nil {
		return reactorAsyncStatus{}, fmt.Errorf("reactor async query entrypoint unavailable")
	}
	return r.queryAsync(r.handle, taskID)
}

func (r *nativeRuntime) ConsumeAsync(taskID int, out []float32) (reactorCallStatus, error) {
	if r == nil || r.consumeAsync == nil {
		return reactorCallStatus{StageCode: 1, DetailCode: -3}, fmt.Errorf("reactor async consume entrypoint unavailable")
	}
	return r.consumeAsync(r.handle, taskID, out)
}

func (r *nativeRuntime) AbandonAsync(taskID int) error {
	if r == nil || r.abandonAsync == nil {
		return fmt.Errorf("reactor async abandon entrypoint unavailable")
	}
	return r.abandonAsync(r.handle, taskID)
}

func (r *nativeRuntime) Gemma4E2BM1InputRMSNorm(input, weight []float32, tokens, modelWidth, inputRowStride uint32, epsilon float32, inputGeneration, weightGeneration, exactSourceHash uint64) ([]float32, []float32, reactorGemma4E2BM1InputRMSNormResult, error) {
	if r == nil || r.gemmaInputRN == nil {
		return nil, nil, reactorGemma4E2BM1InputRMSNormResult{StageCode: 1, DetailCode: -3}, fmt.Errorf("gemma4e2b m1 input rmsnorm entrypoint unavailable")
	}
	return r.gemmaInputRN(r.handle, input, weight, tokens, modelWidth, inputRowStride, epsilon, inputGeneration, weightGeneration, exactSourceHash)
}

// Gemma4E2BM1ProjectionActivationRMSNorm preserves the model-private BF16
// storage boundary between layer RMSNorm and every Q/K/V projection.
func (r *nativeRuntime) Gemma4E2BM1ProjectionActivationRMSNorm(input, weight []float32, tokens, modelWidth, inputRowStride uint32, epsilon float32, inputGeneration, weightGeneration, exactSourceHash uint64) ([]float32, []float32, reactorGemma4E2BM1InputRMSNormResult, error) {
	if r == nil || r.gemmaProjectionActivationRN == nil {
		return nil, nil, reactorGemma4E2BM1InputRMSNormResult{StageCode: 1, DetailCode: -3}, fmt.Errorf("gemma4e2b m1 projection activation rmsnorm entrypoint unavailable")
	}
	return r.gemmaProjectionActivationRN(r.handle, input, weight, tokens, modelWidth, inputRowStride, epsilon, inputGeneration, weightGeneration, exactSourceHash)
}

// Gemma4E2BM1HeadRMSNorm admits flattened [token, head, channel] rows only.
// The model-private boundary prevents an accidental layer-width substitution.
func (r *nativeRuntime) Gemma4E2BM1HeadRMSNorm(input, weight []float32, rows, headWidth, inputRowStride uint32, epsilon float32, inputGeneration, weightGeneration, exactSourceHash uint64) ([]float32, []float32, reactorGemma4E2BM1InputRMSNormResult, error) {
	if r == nil || r.gemmaHeadRN == nil {
		return nil, nil, reactorGemma4E2BM1InputRMSNormResult{StageCode: 1, DetailCode: -3}, fmt.Errorf("gemma4e2b m1 head rmsnorm entrypoint unavailable")
	}
	return r.gemmaHeadRN(r.handle, input, weight, rows, headWidth, inputRowStride, epsilon, inputGeneration, weightGeneration, exactSourceHash)
}

func (r *nativeRuntime) Environment() string {
	if r == nil {
		return "unknown"
	}
	switch r.caps.BackendType {
	case 2:
		if runtime.GOOS == "windows" {
			return "windows_native_vulkan"
		}
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
		return candidates
	}
	add(filepath.Join("internal", "prometheus", "reactor", reactorLibraryBasename()))
	add(filepath.Join("out", "prometheus", "native", reactorLibraryBasename()))
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), reactorLibraryBasename()))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		pkgDir := filepath.Dir(file)
		add(filepath.Join(pkgDir, "reactor", reactorLibraryBasename()))
		add(filepath.Join(pkgDir, "..", "..", "out", "prometheus", "native", reactorLibraryBasename()))
	}
	return candidates
}

func reactorTestFlagsFromEnv() uint32 {
	value := os.Getenv("OCT_PROMETHEUS_REACTOR_TEST_FLAGS")
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}

func reactorLibraryBasename() string {
	if runtime.GOOS == "windows" {
		return "prometheus_reactor.dll"
	}
	return "libprometheus_reactor.so"
}

func discoverShaderPackageRoot(reactorPath string) string {
	if reactorPath == "" {
		return ""
	}
	base := filepath.Dir(reactorPath)
	candidates := []string{
		filepath.Join(base, "shaders"),
		filepath.Join(base, "SerialCanonical", "shaders"),
		filepath.Join(base, "..", "SerialCanonical", "shaders"),
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		pkgDir := filepath.Dir(file)
		candidates = append(candidates,
			filepath.Join(pkgDir, "..", "..", "out", "prometheus", "native", "shaders"),
			filepath.Join(pkgDir, "..", "..", "out", "prometheus", "native", "SerialCanonical", "shaders"),
		)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
			return filepath.Clean(candidate)
		}
	}
	return ""
}
