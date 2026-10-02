package octrandom

const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// FNV1a64 hashes the bytes of s (its UTF-8 encoding, for an Oct String) with
// 64-bit FNV-1a. It is the label hash for Fork and is part of the Random v2
// specification, so it must never change.
func FNV1a64(s string) uint64 {
	h := fnvOffset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}
