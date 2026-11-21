package layout

import (
	"fmt"
	"math"

	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

// ValidateSpan checks one axis of a span box before resolution: finite
// values, non-negative sizes and insets, and fractions within [0,1] in order.
func ValidateSpan(s uiir.SpanSpec, axis string) error {
	if math.IsNaN(s.A) || math.IsInf(s.A, 0) || math.IsNaN(s.B) || math.IsInf(s.B, 0) {
		return fmt.Errorf("layout.err.invalid_span: %s span values must be finite", axis)
	}
	switch s.Kind {
	case uiir.SpanStart, uiir.SpanEnd:
		if s.B < 0 {
			return fmt.Errorf("layout.err.invalid_span: %s span size must be non-negative, got %g", axis, s.B)
		}
	case uiir.SpanStretch:
		if s.A < 0 || s.B < 0 {
			return fmt.Errorf("layout.err.invalid_span: %s stretch insets must be non-negative, got %g and %g", axis, s.A, s.B)
		}
	case uiir.SpanCenter:
		if s.B < 0 {
			return fmt.Errorf("layout.err.invalid_span: %s center size must be non-negative, got %g", axis, s.B)
		}
	case uiir.SpanFraction:
		if s.A < 0 || s.A > 1 || s.B < 0 || s.B > 1 || s.B < s.A {
			return fmt.Errorf("layout.err.invalid_span: %s fractions must satisfy 0 <= from <= to <= 1, got %g and %g", axis, s.A, s.B)
		}
	default:
		return fmt.Errorf("layout.err.invalid_span: unsupported %s span kind %q", axis, s.Kind)
	}
	return nil
}

// ResolveSpan returns the position and extent of one axis of a span box
// inside a parent interval [start, start+size]. Stretch insets that exceed
// the parent are an error rather than a negative or clamped size.
func ResolveSpan(s uiir.SpanSpec, start, size float64, axis string) (float64, float64, error) {
	if err := ValidateSpan(s, axis); err != nil {
		return 0, 0, err
	}
	switch s.Kind {
	case uiir.SpanStart:
		return start + s.A, s.B, nil
	case uiir.SpanEnd:
		return start + size - s.A - s.B, s.B, nil
	case uiir.SpanStretch:
		extent := size - s.A - s.B
		if extent < 0 {
			return 0, 0, fmt.Errorf("layout.err.span_overflow: %s stretch insets %g + %g exceed the parent's %g", axis, s.A, s.B, size)
		}
		return start + s.A, extent, nil
	case uiir.SpanCenter:
		return start + (size-s.B)/2, s.B, nil
	default: // SpanFraction
		return start + size*s.A, size * (s.B - s.A), nil
	}
}
