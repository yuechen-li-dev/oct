package makecmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/interpret"
)

func TestFlowCheckpointValueRenderParseRoundTripAggregates(t *testing.T) {
	payload := interpret.FlowCheckpointValue{
		Kind:       "Record",
		RecordType: "Snapshot",
		Fields: []interpret.FlowCheckpointField{
			{
				Name: "Choice",
				Type: "Choice",
				Value: interpret.FlowCheckpointValue{
					Kind:     "Enum",
					EnumType: "Choice",
					Variant:  "Some",
					Payload:  &interpret.FlowCheckpointValue{Kind: "Array", Array: []interpret.FlowCheckpointValue{{Kind: "Int", Int: 7}}},
				},
			},
			{
				Name:  "Vector",
				Type:  "Vector<Int, 2>",
				Value: interpret.FlowCheckpointValue{Kind: "Vector", Vector: []interpret.FlowCheckpointValue{{Kind: "Int", Int: 2}, {Kind: "Int", Int: 3}}},
			},
			{
				Name: "Matrix",
				Type: "Matrix<Int, 1, 2>",
				Value: interpret.FlowCheckpointValue{
					Kind: "Matrix", MatrixRows: 1, MatrixCols: 2,
					Matrix: []interpret.FlowCheckpointValue{{Kind: "Int", Int: 4}, {Kind: "Int", Int: 5}},
				},
			},
		},
	}

	var rendered strings.Builder
	renderCPValue(&rendered, payload)
	got := parseCPValue(rendered.String())
	if !reflect.DeepEqual(got, payload) {
		t.Fatalf("checkpoint value round trip mismatch:\nrendered: %s\nwant: %#v\n got: %#v", rendered.String(), payload, got)
	}
}
