package pkgmgr

import "testing"

func TestNormalizeSourceNormalizesWindowsFileURLForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "backslash form",
			input:  `file://C:\Users\oct\repo`,
			output: "file:///C:/Users/oct/repo",
		},
		{
			name:   "host drive form",
			input:  "file://C:/Users/github.com/yuechen-li-dev/oct/repo",
			output: "file:///C:/Users/github.com/yuechen-li-dev/oct/repo",
		},
		{
			name:   "already normalized",
			input:  "file:///C:/Users/github.com/yuechen-li-dev/oct/repo",
			output: "file:///C:/Users/github.com/yuechen-li-dev/oct/repo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeSource(tt.input)
			if err != nil {
				t.Fatalf("normalizeSource(%q) error = %v", tt.input, err)
			}
			if got != tt.output {
				t.Fatalf("normalizeSource(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}
