package ocfmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSourceIdempotent(t *testing.T) {
	input := "package Main\n\nfn main()->Int{\n    let x=1+2\n    return x\n}\n"
	first, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format first: %v", err)
	}
	second, err := FormatSource(first)
	if err != nil {
		t.Fatalf("format second: %v", err)
	}
	if first != second {
		t.Fatalf("format should be idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestFormatSourceConvergence(t *testing.T) {
	left := "package Main\nfn sum(a:Int,b:Int)->Int{\nreturn a+b\n}\n"
	right := "package   Main\nfn sum( a : Int, b : Int ) -> Int {\n    return a + b\n}\n"
	outLeft, err := FormatSource(left)
	if err != nil {
		t.Fatalf("format left: %v", err)
	}
	outRight, err := FormatSource(right)
	if err != nil {
		t.Fatalf("format right: %v", err)
	}
	if outLeft != outRight {
		t.Fatalf("expected convergence\nleft:\n%s\nright:\n%s", outLeft, outRight)
	}
}

func TestFormatSourcePreservesComments(t *testing.T) {
	input := "package Main\n\n///   Adds values\nfn add(a:Int,b:Int)->Int{ // inline comment\n// ordinary comment\nreturn a+b\n}\n"
	out, err := FormatSource(input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	mustContain(t, out, "///   Adds values")
	mustContain(t, out, "// inline comment")
	mustContain(t, out, "// ordinary comment")
}

func TestFormatPathDirectory(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.oct")
	f2 := filepath.Join(dir, "b.octest")
	f3 := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(f1, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package Main\nfn test()->Int{return 2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f3, []byte("no change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("format path: %v", err)
	}
	got1, _ := os.ReadFile(f1)
	got2, _ := os.ReadFile(f2)
	got3, _ := os.ReadFile(f3)
	mustContain(t, string(got1), "fn main() -> Int")
	mustContain(t, string(got2), "fn test() -> Int")
	if string(got3) != "no change" {
		t.Fatalf("non-oct file changed")
	}
}

func TestFormatSourceRejectsInvalidSource(t *testing.T) {
	if _, err := FormatSource("package Main\nfn bad( {\n"); err == nil {
		t.Fatalf("expected parse/lex error")
	}
}

func mustContain(t *testing.T, s string, sub string) {
	t.Helper()
	if !contains(s, sub) {
		t.Fatalf("expected output to contain %q\n%s", sub, s)
	}
}

func contains(s string, sub string) bool {
	return strings.Contains(s, sub)
}
