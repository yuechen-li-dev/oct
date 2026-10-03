package interpret

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/octrandom"
)

// isUnreservedRandomBuiltinCall reports whether callee, as called from package
// pkgName, is a Random or Entropy builtin that builtin.IsName does not report:
// an unqualified name, which resolves only inside the builtin's own package. The package comparison comes first because this runs on every
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

// evalRandomStreamBuiltin executes a Random builtin. All generation is in
// internal/octrandom, which generated programs call as well, so the two lanes
// cannot produce different streams. The typechecker has already checked the
// argument count and types against the builtin table. pkgName is the package
// of the calling code.
func evalRandomStreamBuiltin(random builtin.RandomBuiltin, pkgName string, args []Value) (Value, error) {
	if len(args) != len(random.Parameters) {
		return Value{}, fmt.Errorf("runtime invariant violation: %s expects %d arguments, got %d", random.Name(), len(random.Parameters), len(args))
	}
	if random.Name() == "Random.Seeded" {
		return randomStreamValue(octrandom.Seeded(args[0].Int), pkgName), nil
	}
	stream, err := randomStreamKey(args[0])
	if err != nil {
		return Value{}, err
	}
	var value Value
	switch random.Name() {
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
	switch random.Name() {
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
