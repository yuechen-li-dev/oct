package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSdslvTestInputArchitectureGuards(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		path      string
		wants     []string
		forbidden []string
	}{
		{
			path: filepath.Join(root, "internal", "sdslv", "emit", "hlsl", "hlsl.go"),
			wants: []string{
				"target.Name != vdmir.TestInputResourceName",
			},
			forbidden: []string{
				"emitTestGuardedRead",
				"guardedReadForTestInput",
			},
		},
		{
			path: filepath.Join(root, "internal", "sdslv", "lower", "lower.go"),
			wants: []string{
				"Name:        vdmir.TestInputResourceName",
			},
			forbidden: []string{
				"TestInputCall",
				"localTestInputArray",
			},
		},
		{
			path: filepath.Join(root, "internal", "sdslv", "ast", "ast.go"),
			forbidden: []string{
				"TestInputDecl",
				"DescriptorBindingDecl",
				"DescriptorSetDecl",
			},
		},
		{
			path: filepath.Join(root, "internal", "sdslv", "parse", "parse.go"),
			forbidden: []string{
				"parseTestInputResource",
				"parseDescriptorBinding",
				"parseDescriptorSet",
			},
		},
		{
			path: filepath.Join(root, "internal", "sdslv", "test", "compile.go"),
			forbidden: []string{
				"Compile(manifest",
				"emitTestGuardedRead",
			},
		},
		{
			path: filepath.Join(root, "internal", "sdslv", "vdmir", "vdmir.go"),
			wants: []string{
				"const TestInputResourceName = \"__sdslv_test_input\"",
			},
			forbidden: []string{
				"VkBuffer",
				"VkDevice",
				"Vulkan",
				"manifest.json",
				"DescriptorPool",
			},
		},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range tc.wants {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", tc.path, want)
			}
		}
		for _, forbidden := range tc.forbidden {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s retains forbidden pattern %q", tc.path, forbidden)
			}
		}
	}
}

func TestSdslvTestInputUsesHiddenResourceNotLocalArraySubstitution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.sdslvtest")
	src := "[Fact]\n[TestInputFloat(-0.0, 1.5)]\nfn ReadInput() -> void { let x: f32 = read TestInput.Float[0u] when 0u < TestInput.Length else 1.0; let bits: u32 = HLSL<u32>(x) { return asuint(x); }; Assert.Equal(2147483648u, bits); }\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	suite, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Compile(suite, filepath.Join(t.TempDir(), "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	hlsl, err := os.ReadFile(groups[0].HLSLPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(hlsl)
	for _, want := range []string{
		"StructuredBuffer<uint> __sdslv_test_input;",
		"asfloat(__sdslv_test_input[0u])",
		"asuint(x)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated HLSL missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{
		"static const uint __sdslv_test_input",
		"uint __sdslv_test_input_local[",
		"TestInput(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generated HLSL retained forbidden substitution %q:\n%s", forbidden, text)
		}
	}
}
