package conceptvulkan

import (
	"fmt"
	"strings"
)

const (
	evt1AutomataMaxDecls              = 8
	evt1AutomataMaxMachinesPerDecl    = 16
	evt1AutomataMaxStatesPerMachine   = 32
	evt1AutomataMaxHandlersPerState   = 16
	evt1AutomataMaxStatesPerDecl      = 128
	evt1AutomataMaxTransitionsPerDecl = 256
	evt1AutomataMaxPushDepth          = 8
	evt1AutomataMaxValidationWork     = 2048
)

type EVT1QualifiedEnumMember struct {
	EnumName   string `json:"enum_name"`
	MemberName string `json:"member_name"`
	Span       Span   `json:"span"`
}

type EVT1StateRef struct {
	MachineName string `json:"machine_name,omitempty"`
	StateName   string `json:"state_name"`
	Span        Span   `json:"span"`
}

type EVT1TransitionKind string

const (
	EVT1TransitionGoto EVT1TransitionKind = "goto"
	EVT1TransitionPush EVT1TransitionKind = "push"
)

type EVT1TransitionDecl struct {
	Signal        EVT1QualifiedEnumMember `json:"signal"`
	Kind          EVT1TransitionKind      `json:"kind"`
	TargetState   EVT1StateRef            `json:"target_state,omitempty"`
	PushMachine   string                  `json:"push_machine,omitempty"`
	Continuation  EVT1StateRef            `json:"continuation_state,omitempty"`
	Span          Span                    `json:"span"`
}

type EVT1CompletionDecl struct {
	Kind string `json:"kind"`
	Span Span   `json:"span"`
}

type EVT1StateDecl struct {
	Name       string               `json:"name"`
	Initial    bool                 `json:"initial,omitempty"`
	Terminal   bool                 `json:"terminal,omitempty"`
	Handlers   []EVT1TransitionDecl `json:"handlers,omitempty"`
	Completion []EVT1CompletionDecl `json:"completion,omitempty"`
	Span       Span                 `json:"span"`
}

type EVT1MachineDecl struct {
	Name    string         `json:"name"`
	Initial bool           `json:"initial,omitempty"`
	States  []EVT1StateDecl `json:"states,omitempty"`
	Span    Span           `json:"span"`
}

type EVT1AutomataDecl struct {
	Name       string          `json:"name"`
	SignalType EVT1Type        `json:"signal_type"`
	Machines   []EVT1MachineDecl `json:"machines,omitempty"`
	Span       Span            `json:"span"`
}

type evt1AutomataInfo struct {
	Decl                 EVT1AutomataDecl
	SignalEnum           EVT1EnumDecl
	RootMachine          string
	MachineOrdinal       map[string]int
	StateOrdinal         map[string]map[string]int
	MachineReachable     map[string]bool
	StateReachable       map[string]map[string]bool
	ReachablePushTargets map[string]bool
	HasReachableFinish   map[string]bool
	HasReachablePop      map[string]bool
	MaxActiveDepth       int
	ContinuationCapacity int
	CompletionStepBound  int
	GraphIdentity        string
}

type EVT1MIRAutomata struct {
	Name            string               `json:"name"`
	SignalEnum      string               `json:"signal_enum"`
	RootMachine     string               `json:"root_machine"`
	MaxActiveDepth  int                  `json:"max_active_depth"`
	ContinuationCapacity int             `json:"continuation_capacity"`
	CompletionStepBound  int             `json:"completion_step_bound"`
	GraphIdentity   string               `json:"graph_identity"`
	SourceSpan      Span                 `json:"source_span"`
	Machines        []EVT1MIRMachine     `json:"machines,omitempty"`
}

type EVT1MIRMachine struct {
	Name       string              `json:"name"`
	Initial    bool                `json:"initial,omitempty"`
	RuntimeOrdinal int             `json:"runtime_ordinal"`
	Reachable  bool                `json:"reachable,omitempty"`
	SourceSpan Span                `json:"source_span"`
	States     []EVT1MIRState      `json:"states,omitempty"`
}

type EVT1MIRState struct {
	Name       string                 `json:"name"`
	Initial    bool                   `json:"initial,omitempty"`
	Terminal   bool                   `json:"terminal,omitempty"`
	RuntimeOrdinal int                `json:"runtime_ordinal"`
	Reachable  bool                   `json:"reachable,omitempty"`
	Completion string                 `json:"completion,omitempty"`
	SourceSpan Span                   `json:"source_span"`
	Handlers   []EVT1MIRTransition    `json:"handlers,omitempty"`
}

type EVT1MIRTransition struct {
	Signal            string `json:"signal"`
	Kind              string `json:"kind"`
	TargetState       string `json:"target_state,omitempty"`
	PushMachine       string `json:"push_machine,omitempty"`
	ContinuationState string `json:"continuation_state,omitempty"`
	SourceSpan        Span   `json:"source_span"`
}

type evt1AutomataPushEdge struct {
	FromMachine string
	ToMachine   string
	Span        Span
}

func evt1ValidateAutomataDecls(env *evt1Env, module EVT1Module) error {
	if len(module.Automata) > evt1AutomataMaxDecls {
		return evt1Diagnostic("CV4265", fmt.Sprintf("automata declaration count %d exceeds limit %d", len(module.Automata), evt1AutomataMaxDecls), module.Automata[evt1AutomataMaxDecls].Span)
	}
	for _, decl := range module.Automata {
		info, err := evt1ValidateAutomataDecl(env, decl)
		if err != nil {
			return err
		}
		env.automataInfo[decl.Name] = info
	}
	return nil
}

func evt1ValidateAutomataDecl(env *evt1Env, decl EVT1AutomataDecl) (*evt1AutomataInfo, error) {
	if err := validateKnownType(env, decl.SignalType, decl.SignalType.Span, "", false); err != nil {
		return nil, err
	}
	signalType, err := evt1ResolveType(env, nil, decl.SignalType)
	if err != nil {
		return nil, err
	}
	if signalType.Kind != EVT1TypeEnum {
		return nil, evt1Diagnostic("CV4241", fmt.Sprintf("automata %s signal type must name an enum, got %s", decl.Name, signalType.String()), decl.SignalType.Span)
	}
	if len(decl.Machines) == 0 {
		return nil, evt1Diagnostic("CV4242", fmt.Sprintf("automata %s requires at least one machine", decl.Name), decl.Span)
	}
	if len(decl.Machines) > evt1AutomataMaxMachinesPerDecl {
		return nil, evt1Diagnostic("CV4265", fmt.Sprintf("automata %s machine count %d exceeds limit %d", decl.Name, len(decl.Machines), evt1AutomataMaxMachinesPerDecl), decl.Span)
	}
	info := &evt1AutomataInfo{
		Decl:                 decl,
		SignalEnum:           env.enums[signalType.Name],
		MachineOrdinal:       map[string]int{},
		StateOrdinal:         map[string]map[string]int{},
		MachineReachable:     map[string]bool{},
		StateReachable:       map[string]map[string]bool{},
		ReachablePushTargets: map[string]bool{},
		HasReachableFinish:   map[string]bool{},
		HasReachablePop:      map[string]bool{},
	}
	for _, variant := range info.SignalEnum.Variants {
		if len(variant.Payload) > 0 {
			return nil, evt1Diagnostic("CV4269", fmt.Sprintf("automata %s signal enum %s must use only nullary variants in M1, but %s carries payload", decl.Name, info.SignalEnum.Name, variant.Name), variant.Span)
		}
	}
	machineNames := map[string]Span{}
	initialMachines := 0
	totalStates := 0
	totalTransitions := 0
	for machineIndex, machine := range decl.Machines {
		if _, exists := machineNames[machine.Name]; exists {
			return nil, evt1Diagnostic("CV4244", fmt.Sprintf("duplicate machine %s in automata %s", machine.Name, decl.Name), machine.Span)
		}
		info.MachineOrdinal[machine.Name] = machineIndex
		info.StateOrdinal[machine.Name] = map[string]int{}
		machineNames[machine.Name] = machine.Span
		if machine.Initial {
			initialMachines++
			info.RootMachine = machine.Name
		}
		if len(machine.States) == 0 {
			return nil, evt1Diagnostic("CV4245", fmt.Sprintf("machine %s in automata %s requires at least one state", machine.Name, decl.Name), machine.Span)
		}
		if len(machine.States) > evt1AutomataMaxStatesPerMachine {
			return nil, evt1Diagnostic("CV4265", fmt.Sprintf("machine %s state count %d exceeds limit %d", machine.Name, len(machine.States), evt1AutomataMaxStatesPerMachine), machine.Span)
		}
		totalStates += len(machine.States)
		stateNames := map[string]Span{}
		initialStates := 0
		for stateIndex, state := range machine.States {
			if _, exists := stateNames[state.Name]; exists {
				return nil, evt1Diagnostic("CV4247", fmt.Sprintf("duplicate state %s in machine %s", state.Name, machine.Name), state.Span)
			}
			info.StateOrdinal[machine.Name][state.Name] = stateIndex
			stateNames[state.Name] = state.Span
			if state.Initial {
				initialStates++
			}
			if state.Terminal {
				if len(state.Handlers) > 0 || len(state.Completion) != 1 {
					return nil, evt1Diagnostic("CV4249", fmt.Sprintf("terminal state %s in machine %s must contain exactly one completion action", state.Name, machine.Name), state.Span)
				}
			} else {
				if len(state.Handlers) == 0 {
					return nil, evt1Diagnostic("CV4250", fmt.Sprintf("nonterminal state %s in machine %s requires at least one handler", state.Name, machine.Name), state.Span)
				}
				if len(state.Completion) > 0 {
					return nil, evt1Diagnostic("CV4251", fmt.Sprintf("nonterminal state %s in machine %s cannot use pop or finish", state.Name, machine.Name), state.Completion[0].Span)
				}
			}
			if len(state.Handlers) > evt1AutomataMaxHandlersPerState {
				return nil, evt1Diagnostic("CV4265", fmt.Sprintf("state %s in machine %s handler count %d exceeds limit %d", state.Name, machine.Name, len(state.Handlers), evt1AutomataMaxHandlersPerState), state.Span)
			}
			totalTransitions += len(state.Handlers)
			seenSignals := map[string]Span{}
			for _, handler := range state.Handlers {
				key := handler.Signal.EnumName + "::" + handler.Signal.MemberName
				if other, exists := seenSignals[key]; exists {
					return nil, evt1Diagnostic("CV4253", fmt.Sprintf("duplicate transition for (%s, %s, %s)", machine.Name, state.Name, key), other)
				}
				seenSignals[key] = handler.Signal.Span
				if handler.Signal.EnumName != signalType.Name {
					return nil, evt1Diagnostic("CV4252", fmt.Sprintf("state handler in %s::%s must use signal enum %s, got %s", machine.Name, state.Name, signalType.Name, handler.Signal.EnumName), handler.Signal.Span)
				}
				variant, ok := evt1LookupVariant(info.SignalEnum, handler.Signal.MemberName)
				if !ok {
					return nil, evt1Diagnostic("CV4252", fmt.Sprintf("unknown signal member %s::%s", handler.Signal.EnumName, handler.Signal.MemberName), handler.Signal.Span)
				}
				if len(variant.Payload) > 0 {
					return nil, evt1Diagnostic("CV4252", fmt.Sprintf("signal handler %s::%s must use a nullary enum member in M0", handler.Signal.EnumName, handler.Signal.MemberName), handler.Signal.Span)
				}
			}
		}
		if initialStates != 1 {
			return nil, evt1Diagnostic("CV4246", fmt.Sprintf("machine %s in automata %s requires exactly one initial state", machine.Name, decl.Name), machine.Span)
		}
	}
	if totalStates > evt1AutomataMaxStatesPerDecl {
		return nil, evt1Diagnostic("CV4265", fmt.Sprintf("automata %s total state count %d exceeds limit %d", decl.Name, totalStates, evt1AutomataMaxStatesPerDecl), decl.Span)
	}
	if totalTransitions > evt1AutomataMaxTransitionsPerDecl {
		return nil, evt1Diagnostic("CV4265", fmt.Sprintf("automata %s total transition count %d exceeds limit %d", decl.Name, totalTransitions, evt1AutomataMaxTransitionsPerDecl), decl.Span)
	}
	if initialMachines != 1 {
		return nil, evt1Diagnostic("CV4243", fmt.Sprintf("automata %s requires exactly one initial machine", decl.Name), decl.Span)
	}
	if err := evt1ValidateAutomataReferences(info); err != nil {
		return nil, err
	}
	if err := evt1ValidateAutomataPushCycles(info); err != nil {
		return nil, err
	}
	if err := evt1ComputeAutomataReachability(info); err != nil {
		return nil, err
	}
	if info.MaxActiveDepth > evt1AutomataMaxPushDepth {
		return nil, evt1Diagnostic("CV4266", fmt.Sprintf("automata %s maximum active machine depth %d exceeds limit %d", decl.Name, info.MaxActiveDepth, evt1AutomataMaxPushDepth), decl.Span)
	}
	info.ContinuationCapacity = info.MaxActiveDepth - 1
	info.CompletionStepBound = info.MaxActiveDepth
	info.GraphIdentity = evt1AutomataGraphIdentity(info)
	return info, nil
}

func evt1ValidateAutomataReferences(info *evt1AutomataInfo) error {
	machineIndex := map[string]EVT1MachineDecl{}
	for _, machine := range info.Decl.Machines {
		machineIndex[machine.Name] = machine
	}
	for _, machine := range info.Decl.Machines {
		stateIndex := map[string]EVT1StateDecl{}
		for _, state := range machine.States {
			stateIndex[state.Name] = state
		}
		if machine.Name == info.RootMachine {
			for _, state := range machine.States {
				for _, completion := range state.Completion {
					if completion.Kind == "pop" {
						return evt1Diagnostic("CV4262", fmt.Sprintf("root machine %s cannot use pop in terminal state %s", machine.Name, state.Name), completion.Span)
					}
				}
			}
		}
		for _, state := range machine.States {
			for _, handler := range state.Handlers {
				switch handler.Kind {
				case EVT1TransitionGoto:
					if handler.TargetState.MachineName != "" {
						return evt1Diagnostic("CV4254", fmt.Sprintf("goto in machine %s must target a state in the same machine, not %s::%s", machine.Name, handler.TargetState.MachineName, handler.TargetState.StateName), handler.TargetState.Span)
					}
					if _, ok := stateIndex[handler.TargetState.StateName]; !ok {
						return evt1Diagnostic("CV4254", fmt.Sprintf("unknown goto target %s in machine %s", handler.TargetState.StateName, machine.Name), handler.TargetState.Span)
					}
				case EVT1TransitionPush:
					if _, ok := machineIndex[handler.PushMachine]; !ok {
						return evt1Diagnostic("CV4255", fmt.Sprintf("unknown pushed machine %s in automata %s", handler.PushMachine, info.Decl.Name), handler.Span)
					}
					if handler.Continuation.MachineName != "" {
						if handler.Continuation.MachineName == handler.PushMachine {
							return evt1Diagnostic("CV4256", fmt.Sprintf("push continuation after %s must name a caller state, not %s::%s", handler.PushMachine, handler.Continuation.MachineName, handler.Continuation.StateName), handler.Continuation.Span)
						}
						return evt1Diagnostic("CV4256", fmt.Sprintf("push continuation in machine %s must target a local caller state, not %s::%s", machine.Name, handler.Continuation.MachineName, handler.Continuation.StateName), handler.Continuation.Span)
					}
					if _, ok := stateIndex[handler.Continuation.StateName]; !ok {
						return evt1Diagnostic("CV4256", fmt.Sprintf("unknown push continuation state %s in machine %s", handler.Continuation.StateName, machine.Name), handler.Continuation.Span)
					}
				}
			}
		}
	}
	return nil
}

func evt1ValidateAutomataPushCycles(info *evt1AutomataInfo) error {
	graph := map[string][]evt1AutomataPushEdge{}
	for _, machine := range info.Decl.Machines {
		for _, state := range machine.States {
			for _, handler := range state.Handlers {
				if handler.Kind != EVT1TransitionPush {
					continue
				}
				graph[machine.Name] = append(graph[machine.Name], evt1AutomataPushEdge{
					FromMachine: machine.Name,
					ToMachine:   handler.PushMachine,
					Span:        handler.Span,
				})
			}
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var stack []string
	var dfs func(string) error
	dfs = func(name string) error {
		if visiting[name] {
			cycle := append(append([]string{}, stack...), name)
			return evt1Diagnostic("CV4257", "machine push cycles are not allowed in M0: "+strings.Join(cycle, " -> "), graph[stack[len(stack)-1]][0].Span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		stack = append(stack, name)
		for _, edge := range graph[name] {
			if visiting[edge.ToMachine] {
				cycle := append(append([]string{}, stack...), edge.ToMachine)
				return evt1Diagnostic("CV4257", "machine push cycles are not allowed in M0: "+strings.Join(cycle, " -> "), edge.Span)
			}
			if err := dfs(edge.ToMachine); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		visiting[name] = false
		return nil
	}
	for _, machine := range info.Decl.Machines {
		if err := dfs(machine.Name); err != nil {
			return err
		}
	}
	return nil
}

func evt1ComputeAutomataReachability(info *evt1AutomataInfo) error {
	work := 0
	machineIndex := map[string]EVT1MachineDecl{}
	initialState := map[string]string{}
	for _, machine := range info.Decl.Machines {
		machineIndex[machine.Name] = machine
		info.StateReachable[machine.Name] = map[string]bool{}
		for _, state := range machine.States {
			if state.Initial {
				initialState[machine.Name] = state.Name
				break
			}
		}
	}
	for _, machine := range info.Decl.Machines {
		queue := []string{initialState[machine.Name]}
		info.StateReachable[machine.Name][initialState[machine.Name]] = true
		for len(queue) > 0 {
			work++
			if work > evt1AutomataMaxValidationWork {
				return evt1Diagnostic("CV4265", fmt.Sprintf("automata %s reachability work exceeded limit %d", info.Decl.Name, evt1AutomataMaxValidationWork), info.Decl.Span)
			}
			current := queue[0]
			queue = queue[1:]
			state := evt1LookupState(machine, current)
			if state.Terminal {
				for _, completion := range state.Completion {
					if completion.Kind == "finish" {
						info.HasReachableFinish[machine.Name] = true
					}
					if completion.Kind == "pop" {
						info.HasReachablePop[machine.Name] = true
					}
				}
			}
			for _, handler := range state.Handlers {
				next := ""
				if handler.Kind == EVT1TransitionGoto {
					next = handler.TargetState.StateName
				} else {
					next = handler.Continuation.StateName
				}
				if !info.StateReachable[machine.Name][next] {
					info.StateReachable[machine.Name][next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	queue := []string{info.RootMachine}
	info.MachineReachable[info.RootMachine] = true
	for len(queue) > 0 {
		work++
		if work > evt1AutomataMaxValidationWork {
			return evt1Diagnostic("CV4265", fmt.Sprintf("automata %s machine reachability work exceeded limit %d", info.Decl.Name, evt1AutomataMaxValidationWork), info.Decl.Span)
		}
		current := queue[0]
		queue = queue[1:]
		machine := machineIndex[current]
		for _, state := range machine.States {
			if !info.StateReachable[current][state.Name] {
				continue
			}
			for _, handler := range state.Handlers {
				if handler.Kind != EVT1TransitionPush {
					continue
				}
				info.ReachablePushTargets[handler.PushMachine] = true
				if !info.MachineReachable[handler.PushMachine] {
					info.MachineReachable[handler.PushMachine] = true
					queue = append(queue, handler.PushMachine)
				}
			}
		}
	}
	for _, machine := range info.Decl.Machines {
		if !info.MachineReachable[machine.Name] {
			return evt1Diagnostic("CV4258", fmt.Sprintf("machine %s in automata %s is unreachable from root machine %s", machine.Name, info.Decl.Name, info.RootMachine), machine.Span)
		}
		for _, state := range machine.States {
			if !info.StateReachable[machine.Name][state.Name] {
				return evt1Diagnostic("CV4259", fmt.Sprintf("state %s in machine %s is unreachable from that machine's initial state", state.Name, machine.Name), state.Span)
			}
		}
	}
	for machineName := range info.ReachablePushTargets {
		if !info.HasReachablePop[machineName] {
			return evt1Diagnostic("CV4260", fmt.Sprintf("pushed machine %s requires a reachable terminal pop completion", machineName), machineIndex[machineName].Span)
		}
	}
	if !info.HasReachableFinish[info.RootMachine] {
		return evt1Diagnostic("CV4261", fmt.Sprintf("root machine %s requires a reachable terminal finish completion", info.RootMachine), machineIndex[info.RootMachine].Span)
	}
	depth, err := evt1LongestPushDepth(info, machineIndex, map[string]int{})
	if err != nil {
		return err
	}
	info.MaxActiveDepth = depth
	return nil
}

func evt1LongestPushDepth(info *evt1AutomataInfo, machineIndex map[string]EVT1MachineDecl, memo map[string]int) (int, error) {
	var visit func(string) (int, error)
	visit = func(name string) (int, error) {
		if depth, ok := memo[name]; ok {
			return depth, nil
		}
		best := 1
		machine := machineIndex[name]
		for _, state := range machine.States {
			if !info.StateReachable[name][state.Name] {
				continue
			}
			for _, handler := range state.Handlers {
				if handler.Kind != EVT1TransitionPush || !info.MachineReachable[handler.PushMachine] {
					continue
				}
				child, err := visit(handler.PushMachine)
				if err != nil {
					return 0, err
				}
				if 1+child > best {
					best = 1 + child
				}
			}
		}
		memo[name] = best
		return best, nil
	}
	return visit(info.RootMachine)
}

func evt1LookupState(machine EVT1MachineDecl, name string) EVT1StateDecl {
	for _, state := range machine.States {
		if state.Name == name {
			return state
		}
	}
	return EVT1StateDecl{}
}

func evt1AutomataGraphIdentity(info *evt1AutomataInfo) string {
	var b strings.Builder
	b.WriteString(info.Decl.Name)
	b.WriteString("|")
	b.WriteString(info.SignalEnum.Name)
	b.WriteString("|")
	b.WriteString(info.RootMachine)
	b.WriteString("|")
	b.WriteString(fmt.Sprintf("depth=%d", info.MaxActiveDepth))
	for _, machine := range info.Decl.Machines {
		b.WriteString("|machine:")
		if machine.Initial {
			b.WriteString("initial:")
		}
		b.WriteString(machine.Name)
		for _, state := range machine.States {
			b.WriteString("|state:")
			if state.Initial {
				b.WriteString("initial:")
			}
			if state.Terminal {
				b.WriteString("terminal:")
			}
			b.WriteString(state.Name)
			for _, handler := range state.Handlers {
				b.WriteString("|on:")
				b.WriteString(handler.Signal.EnumName)
				b.WriteString("::")
				b.WriteString(handler.Signal.MemberName)
				b.WriteString(":")
				b.WriteString(string(handler.Kind))
				switch handler.Kind {
				case EVT1TransitionGoto:
					b.WriteString(":")
					if handler.TargetState.MachineName != "" {
						b.WriteString(handler.TargetState.MachineName)
						b.WriteString("::")
					}
					b.WriteString(handler.TargetState.StateName)
				case EVT1TransitionPush:
					b.WriteString(":")
					b.WriteString(handler.PushMachine)
					b.WriteString(":")
					if handler.Continuation.MachineName != "" {
						b.WriteString(handler.Continuation.MachineName)
						b.WriteString("::")
					}
					b.WriteString(handler.Continuation.StateName)
				}
			}
			for _, completion := range state.Completion {
				b.WriteString("|complete:")
				b.WriteString(completion.Kind)
			}
		}
	}
	return digest([]byte(b.String()))
}
