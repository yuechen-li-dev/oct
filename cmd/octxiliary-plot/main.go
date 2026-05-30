// Command octxiliary-plot serves the Plot standard-library Octxiliary wrapper.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/internal/plotrender"
)

type plotSize struct {
	width  int
	height int
}

type plotLabels struct {
	title  string
	x      string
	y      string
	legend string
}

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

func dispatch(req octxiliary.Request) (octxiliary.Value, error) {
	if req.Family != "Plot" {
		return octxiliary.Value{}, fmt.Errorf("unknown family %q", req.Family)
	}
	if !req.HasArgs {
		return octxiliary.Value{}, fmt.Errorf("generic args missing")
	}
	switch req.Function {
	case "PlotRenderLine":
		if err := expect(req.Args, octxiliary.ValueFloatArray, octxiliary.ValueFloatArray, octxiliary.ValueString, octxiliary.ValueRecord, octxiliary.ValueRecord); err != nil {
			return octxiliary.Value{}, err
		}
		return renderXY(req.Function, plotrender.KindLine, req.Args)
	case "PlotRenderScatter":
		if err := expect(req.Args, octxiliary.ValueFloatArray, octxiliary.ValueFloatArray, octxiliary.ValueString, octxiliary.ValueRecord, octxiliary.ValueRecord); err != nil {
			return octxiliary.Value{}, err
		}
		return renderXY(req.Function, plotrender.KindScatter, req.Args)
	case "PlotRenderHistogram":
		if err := expect(req.Args, octxiliary.ValueFloatArray, octxiliary.ValueInt, octxiliary.ValueString, octxiliary.ValueRecord, octxiliary.ValueRecord); err != nil {
			return octxiliary.Value{}, err
		}
		return renderHistogram(req.Function, req.Args)
	default:
		return octxiliary.Value{}, fmt.Errorf("unknown function %q", req.Function)
	}
}

func renderXY(functionName string, kind plotrender.Kind, args []octxiliary.Value) (octxiliary.Value, error) {
	size, err := decodeSize(args[3])
	if err != nil {
		return octxiliary.Value{}, err
	}
	labels, err := decodeLabels(args[4])
	if err != nil {
		return octxiliary.Value{}, err
	}
	if err := finiteFloats(args[0].Floats, "x"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := finiteFloats(args[1].Floats, "y"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := plotrender.Render(plotrender.Request{FunctionName: functionName, Kind: kind, XS: args[0].Floats, YS: args[1].Floats, OutputPath: args[2].String, Width: plotrender.PixelLength(size.width), Height: plotrender.PixelLength(size.height), Title: labels.title, XLabel: labels.x, YLabel: labels.y, Legend: labels.legend}); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func renderHistogram(functionName string, args []octxiliary.Value) (octxiliary.Value, error) {
	size, err := decodeSize(args[3])
	if err != nil {
		return octxiliary.Value{}, err
	}
	labels, err := decodeLabels(args[4])
	if err != nil {
		return octxiliary.Value{}, err
	}
	if err := finiteFloats(args[0].Floats, "values"); err != nil {
		return octxiliary.Value{}, err
	}
	if err := plotrender.Render(plotrender.Request{FunctionName: functionName, Kind: plotrender.KindHistogram, XS: args[0].Floats, OutputPath: args[2].String, Width: plotrender.PixelLength(size.width), Height: plotrender.PixelLength(size.height), Title: labels.title, XLabel: labels.x, YLabel: labels.y, Legend: labels.legend, HistogramBin: args[1].Int}); err != nil {
		return octxiliary.Value{}, err
	}
	return octxiliary.Value{Kind: octxiliary.ValueInt, Int: 0}, nil
}

func decodeSize(value octxiliary.Value) (plotSize, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != "Plot.Size" {
		return plotSize{}, fmt.Errorf("expected Plot.Size record")
	}
	if len(value.Fields) != 2 || value.Fields[0].Name != "Width" || value.Fields[1].Name != "Height" {
		return plotSize{}, fmt.Errorf("Plot.Size fields must be Width, Height")
	}
	if value.Fields[0].Value.Kind != octxiliary.ValueInt || value.Fields[1].Value.Kind != octxiliary.ValueInt {
		return plotSize{}, fmt.Errorf("Plot.Size fields must be Int")
	}
	return plotSize{width: value.Fields[0].Value.Int, height: value.Fields[1].Value.Int}, nil
}

func decodeLabels(value octxiliary.Value) (plotLabels, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != "Plot.Labels" {
		return plotLabels{}, fmt.Errorf("expected Plot.Labels record")
	}
	want := []string{"Title", "X", "Y", "Legend"}
	if len(value.Fields) != len(want) {
		return plotLabels{}, fmt.Errorf("Plot.Labels fields must be Title, X, Y, Legend")
	}
	strings := make([]string, len(want))
	for i, name := range want {
		if value.Fields[i].Name != name {
			return plotLabels{}, fmt.Errorf("Plot.Labels fields must be Title, X, Y, Legend")
		}
		if value.Fields[i].Value.Kind != octxiliary.ValueString {
			return plotLabels{}, fmt.Errorf("Plot.Labels field %s must be String", name)
		}
		strings[i] = value.Fields[i].Value.String
	}
	return plotLabels{title: strings[0], x: strings[1], y: strings[2], legend: strings[3]}, nil
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
