package octxiliary

import "testing"

func TestRequestRoundTrip(t *testing.T) {
	req := Request{ID: 1, Family: "IO.File", Function: "FileReadText", Path: "/tmp/a"}
	got, _ := ParseRequest(EncodeRequest(req))
	if got.Function != req.Function || got.Path != req.Path {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}
