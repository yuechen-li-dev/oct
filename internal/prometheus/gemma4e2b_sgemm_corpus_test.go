package prometheus

import (
	"math"
	"os"
	"testing"
)

type gemma4e2bSgemmCorpusRecord struct {
	Name             string
	M                int
	N                int
	K                int
	SelectedVariant  string
	DispatchGroupsX  int
	DispatchGroupsY  int
	WrittenCount     int
	FiniteCount      int
	ZeroCount        int
	MaxAbsoluteError float64
	MaxRelativeError float64
	RelativeL2       float64
	WorstIndex       int
	ReferenceAtWorst float32
	ActualAtWorst    float32
	DetailCode       int
}

func TestGemma4E2BM1ProductionSGEMMSmallMCorpusRTX(t *testing.T) {
	if testing.Short() || os.Getenv("OCT_RUN_PROMETHEUS_INTEGRATION") != "1" {
		t.Skip("set OCT_RUN_PROMETHEUS_INTEGRATION=1 to run the real small-M SGEMM corpus")
	}
	runtime, err := newNativeRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	type shapeCase struct {
		name    string
		m, n, k int
	}
	cases := []shapeCase{
		{"m1_n256_k1536", 1, 256, 1536},
		{"m2_n256_k1536", 2, 256, 1536},
		{"m7_n256_k1536", 7, 256, 1536},
		{"m15_n256_k1536", 15, 256, 1536},
		{"m16_n256_k1536", 16, 256, 1536},
		{"m17_n256_k1536", 17, 256, 1536},
		{"m31_n256_k1536", 31, 256, 1536},
		{"m32_n256_k1536", 32, 256, 1536},
		{"m33_n256_k1536", 33, 256, 1536},
		{"canonical_q_m15_n2048_k1536", 15, 2048, 1536},
		{"compact_exhaustive_m17_n19_k17", 17, 19, 17},
		{"alternate_m15_n256_k1536", 15, 256, 1536},
		{"alternate_m32_n256_k1536", 32, 256, 1536},
		{"alternate_m15_repeat_n256_k1536", 15, 256, 1536},
	}
	records := make([]gemma4e2bSgemmCorpusRecord, 0, len(cases))
	for _, testCase := range cases {
		a := deterministicCorpusFP32(testCase.m * testCase.k)
		b := deterministicCorpusBF16(testCase.k * testCase.n)
		reference := cpuMatmulRowMajor(a, b, testCase.m, testCase.n, testCase.k)
		actual, status, err := runtime.SGEMMWithStatus(testCase.m, testCase.n, testCase.k, a, b)
		if err != nil {
			t.Fatalf("%s: %v", testCase.name, err)
		}
		policy := compareAgainstOracle(reference, actual)
		comparison := compareVectors(actual, reference)
		finite, zeros := countFiniteAndZero(actual)
		record := gemma4e2bSgemmCorpusRecord{
			Name:             testCase.name,
			M:                testCase.m,
			N:                testCase.n,
			K:                testCase.k,
			SelectedVariant:  "baseline-scalar-package-kernel-1",
			DispatchGroupsX:  ceilDiv(testCase.m, 8),
			DispatchGroupsY:  ceilDiv(testCase.n, 8),
			WrittenCount:     len(actual),
			FiniteCount:      finite,
			ZeroCount:        zeros,
			MaxAbsoluteError: policy.MaxAbsError,
			MaxRelativeError: policy.MaxRelError,
			RelativeL2:       comparison.RelativeL2,
			WorstIndex:       comparison.WorstIndex,
			ReferenceAtWorst: comparison.Reference,
			ActualAtWorst:    comparison.Actual,
			DetailCode:       status.DetailCode,
		}
		records = append(records, record)
		if !policy.Pass || finite != len(actual) {
			t.Fatalf("%s failed production SGEMM corpus: %+v", testCase.name, record)
		}
	}
	t.Logf("small-M production SGEMM corpus: %+v", records)
}

func deterministicCorpusFP32(count int) []float32 {
	values := make([]float32, count)
	for index := range values {
		values[index] = float32((index*17)%101-50) / 31.0
	}
	return values
}

func deterministicCorpusBF16(count int) []float32 {
	values := make([]float32, count)
	for index := range values {
		value := float32((index*29)%127-63) / 37.0
		values[index] = bf16ToFloat32(float32ToBF16(value))
	}
	return values
}

func countFiniteAndZero(values []float32) (finite, zeros int) {
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			continue
		}
		finite++
		if value == 0 {
			zeros++
		}
	}
	return finite, zeros
}

func ceilDiv(value, divisor int) int {
	if divisor <= 0 {
		panic("invalid divisor")
	}
	return (value + divisor - 1) / divisor
}
