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
	f3 := filepath.Join(dir, "c.octfail")
	f4 := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f1, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package Main\n[Fact]\nfn test()->Void{return}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f3, []byte("expect error: \"boom\"\n\npackage Main\nfn Main()->Int{return 1kg + 2s}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f4, []byte("no change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("format path: %v", err)
	}
	got1, _ := os.ReadFile(f1)
	got2, _ := os.ReadFile(f2)
	got3, _ := os.ReadFile(f3)
	got4, _ := os.ReadFile(f4)
	mustContain(t, string(got1), "fn main() -> Int")
	mustContain(t, string(got2), "[Fact]")
	mustContain(t, string(got2), "fn test() -> Void")
	mustContain(t, string(got3), "expect error: \"boom\"")
	mustContain(t, string(got3), "fn Main() -> Int")
	if string(got4) != "no change" {
		t.Fatalf("non-oct file changed")
	}
}

func TestFormatPathSingleFileRespectsExtension(t *testing.T) {
	dir := t.TempDir()
	octPath := filepath.Join(dir, "a.oct")
	octestPath := filepath.Join(dir, "b.octest")
	octfailPath := filepath.Join(dir, "c.octfail")

	if err := os.WriteFile(octPath, []byte("package Main\nfn main()->Int{return 1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(octestPath, []byte("package Main\n[Fact]\nfn test()->Void{return}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(octfailPath, []byte("expect error: \"bad\"\n\npackage Main\nfn Main()->Int{return 1kg+2s}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{octPath, octestPath, octfailPath} {
		if err := FormatPath(path); err != nil {
			t.Fatalf("format %s: %v", path, err)
		}
	}

	gotOct, _ := os.ReadFile(octPath)
	gotOctest, _ := os.ReadFile(octestPath)
	gotOctfail, _ := os.ReadFile(octfailPath)
	mustContain(t, string(gotOct), "fn main() -> Int")
	mustContain(t, string(gotOctest), "[Fact]")
	mustContain(t, string(gotOctest), "fn test() -> Void")
	mustContain(t, string(gotOctfail), "expect error: \"bad\"")
	mustContain(t, string(gotOctfail), "fn Main() -> Int")
}

func TestFormatPathDirectoryIdempotentForMixedTypes(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.oct":     "package Main\nfn main() -> Int {\n    return 1\n}\n",
		"b.octest":  "package Main\n[Fact]\nfn test() -> Void {\n    return\n}\n",
		"c.octfail": "expect error: \"boom\"\n\npackage Main\nfn Main() -> Int {\n    return 1kg + 2s\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := FormatPath(dir); err != nil {
		t.Fatalf("first format path: %v", err)
	}

	first := map[string]string{}
	for name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		first[name] = string(data)
	}

	if err := FormatPath(dir); err != nil {
		t.Fatalf("second format path: %v", err)
	}
	for name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if first[name] != string(data) {
			t.Fatalf("expected %s to be stable after second format", name)
		}
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
