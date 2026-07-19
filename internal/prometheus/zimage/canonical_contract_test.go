package zimage

import (
	"math"
	"os"
	"path/filepath"
	"strings"
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

func TestM1BRopeMateChannelsRemainWithinTheirHeadAndAxisPairs(t *testing.T) {
	axisBoundaries := [][2]int{{0, 32}, {32, 80}, {80, 128}}
	channels := []int{0, 1, 30, 31, 32, 33, 78, 79, 80, 81, 126, 127}
	for _, head := range []int{0, CanonicalNoiseRefiner0Heads - 1} {
		for _, channel := range channels {
			mate := channel + 1 - 2*(channel%2)
			if mate/2 != channel/2 {
				t.Fatalf("channel %d mate %d does not preserve pair", channel, mate)
			}
			for _, axis := range axisBoundaries {
				if channel >= axis[0] && channel < axis[1] && (mate < axis[0] || mate >= axis[1]) {
					t.Fatalf("channel %d mate %d crosses axis [%d,%d)", channel, mate, axis[0], axis[1])
				}
			}
			index := (1023*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize + channel
			mateIndex := (1023*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize + mate
			if index/CanonicalNoiseRefiner0HeadSize != mateIndex/CanonicalNoiseRefiner0HeadSize {
				t.Fatalf("head %d channel %d mate %d crosses a head", head, channel, mate)
			}
		}
	}

	for _, source := range []string{"nr0_q_norm_rope.sdslv", "nr0_k_norm_rope.sdslv"} {
		path := filepath.Join("..", "shaders", "sdslv", "production", "zimage", source)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		if !strings.Contains(string(contents), "mateChannel: u32 = channel + 1u - 2u * (channel % 2u);") ||
			!strings.Contains(string(contents), "HeadValues[mateChannel]") {
			t.Fatalf("%s does not retain head-local mate-channel indexing", source)
		}
	}

	qPath := filepath.Join("..", "shaders", "sdslv", "production", "zimage", "nr0_q_norm_rope.sdslv")
	qSource, err := os.ReadFile(qPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(qSource), "token * 11520u + ((head * params.HeadWidth) + channel)") {
		t.Fatal("query physical index does not preserve the fused per-token Q|K|V stride")
	}
	for _, token := range []int{0, 1, 1023} {
		for _, head := range []int{0, CanonicalNoiseRefiner0Heads - 1} {
			first := token*11520 + head*CanonicalNoiseRefiner0HeadSize
			last := first + CanonicalNoiseRefiner0HeadSize - 1
			if first/11520 != token || last/11520 != token {
				t.Fatalf("token %d head %d query span [%d,%d] crosses a fused token row", token, head, first, last)
			}
			if first%11520 >= 3840 || last%11520 >= 3840 {
				t.Fatalf("token %d head %d query span crosses into K", token, head)
			}
		}
	}
}
