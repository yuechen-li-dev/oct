package plotrender

import (
	"fmt"
	"math"
	"path/filepath"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type Kind string

const (
	KindLine      Kind = "line"
	KindScatter   Kind = "scatter"
	KindHistogram Kind = "histogram"
)

type Request struct {
	FunctionName string
	Kind         Kind
	XS           []float64
	YS           []float64
	OutputPath   string
	Width        vg.Length
	Height       vg.Length
	Title        string
	XLabel       string
	YLabel       string
	Legend       string
	HistogramBin int
}

func PixelLength(px int) vg.Length {
	return vg.Length(float64(px)) * vg.Points(1)
}

func Render(request Request) error {
	if filepath.Ext(request.OutputPath) != ".png" {
		return fmt.Errorf("runtime error: plot output path must end with .png")
	}
	if request.Width <= 0 || request.Height <= 0 {
		return fmt.Errorf("runtime error: function '%s' requires width and height to be positive", request.FunctionName)
	}
	if err := validateFinite(request.XS, "x"); err != nil {
		return fmt.Errorf("runtime error: function '%s' %w", request.FunctionName, err)
	}
	if err := validateFinite(request.YS, "y"); err != nil {
		return fmt.Errorf("runtime error: function '%s' %w", request.FunctionName, err)
	}

	p := plot.New()
	if request.Title != "" {
		p.Title.Text = request.Title
	}
	if request.XLabel != "" {
		p.X.Label.Text = request.XLabel
	}
	if request.YLabel != "" {
		p.Y.Label.Text = request.YLabel
	}

	switch request.Kind {
	case KindLine, KindScatter:
		if len(request.XS) == 0 || len(request.YS) == 0 {
			return fmt.Errorf("runtime error: function '%s' requires non-empty x and y arrays", request.FunctionName)
		}
		if len(request.XS) != len(request.YS) {
			return fmt.Errorf("runtime error: function '%s' requires x and y arrays of equal length", request.FunctionName)
		}
		points := make(plotter.XYs, len(request.XS))
		for idx := range request.XS {
			points[idx].X = request.XS[idx]
			points[idx].Y = request.YS[idx]
		}
		switch request.Kind {
		case KindLine:
			line, err := plotter.NewLine(points)
			if err != nil {
				return fmt.Errorf("runtime error: plotting backend failed: %w", err)
			}
			p.Add(line)
			if request.Legend != "" {
				p.Legend.Add(request.Legend, line)
			}
		case KindScatter:
			scatter, err := plotter.NewScatter(points)
			if err != nil {
				return fmt.Errorf("runtime error: plotting backend failed: %w", err)
			}
			p.Add(scatter)
			if request.Legend != "" {
				p.Legend.Add(request.Legend, scatter)
			}
		}
	case KindHistogram:
		if len(request.XS) == 0 {
			return fmt.Errorf("runtime error: function '%s' requires non-empty values array", request.FunctionName)
		}
		if request.HistogramBin <= 0 {
			return fmt.Errorf("runtime error: function '%s' requires a positive bin count", request.FunctionName)
		}
		values := make(plotter.Values, len(request.XS))
		for idx := range request.XS {
			values[idx] = request.XS[idx]
		}
		histogram, err := plotter.NewHist(values, request.HistogramBin)
		if err != nil {
			return fmt.Errorf("runtime error: plotting backend failed: %w", err)
		}
		p.Add(histogram)
		if request.Legend != "" {
			p.Legend.Add(request.Legend, histogram)
		}
	default:
		return fmt.Errorf("runtime invariant violation: unsupported plotting kind '%s'", request.Kind)
	}

	if err := p.Save(request.Width, request.Height, filepath.Clean(request.OutputPath)); err != nil {
		return fmt.Errorf("runtime error: plotting backend failed: %w", err)
	}
	return nil
}

func validateFinite(values []float64, label string) error {
	for idx, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("requires finite %s value at index %d", label, idx)
		}
	}
	return nil
}
