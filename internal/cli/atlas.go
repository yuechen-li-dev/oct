package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/atlas"
)

type atlasArgs struct {
	project string
	out     string
	depth   int
}

func executeAtlas(args []string, stdout, stderr io.Writer, workingDir string) error {
	if len(args) == 0 || isHelpArg(args) {
		return writeAtlasHelp(stdout)
	}
	command := args[0]
	if command != "build" && command != "verify" && command != "show" && command != "explain" && command != "affected-by" && command != "coverage" {
		return reportCommandError(stderr, "atlas", fmt.Errorf("unknown atlas command %q", command))
	}
	rest := args[1:]
	id := ""
	if command == "show" || command == "explain" || command == "affected-by" {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
			return reportCommandError(stderr, "atlas "+command, fmt.Errorf("missing Atlas ID"))
		}
		id, rest = rest[0], rest[1:]
	}
	options, err := parseAtlasArgs(rest, workingDir)
	if err != nil {
		return reportCommandError(stderr, "atlas "+command, err)
	}
	graph, err := atlas.Compile(options.project)
	if err != nil {
		return reportCommandError(stderr, "atlas "+command, err)
	}
	switch command {
	case "build":
		out := options.out
		if out == "" {
			out = filepath.Join(options.project, "atlas.octagon")
		}
		if !filepath.IsAbs(out) {
			out = filepath.Join(workingDir, out)
		}
		if err := atlas.WriteOctagon(out, graph); err != nil {
			return reportCommandError(stderr, "atlas build", err)
		}
		_, err = fmt.Fprintf(stdout, "Atlas built: %s\nnodes: %d\nedges: %d\nwarnings: %d\n", out, len(graph.Nodes), len(graph.Links), len(graph.Warnings))
		return err
	case "verify":
		if policyErrors := graph.PolicyErrors(); len(policyErrors) > 0 {
			return reportCommandError(stderr, "atlas verify", fmt.Errorf("Atlas project policy failed: %s", strings.Join(policyErrors, "; ")))
		}
		_, err = fmt.Fprintf(stdout, "Atlas valid: %s\nnodes: %d\nedges: %d\nwarnings: %d\n", graph.Package, len(graph.Nodes), len(graph.Links), len(graph.Warnings))
		for _, warning := range graph.Warnings {
			_, _ = fmt.Fprintln(stdout, "warning: "+warning)
		}
		return err
	case "show":
		node, ok := graph.Node(id)
		if !ok {
			return reportCommandError(stderr, "atlas show", fmt.Errorf("unknown Atlas ID %q", id))
		}
		return writeAtlasNode(stdout, node)
	case "explain":
		body, err := renderAtlasExplanation(graph, id, options.depth)
		if err != nil {
			return reportCommandError(stderr, "atlas explain", err)
		}
		if options.out != "" {
			out := options.out
			if !filepath.IsAbs(out) {
				out = filepath.Join(workingDir, out)
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return reportCommandError(stderr, "atlas explain", err)
			}
			if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
				return reportCommandError(stderr, "atlas explain", err)
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		}
		_, err = fmt.Fprint(stdout, body)
		return err
	case "affected-by":
		nodes, err := graph.AffectedBy(id, options.depth)
		if err != nil {
			return reportCommandError(stderr, "atlas affected-by", err)
		}
		_, _ = fmt.Fprintln(stdout, id)
		for _, node := range nodes {
			_, _ = fmt.Fprintf(stdout, "  -> %s (%s)\n", node.ID, node.Kind)
		}
		return nil
	case "coverage":
		return writeAtlasCoverage(stdout, graph)
	}
	return nil
}

func parseAtlasArgs(args []string, workingDir string) (atlasArgs, error) {
	result := atlasArgs{project: workingDir, depth: 4}
	pathSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return result, fmt.Errorf("--project requires a path")
			}
			i++
			result.project = resolveWorkingPath(workingDir, args[i])
			pathSet = true
		case "--out":
			if i+1 >= len(args) {
				return result, fmt.Errorf("--out requires a path")
			}
			i++
			result.out = args[i]
		case "--depth":
			if i+1 >= len(args) {
				return result, fmt.Errorf("--depth requires an integer")
			}
			i++
			depth, err := strconv.Atoi(args[i])
			if err != nil || depth < 1 || depth > 32 {
				return result, fmt.Errorf("--depth must be between 1 and 32")
			}
			result.depth = depth
		default:
			if strings.HasPrefix(args[i], "-") {
				return result, fmt.Errorf("unknown option %q", args[i])
			}
			if pathSet {
				return result, fmt.Errorf("only one project path may be provided")
			}
			result.project = resolveWorkingPath(workingDir, args[i])
			pathSet = true
		}
	}
	return result, nil
}

func writeAtlasNode(out io.Writer, n atlas.Node) error {
	_, err := fmt.Fprintf(out, "%s\nKind: %s\n", n.ID, n.Kind)
	if n.Title != "" {
		_, _ = fmt.Fprintln(out, "Title: "+n.Title)
	}
	if n.Text != "" {
		_, _ = fmt.Fprintln(out, "Text: "+n.Text)
	}
	if n.URI != "" {
		_, _ = fmt.Fprintln(out, "URI: "+n.URI)
	}
	if n.Version != "" {
		_, _ = fmt.Fprintln(out, "Version: "+n.Version)
	}
	if n.Digest != "" {
		_, _ = fmt.Fprintln(out, "Digest: "+n.Digest)
	}
	if n.Authority != "" {
		_, _ = fmt.Fprintln(out, "Authority: "+n.Authority)
	}
	if n.Locator != "" {
		_, _ = fmt.Fprintln(out, "Locator: "+n.Locator)
	}
	if n.Reference != "" {
		_, _ = fmt.Fprintln(out, "Reference: "+n.Reference)
	}
	if n.Output != "" {
		_, _ = fmt.Fprintln(out, "Output: "+n.Output)
	}
	if n.Source.Path != "" {
		_, _ = fmt.Fprintf(out, "Source: %s:%d:%d\n", n.Source.Path, n.Source.Line, n.Source.Column)
	}
	return err
}

func renderAtlasExplanation(g atlas.Graph, id string, depth int) (string, error) {
	node, ok := g.Node(id)
	if !ok {
		return "", fmt.Errorf("unknown Atlas ID %q", id)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", node.ID)
	fmt.Fprintf(&b, "**Kind:** %s\n\n", node.Kind)
	if node.Title != "" {
		b.WriteString(node.Title + "\n\n")
	}
	if node.Text != "" {
		b.WriteString(node.Text + "\n\n")
	}
	if node.Locator != "" {
		b.WriteString("**Locator:** " + node.Locator + "\n\n")
	}
	if node.Reference != "" {
		b.WriteString("**Reference:** `" + node.Reference + "`\n\n")
	}
	if node.Output != "" {
		b.WriteString("**Output:** `" + node.Output + "`\n\n")
	}
	type related struct{ relation, direction, id string }
	var relations []related
	for _, e := range g.Links {
		if e.From == id {
			relations = append(relations, related{e.Relation, "out", e.To})
		}
		if e.To == id {
			relations = append(relations, related{e.Relation, "in", e.From})
		}
	}
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].relation != relations[j].relation {
			return relations[i].relation < relations[j].relation
		}
		if relations[i].direction != relations[j].direction {
			return relations[i].direction < relations[j].direction
		}
		return relations[i].id < relations[j].id
	})
	if len(relations) > 0 {
		b.WriteString("## Relationships\n\n")
		for _, r := range relations {
			arrow := "--" + r.relation + "-->"
			if r.direction == "in" {
				arrow = "<--" + r.relation + "--"
			}
			target, _ := g.Node(r.id)
			status := ""
			if target.Kind == atlas.EvidenceKind {
				status = " — " + target.EvidenceState
			}
			fmt.Fprintf(&b, "- `%s` %s `%s` (%s)%s\n", id, arrow, r.id, target.Kind, status)
		}
		b.WriteString("\n")
	}
	if node.Kind == atlas.RequirementKind || node.Kind == atlas.ClaimKind {
		var sourceLines []string
		for _, edge := range g.Links {
			if edge.Relation != "Supports" || edge.To != id {
				continue
			}
			support, _ := g.Node(edge.From)
			if support.Kind != atlas.CitationKind {
				continue
			}
			sourceLines = append(sourceLines, "- `"+support.ID+"`: "+support.Locator)
			for _, interpretationEdge := range g.Links {
				if interpretationEdge.Relation == "Interprets" && interpretationEdge.To == support.ID {
					interpretation, _ := g.Node(interpretationEdge.From)
					sourceLines = append(sourceLines, "  - interpreted by `"+interpretation.ID+"`: "+interpretation.Text)
				}
			}
		}
		if len(sourceLines) > 0 {
			sort.Strings(sourceLines)
			b.WriteString("## Source chain\n\n" + strings.Join(sourceLines, "\n") + "\n\n")
		}
	}
	// The bounded dependency expansion is deterministic and intentionally uses
	// only authored dependency semantics; there is no heuristic path ranking.
	type item struct {
		id    string
		level int
	}
	queue := []item{{id, 0}}
	seen := map[string]bool{id: true}
	var paths []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.level >= depth {
			continue
		}
		var next []atlas.Link
		for _, e := range g.Links {
			if e.From == cur.id && (e.Relation == "DerivedFrom" || e.Relation == "DependsOn") {
				next = append(next, e)
			}
		}
		sort.Slice(next, func(i, j int) bool {
			if next[i].Relation != next[j].Relation {
				return next[i].Relation < next[j].Relation
			}
			return next[i].To < next[j].To
		})
		for _, e := range next {
			paths = append(paths, strings.Repeat("  ", cur.level)+"- `"+e.From+"` --"+e.Relation+"--> `"+e.To+"`")
			if !seen[e.To] {
				seen[e.To] = true
				queue = append(queue, item{e.To, cur.level + 1})
			}
		}
	}
	if len(paths) > 0 {
		b.WriteString("## Dependency path\n\n")
		b.WriteString(strings.Join(paths, "\n") + "\n\n")
	}
	b.WriteString("_Atlas records project claims; graph consistency and test status do not establish external legal or scientific truth._\n")
	return b.String(), nil
}

func writeAtlasCoverage(out io.Writer, g atlas.Graph) error {
	c := g.Coverage
	_, err := fmt.Fprintf(out, "Atlas coverage for %s\nRequirements: %d total, %d implemented, %d verified\nClaims: %d total, %d supported\nInterpretations: %d total, %d cited\nFacts: %d total, %d linked\nTheories: %d total, %d linked\nArtifacts: %d total, %d linked\nOrphans: %d\nContradictions: %d\nSuperseded: %d\n", g.Package, c.RequirementsTotal, c.RequirementsImplemented, c.RequirementsVerified, c.ClaimsTotal, c.ClaimsSupported, c.InterpretationsTotal, c.InterpretationsCited, c.FactsTotal, c.FactsLinked, c.TheoriesTotal, c.TheoriesLinked, c.ArtifactsTotal, c.ArtifactsLinked, c.Orphans, c.Contradictions, c.Superseded)
	for _, warning := range g.Warnings {
		_, _ = fmt.Fprintln(out, "warning: "+warning)
	}
	return err
}

func writeAtlasHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct atlas <build|verify|show|explain|affected-by|coverage> ...\n\ncommands:\n  build [project] [--out atlas.octagon]            Compile the canonical deterministic graph\n  verify [project]                                 Validate graph, references, cycles, and project policies\n  show <ID> [project]                              Show one Atlas node\n  explain <ID> [project] [--depth N] [--out file] Render deterministic Markdown relationships\n  affected-by <ID> [project] [--depth N]           Traverse reverse semantic dependencies\n  coverage [project]                               Report knowledge coverage and orphans\n\nA project opts in with fn AtlasDocument() -> Atlas.Document. Atlas is build-time metadata and does not alter ordinary runtime codegen.")
	return err
}
