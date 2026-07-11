package validate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// FlowTerminatorKind is the backend-neutral, validated control contract for a
// flow state. M31b consumes this data; it must never rediscover source order.
type FlowTerminatorKind string

const (
	FlowFallthrough FlowTerminatorKind = "fallthrough"
	FlowPush        FlowTerminatorKind = "push"
	FlowPop         FlowTerminatorKind = "pop"
	FlowGoto        FlowTerminatorKind = "goto"
	FlowFinish      FlowTerminatorKind = "finish"
)

const FlowCompleteStateID = -1

type ValidatedFlowTerminator struct {
	Kind     FlowTerminatorKind
	Target   int // FlowCompleteStateID when no target exists.
	ReturnTo int // resolved ordinary successor for push; otherwise FlowCompleteStateID.
	Span     source.Span
}

type ValidatedFlowState struct {
	ID                  int
	Name                string
	NameSpan            source.Span
	Statements          []ast.Stmt // transition is excluded; terminator owns it.
	Terminator          ValidatedFlowTerminator
	HasWorkgroupBarrier bool
	Reachable           bool
	ReachableDepths     []uint32
	SourceSpan          source.Span
}

type ValidatedFlow struct {
	Name          string
	Entry         int
	States        []ValidatedFlowState
	MaxStackDepth uint32
	HasPushPop    bool
	HasGoto       bool
}

type FlowIssue struct {
	Span     source.Span
	Code     string
	Severity diagnostic.Severity
	Message  string
	Related  []diagnostic.Related
}

// ValidateFlow establishes the complete M31a static model. It is exported so
// lowering can receive this validated representation directly in M31b without
// reparsing source or resolving state names from the AST again.
func ValidateFlow(flow ast.FlowStmt) (ValidatedFlow, []FlowIssue) {
	out := ValidatedFlow{Name: flow.Name, Entry: 0}
	issues := []FlowIssue{}
	nameToID := make(map[string]int, len(flow.States))
	for i, state := range flow.States {
		if prior, exists := nameToID[state.Name]; exists {
			issues = append(issues, FlowIssue{
				Span:     stateNameSpan(state),
				Code:     "SDSL-V3101",
				Severity: diagnostic.SeverityError,
				Message:  fmt.Sprintf("flow %s: duplicate state name %s", flow.Name, state.Name),
				Related:  []diagnostic.Related{{Message: "first state is here", Span: stateNameSpan(flow.States[prior])}},
			})
			continue
		}
		nameToID[state.Name] = i
	}
	out.States = make([]ValidatedFlowState, len(flow.States))
	for i, state := range flow.States {
		body, term, localIssues := flowStateTerminator(state, i, len(flow.States), nameToID)
		issues = append(issues, localIssues...)
		out.States[i] = ValidatedFlowState{
			ID:                  i,
			Name:                state.Name,
			NameSpan:            stateNameSpan(state),
			Statements:          body,
			Terminator:          term,
			HasWorkgroupBarrier: blockHasBarrier(state.Body),
			SourceSpan:          state.Span,
		}
		switch term.Kind {
		case FlowPush, FlowPop:
			out.HasPushPop = true
		case FlowGoto:
			out.HasGoto = true
		}
	}
	if len(issues) != 0 {
		return out, issues
	}
	issues = append(issues, rejectPushCycles(out)...)
	if len(issues) != 0 {
		return out, issues
	}
	analysis := analyzeFlowStack(out)
	out.MaxStackDepth = analysis.maxDepth
	for id, depths := range analysis.depths {
		if len(depths) == 0 {
			continue
		}
		out.States[id].Reachable = true
		out.States[id].ReachableDepths = depths
	}
	issues = append(issues, analysis.issues...)
	for _, state := range out.States {
		if !state.Reachable {
			issues = append(issues, FlowIssue{
				Span:     state.NameSpan,
				Code:     "SDSL-V3111",
				Severity: diagnostic.SeverityError,
				Message:  fmt.Sprintf("flow %s state %s is unreachable", out.Name, state.Name),
			})
		}
	}
	return out, issues
}

func stateNameSpan(state ast.StateBlock) source.Span {
	if state.NameSpan.Known() {
		return state.NameSpan
	}
	return state.Span
}

func flowStateTerminator(state ast.StateBlock, stateID, stateCount int, names map[string]int) ([]ast.Stmt, ValidatedFlowTerminator, []FlowIssue) {
	ordinary := FlowCompleteStateID
	if stateID+1 < stateCount {
		ordinary = stateID + 1
	}
	term := ValidatedFlowTerminator{Kind: FlowFallthrough, Target: ordinary, ReturnTo: FlowCompleteStateID, Span: state.Span}
	body := state.Body.Statements
	issues := []FlowIssue{}
	for i, stmt := range body {
		if nestedTransition(stmt) {
			issues = append(issues, issue(ast.StmtSpan(stmt), "SDSL-V3102", "flow transition must be the final top-level statement of a state"))
		}
		if kind, target, targetSpan, ok := directTransition(stmt); ok {
			if i != len(body)-1 {
				issues = append(issues, issue(ast.StmtSpan(body[i+1]), "SDSL-V3103", "statement is unreachable after flow transition"))
			}
			term = ValidatedFlowTerminator{Kind: kind, Target: FlowCompleteStateID, ReturnTo: FlowCompleteStateID, Span: ast.StmtSpan(stmt)}
			if kind == FlowPush || kind == FlowGoto {
				id, exists := names[target]
				if !exists {
					issues = append(issues, issue(targetSpan, "SDSL-V3104", fmt.Sprintf("unknown %s target %s", kind, target)))
				} else {
					term.Target = id
				}
			}
			if kind == FlowPush {
				term.ReturnTo = ordinary
			}
			return body[:i], term, issues
		}
	}
	return body, term, issues
}

func issue(span source.Span, code, message string) FlowIssue {
	return FlowIssue{Span: span, Code: code, Severity: diagnostic.SeverityError, Message: message}
}

func directTransition(stmt ast.Stmt) (FlowTerminatorKind, string, source.Span, bool) {
	switch s := stmt.(type) {
	case ast.PushFlowStateStmt:
		return FlowPush, s.Target, s.TargetSpan, true
	case ast.PopFlowStateStmt:
		return FlowPop, "", source.Span{}, true
	case ast.GotoFlowStateStmt:
		return FlowGoto, s.Target, s.TargetSpan, true
	case ast.FinishFlowStmt:
		return FlowFinish, "", source.Span{}, true
	}
	return "", "", source.Span{}, false
}

func nestedTransition(stmt ast.Stmt) bool {
	var block func(ast.Block) bool
	block = func(b ast.Block) bool {
		for _, x := range b.Statements {
			if _, _, _, ok := directTransition(x); ok || nestedTransition(x) {
				return true
			}
		}
		return false
	}
	switch s := stmt.(type) {
	case ast.IfStmt:
		return block(s.ThenBody) || (s.ElseBody != nil && block(*s.ElseBody))
	case ast.GuardWhenStmt:
		for _, c := range s.Cases {
			if block(c.Body) {
				return true
			}
		}
		return s.ElseBody != nil && block(*s.ElseBody)
	case ast.ForStmt:
		return block(s.Body)
	case ast.ComptimeIfStmt:
		return block(s.ThenBody) || (s.ElseBody != nil && block(*s.ElseBody))
	case ast.ComptimeForStmt:
		return block(s.Body)
	}
	return false
}

func rejectPushCycles(flow ValidatedFlow) []FlowIssue {
	color := make([]uint8, len(flow.States))
	path := []int{}
	issues := []FlowIssue{}
	var visit func(int)
	visit = func(id int) {
		color[id] = 1
		path = append(path, id)
		if t := flow.States[id].Terminator; t.Kind == FlowPush && t.Target >= 0 {
			if color[t.Target] == 1 {
				start := 0
				for path[start] != t.Target {
					start++
				}
				names := make([]string, 0, len(path)-start+1)
				related := make([]diagnostic.Related, 0, len(path)-start)
				for _, x := range path[start:] {
					names = append(names, flow.States[x].Name)
					related = append(related, diagnostic.Related{Message: "push-cycle state is here", Span: flow.States[x].NameSpan})
				}
				names = append(names, flow.States[t.Target].Name)
				issues = append(issues, FlowIssue{
					Span:     t.Span,
					Code:     "SDSL-V3105",
					Severity: diagnostic.SeverityError,
					Message:  "push cycle produces unbounded flow stack: " + strings.Join(names, " -> "),
					Related:  related,
				})
			} else if color[t.Target] == 0 {
				visit(t.Target)
			}
		}
		path = path[:len(path)-1]
		color[id] = 2
	}
	for i := range flow.States {
		if color[i] == 0 {
			visit(i)
		}
	}
	return issues
}

type flowConfig struct {
	state   int
	returns []int
}

type flowAnalysis struct {
	maxDepth uint32
	depths   map[int][]uint32
	issues   []FlowIssue
}

func analyzeFlowStack(flow ValidatedFlow) flowAnalysis {
	queue := []flowConfig{{state: flow.Entry}}
	seen := map[string]bool{}
	shapeByState := make([]map[string][]int, len(flow.States))
	depthByState := make([]map[int]struct{}, len(flow.States))
	var max uint32
	issues := []FlowIssue{}
	for len(queue) != 0 {
		c := queue[0]
		queue = queue[1:]
		if c.state == FlowCompleteStateID {
			continue
		}
		if len(c.returns) > len(flow.States) {
			issues = append(issues, issue(flow.States[c.state].SourceSpan, "SDSL-V3106", "flow stack is unbounded"))
			continue
		}
		key := flowConfigKey(c.state, c.returns)
		if seen[key] {
			continue
		}
		seen[key] = true
		if uint32(len(c.returns)) > max {
			max = uint32(len(c.returns))
		}
		if shapeByState[c.state] == nil {
			shapeByState[c.state] = map[string][]int{}
			depthByState[c.state] = map[int]struct{}{}
		}
		shapeByState[c.state][stackKey(c.returns)] = append([]int(nil), c.returns...)
		depthByState[c.state][len(c.returns)] = struct{}{}
		t := flow.States[c.state].Terminator
		switch t.Kind {
		case FlowFallthrough:
			if t.Target == FlowCompleteStateID {
				if len(c.returns) != 0 {
					issues = append(issues, issue(t.Span, "SDSL-V3107", "pushed subflow falls through to flow completion without pop or finish"))
				}
			} else {
				queue = append(queue, flowConfig{state: t.Target, returns: cloneStack(c.returns)})
			}
		case FlowPush:
			returns := append(cloneStack(c.returns), t.ReturnTo)
			queue = append(queue, flowConfig{state: t.Target, returns: returns})
		case FlowPop:
			if len(c.returns) == 0 {
				issues = append(issues, issue(t.Span, "SDSL-V3108", "pop may execute without a pushed return frame"))
				continue
			}
			returns := cloneStack(c.returns[:len(c.returns)-1])
			if next := c.returns[len(c.returns)-1]; next != FlowCompleteStateID {
				queue = append(queue, flowConfig{state: next, returns: returns})
			}
		case FlowGoto:
			if len(c.returns) != 0 {
				issues = append(issues, issue(t.Span, "SDSL-V3109", "goto from nonzero flow-stack depth is not allowed in M31a"))
			} else {
				queue = append(queue, flowConfig{state: t.Target, returns: nil})
			}
		case FlowFinish:
		}
	}
	depths := make(map[int][]uint32, len(flow.States))
	for id := range flow.States {
		if depthByState[id] == nil {
			continue
		}
		values := make([]uint32, 0, len(depthByState[id]))
		for depth := range depthByState[id] {
			values = append(values, uint32(depth))
		}
		slices.Sort(values)
		depths[id] = values
		if len(values) > 1 {
			issues = append(issues, FlowIssue{
				Span:     flow.States[id].NameSpan,
				Code:     "SDSL-V3110",
				Severity: diagnostic.SeverityError,
				Message:  fmt.Sprintf("flow %s state %s is reachable with mixed flow-stack depths %s", flow.Name, flow.States[id].Name, formatDepths(values)),
			})
		}
		if flow.States[id].HasWorkgroupBarrier && len(shapeByState[id]) > 1 {
			issues = append(issues, FlowIssue{
				Span:     flow.States[id].NameSpan,
				Code:     "SDSL-V3114",
				Severity: diagnostic.SeverityError,
				Message:  fmt.Sprintf("barrier state %s is reachable with ambiguous flow-stack shape", flow.States[id].Name),
			})
		}
	}
	return flowAnalysis{maxDepth: max, depths: depths, issues: issues}
}

func cloneStack(stack []int) []int {
	if len(stack) == 0 {
		return nil
	}
	return append([]int(nil), stack...)
}

func flowConfigKey(state int, stack []int) string {
	return strconv.Itoa(state) + ":" + stackKey(stack)
}

func stackKey(stack []int) string {
	if len(stack) == 0 {
		return "[]"
	}
	parts := make([]string, len(stack))
	for i, value := range stack {
		parts[i] = strconv.Itoa(value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func formatDepths(depths []uint32) string {
	parts := make([]string, len(depths))
	for i, depth := range depths {
		parts[i] = strconv.FormatUint(uint64(depth), 10)
	}
	return strings.Join(parts, ", ")
}

func blockHasBarrier(block ast.Block) bool {
	for _, stmt := range block.Statements {
		if stmtHasBarrier(stmt) {
			return true
		}
	}
	return false
}

func stmtHasBarrier(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case ast.ExprStmt:
		return exprHasBarrier(s.Value)
	case ast.LetStmt:
		return s.Value != nil && exprHasBarrier(s.Value)
	case ast.AssignStmt:
		return exprHasBarrier(s.Target) || exprHasBarrier(s.Value)
	case ast.GuardedWriteStmt:
		return exprHasBarrier(s.Target) || exprHasBarrier(s.Value) || exprHasBarrier(s.Condition)
	case ast.ReturnStmt:
		return s.Value != nil && exprHasBarrier(s.Value)
	case ast.IfStmt:
		return exprHasBarrier(s.Condition) || blockHasBarrier(s.ThenBody) || (s.ElseBody != nil && blockHasBarrier(*s.ElseBody))
	case ast.GuardWhenStmt:
		for _, c := range s.Cases {
			if exprHasBarrier(c.Condition) || blockHasBarrier(c.Body) {
				return true
			}
		}
		return s.ElseBody != nil && blockHasBarrier(*s.ElseBody)
	case ast.ForStmt:
		return exprHasBarrier(s.Start) || exprHasBarrier(s.End) || exprHasBarrier(s.Step) || blockHasBarrier(s.Body)
	case ast.ComptimeIfStmt:
		return exprHasBarrier(s.Condition) || blockHasBarrier(s.ThenBody) || (s.ElseBody != nil && blockHasBarrier(*s.ElseBody))
	case ast.ComptimeMatchStmt:
		if exprHasBarrier(s.Subject) {
			return true
		}
		for _, arm := range s.Arms {
			if (arm.Pattern != nil && exprHasBarrier(arm.Pattern)) || blockHasBarrier(arm.Body) {
				return true
			}
		}
	case ast.ComptimeWhenUtilityStmt:
		for _, c := range s.Cases {
			if (c.Condition != nil && exprHasBarrier(c.Condition)) || exprHasBarrier(c.Score) || blockHasBarrier(c.Body) {
				return true
			}
		}
		return s.ElseBody != nil && blockHasBarrier(*s.ElseBody)
	case ast.ComptimeForStmt:
		return exprHasBarrier(s.Start) || exprHasBarrier(s.End) || blockHasBarrier(s.Body)
	}
	return false
}

func exprHasBarrier(expr ast.Expr) bool {
	switch e := expr.(type) {
	case ast.CallExpr:
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && isBarrierBuiltin(id.Name) {
			return true
		}
		if exprHasBarrier(e.Callee) {
			return true
		}
		for _, arg := range e.Arguments {
			if exprHasBarrier(arg) {
				return true
			}
		}
	case ast.FieldAccessExpr:
		return exprHasBarrier(e.Target)
	case ast.IndexExpr:
		return exprHasBarrier(e.Target) || exprHasBarrier(e.Index) || (e.HasSecond && exprHasBarrier(e.Index2))
	case ast.GuardedReadExpr:
		return exprHasBarrier(e.Target) || exprHasBarrier(e.Condition) || exprHasBarrier(e.Fallback)
	case ast.BinaryExpr:
		return exprHasBarrier(e.Left) || exprHasBarrier(e.Right)
	case ast.UnaryExpr:
		return exprHasBarrier(e.Operand)
	case ast.ParenExpr:
		return exprHasBarrier(e.Inner)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			if exprHasBarrier(field.Value) {
				return true
			}
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			if exprHasBarrier(field.Value) {
				return true
			}
		}
	case ast.DeriveExpr:
		for _, field := range e.Fields {
			if exprHasBarrier(field.Value) {
				return true
			}
		}
	case ast.WhenUtilityExpr:
		if e.Else != nil && exprHasBarrier(e.Else) {
			return true
		}
		for _, c := range e.Cases {
			if exprHasBarrier(c.Value) || exprHasBarrier(c.Condition) || exprHasBarrier(c.Score) {
				return true
			}
		}
	case ast.WithExpr:
		if exprHasBarrier(e.Base) {
			return true
		}
		for _, update := range e.Updates {
			if exprHasBarrier(update.Value) {
				return true
			}
		}
	case ast.MatchExpr:
		if exprHasBarrier(e.Subject) {
			return true
		}
		for _, arm := range e.Arms {
			if exprHasBarrier(arm.Value) {
				return true
			}
		}
	case ast.ReductionExpr:
		return exprHasBarrier(e.Start) || exprHasBarrier(e.End) || exprHasBarrier(e.Step) || exprHasBarrier(e.Body)
	}
	return false
}
