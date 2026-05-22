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
