package build

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/builtin"
)

// usesRandomHelpers reports whether the generated program calls a Random
// builtin that needs the emitted Random helper block, its record types and
// the Go imports they use. Draws and entropy reads need them. Calls are
// recorded under the name of the implementing builtin, so only table entries
// with their own implementation are consulted.
func usesRandomHelpers(usedBuiltins map[string]bool) bool {
	for _, random := range builtin.RandomBuiltins() {
		if !random.HasOwnImplementation() {
			continue
		}
		if random.Kind != builtin.RandomDraw && random.Kind != builtin.RandomEntropy {
			continue
		}
		if usedBuiltins[random.Name()] {
			return true
		}
	}
	return false
}

// isRandomDrawImplementation reports whether callee is the compiled name of a
// Random draw. A draw returns a result record, so it cannot be the callee of a
// destructuring call.
func isRandomDrawImplementation(callee string) bool {
	random, ok := builtin.LookupRandomQualified(callee)
	return ok && random.Kind == builtin.RandomDraw && random.HasOwnImplementation()
}

// octrandomImportPath is the package that implements Random v2 for both
// lanes. A generated program imports it directly, so the compiled lane runs
// the same Go code as the interpreter rather than an emitted copy.
const octrandomImportPath = "github.com/yuechen-li-dev/oct/internal/octrandom"

// usesRandomStreamBuiltins reports whether the generated program calls any
// Random v2 builtin.
func usesRandomStreamBuiltins(usedBuiltins map[string]bool) bool {
	for _, random := range builtin.RandomBuiltins() {
		if !random.Legacy && usedBuiltins[random.Name()] {
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

// emitRandomStreamCall emits a call to a Random v2 builtin. callee is the
// qualified builtin name, such as "Random.Unit"; the helper it calls is
// "__octRandomUnit".
func emitRandomStreamCall(callee string, target string, args []string) (string, error) {
	random, ok := builtin.LookupRandomQualified(callee)
	if !ok || random.Legacy {
		return "", fmt.Errorf("compiled mode does not yet support builtin %s", callee)
	}
	if len(args) != len(random.Parameters) {
		return "", fmt.Errorf("function '%s' expects %d arguments, got %d", callee, len(random.Parameters), len(args))
	}
	return fmt.Sprintf("%s = __octRandom%s(%s)", target, random.Symbol, strings.Join(args, ", ")), nil
}
