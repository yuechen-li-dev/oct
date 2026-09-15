package build

import "fmt"

// DefinitionKind identifies where a whole-local definition originates.
type DefinitionKind string

const (
	DefinitionParameter DefinitionKind = "parameter"
	DefinitionCapture   DefinitionKind = "capture"
	DefinitionStatement DefinitionKind = "statement"
)

// DefinitionID gives a definition a deterministic identity derived from its
// MIR position. Statement is -1 for parameters and captures; Result
// distinguishes multiple results produced by one MIR statement.
type DefinitionID struct {
	Kind      DefinitionKind
	Block     string
	Statement int
	Result    int
	Local     string
}

// UseSite identifies one local read. Operand is a stable path through the
// statement or terminator's structured MIR values.
type UseSite struct {
	Block     string
	Statement int
	Operand   string
	Local     string
}

type LocalUse struct {
	Site UseSite
}

// CompoundDefinition records a write below a whole-local boundary. Chapter 3
// deliberately does not make alias claims about these locations.
type CompoundDefinition struct {
	Block     string
	Statement int
	Kind      string
	Target    string
}

// UseDefInfo is an immutable inventory of whole-local definitions, local uses,
// and explicitly untracked compound writes in one MIR function.
type UseDefInfo struct {
	Definitions         []DefinitionID
	InitialDefinitions  []DefinitionID
	DefinitionsByBlock  map[string][]DefinitionID
	Uses                []LocalUse
	UsesByBlock         map[string][]LocalUse
	CompoundDefinitions []CompoundDefinition
}

// ExtractUseDefInfo walks structured MIR without relying on any backend text.
// It fails closed when a statement or value cannot expose its local reads.
func ExtractUseDefInfo(fn MIRFunction) (UseDefInfo, error) {
	info := UseDefInfo{
		DefinitionsByBlock: make(map[string][]DefinitionID, len(fn.Blocks)),
		UsesByBlock:        make(map[string][]LocalUse, len(fn.Blocks)),
	}
	entry := ""
	if len(fn.Blocks) > 0 {
		entry = fn.Blocks[0].Label
	}
	for i, param := range fn.Params {
		if param.Name == "" {
			return UseDefInfo{}, fmt.Errorf("function %s.%s has parameter %d with an empty MIR local name", fn.Package, fn.Name, i)
		}
		def := DefinitionID{Kind: DefinitionParameter, Block: entry, Statement: -1, Result: i, Local: param.Name}
		info.Definitions = append(info.Definitions, def)
		info.InitialDefinitions = append(info.InitialDefinitions, def)
	}
	for i, capture := range fn.CaptureEnv {
		if capture.Parameter == "" {
			return UseDefInfo{}, fmt.Errorf("function %s.%s has capture %d with an empty MIR local name", fn.Package, fn.Name, i)
		}
		def := DefinitionID{Kind: DefinitionCapture, Block: entry, Statement: -1, Result: i, Local: capture.Parameter}
		info.Definitions = append(info.Definitions, def)
		info.InitialDefinitions = append(info.InitialDefinitions, def)
	}

	for _, block := range fn.Blocks {
		info.DefinitionsByBlock[block.Label] = []DefinitionID{}
		info.UsesByBlock[block.Label] = []LocalUse{}
		for statement, stmt := range block.Statements {
			defs, uses, compound, err := extractStatementUseDefs(block.Label, statement, stmt)
			if err != nil {
				return UseDefInfo{}, fmt.Errorf("MIR statement %s.%s:%s[%d]: %w", fn.Package, fn.Name, block.Label, statement, err)
			}
			info.Definitions = append(info.Definitions, defs...)
			info.DefinitionsByBlock[block.Label] = append(info.DefinitionsByBlock[block.Label], defs...)
			info.Uses = append(info.Uses, uses...)
			info.UsesByBlock[block.Label] = append(info.UsesByBlock[block.Label], uses...)
			info.CompoundDefinitions = append(info.CompoundDefinitions, compound...)
		}
		uses, err := extractTerminatorUses(block.Label, len(block.Statements), block.Terminator)
		if err != nil {
			return UseDefInfo{}, fmt.Errorf("MIR terminator %s.%s:%s: %w", fn.Package, fn.Name, block.Label, err)
		}
		info.Uses = append(info.Uses, uses...)
		info.UsesByBlock[block.Label] = append(info.UsesByBlock[block.Label], uses...)
	}
	return info, nil
}

func extractStatementUseDefs(block string, statement int, stmt MIRStmt) ([]DefinitionID, []LocalUse, []CompoundDefinition, error) {
	var defs []DefinitionID
	var uses []LocalUse
	var compound []CompoundDefinition
	addDef := func(result int, local string) error {
		if local == "" {
			return fmt.Errorf("has an empty definition target")
		}
		if local != "_" {
			defs = append(defs, DefinitionID{Kind: DefinitionStatement, Block: block, Statement: statement, Result: result, Local: local})
		}
		return nil
	}
	addLocalUse := func(local, operand string) error {
		if local == "" {
			return fmt.Errorf("has an empty local use")
		}
		uses = append(uses, LocalUse{Site: UseSite{Block: block, Statement: statement, Operand: operand, Local: local}})
		return nil
	}
	walk := func(value MIRValue, operand string) error {
		var err error
		uses, err = appendMIRValueUses(uses, block, statement, operand, value)
		return err
	}

	switch s := stmt.(type) {
	case MIRAssign:
		if err := walk(s.Value, "value"); err != nil {
			return nil, nil, nil, err
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case MIRRowAssign:
		if err := addLocalUse(s.Target, "target"); err != nil {
			return nil, nil, nil, err
		}
		if err := walk(s.Index, "index"); err != nil {
			return nil, nil, nil, err
		}
		if err := walk(s.Value, "value"); err != nil {
			return nil, nil, nil, err
		}
		compound = append(compound, CompoundDefinition{Block: block, Statement: statement, Kind: "row", Target: s.Target})
	case MIRIndexAssign:
		if err := addLocalUse(s.Target, "target"); err != nil {
			return nil, nil, nil, err
		}
		for i, index := range s.Indices {
			if err := walk(index, fmt.Sprintf("index[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := walk(s.Value, "value"); err != nil {
			return nil, nil, nil, err
		}
		compound = append(compound, CompoundDefinition{Block: block, Statement: statement, Kind: "index", Target: s.Target})
	case MIRCall:
		if s.FunctionValue {
			if err := addLocalUse(s.Callee, "callee"); err != nil {
				return nil, nil, nil, err
			}
		}
		for i, arg := range s.Args {
			if err := walk(arg, fmt.Sprintf("arg[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case MIRGenericOctxiliaryCall:
		for i, arg := range s.Args {
			if err := walk(arg, fmt.Sprintf("arg[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case MIRDestructureCall:
		for i, arg := range s.Args {
			if err := walk(arg, fmt.Sprintf("arg[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		for i, target := range s.Targets {
			if err := addDef(i, target); err != nil {
				return nil, nil, nil, err
			}
		}
	case MIRConstructRecord:
		if len(s.FieldNames) != len(s.FieldVals) {
			return nil, nil, nil, fmt.Errorf("record construction has %d field names but %d values", len(s.FieldNames), len(s.FieldVals))
		}
		for i, value := range s.FieldVals {
			if err := walk(value, fmt.Sprintf("field[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case MIRConstructArray:
		for i, value := range s.Values {
			if err := walk(value, fmt.Sprintf("element[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case MIRBatchMap:
		if err := walk(s.Input, "input"); err != nil {
			return nil, nil, nil, err
		}
		for i, capture := range s.Captures {
			if err := walk(capture, fmt.Sprintf("capture[%d]", i)); err != nil {
				return nil, nil, nil, err
			}
		}
		if err := addDef(0, s.Target); err != nil {
			return nil, nil, nil, err
		}
	case nil:
		return nil, nil, nil, fmt.Errorf("is nil")
	default:
		return nil, nil, nil, fmt.Errorf("has unsupported statement type %T", stmt)
	}
	return defs, uses, compound, nil
}

func extractTerminatorUses(block string, statement int, term MIRTerminator) ([]LocalUse, error) {
	switch t := term.(type) {
	case MIRReturn:
		if t.Value == nil {
			return nil, nil
		}
		return appendMIRValueUses(nil, block, statement, "return", t.Value)
	case MIRJump:
		return nil, nil
	case MIRBranch:
		return appendMIRValueUses(nil, block, statement, "condition", t.Cond)
	case MIRFail:
		return appendMIRValueUses(nil, block, statement, "fail", t.Value)
	case nil:
		return nil, fmt.Errorf("is nil")
	default:
		return nil, fmt.Errorf("has unsupported terminator type %T", term)
	}
}

func appendMIRValueUses(uses []LocalUse, block string, statement int, operand string, value MIRValue) ([]LocalUse, error) {
	if value == nil {
		return nil, fmt.Errorf("%s has a nil MIR value", operand)
	}
	walk := func(current []LocalUse, suffix string, child MIRValue) ([]LocalUse, error) {
		return appendMIRValueUses(current, block, statement, operand+suffix, child)
	}
	var err error
	switch v := value.(type) {
	case MIRLiteral, MIRFunctionRef:
		return uses, nil
	case MIRLocal:
		if v.Name == "" {
			return nil, fmt.Errorf("%s has an empty MIR local name", operand)
		}
		return append(uses, LocalUse{Site: UseSite{Block: block, Statement: statement, Operand: operand, Local: v.Name}}), nil
	case MIRUnary:
		return walk(uses, ".value", v.Value)
	case MIRBinary:
		if uses, err = walk(uses, ".left", v.Left); err != nil {
			return nil, err
		}
		return walk(uses, ".right", v.Right)
	case MIRConvert:
		return walk(uses, ".value", v.Value)
	case MIRIndex:
		if uses, err = walk(uses, ".target", v.Target); err != nil {
			return nil, err
		}
		return walk(uses, ".index", v.Index)
	case MIRFieldAccess:
		return walk(uses, ".target", v.Target)
	case MIRClone:
		return walk(uses, ".value", v.Value)
	case MIRRangeValue:
		if v.HasStart {
			if uses, err = walk(uses, ".start", v.Start); err != nil {
				return nil, err
			}
		}
		if v.HasEnd {
			if uses, err = walk(uses, ".end", v.End); err != nil {
				return nil, err
			}
		}
		if v.HasStep {
			if uses, err = walk(uses, ".step", v.Step); err != nil {
				return nil, err
			}
		}
		return uses, nil
	case MIRArrayConvert:
		return walk(uses, ".value", v.Value)
	case MIRLength:
		return walk(uses, ".value", v.Value)
	case MIRMatrixColumnCount:
		return walk(uses, ".value", v.Value)
	case MIRResultValue:
		if v.IsError {
			return walk(uses, ".error", v.Error)
		}
		return walk(uses, ".value", v.Value)
	case MIREnumValue:
		if v.Payload == nil {
			return uses, nil
		}
		return walk(uses, ".payload", v.Payload)
	case MIREnumPayload:
		return walk(uses, ".value", v.Value)
	case MIRIntrinsicValue:
		for i, arg := range v.Args {
			uses, err = walk(uses, fmt.Sprintf(".arg[%d]", i), arg)
			if err != nil {
				return nil, err
			}
		}
		return uses, nil
	case MIRBackendValue:
		return nil, fmt.Errorf("%s uses opaque backend value %q (%s); local uses are not structurally available", operand, v.Backend, v.Reason)
	default:
		return nil, fmt.Errorf("%s has unsupported MIR value type %T", operand, value)
	}
}
