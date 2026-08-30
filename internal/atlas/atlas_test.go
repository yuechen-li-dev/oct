package atlas

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func semanticNode(id string, kind Kind) Node { return Node{ID: id, Kind: kind} }

func TestValidateRejectsDuplicateIDUnknownEndpointAndCycles(t *testing.T) {
	tests := []struct {
		name  string
		graph Graph
		want  string
	}{
		{name: "duplicate", graph: Graph{Nodes: []Node{semanticNode("Claim.X", ClaimKind), semanticNode("Claim.X", ClaimKind)}}, want: "duplicate Atlas ID \"Claim.X\""},
		{name: "unknown endpoint", graph: Graph{Nodes: []Node{semanticNode("Claim.X", ClaimKind)}, Links: []Link{{From: "Claim.X", Relation: "DependsOn", To: "Claim.Missing"}}}, want: "unknown node \"Claim.Missing\""},
		{name: "self supersede", graph: Graph{Nodes: []Node{semanticNode("Requirement.X", RequirementKind)}, Links: []Link{{From: "Requirement.X", Relation: "Supersedes", To: "Requirement.X"}}}, want: "cannot supersede itself"},
		{name: "derived cycle", graph: Graph{Nodes: []Node{semanticNode("Claim.A", ClaimKind), semanticNode("Claim.B", ClaimKind)}, Links: []Link{{From: "Claim.A", Relation: "DerivedFrom", To: "Claim.B"}, {From: "Claim.B", Relation: "DerivedFrom", To: "Claim.A"}}}, want: "DerivedFrom cycle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.graph.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func TestAffectedByUsesRelationSemantics(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			semanticNode("Citation.Source", CitationKind), semanticNode("Interpretation.Bounded", InterpretationKind),
			semanticNode("Requirement.Rule", RequirementKind), semanticNode("Symbol.Code", SymbolKind),
			semanticNode("Evidence.Test", EvidenceKind), semanticNode("Artifact.Report", ArtifactKind),
		},
		Links: []Link{
			{From: "Citation.Source", Relation: "Supports", To: "Requirement.Rule"},
			{From: "Interpretation.Bounded", Relation: "Interprets", To: "Citation.Source"},
			{From: "Symbol.Code", Relation: "Implements", To: "Requirement.Rule"},
			{From: "Evidence.Test", Relation: "Verifies", To: "Requirement.Rule"},
			{From: "Artifact.Report", Relation: "Explains", To: "Requirement.Rule"},
		},
	}
	nodes, err := g.AffectedBy("Citation.Source", 4)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(nodes))
	for i := range nodes {
		got[i] = nodes[i].ID
	}
	want := []string{"Interpretation.Bounded", "Requirement.Rule", "Artifact.Report", "Evidence.Test", "Symbol.Code"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("affected traversal\nwant %v\ngot  %v", want, got)
	}
}

func TestCoverageReportsOrphansAndContradictions(t *testing.T) {
	g := Graph{Nodes: []Node{
		semanticNode("Requirement.Tracked", RequirementKind), semanticNode("Claim.A", ClaimKind), semanticNode("Claim.B", ClaimKind),
		{ID: "Evidence.F", Kind: EvidenceKind, ReferenceKind: "Fact"}, semanticNode("Symbol.F", SymbolKind),
	}, Links: []Link{
		{From: "Symbol.F", Relation: "Implements", To: "Requirement.Tracked"},
		{From: "Claim.A", Relation: "Contradicts", To: "Claim.B"},
	}}
	c, warnings := g.measure()
	if c.RequirementsImplemented != 1 || c.RequirementsVerified != 0 || c.Contradictions != 1 {
		t.Fatalf("unexpected coverage: %+v", c)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "no verifier") {
		t.Fatalf("expected verifier orphan warning, got %v", warnings)
	}
}

func TestWriteOctagonIsDeterministicAndParseable(t *testing.T) {
	g := Graph{Package: "Fixture", Nodes: []Node{semanticNode("Claim.B", ClaimKind), semanticNode("Claim.A", ClaimKind)}, Links: []Link{{From: "Claim.B", Relation: "DependsOn", To: "Claim.A"}}}
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.octagon"), filepath.Join(dir, "second.octagon")
	if err := WriteOctagon(first, g); err != nil {
		t.Fatal(err)
	}
	if err := WriteOctagon(second, g); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(first)
	b, _ := os.ReadFile(second)
	if !bytes.Equal(a, b) {
		t.Fatal("canonical Atlas serialization changed between identical writes")
	}
	if _, err := octagon.Load(first); err != nil {
		t.Fatalf("compiled Atlas artifact is not valid Octagon: %v\n%s", err, a)
	}
	if strings.Index(string(a), "Claim.A") > strings.Index(string(a), "Claim.B") {
		t.Fatal("nodes are not sorted by stable ID")
	}
}

func TestPolicyAndRiemannDogfoodCompile(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	policy, err := Compile(filepath.Join(repoRoot, "Experiments", "PolicyLab", "M0"))
	if err != nil {
		t.Fatalf("Policy Lab Atlas: %v", err)
	}
	if len(policy.PolicyErrors()) != 0 || policy.Coverage.RequirementsVerified != 2 {
		t.Fatalf("Policy Lab policy/coverage: errors=%v coverage=%+v", policy.PolicyErrors(), policy.Coverage)
	}
	for _, node := range policy.Nodes {
		if node.DeclaredAt.Path == "" || node.DeclaredAt.Line == 0 {
			t.Fatalf("columnar node %s lost declaration provenance: %+v", node.ID, node.DeclaredAt)
		}
	}
	riemann, err := Compile(filepath.Join(repoRoot, "Experiments", "RiemannAtlas", "M0"))
	if err != nil {
		t.Fatalf("Riemann Atlas: %v", err)
	}
	node, ok := riemann.Node("Claim.Riemann.ContactPreservingTwoAtomComponentGlobalOptimum")
	if !ok || !strings.Contains(node.Text, "not a proof of the Riemann Hypothesis") {
		t.Fatalf("bounded Riemann claim missing or overbroad: %+v", node)
	}
}
