package build

import (
	"fmt"
	"strings"
)

// CFG is the backend-independent control-flow graph of one MIR function.
// BlockOrder, successor lists, predecessor lists, and Reachable all follow the
// function's MIR block order so diagnostics and later analyses are stable.
type CFG struct {
	Entry        string
	BlockOrder   []string
	Successors   map[string][]string
	Predecessors map[string][]string
	Reachable    []string
}

// BuildCFG derives control-flow edges from MIR terminators without changing
// the function. Every block must have a unique label and an explicit
// terminator, and every jump or branch target must name a block in the same
// function.
func BuildCFG(fn MIRFunction) (CFG, error) {
	if len(fn.Blocks) == 0 {
		return CFG{}, fmt.Errorf("function %s.%s has no MIR blocks", fn.Package, fn.Name)
	}

	cfg := CFG{
		Entry:        fn.Blocks[0].Label,
		BlockOrder:   make([]string, 0, len(fn.Blocks)),
		Successors:   make(map[string][]string, len(fn.Blocks)),
		Predecessors: make(map[string][]string, len(fn.Blocks)),
	}
	labels := make(map[string]struct{}, len(fn.Blocks))
	for _, block := range fn.Blocks {
		if block.Label == "" {
			return CFG{}, fmt.Errorf("function %s.%s has an MIR block with an empty label", fn.Package, fn.Name)
		}
		if _, exists := labels[block.Label]; exists {
			return CFG{}, fmt.Errorf("function %s.%s has duplicate MIR block label %s", fn.Package, fn.Name, block.Label)
		}
		labels[block.Label] = struct{}{}
		cfg.BlockOrder = append(cfg.BlockOrder, block.Label)
		cfg.Successors[block.Label] = []string{}
		cfg.Predecessors[block.Label] = []string{}
	}

	for _, block := range fn.Blocks {
		var successors []string
		switch term := block.Terminator.(type) {
		case MIRReturn, MIRFail:
			successors = []string{}
		case MIRJump:
			successors = []string{term.Target}
		case MIRBranch:
			successors = []string{term.TrueTarget}
			if term.FalseTarget != term.TrueTarget {
				successors = append(successors, term.FalseTarget)
			}
		case nil:
			return CFG{}, fmt.Errorf("MIR block %s.%s:%s has no terminator", fn.Package, fn.Name, block.Label)
		default:
			return CFG{}, fmt.Errorf("MIR block %s.%s:%s has unsupported terminator %T", fn.Package, fn.Name, block.Label, block.Terminator)
		}
		for _, successor := range successors {
			if _, exists := labels[successor]; !exists {
				return CFG{}, fmt.Errorf("MIR block %s.%s:%s references missing target %s", fn.Package, fn.Name, block.Label, successor)
			}
		}
		cfg.Successors[block.Label] = successors
	}

	for _, label := range cfg.BlockOrder {
		for _, successor := range cfg.Successors[label] {
			cfg.Predecessors[successor] = append(cfg.Predecessors[successor], label)
		}
	}
	cfg.Reachable = reachableBlocks(cfg.Entry, cfg.Successors)
	return cfg, nil
}

func reachableBlocks(entry string, successors map[string][]string) []string {
	seen := make(map[string]bool, len(successors))
	worklist := []string{entry}
	reachable := make([]string, 0, len(successors))
	for len(worklist) > 0 {
		label := worklist[0]
		worklist = worklist[1:]
		if seen[label] {
			continue
		}
		seen[label] = true
		reachable = append(reachable, label)
		worklist = append(worklist, successors[label]...)
	}
	return reachable
}

// DumpCFG returns a deterministic diagnostic view of a function's CFG.
func DumpCFG(fn MIRFunction) (string, error) {
	cfg, err := BuildCFG(fn)
	if err != nil {
		return "", err
	}
	reachable := make(map[string]bool, len(cfg.Reachable))
	for _, label := range cfg.Reachable {
		reachable[label] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "function %s.%s entry=%s\n", fn.Package, fn.Name, cfg.Entry)
	for _, label := range cfg.BlockOrder {
		fmt.Fprintf(&b, "\nblock %s\n", label)
		fmt.Fprintf(&b, "  successors: %s\n", labelsOrDash(cfg.Successors[label]))
		fmt.Fprintf(&b, "  predecessors: %s\n", labelsOrDash(cfg.Predecessors[label]))
		fmt.Fprintf(&b, "  reachable: %t\n", reachable[label])
	}
	return b.String(), nil
}

func labelsOrDash(labels []string) string {
	if len(labels) == 0 {
		return "-"
	}
	return strings.Join(labels, ", ")
}
