package build

import "github.com/yuechen-li-dev/oct/internal/builtin"

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
