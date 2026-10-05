// Command octxiliary-plot serves the Plot standard-library Octxiliary wrapper.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/internal/plotrender"
)

func main() {
	if err := octxiliary.ReadHandshake(os.Stdin); err != nil {
		return
	}
	if err := octxiliary.WriteHandshake(os.Stdout); err != nil {
		return
	}
	for {
		frame, err := octxiliary.ReadFrame(os.Stdin)
		if err != nil {
			return
		}
		req, parseErr := octxiliary.ParseRequest(frame)
		resp := octxiliary.Response{ID: req.ID}
		if parseErr != nil {
			resp.OK = false
			resp.Error = parseErr.Error()
			_ = octxiliary.WriteResponseFrame(os.Stdout, resp)
			continue
		}
		value, err := dispatch(req)
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
		} else {
			resp.OK = true
			resp.Value = value
			resp.HasValue = true
		}
		if err := octxiliary.WriteResponseFrame(os.Stdout, resp); err != nil {
			return
		}
	}
}

// The three wire functions take the arguments of the builtins of the same
// name: the data, the output path, the width and height in pixels, and the
// title, x label, y label and legend.
func dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != "Plot" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	xyKinds := []octxiliary.ValueKind{octxiliary.ValueFloatArray, octxiliary.ValueFloatArray, octxiliary.ValueString, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString}
	switch req.Function {
	case "PlotRenderLine":
		if err := expect(req.Args, xyKinds...); err != nil {
			return octxiliary.Value{}, err
		}
		return renderXY(req.Function, plotrender.KindLine, req.Args)
	case "PlotRenderScatter":
		if err := expect(req.Args, xyKinds...); err != nil {
			return octxiliary.Value{}, err
		}
		return renderXY(req.Function, plotrender.KindScatter, req.Args)
	case "PlotRenderHistogram":
		if err := expect(req.Args, octxiliary.ValueFloatArray, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueInt, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString, octxiliary.ValueString); err != nil {
			return octxiliary.Value{}, err
		}
		return renderHistogram(req.Function, req.Args)
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func renderXY(functionName string, kind plotrender.Kind, args []octxiliary.Value) (octxiliary.Value, error) {
	if err := finiteFloats(args[0].Floats, "x"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := finiteFloats(args[1].Floats, "y"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := plotrender.Render(plotrender.Request{FunctionName: functionName, Kind: kind, XS: args[0].Floats, YS: args[1].Floats, OutputPath: args[2].String, Width: plotrender.PixelLength(args[3].Int), Height: plotrender.PixelLength(args[4].Int), Title: args[5].String, XLabel: args[6].String, YLabel: args[7].String, Legend: args[8].String}); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func renderHistogram(functionName string, args []octxiliary.Value) (octxiliary.Value, error) {
	if err := finiteFloats(args[0].Floats, "values"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := plotrender.Render(plotrender.Request{FunctionName: functionName, Kind: plotrender.KindHistogram, XS: args[0].Floats, OutputPath: args[2].String, Width: plotrender.PixelLength(args[3].Int), Height: plotrender.PixelLength(args[4].Int), Title: args[5].String, XLabel: args[6].String, YLabel: args[7].String, Legend: args[8].String, HistogramBin: args[1].Int}); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func finiteFloats(values []float64, label string) error {
	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s value at index %d must be finite", label, i)
		}
	}
	return nil
}

func expect(args []octxiliary.Value, kinds ...octxiliary.ValueKind) error {
	if len(args) != len(kinds) {
		return fmt.Errorf("expected %d args, got %d", len(kinds), len(args))
	}
	for i, kind := range kinds {
		if args[i].Kind != kind {
			return fmt.Errorf("arg %d expected %s, got %s", i+1, kind, args[i].Kind)
		}
	}
	return nil
}
