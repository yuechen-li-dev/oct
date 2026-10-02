package octrandom

import (
	"errors"
	"math"
	"testing"
)

// philoxWords recomputes w0 and w1 straight from the specification's counter
// table, without going through the package's own helpers.
func philoxWords(key uint64, c0, c1, c2, c3 uint32) (uint64, uint64) {
	out := Philox4x32([4]uint32{c0, c1, c2, c3}, [2]uint32{uint32(key), uint32(key >> 32)})
	return uint64(out[0]) | uint64(out[1])<<32, uint64(out[2]) | uint64(out[3])<<32
}

var layoutKeys = []Key{0, 1, 42, 0xffffffffffffffff, 0x0123456789abcdef, 1 << 32, 1 << 63}

var layoutIndices = []int64{0, 1, 2, 0xffffffff, 1 << 32, 1<<32 + 7, math.MaxInt64}

func TestSeededUsesSeedBitsAsKey(t *testing.T) {
	cases := []struct {
		seed int64
		want Key
	}{
		{0, 0},
		{42, 42},
		{-1, 0xffffffffffffffff},
		{math.MinInt64, 1 << 63},
		{math.MaxInt64, 1<<63 - 1},
	}
	for _, c := range cases {
		if got := Seeded(c.seed); got != c.want {
			t.Errorf("Seeded(%d) = %#x, want %#x", c.seed, uint64(got), uint64(c.want))
		}
	}
}

func TestChildCounterLayout(t *testing.T) {
	for _, k := range layoutKeys {
		for _, i := range layoutIndices {
			got, err := Child(k, i)
			if err != nil {
				t.Fatalf("Child(%#x, %d): %v", uint64(k), i, err)
			}
			want, _ := philoxWords(uint64(k), uint32(uint64(i)), uint32(uint64(i)>>32), 0, 1)
			if uint64(got) != want {
				t.Errorf("Child(%#x, %d) = %#x, want %#x", uint64(k), i, uint64(got), want)
			}
		}
	}
}

func TestForkCounterLayout(t *testing.T) {
	for _, k := range layoutKeys {
		for _, label := range []string{"", "jitter", "spike", "spike-sign", "噪声"} {
			h := FNV1a64(label)
			want, _ := philoxWords(uint64(k), uint32(h), uint32(h>>32), 0, 2)
			if got := Fork(k, label); uint64(got) != want {
				t.Errorf("Fork(%#x, %q) = %#x, want %#x", uint64(k), label, uint64(got), want)
			}
		}
	}
}

func TestDrawCounterLayout(t *testing.T) {
	for _, k := range layoutKeys {
		for _, i := range layoutIndices {
			for _, block := range []uint32{0, 1, 2, 0xffffffff} {
				want0, want1 := philoxWords(uint64(k), uint32(uint64(i)), uint32(uint64(i)>>32), block, 0)
				got0, got1 := drawWords(k, i, block)
				if got0 != want0 || got1 != want1 {
					t.Errorf("drawWords(%#x, %d, %d) = %#x %#x, want %#x %#x", uint64(k), i, block, got0, got1, want0, want1)
				}
			}
		}
	}
}

func TestChildRejectsNegativeIndex(t *testing.T) {
	for _, i := range []int64{-1, -2, math.MinInt64} {
		if _, err := Child(7, i); !errors.Is(err, ErrNegativeIndex) {
			t.Errorf("Child(7, %d) error = %v, want ErrNegativeIndex", i, err)
		}
	}
}

func TestForkIsDeterministicAndLabelSensitive(t *testing.T) {
	root := Seeded(2026)
	if Fork(root, "jitter") != Fork(root, "jitter") {
		t.Fatal("same label produced different streams")
	}
	labels := []string{"", "a", "b", "jitter", "Jitter", "jitter ", "spike", "spike-sign", "sign-spike"}
	seen := make(map[Key]string)
	for _, label := range labels {
		k := Fork(root, label)
		if prev, dup := seen[k]; dup {
			t.Fatalf("labels %q and %q fork to the same stream", prev, label)
		}
		if k == root {
			t.Fatalf("Fork(root, %q) returned the root stream", label)
		}
		seen[k] = label
	}
	if Fork(Seeded(1), "jitter") == Fork(Seeded(2), "jitter") {
		t.Fatal("different parents produced the same fork")
	}
}

// A draw, a Child and a Fork that reach the generator with the same first two
// counter words must still be separated by the domain word.
func TestDomainsAreSeparated(t *testing.T) {
	root := Seeded(99)
	label := ""
	h := FNV1a64(label)
	for int64(h) < 0 {
		label += "x"
		h = FNV1a64(label)
	}
	i := int64(h)

	fork := Fork(root, label)
	child, err := Child(root, i)
	if err != nil {
		t.Fatal(err)
	}
	draw, _ := drawWords(root, i, 0)

	if fork == child {
		t.Error("Fork and Child share a derivation")
	}
	if uint64(fork) == draw {
		t.Error("Fork key equals draw word")
	}
	if uint64(child) == draw {
		t.Error("Child key equals draw word")
	}
}

func TestChildStreamsAreDistinct(t *testing.T) {
	root := Seeded(5)
	seen := make(map[Key]int64)
	for i := int64(0); i < 5000; i++ {
		k, err := Child(root, i)
		if err != nil {
			t.Fatal(err)
		}
		if prev, dup := seen[k]; dup {
			t.Fatalf("Child indices %d and %d give the same stream", prev, i)
		}
		seen[k] = i
	}
}
