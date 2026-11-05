// Package syntax shares parsing mechanics between Oct language profiles.
// Each profile owns its tokens, precedence rules, operands, and AST.
package syntax

// ParseBinary parses left-associative binary operators by precedence climbing.
// Operator reports the current operator without consuming it. Returning false
// also lets a profile stop at a context-dependent delimiter such as '>'.
func ParseBinary[E any](minimum int, operand func() (E, error), operator func() (string, int, bool), advance func(), combine func(E, string, E) E) (E, error) {
	left, err := operand()
	if err != nil {
		return left, err
	}
	for {
		op, precedence, ok := operator()
		if !ok || precedence < minimum {
			return left, nil
		}
		advance()
		right, err := ParseBinary(precedence+1, operand, operator, advance, combine)
		if err != nil {
			return right, err
		}
		left = combine(left, op, right)
	}
}
