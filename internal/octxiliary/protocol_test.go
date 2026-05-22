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
