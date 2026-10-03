package octrandom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

// withEntropySource runs fn with the entropy source replaced, and restores the
// real one afterwards.
func withEntropySource(t *testing.T, source io.Reader, fn func()) {
	t.Helper()
	saved := entropySource
	entropySource = source
	defer func() { entropySource = saved }()
	fn()
}

func wordsSource(words ...uint64) io.Reader {
	var buf bytes.Buffer
	for _, w := range words {
		binary.Write(&buf, binary.LittleEndian, w)
	}
	return &buf
}

type failingSource struct{}

func (failingSource) Read([]byte) (int, error) { return 0, errors.New("device unavailable") }

func TestEntropySeedAndUnitUseOneWordEach(t *testing.T) {
	withEntropySource(t, wordsSource(0xfedcba9876543210, 0, math.MaxUint64, 1<<63), func() {
		seed, err := EntropySeed()
		if err != nil || seed != int64(-0x123456789abcdf0) {
			t.Errorf("EntropySeed = %#x, %v; want the word reinterpreted as signed", uint64(seed), err)
		}
		if u, err := EntropyUnit(); err != nil || u != 0 {
			t.Errorf("EntropyUnit(word 0) = %v, %v; want 0", u, err)
		}
		if u, err := EntropyUnit(); err != nil || u != 1-math.Ldexp(1, -53) {
			t.Errorf("EntropyUnit(word max) = %v, %v; want 1 - 2^-53", u, err)
		}
		if u, err := EntropyUnit(); err != nil || u != 0.5 {
			t.Errorf("EntropyUnit(word 2^63) = %v, %v; want 0.5", u, err)
		}
	})
}

func TestEntropyIntBetweenIsUnbiasedByRejection(t *testing.T) {
	// For a d6, 2^64 mod 6 is 4, so the top four words are rejected and the
	// next word is used. The first call therefore skips two words and takes
	// 7. Each accepted word w gives 1 + w mod 6; the last, 2^64 - 5, is the
	// largest accepted word and gives 1 + 5.
	withEntropySource(t, wordsSource(math.MaxUint64, math.MaxUint64-3, 7, 0, 5, math.MaxUint64-4), func() {
		for call, want := range []int64{2, 1, 6, 6} {
			got, err := EntropyIntBetween(1, 6)
			if err != nil || got != want {
				t.Fatalf("call %d: EntropyIntBetween(1, 6) = %d, %v; want %d", call, got, err, want)
			}
		}
		// All six words are consumed: two rejected and four accepted.
		if _, err := EntropyIntBetween(1, 6); err == nil {
			t.Fatal("a seventh word was available; the rejected words were not consumed")
		}
	})
}

func TestEntropyIntBetweenRanges(t *testing.T) {
	withEntropySource(t, wordsSource(0x8000000000000001, 12345, 99), func() {
		if got, err := EntropyIntBetween(math.MinInt64, math.MaxInt64); err != nil || got != math.MinInt64+1 {
			t.Errorf("full range = %d, %v; want the word reinterpreted as signed", got, err)
		}
		if got, err := EntropyIntBetween(7, 7); err != nil || got != 7 {
			t.Errorf("lo == hi = %d, %v; want 7", got, err)
		}
		if got, err := EntropyIntBetween(-5, 5); err != nil || got != -5+99%11 {
			t.Errorf("[-5, 5] = %d, %v; want %d", got, err, -5+99%11)
		}
	})
}

func TestEntropyBytes(t *testing.T) {
	withEntropySource(t, bytes.NewReader([]byte{1, 2, 3, 4, 5, 6}), func() {
		got, err := EntropyBytes(4)
		if err != nil || !bytes.Equal(got, []byte{1, 2, 3, 4}) {
			t.Errorf("EntropyBytes(4) = %v, %v", got, err)
		}
		empty, err := EntropyBytes(0)
		if err != nil || len(empty) != 0 || empty == nil {
			t.Errorf("EntropyBytes(0) = %v, %v; want an empty, non-nil slice", empty, err)
		}
	})
}

func TestEntropyPreconditionsAreNotSourceFailures(t *testing.T) {
	if _, err := EntropyIntBetween(6, 1); !errors.Is(err, ErrEntropyIntRange) || !IsPrecondition(err) {
		t.Errorf("EntropyIntBetween(6, 1) error = %v; want the range precondition", err)
	}
	if _, err := EntropyBytes(-1); !errors.Is(err, ErrEntropyByteCount) || !IsPrecondition(err) {
		t.Errorf("EntropyBytes(-1) error = %v; want the count precondition", err)
	}
	// A precondition is reported before any entropy is read.
	withEntropySource(t, failingSource{}, func() {
		if _, err := EntropyIntBetween(6, 1); !IsPrecondition(err) {
			t.Errorf("EntropyIntBetween(6, 1) with a failing source = %v; want the precondition", err)
		}
		if _, err := EntropyBytes(-1); !IsPrecondition(err) {
			t.Errorf("EntropyBytes(-1) with a failing source = %v; want the precondition", err)
		}
	})
}

func TestEntropySourceFailureIsAnOrdinaryError(t *testing.T) {
	withEntropySource(t, failingSource{}, func() {
		calls := map[string]func() error{
			"EntropySeed":       func() error { _, err := EntropySeed(); return err },
			"EntropyUnit":       func() error { _, err := EntropyUnit(); return err },
			"EntropyIntBetween": func() error { _, err := EntropyIntBetween(1, 6); return err },
			"EntropyBytes":      func() error { _, err := EntropyBytes(4); return err },
		}
		for name, call := range calls {
			err := call()
			if err == nil {
				t.Errorf("%s succeeded with a failing source", name)
				continue
			}
			if IsPrecondition(err) {
				t.Errorf("%s reported a source failure as a precondition: %v", name, err)
			}
		}
	})
	// A source that runs dry part-way through a read is also a failure.
	withEntropySource(t, bytes.NewReader([]byte{1, 2, 3}), func() {
		if _, err := EntropySeed(); err == nil || IsPrecondition(err) {
			t.Errorf("EntropySeed with a short source = %v; want a source failure", err)
		}
		if _, err := EntropyBytes(8); err == nil || IsPrecondition(err) {
			t.Errorf("EntropyBytes with a short source = %v; want a source failure", err)
		}
	})
}

func TestIsPreconditionCoversEveryDeterministicSentinel(t *testing.T) {
	for _, err := range []error{ErrNegativeIndex, ErrFloatRange, ErrFloatSpan, ErrIntRange, ErrNormalStddev, ErrNormalNotReal, ErrEntropyIntRange, ErrEntropyByteCount} {
		if !IsPrecondition(err) {
			t.Errorf("IsPrecondition(%v) = false", err)
		}
	}
	if IsPrecondition(nil) || IsPrecondition(errors.New("other")) || IsPrecondition(errEntropyUnreadable) {
		t.Error("IsPrecondition accepted an error that is not a precondition")
	}
}

// The real source: values are in range and do not repeat.
func TestEntropyRealSourceSmoke(t *testing.T) {
	seen := map[int64]bool{}
	for i := 0; i < 64; i++ {
		seed, err := EntropySeed()
		if err != nil {
			t.Fatal(err)
		}
		if seen[seed] {
			t.Fatalf("EntropySeed repeated %d within 64 draws", seed)
		}
		seen[seed] = true

		u, err := EntropyUnit()
		if err != nil || u < 0 || u >= 1 {
			t.Fatalf("EntropyUnit = %v, %v", u, err)
		}
		d, err := EntropyIntBetween(1, 6)
		if err != nil || d < 1 || d > 6 {
			t.Fatalf("EntropyIntBetween(1, 6) = %v, %v", d, err)
		}
	}
	a, _ := EntropyBytes(32)
	b, _ := EntropyBytes(32)
	if len(a) != 32 || len(b) != 32 || bytes.Equal(a, b) {
		t.Fatalf("EntropyBytes(32) twice = %x and %x", a, b)
	}
}
