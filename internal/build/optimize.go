package build

import (
	"fmt"
	"math"
)

type OptimizationStats struct {
	Iterations           int
	ValuesFolded         int
	LocalReadsPropagated int
	BranchesFolded       int
}

// OptimizeMIR runs the bounded Chapter 4 scalar pipeline to stability. It
// returns a rewritten module and never mutates its input.
func OptimizeMIR(module MIRModule) (MIRModule, OptimizationStats, error) {
	optimized := module
	optimized.Functions = make([]MIRFunction, len(module.Functions))
	stats := OptimizationStats{}
	for i, fn := range module.Functions {
		result, fnStats, err := OptimizeFunction(fn)
		if err != nil {
			return MIRModule{}, stats, err
		}
		optimized.Functions[i] = result
		stats.ValuesFolded += fnStats.ValuesFolded
		stats.LocalReadsPropagated += fnStats.LocalReadsPropagated
		stats.BranchesFolded += fnStats.BranchesFolded
		if fnStats.Iterations > stats.Iterations {
			stats.Iterations = fnStats.Iterations
		}
	}
	return optimized, stats, nil
}

// OptimizeFunction composes folding, analysis, propagation, folding, and
// constant-branch rewriting. Thirty-two iterations is a defensive bound; the
// implemented rewrites only remove local reads/operators/branches and normally
// converge in two iterations.
func OptimizeFunction(fn MIRFunction) (MIRFunction, OptimizationStats, error) {
	current := cloneMIRFunction(fn)
	stats := OptimizationStats{}
	for iteration := 1; iteration <= 32; iteration++ {
		changed := false
		var count int
		current, count = foldFunction(current)
		stats.ValuesFolded += count
		changed = changed || count > 0

		analysis, err := AnalyzeConstants(current)
		if err != nil {
			return MIRFunction{}, stats, err
		}
		current, count = rewriteFunctionConstants(current, analysis)
		stats.LocalReadsPropagated += count
		changed = changed || count > 0

		current, count = foldFunction(current)
		stats.ValuesFolded += count
		changed = changed || count > 0

		current, count = foldConstantBranches(current)
		stats.BranchesFolded += count
		changed = changed || count > 0
		stats.Iterations = iteration
		if !changed {
			return current, stats, nil
		}
	}
	return MIRFunction{}, stats, fmt.Errorf("constant optimizer did not converge for %s.%s within 32 iterations", fn.Package, fn.Name)
}

// FoldValue recursively folds scalar children before their parent.
func FoldValue(value MIRValue) (MIRValue, bool) {
	return foldValueWithConstants(value, nil)
}

func foldValueWithConstants(value MIRValue, constants LocalConstantMap) (MIRValue, bool) {
	if value == nil {
		return nil, false
	}
	if local, ok := value.(MIRLocal); ok && constants != nil {
		if fact := constants[local.Name]; fact.Kind == ConstantKnown {
			return constantLiteral(fact.Constant), true
		}
	}
	changed := false
	fold := func(child MIRValue) MIRValue {
		result, childChanged := foldValueWithConstants(child, constants)
		changed = changed || childChanged
		return result
	}
	switch v := value.(type) {
	case MIRUnary:
		v.Value = fold(v.Value)
		if operand, ok := v.Value.(MIRLiteral); ok {
			if result, ok := foldUnary(v.Op, operand); ok {
				return result, true
			}
		}
		return v, changed
	case MIRBinary:
		v.Left, v.Right = fold(v.Left), fold(v.Right)
		left, leftOK := v.Left.(MIRLiteral)
		right, rightOK := v.Right.(MIRLiteral)
		if leftOK && rightOK {
			if result, ok := foldBinary(v.Op, left, right); ok {
				return result, true
			}
		}
		return v, changed
	case MIRConvert:
		v.Value = fold(v.Value)
		if literal, ok := v.Value.(MIRLiteral); ok {
			if result, ok := foldConversion(v.TargetType, literal); ok {
				return result, true
			}
		}
		return v, changed
	case MIRIndex:
		v.Target, v.Index = fold(v.Target), fold(v.Index)
		return v, changed
	case MIRFieldAccess:
		v.Target = fold(v.Target)
		return v, changed
	case MIRClone:
		v.Value = fold(v.Value)
		return v, changed
	case MIRRangeValue:
		v.Start, v.End, v.Step = fold(v.Start), fold(v.End), fold(v.Step)
		return v, changed
	case MIRArrayConvert:
		v.Value = fold(v.Value)
		return v, changed
	case MIRLength:
		v.Value = fold(v.Value)
		return v, changed
	case MIRMatrixColumnCount:
		v.Value = fold(v.Value)
		return v, changed
	case MIRResultValue:
		v.Value, v.Error = fold(v.Value), fold(v.Error)
		return v, changed
	case MIREnumValue:
		v.Payload = fold(v.Payload)
		return v, changed
	case MIREnumPayload:
		v.Value = fold(v.Value)
		return v, changed
	case MIRIntrinsicValue:
		for i := range v.Args {
			v.Args[i] = fold(v.Args[i])
		}
		if result, ok := foldIntrinsic(v); ok {
			return result, true
		}
		return v, changed
	default:
		return value, changed
	}
}

func foldUnary(op string, literal MIRLiteral) (MIRLiteral, bool) {
	value, ok := constantFromLiteral(literal)
	if !ok {
		return MIRLiteral{}, false
	}
	switch {
	case value.Kind == "Bool" && (op == "!" || op == "not"):
		value.Bool = !value.Bool
	case value.Kind == "Int" && op == "-":
		value.Int = -value.Int
	case value.Kind == "Float" && op == "-":
		value.Float = -value.Float
	default:
		return MIRLiteral{}, false
	}
	return constantLiteral(value), true
}

func foldBinary(op string, leftLiteral, rightLiteral MIRLiteral) (MIRLiteral, bool) {
	left, leftOK := constantFromLiteral(leftLiteral)
	right, rightOK := constantFromLiteral(rightLiteral)
	if !leftOK || !rightOK || left.Kind != right.Kind {
		return MIRLiteral{}, false
	}
	if left.Kind == "Int" {
		switch op {
		case "+":
			left.Int += right.Int
		case "-":
			left.Int -= right.Int
		case "*":
			left.Int *= right.Int
		case "==":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int == right.Int}), true
		case "!=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int != right.Int}), true
		case "<":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int < right.Int}), true
		case ">":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int > right.Int}), true
		case "<=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int <= right.Int}), true
		case ">=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Int >= right.Int}), true
		default:
			return MIRLiteral{}, false
		}
		return constantLiteral(left), true
	}
	if left.Kind == "Float" {
		result := left.Float
		switch op {
		case "+":
			result += right.Float
		case "-":
			result -= right.Float
		case "*":
			result *= right.Float
		case "==":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float == right.Float}), true
		case "!=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float != right.Float}), true
		case "<":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float < right.Float}), true
		case ">":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float > right.Float}), true
		case "<=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float <= right.Float}), true
		case ">=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Float >= right.Float}), true
		default:
			return MIRLiteral{}, false
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return MIRLiteral{}, false
		}
		return constantLiteral(Constant{Kind: "Float", Float: result}), true
	}
	if left.Kind == "Bool" {
		switch op {
		case "&&", "and":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Bool && right.Bool}), true
		case "||", "or":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Bool || right.Bool}), true
		case "==":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Bool == right.Bool}), true
		case "!=":
			return constantLiteral(Constant{Kind: "Bool", Bool: left.Bool != right.Bool}), true
		}
	}
	return MIRLiteral{}, false
}

func foldConversion(target string, literal MIRLiteral) (MIRLiteral, bool) {
	value, ok := constantFromLiteral(literal)
	if !ok {
		return MIRLiteral{}, false
	}
	if target == value.Kind {
		return literal, true
	}
	if value.Kind == "Int" && target == "Float" {
		return constantLiteral(Constant{Kind: "Float", Float: float64(value.Int)}), true
	}
	if value.Kind == "Float" && target == "Int" && value.Float >= -9223372036854775808.0 && value.Float < 9223372036854775808.0 {
		return constantLiteral(Constant{Kind: "Int", Int: int64(value.Float)}), true
	}
	return MIRLiteral{}, false
}

func foldIntrinsic(value MIRIntrinsicValue) (MIRLiteral, bool) {
	if len(value.Args) != 2 {
		return MIRLiteral{}, false
	}
	leftLiteral, leftOK := value.Args[0].(MIRLiteral)
	rightLiteral, rightOK := value.Args[1].(MIRLiteral)
	if !leftOK || !rightOK {
		return MIRLiteral{}, false
	}
	left, leftOK := constantFromLiteral(leftLiteral)
	right, rightOK := constantFromLiteral(rightLiteral)
	if !leftOK || !rightOK || left.Kind != right.Kind {
		return MIRLiteral{}, false
	}
	switch value.Kind {
	case "safe-divide":
		if left.Kind == "Int" && right.Int != 0 && !(left.Int == math.MinInt64 && right.Int == -1) {
			return constantLiteral(Constant{Kind: "Int", Int: left.Int / right.Int}), true
		}
		if left.Kind == "Float" && right.Float != 0 {
			result := left.Float / right.Float
			if !math.IsNaN(result) && !math.IsInf(result, 0) {
				return constantLiteral(Constant{Kind: "Float", Float: result}), true
			}
		}
	case "euclidean-modulo":
		if left.Kind == "Int" && right.Int != 0 {
			result := left.Int % right.Int
			if result < 0 {
				if right.Int > 0 {
					result += right.Int
				} else {
					result -= right.Int
				}
			}
			return constantLiteral(Constant{Kind: "Int", Int: result}), true
		}
	}
	return MIRLiteral{}, false
}

func cloneMIRFunction(fn MIRFunction) MIRFunction {
	result := fn
	result.Blocks = make([]MIRBlock, len(fn.Blocks))
	for i, block := range fn.Blocks {
		result.Blocks[i] = block
		result.Blocks[i].Statements = append([]MIRStmt(nil), block.Statements...)
	}
	return result
}

func foldFunction(fn MIRFunction) (MIRFunction, int) {
	result := cloneMIRFunction(fn)
	count := 0
	for bi, block := range result.Blocks {
		for si, stmt := range block.Statements {
			result.Blocks[bi].Statements[si], count = mapStatementValues(stmt, nil, count)
		}
		result.Blocks[bi].Terminator, count = mapTerminatorValues(block.Terminator, nil, count)
	}
	return result, count
}

func rewriteFunctionConstants(fn MIRFunction, analysis ConstantAnalysis) (MIRFunction, int) {
	result := cloneMIRFunction(fn)
	count := 0
	for bi, block := range result.Blocks {
		before, reachable := analysis.BeforeStatement[block.Label]
		if !reachable {
			continue
		}
		for si, stmt := range block.Statements {
			result.Blocks[bi].Statements[si], count = mapStatementValues(stmt, before[si], count)
		}
		result.Blocks[bi].Terminator, count = mapTerminatorValues(block.Terminator, before[len(block.Statements)], count)
	}
	return result, count
}

func mapOneValue(value MIRValue, constants LocalConstantMap, count int) (MIRValue, int) {
	var changed bool
	if constants == nil {
		value, changed = FoldValue(value)
	} else {
		value, changed = substituteConstants(value, constants)
	}
	if changed {
		count++
	}
	return value, count
}

func substituteConstants(value MIRValue, constants LocalConstantMap) (MIRValue, bool) {
	if value == nil {
		return nil, false
	}
	changed := false
	mapChild := func(child MIRValue) MIRValue {
		var c bool
		child, c = substituteConstants(child, constants)
		changed = changed || c
		return child
	}
	switch v := value.(type) {
	case MIRLocal:
		if fact := constants[v.Name]; fact.Kind == ConstantKnown {
			return constantLiteral(fact.Constant), true
		}
	case MIRUnary:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRBinary:
		v.Left, v.Right = mapChild(v.Left), mapChild(v.Right)
		return v, changed
	case MIRConvert:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRIndex:
		v.Target, v.Index = mapChild(v.Target), mapChild(v.Index)
		return v, changed
	case MIRFieldAccess:
		v.Target = mapChild(v.Target)
		return v, changed
	case MIRClone:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRRangeValue:
		v.Start, v.End, v.Step = mapChild(v.Start), mapChild(v.End), mapChild(v.Step)
		return v, changed
	case MIRArrayConvert:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRLength:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRMatrixColumnCount:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRResultValue:
		v.Value, v.Error = mapChild(v.Value), mapChild(v.Error)
		return v, changed
	case MIREnumValue:
		v.Payload = mapChild(v.Payload)
		return v, changed
	case MIREnumPayload:
		v.Value = mapChild(v.Value)
		return v, changed
	case MIRIntrinsicValue:
		for i := range v.Args {
			v.Args[i] = mapChild(v.Args[i])
		}
		return v, changed
	}
	return value, changed
}

func mapStatementValues(stmt MIRStmt, constants LocalConstantMap, count int) (MIRStmt, int) {
	mapValue := func(value MIRValue) MIRValue { value, count = mapOneValue(value, constants, count); return value }
	switch s := stmt.(type) {
	case MIRAssign:
		s.Value = mapValue(s.Value)
		return s, count
	case MIRRowAssign:
		s.Index, s.Value = mapValue(s.Index), mapValue(s.Value)
		return s, count
	case MIRIndexAssign:
		for i := range s.Indices {
			s.Indices[i] = mapValue(s.Indices[i])
		}
		s.Value = mapValue(s.Value)
		return s, count
	case MIRCall:
		for i := range s.Args {
			s.Args[i] = mapValue(s.Args[i])
		}
		return s, count
	case MIRGenericOctxiliaryCall:
		for i := range s.Args {
			s.Args[i] = mapValue(s.Args[i])
		}
		return s, count
	case MIRDestructureCall:
		for i := range s.Args {
			s.Args[i] = mapValue(s.Args[i])
		}
		return s, count
	case MIRConstructRecord:
		for i := range s.FieldVals {
			s.FieldVals[i] = mapValue(s.FieldVals[i])
		}
		return s, count
	case MIRConstructArray:
		for i := range s.Values {
			s.Values[i] = mapValue(s.Values[i])
		}
		return s, count
	case MIRBatchMap:
		s.Input = mapValue(s.Input)
		for i := range s.Captures {
			s.Captures[i] = mapValue(s.Captures[i])
		}
		return s, count
	default:
		return stmt, count
	}
}

func mapTerminatorValues(term MIRTerminator, constants LocalConstantMap, count int) (MIRTerminator, int) {
	switch t := term.(type) {
	case MIRReturn:
		t.Value, count = mapOneValue(t.Value, constants, count)
		return t, count
	case MIRBranch:
		t.Cond, count = mapOneValue(t.Cond, constants, count)
		return t, count
	case MIRFail:
		t.Value, count = mapOneValue(t.Value, constants, count)
		return t, count
	default:
		return term, count
	}
}

func foldConstantBranches(fn MIRFunction) (MIRFunction, int) {
	result := cloneMIRFunction(fn)
	count := 0
	for i, block := range result.Blocks {
		branch, ok := block.Terminator.(MIRBranch)
		if !ok {
			continue
		}
		literal, ok := branch.Cond.(MIRLiteral)
		if !ok || literal.Type != "Bool" {
			continue
		}
		if literal.Value == "true" {
			result.Blocks[i].Terminator = MIRJump{Target: branch.TrueTarget}
			count++
		}
		if literal.Value == "false" {
			result.Blocks[i].Terminator = MIRJump{Target: branch.FalseTarget}
			count++
		}
	}
	return result, count
}
