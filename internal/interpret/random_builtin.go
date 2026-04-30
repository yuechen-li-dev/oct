package interpret

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
)

func randomNext(s [4]uint64) ([4]uint64, uint64) {
	result := rotl(s[1]*5, 7) * 9
	t := s[1] << 17
	s[2] ^= s[0]
	s[3] ^= s[1]
	s[1] ^= s[2]
	s[0] ^= s[3]
	s[2] ^= t
	s[3] = rotl(s[3], 45)
	return s, result
}
func rotl(x uint64, k int) uint64 { return (x << k) | (x >> (64 - k)) }
func splitMix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
func seedState(seed int64) [4]uint64 {
	x := uint64(seed)
	return [4]uint64{splitMix64(x), splitMix64(x + 1), splitMix64(x + 2), splitMix64(x + 3)}
}
func rngStateFromValue(v Value) ([4]uint64, error) {
	if v.Kind != ValueRecord {
		return [4]uint64{}, fmt.Errorf("runtime error: rng must be Rng")
	}
	f := v.Record.Fields
	return [4]uint64{uint64(f["_State0"].Int), uint64(f["_State1"].Int), uint64(f["_State2"].Int), uint64(f["_State3"].Int)}, nil
}
func rngValueFromState(s [4]uint64) Value {
	return Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Random.Rng", FieldOrder: []string{"_State0", "_State1", "_State2", "_State3"}, Fields: map[string]Value{"_State0": {Kind: ValueInt, Int: int64(s[0])}, "_State1": {Kind: ValueInt, Int: int64(s[1])}, "_State2": {Kind: ValueInt, Int: int64(s[2])}, "_State3": {Kind: ValueInt, Int: int64(s[3])}}}}
}
func toFloat01(x uint64) float64 { return float64(x>>11) * (1.0 / (1 << 53)) }
func cryptoU64() (uint64, error) {
	var b [8]byte
	_, e := rand.Read(b[:])
	if e != nil {
		return 0, e
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}
func cryptoReadBytes(dst []byte) error {
	_, err := rand.Read(dst)
	return err
}
func cryptoInt(min, max int64) (int64, error) {
	if min > max {
		return 0, fmt.Errorf("runtime error: min must be <= max")
	}
	span := max - min + 1
	n, err := rand.Int(rand.Reader, big.NewInt(span))
	if err != nil {
		return 0, err
	}
	return min + n.Int64(), nil
}
func normalFromPair(u1, u2 float64) float64 {
	r := math.Sqrt(-2 * math.Log(u1))
	return r * math.Cos(2*math.Pi*u2)
}
func randomIntResultValue(next Value, value int64) Value {
	return Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Random.RandIntResult", FieldOrder: []string{"Next", "Value"}, Fields: map[string]Value{"Next": next, "Value": {Kind: ValueInt, Int: value}}}}
}
func randomFloatResultValue(next Value, value float64) Value {
	return Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Random.RandFloatResult", FieldOrder: []string{"Next", "Value"}, Fields: map[string]Value{"Next": next, "Value": {Kind: ValueFloat, Float: value}}}}
}
func randomBoolResultValue(next Value, value bool) Value {
	return Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Random.RandBoolResult", FieldOrder: []string{"Next", "Value"}, Fields: map[string]Value{"Next": next, "Value": {Kind: ValueBool, Bool: value}}}}
}
