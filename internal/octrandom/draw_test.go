package octrandom

import (
	"errors"
	"math"
	"testing"
)

func mustUnit(t *testing.T, k Key, i int64) float64 {
	t.Helper()
	v, err := Unit(k, i)
	if err != nil {
		t.Fatalf("Unit(%#x, %d): %v", uint64(k), i, err)
	}
	return v
}

func mustBetween(t *testing.T, k Key, i int64, lo, hi float64) float64 {
	t.Helper()
	v, err := Between(k, i, lo, hi)
	if err != nil {
		t.Fatalf("Between(%#x, %d, %v, %v): %v", uint64(k), i, lo, hi, err)
	}
	return v
}

func mustIntBetween(t *testing.T, k Key, i int64, lo, hi int64) int64 {
	t.Helper()
	v, err := IntBetween(k, i, lo, hi)
	if err != nil {
		t.Fatalf("IntBetween(%#x, %d, %d, %d): %v", uint64(k), i, lo, hi, err)
	}
	return v
}

func mustNormal(t *testing.T, k Key, i int64, mean, stddev float64) float64 {
	t.Helper()
	v, err := Normal(k, i, mean, stddev)
	if err != nil {
		t.Fatalf("Normal(%#x, %d, %v, %v): %v", uint64(k), i, mean, stddev, err)
	}
	return v
}

// ---- Unit ----

func TestUnitFromWordBounds(t *testing.T) {
	if got := unitFromWord(0); got != 0 {
		t.Errorf("unitFromWord(0) = %v, want 0", got)
	}
	top := unitFromWord(math.MaxUint64)
	if want := 1 - math.Ldexp(1, -53); top != want {
		t.Errorf("unitFromWord(max) = %v, want 1 - 2^-53", top)
	}
	if top >= 1 {
		t.Errorf("unitFromWord(max) = %v, must stay below 1", top)
	}
	// The low 11 bits are discarded.
	if unitFromWord(0x7ff) != 0 {
		t.Error("low 11 bits of the word must not affect Unit")
	}
	if got, want := unitFromWord(1<<11), math.Ldexp(1, -53); got != want {
		t.Errorf("unitFromWord(1<<11) = %v, want 2^-53", got)
	}
}

func TestUnitMatchesSpecification(t *testing.T) {
	for _, k := range layoutKeys {
		for _, i := range layoutIndices {
			w0, _ := philoxWords(uint64(k), uint32(uint64(i)), uint32(uint64(i)>>32), 0, 0)
			want := float64(w0>>11) * math.Ldexp(1, -53)
			if got := mustUnit(t, k, i); got != want {
				t.Errorf("Unit(%#x, %d) = %v, want %v", uint64(k), i, got, want)
			}
		}
	}
}

func TestUnitRangePurityAndMean(t *testing.T) {
	k := Fork(Seeded(2026), "unit")
	const n = 200000
	sum := 0.0
	for i := int64(0); i < n; i++ {
		v := mustUnit(t, k, i)
		if v < 0 || v >= 1 {
			t.Fatalf("Unit at %d = %v, outside [0, 1)", i, v)
		}
		sum += v
	}
	if mean := sum / n; mean < 0.495 || mean > 0.505 {
		t.Errorf("mean of %d Unit draws = %v, want within 0.005 of 0.5", n, mean)
	}
	// Purity: any order, any repetition.
	for _, i := range []int64{9, 0, 9, 123456, 0} {
		if mustUnit(t, k, i) != mustUnit(t, k, i) {
			t.Fatalf("Unit(%d) is not repeatable", i)
		}
	}
}

// ---- Between ----

func TestBetweenMatchesSpecification(t *testing.T) {
	k := Seeded(11)
	for i := int64(0); i < 1000; i++ {
		u := mustUnit(t, k, i)
		lo, hi := -3.5, 12.25
		want := lo + float64((hi-lo)*u)
		if got := mustBetween(t, k, i, lo, hi); got != want {
			t.Fatalf("Between at %d = %v, want %v", i, got, want)
		}
	}
}

func TestBetweenStaysHalfOpen(t *testing.T) {
	k := Seeded(3)
	// An interval one ulp wide: lo + span*u rounds up to hi for about half of
	// all draws, so the upper-bound clamp is exercised constantly.
	lo := 1.0
	hi := math.Nextafter(1, 2)
	for i := int64(0); i < 2000; i++ {
		if got := mustBetween(t, k, i, lo, hi); got != lo {
			t.Fatalf("Between(%v, %v) at %d = %v, want lo", lo, hi, i, got)
		}
	}
	ranges := [][2]float64{{0, 1}, {-1, 1}, {1e-300, 2e-300}, {-1e300, 1e300}, {1e15, 1e15 + 2}, {0, 5e-324}}
	for _, r := range ranges {
		for i := int64(0); i < 5000; i++ {
			got := mustBetween(t, k, i, r[0], r[1])
			if got < r[0] || got >= r[1] {
				t.Fatalf("Between(%v, %v) at %d = %v, outside [lo, hi)", r[0], r[1], i, got)
			}
		}
	}
}

func TestBetweenFromUnitClampsAtUpperBound(t *testing.T) {
	// Force the worst case directly: the largest Unit value on an interval
	// where lo + span*u rounds to hi.
	lo, hi := 1.0, math.Nextafter(1, 2)
	u := 1 - math.Ldexp(1, -53)
	if got := betweenFromUnit(u, lo, hi-lo, hi); got != lo {
		t.Errorf("betweenFromUnit clamp = %v, want %v", got, lo)
	}
	if got := betweenFromUnit(0, -2, 5, 3); got != -2 {
		t.Errorf("betweenFromUnit(0) = %v, want lo", got)
	}
}

func TestBetweenDegenerateAndInvalidRanges(t *testing.T) {
	k := Seeded(1)
	if got := mustBetween(t, k, 0, 4.5, 4.5); got != 4.5 {
		t.Errorf("Between(lo == hi) = %v, want 4.5", got)
	}
	inf, nan := math.Inf(1), math.NaN()
	cases := []struct {
		name   string
		i      int64
		lo, hi float64
		want   error
	}{
		{"negative index", -1, 0, 1, ErrNegativeIndex},
		{"lo above hi", 0, 2, 1, ErrFloatRange},
		{"infinite hi", 0, 0, inf, ErrFloatSpan},
		{"infinite lo", 0, -inf, 0, ErrFloatSpan},
		{"NaN lo", 0, nan, 1, ErrFloatSpan},
		{"NaN hi", 0, 0, nan, ErrFloatSpan},
		{"span overflows", 0, -math.MaxFloat64, math.MaxFloat64, ErrFloatSpan},
	}
	for _, c := range cases {
		if _, err := Between(k, c.i, c.lo, c.hi); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
		}
	}
}

// ---- IntBetween ----

// referenceIntBetween restates the specification independently of the
// implementation: it walks the words in order and accepts the first one below
// 2^64 - (2^64 mod span), computing the remainder and the bound a different
// way from IntBetween.
func referenceIntBetween(k Key, i int64, lo, hi int64) (value int64, wordsUsed int) {
	span := uint64(hi) - uint64(lo) + 1
	if span == 0 {
		w0, _ := philoxWords(uint64(k), uint32(uint64(i)), uint32(uint64(i)>>32), 0, 0)
		return int64(w0), 1
	}
	// 2^64 mod span, computed as ((2^64 - 1) mod span + 1) mod span.
	rem := (math.MaxUint64%span + 1) % span
	for block := uint32(0); ; block++ {
		w0, w1 := philoxWords(uint64(k), uint32(uint64(i)), uint32(uint64(i)>>32), block, 0)
		for _, w := range []uint64{w0, w1} {
			wordsUsed++
			if w <= math.MaxUint64-rem {
				return int64(uint64(lo) + w%span), wordsUsed
			}
		}
	}
}

func TestIntBetweenMatchesSpecification(t *testing.T) {
	ranges := [][2]int64{
		{1, 6}, {0, 0}, {-5, 5}, {1, 100}, {0, 1},
		{math.MinInt64, math.MaxInt64},     // span wraps to 0
		{math.MinInt64, 0},                 // span 2^63 + 1: about half of all words rejected
		{0, math.MaxInt64},                 // span 2^63: nothing rejected
		{math.MinInt64 + 1, math.MaxInt64}, // span 2^64 - 1
		{math.MaxInt64 - 2, math.MaxInt64},
		{math.MinInt64, math.MinInt64 + 2},
	}
	k := Fork(Seeded(77), "ints")
	for _, r := range ranges {
		for i := int64(0); i < 3000; i++ {
			want, _ := referenceIntBetween(k, i, r[0], r[1])
			got := mustIntBetween(t, k, i, r[0], r[1])
			if got != want {
				t.Fatalf("IntBetween(%d, %d) at %d = %d, want %d", r[0], r[1], i, got, want)
			}
			if got < r[0] || got > r[1] {
				t.Fatalf("IntBetween(%d, %d) at %d = %d, out of range", r[0], r[1], i, got)
			}
		}
	}
}

// The worst-case span must actually reject words, reach past the first block,
// and still agree with the reference.
func TestIntBetweenRejectionReachesLaterBlocks(t *testing.T) {
	k := Fork(Seeded(77), "rejection")
	lo, hi := int64(math.MinInt64), int64(0)
	usedHistogram := make(map[int]int)
	for i := int64(0); i < 20000; i++ {
		want, used := referenceIntBetween(k, i, lo, hi)
		usedHistogram[used]++
		if got := mustIntBetween(t, k, i, lo, hi); got != want {
			t.Fatalf("index %d (%d words): got %d, want %d", i, used, got, want)
		}
	}
	if usedHistogram[1] == 20000 {
		t.Fatal("no word was ever rejected; the worst-case span is not being exercised")
	}
	beyondFirstBlock := 0
	for used, count := range usedHistogram {
		if used > 2 {
			beyondFirstBlock += count
		}
	}
	if beyondFirstBlock == 0 {
		t.Fatal("no draw needed a second block; block advancement is untested")
	}
	// With acceptance probability just under 1/2, about half the draws take
	// the first word.
	if first := usedHistogram[1]; first < 9500 || first > 10500 {
		t.Errorf("first-word acceptances = %d of 20000, want about 10000", first)
	}
}

func TestAcceptWord(t *testing.T) {
	if !acceptWord(math.MaxUint64, 0) {
		t.Error("rem 0 must accept every word")
	}
	// span 6: 2^64 mod 6 = 4, so the top 4 words are rejected.
	const rem = 4
	if !acceptWord(math.MaxUint64-4, rem) {
		t.Error("2^64 - 5 must be accepted for rem 4")
	}
	for w := uint64(math.MaxUint64 - 3); w != 0; w++ {
		if acceptWord(w, rem) {
			t.Errorf("%#x must be rejected for rem 4", w)
		}
	}
}

func TestIntBetweenDieIsUniform(t *testing.T) {
	k := Fork(Seeded(6), "die")
	const n = 120000
	var counts [7]int
	for i := int64(0); i < n; i++ {
		counts[mustIntBetween(t, k, i, 1, 6)]++
	}
	// Chi-square with 5 degrees of freedom; 20.5 is the 0.001 critical value.
	expected := float64(n) / 6
	chi := 0.0
	for face := 1; face <= 6; face++ {
		d := float64(counts[face]) - expected
		chi += d * d / expected
	}
	if chi > 20.5 {
		t.Errorf("chi-square = %.2f for counts %v, want <= 20.5", chi, counts[1:])
	}
}

func TestIntBetweenInvalidArguments(t *testing.T) {
	if _, err := IntBetween(1, -1, 1, 6); !errors.Is(err, ErrNegativeIndex) {
		t.Errorf("negative index: error = %v", err)
	}
	if _, err := IntBetween(1, 0, 6, 1); !errors.Is(err, ErrIntRange) {
		t.Errorf("lo above hi: error = %v", err)
	}
}

// ---- Normal ----

func TestNormalFromWordsEdges(t *testing.T) {
	// u1 == 1 exactly: the radius is zero, so the draw is the mean.
	if got := normalFromWords(math.MaxUint64, 0x123456789abcdef0, 3.5, 2); got != 3.5 {
		t.Errorf("u1 = 1: got %v, want the mean 3.5", got)
	}
	// u1 == 2^-53, u2 == 0: the largest magnitude Box-Muller can produce.
	extreme := normalFromWords(0, 0, 0, 1)
	want := math.Sqrt(-2 * math.Log(math.Ldexp(1, -53)))
	if extreme != want {
		t.Errorf("extreme draw = %v, want %v", extreme, want)
	}
	if math.IsInf(extreme, 0) || math.IsNaN(extreme) || extreme < 8.5 || extreme > 8.6 {
		t.Errorf("extreme draw = %v, want a finite value near 8.57", extreme)
	}
	// Every corner of the word space stays finite.
	for _, w0 := range []uint64{0, 1 << 11, math.MaxUint64, 1 << 63} {
		for _, w1 := range []uint64{0, 1 << 11, math.MaxUint64, 1 << 63, 1 << 62} {
			v := normalFromWords(w0, w1, 0, 1)
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("normalFromWords(%#x, %#x) = %v", w0, w1, v)
			}
		}
	}
}

func TestNormalZeroStddevReturnsMean(t *testing.T) {
	k := Seeded(8)
	for i := int64(0); i < 1000; i++ {
		if got := mustNormal(t, k, i, -7.25, 0); got != -7.25 {
			t.Fatalf("Normal(stddev 0) at %d = %v, want -7.25", i, got)
		}
	}
}

func TestNormalIsAffineInItsParameters(t *testing.T) {
	k := Seeded(8)
	for i := int64(0); i < 1000; i++ {
		z := mustNormal(t, k, i, 0, 1)
		want := 10 + float64(2.5*z)
		if got := mustNormal(t, k, i, 10, 2.5); got != want {
			t.Fatalf("Normal(10, 2.5) at %d = %v, want %v", i, got, want)
		}
	}
}

func TestNormalMoments(t *testing.T) {
	k := Fork(Seeded(2026), "normal")
	const n = 200000
	var sum, sumSq, sumCube float64
	within1, within2 := 0, 0
	for i := int64(0); i < n; i++ {
		z := mustNormal(t, k, i, 0, 1)
		sum += z
		sumSq += z * z
		sumCube += z * z * z
		if math.Abs(z) < 1 {
			within1++
		}
		if math.Abs(z) < 2 {
			within2++
		}
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	skew := sumCube / n
	if math.Abs(mean) > 0.01 {
		t.Errorf("mean = %v, want within 0.01 of 0", mean)
	}
	if math.Abs(variance-1) > 0.02 {
		t.Errorf("variance = %v, want within 0.02 of 1", variance)
	}
	if math.Abs(skew) > 0.03 {
		t.Errorf("third moment = %v, want within 0.03 of 0", skew)
	}
	if f := float64(within1) / n; math.Abs(f-0.6827) > 0.005 {
		t.Errorf("fraction within 1 sigma = %v, want about 0.6827", f)
	}
	if f := float64(within2) / n; math.Abs(f-0.9545) > 0.003 {
		t.Errorf("fraction within 2 sigma = %v, want about 0.9545", f)
	}
}

func TestNormalInvalidArguments(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()
	cases := []struct {
		name         string
		i            int64
		mean, stddev float64
		want         error
	}{
		{"negative index", -1, 0, 1, ErrNegativeIndex},
		{"negative stddev", 0, 0, -1, ErrNormalStddev},
		{"infinite stddev", 0, 0, inf, ErrNormalNotReal},
		{"NaN stddev", 0, 0, nan, ErrNormalNotReal},
		{"infinite mean", 0, -inf, 1, ErrNormalNotReal},
		{"NaN mean", 0, nan, 1, ErrNormalNotReal},
	}
	for _, c := range cases {
		if _, err := Normal(1, c.i, c.mean, c.stddev); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
		}
	}
}

// ---- Ladder invariants ----

// I3 parameter isolation, in the shape of the measurement-lab loop that
// motivated v2: changing or zeroing the jitter parameters must not move the
// spike stream.
func TestParameterIsolationAcrossForks(t *testing.T) {
	root := Seeded(1234)
	jitter, spike := Fork(root, "jitter"), Fork(root, "spike")
	for i := int64(0); i < 2000; i++ {
		before := mustUnit(t, spike, i)
		mustNormal(t, jitter, i, 0, 0)
		mustNormal(t, jitter, i, 0, 5)
		if after := mustUnit(t, spike, i); after != before {
			t.Fatalf("spike draw at %d changed after jitter draws", i)
		}
	}
}

// Two forks of one seed must not be shifted or equal copies of each other.
func TestForkedStreamsAreUncorrelated(t *testing.T) {
	root := Seeded(1234)
	a, b := Fork(root, "jitter"), Fork(root, "spike")
	const n = 100000
	var sumA, sumB, sumAB, sumAA, sumBB float64
	for i := int64(0); i < n; i++ {
		x, y := mustUnit(t, a, i)-0.5, mustUnit(t, b, i)-0.5
		sumA += x
		sumB += y
		sumAB += x * y
		sumAA += x * x
		sumBB += y * y
	}
	cov := sumAB/n - (sumA/n)*(sumB/n)
	corr := cov / math.Sqrt((sumAA/n)*(sumBB/n))
	if math.Abs(corr) > 0.015 {
		t.Errorf("correlation between forks = %v, want within 0.015 of 0", corr)
	}
}

// Adjacent indices of one stream must not be serially correlated.
func TestAdjacentIndicesAreUncorrelated(t *testing.T) {
	k := Fork(Seeded(1234), "serial")
	const n = 100000
	var sumXY, sumXX float64
	prev := mustUnit(t, k, 0) - 0.5
	for i := int64(1); i <= n; i++ {
		cur := mustUnit(t, k, i) - 0.5
		sumXY += prev * cur
		sumXX += prev * prev
		prev = cur
	}
	if corr := sumXY / sumXX; math.Abs(corr) > 0.015 {
		t.Errorf("lag-1 correlation = %v, want within 0.015 of 0", corr)
	}
}
