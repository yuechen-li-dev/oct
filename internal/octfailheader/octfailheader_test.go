package octfailheader

import (
	"strings"
	"testing"
)

func TestSplitReadsTheExpectationBlock(t *testing.T) {
	cases := []struct {
		content     string
		phase       Phase
		texts       string
		source      string
		errContains string
	}{
		{"expect error: \"a\"\nbody\n", Compile, "a", "body\n", ""},
		{"\n\nexpect runtime error: \"b c\"\nbody\n", Runtime, "b c", "body\n", ""},
		{"expect artifact error: \"d\"\n\nbody\n", Artifact, "d", "\nbody\n", ""},
		{"expect error: \"a\"\nexpect error: \"b \"quoted\"\"\n\nbody\n", Compile, "a|b \"quoted\"", "\nbody\n", ""},
		{"expect runtime error: \"a\"\n  expect runtime error: \"b\"  \nbody\n", Runtime, "a|b", "body\n", ""},
		{"expect runtime error: \"\"\nbody\n", "", "", "", "non-empty"},
		{"expect warning: \"a\"\nbody\n", "", "", "", "malformed expectation header"},
		{"body\n", "", "", "", "malformed expectation header"},
		{"expect error: \"a\"\nexpect error: b\nbody\n", "", "", "", "malformed expectation header"},
		{"expect error: \"a\"\nexpect runtime error: \"b\"\n", "", "", "", "must all name the same phase"},
		{"expect artifact error: \"a\"\nexpect error: \"b\"\n", "", "", "", "must all name the same phase"},
		{"expect error: \"a\"\nbody\nexpect error: \"b\"\n", "", "", "", "must come first"},
		{"expect error: \"a\"\n\nexpect error: \"b\"\nbody\n", "", "", "", "must come first"},
		{"\n \n", "", "", "", "missing expectation header"},
	}
	for _, c := range cases {
		header, source, err := Split(c.content)
		if c.errContains != "" {
			if err == nil || !strings.Contains(err.Error(), c.errContains) {
				t.Errorf("%q: err = %v, want one containing %q", c.content, err, c.errContains)
			}
			continue
		}
		if err != nil || header.Phase != c.phase || strings.Join(header.Texts, "|") != c.texts || source != c.source {
			t.Errorf("%q: got (%q, %q, %q, %v)", c.content, header.Phase, header.Texts, source, err)
		}
	}
}
