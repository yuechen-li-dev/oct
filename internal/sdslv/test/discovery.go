// Package test owns SDSL-V test-suite discovery.  It deliberately does not
// share Prometheus' SGEMM dispatch contract: .sdslvtest cases are transient
// compiler artifacts with a fixed assertion-result interface.
package test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
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
	StableID    string   `json:"stable_id"`
	DisplayName string   `json:"display_name"`
	Source      string   `json:"source"`
	Function    string   `json:"function"`
	Kind        string   `json:"kind"`
	TheoryRow   *int     `json:"theory_row,omitempty"`
	InlineData  []string `json:"inline_data,omitempty"`
	Launch      Launch   `json:"launch"`
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

// Discover validates the deliberately small M29 attribute surface and returns
// deterministic per-Fact/per-row cases.  Source parsing remains owned by the
// SDSL-V parser; this scanner only reads file-level test annotations, which
// are not valid production SDSL-V declarations.
func Discover(path string) (Manifest, error) {
	return discoverAST(path)
}

// discoverAST is the M29a compiler-front-end authority.  It deliberately
// consumes ordinary lexer/parser nodes; M29b will consume the same functions
// for assertion/body lowering.
func discoverAST(path string) (Manifest, error) {
	file, err := source.Load(path)
	if err != nil {
		return Manifest{}, err
	}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return Manifest{}, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return Manifest{}, err
	}
	validated, diagnostics := validate.ValidatedTests(module)
	if len(diagnostics) != 0 {
		return Manifest{}, diagnostic.Error(diagnostics)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}
	identity := sourceIdentity(abs)
	m := Manifest{SchemaVersion: 1, ResultABIVersion: ResultABIVersion, Source: identity}
	m.Interface.DescriptorSet = 0
	m.Interface.Binding = 0
	m.Interface.Resource = "compiler-owned assertion result buffer"
	for _, test := range validated {
		if test.Fact != nil {
			m.Cases = append(m.Cases, newCase(identity, test.Function.Name, "Fact", nil, nil, Launch{WorkgroupSize: test.Launch.WorkgroupSize, DispatchGroups: test.Launch.DispatchGroups}))
			continue
		}
		for row, values := range test.Rows {
			inline := make([]string, len(values.Values))
			for i, value := range values.Values {
				inline[i], _ = attributeLiteral(value)
			}
			r := row
			m.Cases = append(m.Cases, newCase(identity, test.Function.Name, "Theory", &r, inline, Launch{WorkgroupSize: test.Launch.WorkgroupSize, DispatchGroups: test.Launch.DispatchGroups}))
		}
	}
	if len(m.Cases) == 0 {
		return Manifest{}, fmt.Errorf("no [Fact] or [Theory] tests found in %s", path)
	}
	sort.SliceStable(m.Cases, func(i, j int) bool { return m.Cases[i].StableID < m.Cases[j].StableID })
	return m, nil
}

func attributeLiteral(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case ast.IntegerLiteral:
		return x.Value, true
	case ast.FloatLiteral:
		return x.Value, true
	case ast.BoolLiteral:
		if x.Value {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
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

func newCase(source, fn, kind string, row *int, values []string, launch Launch) Case {
	identity := source + "\x00" + fn + "\x00" + kind
	display := fn
	if row != nil {
		identity += fmt.Sprintf("\x00%d\x00%s", *row, strings.Join(values, "\x00"))
		display += fmt.Sprintf("[%d]", *row)
	}
	sum := sha256.Sum256([]byte(identity))
	return Case{StableID: "sdslv-" + hex.EncodeToString(sum[:12]), DisplayName: display, Source: filepath.ToSlash(source), Function: fn, Kind: kind, TheoryRow: row, InlineData: values, Launch: launch}
}
func WriteManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
