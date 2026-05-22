package interpret

import (
	"gonum.org/v1/plot/vg"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func plotWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"PlotRenderLine":      (*interpreter).evalPlotRenderLineBuiltin,
		"PlotRenderScatter":   (*interpreter).evalPlotRenderScatterBuiltin,
		"PlotRenderHistogram": (*interpreter).evalPlotRenderHistogramBuiltin,
	}
}

func (i *interpreter) evalPlotRenderLineBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalPlotRenderXYBuiltin(env, pkgName, callee, argumentExprs, plotKindLine)
}

func (i *interpreter) evalPlotRenderScatterBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalPlotRenderXYBuiltin(env, pkgName, callee, argumentExprs, plotKindScatter)
}

func (i *interpreter) evalPlotRenderXYBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr, kind plotKind) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(9); err != nil {
		return evalResult{}, err
	}
	xs, errResult, err := numericArrayArg(call, callee, 0, "x")
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	ys, errResult, err := numericArrayArg(call, callee, 1, "y")
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	outputPath, errResult, err := call.stringArg(2)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	width, height, title, xLabel, yLabel, legend, errResult, err := evalPlotMetaArgs(call)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}

	err = renderPlot(plotRenderRequest{
		functionName: callee,
		kind:         kind,
		xs:           xs,
		ys:           ys,
		outputPath:   outputPath,
		width:        width,
		height:       height,
		title:        title,
		xLabel:       xLabel,
		yLabel:       yLabel,
		legend:       legend,
	})
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalPlotRenderHistogramBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(9); err != nil {
		return evalResult{}, err
	}
	values, errResult, err := numericArrayArg(call, callee, 0, "values")
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	bins, errResult, err := call.intArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	outputPath, errResult, err := call.stringArg(2)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	width, height, title, xLabel, yLabel, legend, errResult, err := evalPlotMetaArgs(call)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	err = renderPlot(plotRenderRequest{
		functionName: callee,
		kind:         plotKindHistogram,
		xs:           values,
		outputPath:   outputPath,
		width:        width,
		height:       height,
		title:        title,
		xLabel:       xLabel,
		yLabel:       yLabel,
		legend:       legend,
		histogramBin: int(bins),
	})
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	return wrapperIntResult(0), nil
}

func evalPlotMetaArgs(call wrapperCall) (width vg.Length, height vg.Length, title string, xLabel string, yLabel string, legend string, errResult *evalResult, err error) {
	widthValue, errResult, err := pixelLengthArg(call, 3)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	heightValue, errResult, err := pixelLengthArg(call, 4)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	title, errResult, err = call.stringArg(5)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	xLabel, errResult, err = call.stringArg(6)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	yLabel, errResult, err = call.stringArg(7)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	legend, errResult, err = call.stringArg(8)
	if err != nil || errResult != nil {
		return 0, 0, "", "", "", "", errResult, err
	}
	return widthValue, heightValue, title, xLabel, yLabel, legend, nil, nil
}

func numericArrayArg(call wrapperCall, callee string, index int, name string) ([]float64, *evalResult, error) {
	argument, errResult, err := call.evalArg(index)
	if err != nil || errResult != nil {
		return nil, errResult, err
	}
	points, decodeErr := toPlotData(callee, argument, name)
	if decodeErr != nil {
		result := wrapperErrorResult(callee, decodeErr)
		return nil, &result, nil
	}
	return points, nil, nil
}

func pixelLengthArg(call wrapperCall, index int) (vg.Length, *evalResult, error) {
	argument, errResult, err := call.evalArg(index)
	if err != nil || errResult != nil {
		return 0, errResult, err
	}
	if argument.Kind != ValueInt || argument.Dimension != imagePixelDimension {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects Int<px>", index+1))
		return 0, &result, nil
	}
	if argument.Int <= 0 {
		result := wrapperErrorResult(call.callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument %d expects a positive Int<px>", index+1))
		return 0, &result, nil
	}
	return vg.Length(float64(argument.Int)) * vg.Points(1), nil, nil
}
