package interpret

import (
	"fmt"

	"gonum.org/v1/plot/vg"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/plotrender"
)

const (
	defaultPlotWidth  = 4 * vg.Inch
	defaultPlotHeight = 4 * vg.Inch
)

type plotKind = plotrender.Kind

const (
	plotKindLine      = plotrender.KindLine
	plotKindScatter   = plotrender.KindScatter
	plotKindHistogram = plotrender.KindHistogram
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
	logicalPath := attributedOutputPath(pathResult.value.Text)
	actualPath := logicalPath
	if i.artifactCapability != nil {
		actualPath, err = i.artifactCapability.StageArtifactOutput(ArtifactOutputRequest{
			Path: logicalPath, Package: i.artifactPackage, Function: i.currentFunctionName,
			SourcePath: i.artifactSourcePath, Kind: "plot.line",
		})
		if err != nil {
			return Value{}, fmt.Errorf("runtime error: %w", err)
		}
	}
	if err := renderPlot(plotRenderRequest{
		functionName: callee,
		kind:         kind,
		xs:           xs,
		ys:           ys,
		outputPath:   actualPath,
		width:        defaultPlotWidth,
		height:       defaultPlotHeight,
		xLabel:       "x",
		yLabel:       "y",
	}); err != nil {
		return Value{}, err
	}
	i.recordArtifactWrite(logicalPath)

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
	return plotrender.Render(plotrender.Request{
		FunctionName: request.functionName,
		Kind:         request.kind,
		XS:           request.xs,
		YS:           request.ys,
		OutputPath:   attributedOutputPath(request.outputPath),
		Width:        request.width,
		Height:       request.height,
		Title:        request.title,
		XLabel:       request.xLabel,
		YLabel:       request.yLabel,
		Legend:       request.legend,
		HistogramBin: request.histogramBin,
	})
}
