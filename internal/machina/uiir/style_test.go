package uiir

import (
	"strings"
	"testing"
)

func TestStyleRoundTripsAndUnstyledBytesAreStable(t *testing.T) {
	plain := &Node{Kind: NodeText, Text: "x"}
	plainJSON, err := SerializeCanonicalJSON(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plainJSON, "style") {
		t.Fatalf("unstyled document gained a style key: %s", plainJSON)
	}
	styled := &Node{Kind: NodeText, Text: "x", Style: &StyleSpec{
		Background: RGBA{1, 2, 3, 4}, Foreground: RGBA{5, 6, 7, 8}, BorderColor: RGBA{9, 10, 11, 12}, BorderWidth: 2, Radius: 3,
	}}
	js, err := SerializeCanonicalJSON(styled)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DeserializeCanonicalJSON(js)
	if err != nil {
		t.Fatal(err)
	}
	if back.Style == nil || *back.Style != *styled.Style {
		t.Fatalf("style did not round-trip: %+v", back.Style)
	}
	if Signature(plain) == Signature(styled) {
		t.Fatal("style must be part of the signature")
	}
	if c := Clone(styled); c.Style == styled.Style || *c.Style != *styled.Style {
		t.Fatal("Clone must deep-copy the style")
	}
}
