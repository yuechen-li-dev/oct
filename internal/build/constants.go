package build

import (
	"math"
	"strconv"
)

// Constant is the deliberately small compile-time value model used by the M0
// optimizer. It mirrors existing scalar MIR literals rather than introducing
// a second type system.
type Constant struct {
	Kind  string
	Int   int64
	Float float64
	Bool  bool
}

type ConstantStateKind uint8

const (
	ConstantUnknown ConstantStateKind = iota
	ConstantKnown
	ConstantOverdefined
)

// ConstantState is one fact in the forward constant lattice. Unknown is the
// solver's bottom value, Known carries one exact scalar, and Overdefined means
// execution can produce no single proven constant.
type ConstantState struct {
	Kind     ConstantStateKind
	Constant Constant
}

type LocalConstantMap map[string]ConstantState

// ConstantAnalysis records immutable block-entry, block-exit, and
// statement-entry facts. Statement entry index len(block.Statements) is the
// terminator entry.
type ConstantAnalysis struct {
	CFG             CFG
	In              map[string]LocalConstantMap
	Out             map[string]LocalConstantMap
	BeforeStatement map[string][]LocalConstantMap
	Unreachable     []string
	BlocksProcessed int
}

// AnalyzeConstants computes scalar-local constants without mutating MIR. Calls
// and aggregate-producing statements define Overdefined values; compound
// writes do not redefine their aggregate as a scalar.
func AnalyzeConstants(fn MIRFunction) (ConstantAnalysis, error) {
	cfg, err := BuildCFG(fn)
	if err != nil {
		return ConstantAnalysis{}, err
	}
	tracked := trackedScalarLocals(fn)
	result := ConstantAnalysis{
		CFG:             cfg,
		In:              make(map[string]LocalConstantMap, len(cfg.Reachable)),
		Out:             make(map[string]LocalConstantMap, len(cfg.Reachable)),
		BeforeStatement: make(map[string][]LocalConstantMap, len(cfg.Reachable)),
	}
	reachable := make(map[string]bool, len(cfg.Reachable))
	blocks := make(map[string]MIRBlock, len(fn.Blocks))
	for _, label := range cfg.Reachable {
		reachable[label] = true
		result.In[label] = unknownConstantMap(tracked)
		result.Out[label] = unknownConstantMap(tracked)
	}
	for _, block := range fn.Blocks {
		blocks[block.Label] = block
		if !reachable[block.Label] {
			result.Unreachable = append(result.Unreachable, block.Label)
		}
	}

	entry := unknownConstantMap(tracked)
	for _, param := range fn.Params {
		if _, ok := tracked[param.Name]; ok {
			entry[param.Name] = ConstantState{Kind: ConstantOverdefined}
		}
	}
	for _, capture := range fn.CaptureEnv {
		if _, ok := tracked[capture.Parameter]; ok {
			entry[capture.Parameter] = ConstantState{Kind: ConstantOverdefined}
		}
	}

	worklist := append([]string(nil), cfg.Reachable...)
	queued := make(map[string]bool, len(worklist))
	for _, label := range worklist {
		queued[label] = true
	}
	for len(worklist) > 0 {
		label := worklist[0]
		worklist = worklist[1:]
		queued[label] = false
		result.BlocksProcessed++

		incoming := unknownConstantMap(tracked)
		if label == cfg.Entry {
			incoming = mergeConstantMaps(incoming, entry, tracked)
		}
		for _, predecessor := range cfg.Predecessors[label] {
			if reachable[predecessor] {
				incoming = mergeConstantMaps(incoming, result.Out[predecessor], tracked)
			}
		}
		out := transferConstants(incoming, blocks[label].Statements, tracked)
		inChanged := !equalConstantMaps(incoming, result.In[label], tracked)
		outChanged := !equalConstantMaps(out, result.Out[label], tracked)
		if inChanged {
			result.In[label] = incoming
		}
		if !outChanged {
			continue
		}
		result.Out[label] = out
		for _, successor := range cfg.Successors[label] {
			if reachable[successor] && !queued[successor] {
				worklist = append(worklist, successor)
				queued[successor] = true
			}
		}
	}

	for _, label := range cfg.Reachable {
		state := cloneConstantMap(result.In[label])
		block := blocks[label]
		before := make([]LocalConstantMap, len(block.Statements)+1)
		for i, stmt := range block.Statements {
			before[i] = cloneConstantMap(state)
			state = transferConstantStatement(state, stmt, tracked)
		}
		before[len(block.Statements)] = cloneConstantMap(state)
		result.BeforeStatement[label] = before
	}
	return result, nil
}

func trackedScalarLocals(fn MIRFunction) map[string]struct{} {
	tracked := make(map[string]struct{})
	add := func(name, typ string) {
		if name != "" && name != "_" && (typ == "Bool" || typ == "Int" || typ == "Float") {
			tracked[name] = struct{}{}
		}
	}
	for _, field := range fn.Params {
		add(field.Name, field.Type)
	}
	for _, capture := range fn.CaptureEnv {
		add(capture.Parameter, capture.Type)
	}
	for _, field := range fn.Locals {
		add(field.Name, field.Type)
	}
	return tracked
}

func unknownConstantMap(tracked map[string]struct{}) LocalConstantMap {
	result := make(LocalConstantMap, len(tracked))
	for local := range tracked {
		result[local] = ConstantState{Kind: ConstantUnknown}
	}
	return result
}

func cloneConstantMap(input LocalConstantMap) LocalConstantMap {
	result := make(LocalConstantMap, len(input))
	for local, state := range input {
		result[local] = state
	}
	return result
}

func mergeConstantMaps(left, right LocalConstantMap, tracked map[string]struct{}) LocalConstantMap {
	result := make(LocalConstantMap, len(tracked))
	for local := range tracked {
		result[local] = mergeConstantState(left[local], right[local])
	}
	return result
}

func mergeConstantState(left, right ConstantState) ConstantState {
	if left.Kind == ConstantUnknown {
		return right
	}
	if right.Kind == ConstantUnknown {
		return left
	}
	if left.Kind == ConstantOverdefined || right.Kind == ConstantOverdefined {
		return ConstantState{Kind: ConstantOverdefined}
	}
	if equalConstant(left.Constant, right.Constant) {
		return left
	}
	return ConstantState{Kind: ConstantOverdefined}
}

func equalConstantMaps(left, right LocalConstantMap, tracked map[string]struct{}) bool {
	for local := range tracked {
		a, b := left[local], right[local]
		if a.Kind != b.Kind || (a.Kind == ConstantKnown && !equalConstant(a.Constant, b.Constant)) {
			return false
		}
	}
	return true
}

func equalConstant(left, right Constant) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case "Int":
		return left.Int == right.Int
	case "Float":
		return math.Float64bits(left.Float) == math.Float64bits(right.Float)
	case "Bool":
		return left.Bool == right.Bool
	default:
		return false
	}
}

func transferConstants(input LocalConstantMap, statements []MIRStmt, tracked map[string]struct{}) LocalConstantMap {
	state := cloneConstantMap(input)
	for _, stmt := range statements {
		state = transferConstantStatement(state, stmt, tracked)
	}
	return state
}

func transferConstantStatement(input LocalConstantMap, stmt MIRStmt, tracked map[string]struct{}) LocalConstantMap {
	state := cloneConstantMap(input)
	define := func(target string, value MIRValue) {
		if _, ok := tracked[target]; !ok {
			return
		}
		if constant, ok := evaluateConstant(value, state); ok {
			state[target] = ConstantState{Kind: ConstantKnown, Constant: constant}
		} else {
			state[target] = ConstantState{Kind: ConstantOverdefined}
		}
	}
	overdefine := func(target string) {
		if _, ok := tracked[target]; ok {
			state[target] = ConstantState{Kind: ConstantOverdefined}
		}
	}
	switch s := stmt.(type) {
	case MIRAssign:
		define(s.Target, s.Value)
	case MIRCall:
		overdefine(s.Target)
	case MIRGenericOctxiliaryCall:
		overdefine(s.Target)
	case MIRDestructureCall:
		for _, target := range s.Targets {
			overdefine(target)
		}
	case MIRConstructRecord:
		overdefine(s.Target)
	case MIRConstructArray:
		overdefine(s.Target)
	case MIRBatchMap:
		overdefine(s.Target)
	}
	return state
}

func evaluateConstant(value MIRValue, state LocalConstantMap) (Constant, bool) {
	if local, ok := value.(MIRLocal); ok {
		fact := state[local.Name]
		return fact.Constant, fact.Kind == ConstantKnown
	}
	folded, _ := foldValueWithConstants(value, state)
	literal, ok := folded.(MIRLiteral)
	if !ok {
		return Constant{}, false
	}
	return constantFromLiteral(literal)
}

func constantFromLiteral(literal MIRLiteral) (Constant, bool) {
	switch literal.Type {
	case "Bool":
		value, err := strconv.ParseBool(literal.Value)
		return Constant{Kind: "Bool", Bool: value}, err == nil
	case "Int":
		value, err := strconv.ParseInt(literal.Value, 10, 64)
		return Constant{Kind: "Int", Int: value}, err == nil
	case "Float":
		value, err := strconv.ParseFloat(literal.Value, 64)
		return Constant{Kind: "Float", Float: value}, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
	default:
		return Constant{}, false
	}
}

func constantLiteral(value Constant) MIRLiteral {
	switch value.Kind {
	case "Bool":
		return MIRLiteral{Type: "Bool", Value: strconv.FormatBool(value.Bool)}
	case "Int":
		return MIRLiteral{Type: "Int", Value: strconv.FormatInt(value.Int, 10)}
	case "Float":
		return MIRLiteral{Type: "Float", Value: strconv.FormatFloat(value.Float, 'g', -1, 64)}
	default:
		return MIRLiteral{}
	}
}
