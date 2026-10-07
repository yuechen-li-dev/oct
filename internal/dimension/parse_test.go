package dimension

import "testing"

// Parse reads what String writes, for every dimension.
func TestParseReadsWhatStringWrites(t *testing.T) {
	exponents := []int{-3, -1, 0, 1, 2}
	count := 0
	var walk func(d Dimension, base int)
	walk = func(d Dimension, base int) {
		if base == int(baseCount) {
			count++
			got, ok := Parse(d.String())
			if !ok || got != d {
				t.Errorf("Parse(%q) = %v, %v; want %v", d.String(), got, ok, d)
			}
			return
		}
		// Every exponent for the first four bases, and two for the rest,
		// keeps the walk small.
		choices := exponents
		if base >= 4 {
			choices = []int{0, 1}
		}
		for _, exponent := range choices {
			d.Exponents[base] = exponent
			walk(d, base+1)
		}
	}
	walk(Dimension{}, 0)
	if count != 5*5*5*5*2*2*2*2*2 {
		t.Fatalf("walked %d dimensions", count)
	}
}

func TestParseRefusesWhatStringDoesNotWrite(t *testing.T) {
	for _, text := range []string{"x", "m/", "/s", "m^1", "m^0", "m^-1", "m^two", "m*m", "m//s", "m/s/s", "1*m", "1", "Hz", " m", "m*"} {
		if got, ok := Parse(text); ok {
			t.Errorf("Parse(%q) = %v, want a refusal", text, got)
		}
	}
}
