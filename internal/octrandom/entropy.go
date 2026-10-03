package octrandom

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// The functions in this file back Oct's Entropy package. They read the
// operating system's cryptographically secure random source, so their results
// are not reproducible and they are the only part of this package that is not
// a pure function of its arguments.
//
// Each can fail in two different ways. A violated precondition is a
// programmer error, reported by one of the sentinel errors below; callers turn
// it into a non-recoverable runtime error. Any other error means the entropy
// source itself failed, and callers surface it as an ordinary Oct Error.

// Precondition failures of the Entropy functions.
var (
	ErrEntropyIntRange   = errors.New("entropy: IntBetween requires lo <= hi")
	ErrEntropyByteCount  = errors.New("entropy: Bytes requires count >= 0")
	errEntropyUnreadable = errors.New("entropy: the operating system random source failed")
)

// preconditionErrors lists every sentinel that reports a violated
// precondition, for this file and for the deterministic draws.
var preconditionErrors = []error{
	ErrNegativeIndex,
	ErrFloatRange,
	ErrFloatSpan,
	ErrIntRange,
	ErrNormalStddev,
	ErrNormalNotReal,
	ErrEntropyIntRange,
	ErrEntropyByteCount,
}

// IsPrecondition reports whether err is a violated precondition, as opposed
// to a failure of the entropy source.
func IsPrecondition(err error) bool {
	for _, sentinel := range preconditionErrors {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// entropySource is the random source the Entropy functions read. It is a
// variable only so that tests can substitute a deterministic or failing
// source; production code never assigns it.
var entropySource io.Reader = rand.Reader

// SetEntropySourceForTest replaces the random source and returns a function
// that restores it. It exists for host-side tests of the one path that the
// real source cannot be made to take: a failed read. No production code calls
// it, and it is not safe to use while another goroutine draws entropy.
func SetEntropySourceForTest(source io.Reader) (restore func()) {
	previous := entropySource
	entropySource = source
	return func() { entropySource = previous }
}

func entropyWord() (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(entropySource, buf[:]); err != nil {
		return 0, fmt.Errorf("%w: %v", errEntropyUnreadable, err)
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

// EntropySeed returns a seed drawn from the operating system: every Int value
// is equally likely.
func EntropySeed() (int64, error) {
	w, err := entropyWord()
	return int64(w), err
}

// EntropyUnit returns a uniform value in [0, 1).
func EntropyUnit() (float64, error) {
	w, err := entropyWord()
	if err != nil {
		return 0, err
	}
	return unitFromWord(w), nil
}

// EntropyIntBetween returns an unbiased uniform integer in [lo, hi]. It uses
// the same rejection rule as IntBetween, drawing a fresh word for each try.
func EntropyIntBetween(lo, hi int64) (int64, error) {
	if lo > hi {
		return 0, ErrEntropyIntRange
	}
	span := uint64(hi) - uint64(lo) + 1
	rem := uint64(0)
	if span != 0 {
		rem = (-span) % span
	}
	for {
		w, err := entropyWord()
		if err != nil {
			return 0, err
		}
		if span == 0 {
			return int64(w), nil
		}
		if acceptWord(w, rem) {
			return int64(uint64(lo) + w%span), nil
		}
	}
}

// EntropyBytes returns count random bytes.
func EntropyBytes(count int64) ([]byte, error) {
	if count < 0 {
		return nil, ErrEntropyByteCount
	}
	out := make([]byte, count)
	if _, err := io.ReadFull(entropySource, out); err != nil {
		return nil, fmt.Errorf("%w: %v", errEntropyUnreadable, err)
	}
	return out, nil
}
