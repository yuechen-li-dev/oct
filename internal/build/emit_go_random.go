package build

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/builtin"
)

// octrandomImportPath is the package that implements Random and Entropy
// for both lanes. A generated program imports it directly, so the compiled
// lane runs the same Go code as the interpreter rather than an emitted copy.
const octrandomImportPath = "github.com/yuechen-li-dev/oct/internal/octrandom"

// usesRandomStreamBuiltins reports whether the generated program calls any
// Random builtin.
func usesRandomStreamBuiltins(usedBuiltins map[string]bool) bool {
	return usesTableBuiltins(usedBuiltins, builtin.RandomNamespace)
}

// usesEntropyBuiltins reports whether the generated program calls any Entropy
// builtin.
func usesEntropyBuiltins(usedBuiltins map[string]bool) bool {
	return usesTableBuiltins(usedBuiltins, builtin.EntropyNamespace)
}

func usesTableBuiltins(usedBuiltins map[string]bool, namespace string) bool {
	for _, random := range builtin.RandomBuiltins() {
		if random.Namespace == namespace && usedBuiltins[random.Name()] {
			return true
		}
	}
	return false
}

// randomStreamRecordType declares the Go form of the Oct record Random.Stream
// for a program whose reachable records do not already include it.
const randomStreamRecordType = "type Random_Stream struct {\n\t_Key int\n}\n\n"

// randomStreamHelpers adapts the generated program's types to
// internal/octrandom. It contains no generation logic: each function converts
// its arguments, calls the package, and turns a violated precondition into
// the same runtime error the interpreter reports.
const randomStreamHelpers = `
func __octRandomCheck(err error) {
	if err != nil {
		panic("runtime error: " + err.Error())
	}
}
func __octRandomKey(s Random_Stream) octrandom.Key { return octrandom.Key(uint64(s._Key)) }
func __octRandomStream(k octrandom.Key) Random_Stream { return Random_Stream{_Key: int(k)} }
func __octRandomSeeded(seed int) Random_Stream {
	return __octRandomStream(octrandom.Seeded(int64(seed)))
}
func __octRandomFork(s Random_Stream, label string) Random_Stream {
	return __octRandomStream(octrandom.Fork(__octRandomKey(s), label))
}
func __octRandomChild(s Random_Stream, i int) Random_Stream {
	k, err := octrandom.Child(__octRandomKey(s), int64(i))
	__octRandomCheck(err)
	return __octRandomStream(k)
}
func __octRandomUnit(s Random_Stream, i int) float64 {
	v, err := octrandom.Unit(__octRandomKey(s), int64(i))
	__octRandomCheck(err)
	return v
}
func __octRandomBetween(s Random_Stream, i int, lo float64, hi float64) float64 {
	v, err := octrandom.Between(__octRandomKey(s), int64(i), lo, hi)
	__octRandomCheck(err)
	return v
}
func __octRandomIntBetween(s Random_Stream, i int, lo int, hi int) int {
	v, err := octrandom.IntBetween(__octRandomKey(s), int64(i), int64(lo), int64(hi))
	__octRandomCheck(err)
	return int(v)
}
func __octRandomNormal(s Random_Stream, i int, mean float64, stddev float64) float64 {
	v, err := octrandom.Normal(__octRandomKey(s), int64(i), mean, stddev)
	__octRandomCheck(err)
	return v
}
`

// emitRandomStreamCall emits a call to a Random builtin. callee is the
// qualified builtin name, such as "Random.Unit"; the helper it calls is
// "__octRandomUnit".
func emitRandomStreamCall(callee string, target string, args []string) (string, error) {
	random, err := tableBuiltinForEmit(callee, builtin.RandomNamespace, args)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = __octRandom%s(%s)", target, random.Symbol, strings.Join(args, ", ")), nil
}

// entropyHelpers adapts the generated program's types to internal/octrandom,
// as randomStreamHelpers does. A violated precondition is the same runtime
// error the interpreter reports; any other error is a failure of the operating
// system's random source, which the caller returns to the program as an Error.
const entropyHelpers = `
func __octEntropyCheck(err error) {
	if err != nil && octrandom.IsPrecondition(err) {
		panic("runtime error: " + err.Error())
	}
}
func __octEntropySeed() (int, error) {
	v, err := octrandom.EntropySeed()
	return int(v), err
}
func __octEntropyIntBetween(lo int, hi int) (int, error) {
	v, err := octrandom.EntropyIntBetween(int64(lo), int64(hi))
	__octEntropyCheck(err)
	return int(v), err
}
func __octEntropyUnit() (float64, error) {
	return octrandom.EntropyUnit()
}
func __octEntropyBytes(count int) ([]byte, error) {
	v, err := octrandom.EntropyBytes(int64(count))
	__octEntropyCheck(err)
	return v, err
}
`

// emitEntropyCall emits a call to an Entropy builtin. callee is the qualified
// builtin name, such as "Entropy.Seed"; the helper it calls is
// "__octEntropySeed". The call is fallible, so its value is the Go result type
// of the builtin's Oct result type.
func emitEntropyCall(callee string, target string, args []string) (string, error) {
	random, err := tableBuiltinForEmit(callee, builtin.EntropyNamespace, args)
	if err != nil {
		return "", err
	}
	result := goResultTypeName(random.Result)
	return fmt.Sprintf("%s = func() %s { __v, __err := __octEntropy%s(%s); if __err != nil { return %s{Err: __err.Error(), IsErr: true} }; return %s{Value: __v} }()",
		target, result, random.Symbol, strings.Join(args, ", "), result, result), nil
}

func tableBuiltinForEmit(callee string, namespace string, args []string) (builtin.RandomBuiltin, error) {
	random, ok := builtin.LookupRandomQualified(callee)
	if !ok || random.Namespace != namespace {
		return builtin.RandomBuiltin{}, fmt.Errorf("compiled mode does not yet support builtin %s", callee)
	}
	if len(args) != len(random.Parameters) {
		return builtin.RandomBuiltin{}, fmt.Errorf("function '%s' expects %d arguments, got %d", callee, len(random.Parameters), len(args))
	}
	return random, nil
}
