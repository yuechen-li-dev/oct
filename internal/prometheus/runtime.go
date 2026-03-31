package prometheus

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"oct/internal/interpret"
)

var errNativeUnavailable = errors.New("prometheus runtime unavailable")

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
	Correctness      CorrectnessResult
	TimingMode       string
	WallTimeNs       int64
	Notes            string
}

type CorpusReport struct {
	Runs []SGEMMRunResult
}

var StarterCorpus = []Shape{
	{M: 1, N: 1, K: 1},
	{M: 4, N: 4, K: 4},
	{M: 3, N: 5, K: 7},
	{M: 64, N: 64, K: 64},
	{M: 256, N: 64, K: 1024},
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
	if req.Backend != BackendCPU && req.Backend != BackendPrometheus {
		return SGEMMRunResult{}, fmt.Errorf("backend must be one of %q or %q", BackendCPU, BackendPrometheus)
	}
	if req.Shape.M <= 0 || req.Shape.N <= 0 || req.Shape.K <= 0 {
		return SGEMMRunResult{}, fmt.Errorf("shape dimensions must be positive")
	}
	if len(req.A) != req.Shape.M*req.Shape.K || len(req.B) != req.Shape.K*req.Shape.N {
		return SGEMMRunResult{}, fmt.Errorf("matrix dimensions do not match shape")
	}

	result := SGEMMRunResult{
		RequestedBackend: req.Backend,
		Shape:            req.Shape,
		TimingMode:       "end_to_end",
	}

	reference := cpuSGEMM(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)

	if req.Backend == BackendCPU {
		start := time.Now()
		actual := cpuSGEMM(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)
		result.WallTimeNs = time.Since(start).Nanoseconds()
		result.UsedBackend = BackendCPU
		result.Status = OkStatus()
		result.Correctness = compareAgainstOracle(reference, actual)
		if !result.Correctness.Pass {
			return result, fmt.Errorf("correctness gate failed for backend=%s", result.UsedBackend)
		}
		return result, nil
	}

	if prometheusForceSubmitFailure() {
		result.UsedBackend = BackendPrometheus
		result.Status = ErrorStatus(StageSubmit, 3)
		return result, fmt.Errorf("prometheus execution failed stage=%s code=%d", result.Status.ErrorStage, result.Status.ErrorCode)
	}

	rt, err := newNativeRuntime()
	if err != nil {
		if errors.Is(err, errNativeUnavailable) {
			start := time.Now()
			actual := cpuSGEMM(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)
			result.WallTimeNs = time.Since(start).Nanoseconds()
			result.UsedBackend = BackendCPU
			result.Status = FallbackStatus("prometheus_unavailable")
			result.Notes = "prometheus unavailable before execution; used cpu fallback"
			result.Correctness = compareAgainstOracle(reference, actual)
			if !result.Correctness.Pass {
				return result, fmt.Errorf("correctness gate failed for fallback backend=%s", result.UsedBackend)
			}
			return result, nil
		}
		result.UsedBackend = BackendPrometheus
		result.Status = ErrorStatus(StageInit, -1)
		return result, err
	}
	defer rt.Close()

	start := time.Now()
	actual, status, err := rt.SGEMM(req.Shape.M, req.Shape.N, req.Shape.K, req.A, req.B)
	result.WallTimeNs = time.Since(start).Nanoseconds()
	result.UsedBackend = BackendPrometheus
	if err != nil {
		result.Status = status
		return result, err
	}

	result.Status = OkStatus()
	result.Correctness = compareAgainstOracle(reference, actual)
	if !result.Correctness.Pass {
		return result, fmt.Errorf("correctness gate failed for backend=%s", result.UsedBackend)
	}
	return result, nil
}

func prometheusForceSubmitFailure() bool {
	return os.Getenv("PROMETHEUS_FORCE_SUBMIT_FAILURE") == "1"
}

func deterministicMatrix(rows, cols int) []float32 {
	out := make([]float32, rows*cols)
	for i := range out {
		v := ((i % 23) - 11)
		out[i] = float32(v) / 7.0
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
			FieldOrder: []string{"BackendRequested", "BackendUsed", "M", "N", "K", "Status", "CorrectnessPass", "MaxAbsError", "MaxRelError", "FailingElementCount", "FirstFailingIndex", "FirstExpectedValue", "FirstActualValue", "TimingMode", "WallTimeNs", "Notes"},
			Fields: map[string]interpret.Value{
				"BackendRequested":    {Kind: interpret.ValueString, Text: string(run.RequestedBackend)},
				"BackendUsed":         {Kind: interpret.ValueString, Text: string(run.UsedBackend)},
				"M":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.M)},
				"N":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.N)},
				"K":                   {Kind: interpret.ValueInt, Int: int64(run.Shape.K)},
				"Status":              {Kind: interpret.ValueString, Text: run.Status.String()},
				"CorrectnessPass":     {Kind: interpret.ValueBool, Bool: run.Correctness.Pass},
				"MaxAbsError":         {Kind: interpret.ValueFloat, Float: run.Correctness.MaxAbsError},
				"MaxRelError":         {Kind: interpret.ValueFloat, Float: run.Correctness.MaxRelError},
				"FailingElementCount": {Kind: interpret.ValueInt, Int: int64(run.Correctness.FailingCount)},
				"FirstFailingIndex":   {Kind: interpret.ValueInt, Int: int64(octagonFailingIndex(run.Correctness.FirstFailingIndex))},
				"FirstExpectedValue":  {Kind: interpret.ValueFloat, Float: float64(run.Correctness.FirstExpectedValue)},
				"FirstActualValue":    {Kind: interpret.ValueFloat, Float: float64(run.Correctness.FirstActualValue)},
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
