package interpret

import (
	"fmt"
	"path/filepath"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

const (
	defaultPlotWidth  = 4 * vg.Inch
	defaultPlotHeight = 4 * vg.Inch
)

type plotKind string

const (
	plotKindLine      plotKind = "line"
	plotKindScatter   plotKind = "scatter"
	plotKindHistogram plotKind = "histogram"
)

type plotRenderRequest struct {
	functionName string
	kind         plotKind
	xs           []float64
	ys           []float64
	outputPath   string
	width        vg.Length
	height       vg.Length
	title        string
	xLabel       string
	yLabel       string
	legend       string
	histogramBin int
}

func (i interpreter) evalPlotBuiltinCallExpr(env *environment, pkgName string, callee string, arguments []ast.Expr) (Value, error) {
	if len(arguments) != 3 {
		return Value{}, fmt.Errorf("runtime invariant violation: function '%s' expects 3 arguments", callee)
	}

	xResult, err := i.evalExpr(env, pkgName, arguments[0])
	if err != nil {
		return Value{}, err
	}
	if xResult.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached function '%s' argument 1", callee)
	}
	yResult, err := i.evalExpr(env, pkgName, arguments[1])
	if err != nil {
		return Value{}, err
	}
	if yResult.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached function '%s' argument 2", callee)
	}
	pathResult, err := i.evalExpr(env, pkgName, arguments[2])
	if err != nil {
		return Value{}, err
	}
	if pathResult.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached function '%s' argument 3", callee)
	}

	if pathResult.value.Kind != ValueString {
		return Value{}, fmt.Errorf("runtime invariant violation: function '%s' argument 3 expects String, got %s", callee, pathResult.value.Kind)
	}

	xs, err := toPlotData(callee, xResult.value, "x")
	if err != nil {
		return Value{}, err
	}
	ys, err := toPlotData(callee, yResult.value, "y")
	if err != nil {
		return Value{}, err
	}
	if len(xs) == 0 || len(ys) == 0 {
		return Value{}, fmt.Errorf("runtime error: function '%s' requires non-empty x and y arrays", callee)
	}
	if len(xs) != len(ys) {
		return Value{}, fmt.Errorf("runtime error: function '%s' requires x and y arrays of equal length", callee)
	}
	kind, err := plotKindFromBuiltin(callee)
	if err != nil {
		return Value{}, err
	}
	if err := renderPlot(plotRenderRequest{
		functionName: callee,
		kind:         kind,
		xs:           xs,
		ys:           ys,
		outputPath:   pathResult.value.Text,
		width:        defaultPlotWidth,
		height:       defaultPlotHeight,
		xLabel:       "x",
		yLabel:       "y",
	}); err != nil {
		return Value{}, err
	}

	return Value{Kind: ValueInt, Int: 0}, nil
}

func plotKindFromBuiltin(callee string) (plotKind, error) {
	switch callee {
	case "PlotLine", "PlotRenderLine":
		return plotKindLine, nil
	case "PlotScatter", "PlotRenderScatter":
		return plotKindScatter, nil
	case "PlotRenderHistogram":
		return plotKindHistogram, nil
	default:
		return "", fmt.Errorf("runtime invariant violation: unsupported plotting built-in function '%s'", callee)
	}
}

func toPlotData(functionName string, value Value, label string) ([]float64, error) {
	if value.Kind != ValueArray {
		return nil, fmt.Errorf("runtime error: function '%s' requires argument %s to be an array", functionName, label)
	}
	points := make([]float64, 0, len(value.Array))
	for _, element := range value.Array {
		if !element.Dimension.IsDimensionless() {
			return nil, fmt.Errorf("runtime error: function '%s' does not accept dimensioned arrays", functionName)
		}
		switch element.Kind {
		case ValueInt:
			points = append(points, float64(element.Int))
		case ValueFloat:
			points = append(points, element.Float)
		default:
			return nil, fmt.Errorf("runtime error: function '%s' requires argument %s to contain only Int or Float values", functionName, label)
		}
	}
	return points, nil
}

func renderPlot(request plotRenderRequest) error {
	request.outputPath = attributedOutputPath(request.outputPath)
	if filepath.Ext(request.outputPath) != ".png" {
		return fmt.Errorf("runtime error: plot output path must end with .png")
	}
	if request.width <= 0 || request.height <= 0 {
		return fmt.Errorf("runtime error: function '%s' requires width and height to be positive", request.functionName)
	}

	p := plot.New()
	if request.title != "" {
		p.Title.Text = request.title
	}
	if request.xLabel != "" {
		p.X.Label.Text = request.xLabel
	}
	if request.yLabel != "" {
		p.Y.Label.Text = request.yLabel
	}

	switch request.kind {
	case plotKindLine, plotKindScatter:
		if len(request.xs) == 0 || len(request.ys) == 0 {
			return fmt.Errorf("runtime error: function '%s' requires non-empty x and y arrays", request.functionName)
		}
		if len(request.xs) != len(request.ys) {
			return fmt.Errorf("runtime error: function '%s' requires x and y arrays of equal length", request.functionName)
		}
		points := make(plotter.XYs, len(request.xs))
		for idx := range request.xs {
			points[idx].X = request.xs[idx]
			points[idx].Y = request.ys[idx]
		}
		switch request.kind {
		case plotKindLine:
			line, err := plotter.NewLine(points)
			if err != nil {
				return fmt.Errorf("runtime error: plotting backend failed: %w", err)
			}
			p.Add(line)
			if request.legend != "" {
				p.Legend.Add(request.legend, line)
			}
		case plotKindScatter:
			scatter, err := plotter.NewScatter(points)
			if err != nil {
				return fmt.Errorf("runtime error: plotting backend failed: %w", err)
			}
			p.Add(scatter)
			if request.legend != "" {
				p.Legend.Add(request.legend, scatter)
			}
		}
	case plotKindHistogram:
		if len(request.xs) == 0 {
			return fmt.Errorf("runtime error: function '%s' requires non-empty values array", request.functionName)
		}
		if request.histogramBin <= 0 {
			return fmt.Errorf("runtime error: function '%s' requires a positive bin count", request.functionName)
		}
		values := make(plotter.Values, len(request.xs))
		for idx := range request.xs {
			values[idx] = request.xs[idx]
		}
		histogram, err := plotter.NewHist(values, request.histogramBin)
		if err != nil {
			return fmt.Errorf("runtime error: plotting backend failed: %w", err)
		}
		p.Add(histogram)
		if request.legend != "" {
			p.Legend.Add(request.legend, histogram)
		}
	default:
		return fmt.Errorf("runtime invariant violation: unsupported plotting kind '%s'", request.kind)
	}

	if err := p.Save(request.width, request.height, filepath.Clean(request.outputPath)); err != nil {
		return fmt.Errorf("runtime error: plotting backend failed: %w", err)
	}
	return nil
}
