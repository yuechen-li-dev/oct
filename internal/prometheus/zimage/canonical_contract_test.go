package zimage

import (
	"math"
	"testing"
)

func TestCanonicalImageCoordinatesUsePaddedTextLength(t *testing.T) {
	padding, err := CanonicalTextPadding(15)
	if err != nil || padding != 17 {
		t.Fatalf("padding = %d, %v; want 17, nil", padding, err)
	}
	cases := []struct {
		token int
		want  CanonicalImageCoordinate
	}{
		{0, CanonicalImageCoordinate{Frame: 33, Row: 0, Col: 0}},
		{31, CanonicalImageCoordinate{Frame: 33, Row: 0, Col: 31}},
		{32, CanonicalImageCoordinate{Frame: 33, Row: 1, Col: 0}},
		{1023, CanonicalImageCoordinate{Frame: 33, Row: 31, Col: 31}},
	}
	for _, test := range cases {
		got, err := CanonicalImageTokenCoordinate(15, test.token)
		if err != nil || got != test.want {
			t.Fatalf("token %d = %+v, %v; want %+v", test.token, got, err, test.want)
		}
	}
}

func TestCanonicalRopeRotationPreservesPairMagnitude(t *testing.T) {
	before := float32(3*3 + 4*4)
	even, odd, err := CanonicalRopeRotate(3, 4, 33, 32, 0)
	if err != nil {
		t.Fatal(err)
	}
	after := even*even + odd*odd
	if math.Abs(float64(after-before)) > 1e-4 {
		t.Fatalf("pair magnitude changed: got %g want %g", after, before)
	}
}

func TestCanonicalRMSNormHasNoMeanSubtraction(t *testing.T) {
	got, err := CanonicalRMSNorm([]float32{2, 2}, []float32{1, 1}, 1e-5)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] < 0.9999 || got[0] > 1.0001 || got[1] < 0.9999 || got[1] > 1.0001 {
		t.Fatalf("RMSNorm = %v; expected uncentered unit-RMS vector", got)
	}
}
