package vdmir

import "testing"

func TestFormatTypeRetainsSemanticSpaceEvidence(t *testing.T) {
	got := FormatType(Type{Kind: TypeFloat4, Name: "float4", Space: "zimage.attention.query_head"})
	if got != "float4@space(zimage.attention.query_head)" {
		t.Fatalf("FormatType() = %q", got)
	}
}

func TestFormatTypeRetainsNDArrayElementSpaceEvidence(t *testing.T) {
	element := Type{Kind: TypeFloat4, Name: "float4", Space: "zimage.attention.query_head"}
	got := FormatType(Type{Kind: TypeNDArray, Name: "ndarray", Element: &element, Shape: []uint32{1024, 30}})
	if got != "ndarray<float4@space(zimage.attention.query_head),[1024,30]>" {
		t.Fatalf("FormatType() = %q", got)
	}
}
