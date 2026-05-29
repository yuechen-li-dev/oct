package octxiliary

import "testing"

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
