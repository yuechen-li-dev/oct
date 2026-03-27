package interpret

import (
	"fmt"
	"path/filepath"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"oct/internal/ast"
)

const (
	plotWidth  = 4 * vg.Inch
	plotHeight = 4 * vg.Inch
)

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
	if !strings.HasSuffix(pathResult.value.Text, ".png") {
		return Value{}, fmt.Errorf("runtime error: plot output path must end with .png")
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

	if err := writePlot(callee, xs, ys, pathResult.value.Text); err != nil {
		return Value{}, err
	}

	return Value{Kind: ValueInt, Int: 0}, nil
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

func writePlot(functionName string, xs []float64, ys []float64, outputPath string) error {
	p := plot.New()
	p.X.Label.Text = "x"
	p.Y.Label.Text = "y"

	points := make(plotter.XYs, len(xs))
	for idx := range xs {
		points[idx].X = xs[idx]
		points[idx].Y = ys[idx]
	}

	switch functionName {
	case "PlotLine":
		line, err := plotter.NewLine(points)
		if err != nil {
			return fmt.Errorf("runtime error: plotting backend failed: %w", err)
		}
		p.Add(line)
	case "PlotScatter":
		scatter, err := plotter.NewScatter(points)
		if err != nil {
			return fmt.Errorf("runtime error: plotting backend failed: %w", err)
		}
		p.Add(scatter)
	default:
		return fmt.Errorf("runtime invariant violation: unsupported plotting built-in function '%s'", functionName)
	}

	if err := p.Save(plotWidth, plotHeight, filepath.Clean(outputPath)); err != nil {
		return fmt.Errorf("runtime error: plotting backend failed: %w", err)
	}
	return nil
}
