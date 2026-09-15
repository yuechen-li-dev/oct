package build

import (
	"fmt"
	"strings"
)

// DefinitionSet is ordered by the function's deterministic definition order.
// Callers receive slices rather than the solver's mutable bit representation.
type DefinitionSet []DefinitionID

// ReachingDefinitions contains block facts plus statement-precise use-def and
// def-use views. Unreachable blocks are listed but have no computed facts.
type ReachingDefinitions struct {
	CFG             CFG
	UseDefs         UseDefInfo
	Gen             map[string]DefinitionSet
	Kill            map[string]DefinitionSet
	In              map[string]DefinitionSet
	Out             map[string]DefinitionSet
	AtUse           map[UseSite]DefinitionSet
	DefUses         map[DefinitionID][]UseSite
	Unreachable     []string
	BlocksProcessed int
}

// AnalyzeReachingDefinitions computes forward may-reaching definitions for
// whole scalar MIR locals. It observes MIR and never mutates it.
func AnalyzeReachingDefinitions(fn MIRFunction) (ReachingDefinitions, error) {
	cfg, err := BuildCFG(fn)
	if err != nil {
		return ReachingDefinitions{}, err
	}
	info, err := ExtractUseDefInfo(fn)
	if err != nil {
		return ReachingDefinitions{}, err
	}

	result := ReachingDefinitions{
		CFG:     cfg,
		UseDefs: info,
		Gen:     make(map[string]DefinitionSet, len(fn.Blocks)),
		Kill:    make(map[string]DefinitionSet, len(fn.Blocks)),
		In:      make(map[string]DefinitionSet, len(fn.Blocks)),
		Out:     make(map[string]DefinitionSet, len(fn.Blocks)),
		AtUse:   make(map[UseSite]DefinitionSet, len(info.Uses)),
		DefUses: make(map[DefinitionID][]UseSite, len(info.Definitions)),
	}
	definitionIndex := make(map[DefinitionID]int, len(info.Definitions))
	definitionsByLocal := make(map[string][]int)
	for i, def := range info.Definitions {
		definitionIndex[def] = i
		definitionsByLocal[def.Local] = append(definitionsByLocal[def.Local], i)
		result.DefUses[def] = []UseSite{}
	}

	blockByLabel := make(map[string]MIRBlock, len(fn.Blocks))
	reachable := make(map[string]bool, len(cfg.Reachable))
	for _, label := range cfg.Reachable {
		reachable[label] = true
	}
	for _, block := range fn.Blocks {
		blockByLabel[block.Label] = block
		if !reachable[block.Label] {
			result.Unreachable = append(result.Unreachable, block.Label)
			continue
		}
		genBits := make([]bool, len(info.Definitions))
		for _, def := range info.DefinitionsByBlock[block.Label] {
			killLocal(genBits, definitionsByLocal[def.Local])
			genBits[definitionIndex[def]] = true
		}
		killBits := make([]bool, len(info.Definitions))
		for i, generated := range genBits {
			if !generated {
				for _, candidate := range info.DefinitionsByBlock[block.Label] {
					if info.Definitions[i].Local == candidate.Local {
						killBits[i] = true
						break
					}
				}
			}
		}
		result.Gen[block.Label] = bitsToDefinitionSet(genBits, info.Definitions)
		result.Kill[block.Label] = bitsToDefinitionSet(killBits, info.Definitions)
		result.In[block.Label] = DefinitionSet{}
		result.Out[block.Label] = DefinitionSet{}
	}

	inBits := make(map[string][]bool, len(cfg.Reachable))
	outBits := make(map[string][]bool, len(cfg.Reachable))
	genBits := make(map[string][]bool, len(cfg.Reachable))
	killBits := make(map[string][]bool, len(cfg.Reachable))
	for _, label := range cfg.Reachable {
		inBits[label] = make([]bool, len(info.Definitions))
		outBits[label] = make([]bool, len(info.Definitions))
		genBits[label] = definitionSetToBits(result.Gen[label], definitionIndex, len(info.Definitions))
		killBits[label] = definitionSetToBits(result.Kill[label], definitionIndex, len(info.Definitions))
	}
	initialBits := definitionSetToBits(info.InitialDefinitions, definitionIndex, len(info.Definitions))

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

		newIn := make([]bool, len(info.Definitions))
		if label == cfg.Entry {
			unionBits(newIn, initialBits)
		}
		for _, predecessor := range cfg.Predecessors[label] {
			if reachable[predecessor] {
				unionBits(newIn, outBits[predecessor])
			}
		}
		newOut := transferDefinitionBits(newIn, genBits[label], killBits[label])
		inBits[label] = newIn
		if equalBits(newOut, outBits[label]) {
			continue
		}
		outBits[label] = newOut
		for _, successor := range cfg.Successors[label] {
			if reachable[successor] && !queued[successor] {
				worklist = append(worklist, successor)
				queued[successor] = true
			}
		}
	}

	for _, label := range cfg.Reachable {
		result.In[label] = bitsToDefinitionSet(inBits[label], info.Definitions)
		result.Out[label] = bitsToDefinitionSet(outBits[label], info.Definitions)
		current := append([]bool(nil), inBits[label]...)
		block := blockByLabel[label]
		uses := info.UsesByBlock[label]
		for statement := 0; statement <= len(block.Statements); statement++ {
			for _, use := range uses {
				if use.Site.Statement != statement {
					continue
				}
				reaching := make([]bool, len(current))
				for _, index := range definitionsByLocal[use.Site.Local] {
					reaching[index] = current[index]
				}
				set := bitsToDefinitionSet(reaching, info.Definitions)
				result.AtUse[use.Site] = set
				for _, def := range set {
					result.DefUses[def] = append(result.DefUses[def], use.Site)
				}
			}
			if statement == len(block.Statements) {
				break
			}
			for _, def := range info.DefinitionsByBlock[label] {
				if def.Statement == statement {
					killLocal(current, definitionsByLocal[def.Local])
					current[definitionIndex[def]] = true
				}
			}
		}
		if !equalBits(current, outBits[label]) {
			return ReachingDefinitions{}, fmt.Errorf("internal reaching-definitions transfer mismatch at block %s", label)
		}
	}
	return result, nil
}

func transferDefinitionBits(in, gen, kill []bool) []bool {
	out := make([]bool, len(in))
	for i := range in {
		out[i] = gen[i] || (in[i] && !kill[i])
	}
	return out
}

func killLocal(bits []bool, indices []int) {
	for _, index := range indices {
		bits[index] = false
	}
}

func unionBits(dst, src []bool) {
	for i := range dst {
		dst[i] = dst[i] || src[i]
	}
}

func equalBits(left, right []bool) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func bitsToDefinitionSet(bits []bool, definitions []DefinitionID) DefinitionSet {
	set := make(DefinitionSet, 0)
	for i, present := range bits {
		if present {
			set = append(set, definitions[i])
		}
	}
	return set
}

func definitionSetToBits(set DefinitionSet, index map[DefinitionID]int, size int) []bool {
	bits := make([]bool, size)
	for _, def := range set {
		bits[index[def]] = true
	}
	return bits
}

// DumpReachingDefinitions returns a stable, book-friendly diagnostic view.
func DumpReachingDefinitions(fn MIRFunction) (string, error) {
	analysis, err := AnalyzeReachingDefinitions(fn)
	if err != nil {
		return "", err
	}
	numbers := make(map[DefinitionID]int, len(analysis.UseDefs.Definitions))
	var b strings.Builder
	fmt.Fprintf(&b, "function %s.%s\n", fn.Package, fn.Name)
	b.WriteString("\ndefinitions\n")
	for i, def := range analysis.UseDefs.Definitions {
		numbers[def] = i
		switch def.Kind {
		case DefinitionParameter:
			fmt.Fprintf(&b, "  D%d: parameter[%d] defines %s\n", i, def.Result, def.Local)
		case DefinitionCapture:
			fmt.Fprintf(&b, "  D%d: capture[%d] defines %s\n", i, def.Result, def.Local)
		default:
			fmt.Fprintf(&b, "  D%d: %s:stmt%d defines %s\n", i, def.Block, def.Statement, def.Local)
		}
	}
	reachable := make(map[string]bool, len(analysis.CFG.Reachable))
	for _, label := range analysis.CFG.Reachable {
		reachable[label] = true
	}
	blockLengths := make(map[string]int, len(fn.Blocks))
	for _, block := range fn.Blocks {
		blockLengths[block.Label] = len(block.Statements)
	}
	for _, label := range analysis.CFG.BlockOrder {
		fmt.Fprintf(&b, "\nblock %s\n", label)
		if !reachable[label] {
			b.WriteString("  unreachable\n")
			continue
		}
		fmt.Fprintf(&b, "  GEN:  %s\n", formatDefinitionSet(analysis.Gen[label], numbers))
		fmt.Fprintf(&b, "  KILL: %s\n", formatDefinitionSet(analysis.Kill[label], numbers))
		fmt.Fprintf(&b, "  IN:   %s\n", formatDefinitionSet(analysis.In[label], numbers))
		fmt.Fprintf(&b, "  OUT:  %s\n", formatDefinitionSet(analysis.Out[label], numbers))
		uses := analysis.UseDefs.UsesByBlock[label]
		if len(uses) > 0 {
			b.WriteString("  uses:\n")
		}
		for _, use := range uses {
			where := fmt.Sprintf("stmt%d", use.Site.Statement)
			if use.Site.Statement == blockLengths[label] {
				where = "term"
			}
			fmt.Fprintf(&b, "    %s %s %s <- %s\n", where, use.Site.Operand, use.Site.Local, formatDefinitionSet(analysis.AtUse[use.Site], numbers))
		}
	}
	return b.String(), nil
}

func formatDefinitionSet(set DefinitionSet, numbers map[DefinitionID]int) string {
	if len(set) == 0 {
		return "{}"
	}
	parts := make([]string, len(set))
	for i, def := range set {
		parts[i] = fmt.Sprintf("D%d:%s", numbers[def], def.Local)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
