// Package test owns SDSL-V test-suite discovery.  It deliberately does not
// share Prometheus' SGEMM dispatch contract: .sdslvtest cases are transient
// compiler artifacts with a fixed assertion-result interface.
package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lower"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const ResultABIVersion uint32 = 1

// InvocationResult is the versioned, fixed-width GPU-to-host ABI.  Every
// field is a 32-bit word so its HLSL/StructuredBuffer layout is unambiguous.
// It is intentionally not a public resource-fixture API.
type InvocationResult struct {
	ABIVersion, Failed, AssertionID, SourceLine, SourceColumn uint32
	InvocationX, InvocationY, InvocationZ                     uint32
	ValueKind, ComponentCount                                 uint32
	ExpectedBits, ActualBits, ToleranceBits                   [4]uint32
}

type Launch struct {
	WorkgroupSize  [3]uint32 `json:"workgroup_size"`
	DispatchGroups [3]uint32 `json:"dispatch_groups"`
}
type Case struct {
	StableID         string                           `json:"stable_id"`
	DisplayName      string                           `json:"display_name"`
	Source           string                           `json:"source"`
	Function         string                           `json:"function"`
	Kind             string                           `json:"kind"`
	TheoryRow        *int                             `json:"theory_row,omitempty"`
	InlineData       []string                         `json:"inline_data,omitempty"`
	Launch           Launch                           `json:"launch"`
	RequiresGPU      bool                             `json:"requires_gpu"`
	ForeignTargets   []string                         `json:"foreign_targets"`
	Capabilities     []string                         `json:"capabilities"`
	FunctionSpan     source.Span                      `json:"function_span"`
	AttributeSpans   validate.TestAttributeSpans      `json:"attribute_spans"`
	RowSpan          *source.Span                     `json:"row_span,omitempty"`
	ValueSpans       []source.Span                    `json:"value_spans,omitempty"`
	TypedValues      []validate.ConstValue            `json:"typed_values,omitempty"`
	LaunchMetadata   validate.ValidatedLaunchMetadata `json:"launch_metadata"`
	Assertions       []validate.ValidatedAssertCall   `json:"assertions"`
	StableIdentity   validate.TestStableIdentity      `json:"stable_identity"`
	TestInputBinding uint32                           `json:"test_input_binding"`
	TestInput        validate.ValidatedTestInput      `json:"test_input"`
	GroupID          string                           `json:"group_id"`
	HLSLPath         string                           `json:"hlsl_path,omitempty"`
	SPIRVPath        string                           `json:"spirv_path,omitempty"`
}
type Manifest struct {
	SchemaVersion    int    `json:"schema_version"`
	ResultABIVersion uint32 `json:"result_abi_version"`
	Source           string `json:"source"`
	Cases            []Case `json:"cases"`
	Interface        struct {
		DescriptorSet uint32 `json:"descriptor_set"`
		Binding       uint32 `json:"binding"`
		Resource      string `json:"resource"`
	} `json:"fixed_interface"`
}

// Discover performs normal parsing and validation, then projects the
// validator-owned declarations into the durable host manifest. It does not
// interpret test attributes, rows, launches, or stable identity.
func Discover(path string) (Manifest, error) {
	suite, err := Prepare(path)
	if err != nil {
		return Manifest{}, err
	}
	return ProjectManifest(suite, nil), nil
}

// discoverAST is the M29a compiler-front-end authority.  It deliberately
// consumes ordinary lexer/parser nodes; M29b will consume the same functions
// for assertion/body lowering.
func Prepare(path string) (Suite, error) {
	file, err := source.Load(path)
	if err != nil {
		return Suite{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return Suite{}, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return Suite{}, err
	}
	validated, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		return Suite{}, diagnostic.Error(diagnostics)
	}
	mir, err := lower.ModuleForTests(module, validated, "HLSL")
	if err != nil {
		return Suite{}, fmt.Errorf("SDSL-V test body lowering: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Suite{}, err
	}
	identity := sourceIdentity(abs)
	cases := make([]CanonicalCase, 0)
	for _, c := range validate.ValidatedTestCases(validated, identity) {
		cases = append(cases, CanonicalCase{Test: c})
	}
	if len(cases) == 0 {
		return Suite{}, fmt.Errorf("no [Fact] or [Theory] tests found in %s", path)
	}
	sort.SliceStable(cases, func(i, j int) bool { return cases[i].Test.StableID < cases[j].Test.StableID })
	return Suite{Source: identity, Cases: cases, Groups: GroupValidatedCases(cases), MIR: mir}, nil
}

// ProjectManifest is a one-way host serialization projection from canonical
// suite/group data. It contains no validation, defaults, or identity logic.
func ProjectManifest(suite Suite, artifacts []Group) Manifest {
	m := Manifest{SchemaVersion: 4, ResultABIVersion: ResultABIVersion, Source: suite.Source}
	m.Interface.DescriptorSet = 0
	m.Interface.Binding = 0
	m.Interface.Resource = "compiler-owned assertion result buffer"
	groupByCase := map[string]Group{}
	for _, g := range artifacts {
		for _, id := range g.Cases {
			groupByCase[id] = g
		}
	}
	groupID := map[string]string{}
	for _, g := range suite.Groups {
		for _, c := range g.Cases {
			groupID[c.Test.StableID] = g.ID
		}
	}
	for _, canonical := range suite.Cases {
		test := canonical.Test
		d := test.Decl
		c := Case{StableID: test.StableID, DisplayName: test.DisplayName, Source: suite.Source, Function: d.Function.Name, Kind: string(d.Kind), Launch: Launch{WorkgroupSize: d.Launch.WorkgroupSize, DispatchGroups: d.Launch.DispatchGroups}, RequiresGPU: d.RequiresGPU, ForeignTargets: d.ForeignTargets, Capabilities: d.Capabilities, FunctionSpan: d.FunctionSpan, AttributeSpans: d.AttributeSpans, LaunchMetadata: d.Launch, Assertions: d.AssertCalls, StableIdentity: d.StableIdentity, TestInputBinding: 1, TestInput: d.TestInput, GroupID: groupID[test.StableID]}
		if g, ok := groupByCase[test.StableID]; ok {
			c.HLSLPath = g.HLSLPath
			c.SPIRVPath = g.SPIRVPath
		}
		if test.Row != nil {
			row := test.Row.Index
			c.TheoryRow = &row
			c.RowSpan = &test.Row.RowSpan
			c.ValueSpans = test.Row.ValueSpans
			c.TypedValues = test.Row.Values
			c.InlineData = make([]string, len(test.Row.Values))
			for i, v := range test.Row.Values {
				c.InlineData[i] = v.Text
			}
		}
		m.Cases = append(m.Cases, c)
	}
	return m
}

// sourceIdentity avoids machine-specific absolute paths for suites under the
// current project root while preserving a canonical absolute fallback for an
// explicitly external file.  It is the durable component of replay identity.
func sourceIdentity(abs string) string {
	wd, err := os.Getwd()
	if err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
