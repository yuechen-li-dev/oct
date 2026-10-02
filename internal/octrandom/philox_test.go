package octrandom

import "testing"

// Known-answer vectors copied verbatim from Random123, tests/kat_vectors
// (https://github.com/DEShawResearch/random123, commit
// 9545ff6413f258be2f04c1d319d99aaef7521150), the three "philox4x32 10" lines:
//
//	philox4x32 10 00000000 00000000 00000000 00000000 00000000 00000000   6627e8d5 e169c58d bc57ac4c 9b00dbd8
//	philox4x32 10 ffffffff ffffffff ffffffff ffffffff ffffffff ffffffff   408f276d 41c83b0e a20bc7c6 6d5451fd
//	philox4x32 10 243f6a88 85a308d3 13198a2e 03707344 a4093822 299f31d0   d16cfe09 94fdcceb 5001e420 24126ea1
//
// Columns are CTR (4 words), KEY (2 words), EXPECTED (4 words).
func TestPhilox4x32KnownAnswers(t *testing.T) {
	cases := []struct {
		name string
		ctr  [4]uint32
		key  [2]uint32
		want [4]uint32
	}{
		{
			name: "zeros",
			ctr:  [4]uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000},
			key:  [2]uint32{0x00000000, 0x00000000},
			want: [4]uint32{0x6627e8d5, 0xe169c58d, 0xbc57ac4c, 0x9b00dbd8},
		},
		{
			name: "ones",
			ctr:  [4]uint32{0xffffffff, 0xffffffff, 0xffffffff, 0xffffffff},
			key:  [2]uint32{0xffffffff, 0xffffffff},
			want: [4]uint32{0x408f276d, 0x41c83b0e, 0xa20bc7c6, 0x6d5451fd},
		},
		{
			name: "pi",
			ctr:  [4]uint32{0x243f6a88, 0x85a308d3, 0x13198a2e, 0x03707344},
			key:  [2]uint32{0xa4093822, 0x299f31d0},
			want: [4]uint32{0xd16cfe09, 0x94fdcceb, 0x5001e420, 0x24126ea1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Philox4x32(c.ctr, c.key)
			if got != c.want {
				t.Fatalf("Philox4x32(%08x, %08x) = %08x, want %08x", c.ctr, c.key, got, c.want)
			}
		})
	}
}

// Philox is a bijection on the counter for a fixed key, so distinct counters
// must give distinct outputs.
func TestPhilox4x32DistinctCountersDistinctOutputs(t *testing.T) {
	key := [2]uint32{0x12345678, 0x9abcdef0}
	seen := make(map[[4]uint32][4]uint32)
	for word := 0; word < 4; word++ {
		for v := uint32(0); v < 2000; v++ {
			var ctr [4]uint32
			ctr[word] = v
			out := Philox4x32(ctr, key)
			if prev, dup := seen[out]; dup && prev != ctr {
				t.Fatalf("counters %08x and %08x collide on output %08x", prev, ctr, out)
			}
			seen[out] = ctr
		}
	}
}
