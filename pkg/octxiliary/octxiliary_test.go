package octxiliary_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	internal "github.com/yuechen-li-dev/oct/internal/octxiliary"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

func TestServeHandshakeAndOneRequestRoundTrip(t *testing.T) {
	in := inputWithRequests(t, internal.Request{ID: 7, Family: "Echo", Function: "EchoString", HasArgs: true, Args: []internal.Value{{Kind: internal.ValueString, String: "hello"}}})
	var out bytes.Buffer
	err := octxiliary.Serve(in, &out, func(req octxiliary.Request) octxiliary.Response {
		text, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Err(req.ID, err)
		}
		return octxiliary.OkString(req.ID, text)
	})
	if err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	responses := readOutputResponses(t, &out, 1)
	if responses[0].ID != 7 || !responses[0].OK || !responses[0].HasValue || responses[0].Value.Kind != internal.ValueString || responses[0].Value.String != "hello" {
		t.Fatalf("unexpected response: %#v", responses[0])
	}
}

func TestServeHandlesMultipleFrames(t *testing.T) {
	in := inputWithRequests(t,
		internal.Request{ID: 1, Family: "Echo", Function: "ByteLength", HasArgs: true, Args: []internal.Value{{Kind: internal.ValueBytes, Bytes: []byte{1, 2, 3}}}},
		internal.Request{ID: 2, Family: "Echo", Function: "ByteLength", HasArgs: true, Args: []internal.Value{{Kind: internal.ValueBytes, Bytes: []byte{4, 5}}}},
	)
	var out bytes.Buffer
	err := octxiliary.Serve(in, &out, func(req octxiliary.Request) octxiliary.Response {
		data, err := octxiliary.ArgBytes(req, 0)
		if err != nil {
			return octxiliary.Err(req.ID, err)
		}
		return octxiliary.OkInt(req.ID, len(data))
	})
	if err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	responses := readOutputResponses(t, &out, 2)
	if responses[0].Value.Int != 3 || responses[1].Value.Int != 2 {
		t.Fatalf("unexpected responses: %#v", responses)
	}
}

func TestServeCleanEOF(t *testing.T) {
	var in bytes.Buffer
	if err := internal.WriteHandshake(&in); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	called := false
	err := octxiliary.Serve(&in, &out, func(req octxiliary.Request) octxiliary.Response {
		called = true
		return octxiliary.OkVoid(req.ID)
	})
	if err != nil {
		t.Fatalf("Serve returned error for clean EOF: %v", err)
	}
	if called {
		t.Fatal("handler called without a frame")
	}
	if err := internal.ReadHandshake(&out); err != nil {
		t.Fatalf("missing sidecar handshake: %v", err)
	}
	if _, err := internal.ReadFrame(&out); !errors.Is(err, io.EOF) {
		t.Fatalf("expected no frames after handshake, got err=%v", err)
	}
}

func TestServeReturnsProtocolErrorForMalformedFrame(t *testing.T) {
	var in bytes.Buffer
	if err := internal.WriteHandshake(&in); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&in, binary.LittleEndian, uint32(10)); err != nil {
		t.Fatal(err)
	}
	in.WriteString("short")
	var out bytes.Buffer
	err := octxiliary.Serve(&in, &out, func(req octxiliary.Request) octxiliary.Response {
		return octxiliary.OkVoid(req.ID)
	})
	if err == nil || !strings.Contains(err.Error(), "read frame") {
		t.Fatalf("expected read frame error, got %v", err)
	}
}

func TestResponseConstructorsEncodeTypedValues(t *testing.T) {
	cases := []struct {
		name string
		resp octxiliary.Response
		kind octxiliary.ValueKind
	}{
		{"void", octxiliary.OkVoid(1), octxiliary.ValueVoid},
		{"int", octxiliary.OkInt(2, 42), octxiliary.ValueInt},
		{"float", octxiliary.OkFloat(3, 1.5), octxiliary.ValueFloat},
		{"bool", octxiliary.OkBool(4, true), octxiliary.ValueBool},
		{"string", octxiliary.OkString(5, "x"), octxiliary.ValueString},
		{"strings", octxiliary.OkStrings(6, []string{"a", "b"}), octxiliary.ValueStringArray},
		{"matrix", octxiliary.OkStringMatrix(7, [][]string{{"a"}, {"b", "c"}}), octxiliary.ValueStringMatrix},
		{"bytes", octxiliary.OkBytes(8, []byte{9}), octxiliary.ValueBytes},
		{"floats", octxiliary.OkFloats(9, []float64{2.5}), octxiliary.ValueFloatArray},
		{"handle", octxiliary.OkHandle(10, "Image", "Image", 3), octxiliary.ValueHandle},
		{"record", octxiliary.OkRecord(11, "Point", []octxiliary.FieldValue{{Name: "x", Value: octxiliary.IntValue(1)}}), octxiliary.ValueRecord},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := internal.ValidateResponse(tc.resp); err != nil {
				t.Fatalf("constructor produced invalid response: %v", err)
			}
			got, err := internal.ParseResponse(internal.EncodeResponse(tc.resp))
			if err != nil {
				t.Fatalf("ParseResponse failed: %v", err)
			}
			if !got.OK || !got.HasValue || got.Value.Kind != tc.kind {
				t.Fatalf("unexpected response roundtrip: %#v", got)
			}
		})
	}
	if got := octxiliary.ErrString(12, "boom"); got.OK || got.Error != "boom" {
		t.Fatalf("unexpected error response: %#v", got)
	}
}

func TestArgHelpersValidateTypeAndIndexErrors(t *testing.T) {
	req := octxiliary.Request{ID: 1, HasArgs: true, Args: []octxiliary.Value{
		octxiliary.StringValue("s"),
		octxiliary.IntValue(2),
		octxiliary.FloatValue(3.5),
		octxiliary.BoolValue(true),
		octxiliary.BytesValue([]byte{1, 2}),
		octxiliary.StringsValue([]string{"a"}),
		octxiliary.StringMatrixValue([][]string{{"a"}}),
		octxiliary.FloatsValue([]float64{1.25}),
		octxiliary.RecordValue("Pair", []octxiliary.FieldValue{{Name: "left", Value: octxiliary.StringValue("L")}}),
	}}
	if got, err := octxiliary.ArgString(req, 0); err != nil || got != "s" {
		t.Fatalf("ArgString = %q, %v", got, err)
	}
	if got, err := octxiliary.ArgInt(req, 1); err != nil || got != 2 {
		t.Fatalf("ArgInt = %d, %v", got, err)
	}
	if got, err := octxiliary.ArgFloat(req, 2); err != nil || got != 3.5 {
		t.Fatalf("ArgFloat = %v, %v", got, err)
	}
	if got, err := octxiliary.ArgBool(req, 3); err != nil || !got {
		t.Fatalf("ArgBool = %v, %v", got, err)
	}
	bytesArg, err := octxiliary.ArgBytes(req, 4)
	if err != nil || !bytes.Equal(bytesArg, []byte{1, 2}) {
		t.Fatalf("ArgBytes = %v, %v", bytesArg, err)
	}
	bytesArg[0] = 99
	if req.Args[4].Bytes[0] != 1 {
		t.Fatal("ArgBytes did not return a defensive copy")
	}
	if got, err := octxiliary.ArgStrings(req, 5); err != nil || len(got) != 1 || got[0] != "a" {
		t.Fatalf("ArgStrings = %#v, %v", got, err)
	}
	if got, err := octxiliary.ArgStringMatrix(req, 6); err != nil || len(got) != 1 || got[0][0] != "a" {
		t.Fatalf("ArgStringMatrix = %#v, %v", got, err)
	}
	if got, err := octxiliary.ArgFloats(req, 7); err != nil || len(got) != 1 || got[0] != 1.25 {
		t.Fatalf("ArgFloats = %#v, %v", got, err)
	}
	if got, err := octxiliary.ArgRecord(req, 8, "Pair"); err != nil || len(got) != 1 || got[0].Name != "left" {
		t.Fatalf("ArgRecord = %#v, %v", got, err)
	}
	if _, err := octxiliary.ArgString(req, 99); err == nil || !strings.Contains(err.Error(), "arg 99") || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected index error, got %v", err)
	}
	if _, err := octxiliary.ArgInt(req, 0); err == nil || !strings.Contains(err.Error(), "expected Int") {
		t.Fatalf("expected kind error, got %v", err)
	}
	if _, err := octxiliary.Arg(octxiliary.Request{}, 0); err == nil || !strings.Contains(err.Error(), "no generic args") {
		t.Fatalf("expected missing args error, got %v", err)
	}
}

func TestArgHandleValidatesFamilyTypeAndID(t *testing.T) {
	req := octxiliary.Request{HasArgs: true, Args: []octxiliary.Value{octxiliary.HandleValue("Image", "Image", 12)}}
	if got, err := octxiliary.ArgHandle(req, 0, "Image", "Image"); err != nil || got != 12 {
		t.Fatalf("ArgHandle = %d, %v", got, err)
	}
	if _, err := octxiliary.ArgHandle(req, 0, "Plot", "Image"); err == nil || !strings.Contains(err.Error(), "handle family") {
		t.Fatalf("expected family error, got %v", err)
	}
	if _, err := octxiliary.ArgHandle(req, 0, "Image", "Mask"); err == nil || !strings.Contains(err.Error(), "handle type") {
		t.Fatalf("expected type error, got %v", err)
	}
	bad := octxiliary.Request{HasArgs: true, Args: []octxiliary.Value{{Kind: octxiliary.ValueHandle, HandleFamily: "Image", HandleType: "Image", HandleID: 0}}}
	if _, err := octxiliary.ArgHandle(bad, 0, "Image", "Image"); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected ID error, got %v", err)
	}
}

func TestDispatcherRoutesAndReturnsUnsupported(t *testing.T) {
	d := octxiliary.NewDispatcher("Echo")
	d.HandleFunc("EchoString", func(req octxiliary.Request) octxiliary.Response {
		text, err := octxiliary.ArgString(req, 0)
		if err != nil {
			return octxiliary.Err(req.ID, err)
		}
		return octxiliary.OkString(req.ID, text)
	})
	known := d.HandleRequest(octxiliary.Request{ID: 5, Family: "Echo", Function: "EchoString", HasArgs: true, Args: []octxiliary.Value{octxiliary.StringValue("hi")}})
	if !known.OK || known.Value.String != "hi" {
		t.Fatalf("known dispatch failed: %#v", known)
	}
	unknown := d.HandleRequest(octxiliary.Request{ID: 6, Family: "Echo", Function: "Missing"})
	if unknown.OK || !strings.Contains(unknown.Error, "unsupported function") {
		t.Fatalf("expected unsupported function, got %#v", unknown)
	}
	wrongFamily := d.HandleRequest(octxiliary.Request{ID: 7, Family: "Other", Function: "EchoString"})
	if wrongFamily.OK || !strings.Contains(wrongFamily.Error, "unsupported family") {
		t.Fatalf("expected unsupported family, got %#v", wrongFamily)
	}
}

func TestSDKOutputCompatibleWithInternalParserEncoder(t *testing.T) {
	resp := octxiliary.OkRecord(44, "Meta", []octxiliary.FieldValue{
		{Name: "name", Value: octxiliary.StringValue("sample")},
		{Name: "data", Value: octxiliary.BytesValue([]byte{1, 2, 3})},
	})
	encoded := internal.EncodeResponse(resp)
	parsed, err := internal.ParseResponse(encoded)
	if err != nil {
		t.Fatalf("internal parser rejected SDK response: %v", err)
	}
	if parsed.ID != 44 || parsed.Value.Kind != internal.ValueRecord || parsed.Value.RecordType != "Meta" || len(parsed.Value.Fields) != 2 {
		t.Fatalf("unexpected parsed response: %#v", parsed)
	}
}

func inputWithRequests(t *testing.T, reqs ...internal.Request) *bytes.Buffer {
	t.Helper()
	var in bytes.Buffer
	if err := internal.WriteHandshake(&in); err != nil {
		t.Fatal(err)
	}
	for _, req := range reqs {
		if err := internal.WriteFrame(&in, internal.EncodeRequest(req)); err != nil {
			t.Fatal(err)
		}
	}
	return &in
}

func readOutputResponses(t *testing.T, out *bytes.Buffer, count int) []internal.Response {
	t.Helper()
	if err := internal.ReadHandshake(out); err != nil {
		t.Fatalf("ReadHandshake failed: %v", err)
	}
	responses := make([]internal.Response, 0, count)
	for i := 0; i < count; i++ {
		frame, err := internal.ReadFrame(out)
		if err != nil {
			t.Fatalf("ReadFrame %d failed: %v", i, err)
		}
		resp, err := internal.ParseResponse(frame)
		if err != nil {
			t.Fatalf("ParseResponse %d failed: %v", i, err)
		}
		responses = append(responses, resp)
	}
	return responses
}
