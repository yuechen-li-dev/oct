package octxiliary

import (
	"math"
	"strings"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	req := Request{ID: 1, Family: "IO.File", Function: "FileReadText", Path: "/tmp/a"}
	got, _ := ParseRequest(EncodeRequest(req))
	if got.Function != req.Function || got.Path != req.Path {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}

func TestWriteRequestRoundTrip(t *testing.T) {
	req := Request{ID: 2, Family: "IO.File", Function: "FileWriteText", Path: "/tmp/b", Text: "hello"}
	got, _ := ParseRequest(EncodeRequest(req))
	if got.Function != req.Function || got.Path != req.Path || got.Text != req.Text {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}

func TestReadLinesRequestRoundTrip(t *testing.T) {
	req := Request{ID: 3, Family: "IO.File", Function: "FileReadLines", Path: "/tmp/c"}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || got.Function != req.Function || got.Path != req.Path {
		t.Fatalf("roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestWriteLinesRequestRoundTrip(t *testing.T) {
	req := Request{ID: 4, Family: "IO.File", Function: "FileWriteLines", Path: "/tmp/d", Lines: []string{"alpha", "", ""}, HasLines: true}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || got.Function != req.Function || got.Path != req.Path || len(got.Lines) != 3 || got.Lines[1] != "" || got.Lines[2] != "" {
		t.Fatalf("roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestMalformedRequestFails(t *testing.T) {
	if _, err := ParseRequest("OctxiliaryRequest { nope }"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWriteBytesRequestRoundTrip(t *testing.T) {
	req := Request{ID: 5, Family: "IO.File", Function: "FileWriteBytes", Path: "/tmp/e", Bytes: []byte{0, 1, 10, 255}, HasBytes: true}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || got.Function != req.Function || got.Path != req.Path || !got.HasBytes || len(got.Bytes) != len(req.Bytes) || got.Bytes[0] != 0 || got.Bytes[3] != 255 {
		t.Fatalf("roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestBytesResponseRoundTrip(t *testing.T) {
	resp := Response{ID: 6, OK: true, Bytes: []byte{0, 42, 255}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || !got.OK || len(got.Bytes) != 3 || got.Bytes[0] != 0 || got.Bytes[1] != 42 || got.Bytes[2] != 255 {
		t.Fatalf("roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestEmptyBytesPayloadRoundTrip(t *testing.T) {
	req := Request{ID: 7, Family: "IO.File", Function: "FileWriteBytes", Path: "/tmp/empty", Bytes: []byte{}, HasBytes: true}
	gotReq, reqErr := ParseRequest(EncodeRequest(req))
	if reqErr != nil || !gotReq.HasBytes || len(gotReq.Bytes) != 0 {
		t.Fatalf("empty request bytes mismatch: %#v err=%v", gotReq, reqErr)
	}

	resp := Response{ID: 8, OK: true, Bytes: []byte{}}
	gotResp, respErr := ParseResponse(EncodeResponse(resp))
	if respErr != nil || !gotResp.OK || len(gotResp.Bytes) != 0 {
		t.Fatalf("empty response bytes mismatch: %#v err=%v", gotResp, respErr)
	}
}

func TestInvalidByteGreaterThan255Fails(t *testing.T) {
	_, err := ParseRequest(`OctxiliaryRequest { id: 9 family: "IO.File" function: "FileWriteBytes" path: "/tmp/x" bytes: { 256 } }`)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestInvalidByteLessThan0Fails(t *testing.T) {
	_, err := ParseRequest(`OctxiliaryRequest { id: 10 family: "IO.File" function: "FileWriteBytes" path: "/tmp/x" bytes: { -1 } }`)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGenericRequestRoundTripAllSupportedArgs(t *testing.T) {
	req := Request{ID: 20, Family: "TestWrapper", Function: "All", HasArgs: true, Args: []Value{
		{Kind: ValueVoid},
		{Kind: ValueInt, Int: 7},
		{Kind: ValueFloat, Float: 3.5},
		{Kind: ValueBool, Bool: true},
		{Kind: ValueString, String: "hello"},
		{Kind: ValueStringArray, Strings: []string{"a", "", "b"}},
		{Kind: ValueBytes, Bytes: []byte{0, 1, 255}},
	}}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || !got.HasArgs || len(got.Args) != len(req.Args) {
		t.Fatalf("generic request roundtrip mismatch: %#v err=%v", got, err)
	}
	if got.Args[1].Int != 7 || got.Args[2].Float != 3.5 || !got.Args[3].Bool || got.Args[4].String != "hello" || got.Args[5].Strings[1] != "" || got.Args[6].Bytes[2] != 255 {
		t.Fatalf("generic request payload mismatch: %#v", got.Args)
	}
}

func TestGenericResponseRoundTripSupportedReturnKinds(t *testing.T) {
	cases := []Value{
		{Kind: ValueVoid},
		{Kind: ValueInt, Int: 9},
		{Kind: ValueFloat, Float: 4.25},
		{Kind: ValueBool, Bool: true},
		{Kind: ValueString, String: "ok"},
		{Kind: ValueStringArray, Strings: []string{"x", "y"}},
		{Kind: ValueBytes, Bytes: []byte{2, 3}},
	}
	for _, value := range cases {
		resp := Response{ID: 21, OK: true, HasValue: true, Value: value}
		got, err := ParseResponse(EncodeResponse(resp))
		if err != nil || !got.OK || !got.HasValue || got.Value.Kind != value.Kind {
			t.Fatalf("generic response roundtrip mismatch for %s: %#v err=%v", value.Kind, got, err)
		}
	}
}

func TestGenericEmptyStringArrayAndBytesRoundTrip(t *testing.T) {
	req := Request{ID: 22, Family: "TestWrapper", Function: "Empty", HasArgs: true, Args: []Value{{Kind: ValueStringArray, Strings: []string{}}, {Kind: ValueBytes, Bytes: []byte{}}}}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || len(got.Args) != 2 || len(got.Args[0].Strings) != 0 || len(got.Args[1].Bytes) != 0 {
		t.Fatalf("empty generic payload mismatch: %#v err=%v", got, err)
	}
}

func TestGenericUnknownKindRejected(t *testing.T) {
	_, err := ParseRequest(`OctxiliaryRequest { id: 23 family: "TestWrapper" function: "Bad" args: [ OctxiliaryValue { kind: "Handle" } ] }`)
	if err == nil {
		t.Fatal("expected unknown kind parse error")
	}
}

func TestGenericMalformedPayloadRejected(t *testing.T) {
	_, err := ParseResponse(`OctxiliaryResponse { id: 24 ok: true value: OctxiliaryValue { kind: "Int" string: "wrong" } }`)
	if err == nil {
		t.Fatal("expected malformed value payload parse error")
	}
}

func TestGenericStringMatrixRequestRoundTrip(t *testing.T) {
	req := Request{ID: 30, Family: "TestWrapper", Function: "EchoRows", HasArgs: true, Args: []Value{{Kind: ValueStringMatrix, Strings2: [][]string{{"a", "b"}, {"c", "d"}}}}}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || !got.HasArgs || len(got.Args) != 1 || got.Args[0].Kind != ValueStringMatrix || got.Args[0].Strings2[1][1] != "d" {
		t.Fatalf("String[][] request roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixResponseRoundTrip(t *testing.T) {
	resp := Response{ID: 31, OK: true, HasValue: true, Value: Value{Kind: ValueStringMatrix, Strings2: [][]string{{"name", "score"}, {"Ada", "10"}}}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || !got.OK || !got.HasValue || got.Value.Kind != ValueStringMatrix || got.Value.Strings2[1][0] != "Ada" {
		t.Fatalf("String[][] response roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixEmptyOuterRoundTrip(t *testing.T) {
	resp := Response{ID: 32, OK: true, HasValue: true, Value: Value{Kind: ValueStringMatrix, Strings2: [][]string{}}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || got.Value.Kind != ValueStringMatrix || len(got.Value.Strings2) != 0 {
		t.Fatalf("empty String[][] response mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixEmptyRowRoundTrip(t *testing.T) {
	resp := Response{ID: 33, OK: true, HasValue: true, Value: Value{Kind: ValueStringMatrix, Strings2: [][]string{{}}}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || got.Value.Kind != ValueStringMatrix || len(got.Value.Strings2) != 1 || len(got.Value.Strings2[0]) != 0 {
		t.Fatalf("empty-row String[][] response mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixRaggedRowsRoundTrip(t *testing.T) {
	rows := [][]string{{"a", "b"}, {"c"}, {"d", "e", "f"}}
	resp := Response{ID: 34, OK: true, HasValue: true, Value: Value{Kind: ValueStringMatrix, Strings2: rows}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || len(got.Value.Strings2) != 3 || len(got.Value.Strings2[1]) != 1 || got.Value.Strings2[2][2] != "f" {
		t.Fatalf("ragged String[][] response mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixEscapedCellsRoundTrip(t *testing.T) {
	rows := [][]string{{"a,b", "quote \" here", "line\nbreak", ""}}
	resp := Response{ID: 35, OK: true, HasValue: true, Value: Value{Kind: ValueStringMatrix, Strings2: rows}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || len(got.Value.Strings2) != 1 || got.Value.Strings2[0][0] != "a,b" || got.Value.Strings2[0][1] != "quote \" here" || got.Value.Strings2[0][2] != "line\nbreak" || got.Value.Strings2[0][3] != "" {
		t.Fatalf("escaped String[][] response mismatch: %#v err=%v", got, err)
	}
}

func TestGenericStringMatrixMalformedNestedPayloadRejected(t *testing.T) {
	_, err := ParseResponse(`OctxiliaryResponse { id: 36 ok: true value: OctxiliaryValue { kind: "String[][]" strings2: [ [ "a" ] "not-a-row" ] } }`)
	if err == nil {
		t.Fatal("expected malformed String[][] payload parse error")
	}
}

func TestGenericFloatArrayRequestRoundTrip(t *testing.T) {
	req := Request{ID: 40, Family: "Plot", Function: "PlotRenderLine", HasArgs: true, Args: []Value{{Kind: ValueFloatArray, Floats: []float64{1, 2.5, -3}}}}
	got, err := ParseRequest(EncodeRequest(req))
	if err != nil || !got.HasArgs || len(got.Args) != 1 || got.Args[0].Kind != ValueFloatArray || len(got.Args[0].Floats) != 3 || got.Args[0].Floats[1] != 2.5 || got.Args[0].Floats[2] != -3 {
		t.Fatalf("Float[] request roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericFloatArrayResponseRoundTrip(t *testing.T) {
	resp := Response{ID: 41, OK: true, HasValue: true, Value: Value{Kind: ValueFloatArray, Floats: []float64{0.25, -7.5}}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || !got.OK || got.Value.Kind != ValueFloatArray || len(got.Value.Floats) != 2 || got.Value.Floats[0] != 0.25 || got.Value.Floats[1] != -7.5 {
		t.Fatalf("Float[] response roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericFloatArrayEmptyRoundTrip(t *testing.T) {
	resp := Response{ID: 42, OK: true, HasValue: true, Value: Value{Kind: ValueFloatArray, Floats: []float64{}}}
	got, err := ParseResponse(EncodeResponse(resp))
	if err != nil || got.Value.Kind != ValueFloatArray || len(got.Value.Floats) != 0 {
		t.Fatalf("empty Float[] response mismatch: %#v err=%v", got, err)
	}
}

func TestGenericFloatArrayMalformedTokenRejected(t *testing.T) {
	_, err := ParseResponse(`OctxiliaryResponse { id: 43 ok: true value: OctxiliaryValue { kind: "Float[]" floats: [ 1 bad 2 ] } }`)
	if err == nil {
		t.Fatal("expected malformed Float[] payload parse error")
	}
}

func TestGenericFloatArrayNonFiniteRejected(t *testing.T) {
	cases := []string{"NaN", "+Inf", "-Inf"}
	for _, token := range cases {
		_, err := ParseResponse(`OctxiliaryResponse { id: 44 ok: true value: OctxiliaryValue { kind: "Float[]" floats: [ ` + token + ` ] } }`)
		if err == nil {
			t.Fatalf("expected non-finite Float[] token %s to be rejected", token)
		}
	}
}

func TestValidateValueRejectsFloatArrayNaN(t *testing.T) {
	err := ValidateValue(Value{Kind: ValueFloatArray, Floats: []float64{math.NaN()}})
	if err == nil {
		t.Fatal("expected NaN Float[] validation error")
	}
	if !strings.Contains(err.Error(), "Float[] contains non-finite value") || !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("expected clear Float[] index error, got %v", err)
	}
}

func TestValidateValueRejectsFloatArrayInf(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{name: "positive", value: math.Inf(1)},
		{name: "negative", value: math.Inf(-1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateValue(Value{Kind: ValueFloatArray, Floats: []float64{1, tc.value}})
			if err == nil {
				t.Fatal("expected Inf Float[] validation error")
			}
			if !strings.Contains(err.Error(), "Float[] contains non-finite value") || !strings.Contains(err.Error(), "index 1") {
				t.Fatalf("expected clear Float[] index error, got %v", err)
			}
		})
	}
}

func TestValidateRequestRejectsGenericNonFiniteFloatArray(t *testing.T) {
	req := Request{ID: 50, Family: "Plot", Function: "PlotRenderLine", HasArgs: true, Args: []Value{{Kind: ValueFloatArray, Floats: []float64{1, math.NaN()}}}}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatal("expected generic request validation error")
	}
	if !strings.Contains(err.Error(), "arg 0") || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("expected arg and index in validation error, got %v", err)
	}
}

func TestValidateRequestIgnoresLegacyRequestWithoutArgs(t *testing.T) {
	req := Request{ID: 51, Family: "IO.File", Function: "FileReadText", Path: "/tmp/x", Args: []Value{{Kind: ValueFloatArray, Floats: []float64{math.NaN()}}}}
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("expected legacy request without HasArgs to validate, got %v", err)
	}
}

func TestValidateResponseRejectsTypedNonFiniteFloatArray(t *testing.T) {
	resp := Response{ID: 52, OK: true, HasValue: true, Value: Value{Kind: ValueFloatArray, Floats: []float64{math.Inf(1)}}}
	err := ValidateResponse(resp)
	if err == nil {
		t.Fatal("expected typed response validation error")
	}
	if !strings.Contains(err.Error(), "response value") || !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("expected response and index in validation error, got %v", err)
	}
}

func TestValidateResponseIgnoresUntypedOrErrorResponse(t *testing.T) {
	value := Value{Kind: ValueFloatArray, Floats: []float64{math.NaN()}}
	cases := []Response{
		{ID: 53, OK: true, HasValue: false, Value: value},
		{ID: 54, OK: false, HasValue: true, Value: value, Error: "failed"},
	}
	for _, resp := range cases {
		if err := ValidateResponse(resp); err != nil {
			t.Fatalf("expected response %#v to validate, got %v", resp, err)
		}
	}
}

func TestValidateValueRejectsRecordFieldNonFiniteFloatArray(t *testing.T) {
	value := Value{Kind: ValueRecord, RecordType: "Plot.Payload", Fields: []FieldValue{{Name: "Values", Value: Value{Kind: ValueFloatArray, Floats: []float64{1, math.Inf(-1)}}}}}
	err := ValidateValue(value)
	if err == nil {
		t.Fatal("expected record field validation error")
	}
	if !strings.Contains(err.Error(), "record field \"Values\"") || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("expected field and index in validation error, got %v", err)
	}
}

func TestValidateValueAcceptsFiniteFloatArrayRoundTrip(t *testing.T) {
	value := Value{Kind: ValueFloatArray, Floats: []float64{0, 1.25, -3.5}}
	if err := ValidateValue(value); err != nil {
		t.Fatalf("expected finite Float[] to validate, got %v", err)
	}
	got, err := ParseResponse(EncodeResponse(Response{ID: 55, OK: true, HasValue: true, Value: value}))
	if err != nil || got.Value.Kind != ValueFloatArray || len(got.Value.Floats) != 3 || got.Value.Floats[2] != -3.5 {
		t.Fatalf("finite Float[] roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestEncodeFloatListNoNonFinitePlaceholder(t *testing.T) {
	encoded := EncodeResponse(Response{ID: 56, OK: true, HasValue: true, Value: Value{Kind: ValueFloatArray, Floats: []float64{math.NaN()}}})
	if strings.Contains(encoded, "<non-finite>") {
		t.Fatalf("encoder emitted deprecated non-finite placeholder: %s", encoded)
	}
}

func TestGenericRecordValueRoundTrip(t *testing.T) {
	value := Value{Kind: ValueRecord, RecordType: "Plot.Size", Fields: []FieldValue{
		{Name: "Width", Value: Value{Kind: ValueInt, Int: 800}},
		{Name: "Height", Value: Value{Kind: ValueInt, Int: 600}},
	}}
	got, err := ParseResponse(EncodeResponse(Response{ID: 45, OK: true, HasValue: true, Value: value}))
	if err != nil || got.Value.Kind != ValueRecord || got.Value.RecordType != "Plot.Size" || len(got.Value.Fields) != 2 || got.Value.Fields[1].Value.Int != 600 {
		t.Fatalf("record response roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericRecordStringFieldsRoundTrip(t *testing.T) {
	value := Value{Kind: ValueRecord, RecordType: "Plot.Labels", Fields: []FieldValue{{Name: "Title", Value: Value{Kind: ValueString, String: "Demo"}}, {Name: "X", Value: Value{Kind: ValueString, String: "x"}}}}
	got, err := ParseRequest(EncodeRequest(Request{ID: 46, Family: "Plot", Function: "PlotRenderLine", HasArgs: true, Args: []Value{value}}))
	if err != nil || len(got.Args) != 1 || got.Args[0].RecordType != "Plot.Labels" || got.Args[0].Fields[0].Value.String != "Demo" {
		t.Fatalf("record request roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestGenericMalformedRecordFieldRejected(t *testing.T) {
	_, err := ParseResponse(`OctxiliaryResponse { id: 47 ok: true value: OctxiliaryValue { kind: "Record" recordType: "Plot.Size" fields: [ OctxiliaryField { name: "Width" } ] } }`)
	if err == nil {
		t.Fatal("expected malformed record field parse error")
	}
}

func TestHandleRequestArgRoundTrip(t *testing.T) {
	value := Value{Kind: ValueHandle, HandleFamily: "Xlsx", HandleType: "IO.Workbook", HandleID: 1}
	got, err := ParseRequest(EncodeRequest(Request{ID: 60, Family: "Xlsx", Function: "XlsxAddSheet", HasArgs: true, Args: []Value{value}}))
	if err != nil || len(got.Args) != 1 || got.Args[0].Kind != ValueHandle || got.Args[0].HandleFamily != "Xlsx" || got.Args[0].HandleType != "IO.Workbook" || got.Args[0].HandleID != 1 {
		t.Fatalf("handle request roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestHandleResponseValueRoundTrip(t *testing.T) {
	value := Value{Kind: ValueHandle, HandleFamily: "Xlsx", HandleType: "IO.Workbook", HandleID: 2}
	got, err := ParseResponse(EncodeResponse(Response{ID: 61, OK: true, HasValue: true, Value: value}))
	if err != nil || !got.HasValue || got.Value.Kind != ValueHandle || got.Value.HandleFamily != "Xlsx" || got.Value.HandleType != "IO.Workbook" || got.Value.HandleID != 2 {
		t.Fatalf("handle response roundtrip mismatch: %#v err=%v", got, err)
	}
}

func TestHandleMissingFamilyRejected(t *testing.T) {
	if err := ValidateValue(Value{Kind: ValueHandle, HandleType: "IO.Workbook", HandleID: 1}); err == nil || !strings.Contains(err.Error(), "handleFamily") {
		t.Fatalf("expected handleFamily validation error, got %v", err)
	}
	_, err := ParseResponse(`OctxiliaryResponse { id: 62 ok: true value: OctxiliaryValue { kind: "Handle" handleType: "IO.Workbook" handleID: 1 } }`)
	if err == nil || !strings.Contains(err.Error(), "handleFamily") {
		t.Fatalf("expected missing handleFamily parse error, got %v", err)
	}
}

func TestHandleMissingTypeRejected(t *testing.T) {
	if err := ValidateValue(Value{Kind: ValueHandle, HandleFamily: "Xlsx", HandleID: 1}); err == nil || !strings.Contains(err.Error(), "handleType") {
		t.Fatalf("expected handleType validation error, got %v", err)
	}
	_, err := ParseResponse(`OctxiliaryResponse { id: 63 ok: true value: OctxiliaryValue { kind: "Handle" handleFamily: "Xlsx" handleID: 1 } }`)
	if err == nil || !strings.Contains(err.Error(), "handleType") {
		t.Fatalf("expected missing handleType parse error, got %v", err)
	}
}

func TestHandleNonPositiveIDRejected(t *testing.T) {
	for _, id := range []int{0, -1} {
		err := ValidateValue(Value{Kind: ValueHandle, HandleFamily: "Xlsx", HandleType: "IO.Workbook", HandleID: id})
		if err == nil || !strings.Contains(err.Error(), "positive") {
			t.Fatalf("expected positive handleID validation error for %d, got %v", id, err)
		}
	}
	_, err := ParseResponse(`OctxiliaryResponse { id: 64 ok: true value: OctxiliaryValue { kind: "Handle" handleFamily: "Xlsx" handleType: "IO.Workbook" handleID: 0 } }`)
	if err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected non-positive handleID parse error, got %v", err)
	}
}

func TestHandleNestedInsideRecordValidates(t *testing.T) {
	value := Value{Kind: ValueRecord, RecordType: "Test.Payload", Fields: []FieldValue{{Name: "Workbook", Value: Value{Kind: ValueHandle, HandleFamily: "Xlsx", HandleType: "IO.Workbook", HandleID: 3}}}}
	if err := ValidateValue(value); err != nil {
		t.Fatalf("expected nested handle record to validate, got %v", err)
	}
	value.Fields[0].Value.HandleID = 0
	if err := ValidateValue(value); err == nil || !strings.Contains(err.Error(), "record field \"Workbook\"") || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected nested handle validation error, got %v", err)
	}
}
