package prometheus

import (
	"errors"
	"fmt"
	"math"
	"time"

	"oct/internal/interpret"
)

const (
	defaultAbsTolerance = 1e-4
	defaultRelTolerance = 1e-4
)

type Shape struct {
	M int
	N int
	K int
}

type SGEMMRequest struct {
	Backend Backend
	Shape   Shape
	A       []float32
	B       []float32
}

type CorrectnessResult struct {
	Pass               bool
	MaxAbsError        float64
	MaxRelError        float64
	FailingCount       int
	FirstFailingIndex  int
	FirstExpectedValue float32
	FirstActualValue   float32
}

type SGEMMRunResult struct {
	RequestedBackend Backend
	UsedBackend      Backend
	Shape            Shape
	Status           RunStatus
	DetailCode       int
	DetailName       string
	Correctness      CorrectnessResult
	CPUTimeNs        int64
	VulkanTimeNs     int64
	VulkanEnv        string
	TimingMode       string
	WallTimeNs       int64
	Notes            string
}

type CorpusReport struct {
	Runs []SGEMMRunResult
}

type AsyncValidationResult struct {
	RequestedBackend  Backend
	UsedBackend       Backend
	Shape             Shape
	Outcome           string
	Environment       string
	SubmitStage       string
	SubmitDetailCode  int
	SubmitDetailName  string
	QueryLifecycle    string
	QueryReady        bool
	QueryFailed       bool
	QueryConsumed     bool
	QueryOutstanding  int
	QueryAttempts     int
	ConsumeStage      string
	ConsumeDetailCode int
	ConsumeDetailName string
	Correctness       CorrectnessResult
	WallTimeNs        int64
	Notes             string
}

var StarterCorpus = []Shape{
	{M: 1, N: 1, K: 1},
	{M: 4, N: 4, K: 4},
	{M: 16, N: 16, K: 16},
	{M: 32, N: 32, K: 32},
	{M: 3, N: 5, K: 7},
}

func RunStarterCorpus(backend Backend) (CorpusReport, error) {
	report := CorpusReport{Runs: make([]SGEMMRunResult, 0, len(StarterCorpus))}
	for _, shape := range StarterCorpus {
		a := deterministicMatrix(shape.M, shape.K)
		b := deterministicMatrix(shape.K, shape.N)
		run, err := RunSGEMM(SGEMMRequest{Backend: backend, Shape: shape, A: a, B: b})
		report.Runs = append(report.Runs, run)
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func RunSGEMM(req SGEMMRequest) (SGEMMRunResult, error) {
	_, result, err := runSGEMMWithOutput(req)
	return result, err
}

func RunCompiledMatMulMM(left [][]float64, right [][]float64) ([][]float64, SGEMMRunResult, error) {
	req, result, err := compiledMatMulRequest(left, right)
	if err != nil {
		return nil, result, err
	}
	out, result, err := runSGEMMWithOutput(req)
	if len(out) == 0 {
		return nil, result, err
	}
	return reshapeMatrixFloat64(req.Shape.M, req.Shape.N, out), result, err
}

func runSGEMMWithOutput(req SGEMMRequest) ([]float32, SGEMMRunResult, error) {
	if req.Backend != BackendCPU && req.Backend != BackendPrometheus {
		return nil, SGEMMRunResult{}, fmt.Errorf("backend must be one of %q or %q", BackendCPU, BackendPrometheus)
	}
	if req.Shape.M <= 0 || req.Shape.N <= 0 || req.Shape.K <= 0 {
		return nil, SGEMMRunResult{}, fmt.Errorf("shape dimensions must be positive")
	}
	if len(req.A) != req.Shape.M*req.Shape.K || len(req.B) != req.Shape.K*req.Shape.N {
		return nil, SGEMMRunResult{}, fmt.Errorf("matrix dimensions do not match shape")
	}

	result := SGEMMRunResult{
		RequestedBackend: req.Backend,
		Shape:            req.Shape,
		TimingMode:       "end_to_end",
		CPUTimeNs:        0,
		VulkanTimeNs:     0,
		VulkanEnv:        "not_applicable",
		DetailCode:       0,
		DetailName:       "not_applicable",
	}

	reference, cpuDuration := timedCPUSGEMM(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)
	result.CPUTimeNs = cpuDuration.Nanoseconds()

	if req.Backend == BackendCPU {
		actual := reference
		result.WallTimeNs = result.CPUTimeNs
		result.UsedBackend = BackendCPU
		result.Status = OkStatus()
		result.DetailName = "cpu"
		result.Correctness = compareAgainstOracle(reference, actual)
		if !result.Correctness.Pass {
			return actual, result, fmt.Errorf("correctness gate failed for backend=%s", result.UsedBackend)
		}
		return actual, result, nil
	}

	rt, err := newNativeRuntime()
	if err != nil {
		if isPrometheusUnavailable(err) {
			actual := reference
			result.WallTimeNs = result.CPUTimeNs
			result.UsedBackend = BackendCPU
			result.Status = FallbackStatus("prometheus_unavailable")
			result.Notes = fmt.Sprintf("prometheus reactor unavailable; used cpu fallback: %v", err)
			result.VulkanEnv = "unavailable"
			result.DetailName = "fallback_cpu"
			result.Correctness = compareAgainstOracle(reference, actual)
			if !result.Correctness.Pass {
				return actual, result, fmt.Errorf("correctness gate failed for fallback backend=%s", result.UsedBackend)
			}
			return actual, result, nil
		}
		result.UsedBackend = BackendPrometheus
		result.Status = ErrorStatus(StageInit, bridgeIssueStatusCode(err))
		result.DetailCode = bridgeIssueStatusCode(err)
		result.DetailName = detailCodeName(result.DetailCode)
		return nil, result, err
	}
	defer rt.Close()
	result.VulkanEnv = rt.Environment()
	if result.VulkanEnv == "software_vulkan_llvmpipe_or_cpu" {
		result.Notes = "software Vulkan path (llvmpipe/CPU device): timings are not representative of hardware GPU acceleration"
	}

	start := time.Now()
	actual, callStatus, err := rt.SGEMMWithStatus(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)
	result.VulkanTimeNs = time.Since(start).Nanoseconds()
	result.WallTimeNs = result.VulkanTimeNs
	result.UsedBackend = BackendPrometheus
	result.DetailCode = callStatus.DetailCode
	result.DetailName = detailCodeName(callStatus.DetailCode)
	if err != nil {
		result.Status = ErrorStatus(stageFromNativeCode(callStatus.StageCode), callStatus.DetailCode)
		return nil, result, err
	}

	result.Status = OkStatus()
	result.Correctness = compareAgainstOracle(reference, actual)
	if !result.Correctness.Pass {
		return actual, result, fmt.Errorf("correctness gate failed for backend=%s", result.UsedBackend)
	}
	return actual, result, nil
}

func isPrometheusUnavailable(err error) bool {
	var issue *ReactorIssue
	if !errors.As(err, &issue) {
		return false
	}
	switch issue.Code {
	case ReactorIssueNotFound, ReactorIssueLoadFailed, ReactorIssueSymbolMissing, ReactorIssueABIMismatch, ReactorIssueCreateFailed, ReactorIssueProbeFailed:
		return true
	default:
		return false
	}
}

func bridgeIssueStatusCode(err error) int {
	var issue *ReactorIssue
	if !errors.As(err, &issue) {
		return -1
	}
	switch issue.Code {
	case ReactorIssueLoadFailed:
		return -10
	case ReactorIssueSymbolMissing:
		return -11
	case ReactorIssueABIMismatch:
		return -12
	case ReactorIssueCreateFailed:
		return -13
	case ReactorIssueProbeFailed:
		return -14
	default:
		return -1
	}
}

func deterministicMatrix(rows, cols int) []float32 {
	out := make([]float32, rows*cols)
	for i := range out {
		v := ((i % 23) - 11)
		out[i] = float32(v) / 7.0
	}
	return out
}

func compiledMatMulRequest(left [][]float64, right [][]float64) (SGEMMRequest, SGEMMRunResult, error) {
	result := SGEMMRunResult{
		RequestedBackend: BackendPrometheus,
		UsedBackend:      BackendPrometheus,
		Status:           ErrorStatus(StageInit, -2),
		TimingMode:       "end_to_end",
		VulkanEnv:        "not_applicable",
	}
	if len(left) == 0 || len(right) == 0 {
		return SGEMMRequest{}, result, fmt.Errorf("compiled Prometheus matmul requires non-empty matrices")
	}
	k, err := consistentColumnCount(left)
	if err != nil {
		return SGEMMRequest{}, result, err
	}
	if k == 0 {
		return SGEMMRequest{}, result, fmt.Errorf("compiled Prometheus matmul requires matrices with at least one column")
	}
	rightRows := len(right)
	n, err := consistentColumnCount(right)
	if err != nil {
		return SGEMMRequest{}, result, err
	}
	if n == 0 {
		return SGEMMRequest{}, result, fmt.Errorf("compiled Prometheus matmul requires matrices with at least one column")
	}
	if k != rightRows {
		return SGEMMRequest{}, result, fmt.Errorf("compiled Prometheus matmul shape mismatch: left columns=%d right rows=%d", k, rightRows)
	}
	return SGEMMRequest{
		Backend: BackendPrometheus,
		Shape:   Shape{M: len(left), N: n, K: k},
		A:       flattenMatrixFloat32(left),
		B:       flattenMatrixFloat32(right),
	}, result, nil
}

func consistentColumnCount(matrix [][]float64) (int, error) {
	width := len(matrix[0])
	for row := 1; row < len(matrix); row++ {
		if len(matrix[row]) != width {
			return 0, fmt.Errorf("compiled Prometheus matmul requires rectangular matrices")
		}
	}
	return width, nil
}

func flattenMatrixFloat32(matrix [][]float64) []float32 {
	out := make([]float32, 0, len(matrix)*len(matrix[0]))
	for _, row := range matrix {
		for _, value := range row {
			out = append(out, float32(value))
		}
	}
	return out
}

func reshapeMatrixFloat64(rows, cols int, values []float32) [][]float64 {
	out := make([][]float64, rows)
	for row := 0; row < rows; row++ {
		out[row] = make([]float64, cols)
		for col := 0; col < cols; col++ {
			out[row][col] = float64(values[row*cols+col])
		}
	}
	return out
}

func cpuSGEMM(m, n, k int, a, b []float32) []float32 {
	c := make([]float32, m*n)
	for row := 0; row < m; row++ {
		for col := 0; col < n; col++ {
			sum := float32(0)
			for kk := 0; kk < k; kk++ {
				sum += a[row*k+kk] * b[kk*n+col]
			}
			c[row*n+col] = sum
		}
	}
	return c
}

func timedCPUSGEMM(m, n, k int, a, b []float32) ([]float32, time.Duration) {
	start := time.Now()
	out := cpuSGEMM(m, n, k, a, b)
	return out, time.Since(start)
}

func compareAgainstOracle(expected, actual []float32) CorrectnessResult {
	res := CorrectnessResult{Pass: true, FirstFailingIndex: -1}
	for idx := range expected {
		e := expected[idx]
		a := actual[idx]
		if !isFinite(float64(e)) || !isFinite(float64(a)) {
			if res.FirstFailingIndex < 0 {
				res.FirstFailingIndex = idx
				res.FirstExpectedValue = e
				res.FirstActualValue = a
			}
			res.FailingCount++
			res.Pass = false
			continue
		}
		absErr := math.Abs(float64(e - a))
		relErr := 0.0
		if e != 0 {
			relErr = absErr / math.Abs(float64(e))
		}
		if absErr > res.MaxAbsError {
			res.MaxAbsError = absErr
		}
		if relErr > res.MaxRelError {
			res.MaxRelError = relErr
		}
		if absErr > defaultAbsTolerance && relErr > defaultRelTolerance {
			if res.FirstFailingIndex < 0 {
				res.FirstFailingIndex = idx
				res.FirstExpectedValue = e
				res.FirstActualValue = a
			}
			res.FailingCount++
			res.Pass = false
		}
	}
	return res
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func WriteOctagonReport(path string, report CorpusReport) error {
	return interpret.WriteOctagon(path, reportToOctagon(report))
}

func reportToOctagon(report CorpusReport) interpret.Value {
	runs := make([]interpret.Value, 0, len(report.Runs))
	for _, run := range report.Runs {
		runs = append(runs, interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
			TypeName:   "PrometheusSgemmRun",
			FieldOrder: []string{"BackendRequested", "BackendUsed", "M", "N", "K", "Status", "DetailCode", "DetailName", "CorrectnessPass", "MaxAbsError", "MaxRelError", "FailingElementCount", "FirstFailingIndex", "FirstExpectedValue", "FirstActualValue", "CPUTimeNs", "VulkanTimeNs", "VulkanEnv", "TimingMode", "WallTimeNs", "Notes"},
			Fields: map[string]interpret.Value{
				"BackendRequested":    {Kind: interpret.ValueString, Text: string(run.RequestedBackend)},
				"BackendUsed":         {Kind: interpret.ValueString, Text: string(run.UsedBackend)},
				"M":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.M)},
				"N":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.N)},
				"K":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.K)},
				"Status":              {Kind: interpret.ValueString, Text: run.Status.String()},
				"DetailCode":          {Kind: interpret.ValueInt, Int: int64(run.DetailCode)},
				"DetailName":          {Kind: interpret.ValueString, Text: run.DetailName},
				"CorrectnessPass":     {Kind: interpret.ValueBool, Bool: run.Correctness.Pass},
				"MaxAbsError":         {Kind: interpret.ValueFloat, Float: run.Correctness.MaxAbsError},
				"MaxRelError":         {Kind: interpret.ValueFloat, Float: run.Correctness.MaxRelError},
				"FailingElementCount": {Kind: interpret.ValueInt, Int: int64(run.Correctness.FailingCount)},
				"FirstFailingIndex":   {Kind: interpret.ValueInt, Int: int64(octagonFailingIndex(run.Correctness.FirstFailingIndex))},
				"FirstExpectedValue":  {Kind: interpret.ValueFloat, Float: float64(run.Correctness.FirstExpectedValue)},
				"FirstActualValue":    {Kind: interpret.ValueFloat, Float: float64(run.Correctness.FirstActualValue)},
				"CPUTimeNs":           {Kind: interpret.ValueInt, Int: run.CPUTimeNs},
				"VulkanTimeNs":        {Kind: interpret.ValueInt, Int: run.VulkanTimeNs},
				"VulkanEnv":           {Kind: interpret.ValueString, Text: run.VulkanEnv},
				"TimingMode":          {Kind: interpret.ValueString, Text: run.TimingMode},
				"WallTimeNs":          {Kind: interpret.ValueInt, Int: run.WallTimeNs},
				"Notes":               {Kind: interpret.ValueString, Text: run.Notes},
			},
		}})
	}

	return interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{
		TypeName:   "PrometheusSgemmReport",
		FieldOrder: []string{"Runs"},
		Fields: map[string]interpret.Value{
			"Runs": {Kind: interpret.ValueArray, Array: runs},
		},
	}}
}

func octagonFailingIndex(index int) int {
	if index < 0 {
		return 0
	}
	return index
}

func detailCodeName(code int) string {
	switch code {
	case 0:
		return "not_applicable"
	case -10:
		return "reactor_load_failed"
	case -11:
		return "reactor_symbol_missing"
	case -12:
		return "reactor_abi_mismatch"
	case -13:
		return "reactor_create_failed"
	case -14:
		return "reactor_probe_failed"
	case -6001:
		return "injected_upload_failure"
	case -6002:
		return "injected_dispatch_failure"
	case -6003:
		return "injected_download_failure"
	case -6004:
		return "size_overflow"
	case -6005:
		return "reuse_in_flight"
	case -6006:
		return "capability_mismatch"
	case 6101:
		return "direct"
	case 6102:
		return "staged_upload"
	case 6103:
		return "staged_upload_readback"
	case 6104:
		return "fallback_to_direct"
	case 6105:
		return "direct_tiled"
	case 6106:
		return "staged_upload_tiled"
	case 6107:
		return "staged_upload_readback_tiled"
	case -6108:
		return "async_not_ready"
	case -6109:
		return "async_no_task"
	case -6110:
		return "async_already_consumed"
	case -6111:
		return "async_invalid_task"
	case -6112:
		return "async_submit_rejected"
	case -6113:
		return "async_software_suppressed"
	case -6114:
		return "async_failed"
	case -6115:
		return "async_unconsumed"
	case -6116:
		return "injected_async_poll_failure"
	default:
		return fmt.Sprintf("detail_%d", code)
	}
}

func asyncLifecycleName(state uint32) string {
	switch state {
	case 0:
		return "idle"
	case 1:
		return "submitted"
	case 2:
		return "ready"
	case 3:
		return "failed"
	case 4:
		return "consumed"
	default:
		return fmt.Sprintf("state_%d", state)
	}
}

func errorStageName(stage ErrorStage) string {
	if stage == "" {
		return "unknown"
	}
	return string(stage)
}

func WriteAsyncValidationOctagon(path string, result AsyncValidationResult) error {
	return interpret.WriteOctagon(path, asyncValidationToOctagon(result))
}

func asyncValidationToOctagon(result AsyncValidationResult) interpret.Value {
	return interpret.Value{
		Kind: interpret.ValueRecord,
		Record: interpret.RecordValue{
			TypeName: "PrometheusAsyncValidation",
			FieldOrder: []string{
				"BackendRequested", "BackendUsed", "M", "N", "K", "Outcome", "Environment",
				"SubmitStage", "SubmitDetailCode", "SubmitDetailName",
				"QueryLifecycle", "QueryReady", "QueryFailed", "QueryConsumed", "QueryOutstanding", "QueryAttempts",
				"ConsumeStage", "ConsumeDetailCode", "ConsumeDetailName",
				"CorrectnessPass", "MaxAbsError", "MaxRelError", "FailingElementCount",
				"FirstFailingIndex", "FirstExpectedValue", "FirstActualValue",
				"WallTimeNs", "Notes",
			},
			Fields: map[string]interpret.Value{
				"BackendRequested":    {Kind: interpret.ValueString, Text: string(result.RequestedBackend)},
				"BackendUsed":         {Kind: interpret.ValueString, Text: string(result.UsedBackend)},
				"M":                   {Kind: interpret.ValueInt, Int: int64(result.Shape.M)},
				"N":                   {Kind: interpret.ValueInt, Int: int64(result.Shape.N)},
				"K":                   {Kind: interpret.ValueInt, Int: int64(result.Shape.K)},
				"Outcome":             {Kind: interpret.ValueString, Text: result.Outcome},
				"Environment":         {Kind: interpret.ValueString, Text: result.Environment},
				"SubmitStage":         {Kind: interpret.ValueString, Text: result.SubmitStage},
				"SubmitDetailCode":    {Kind: interpret.ValueInt, Int: int64(result.SubmitDetailCode)},
				"SubmitDetailName":    {Kind: interpret.ValueString, Text: result.SubmitDetailName},
				"QueryLifecycle":      {Kind: interpret.ValueString, Text: result.QueryLifecycle},
				"QueryReady":          {Kind: interpret.ValueBool, Bool: result.QueryReady},
				"QueryFailed":         {Kind: interpret.ValueBool, Bool: result.QueryFailed},
				"QueryConsumed":       {Kind: interpret.ValueBool, Bool: result.QueryConsumed},
				"QueryOutstanding":    {Kind: interpret.ValueInt, Int: int64(result.QueryOutstanding)},
				"QueryAttempts":       {Kind: interpret.ValueInt, Int: int64(result.QueryAttempts)},
				"ConsumeStage":        {Kind: interpret.ValueString, Text: result.ConsumeStage},
				"ConsumeDetailCode":   {Kind: interpret.ValueInt, Int: int64(result.ConsumeDetailCode)},
				"ConsumeDetailName":   {Kind: interpret.ValueString, Text: result.ConsumeDetailName},
				"CorrectnessPass":     {Kind: interpret.ValueBool, Bool: result.Correctness.Pass},
				"MaxAbsError":         {Kind: interpret.ValueFloat, Float: result.Correctness.MaxAbsError},
				"MaxRelError":         {Kind: interpret.ValueFloat, Float: result.Correctness.MaxRelError},
				"FailingElementCount": {Kind: interpret.ValueInt, Int: int64(result.Correctness.FailingCount)},
				"FirstFailingIndex":   {Kind: interpret.ValueInt, Int: int64(octagonFailingIndex(result.Correctness.FirstFailingIndex))},
				"FirstExpectedValue":  {Kind: interpret.ValueFloat, Float: float64(result.Correctness.FirstExpectedValue)},
				"FirstActualValue":    {Kind: interpret.ValueFloat, Float: float64(result.Correctness.FirstActualValue)},
				"WallTimeNs":          {Kind: interpret.ValueInt, Int: result.WallTimeNs},
				"Notes":               {Kind: interpret.ValueString, Text: result.Notes},
			},
		},
	}
}

func ValidateAsyncSGEMMOnHardware(shape Shape) (AsyncValidationResult, error) {
	result := AsyncValidationResult{
		RequestedBackend:  BackendPrometheus,
		UsedBackend:       BackendPrometheus,
		Shape:             shape,
		Outcome:           "error(init,-2)",
		Environment:       "not_applicable",
		SubmitStage:       "unknown",
		SubmitDetailName:  "not_applicable",
		QueryLifecycle:    "idle",
		ConsumeStage:      "unknown",
		ConsumeDetailName: "not_applicable",
	}

	a := deterministicMatrix(shape.M, shape.K)
	b := deterministicMatrix(shape.K, shape.N)
	reference, _ := timedCPUSGEMM(shape.M, shape.N, shape.K, a, b)

	rt, err := newNativeRuntime()
	if err != nil {
		result.UsedBackend = BackendCPU
		result.Environment = "unavailable"
		result.Outcome = "skipped(prometheus_unavailable)"
		result.Notes = err.Error()
		return result, nil
	}
	defer rt.Close()

	result.Environment = rt.Environment()
	if result.Environment == "software_vulkan_llvmpipe_or_cpu" {
		result.Outcome = "skipped(async_software_suppressed)"
		result.Notes = "software Vulkan path does not truthfully exercise async hardware validation"
		return result, nil
	}

	start := time.Now()
	taskID, submitStatus, err := rt.SubmitAsync(shape.M, shape.N, shape.K, a, b)
	result.WallTimeNs = time.Since(start).Nanoseconds()
	result.SubmitStage = errorStageName(stageFromNativeCode(submitStatus.StageCode))
	result.SubmitDetailCode = submitStatus.DetailCode
	result.SubmitDetailName = detailCodeName(submitStatus.DetailCode)
	if err != nil {
		result.Outcome = fmt.Sprintf("error(%s,%d)", result.SubmitStage, submitStatus.DetailCode)
		return result, err
	}

	var last reactorAsyncStatus
	for attempts := 1; attempts <= 2000; attempts++ {
		last, err = rt.QueryAsync(taskID)
		result.QueryAttempts = attempts
		if err != nil {
			result.Outcome = "error(query,-1)"
			return result, err
		}
		result.QueryLifecycle = asyncLifecycleName(last.LifecycleState)
		result.QueryReady = last.Ready
		result.QueryFailed = last.Failed
		result.QueryConsumed = last.Consumed
		result.QueryOutstanding = int(last.OutstandingTasks)
		if last.LifecycleState == 2 || last.LifecycleState == 3 {
			break
		}
	}

	if last.LifecycleState != 2 {
		result.Outcome = fmt.Sprintf("error(query,%d)", last.DetailCode)
		result.Notes = "async task did not reach ready state"
		return result, fmt.Errorf("async task did not reach ready state: lifecycle=%s detail=%d", result.QueryLifecycle, last.DetailCode)
	}

	out := make([]float32, shape.M*shape.N)
	consumeStatus, err := rt.ConsumeAsync(taskID, out)
	result.ConsumeStage = errorStageName(stageFromNativeCode(consumeStatus.StageCode))
	result.ConsumeDetailCode = consumeStatus.DetailCode
	result.ConsumeDetailName = detailCodeName(consumeStatus.DetailCode)
	if err != nil {
		result.Outcome = fmt.Sprintf("error(%s,%d)", result.ConsumeStage, consumeStatus.DetailCode)
		return result, err
	}

	result.Correctness = compareAgainstOracle(reference, out)
	if !result.Correctness.Pass {
		result.Outcome = "error(correctness,-1)"
		return result, fmt.Errorf("async correctness gate failed")
	}
	result.Outcome = "ok"
	return result, nil
}
