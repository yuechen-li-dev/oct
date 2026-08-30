// Package atlas compiles ordinary Oct Atlas.Document values into a validated,
// deterministic, build-time-only semantic graph.
package atlas

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

type Kind string

const (
	AuthorityKind      Kind = "Authority"
	CitationKind       Kind = "Citation"
	RequirementKind    Kind = "Requirement"
	InterpretationKind Kind = "Interpretation"
	ClaimKind          Kind = "Claim"
	SymbolKind         Kind = "SymbolRef"
	EvidenceKind       Kind = "EvidenceRef"
	ArtifactKind       Kind = "ArtifactRef"
)

type Source struct {
	Package string
	Path    string
	Line    int
	Column  int
}

type Node struct {
	ID            string
	Kind          Kind
	Title         string
	Text          string
	URI           string
	Version       string
	Digest        string
	Authority     string
	Locator       string
	Reference     string
	ReferenceKind string
	Output        string
	EvidenceState string
	DeclaredAt    Source
	Source        Source
}

type Link struct {
	From     string
	Relation string
	To       string
}

type Policy struct {
	RequireRequirementImplementation bool
	RequireRequirementVerifier       bool
	RequireInterpretationCitation    bool
	RequireClaimSupport              bool
}

type Coverage struct {
	RequirementsTotal       int
	RequirementsImplemented int
	RequirementsVerified    int
	ClaimsTotal             int
	ClaimsSupported         int
	InterpretationsTotal    int
	InterpretationsCited    int
	FactsTotal              int
	FactsLinked             int
	TheoriesTotal           int
	TheoriesLinked          int
	ArtifactsTotal          int
	ArtifactsLinked         int
	Orphans                 int
	Contradictions          int
	Superseded              int
}

type Graph struct {
	Root       string
	Package    string
	Nodes      []Node
	Links      []Link
	Warnings   []string
	Coverage   Coverage
	BuildTime  time.Duration
	SourceSize int64
	Policy     Policy
	program    project.Program
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*$`)

func Compile(path string) (Graph, error) {
	started := time.Now()
	program, err := project.LoadForTest(path)
	if err != nil {
		return Graph{}, err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return Graph{}, err
	}
	pkg, ok := program.Packages[program.Entry]
	if !ok {
		return Graph{}, fmt.Errorf("Atlas entry package %q is not loaded", program.Entry)
	}
	found := false
	for _, fn := range pkg.Functions {
		if fn.Name == "AtlasDocument" {
			found = true
			if len(fn.Parameters) != 0 || fn.ReturnType.Package != "Atlas" || fn.ReturnType.Name != "Document" && fn.ReturnType.Name != "TableDocument" {
				return Graph{}, fmt.Errorf("%s.AtlasDocument must return Atlas.Document or Atlas.TableDocument", program.Entry)
			}
			break
		}
	}
	if !found {
		return Graph{}, fmt.Errorf("no Atlas document found: package %s must define AtlasDocument returning Atlas.Document or Atlas.TableDocument", program.Entry)
	}
	value, err := interpret.CallFunctionWithArgsAndOptions(program, program.Entry, "AtlasDocument", nil, io.Discard, interpret.ExecuteOptions{})
	if err != nil {
		return Graph{}, fmt.Errorf("evaluate %s.AtlasDocument: %w", program.Entry, err)
	}
	graph, err := graphFromValue(value)
	if err != nil {
		return Graph{}, err
	}
	graph.Root, graph.Package, graph.program = program.Root, program.Entry, program
	graph.attachDeclarationSources()
	if err := graph.resolveReferences(); err != nil {
		return Graph{}, err
	}
	if err := graph.Validate(); err != nil {
		return Graph{}, err
	}
	graph.Coverage, graph.Warnings = graph.measure()
	graph.SourceSize = sourceBytes(program)
	graph.BuildTime = time.Since(started)
	return graph, nil
}

func (g *Graph) attachDeclarationSources() {
	pkg := g.program.Packages[g.Package]
	path := ""
	for _, fn := range pkg.Functions {
		if fn.Name == "AtlasDocument" {
			path = fn.SourcePath
			break
		}
	}
	if path == "" {
		return
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(body), "\n")
	used := make(map[string]int)
	for i := range g.Nodes {
		needle := strconv.Quote(g.Nodes[i].ID)
		skip := used[g.Nodes[i].ID]
		for lineNumber, line := range lines {
			offset := 0
			for {
				column := strings.Index(line[offset:], needle)
				if column < 0 {
					break
				}
				column += offset
				if skip > 0 {
					skip--
					offset = column + len(needle)
					continue
				}
				g.Nodes[i].DeclaredAt = Source{Package: g.Package, Path: slashRelative(g.Root, path), Line: lineNumber + 1, Column: column + 1}
				used[g.Nodes[i].ID]++
				break
			}
			if g.Nodes[i].DeclaredAt.Path != "" {
				break
			}
		}
	}
}

func graphFromValue(value interpret.Value) (Graph, error) {
	if value.Kind != interpret.ValueRecord || shortType(value.Record.TypeName) != "Document" && shortType(value.Record.TypeName) != "TableDocument" {
		return Graph{}, fmt.Errorf("AtlasDocument returned %s, expected Atlas.Document or Atlas.TableDocument", value.Kind)
	}
	var graph Graph
	var err error
	add := func(field string, kind Kind, decode func(interpret.RecordValue) (Node, error)) error {
		values, e := recordCollection(value.Record, field)
		if e != nil {
			return e
		}
		for _, item := range values {
			if item.Kind != interpret.ValueRecord {
				return fmt.Errorf("Atlas.Document.%s must contain records", field)
			}
			node, e := decode(item.Record)
			if e != nil {
				return fmt.Errorf("Atlas.Document.%s: %w", field, e)
			}
			node.Kind = kind
			graph.Nodes = append(graph.Nodes, node)
		}
		return nil
	}
	if err = add("Authorities", AuthorityKind, decodeAuthority); err != nil {
		return Graph{}, err
	}
	if err = add("Citations", CitationKind, decodeCitation); err != nil {
		return Graph{}, err
	}
	if err = add("Requirements", RequirementKind, decodeText); err != nil {
		return Graph{}, err
	}
	if err = add("Interpretations", InterpretationKind, decodeText); err != nil {
		return Graph{}, err
	}
	if err = add("Claims", ClaimKind, decodeText); err != nil {
		return Graph{}, err
	}
	if err = add("Symbols", SymbolKind, decodeSymbol); err != nil {
		return Graph{}, err
	}
	if err = add("Evidence", EvidenceKind, decodeEvidence); err != nil {
		return Graph{}, err
	}
	if err = add("Artifacts", ArtifactKind, decodeArtifact); err != nil {
		return Graph{}, err
	}
	links, err := recordCollection(value.Record, "Links")
	if err != nil {
		return Graph{}, err
	}
	for _, item := range links {
		if item.Kind != interpret.ValueRecord {
			return Graph{}, fmt.Errorf("Atlas.Document.Links must contain Atlas.Link records")
		}
		from, e := stringField(item.Record, "From")
		if e != nil {
			return Graph{}, e
		}
		relation, e := enumField(item.Record, "Relation")
		if e != nil {
			return Graph{}, e
		}
		to, e := stringField(item.Record, "To")
		if e != nil {
			return Graph{}, e
		}
		graph.Links = append(graph.Links, Link{From: from, Relation: relation, To: to})
	}
	policy, ok := value.Record.Fields["Policy"]
	if !ok || policy.Kind != interpret.ValueRecord {
		return Graph{}, fmt.Errorf("Atlas.Document.Policy must be Atlas.Policy")
	}
	graph.Policy, err = decodePolicy(policy.Record)
	return graph, err
}

func decodeAuthority(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	title, err := stringField(r, "Title")
	if err != nil {
		return Node{}, err
	}
	uri, err := stringField(r, "URI")
	if err != nil {
		return Node{}, err
	}
	version, err := stringField(r, "Version")
	if err != nil {
		return Node{}, err
	}
	digest, err := stringField(r, "Digest")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Title: title, URI: uri, Version: version, Digest: digest}, nil
}
func decodeCitation(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	authority, err := stringField(r, "Authority")
	if err != nil {
		return Node{}, err
	}
	locator, err := stringField(r, "Locator")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Authority: authority, Locator: locator}, nil
}
func decodeText(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	text, err := stringField(r, "Text")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Text: text}, nil
}
func decodeSymbol(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	ref, err := stringField(r, "Symbol")
	if err != nil {
		return Node{}, err
	}
	kind, err := enumField(r, "Kind")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Reference: ref, ReferenceKind: kind}, nil
}
func decodeEvidence(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	ref, err := stringField(r, "Evidence")
	if err != nil {
		return Node{}, err
	}
	kind, err := enumField(r, "Kind")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Reference: ref, ReferenceKind: kind, EvidenceState: "not run"}, nil
}
func decodeArtifact(r interpret.RecordValue) (Node, error) {
	id, err := stringField(r, "ID")
	if err != nil {
		return Node{}, err
	}
	ref, err := stringField(r, "Artifact")
	if err != nil {
		return Node{}, err
	}
	output, err := stringField(r, "Output")
	if err != nil {
		return Node{}, err
	}
	return Node{ID: id, Reference: ref, ReferenceKind: "Artifact", Output: output}, nil
}
func decodePolicy(r interpret.RecordValue) (Policy, error) {
	fields := []string{"RequireRequirementImplementation", "RequireRequirementVerifier", "RequireInterpretationCitation", "RequireClaimSupport"}
	values := make([]bool, len(fields))
	for i, field := range fields {
		value, ok := r.Fields[field]
		if !ok || value.Kind != interpret.ValueBool {
			return Policy{}, fmt.Errorf("Atlas.Policy.%s must be Bool", field)
		}
		values[i] = value.Bool
	}
	return Policy{values[0], values[1], values[2], values[3]}, nil
}
func stringField(r interpret.RecordValue, name string) (string, error) {
	value, ok := r.Fields[name]
	if !ok || value.Kind != interpret.ValueString {
		return "", fmt.Errorf("%s.%s must be String", shortType(r.TypeName), name)
	}
	return value.Text, nil
}
func enumField(r interpret.RecordValue, name string) (string, error) {
	value, ok := r.Fields[name]
	if !ok || value.Kind != interpret.ValueEnum {
		return "", fmt.Errorf("%s.%s must be an enum value", shortType(r.TypeName), name)
	}
	return value.Enum.Variant, nil
}
func recordCollection(r interpret.RecordValue, name string) ([]interpret.Value, error) {
	value, ok := r.Fields[name]
	if !ok {
		return nil, fmt.Errorf("Atlas document is missing %s", name)
	}
	if value.Kind == interpret.ValueArray {
		return value.Array, nil
	}
	if value.Kind != interpret.ValueRecord {
		return nil, fmt.Errorf("Atlas document %s must be a record array or record table", name)
	}
	table := value.Record
	extent := table.StaticExtent
	if extent == 0 && len(table.FieldOrder) > 0 {
		column := table.Fields[table.FieldOrder[0]]
		if column.Kind != interpret.ValueArray {
			return nil, fmt.Errorf("Atlas table %s has non-array column %s", name, table.FieldOrder[0])
		}
		extent = len(column.Array)
	}
	rows := make([]interpret.Value, extent)
	for row := 0; row < extent; row++ {
		fields := make(map[string]interpret.Value, len(table.Fields))
		for _, field := range table.FieldOrder {
			column := table.Fields[field]
			if column.Kind != interpret.ValueArray || len(column.Array) != extent {
				return nil, fmt.Errorf("Atlas table %s column %s has inconsistent extent", name, field)
			}
			fields[field] = column.Array[row]
		}
		rows[row] = interpret.Value{Kind: interpret.ValueRecord, Record: interpret.RecordValue{TypeName: table.TypeName, FieldOrder: append([]string(nil), table.FieldOrder...), Fields: fields}}
	}
	return rows, nil
}
func shortType(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (g *Graph) resolveReferences() error {
	for i := range g.Nodes {
		n := &g.Nodes[i]
		switch n.Kind {
		case AuthorityKind:
			if n.Digest == "" && n.URI != "" && !strings.Contains(n.URI, "://") {
				path := n.URI
				if !filepath.IsAbs(path) {
					path = filepath.Join(g.Root, path)
				}
				if body, err := os.ReadFile(path); err == nil {
					n.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(body))
				}
			}
		case SymbolKind:
			source, err := resolveSymbol(g.program, n.Reference, n.ReferenceKind, false)
			if err != nil {
				return fmt.Errorf("Atlas implementation reference %q does not resolve: %w", n.Reference, err)
			}
			n.Source = source
		case EvidenceKind:
			source, err := resolveSymbol(g.program, n.Reference, n.ReferenceKind, true)
			if err != nil {
				return fmt.Errorf("Atlas %s reference %q does not resolve: %w", n.ReferenceKind, n.Reference, err)
			}
			n.Source = source
		case ArtifactKind:
			source, err := resolveSymbol(g.program, n.Reference, "Artifact", true)
			if err != nil {
				return fmt.Errorf("Atlas Artifact reference %q does not resolve: %w", n.Reference, err)
			}
			n.Source = source
			if n.Output == "" || filepath.IsAbs(n.Output) || strings.HasPrefix(filepath.Clean(n.Output), "..") {
				return fmt.Errorf("Atlas Artifact reference %q has invalid logical output %q", n.Reference, n.Output)
			}
		}
	}
	return nil
}

func resolveSymbol(program project.Program, qualified, kind string, evidence bool) (Source, error) {
	parts := strings.Split(qualified, ".")
	if len(parts) < 2 {
		return Source{}, fmt.Errorf("expected Package.Symbol identity")
	}
	pkgName, name := strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
	pkg, ok := program.Packages[pkgName]
	if !ok {
		return Source{}, fmt.Errorf("package %s is not loaded", pkgName)
	}
	if evidence || kind == "Function" || kind == "Artifact" {
		for _, fn := range pkg.Functions {
			if fn.Name == name {
				valid := kind == "Function" || kind == "Artifact" && fn.IsArtifact || kind == "Fact" && fn.IsFact || kind == "Theory" && fn.IsTheory
				if !valid {
					return Source{}, fmt.Errorf("%s.%s exists but is not a %s", pkgName, name, kind)
				}
				return declarationSource(program.Root, pkgName, fn.SourcePath, "fn", name), nil
			}
		}
		return Source{}, fmt.Errorf("%s %s.%s was not found", kind, pkgName, name)
	}
	if kind == "Flow" {
		for _, d := range pkg.Flows {
			if d.Name == name {
				return findPackageDeclaration(program.Root, pkg, pkgName, "flow", name), nil
			}
		}
	}
	if kind == "Concept" {
		for _, d := range pkg.Concepts {
			if d.Name == name {
				return findPackageDeclaration(program.Root, pkg, pkgName, "concept", name), nil
			}
		}
	}
	if kind == "Record" {
		for _, d := range pkg.Records {
			if d.Name == name && !d.IsConcept {
				return findPackageDeclaration(program.Root, pkg, pkgName, "record", name), nil
			}
		}
	}
	if kind == "Enum" {
		for _, d := range pkg.Enums {
			if d.Name == name {
				return findPackageDeclaration(program.Root, pkg, pkgName, "enum", name), nil
			}
		}
	}
	return Source{}, fmt.Errorf("%s %s.%s was not found", kind, pkgName, name)
}

func declarationSource(root, pkg, path, keyword, name string) Source {
	result := Source{Package: pkg, Path: slashRelative(root, path), Line: 1, Column: 1}
	body, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	needle := keyword + " " + name
	for i, line := range strings.Split(string(body), "\n") {
		if col := strings.Index(strings.TrimLeft(line, " \t"), needle); col == 0 {
			result.Line, result.Column = i+1, len(line)-len(strings.TrimLeft(line, " \t"))+1
			break
		}
	}
	return result
}
func findPackageDeclaration(root string, pkg project.Package, pkgName, keyword, name string) Source {
	entries, _ := os.ReadDir(pkg.Directory)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".oct" {
			continue
		}
		path := filepath.Join(pkg.Directory, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(body), "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, keyword+" "+name) {
				return Source{Package: pkgName, Path: slashRelative(root, path), Line: i + 1, Column: len(line) - len(trimmed) + 1}
			}
		}
	}
	return Source{Package: pkgName, Path: slashRelative(root, pkg.Directory), Line: 1, Column: 1}
}
func slashRelative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func (g *Graph) Validate() error {
	index := map[string]Node{}
	for _, n := range g.Nodes {
		if !idPattern.MatchString(n.ID) {
			return fmt.Errorf("invalid Atlas ID %q: use dot-separated ASCII letters, digits, _, or -", n.ID)
		}
		if first, ok := index[n.ID]; ok {
			return fmt.Errorf("duplicate Atlas ID %q (first declared as %s at %s, again as %s at %s)", n.ID, first.Kind, formatSource(first.DeclaredAt), n.Kind, formatSource(n.DeclaredAt))
		}
		index[n.ID] = n
	}
	allowedRelations := map[string]bool{"Supports": true, "Interprets": true, "Implements": true, "Verifies": true, "DerivedFrom": true, "DependsOn": true, "Supersedes": true, "Explains": true, "Produces": true, "Contradicts": true}
	for _, edge := range g.Links {
		from, fromOK := index[edge.From]
		to, toOK := index[edge.To]
		if !fromOK {
			return fmt.Errorf("Atlas link references unknown node %q", edge.From)
		}
		if !toOK {
			return fmt.Errorf("Atlas link references unknown node %q", edge.To)
		}
		if !allowedRelations[edge.Relation] {
			return fmt.Errorf("Atlas link uses unknown relation %q", edge.Relation)
		}
		if err := validateEndpoint(edge, from, to); err != nil {
			return err
		}
	}
	for _, n := range g.Nodes {
		if n.Kind == CitationKind {
			authority, ok := index[n.Authority]
			if !ok || authority.Kind != AuthorityKind {
				return fmt.Errorf("Atlas Citation %q references unknown Authority %q", n.ID, n.Authority)
			}
		}
	}
	for _, relation := range []string{"Supersedes", "DerivedFrom"} {
		if cycle := cycleFor(g.Links, relation); len(cycle) > 0 {
			return fmt.Errorf("Atlas %s cycle: %s", relation, strings.Join(cycle, " -> "))
		}
	}
	return nil
}

func formatSource(source Source) string {
	if source.Path == "" {
		return "unknown source"
	}
	return fmt.Sprintf("%s:%d:%d", source.Path, source.Line, source.Column)
}

func validateEndpoint(e Link, from, to Node) error {
	fail := func() error {
		return fmt.Errorf("Atlas relation %s cannot link %s %q to %s %q", e.Relation, from.Kind, from.ID, to.Kind, to.ID)
	}
	switch e.Relation {
	case "Interprets":
		if from.Kind != InterpretationKind || to.Kind != CitationKind && to.Kind != AuthorityKind {
			return fail()
		}
	case "Implements":
		if from.Kind != SymbolKind || to.Kind != RequirementKind && to.Kind != ClaimKind {
			return fail()
		}
	case "Verifies":
		if from.Kind != EvidenceKind || to.Kind != RequirementKind && to.Kind != ClaimKind {
			return fail()
		}
	case "DerivedFrom":
		if from.Kind != ClaimKind || to.Kind != ClaimKind {
			return fail()
		}
	case "Supersedes":
		if from.ID == to.ID {
			return fmt.Errorf("Atlas node %q cannot supersede itself", from.ID)
		}
		if from.Kind != to.Kind || from.Kind == SymbolKind || from.Kind == EvidenceKind || from.Kind == ArtifactKind {
			return fail()
		}
	case "Explains":
		if from.Kind != ArtifactKind {
			return fail()
		}
	case "Contradicts":
		if from.Kind == SymbolKind || from.Kind == EvidenceKind || from.Kind == ArtifactKind || to.Kind == SymbolKind || to.Kind == EvidenceKind || to.Kind == ArtifactKind {
			return fail()
		}
	}
	return nil
}

func cycleFor(edges []Link, relation string) []string {
	adj := map[string][]string{}
	nodes := map[string]bool{}
	for _, e := range edges {
		if e.Relation == relation {
			adj[e.From] = append(adj[e.From], e.To)
			nodes[e.From], nodes[e.To] = true, true
		}
	}
	for k := range adj {
		sort.Strings(adj[k])
	}
	state := map[string]int{}
	stack := []string{}
	var found []string
	var visit func(string) bool
	visit = func(n string) bool {
		state[n] = 1
		stack = append(stack, n)
		for _, next := range adj[n] {
			if state[next] == 1 {
				start := 0
				for stack[start] != next {
					start++
				}
				found = append(append([]string{}, stack[start:]...), next)
				return true
			}
			if state[next] == 0 && visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = 2
		return false
	}
	keys := make([]string, 0, len(nodes))
	for n := range nodes {
		keys = append(keys, n)
	}
	sort.Strings(keys)
	for _, n := range keys {
		if state[n] == 0 && visit(n) {
			return found
		}
	}
	return nil
}

func (g Graph) measure() (Coverage, []string) {
	var c Coverage
	var warnings []string
	incoming, outgoing := map[string][]Link{}, map[string][]Link{}
	for _, e := range g.Links {
		incoming[e.To] = append(incoming[e.To], e)
		outgoing[e.From] = append(outgoing[e.From], e)
		if e.Relation == "Contradicts" {
			c.Contradictions++
		}
		if e.Relation == "Supersedes" {
			c.Superseded++
		}
	}
	for _, n := range g.Nodes {
		switch n.Kind {
		case RequirementKind:
			c.RequirementsTotal++
			implemented, verified := hasRelation(incoming[n.ID], "Implements"), hasRelation(incoming[n.ID], "Verifies")
			if implemented {
				c.RequirementsImplemented++
			} else {
				warnings = append(warnings, fmt.Sprintf("Requirement %s has no implementation", n.ID))
			}
			if verified {
				c.RequirementsVerified++
			} else {
				warnings = append(warnings, fmt.Sprintf("Requirement %s has no verifier", n.ID))
			}
		case ClaimKind:
			c.ClaimsTotal++
			supported := hasRelation(outgoing[n.ID], "DerivedFrom") || hasRelation(incoming[n.ID], "Supports") || hasRelation(incoming[n.ID], "Verifies")
			if supported {
				c.ClaimsSupported++
			} else {
				warnings = append(warnings, fmt.Sprintf("Claim %s has no support or derivation", n.ID))
			}
		case InterpretationKind:
			c.InterpretationsTotal++
			cited := hasRelation(outgoing[n.ID], "Interprets")
			if cited {
				c.InterpretationsCited++
			} else {
				warnings = append(warnings, fmt.Sprintf("Interpretation %s has no citation", n.ID))
			}
		case EvidenceKind:
			linked := len(outgoing[n.ID])+len(incoming[n.ID]) > 0
			if n.ReferenceKind == "Fact" {
				c.FactsTotal++
				if linked {
					c.FactsLinked++
				}
			} else {
				c.TheoriesTotal++
				if linked {
					c.TheoriesLinked++
				}
			}
			if !linked {
				warnings = append(warnings, fmt.Sprintf("%s %s is linked to nothing", n.ReferenceKind, n.ID))
			}
		case ArtifactKind:
			c.ArtifactsTotal++
			if len(outgoing[n.ID])+len(incoming[n.ID]) > 0 {
				c.ArtifactsLinked++
			} else {
				warnings = append(warnings, fmt.Sprintf("Artifact %s is linked to nothing", n.ID))
			}
		}
	}
	sort.Strings(warnings)
	c.Orphans = len(warnings)
	return c, warnings
}
func hasRelation(edges []Link, relation string) bool {
	for _, e := range edges {
		if e.Relation == relation {
			return true
		}
	}
	return false
}

func (g Graph) PolicyErrors() []string {
	var out []string
	if g.Policy.RequireRequirementImplementation && g.Coverage.RequirementsImplemented != g.Coverage.RequirementsTotal {
		out = append(out, "project policy requires every Requirement to have an implementation")
	}
	if g.Policy.RequireRequirementVerifier && g.Coverage.RequirementsVerified != g.Coverage.RequirementsTotal {
		out = append(out, "project policy requires every Requirement to have a verifier")
	}
	if g.Policy.RequireInterpretationCitation && g.Coverage.InterpretationsCited != g.Coverage.InterpretationsTotal {
		out = append(out, "project policy requires every Interpretation to cite an Authority or Citation")
	}
	if g.Policy.RequireClaimSupport && g.Coverage.ClaimsSupported != g.Coverage.ClaimsTotal {
		out = append(out, "project policy requires every Claim to have support or derivation")
	}
	return out
}

func (g Graph) Node(id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

func (g Graph) AffectedBy(id string, depth int) ([]Node, error) {
	if _, ok := g.Node(id); !ok {
		return nil, fmt.Errorf("unknown Atlas ID %q", id)
	}
	if depth <= 0 {
		depth = 4
	}
	impact := map[string][]string{}
	for _, e := range g.Links {
		switch e.Relation {
		case "Supports", "Produces":
			impact[e.From] = append(impact[e.From], e.To)
		case "Contradicts":
			impact[e.From] = append(impact[e.From], e.To)
			impact[e.To] = append(impact[e.To], e.From)
		default:
			// Subject-to-object grammar means the subject depends on the
			// object for Interprets, Implements, Verifies, DerivedFrom,
			// DependsOn, Supersedes, and Explains.
			impact[e.To] = append(impact[e.To], e.From)
		}
	}
	for k := range impact {
		sort.Strings(impact[k])
	}
	type item struct {
		id    string
		depth int
	}
	queue := []item{{id, 0}}
	seen := map[string]bool{id: true}
	var out []Node
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= depth {
			continue
		}
		for _, next := range impact[cur.id] {
			if seen[next] {
				continue
			}
			seen[next] = true
			n, _ := g.Node(next)
			out = append(out, n)
			queue = append(queue, item{next, cur.depth + 1})
		}
	}
	return out, nil
}

func sourceBytes(program project.Program) int64 {
	var total int64
	seen := map[string]bool{}
	for _, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if seen[fn.SourcePath] {
				continue
			}
			seen[fn.SourcePath] = true
			if info, err := os.Stat(fn.SourcePath); err == nil {
				total += info.Size()
			}
		}
	}
	return total
}

func WriteOctagon(path string, g Graph) error {
	nodes := append([]Node(nil), g.Nodes...)
	links := append([]Link(nil), g.Links...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(links, func(i, j int) bool {
		if links[i].From != links[j].From {
			return links[i].From < links[j].From
		}
		if links[i].Relation != links[j].Relation {
			return links[i].Relation < links[j].Relation
		}
		return links[i].To < links[j].To
	})
	var b strings.Builder
	b.WriteString("AtlasGraph {\n    FormatVersion: 1\n    Package: " + strconv.Quote(g.Package) + "\n    Nodes: [\n")
	for i, n := range nodes {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "        AtlasNode { ID: %s Kind: %s Title: %s Text: %s URI: %s Version: %s Digest: %s Authority: %s Locator: %s Reference: %s ReferenceKind: %s Output: %s EvidenceState: %s DeclaredPath: %s DeclaredLine: %d DeclaredColumn: %d SourcePackage: %s SourcePath: %s SourceLine: %d SourceColumn: %d }", q(n.ID), q(string(n.Kind)), q(n.Title), q(n.Text), q(n.URI), q(n.Version), q(n.Digest), q(n.Authority), q(n.Locator), q(n.Reference), q(n.ReferenceKind), q(n.Output), q(n.EvidenceState), q(n.DeclaredAt.Path), n.DeclaredAt.Line, n.DeclaredAt.Column, q(n.Source.Package), q(n.Source.Path), n.Source.Line, n.Source.Column)
	}
	b.WriteString("\n    ]\n    Edges: [\n")
	for i, e := range links {
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "        AtlasEdge { From: %s Relation: %s To: %s }", q(e.From), q(e.Relation), q(e.To))
	}
	b.WriteString("\n    ]\n}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
func q(s string) string { return strconv.Quote(s) }

// Sorted returns stable node and edge copies for presentation and tests.
func (g Graph) Sorted() ([]Node, []Link) {
	nodes := append([]Node(nil), g.Nodes...)
	links := append([]Link(nil), g.Links...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(links, func(i, j int) bool {
		if links[i].From != links[j].From {
			return links[i].From < links[j].From
		}
		if links[i].Relation != links[j].Relation {
			return links[i].Relation < links[j].Relation
		}
		return links[i].To < links[j].To
	})
	return nodes, links
}
