package interpret

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/octrandom"
)

// isUnreservedRandomBuiltinCall reports whether callee, as called from package
// pkgName, is a Random or Entropy builtin that builtin.IsName does not report:
// an unqualified non-legacy name, which resolves only inside the builtin's own
// package. The package comparison comes first because this runs on every
// direct call the interpreter evaluates.
func isUnreservedRandomBuiltinCall(callee string, pkgName string) bool {
	if !builtin.IsRandomNamespace(pkgName) {
		return false
	}
	_, ok := builtin.LookupRandomIn(pkgName, callee)
	return ok
}

// randomStreamValue builds the record Stream for code running in package
// pkgName. The interpreter names a record value the way a literal written in
// that package would: bare inside the declaring package and package-qualified
// elsewhere. A builtin-made stream follows the same rule so that it compares
// equal to a hand-written Stream literal with the same key.
func randomStreamValue(key octrandom.Key, pkgName string) Value {
	typeName := "Stream"
	if pkgName != builtin.RandomNamespace {
		typeName = builtin.RandomNamespace + "." + typeName
	}
	return Value{Kind: ValueRecord, Record: RecordValue{
		TypeName:   typeName,
		FieldOrder: []string{"_Key"},
		Fields:     map[string]Value{"_Key": {Kind: ValueInt, Int: int64(key)}},
	}}
}

func randomStreamKey(v Value) (octrandom.Key, error) {
	if v.Kind != ValueRecord {
		return 0, fmt.Errorf("runtime invariant violation: Random stream argument is %s, not a Stream record", v.Kind)
	}
	key, ok := v.Record.Fields["_Key"]
	if !ok || key.Kind != ValueInt {
		return 0, fmt.Errorf("runtime invariant violation: Random stream argument has no Int field _Key")
	}
	return octrandom.Key(uint64(key.Int)), nil
}

// evalRandomStreamBuiltin executes a Random v2 builtin. All generation is in
// internal/octrandom, which generated programs call as well, so the two lanes
// cannot produce different streams. The typechecker has already checked the
// argument count and types against the builtin table. pkgName is the package
// of the calling code.
func evalRandomStreamBuiltin(random builtin.RandomBuiltin, pkgName string, args []Value) (Value, error) {
	if len(args) != len(random.Parameters) {
		return Value{}, fmt.Errorf("runtime invariant violation: %s expects %d arguments, got %d", random.Name(), len(random.Parameters), len(args))
	}
	if random.Implementation() == "Random.Seeded" {
		return randomStreamValue(octrandom.Seeded(args[0].Int), pkgName), nil
	}
	stream, err := randomStreamKey(args[0])
	if err != nil {
		return Value{}, err
	}
	var value Value
	switch random.Implementation() {
	case "Random.Fork":
		value = randomStreamValue(octrandom.Fork(stream, args[1].Text), pkgName)
	case "Random.Child":
		var child octrandom.Key
		child, err = octrandom.Child(stream, args[1].Int)
		value = randomStreamValue(child, pkgName)
	case "Random.Unit":
		value.Kind = ValueFloat
		value.Float, err = octrandom.Unit(stream, args[1].Int)
	case "Random.Between":
		value.Kind = ValueFloat
		value.Float, err = octrandom.Between(stream, args[1].Int, args[2].Float, args[3].Float)
	case "Random.IntBetween":
		value.Kind = ValueInt
		value.Int, err = octrandom.IntBetween(stream, args[1].Int, args[2].Int, args[3].Int)
	case "Random.Normal":
		value.Kind = ValueFloat
		value.Float, err = octrandom.Normal(stream, args[1].Int, args[2].Float, args[3].Float)
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported built-in function %s", random.Name())
	}
	if err != nil {
		return Value{}, fmt.Errorf("runtime error: %w", err)
	}
	return value, nil
}

// isEntropyRandomBuiltin reports whether callee, as called from package
// pkgName, is a builtin that reads ambient operating-system entropy.
func isEntropyRandomBuiltin(callee string, pkgName string) bool {
	random, ok := builtin.ResolveRandomCall(callee, pkgName)
	return ok && random.Kind == builtin.RandomEntropy
}

// evalEntropyBuiltin executes an Entropy builtin. A violated precondition is a
// runtime error, as it is for a Random builtin. A failure of the operating
// system's random source is the only Error the call returns to the program.
func evalEntropyBuiltin(random builtin.RandomBuiltin, args []Value) (evalResult, error) {
	if len(args) != len(random.Parameters) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects %d arguments, got %d", random.Name(), len(random.Parameters), len(args))
	}
	var value Value
	var err error
	switch random.Implementation() {
	case "Entropy.Seed":
		value.Kind = ValueInt
		value.Int, err = octrandom.EntropySeed()
	case "Entropy.IntBetween":
		value.Kind = ValueInt
		value.Int, err = octrandom.EntropyIntBetween(args[0].Int, args[1].Int)
	case "Entropy.Unit":
		value.Kind = ValueFloat
		value.Float, err = octrandom.EntropyUnit()
	case "Entropy.Bytes":
		value.Kind = ValueBytes
		value.Bytes, err = octrandom.EntropyBytes(args[0].Int)
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported built-in function %s", random.Name())
	}
	if err != nil {
		if octrandom.IsPrecondition(err) {
			return evalResult{}, fmt.Errorf("runtime error: %w", err)
		}
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: err.Error()}}}, nil
	}
	return evalResult{value: value}, nil
}

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
