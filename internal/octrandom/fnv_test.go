package octrandom

import (
	"hash/fnv"
	"testing"
)

// Reference values from the FNV test suite published by Fowler, Noll and Vo
// (http://www.isthe.com/chongo/tech/comp/fnv/, test_fnv.c, FNV-1a 64-bit).
func TestFNV1a64ReferenceVectors(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"", 0xcbf29ce484222325},
		{"a", 0xaf63dc4c8601ec8c},
		{"b", 0xaf63df4c8601f1a5},
		{"foo", 0xdcb27518fed9d577},
		{"foobar", 0x85944171f73967e8},
	}
	for _, c := range cases {
		if got := FNV1a64(c.in); got != c.want {
			t.Errorf("FNV1a64(%q) = %#016x, want %#016x", c.in, got, c.want)
		}
	}
}

// Cross-check against the standard library on inputs including multi-byte
// UTF-8 and embedded NUL, since labels are hashed as raw UTF-8 bytes.
func TestFNV1a64MatchesStandardLibrary(t *testing.T) {
	for _, in := range []string{"jitter", "spike", "spike-sign", "σ-noise", "噪声", "a\x00b", "x y\tz\n"} {
		h := fnv.New64a()
		h.Write([]byte(in))
		if got, want := FNV1a64(in), h.Sum64(); got != want {
			t.Errorf("FNV1a64(%q) = %#016x, hash/fnv = %#016x", in, got, want)
		}
	}
}
