package octrandom

import "math/bits"

// Philox4x32 constants from Salmon, Moraes, Dror and Shaw, "Parallel Random
// Numbers: As Easy as 1, 2, 3" (SC'11), as published in Random123
// (include/Random123/philox.h).
const (
	philoxM0 uint32 = 0xD2511F53
	philoxM1 uint32 = 0xCD9E8D57
	philoxW0 uint32 = 0x9E3779B9
	philoxW1 uint32 = 0xBB67AE85

	philoxRounds = 10
)

// Philox4x32 applies the ten-round Philox4x32 bijection to ctr under key.
func Philox4x32(ctr [4]uint32, key [2]uint32) [4]uint32 {
	for round := 0; round < philoxRounds; round++ {
		if round > 0 {
			key[0] += philoxW0
			key[1] += philoxW1
		}
		hi0, lo0 := bits.Mul32(philoxM0, ctr[0])
		hi1, lo1 := bits.Mul32(philoxM1, ctr[2])
		ctr = [4]uint32{
			hi1 ^ ctr[1] ^ key[0],
			lo1,
			hi0 ^ ctr[3] ^ key[1],
			lo0,
		}
	}
	return ctr
}
