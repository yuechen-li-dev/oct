package build

import (
	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
	"github.com/yuechen-li-dev/oct/internal/project"
)

type MIRModule struct {
	EntryPackage    string
	EntryFunc       string
	EntryReturn     string
	EntryFallible   bool
	Records         []MIRRecord
	Refinements     []MIRRefinement
	Enums           []MIREnum
	Flows           []MIRFlow
	Functions       []MIRFunction
	LayoutContracts []layoutcontract.Contract
	Selectors       []MIRSelector
	Templates       []MIRTemplateSpecialization
}

// MIRTemplateSpecialization is compile-time-only authoring provenance. It is
// emitted as a generated-source comment and never participates in execution.
type MIRTemplateSpecialization struct {
	Kind          string
	Package       string
	ConcreteName  string
	OriginPackage string
	OriginName    string
	TypeArguments []string
}

// MIRSelector is compile-time provenance for a selector getter. Execution uses
// the ordinary concrete function in Functions; semantic consumers reuse the
// existing exact-subject FieldRef instead of a second field-identity system.
type MIRSelector struct {
	Package string
	Name    string
	Owner   string
	Result  string
	Field   layoutcontract.FieldRef
}

type MIRRefinement struct {
	Package string
	Name    string
	Base    string
}

type MIRRecord struct {
	Package string
	Name    string
	Fields  []MIRField
	Kind    MIRRecordKind
	Subject layoutcontract.DataSubjectRef
}

type MIRRecordKind string

const (
	MIRRecordOrdinary  MIRRecordKind = "record"
	MIRRecordTableKind MIRRecordKind = "record-table"
	MIRRecordTableRow  MIRRecordKind = "synthetic-table-row"
)

type MIREnum struct {
	Package  string
	Name     string
	Variants []MIREnumVariant
}

type MIREnumVariant struct {
	Name        string
	PayloadType string
}

type MIRField struct {
	Name string
	Type string
}

type MIRCapture struct {
	Name      string
	Parameter string
	Type      string
}

type MIRFunction struct {
	Package         string
	Name            string
	Params          []MIRField
	CaptureEnv      []MIRCapture
	Return          string
	IsFallible      bool
	ErrorType       string
	Locals          []MIRField
	Blocks          []MIRBlock
	UsesUtilityWhen bool
}

type MIRBlock struct {
	Label      string
	Statements []MIRStmt
	Terminator MIRTerminator
}

type MIRStmt interface{ mirStmt() }

type MIRAssign struct {
	Target string
	Value  MIRValue
}

func (MIRAssign) mirStmt() {}

// MIRRowAssign is a whole-row write to a two-dimensional array.  It remains
// distinct from MIRAssign so emission can retain Oct's value-copy semantics
// and its row-specific runtime checks.
type MIRRowAssign struct {
	Target string
	Index  MIRValue
	Value  MIRValue
}

func (MIRRowAssign) mirStmt() {}

type MIRIndexAssign struct {
	Target  string
	Indices []MIRValue
	Value   MIRValue
}

func (MIRIndexAssign) mirStmt() {}

type MIRCall struct {
	Target        string
	Callee        string
	Args          []MIRValue
	ArgTypes      []string
	Builtin       bool
	RetType       string
	FunctionValue bool
}

func (MIRCall) mirStmt() {}

type MIRGenericOctxiliaryCall struct {
	Target         string
	PackageName    string
	OctName        string
	Family         string
	WireName       string
	SidecarCommand string
	Args           []MIRValue
	ArgTypes       []string
	RetType        string
	Fallible       bool
	TransportTypes []project.TransportTypeMetadata
}

func (MIRGenericOctxiliaryCall) mirStmt() {}

type MIRDestructureCall struct {
	Targets  []string
	Callee   string
	Args     []MIRValue
	Builtin  bool
	RetTypes []string
}

func (MIRDestructureCall) mirStmt() {}

type MIRConstructRecord struct {
	Target              string
	TypeName            string
	FieldNames          []string
	FieldVals           []MIRValue
	TemplateOrigin      string
	TemplateOverrideSet []string
}

func (MIRConstructRecord) mirStmt() {}

type MIRConstructArray struct {
	Target   string
	ElemType string
	Values   []MIRValue
}

func (MIRConstructArray) mirStmt() {}

type MIRBatchMap struct {
	Target     string
	Input      MIRValue
	Worker     string
	InputType  string
	ResultType string
	Captures   []MIRValue
	Nested     bool
}

func (MIRBatchMap) mirStmt() {}

type MIRTerminator interface{ mirTerminator() }

type MIRReturn struct{ Value MIRValue }

func (MIRReturn) mirTerminator() {}

type MIRJump struct{ Target string }

func (MIRJump) mirTerminator() {}

type MIRBranch struct {
	Cond                    MIRValue
	TrueTarget, FalseTarget string
}

func (MIRBranch) mirTerminator() {}

type MIRFail struct{ Value MIRValue }

func (MIRFail) mirTerminator() {}
