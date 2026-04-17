package interpret

import "testing"

func TestWasmModuleBuilderAppendsCustomSection(t *testing.T) {
	builder := newWasmModuleBuilder()
	builder.appendCustomSection("oct.test", []byte("payload"))
	out := builder.bytesOut()
	if len(out) < 8+1 {
		t.Fatalf("expected wasm header + section, got %d bytes", len(out))
	}
	if out[0] != 0x00 || out[1] != 0x61 || out[2] != 0x73 || out[3] != 0x6d {
		t.Fatalf("missing wasm magic")
	}
	if out[4] != 0x01 || out[5] != 0x00 || out[6] != 0x00 || out[7] != 0x00 {
		t.Fatalf("missing wasm version")
	}
	if out[8] != 0x00 {
		t.Fatalf("expected custom section id=0, got %d", out[8])
	}
}

func TestEncodeULEB128(t *testing.T) {
	cases := map[uint32][]byte{
		0:     {0x00},
		1:     {0x01},
		127:   {0x7f},
		128:   {0x80, 0x01},
		300:   {0xac, 0x02},
		16384: {0x80, 0x80, 0x01},
	}
	for in, expect := range cases {
		got := encodeULEB128(in)
		if len(got) != len(expect) {
			t.Fatalf("uleb %d len mismatch: got=%v expect=%v", in, got, expect)
		}
		for i := range got {
			if got[i] != expect[i] {
				t.Fatalf("uleb %d byte %d mismatch: got=%v expect=%v", in, i, got, expect)
			}
		}
	}
}

func TestEncodeSLEB128(t *testing.T) {
	cases := map[int32][]byte{
		0:    {0x00},
		1:    {0x01},
		-1:   {0x7f},
		127:  {0xff, 0x00},
		128:  {0x80, 0x01},
		8192: {0x80, 0xc0, 0x00},
	}
	for in, expect := range cases {
		got := encodeSLEB128(in)
		if len(got) != len(expect) {
			t.Fatalf("sleb %d len mismatch: got=%v expect=%v", in, got, expect)
		}
		for i := range got {
			if got[i] != expect[i] {
				t.Fatalf("sleb %d byte %d mismatch: got=%v expect=%v", in, i, got, expect)
			}
		}
	}
}
