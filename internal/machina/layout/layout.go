package layout

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/machina/uiir"
)

const (
	RootLayoutExtent float64 = 1000.0
	AbsCoordLimit    float64 = 1000000.0
	MinZOrder        int64   = -5
	MaxZOrder        int64   = 5
)

func ResolveBoxes(node *uiir.Node, parent *uiir.ResolvedRect) (*uiir.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("ui box resolution requires non-nil node")
	}
	if parent == nil {
		return nil, fmt.Errorf("ui box resolution requires non-nil parent")
	}
	resolved := uiir.Clone(node)
	resolved.Layout = &uiir.ResolvedRect{X: parent.X, Y: parent.Y, Width: parent.Width, Height: parent.Height}
	if resolved.Box != nil {
		rect, err := ResolveBox(resolved.Box, parent)
		if err != nil {
			return nil, err
		}
		resolved.Layout = rect
		resolved.Box.ResolvedRect = rect
	}
	for idx := range resolved.Children {
		child, err := ResolveBoxes(resolved.Children[idx], resolved.Layout)
		if err != nil {
			return nil, err
		}
		resolved.Children[idx] = child
	}
	return resolved, nil
}

func ResolveBox(spec *uiir.BoxSpec, parent *uiir.ResolvedRect) (*uiir.ResolvedRect, error) {
	if spec == nil {
		return nil, fmt.Errorf("box specification is required")
	}
	if parent == nil {
		return nil, fmt.Errorf("parent rect is required")
	}
	if spec.ZOrder < MinZOrder || spec.ZOrder > MaxZOrder {
		return nil, fmt.Errorf("z-order %d out of range [%d,%d]", spec.ZOrder, MinZOrder, MaxZOrder)
	}
	switch spec.Kind {
	case uiir.BoxAbsolute:
		if spec.X < -AbsCoordLimit || spec.X > AbsCoordLimit || spec.Y < -AbsCoordLimit || spec.Y > AbsCoordLimit || spec.Width < 0 || spec.Height < 0 || spec.Width > AbsCoordLimit || spec.Height > AbsCoordLimit {
			return nil, fmt.Errorf("absolute box values out of range")
		}
		return &uiir.ResolvedRect{X: parent.X + spec.X, Y: parent.Y + spec.Y, Width: spec.Width, Height: spec.Height}, nil
	case uiir.BoxAnchored:
		if spec.Left < 0 || spec.Left > 1 || spec.Top < 0 || spec.Top > 1 || spec.Right < 0 || spec.Right > 1 || spec.Bottom < 0 || spec.Bottom > 1 {
			return nil, fmt.Errorf("anchored box fractions must be within [0,1]")
		}
		if spec.Right < spec.Left || spec.Bottom < spec.Top {
			return nil, fmt.Errorf("anchored box right/bottom must be >= left/top")
		}
		x := parent.X + parent.Width*spec.Left
		y := parent.Y + parent.Height*spec.Top
		w := parent.Width * (spec.Right - spec.Left)
		h := parent.Height * (spec.Bottom - spec.Top)
		return &uiir.ResolvedRect{X: x, Y: y, Width: w, Height: h}, nil
	default:
		return nil, fmt.Errorf("unsupported box kind %q", spec.Kind)
	}
}
