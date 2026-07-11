package validate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// TestKind is the validator-owned classification of a .sdslvtest declaration.
type TestKind string

const (
	TestKindFact   TestKind = "Fact"
	TestKindTheory TestKind = "Theory"
)

// ConstValue preserves a validated scalar InlineData value without requiring a
// downstream consumer to inspect AST expressions or source text.
type ConstValue struct {
	Kind, Text string
	Bool       bool
	Span       source.Span
}
type TestStableIdentity struct {
	Source, Function string
	Kind             TestKind
}
type TestAttributeSpans struct {
	Fact, Theory                  source.Span
	InlineRows                    []source.Span
	WorkgroupSize, DispatchGroups source.Span
}

// ValidatedTestDecl is the sole compiler-owned M29 declaration handoff. It is
// immutable by convention: slices are built by the validator and consumers
// project them without attributing new semantics to source text or AST nodes.
type ValidatedTestDecl struct {
	Function       ast.FunctionDecl
	Kind           TestKind
	StableIdentity TestStableIdentity
	InlineRows     []ValidatedTheoryRow
	Launch         ValidatedLaunchMetadata
	AssertCalls    []ValidatedAssertCall
	RequiresGPU    bool
	ForeignTargets []string
	Capabilities   []string
	FunctionSpan   source.Span
	AttributeSpans TestAttributeSpans
}
type ValidatedTheoryRow struct {
	Index      int
	Values     []ConstValue
	RowSpan    source.Span
	ValueSpans []source.Span
	Identity   string
}
type ValidatedLaunchMetadata struct {
	WorkgroupSize, DispatchGroups [3]uint32
	WorkgroupSpan                 source.Span
	WorkgroupArgSpans             [3]source.Span
	DispatchSpan                  source.Span
	DispatchArgSpans              [3]source.Span
}
type AssertKind string
type ValidatedAssertCall struct {
	Kind         AssertKind
	Call         ast.CallExpr
	Operands     []ast.Expr
	CallSpan     source.Span
	OperandSpans []source.Span
	LexicalIndex int
}

// ValidatedTestCase is the canonical per-replay projection. Stable ID creation
// lives here so grouping and artifact ordering cannot affect case identity.
type ValidatedTestCase struct {
	Decl                  ValidatedTestDecl
	Row                   *ValidatedTheoryRow
	StableID, DisplayName string
}

func ValidatedTests(module ast.Module) ([]ValidatedTestDecl, []diagnostic.Diagnostic) {
	if diagnostics := Diagnostics(module); len(diagnostics) != 0 {
		return nil, diagnostics
	}
	var out []ValidatedTestDecl
	for _, decl := range module.Decls {
		fn, ok := decl.(ast.FunctionDecl)
		if !ok {
			continue
		}
		kind, spans, ok := validatedTestKind(fn)
		if !ok {
			continue
		}
		d := ValidatedTestDecl{Function: fn, Kind: kind, StableIdentity: TestStableIdentity{Source: module.Source.Path, Function: fn.Name, Kind: kind}, FunctionSpan: fn.Span, AttributeSpans: spans, Launch: ValidatedLaunchMetadata{WorkgroupSize: [3]uint32{1, 1, 1}, DispatchGroups: [3]uint32{1, 1, 1}}}
		for i := range fn.Attributes {
			a := fn.Attributes[i]
			switch a.Name {
			case "InlineData":
				row := ValidatedTheoryRow{Index: len(d.InlineRows), RowSpan: a.Span}
				for _, e := range a.Arguments {
					v := constValue(e)
					row.Values = append(row.Values, v)
					row.ValueSpans = append(row.ValueSpans, v.Span)
				}
				row.Identity = theoryIdentity(row)
				d.InlineRows = append(d.InlineRows, row)
				d.AttributeSpans.InlineRows = append(d.AttributeSpans.InlineRows, a.Span)
			case "WorkgroupSize":
				d.Launch.WorkgroupSize = launchValues(a.Arguments)
				d.Launch.WorkgroupSpan = a.Span
				d.Launch.WorkgroupArgSpans = argumentSpans(a.Arguments)
			case "DispatchGroups":
				d.Launch.DispatchGroups = launchValues(a.Arguments)
				d.Launch.DispatchSpan = a.Span
				d.Launch.DispatchArgSpans = argumentSpans(a.Arguments)
			}
		}
		d.AssertCalls = assertionCalls(fn.Body)
		d.RequiresGPU = containsForeignShader(fn.Body)
		d.ForeignTargets = foreignTargets(fn.Body)
		out = append(out, d)
	}
	return out, nil
}

func foreignTargets(b ast.Block) []string {
	seen := map[string]bool{}
	var addExpr func(ast.Expr)
	addExpr = func(e ast.Expr) {
		switch x := e.(type) {
		case ast.ForeignShaderExpr:
			seen[x.TargetLanguage] = true
		case ast.CallExpr:
			addExpr(x.Callee)
			for _, a := range x.Arguments {
				addExpr(a)
			}
		case ast.BinaryExpr:
			addExpr(x.Left)
			addExpr(x.Right)
		case ast.UnaryExpr:
			addExpr(x.Operand)
		case ast.ParenExpr:
			addExpr(x.Inner)
		}
	}
	var walk func(ast.Block)
	walk = func(block ast.Block) {
		for _, s := range block.Statements {
			switch x := s.(type) {
			case ast.ForeignShaderStmt:
				seen[x.TargetLanguage] = true
			case ast.ExprStmt:
				addExpr(x.Value)
			case ast.LetStmt:
				addExpr(x.Value)
			case ast.ReturnStmt:
				addExpr(x.Value)
			case ast.IfStmt:
				addExpr(x.Condition)
				walk(x.ThenBody)
				if x.ElseBody != nil {
					walk(*x.ElseBody)
				}
			}
		}
	}
	walk(b)
	out := make([]string, 0, len(seen))
	for target := range seen {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

func containsForeignShader(b ast.Block) bool {
	var expr func(ast.Expr) bool
	expr = func(e ast.Expr) bool {
		switch x := e.(type) {
		case ast.ForeignShaderExpr:
			return true
		case ast.CallExpr:
			if expr(x.Callee) {
				return true
			}
			for _, a := range x.Arguments {
				if expr(a) {
					return true
				}
			}
		case ast.BinaryExpr:
			return expr(x.Left) || expr(x.Right)
		case ast.UnaryExpr:
			return expr(x.Operand)
		case ast.ParenExpr:
			return expr(x.Inner)
		}
		return false
	}
	for _, s := range b.Statements {
		switch x := s.(type) {
		case ast.ForeignShaderStmt:
			return true
		case ast.ExprStmt:
			if expr(x.Value) {
				return true
			}
		case ast.LetStmt:
			if expr(x.Value) {
				return true
			}
		case ast.ReturnStmt:
			if expr(x.Value) {
				return true
			}
		case ast.IfStmt:
			if expr(x.Condition) || containsForeignShader(x.ThenBody) || (x.ElseBody != nil && containsForeignShader(*x.ElseBody)) {
				return true
			}
		}
	}
	return false
}

func validatedTestKind(fn ast.FunctionDecl) (TestKind, TestAttributeSpans, bool) {
	var s TestAttributeSpans
	for _, a := range fn.Attributes {
		switch a.Name {
		case "Fact":
			s.Fact = a.Span
			return TestKindFact, s, true
		case "Theory":
			s.Theory = a.Span
			return TestKindTheory, s, true
		}
	}
	return "", s, false
}
func constValue(e ast.Expr) ConstValue {
	s := ast.ExprSpan(e)
	switch x := e.(type) {
	case ast.IntegerLiteral:
		return ConstValue{Kind: "integer", Text: x.Value, Span: s}
	case ast.FloatLiteral:
		return ConstValue{Kind: "float", Text: x.Value, Span: s}
	case ast.BoolLiteral:
		return ConstValue{Kind: "bool", Text: strconv.FormatBool(x.Value), Bool: x.Value, Span: s}
	case ast.StringLiteral:
		return ConstValue{Kind: "string", Text: x.Value, Span: s}
	}
	return ConstValue{Span: s}
}
func theoryIdentity(r ValidatedTheoryRow) string {
	values := make([]string, len(r.Values))
	for i, v := range r.Values {
		values[i] = v.Text
	}
	return fmt.Sprintf("%d\x00%s", r.Index, strings.Join(values, "\x00"))
}
func launchValues(args []ast.Expr) (out [3]uint32) {
	for i, e := range args {
		x := e.(ast.IntegerLiteral)
		n, _ := strconv.ParseUint(strings.TrimRight(x.Value, "uU"), 10, 32)
		out[i] = uint32(n)
	}
	return
}
func argumentSpans(args []ast.Expr) (out [3]source.Span) {
	for i, e := range args {
		out[i] = ast.ExprSpan(e)
	}
	return
}

// ValidatedTestCases assigns replay identities solely from validated metadata.
func ValidatedTestCases(tests []ValidatedTestDecl, source string) []ValidatedTestCase {
	var out []ValidatedTestCase
	for _, d := range tests {
		d.StableIdentity.Source = source
		if d.Kind == TestKindFact {
			out = append(out, newValidatedCase(d, nil))
			continue
		}
		for i := range d.InlineRows {
			row := d.InlineRows[i]
			out = append(out, newValidatedCase(d, &row))
		}
	}
	return out
}
func newValidatedCase(d ValidatedTestDecl, row *ValidatedTheoryRow) ValidatedTestCase {
	identity := d.StableIdentity.Source + "\x00" + d.StableIdentity.Function + "\x00" + string(d.Kind)
	display := d.StableIdentity.Function
	if row != nil {
		identity += "\x00" + row.Identity
		display += fmt.Sprintf("[%d]", row.Index)
	}
	sum := sha256.Sum256([]byte(identity))
	return ValidatedTestCase{Decl: d, Row: row, StableID: "sdslv-" + hex.EncodeToString(sum[:12]), DisplayName: display}
}

func assertionCalls(b ast.Block) []ValidatedAssertCall {
	var out []ValidatedAssertCall
	var walkExpr func(ast.Expr)
	walkExpr = func(e ast.Expr) {
		switch x := e.(type) {
		case ast.CallExpr:
			if name := testAssertName(x.Callee); name != "" {
				spans := make([]source.Span, len(x.Arguments))
				for i, a := range x.Arguments {
					spans[i] = ast.ExprSpan(a)
				}
				out = append(out, ValidatedAssertCall{Kind: AssertKind(name), Call: x, Operands: x.Arguments, CallSpan: x.Span, OperandSpans: spans, LexicalIndex: len(out)})
			}
			walkExpr(x.Callee)
			for _, a := range x.Arguments {
				walkExpr(a)
			}
		case ast.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case ast.UnaryExpr:
			walkExpr(x.Operand)
		case ast.ParenExpr:
			walkExpr(x.Inner)
		case ast.FieldAccessExpr:
			walkExpr(x.Target)
		case ast.IndexExpr:
			walkExpr(x.Target)
			walkExpr(x.Index)
			if x.HasSecond {
				walkExpr(x.Index2)
			}
		}
	}
	var walkBlock func(ast.Block)
	walkBlock = func(block ast.Block) {
		for _, s := range block.Statements {
			switch x := s.(type) {
			case ast.ExprStmt:
				walkExpr(x.Value)
			case ast.LetStmt:
				walkExpr(x.Value)
			case ast.ReturnStmt:
				walkExpr(x.Value)
			case ast.IfStmt:
				walkExpr(x.Condition)
				walkBlock(x.ThenBody)
				if x.ElseBody != nil {
					walkBlock(*x.ElseBody)
				}
			}
		}
	}
	walkBlock(b)
	return out
}
