package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type flowFeatures struct {
	NeedsHistory        bool
	NeedsResume         bool
	NeedsUtilityMap     bool
	NeedsGenericUtility bool
	NeedsScalarUtility  bool
	ScalarUtilitySites  map[int]string
	NeedsInput          bool
	NeedsYield          bool
}

func analyzeFlowFeatures(flow MIRFlow, usedBuiltins map[string]bool) flowFeatures {
	features := flowFeatures{
		NeedsHistory:       usedBuiltins["StateHistory"],
		ScalarUtilitySites: map[int]string{},
		NeedsInput:         flow.TurnInput != nil,
		NeedsYield:         flow.YieldType != "",
	}
	var visitStmt func(MIRFlowStmt)
	var visitAction func(MIRFlowWhenAction)
	var visitExpr func(MIRFlowExpr)
	visitExpr = func(expr MIRFlowExpr) {
		switch e := expr.(type) {
		case MIRFlowUtilityWhenExpr:
			if e.ControllerBound && isDirectPolicyScalarType(e.ResultType) {
				features.NeedsScalarUtility = true
				features.ScalarUtilitySites[e.SiteID] = e.ResultType
			} else {
				features.NeedsGenericUtility = true
				if e.ControllerBound {
					features.NeedsUtilityMap = true
				}
			}
			visitExpr(e.Hysteresis)
			visitExpr(e.MinCommit)
			for _, candidate := range e.Cases {
				visitExpr(candidate.Value)
				visitExpr(candidate.Condition)
				visitExpr(candidate.Score)
			}
			visitExpr(e.Else)
		}
	}
	visitAction = func(action MIRFlowWhenAction) {
		switch a := action.(type) {
		case MIRFlowWhenReturn:
			visitExpr(a.Value)
		case MIRFlowWhenBlock:
			for _, statement := range a.Statements {
				visitStmt(statement)
			}
		}
	}
	visitStmt = func(stmt MIRFlowStmt) {
		switch s := stmt.(type) {
		case MIRFlowRemember, MIRFlowResume:
			features.NeedsResume = true
		case MIRFlowFieldAssign:
			visitExpr(s.Value)
		case MIRFlowFieldIndexAssign:
			visitExpr(s.Value)
			for _, index := range s.Indices {
				visitExpr(index)
			}
		case MIRFlowLetStmt:
			visitExpr(s.Value)
		case MIRFlowLocalAssign:
			visitExpr(s.Value)
		case MIRFlowWhile:
			visitExpr(s.Condition)
			for _, nested := range s.Body {
				visitStmt(nested)
			}
		case MIRFlowFor:
			visitExpr(s.Start)
			visitExpr(s.End)
			visitExpr(s.Step)
			for _, nested := range s.Body {
				visitStmt(nested)
			}
		case MIRFlowFallibleMatch:
			visitExpr(s.Subject)
			for _, nested := range s.OkBody {
				visitStmt(nested)
			}
			for _, nested := range s.ErrBody {
				visitStmt(nested)
			}
		case MIRFlowReturn:
			if s.Value != nil {
				visitExpr(s.Value)
			}
		case MIRFlowYield:
			visitExpr(s.Value)
		case MIRFlowIf:
			visitExpr(s.Condition)
			for _, statement := range s.Then {
				visitStmt(statement)
			}
			for _, statement := range s.Else {
				visitStmt(statement)
			}
		case MIRFlowWhen:
			for _, c := range s.Cases {
				visitExpr(c.Condition)
				visitAction(c.Action)
			}
			visitAction(s.Else)
		}
	}
	for _, state := range flow.States {
		for _, statement := range state.Statements {
			visitStmt(statement)
		}
	}
	return features
}

func isDirectPolicyScalarType(typeName string) bool {
	if typeName == "Bool" || typeName == "String" || typeName == "Int" || typeName == "Float" {
		return true
	}
	return (strings.HasPrefix(typeName, "Int<") || strings.HasPrefix(typeName, "Float<")) && strings.HasSuffix(typeName, ">")
}

func emitGoFlow(b *strings.Builder, flow MIRFlow, features flowFeatures) error {
	structName := "__octFlow_" + flow.Package + "_" + flow.Name
	resultType := flow.Return
	fmt.Fprintf(b, "type %s struct {\n", structName)
	b.WriteString("\tstarted bool\n\tcompleted bool\n\tcurrentState int\n\tinstruction int\n")
	fmt.Fprintf(b, "\tresult %s\n\thasResult bool\n", goFlowResultType(resultType))
	if features.NeedsInput {
		fmt.Fprintf(b, "\t%s %s\n", flow.TurnInput.Name, goType(flow.TurnInput.Type))
	}
	if features.NeedsYield {
		fmt.Fprintf(b, "\tlastYield %s\n\thasYield bool\n", goType(flow.YieldType))
	}
	if features.NeedsHistory {
		b.WriteString("\thistory []string\n")
	}
	if features.NeedsResume {
		b.WriteString("\thasResumeTarget bool\n\tresumeTarget int\n")
	}
	if features.NeedsUtilityMap {
		b.WriteString("\tutilitySites map[int]__octUtilitySiteState\n")
	}
	utilitySiteIDs := make([]int, 0, len(features.ScalarUtilitySites))
	for siteID := range features.ScalarUtilitySites {
		utilitySiteIDs = append(utilitySiteIDs, siteID)
	}
	sort.Ints(utilitySiteIDs)
	for _, siteID := range utilitySiteIDs {
		fmt.Fprintf(b, "\tutilitySite%d __octScalarUtilitySiteState[%s]\n", siteID, goType(features.ScalarUtilitySites[siteID]))
	}
	for _, p := range flow.Parameters {
		fmt.Fprintf(b, "\t%s %s\n", p.Name, goType(p.Type))
	}
	if len(flow.Board) > 0 {
		b.WriteString("\tboard struct {\n")
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\t\t%s %s\n", field.Name, goType(field.Type))
		}
		b.WriteString("\t}\n")
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(b, "func fn_%s_%s(", flow.Package, flow.Name)
	for i, p := range flow.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s %s", p.Name, goType(p.Type))
	}
	fmt.Fprintf(b, ") %s {\n", goType(flowInstanceTypeString(resultType)))
	fmt.Fprintf(b, "\t__flow := &%s{}\n", structName)
	if features.NeedsUtilityMap {
		b.WriteString("\t__flow.utilitySites = map[int]__octUtilitySiteState{}\n")
	}
	for _, p := range flow.Parameters {
		fmt.Fprintf(b, "\t__flow.%s = %s\n", p.Name, p.Name)
	}
	b.WriteString("\treturn __flow\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octActive() string {\n", structName)
	b.WriteString("\tif !f.started || f.completed { return \"\" }\n\tswitch f.currentState {\n")
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\tcase %d: return %q\n", idx, state.Name)
	}
	b.WriteString("\tdefault: return \"\"\n\t}\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octComplete() bool { return f.completed }\n\n", structName)
	if features.NeedsYield {
		fmt.Fprintf(b, "func (f *%s) __octDidYield() bool { return f.hasYield }\n\n", structName)
	} else {
		fmt.Fprintf(b, "func (f *%s) __octDidYield() bool { return false }\n\n", structName)
	}
	fmt.Fprintf(b, "func (f *%s) __octYielded() (any, bool) {\n", structName)
	if features.NeedsYield {
		b.WriteString("\treturn f.lastYield, f.hasYield\n}\n\n")
	} else {
		b.WriteString("\treturn nil, false\n}\n\n")
	}
	fmt.Fprintf(b, "func (f *%s) __octResult() (%s, bool) {\n", structName, goFlowResultType(resultType))
	b.WriteString("\treturn f.result, f.hasResult\n}\n\n")
	fmt.Fprintf(b, "func (f *%s) __octStateHistory() []string {\n", structName)
	if features.NeedsHistory {
		b.WriteString("\tout := make([]string, len(f.history))\n\tcopy(out, f.history)\n\treturn out\n}\n\n")
	} else {
		b.WriteString("\treturn nil\n}\n\n")
	}
	if features.NeedsResume {
		fmt.Fprintf(b, "func (f *%s) __octStateName(id int) string {\n", structName)
		b.WriteString("\tswitch id {\n")
		for idx, state := range flow.States {
			fmt.Fprintf(b, "\tcase %d: return %q\n", idx, state.Name)
		}
		b.WriteString("\tdefault: return \"\"\n\t}\n}\n\n")
	}
	fmt.Fprintf(b, "func (f *%s) __octResumeTarget() string {\n", structName)
	if features.NeedsResume {
		b.WriteString("\tif !f.hasResumeTarget { return \"\" }\n\treturn f.__octStateName(f.resumeTarget)\n}\n\n")
	} else {
		b.WriteString("\treturn \"\"\n}\n\n")
	}
	fmt.Fprintf(b, "func (f *%s) __octBoardSnapshot() (any, bool) {\n", structName)
	if len(flow.Board) == 0 {
		b.WriteString("\treturn nil, false\n}\n\n")
	} else {
		fmt.Fprintf(b, "\treturn %s_%sBoardSnapshot{\n", flow.Package, flow.Name)
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\t\t%s: %s,\n", field.Name, cloneCompiledValueExpr("f.board."+field.Name, field.Type))
		}
		b.WriteString("\t}, true\n}\n\n")
	}
	fmt.Fprintf(b, "func (f *%s) __octStep(__input any) {\n", structName)
	for _, local := range collectFlowLetLocals(flow) {
		fmt.Fprintf(b, "\tvar %s %s\n", local.Name, goType(local.Type))
	}
	if features.NeedsInput {
		fmt.Fprintf(b, "\tif f.completed { panic(\"runtime error: input supplied after flow completion\") }\n\t__typedInput, __ok := __input.(%s); if !__ok { panic(\"runtime invariant violation: wrong flow turn input type\") }; f.%s = __typedInput\n\tdefer func() { var __zero %s; f.%s = __zero }()\n", goType(flow.TurnInput.Type), flow.TurnInput.Name, goType(flow.TurnInput.Type), flow.TurnInput.Name)
	} else {
		b.WriteString("\tif __input != nil { panic(\"runtime invariant violation: input supplied to non-input flow\") }\n\tif f.completed { return }\n")
	}
	if features.NeedsYield {
		b.WriteString("\tf.hasYield = false\n")
	}
	entryID := 0
	for idx, st := range flow.States {
		if st.Name == flow.EntryState {
			entryID = idx
			break
		}
	}
	if features.NeedsHistory {
		fmt.Fprintf(b, "\tif !f.started { f.started = true; f.currentState = %d; f.instruction = 0; f.history = append(f.history, %q) }\n", entryID, flow.EntryState)
	} else {
		fmt.Fprintf(b, "\tif !f.started { f.started = true; f.currentState = %d; f.instruction = 0 }\n", entryID)
	}
	b.WriteString("__octFlowMachine:\n\tfor {\n\t\tif false { break __octFlowMachine }\n\t\tswitch f.currentState {\n")
	stateIDs := map[string]int{}
	for idx, state := range flow.States {
		stateIDs[state.Name] = idx
	}
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\t\tcase %d:\n\t\t\tswitch f.instruction {\n", idx)
		for stmtIdx, stmt := range state.Statements {
			fmt.Fprintf(b, "\t\t\tcase %d:\n", stmtIdx)
			src, err := emitGoFlowStmt(stmt, flow.Package, stateIDs, resultType, features)
			if err != nil {
				return fmt.Errorf("flow %s.%s state %s: %w", flow.Package, flow.Name, state.Name, err)
			}
			for _, line := range strings.Split(src, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				fmt.Fprintf(b, "\t\t\t\t%s\n", line)
			}
		}
		fmt.Fprintf(b, "\t\t\tdefault:\n\t\t\t\tpanic(\"runtime invariant violation: flow state %s exited without suspend or return\")\n", state.Name)
		b.WriteString("\t\t\t}\n")
	}
	b.WriteString("\t\tdefault:\n\t\t\tpanic(\"runtime invariant violation: unknown flow state\")\n\t\t}\n\t}\n}\n\n")
	return nil
}

// emitGoFlowHostFacade emits the deliberately narrow embeddable-Go ABI.  The
// private flow remains the single state-machine implementation; this layer only
// gives external Go typed construction/turn methods and a logical checkpoint.
func emitGoFlowHostFacade(b *strings.Builder, flow MIRFlow, features flowFeatures, publicName string, refinements []MIRRefinement) error {
	structName := "__octFlow_" + flow.Package + "_" + flow.Name
	machineName := publicName
	turnName := publicName + "Turn"
	fmt.Fprintf(b, "// %s is the generated, typed host facade for the Oct flow %s.%s.\n", machineName, flow.Package, flow.Name)
	fmt.Fprintf(b, "type %s struct { flow *%s }\n\n", machineName, structName)
	fmt.Fprintf(b, "func New%s(", publicName)
	for i, parameter := range flow.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s %s", parameter.Name, goType(parameter.Type))
	}
	b.WriteString(") *")
	b.WriteString(machineName)
	b.WriteString(" {\n")
	fmt.Fprintf(b, "\treturn &%s{flow: fn_%s_%s(", machineName, flow.Package, flow.Name)
	for i, parameter := range flow.Parameters {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(parameter.Name)
	}
	fmt.Fprintf(b, ").(*%s)}\n}\n\n", structName)

	fmt.Fprintf(b, "type %s struct { machine *%s }\n\n", turnName, machineName)
	fmt.Fprintf(b, "func (m *%s) Step(", machineName)
	if features.NeedsInput {
		fmt.Fprintf(b, "input %s", goType(flow.TurnInput.Type))
	}
	fmt.Fprintf(b, ") (turn %s, err error) {\n", turnName)
	b.WriteString("\tif m == nil || m.flow == nil { return turn, fmt.Errorf(\"flow machine is nil\") }\n")
	b.WriteString("\tdefer func() { if recovered := recover(); recovered != nil { err = fmt.Errorf(\"flow step: %v\", recovered) } }()\n")
	if features.NeedsInput {
		if refinement, ok := findMIRRefinement(flow.TurnInput.Type, refinements); ok {
			fmt.Fprintf(b, "\tadmitted := fn_%s___oct_refine_%s(input)\n", refinement.Package, refinement.Name)
			b.WriteString("\tif admitted.IsErr { return turn, fmt.Errorf(\"%s\", admitted.Err) }\n")
			b.WriteString("\tm.flow.__octStep(admitted.Value)\n")
		} else {
			b.WriteString("\tm.flow.__octStep(input)\n")
		}
	} else {
		b.WriteString("\tm.flow.__octStep(nil)\n")
	}
	fmt.Fprintf(b, "\treturn %s{machine: m}, nil\n}\n\n", turnName)
	fmt.Fprintf(b, "func (t %s) DidYield() bool { return t.machine != nil && t.machine.flow != nil && t.machine.flow.__octDidYield() }\n", turnName)
	fmt.Fprintf(b, "func (t %s) Active() string { if t.machine == nil || t.machine.flow == nil { return \"\" }; return t.machine.flow.__octActive() }\n", turnName)
	fmt.Fprintf(b, "func (t %s) Complete() bool { return t.machine != nil && t.machine.flow != nil && t.machine.flow.__octComplete() }\n", turnName)
	if features.NeedsYield {
		fmt.Fprintf(b, "func (t %s) Yielded() (%s, error) {\n", turnName, goType(flow.YieldType))
		fmt.Fprintf(b, "\tvar zero %s\n", goType(flow.YieldType))
		b.WriteString("\tif t.machine == nil || t.machine.flow == nil { return zero, fmt.Errorf(\"flow turn is nil\") }\n")
		b.WriteString("\tvalue, ok := t.machine.flow.__octYielded(); if !ok { return zero, fmt.Errorf(\"last turn did not yield\") }\n")
		fmt.Fprintf(b, "\ttyped, ok := value.(%s); if !ok { return zero, fmt.Errorf(\"yield type invariant violated\") }; return typed, nil\n}\n\n", goType(flow.YieldType))
	}
	if flow.Return == "Void" {
		fmt.Fprintf(b, "func (m *%s) Result() error { if m == nil || m.flow == nil { return fmt.Errorf(\"flow machine is nil\") }; _, ok := m.flow.__octResult(); if !ok { return fmt.Errorf(\"flow has not completed\") }; return nil }\n\n", machineName)
	} else {
		fmt.Fprintf(b, "func (m *%s) Result() (%s, error) {\n", machineName, goFlowResultType(flow.Return))
		fmt.Fprintf(b, "\tvar zero %s; if m == nil || m.flow == nil { return zero, fmt.Errorf(\"flow machine is nil\") }; value, ok := m.flow.__octResult(); if !ok { return zero, fmt.Errorf(\"flow has not completed\") }; return value, nil\n}\n\n", goFlowResultType(flow.Return))
	}
	if len(flow.Board) > 0 {
		fmt.Fprintf(b, "func (m *%s) Board() (%s_%sBoardSnapshot, error) {\n", machineName, flow.Package, flow.Name)
		fmt.Fprintf(b, "\tvar zero %s_%sBoardSnapshot; if m == nil || m.flow == nil { return zero, fmt.Errorf(\"flow machine is nil\") }; value, ok := m.flow.__octBoardSnapshot(); if !ok { return zero, fmt.Errorf(\"flow has no board\") }; return value.(%s_%sBoardSnapshot), nil\n}\n\n", flow.Package, flow.Name, flow.Package, flow.Name)
	}
	if !features.NeedsYield {
		return nil
	}
	return emitGoFlowCheckpointFacade(b, flow, features, publicName, structName, machineName)
}

func findMIRRefinement(typeName string, refinements []MIRRefinement) (MIRRefinement, bool) {
	for _, refinement := range refinements {
		if typeName == refinement.Package+"."+refinement.Name || typeName == refinement.Name {
			return refinement, true
		}
	}
	return MIRRefinement{}, false
}

func emitGoFlowCheckpointFacade(b *strings.Builder, flow MIRFlow, features flowFeatures, publicName string, structName string, machineName string) error {
	cpName := publicName + "Checkpoint"
	payloadName := "__oct" + publicName + "CheckpointPayload"
	reasonName := publicName + "CheckpointReason"
	errName := publicName + "CheckpointError"
	fingerprint := compiledFlowFingerprint(flow, features)
	boardSchema := compiledFlowBoardSchema(flow)
	constructorSchema := compiledFlowConstructorSchema(flow)
	yieldSchema := flow.YieldType
	utilitySchema := compiledFlowUtilitySchema(features)

	fmt.Fprintf(b, "type %s string\n\n", reasonName)
	fmt.Fprintf(b, "const (\n\t%sVersionMismatch %s = \"CheckpointVersionMismatch\"\n\t%sFlowMismatch %s = \"FlowMismatch\"\n\t%sFingerprintMismatch %s = \"FlowFingerprintMismatch\"\n\t%sSchemaMismatch %s = \"SchemaMismatch\"\n\t%sStateMissing %s = \"StateMissing\"\n\t%sContinuationInvalid %s = \"ContinuationPositionInvalid\"\n\t%sBoardSchemaMismatch %s = \"BoardSchemaMismatch\"\n\t%sConstructionMismatch %s = \"ConstructionParameterMismatch\"\n\t%sUtilitySiteMismatch %s = \"UtilitySiteMismatch\"\n\t%sYieldSchemaMismatch %s = \"YieldSchemaMismatch\"\n\t%sNotAtYield %s = \"NotAtYieldBoundary\"\n)\n\n",
		publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName, publicName, reasonName)
	fmt.Fprintf(b, "type %s struct { Reason %s; Detail string }\n", errName, reasonName)
	fmt.Fprintf(b, "func (e %s) Error() string { if e.Detail == \"\" { return \"flow checkpoint: \" + string(e.Reason) }; return \"flow checkpoint: \" + string(e.Reason) + \": \" + e.Detail }\n\n", errName)
	fmt.Fprintf(b, "func __oct%sCheckpointError(reason %s, detail string) error { return %s{Reason: reason, Detail: detail} }\n\n", publicName, reasonName, errName)

	fmt.Fprintf(b, "type %s struct { data []byte }\n", cpName)
	fmt.Fprintf(b, "func (c %s) Bytes() []byte { out := make([]byte, len(c.data)); copy(out, c.data); return out }\n", cpName)
	fmt.Fprintf(b, "func Parse%s(data []byte) (%s, error) { var payload %s; if err := json.Unmarshal(data, &payload); err != nil { return %s{}, err }; out := make([]byte, len(data)); copy(out, data); return %s{data: out}, nil }\n\n", cpName, cpName, payloadName, cpName, cpName)

	if len(flow.Board) > 0 {
		fmt.Fprintf(b, "type __oct%sCheckpointBoard struct {\n", publicName)
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\t%s %s `json:%q`\n", field.Name, goType(field.Type), field.Name)
		}
		b.WriteString("}\n\n")
	}
	fmt.Fprintf(b, "type %s struct {\n", payloadName)
	b.WriteString("\tVersion int `json:\"version\"`\n\tPackage string `json:\"package\"`\n\tFlow string `json:\"flow\"`\n\tFingerprint string `json:\"fingerprint\"`\n\tBoardSchema string `json:\"board_schema\"`\n\tConstructionSchema string `json:\"construction_schema\"`\n\tUtilitySchema string `json:\"utility_schema\"`\n\tYieldSchema string `json:\"yield_schema\"`\n\tCurrentState string `json:\"current_state\"`\n\tInstruction int `json:\"instruction\"`\n")
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(b, "\tParameter%s %s `json:%q`\n", parameter.Name, goType(parameter.Type), "parameter_"+parameter.Name)
	}
	if len(flow.Board) > 0 {
		fmt.Fprintf(b, "\tBoard __oct%sCheckpointBoard `json:\"board\"`\n", publicName)
	}
	if features.NeedsResume {
		b.WriteString("\tHasResumeTarget bool `json:\"has_resume_target\"`\n\tResumeTarget string `json:\"resume_target\"`\n")
	}
	if features.NeedsHistory {
		b.WriteString("\tHistory []string `json:\"history\"`\n")
	}
	for _, siteID := range sortedUtilitySiteIDs(features) {
		fmt.Fprintf(b, "\tUtilitySite%d __octScalarUtilitySiteState[%s] `json:\"utility_site_%d\"`\n", siteID, goType(features.ScalarUtilitySites[siteID]), siteID)
	}
	fmt.Fprintf(b, "\tLastYield %s `json:\"last_yield\"`\n\tHasYield bool `json:\"has_yield\"`\n}\n\n", goType(flow.YieldType))

	fmt.Fprintf(b, "func (m *%s) Checkpoint() (%s, error) {\n", machineName, cpName)
	fmt.Fprintf(b, "\tif m == nil || m.flow == nil { return %s{}, __oct%sCheckpointError(%sNotAtYield, \"nil flow machine\") }\n", cpName, publicName, publicName)
	fmt.Fprintf(b, "\tif m.flow.completed || !m.flow.hasYield { return %s{}, __oct%sCheckpointError(%sNotAtYield, \"checkpoint requires the completed turn to have yielded\") }\n", cpName, publicName, publicName)
	if features.NeedsUtilityMap {
		fmt.Fprintf(b, "\treturn %s{}, __oct%sCheckpointError(%sUtilitySiteMismatch, \"generic utility-site values are not checkpointable through the typed host ABI\")\n", cpName, publicName, publicName)
		b.WriteString("}\n\n")
	} else {
		fmt.Fprintf(b, "\tpayload := %s{Version: 1, Package: %q, Flow: %q, Fingerprint: %q, BoardSchema: %q, ConstructionSchema: %q, UtilitySchema: %q, YieldSchema: %q, CurrentState: m.flow.__octActive(), Instruction: m.flow.instruction, LastYield: m.flow.lastYield, HasYield: m.flow.hasYield}\n", payloadName, flow.Package, flow.Name, fingerprint, boardSchema, constructorSchema, utilitySchema, yieldSchema)
		for _, parameter := range flow.Parameters {
			fmt.Fprintf(b, "\tpayload.Parameter%s = m.flow.%s\n", parameter.Name, parameter.Name)
		}
		if len(flow.Board) > 0 {
			fmt.Fprintf(b, "\tpayload.Board = __oct%sCheckpointBoard{", publicName)
			for i, field := range flow.Board {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(b, "%s: m.flow.board.%s", field.Name, field.Name)
			}
			b.WriteString("}\n")
		}
		if features.NeedsResume {
			b.WriteString("\tpayload.HasResumeTarget = m.flow.hasResumeTarget\n\tif m.flow.hasResumeTarget { payload.ResumeTarget = m.flow.__octStateName(m.flow.resumeTarget) }\n")
		}
		if features.NeedsHistory {
			b.WriteString("\tpayload.History = append([]string(nil), m.flow.history...)\n")
		}
		for _, siteID := range sortedUtilitySiteIDs(features) {
			fmt.Fprintf(b, "\tpayload.UtilitySite%d = m.flow.utilitySite%d\n", siteID, siteID)
		}
		fmt.Fprintf(b, "\tdata, err := json.Marshal(payload); if err != nil { return %s{}, err }; return %s{data: data}, nil\n}\n\n", cpName, cpName)
	}

	fmt.Fprintf(b, "func Restore%s(checkpoint %s) (*%s, error) {\n", publicName, cpName, machineName)
	fmt.Fprintf(b, "\tvar payload %s; if err := json.Unmarshal(checkpoint.data, &payload); err != nil { return nil, err }\n", payloadName)
	fmt.Fprintf(b, "\tif payload.Version != 1 { return nil, __oct%sCheckpointError(%sVersionMismatch, fmt.Sprintf(\"version %%d\", payload.Version)) }\n", publicName, publicName)
	fmt.Fprintf(b, "\tif payload.Package != %q || payload.Flow != %q { return nil, __oct%sCheckpointError(%sFlowMismatch, payload.Package+\".\"+payload.Flow) }\n", flow.Package, flow.Name, publicName, publicName)
	fmt.Fprintf(b, "\tif payload.Fingerprint != %q { return nil, __oct%sCheckpointError(%sFingerprintMismatch, \"compiled flow changed\") }\n", fingerprint, publicName, publicName)
	fmt.Fprintf(b, "\tif payload.BoardSchema != %q { return nil, __oct%sCheckpointError(%sBoardSchemaMismatch, \"board schema changed\") }\n", boardSchema, publicName, publicName)
	fmt.Fprintf(b, "\tif payload.ConstructionSchema != %q { return nil, __oct%sCheckpointError(%sConstructionMismatch, \"construction schema changed\") }\n", constructorSchema, publicName, publicName)
	fmt.Fprintf(b, "\tif payload.UtilitySchema != %q { return nil, __oct%sCheckpointError(%sUtilitySiteMismatch, \"utility site schema changed\") }\n", utilitySchema, publicName, publicName)
	fmt.Fprintf(b, "\tif payload.YieldSchema != %q { return nil, __oct%sCheckpointError(%sYieldSchemaMismatch, \"yield schema changed\") }\n", yieldSchema, publicName, publicName)
	fmt.Fprintf(b, "\tif !payload.HasYield { return nil, __oct%sCheckpointError(%sYieldSchemaMismatch, \"checkpoint is not a yielded machine boundary\") }\n", publicName, publicName)
	fmt.Fprintf(b, "\tstateID, ok := __oct%sStateID(payload.CurrentState); if !ok { return nil, __oct%sCheckpointError(%sStateMissing, payload.CurrentState) }\n", publicName, publicName, publicName)
	fmt.Fprintf(b, "\tif !__oct%sContinuationValid(stateID, payload.Instruction) { return nil, __oct%sCheckpointError(%sContinuationInvalid, fmt.Sprintf(\"%%s[%%d]\", payload.CurrentState, payload.Instruction)) }\n", publicName, publicName, publicName)
	if features.NeedsHistory {
		fmt.Fprintf(b, "\tfor _, state := range payload.History { if _, ok := __oct%sStateID(state); !ok { return nil, __oct%sCheckpointError(%sStateMissing, state) } }\n", publicName, publicName, publicName)
	}
	fmt.Fprintf(b, "\tflow := &%s{started: true, currentState: stateID, instruction: payload.Instruction, lastYield: payload.LastYield, hasYield: payload.HasYield}\n", structName)
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(b, "\tflow.%s = payload.Parameter%s\n", parameter.Name, parameter.Name)
	}
	if len(flow.Board) > 0 {
		for _, field := range flow.Board {
			fmt.Fprintf(b, "\tflow.board.%s = payload.Board.%s\n", field.Name, field.Name)
		}
	}
	if features.NeedsResume {
		b.WriteString("\tflow.hasResumeTarget = payload.HasResumeTarget\n")
		fmt.Fprintf(b, "\tif payload.HasResumeTarget { target, ok := __oct%sStateID(payload.ResumeTarget); if !ok { return nil, __oct%sCheckpointError(%sStateMissing, payload.ResumeTarget) }; flow.resumeTarget = target }\n", publicName, publicName, publicName)
	}
	if features.NeedsHistory {
		b.WriteString("\tflow.history = append([]string(nil), payload.History...)\n")
	}
	if features.NeedsUtilityMap {
		b.WriteString("\tflow.utilitySites = map[int]__octUtilitySiteState{}\n")
	}
	for _, siteID := range sortedUtilitySiteIDs(features) {
		fmt.Fprintf(b, "\tflow.utilitySite%d = payload.UtilitySite%d\n", siteID, siteID)
	}
	fmt.Fprintf(b, "\treturn &%s{flow: flow}, nil\n}\n\n", machineName)

	fmt.Fprintf(b, "func __oct%sStateID(name string) (int, bool) { switch name {\n", publicName)
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\tcase %q: return %d, true\n", state.Name, idx)
	}
	b.WriteString("\tdefault: return 0, false\n} }\n")
	fmt.Fprintf(b, "func __oct%sContinuationValid(state int, instruction int) bool { switch state {\n", publicName)
	for idx, state := range flow.States {
		fmt.Fprintf(b, "\tcase %d: return instruction >= 0 && instruction <= %d\n", idx, len(state.Statements))
	}
	b.WriteString("\tdefault: return false\n} }\n\n")
	return nil
}

func sortedUtilitySiteIDs(features flowFeatures) []int {
	ids := make([]int, 0, len(features.ScalarUtilitySites))
	for id := range features.ScalarUtilitySites {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func compiledFlowFingerprint(flow MIRFlow, features flowFeatures) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package:%s\nflow:%s\ninput:%v\nyield:%s\nreturn:%s\nentry:%s\n", flow.Package, flow.Name, flow.TurnInput, flow.YieldType, flow.Return, flow.EntryState)
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(&b, "parameter:%s:%s\n", parameter.Name, parameter.Type)
	}
	for _, field := range flow.Board {
		fmt.Fprintf(&b, "board:%s:%s\n", field.Name, field.Type)
	}
	for _, state := range flow.States {
		fmt.Fprintf(&b, "state:%s\n", state.Name)
		for _, statement := range state.Statements {
			fmt.Fprintf(&b, "statement:%s\n", dumpFlowStmt(statement))
		}
	}
	fmt.Fprintf(&b, "utility:%s\n", compiledFlowUtilitySchema(features))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func compiledFlowBoardSchema(flow MIRFlow) string {
	var b strings.Builder
	for _, field := range flow.Board {
		fmt.Fprintf(&b, "%s:%s;", field.Name, field.Type)
	}
	return b.String()
}

func compiledFlowConstructorSchema(flow MIRFlow) string {
	var b strings.Builder
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(&b, "%s:%s;", parameter.Name, parameter.Type)
	}
	return b.String()
}

func compiledFlowUtilitySchema(features flowFeatures) string {
	var b strings.Builder
	if features.NeedsUtilityMap {
		b.WriteString("generic;")
	}
	for _, id := range sortedUtilitySiteIDs(features) {
		fmt.Fprintf(&b, "%d:%s;", id, features.ScalarUtilitySites[id])
	}
	return b.String()
}

func collectFlowLetLocals(flow MIRFlow) []MIRField {
	seen := map[string]struct{}{}
	locals := []MIRField{}
	var visitStmt func(MIRFlowStmt)
	var visitWhenAction func(MIRFlowWhenAction)
	visitWhenAction = func(action MIRFlowWhenAction) {
		switch a := action.(type) {
		case MIRFlowWhenBlock:
			for _, st := range a.Statements {
				visitStmt(st)
			}
		}
	}
	visitStmt = func(stmt MIRFlowStmt) {
		switch s := stmt.(type) {
		case MIRFlowLetStmt:
			if _, ok := seen[s.Name]; ok {
				return
			}
			seen[s.Name] = struct{}{}
			locals = append(locals, MIRField{Name: s.Name, Type: s.Type})
		case MIRFlowIf:
			for _, st := range s.Then {
				visitStmt(st)
			}
			for _, st := range s.Else {
				visitStmt(st)
			}
		case MIRFlowWhile:
			for _, st := range s.Body {
				visitStmt(st)
			}
		case MIRFlowFor:
			for _, st := range s.Body {
				visitStmt(st)
			}
		case MIRFlowFallibleMatch:
			for _, st := range s.OkBody {
				visitStmt(st)
			}
			for _, st := range s.ErrBody {
				visitStmt(st)
			}
		case MIRFlowWhen:
			for _, c := range s.Cases {
				visitWhenAction(c.Action)
			}
			visitWhenAction(s.Else)
		}
	}
	for _, state := range flow.States {
		for _, st := range state.Statements {
			visitStmt(st)
		}
	}
	return locals
}

func emitGoFlowStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string, features flowFeatures) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowGoto:
		target, ok := stateIDs[s.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", s.Target)
		}
		if features.NeedsHistory {
			return fmt.Sprintf("f.currentState = %d; f.instruction = 0; f.history = append(f.history, %q); continue __octFlowMachine", target, s.Target), nil
		}
		return fmt.Sprintf("f.currentState = %d; f.instruction = 0; continue __octFlowMachine", target), nil
	case MIRFlowSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowYield:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.lastYield = %s\nf.hasYield = true\nf.instruction++\nreturn", v), nil
	case MIRFlowRemember:
		return "f.hasResumeTarget = true\nf.resumeTarget = f.currentState\nf.instruction++\ncontinue", nil
	case MIRFlowResume:
		resume := "if !f.hasResumeTarget { panic(\"runtime error: resume called with empty resume slot\") }\n__resumeTarget := f.resumeTarget\nf.hasResumeTarget = false\nf.resumeTarget = -1\nf.currentState = __resumeTarget\nf.instruction = 0"
		if features.NeedsHistory {
			resume += "\nf.history = append(f.history, f.__octStateName(__resumeTarget))"
		}
		return resume + "\ncontinue __octFlowMachine", nil
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s\nf.instruction++\ncontinue", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s\nf.instruction++\ncontinue", s.Target, s.Field, v), nil
	case MIRFlowFieldIndexAssign:
		return emitGoFlowFieldIndexAssign(s, pkg, true)
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s\nf.instruction++\ncontinue", s.Name, v), nil
	case MIRFlowLocalAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s\nf.instruction++\ncontinue", s.Name, v), nil
	case MIRFlowWhile:
		condition, err := emitGoFlowExpr(s.Condition, pkg)
		if err != nil {
			return "", err
		}
		body, err := emitGoFlowInlineBlock(s.Body, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("for %s {\n%s\n}\nf.instruction++\ncontinue", condition, body), nil
	case MIRFlowFor:
		start, err := emitGoFlowExpr(s.Start, pkg)
		if err != nil {
			return "", err
		}
		end, err := emitGoFlowExpr(s.End, pkg)
		if err != nil {
			return "", err
		}
		step, err := emitGoFlowExpr(s.Step, pkg)
		if err != nil {
			return "", err
		}
		body, err := emitGoFlowInlineBlock(s.Body, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		comparison, update := "<", "+="
		if s.Descending {
			comparison, update = ">", "-="
		}
		return fmt.Sprintf("for %s := %s; %s %s %s; %s %s %s {\n%s\n}\nf.instruction++\ncontinue", s.Name, start, s.Name, comparison, end, s.Name, update, step, body), nil
	case MIRFlowFallibleMatch:
		subject, err := emitGoFlowExpr(s.Subject, pkg)
		if err != nil {
			return "", err
		}
		okBody, err := emitGoFlowInlineBlock(s.OkBody, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		errBody, err := emitGoFlowInlineBlock(s.ErrBody, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("__oct_match := %s\nif __oct_match.IsErr {\n%s := __oct_match.Err\n_ = %s\n%s\n} else {\n%s := __oct_match.Value\n_ = %s\n%s\n}\nf.instruction++\ncontinue", subject, s.ErrName, s.ErrName, errBody, s.OkName, s.OkName, okBody), nil
	case MIRFlowReturn:
		if s.Value == nil {
			if resultType == "Void" {
				return "f.result = __octVoid{}\nf.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
			}
			return "f.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
		}
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	case MIRFlowIf:
		cond, err := emitGoFlowExpr(s.Condition, pkg)
		if err != nil {
			return "", err
		}
		thenSrc, err := emitGoFlowInlineBlock(s.Then, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		out := "if " + cond + " {\n" + thenSrc + "\n}"
		if len(s.Else) > 0 {
			elseSrc, err := emitGoFlowInlineBlock(s.Else, pkg, stateIDs, resultType, features)
			if err != nil {
				return "", err
			}
			out += " else {\n" + elseSrc + "\n}"
		}
		out += "\nf.instruction++\ncontinue"
		return out, nil
	case MIRFlowWhen:
		lines := []string{}
		for _, c := range s.Cases {
			cond, err := emitGoFlowExpr(c.Condition, pkg)
			if err != nil {
				return "", err
			}
			action, err := emitGoFlowWhenAction(c.Action, pkg, stateIDs, resultType, features)
			if err != nil {
				return "", err
			}
			lines = append(lines, fmt.Sprintf("if %s {\n%s\n}", cond, action))
		}
		elseAction, err := emitGoFlowWhenAction(s.Else, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		lines = append(lines, elseAction)
		return strings.Join(lines, "\n"), nil
	default:
		return "", unsupported(fmt.Sprintf("flow statement %T", stmt))
	}
}

func emitGoFlowFieldIndexAssign(s MIRFlowFieldIndexAssign, pkg string, advance bool) (string, error) {
	v, err := emitGoFlowExpr(s.Value, pkg)
	if err != nil {
		return "", err
	}
	indices := make([]string, 0, len(s.Indices))
	for _, index := range s.Indices {
		lowered, err := emitGoFlowExpr(index, pkg)
		if err != nil {
			return "", err
		}
		indices = append(indices, lowered)
	}
	target := "f." + s.Target + "." + s.Field
	if s.Target == "board" {
		target = "f.board." + s.Field
	}
	assignment := fmt.Sprintf("%s[%s] = %s", target, strings.Join(indices, "]["), v)
	if advance {
		assignment += "\nf.instruction++\ncontinue"
	}
	return assignment, nil
}

func emitGoFlowInlineBlock(stmts []MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string, features flowFeatures) (string, error) {
	lines := make([]string, 0, len(stmts))
	for _, stmt := range stmts {
		src, err := emitGoFlowInlineStmt(stmt, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		lines = append(lines, src)
	}
	return strings.Join(lines, "\n"), nil
}

func emitGoFlowInlineStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string, features flowFeatures) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s", s.Target, s.Field, v), nil
	case MIRFlowFieldIndexAssign:
		return emitGoFlowFieldIndexAssign(s, pkg, false)
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name, v), nil
	case MIRFlowLocalAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name, v), nil
	case MIRFlowRemember:
		return "f.hasResumeTarget = true\nf.resumeTarget = f.currentState", nil
	case MIRFlowIf:
		condition, err := emitGoFlowExpr(s.Condition, pkg)
		if err != nil {
			return "", err
		}
		thenSource, err := emitGoFlowInlineBlock(s.Then, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		out := "if " + condition + " {\n" + thenSource + "\n}"
		if len(s.Else) > 0 {
			elseSource, err := emitGoFlowInlineBlock(s.Else, pkg, stateIDs, resultType, features)
			if err != nil {
				return "", err
			}
			out += " else {\n" + elseSource + "\n}"
		}
		return out, nil
	case MIRFlowWhile:
		condition, err := emitGoFlowExpr(s.Condition, pkg)
		if err != nil {
			return "", err
		}
		body, err := emitGoFlowInlineBlock(s.Body, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("for %s {\n%s\n}", condition, body), nil
	case MIRFlowFor:
		start, err := emitGoFlowExpr(s.Start, pkg)
		if err != nil {
			return "", err
		}
		end, err := emitGoFlowExpr(s.End, pkg)
		if err != nil {
			return "", err
		}
		step, err := emitGoFlowExpr(s.Step, pkg)
		if err != nil {
			return "", err
		}
		body, err := emitGoFlowInlineBlock(s.Body, pkg, stateIDs, resultType, features)
		if err != nil {
			return "", err
		}
		comparison, update := "<", "+="
		if s.Descending {
			comparison, update = ">", "-="
		}
		return fmt.Sprintf("for %s := %s; %s %s %s; %s %s %s {\n%s\n}", s.Name, start, s.Name, comparison, end, s.Name, update, step, body), nil
	default:
		return emitGoFlowStmt(s, pkg, stateIDs, resultType, features)
	}
}

func emitGoFlowWhenAction(action MIRFlowWhenAction, pkg string, stateIDs map[string]int, resultType string, features flowFeatures) (string, error) {
	switch a := action.(type) {
	case MIRFlowWhenGoto:
		target, ok := stateIDs[a.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", a.Target)
		}
		transition := fmt.Sprintf("f.currentState = %d\nf.instruction = 0", target)
		if features.NeedsHistory {
			transition += fmt.Sprintf("\nf.history = append(f.history, %q)", a.Target)
		}
		return transition + "\ncontinue __octFlowMachine", nil
	case MIRFlowWhenSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowWhenReturn:
		v, err := emitGoFlowExpr(a.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	case MIRFlowWhenBlock:
		lines := make([]string, 0, len(a.Statements))
		for _, statement := range a.Statements {
			src, err := emitGoFlowWhenBlockStmt(statement, pkg, stateIDs, resultType, features)
			if err != nil {
				return "", err
			}
			lines = append(lines, src)
		}
		return strings.Join(lines, "\n"), nil
	default:
		return "", unsupported(fmt.Sprintf("flow when action %T", action))
	}
}

func emitGoFlowWhenBlockStmt(stmt MIRFlowStmt, pkg string, stateIDs map[string]int, resultType string, features flowFeatures) (string, error) {
	switch s := stmt.(type) {
	case MIRFlowRemember:
		return "f.hasResumeTarget = true\nf.resumeTarget = f.currentState", nil
	case MIRFlowLetStmt:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", s.Name, v), nil
	case MIRFlowFieldAssign:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		if s.Target == "board" {
			return fmt.Sprintf("f.board.%s = %s", s.Field, v), nil
		}
		return fmt.Sprintf("f.%s.%s = %s", s.Target, s.Field, v), nil
	case MIRFlowFieldIndexAssign:
		return emitGoFlowFieldIndexAssign(s, pkg, false)
	case MIRFlowGoto:
		target, ok := stateIDs[s.Target]
		if !ok {
			return "", fmt.Errorf("unknown goto target %s", s.Target)
		}
		transition := fmt.Sprintf("f.currentState = %d\nf.instruction = 0", target)
		if features.NeedsHistory {
			transition += fmt.Sprintf("\nf.history = append(f.history, %q)", s.Target)
		}
		return transition + "\ncontinue __octFlowMachine", nil
	case MIRFlowSuspend:
		return "f.instruction++\nreturn", nil
	case MIRFlowYield:
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.lastYield = %s\nf.hasYield = true\nf.instruction++\nreturn", v), nil
	case MIRFlowResume:
		resume := "if !f.hasResumeTarget { panic(\"runtime error: resume called with empty resume slot\") }\n__resumeTarget := f.resumeTarget\nf.hasResumeTarget = false\nf.resumeTarget = -1\nf.currentState = __resumeTarget\nf.instruction = 0"
		if features.NeedsHistory {
			resume += "\nf.history = append(f.history, f.__octStateName(__resumeTarget))"
		}
		return resume + "\ncontinue __octFlowMachine", nil
	case MIRFlowReturn:
		if s.Value == nil {
			if resultType == "Void" {
				return "f.result = __octVoid{}\nf.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
			}
			return "f.completed = true\nf.hasResult = true\nf.currentState = -1\nreturn", nil
		}
		v, err := emitGoFlowExpr(s.Value, pkg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("f.result = %s\nf.hasResult = true\nf.completed = true\nf.currentState = -1\nreturn", v), nil
	default:
		return "", unsupported(fmt.Sprintf("flow when block statement %T", stmt))
	}
}

func emitGoFlowExpr(expr MIRFlowExpr, pkg string) (string, error) {
	switch value := expr.(type) {
	case MIRFlowSharedExpr:
		return emitGoSharedExpression(value)
	case MIRFlowUtilityWhenExpr:
		hysteresis, err := emitGoFlowExpr(value.Hysteresis, pkg)
		if err != nil {
			return "", err
		}
		minCommit, err := emitGoFlowExpr(value.MinCommit, pkg)
		if err != nil {
			return "", err
		}
		elseExpr, err := emitGoFlowExpr(value.Else, pkg)
		if err != nil {
			return "", err
		}
		cases := make([]string, 0, len(value.Cases))
		valueType := goType(value.ResultType)
		for _, candidate := range value.Cases {
			candidateValue, err := emitGoFlowExpr(candidate.Value, pkg)
			if err != nil {
				return "", err
			}
			condition, err := emitGoFlowExpr(candidate.Condition, pkg)
			if err != nil {
				return "", err
			}
			score, err := emitGoFlowExpr(candidate.Score, pkg)
			if err != nil {
				return "", err
			}
			cases = append(cases, fmt.Sprintf("{Valid: %s, Value: %s, Score: %s}", condition, candidateValue, score))
		}
		sites := "map[int]__octUtilitySiteState{}"
		if value.ControllerBound {
			if isDirectPolicyScalarType(value.ResultType) {
				return fmt.Sprintf("__octUtilSelectScalar[%s](&f.utilitySite%d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
					valueType, value.SiteID, hysteresis, minCommit, valueType, strings.Join(cases, ", "), elseExpr), nil
			}
			sites = "f.utilitySites"
		}
		return fmt.Sprintf("__octUtilSelect[%s](%s, %d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
			valueType, sites, value.SiteID, hysteresis, minCommit, valueType, strings.Join(cases, ", "), elseExpr), nil
	default:
		return "", fmt.Errorf("internal error: unsupported FLOW expression representation %T", expr)
	}
}

func emitGoSharedExpression(expr MIRFlowSharedExpr) (string, error) {
	if len(expr.Blocks) == 0 {
		return "", fmt.Errorf("shared compiled expression has no MIR blocks")
	}
	var b strings.Builder
	resultType := goType(expr.Type)
	if expr.Fallible {
		resultType = goResultTypeName(expr.Type)
	}
	fmt.Fprintf(&b, "func() %s {\n", resultType)
	for _, local := range expr.Locals {
		localType := goType(local.Type)
		if local.Type == "Void" {
			localType = "__octVoid"
		}
		fmt.Fprintf(&b, "var %s %s\n", local.Name, localType)
		if local.Name != "_" {
			fmt.Fprintf(&b, "_ = %s\n", local.Name)
		}
	}
	labels := make(map[string]int, len(expr.Blocks))
	for index, block := range expr.Blocks {
		labels[block.Label] = index
	}
	pc := internalName(internalProgramCounter, -1)
	fmt.Fprintf(&b, "%s := 0\nfor {\nswitch %s {\n", pc, pc)
	for index, block := range expr.Blocks {
		fmt.Fprintf(&b, "case %d:\n", index)
		for _, statement := range block.Statements {
			source, err := goStmt(statement)
			if err != nil {
				return "", err
			}
			b.WriteString(source)
			b.WriteByte('\n')
		}
		terminator := block.Terminator
		if terminator == nil {
			if index+1 >= len(expr.Blocks) {
				return "", fmt.Errorf("shared compiled expression final block %s has no terminator", block.Label)
			}
			terminator = MIRJump{Target: expr.Blocks[index+1].Label}
		}
		source, err := goTerminator(terminator, labels, pc)
		if err != nil {
			return "", err
		}
		b.WriteString(source)
		b.WriteByte('\n')
	}
	b.WriteString("}\n}\n}()")
	return b.String(), nil
}
