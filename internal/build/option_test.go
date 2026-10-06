package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
)

func TestParseOptionType(t *testing.T) {
	cases := []struct {
		input   string
		payload string
		ok      bool
	}{
		{"Option<Int>", "Int", true},
		{"Option<Float<m/s>>", "Float<m/s>", true},
		{"Option<Main.Reading>", "Main.Reading", true},
		{"Option<Int[]>", "Int[]", true},
		{"Option<Option<Int>>", "Option<Int>", true},
		{"Option<Option<Int>[]>", "Option<Int>[]", true},
		{"Option<fn(Int) -> Option<Int>>", "fn(Int) -> Option<Int>", true},
		{"Option<Vector<Float>>", "Vector<Float>", true},
		{"Option<Int>[]", "", false},
		{"Option<Int>, Option<Int>", "", false},
		{"Option", "", false},
		{"Main.Option<Int>", "", false},
		{"fn(Option<Int>) -> Option<Int>", "", false},
		{"Vector<Option<Int>>", "", false},
	}
	for _, tc := range cases {
		payload, ok := parseOptionType(tc.input)
		if ok != tc.ok || payload != tc.payload {
			t.Errorf("parseOptionType(%q) = %q, %v; want %q, %v", tc.input, payload, ok, tc.payload, tc.ok)
		}
	}
}

func TestGoTypeOfAnOption(t *testing.T) {
	cases := map[string]string{
		"Option<Int>":            "__octOption_Int",
		"Option<Float<m>>":       "__octOption_Float_3cm_3e",
		"Option<Main.Reading>[]": "[]__octOption_Main_2eReading",
		"Option<Option<String>>": "__octOption_Option_3cString_3e",
		"Option<Int[]>":          "__octOption_Int_5b_5d",
		"Option<My_Pkg.A_b>":     "__octOption_My_5fPkg_2eA_5fb",
	}
	for input, want := range cases {
		if got := goType(input); got != want {
			t.Errorf("goType(%q) = %q, want %q", input, got, want)
		}
	}
}

// The Go name of an option type spells its payload type, and the emitter
// reads it back to declare the type.
func TestOptionGoTypeNamesReadBack(t *testing.T) {
	for _, payload := range []string{"Int", "Float<m/s^2>", "Main.Reading", "Int[][]", "Option<Option<Bool>>", "fn(Int, My_Pkg.A_b) -> Float ! Error", "Vector<Float<m>>"} {
		got, ok := optionPayloadOfGoType(goOptionType(payload))
		if !ok || got != payload {
			t.Errorf("payload %q read back as %q, %v", payload, got, ok)
		}
	}
	for _, name := range []string{"__octOption_", "__octOption_Int_", "__octOption_Int_5", "__octOption_Int_zz", "__octOptionEqual", "Main_Reading"} {
		if payload, ok := optionPayloadOfGoType(name); ok {
			t.Errorf("optionPayloadOfGoType(%q) = %q, want no payload", name, payload)
		}
	}
}

func TestAppendOptionDeclarations(t *testing.T) {
	plain := "package main\n\nfunc main() {}\n"
	if got := appendOptionDeclarations(plain); got != plain {
		t.Fatalf("a program that names no option was changed:\n%s", got)
	}
	outer := goOptionType("Option<Int>")
	inner := goOptionType("Int")
	src := "package main\n\nvar x " + outer + "\n\nfunc __octEnumMetaOf() {}\n"
	got := appendOptionDeclarations(src)
	for _, want := range []string{
		"type " + outer + " struct {",
		"type " + inner + " struct {",
		"func (" + outer + ") __octOptionPayloadType() reflect.Type {\n\treturn reflect.TypeOf((*" + inner + ")(nil)).Elem()",
		"func (" + inner + ") __octOptionPayloadType() reflect.Type {\n\treturn reflect.TypeOf((*int)(nil)).Elem()",
		"Option_Some_tag = 1",
		"func __octOptionEqual(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("declarations lack %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "type "+inner+" struct {") != 1 || strings.Count(got, "Option_Some_tag = 1") != 1 {
		t.Errorf("a declaration was written more than once:\n%s", got)
	}
	if again := appendOptionDeclarations(src); again != got {
		t.Error("the declarations are not written in one order every time")
	}
	withoutOctagon := appendOptionDeclarations("package main\n\nvar x " + inner + "\n")
	if strings.Contains(withoutOctagon, "__octOptionPayloadType") {
		t.Errorf("a program without the Octagon runtime got its payload-type method:\n%s", withoutOctagon)
	}
}

func TestOptionVariantPayloadTypes(t *testing.T) {
	program := project.Program{Packages: map[string]project.Package{"Main": {Name: "Main"}}}
	if payload, ok := lookupEnumVariantPayloadTypeForProgram(program, "Main", "Option<Float<m>>", "Some"); !ok || payload != "Float<m>" {
		t.Errorf("Some of Option<Float<m>> has payload %q, %v", payload, ok)
	}
	if _, ok := lookupEnumVariantPayloadTypeForProgram(program, "Main", "Option<Float<m>>", "None"); ok {
		t.Error("None of an option was given a payload type")
	}
}

// Lowering reads the type argument of `Option.Some(value)` where the
// typechecker wrote it, and refuses a program that was not typechecked.
func TestLoweringRefusesAnOptionTheTypecheckerDidNotResolve(t *testing.T) {
	dir := t.TempDir()
	source := `package Main

fn Main() -> Int {
    let level: Option<Float> = Option.Some(1)
    return 0
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.oct"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = lowerProgram(program, compileOptions{})
	if err == nil || !strings.Contains(err.Error(), "`Option.Some` reached lowering without the type the typechecker gives it") {
		t.Fatalf("lowerProgram on an unchecked program: %v", err)
	}
}
