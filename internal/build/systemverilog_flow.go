package build

import (
	"fmt"
	"math/bits"
	"sort"
	"strconv"
	"strings"
)

const maxSVFlowDispatchSteps = 1024

type svFlowUtilitySite struct {
	ID         int
	Type       string
	Persistent bool
}

type svFlowNode struct {
	state       int
	instruction int
}

type svFlowEmit struct {
	c                *svContext
	flow             MIRFlow
	stateNames       map[string]string
	stateIDs         map[string]int
	boardNames       map[string]string
	boardNextNames   map[string]string
	parameterNames   map[string]string
	parameterPorts   map[string]string
	inputName        string
	localNames       map[string]string
	utilitySites     map[int]svFlowUtilitySite
	expressionSerial int
	indent           string
}

func (c *svContext) checkFlow(flow MIRFlow) error {
	name := flow.Package + "." + flow.Name
	if len(flow.States) == 0 {
		return fmt.Errorf("flow %s has no states", name)
	}
	if len(flow.States) > maxSVFlowDispatchSteps {
		return fmt.Errorf("flow %s has %d states; Verilog M2 limit is %d", name, len(flow.States), maxSVFlowDispatchSteps)
	}
	entryFound := false
	for _, state := range flow.States {
		if state.Name == flow.EntryState {
			entryFound = true
		}
	}
	if !entryFound {
		return fmt.Errorf("flow %s entry state %q does not exist", name, flow.EntryState)
	}
	for _, field := range flow.Parameters {
		if _, err := c.typeWidth(field.Type); err != nil {
			return fmt.Errorf("flow %s construction parameter %s: %w", name, sourceOrMIRName(field), err)
		}
	}
	if flow.TurnInput != nil {
		if _, err := c.typeWidth(flow.TurnInput.Type); err != nil {
			return fmt.Errorf("flow %s turn input %s: %w", name, sourceOrMIRName(*flow.TurnInput), err)
		}
	}
	if flow.YieldType != "" {
		if _, err := c.typeWidth(flow.YieldType); err != nil {
			return fmt.Errorf("flow %s yield type: %w", name, err)
		}
	}
	if flow.Return != "Void" {
		if _, err := c.typeWidth(flow.Return); err != nil {
			return fmt.Errorf("flow %s return type: %w", name, err)
		}
	}
	for _, field := range flow.Board {
		if _, err := c.typeWidth(field.Type); err != nil {
			return fmt.Errorf("flow %s board field %s: %w", name, sourceOrMIRName(field), err)
		}
	}
	for _, state := range flow.States {
		for _, statement := range state.Statements {
			if err := c.checkFlowStmt(flow, state.Name, statement, false); err != nil {
				return fmt.Errorf("flow %s state %s: %w", name, state.Name, err)
			}
		}
	}
	if err := checkSVFlowActivationGraph(flow); err != nil {
		return fmt.Errorf("flow %s: %w", name, err)
	}
	return nil
}

func (c *svContext) checkFlowStmt(flow MIRFlow, state string, statement MIRFlowStmt, inlineLoop bool) error {
	switch node := statement.(type) {
	case MIRFlowGoto, MIRFlowSuspend, MIRFlowRemember, MIRFlowResume:
		if inlineLoop {
			return fmt.Errorf("terminal FLOW control is not supported inside a hardware for loop")
		}
		return nil
	case MIRFlowYield:
		if inlineLoop {
			return fmt.Errorf("yield is not supported inside a hardware for loop")
		}
		return c.checkFlowExpr(flow, node.Value)
	case MIRFlowFieldAssign:
		if node.Target != "board" {
			return fmt.Errorf("persistent assignment target %s is not the FLOW board", node.Target)
		}
		found := false
		for _, field := range flow.Board {
			found = found || field.Name == node.Field
		}
		if !found {
			return fmt.Errorf("assignment targets unknown board field %s", node.Field)
		}
		return c.checkFlowExpr(flow, node.Value)
	case MIRFlowFieldIndexAssign:
		return fmt.Errorf("Verilog profile does not support indexed board storage; dynamic collections are not hardware tensors")
	case MIRFlowLetStmt:
		if _, err := c.typeWidth(node.Type); err != nil {
			return fmt.Errorf("local %s: %w", node.Name, err)
		}
		return c.checkFlowExpr(flow, node.Value)
	case MIRFlowLocalAssign:
		return c.checkFlowExpr(flow, node.Value)
	case MIRFlowWhile:
		return fmt.Errorf("runtime-dependent while loop is not legal sequential hardware; use a literal-bounded for loop")
	case MIRFlowFor:
		start, startOK := svFlowLiteralInt(node.Start)
		end, endOK := svFlowLiteralInt(node.End)
		step, stepOK := svFlowLiteralInt(node.Step)
		if !startOK || !endOK || !stepOK || step <= 0 {
			return fmt.Errorf("FLOW for loop requires integer-literal bounds and a positive integer-literal step")
		}
		iterations := int64(0)
		if node.Descending {
			if start < end {
				return fmt.Errorf("descending FLOW for loop start must be greater than or equal to end")
			}
			iterations = (start - end + step - 1) / step
		} else {
			if start > end {
				return fmt.Errorf("FLOW for loop start must be less than or equal to end")
			}
			iterations = (end - start + step - 1) / step
		}
		if iterations > maxSVFlowDispatchSteps {
			return fmt.Errorf("statically bounded FLOW loop has %d iterations; M2 limit is %d", iterations, maxSVFlowDispatchSteps)
		}
		for _, nested := range node.Body {
			if err := c.checkFlowStmt(flow, state, nested, true); err != nil {
				return err
			}
		}
		return nil
	case MIRFlowFallibleMatch:
		return fmt.Errorf("Verilog profile does not support fallible FLOW match paths")
	case MIRFlowReturn:
		if inlineLoop {
			return fmt.Errorf("return is not supported inside a hardware for loop")
		}
		if node.Value != nil {
			return c.checkFlowExpr(flow, node.Value)
		}
		return nil
	case MIRFlowIf:
		if err := c.checkFlowExpr(flow, node.Condition); err != nil {
			return err
		}
		for _, branch := range [][]MIRFlowStmt{node.Then, node.Else} {
			for _, nested := range branch {
				if err := c.checkFlowStmt(flow, state, nested, inlineLoop); err != nil {
					return err
				}
			}
		}
		return nil
	case MIRFlowWhen:
		if inlineLoop {
			return fmt.Errorf("guard when is not supported inside a hardware for loop")
		}
		for _, candidate := range node.Cases {
			if err := c.checkFlowExpr(flow, candidate.Condition); err != nil {
				return err
			}
			if err := c.checkFlowWhenAction(flow, state, candidate.Action); err != nil {
				return err
			}
		}
		return c.checkFlowWhenAction(flow, state, node.Else)
	default:
		return fmt.Errorf("Verilog profile does not support FLOW statement %T", statement)
	}
}

func (c *svContext) checkFlowWhenAction(flow MIRFlow, state string, action MIRFlowWhenAction) error {
	switch node := action.(type) {
	case MIRFlowWhenGoto, MIRFlowWhenSuspend:
		return nil
	case MIRFlowWhenReturn:
		return c.checkFlowExpr(flow, node.Value)
	case MIRFlowWhenBlock:
		for _, statement := range node.Statements {
			if err := c.checkFlowStmt(flow, state, statement, false); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("Verilog profile does not support FLOW when action %T", action)
	}
}

func (c *svContext) checkFlowExpr(flow MIRFlow, expression MIRFlowExpr) error {
	switch node := expression.(type) {
	case MIRFlowSharedExpr:
		if node.Fallible {
			return fmt.Errorf("Verilog profile does not support fallible FLOW expressions")
		}
		if len(node.Blocks) != 1 {
			return fmt.Errorf("Verilog M2 FLOW expressions must lower to one acyclic MIR block; statement-level if/when remain supported")
		}
		if _, ok := node.Blocks[0].Terminator.(MIRReturn); !ok {
			return fmt.Errorf("Verilog M2 FLOW expression must end in an ordinary MIR return")
		}
		pseudo := MIRFunction{Package: flow.Package, Name: flow.Name + "_state_expression", Return: node.Type, Locals: node.Locals, Blocks: node.Blocks}
		return c.checkFunction(pseudo)
	case MIRFlowUtilityWhenExpr:
		if _, err := c.typeWidth(node.ResultType); err != nil {
			return fmt.Errorf("utility result: %w", err)
		}
		for _, part := range []MIRFlowExpr{node.Hysteresis, node.MinCommit, node.Else} {
			if err := c.checkFlowExpr(flow, part); err != nil {
				return err
			}
		}
		for _, candidate := range node.Cases {
			for _, part := range []MIRFlowExpr{candidate.Condition, candidate.Score, candidate.Value} {
				if err := c.checkFlowExpr(flow, part); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("Verilog profile does not support FLOW expression %T", expression)
	}
}

func svFlowLiteralInt(expression MIRFlowExpr) (int64, bool) {
	shared, ok := expression.(MIRFlowSharedExpr)
	if !ok || len(shared.Blocks) != 1 || len(shared.Blocks[0].Statements) != 0 {
		return 0, false
	}
	returned, ok := shared.Blocks[0].Terminator.(MIRReturn)
	if !ok {
		return 0, false
	}
	literal, ok := returned.Value.(MIRLiteral)
	if !ok || !isSVIntType(literal.Type) {
		return 0, false
	}
	value, err := strconv.ParseInt(literal.Value, 10, 64)
	return value, err == nil
}

// M2 deliberately rejects any activation path that can cycle without first
// reaching a turn boundary. This is not HLS scheduling: it is the finite proof
// that direct combinational FLOW dispatch will terminate in one clock cycle.
func checkSVFlowActivationGraph(flow MIRFlow) error {
	stateIDs := map[string]int{}
	for i, state := range flow.States {
		stateIDs[state.Name] = i
	}
	edges := map[svFlowNode][]svFlowNode{}
	for stateIndex, state := range flow.States {
		for instruction, statement := range state.Statements {
			from := svFlowNode{stateIndex, instruction}
			next := svFlowNode{stateIndex, instruction + 1}
			edges[from] = svFlowStatementEdges(statement, next, stateIDs, flow)
		}
	}
	visiting, visited := map[svFlowNode]bool{}, map[svFlowNode]bool{}
	var visit func(svFlowNode) error
	visit = func(current svFlowNode) error {
		if current.state < 0 || current.state >= len(flow.States) || current.instruction >= len(flow.States[current.state].Statements) {
			return nil
		}
		if visiting[current] {
			return fmt.Errorf("recursive FLOW control can execute indefinitely within one turn near state %s instruction %d; insert suspend/yield or make the control path acyclic", flow.States[current.state].Name, current.instruction)
		}
		if visited[current] {
			return nil
		}
		visiting[current] = true
		for _, target := range edges[current] {
			if err := visit(target); err != nil {
				return err
			}
		}
		visiting[current] = false
		visited[current] = true
		return nil
	}
	for current := range edges {
		if err := visit(current); err != nil {
			return err
		}
	}
	return nil
}

func svFlowStatementEdges(statement MIRFlowStmt, next svFlowNode, stateIDs map[string]int, flow MIRFlow) []svFlowNode {
	switch value := statement.(type) {
	case MIRFlowGoto:
		return []svFlowNode{{state: stateIDs[value.Target], instruction: 0}}
	case MIRFlowSuspend, MIRFlowYield, MIRFlowReturn:
		return nil
	case MIRFlowResume:
		out := []svFlowNode{}
		for stateIndex, state := range flow.States {
			if stateHasRemember(state.Statements) {
				out = append(out, svFlowNode{state: stateIndex, instruction: 0})
			}
		}
		return out
	case MIRFlowIf:
		return append(svFlowBlockEdges(value.Then, next, stateIDs, flow), svFlowBlockEdges(value.Else, next, stateIDs, flow)...)
	case MIRFlowWhen:
		out := []svFlowNode{}
		for _, candidate := range value.Cases {
			out = append(out, svFlowActionEdges(candidate.Action, next, stateIDs, flow)...)
		}
		return append(out, svFlowActionEdges(value.Else, next, stateIDs, flow)...)
	default:
		return []svFlowNode{next}
	}
}

func svFlowBlockEdges(statements []MIRFlowStmt, next svFlowNode, stateIDs map[string]int, flow MIRFlow) []svFlowNode {
	if len(statements) == 0 {
		return []svFlowNode{next}
	}
	// Inline statements execute atomically. Only their first terminal transfer
	// affects the top-level activation graph; ordinary prefixes are irrelevant.
	for _, statement := range statements {
		edges := svFlowStatementEdges(statement, next, stateIDs, flow)
		switch statement.(type) {
		case MIRFlowGoto, MIRFlowSuspend, MIRFlowYield, MIRFlowResume, MIRFlowReturn, MIRFlowWhen:
			return edges
		}
	}
	return []svFlowNode{next}
}

func svFlowActionEdges(action MIRFlowWhenAction, next svFlowNode, stateIDs map[string]int, flow MIRFlow) []svFlowNode {
	switch value := action.(type) {
	case MIRFlowWhenGoto:
		return []svFlowNode{{state: stateIDs[value.Target], instruction: 0}}
	case MIRFlowWhenSuspend, MIRFlowWhenReturn:
		return nil
	case MIRFlowWhenBlock:
		return svFlowBlockEdges(value.Statements, next, stateIDs, flow)
	default:
		return []svFlowNode{next}
	}
}

func stateHasRemember(statements []MIRFlowStmt) bool {
	for _, statement := range statements {
		switch node := statement.(type) {
		case MIRFlowRemember:
			return true
		case MIRFlowIf:
			if stateHasRemember(node.Then) || stateHasRemember(node.Else) {
				return true
			}
		case MIRFlowWhen:
			for _, candidate := range node.Cases {
				if block, ok := candidate.Action.(MIRFlowWhenBlock); ok && stateHasRemember(block.Statements) {
					return true
				}
			}
			if block, ok := node.Else.(MIRFlowWhenBlock); ok && stateHasRemember(block.Statements) {
				return true
			}
		}
	}
	return false
}

func svFlowStateWidth(flow MIRFlow) int {
	if len(flow.States) <= 1 {
		return 1
	}
	return bits.Len(uint(len(flow.States) - 1))
}

func svFlowInstructionWidth(flow MIRFlow) int {
	max := 1
	for _, state := range flow.States {
		if len(state.Statements)+1 > max {
			max = len(state.Statements) + 1
		}
	}
	if max <= 1 {
		return 1
	}
	return bits.Len(uint(max - 1))
}

func collectSVFlowUtilitySites(flow MIRFlow) map[int]svFlowUtilitySite {
	sites := map[int]svFlowUtilitySite{}
	var visitExpr func(MIRFlowExpr)
	var visitStmt func(MIRFlowStmt)
	var visitAction func(MIRFlowWhenAction)
	visitExpr = func(expression MIRFlowExpr) {
		if utility, ok := expression.(MIRFlowUtilityWhenExpr); ok {
			sites[utility.SiteID] = svFlowUtilitySite{ID: utility.SiteID, Type: utility.ResultType, Persistent: utility.ControllerBound}
			visitExpr(utility.Hysteresis)
			visitExpr(utility.MinCommit)
			visitExpr(utility.Else)
			for _, candidate := range utility.Cases {
				visitExpr(candidate.Condition)
				visitExpr(candidate.Score)
				visitExpr(candidate.Value)
			}
		}
	}
	visitAction = func(action MIRFlowWhenAction) {
		switch node := action.(type) {
		case MIRFlowWhenReturn:
			visitExpr(node.Value)
		case MIRFlowWhenBlock:
			for _, statement := range node.Statements {
				visitStmt(statement)
			}
		}
	}
	visitStmt = func(statement MIRFlowStmt) {
		switch node := statement.(type) {
		case MIRFlowYield:
			visitExpr(node.Value)
		case MIRFlowFieldAssign:
			visitExpr(node.Value)
		case MIRFlowLetStmt:
			visitExpr(node.Value)
		case MIRFlowLocalAssign:
			visitExpr(node.Value)
		case MIRFlowReturn:
			if node.Value != nil {
				visitExpr(node.Value)
			}
		case MIRFlowIf:
			visitExpr(node.Condition)
			for _, branch := range [][]MIRFlowStmt{node.Then, node.Else} {
				for _, nested := range branch {
					visitStmt(nested)
				}
			}
		case MIRFlowFor:
			visitExpr(node.Start)
			visitExpr(node.End)
			visitExpr(node.Step)
			for _, nested := range node.Body {
				visitStmt(nested)
			}
		case MIRFlowWhen:
			for _, candidate := range node.Cases {
				visitExpr(candidate.Condition)
				visitAction(candidate.Action)
			}
			visitAction(node.Else)
		}
	}
	for _, state := range flow.States {
		for _, statement := range state.Statements {
			visitStmt(statement)
		}
	}
	return sites
}

func sortedSVFlowUtilitySites(sites map[int]svFlowUtilitySite) []svFlowUtilitySite {
	result := make([]svFlowUtilitySite, 0, len(sites))
	for _, site := range sites {
		result = append(result, site)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func sortedPersistentSVFlowUtilitySites(sites map[int]svFlowUtilitySite) []svFlowUtilitySite {
	all := sortedSVFlowUtilitySites(sites)
	result := all[:0]
	for _, site := range all {
		if site.Persistent {
			result = append(result, site)
		}
	}
	return result
}

func (c *svContext) emitFlowModule(b *strings.Builder, moduleName string, flow MIRFlow) error {
	emit := &svFlowEmit{
		c:              c,
		flow:           flow,
		stateNames:     map[string]string{},
		stateIDs:       map[string]int{},
		boardNames:     map[string]string{},
		boardNextNames: map[string]string{},
		parameterNames: map[string]string{},
		parameterPorts: map[string]string{},
		localNames:     map[string]string{},
		utilitySites:   collectSVFlowUtilitySites(flow),
	}
	used := map[string]int{
		"Clock": 1, "Reset": 1, "Done": 1, "Result": 1, "YieldValid": 1,
		"YieldValue": 1, "Suspended": 1, "Fault": 1, "StateView": 1,
		"InstructionView": 1, "HasResumeTarget": 1, "ResumeStateView": 1,
	}
	for index, state := range flow.States {
		emit.stateIDs[state.Name] = index
		emit.stateNames[state.Name] = uniqueSVName("State_"+sanitizeSVIdentifier(state.Name), used)
	}
	for _, parameter := range flow.Parameters {
		port := uniqueSVName(sanitizeSVIdentifier(sourceOrMIRName(parameter)), used)
		emit.parameterPorts[parameter.Name] = port
		emit.parameterNames[parameter.Name] = uniqueSVName("Param_"+sanitizeSVIdentifier(sourceOrMIRName(parameter)), used)
	}
	if flow.TurnInput != nil {
		emit.inputName = uniqueSVName("Turn_"+sanitizeSVIdentifier(sourceOrMIRName(*flow.TurnInput)), used)
	}
	for _, field := range flow.Board {
		name := uniqueSVName("Board_"+sanitizeSVIdentifier(sourceOrMIRName(field)), used)
		emit.boardNames[field.Name] = name
		emit.boardNextNames[field.Name] = uniqueSVName("Next"+name, used)
	}
	for _, local := range collectSVFlowLocals(flow) {
		emit.localNames[local.Name] = uniqueSVName("Local_"+sanitizeSVIdentifier(sourceOrMIRName(local)), used)
	}

	stateWidth := svFlowStateWidth(flow)
	instructionWidth := svFlowInstructionWidth(flow)
	fmt.Fprintf(b, "module %s(\n", moduleName)
	ports := []string{"    input  logic Clock", "    input  logic Reset"}
	for _, parameter := range flow.Parameters {
		ports = append(ports, fmt.Sprintf("    input  %s %s", c.svType(parameter.Type), emit.parameterPorts[parameter.Name]))
	}
	if flow.TurnInput != nil {
		ports = append(ports, fmt.Sprintf("    input  %s %s", c.svType(flow.TurnInput.Type), emit.inputName))
	}
	ports = append(ports,
		"    output logic Done",
		"    output logic Suspended",
		"    output logic Fault",
		fmt.Sprintf("    output logic [%d:0] StateView", stateWidth-1),
		fmt.Sprintf("    output logic [%d:0] InstructionView", instructionWidth-1),
	)
	if flow.Return != "Void" {
		ports = append(ports, fmt.Sprintf("    output %s Result", c.svType(flow.Return)))
	}
	if flow.YieldType != "" {
		ports = append(ports, "    output logic YieldValid", fmt.Sprintf("    output %s YieldValue", c.svType(flow.YieldType)))
	}
	if flowNeedsResume(flow) {
		ports = append(ports, "    output logic HasResumeTarget", fmt.Sprintf("    output logic [%d:0] ResumeStateView", stateWidth-1))
	}
	for _, field := range flow.Board {
		ports = append(ports, fmt.Sprintf("    output %s %s", c.svType(field.Type), emit.boardNames[field.Name]))
	}
	for _, site := range sortedPersistentSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilitySite%d", site.ID)
		ports = append(ports,
			fmt.Sprintf("    output logic %sHasCurrent", prefix),
			fmt.Sprintf("    output %s %sCurrent", c.svType(site.Type), prefix),
			fmt.Sprintf("    output logic signed [63:0] %sScore", prefix),
			fmt.Sprintf("    output logic signed [63:0] %sCommitAge", prefix),
		)
	}
	b.WriteString(strings.Join(ports, ",\n"))
	b.WriteString("\n);\n\n")

	fmt.Fprintf(b, "typedef enum logic [%d:0] {\n", stateWidth-1)
	stateDecls := make([]string, len(flow.States))
	for index, state := range flow.States {
		stateDecls[index] = fmt.Sprintf("    %s = %d'd%d", emit.stateNames[state.Name], stateWidth, index)
	}
	b.WriteString(strings.Join(stateDecls, ",\n"))
	b.WriteString("\n} StateT;\n\nStateT State;\nStateT NextState;\n")
	fmt.Fprintf(b, "logic [%d:0] Instruction;\nlogic [%d:0] NextInstruction;\n", instructionWidth-1, instructionWidth-1)
	b.WriteString("logic NextDone;\nlogic NextSuspended;\nlogic NextFault;\n")
	if flow.Return != "Void" {
		fmt.Fprintf(b, "%s NextResult;\n", c.svType(flow.Return))
	}
	if flow.YieldType != "" {
		b.WriteString("logic NextYieldValid;\n")
		fmt.Fprintf(b, "%s NextYieldValue;\n", c.svType(flow.YieldType))
	}
	if flowNeedsResume(flow) {
		b.WriteString("logic NextHasResumeTarget;\nStateT ResumeState;\nStateT NextResumeState;\n")
	}
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(b, "%s %s;\n", c.svType(parameter.Type), emit.parameterNames[parameter.Name])
	}
	for _, field := range flow.Board {
		fmt.Fprintf(b, "%s %s;\n", c.svType(field.Type), emit.boardNextNames[field.Name])
	}
	for _, site := range sortedPersistentSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilitySite%d", site.ID)
		fmt.Fprintf(b, "logic Next%sHasCurrent;\n%s Next%sCurrent;\nlogic signed [63:0] Next%sScore;\nlogic signed [63:0] Next%sCommitAge;\n", prefix, c.svType(site.Type), prefix, prefix, prefix)
	}
	for _, site := range sortedSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilityComb%d", site.ID)
		fmt.Fprintf(b, "logic %sValid;\nlogic %sCurrentStillValid;\n%s %sBestValue;\nlogic signed [63:0] %sBestScore;\nlogic signed [63:0] %sHysteresis;\nlogic signed [63:0] %sMinCommit;\n", prefix, prefix, c.svType(site.Type), prefix, prefix, prefix, prefix)
	}
	for _, local := range collectSVFlowLocals(flow) {
		fmt.Fprintf(b, "%s %s;\n", c.svType(local.Type), emit.localNames[local.Name])
	}
	b.WriteString("\nassign StateView = State;\nassign InstructionView = Instruction;\n")
	if flowNeedsResume(flow) {
		b.WriteString("assign ResumeStateView = ResumeState;\n")
	}

	// Every ordinary helper remains an ordinary MIR function. Emitting it inside
	// the FLOW module does not create a second expression implementation.
	for _, helper := range c.module.Functions {
		b.WriteByte('\n')
		if err := c.emitHelper(b, helper); err != nil {
			return err
		}
	}

	b.WriteString("\nalways_comb begin : oct_flow_next\n")
	emit.indent = "    "
	b.WriteString("    integer __oct_dispatch;\n    logic __oct_running;\n    logic __oct_redispatch;\n")
	b.WriteString("    NextState = State;\n    NextInstruction = Instruction;\n    NextDone = Done;\n    NextSuspended = 1'b0;\n    NextFault = Fault;\n")
	if flow.Return != "Void" {
		b.WriteString("    NextResult = Result;\n")
	}
	if flow.YieldType != "" {
		b.WriteString("    NextYieldValid = 1'b0;\n    NextYieldValue = YieldValue;\n")
	}
	if flowNeedsResume(flow) {
		b.WriteString("    NextHasResumeTarget = HasResumeTarget;\n    NextResumeState = ResumeState;\n")
	}
	for _, field := range flow.Board {
		fmt.Fprintf(b, "    %s = %s;\n", emit.boardNextNames[field.Name], emit.boardNames[field.Name])
	}
	for _, site := range sortedPersistentSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilitySite%d", site.ID)
		fmt.Fprintf(b, "    Next%sHasCurrent = %sHasCurrent;\n    Next%sCurrent = %sCurrent;\n    Next%sScore = %sScore;\n    Next%sCommitAge = %sCommitAge;\n", prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)
	}
	for _, site := range sortedSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilityComb%d", site.ID)
		fmt.Fprintf(b, "    %sValid = 1'b0;\n    %sCurrentStillValid = 1'b0;\n    %sBestValue = '0;\n    %sBestScore = '0;\n    %sHysteresis = '0;\n    %sMinCommit = '0;\n", prefix, prefix, prefix, prefix, prefix, prefix)
	}
	for _, local := range collectSVFlowLocals(flow) {
		fmt.Fprintf(b, "    %s = '0;\n", emit.localNames[local.Name])
	}
	b.WriteString("    __oct_running = !Done && !Fault;\n    __oct_redispatch = 1'b0;\n")
	dispatchSteps := svFlowDispatchBound(flow)
	fmt.Fprintf(b, "    for (__oct_dispatch = 0; __oct_dispatch < %d; __oct_dispatch = __oct_dispatch + 1) begin : oct_flow_dispatch\n", dispatchSteps)
	b.WriteString("        if (__oct_running) begin\n            __oct_redispatch = 1'b0;\n            case (NextState)\n")
	emit.indent = "                "
	for _, state := range flow.States {
		fmt.Fprintf(b, "                %s: begin\n                    case (NextInstruction)\n", emit.stateNames[state.Name])
		for instruction, statement := range state.Statements {
			fmt.Fprintf(b, "                    %d'd%d: begin\n", instructionWidth, instruction)
			emit.indent = "                        "
			if err := emit.emitStatement(b, statement, state.Name); err != nil {
				return fmt.Errorf("flow %s.%s state %s: %w", flow.Package, flow.Name, state.Name, err)
			}
			b.WriteString("                        if (__oct_running && !__oct_redispatch) NextInstruction = NextInstruction + 1'b1;\n")
			b.WriteString("                    end\n")
		}
		b.WriteString("                    default: begin NextFault = 1'b1; __oct_running = 1'b0; end\n                    endcase\n                end\n")
	}
	b.WriteString("                default: begin NextFault = 1'b1; __oct_running = 1'b0; end\n            endcase\n        end\n    end\n")
	b.WriteString("    if (__oct_running) begin\n        NextFault = 1'b1;\n        __oct_running = 1'b0;\n    end\nend\n\n")

	b.WriteString("always_ff @(posedge Clock) begin\n    if (Reset) begin\n")
	fmt.Fprintf(b, "        State <= %s;\n        Instruction <= '0;\n", emit.stateNames[flow.EntryState])
	b.WriteString("        Done <= 1'b0;\n        Suspended <= 1'b0;\n        Fault <= 1'b0;\n")
	if flow.Return != "Void" {
		b.WriteString("        Result <= '0;\n")
	}
	if flow.YieldType != "" {
		b.WriteString("        YieldValid <= 1'b0;\n        YieldValue <= '0;\n")
	}
	if flowNeedsResume(flow) {
		b.WriteString("        HasResumeTarget <= 1'b0;\n        ResumeState <= ")
		b.WriteString(emit.stateNames[flow.EntryState] + ";\n")
	}
	for _, parameter := range flow.Parameters {
		fmt.Fprintf(b, "        %s <= %s;\n", emit.parameterNames[parameter.Name], emit.parameterPorts[parameter.Name])
	}
	for _, field := range flow.Board {
		fmt.Fprintf(b, "        %s <= '0;\n", emit.boardNames[field.Name])
	}
	for _, site := range sortedPersistentSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilitySite%d", site.ID)
		fmt.Fprintf(b, "        %sHasCurrent <= 1'b0;\n        %sCurrent <= '0;\n        %sScore <= '0;\n        %sCommitAge <= '0;\n", prefix, prefix, prefix, prefix)
	}
	b.WriteString("    end else begin\n        State <= NextState;\n        Instruction <= NextInstruction;\n        Done <= NextDone;\n        Suspended <= NextSuspended;\n        Fault <= NextFault;\n")
	if flow.Return != "Void" {
		b.WriteString("        Result <= NextResult;\n")
	}
	if flow.YieldType != "" {
		b.WriteString("        YieldValid <= NextYieldValid;\n        YieldValue <= NextYieldValue;\n")
	}
	if flowNeedsResume(flow) {
		b.WriteString("        HasResumeTarget <= NextHasResumeTarget;\n        ResumeState <= NextResumeState;\n")
	}
	for _, field := range flow.Board {
		fmt.Fprintf(b, "        %s <= %s;\n", emit.boardNames[field.Name], emit.boardNextNames[field.Name])
	}
	for _, site := range sortedPersistentSVFlowUtilitySites(emit.utilitySites) {
		prefix := fmt.Sprintf("UtilitySite%d", site.ID)
		fmt.Fprintf(b, "        %sHasCurrent <= Next%sHasCurrent;\n        %sCurrent <= Next%sCurrent;\n        %sScore <= Next%sScore;\n        %sCommitAge <= Next%sCommitAge;\n", prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)
	}
	b.WriteString("    end\nend\n\nendmodule\n")
	return nil
}

func flowNeedsResume(flow MIRFlow) bool {
	for _, state := range flow.States {
		if stateHasRemember(state.Statements) || stateHasResume(state.Statements) {
			return true
		}
	}
	return false
}

func collectSVFlowLocals(flow MIRFlow) []MIRField {
	locals := collectFlowLetLocals(flow)
	seen := map[string]bool{}
	for _, local := range locals {
		seen[local.Name] = true
	}
	var visit func([]MIRFlowStmt)
	visit = func(statements []MIRFlowStmt) {
		for _, statement := range statements {
			switch node := statement.(type) {
			case MIRFlowFor:
				if !seen[node.Name] {
					seen[node.Name] = true
					locals = append(locals, MIRField{Name: node.Name, Type: "Int"})
				}
				visit(node.Body)
			case MIRFlowIf:
				visit(node.Then)
				visit(node.Else)
			case MIRFlowWhen:
				for _, candidate := range node.Cases {
					if block, ok := candidate.Action.(MIRFlowWhenBlock); ok {
						visit(block.Statements)
					}
				}
				if block, ok := node.Else.(MIRFlowWhenBlock); ok {
					visit(block.Statements)
				}
			}
		}
	}
	for _, state := range flow.States {
		visit(state.Statements)
	}
	return locals
}

func stateHasResume(statements []MIRFlowStmt) bool {
	for _, statement := range statements {
		switch node := statement.(type) {
		case MIRFlowResume:
			return true
		case MIRFlowIf:
			if stateHasResume(node.Then) || stateHasResume(node.Else) {
				return true
			}
		case MIRFlowWhen:
			for _, candidate := range node.Cases {
				if block, ok := candidate.Action.(MIRFlowWhenBlock); ok && stateHasResume(block.Statements) {
					return true
				}
			}
			if block, ok := node.Else.(MIRFlowWhenBlock); ok && stateHasResume(block.Statements) {
				return true
			}
		}
	}
	return false
}

func svFlowDispatchBound(flow MIRFlow) int {
	count := 1
	for _, state := range flow.States {
		count += len(state.Statements)
	}
	if count > maxSVFlowDispatchSteps {
		return maxSVFlowDispatchSteps
	}
	return count
}

func (e *svFlowEmit) emitStatement(b *strings.Builder, statement MIRFlowStmt, currentState string) error {
	switch node := statement.(type) {
	case MIRFlowGoto:
		fmt.Fprintf(b, "%sNextState = %s;\n%sNextInstruction = '0;\n%s__oct_redispatch = 1'b1;\n", e.indent, e.stateNames[node.Target], e.indent, e.indent)
	case MIRFlowSuspend:
		fmt.Fprintf(b, "%sNextSuspended = 1'b1;\n%sNextInstruction = NextInstruction + 1'b1;\n%s__oct_running = 1'b0;\n", e.indent, e.indent, e.indent)
	case MIRFlowYield:
		if err := e.emitExpression(b, node.Value, "NextYieldValue"); err != nil {
			return err
		}
		fmt.Fprintf(b, "%sNextYieldValid = 1'b1;\n%sNextInstruction = NextInstruction + 1'b1;\n%s__oct_running = 1'b0;\n", e.indent, e.indent, e.indent)
	case MIRFlowRemember:
		fmt.Fprintf(b, "%sNextHasResumeTarget = 1'b1;\n%sNextResumeState = NextState;\n", e.indent, e.indent)
	case MIRFlowResume:
		fmt.Fprintf(b, "%sif (!NextHasResumeTarget) begin\n%s    NextFault = 1'b1;\n%s    __oct_running = 1'b0;\n%s end else begin\n%s    NextState = NextResumeState;\n%s    NextInstruction = '0;\n%s    NextHasResumeTarget = 1'b0;\n%s    NextResumeState = %s;\n%s    __oct_redispatch = 1'b1;\n%send\n", e.indent, e.indent, e.indent, e.indent, e.indent, e.indent, e.indent, e.indent, e.stateNames[e.flow.EntryState], e.indent, e.indent)
	case MIRFlowFieldAssign:
		return e.emitExpression(b, node.Value, e.boardNextNames[node.Field])
	case MIRFlowLetStmt:
		return e.emitExpression(b, node.Value, e.localNames[node.Name])
	case MIRFlowLocalAssign:
		return e.emitExpression(b, node.Value, e.localNames[node.Name])
	case MIRFlowFor:
		start, _ := svFlowLiteralInt(node.Start)
		end, _ := svFlowLiteralInt(node.End)
		step, _ := svFlowLiteralInt(node.Step)
		loopName := e.localNames[node.Name]
		comparison, operation := "<", "+"
		if node.Descending {
			comparison, operation = ">", "-"
		}
		fmt.Fprintf(b, "%sfor (%s = 64'sd%d; %s %s 64'sd%d; %s = %s %s 64'sd%d) begin\n", e.indent, loopName, start, loopName, comparison, end, loopName, loopName, operation, step)
		oldIndent := e.indent
		e.indent += "    "
		for _, nested := range node.Body {
			if err := e.emitInlineStatement(b, nested, currentState); err != nil {
				return err
			}
		}
		e.indent = oldIndent
		fmt.Fprintf(b, "%send\n", e.indent)
	case MIRFlowReturn:
		if node.Value != nil && e.flow.Return != "Void" {
			if err := e.emitExpression(b, node.Value, "NextResult"); err != nil {
				return err
			}
		}
		fmt.Fprintf(b, "%sNextDone = 1'b1;\n%s__oct_running = 1'b0;\n", e.indent, e.indent)
	case MIRFlowIf:
		condition, err := e.flowExpressionString(node.Condition)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%sif (%s) begin\n", e.indent, condition)
		oldIndent := e.indent
		e.indent += "    "
		for _, nested := range node.Then {
			if err := e.emitInlineStatement(b, nested, currentState); err != nil {
				return err
			}
		}
		e.indent = oldIndent
		if len(node.Else) > 0 {
			fmt.Fprintf(b, "%send else begin\n", e.indent)
			e.indent += "    "
			for _, nested := range node.Else {
				if err := e.emitInlineStatement(b, nested, currentState); err != nil {
					return err
				}
			}
			e.indent = oldIndent
		}
		fmt.Fprintf(b, "%send\n", e.indent)
	case MIRFlowWhen:
		oldIndent := e.indent
		for index, candidate := range node.Cases {
			condition, err := e.flowExpressionString(candidate.Condition)
			if err != nil {
				return err
			}
			keyword := "if"
			if index > 0 {
				keyword = "else if"
			}
			fmt.Fprintf(b, "%s%s (%s) begin\n", e.indent, keyword, condition)
			e.indent += "    "
			if err := e.emitWhenAction(b, candidate.Action, currentState); err != nil {
				return err
			}
			e.indent = oldIndent
			fmt.Fprintf(b, "%send ", e.indent)
		}
		b.WriteString("else begin\n")
		e.indent += "    "
		if err := e.emitWhenAction(b, node.Else, currentState); err != nil {
			return err
		}
		e.indent = oldIndent
		fmt.Fprintf(b, "%send\n", e.indent)
	default:
		return fmt.Errorf("emit unsupported FLOW statement %T", statement)
	}
	return nil
}

func (e *svFlowEmit) emitInlineStatement(b *strings.Builder, statement MIRFlowStmt, currentState string) error {
	fmt.Fprintf(b, "%sif (__oct_running && !__oct_redispatch) begin\n", e.indent)
	oldIndent := e.indent
	e.indent += "    "
	if err := e.emitStatement(b, statement, currentState); err != nil {
		return err
	}
	e.indent = oldIndent
	fmt.Fprintf(b, "%send\n", e.indent)
	return nil
}

func (e *svFlowEmit) emitWhenAction(b *strings.Builder, action MIRFlowWhenAction, currentState string) error {
	switch node := action.(type) {
	case MIRFlowWhenGoto:
		return e.emitStatement(b, MIRFlowGoto{Target: node.Target}, currentState)
	case MIRFlowWhenSuspend:
		return e.emitStatement(b, MIRFlowSuspend{}, currentState)
	case MIRFlowWhenReturn:
		return e.emitStatement(b, MIRFlowReturn{Value: node.Value}, currentState)
	case MIRFlowWhenBlock:
		for _, statement := range node.Statements {
			if err := e.emitInlineStatement(b, statement, currentState); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("emit unsupported FLOW when action %T", action)
	}
}

func (e *svFlowEmit) tempName(kind string) string {
	e.expressionSerial++
	return fmt.Sprintf("__oct_flow_%s_%d", kind, e.expressionSerial)
}

func (e *svFlowEmit) emitExpression(b *strings.Builder, expression MIRFlowExpr, target string) error {
	switch node := expression.(type) {
	case MIRFlowSharedExpr:
		return e.emitSharedExpression(b, node, target)
	case MIRFlowUtilityWhenExpr:
		return e.emitUtilityExpression(b, node, target)
	default:
		return fmt.Errorf("emit unsupported FLOW expression %T", expression)
	}
}

func (e *svFlowEmit) emitSharedExpression(b *strings.Builder, expression MIRFlowSharedExpr, target string) error {
	if len(expression.Blocks) == 1 {
		value, err := e.sharedExpressionString(expression)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%s%s = %s;\n", e.indent, target, value)
		return nil
	}

	// The legality boundary currently rejects this general CFG path for FLOW.
	// Keeping the emitter complete prevents accidental source reparsing when the
	// admitted expression surface is expanded in a later milestone.
	serial := e.expressionSerial + 1
	e.expressionSerial = serial
	label := fmt.Sprintf("oct_flow_expr_%d", serial)
	names := map[string]string{}
	used := map[string]int{}
	for _, local := range expression.Locals {
		names[local.Name] = uniqueSVName(sanitizeSVIdentifier(local.Name), used)
	}
	active := map[string]string{}
	for _, block := range expression.Blocks {
		active[block.Label] = uniqueSVName("__oct_active_"+sanitizeSVIdentifier(block.Label), used)
	}
	order, err := topologicalSVBlocks(MIRFunction{Package: e.flow.Package, Name: label, Return: expression.Type, Locals: expression.Locals, Blocks: expression.Blocks})
	if err != nil {
		return err
	}
	loops, skipped, _, err := analyzeStaticSVLoops(MIRFunction{Package: e.flow.Package, Name: label, Return: expression.Type, Locals: expression.Locals, Blocks: expression.Blocks})
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "%sbegin : %s\n", e.indent, label)
	for _, local := range expression.Locals {
		fmt.Fprintf(b, "%s    %s %s;\n", e.indent, e.c.svType(local.Type), names[local.Name])
	}
	for _, block := range expression.Blocks {
		fmt.Fprintf(b, "%s    logic %s;\n", e.indent, active[block.Label])
	}
	for _, local := range expression.Locals {
		fmt.Fprintf(b, "%s    %s = '0;\n", e.indent, names[local.Name])
	}
	for _, block := range expression.Blocks {
		fmt.Fprintf(b, "%s    %s = 1'b0;\n", e.indent, active[block.Label])
	}
	if len(expression.Blocks) > 0 {
		fmt.Fprintf(b, "%s    %s = 1'b1;\n", e.indent, active[expression.Blocks[0].Label])
	}
	for _, blockIndex := range order {
		block := expression.Blocks[blockIndex]
		if skipped[block.Label] {
			continue
		}
		fmt.Fprintf(b, "%s    if (%s) begin : oct_block_%s\n", e.indent, active[block.Label], sanitizeSVIdentifier(block.Label))
		for _, statement := range block.Statements {
			if err := e.emitSharedMIRStatement(b, statement, names, e.indent+"        "); err != nil {
				return err
			}
		}
		if loop, ok := loops[block.Label]; ok {
			body := expression.Blocks[blockIndexByLabel(MIRFunction{Blocks: expression.Blocks}, loop.bodyLabel)]
			fmt.Fprintf(b, "%s        for (%s = 64'sd%d; %s %s 64'sd%d; %s = %s %s 64'sd%d) begin\n", e.indent, names[loop.loopVar], loop.start, names[loop.loopVar], loop.operator, loop.end, names[loop.loopVar], names[loop.loopVar], map[string]string{"<": "+", ">": "-"}[loop.operator], loop.step)
			for _, statement := range body.Statements {
				if err := e.emitSharedMIRStatement(b, statement, names, e.indent+"            "); err != nil {
					return err
				}
			}
			fmt.Fprintf(b, "%s        end\n%s        %s = 1'b1;\n", e.indent, e.indent, active[loop.exitLabel])
		} else {
			switch terminator := block.Terminator.(type) {
			case MIRReturn:
				if terminator.Value != nil {
					value, err := e.emitSharedMIRValue(terminator.Value, names)
					if err != nil {
						return err
					}
					fmt.Fprintf(b, "%s        %s = %s;\n", e.indent, target, value)
				}
			case MIRJump:
				fmt.Fprintf(b, "%s        %s = 1'b1;\n", e.indent, active[terminator.Target])
			case MIRBranch:
				condition, err := e.emitSharedMIRValue(terminator.Cond, names)
				if err != nil {
					return err
				}
				fmt.Fprintf(b, "%s        if (%s) %s = 1'b1;\n%s        else %s = 1'b1;\n", e.indent, condition, active[terminator.TrueTarget], e.indent, active[terminator.FalseTarget])
			default:
				return fmt.Errorf("FLOW shared expression has unsupported terminator %T", block.Terminator)
			}
		}
		fmt.Fprintf(b, "%s    end\n", e.indent)
	}
	fmt.Fprintf(b, "%send\n", e.indent)
	return nil
}

func (e *svFlowEmit) sharedExpressionString(expression MIRFlowSharedExpr) (string, error) {
	values := map[string]string{}
	for _, statement := range expression.Blocks[0].Statements {
		switch node := statement.(type) {
		case MIRAssign:
			value, err := e.emitSharedMIRValue(node.Value, values)
			if err != nil {
				return "", err
			}
			values[node.Target] = value
		case MIRConstructRecord:
			value, err := e.emitSharedRecord(node, values)
			if err != nil {
				return "", err
			}
			values[node.Target] = value
		case MIRCall:
			arguments := make([]string, len(node.Args))
			for index, argument := range node.Args {
				value, err := e.emitSharedMIRValue(argument, values)
				if err != nil {
					return "", err
				}
				arguments[index] = value
			}
			values[node.Target] = fmt.Sprintf("%s(%s)", helperSVName(qualifyCallee(e.flow.Package, node.Callee)), strings.Join(arguments, ", "))
		default:
			return "", fmt.Errorf("FLOW shared expression contains unsupported MIR statement %T", statement)
		}
	}
	returned := expression.Blocks[0].Terminator.(MIRReturn)
	return e.emitSharedMIRValue(returned.Value, values)
}

func (e *svFlowEmit) emitSharedMIRStatement(b *strings.Builder, statement MIRStmt, names map[string]string, indent string) error {
	switch node := statement.(type) {
	case MIRAssign:
		value, err := e.emitSharedMIRValue(node.Value, names)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%s%s = %s;\n", indent, names[node.Target], value)
	case MIRConstructRecord:
		value, err := e.emitSharedRecord(node, names)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%s%s = %s;\n", indent, names[node.Target], value)
	case MIRCall:
		args := make([]string, len(node.Args))
		for index, argument := range node.Args {
			value, err := e.emitSharedMIRValue(argument, names)
			if err != nil {
				return err
			}
			args[index] = value
		}
		fmt.Fprintf(b, "%s%s = %s(%s);\n", indent, names[node.Target], helperSVName(qualifyCallee(e.flow.Package, node.Callee)), strings.Join(args, ", "))
	default:
		return fmt.Errorf("FLOW shared expression contains unsupported MIR statement %T", statement)
	}
	return nil
}

func (e *svFlowEmit) emitSharedRecord(statement MIRConstructRecord, names map[string]string) (string, error) {
	record := e.c.records[statement.TypeName]
	values := map[string]MIRValue{}
	for index, name := range statement.FieldNames {
		values[name] = statement.FieldVals[index]
	}
	parts := make([]string, 0, len(record.Fields))
	for index := len(record.Fields) - 1; index >= 0; index-- {
		field := record.Fields[index]
		value, ok := values[field.Name]
		if !ok {
			return "", fmt.Errorf("record %s missing field %s", statement.TypeName, field.Name)
		}
		emitted, err := e.emitSharedMIRValue(qualifyRecordValue(value, statement.TypeName, field.Type), names)
		if err != nil {
			return "", err
		}
		parts = append(parts, emitted)
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

func (e *svFlowEmit) emitSharedMIRValue(value MIRValue, names map[string]string) (string, error) {
	if external, ok := e.externalFlowValue(value); ok {
		return external, nil
	}
	switch node := value.(type) {
	case MIRLiteral:
		return e.c.emitValue(node, names)
	case MIRLocal:
		if name, ok := names[node.Name]; ok {
			return name, nil
		}
		if name, ok := e.localNames[node.Name]; ok {
			return name, nil
		}
		return "", fmt.Errorf("unresolved FLOW expression local %s", node.Name)
	case MIRUnary:
		inner, err := e.emitSharedMIRValue(node.Value, names)
		return "(" + node.Op + inner + ")", err
	case MIRBinary:
		left, err := e.emitSharedMIRValue(node.Left, names)
		if err != nil {
			return "", err
		}
		right, err := e.emitSharedMIRValue(node.Right, names)
		if err != nil {
			return "", err
		}
		return "(" + left + " " + node.Op + " " + right + ")", nil
	case MIRFieldAccess:
		target, err := e.emitSharedMIRValue(node.Target, names)
		if err != nil {
			return "", err
		}
		offset, width, err := e.c.recordFieldSlice(e.flowValueType(node.Target), node.Field)
		if err != nil {
			return "", err
		}
		return svSlice(target, offset, width), nil
	case MIRClone:
		return e.emitSharedMIRValue(node.Value, names)
	case MIREnumValue:
		return e.emitSharedEnumValue(node, names)
	case MIREnumPayload:
		target, err := e.emitSharedMIRValue(node.Value, names)
		if err != nil {
			return "", err
		}
		width, err := e.c.typeWidth(node.PayloadType)
		if err != nil {
			return "", err
		}
		return svSlice(target, enumTagWidth(e.c.enums[mirValueType(node.Value)]), width), nil
	case MIRIntrinsicValue:
		if node.Kind != "enum-is" || len(node.Args) != 1 || len(node.Metadata) < 2 {
			return "", fmt.Errorf("emit unsupported FLOW intrinsic %s", node.Kind)
		}
		target, err := e.emitSharedMIRValue(node.Args[0], names)
		if err != nil {
			return "", err
		}
		enum := e.c.enums[node.Metadata[0]]
		tag := -1
		for index, variant := range enum.Variants {
			if variant.Name == node.Metadata[1] {
				tag = index
			}
		}
		width := enumTagWidth(enum)
		return fmt.Sprintf("(%s == %d'd%d)", svSlice(target, 0, width), width, tag), nil
	default:
		return "", fmt.Errorf("emit unsupported FLOW MIR value %T", value)
	}
}

func (e *svFlowEmit) flowValueType(value MIRValue) string {
	if field, ok := value.(MIRFieldAccess); ok {
		if root, rootOK := field.Target.(MIRLocal); rootOK && root.Name == "f" {
			for _, parameter := range e.flow.Parameters {
				if parameter.Name == field.Field {
					return parameter.Type
				}
			}
			if e.flow.TurnInput != nil && e.flow.TurnInput.Name == field.Field {
				return e.flow.TurnInput.Type
			}
		}
		if board, boardOK := field.Target.(MIRFieldAccess); boardOK {
			if root, rootOK := board.Target.(MIRLocal); rootOK && root.Name == "f" && board.Field == "board" {
				for _, boardField := range e.flow.Board {
					if boardField.Name == field.Field {
						return boardField.Type
					}
				}
			}
		}
	}
	return mirValueType(value)
}

func (e *svFlowEmit) externalFlowValue(value MIRValue) (string, bool) {
	field, ok := value.(MIRFieldAccess)
	if !ok {
		return "", false
	}
	if root, ok := field.Target.(MIRLocal); ok && root.Name == "f" {
		if name, found := e.parameterNames[field.Field]; found {
			return name, true
		}
		if e.flow.TurnInput != nil && field.Field == e.flow.TurnInput.Name {
			return e.inputName, true
		}
	}
	if board, ok := field.Target.(MIRFieldAccess); ok {
		if root, rootOK := board.Target.(MIRLocal); rootOK && root.Name == "f" && board.Field == "board" {
			name, found := e.boardNextNames[field.Field]
			return name, found
		}
	}
	return "", false
}

func (e *svFlowEmit) emitSharedEnumValue(value MIREnumValue, names map[string]string) (string, error) {
	enum := e.c.enums[value.EnumType]
	tag, payloadType := -1, ""
	for index, variant := range enum.Variants {
		if variant.Name == value.Variant {
			tag, payloadType = index, variant.PayloadType
		}
	}
	if tag < 0 {
		return "", fmt.Errorf("unknown enum variant %s.%s", value.EnumType, value.Variant)
	}
	tagWidth := enumTagWidth(enum)
	totalWidth, _ := e.c.typeWidth(value.EnumType)
	payloadWidth := totalWidth - tagWidth
	parts := []string{}
	if payloadWidth > 0 {
		if value.Payload == nil {
			parts = append(parts, fmt.Sprintf("%d'b0", payloadWidth))
		} else {
			payload, err := e.emitSharedMIRValue(value.Payload, names)
			if err != nil {
				return "", err
			}
			actual, _ := e.c.typeWidth(payloadType)
			if actual < payloadWidth {
				payload = fmt.Sprintf("{%d'b0, %s}", payloadWidth-actual, payload)
			}
			parts = append(parts, payload)
		}
	}
	parts = append(parts, fmt.Sprintf("%d'd%d", tagWidth, tag))
	return "{" + strings.Join(parts, ", ") + "}", nil
}

func (e *svFlowEmit) emitUtilityExpression(b *strings.Builder, utility MIRFlowUtilityWhenExpr, target string) error {
	prefix := fmt.Sprintf("UtilityComb%d", utility.SiteID)
	valid := prefix + "Valid"
	currentStillValid := prefix + "CurrentStillValid"
	bestValue := prefix + "BestValue"
	bestScore := prefix + "BestScore"
	hysteresis := prefix + "Hysteresis"
	minCommit := prefix + "MinCommit"
	hysteresisValue, err := e.flowExpressionString(utility.Hysteresis)
	if err != nil {
		return err
	}
	minCommitValue, err := e.flowExpressionString(utility.MinCommit)
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "%s%s = %s;\n%s%s = %s;\n", e.indent, hysteresis, hysteresisValue, e.indent, minCommit, minCommitValue)
	for _, candidate := range utility.Cases {
		condition, err := e.flowExpressionString(candidate.Condition)
		if err != nil {
			return err
		}
		score, err := e.flowExpressionString(candidate.Score)
		if err != nil {
			return err
		}
		value, err := e.flowExpressionString(candidate.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(b, "%sif (%s) begin\n%s    if (!%s || %s > %s) begin\n%s        %s = 1'b1;\n%s        %s = %s;\n%s        %s = %s;\n%s    end\n", e.indent, condition, e.indent, valid, score, bestScore, e.indent, valid, e.indent, bestValue, value, e.indent, bestScore, score, e.indent)
		if utility.ControllerBound {
			statePrefix := fmt.Sprintf("UtilitySite%d", utility.SiteID)
			fmt.Fprintf(b, "%s    if (Next%sHasCurrent && %s == Next%sCurrent) %s = 1'b1;\n", e.indent, statePrefix, value, statePrefix, currentStillValid)
		}
		fmt.Fprintf(b, "%send\n", e.indent)
	}
	elseValue, err := e.flowExpressionString(utility.Else)
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "%sif (!%s) begin\n%s    %s = 1'b1;\n%s    %s = %s;\n%s    %s = 64'sd0;\n%send\n", e.indent, valid, e.indent, valid, e.indent, bestValue, elseValue, e.indent, bestScore, e.indent)
	if utility.ControllerBound {
		statePrefix := fmt.Sprintf("UtilitySite%d", utility.SiteID)
		fmt.Fprintf(b, "%sif (Next%sHasCurrent && %s && ((Next%sCommitAge < %s) || (%s <= Next%sScore + %s))) begin\n%s    %s = Next%sCurrent;\n%s    %s = Next%sScore;\n%send\n", e.indent, statePrefix, currentStillValid, statePrefix, minCommit, bestScore, statePrefix, hysteresis, e.indent, bestValue, statePrefix, e.indent, bestScore, statePrefix, e.indent)
		fmt.Fprintf(b, "%sif (!Next%sHasCurrent || Next%sCurrent != %s) begin\n%s    Next%sHasCurrent = 1'b1;\n%s    Next%sCurrent = %s;\n%s    Next%sScore = %s;\n%s    Next%sCommitAge = 64'sd1;\n%send else begin\n%s    Next%sScore = %s;\n%s    Next%sCommitAge = Next%sCommitAge + 64'sd1;\n%send\n", e.indent, statePrefix, statePrefix, bestValue, e.indent, statePrefix, e.indent, statePrefix, bestValue, e.indent, statePrefix, bestScore, e.indent, statePrefix, e.indent, e.indent, statePrefix, bestScore, e.indent, statePrefix, statePrefix, e.indent)
	}
	fmt.Fprintf(b, "%s%s = %s;\n", e.indent, target, bestValue)
	return nil
}

func (e *svFlowEmit) flowExpressionString(expression MIRFlowExpr) (string, error) {
	shared, ok := expression.(MIRFlowSharedExpr)
	if !ok {
		return "", fmt.Errorf("nested utility expressions are not supported in Verilog M2 policy operands")
	}
	return e.sharedExpressionString(shared)
}
