package toolchain

import (
	"strings"
	"testing"
)

func TestHeaderFromWordsDeterministic(t *testing.T) {
	opts := HeaderOptions{
		Symbol:      "k_sdslv_vector_add_spirv",
		SourcePath:  "examples/SDSL-V/M0/VectorAdd.sdslv",
		EntryPoint:  "VectorAdd_CS",
		CommandLine: "oct sdslv generate-header examples/SDSL-V/M0/VectorAdd.sdslv -o out/sdslv/vector_add_spirv.h --symbol k_sdslv_vector_add_spirv",
	}
	first, err := HeaderFromWords([]uint32{0x07230203, 0x00010000, 0x0008000b, 0x0000002a}, opts)
	if err != nil {
		t.Fatalf("HeaderFromWords() error = %v", err)
	}
	second, err := HeaderFromWords([]uint32{0x07230203, 0x00010000, 0x0008000b, 0x0000002a}, opts)
	if err != nil {
		t.Fatalf("HeaderFromWords() second error = %v", err)
	}
	if first != second {
		t.Fatalf("header generation is not deterministic")
	}
	for _, want := range []string{
		"#ifndef OCT_INTERNAL_SDSLV_K_SDSLV_VECTOR_ADD_SPIRV_H",
		"static const uint32_t k_sdslv_vector_add_spirv[] = {",
		"0x07230203u, 0x00010000u, 0x0008000bu, 0x0000002au",
		"static const uint32_t k_sdslv_vector_add_spirv_word_count = 4u;",
		"static const uint32_t k_sdslv_vector_add_spirv_byte_length = 16u;",
		"// Entry point: VectorAdd_CS",
	} {
		if !strings.Contains(first, want) {
			t.Fatalf("header missing %q:\n%s", want, first)
		}
	}
}
