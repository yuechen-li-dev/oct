package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octxiliary"
)

func TestDispatchLineWritesPNG(t *testing.T) {
	out := filepath.Join(t.TempDir(), "line.png")
	value, err := dispatch(octxiliary.Request{Family: "Plot", Function: "PlotRenderLine", HasArgs: true, Args: []octxiliary.Value{
		{Kind: octxiliary.ValueFloatArray, Floats: []float64{0, 1, 2}},
		{Kind: octxiliary.ValueFloatArray, Floats: []float64{0, 1, 4}},
		{Kind: octxiliary.ValueString, String: out},
		sizeValue(400, 300),
		labelsValue(),
	}})
	if err != nil || value.Kind != octxiliary.ValueInt || value.Int != 0 {
		t.Fatalf("dispatch line failed: value=%#v err=%v", value, err)
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty png: info=%#v err=%v", info, err)
	}
}

func TestDispatchRejectsInvalidArguments(t *testing.T) {
	cases := []struct {
		name string
		args []octxiliary.Value
		want string
	}{
		{name: "extension", args: []octxiliary.Value{{Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueString, String: "bad.jpg"}, sizeValue(400, 300), labelsValue()}, want: ".png"},
		{name: "length", args: []octxiliary.Value{{Kind: octxiliary.ValueFloatArray, Floats: []float64{0, 1}}, {Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "bad.png")}, sizeValue(400, 300), labelsValue()}, want: "equal length"},
		{name: "size", args: []octxiliary.Value{{Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "bad.png")}, sizeValue(0, 300), labelsValue()}, want: "positive"},
		{name: "nan", args: []octxiliary.Value{{Kind: octxiliary.ValueFloatArray, Floats: []float64{math.NaN()}}, {Kind: octxiliary.ValueFloatArray, Floats: []float64{0}}, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "bad.png")}, sizeValue(400, 300), labelsValue()}, want: "finite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dispatch(octxiliary.Request{Family: "Plot", Function: "PlotRenderLine", HasArgs: true, Args: tc.args})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestDispatchHistogramRejectsNonPositiveBins(t *testing.T) {
	_, err := dispatch(octxiliary.Request{Family: "Plot", Function: "PlotRenderHistogram", HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueFloatArray, Floats: []float64{0, 1}}, {Kind: octxiliary.ValueInt, Int: 0}, {Kind: octxiliary.ValueString, String: filepath.Join(t.TempDir(), "hist.png")}, sizeValue(400, 300), labelsValue()}})
	if err == nil || !strings.Contains(err.Error(), "positive bin") {
		t.Fatalf("expected positive bin error, got %v", err)
	}
}

func sizeValue(width, height int) octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: "Plot.Size", Fields: []octxiliary.FieldValue{{Name: "Width", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: width}}, {Name: "Height", Value: octxiliary.Value{Kind: octxiliary.ValueInt, Int: height}}}}
}

func labelsValue() octxiliary.Value {
	return octxiliary.Value{Kind: octxiliary.ValueRecord, RecordType: "Plot.Labels", Fields: []octxiliary.FieldValue{{Name: "Title", Value: octxiliary.Value{Kind: octxiliary.ValueString, String: "Demo"}}, {Name: "X", Value: octxiliary.Value{Kind: octxiliary.ValueString, String: "x"}}, {Name: "Y", Value: octxiliary.Value{Kind: octxiliary.ValueString, String: "y"}}, {Name: "Legend", Value: octxiliary.Value{Kind: octxiliary.ValueString, String: "series"}}}}
}
