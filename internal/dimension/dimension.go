package dimension

import (
	"fmt"
	"strconv"
	"strings"
)

type Base int

const (
	Length Base = iota
	Mass
	Time
	Current
	Temperature
	Amount
	LuminousIntensity
	Pixel
	UI
	baseCount
)

var baseNames = [...]string{"m", "kg", "s", "A", "K", "mol", "cd", "px", "ui"}

type Dimension struct {
	Exponents [baseCount]int
}

func Zero() Dimension {
	return Dimension{}
}

func FromBaseName(name string) (Dimension, bool) {
	if name == "Hz" {
		var dim Dimension
		dim.Exponents[Time] = -1
		return dim, true
	}
	for i, baseName := range baseNames {
		if baseName == name {
			var dim Dimension
			dim.Exponents[i] = 1
			return dim, true
		}
	}
	return Dimension{}, false
}

func (d Dimension) IsDimensionless() bool {
	for _, exponent := range d.Exponents {
		if exponent != 0 {
			return false
		}
	}
	return true
}

func (d Dimension) Multiply(other Dimension) Dimension {
	var result Dimension
	for i := range result.Exponents {
		result.Exponents[i] = d.Exponents[i] + other.Exponents[i]
	}
	return result
}

func (d Dimension) Divide(other Dimension) Dimension {
	var result Dimension
	for i := range result.Exponents {
		result.Exponents[i] = d.Exponents[i] - other.Exponents[i]
	}
	return result
}

func (d Dimension) Pow(exponent int) Dimension {
	var result Dimension
	for i := range result.Exponents {
		result.Exponents[i] = d.Exponents[i] * exponent
	}
	return result
}

func (d Dimension) CanSqrt() bool {
	for _, exponent := range d.Exponents {
		if exponent%2 != 0 {
			return false
		}
	}
	return true
}

func (d Dimension) Sqrt() Dimension {
	var result Dimension
	for i := range result.Exponents {
		result.Exponents[i] = d.Exponents[i] / 2
	}
	return result
}

func (d Dimension) String() string {
	if d.IsDimensionless() {
		return ""
	}

	numerator := make([]string, 0, len(d.Exponents))
	denominator := make([]string, 0, len(d.Exponents))
	for i, exponent := range d.Exponents {
		switch {
		case exponent > 0:
			numerator = append(numerator, formatUnitTerm(baseNames[i], exponent))
		case exponent < 0:
			denominator = append(denominator, formatUnitTerm(baseNames[i], -exponent))
		}
	}

	var builder strings.Builder
	if len(numerator) == 0 {
		builder.WriteString("1")
	} else {
		builder.WriteString(strings.Join(numerator, "*"))
	}
	if len(denominator) > 0 {
		builder.WriteString("/")
		builder.WriteString(strings.Join(denominator, "*"))
	}
	return builder.String()
}

func formatUnitTerm(name string, exponent int) string {
	if exponent == 1 {
		return name
	}
	return fmt.Sprintf("%s^%d", name, exponent)
}

// Parse reads a dimension as String writes it: `m/s`, `kg*m^2/s^2`, `1/s`,
// and "" for no dimension.
func Parse(text string) (Dimension, bool) {
	if text == "" {
		return Dimension{}, true
	}
	numerator, denominator, divided := strings.Cut(text, "/")
	if (!divided && numerator == "1") || (divided && denominator == "") {
		return Dimension{}, false
	}
	result := Dimension{}
	if numerator != "1" {
		var ok bool
		if result, ok = parseProduct(numerator); !ok {
			return Dimension{}, false
		}
	}
	if divided {
		below, ok := parseProduct(denominator)
		if !ok {
			return Dimension{}, false
		}
		result = result.Divide(below)
	}
	return result, true
}

// parseProduct reads `kg*m^2`.
func parseProduct(text string) (Dimension, bool) {
	result := Dimension{}
	for _, term := range strings.Split(text, "*") {
		name, power, raised := strings.Cut(term, "^")
		exponent := 1
		if raised {
			var err error
			if exponent, err = strconv.Atoi(power); err != nil || exponent < 2 {
				return Dimension{}, false
			}
		}
		index := -1
		for i, baseName := range baseNames {
			if baseName == name {
				index = i
			}
		}
		if index < 0 || result.Exponents[index] != 0 {
			return Dimension{}, false
		}
		result.Exponents[index] = exponent
	}
	return result, true
}
